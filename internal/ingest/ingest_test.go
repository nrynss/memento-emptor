package ingest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// stubSource is a PaymentSource over caller-built tables. It codes to
// the same interface the replay fake serves, so the tests can stage
// deliveries the committed story never covers: a bad signature, the
// dispute codes the story never files, and a refund that arrives before
// its capture.
type stubSource struct {
	events   map[string]fakes.WebhookEvent
	orders   map[string]fakes.OrderRecord
	refunds  map[string]fakes.RefundRecord
	disputes map[string]fakes.DisputeRecord
}

func (s *stubSource) VerifyWebhook(ctx context.Context, eventID string) (fakes.WebhookEvent, error) {
	if err := ctx.Err(); err != nil {
		return fakes.WebhookEvent{}, fmt.Errorf("stub: verify webhook %q: %w", eventID, err)
	}
	event, ok := s.events[eventID]
	if !ok {
		return fakes.WebhookEvent{}, fmt.Errorf("%w: webhook event %q", fakes.ErrNoRecording, eventID)
	}
	return event, nil
}

func (s *stubSource) GetOrder(ctx context.Context, orderID string) (fakes.OrderRecord, error) {
	if err := ctx.Err(); err != nil {
		return fakes.OrderRecord{}, fmt.Errorf("stub: get order %q: %w", orderID, err)
	}
	order, ok := s.orders[orderID]
	if !ok {
		return fakes.OrderRecord{}, fmt.Errorf("%w: order %q", fakes.ErrNoRecording, orderID)
	}
	return order, nil
}

func (s *stubSource) GetRefund(ctx context.Context, refundID string) (fakes.RefundRecord, error) {
	if err := ctx.Err(); err != nil {
		return fakes.RefundRecord{}, fmt.Errorf("stub: get refund %q: %w", refundID, err)
	}
	refund, ok := s.refunds[refundID]
	if !ok {
		return fakes.RefundRecord{}, fmt.Errorf("%w: refund %q", fakes.ErrNoRecording, refundID)
	}
	return refund, nil
}

func (s *stubSource) GetDispute(ctx context.Context, disputeID string) (fakes.DisputeRecord, error) {
	if err := ctx.Err(); err != nil {
		return fakes.DisputeRecord{}, fmt.Errorf("stub: get dispute %q: %w", disputeID, err)
	}
	dispute, ok := s.disputes[disputeID]
	if !ok {
		return fakes.DisputeRecord{}, fmt.Errorf("%w: dispute %q", fakes.ErrNoRecording, disputeID)
	}
	return dispute, nil
}

// fixedClock stamps every event with the same moment so synthetic tests
// never depend on the wall clock.
func fixedClock() func(string) time.Time {
	moment := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	return func(string) time.Time { return moment }
}

// shirtItem builds the test shirt the stub orders carry.
func shirtItem(sku string) domain.Item {
	return domain.Item{
		BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt",
		SKU: sku, Size: "M", Category: "shirts",
	}
}

// TestRedeliveredEventWritesOnce ingests one recorded refund twice. The
// second delivery returns the stored row with Duplicate set and the
// store still holds exactly one row and one pending question.
func TestRedeliveredEventWritesOnce(t *testing.T) {
	service := NewService(fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()))
	ctx := context.Background()

	first, err := service.Handle(ctx, "WH-2EV-01")
	if err != nil {
		t.Fatalf("first delivery failed: %v", err)
	}
	if first.Duplicate {
		t.Error("first delivery reports Duplicate")
	}
	if len(first.Rows) != 1 {
		t.Fatalf("first delivery stores %d rows, want 1", len(first.Rows))
	}
	if first.Pending == nil {
		t.Fatal("refund files no pending question")
	}

	second, err := service.Handle(ctx, "WH-2EV-01")
	if err != nil {
		t.Fatalf("redelivery failed: %v", err)
	}
	if !second.Duplicate {
		t.Error("redelivery does not report Duplicate")
	}
	if len(second.Rows) != 1 {
		t.Errorf("redelivery returns %d rows, want the 1 stored row", len(second.Rows))
	}
	if len(service.Rows()) != 1 {
		t.Errorf("store holds %d rows, want 1", len(service.Rows()))
	}
	if len(service.PendingQuestions()) != 1 {
		t.Errorf("store holds %d pending questions, want 1", len(service.PendingQuestions()))
	}
}

