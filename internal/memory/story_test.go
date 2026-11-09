package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/memory"
)

// adapter implements memory.Graph over the recording fake so the story
// replays the researched promotion bars without a live graph.
type adapter struct {
	inner *fakes.RecordingEvidenceGraph
}

func (a adapter) Derive(ctx context.Context, content, kind string, at time.Time) error {
	return a.inner.Derive(ctx, content, kind, at)
}

func (a adapter) RecordAction(ctx context.Context, action string, dependsOn, against []string, at time.Time) error {
	return a.inner.RecordAction(ctx, action, dependsOn, against, at)
}

func (a adapter) Inspect(ctx context.Context, content string) (memory.Insight, error) {
	report, err := a.inner.Inspect(ctx, content)
	if err != nil {
		return memory.Insight{}, err
	}
	return memory.Insight{
		State:                report.State,
		Contradicted:         report.Contradicted,
		Dependents:           report.Dependents,
		DistinctInteractions: report.DistinctInteractions,
		Coverage:             report.Coverage,
		Warning:              report.Warning,
	}, nil
}

// storyGraph builds the writer's graph over the committed story contents.
func storyGraph() adapter {
	return adapter{inner: fakes.NewRecordingEvidenceGraph(fakes.GraphContents(), fakes.StoryRecalls())}
}

// mustParse parses one RFC 3339 time or fails the test.
func mustParse(t *testing.T, raw string) time.Time {
	t.Helper()
	moment, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time %q: %v", raw, err)
	}
	return moment
}

