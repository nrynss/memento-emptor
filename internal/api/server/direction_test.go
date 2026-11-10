package server

import (
	"context"
	"testing"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// againstEdges counts the graph writes that declare a contradiction.
func againstEdges(spy *spyGraph) int {
	spy.mu.Lock()
	defer spy.mu.Unlock()
	out := 0
	for _, action := range spy.actions {
		out += len(action.against)
	}
	return out
}

// structuralWrites counts the graph's structural writes.
func structuralWrites(spy *spyGraph) int {
	spy.mu.Lock()
	defer spy.mu.Unlock()
	return len(spy.actions)
}

// fitBelief finds the ledger row for the fit belief carrying one value.
func fitBelief(t *testing.T, svc *Service, shopper, value string) contract.LedgerBeliefRow {
	t.Helper()
	ledger, err := svc.LedgerBeliefs(context.Background(), contract.LedgerBeliefsRequest{ShopperID: shopper, IncludeSeeded: true})
	if err != nil {
		t.Fatalf("ledger beliefs: %v", err)
	}
	for _, row := range ledger.Beliefs {
		if row.Belief.Kind == domain.BeliefKindFit && row.Belief.Value == value {
			return row
		}
	}
	t.Fatalf("no fit belief with value %q", value)
	return contract.LedgerBeliefRow{}
}

// TestDirectionMismatchAttachesNowhere proves the pinned rule through the
// service on a runs-small-only graph: a too-large answer neither supports
// the runs-small belief — its evidence count does not move and the graph
// sees no new structural write — nor contradicts it — no against edge is
// declared and no warning renders. The question still closes, and the row
// stays stored with its reason, visible as unattached in the ledger
// evidence with an empty belief link. A later same-direction answer
// attaches to the same belief.
func TestDirectionMismatchAttachesNowhere(t *testing.T) {
	ctx := context.Background()
	svc, spy := storyHarness(t)
	shopper := "shopper_01"

	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-01"); err != nil {
		t.Fatalf("ingest WH-2EV-01: %v", err)
	}
	first := answerReason(t, svc, shopper, "WH-2EV-01", "too tight in the shoulders")
	if !first.Accepted || first.BeliefID == nil {
		t.Fatalf("first return: accepted %v belief %v, want true and attached", first.Accepted, first.BeliefID)
	}
	smallID := *first.BeliefID
	writes := structuralWrites(spy)

	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-02"); err != nil {
		t.Fatalf("ingest WH-2EV-02: %v", err)
	}
	mismatch := answerReason(t, svc, shopper, "WH-2EV-02", "it swims on me, far too big")
	if mismatch.Accepted || mismatch.AskAgain || mismatch.BeliefID != nil {
		t.Fatalf("too-large answer: accepted %v ask again %v belief %v, want false false nil",
			mismatch.Accepted, mismatch.AskAgain, mismatch.BeliefID)
	}
	if mismatch.Category != domain.CategoryFitTooLarge {
		t.Fatalf("too-large answer: category %q, want fit_too_large", string(mismatch.Category))
	}
	if mismatch.CountsToward != nil {
		t.Fatalf("too-large answer counts toward %q, want nothing", string(*mismatch.CountsToward))
	}
	pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("pending after mismatch: %v", err)
	}
	if len(pending.Questions) != 0 {
		t.Fatalf("pending holds %d questions, want 0: the classified answer closed it", len(pending.Questions))
	}

	if got := structuralWrites(spy); got != writes {
		t.Errorf("graph saw %d structural writes, want %d: the mismatch wrote nothing", got, writes)
	}
	if got := againstEdges(spy); got != 0 {
		t.Errorf("graph saw %d against edges, want 0: contradictions stay shopper-driven", got)
	}
	small := fitBelief(t, svc, shopper, "runs small, take size L")
	if small.Belief.BeliefID != smallID {
		t.Fatalf("sizing belief id %q, want the first answer's %q", small.Belief.BeliefID, smallID)
	}
	if small.EvidenceCount != 1 {
		t.Errorf("sizing belief holds %d rows, want 1: the too-large return added no support", small.EvidenceCount)
	}
	if small.Belief.Contradicted {
		t.Error("sizing belief reads contradicted: the mismatch declared nothing against it")
	}

	ledger, err := svc.LedgerEvidence(ctx, contract.LedgerEvidenceRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("ledger evidence: %v", err)
	}
	seenAttached, seenUnattached := false, false
	for _, row := range ledger.Evidence {
		switch row.Evidence.Reason.Category {
		case domain.CategoryFitTooSmall:
			if len(row.BeliefIDs) != 1 || row.BeliefIDs[0] != smallID {
				t.Errorf("too-small row links %v, want [%q]", row.BeliefIDs, smallID)
			}
			seenAttached = true
		case domain.CategoryFitTooLarge:
			if len(row.BeliefIDs) != 0 {
				t.Errorf("too-large row links %v, want unattached", row.BeliefIDs)
			}
			if row.Evidence.CountsToward != nil {
				t.Errorf("too-large row counts toward %q, want nothing", string(*row.Evidence.CountsToward))
			}
			seenUnattached = true
		}
	}
	if !seenAttached || !seenUnattached {
		t.Fatalf("ledger evidence shows attached %v unattached %v, want both", seenAttached, seenUnattached)
	}

	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-03"); err != nil {
		t.Fatalf("ingest WH-2EV-03: %v", err)
	}
	third := answerReason(t, svc, shopper, "WH-2EV-03", "the sleeves are way too short and I can't button it")
	if !third.Accepted || third.BeliefID == nil || *third.BeliefID != smallID {
		t.Fatalf("later same-direction answer attaches to %v, want the runs-small %q", third.BeliefID, smallID)
	}
	small = fitBelief(t, svc, shopper, "runs small, take size L")
	if small.EvidenceCount != 2 {
		t.Errorf("sizing belief holds %d rows, want 2 after the matching return", small.EvidenceCount)
	}
	if got := againstEdges(spy); got != 0 {
		t.Errorf("graph saw %d against edges, want 0", got)
	}
}

