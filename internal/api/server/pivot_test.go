package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
	"github.com/nrynss/memento-emptor/internal/ingest"
	"github.com/nrynss/memento-emptor/internal/pool"
)

// fakePoolIndex answers the pool reader from a canned result or a canned
// error. An error answers ErrUnavailable the way a refused connection
// does, so recall proves it carries on without the pool.
type fakePoolIndex struct {
	mu      sync.Mutex
	result  pool.Result
	err     error
	queries []pool.Query
	warmed  int
}

// Warm records the warm call.
func (f *fakePoolIndex) Warm(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.warmed++
	return nil
}

// Aggregate records the query and answers the canned result or error.
func (f *fakePoolIndex) Aggregate(ctx context.Context, q pool.Query) (pool.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, q)
	if f.err != nil {
		return pool.Result{}, f.err
	}
	return f.result, nil
}

// lastQuery returns the most recent query.
func (f *fakePoolIndex) lastQuery() pool.Query {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.queries[len(f.queries)-1]
}

// earnedSmallPool returns the settled counts behind an Earned small pool:
// 29 small outcomes of 47 fit outcomes with 25 distinct shoppers, against
// 6 large. The seeded and live splits and the recent window earn the same
// way, so the label cannot flip on the window.
func earnedSmallPool() pool.Result {
	counts := func(settled, kept, retSmall, retLarge, fw, fbSmall, fbLarge, fitTooSmall, fitTooLarge int, shoppers pool.ShopperCounts) pool.Counts {
		return pool.Counts{
			Settled: settled,
			Kept:    kept,
			Returns: pool.DirectionCounts{TooSmall: retSmall, TooLarge: retLarge},
			Feedback: pool.VerdictCounts{
				FitsWell: fw,
				TooSmall: fbSmall,
				TooLarge: fbLarge,
			},
			Reasons: map[domain.Category]int{
				domain.CategoryFitTooSmall: fitTooSmall,
				domain.CategoryFitTooLarge: fitTooLarge,
				"damaged":                  2,
				"not_as_described":         1,
				domain.CategoryChangedMind: 4,
			},
			Shoppers: shoppers,
		}
	}
	all := counts(60, 6, 24, 4, 12, 5, 2, 24, 4, pool.ShopperCounts{Fit: 39, TooSmall: 25, TooLarge: 6})
	seeded := counts(48, 5, 20, 3, 9, 4, 2, 20, 3, pool.ShopperCounts{Fit: 30, TooSmall: 19, TooLarge: 5})
	live := counts(12, 1, 4, 1, 3, 1, 0, 4, 1, pool.ShopperCounts{Fit: 9, TooSmall: 6, TooLarge: 1})
	recent := counts(44, 5, 20, 3, 8, 4, 1, 20, 3, pool.ShopperCounts{Fit: 22, TooSmall: 18, TooLarge: 3})
	return pool.Result{All: all, Seeded: seeded, Live: live, Recent: recent}
}

// fakeFitIndex keeps fit rows in memory. When incomplete is positive the
// next deletes report a partial failure, so the opt-out proves its retry.
type fakeFitIndex struct {
	mu         sync.Mutex
	rows       map[string]domain.FitRow
	puts       int
	deletes    int
	incomplete int
}

// Create succeeds without doing anything.
func (f *fakeFitIndex) Create(ctx context.Context) error { return nil }

// Put stores one purchase's whole document.
func (f *fakeFitIndex) Put(ctx context.Context, row domain.FitRow) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rows == nil {
		f.rows = map[string]domain.FitRow{}
	}
	f.rows[row.TxnHash] = row
	f.puts++
	return nil
}

// DeleteShopper removes one shopper's rows in one line. While incomplete
// is positive the call skips its first matching row and reports the
// partial failure, so the opt-out proves its retry.
func (f *fakeFitIndex) DeleteShopper(_ context.Context, line domain.ProductLine, shopperHash string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes++
	removed := 0
	skipped := false
	for id, row := range f.rows {
		if row.Line == line && row.ShopperHash == shopperHash {
			if f.incomplete > 0 && !skipped {
				f.incomplete--
				skipped = true
				continue
			}
			delete(f.rows, id)
			removed++
		}
	}
	if skipped {
		return removed, fmt.Errorf("%w: %d rows went, the rest failed", fitindex.ErrIncompleteDelete, removed)
	}
	return removed, nil
}

// rowCount reports how many documents the index holds.
func (f *fakeFitIndex) rowCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.rows)
}

// staticLines resolves every item to its brand and category with the
// regular cut: the tests' catalog, which the demo path leaves unwired.
func staticLines(_ context.Context, item domain.Item) (domain.ProductLine, error) {
	return domain.ProductLine{Brand: item.BrandID, Category: item.Category, FitHandle: "regular"}, nil
}

// pivotAllowlist covers the pivot story's graph contents: the story's
// own world plus the pooling notes the opt-out derives.
func pivotAllowlist() []string {
	return append(storyAllowlist(), "shopper opted out of pooling", "shopper opted back in to pooling")
}

