package server

import (
	"context"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// secondCorrectionWords is a correction the story world does not know. The
// harnesses widen their allowlist by it so a correction recorded after a
// restart has something the recording graph has seen.
const secondCorrectionWords = "the collar is loose, take size M"

// correctionHarness builds a service over the pivot's fakes with the
// standing store on disk and the allowlist widened by the second
// correction's words. Two services over one directory are the same store
// before and after a restart.
func correctionHarness(t *testing.T, dir string) *Service {
	t.Helper()
	store, err := evidence.NewStore(testContract{kind: "correction-id"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	allowed := append(pivotAllowlist(), secondCorrectionWords)
	fixed := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:    fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier:  classifier,
		Store:       store,
		Graph:       fakes.NewRecordingEvidenceGraph(allowed, fakes.StoryRecalls()),
		SchemaDir:   apiDir(t),
		Clock:       func() time.Time { return fixed },
		StandingDir: dir,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

// TestRestoredCorrectionIDIsNeverMintedAgain proves a correction id names
// one correction for the life of the store: after a restart the journal's
// id still resolves, and the next correction gets a fresh id instead of
// minting over it.
func TestRestoredCorrectionIDIsNeverMintedAgain(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	shopper := "corr_id_shopper"

	svc := correctionHarness(t, dir)
	beliefID := earnSmallBelief(t, svc, shopper)
	first, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: "I've lost weight, use M", ContradictsBeliefID: beliefID,
	})
	if err != nil {
		t.Fatalf("record the first correction: %v", err)
	}
	restoredID := first.Correction.CorrectionID
	if restoredID == "" {
		t.Fatal("the first correction has no id")
	}

	// The restart finds the belief gone but the correction still on file.
	restarted := correctionHarness(t, dir)
	recall, err := restarted.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall after the restart: %v", err)
	}
	if recall.OverridingCorrection == nil || recall.OverridingCorrection.CorrectionID != restoredID {
		t.Fatalf("the restored correction %q does not decide after the restart: %+v", restoredID, recall.OverridingCorrection)
	}

	// The write restores the brand context, so the correction is journalled
	// against Brand X rather than unscoped.
	second, err := restarted.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: secondCorrectionWords,
	})
	if err != nil {
		t.Fatalf("record the second correction: %v", err)
	}
	secondID := second.Correction.CorrectionID
	if secondID == restoredID {
		t.Fatalf("the second correction reused the restored id %q", secondID)
	}
	if want := "corr_000002"; secondID != want {
		t.Fatalf("the second correction is %q, want the fresh %q", secondID, want)
	}

	restarted.mu.Lock()
	st := restarted.shoppers[shopper]
	firstView, ok := st.corrections[restoredID]
	if !ok {
		restarted.mu.Unlock()
		t.Fatalf("the restored correction %q left the store", restoredID)
	}
	secondView, ok := st.corrections[secondID]
	if !ok {
		restarted.mu.Unlock()
		t.Fatalf("the second correction %q is not in the store", secondID)
	}
	order := append([]string(nil), st.corrOrder...)
	restarted.mu.Unlock()

	if firstView.view.Text != "I've lost weight, use M" {
		t.Fatalf("the restored correction now reads %q", firstView.view.Text)
	}
	if firstView.view.ContradictsBeliefID != beliefID {
		t.Fatalf("the restored correction cites %q, want its original belief %q", firstView.view.ContradictsBeliefID, beliefID)
	}
	if secondView.view.Text != secondCorrectionWords {
		t.Fatalf("the second correction reads %q, want %q", secondView.view.Text, secondCorrectionWords)
	}
	if len(order) != 2 {
		t.Fatalf("the store holds %v, want the two corrections once each", order)
	}
}

