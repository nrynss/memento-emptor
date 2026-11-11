package reasons

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// TestSettleAccepted proves an accepted classification becomes a reason
// carrying the shopper's words, and the reason counts toward the belief
// its category matches.
func TestSettleAccepted(t *testing.T) {
	assertOffline(t)
	client := NewClient(DefaultModel, DefaultEndpoint, fixtureDo(t, nil))
	ctx := context.Background()
	cases := []struct {
		answer string
		toward domain.BeliefKind
	}{
		{"too tight in the shoulders", domain.BeliefKindFit},
		{"it swims on me, far too big", domain.BeliefKindFit},
		{"arrived broken", domain.BeliefKindSeller},
		{"it never arrived, tracking has been stuck for weeks", domain.BeliefKindSeller},
		{"changed my mind", domain.BeliefKindNone},
	}
	for _, tc := range cases {
		class, err := client.Classify(ctx, tc.answer)
		if err != nil {
			t.Fatal(err)
		}
		reason, askAgain := Settle(tc.answer, class)
		if askAgain || reason == nil {
			t.Errorf("clear answer %q stays pending", tc.answer)
			continue
		}
		if reason.Text != tc.answer || reason.Category != class.Category || reason.Confidence != class.Confidence {
			t.Errorf("settled reason = %+v, want the classified answer", reason)
		}
		evidence := domain.Evidence{Reason: reason}
		if got := evidence.CountsToward(); got != tc.toward {
			t.Errorf("answer %q counts toward %v, want %v", tc.answer, got, tc.toward)
		}
	}
}

// TestSettleAskAgain proves a below-floor answer leaves a nil reason, so
// the refund stays a pending question and counts toward nothing.
func TestSettleAskAgain(t *testing.T) {
	assertOffline(t)
	client := NewClient(DefaultModel, DefaultEndpoint, fixtureDo(t, nil))
	ctx := context.Background()
	for _, answer := range []string{"not great", "it wasn't what I wanted", "meh, sending it back"} {
		class, err := client.Classify(ctx, answer)
		if err != nil {
			t.Fatal(err)
		}
		reason, askAgain := Settle(answer, class)
		if !askAgain || reason != nil {
			t.Errorf("vague answer %q settled to %+v, want pending", answer, reason)
			continue
		}
		if got := (domain.Evidence{}).CountsToward(); got != domain.BeliefKindNone {
			t.Errorf("pending refund counts toward %v, want nothing", got)
		}
	}
}

// TestNeverExplainedCountsTowardNothing pins the product rule: a refund
// the shopper never explains is an outcome with no reason, and no code
// path may turn it into a belief.
func TestNeverExplainedCountsTowardNothing(t *testing.T) {
	evidence := domain.Evidence{
		Source:  domain.SourcePayment,
		Outcome: domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED"},
		Reason:  nil,
	}
	if got := evidence.CountsToward(); got != domain.BeliefKindNone {
		t.Errorf("unexplained refund counts toward %v, want nothing", got)
	}
	fake, err := fakes.NewRecordingReasonClassifier(filepath.Join(fixtureRoot(t), "jev"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(fake.RecordedAnswers()); got != 13 {
		t.Fatalf("labelled set covers %d answers, want 13", got)
	}
}
