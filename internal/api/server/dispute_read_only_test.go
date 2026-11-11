package server

import (
	"context"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// TestDisputeReachesLedgerAndExplainNeverRecall pins product.md's step 5 as
// a read-only demonstration. The shopper holds a sizing belief earned from
// three fit returns and a not-arrived dispute classified from PayPal's
// reason code. The layer's own belief and ledger reads serve the seller
// belief with its one dispute row, and the sizing belief keeps exactly its
// three fit rows. Recall for the brand serves the fit belief, its guidance
// and the pool, and the seller belief is absent by design: recall is the
// recommendation surface, seller evidence is not a recommendation, and
// serving it there would put a non-fit belief in the agent's
// recommendation path. No agent turn can reach the dispute by reading:
// recall does not serve its id, and the ledger reads that name it are
// shopper-only. That is why the step is recorded as a read-only
// demonstration rather than a turn.
func TestDisputeReachesLedgerAndExplainNeverRecall(t *testing.T) {
	ctx := context.Background()
	svc, _ := pivotHarness(t, &fakePoolIndex{result: earnedSmallPool()}, &fakeFitIndex{}, "")
	shopper := "shopper_01"
	fitID := earnSmallBelief(t, svc, shopper)
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-04"); err != nil {
		t.Fatalf("ingest the not-arrived dispute: %v", err)
	}

	// ledger_beliefs: the seller belief is a row of its own, Claimed with
	// one evidence row, and the sizing belief is untouched at three.
	ledger, err := svc.LedgerBeliefs(ctx, contract.LedgerBeliefsRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("ledger beliefs: %v", err)
	}
	sellerID := ""
	for _, row := range ledger.Beliefs {
		if row.Belief.BeliefID == fitID && row.EvidenceCount != 3 {
			t.Errorf("sizing belief holds %d rows, want its three fit returns", row.EvidenceCount)
		}
		if row.Belief.Kind != domain.BeliefKindSeller {
			continue
		}
		sellerID = row.Belief.BeliefID
		if row.Belief.Status != domain.StatusClaimed {
			t.Errorf("seller belief reads %q, want Claimed: one dispute never earns", row.Belief.Status)
		}
		if row.EvidenceCount != 1 {
			t.Errorf("seller belief holds %d rows, want its one dispute", row.EvidenceCount)
		}
		if row.Belief.Value != sellerBeliefValue {
			t.Errorf("seller belief value %q, want %q", row.Belief.Value, sellerBeliefValue)
		}
		if row.Belief.Subject.Kind != domain.SubjectSeller {
			t.Errorf("seller belief subject is %q, want a seller", row.Belief.Subject.Kind)
		}
	}
	if sellerID == "" {
		t.Fatal("ledger lists no seller belief: the dispute wrote nowhere to read")
	}

	// ledger_belief: the row in full carries the dispute, its code's
	// reason and what it counts toward.
	full, err := svc.LedgerBelief(ctx, contract.LedgerBeliefRequest{ShopperID: shopper, BeliefID: sellerID})
	if err != nil {
		t.Fatalf("ledger belief %s: %v", sellerID, err)
	}
	if full.Belief.BeliefID != sellerID || full.Belief.Kind != domain.BeliefKindSeller {
		t.Fatalf("ledger belief reads %q of kind %q, want the seller belief", full.Belief.BeliefID, full.Belief.Kind)
	}
	if len(full.Belief.Evidence) != 1 {
		t.Fatalf("seller belief carries %d evidence rows, want its one dispute", len(full.Belief.Evidence))
	}
	dispute := full.Belief.Evidence[0]
	if dispute.Outcome.Kind != domain.OutcomeDispute {
		t.Errorf("seller evidence is a %q, want the dispute", dispute.Outcome.Kind)
	}
	if dispute.Reason == nil || dispute.Reason.Category != domain.CategoryNotArrived {
		t.Errorf("seller evidence reason reads %+v, want not_arrived", dispute.Reason)
	}
	if dispute.CountsToward == nil || *dispute.CountsToward != domain.BeliefKindSeller {
		t.Errorf("seller evidence counts toward %v, want seller_reliability", dispute.CountsToward)
	}
	if dispute.Item.Category == "shirts" {
		t.Errorf("seller evidence is the oxford shirt: the dispute would count toward sizing")
	}

	// ledger_evidence: the same row, linked to the seller belief, and no
	// fit row is linked to it.
	rows, err := svc.LedgerEvidence(ctx, contract.LedgerEvidenceRequest{ShopperID: shopper, BeliefID: sellerID})
	if err != nil {
		t.Fatalf("ledger evidence for %s: %v", sellerID, err)
	}
	if len(rows.Evidence) != 1 {
		t.Fatalf("seller belief links %d evidence rows, want its one dispute", len(rows.Evidence))
	}
	if len(rows.Evidence[0].BeliefIDs) != 1 || rows.Evidence[0].BeliefIDs[0] != sellerID {
		t.Errorf("seller evidence links %v, want only %q", rows.Evidence[0].BeliefIDs, sellerID)
	}

	// explain_belief: the seller belief is explainable like any other, so
	// an agent-admitted read can render it when the belief id is known.
	explained, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: sellerID})
	if err != nil {
		t.Fatalf("explain the seller belief: %v", err)
	}
	if explained.Belief.BeliefID != sellerID || explained.Belief.Kind != domain.BeliefKindSeller {
		t.Fatalf("explain reads %q of kind %q, want the seller belief", explained.Belief.BeliefID, explained.Belief.Kind)
	}
	if len(explained.Belief.Evidence) != 1 || explained.Belief.Evidence[0].EvidenceID != dispute.EvidenceID {
		t.Errorf("explain shows %d evidence rows, want the dispute %q",
			len(explained.Belief.Evidence), dispute.EvidenceID)
	}
	if len(explained.Corrections) != 0 {
		t.Errorf("explain carries %d corrections, want none against the seller belief", len(explained.Corrections))
	}

	// recall_for_purchase: the fit belief and the pool are served, and the
	// seller belief is absent. A seller belief is not a recommendation and
	// matches none of the five precedence steps, so it never reaches this
	// surface.
	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: shopper, BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(recall.Beliefs) != 1 {
		t.Fatalf("recall serves %d beliefs, want the one fit belief", len(recall.Beliefs))
	}
	served := recall.Beliefs[0]
	if served.BeliefID != fitID || served.Kind != domain.BeliefKindFit {
		t.Fatalf("recall serves %q of kind %q, want the fit belief %q", served.BeliefID, served.Kind, fitID)
	}
	if len(served.Evidence) != 3 {
		t.Errorf("served fit belief carries %d rows, want its three fit returns", len(served.Evidence))
	}
	for _, ev := range served.Evidence {
		if ev.EvidenceID == dispute.EvidenceID {
			t.Errorf("the dispute %q rides in the sizing belief's evidence", ev.EvidenceID)
		}
	}
	if recall.Guidance.Basis != contract.GuidanceOwnEvidence {
		t.Errorf("guidance basis %q, want the shopper's own evidence", recall.Guidance.Basis)
	}
	if !strings.Contains(recall.Guidance.Text, "Take size L for Brand X") {
		t.Errorf("guidance text %q, want it to take size L", recall.Guidance.Text)
	}
	if recall.Pool == nil || recall.PoolLookup != contract.PoolConsulted {
		t.Errorf("recall carries no consulted pool beside the fit belief: %+v", recall.Pool)
	}
	for _, view := range recall.Beliefs {
		if view.BeliefID == sellerID || view.Kind != domain.BeliefKindFit {
			t.Errorf("recall serves %q of kind %q: a seller belief reached the recommendation surface", view.BeliefID, view.Kind)
		}
	}

	// A recall that names no brand serves every fit belief the shopper
	// holds and still no seller belief: the kind, not the brand filter, is
	// what keeps seller evidence off this surface.
	wide, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("brandless recall: %v", err)
	}
	if len(wide.Beliefs) != 1 || wide.Beliefs[0].BeliefID != fitID {
		t.Fatalf("brandless recall serves %d beliefs, want only the fit belief %q", len(wide.Beliefs), fitID)
	}
}
