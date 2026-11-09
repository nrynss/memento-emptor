package domain

import (
	"errors"
	"testing"
	"time"
)

func TestEvidenceWithoutReasonCountsTowardNothing(t *testing.T) {
	e := Evidence{
		Source: SourcePayment,
		Outcome: Outcome{
			Kind:        OutcomeRefund,
			Status:      "COMPLETED",
			AmountMinor: 3400,
			Currency:    "USD",
		},
		Item: Item{
			BrandID:   "SMwk",
			BrandName: "Brand X",
			LineName:  "Button Down Oxford Shirt",
			SKU:       "BRX-OXF-M",
			Size:      "M",
			Category:  "dress-shirts",
		},
		Reason:     nil,
		OccurredAt: time.Now().UTC(),
	}
	if got := e.CountsToward(); got != BeliefKindNone {
		t.Errorf("refund without reason counts toward %q, want nothing", got)
	}
}

func TestEvidenceWithReasonCountsTowardItsKind(t *testing.T) {
	fit := Evidence{
		Source:  SourceShopper,
		Outcome: Outcome{Kind: OutcomeRefund, Status: "COMPLETED"},
		Reason:  &Reason{Category: CategoryFitTooSmall, Text: "too tight in the shoulders", Confidence: 1},
	}
	if got := fit.CountsToward(); got != BeliefKindFit {
		t.Errorf("fit return counts toward %q, want fit", got)
	}
	seller := Evidence{
		Source:  SourcePayment,
		Outcome: Outcome{Kind: OutcomeDispute, Status: "OPEN"},
		Reason:  &Reason{Category: CategoryNotArrived, Text: string(DisputeMerchandiseNotReceived)},
	}
	if got := seller.CountsToward(); got != BeliefKindSeller {
		t.Errorf("non-arrival dispute counts toward %q, want seller reliability", got)
	}
	changed := Evidence{
		Source:  SourceShopper,
		Outcome: Outcome{Kind: OutcomeRefund, Status: "COMPLETED"},
		Reason:  &Reason{Category: CategoryChangedMind, Text: "changed my mind", Confidence: 1},
	}
	if got := changed.CountsToward(); got != BeliefKindNone {
		t.Errorf("changed-mind refund counts toward %q, want nothing", got)
	}
}

func TestSeedMarkIsSet(t *testing.T) {
	if SeedMark == "" {
		t.Fatal("SeedMark is empty")
	}
	e := Evidence{Seeded: true}
	if !e.Seeded {
		t.Fatal("seeded evidence lost its mark")
	}
}

// stubContract is a test-only EmbedderContract with no model constants.
type stubContract struct {
	kind string
}

func (s stubContract) EmbeddingKind() string { return s.kind }

func (s stubContract) Validate() error {
	if s.kind == "" {
		return ErrEmptyEmbedderKind
	}
	return nil
}

func TestEmbedderContractIsSwappable(t *testing.T) {
	var first EmbedderContract = stubContract{kind: "first-backend"}
	var second EmbedderContract = stubContract{kind: "second-backend"}
	if first.EmbeddingKind() == second.EmbeddingKind() {
		t.Fatal("two backends share one kind")
	}
	for _, c := range []EmbedderContract{first, second} {
		if err := c.Validate(); err != nil {
			t.Errorf("contract %q failed validation: %v", c.EmbeddingKind(), err)
		}
	}
}

func TestEmbedderContractRejectsEmptyKind(t *testing.T) {
	var c EmbedderContract = stubContract{}
	if err := c.Validate(); !errors.Is(err, ErrEmptyEmbedderKind) {
		t.Fatalf("empty contract error = %v, want ErrEmptyEmbedderKind", err)
	}
}
