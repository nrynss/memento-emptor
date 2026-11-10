package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// The two recorded answers the rival tests replay: one per direction.
// Both classify at 1.00, so every answer below lands classified.
const (
	rivalSmallAnswer = "too tight in the shoulders"
	rivalLargeAnswer = "it swims on me, far too big"
)

// rivalRefund builds one extra refund event with its order and refund
// records, shaped exactly like the frozen story's refunds: the order read
// carries the line item, the refund record carries no reason, and the
// custom id propagates across the link. These events live only in this
// test's inputs: the frozen story holds four refunds, and the reset
// tests need seven answers on one shopper.
func rivalRefund(n, eventID string) (fakes.WebhookEvent, fakes.OrderRecord, fakes.RefundRecord) {
	orderID := "ORD-7XK-" + n
	link := domain.TransactionLink{
		OrderID:   orderID,
		CaptureID: "CAP-7XK-" + n,
		RefundID:  "RFD-7XK-" + n,
		CustomID:  "memento-shopper01-" + n,
		EventID:   eventID,
	}
	item := domain.Item{
		BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt",
		SKU: "rival-tee-" + n, Size: "M", Category: "shirts",
	}
	outcome := domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"}
	event := fakes.WebhookEvent{
		EventID: eventID, EventType: "PAYMENT.CAPTURE.REFUNDED", Verified: true,
		Outcome: outcome, Link: link,
		Provenance: "testdata/paypal/webhook-delivery-refunded.json",
	}
	order := fakes.OrderRecord{
		OrderID: orderID, Items: []domain.Item{item}, Link: link,
		Provenance: "testdata/paypal/order-get-approved-line-items.json",
	}
	refund := fakes.RefundRecord{
		RefundID: link.RefundID, Outcome: outcome, Link: link,
		Provenance: "testdata/paypal/refund-record.json",
	}
	return event, order, refund
}

// rivalInputs extends the frozen story with six extra refunds, so one
// shopper can answer seven questions in a row.
func rivalInputs() fakes.StoryInputs {
	inputs := fakes.DemoStoryInputs()
	for i, event := range []string{"WH-2EV-07", "WH-2EV-08", "WH-2EV-09", "WH-2EV-10", "WH-2EV-11", "WH-2EV-12"} {
		n := fmt.Sprintf("%02d", 7+i)
		e, o, r := rivalRefund(n, event)
		inputs.Events = append(inputs.Events, e)
		inputs.Orders = append(inputs.Orders, o)
		inputs.Refunds = append(inputs.Refunds, r)
	}
	return inputs
}

// rivalAllowlist covers every content the rival drive writes: both fit
// belief contents, the correction verbatim, and the stubs for every
// refund event across the doc ids one fresh store can assign.
func rivalAllowlist(events []string) []string {
	out := []string{"I've lost weight, use M"}
	brand := domain.Subject{Kind: domain.SubjectBrand, ID: "brand-x-001", Name: "Brand X"}
	out = append(out, beliefContent(domain.BeliefKindFit, brand, fitBeliefValue(domain.CategoryFitTooSmall, "Brand X", "M")))
	out = append(out, beliefContent(domain.BeliefKindFit, brand, fitBeliefValue(domain.CategoryFitTooLarge, "Brand X", "M")))
	for _, event := range events {
		for i := 1; i <= 16; i++ {
			out = append(out, fmt.Sprintf("evidence[doc-%04d] refund %s", i, event))
		}
	}
	return out
}

