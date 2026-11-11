package server

import (
	"context"
	"errors"
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

	_, err = svc.RecordReason(ctx, contract.RecordReasonRequest{
		ShopperID:  shopper,
		QuestionID: question,
		AnswerText: "it swims on me, far too big",
	})
	if !errors.Is(err, ErrUnknownQuestion) {
		t.Fatalf("second answer = %v, want %v: a closed question is no longer pending",
			err, ErrUnknownQuestion)
	}
	for _, row := range fitRowsOf(fitIdx) {
		if row.State != domain.FitStateReasonClassified || row.Reason != domain.CategoryFitTooSmall {
			t.Fatalf("row after refused second answer = state %q reason %q, want the first answer unchanged",
				row.State, row.Reason)
		}
	}
	pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("pending questions: %v", err)
	}
	if len(pending.Questions) != 0 {
		t.Fatalf("pending holds %d questions, want 0: the first answer closed it", len(pending.Questions))
	}
}
