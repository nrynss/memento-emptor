package fakes

import (
	"context"
	"fmt"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// WebhookEvent is one verified delivery from the payment source: the event
// id that keys ingest idempotency, the event type, and the outcome it
// carries with the links that join it to its order.
type WebhookEvent struct {
	// EventID keys webhook idempotency on the payment source's event id.
	EventID string `json:"event_id"`
	// EventType is the delivery's type, such as PAYMENT.CAPTURE.REFUNDED.
	EventType string `json:"event_type"`
	// Verified is true when signature verification said SUCCESS, as every
	// recorded delivery's verification did.
	Verified bool `json:"verified"`
	// Outcome is what happened to the money.
	Outcome domain.Outcome `json:"outcome"`
	// Item is the purchased thing the outcome concerns. It is empty when
	// the delivery carries no line data and the layer must read the order.
	Item domain.Item `json:"item"`
	// Link ties the outcome to its transaction across order, capture,
	// refund and dispute records.
	Link domain.TransactionLink `json:"link"`
	// DisputeCode carries the source's reason code for dispute events and
	// is empty otherwise.
	DisputeCode domain.DisputeCode `json:"dispute_code,omitempty"`
	// Provenance names the P0 recording this shape replays.
	Provenance string `json:"provenance"`
}

// OrderRecord is the order read: the only place line items live. Capture
// records carry no items, so the layer joins on the custom id, never on a
// reporting call the app is not authorized for.
type OrderRecord struct {
	// OrderID is the checkout order id.
	OrderID string `json:"order_id"`
	// Items are the order's line items.
	Items []domain.Item `json:"items"`
	// Link ties the order to its transaction.
	Link domain.TransactionLink `json:"link"`
	// Provenance names the P0 recording this shape replays.
	Provenance string `json:"provenance"`
}

// RefundRecord is one refund read. Recorded refunds carry no reason field,
// twice confirmed live, so HasReason is always false and the reason comes
// from the shopper, never from this record.
type RefundRecord struct {
	// RefundID is the refund id.
	RefundID string `json:"refund_id"`
	// Outcome is the completed refund and its amount.
	Outcome domain.Outcome `json:"outcome"`
	// Link ties the refund to its transaction.
	Link domain.TransactionLink `json:"link"`
	// HasReason is always false: no recorded refund carries a reason.
	HasReason bool `json:"has_reason"`
	// Provenance names the P0 recording this shape replays.
	Provenance string `json:"provenance"`
}

// DisputeRecord is one dispute read. Disputes carry the source's reason
// top-level and per item, so the category is classified on arrival.
type DisputeRecord struct {
	// DisputeID is the dispute id.
	DisputeID string `json:"dispute_id"`
	// Code is the source's reason code.
	Code domain.DisputeCode `json:"code"`
	// Category is the code's mapped reason category.
	Category domain.Category `json:"category"`
	// Item is the disputed item.
	Item domain.Item `json:"item"`
	// Link ties the dispute to its transaction.
	Link domain.TransactionLink `json:"link"`
	// Provenance names the P0 recording this shape replays.
	Provenance string `json:"provenance"`
}

// PaymentSource is the seam the real payment client implements in P2: the
// verified webhook deliveries and the reads that join them to their items.
// Every method takes ctx first; every unknown id fails with
// ErrNoRecording.
type PaymentSource interface {
	// VerifyWebhook returns the verified delivery for one event id.
	VerifyWebhook(ctx context.Context, eventID string) (WebhookEvent, error)
	// GetOrder returns the order read with its line items.
	GetOrder(ctx context.Context, orderID string) (OrderRecord, error)
	// GetRefund returns the refund read, which never carries a reason.
	GetRefund(ctx context.Context, refundID string) (RefundRecord, error)
	// GetDispute returns the dispute read with its reason code mapped.
	GetDispute(ctx context.Context, disputeID string) (DisputeRecord, error)
}

// RecordingPaymentSource is a PaymentSource that replays the P0
// recordings' shapes under the ledger story's ids. Unknown ids fail with
// ErrNoRecording; the fake never invents a transaction.
type RecordingPaymentSource struct {
	events   map[string]WebhookEvent
	orders   map[string]OrderRecord
	refunds  map[string]RefundRecord
	disputes map[string]DisputeRecord
}

// NewRecordingPaymentSource builds the payment fake over one story table.
// Each entry carries its provenance: the P0 recording whose shape it
// replays. The story table lives in testdata/replay/p2-inputs.json; see
// LoadStoryInputs.
func NewRecordingPaymentSource(inputs StoryInputs) *RecordingPaymentSource {
	fake := &RecordingPaymentSource{
		events:   make(map[string]WebhookEvent, len(inputs.Events)),
		orders:   make(map[string]OrderRecord, len(inputs.Orders)),
		refunds:  make(map[string]RefundRecord, len(inputs.Refunds)),
		disputes: make(map[string]DisputeRecord, len(inputs.Disputes)),
	}
	for _, event := range inputs.Events {
		fake.events[event.EventID] = event
	}
	for _, order := range inputs.Orders {
		fake.orders[order.OrderID] = order
	}
	for _, refund := range inputs.Refunds {
		fake.refunds[refund.RefundID] = refund
	}
	for _, dispute := range inputs.Disputes {
		fake.disputes[dispute.DisputeID] = dispute
	}
	return fake
}

// VerifyWebhook returns the verified delivery for one event id, or
// ErrNoRecording when no recording covers it.
func (f *RecordingPaymentSource) VerifyWebhook(ctx context.Context, eventID string) (WebhookEvent, error) {
	if err := ctx.Err(); err != nil {
		return WebhookEvent{}, fmt.Errorf("fakes: verify webhook %q: %w", eventID, err)
	}
	event, ok := f.events[eventID]
	if !ok {
		return WebhookEvent{}, fmt.Errorf("%w: webhook event %q", ErrNoRecording, eventID)
	}
	return event, nil
}

// GetOrder returns the order read with its line items, or ErrNoRecording
// when no recording covers the order.
func (f *RecordingPaymentSource) GetOrder(ctx context.Context, orderID string) (OrderRecord, error) {
	if err := ctx.Err(); err != nil {
		return OrderRecord{}, fmt.Errorf("fakes: get order %q: %w", orderID, err)
	}
	order, ok := f.orders[orderID]
	if !ok {
		return OrderRecord{}, fmt.Errorf("%w: order %q", ErrNoRecording, orderID)
	}
	return order, nil
}

// GetRefund returns the refund read, or ErrNoRecording when no recording
// covers the refund.
func (f *RecordingPaymentSource) GetRefund(ctx context.Context, refundID string) (RefundRecord, error) {
	if err := ctx.Err(); err != nil {
		return RefundRecord{}, fmt.Errorf("fakes: get refund %q: %w", refundID, err)
	}
	refund, ok := f.refunds[refundID]
	if !ok {
		return RefundRecord{}, fmt.Errorf("%w: refund %q", ErrNoRecording, refundID)
	}
	return refund, nil
}

// GetDispute returns the dispute read, or ErrNoRecording when no
// recording covers the dispute.
func (f *RecordingPaymentSource) GetDispute(ctx context.Context, disputeID string) (DisputeRecord, error) {
	if err := ctx.Err(); err != nil {
		return DisputeRecord{}, fmt.Errorf("fakes: get dispute %q: %w", disputeID, err)
	}
	dispute, ok := f.disputes[disputeID]
	if !ok {
		return DisputeRecord{}, fmt.Errorf("%w: dispute %q", ErrNoRecording, disputeID)
	}
	return dispute, nil
}

// storyItem builds the demo story's returned shirt item. All three fit
// returns share the brand, line and size; the SKUs follow the seeded
// history's rotation across the line's colorways.
func storyItem(sku string) domain.Item {
	return domain.Item{
		BrandID:   "brand-x-001",
		BrandName: "Brand X",
		LineName:  "oxford shirt",
		SKU:       sku,
		Size:      "M",
		Category:  "shirts",
	}
}

// storyLink mints the story's transaction link. The custom id propagates
// across order, capture, refund and dispute, so it is the join key; the
// event id keys webhook idempotency.
func storyLink(order, capture, refund, dispute, custom, event string) domain.TransactionLink {
	return domain.TransactionLink{
		OrderID:   order,
		CaptureID: capture,
		RefundID:  refund,
		DisputeID: dispute,
		CustomID:  custom,
		EventID:   event,
	}
}

// StoryInputs is the payment leg of the demo story, served from
// testdata/replay/p2-inputs.json so P2 can ingest it alone.
type StoryInputs struct {
	// Events are the webhook deliveries in arrival order.
	Events []WebhookEvent `json:"events"`
	// Orders are the order reads behind the events.
	Orders []OrderRecord `json:"orders"`
	// Refunds are the refund reads behind the refund events.
	Refunds []RefundRecord `json:"refunds"`
	// Disputes are the dispute reads behind the dispute event.
	Disputes []DisputeRecord `json:"disputes"`
}

// DemoStoryInputs returns the payment leg of the demo story: three fit
// refunds, one non-arrival dispute, one pending refund with no reason,
// and the approved purchase's capture. Shapes replay the P0 recordings
// named in each entry's provenance; ids are the ledger story's.
func DemoStoryInputs() StoryInputs {
	refund := func(n, sku, event string, minor int64) (WebhookEvent, OrderRecord, RefundRecord) {
		order := "ORD-7XK-" + n
		capture := "CAP-7XK-" + n
		refundID := "RFD-7XK-" + n
		custom := "memento-shopper01-" + n
		link := storyLink(order, capture, refundID, "", custom, event)
		item := storyItem(sku)
		outcome := domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: minor, Currency: "USD"}
		return WebhookEvent{
				EventID: event, EventType: "PAYMENT.CAPTURE.REFUNDED", Verified: true,
				Outcome: outcome, Item: domain.Item{}, Link: link,
				Provenance: "testdata/paypal/webhook-delivery-refunded.json",
			},
			OrderRecord{
				OrderID: order, Items: []domain.Item{item}, Link: link,
				Provenance: "testdata/paypal/order-get-approved-line-items.json",
			},
			RefundRecord{
				RefundID: refundID, Outcome: outcome, Link: link, HasReason: false,
				Provenance: "testdata/paypal/refund-record.json",
			}
	}
	e1, o1, r1 := refund("01", "tbnrl46", "WH-2EV-01", 19500)
	e2, o2, r2 := refund("02", "tbnrl46", "WH-2EV-02", 19500)
	e3, o3, r3 := refund("03", "tbnrl46", "WH-2EV-03", 19500)
	e5, o5, r5 := refund("05", "tbnrl46", "WH-2EV-05", 19500)

	disputeLink := storyLink("ORD-7XK-04", "CAP-7XK-04", "", "PP-D-51713", "memento-shopper01-04", "WH-2EV-04")
	disputeItem := domain.Item{
		BrandID: "brand-x-001", BrandName: "Brand X", LineName: "chino",
		SKU: "chino-khaki-32", Size: "32", Category: "pants",
	}
	disputeOutcome := domain.Outcome{Kind: domain.OutcomeDispute, Status: "OPEN", AmountMinor: 8900, Currency: "USD"}
	disputeEvent := WebhookEvent{
		EventID: "WH-2EV-04", EventType: "CUSTOMER.DISPUTE.CREATED", Verified: true,
		Outcome: disputeOutcome, Link: disputeLink,
		DisputeCode: domain.DisputeMerchandiseNotReceived,
		Provenance:  "testdata/paypal/webhook-delivery-dispute.json",
	}
	dispute := DisputeRecord{
		DisputeID: "PP-D-51713", Code: domain.DisputeMerchandiseNotReceived,
		Category: domain.CategoryNotArrived, Item: disputeItem, Link: disputeLink,
		Provenance: "testdata/paypal/dispute-get.json",
	}

	captureLink := storyLink("ORD-7XK-06", "CAP-7XK-06", "", "", "memento-shopper01-06", "WH-2EV-06")
	captureItem := domain.Item{
		BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt",
		SKU: "PSW8GeK", Size: "M", Category: "shirts",
	}
	captureEvent := WebhookEvent{
		EventID: "WH-2EV-06", EventType: "PAYMENT.CAPTURE.COMPLETED", Verified: true,
		Outcome:    domain.Outcome{Kind: domain.OutcomeCapture, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"},
		Link:       captureLink,
		Provenance: "testdata/paypal/capture-completed.json",
	}
	captureOrder := OrderRecord{
		OrderID: "ORD-7XK-06", Items: []domain.Item{captureItem}, Link: captureLink,
		Provenance: "testdata/paypal/order-get-approved-line-items.json",
	}

	return StoryInputs{
		Events:   []WebhookEvent{e1, e2, e3, disputeEvent, e5, captureEvent},
		Orders:   []OrderRecord{o1, o2, o3, {OrderID: "ORD-7XK-04", Items: []domain.Item{disputeItem}, Link: disputeLink, Provenance: "testdata/paypal/order-get-approved-line-items.json"}, o5, captureOrder},
		Refunds:  []RefundRecord{r1, r2, r3, r5},
		Disputes: []DisputeRecord{dispute},
	}
}

// OccurredAt returns the story's event time for one event id. Event time
// is fixed so promotion is deterministic without waiting; the three
// returns span twelve days, honestly spread out.
func OccurredAt(eventID string) time.Time {
	dates := map[string]string{
		"WH-2EV-01": "2026-10-14T10:00:00Z",
		"WH-2EV-02": "2026-10-20T10:00:00Z",
		"WH-2EV-03": "2026-10-26T10:00:00Z",
		"WH-2EV-04": "2026-11-06T11:30:00Z",
		"WH-2EV-05": "2026-11-07T09:15:00Z",
		"WH-2EV-06": "2026-11-08T20:10:00Z",
	}
	raw, ok := dates[eventID]
	if !ok {
		return time.Time{}
	}
	moment, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return moment
}