// pivotHarness builds a service with the pivot half wired: a fake pool
// index, a fake fit index, the seed identity salt and a static line
// resolver. A nil pool index leaves pool reads off; a nil fit index
// leaves fit writes off. The standing store is in memory unless dir is
// set, in which case journals survive the harness.
func pivotHarness(t *testing.T, poolIdx pool.Index, fitIdx fitindex.Index, dir string) (*Service, *spyGraph) {
	t.Helper()
	store, err := evidence.NewStore(testContract{kind: "pivot"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	spy := &spyGraph{inner: fakes.NewRecordingEvidenceGraph(pivotAllowlist(), nil)}
	fixed := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	deps := Deps{
		Payments:   fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      spy,
		SchemaDir:  apiDir(t),
		Clock:      func() time.Time { return fixed },
		Pool:       poolIdx,
		FitRows:    fitIdx,
		Lines:      fitindex.LinesFunc(staticLines),
	}
	if dir == "" {
		deps.IdentitySalt = identity.SeedSaltHex
	} else {
		deps.IdentitySalt = identity.SeedSaltHex
		deps.StandingDir = dir
	}
	svc, err := NewService(deps)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc, spy
}

// answerTwoLarge records two too-large answers for one shopper's first
// two refunds, earning a Building runs-large belief.
func answerTwoLarge(t *testing.T, svc *Service, shopper string) {
	t.Helper()
	ctx := context.Background()
	for _, event := range []string{"WH-2EV-01", "WH-2EV-02"} {
		if _, err := svc.IngestWebhook(ctx, shopper, event); err != nil {
			t.Fatalf("ingest %s: %v", event, err)
		}
		if res := answerReason(t, svc, shopper, event, "it swims on me, far too big"); !res.Accepted {
			t.Fatalf("too-large answer for %s was not accepted", event)
		}
	}
}

// TestRecallNewShopperDecidedByPool proves the day-one move no single
// shopper's history can make: a shopper with no memory at all gets the
// pooled belief adjusted from the size under consideration, with the
// pool's counts riding along for the citation.
func TestRecallNewShopperDecidedByPool(t *testing.T) {
	ctx := context.Background()
	poolIdx := &fakePoolIndex{result: earnedSmallPool()}
	svc, _ := pivotHarness(t, poolIdx, &fakeFitIndex{}, "")
	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: "new_shopper", BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(recall.Beliefs) != 0 {
		t.Fatalf("new shopper recall serves %d beliefs, want none", len(recall.Beliefs))
	}
	if recall.Pool == nil || recall.PoolLookup != contract.PoolConsulted {
		t.Fatalf("new shopper recall carries no consulted pool: %+v", recall.Pool)
	}
	if recall.Pool.Status != domain.StatusEarned || recall.Pool.Direction == nil ||
		*recall.Pool.Direction != domain.FitDirectionSmall {
		t.Fatalf("pool reads %+v, want Earned small", recall.Pool)
	}
	if recall.Pool.Precedence == nil || recall.Pool.Precedence.Step != contract.GuidancePool || recall.Pool.Precedence.Overridden {
		t.Fatalf("pool precedence %+v, want the deciding pool step", recall.Pool.Precedence)
	}
	if recall.Pool.Seeded.Settled == 0 || recall.Pool.Live.Settled == 0 {
		t.Fatalf("pool hides its seeded split: %+v", recall.Pool)
	}
	if recall.Guidance.Basis != contract.GuidancePool || recall.Guidance.BeliefID != nil {
		t.Fatalf("guidance %+v, want a pool decision with no belief", recall.Guidance)
	}
	want := "Size up to L for brand-x-001: 29 of the 47 fit-classified settled purchases at size M came back too small, against 6 too large."
	if recall.Guidance.Text != want {
		t.Fatalf("guidance text %q, want %q", recall.Guidance.Text, want)
	}
	query := poolIdx.lastQuery()
	if query.Line != (domain.ProductLine{Brand: "brand-x-001", Category: "shirts", FitHandle: "regular"}) || query.Size != "M" {
		t.Fatalf("pool query %+v, want the line and the size under consideration", query)
	}
}

// TestRecallUnlikeCrowdDecidedByOwnEvidence proves naive pooling fails
// here on purpose: the shopper's own Building runs-large belief beats
// the Earned small pool, and both ride the response with the pool marked
// overridden, never hidden.
func TestRecallUnlikeCrowdDecidedByOwnEvidence(t *testing.T) {
	ctx := context.Background()
	poolIdx := &fakePoolIndex{result: earnedSmallPool()}
	svc, _ := pivotHarness(t, poolIdx, &fakeFitIndex{}, "")
	shopper := "unlike_shopper"
	answerTwoLarge(t, svc, shopper)
	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: shopper, BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(recall.Beliefs) != 1 {
		t.Fatalf("recall serves %d beliefs, want the 1 Building own belief", len(recall.Beliefs))
	}
	belief := recall.Beliefs[0]
	if belief.Status != domain.StatusBuilding || belief.Standing != domain.GraphCandidate {
		t.Fatalf("own belief reads %q/%q, want Building/Candidate", belief.Status, belief.Standing)
	}
	if belief.Precedence == nil || belief.Precedence.Step != contract.GuidanceOwnEvidence || belief.Precedence.Overridden {
		t.Fatalf("own belief precedence %+v, want the deciding own step", belief.Precedence)
	}
	if recall.Pool == nil || recall.Pool.Precedence == nil ||
		!recall.Pool.Precedence.Overridden || *recall.Pool.Precedence.OverriddenBy != contract.GuidanceOwnEvidence {
		t.Fatalf("pool precedence %+v, want overridden by own_evidence", recall.Pool.Precedence)
	}
	if recall.Guidance.Basis != contract.GuidanceOwnEvidence {
		t.Fatalf("guidance %+v, want own_evidence", recall.Guidance)
	}
	want := "Take size S for Brand X: your own two returns say it runs large, ahead of the pool."
	if recall.Guidance.Text != want {
		t.Fatalf("guidance text %q, want %q", recall.Guidance.Text, want)
	}
}

// TestRecallServesOneReturnBesideThePool proves thin evidence is served,
// never withheld: a lone too-large return makes a Claimed belief with one
// row, recall carries it with its own-evidence step, and the Earned pool
// rides beside it marked overridden. Before this rule the one-return belief
// was dropped, so the pool decided a shopper whose own history disagreed
// with it.
func TestRecallServesOneReturnBesideThePool(t *testing.T) {
	ctx := context.Background()
	poolIdx := &fakePoolIndex{result: earnedSmallPool()}
	svc, _ := pivotHarness(t, poolIdx, &fakeFitIndex{}, "")
	shopper := "thin_shopper"
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-01"); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res := answerReason(t, svc, shopper, "WH-2EV-01", "it swims on me, far too big"); !res.Accepted {
		t.Fatal("the too-large answer was not accepted")
	}
	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: shopper, BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(recall.Beliefs) != 1 {
		t.Fatalf("one return serves %d beliefs, want the 1 thin fit belief", len(recall.Beliefs))
	}
	belief := recall.Beliefs[0]
	if belief.Status != domain.StatusClaimed || belief.Standing != domain.GraphNone {
		t.Fatalf("thin belief reads %q/%q, want Claimed/None: thin evidence is labelled, not withheld",
			belief.Status, belief.Standing)
	}
	if len(belief.Evidence) != 1 {
		t.Fatalf("thin belief carries %d evidence rows, want its one", len(belief.Evidence))
	}
	if belief.Precedence == nil || belief.Precedence.Step != contract.GuidanceOwnEvidence || belief.Precedence.Overridden {
		t.Fatalf("thin belief precedence %+v, want the deciding own-evidence step", belief.Precedence)
	}
	if recall.Pool == nil || recall.Pool.Precedence == nil ||
		!recall.Pool.Precedence.Overridden || *recall.Pool.Precedence.OverriddenBy != contract.GuidanceOwnEvidence {
		t.Fatalf("pool precedence %+v, want the pool beside it overridden by own_evidence", recall.Pool.Precedence)
	}
	if recall.Guidance.Basis != contract.GuidanceOwnEvidence {
		t.Fatalf("guidance %+v, want own_evidence", recall.Guidance)
	}
	want := "Take size S for Brand X: your own one return says it runs large, ahead of the pool."
	if recall.Guidance.Text != want {
		t.Fatalf("guidance text %q, want %q", recall.Guidance.Text, want)
	}
}

// TestRecallUnreachablePoolCarriesOn proves recall never pretends an
// empty pool: when the index cannot be reached the lookup reads
// unavailable, the pool object is absent, and the shopper's own evidence
// decides alone.
func TestRecallUnreachablePoolCarriesOn(t *testing.T) {
	ctx := context.Background()
	poolIdx := &fakePoolIndex{err: pool.ErrUnavailable}
	svc, _ := pivotHarness(t, poolIdx, &fakeFitIndex{}, "")
	shopper := "offline_shopper"
	answerTwoLarge(t, svc, shopper)
	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: shopper, BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if recall.Pool != nil || recall.PoolLookup != contract.PoolUnavailable {
		t.Fatalf("unreachable pool reads %+v/%q, want absent/unavailable", recall.Pool, recall.PoolLookup)
	}
	if recall.Guidance.Basis != contract.GuidanceOwnEvidence {
		t.Fatalf("guidance %+v, want own_evidence deciding alone", recall.Guidance)
	}
	want := "Take size S for Brand X"
	if recall.Guidance.Text != want {
		t.Fatalf("guidance text %q, want %q", recall.Guidance.Text, want)
	}
}

// TestStandingCorrectionHonouredAcrossSessions proves a correction with
// no belief stands on its own: it decides the very next recall and is
// still in force in a later session, with identical repeats staying one
// correction that never refreshes its event time.
func TestStandingCorrectionHonouredAcrossSessions(t *testing.T) {
	ctx := context.Background()
	svc, _ := pivotHarness(t, nil, nil, "")
	shopper := "standing_shopper"
	first, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: "I've lost weight, use M",
	})
	if err != nil {
		t.Fatalf("record standing correction: %v", err)
	}
	repeat, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: "I've lost weight, use M",
	})
	if err != nil {
		t.Fatalf("repeat standing correction: %v", err)
	}
	if repeat.Correction.CorrectionID != first.Correction.CorrectionID ||
		repeat.Correction.RecordedAt != first.Correction.RecordedAt {
		t.Fatalf("identical repeat is a new correction: %+v vs %+v", repeat.Correction, first.Correction)
	}
	svc.mu.Lock()
	journalLen := svc.shoppers[shopper].journal.Len()
	svc.mu.Unlock()
	if journalLen != 0 {
		t.Fatalf("unscoped standing journal holds %d entries, want none: it spans every brand and is never declared", journalLen)
	}
	if first.BeliefID != "" || first.Contradicted || first.Status != domain.StatusClaimed || first.Standing != domain.GraphNone {
		t.Fatalf("standing correction answers %+v, want a lone Claimed note", first)
	}
	if first.Guidance.Basis != contract.GuidanceCorrection || first.Guidance.Text != "Take size M" ||
		first.Guidance.BeliefID != nil {
		t.Fatalf("standing guidance %+v, want the correction alone", first.Guidance)
	}
	recall, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall after standing: %v", err)
	}
	if recall.OverridingCorrection == nil || recall.Guidance.Basis != contract.GuidanceCorrection {
		t.Fatalf("standing correction does not decide: %+v", recall.Guidance)
	}
	later, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001", TraceID: "later-session"})
	if err != nil {
		t.Fatalf("later recall: %v", err)
	}
	if later.OverridingCorrection == nil ||
		later.OverridingCorrection.CorrectionID != recall.OverridingCorrection.CorrectionID ||
		later.Guidance.Text != "Take size M" {
		t.Fatalf("standing correction lapsed in a later session: %+v", later.Guidance)
	}
}

