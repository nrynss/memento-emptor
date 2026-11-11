package domain

import (
	"errors"
	"fmt"
)

// BeliefKind names the two kinds of belief. The reason category decides
// which kind a piece of evidence counts toward; cross-brand sizing and
// style preferences are out.
type BeliefKind string

// The belief kinds.
const (
	// BeliefKindFit is sizing for a brand for this shopper, such as
	// Brand X runs small, size up to L.
	BeliefKindFit BeliefKind = "fit"
	// BeliefKindSeller is seller reliability for this shopper.
	BeliefKindSeller BeliefKind = "seller_reliability"
	// BeliefKindNone is the zero value: the evidence counts toward
	// nothing.
	BeliefKindNone BeliefKind = ""
)

// SubjectKind names what a belief is about.
type SubjectKind string

// The belief subjects.
const (
	// SubjectBrand is a brand, for fit beliefs.
	SubjectBrand SubjectKind = "brand"
	// SubjectSeller is a seller, for seller-reliability beliefs.
	SubjectSeller SubjectKind = "seller"
)

// Subject is what a belief is about: a brand or a seller. Its JSON keys are
// the contract's: every belief schema declares kind, id and name with no
// additional properties, so the tags belong here rather than in a rename.
type Subject struct {
	// Kind is whether the subject is a brand or a seller.
	Kind SubjectKind `json:"kind"`
	// ID identifies the subject, such as the brand id.
	ID string `json:"id"`
	// Name is the subject's display name.
	Name string `json:"name"`
}

// BeliefStatus is what the ledger shows for a belief.
type BeliefStatus string

// The ledger statuses.
const (
	// StatusClaimed is a general statement the shopper made. It is not
	// evidence and it never promotes.
	StatusClaimed BeliefStatus = "Claimed"
	// StatusBuilding means evidence is accumulating but the bar is not met.
	StatusBuilding BeliefStatus = "Building"
	// StatusEarned means enough documented evidence, over enough time, to
	// act on.
	StatusEarned BeliefStatus = "Earned"
	// StatusOutOfTrue means something newer disagrees. The belief is kept
	// and shown beside the contradiction, never deleted.
	StatusOutOfTrue BeliefStatus = "Out of true"
)

// BeliefStatuses lists every ledger status.
var BeliefStatuses = []BeliefStatus{
	StatusClaimed,
	StatusBuilding,
	StatusEarned,
	StatusOutOfTrue,
}

// GraphState is the promotion state of a belief in the evidence graph.
// Contradicted is the overlay a correction declares: the belief keeps
// its state and is shown beside the warning.
type GraphState string

// The evidence-graph states the ledger reads.
const (
	// GraphNone is an assertion that never promoted: the ledger's Claimed.
	GraphNone GraphState = "None"
	// GraphCandidate is accumulating evidence: the ledger's Building.
	GraphCandidate GraphState = "Candidate"
	// GraphVenerable is evidence over enough time: the ledger's Earned.
	// Three fit returns can reach it at most; the bar above it needs more
	// than five dependents.
	GraphVenerable GraphState = "Venerable"
	// GraphContradicted is a belief with a declared contradiction. The
	// belief stays at its state and the warning renders beside it: the
	// ledger's Out of true.
	GraphContradicted GraphState = "contradicted"
)

// GraphStates lists every evidence-graph state the ledger reads.
var GraphStates = []GraphState{
	GraphNone,
	GraphCandidate,
	GraphVenerable,
	GraphContradicted,
}

// ErrUnknownBeliefStatus is returned when a belief status is not in the ledger table.
var ErrUnknownBeliefStatus = errors.New("domain: unknown belief status")

// ErrUnknownGraphState is returned when a graph state is not in the ledger table.
var ErrUnknownGraphState = errors.New("domain: unknown graph state")

// GraphStateFor maps a ledger status to its evidence-graph state.
func GraphStateFor(status BeliefStatus) (GraphState, error) {
	switch status {
	case StatusClaimed:
		return GraphNone, nil
	case StatusBuilding:
		return GraphCandidate, nil
	case StatusEarned:
		return GraphVenerable, nil
	case StatusOutOfTrue:
		return GraphContradicted, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownBeliefStatus, string(status))
	}
}

// StatusForGraph maps an evidence-graph state to its ledger status.
func StatusForGraph(state GraphState) (BeliefStatus, error) {
	switch state {
	case GraphNone:
		return StatusClaimed, nil
	case GraphCandidate:
		return StatusBuilding, nil
	case GraphVenerable:
		return StatusEarned, nil
	case GraphContradicted:
		return StatusOutOfTrue, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownGraphState, string(state))
	}
}

// Belief is what the layer remembers about a shopper: a kind, a subject,
// the value in words, and its status.
type Belief struct {
	// Kind is whether the belief is about fit or seller reliability.
	Kind BeliefKind
	// Subject is the brand or seller the belief is about.
	Subject Subject
	// Value states the belief in words, such as runs small, take size L.
	Value string
	// Status is the ledger status of the belief.
	Status BeliefStatus
}
