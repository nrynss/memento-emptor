package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/memory"
)

// The opposite-direction belief and return. Neither is in the frozen
// story, so the direction graph allowlists them beside the story's
// contents.
const (
	beliefRunsLarge = "runs large, take size S"
	returnTooLarge  = "Fit return 4: oxford shirt size M refunded; far too big"
)

// directionGraph builds the writer's graph over the story's contents plus
// the opposite-direction belief and return.
func directionGraph() adapter {
	allowed := append(append([]string{}, fakes.GraphContents()...), beliefRunsLarge, returnTooLarge)
	return adapter{inner: fakes.NewRecordingEvidenceGraph(allowed, fakes.StoryRecalls())}
}

// tooLargeEvidence builds one classified too-large return at the given
// event time.
func tooLargeEvidence(at time.Time) domain.Evidence {
	ev := fitReturn(at, "far too big")
	ev.Reason = &domain.Reason{Category: domain.CategoryFitTooLarge, Text: "far too big", Confidence: 1.0}
	return ev
}

// earnSmall replays the three documented small returns against the
// runs-small belief, so the mismatch test starts from an Earned belief.
func earnSmall(t *testing.T, ctx context.Context, g memory.Graph) {
	t.Helper()
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
		written, err := memory.WriteEvidence(ctx, g, fakes.BeliefRunsSmall, domain.BeliefKindFit, domain.FitDirectionSmall, ret.action, fitReturn(fakes.OccurredAt(ret.event), ret.text))
		if err != nil {
			t.Fatalf("write return %d: %v", i+1, err)
		}
		if !written {
			t.Fatalf("write return %d reported nothing written", i+1)
		}
	}
	reading := readStatus(t, ctx, g, fakes.BeliefRunsSmall)
	if reading.Status != domain.StatusEarned || reading.Dependents != 3 {
		t.Fatalf("seeded sizing reads status %q dependents %d, want Earned 3", reading.Status, reading.Dependents)
	}
}

// TestDirectionMismatchNeitherSupportsNorContradicts proves the pinned
// rule at the writer: a too-large return offered to an Earned runs-small
// belief is refused with ErrDirectionMismatch. It adds no support — the
// belief keeps its three dependents and its Venerable standing — and it
// declares nothing against the belief, so no contradiction warning
// appears. Contradictions stay shopper-driven.
func TestDirectionMismatchNeitherSupportsNorContradicts(t *testing.T) {
	ctx := context.Background()
	g := directionGraph()
	earnSmall(t, ctx, g)

	written, err := memory.WriteEvidence(ctx, g, fakes.BeliefRunsSmall, domain.BeliefKindFit, domain.FitDirectionSmall, returnTooLarge, tooLargeEvidence(fakes.OccurredAt("WH-2EV-05")))
	if !errors.Is(err, memory.ErrDirectionMismatch) {
		t.Fatalf("too-large return offered to runs-small: error = %v, want ErrDirectionMismatch", err)
	}
	if written {
		t.Error("mismatched return reported written true, want false")
	}
	reading := readStatus(t, ctx, g, fakes.BeliefRunsSmall)
	if reading.Status != domain.StatusEarned || reading.Standing != domain.GraphVenerable || reading.Dependents != 3 {
		t.Errorf("mismatched return moved sizing: status %q standing %q dependents %d, want Earned Venerable 3",
			reading.Status, reading.Standing, reading.Dependents)
	}
	if reading.Contradicted || reading.Warning != "" {
		t.Errorf("mismatched return contradicts sizing: contradicted %v warning %q, want false and empty",
			reading.Contradicted, reading.Warning)
	}
}

// TestDirectionMismatchBothWays proves the refusal is symmetric: a
// too-small return offered to a runs-large belief is refused the same
// way, with nothing written and no contradiction declared.
func TestDirectionMismatchBothWays(t *testing.T) {
	ctx := context.Background()
	g := directionGraph()
	at := fakes.OccurredAt("WH-2EV-01")
	if err := memory.EnsureBelief(ctx, g, beliefRunsLarge, at); err != nil {
		t.Fatalf("ensure runs-large belief: %v", err)
	}

	written, err := memory.WriteEvidence(ctx, g, beliefRunsLarge, domain.BeliefKindFit, domain.FitDirectionLarge, fakes.ReturnOne, fitReturn(at, "too tight"))
	if !errors.Is(err, memory.ErrDirectionMismatch) {
		t.Fatalf("too-small return offered to runs-large: error = %v, want ErrDirectionMismatch", err)
	}
	if written {
		t.Error("mismatched return reported written true, want false")
	}
	reading := readStatus(t, ctx, g, beliefRunsLarge)
	if reading.Dependents != 0 {
		t.Errorf("refused write left %d dependents, want 0", reading.Dependents)
	}
	if reading.Contradicted || reading.Warning != "" {
		t.Errorf("mismatched return contradicts the belief: contradicted %v warning %q, want false and empty",
			reading.Contradicted, reading.Warning)
	}
}

// TestDirectionMatchAttaches proves the other half of the rule: a
// too-large return offered to a runs-large belief counts toward it.
func TestDirectionMatchAttaches(t *testing.T) {
	ctx := context.Background()
	g := directionGraph()
	at := fakes.OccurredAt("WH-2EV-01")
	if err := memory.EnsureBelief(ctx, g, beliefRunsLarge, at); err != nil {
		t.Fatalf("ensure runs-large belief: %v", err)
	}

	written, err := memory.WriteEvidence(ctx, g, beliefRunsLarge, domain.BeliefKindFit, domain.FitDirectionLarge, returnTooLarge, tooLargeEvidence(at))
	if err != nil {
		t.Fatalf("matching too-large return returned error: %v", err)
	}
	if !written {
		t.Fatal("matching too-large return reported nothing written")
	}
	reading := readStatus(t, ctx, g, beliefRunsLarge)
	if reading.Dependents != 1 {
		t.Errorf("matching return left %d dependents, want 1", reading.Dependents)
	}
	if reading.Contradicted {
		t.Error("matching return contradicts the belief it supports")
	}
}
