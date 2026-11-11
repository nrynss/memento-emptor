package server

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// restartShopper is the demo shopper the frozen seed replays for.
const restartShopper = "shopper_01"

// liveEventID is the live refund the restart tests store: the seed leaves
// it an open question, so it holds no ledger row of its own.
const liveEventID = "WH-2EV-05"

// evidenceJournalPath names one shopper's evidence journal under a
// standing directory, beside the corrections journal.
func evidenceJournalPath(dir, shopper string) string {
	return filepath.Join(dir, shopper+".evidence.json")
}

// bootRestartedService builds one layer boot over dir: a fresh store, the
// frozen demo seed replayed, and the same recording seams on every boot.
// An empty dir keeps the evidence in memory, as the demo's offline boot
// does without a standing directory.
func bootRestartedService(t *testing.T, dir string, seed Seed) (*Service, *evidence.Store) {
	t.Helper()
	store, err := evidence.NewStore(testContract{kind: "server-restart"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	allowed := append(storyAllowlist(), SeedGraphContents(seed)...)
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
	if err := svc.LoadSeed(context.Background(), seed); err != nil {
		t.Fatalf("load seed: %v", err)
	}
	return svc, store
}

// fileLiveRefund delivers a live refund the seed holds no row for and
// returns the question the delivery filed, which the seed's own question
// for the event does not name.
func fileLiveRefund(t *testing.T, svc *Service) string {
	t.Helper()
	ctx := context.Background()
	before := map[string]bool{}
	pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: restartShopper})
	if err != nil {
		t.Fatalf("pending before live refund: %v", err)
	}
	for _, question := range pending.Questions {
		before[question.QuestionID] = true
	}
	if _, err := svc.IngestWebhook(ctx, restartShopper, liveEventID); err != nil {
		t.Fatalf("ingest live refund: %v", err)
	}
	pending, err = svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: restartShopper})
	if err != nil {
		t.Fatalf("pending after live refund: %v", err)
	}
	questionID := ""
	for _, question := range pending.Questions {
		if question.Link.EventID == liveEventID && !before[question.QuestionID] {
			questionID = question.QuestionID
		}
	}
	if questionID == "" {
		t.Fatal("the live refund filed no question of its own")
	}
	return questionID
}

// driveLiveEvidence delivers a live refund the seed holds no row for and
// records its classified reason, so the layer holds evidence no boot
// replay can rebuild.
func driveLiveEvidence(t *testing.T, svc *Service) {
	t.Helper()
	ctx := context.Background()
	questionID := fileLiveRefund(t, svc)
	res, err := svc.RecordReason(ctx, contract.RecordReasonRequest{
		ShopperID:  restartShopper,
		QuestionID: questionID,
		AnswerText: "too tight in the shoulders",
	})
	if err != nil {
		t.Fatalf("record the live reason: %v", err)
	}
	if !res.Accepted || res.BeliefID == nil {
		t.Fatalf("live reason: accepted %v belief %v, want it attached", res.Accepted, res.BeliefID)
	}
}

// restartReads gathers every read a restart must keep: the ledger's
// belief and evidence panels, recall's citations, explain_belief's
// evidence and history, and the open questions.
type restartReads struct {
	Beliefs  contract.LedgerBeliefsResponse
	Evidence contract.LedgerEvidenceResponse
	Recall   contract.RecallResponse
	Explain  contract.ExplainBeliefResponse
	Pending  contract.PendingQuestionsResponse
}

// readRestartState runs those reads against one boot.
func readRestartState(t *testing.T, svc *Service) restartReads {
	t.Helper()
	ctx := context.Background()
	beliefs, err := svc.LedgerBeliefs(ctx, contract.LedgerBeliefsRequest{ShopperID: restartShopper, IncludeSeeded: true})
	if err != nil {
		t.Fatalf("ledger beliefs: %v", err)
	}
	ledger, err := svc.LedgerEvidence(ctx, contract.LedgerEvidenceRequest{ShopperID: restartShopper})
	if err != nil {
		t.Fatalf("ledger evidence: %v", err)
	}
	recall, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: restartShopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: restartShopper})
	if err != nil {
		t.Fatalf("pending questions: %v", err)
	}
	sizing := ""
	for _, row := range beliefs.Beliefs {
		if row.Belief.Kind == domain.BeliefKindFit && row.Belief.Value == "runs small, take size L" {
			sizing = row.Belief.BeliefID
		}
	}
	if sizing == "" {
		t.Fatal("the ledger holds no sizing belief to explain")
	}
	explained, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: restartShopper, BeliefID: sizing})
	if err != nil {
		t.Fatalf("explain belief: %v", err)
	}
	return restartReads{Beliefs: beliefs, Evidence: ledger, Recall: recall, Explain: explained, Pending: pending}
}

