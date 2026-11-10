package domain

import "strings"

// FitDirection is which way a fit reason or a fit belief points. Small
// means the item ran small and the shopper sizes up; large means it ran
// large and the shopper sizes down. Anything that is not about fit, and
// any fit belief that states no direction, points nowhere.
type FitDirection string

// The fit directions.
const (
	// FitDirectionNone is the zero value: no direction. Non-fit reasons,
	// seller beliefs, general statements and the emerging fits-well
	// belief all point nowhere.
	FitDirectionNone FitDirection = ""
	// FitDirectionSmall means the item ran small: too tight, too short,
	// cannot button. Evidence with this direction counts toward a
	// runs-small belief only.
	FitDirectionSmall FitDirection = "small"
	// FitDirectionLarge means the item ran large: too loose, swims on
	// the shopper. Evidence with this direction counts toward a
	// runs-large belief only.
	FitDirectionLarge FitDirection = "large"
)

// Direction reports which way a reason category points. Only the two fit
// categories point anywhere; every other category points nowhere, so it
// never constrains a match.
func (c Category) Direction() FitDirection {
	switch c {
	case CategoryFitTooSmall:
		return FitDirectionSmall
	case CategoryFitTooLarge:
		return FitDirectionLarge
	default:
		return FitDirectionNone
	}
}

// DirectionForValue reports which way a belief value points. It recognizes
// the two values the layer composes for new fit beliefs: a runs-small
// value points small and a runs-large value points large. Every other
// value — seller beliefs, general statements, the emerging fits-well
// belief — points nowhere.
func DirectionForValue(value string) FitDirection {
	lowered := strings.ToLower(value)
	switch {
	case strings.Contains(lowered, "runs small"):
		return FitDirectionSmall
	case strings.Contains(lowered, "runs large"):
		return FitDirectionLarge
	default:
		return FitDirectionNone
	}
}
