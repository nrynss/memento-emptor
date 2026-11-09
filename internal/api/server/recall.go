package server

import (
	"errors"
	"fmt"
	"sort"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// ErrOverrideMissing is returned when a recall would serve a contradicted
// belief without its override. The service fails the call instead of
// letting an agent recommend from a belief the shopper has overruled.
var ErrOverrideMissing = errors.New("server: contradicted belief has no overriding correction")

// assembleRecall builds the recall response from its parts. A contradicted
// belief is unrepresentable without its override: when one applies and no
// correction carries it, assembly fails instead of serving the belief
// alone. Every handler builds recall through here, so no code path can
// return the belief without the correction beside it.
func assembleRecall(shopper string, beliefs []contract.BeliefView, overriding *contract.CorrectionView, guidance contract.GuidanceView, pending int, trace *string) (contract.RecallResponse, error) {
	for _, belief := range beliefs {
		if !belief.Contradicted {
			continue
		}
		if overriding == nil {
			return contract.RecallResponse{}, fmt.Errorf("%w: belief %q", ErrOverrideMissing, belief.BeliefID)
		}
		if belief.ContradictedBy == nil || belief.ContradictedBy.CorrectionID != overriding.CorrectionID {
			return contract.RecallResponse{}, fmt.Errorf("%w: belief %q", ErrOverrideMissing, belief.BeliefID)
		}
	}
	if overriding != nil && guidance.Basis != contract.GuidanceCorrection {
		return contract.RecallResponse{}, fmt.Errorf("%w: override present without correction guidance", ErrOverrideMissing)
	}
	if beliefs == nil {
		beliefs = []contract.BeliefView{}
	}
	return contract.RecallResponse{
		ShopperID:            shopper,
		Beliefs:              beliefs,
		OverridingCorrection: overriding,
		Guidance:             guidance,
		PendingCount:         pending,
		TraceID:              trace,
	}, nil
}

// recallBeliefs selects the beliefs a recall serves for one brand: the
// earned fit beliefs, strongest first. Building beliefs stay on the
// ledger until the evidence earns them; seller beliefs surface through
// the ledger and explain reads, never through a sizing recall. A
// contradicted belief is served with its warning inline, never alone:
// the assembler beside this function enforces the override.
func recallBeliefs(views []contract.BeliefView, brandID string) []contract.BeliefView {
	out := []contract.BeliefView{}
	for _, view := range views {
		if view.Kind != domain.BeliefKindFit {
			continue
		}
		if brandID != "" && view.Subject.ID != brandID {
			continue
		}
		if view.Standing != domain.GraphVenerable {
			continue
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Evidence) != len(out[j].Evidence) {
			return len(out[i].Evidence) > len(out[j].Evidence)
		}
		return out[i].BeliefID < out[j].BeliefID
	})
	return out
}