// TestNamedStandingDeclaredWhenBeliefArrives proves the journal path: a
// standing recorded with brand context is declared against the matching
// belief the moment the belief is written, and recall shows the warning
// beside the standing belief.
func TestNamedStandingDeclaredWhenBeliefArrives(t *testing.T) {
	ctx := context.Background()
	svc, _ := pivotHarness(t, nil, nil, "")
	shopper := "named_shopper"
	if _, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"}); err != nil {
		t.Fatalf("first recall: %v", err)
	}
	standing, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: "I've lost weight, use M",
	})
	if err != nil {
		t.Fatalf("record standing correction: %v", err)
	}
	if standing.BeliefID != "" {
		t.Fatalf("standing with no belief contradicts %q", standing.BeliefID)
	}
	for _, event := range []string{"WH-2EV-01", "WH-2EV-02", "WH-2EV-03"} {
		if _, err := svc.IngestWebhook(ctx, shopper, event); err != nil {
			t.Fatalf("ingest %s: %v", event, err)
		}
		answerReason(t, svc, shopper, event, "too tight in the shoulders")
	}
	recall, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall after belief: %v", err)
	}
	if recall.OverridingCorrection == nil || recall.Guidance.Basis != contract.GuidanceCorrection {
		t.Fatalf("named standing does not decide after its belief: %+v", recall.Guidance)
	}
	if len(recall.Beliefs) != 1 || !recall.Beliefs[0].Contradicted {
		t.Fatalf("belief carries no contradiction: %+v", recall.Beliefs)
	}
	inline := recall.Beliefs[0].ContradictedBy
	if inline == nil || inline.CorrectionID != recall.OverridingCorrection.CorrectionID ||
		inline.ContradictsBeliefID != recall.Beliefs[0].BeliefID {
		t.Fatalf("correction is not declared against the belief: %+v", inline)
	}
	if recall.Guidance.BeliefID == nil || *recall.Guidance.BeliefID != recall.Beliefs[0].BeliefID {
		t.Fatalf("guidance names no contradicted belief: %+v", recall.Guidance)
	}
}