// TestBadSignatureRejectsDelivery stages a delivery that fails
// verification. Ingest rejects it with ErrBadSignature and stores
// nothing: an unverified delivery never becomes evidence.
func TestBadSignatureRejectsDelivery(t *testing.T) {
	item := shirtItem("tbnrl46")
	source := &stubSource{
		events: map[string]fakes.WebhookEvent{
			"WH-FORGE": {
				EventID: "WH-FORGE", EventType: "PAYMENT.CAPTURE.REFUNDED", Verified: false,
				Outcome: domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"},
				Link:    domain.TransactionLink{OrderID: "ORD-F", CaptureID: "CAP-F", RefundID: "RFD-F", CustomID: "forged", EventID: "WH-FORGE"},
			},
		},
		orders: map[string]fakes.OrderRecord{
			"ORD-F": {OrderID: "ORD-F", Items: []domain.Item{item}},
		},
		refunds: map[string]fakes.RefundRecord{
			"RFD-F": {RefundID: "RFD-F"},
		},
	}
	service := NewService(source, WithClock(fixedClock()))

	if _, err := service.Handle(context.Background(), "WH-FORGE"); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("forged delivery error = %v, want ErrBadSignature", err)
	}
	if len(service.Rows()) != 0 {
		t.Errorf("store holds %d rows after a rejected delivery, want 0", len(service.Rows()))
	}
	if len(service.PendingQuestions()) != 0 {
		t.Errorf("store holds %d pending questions after a rejected delivery, want 0", len(service.PendingQuestions()))
	}
}

