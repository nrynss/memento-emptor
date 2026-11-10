package domain

import "time"

// The pool's thresholds, fixed before the comparison ran. This is the one
// definition: the pool reader, the ledger and the merchant app all read
// them from here, and docs/interface.md names each one for callers.
const (
	// PoolBuildingShoppers is the distinct-shopper floor at which a pool
	// has something to say. Below it the pool says nothing.
	PoolBuildingShoppers = 3
	// PoolEarnedShoppers is the distinct shoppers one direction needs
	// before it is Earned.
	PoolEarnedShoppers = 8
	// PoolEarnedSharePercent is the share of fit-classified settled
	// outcomes one direction needs before it is Earned.
	PoolEarnedSharePercent = 60
	// PoolEarnedRatio is how many times the opposite direction a direction
	// must reach before it is Earned.
	PoolEarnedRatio = 2
	// PoolRecentWindow is how far back the Out of true rule looks in event
	// time.
	PoolRecentWindow = 90 * 24 * time.Hour
)

// PoolCounts are the settled counts for one product line and size that the
// pooled status rule reads. A purchase still inside its return window, and
// a refund with no reason yet, settle nothing and appear in neither a
// direction nor the Building floor.
type PoolCounts struct {
	// TooSmall counts settled outcomes pointing small: a return whose
	// reason is that direction, or fit feedback with that verdict.
	TooSmall int
	// TooLarge counts settled outcomes pointing large.
	TooLarge int
	// FitsWell counts settled feedback saying the size fits. It points
	// nowhere, so it belongs to neither direction, but it is fit evidence
	// and dilutes a direction's share.
	FitsWell int
	// ShoppersTooSmall counts distinct shoppers pointing small.
	ShoppersTooSmall int
	// ShoppersTooLarge counts distinct shoppers pointing large.
	ShoppersTooLarge int
	// Shoppers counts distinct shoppers with any fit-classified outcome.
	// It is the Building floor and is not the sum of the two directions: a
	// shopper who returned one size each way counts once here.
	Shoppers int
}

// fitOutcomes is the 60% share's denominator: every settled outcome with a
// fit classification, whichever way it points.
func (c PoolCounts) fitOutcomes() int {
	return c.TooSmall + c.TooLarge + c.FitsWell
}

// PoolVerdict is what the pool's rule decides: the ledger label and the
// direction it rates. Direction is FitDirectionNone when no direction
// leads, and recall's pool step does not apply.
type PoolVerdict struct {
	// Status is the pooled label, in the ledger's words.
	Status BeliefStatus
	// Direction is the direction the label rates, or FitDirectionNone.
	Direction FitDirection
}

// DecidePool decides the pooled label for one product line and size from
// the all-time counts and the counts of the last PoolRecentWindow of event
// time. Below PoolBuildingShoppers distinct shoppers the pool says nothing
// and reads Claimed, the ledger word for a belief no evidence stands
// behind. At the floor it reads Building, with the direction ahead on
// outcomes. Once a direction is Earned, the recent window is judged on its
// own: earning the opposite direction, or failing the same test, marks the
// pool Out of true while the earned direction is kept. A window below the
// floor has nothing to say, so a line that has gone quiet keeps its label.
func DecidePool(all, recent PoolCounts) PoolVerdict {
	if all.Shoppers < PoolBuildingShoppers {
		return PoolVerdict{Status: StatusClaimed}
	}
	direction, earned := earnedDirection(all)
	if !earned {
		return PoolVerdict{Status: StatusBuilding, Direction: leadingDirection(all)}
	}
	if recent.Shoppers >= PoolBuildingShoppers {
		if recentDirection, ok := earnedDirection(recent); !ok || recentDirection != direction {
			return PoolVerdict{Status: StatusOutOfTrue, Direction: direction}
		}
	}
	return PoolVerdict{Status: StatusEarned, Direction: direction}
}

// earnedDirection reports which direction the counts earn, if either.
func earnedDirection(c PoolCounts) (FitDirection, bool) {
	if earns(c, FitDirectionSmall) {
		return FitDirectionSmall, true
	}
	if earns(c, FitDirectionLarge) {
		return FitDirectionLarge, true
	}
	return FitDirectionNone, false
}

// earns reports whether one direction meets the Earned rule on these
// counts: enough distinct shoppers, the share of fit evidence, and twice
// the opposite direction.
func earns(c PoolCounts, direction FitDirection) bool {
	outcomes, opposite, shoppers := c.TooSmall, c.TooLarge, c.ShoppersTooSmall
	if direction == FitDirectionLarge {
		outcomes, opposite, shoppers = c.TooLarge, c.TooSmall, c.ShoppersTooLarge
	}
	fit := c.fitOutcomes()
	if fit == 0 {
		return false
	}
	return shoppers >= PoolEarnedShoppers &&
		outcomes*100 >= fit*PoolEarnedSharePercent &&
		outcomes >= PoolEarnedRatio*opposite
}

// leadingDirection names the direction ahead on outcomes, or none when the
// two are level. It is only a label for a Building pool, never a claim the
// evidence has earned.
func leadingDirection(c PoolCounts) FitDirection {
	switch {
	case c.TooSmall > c.TooLarge:
		return FitDirectionSmall
	case c.TooLarge > c.TooSmall:
		return FitDirectionLarge
	default:
		return FitDirectionNone
	}
}