// seededCorrectionHarness builds a service replayed to the frozen seed's
// end state with the standing store on disk, so a restart restores the
// corrections the seed wrote.
func seededCorrectionHarness(t *testing.T, dir string, seed Seed) *Service {
	t.Helper()
	store, err := evidence.NewStore(testContract{kind: "correction-seed"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	allowed := append(SeedGraphContents(seed), secondCorrectionWords)
	fixed := time.Date(2026, 11, 8, 18, 40, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:    fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier:  classifier,
		Store:       store,
		Graph:       fakes.NewRecordingEvidenceGraph(allowed, fakes.StoryRecalls()),
		SchemaDir:   apiDir(t),
		Clock:       func() time.Time { return fixed },
		StandingDir: dir,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

// seededFitBelief is the frozen seed's contradicted sizing belief: the row
// the seeded correction was declared against.
const seededFitBelief = "blf_8f2c41"

// assertCorrectionIDsOnce fails when a listing names one correction id
// twice, and reports the ids it listed.
func assertCorrectionIDsOnce(t *testing.T, listed []contract.CorrectionView) map[string]bool {
	t.Helper()
	seen := map[string]bool{}
	for _, view := range listed {
		if view.CorrectionID == "" {
			t.Fatalf("a listed correction carries no id: %+v", view)
		}
		if seen[view.CorrectionID] {
			t.Fatalf("correction %q is listed twice", view.CorrectionID)
		}
		seen[view.CorrectionID] = true
	}
	return seen
}

// TestRestartAndReseedListEachCorrectionOnce proves the explain read shows
// one row per correction after a restart replays the seed beside the
// restored journal: the seeded view and the live one are each named once,
// and the count matches the store's distinct corrections.
func TestRestartAndReseedListEachCorrectionOnce(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	seed, err := SeedFromExamples(apiDir(t))
	if err != nil {
		t.Fatalf("build seed: %v", err)
	}
	shopper := seed.ShopperID

	svc := seededCorrectionHarness(t, dir, seed)
	if err := svc.LoadSeed(ctx, seed); err != nil {
		t.Fatalf("load seed: %v", err)
	}
	if _, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"}); err != nil {
		t.Fatalf("recall before the live correction: %v", err)
	}
	// The write persists the journal, so the restart has a snapshot to
	// restore beside the seed it replays.
	live, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{ShopperID: shopper, Text: secondCorrectionWords})
	if err != nil {
		t.Fatalf("record the live correction: %v", err)
	}

	restarted := seededCorrectionHarness(t, dir, seed)
	if err := restarted.LoadSeed(ctx, seed); err != nil {
		t.Fatalf("replay the seed after the restart: %v", err)
	}
	explained, err := restarted.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: seededFitBelief})
	if err != nil {
		t.Fatalf("explain the seeded belief: %v", err)
	}
	listed := assertCorrectionIDsOnce(t, explained.Corrections)
	if !listed["corr_3b7e52"] {
		t.Fatalf("the seeded correction is missing from the listing: %v", explained.Corrections)
	}
	if live.Correction.CorrectionID == "" || !listed[live.Correction.CorrectionID] {
		t.Fatalf("the live correction %q is missing from the listing: %v", live.Correction.CorrectionID, explained.Corrections)
	}

	restarted.mu.Lock()
	st := restarted.shoppers[shopper]
	distinct := 0
	for id, rec := range st.corrections {
		if rec.view.ContradictsBeliefID == seededFitBelief {
			distinct++
			if !listed[id] {
				restarted.mu.Unlock()
				t.Fatalf("correction %q cites the belief but is not listed", id)
			}
		}
	}
	restarted.mu.Unlock()
	if distinct != 2 {
		t.Fatalf("the store holds %d corrections against the belief, want the seeded one and the live one", distinct)
	}
	if len(listed) != distinct {
		t.Fatalf("the listing holds %d corrections, the store %d", len(listed), distinct)
	}

	// The ledger read lists through the same order, so it holds the same
	// one row per correction.
	ledger, err := restarted.LedgerBelief(ctx, contract.LedgerBeliefRequest{ShopperID: shopper, BeliefID: seededFitBelief})
	if err != nil {
		t.Fatalf("ledger read of the seeded belief: %v", err)
	}
	if want := len(listed); len(ledger.Corrections) != want {
		t.Fatalf("the ledger lists %d corrections, explain lists %d", len(ledger.Corrections), want)
	}
	assertCorrectionIDsOnce(t, ledger.Corrections)
}

// TestExplainBeliefListsARepeatedOrderOnce proves the read itself never
// doubles a correction: a journal written before the order deduplicated
// names one id twice, and the listing still shows one row.
func TestExplainBeliefListsARepeatedOrderOnce(t *testing.T) {
	ctx := context.Background()
	svc := seededService(t)
	shopper := "shopper_01"
	before, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: seededFitBelief})
	if err != nil {
		t.Fatalf("explain the seeded belief: %v", err)
	}
	listed := assertCorrectionIDsOnce(t, before.Corrections)
	if !listed["corr_3b7e52"] {
		t.Fatalf("the seeded correction is missing from the listing: %v", before.Corrections)
	}

	svc.mu.Lock()
	st := svc.shoppers[shopper]
	st.corrOrder = append(st.corrOrder, "corr_3b7e52")
	svc.mu.Unlock()

	after, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: seededFitBelief})
	if err != nil {
		t.Fatalf("explain the seeded belief with a repeated order: %v", err)
	}
	if len(after.Corrections) != len(before.Corrections) {
		t.Fatalf("a repeated order doubled the listing: %d rows, want %d", len(after.Corrections), len(before.Corrections))
	}
	assertCorrectionIDsOnce(t, after.Corrections)
}