// TestStandingSurvivesRestart proves the journal outlives the process: a
// second service over the same directory re-records the undeclared
// standing, so the next belief still declares it.
func TestStandingSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	svc, _ := pivotHarness(t, nil, nil, dir)
	shopper := "restart_shopper"
	if _, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"}); err != nil {
		t.Fatalf("first recall: %v", err)
	}
	if _, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: "I've lost weight, use M",
	}); err != nil {
		t.Fatalf("record standing correction: %v", err)
	}
	restarted, spy := pivotHarness(t, nil, nil, dir)
	recall, err := restarted.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall after restart: %v", err)
	}
	if recall.OverridingCorrection == nil || recall.Guidance.Basis != contract.GuidanceCorrection {
		t.Fatalf("standing lapsed across the restart: %+v", recall.Guidance)
	}
	spy.mu.Lock()
	derived := append([]string(nil), spy.derives...)
	spy.mu.Unlock()
	found := false
	for _, content := range derived {
		if content == "I've lost weight, use M" {
			found = true
		}
	}
	if !found {
		t.Fatalf("restart re-recorded no standing note: %v", derived)
	}
}

// TestSetPoolingOptsOut proves the consent path: opting out records the
// choice in the shopper's own memory, removes the rows already written
// across every line the session knows, and stops new rows. Opting back
// in removes nothing.
func TestSetPoolingOptsOut(t *testing.T) {
	ctx := context.Background()
	fitIdx := &fakeFitIndex{}
	svc, spy := pivotHarness(t, nil, fitIdx, "")
	shopper := "pooling_shopper"
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-06"); err != nil {
		t.Fatalf("ingest capture: %v", err)
	}
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-01"); err != nil {
		t.Fatalf("ingest refund: %v", err)
	}
	answerReason(t, svc, shopper, "WH-2EV-01", "too tight in the shoulders")
	if fitIdx.rowCount() != 2 {
		t.Fatalf("fit index holds %d rows, want the capture and the refund", fitIdx.rowCount())
	}
	out, err := svc.SetPooling(ctx, contract.SetPoolingRequest{ShopperID: shopper, Pooling: false})
	if err != nil {
		t.Fatalf("set pooling: %v", err)
	}
	if out.Pooling || out.RowsRemoved != 2 {
		t.Fatalf("opt-out answers %+v, want pooling false with 2 rows removed", out)
	}
	if fitIdx.rowCount() != 0 {
		t.Fatalf("fit index holds %d rows after opt-out, want none", fitIdx.rowCount())
	}
	spy.mu.Lock()
	derived := append([]string(nil), spy.derives...)
	spy.mu.Unlock()
	found := false
	for _, content := range derived {
		if content == "shopper opted out of pooling" {
			found = true
		}
	}
	if !found {
		t.Fatalf("opt-out left no record in the shopper's memory: %v", derived)
	}
	put := fitIdx.puts
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-02"); err != nil {
		t.Fatalf("ingest after opt-out: %v", err)
	}
	answerReason(t, svc, shopper, "WH-2EV-02", "too tight in the shoulders")
	if fitIdx.puts != put {
		t.Fatalf("opted-out shopper wrote %d fit rows, want none", fitIdx.puts-put)
	}
	back, err := svc.SetPooling(ctx, contract.SetPoolingRequest{ShopperID: shopper, Pooling: true})
	if err != nil {
		t.Fatalf("opt back in: %v", err)
	}
	if !back.Pooling || back.RowsRemoved != 0 {
		t.Fatalf("opt-in answers %+v, want pooling true with nothing removed", back)
	}
}

// TestSetPoolingRetriesIncompleteDelete proves a partial delete is not
// done: the opt-out retries the session's full line list and reports the
// rows every attempt removed.
func TestSetPoolingRetriesIncompleteDelete(t *testing.T) {
	ctx := context.Background()
	fitIdx := &fakeFitIndex{incomplete: 1}
	svc, _ := pivotHarness(t, nil, fitIdx, "")
	shopper := "retry_shopper"
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-06"); err != nil {
		t.Fatalf("ingest capture: %v", err)
	}
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-01"); err != nil {
		t.Fatalf("ingest refund: %v", err)
	}
	answerReason(t, svc, shopper, "WH-2EV-01", "too tight in the shoulders")
	out, err := svc.SetPooling(ctx, contract.SetPoolingRequest{ShopperID: shopper, Pooling: false})
	if err != nil {
		t.Fatalf("set pooling: %v", err)
	}
	if out.RowsRemoved != 2 {
		t.Fatalf("opt-out removed %d rows, want 2 across the retry", out.RowsRemoved)
	}
	if fitIdx.deletes != 2 {
		t.Fatalf("opt-out deleted %d times, want the attempt and its retry", fitIdx.deletes)
	}
	if fitIdx.rowCount() != 0 {
		t.Fatalf("fit index holds %d rows, want none", fitIdx.rowCount())
	}
}

// TestProductFitServed proves product_fit reads the pooled belief only:
// the status, the direction, the counts with their seeded split, and no
// shopper row. It carries no precedence step; recall sets that.
func TestProductFitServed(t *testing.T) {
	ctx := context.Background()
	svc, _ := pivotHarness(t, &fakePoolIndex{result: earnedSmallPool()}, nil, "")
	res, err := svc.ProductFit(ctx, contract.ProductFitRequest{
		BrandID: "brand-x-001", Category: "shirts", FitHandle: "regular", Size: "M",
	})
	if err != nil {
		t.Fatalf("product fit: %v", err)
	}
	view := res.Pool
	if view.Line != (contract.ProductLineView{BrandID: "brand-x-001", Category: "shirts", FitHandle: "regular"}) ||
		view.Size != "M" || view.Status != domain.StatusEarned {
		t.Fatalf("pool view %+v, want the Earned small line", view)
	}
	if view.Direction == nil || *view.Direction != domain.FitDirectionSmall {
		t.Fatalf("pool direction %+v, want small", view.Direction)
	}
	if view.Counts.Settled != 60 || view.Counts.Shoppers.Fit != 39 || view.Counts.Reasons.FitTooSmall != 24 {
		t.Fatalf("pool counts %+v, want the settled denominator", view.Counts)
	}
	if view.Counts.Reasons.DamagedOrDefective != 3 || view.Counts.Reasons.NotArrived != 0 {
		t.Fatalf("pool reasons %+v, want damaged 2 + not_as_described 1 under damage", view.Counts.Reasons)
	}
	if view.Seeded.Settled != 48 || view.Live.Settled != 12 || view.Recent.Settled != 44 {
		t.Fatalf("pool splits seeded %d live %d recent %d, want 48 12 44",
			view.Seeded.Settled, view.Live.Settled, view.Recent.Settled)
	}
	if view.Precedence != nil {
		t.Fatalf("product_fit carries a precedence step: %+v", view.Precedence)
	}
}