// rivalHarness builds a service on the extended inputs with a pinned clock
// and a spy over the graph.
func rivalHarness(t *testing.T) (*Service, *spyGraph) {
	t.Helper()
	store, err := evidence.NewStore(testContract{kind: "server-rival"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	events := []string{
		"WH-2EV-01", "WH-2EV-02", "WH-2EV-03", "WH-2EV-05",
		"WH-2EV-07", "WH-2EV-08", "WH-2EV-09", "WH-2EV-10", "WH-2EV-11", "WH-2EV-12",
	}
	spy := &spyGraph{inner: fakes.NewRecordingEvidenceGraph(rivalAllowlist(events), fakes.StoryRecalls())}
	fixed := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:   fakes.NewRecordingPaymentSource(rivalInputs()),
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

// answerOne ingests one refund event and records one classified answer
// for its question.
func answerOne(t *testing.T, svc *Service, shopper, event, answer string) contract.RecordReasonResponse {
	t.Helper()
	if _, err := svc.IngestWebhook(context.Background(), shopper, event); err != nil {
		t.Fatalf("ingest %s: %v", event, err)
	}
	return answerReason(t, svc, shopper, event, answer)
}

// ledgerFitBeliefs returns one shopper's fit belief rows.
func ledgerFitBeliefs(t *testing.T, svc *Service, shopper string) []contract.LedgerBeliefRow {
	t.Helper()
	ledger, err := svc.LedgerBeliefs(context.Background(), contract.LedgerBeliefsRequest{ShopperID: shopper, IncludeSeeded: true})
	if err != nil {
		t.Fatalf("ledger beliefs: %v", err)
	}
	out := []contract.LedgerBeliefRow{}
	for _, row := range ledger.Beliefs {
		if row.Belief.Kind == domain.BeliefKindFit {
			out = append(out, row)
		}
	}
	return out
}

// unattachedLarge counts the ledger evidence rows that carry a too-large
// reason with no belief link.
func unattachedLarge(t *testing.T, svc *Service, shopper string) int {
	t.Helper()
	ledger, err := svc.LedgerEvidence(context.Background(), contract.LedgerEvidenceRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("ledger evidence: %v", err)
	}
	out := 0
	for _, row := range ledger.Evidence {
		if row.Evidence.Reason != nil && row.Evidence.Reason.Category == domain.CategoryFitTooLarge {
			if len(row.BeliefIDs) != 0 {
				t.Errorf("too-large row links %v, want unattached", row.BeliefIDs)
				continue
			}
			out++
		}
	}
	return out
}

// TestRivalBirthAtThree proves the pinned rule through the service: two
// consecutive opposite-direction returns land unattached, and the third
// births a same-kind opposite-direction rival belief the answer attaches
// to. The rival starts unpromoted, and the existing belief is untouched.
func TestRivalBirthAtThree(t *testing.T) {
	ctx := context.Background()
	svc, spy := rivalHarness(t)
	shopper := "shopper_rival_birth"

	first := answerOne(t, svc, shopper, "WH-2EV-01", rivalSmallAnswer)
	if !first.Accepted || first.BeliefID == nil {
		t.Fatalf("first return: accepted %v belief %v, want true and attached", first.Accepted, first.BeliefID)
	}
	smallID := *first.BeliefID

	second := answerOne(t, svc, shopper, "WH-2EV-02", rivalLargeAnswer)
	if second.Accepted || second.BeliefID != nil {
		t.Fatalf("first opposite: accepted %v belief %v, want false nil", second.Accepted, second.BeliefID)
	}
	third := answerOne(t, svc, shopper, "WH-2EV-03", rivalLargeAnswer)
	if third.Accepted || third.BeliefID != nil {
		t.Fatalf("second opposite: accepted %v belief %v, want false nil", third.Accepted, third.BeliefID)
	}
	if rows := ledgerFitBeliefs(t, svc, shopper); len(rows) != 1 {
		t.Fatalf("below the threshold the shopper holds %d fit beliefs, want 1: no rival yet", len(rows))
	}

	birth := answerOne(t, svc, shopper, "WH-2EV-05", rivalLargeAnswer)
	if !birth.Accepted || birth.AskAgain || birth.BeliefID == nil {
		t.Fatalf("third opposite: accepted %v ask again %v belief %v, want true false attached",
			birth.Accepted, birth.AskAgain, birth.BeliefID)
	}
	if birth.Category != domain.CategoryFitTooLarge {
		t.Fatalf("birth answer: category %q, want fit_too_large", string(birth.Category))
	}
	if birth.CountsToward == nil || *birth.CountsToward != domain.BeliefKindFit {
		t.Fatalf("birth answer counts toward %v, want fit", birth.CountsToward)
	}
	rivalID := *birth.BeliefID
	if rivalID == smallID {
		t.Fatal("the third opposite attached to the runs-small belief, want a new rival")
	}
	pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("pending after birth: %v", err)
	}
	if len(pending.Questions) != 0 {
		t.Fatalf("pending holds %d questions, want 0: the birth answer closed it", len(pending.Questions))
	}

	rows := ledgerFitBeliefs(t, svc, shopper)
	if len(rows) != 2 {
		t.Fatalf("after the birth the shopper holds %d fit beliefs, want 2", len(rows))
	}
	byID := map[string]contract.LedgerBeliefRow{}
	for _, row := range rows {
		byID[row.Belief.BeliefID] = row
	}
	rival, ok := byID[rivalID]
	if !ok {
		t.Fatalf("no ledger row for the rival %q", rivalID)
	}
	if rival.Belief.Value != "runs large, take size S" {
		t.Errorf("rival value %q, want the reason's direction in words", rival.Belief.Value)
	}
	if rival.Belief.Status != domain.StatusClaimed || rival.Belief.Standing != domain.GraphNone {
		t.Errorf("rival reads %q/%q, want Claimed/None: birth is not promotion",
			rival.Belief.Status, rival.Belief.Standing)
	}
	if rival.EvidenceCount != 1 {
		t.Errorf("rival holds %d rows, want the 1 birth return", rival.EvidenceCount)
	}
	if rival.Belief.Contradicted {
		t.Error("the newborn rival reads contradicted: nothing declared against it")
	}
	small, ok := byID[smallID]
	if !ok {
		t.Fatalf("no ledger row for the existing belief %q", smallID)
	}
	if small.EvidenceCount != 1 {
		t.Errorf("existing belief holds %d rows, want 1: the opposites added no support", small.EvidenceCount)
	}
	if small.Belief.Contradicted || small.Belief.Standing != domain.GraphNone {
		t.Errorf("existing belief reads contradicted %v standing %q, want false None: it is untouched",
			small.Belief.Contradicted, small.Belief.Standing)
	}
	if got := againstEdges(spy); got != 0 {
		t.Errorf("graph saw %d against edges, want 0: birth declares nothing", got)
	}
}

// TestRivalNoBirthBelowThree proves the threshold is exact: one
// opposite-direction return, and two, both land unattached with the
// question closed and no rival born.
func TestRivalNoBirthBelowThree(t *testing.T) {
	svc, spy := rivalHarness(t)
	shopper := "shopper_rival_below"

	first := answerOne(t, svc, shopper, "WH-2EV-01", rivalSmallAnswer)
	if !first.Accepted || first.BeliefID == nil {
		t.Fatalf("first return: accepted %v belief %v, want true and attached", first.Accepted, first.BeliefID)
	}
	for i, event := range []string{"WH-2EV-02", "WH-2EV-03"} {
		res := answerOne(t, svc, shopper, event, rivalLargeAnswer)
		if res.Accepted || res.AskAgain || res.BeliefID != nil {
			t.Fatalf("opposite %d: accepted %v ask again %v belief %v, want false false nil",
				i+1, res.Accepted, res.AskAgain, res.BeliefID)
		}
		if res.Category != domain.CategoryFitTooLarge {
			t.Fatalf("opposite %d: category %q, want fit_too_large", i+1, string(res.Category))
		}
		if res.CountsToward != nil {
			t.Fatalf("opposite %d counts toward %q, want nothing", i+1, string(*res.CountsToward))
		}
	}
	if rows := ledgerFitBeliefs(t, svc, shopper); len(rows) != 1 {
		t.Fatalf("after two opposites the shopper holds %d fit beliefs, want 1: no rival", len(rows))
	}
	if got := unattachedLarge(t, svc, shopper); got != 2 {
		t.Fatalf("ledger shows %d unattached too-large rows, want 2", got)
	}
	if got := againstEdges(spy); got != 0 {
		t.Errorf("graph saw %d against edges, want 0", got)
	}
}

// TestRivalResetOnInterruption proves the streak breaks on a
// same-direction return: opposite, opposite, same, opposite, opposite
// births nothing, and the very next opposite births the rival.
func TestRivalResetOnInterruption(t *testing.T) {
	svc, spy := rivalHarness(t)
	shopper := "shopper_rival_reset"

	first := answerOne(t, svc, shopper, "WH-2EV-01", rivalSmallAnswer)
	if !first.Accepted || first.BeliefID == nil {
		t.Fatalf("first return: accepted %v belief %v, want true and attached", first.Accepted, first.BeliefID)
	}
	smallID := *first.BeliefID

	for _, event := range []string{"WH-2EV-02", "WH-2EV-03"} {
		if res := answerOne(t, svc, shopper, event, rivalLargeAnswer); res.Accepted {
			t.Fatalf("%s opposite was accepted: the run is at 2", event)
		}
	}
	matched := answerOne(t, svc, shopper, "WH-2EV-05", rivalSmallAnswer)
	if !matched.Accepted || matched.BeliefID == nil || *matched.BeliefID != smallID {
		t.Fatalf("same-direction answer attaches to %v, want the runs-small %q", matched.BeliefID, smallID)
	}
	for _, event := range []string{"WH-2EV-07", "WH-2EV-08"} {
		if res := answerOne(t, svc, shopper, event, rivalLargeAnswer); res.Accepted {
			t.Fatalf("%s opposite was accepted after the interruption: want unattached", event)
		}
	}
	if rows := ledgerFitBeliefs(t, svc, shopper); len(rows) != 1 {
		t.Fatalf("after opposite, opposite, same, opposite, opposite the shopper holds %d fit beliefs, want 1", len(rows))
	}
	if got := unattachedLarge(t, svc, shopper); got != 4 {
		t.Fatalf("ledger shows %d unattached too-large rows, want 4", got)
	}

	birth := answerOne(t, svc, shopper, "WH-2EV-09", rivalLargeAnswer)
	if !birth.Accepted || birth.BeliefID == nil || *birth.BeliefID == smallID {
		t.Fatalf("third consecutive opposite after the reset attaches to %v, want a new rival", birth.BeliefID)
	}
	rival := fitBelief(t, svc, shopper, "runs large, take size S")
	if rival.Belief.BeliefID != *birth.BeliefID {
		t.Fatalf("rival id %q, want the birth answer's %q", rival.Belief.BeliefID, *birth.BeliefID)
	}
	if rival.EvidenceCount != 1 {
		t.Errorf("rival holds %d rows, want the 1 birth return", rival.EvidenceCount)
	}
	small := fitBelief(t, svc, shopper, "runs small, take size L")
	if small.EvidenceCount != 2 {
		t.Errorf("existing belief holds %d rows, want 2: only the same-direction return joined it", small.EvidenceCount)
	}
	if got := againstEdges(spy); got != 0 {
		t.Errorf("graph saw %d against edges, want 0", got)
	}
}

// TestRivalCorrectionResetsStreaks proves a shopper correction resets the
// run: opposite, opposite, correction, opposite, opposite births nothing,
// and the next opposite births the rival. The correction's own against
// edge is the only one on the graph.
func TestRivalCorrectionResetsStreaks(t *testing.T) {
	ctx := context.Background()
	svc, spy := rivalHarness(t)
	shopper := "shopper_rival_correction"

	first := answerOne(t, svc, shopper, "WH-2EV-01", rivalSmallAnswer)
	if !first.Accepted || first.BeliefID == nil {
		t.Fatalf("first return: accepted %v belief %v, want true and attached", first.Accepted, first.BeliefID)
	}
	smallID := *first.BeliefID

	for _, event := range []string{"WH-2EV-02", "WH-2EV-03"} {
		if res := answerOne(t, svc, shopper, event, rivalLargeAnswer); res.Accepted {
			t.Fatalf("%s opposite was accepted: the run is at 2", event)
		}
	}
	correction, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID:           shopper,
		Text:                "I've lost weight, use M",
		ContradictsBeliefID: smallID,
	})
	if err != nil {
		t.Fatalf("record correction: %v", err)
	}
	if !correction.Contradicted {
		t.Fatal("correction did not contradict the belief it names")
	}
	for _, event := range []string{"WH-2EV-05", "WH-2EV-07"} {
		if res := answerOne(t, svc, shopper, event, rivalLargeAnswer); res.Accepted {
			t.Fatalf("%s opposite was accepted after the correction: want unattached", event)
		}
	}
	if rows := ledgerFitBeliefs(t, svc, shopper); len(rows) != 1 {
		t.Fatalf("after opposite, opposite, correction, opposite, opposite the shopper holds %d fit beliefs, want 1", len(rows))
	}

	birth := answerOne(t, svc, shopper, "WH-2EV-08", rivalLargeAnswer)
	if !birth.Accepted || birth.BeliefID == nil || *birth.BeliefID == smallID {
		t.Fatalf("third consecutive opposite after the correction attaches to %v, want a new rival", birth.BeliefID)
	}
	rival := fitBelief(t, svc, shopper, "runs large, take size S")
	if rival.Belief.Status != domain.StatusClaimed || rival.Belief.Standing != domain.GraphNone {
		t.Errorf("rival reads %q/%q, want Claimed/None: birth is not promotion",
			rival.Belief.Status, rival.Belief.Standing)
	}
	small := fitBelief(t, svc, shopper, "runs small, take size L")
	if small.EvidenceCount != 1 || !small.Belief.Contradicted {
		t.Errorf("existing belief holds %d rows contradicted %v, want 1 true: the correction stands, the opposites added nothing",
			small.EvidenceCount, small.Belief.Contradicted)
	}
	if got := againstEdges(spy); got != 1 {
		t.Errorf("graph saw %d against edges, want 1: only the correction declares against", got)
	}
}

