package server

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// recordedAction is one observed graph write.
type recordedAction struct {
	action    string
	dependsOn []string
	against   []string
}

// spyGraph delegates to the recording fake and records every write, so
// the test proves what the service asked the graph to hold: stubs with
// doc ids and no prose, corrections declared against.
type spyGraph struct {
	inner   *fakes.RecordingEvidenceGraph
	mu      sync.Mutex
	derives []string
	actions []recordedAction
}

// Derive records one belief or statement.
func (g *spyGraph) Derive(ctx context.Context, content, kind string, eventTime time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.derives = append(g.derives, content)
	return g.inner.Derive(ctx, content, kind, eventTime)
}

// RecordAction records one structural write.
func (g *spyGraph) RecordAction(ctx context.Context, action string, dependsOn, against []string, eventTime time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.actions = append(g.actions, recordedAction{action: action, dependsOn: dependsOn, against: against})
	return g.inner.RecordAction(ctx, action, dependsOn, against, eventTime)
}

// Recall replays the recorded hits.
func (g *spyGraph) Recall(ctx context.Context, query string) ([]string, error) {
	return g.inner.Recall(ctx, query)
}

// Inspect reports the promotion state of one belief.
func (g *spyGraph) Inspect(ctx context.Context, content string) (fakes.BeliefReport, error) {
	return g.inner.Inspect(ctx, content)
}

// storyHarness builds a service on the fakes with a pinned clock and a
// spy over the graph, and returns the spy for write assertions.
func storyHarness(t *testing.T) (*Service, *spyGraph) {
	t.Helper()
	store, err := evidence.NewStore(testContract{kind: "server-story"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	spy := &spyGraph{inner: fakes.NewRecordingEvidenceGraph(storyAllowlist(), fakes.StoryRecalls())}
	fixed := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:   fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      spy,
		SchemaDir:  apiDir(t),
		Clock:      func() time.Time { return fixed },
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc, spy
}

// storyAllowlist covers every content the story drive writes: the three
// belief contents, the correction verbatim, and the stubs for every story
// event across the doc ids one fresh store can assign. It also covers the
// runs-large belief the direction test earns, so the same harness serves
// both drives.
func storyAllowlist() []string {
	out := []string{"I've lost weight, use M"}
	brand := domain.Subject{Kind: domain.SubjectBrand, ID: "brand-x-001", Name: "Brand X"}
	out = append(out, beliefContent(domain.BeliefKindFit, brand, fitBeliefValue(domain.CategoryFitTooSmall, "Brand X", "M")))
	out = append(out, beliefContent(domain.BeliefKindFit, brand, fitBeliefValue(domain.CategoryFitTooLarge, "Brand X", "M")))
	seller := domain.Subject{Kind: domain.SubjectSeller, ID: "seller-for-brand-x-001", Name: "Brand X seller"}
	out = append(out, beliefContent(domain.BeliefKindSeller, seller, sellerBeliefValue))
	out = append(out, beliefContent(domain.BeliefKindFit, brand, feedbackBeliefValue("M")))
	keys := []string{
		"refund WH-2EV-01", "refund WH-2EV-02", "refund WH-2EV-03",
		"dispute WH-2EV-04", "refund WH-2EV-05", "capture WH-2EV-06",
	}
	for _, key := range keys {
		for i := 1; i <= 8; i++ {
			out = append(out, "evidence[doc-000"+string(rune('0'+i))+"] "+key)
		}
	}
	return out
}

// questionForEvent returns the open question filed by one webhook event.
func questionForEvent(t *testing.T, svc *Service, shopper, eventID string) string {
	t.Helper()
	pending, err := svc.PendingQuestions(context.Background(), contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("pending questions: %v", err)
	}
	for _, question := range pending.Questions {
		if question.Link.EventID == eventID {
			return question.QuestionID
		}
	}
	t.Fatalf("no pending question for event %s", eventID)
	return ""
}

// answerReason records one classified answer for one event's question.
func answerReason(t *testing.T, svc *Service, shopper, eventID, answer string) contract.RecordReasonResponse {
	t.Helper()
	res, err := svc.RecordReason(context.Background(), contract.RecordReasonRequest{
		ShopperID:  shopper,
		QuestionID: questionForEvent(t, svc, shopper, eventID),
		AnswerText: answer,
	})
	if err != nil {
		t.Fatalf("record reason for %s: %v", eventID, err)
	}
	return res
}

// TestChangedMindClosesQuestionWithoutBelief proves a clear answer that
// counts toward nothing closes its question without writing a belief: the
// refund stays an outcome with no cause, and no code path turns it into
// evidence. The writer-level rule has its own test; this one covers the
// service path that attaches the reason and closes the question.
func TestChangedMindClosesQuestionWithoutBelief(t *testing.T) {
	ctx := context.Background()
	svc, spy := storyHarness(t)
	shopper := "shopper_01"

	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-01"); err != nil {
		t.Fatalf("ingest WH-2EV-01: %v", err)
	}
	res := answerReason(t, svc, shopper, "WH-2EV-01", "changed my mind")
	if res.Accepted || res.AskAgain || res.BeliefID != nil {
		t.Fatalf("changed mind: accepted %v ask again %v belief %v, want false false nil",
			res.Accepted, res.AskAgain, res.BeliefID)
	}
	if res.Category != domain.CategoryChangedMind {
		t.Fatalf("changed mind: category %q, want changed_mind", string(res.Category))
	}
	if res.CountsToward != nil {
		t.Fatalf("changed mind counts toward %q, want nothing", string(*res.CountsToward))
	}
	pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("pending after changed mind: %v", err)
	}
	if len(pending.Questions) != 0 {
		t.Fatalf("pending holds %d questions, want 0: the answer closed it", len(pending.Questions))
	}
	ledger, err := svc.LedgerBeliefs(ctx, contract.LedgerBeliefsRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("ledger beliefs: %v", err)
	}
	if len(ledger.Beliefs) != 0 {
		t.Fatalf("ledger holds %d beliefs, want 0: nothing was earned", len(ledger.Beliefs))
	}
	spy.mu.Lock()
	actions := len(spy.actions)
	derives := len(spy.derives)
	spy.mu.Unlock()
	if actions != 0 || derives != 0 {
		t.Fatalf("graph saw %d actions and %d derives, want none", actions, derives)
	}
}

