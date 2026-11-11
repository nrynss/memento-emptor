package server

import (
	"context"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/standing"
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

// TestExplainBeliefListsARepeatedOrderOnce proves the read does not rely
// on the order being deduplicated: an order that names one id twice still
// lists one row.
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

// t97Corrections is the correction journal the T9.7 live run left behind:
// five ids in seven rows, because the seed replaying beside the restored
// journal appended corr_3b7e52 and corr_000001 a second time. The rows are
// the ones demo/evidence/run-6/02-belief-explained.json served, oldest
// first.
func t97Corrections() []contract.CorrectionView {
	views := []contract.CorrectionView{
		{
			CorrectionID: "corr_3b7e52", Text: "I've lost weight, use M",
			ContradictsBeliefID: seededFitBelief, RecordedAt: "2026-11-05T14:02:11Z", TakesPrecedence: true,
		},
		{
			CorrectionID: "corr_000001", Text: "I've lost weight - for Brand X oxfords, use M for me",
			ContradictsBeliefID: seededFitBelief, RecordedAt: "2026-11-11T11:29:21Z", TakesPrecedence: true,
		},
		{
			CorrectionID: "corr_000002", Text: "I've dropped a size since October. For Brand X oxfords, take M for me, and record that as my correction.",
			ContradictsBeliefID: seededFitBelief, RecordedAt: "2026-11-11T11:32:11Z", TakesPrecedence: true,
		},
		{
			CorrectionID: "corr_000003", Text: "I've dropped a size since October. For Brand X oxfords, I take M now - please record that as my correction.",
			ContradictsBeliefID: seededFitBelief, RecordedAt: "2026-11-11T11:34:39Z", TakesPrecedence: true,
		},
		{
			CorrectionID: "corr_000004", Text: "Use M for me on Brand X oxfords from now on - I've gone down a size since October. Record that correction.",
			ContradictsBeliefID: seededFitBelief, RecordedAt: "2026-11-11T11:40:20Z", TakesPrecedence: true,
		},
	}
	// The two the run appended a second time, at the end of its order.
	return append(views, views[1], views[0])
}

// TestRestoredJournalOfSevenRowsServesFive proves the fix against the
// journal the T9.7 run left: a snapshot naming five correction ids in
// seven rows restores to one row per id, holds the order at five, keeps
// the view and place each id already had, mints the next correction past
// every restored id, and writes the rows back once each.
func TestRestoredJournalOfSevenRowsServesFive(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	seed, err := SeedFromExamples(apiDir(t))
	if err != nil {
		t.Fatalf("build seed: %v", err)
	}
	store, err := standing.NewDirStore(dir)
	if err != nil {
		t.Fatalf("open standing store: %v", err)
	}
	snap := standing.Snapshot{Pooling: true, Corrections: t97Corrections()}
	if err := store.Save(ctx, seed.ShopperID, snap); err != nil {
		t.Fatalf("save the run's journal: %v", err)
	}

	svc := seededCorrectionHarness(t, dir, seed)
	if err := svc.LoadSeed(ctx, seed); err != nil {
		t.Fatalf("replay the seed beside the restored journal: %v", err)
	}
	explained, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: seed.ShopperID, BeliefID: seededFitBelief})
	if err != nil {
		t.Fatalf("explain the seeded belief: %v", err)
	}
	listed := assertCorrectionIDsOnce(t, explained.Corrections)
	if len(listed) != 5 {
		t.Fatalf("the run's seven rows served %d corrections, want the five its ids name", len(listed))
	}
	// The order is unchanged: newest first, the duplicated rows in the
	// one place their id already held.
	for i, want := range []string{"corr_000004", "corr_000003", "corr_000002", "corr_000001", "corr_3b7e52"} {
		if got := explained.Corrections[i].CorrectionID; got != want {
			t.Fatalf("row %d is %q, want the run's %q", i, got, want)
		}
	}
	ledger, err := svc.LedgerBelief(ctx, contract.LedgerBeliefRequest{ShopperID: seed.ShopperID, BeliefID: seededFitBelief})
	if err != nil {
		t.Fatalf("ledger read of the seeded belief: %v", err)
	}
	if len(ledger.Corrections) != 5 {
		t.Fatalf("the ledger lists %d corrections, want the same five", len(ledger.Corrections))
	}
	assertCorrectionIDsOnce(t, ledger.Corrections)

	// The duplicate rows did not double the timeline: the correction the
	// seed's history recorded stays one event, as it stays one row.
	correctionEvents := 0
	for _, event := range explained.Timeline {
		if event.Kind == contract.TimelineCorrection {
			correctionEvents++
		}
	}
	if correctionEvents != 1 {
		t.Fatalf("the timeline holds %d correction events, want the seed's one", correctionEvents)
	}

	svc.mu.Lock()
	order := len(svc.shoppers[seed.ShopperID].corrOrder)
	svc.mu.Unlock()
	if order != 5 {
		t.Fatalf("the restored order holds %d rows, want the five ids once each", order)
	}

	// A second sighting of an id keeps the view on file: a seed that
	// changed one must not rewrite the journal's copy.
	svc.mu.Lock()
	svc.shoppers[seed.ShopperID].takeCorrection("corr_000004", &correctionRecord{
		view: contract.CorrectionView{CorrectionID: "corr_000004", Text: "rewritten by a later seed"},
	})
	svc.mu.Unlock()
	again, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: seed.ShopperID, BeliefID: seededFitBelief})
	if err != nil {
		t.Fatalf("explain the seeded belief after a second sighting: %v", err)
	}
	if len(again.Corrections) != 5 || again.Corrections[0].CorrectionID != "corr_000004" {
		t.Fatalf("a second sighting moved or doubled the row: %v", again.Corrections)
	}
	if want := "Use M for me on Brand X oxfords from now on - I've gone down a size since October. Record that correction."; again.Corrections[0].Text != want {
		t.Fatalf("the second sighting rewrote the row to %q, want %q", again.Corrections[0].Text, want)
	}

	// The restored ids keep their numbers, so the next correction is 000005
	// rather than a second 000001.
	recorded, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: seed.ShopperID, Text: secondCorrectionWords, ContradictsBeliefID: seededFitBelief,
	})
	if err != nil {
		t.Fatalf("record a correction after the restore: %v", err)
	}
	if want := "corr_000005"; recorded.Correction.CorrectionID != want {
		t.Fatalf("the next correction is %q, want the fresh %q", recorded.Correction.CorrectionID, want)
	}

	// The five rows go back to disk once each, beside the new correction.
	written, found, err := store.Load(ctx, seed.ShopperID)
	if err != nil || !found {
		t.Fatalf("read the journal back (found %t): %v", found, err)
	}
	if len(written.Corrections) != 6 {
		t.Fatalf("the journal was written with %d rows, want the six distinct corrections", len(written.Corrections))
	}
	assertCorrectionIDsOnce(t, written.Corrections)
}