// TestRivalFourthOppositeJoinsRival proves the birth consumes the run: a
// fourth consecutive opposite-direction answer attaches to the newborn
// rival as a same-direction return instead of birthing a second rival.
// The existing belief stays untouched and nothing declares against.
func TestRivalFourthOppositeJoinsRival(t *testing.T) {
	svc, spy := rivalHarness(t)
	shopper := "shopper_rival_fourth"

	first := answerOne(t, svc, shopper, "WH-2EV-01", rivalSmallAnswer)
	if !first.Accepted || first.BeliefID == nil {
		t.Fatalf("first return: accepted %v belief %v, want true and attached", first.Accepted, first.BeliefID)
	}
	smallID := *first.BeliefID

	answerOne(t, svc, shopper, "WH-2EV-02", rivalLargeAnswer)
	answerOne(t, svc, shopper, "WH-2EV-03", rivalLargeAnswer)
	birth := answerOne(t, svc, shopper, "WH-2EV-05", rivalLargeAnswer)
	if !birth.Accepted || birth.BeliefID == nil || *birth.BeliefID == smallID {
		t.Fatalf("birth attaches to %v, want a new rival", birth.BeliefID)
	}
	rivalID := *birth.BeliefID

	fourth := answerOne(t, svc, shopper, "WH-2EV-07", rivalLargeAnswer)
	if !fourth.Accepted || fourth.BeliefID == nil || *fourth.BeliefID != rivalID {
		t.Fatalf("fourth consecutive opposite attaches to %v, want the rival %q", fourth.BeliefID, rivalID)
	}
	if rows := ledgerFitBeliefs(t, svc, shopper); len(rows) != 2 {
		t.Fatalf("after the fourth opposite the shopper holds %d fit beliefs, want 2: no second rival", len(rows))
	}
	rival := fitBelief(t, svc, shopper, "runs large, take size S")
	if rival.EvidenceCount != 2 {
		t.Errorf("rival holds %d rows, want 2: the fourth opposite joined it", rival.EvidenceCount)
	}
	small := fitBelief(t, svc, shopper, "runs small, take size L")
	if small.EvidenceCount != 1 {
		t.Errorf("existing belief holds %d rows, want 1: the opposites added no support", small.EvidenceCount)
	}
	if got := againstEdges(spy); got != 0 {
		t.Errorf("graph saw %d against edges, want 0: birth declares nothing", got)
	}
}
