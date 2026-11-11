package round2

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/seed"
)

// Rung names the precedence step that decided a size, in product.md's
// order. It is the same vocabulary the recall contract's guidance basis
// uses.
type Rung string

// The precedence rungs, strongest first.
const (
	// RungCorrection is the shopper's correction in force.
	RungCorrection Rung = "correction"
	// RungOwnEvidence is the shopper's own fit evidence for the brand.
	RungOwnEvidence Rung = "own_evidence"
	// RungPool is the pooled product fit, adjusted from the usual size.
	RungPool Rung = "pool"
	// RungStatement is the shopper's general statement, labelled Claimed.
	RungStatement Rung = "statement"
	// RungNone means nothing decided the size.
	RungNone Rung = "none"
)

// Decision is what a perfect reasoner answers from one arm's input, and
// what it cannot answer without.
type Decision struct {
	// Hold is true when the input holds a size to recommend.
	Hold bool
	// Size is the size the input supports, or empty when it holds none.
	Size string
	// Rung is the precedence step that decided it.
	Rung Rung
	// Missing names the fact the input lacks when it holds no size.
	Missing string
}

// sizeLadder orders apparel sizes for recommendations, and
// neighbourSize moves one rung. It is the layer's own ladder and shift,
// restated here because the round-2 reference rule must be able to run
// offline; a test pins the two sizes every round-2 answer lands on.
var sizeLadder = []string{"XXS", "XS", "S", "M", "L", "XL", "XXL"}

// neighbourSize moves one rung on the ladder in the given direction. An
// unknown size or the ladder's edge stays where it is.
func neighbourSize(size string, up bool) string {
	for i, rung := range sizeLadder {
		if !strings.EqualFold(rung, size) {
			continue
		}
		if up && i+1 < len(sizeLadder) {
			return sizeLadder[i+1]
		}
		if !up && i > 0 {
			return sizeLadder[i-1]
		}
		return sizeLadder[i]
	}
	return size
}

// shiftSize adjusts a size one rung the way a direction says.
func shiftSize(size string, direction domain.FitDirection) string {
	switch direction {
	case domain.FitDirectionSmall:
		return neighbourSize(size, true)
	case domain.FitDirectionLarge:
		return neighbourSize(size, false)
	default:
		return size
	}
}

// SizeForDirection returns the size one rung the way a direction says, or
// the size unchanged when the direction points nowhere. It is exported so
// the grader and the expected recall fixture shift sizes by one rule.
func SizeForDirection(size string, direction domain.FitDirection) string {
	return shiftSize(size, direction)
}

// ownBelief is one belief the shopper's own fit returns form, as the
// layer forms it.
type ownBelief struct {
	direction domain.FitDirection
	size      string
	evidence  int
	lastDay   float64
	days      []float64
	rival     bool
}