// liveEvidenceID names the ledger row the live drive stored: the only
// row for the live event, and the only one carrying its answer.
func liveEvidenceID(t *testing.T, reads restartReads) string {
	t.Helper()
	var live []string
	for _, row := range reads.Evidence.Evidence {
		if row.Evidence.Link.EventID == liveEventID && row.Evidence.Reason != nil {
			live = append(live, row.Evidence.EvidenceID)
		}
	}
	if len(live) != 1 {
		t.Fatalf("ledger holds %d rows for the live event, want the one the drive recorded: %v", len(live), live)
	}
	return live[0]
}

// assertSameRestartReads proves a later boot answers every read exactly
// as the boot before it did.
func assertSameRestartReads(t *testing.T, want, got restartReads) {
	t.Helper()
	if !reflect.DeepEqual(want.Beliefs, got.Beliefs) {
		t.Errorf("ledger beliefs differ after a restart:\nbefore: %s\nafter:  %s", pretty(want.Beliefs), pretty(got.Beliefs))
	}
	if !reflect.DeepEqual(want.Evidence, got.Evidence) {
		t.Errorf("ledger evidence differs after a restart:\nbefore: %s\nafter:  %s", pretty(want.Evidence), pretty(got.Evidence))
	}
	if !reflect.DeepEqual(want.Recall, got.Recall) {
		t.Errorf("recall differs after a restart:\nbefore: %s\nafter:  %s", pretty(want.Recall), pretty(got.Recall))
	}
	if !reflect.DeepEqual(want.Explain, got.Explain) {
		t.Errorf("explain_belief differs after a restart:\nbefore: %s\nafter:  %s", pretty(want.Explain), pretty(got.Explain))
	}
	if !reflect.DeepEqual(want.Pending, got.Pending) {
		t.Errorf("pending questions differ after a restart:\nbefore: %s\nafter:  %s", pretty(want.Pending), pretty(got.Pending))
	}
}

// assertLiveCitation proves one boot still cites the live row behind the
// belief, last, where the live write appended it.
func assertLiveCitation(t *testing.T, reads restartReads, liveID string) {
	t.Helper()
	ids := make([]string, 0, len(reads.Explain.Belief.Evidence))
	for _, row := range reads.Explain.Belief.Evidence {
		ids = append(ids, row.EvidenceID)
	}
	if len(ids) == 0 || ids[len(ids)-1] != liveID {
		t.Errorf("belief cites %v, want the live row %s last", ids, liveID)
	}
	found := false
	for _, row := range reads.Recall.Beliefs {
		for _, ev := range row.Evidence {
			if ev.EvidenceID == liveID {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("recall cites none of the live row %s", liveID)
	}
}

// TestEvidenceJournalSurvivesRestart proves a layer restart over the
// same standing directory keeps the live evidence: the journal file is
// written beside the corrections journal, the ledger, recall citations
// and explain_belief answer exactly as before, and a second restart
// changes neither the answers nor the journal file.
func TestEvidenceJournalSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	seed, err := SeedFromExamples(apiDir(t))
	if err != nil {
		t.Fatalf("build seed: %v", err)
	}

	first, firstStore := bootRestartedService(t, dir, seed)
	driveLiveEvidence(t, first)
	before := readRestartState(t, first)
	liveID := liveEvidenceID(t, before)
	assertLiveCitation(t, before, liveID)
	if got := len(firstStore.Documents()); got != 6 {
		t.Fatalf("live boot holds %d documents, want the 5 seeded plus the live one", got)
	}

	journal := evidenceJournalPath(dir, restartShopper)
	saved, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("the evidence journal is not under the standing dir: %v", err)
	}
	if len(bytes.TrimSpace(saved)) == 0 {
		t.Fatal("the evidence journal is empty")
	}

	second, secondStore := bootRestartedService(t, dir, seed)
	afterFirst := readRestartState(t, second)
	assertSameRestartReads(t, before, afterFirst)
	assertLiveCitation(t, afterFirst, liveID)
	if got := len(secondStore.Documents()); got != 6 {
		t.Errorf("restarted store holds %d documents, want the 6 the journal names", got)
	}
	if _, err := secondStore.LedgerSearch("too tight in the shoulders"); err != nil {
		t.Errorf("restored store cannot search its evidence: %v", err)
	}
	kept, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the journal after a restart: %v", err)
	}
	if !bytes.Equal(saved, kept) {
		t.Error("the boot replay rewrote the evidence journal")
	}

	third, _ := bootRestartedService(t, dir, seed)
	afterSecond := readRestartState(t, third)
	assertSameRestartReads(t, afterFirst, afterSecond)
	kept, err = os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the journal after a second restart: %v", err)
	}
	if !bytes.Equal(saved, kept) {
		t.Error("a second restart rewrote the evidence journal")
	}
}

