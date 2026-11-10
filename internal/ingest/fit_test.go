package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// recordingFit is a FitRecorder that keeps every row it was handed.
type recordingFit struct {
	events []domain.Evidence
	err    error
}

// Record keeps one row, or fails with the staged error.
func (r *recordingFit) Record(ctx context.Context, ev domain.Evidence) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, ev)
	return nil
}

// TestFitRowWrittenFromTheSameEvent drives the story's capture and one of
// its refunds through a service with a fit recorder. Each delivery hands
// the recorder the row it stored, so the fit row comes from the same event,
// and a refund's row still carries no reason.
func TestFitRowWrittenFromTheSameEvent(t *testing.T) {
	fit := &recordingFit{}
	service := NewService(fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()), WithFitRecorder(fit))
	ctx := context.Background()

	if _, err := service.Handle(ctx, "WH-2EV-06"); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(fit.events) != 1 {
		t.Fatalf("capture recorded %d fit rows, want 1", len(fit.events))
	}
	capture := fit.events[0]
	if capture.Outcome.Kind != domain.OutcomeCapture {
		t.Errorf("capture fit row outcome = %q, want capture", capture.Outcome.Kind)
	}
	if capture.Link.EventID != "WH-2EV-06" {
		t.Errorf("capture fit row event = %q, want the delivery that wrote it", capture.Link.EventID)
	}
	if capture.Item.SKU != "PSW8GeK" {
		t.Errorf("capture fit row item = %q, want the captured SKU", capture.Item.SKU)
	}
	if capture.Reason != nil {
		t.Error("capture fit row carries a reason; a capture proves only that the purchase happened")
	}

	if _, err := service.Handle(ctx, "WH-2EV-01"); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if len(fit.events) != 2 {
		t.Fatalf("refund recorded %d fit rows, want 2", len(fit.events))
	}
	refund := fit.events[1]
	if refund.Outcome.Kind != domain.OutcomeRefund || refund.Reason != nil {
		t.Errorf("refund fit row outcome/reason = %q/%v, want a reasonless refund", refund.Outcome.Kind, refund.Reason)
	}
	if !refund.OccurredAt.Equal(fakes.OccurredAt("WH-2EV-01")) {
		t.Errorf("refund fit row time = %s, want the delivery's event time", refund.OccurredAt)
	}
}

// TestDisputeWritesNoFitRow files the story's non-arrival dispute. It is
// about the seller, not the purchase's fit, so no fit row is written.
func TestDisputeWritesNoFitRow(t *testing.T) {
	fit := &recordingFit{}
	service := NewService(fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()), WithFitRecorder(fit))
	if _, err := service.Handle(context.Background(), "WH-2EV-04"); err != nil {
		t.Fatalf("dispute: %v", err)
	}
	if len(fit.events) != 0 {
		t.Errorf("dispute recorded %d fit rows, want 0", len(fit.events))
	}
}

// TestRedeliveredWebhookWritesOneFitRow delivers the same refund twice.
// The duplicate writes no second fit row, and the row the first delivery
// wrote is the one that stays.
func TestRedeliveredWebhookWritesOneFitRow(t *testing.T) {
	fit := &recordingFit{}
	service := NewService(fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()), WithFitRecorder(fit))
	ctx := context.Background()

	if _, err := service.Handle(ctx, "WH-2EV-01"); err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	second, err := service.Handle(ctx, "WH-2EV-01")
	if err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if !second.Duplicate {
		t.Error("redelivery does not report Duplicate")
	}
	if len(fit.events) != 1 {
		t.Errorf("redelivery recorded %d fit rows, want the 1 the first delivery wrote", len(fit.events))
	}
}

// TestFitRecorderFailureFailsTheDelivery stages a fit write that fails. The
// delivery fails and stores nothing, so the payment source's retry writes
// the evidence and the fit row together.
func TestFitRecorderFailureFailsTheDelivery(t *testing.T) {
	fit := &recordingFit{err: errors.New("fit index unavailable")}
	service := NewService(fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()), WithFitRecorder(fit))
	ctx := context.Background()

	if _, err := service.Handle(ctx, "WH-2EV-01"); err == nil {
		t.Fatal("delivery with a failing fit write succeeded")
	}
	if len(service.Rows()) != 0 || len(service.PendingQuestions()) != 0 {
		t.Errorf("failed delivery stored %d rows and %d questions, want 0 and 0",
			len(service.Rows()), len(service.PendingQuestions()))
	}

	fit.err = nil
	result, err := service.Handle(ctx, "WH-2EV-01")
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if result.Duplicate || len(result.Rows) != 1 || result.Pending == nil {
		t.Errorf("retry stored duplicate=%v rows=%d pending=%v, want a fresh refund and its question",
			result.Duplicate, len(result.Rows), result.Pending != nil)
	}
	if len(fit.events) != 1 {
		t.Errorf("retry recorded %d fit rows, want 1", len(fit.events))
	}
}
