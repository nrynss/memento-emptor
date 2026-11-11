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

// The frozen missing-fact texts. The reference reasoner returns one of
// these when an arm's input holds no answer, and the hold table in cases.go
// names the same string, so a change to either fails TestFairness rather
// than drifting apart. The freeze quotes them in FROZEN.md as the reason
// each arm is not expected to answer.
const (
	// MissingNoMemory is arm 1's reason: it carries nothing.
	MissingNoMemory = "no memory at all"
	// MissingNoBrandHistory is arm 2's reason on a brand the shopper has
	// never bought: the toolkit holds no purchase on that line.
	MissingNoBrandHistory = "no history with the candidate brand at all: the toolkit holds no purchase on that line"
	// MissingNoCause is arm 2's reason where every candidate-line purchase
	// came back: the returns at one size mix fit with causes the toolkit
	// never returns, and no purchase stayed.
	MissingNoCause = "no refund reason: the returns at one size mix fit with damage, non-arrival, changed mind and no reason at all, and no purchase on the line stayed"
	// MissingKeptTie is arm 2's reason where more than one unreturned
	// purchase sits on the candidate line in different sizes, so the kept
	// count cannot choose between them.
	MissingKeptTie = "more than one unreturned purchase on the candidate line, in different sizes, and nothing in the records chooses between them: PayPal carries no refund reason, and the records carry no return window"
	// MissingKeptNotCorrected is the hold table's reason for arm 2 after a
	// correction: the records' one positive signal is a kept purchase, and
	// the correction appears in no PayPal field.
	MissingKeptNotCorrected = "the correction appears in no PayPal field: the records' only positive signal is the kept purchase it replaces"
	// MissingNoRecord is arm 2's reason when the toolkit holds nothing.
	MissingNoRecord = "no history at all"
	// MissingStatementOnly is the shopper-only arm's reason on a brand the
	// shopper has never bought: the statement is memory, and it answers the
	// usual size rather than the right one.
	MissingStatementOnly = "only the shopper's general statement, which is their usual size and not the answer; no history with the candidate brand and no pool in this arm"
	// MissingNothing is the reason when no step has anything to say.
	MissingNothing = "no fit evidence, no pool and no statement: the input says nothing about the size"
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
		return Decision{Hold: false, Rung: RungNone, Missing: MissingNoMemory}, nil
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
		Missing: MissingNothing,
	}, nil
}

// decidePayPal reads arm 2's input as the agent receives it: the toolkit's
// records, with sizes, statuses and PayPal's dispute reason codes but no
// refund reason. A refund's status cannot say whether the return was about
// fit, so a returned purchase carries no direction. A purchase that stayed
// is different: the records show a capture with no refund and no dispute,
// which is the observable the layer settles as SignalKeptPastWindow and
// FitSettledByKept, a reason-free positive signal that the size worn was
// right. The reasoner counts those purchases the way the layer's usual-size
// rule does (domain.UsualSizeFor over the kept tally) and answers the size
// most often kept on the candidate line. Where kept purchases disagree the
// rule has no size, and the decision says so. A cause word appearing in the
// records is a leak, and the decision names it rather than pretending the
// arm is blind.
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
		return Decision{Hold: false, Rung: RungNone, Missing: MissingNoRecord}, nil
	}
	if !strings.Contains(string(input), shopper.Candidate.BrandName) {
		return Decision{
			Hold:    false,
			Rung:    RungNone,
			Missing: MissingNoBrandHistory,
		}, nil
	}
	kept := payPalKeptSizes(shopper, records)
	if len(kept) == 0 {
		return Decision{
			Hold:    false,
			Rung:    RungNone,
			Missing: MissingNoCause,
		}, nil
	}
	usual := domain.UsualSizeFor(kept, "")
	if usual.Size == "" {
		return Decision{
			Hold:    false,
			Rung:    RungNone,
			Missing: MissingKeptTie,
		}, nil
	}
	return Decision{Hold: true, Size: usual.Size, Rung: RungOwnEvidence}, nil
}

