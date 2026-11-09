package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/memory"
)

// reasonlessRefund builds a refund outcome with no reason: the payment
// side with no cause. It must count toward nothing.
func reasonlessRefund(at time.Time) domain.Evidence {
	return domain.Evidence{
		Source:     domain.SourcePayment,
		Outcome:    domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"},
		Item:       domain.Item{BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt", SKU: "tbnrl46", Size: "M", Category: "shirts"},
		Reason:     nil,
		Link:       domain.TransactionLink{OrderID: "ORD-7XK-05", CaptureID: "CAP-7XK-05", RefundID: "RFD-7XK-05", CustomID: "memento-shopper01-05", EventID: "WH-2EV-05"},
		OccurredAt: at,
	}
}

// TestReasonlessRefundWritesNothing is the developer-rule proof: a refund
// with no reason writes nothing to any belief. The graph is seeded with
// the earned sizing belief first, so the test would catch even a stray
// structural edge.
func TestReasonlessRefundWritesNothing(t *testing.T) {
	ctx := context.Background()
	g := storyGraph()
	seededAt := fakes.OccurredAt("WH-2EV-01")

	if err := memory.EnsureBelief(ctx, g, fakes.BeliefRunsSmall, seededAt); err != nil {
		t.Fatalf("ensure sizing belief: %v", err)
	}
	for i, ret := range []struct {
		event  string
		action string
		text   string
	}{
		{"WH-2EV-01", fakes.ReturnOne, "too tight in the shoulders"},
		{"WH-2EV-02", fakes.ReturnTwo, "too tight across the chest"},
		{"WH-2EV-03", fakes.ReturnThree, "sleeves too short in M"},
	} {
		if _, err := memory.WriteEvidence(ctx, g, fakes.BeliefRunsSmall, domain.BeliefKindFit, ret.action, fitReturn(fakes.OccurredAt(ret.event), ret.text)); err != nil {
			t.Fatalf("seed return %d: %v", i+1, err)
		}
	}
	before := readStatus(t, ctx, g, fakes.BeliefRunsSmall)
	if before.Status != domain.StatusEarned || before.Dependents != 3 {
		t.Fatalf("seeded sizing reads status %q dependents %d, want Earned 3", before.Status, before.Dependents)
	}

	written, err := memory.WriteEvidence(ctx, g, fakes.BeliefRunsSmall, domain.BeliefKindFit, fakes.DisputeNeverArrived, reasonlessRefund(fakes.OccurredAt("WH-2EV-05")))
	if err != nil {
		t.Fatalf("reasonless refund returned error: %v", err)
	}
	if written {
		t.Error("reasonless refund reported written true, want false")
	}
	after := readStatus(t, ctx, g, fakes.BeliefRunsSmall)
	if after != before {
		t.Errorf("reasonless refund moved sizing: %+v, want unchanged %+v", after, before)
	}
}

// TestUnmatchedReasonsWriteNothing covers the other reasonless shapes: a
// changed mind and an unclear answer count toward nothing and write
// nothing, without error.
func TestUnmatchedReasonsWriteNothing(t *testing.T) {
	ctx := context.Background()
	for _, category := range []domain.Category{domain.CategoryChangedMind, domain.CategoryUnclear, domain.CategoryNone} {
		g := storyGraph()
		at := fakes.OccurredAt("WH-2EV-01")
		if err := memory.EnsureBelief(ctx, g, fakes.BeliefRunsSmall, at); err != nil {
			t.Fatalf("ensure belief: %v", err)
		}
		ev := fitReturn(at, "shoppers words")
		ev.Reason = &domain.Reason{Category: category, Text: "shoppers words", Confidence: 1.0}
		written, err := memory.WriteEvidence(ctx, g, fakes.BeliefRunsSmall, domain.BeliefKindFit, fakes.ReturnOne, ev)
		if err != nil {
			t.Errorf("category %q returned error: %v", category, err)
			continue
		}
		if written {
			t.Errorf("category %q reported written true, want false", category)
		}
		reading := readStatus(t, ctx, g, fakes.BeliefRunsSmall)
		if reading.Dependents != 0 {
			t.Errorf("category %q left %d dependents, want 0", category, reading.Dependents)
		}
	}
}

// TestKindMismatchRefuses proves the writer routes evidence only to the
// belief its reason matches: a non-arrival dispute offered to the sizing
// belief, or fit evidence offered to the seller belief, fails and writes
// nothing.
func TestKindMismatchRefuses(t *testing.T) {
	ctx := context.Background()
	g := storyGraph()
	at := fakes.OccurredAt("WH-2EV-01")
	if err := memory.EnsureBelief(ctx, g, fakes.BeliefRunsSmall, at); err != nil {
		t.Fatalf("ensure sizing belief: %v", err)
	}
	if err := memory.EnsureBelief(ctx, g, fakes.BeliefShipsReliably, at); err != nil {
		t.Fatalf("ensure seller belief: %v", err)
	}

	dispute := fitReturn(at, "never arrived")
	dispute.Reason = &domain.Reason{Category: domain.CategoryNotArrived, Text: "MERCHANDISE_OR_SERVICE_NOT_RECEIVED", Confidence: 1.0}
	if _, err := memory.WriteEvidence(ctx, g, fakes.BeliefRunsSmall, domain.BeliefKindFit, fakes.DisputeNeverArrived, dispute); !errors.Is(err, memory.ErrKindMismatch) {
		t.Errorf("dispute offered to sizing: error = %v, want ErrKindMismatch", err)
	}
	if _, err := memory.WriteEvidence(ctx, g, fakes.BeliefShipsReliably, domain.BeliefKindSeller, fakes.ReturnOne, fitReturn(at, "too tight")); !errors.Is(err, memory.ErrKindMismatch) {
		t.Errorf("fit return offered to seller: error = %v, want ErrKindMismatch", err)
	}
	if _, err := memory.WriteFeedback(ctx, g, fakes.BeliefShipsReliably, domain.BeliefKindSeller, fakes.FeedbackFitsWell, contract.FeedbackFitsWell, at); !errors.Is(err, memory.ErrKindMismatch) {
		t.Errorf("fits-well feedback offered to seller: error = %v, want ErrKindMismatch", err)
	}
	for _, content := range []string{fakes.BeliefRunsSmall, fakes.BeliefShipsReliably} {
		reading := readStatus(t, ctx, g, content)
		if reading.Dependents != 0 {
			t.Errorf("refused write left %d dependents on %q, want 0", reading.Dependents, content)
		}
	}
}

// TestVerdictCountsToward checks every frozen feedback verdict maps to
// its belief kind, and nothing else does.
func TestVerdictCountsToward(t *testing.T) {
	cases := []struct {
		verdict contract.FeedbackVerdict
		want    domain.BeliefKind
	}{
		{contract.FeedbackFitsWell, domain.BeliefKindFit},
		{contract.FeedbackTooSmall, domain.BeliefKindFit},
		{contract.FeedbackTooLarge, domain.BeliefKindFit},
		{contract.FeedbackDamagedOrDefective, domain.BeliefKindSeller},
		{contract.FeedbackNotArrived, domain.BeliefKindSeller},
		{contract.FeedbackChangedMind, domain.BeliefKindNone},
		{contract.FeedbackUnclear, domain.BeliefKindNone},
	}
	for _, tc := range cases {
		got, err := memory.VerdictCountsToward(tc.verdict)
		if err != nil {
			t.Errorf("VerdictCountsToward(%q) returned error: %v", tc.verdict, err)
			continue
		}
		if got != tc.want {
			t.Errorf("VerdictCountsToward(%q) = %q, want %q", tc.verdict, got, tc.want)
		}
	}
	if _, err := memory.VerdictCountsToward("fits_poorly"); !errors.Is(err, memory.ErrUnknownVerdict) {
		t.Errorf("unknown verdict error = %v, want ErrUnknownVerdict", err)
	}
}

// TestUnclearFeedbackWritesNothing proves an unclear verdict never becomes
// evidence.
func TestUnclearFeedbackWritesNothing(t *testing.T) {
	ctx := context.Background()
	g := storyGraph()
	at := fakes.OccurredAt("WH-2EV-06")
	if err := memory.EnsureBelief(ctx, g, fakes.BeliefSizeMNow, at); err != nil {
		t.Fatalf("ensure rival belief: %v", err)
	}
	written, err := memory.WriteFeedback(ctx, g, fakes.BeliefSizeMNow, domain.BeliefKindFit, fakes.FeedbackFitsWell, contract.FeedbackUnclear, at)
	if err != nil {
		t.Fatalf("unclear feedback returned error: %v", err)
	}
	if written {
		t.Error("unclear feedback reported written true, want false")
	}
	reading := readStatus(t, ctx, g, fakes.BeliefSizeMNow)
	if reading.Dependents != 0 {
		t.Errorf("unclear feedback left %d dependents, want 0", reading.Dependents)
	}
}

// TestStatementsAndKeptNeverPromote proves assertions and weak
// observations never become structural evidence, however often repeated.
func TestStatementsAndKeptNeverPromote(t *testing.T) {
	ctx := context.Background()
	g := storyGraph()
	at := fakes.OccurredAt("WH-2EV-01")
	for i := 0; i < 4; i++ {
		if err := memory.RecordStatement(ctx, g, fakes.BeliefUsuallyM, at); err != nil {
			t.Fatalf("record statement %d: %v", i+1, err)
		}
		// The kept observation derives as a note under a covered
		// content: the kind decides promotion, never the words.
		if err := memory.RecordKept(ctx, g, fakes.BeliefShipsReliably, at); err != nil {
			t.Fatalf("record kept %d: %v", i+1, err)
		}
	}
	for _, content := range []string{fakes.BeliefUsuallyM, fakes.BeliefShipsReliably} {
		reading := readStatus(t, ctx, g, content)
		if reading.Status != domain.StatusClaimed || reading.Standing != domain.GraphNone || reading.Dependents != 0 {
			t.Errorf("%q reads status %q standing %q dependents %d, want Claimed None 0",
				content, reading.Status, reading.Standing, reading.Dependents)
		}
	}
}

// TestLedgerStatusMapping checks the four ledger statuses read off the
// graph state plus the contradiction overlay, with the standing kept
// beside the warning.
func TestLedgerStatusMapping(t *testing.T) {
	cases := []struct {
		state        domain.GraphState
		contradicted bool
		want         domain.BeliefStatus
	}{
		{domain.GraphNone, false, domain.StatusClaimed},
		{domain.GraphCandidate, false, domain.StatusBuilding},
		{domain.GraphVenerable, false, domain.StatusEarned},
		{domain.GraphNone, true, domain.StatusOutOfTrue},
		{domain.GraphCandidate, true, domain.StatusOutOfTrue},
		{domain.GraphVenerable, true, domain.StatusOutOfTrue},
	}
	for _, tc := range cases {
		got, err := memory.LedgerStatus(tc.state, tc.contradicted)
		if err != nil {
			t.Errorf("LedgerStatus(%q, %v) returned error: %v", tc.state, tc.contradicted, err)
			continue
		}
		if got != tc.want {
			t.Errorf("LedgerStatus(%q, %v) = %q, want %q", tc.state, tc.contradicted, got, tc.want)
		}
	}
	if _, err := memory.LedgerStatus("Canonical", false); !errors.Is(err, domain.ErrUnknownGraphState) {
		t.Errorf("unknown state error = %v, want ErrUnknownGraphState", err)
	}
}