// TestProductFitWithoutPoolRefuses proves the unwired layer refuses
// rather than report an empty pool.
func TestProductFitWithoutPoolRefuses(t *testing.T) {
	ctx := context.Background()
	svc, _ := pivotHarness(t, nil, nil, "")
	if _, err := svc.ProductFit(ctx, contract.ProductFitRequest{
		BrandID: "brand-x-001", Category: "shirts", FitHandle: "regular", Size: "M",
	}); !errors.Is(err, ErrPoolUnavailable) {
		t.Fatalf("product fit without a pool answers %v, want %v", err, ErrPoolUnavailable)
	}
}

// acceptVerifier checks raw deliveries on the test webhook route.
type acceptVerifier struct {
	err error
}

// TestPivotToolsDispatched proves product_fit and set_pooling ride the
// frozen surface: dispatched by name over strict request JSON, with
// unknown fields rejected like every other tool.
func TestPivotToolsDispatched(t *testing.T) {
	ctx := context.Background()
	svc, _ := pivotHarness(t, &fakePoolIndex{result: earnedSmallPool()}, &fakeFitIndex{}, "")
	raw, _ := json.Marshal(contract.ProductFitRequest{
		BrandID: "brand-x-001", Category: "shirts", FitHandle: "regular", Size: "M",
	})
	out, err := svc.dispatch(ctx, "product_fit", raw)
	if err != nil {
		t.Fatalf("dispatch product_fit: %v", err)
	}
	var fit contract.ProductFitResponse
	decoder := json.NewDecoder(bytes.NewReader(out))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fit); err != nil {
		t.Fatalf("product_fit response does not match its Go type: %v", err)
	}
	if fit.Pool.Status != domain.StatusEarned {
		t.Fatalf("dispatched pool status %q, want Earned", fit.Pool.Status)
	}
	setRaw, _ := json.Marshal(contract.SetPoolingRequest{ShopperID: "dispatch_shopper", Pooling: true})
	setOut, err := svc.dispatch(ctx, "set_pooling", setRaw)
	if err != nil {
		t.Fatalf("dispatch set_pooling: %v", err)
	}
	var set contract.SetPoolingResponse
	decoder = json.NewDecoder(bytes.NewReader(setOut))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&set); err != nil {
		t.Fatalf("set_pooling response does not match its Go type: %v", err)
	}
	if !set.Pooling || set.RowsRemoved != 0 {
		t.Fatalf("dispatched opt-in answers %+v, want pooling true with nothing removed", set)
	}
	if _, err := svc.dispatch(ctx, "product_fit", append(raw[:len(raw)-1], ",\"bogus\":1}"...)); err == nil {
		t.Fatal("product_fit with an unknown field is accepted")
	}
}

// TestCallsScopeHonoursGuardHeader proves the agent-only call log narrows
// to one shopper when the edge scoped the request: the layer reads the
// guard's X-Memento-Shopper header and never re-reads Authorization.
func TestCallsScopeHonoursGuardHeader(t *testing.T) {
	svc, _ := pivotHarness(t, nil, nil, "")
	httpServer := httptest.NewServer(svc.Handler())
	defer httpServer.Close()
	post := func(tool, body string) {
		t.Helper()
		res, err := httpServer.Client().Post(httpServer.URL+"/tools/"+tool, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST %s: %v", tool, err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("POST %s status %d", tool, res.StatusCode)
		}
	}
	post("pending_questions", `{"shopper_id":"caller_a"}`)
	post("pending_questions", `{"shopper_id":"caller_b"}`)
	get := func(header string) []CallEntry {
		t.Helper()
		req, err := http.NewRequest("GET", httpServer.URL+"/calls", nil)
		if err != nil {
			t.Fatalf("build calls request: %v", err)
		}
		if header != "" {
			req.Header.Set("X-Memento-Shopper", header)
		}
		res, err := httpServer.Client().Do(req)
		if err != nil {
			t.Fatalf("GET /calls: %v", err)
		}
		defer res.Body.Close()
		var log struct {
			Calls []CallEntry `json:"calls"`
		}
		if err := json.NewDecoder(res.Body).Decode(&log); err != nil {
			t.Fatalf("parse calls: %v", err)
		}
		return log.Calls
	}
	if got := get(""); len(got) != 2 {
		t.Fatalf("unscoped log holds %d calls, want the agent-only whole log", len(got))
	}
	scoped := get("caller_a")
	if len(scoped) != 1 || scoped[0].ShopperID != "caller_a" {
		t.Fatalf("scoped log holds %v, want only caller_a's call", scoped)
	}
}

// TestMCPDecoderIsStrict proves the MCP decoder takes one object with no
// unknown fields: trailing data and unknown envelope fields fail the
// protocol, and unknown tool arguments fail the call.
func TestMCPDecoderIsStrict(t *testing.T) {
	svc, _ := pivotHarness(t, nil, nil, "")
	httpServer := httptest.NewServer(svc.Handler())
	defer httpServer.Close()
	post := func(body string) (int, map[string]any) {
		t.Helper()
		res, err := httpServer.Client().Post(httpServer.URL+"/mcp", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST /mcp: %v", err)
		}
		defer res.Body.Close()
		var decoded map[string]any
		if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
			t.Fatalf("parse rpc response: %v", err)
		}
		return res.StatusCode, decoded
	}
	call := func(params string) string {
		return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` + params + `}`
	}
	if _, res := post(call(`{"name":"pending_questions","arguments":{"shopper_id":"s"}}`) + ` {"trailing":1}`); res["error"] == nil {
		t.Fatalf("trailing data after the request object is accepted: %v", res)
	}
	if _, res := post(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{},"bogus":1}`); res["error"] == nil {
		t.Fatalf("unknown envelope field is accepted: %v", res)
	}
	_, res := post(call(`{"name":"pending_questions","arguments":{"shopper_id":"s","bogus":1}}`))
	result, ok := res["result"].(map[string]any)
	if !ok || result["isError"] != true {
		t.Fatalf("unknown tool argument is accepted: %v", res)
	}
}

