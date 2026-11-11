package server

import (
	"context"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// feedbackSends is one purchase's feedback delivered three times: the
// first recording, an exact retry, and a duplicated delivery in other
// words. The layer joins on the purchase, not the words, so all three are
// the same feedback.
var feedbackSends = []struct {
	name string
	text string
}{
	{name: "first", text: "The M fits well across the shoulders."},
	{name: "retry", text: "The M fits well across the shoulders."},
	{name: "other words", text: "fits well on me"},
}

// TestFeedbackIsIdempotentPerPurchase proves one purchase's feedback is
// recorded once: three deliveries leave one evidence row on the belief,
// one feedback id, and the status the belief held after the first.
func TestFeedbackIsIdempotentPerPurchase(t *testing.T) {
	ctx := context.Background()
	shopper := "shopper_01"
	svc, spy := storyHarness(t)
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-06"); err != nil {
		t.Fatalf("ingest capture: %v", err)
	}
	spy.mu.Lock()
	before := len(spy.actions)
	spy.mu.Unlock()

	var first contract.RecordFeedbackResponse
	var status domain.BeliefStatus
	var evidenceID string
	for _, send := range feedbackSends {
		res, err := svc.RecordFeedback(ctx, contract.RecordFeedbackRequest{
			ShopperID:    shopper,
			CustomID:     "memento-shopper01-06",
			SKU:          "PSW8GeK",
			Size:         "M",
			FeedbackText: send.text,
		})
		if err != nil {
			t.Fatalf("%s send: %v", send.name, err)
		}
		if !res.Accepted || res.Verdict != contract.FeedbackFitsWell {
			t.Fatalf("%s send: accepted %v verdict %q, want true fits_well", send.name, res.Accepted, res.Verdict)
		}
		if res.CountsToward == nil || *res.CountsToward != domain.BeliefKindFit {
			t.Fatalf("%s send: counts toward %v, want fit", send.name, res.CountsToward)
		}
		if res.BeliefID == nil {
			t.Fatalf("%s send: no belief named", send.name)
		}
		if first.BeliefID == nil {
			first = res
			if res.FeedbackID == "" {
				t.Fatal("first send minted no feedback id")
			}
		}
		explained, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: *first.BeliefID})
		if err != nil {
			t.Fatalf("%s send: explain belief: %v", send.name, err)
		}
		if len(explained.Belief.Evidence) != 1 {
			t.Fatalf("%s send left %d evidence rows, want 1", send.name, len(explained.Belief.Evidence))
		}
		if status == "" {
			status = explained.Belief.Status
			evidenceID = explained.Belief.Evidence[0].EvidenceID
		}
		if explained.Belief.Evidence[0].EvidenceID != evidenceID {
			t.Fatalf("%s send: the one row is %q, want the purchase's row %q",
				send.name, explained.Belief.Evidence[0].EvidenceID, evidenceID)
		}
		if explained.Belief.Status != status {
			t.Fatalf("%s send moved the status %q → %q, want it to stay", send.name, status, explained.Belief.Status)
		}
		if res.FeedbackID != first.FeedbackID {
			t.Fatalf("%s send: feedback id %q, want the first id %q", send.name, res.FeedbackID, first.FeedbackID)
		}
		if *res.BeliefID != *first.BeliefID {
			t.Fatalf("%s send: belief %q, want %q", send.name, *res.BeliefID, *first.BeliefID)
		}
	}

	spy.mu.Lock()
	after := len(spy.actions)
	spy.mu.Unlock()
	if after-before != 1 {
		t.Fatalf("three sends wrote %d actions, want 1", after-before)
	}
	st, err := svc.state(ctx, shopper)
	if err != nil {
		t.Fatalf("partition: %v", err)
	}
	if st.seq["fb"] != 1 {
		t.Fatalf("three sends minted %d feedback ids, want 1", st.seq["fb"])
	}
}

