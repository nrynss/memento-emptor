package server

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/standing"
)

// bootCorrectionService builds one boot of the served layer over the
// standing directory: a fresh evidence store, as every boot has, over the
// graph it is given. Two services over one directory and one graph are the
// layer before and after a restart against the same durable hub.
func bootCorrectionService(t *testing.T, dir string, graph *spyGraph) *Service {
	t.Helper()
	store, err := evidence.NewStore(testContract{kind: "correction-boot"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	fixed := time.Date(2026, 11, 8, 18, 40, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:    fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier:  classifier,
		Store:       store,
		Graph:       graph,
		SchemaDir:   apiDir(t),
		Clock:       func() time.Time { return fixed },
		StandingDir: dir,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

// correctionWrites counts the graph's structural writes of one correction:
// the record_action calls that declare those words against a belief.
func correctionWrites(spy *spyGraph, text string) int {
	spy.mu.Lock()
	defer spy.mu.Unlock()
	out := 0
	for _, action := range spy.actions {
		if action.action == text && len(action.against) == 1 {
			out++
		}
	}
	return out
}

// seededCorrection returns the frozen seed's correction view.
func seededCorrection(t *testing.T, seed Seed) contract.CorrectionView {
	t.Helper()
	for _, belief := range seed.Beliefs {
		if belief.ContradictedBy != nil {
			return *belief.ContradictedBy
		}
	}
	t.Fatal("the frozen seed carries no correction")
	return contract.CorrectionView{}
}

// saveCorrections writes the corrections journal a boot leaves: the
// standing directory holds the correction views, as the durable store
// does.
func saveCorrections(t *testing.T, ctx context.Context, dir, shopper string, views []contract.CorrectionView) {
	t.Helper()
	store, err := standing.NewDirStore(dir)
	if err != nil {
		t.Fatalf("open standing store: %v", err)
	}
	if err := store.Save(ctx, shopper, standing.Snapshot{Pooling: true, Corrections: views}); err != nil {
		t.Fatalf("save the corrections journal: %v", err)
	}
}

// TestBootReplayDoesNotRewriteARestoredCorrection proves the boot replay
// writes a correction's graph declaration once for its life, not once per
// boot. The first boot over an empty standing directory writes the frozen
// seed's correction; the journal then holds its view, and the second boot
// over the same directory and the same durable graph writes nothing for
// it. Both boots serve the same corrections, each once and in its one
// place in the arrival order.
func TestBootReplayDoesNotRewriteARestoredCorrection(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	seed, err := SeedFromExamples(apiDir(t))
	if err != nil {
		t.Fatalf("build seed: %v", err)
	}
	shopper := seed.ShopperID
	correction := seededCorrection(t, seed)

	// One graph across both boots: the hub's store outlives the layer's
	// process, so a write the first boot issued is still held when the
	// second boot starts.
	spy := &spyGraph{inner: fakes.NewRecordingEvidenceGraph(SeedGraphContents(seed), fakes.StoryRecalls())}

	first := bootCorrectionService(t, dir, spy)
	if err := first.LoadSeed(ctx, seed); err != nil {
		t.Fatalf("load seed on the first boot: %v", err)
	}
	if got := correctionWrites(spy, correction.Text); got != 1 {
		t.Fatalf("the first boot wrote the correction %d times, want its one declaration", got)
	}
	before, err := first.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: seededFitBelief})
	if err != nil {
		t.Fatalf("explain on the first boot: %v", err)
	}
	if len(before.Corrections) != 1 || before.Corrections[0].CorrectionID != correction.CorrectionID {
		t.Fatalf("the first boot serves %+v, want the seed's one correction", before.Corrections)
	}

	// The durable journal after the first boot's writes: it holds the
	// correction's view under its id.
	saveCorrections(t, ctx, dir, shopper, []contract.CorrectionView{correction})

	second := bootCorrectionService(t, dir, spy)
	if err := second.LoadSeed(ctx, seed); err != nil {
		t.Fatalf("load seed on the second boot: %v", err)
	}
	if got := correctionWrites(spy, correction.Text); got != 1 {
		t.Fatalf("the second boot re-wrote the restored correction: %d graph writes, want the first boot's one", got)
	}
	after, err := second.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: seededFitBelief})
	if err != nil {
		t.Fatalf("explain on the second boot: %v", err)
	}
	if !reflect.DeepEqual(before.Corrections, after.Corrections) {
		t.Fatalf("the boots serve different corrections:\nfirst:  %+v\nsecond: %+v", before.Corrections, after.Corrections)
	}
	if len(after.Corrections) != 1 {
		t.Fatalf("the second boot lists the correction %d times, want once", len(after.Corrections))
	}
	ledger, err := second.LedgerBelief(ctx, contract.LedgerBeliefRequest{ShopperID: shopper, BeliefID: seededFitBelief})
	if err != nil {
		t.Fatalf("ledger read on the second boot: %v", err)
	}
	if !reflect.DeepEqual(after.Corrections, ledger.Corrections) {
		t.Fatalf("explain and the ledger serve different corrections:\nexplain: %+v\nledger:  %+v", after.Corrections, ledger.Corrections)
	}

	second.mu.Lock()
	order := append([]string(nil), second.shoppers[shopper].corrOrder...)
	second.mu.Unlock()
	if len(order) != 1 || order[0] != correction.CorrectionID {
		t.Fatalf("the second boot's order holds %v, want the restored id once", order)
	}
}