// Verify answers the canned verdict.
func (v acceptVerifier) Verify(_ context.Context, _ ingest.Delivery) error {
	return v.err
}

// TestWebhookRouteVerifiesDelivery proves the deployed route reads the
// raw body bounded, resolves the shopper from the custom id, and passes
// the exact bytes to the verifier: a good delivery stores, a rejected
// one and a delivery naming no shopper store nothing.
func TestWebhookRouteVerifiesDelivery(t *testing.T) {
	newRoute := func(verifier ingest.Verifier) *Service {
		store, err := evidence.NewStore(testContract{kind: "webhook"})
		if err != nil {
			t.Fatalf("open evidence store: %v", err)
		}
		classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
		if err != nil {
			t.Fatalf("open classifier fake: %v", err)
		}
		svc, err := NewService(Deps{
			Payments:   fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
			Classifier: classifier,
			Store:      store,
			Graph:      fakes.NewRecordingEvidenceGraph(storyAllowlist(), nil),
			SchemaDir:  apiDir(t),
			Clock:      func() time.Time { return time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC) },
			Verifier:   verifier,
		})
		if err != nil {
			t.Fatalf("new service: %v", err)
		}
		return svc
	}
	post := func(svc *Service, body string) int {
		server := httptest.NewServer(svc.Handler())
		defer server.Close()
		res, err := server.Client().Post(server.URL+"/webhooks/payment", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("post webhook: %v", err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	good := `{"id":"WH-2EV-01","resource":{"custom_id":"webhook_shopper"}}`
	if status := post(newRoute(acceptVerifier{}), good); status != 200 {
		t.Fatalf("good delivery status %d, want 200", status)
	}
	// A replayed delivery writes nothing new at the deployed route too:
	// the same service serves the event twice and still holds the one
	// question the first delivery filed.
	replayed := newRoute(acceptVerifier{})
	for i := 0; i < 2; i++ {
		if status := post(replayed, good); status != 200 {
			t.Fatalf("delivery %d status %d, want 200", i+1, status)
		}
	}
	held, err := replayed.PendingQuestions(context.Background(), contract.PendingQuestionsRequest{ShopperID: "webhook_shopper"})
	if err != nil {
		t.Fatalf("pending questions after replay: %v", err)
	}
	if len(held.Questions) != 1 {
		t.Fatalf("replayed delivery holds %d questions, want the 1 the first delivery filed", len(held.Questions))
	}
	refused := newRoute(acceptVerifier{err: errors.New("nope")})
	if status := post(refused, good); status != 400 {
		t.Fatalf("rejected delivery status %d, want 400", status)
	}
	pending, err := refused.PendingQuestions(context.Background(), contract.PendingQuestionsRequest{ShopperID: "webhook_shopper"})
	if err != nil {
		t.Fatalf("pending questions: %v", err)
	}
	if len(pending.Questions) != 0 {
		t.Fatalf("rejected delivery stored %d questions, want none", len(pending.Questions))
	}
	if status := post(newRoute(acceptVerifier{}), `{"id":"WH-2EV-01","resource":{}}`); status != 400 {
		t.Fatalf("delivery naming no shopper status %d, want 400", status)
	}
}

// TestRecallConsultedEmptyPoolServes proves the consulted-empty decision:
// a wired pool consulted on an empty line answers Claimed, which recall
// serves as an absent pool with a consulted lookup — the frozen
// example-none shape — instead of failing. The pool decides nothing; with
// no other memory the guidance says so.
func TestRecallConsultedEmptyPoolServes(t *testing.T) {
	ctx := context.Background()
	poolIdx := &fakePoolIndex{result: pool.Result{}}
	svc, _ := pivotHarness(t, poolIdx, &fakeFitIndex{}, "")
	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: "empty_line_shopper", BrandID: "brand-n-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall on an empty line: %v", err)
	}
	if recall.Pool != nil {
		t.Fatalf("empty pool serves %+v, want no pool object", recall.Pool)
	}
	if recall.PoolLookup != contract.PoolConsulted {
		t.Fatalf("pool lookup %q, want consulted with an empty pool", recall.PoolLookup)
	}
	if recall.Guidance.Basis != contract.GuidanceNone {
		t.Fatalf("guidance %+v, want none deciding", recall.Guidance)
	}
	poolIdx.mu.Lock()
	queries := len(poolIdx.queries)
	poolIdx.mu.Unlock()
	if queries != 1 {
		t.Fatalf("pool consulted %d times, want once", queries)
	}
}

// earnSmallBelief ingests three too-small refunds and answers them, so the
// shopper holds one Earned runs-small belief, and returns its id.
func earnSmallBelief(t *testing.T, svc *Service, shopper string) string {
	t.Helper()
	ctx := context.Background()
	for _, event := range []string{"WH-2EV-01", "WH-2EV-02", "WH-2EV-03"} {
		if _, err := svc.IngestWebhook(ctx, shopper, event); err != nil {
			t.Fatalf("ingest %s: %v", event, err)
		}
		if res := answerReason(t, svc, shopper, event, "too tight in the shoulders"); !res.Accepted {
			t.Fatalf("too-small answer for %s was not accepted", event)
		}
	}
	recall, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall after three returns: %v", err)
	}
	if len(recall.Beliefs) != 1 {
		t.Fatalf("recall serves %d beliefs, want the 1 earned sizing belief", len(recall.Beliefs))
	}
	return recall.Beliefs[0].BeliefID
}

