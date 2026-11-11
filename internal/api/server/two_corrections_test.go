package server

import (
	"context"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// The two corrections this file's shopper declares: one against each fit
// belief the shopper holds for the brand. Both name the size that works
// now, so the guidance and the inline warnings agree on words.
const (
	twoCorrectionsSmallText = "I've lost weight, use M"
	twoCorrectionsRivalText = "M fits me now"
)

// twoCorrectionHarness builds a service on the frozen story's four refunds
// with a graph allowlist widened by the second correction, so one shopper
// can contest two beliefs for one brand with different words.
func twoCorrectionHarness(t *testing.T) (*Service, *spyGraph) {
	t.Helper()
	store, err := evidence.NewStore(testContract{kind: "server-rival"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	events := []string{"WH-2EV-01", "WH-2EV-02", "WH-2EV-03", "WH-2EV-05"}
	allowed := append(rivalAllowlist(events), twoCorrectionsRivalText)
	spy := &spyGraph{inner: fakes.NewRecordingEvidenceGraph(allowed, fakes.StoryRecalls())}
	fixed := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:   fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      spy,
		SchemaDir:  apiDir(t),
		Clock:      func() time.Time { return fixed },
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc, spy
}

// TestTwoCorrectedBeliefsDoNotMakeRecallRefuse is the P8 review's High,
// reproduced and pinned: a shopper holds two fit beliefs for one brand
// because rival birth gave them one each way, and corrects both. Recall
// serves them both, and each carries its own correction inline, so no
// contradiction is left without its override and no recall for the brand
// refuses. The beliefs are never deleted: both stay on the ledger, marked
// contradicted, which is rule 4's "the correction wins at once, and the
// belief stays".
func TestTwoCorrectedBeliefsDoNotMakeRecallRefuse(t *testing.T) {
	ctx := context.Background()
	svc, spy := twoCorrectionHarness(t)
	shopper := "shopper_two_corrections"

	first := answerOne(t, svc, shopper, "WH-2EV-01", rivalSmallAnswer)
	if !first.Accepted || first.BeliefID == nil {
		t.Fatalf("first return: accepted %v belief %v, want true and attached", first.Accepted, first.BeliefID)
	}
	smallID := *first.BeliefID
	for _, event := range []string{"WH-2EV-02", "WH-2EV-03"} {
		if res := answerOne(t, svc, shopper, event, rivalLargeAnswer); res.Accepted {
			t.Fatalf("%s opposite was accepted: the run is at 2", event)
		}
	}
	birth := answerOne(t, svc, shopper, "WH-2EV-05", rivalLargeAnswer)
	if !birth.Accepted || birth.BeliefID == nil || *birth.BeliefID == smallID {
		t.Fatalf("third opposite attaches to %v, want the newborn rival", birth.BeliefID)
	}
	rivalID := *birth.BeliefID
	if rows := ledgerFitBeliefs(t, svc, shopper); len(rows) != 2 {
		t.Fatalf("after the birth the shopper holds %d fit beliefs, want 2", len(rows))
	}

	small, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID:           shopper,
		Text:                twoCorrectionsSmallText,
		ContradictsBeliefID: smallID,
	})
	if err != nil {
		t.Fatalf("correct the runs-small belief: %v", err)
	}
	rival, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID:           shopper,
		Text:                twoCorrectionsRivalText,
		ContradictsBeliefID: rivalID,
	})
	if err != nil {
		t.Fatalf("correct the runs-large rival: %v", err)
	}
	for name, res := range map[string]contract.RecordCorrectionResponse{"runs small": small, "runs large": rival} {
		if !res.Contradicted {
			t.Fatalf("the %s correction contradicts no belief: %+v", name, res)
		}
	}
	if small.BeliefID != smallID || rival.BeliefID != rivalID {
		t.Fatalf("corrections cite %q and %q, want each its own belief %q and %q",
			small.BeliefID, rival.BeliefID, smallID, rivalID)
	}
	if got := againstEdges(spy); got != 2 {
		t.Errorf("graph saw %d against edges, want 2: each correction declares against its own belief", got)
	}

	rows := ledgerFitBeliefs(t, svc, shopper)
	if len(rows) != 2 {
		t.Fatalf("the ledger holds %d fit beliefs after both corrections, want 2: the beliefs stay", len(rows))
	}
	for _, row := range rows {
		if !row.Belief.Contradicted || row.Belief.ContradictedBy == nil {
			t.Fatalf("ledger row %q reads contradicted %v with %+v, want each belief marked with its own correction",
				row.Belief.BeliefID, row.Belief.Contradicted, row.Belief.ContradictedBy)
		}
	}

	recall, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("recall refuses two corrected beliefs for one brand: %v", err)
	}
	if len(recall.Beliefs) != 2 {
		t.Fatalf("recall serves %d beliefs, want both corrected ones", len(recall.Beliefs))
	}
	inline := map[string]string{}
	for _, belief := range recall.Beliefs {
		if !belief.Contradicted || belief.ContradictedBy == nil {
			t.Fatalf("served belief %q carries contradicted %v with %+v, want its own correction",
				belief.BeliefID, belief.Contradicted, belief.ContradictedBy)
		}
		if belief.ContradictedBy.ContradictsBeliefID != belief.BeliefID {
			t.Fatalf("belief %q carries the correction of %q", belief.BeliefID, belief.ContradictedBy.ContradictsBeliefID)
		}
		// Rule 4: the correction decides for each belief, and the belief stays
		// beside it marked overridden.
		if belief.Precedence == nil || !belief.Precedence.Overridden ||
			belief.Precedence.OverriddenBy == nil || *belief.Precedence.OverriddenBy != contract.GuidanceCorrection {
			t.Fatalf("served belief %q precedence reads %+v, want it overridden by the correction", belief.BeliefID, belief.Precedence)
		}
		inline[belief.ContradictedBy.CorrectionID] = belief.BeliefID
	}
	if len(inline) != 2 {
		t.Fatalf("the two served beliefs carry %d distinct corrections, want one each: %v", len(inline), inline)
	}
	if _, ok := inline[small.Correction.CorrectionID]; !ok {
		t.Errorf("the runs-small correction %q rides on no served belief", small.Correction.CorrectionID)
	}
	if _, ok := inline[rival.Correction.CorrectionID]; !ok {
		t.Errorf("the runs-large correction %q rides on no served belief", rival.Correction.CorrectionID)
	}

	over := recall.OverridingCorrection
	if over == nil || recall.Guidance.Basis != contract.GuidanceCorrection {
		t.Fatalf("no correction decides with two served: %+v", recall.Guidance)
	}
	if _, ok := inline[over.CorrectionID]; !ok {
		t.Fatalf("the top-level override %q is no served belief's own correction", over.CorrectionID)
	}
	// The harness pins one clock for both corrections, so the two record
	// times tie and recency cannot decide: the pick falls to arrival order,
	// and the rival's correction, recorded second, is the top level.
	if over.CorrectionID != rival.Correction.CorrectionID {
		t.Errorf("the top-level override is %q, want the later correction %q",
			over.CorrectionID, rival.Correction.CorrectionID)
	}
	if recall.Guidance.CorrectionID == nil || *recall.Guidance.CorrectionID != over.CorrectionID {
		t.Fatalf("guidance names %v, want the top-level override %q", recall.Guidance.CorrectionID, over.CorrectionID)
	}
	if recall.Guidance.BeliefID == nil || *recall.Guidance.BeliefID != rivalID {
		t.Fatalf("guidance points at %v, want the belief the override contradicts %q", recall.Guidance.BeliefID, rivalID)
	}

	again, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		t.Fatalf("the second recall refuses what the first served: %v", err)
	}
	if again.OverridingCorrection == nil || again.OverridingCorrection.CorrectionID != over.CorrectionID {
		t.Fatalf("the second recall decides from %+v, want the same override %q", again.OverridingCorrection, over.CorrectionID)
	}
	servedAgain := map[string]bool{}
	for _, belief := range again.Beliefs {
		if belief.ContradictedBy == nil {
			t.Fatalf("the second recall serves belief %q with no inline correction", belief.BeliefID)
		}
		servedAgain[belief.BeliefID] = true
	}
	if !servedAgain[smallID] || !servedAgain[rivalID] {
		t.Fatalf("the second recall serves %v, want both corrected beliefs %q and %q", servedAgain, smallID, rivalID)
	}
}