// ledgerRow returns the ledger row with one id.
func ledgerRow(t *testing.T, reads restartReads, id string) contract.LedgerEvidenceRow {
	t.Helper()
	for _, row := range reads.Evidence.Evidence {
		if row.Evidence.EvidenceID == id {
			return row
		}
	}
	t.Fatalf("the ledger holds no row %q", id)
	return contract.LedgerEvidenceRow{}
}

// assertUnattachedAnswer proves one ledger row carries the shopper's
// words, counts toward nothing, and keeps the live event's link.
func assertUnattachedAnswer(t *testing.T, reads restartReads, liveID string) {
	t.Helper()
	row := ledgerRow(t, reads, liveID)
	if row.Evidence.Reason == nil || row.Evidence.Reason.Text != "changed my mind" {
		t.Fatalf("the unattached row reason = %+v, want the shopper's words", row.Evidence.Reason)
	}
	if row.Evidence.CountsToward != nil || len(row.BeliefIDs) != 0 {
		t.Fatalf("the unattached row counts toward %v %v, want nothing", row.Evidence.CountsToward, row.BeliefIDs)
	}
	if row.Evidence.Link.EventID != liveEventID {
		t.Fatalf("the unattached row names event %q, want %q", row.Evidence.Link.EventID, liveEventID)
	}
}

// TestEvidenceJournalKeepsAnUnattachedAnswer proves an answer that counts
// toward nothing is saved like any other classified answer: the row keeps
// the shopper's words, the question stays closed, and a restart answers
// both the same.
func TestEvidenceJournalKeepsAnUnattachedAnswer(t *testing.T) {
	dir := t.TempDir()
	seed, err := SeedFromExamples(apiDir(t))
	if err != nil {
		t.Fatalf("build seed: %v", err)
	}
	first, firstStore := bootRestartedService(t, dir, seed)
	questionID := fileLiveRefund(t, first)
	res, err := first.RecordReason(context.Background(), contract.RecordReasonRequest{
		ShopperID:  restartShopper,
		QuestionID: questionID,
		AnswerText: "changed my mind",
	})
	if err != nil {
		t.Fatalf("record the unattached answer: %v", err)
	}
	if res.Accepted || res.AskAgain || res.CountsToward != nil || res.BeliefID != nil {
		t.Fatalf("unattached answer = %+v, want it classified and stored with no belief", res)
	}
	before := readRestartState(t, first)
	liveID := liveEvidenceID(t, before)
	assertUnattachedAnswer(t, before, liveID)
	if got := len(firstStore.Documents()); got != 5 {
		t.Fatalf("live boot holds %d documents, want the 5 the seed indexes: an unattached answer writes no document", got)
	}

	second, _ := bootRestartedService(t, dir, seed)
	after := readRestartState(t, second)
	assertSameRestartReads(t, before, after)
	if got := liveEvidenceID(t, after); got != liveID {
		t.Errorf("restarted ledger holds the unattached row as %q, want %q", got, liveID)
	}
	assertUnattachedAnswer(t, after, liveID)
}

// TestEvidenceRestartWithoutJournalKeepsMemoryBehaviour proves the empty
// standing dir keeps the old shape: a restart drops the live rows and
// their citations, and only the boot replay comes back.
func TestEvidenceRestartWithoutJournalKeepsMemoryBehaviour(t *testing.T) {
	seed, err := SeedFromExamples(apiDir(t))
	if err != nil {
		t.Fatalf("build seed: %v", err)
	}
	first, _ := bootRestartedService(t, "", seed)
	driveLiveEvidence(t, first)
	before := readRestartState(t, first)
	liveID := liveEvidenceID(t, before)
	if got := len(before.Evidence.Evidence); got != 6 {
		t.Fatalf("live boot holds %d ledger rows, want 6", got)
	}

	second, secondStore := bootRestartedService(t, "", seed)
	after := readRestartState(t, second)
	if got := len(after.Evidence.Evidence); got != 5 {
		t.Errorf("restart without a journal holds %d ledger rows, want the 5 the seed replays", got)
	}
	for _, row := range after.Evidence.Evidence {
		if row.Evidence.EvidenceID == liveID {
			t.Errorf("restart without a journal kept the live row %s", liveID)
		}
	}
	for _, row := range after.Explain.Belief.Evidence {
		if row.EvidenceID == liveID {
			t.Errorf("restart without a journal cites the live row %s", liveID)
		}
	}
	if got := len(secondStore.Documents()); got != 5 {
		t.Errorf("restart without a journal holds %d documents, want the 5 the seed indexes", got)
	}
}