// assertCorrectionDecides proves a recall is decided by the correction
// declared against the served belief, and reports the belief id it cites.
func assertCorrectionDecides(t *testing.T, svc *Service, shopper string) string {
	t.Helper()
	recall, err := svc.Recall(context.Background(), contract.RecallRequest{
		ShopperID: shopper, BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if recall.OverridingCorrection == nil || recall.Guidance.Basis != contract.GuidanceCorrection {
		t.Fatalf("correction does not decide: %+v", recall.Guidance)
	}
	if len(recall.Beliefs) != 1 || !recall.Beliefs[0].Contradicted {
		t.Fatalf("recall serves %d beliefs contradicted %v, want the 1 contradicted belief",
			len(recall.Beliefs), len(recall.Beliefs) == 1 && recall.Beliefs[0].Contradicted)
	}
	cited := recall.OverridingCorrection.ContradictsBeliefID
	if cited == "" || cited != recall.Beliefs[0].BeliefID {
		t.Fatalf("correction cites %q, want the served belief %q", cited, recall.Beliefs[0].BeliefID)
	}
	return cited
}

// restartDiverged rebuilds the service over the same standing directory
// with a divergent mint order: the dispute first takes blf_000001 for a
// seller belief, so the rebuilt fit belief cannot reuse its old id. A
// re-link by id coincidence would leave the stale view dangling; only a
// re-link by content match keeps the correction in force.
func restartDiverged(t *testing.T, dir, shopper string) *Service {
	t.Helper()
	ctx := context.Background()
	svc, _ := pivotHarness(t, nil, nil, dir)
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-04"); err != nil {
		t.Fatalf("ingest dispute: %v", err)
	}
	for _, event := range []string{"WH-2EV-01", "WH-2EV-02", "WH-2EV-03"} {
		if _, err := svc.IngestWebhook(ctx, shopper, event); err != nil {
			t.Fatalf("re-ingest %s: %v", event, err)
		}
		if res := answerReason(t, svc, shopper, event, "too tight in the shoulders"); !res.Accepted {
			t.Fatalf("too-small answer for %s was not accepted", event)
		}
	}
	return svc
}

// TestDeclaredCorrectionRelinksAfterRestart proves a restarted layer
// re-links a directly declared correction to the rewritten belief by
// content match: the restored view cites a stale belief id that matches
// nothing, the rewritten belief has a new id, and the correction still
// decides with the new id cited.
func TestDeclaredCorrectionRelinksAfterRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	svc, _ := pivotHarness(t, nil, nil, dir)
	shopper := "relink_shopper"
	beliefID := earnSmallBelief(t, svc, shopper)
	if _, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: "I've lost weight, use M", ContradictsBeliefID: beliefID,
	}); err != nil {
		t.Fatalf("record declared correction: %v", err)
	}
	if got := assertCorrectionDecides(t, svc, shopper); got != beliefID {
		t.Fatalf("correction cites %q before the restart, want %q", got, beliefID)
	}
	restarted := restartDiverged(t, dir, shopper)
	got := assertCorrectionDecides(t, restarted, shopper)
	if got == beliefID {
		t.Fatalf("correction still cites %q: re-linked by id coincidence, want the rewritten belief", beliefID)
	}
}

// TestStandingDeclaredRelinksAfterRestart proves the same re-link for a
// correction that stood first and was declared when its belief arrived:
// the journal entry survives the restart, re-declares against the
// rewritten belief, and the restored view follows it to the new id.
func TestStandingDeclaredRelinksAfterRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	svc, _ := pivotHarness(t, nil, nil, dir)
	shopper := "standing_relink_shopper"
	if _, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"}); err != nil {
		t.Fatalf("first recall: %v", err)
	}
	if _, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: "I've lost weight, use M",
	}); err != nil {
		t.Fatalf("record standing correction: %v", err)
	}
	beliefID := earnSmallBelief(t, svc, shopper)
	if got := assertCorrectionDecides(t, svc, shopper); got != beliefID {
		t.Fatalf("standing correction cites %q after its belief, want %q", got, beliefID)
	}
	restarted := restartDiverged(t, dir, shopper)
	got := assertCorrectionDecides(t, restarted, shopper)
	if got == beliefID {
		t.Fatalf("correction still cites %q: re-linked by id coincidence, want the rewritten belief", beliefID)
	}
}

// shopperBeliefKinds maps each belief id in one shopper's partition to
// its kind. The caller must not hold the service lock.
func shopperBeliefKinds(t *testing.T, svc *Service, shopper string) map[string]domain.BeliefKind {
	t.Helper()
	svc.mu.Lock()
	defer svc.mu.Unlock()
	st, ok := svc.shoppers[shopper]
	if !ok {
		t.Fatalf("no partition for %q", shopper)
	}
	out := map[string]domain.BeliefKind{}
	for id, rec := range st.beliefs {
		out[id] = rec.kind
	}
	return out
}

// shopperCitations maps each correction id in one shopper's partition to
// the belief id it contradicts. The caller must not hold the service
// lock.
func shopperCitations(t *testing.T, svc *Service, shopper string) map[string]string {
	t.Helper()
	svc.mu.Lock()
	defer svc.mu.Unlock()
	st, ok := svc.shoppers[shopper]
	if !ok {
		t.Fatalf("no partition for %q", shopper)
	}
	out := map[string]string{}
	for id, rec := range st.corrections {
		out[id] = rec.view.ContradictsBeliefID
	}
	return out
}

