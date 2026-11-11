package precedence

import (
	"github.com/nrynss/memento-emptor/internal/api/contract"
)

// Presence says which precedence steps have something to say about one
// purchase. Correction is the shopper's correction in force; OwnEvidence
// is the shopper's own fit evidence for the brand; Pool is a consulted
// pool with something to say; Statement is the shopper's general
// statement. An unreachable pool is not presence: recall carries on
// without it, the step never decides, and no item is marked against it.
type Presence struct {
	// Correction is true while a correction applies to this purchase.
	Correction bool
	// OwnEvidence is true while the shopper's own fit evidence is served.
	OwnEvidence bool
	// Pool is true while the consulted pool names a direction: a Building
	// pool with a leading direction, an Earned pool, or an Out of true
	// pool. A Claimed pool says nothing, and a Building pool that names no
	// direction (the two directions level, or every fit outcome
	// fits-well) is consulted but takes no step.
	Pool bool
	// Statement is true while a general statement is served.
	Statement bool
}

// Outcome is what the order decided: the winning step, and for every
// present but weaker step, the winner that overrode it. A step that is
// absent, or a pool that could not be reached, appears in neither.
type Outcome struct {
	// Winner is the step that decided: the strongest present step, or
	// none when no step has anything to say.
	Winner contract.GuidanceBasis
	// Overridden maps each present but weaker step to the winner. It is
	// empty when at most one step is present.
	Overridden map[contract.GuidanceBasis]contract.GuidanceBasis
}

// Decide resolves one purchase's presence to its outcome. The strongest
// present step wins; every weaker present step is marked overridden by
// it. With no step present, nothing decides.
func Decide(p Presence) Outcome {
	present := []contract.GuidanceBasis{}
	if p.Correction {
		present = append(present, contract.GuidanceCorrection)
	}
	if p.OwnEvidence {
		present = append(present, contract.GuidanceOwnEvidence)
	}
	if p.Pool {
		present = append(present, contract.GuidancePool)
	}
	if p.Statement {
		present = append(present, contract.GuidanceStatement)
	}
	if len(present) == 0 {
		return Outcome{Winner: contract.GuidanceNone, Overridden: map[contract.GuidanceBasis]contract.GuidanceBasis{}}
	}
	winner := present[0]
	for _, step := range present[1:] {
		if step.Rank() < winner.Rank() {
			winner = step
		}
	}
	overridden := make(map[contract.GuidanceBasis]contract.GuidanceBasis, len(present))
	for _, step := range present {
		if step != winner {
			overridden[step] = winner
		}
	}
	return Outcome{Winner: winner, Overridden: overridden}
}

// View renders one served item's precedence: the step it came from, and
// whether the outcome's winner decided instead. An item from the winning
// step is never marked; an item from a weaker step carries the winner.
func View(step, winner contract.GuidanceBasis) contract.PrecedenceView {
	if step == winner {
		return contract.PrecedenceView{Step: step}
	}
	stronger := winner
	return contract.PrecedenceView{Step: step, Overridden: true, OverriddenBy: &stronger}
}