// TestDirectionMatchAttachesToOppositeBelief proves the other half of the
// rule: when a runs-large belief is already present, a too-large answer
// attaches to it, while a too-small answer on the same graph still
// attaches nowhere and contradicts nothing.
func TestDirectionMatchAttachesToOppositeBelief(t *testing.T) {
	ctx := context.Background()
	svc, spy := storyHarness(t)
	shopper := "shopper_01"

	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-01"); err != nil {
		t.Fatalf("ingest WH-2EV-01: %v", err)
	}
	first := answerReason(t, svc, shopper, "WH-2EV-01", "it swims on me, far too big")
	if !first.Accepted || first.BeliefID == nil {
		t.Fatalf("first too-large return: accepted %v belief %v, want true and attached", first.Accepted, first.BeliefID)
	}
	largeID := *first.BeliefID
	large := fitBelief(t, svc, shopper, "runs large, take size S")
	if large.Belief.BeliefID != largeID {
		t.Fatalf("runs-large belief id %q, want the first answer's %q", large.Belief.BeliefID, largeID)
	}

	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-02"); err != nil {
		t.Fatalf("ingest WH-2EV-02: %v", err)
	}
	second := answerReason(t, svc, shopper, "WH-2EV-02", "it swims on me, far too big")
	if !second.Accepted || second.BeliefID == nil || *second.BeliefID != largeID {
		t.Fatalf("second too-large answer attaches to %v, want the runs-large %q", second.BeliefID, largeID)
	}
	large = fitBelief(t, svc, shopper, "runs large, take size S")
	if large.EvidenceCount != 2 {
		t.Errorf("runs-large belief holds %d rows, want 2", large.EvidenceCount)
	}

	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-03"); err != nil {
		t.Fatalf("ingest WH-2EV-03: %v", err)
	}
	writes := structuralWrites(spy)
	mismatch := answerReason(t, svc, shopper, "WH-2EV-03", "too tight in the shoulders")
	if mismatch.Accepted || mismatch.AskAgain || mismatch.BeliefID != nil {
		t.Fatalf("too-small answer: accepted %v ask again %v belief %v, want false false nil",
			mismatch.Accepted, mismatch.AskAgain, mismatch.BeliefID)
	}
	if got := structuralWrites(spy); got != writes {
		t.Errorf("graph saw %d structural writes, want %d: the mismatch wrote nothing", got, writes)
	}
	large = fitBelief(t, svc, shopper, "runs large, take size S")
	if large.EvidenceCount != 2 {
		t.Errorf("runs-large belief holds %d rows, want 2: the too-small return added no support", large.EvidenceCount)
	}
	if large.Belief.Contradicted {
		t.Error("runs-large belief reads contradicted: the mismatch declared nothing against it")
	}
	if got := againstEdges(spy); got != 0 {
		t.Errorf("graph saw %d against edges, want 0", got)
	}
}