// TestSameWordsTwoSubjectsSurviveDivergentRebuild is the throwaway
// reproduction the T8.5c review proved by hand: the same words declared
// against two subjects — a fit belief and a seller belief — must stay
// attached to their own beliefs across a divergent rebuild, with no
// recall hard-failing ErrOverrideMissing. The rebuild reuses ids across
// subjects (the dispute first takes the fit belief's old id), so a
// re-link by words alone collapses both views onto the first-rebuilt
// belief; the persisted view-to-entry pairing re-links each view by its
// own entry instead.
func TestSameWordsTwoSubjectsSurviveDivergentRebuild(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	svc, _ := pivotHarness(t, nil, nil, dir)
	shopper := "two_subject_shopper"
	fitID := earnSmallBelief(t, svc, shopper)
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-04"); err != nil {
		t.Fatalf("ingest dispute: %v", err)
	}
	sellerID := ""
	for id, kind := range shopperBeliefKinds(t, svc, shopper) {
		if kind == domain.BeliefKindSeller {
			sellerID = id
		}
	}
	if sellerID == "" {
		t.Fatal("no seller belief after the dispute")
	}
	const words = "I've lost weight, use M"
	if _, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: words, ContradictsBeliefID: fitID,
	}); err != nil {
		t.Fatalf("record fit correction: %v", err)
	}
	if _, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: words, ContradictsBeliefID: sellerID,
	}); err != nil {
		t.Fatalf("record seller correction: %v", err)
	}
	before := shopperCitations(t, svc, shopper)
	if len(before) != 2 {
		t.Fatalf("two same-words corrections hold %d views, want 2", len(before))
	}
	seen := map[string]bool{}
	for _, cited := range before {
		if cited != fitID && cited != sellerID {
			t.Fatalf("a view cites %q, want the fit or the seller belief", cited)
		}
		seen[cited] = true
	}
	if len(seen) != 2 {
		t.Fatalf("both views cite %v, want one per belief", before)
	}
	restarted := restartDiverged(t, dir, shopper)
	after := shopperCitations(t, restarted, shopper)
	if len(after) != 2 {
		t.Fatalf("rebuilt corrections hold %d views, want 2", len(after))
	}
	restarted.mu.Lock()
	rst := restarted.shoppers[shopper]
	for view, cited := range after {
		link, ok := rst.pairing[view]
		if !ok {
			restarted.mu.Unlock()
			t.Fatalf("view %q lost its entry pairing across the restart", view)
		}
		belief, ok := rst.beliefs[cited]
		if !ok {
			restarted.mu.Unlock()
			t.Fatalf("view %q cites %q, which names no rebuilt belief", view, cited)
		}
		if belief.subject.ID != link.Subject.ID || belief.subject.Kind != link.Subject.Kind {
			restarted.mu.Unlock()
			t.Fatalf("view %q paired to %q %q cites %q %q: cross-linked across subjects",
				view, link.Subject.Kind, link.Subject.ID, belief.subject.Kind, belief.subject.ID)
		}
	}
	byKind := map[domain.BeliefKind]string{}
	for view, cited := range after {
		byKind[rst.beliefs[cited].kind] = view
	}
	restarted.mu.Unlock()
	if len(byKind) != 2 {
		t.Fatalf("both rebuilt views cite the same kind: %v", after)
	}
	cited := assertCorrectionDecides(t, restarted, shopper)
	kinds := shopperBeliefKinds(t, restarted, shopper)
	if kinds[cited] != domain.BeliefKindFit {
		t.Fatalf("the deciding correction cites %q of kind %q, want the rewritten fit belief", cited, kinds[cited])
	}
	sellerCited := ""
	for belief, kind := range kinds {
		if kind == domain.BeliefKindSeller {
			sellerCited = belief
		}
	}
	explained, err := restarted.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: sellerCited})
	if err != nil {
		t.Fatalf("explain the rewritten seller belief: %v", err)
	}
	if !explained.Belief.Contradicted || explained.Belief.ContradictedBy == nil ||
		explained.Belief.ContradictedBy.ContradictsBeliefID != sellerCited {
		t.Fatalf("the seller belief carries no warning from its own view: %+v", explained.Belief.ContradictedBy)
	}
}

// TestDeclaredViewWithoutRewrittenBeliefStillDecides proves a restored
// declared view is never silent: in the restart-before-redelivery window
// the cited belief id matches nothing, yet recall still honours the
// correction by its paired subject instead of dropping it until the
// belief is rewritten. The guidance recommends from the correction alone,
// pointing at no stale belief id.
func TestDeclaredViewWithoutRewrittenBeliefStillDecides(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	svc, _ := pivotHarness(t, nil, nil, dir)
	shopper := "dangling_shopper"
	beliefID := earnSmallBelief(t, svc, shopper)
	if _, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: "I've lost weight, use M", ContradictsBeliefID: beliefID,
	}); err != nil {
		t.Fatalf("record declared correction: %v", err)
	}
	if got := assertCorrectionDecides(t, svc, shopper); got != beliefID {
		t.Fatalf("correction cites %q before the restart, want %q", got, beliefID)
	}
	restarted, _ := pivotHarness(t, nil, nil, dir)
	recall, err := restarted.Recall(ctx, contract.RecallRequest{
		ShopperID: shopper, BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall with no rewritten belief: %v", err)
	}
	if len(recall.Beliefs) != 0 {
		t.Fatalf("recall serves %d beliefs with nothing rewritten, want none", len(recall.Beliefs))
	}
	if recall.OverridingCorrection == nil || recall.Guidance.Basis != contract.GuidanceCorrection {
		t.Fatalf("the dangling correction does not decide: %+v", recall.Guidance)
	}
	if recall.OverridingCorrection.ContradictsBeliefID != beliefID {
		t.Fatalf("the dangling view cites %q, want the stale %q preserved", recall.OverridingCorrection.ContradictsBeliefID, beliefID)
	}
	if recall.Guidance.BeliefID != nil {
		t.Fatalf("the dangling guidance points at belief %q, want the correction alone", *recall.Guidance.BeliefID)
	}
	if want := "Take size M for Brand X"; recall.Guidance.Text != want {
		t.Fatalf("dangling guidance text %q, want %q", recall.Guidance.Text, want)
	}
	if recall.Pool != nil || recall.PoolLookup != contract.PoolUnavailable {
		t.Fatalf("dangling recall carries pool %+v/%q, want null/unavailable", recall.Pool, recall.PoolLookup)
	}
}

// TestSameSubjectTwoBeliefsCiteStrongest proves the deterministic half of
// the same-subject collapse: a standing recorded while two fit beliefs
// share one subject declares against exactly one of them — the strongest,
// most evidence first — instead of citing by map order while the entry
// declares elsewhere. Any other split hard-fails the next recall with
// ErrOverrideMissing; this one keeps both readings representable.
func TestSameSubjectTwoBeliefsCiteStrongest(t *testing.T) {
	ctx := context.Background()
	svc := seededService(t)
	shopper := "shopper_01"
	if _, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"}); err != nil {
		t.Fatalf("first recall: %v", err)
	}
	standing, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: "I've lost weight, use M",
	})
	if err != nil {
		t.Fatalf("record standing correction: %v", err)
	}
	if standing.BeliefID != "blf_8f2c41" {
		t.Fatalf("standing declares against %q, want the strongest fit belief blf_8f2c41", standing.BeliefID)
	}
	svc.mu.Lock()
	st := svc.shoppers[shopper]
	declared := 0
	for _, entry := range st.journal.Standing() {
		if entry.Against != "" {
			declared++
		}
	}
	svc.mu.Unlock()
	if declared != 1 {
		t.Fatalf("the standing declared %d entries, want exactly the one match", declared)
	}
	recall, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall after the standing: %v", err)
	}
	if recall.OverridingCorrection == nil || recall.Guidance.Basis != contract.GuidanceCorrection {
		t.Fatalf("the standing correction does not decide: %+v", recall.Guidance)
	}
}
