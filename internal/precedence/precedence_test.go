package precedence

import (
	"reflect"
	"testing"

	"github.com/nrynss/memento-emptor/internal/api/contract"
)

// TestDecideEveryStepDeciding proves each of the five steps decides when
// it is the only one present, and that nothing decides when no step is.
func TestDecideEveryStepDeciding(t *testing.T) {
	cases := []struct {
		name     string
		presence Presence
		winner   contract.GuidanceBasis
	}{
		{"correction", Presence{Correction: true}, contract.GuidanceCorrection},
		{"own evidence", Presence{OwnEvidence: true}, contract.GuidanceOwnEvidence},
		{"pool", Presence{Pool: true}, contract.GuidancePool},
		{"statement", Presence{Statement: true}, contract.GuidanceStatement},
		{"nothing", Presence{}, contract.GuidanceNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := Decide(tc.presence)
			if outcome.Winner != tc.winner {
				t.Fatalf("winner %q, want %q", string(outcome.Winner), string(tc.winner))
			}
			if len(outcome.Overridden) != 0 {
				t.Fatalf("a lone step overrides %v, want none", outcome.Overridden)
			}
		})
	}
}

// TestDecideEveryDisagreeingPair proves every pair of disagreeing steps:
// the stronger wins and the weaker is marked overridden by it, never
// hidden.
func TestDecideEveryDisagreeingPair(t *testing.T) {
	cases := []struct {
		name     string
		presence Presence
		winner   contract.GuidanceBasis
		loser    contract.GuidanceBasis
	}{
		{"correction over own evidence", Presence{Correction: true, OwnEvidence: true}, contract.GuidanceCorrection, contract.GuidanceOwnEvidence},
		{"correction over pool", Presence{Correction: true, Pool: true}, contract.GuidanceCorrection, contract.GuidancePool},
		{"correction over statement", Presence{Correction: true, Statement: true}, contract.GuidanceCorrection, contract.GuidanceStatement},
		{"own evidence over pool", Presence{OwnEvidence: true, Pool: true}, contract.GuidanceOwnEvidence, contract.GuidancePool},
		{"own evidence over statement", Presence{OwnEvidence: true, Statement: true}, contract.GuidanceOwnEvidence, contract.GuidanceStatement},
		{"pool over statement", Presence{Pool: true, Statement: true}, contract.GuidancePool, contract.GuidanceStatement},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := Decide(tc.presence)
			if outcome.Winner != tc.winner {
				t.Fatalf("winner %q, want %q", string(outcome.Winner), string(tc.winner))
			}
			want := map[contract.GuidanceBasis]contract.GuidanceBasis{tc.loser: tc.winner}
			if !reflect.DeepEqual(outcome.Overridden, want) {
				t.Fatalf("overridden %v, want %v", outcome.Overridden, want)
			}
		})
	}
}

// TestDecideStrongestOfThree proves the strongest of three present steps
// wins while both weaker steps are marked against it.
func TestDecideStrongestOfThree(t *testing.T) {
	outcome := Decide(Presence{Correction: true, OwnEvidence: true, Pool: true})
	if outcome.Winner != contract.GuidanceCorrection {
		t.Fatalf("winner %q, want correction", string(outcome.Winner))
	}
	want := map[contract.GuidanceBasis]contract.GuidanceBasis{
		contract.GuidanceOwnEvidence: contract.GuidanceCorrection,
		contract.GuidancePool:        contract.GuidanceCorrection,
	}
	if !reflect.DeepEqual(outcome.Overridden, want) {
		t.Fatalf("overridden %v, want %v", outcome.Overridden, want)
	}
}

// TestDecideUnreachablePool proves a pool the index could not reach never
// decides and is never marked: with own evidence present, own evidence
// decides alone; with nothing else, nothing decides and the answer is not
// an empty pool.
func TestDecideUnreachablePool(t *testing.T) {
	alone := Decide(Presence{OwnEvidence: true})
	if alone.Winner != contract.GuidanceOwnEvidence {
		t.Fatalf("unreachable pool with own evidence: winner %q, want own_evidence", string(alone.Winner))
	}
	if _, ok := alone.Overridden[contract.GuidancePool]; ok {
		t.Fatalf("unreachable pool is marked overridden: %v", alone.Overridden)
	}
	empty := Decide(Presence{})
	if empty.Winner != contract.GuidanceNone {
		t.Fatalf("unreachable pool alone: winner %q, want none", string(empty.Winner))
	}
}

// TestView proves the served marking: the winner rides unmarked, a weaker
// step rides overridden by the winner.
func TestView(t *testing.T) {
	won := View(contract.GuidancePool, contract.GuidancePool)
	if won.Overridden || won.OverriddenBy != nil {
		t.Fatalf("winning step is marked: %+v", won)
	}
	lost := View(contract.GuidancePool, contract.GuidanceOwnEvidence)
	if !lost.Overridden || lost.OverriddenBy == nil || *lost.OverriddenBy != contract.GuidanceOwnEvidence {
		t.Fatalf("losing step is unmarked: %+v", lost)
	}
}