// TestRefundBeforeCapture stages one purchase whose refund delivery
// arrives before its capture delivery. Both rows land exactly once, the
// refund files its pending question without waiting for the capture,
// and redelivering both changes nothing.
func TestRefundBeforeCapture(t *testing.T) {
	item := shirtItem("tbnrl46")
	outcome := func(kind domain.OutcomeKind) domain.Outcome {
		return domain.Outcome{Kind: kind, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"}
	}
	refundLink := domain.TransactionLink{OrderID: "ORD-T", CaptureID: "CAP-T", RefundID: "RFD-T", CustomID: "shopper-t", EventID: "WH-T-RFD"}
	captureLink := domain.TransactionLink{OrderID: "ORD-T", CaptureID: "CAP-T", CustomID: "shopper-t", EventID: "WH-T-CAP"}
	source := &stubSource{
		events: map[string]fakes.WebhookEvent{
			"WH-T-RFD": {EventID: "WH-T-RFD", EventType: "PAYMENT.CAPTURE.REFUNDED", Verified: true, Outcome: outcome(domain.OutcomeRefund), Link: refundLink},
			"WH-T-CAP": {EventID: "WH-T-CAP", EventType: "PAYMENT.CAPTURE.COMPLETED", Verified: true, Outcome: outcome(domain.OutcomeCapture), Link: captureLink},
		},
		orders: map[string]fakes.OrderRecord{
			"ORD-T": {OrderID: "ORD-T", Items: []domain.Item{item}, Link: refundLink},
		},
		refunds: map[string]fakes.RefundRecord{
			"RFD-T": {RefundID: "RFD-T", Outcome: outcome(domain.OutcomeRefund), Link: refundLink},
		},
	}
	service := NewService(source, WithClock(fixedClock()))
	ctx := context.Background()

	refund, err := service.Handle(ctx, "WH-T-RFD")
	if err != nil {
		t.Fatalf("refund before capture failed: %v", err)
	}
	if len(refund.Rows) != 1 || refund.Pending == nil {
		t.Fatalf("refund stores %d rows with pending %v, want 1 row and a question", len(refund.Rows), refund.Pending != nil)
	}
	if refund.Rows[0].Reason != nil {
		t.Error("refund row carries a reason; refunds arrive without one")
	}

	capture, err := service.Handle(ctx, "WH-T-CAP")
	if err != nil {
		t.Fatalf("capture after refund failed: %v", err)
	}
	if len(capture.Rows) != 1 {
		t.Fatalf("capture stores %d rows, want 1", len(capture.Rows))
	}
	if capture.Rows[0].Link.CustomID != "shopper-t" {
		t.Errorf("capture joins on custom id %q, want shopper-t", capture.Rows[0].Link.CustomID)
	}

	for _, id := range []string{"WH-T-RFD", "WH-T-CAP"} {
		redelivered, err := service.Handle(ctx, id)
		if err != nil {
			t.Fatalf("redelivery of %s failed: %v", id, err)
		}
		if !redelivered.Duplicate {
			t.Errorf("redelivery of %s does not report Duplicate", id)
		}
	}
	if len(service.Rows()) != 2 {
		t.Errorf("store holds %d rows, want 2: the refund and its capture", len(service.Rows()))
	}
	if len(service.PendingQuestions()) != 1 {
		t.Errorf("store holds %d pending questions, want 1", len(service.PendingQuestions()))
	}
}

// TestDisputeForEachMappedCode files a dispute for every code in the
// frozen table. The two arrival codes map to the not-arrived category
// and count toward the seller; the account-takeover code maps to no
// category and counts toward nothing. A code outside the table fails
// with the frozen unknown-code error and stores nothing.
func TestDisputeForEachMappedCode(t *testing.T) {
	item := shirtItem("chino-khaki-32")
	disputeFor := func(eventID, disputeID string, code domain.DisputeCode) (fakes.WebhookEvent, fakes.DisputeRecord, fakes.OrderRecord) {
		link := domain.TransactionLink{OrderID: "ORD-D-" + disputeID, CaptureID: "CAP-D-" + disputeID, DisputeID: disputeID, CustomID: "shopper-d", EventID: eventID}
		return fakes.WebhookEvent{
				EventID: eventID, EventType: "CUSTOMER.DISPUTE.CREATED", Verified: true,
				Outcome: domain.Outcome{Kind: domain.OutcomeDispute, Status: "OPEN", AmountMinor: 8900, Currency: "USD"},
				Link:    link, DisputeCode: code,
			},
			fakes.DisputeRecord{DisputeID: disputeID, Code: code, Item: item, Link: link},
			fakes.OrderRecord{OrderID: "ORD-D-" + disputeID, Items: []domain.Item{item}, Link: link}
	}
	source := &stubSource{
		events:   map[string]fakes.WebhookEvent{},
		orders:   map[string]fakes.OrderRecord{},
		disputes: map[string]fakes.DisputeRecord{},
	}
	cases := []struct {
		code     domain.DisputeCode
		category domain.Category
		toward   domain.BeliefKind
	}{
		{domain.DisputeMerchandiseNotReceived, domain.CategoryNotArrived, domain.BeliefKindSeller},
		{domain.DisputeItemNotReceived, domain.CategoryNotArrived, domain.BeliefKindSeller},
		{domain.DisputeUnauthorizedTransaction, domain.CategoryNone, domain.BeliefKindNone},
	}
	for i, tc := range cases {
		eventID := fmt.Sprintf("WH-D-0%d", i)
		event, dispute, order := disputeFor(eventID, fmt.Sprintf("PP-D-0%d", i), tc.code)
		source.events[eventID] = event
		source.disputes[dispute.DisputeID] = dispute
		source.orders[order.OrderID] = order
	}
	badLink := domain.TransactionLink{OrderID: "ORD-D-BAD", DisputeID: "PP-D-BAD", CustomID: "shopper-d", EventID: "WH-D-BAD"}
	source.events["WH-D-BAD"] = fakes.WebhookEvent{
		EventID: "WH-D-BAD", EventType: "CUSTOMER.DISPUTE.CREATED", Verified: true,
		Outcome: domain.Outcome{Kind: domain.OutcomeDispute, Status: "OPEN", AmountMinor: 100, Currency: "USD"},
		Link:    badLink, DisputeCode: "NOT_A_CODE",
	}
	source.disputes["PP-D-BAD"] = fakes.DisputeRecord{DisputeID: "PP-D-BAD", Code: "NOT_A_CODE", Item: item, Link: badLink}
	source.orders["ORD-D-BAD"] = fakes.OrderRecord{OrderID: "ORD-D-BAD", Items: []domain.Item{item}, Link: badLink}

	service := NewService(source, WithClock(fixedClock()))
	ctx := context.Background()
	for i, tc := range cases {
		eventID := fmt.Sprintf("WH-D-0%d", i)
		result, err := service.Handle(ctx, eventID)
		if err != nil {
			t.Fatalf("dispute %s failed: %v", tc.code, err)
		}
		if len(result.Rows) != 1 {
			t.Fatalf("dispute %s stores %d rows, want 1", tc.code, len(result.Rows))
		}
		row := result.Rows[0]
		if row.Reason == nil || row.Reason.Category != tc.category {
			t.Errorf("dispute %s category = %v, want %v", tc.code, row.Reason, tc.category)
		}
		if row.Reason.Text != string(tc.code) {
			t.Errorf("dispute %s keeps text %q, want the code", tc.code, row.Reason.Text)
		}
		if row.CountsToward() != tc.toward {
			t.Errorf("dispute %s counts toward %v, want %v", tc.code, row.CountsToward(), tc.toward)
		}
		if result.Pending != nil {
			t.Errorf("dispute %s files a pending question; only refunds ask the shopper", tc.code)
		}
	}
	if _, err := service.Handle(ctx, "WH-D-BAD"); !errors.Is(err, domain.ErrUnknownDisputeCode) {
		t.Fatalf("unknown dispute code error = %v, want ErrUnknownDisputeCode", err)
	}
	if len(service.Rows()) != len(cases) {
		t.Errorf("store holds %d rows, want %d: the unknown code stores nothing", len(service.Rows()), len(cases))
	}
}

// TestIngestStory replays the committed story's six deliveries through
// the service. Four refunds become reasonless rows plus four pending
// questions, the dispute arrives classified toward the seller, and the
// capture proves the purchase with no reason.
func TestIngestStory(t *testing.T) {
	service := NewService(fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()))
	ctx := context.Background()
	for _, eventID := range []string{"WH-2EV-01", "WH-2EV-02", "WH-2EV-03", "WH-2EV-04", "WH-2EV-05", "WH-2EV-06"} {
		if _, err := service.Handle(ctx, eventID); err != nil {
			t.Fatalf("ingest %s failed: %v", eventID, err)
		}
	}
	rows := service.Rows()
	if len(rows) != 6 {
		t.Fatalf("store holds %d rows, want 6", len(rows))
	}
	for _, row := range rows {
		if row.Source != domain.SourcePayment {
			t.Errorf("row %s source = %v, want payment", row.Link.EventID, row.Source)
		}
		if row.Item.SKU == "" {
			t.Errorf("row %s carries no line item; items come from the order read", row.Link.EventID)
		}
		if row.Seeded {
			t.Errorf("row %s is marked seeded; ingested rows are lived, not generated", row.Link.EventID)
		}
		switch row.Outcome.Kind {
		case domain.OutcomeRefund, domain.OutcomeCapture:
			if row.Reason != nil {
				t.Errorf("row %s (%s) carries a reason; it counts toward nothing", row.Link.EventID, row.Outcome.Kind)
			}
			if row.CountsToward() != domain.BeliefKindNone {
				t.Errorf("row %s (%s) counts toward %v, want nothing", row.Link.EventID, row.Outcome.Kind, row.CountsToward())
			}
		case domain.OutcomeDispute:
			if row.Reason == nil || row.Reason.Category != domain.CategoryNotArrived {
				t.Errorf("dispute row category = %v, want not_arrived", row.Reason)
			}
			if row.CountsToward() != domain.BeliefKindSeller {
				t.Errorf("dispute row counts toward %v, want seller reliability", row.CountsToward())
			}
		}
	}
	pending := service.PendingQuestions()
	if len(pending) != 4 {
		t.Fatalf("store holds %d pending questions, want 4: every refund awaits its reason", len(pending))
	}
	for _, question := range pending {
		if len(question.Items) == 0 {
			t.Errorf("pending %s names no items to ask about", question.EventID)
		}
	}
}