// TestSeededFeedbackIsNotDoubleCounted proves the seeded arc's own copy
// is the purchase's feedback: a delivery for a purchase the history
// already holds writes nothing, mints no id, and answers with the id the
// fixtures recorded for it. The evidence row listed once, its reason text
// untouched, and the belief's status unmoved.
func TestSeededFeedbackIsNotDoubleCounted(t *testing.T) {
	ctx := context.Background()
	shopper := "shopper_01"
	svc, spy := seededSpyService(t)
	spy.mu.Lock()
	before := len(spy.actions)
	spy.mu.Unlock()
	st, err := svc.state(ctx, shopper)
	if err != nil {
		t.Fatalf("partition: %v", err)
	}
	priorReason := st.evidence["ev_51c2d8"].row.Reason.Text

	for _, send := range feedbackSends {
		res, err := svc.RecordFeedback(ctx, contract.RecordFeedbackRequest{
			ShopperID:    shopper,
			CustomID:     "memento-shopper01-06",
			SKU:          "PSW8GeK",
			Size:         "M",
			FeedbackText: send.text,
		})
		if err != nil {
			t.Fatalf("%s send: %v", send.name, err)
		}
		explained, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: "blf_2b9d60"})
		if err != nil {
			t.Fatalf("%s send: explain seeded belief: %v", send.name, err)
		}
		if len(explained.Belief.Evidence) != 1 || explained.Belief.Evidence[0].EvidenceID != "ev_51c2d8" {
			t.Fatalf("%s send: the seeded belief lists %d evidence rows %v, want the one seeded copy ev_51c2d8",
				send.name, len(explained.Belief.Evidence), evidenceIDs(explained.Belief))
		}
		if explained.Belief.Status != domain.StatusClaimed {
			t.Fatalf("%s send: the seeded belief reads %q, want the Claimed it loaded with", send.name, explained.Belief.Status)
		}
		if res.FeedbackID != "fb_51c2d8" {
			t.Fatalf("%s send: feedback id %q, want the id the history recorded fb_51c2d8", send.name, res.FeedbackID)
		}
		if !res.Accepted || res.Verdict != contract.FeedbackFitsWell {
			t.Fatalf("%s send: accepted %v verdict %q, want true fits_well", send.name, res.Accepted, res.Verdict)
		}
		if res.BeliefID == nil || *res.BeliefID != "blf_2b9d60" {
			t.Fatalf("%s send: belief %v, want blf_2b9d60", send.name, res.BeliefID)
		}
	}

	explained, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: "blf_2b9d60"})
	if err != nil {
		t.Fatalf("explain seeded belief: %v", err)
	}
	if reason := explained.Belief.Evidence[0].Reason; reason == nil || reason.Text != priorReason {
		t.Fatalf("the seeded row's reason is %v, want its own words %q", reason, priorReason)
	}
	if reason := explained.Belief.Evidence[0].Reason; reason == nil || reason.Text != priorReason {
		t.Fatalf("the seeded row's reason is %v, want its own words %q", reason, priorReason)
	}
	spy.mu.Lock()
	after := len(spy.actions)
	spy.mu.Unlock()
	if after != before {
		t.Fatalf("three sends wrote %d actions to the graph, want none", after-before)
	}
	if st.seq["fb"] != 0 {
		t.Fatalf("three sends minted %d feedback ids, want none", st.seq["fb"])
	}
}

// evidenceIDs lists the evidence ids one belief view carries.
func evidenceIDs(view contract.BeliefView) []string {
	out := make([]string, 0, len(view.Evidence))
	for _, ev := range view.Evidence {
		out = append(out, ev.EvidenceID)
	}
	return out
}

// seededSpyService builds the seeded service with a spy over the graph,
// so a repeat delivery can be proved to write nothing.
func seededSpyService(t *testing.T) (*Service, *spyGraph) {
	t.Helper()
	dir := apiDir(t)
	seed, err := SeedFromExamples(dir)
	if err != nil {
		t.Fatalf("build seed: %v", err)
	}
	store, err := evidence.NewStore(testContract{kind: "server-feedback"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	spy := &spyGraph{inner: fakes.NewRecordingEvidenceGraph(SeedGraphContents(seed), fakes.StoryRecalls())}
	fixed := time.Date(2026, 11, 8, 18, 40, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:   fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      spy,
		SchemaDir:  dir,
		Clock:      func() time.Time { return fixed },
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if err := svc.LoadSeed(context.Background(), seed); err != nil {
		t.Fatalf("load seed: %v", err)
	}
	return svc, spy
}