// ownBeliefs walks the shopper's own fit returns for the candidate brand
// under the layer's rule and returns the beliefs it forms, strongest
// first. The rule, mirroring the service's answer path:
//
//   - the first classified fit return starts a belief in its direction,
//     whose value names one rung off the returned size;
//   - a later return pointing a belief's way attaches to it and breaks
//     the opposite-direction run;
//   - an opposite-direction return attaches to nothing while the run is
//     below domain.RivalBirthThreshold, and births a same-kind
//     opposite-direction belief when the run reaches it;
//   - a seller-classified answer never reaches this path, and a
//     directionless answer (changed mind, vague) lands unattached and
//     moves no run.
func ownBeliefs(shopper seed.Round2Shopper) []ownBelief {
	beliefs := []*ownBelief{}
	var streak domain.OppositeStreak
	find := func(direction domain.FitDirection) *ownBelief {
		var best *ownBelief
		for _, belief := range beliefs {
			if belief.direction == direction && (best == nil || belief.evidence > best.evidence) {
				best = belief
			}
		}
		return best
	}
	for _, row := range shopper.History.Rows {
		if row.Item.BrandID != shopper.Candidate.BrandID || row.Signal != domain.SignalClassifiedOutcome || row.Reason == nil {
			continue
		}
		category := row.Reason.Category
		if category.CountsToward() != domain.BeliefKindFit {
			continue
		}
		want := category.Direction()
		if want == domain.FitDirectionNone {
			continue
		}
		if belief := find(want); belief != nil {
			belief.evidence++
			belief.lastDay = row.Day
			belief.days = append(belief.days, row.Day)
			streak.NoteMatched()
			continue
		}
		if len(beliefs) == 0 {
			beliefs = append(beliefs, &ownBelief{
				direction: want,
				size:      shiftSize(row.Item.Size, want),
				evidence:  1,
				lastDay:   row.Day,
				days:      []float64{row.Day},
			})
			continue
		}
		if streak.NoteOpposite() {
			beliefs = append(beliefs, &ownBelief{
				direction: want,
				size:      shiftSize(row.Item.Size, want),
				evidence:  1,
				lastDay:   row.Day,
				days:      []float64{row.Day},
				rival:     true,
			})
		}
	}
	out := make([]ownBelief, 0, len(beliefs))
	for _, belief := range beliefs {
		out = append(out, *belief)
	}
	// Strongest first: most evidence, then the newer belief.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			if out[j].evidence > out[j-1].evidence ||
				(out[j].evidence == out[j-1].evidence && out[j].lastDay > out[j-1].lastDay) {
				out[j], out[j-1] = out[j-1], out[j]
				continue
			}
			break
		}
	}
	return out
}

// Decide runs the frozen reference reasoner for one arm and one shopper on
// the first recommendation, before any correction: the precedence order
// product.md fixes, over exactly the memory that arm's input carries. A
// shopper's own evidence outranks the pool, the pool outranks a general
// statement. The answer comes from the frozen history and population, never
// from the hidden answer.
func Decide(shopper seed.Round2Shopper, arm Arm, population seed.Population) (Decision, error) {
	return decide(shopper, arm, population, false)
}

// DecideAfterCorrection runs the same reasoner with the shopper's
// correction in force. Only the Memento arms carry a correction: it travels
// through the layer, so an agent that never saw the conversation still
// gets it, while a PayPal-only or memoryless input cannot.
func DecideAfterCorrection(shopper seed.Round2Shopper, arm Arm, population seed.Population) (Decision, error) {
	return decide(shopper, arm, population, true)
}

// decide is the precedence order over one arm's input. corrected says
// whether the shopper's correction has been recorded yet.
func decide(shopper seed.Round2Shopper, arm Arm, population seed.Population, corrected bool) (Decision, error) {
	if shopper.ShopperID == "" {
		return Decision{}, fmt.Errorf("%w: no shopper id", ErrNoShopper)
	}
	switch arm {
	case ArmNoMemory:
		return Decision{Hold: false, Rung: RungNone, Missing: "no memory at all"}, nil
	case ArmPayPalHistory:
		return decidePayPal(shopper)
	case ArmMementoShopperOnly, ArmMementoFull:
	default:
		return Decision{}, fmt.Errorf("%w: %d", ErrUnknownArm, int(arm))
	}
	if corrected && shopper.Correction != nil {
		return Decision{Hold: true, Size: shopper.Correction.WantSize, Rung: RungCorrection}, nil
	}
	if beliefs := ownBeliefs(shopper); len(beliefs) > 0 {
		return Decision{Hold: true, Size: beliefs[0].size, Rung: RungOwnEvidence}, nil
	}
	if feedback := FeedbackRows(shopper); len(feedback) > 0 {
		return Decision{Hold: true, Size: feedback[len(feedback)-1].Size, Rung: RungOwnEvidence}, nil
	}
	if arm == ArmMementoFull {
		pool, err := PoolFor(shopper, population)
		if err != nil {
			return Decision{}, err
		}
		if pool.Status != domain.StatusClaimed && pool.Direction != domain.FitDirectionNone && pool.Size != "" {
			return Decision{Hold: true, Size: shiftSize(pool.Size, pool.Direction), Rung: RungPool}, nil
		}
	}
	if stated := StatementSize(shopper); stated != "" {
		return Decision{Hold: true, Size: stated, Rung: RungStatement}, nil
	}
	return Decision{
		Hold:    false,
		Rung:    RungNone,
		Missing: "no fit evidence, no pool and no statement: the input says nothing about the size",
	}, nil
}