// TestUnknownEventRejectsDelivery stages a verified delivery whose kind
// ingest does not store. It fails with ErrUnknownEvent and stores
// nothing.
func TestUnknownEventRejectsDelivery(t *testing.T) {
	source := &stubSource{
		events: map[string]fakes.WebhookEvent{
			"WH-ODD": {
				EventID: "WH-ODD", EventType: "CATALOG.PRODUCT.CREATED", Verified: true,
				Outcome: domain.Outcome{Kind: "mystery", Status: "COMPLETED"},
				Link:    domain.TransactionLink{CustomID: "catalog", EventID: "WH-ODD"},
			},
		},
		orders:   map[string]fakes.OrderRecord{},
		refunds:  map[string]fakes.RefundRecord{},
		disputes: map[string]fakes.DisputeRecord{},
	}
	service := NewService(source, WithClock(fixedClock()))
	if _, err := service.Handle(context.Background(), "WH-ODD"); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("unknown event error = %v, want ErrUnknownEvent", err)
	}
	if len(service.Rows()) != 0 {
		t.Errorf("store holds %d rows after an unknown event, want 0", len(service.Rows()))
	}
}

// TestOrderWithoutLineItemsStagesNothing stages a refund whose order
// read carries no items. Evidence needs the item the outcome is about,
// so ingest fails with ErrNoLineItems and stores neither a row nor a
// question.
func TestOrderWithoutLineItemsStagesNothing(t *testing.T) {
	link := domain.TransactionLink{OrderID: "ORD-EMPTY", CaptureID: "CAP-EMPTY", RefundID: "RFD-EMPTY", CustomID: "empty", EventID: "WH-EMPTY"}
	source := &stubSource{
		events: map[string]fakes.WebhookEvent{
			"WH-EMPTY": {
				EventID: "WH-EMPTY", EventType: "PAYMENT.CAPTURE.REFUNDED", Verified: true,
				Outcome: domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 100, Currency: "USD"},
				Link:    link,
			},
		},
		orders: map[string]fakes.OrderRecord{
			"ORD-EMPTY": {OrderID: "ORD-EMPTY", Link: link},
		},
		refunds: map[string]fakes.RefundRecord{
			"RFD-EMPTY": {RefundID: "RFD-EMPTY", Link: link},
		},
	}
	service := NewService(source, WithClock(fixedClock()))
	if _, err := service.Handle(context.Background(), "WH-EMPTY"); !errors.Is(err, ErrNoLineItems) {
		t.Fatalf("itemless order error = %v, want ErrNoLineItems", err)
	}
	if len(service.Rows()) != 0 || len(service.PendingQuestions()) != 0 {
		t.Errorf("store holds %d rows and %d questions, want 0 and 0",
			len(service.Rows()), len(service.PendingQuestions()))
	}
}