// fitReturn builds one classified fit return at the story's event time.
func fitReturn(at time.Time, text string) domain.Evidence {
	return domain.Evidence{
		Source:     domain.SourcePayment,
		Outcome:    domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"},
		Item:       domain.Item{BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt", SKU: "tbnrl46", Size: "M", Category: "shirts"},
		Reason:     &domain.Reason{Category: domain.CategoryFitTooSmall, Text: text, Confidence: 1.0},
		Link:       domain.TransactionLink{OrderID: "ORD-7XK-01", CaptureID: "CAP-7XK-01", RefundID: "RFD-7XK-01", CustomID: "memento-shopper01-01", EventID: "WH-2EV-01"},
		Seeded:     true,
		OccurredAt: at,
	}
}

// readStatus reads one belief or fails the test.
func readStatus(t *testing.T, ctx context.Context, g memory.Graph, content string) memory.Reading {
	t.Helper()
	reading, err := memory.ReadBelief(ctx, g, content)
	if err != nil {
		t.Fatalf("read belief %q: %v", content, err)
	}
	return reading
}

// TestDemoStoryArc replays the demo's graph writes through the writer: the
// general statement never promotes, the third fit return earns the sizing
// belief, the correction contradicts it without deleting it, and the
// non-arrival dispute leaves sizing untouched while the seller belief
// stays unpromoted.
func TestDemoStoryArc(t *testing.T) {
	ctx := context.Background()
	g := storyGraph()
	seededAt := fakes.OccurredAt("WH-2EV-01")

	if err := memory.EnsureBelief(ctx, g, fakes.BeliefRunsSmall, seededAt); err != nil {
		t.Fatalf("ensure sizing belief: %v", err)
	}
	if err := memory.RecordStatement(ctx, g, fakes.BeliefUsuallyM, seededAt); err != nil {
		t.Fatalf("record general statement: %v", err)
	}

	returns := []struct {
		event  string
		action string
		text   string
	}{
		{"WH-2EV-01", fakes.ReturnOne, "too tight in the shoulders"},
		{"WH-2EV-02", fakes.ReturnTwo, "too tight across the chest"},
		{"WH-2EV-03", fakes.ReturnThree, "sleeves too short in M"},
	}
	want := []struct {
		state      domain.GraphState
		status     domain.BeliefStatus
		dependents int
	}{
		{domain.GraphNone, domain.StatusClaimed, 1},
		{domain.GraphCandidate, domain.StatusBuilding, 2},
		{domain.GraphVenerable, domain.StatusEarned, 3},
	}
	for i, ret := range returns {
		written, err := memory.WriteEvidence(ctx, g, fakes.BeliefRunsSmall, domain.BeliefKindFit, ret.action, fitReturn(fakes.OccurredAt(ret.event), ret.text))
		if err != nil {
			t.Fatalf("write return %d: %v", i+1, err)
		}
		if !written {
			t.Fatalf("write return %d reported nothing written", i+1)
		}
		reading := readStatus(t, ctx, g, fakes.BeliefRunsSmall)
		if reading.Standing != want[i].state || reading.Status != want[i].status || reading.Dependents != want[i].dependents {
			t.Errorf("after return %d: standing %q status %q dependents %d, want %q %q %d",
				i+1, reading.Standing, reading.Status, reading.Dependents,
				want[i].state, want[i].status, want[i].dependents)
		}
		if reading.Contradicted {
			t.Errorf("after return %d: contradicted, want false", i+1)
		}
	}

	statement := readStatus(t, ctx, g, fakes.BeliefUsuallyM)
	if statement.Status != domain.StatusClaimed || statement.Standing != domain.GraphNone || statement.Dependents != 0 {
		t.Errorf("general statement reads status %q standing %q dependents %d, want Claimed None 0",
			statement.Status, statement.Standing, statement.Dependents)
	}

	correctionAt := mustParse(t, "2026-11-05T14:02:11Z")
	if err := memory.RecordCorrection(ctx, g, fakes.CorrectionLostWeight, fakes.BeliefRunsSmall, correctionAt); err != nil {
		t.Fatalf("record correction: %v", err)
	}
	contradicted := readStatus(t, ctx, g, fakes.BeliefRunsSmall)
	if contradicted.Status != domain.StatusOutOfTrue {
		t.Errorf("after correction: status %q, want Out of true", contradicted.Status)
	}
	if contradicted.Standing != domain.GraphVenerable {
		t.Errorf("after correction: standing %q, want Venerable: the belief stays", contradicted.Standing)
	}
	if !contradicted.Contradicted || contradicted.Warning == "" {
		t.Errorf("after correction: contradicted %v warning %q, want true and non-empty", contradicted.Contradicted, contradicted.Warning)
	}
	if contradicted.Dependents != 3 {
		t.Errorf("after correction: dependents %d, want 3: against adds no evidence", contradicted.Dependents)
	}

	before := contradicted
	sellerAt := mustParse(t, "2026-11-06T11:30:00Z")
	if err := memory.EnsureBelief(ctx, g, fakes.BeliefShipsReliably, sellerAt); err != nil {
		t.Fatalf("ensure seller belief: %v", err)
	}
	dispute := domain.Evidence{
		Source:     domain.SourcePayment,
		Outcome:    domain.Outcome{Kind: domain.OutcomeDispute, Status: "OPEN", AmountMinor: 8900, Currency: "USD"},
		Item:       domain.Item{BrandID: "brand-x-001", BrandName: "Brand X", LineName: "chino", SKU: "chino-khaki-32", Size: "32", Category: "pants"},
		Reason:     &domain.Reason{Category: domain.CategoryNotArrived, Text: "MERCHANDISE_OR_SERVICE_NOT_RECEIVED", Confidence: 1.0},
		Link:       domain.TransactionLink{OrderID: "ORD-7XK-04", CaptureID: "CAP-7XK-04", DisputeID: "PP-D-51713", CustomID: "memento-shopper01-04", EventID: "WH-2EV-04"},
		OccurredAt: sellerAt,
	}
	written, err := memory.WriteEvidence(ctx, g, fakes.BeliefShipsReliably, domain.BeliefKindSeller, fakes.DisputeNeverArrived, dispute)
	if err != nil {
		t.Fatalf("write dispute: %v", err)
	}
	if !written {
		t.Fatal("write dispute reported nothing written")
	}
	after := readStatus(t, ctx, g, fakes.BeliefRunsSmall)
	if after != before {
		t.Errorf("dispute moved sizing: %+v, want unchanged %+v", after, before)
	}
	seller := readStatus(t, ctx, g, fakes.BeliefShipsReliably)
	if seller.Status != domain.StatusClaimed || seller.Standing != domain.GraphNone || seller.Dependents != 1 {
		t.Errorf("seller belief reads status %q standing %q dependents %d, want Claimed None 1: one dispute never earns",
			seller.Status, seller.Standing, seller.Dependents)
	}

	captureAt := fakes.OccurredAt("WH-2EV-06")
	if err := memory.EnsureBelief(ctx, g, fakes.BeliefSizeMNow, captureAt); err != nil {
		t.Fatalf("ensure rival belief: %v", err)
	}
	written, err = memory.WriteFeedback(ctx, g, fakes.BeliefSizeMNow, domain.BeliefKindFit, fakes.FeedbackFitsWell, contract.FeedbackFitsWell, captureAt)
	if err != nil {
		t.Fatalf("write feedback: %v", err)
	}
	if !written {
		t.Fatal("write feedback reported nothing written")
	}
	rival := readStatus(t, ctx, g, fakes.BeliefSizeMNow)
	if rival.Status != domain.StatusClaimed || rival.Dependents != 1 {
		t.Errorf("rival belief reads status %q dependents %d, want Claimed 1: one data point never earns",
			rival.Status, rival.Dependents)
	}
	final := readStatus(t, ctx, g, fakes.BeliefRunsSmall)
	if final != before {
		t.Errorf("feedback moved sizing: %+v, want unchanged %+v", final, before)
	}
}
