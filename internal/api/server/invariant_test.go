package server

import (
	"context"
	"errors"
	"testing"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// guidanceStep names the step that overrode the assembler's test belief.
var guidanceStep = contract.GuidanceCorrection

// contradictedView builds a contradicted belief view for the assembler.
func contradictedView() contract.BeliefView {
	correction := contract.CorrectionView{
		CorrectionID:        "corr_1",
		Text:                "I've lost weight, use M",
		ContradictsBeliefID: "blf_1",
		RecordedAt:          "2026-11-05T14:02:11Z",
		TakesPrecedence:     true,
	}
	return contract.BeliefView{
		BeliefID:       "blf_1",
		Kind:           domain.BeliefKindFit,
		Subject:        domain.Subject{Kind: domain.SubjectBrand, ID: "brand-x-001", Name: "Brand X"},
		Value:          "runs small, take size L",
		Status:         domain.StatusOutOfTrue,
		Standing:       domain.GraphVenerable,
		Contradicted:   true,
		ContradictedBy: &correction,
		Evidence:       []contract.EvidenceView{},
		Precedence:     &contract.PrecedenceView{Step: contract.GuidanceOwnEvidence, Overridden: true, OverriddenBy: &guidanceStep},
		UpdatedAt:      "2026-11-05T14:02:11Z",
	}
}

// TestRecallWithoutOverrideIsUnrepresentable proves the hard requirement
// at the type level: no code path builds a recall that serves a
// contradicted belief without its override. The assembler fails closed,
// so an agent can never recommend from the belief alone.
func TestRecallWithoutOverrideIsUnrepresentable(t *testing.T) {
	belief := contradictedView()
	overriding := *belief.ContradictedBy
	guidance := contract.GuidanceView{
		Text:         "Take size M for Brand X",
		Basis:        contract.GuidanceCorrection,
		CorrectionID: &overriding.CorrectionID,
		BeliefID:     &belief.BeliefID,
	}

	if _, err := assembleRecall("shopper_01", []contract.BeliefView{belief}, nil, "", nil, guidance, 0, nil); !errors.Is(err, ErrOverrideMissing) {
		t.Errorf("contradicted belief without override assembles: %v", err)
	}

	other := overriding
	other.CorrectionID = "corr_other"
	if _, err := assembleRecall("shopper_01", []contract.BeliefView{belief}, nil, "", &other, guidance, 0, nil); !errors.Is(err, ErrOverrideMissing) {
		t.Errorf("contradicted belief with a foreign override assembles: %v", err)
	}

	if _, err := assembleRecall("shopper_01", []contract.BeliefView{belief}, nil, "", &overriding,
		contract.GuidanceView{Basis: contract.GuidanceBelief}, 0, nil); !errors.Is(err, ErrOverrideMissing) {
		t.Errorf("override without correction guidance assembles: %v", err)
	}

	if _, err := assembleRecall("shopper_01", []contract.BeliefView{belief}, nil, "", &overriding, guidance, 0, nil); err != nil {
		t.Errorf("valid recall with override fails: %v", err)
	}

	plain := belief
	plain.Contradicted = false
	plain.ContradictedBy = nil
	plain.Status = domain.StatusEarned
	plain.Standing = domain.GraphVenerable
	won := contract.PrecedenceView{Step: contract.GuidanceOwnEvidence}
	plain.Precedence = &won
	if _, err := assembleRecall("shopper_01", []contract.BeliefView{plain}, nil, "", nil,
		contract.GuidanceView{Basis: contract.GuidanceBelief}, 0, nil); err != nil {
		t.Errorf("uncontradicted recall fails: %v", err)
	}
}

// TestRecallWithTwoCorrectionsNeedsEachInline pins the assembler's shape
// for the case two beliefs for one brand are each corrected: the response
// carries one top-level override and each contradicted belief carries its
// own inline, so recall serves both instead of refusing. The override must
// still be one of those inline corrections — a foreign one fails closed,
// a contradicted belief with no inline fails closed, and one carrying a
// correction that cites another belief fails closed — so a second
// correction is never a way to smuggle a belief past its own warning.
func TestRecallWithTwoCorrectionsNeedsEachInline(t *testing.T) {
	first := contradictedView()
	second := contradictedView()
	second.BeliefID = "blf_2"
	second.Value = "runs large, take size S"
	secondCorrection := *second.ContradictedBy
	secondCorrection.CorrectionID = "corr_2"
	secondCorrection.Text = "M fits me now"
	secondCorrection.ContradictsBeliefID = "blf_2"
	second.ContradictedBy = &secondCorrection

	firstInline := *first.ContradictedBy
	guidance := contract.GuidanceView{
		Text:         "Take size M for Brand X",
		Basis:        contract.GuidanceCorrection,
		CorrectionID: &firstInline.CorrectionID,
		BeliefID:     &first.BeliefID,
	}

	if _, err := assembleRecall("shopper_01", []contract.BeliefView{first, second}, nil, "", &firstInline, guidance, 0, nil); err != nil {
		t.Fatalf("two contradicted beliefs with their own inline corrections fail: %v", err)
	}

	foreign := firstInline
	foreign.CorrectionID = "corr_other"
	if _, err := assembleRecall("shopper_01", []contract.BeliefView{first, second}, nil, "", &foreign, guidance, 0, nil); !errors.Is(err, ErrOverrideMissing) {
		t.Errorf("a top-level override of neither served belief assembles: %v", err)
	}

	if _, err := assembleRecall("shopper_01", []contract.BeliefView{first, second}, nil, "", nil, contract.GuidanceView{Basis: contract.GuidanceBelief}, 0, nil); !errors.Is(err, ErrOverrideMissing) {
		t.Errorf("two contradicted beliefs with no override assemble: %v", err)
	}

	bare := second
	bare.ContradictedBy = nil
	if _, err := assembleRecall("shopper_01", []contract.BeliefView{first, bare}, nil, "", &firstInline, guidance, 0, nil); !errors.Is(err, ErrOverrideMissing) {
		t.Errorf("a contradicted belief with no inline correction assembles: %v", err)
	}

	// A correction that cites another belief is not this belief's own: the
	// warning would point at the wrong row, so assembly fails closed.
	crossed := second
	crossed.ContradictedBy = &firstInline
	if _, err := assembleRecall("shopper_01", []contract.BeliefView{first, crossed}, nil, "", &firstInline, guidance, 0, nil); !errors.Is(err, ErrOverrideMissing) {
		t.Errorf("a belief carrying another belief's inline correction assembles: %v", err)
	}
}

// TestRecallFailsClosedOnLostCorrection proves the service never serves
// a contradicted belief whose correction is gone: even with a corrupt
// state, recall errors instead of returning the belief alone.
func TestRecallFailsClosedOnLostCorrection(t *testing.T) {
	ctx := context.Background()
	svc := seededService(t)
	svc.mu.Lock()
	delete(svc.shoppers["shopper_01"].corrections, "corr_3b7e52")
	svc.mu.Unlock()

	if _, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: "shopper_01", BrandID: "brand-x-001"}); !errors.Is(err, ErrOverrideMissing) {
		t.Errorf("recall with a lost correction returns instead of failing: %v", err)
	}
	if _, err := svc.LedgerBelief(ctx, contract.LedgerBeliefRequest{ShopperID: "shopper_01", BeliefID: "blf_8f2c41"}); !errors.Is(err, ErrOverrideMissing) {
		t.Errorf("ledger read with a lost correction returns instead of failing: %v", err)
	}
}