// decidePayPal reads arm 2's input as the agent receives it: the toolkit's
// records, with sizes, statuses and PayPal's dispute reason codes but no
// refund reason. It holds no size: a refund's status cannot say whether the
// return was about fit, and every size that carries a fit return carries
// non-fit returns too. A cause word appearing in the records is a leak, and
// the decision names it rather than pretending the arm is blind.
func decidePayPal(shopper seed.Round2Shopper) (Decision, error) {
	input, err := ArmTwoInput(shopper)
	if err != nil {
		return Decision{}, err
	}
	if leaked := fitWordsIn(string(input)); len(leaked) > 0 {
		return Decision{
			Hold:    false,
			Rung:    RungNone,
			Missing: "a fit cause leaked into the toolkit records: " + strings.Join(leaked, ", "),
		}, nil
	}
	if shopper.Correction != nil && strings.Contains(string(input), shopper.Correction.Text) {
		return Decision{Hold: false, Rung: RungNone, Missing: "the correction leaked into the toolkit records"}, nil
	}
	var records []map[string]any
	if err := json.Unmarshal(input, &records); err != nil {
		return Decision{}, fmt.Errorf("round2: decode arm 2 input for %q: %w", shopper.ShopperID, err)
	}
	if len(records) == 0 {
		return Decision{Hold: false, Rung: RungNone, Missing: "no history at all"}, nil
	}
	if !strings.Contains(string(input), shopper.Candidate.BrandName) {
		return Decision{
			Hold:    false,
			Rung:    RungNone,
			Missing: "no history with the candidate brand at all: the toolkit holds no purchase on that line",
		}, nil
	}
	return Decision{
		Hold:    false,
		Rung:    RungNone,
		Missing: "no refund reason: the returns at one size mix fit with damage, non-arrival, changed mind and no reason at all",
	}, nil
}

// fitWords are the words that would hand arm 2 a cause. The extraction
// must never carry one.
var fitWords = []string{
	"too tight", "too small", "too large", "too loose", "too short", "too long",
	"fits well", "fit return", "runs small", "runs large", "swims", "shoulders",
	"chest", "sleeves", "hem", "seam", "fit_too_small", "fit_too_large",
	"damaged_or_defective", "not_arrived", "changed_mind", "unclear",
}

// fitWordsIn reports which cause words a text carries.
func fitWordsIn(text string) []string {
	lowered := strings.ToLower(text)
	var found []string
	for _, word := range fitWords {
		if strings.Contains(lowered, word) {
			found = append(found, word)
		}
	}
	return found
}

// DecidePoolOnly answers from the pool alone, ignoring the shopper's own
// evidence. It is the naive-pooling comparison the unlike-the-crowd case
// exists to separate: where it differs from Decide, the precedence rule is
// what produced the right answer.
func DecidePoolOnly(shopper seed.Round2Shopper, population seed.Population) (Decision, error) {
	pool, err := PoolFor(shopper, population)
	if err != nil {
		return Decision{}, err
	}
	if pool.Status != domain.StatusClaimed && pool.Direction != domain.FitDirectionNone && pool.Size != "" {
		return Decision{Hold: true, Size: shiftSize(pool.Size, pool.Direction), Rung: RungPool}, nil
	}
	return Decision{Hold: false, Rung: RungNone, Missing: "the pool says nothing"}, nil
}