// TestStoryEndToEnd replays the demo story through the service on the
// fakes: webhooks in, classified answers, Earned after the third return,
// a correction that wins at once while the belief stands, a dispute that
// leaves sizing untouched, and feedback that grows the rival belief.
func TestStoryEndToEnd(t *testing.T) {
	ctx := context.Background()
	svc, spy := storyHarness(t)
	shopper := "shopper_01"

	for _, event := range []string{"WH-2EV-01", "WH-2EV-02", "WH-2EV-03"} {
		if _, err := svc.IngestWebhook(ctx, shopper, event); err != nil {
			t.Fatalf("ingest %s: %v", event, err)
		}
	}

	first := answerReason(t, svc, shopper, "WH-2EV-01", "too tight in the shoulders")
	if !first.Accepted || first.AskAgain {
		t.Fatalf("first return: accepted %v ask again %v, want true false", first.Accepted, first.AskAgain)
	}
	recall, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall after one return: %v", err)
	}
	// One return is thin evidence, not grounds for silence: recall serves
	// the belief with its Claimed label, its one row and the own-evidence
	// step that decides, and the agent shows the shopper how sure the
	// layer is instead of being told nothing.
	if len(recall.Beliefs) != 1 {
		t.Fatalf("one return serves %d beliefs, want the 1 thin fit belief", len(recall.Beliefs))
	}
	thin := recall.Beliefs[0]
	if thin.Status != domain.StatusClaimed || thin.Standing != domain.GraphNone {
		t.Errorf("thin fit belief reads %q/%q, want Claimed/None: a label, never a gate",
			thin.Status, thin.Standing)
	}
	if len(thin.Evidence) != 1 {
		t.Fatalf("thin fit belief carries %d evidence rows, want its one", len(thin.Evidence))
	}
	if thin.Evidence[0].Reason == nil || thin.Evidence[0].Reason.Category != domain.CategoryFitTooSmall {
		t.Errorf("thin fit belief's row reads %+v, want the too-small return", thin.Evidence[0].Reason)
	}
	if thin.Precedence == nil || thin.Precedence.Step != contract.GuidanceOwnEvidence || thin.Precedence.Overridden {
		t.Errorf("thin fit belief precedence %+v, want the deciding own-evidence step", thin.Precedence)
	}
	if recall.Guidance.Basis != contract.GuidanceOwnEvidence || recall.Guidance.Text != "Take size L for Brand X" {
		t.Errorf("one-return guidance %+v, want own evidence toward size L", recall.Guidance)
	}

	answerReason(t, svc, shopper, "WH-2EV-02", "the sleeves are way too short and I can't button it")
	// The third answer stays same-direction: the offline story proves the
	// frozen all-small fixtures, where every return counts toward the
	// runs-small belief. Opposite-direction answers are covered by the
	// direction test, which proves they attach nowhere on a
	// runs-small-only graph.
	third := answerReason(t, svc, shopper, "WH-2EV-03", "sleeves too short in M")
	if !third.Accepted {
		t.Fatal("third return was not accepted as evidence")
	}
	sizingID := ""
	if third.BeliefID != nil {
		sizingID = *third.BeliefID
	}
	earned, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall after three returns: %v", err)
	}
	if len(earned.Beliefs) != 1 {
		t.Fatalf("recall serves %d beliefs, want the 1 earned sizing belief", len(earned.Beliefs))
	}
	sizing := earned.Beliefs[0]
	if sizing.Status != domain.StatusEarned || sizing.Standing != domain.GraphVenerable {
		t.Errorf("sizing reads %q/%q, want Earned/Venerable", sizing.Status, sizing.Standing)
	}
	if sizing.Contradicted || earned.OverridingCorrection != nil {
		t.Error("no correction applies yet, but recall contradicts or overrides")
	}
	if earned.Guidance.Basis != contract.GuidanceBelief || earned.Guidance.Text != "Take size L for Brand X" {
		t.Errorf("guidance %+v, want belief guidance toward size L", earned.Guidance)
	}
	if earned.PendingCount != 0 {
		t.Errorf("pending count %d, want 0 before the fourth refund", earned.PendingCount)
	}

	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-05"); err != nil {
		t.Fatalf("ingest WH-2EV-05: %v", err)
	}
	vague := answerReason(t, svc, shopper, "WH-2EV-05", "not great")
	if vague.Accepted || !vague.AskAgain || vague.BeliefID != nil {
		t.Errorf("vague answer: accepted %v ask again %v belief %v, want false true nil",
			vague.Accepted, vague.AskAgain, vague.BeliefID)
	}
	stillPending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("pending after vague answer: %v", err)
	}
	if len(stillPending.Questions) != 1 {
		t.Errorf("pending holds %d questions, want the 1 vague one staying open", len(stillPending.Questions))
	}

	before, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall before dispute: %v", err)
	}
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-04"); err != nil {
		t.Fatalf("ingest dispute: %v", err)
	}
	after, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall after dispute: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Error("the non-arrival dispute moved the sizing recall")
	}
	sellerRows, err := svc.LedgerBeliefs(ctx, contract.LedgerBeliefsRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("ledger beliefs: %v", err)
	}
	sellerFound := false
	for _, row := range sellerRows.Beliefs {
		if row.Belief.Kind == domain.BeliefKindSeller {
			sellerFound = true
			if row.Belief.Status != domain.StatusClaimed || row.EvidenceCount != 1 {
				t.Errorf("seller belief reads %q with %d rows, want Claimed with 1: one dispute never earns",
					row.Belief.Status, row.EvidenceCount)
			}
		}
	}
	if !sellerFound {
		t.Error("the dispute wrote no seller belief")
	}

	traced, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001", TraceID: "trace-R"})
	if err != nil {
		t.Fatalf("traced recall: %v", err)
	}
	if len(traced.Beliefs) != 1 {
		t.Fatalf("traced recall serves %d beliefs, want 1", len(traced.Beliefs))
	}
	correction, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID:           shopper,
		Text:                "I've lost weight, use M",
		ContradictsBeliefID: sizingID,
		TraceID:             "trace-R",
	})
	if err != nil {
		t.Fatalf("record correction: %v", err)
	}
	if correction.Status != domain.StatusOutOfTrue || correction.Standing != domain.GraphVenerable || !correction.Contradicted {
		t.Errorf("correction leaves %q/%q contradicted %v, want Out of true/Venerable/true",
			correction.Status, correction.Standing, correction.Contradicted)
	}
	if correction.Guidance.Basis != contract.GuidanceCorrection || correction.Guidance.Text != "Take size M for Brand X" {
		t.Errorf("correction guidance %+v, want the M at once", correction.Guidance)
	}
	overridden, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall after correction: %v", err)
	}
	if len(overridden.Beliefs) != 1 || !overridden.Beliefs[0].Contradicted {
		t.Fatal("recall after correction serves no contradicted belief")
	}
	inline := overridden.Beliefs[0].ContradictedBy
	if overridden.OverridingCorrection == nil || inline == nil ||
		overridden.OverridingCorrection.CorrectionID != inline.CorrectionID {
		t.Fatal("the override does not ride both at the top level and inline")
	}
	if overridden.Guidance.Basis != contract.GuidanceCorrection ||
		overridden.Guidance.CorrectionID == nil ||
		*overridden.Guidance.CorrectionID != overridden.OverridingCorrection.CorrectionID {
		t.Error("guidance does not point at the overriding correction")
	}

	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-06"); err != nil {
		t.Fatalf("ingest capture: %v", err)
	}
	feedback, err := svc.RecordFeedback(ctx, contract.RecordFeedbackRequest{
		ShopperID:    shopper,
		CustomID:     "memento-shopper01-06",
		SKU:          "PSW8GeK",
		Size:         "M",
		FeedbackText: "The M fits well across the shoulders.",
	})
	if err != nil {
		t.Fatalf("record feedback: %v", err)
	}
	if !feedback.Accepted || feedback.Verdict != contract.FeedbackFitsWell {
		t.Errorf("feedback: accepted %v verdict %q, want true fits_well", feedback.Accepted, feedback.Verdict)
	}
	final, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("final recall: %v", err)
	}
	if len(final.Beliefs) != 2 || final.Beliefs[0].BeliefID != sizingID {
		t.Fatalf("final recall serves %d beliefs, want the sizing belief first and the fits-well feedback beside it",
			len(final.Beliefs))
	}
	// The feedback's belief carries one row, which is thin, and thin is
	// served: labelled Claimed, after the belief with more evidence.
	rival := final.Beliefs[1]
	if rival.Value != feedbackBeliefValue("M") || rival.Status != domain.StatusClaimed {
		t.Errorf("rival belief reads %q/%q, want the Claimed %q",
			rival.Value, rival.Status, feedbackBeliefValue("M"))
	}
	if len(rival.Evidence) != 1 || rival.Precedence == nil || rival.Precedence.Step != contract.GuidanceOwnEvidence {
		t.Errorf("rival belief carries %d rows and precedence %+v, want its one row on the own-evidence step",
			len(rival.Evidence), rival.Precedence)
	}

	calls := svc.Calls("trace-R")
	if len(calls) != 2 || calls[0].Tool != string(contract.ToolRecallForPurchase) || calls[1].Tool != string(contract.ToolRecordCorrection) {
		t.Errorf("trace-R log shows %v, want recall before the correction it caused", calls)
	}

	spy.mu.Lock()
	defer spy.mu.Unlock()
	prose := []string{
		"too tight in the shoulders",
		"the sleeves are way too short",
		"sleeves too short in M",
		"MERCHANDISE_OR_SERVICE_NOT_RECEIVED",
		"The M fits well across the shoulders.",
	}
	for _, action := range spy.actions {
		if len(action.dependsOn) > 0 {
			if _, err := evidence.StubDocID(action.action); err != nil {
				t.Errorf("evidence action %q carries no doc id: %v", action.action, err)
			}
			for _, words := range prose {
				if strings.Contains(action.action, words) {
					t.Errorf("graph action %q carries shopper prose", action.action)
				}
			}
		}
		if len(action.against) > 0 && action.action != "I've lost weight, use M" {
			t.Errorf("against action %q is not the verbatim correction", action.action)
		}
	}
}
