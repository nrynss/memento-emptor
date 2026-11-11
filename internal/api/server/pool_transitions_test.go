package server

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// TestRecordReasonCannotReanswerAClosedQuestion pins the served layer's
// contested limitation: a clear answer closes its question and removes it
// from the pending set, so record_reason has no way to contest the reason
// later. The fit row keeps the first answer, and no second Classified or
// Contested write is reachable from a served call.
func TestRecordReasonCannotReanswerAClosedQuestion(t *testing.T) {
	ctx := context.Background()
	shopper := "shopper_01"
	fitIdx := &fakeFitIndex{}
	svc, _ := pivotHarness(t, nil, fitIdx, "")
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-01"); err != nil {
		t.Fatalf("ingest WH-2EV-01: %v", err)
	}
	question := questionForEvent(t, svc, shopper, "WH-2EV-01")
	first, err := svc.RecordReason(ctx, contract.RecordReasonRequest{
		ShopperID:  shopper,
		QuestionID: question,
		AnswerText: "too tight in the shoulders",
	})
	if err != nil {
		t.Fatalf("first answer: %v", err)
	}
	if !first.Accepted || first.Category != domain.CategoryFitTooSmall {
		t.Fatalf("first answer = accepted %v category %q, want an accepted fit_too_small",
			first.Accepted, first.Category)
	}
	rows := fitRowsOf(fitIdx)
	if len(rows) != 1 {
		t.Fatalf("fit index holds %d rows, want 1", len(rows))
	}
	for _, row := range rows {
		if row.State != domain.FitStateReasonClassified || !row.Settled ||
			row.Reason != domain.CategoryFitTooSmall {
			t.Fatalf("row after first answer = state %q settled %v reason %q, want settled fit_too_small",
				row.State, row.Settled, row.Reason)
		}
	}
	before := fitRowsOf(fitIdx)

	_, err = svc.RecordReason(ctx, contract.RecordReasonRequest{
		ShopperID:  shopper,
		QuestionID: question,
		AnswerText: "it swims on me, far too big",
	})
	if !errors.Is(err, ErrUnknownQuestion) {
		t.Fatalf("second answer = %v, want %v: a closed question is no longer pending",
			err, ErrUnknownQuestion)
	}
	if after := fitRowsOf(fitIdx); !reflect.DeepEqual(after, before) {
		t.Fatalf("fit rows after refused second answer = %+v, want the first answer unchanged: %+v",
			after, before)
	}
	pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("pending questions: %v", err)
	}
	if len(pending.Questions) != 0 {
		t.Fatalf("pending holds %d questions, want 0: the first answer closed it", len(pending.Questions))
	}
}

// TestRestartRedeliveryReopensTheRefundQuestion pins a limitation, not a
// wanted behaviour: the layer does not persist the event ids that make a
// redelivery a duplicate, so a restart treats the same refund as new,
// files its question again under a new id, and a second answer with a
// different cause settles the row again through the classified path. The
// fit writer's contested state is still not reached. product.md "How a fit
// row is written" records this; a future fix persists the delivered event
// ids, or refuses a redelivery whose event rows the partition already
// holds.
func TestRestartRedeliveryReopensTheRefundQuestion(t *testing.T) {
	ctx := context.Background()
	shopper := "shopper_01"
	fitIdx := &fakeFitIndex{}
	first, _ := pivotHarness(t, nil, fitIdx, "")
	if _, err := first.IngestWebhook(ctx, shopper, "WH-2EV-01"); err != nil {
		t.Fatalf("ingest WH-2EV-01: %v", err)
	}
	question := questionForEvent(t, first, shopper, "WH-2EV-01")
	if _, err := first.RecordReason(ctx, contract.RecordReasonRequest{
		ShopperID:  shopper,
		QuestionID: question,
		AnswerText: "too tight in the shoulders",
	}); err != nil {
		t.Fatalf("first answer: %v", err)
	}
	before := fitRowsOf(fitIdx)
	if len(before) != 1 {
		t.Fatalf("fit index holds %d rows before the restart, want 1", len(before))
	}
	for _, row := range before {
		if row.Reason != domain.CategoryFitTooSmall || !row.Settled {
			t.Fatalf("row before the restart = state %q reason %q settled %v, want a settled fit_too_small",
				row.State, row.Reason, row.Settled)
		}
	}

	// A new service over the same dependency is the restart: the partition
	// is new, while the payment source, the hub and the pool survive.
	restarted, err := NewService(first.deps)
	if err != nil {
		t.Fatalf("restarted service: %v", err)
	}
	res, err := restarted.IngestWebhook(ctx, shopper, "WH-2EV-01")
	if err != nil {
		t.Fatalf("redeliver WH-2EV-01 after restart: %v", err)
	}
	if !res.Pending {
		t.Fatalf("redelivery after restart filed no question, want the refund re-filed")
	}
	pending, err := restarted.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("pending questions: %v", err)
	}
	if len(pending.Questions) != 1 {
		t.Fatalf("pending holds %d questions after the restart, want 1", len(pending.Questions))
	}
	second, err := restarted.RecordReason(ctx, contract.RecordReasonRequest{
		ShopperID:  shopper,
		QuestionID: pending.Questions[0].QuestionID,
		AnswerText: "it swims on me, far too big",
	})
	if err != nil {
		t.Fatalf("second answer: %v", err)
	}
	if !second.Accepted || second.Category != domain.CategoryFitTooLarge {
		t.Fatalf("second answer = accepted %v category %q, want an accepted fit_too_large",
			second.Accepted, second.Category)
	}
	after := fitRowsOf(fitIdx)
	if len(after) != 1 {
		t.Fatalf("fit index holds %d rows after the second answer, want 1", len(after))
	}
	for _, row := range after {
		if row.State != domain.FitStateReasonClassified || row.Reason != domain.CategoryFitTooLarge || !row.Settled {
			t.Fatalf("row after the second answer = state %q reason %q settled %v, want a settled fit_too_large",
				row.State, row.Reason, row.Settled)
		}
		if row.ReasonContested {
			t.Fatalf("fit writer's contested state was reached, want only the classified path")
		}
	}
}
