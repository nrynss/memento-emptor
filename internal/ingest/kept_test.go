package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// keptOnlySource is a PaymentSource whose one delivery is a kept outcome.
// No recorded delivery carries one: the payment source emits captures,
// refunds and disputes, and nothing watches a return window close.
type keptOnlySource struct{}

// VerifyWebhook returns a verified kept delivery.
func (keptOnlySource) VerifyWebhook(_ context.Context, eventID string) (fakes.WebhookEvent, error) {
	return fakes.WebhookEvent{
		EventID:   eventID,
		EventType: "PAYMENT.CAPTURE.KEPT",
		Verified:  true,
		Outcome:   domain.Outcome{Kind: domain.OutcomeKept, Status: "PAST_WINDOW"},
		Link:      domain.TransactionLink{OrderID: "ORD-KEPT-01", CaptureID: "CAP-KEPT-01"},
	}, nil
}

// GetOrder reports no recording: a kept delivery reads no order.
func (keptOnlySource) GetOrder(context.Context, string) (fakes.OrderRecord, error) {
	return fakes.OrderRecord{}, fakes.ErrNoRecording
}

// GetRefund reports no recording: a kept delivery reads no refund.
func (keptOnlySource) GetRefund(context.Context, string) (fakes.RefundRecord, error) {
	return fakes.RefundRecord{}, fakes.ErrNoRecording
}

// GetDispute reports no recording: a kept delivery reads no dispute.
func (keptOnlySource) GetDispute(context.Context, string) (fakes.DisputeRecord, error) {
	return fakes.DisputeRecord{}, fakes.ErrNoRecording
}

// TestKeptDeliveryIsRefusedAndWritesNoFitRow pins the served layer's kept
// limitation: Handle stores captures, refunds and disputes only, so a kept
// outcome is refused as unknown and never reaches the fit recorder.
// product.md records why: a kept settle needs a return window's length and
// a decision that it has closed, and the layer models neither.
func TestKeptDeliveryIsRefusedAndWritesNoFitRow(t *testing.T) {
	fit := &recordingFit{}
	service := NewService(keptOnlySource{}, WithFitRecorder(fit))
	_, err := service.Handle(context.Background(), "WH-KEPT-01")
	if !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("kept delivery error = %v, want %v", err, ErrUnknownEvent)
	}
	if len(fit.events) != 0 {
		t.Fatalf("kept delivery wrote %d fit rows, want none", len(fit.events))
	}
}