// payPalPurchase is one purchase as arm 2's records show it: the size of
// its line item, whether that item is on the candidate line, and whether a
// refund or a dispute came back for its capture.
type payPalPurchase struct {
	size      string
	candidate bool
	returned  bool
}

// payPalPurchases reads every purchase arm 2's records carry. The order
// body names the line item and its size; the transaction listing holds the
// capture and, for a refund, the capture it refers back to; a dispute's
// disputed transaction names the capture it holds. An order is paired with
// its capture by the time both report, which is the only field the two
// share. An order that cannot be tied to exactly one capture is left out:
// without its capture the records cannot show the purchase stayed.
func payPalPurchases(shopper seed.Round2Shopper, records []map[string]any) []payPalPurchase {
	returned := map[string]bool{}
	captures := map[string][]string{}
	for _, record := range records {
		info, ok := record["transaction_info"].(map[string]any)
		if !ok {
			continue
		}
		id := payPalString(info, "transaction_id")
		switch payPalString(info, "transaction_event_code") {
		case "T0006":
			if at := payPalString(info, "transaction_initiation_date"); id != "" && at != "" {
				captures[at] = append(captures[at], id)
			}
		case "T1107":
			if reference := payPalString(info, "paypal_reference_id"); reference != "" {
				returned[reference] = true
			}
		}
	}
	for _, record := range records {
		disputed, ok := record["disputed_transactions"].([]any)
		if !ok {
			continue
		}
		for _, item := range disputed {
			fields, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if id := payPalString(fields, "seller_transaction_id"); id != "" {
				returned[id] = true
			}
		}
	}
	var out []payPalPurchase
	for _, record := range records {
		units, ok := record["purchase_units"].([]any)
		if !ok {
			continue
		}
		ids := captures[payPalString(record, "create_time")]
		if len(ids) != 1 {
			continue
		}
		for _, unit := range units {
			fields, ok := unit.(map[string]any)
			if !ok {
				continue
			}
			items, _ := fields["items"].([]any)
			for _, item := range items {
				line, ok := item.(map[string]any)
				if !ok {
					continue
				}
				out = append(out, payPalPurchase{
					size:      payPalItemSize(line),
					candidate: payPalNamesCandidate(shopper, payPalString(line, "name")),
					returned:  returned[ids[0]],
				})
			}
		}
	}
	return out
}

// payPalNamesCandidate reports whether one line item's name is the
// candidate product. The records carry no brand id, so the name the toolkit
// returns is what the shopper's question names too.
func payPalNamesCandidate(shopper seed.Round2Shopper, name string) bool {
	return strings.Contains(name, shopper.Candidate.BrandName) &&
		strings.Contains(name, shopper.Candidate.LineName)
}

// payPalItemSize reads the size a line item names, or the empty string when
// its description carries none.
func payPalItemSize(line map[string]any) string {
	size, ok := strings.CutPrefix(payPalString(line, "description"), "Size ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(size)
}

// payPalKeptSizes tallies the unreturned purchases on the candidate line by
// size, in the shape the layer's usual-size rule reads: a purchase whose
// capture came back with no refund and no dispute is one kept purchase of
// the size its line item names. The records carry no return window, so this
// is the kept signal's observable, not a claim that each window has closed.
func payPalKeptSizes(shopper seed.Round2Shopper, records []map[string]any) []domain.SizeTally {
	tallies := map[string]*domain.SizeTally{}
	order := []string{}
	for _, purchase := range payPalPurchases(shopper, records) {
		if !purchase.candidate || purchase.returned || purchase.size == "" {
			continue
		}
		tally, ok := tallies[purchase.size]
		if !ok {
			tally = &domain.SizeTally{Size: purchase.size}
			tallies[purchase.size] = tally
			order = append(order, purchase.size)
		}
		tally.Kept++
	}
	out := make([]domain.SizeTally, 0, len(order))
	for _, size := range order {
		out = append(out, *tallies[size])
	}
	return out
}

// payPalString reads one string field from a decoded record, or the empty
// string.
func payPalString(record map[string]any, key string) string {
	value, _ := record[key].(string)
	return value
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
