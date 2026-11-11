package round2

import (
	"errors"
	"fmt"

	"github.com/nrynss/memento-emptor/seed"
)

// Arm is one round-2 comparison condition. Arms differ only in the memory
// the agent is given, never in the shopper, the candidate, the base prompt
// or the model.
type Arm int

// The four round-2 arms, numbered as product.md's table numbers them.
const (
	// ArmNoMemory gives the agent nothing about past purchases.
	ArmNoMemory Arm = 1
	// ArmPayPalHistory gives the agent exactly what the Agent Toolkit
	// returns for this shopper and nothing the layer produced (T10.1).
	ArmPayPalHistory Arm = 2
	// ArmMementoShopperOnly gives the shopper's own beliefs, evidence,
	// labels and corrections, with no pool.
	ArmMementoShopperOnly Arm = 3
	// ArmMementoFull gives the same plus the pooled product fit and the
	// precedence rule that resolves them.
	ArmMementoFull Arm = 4
)

// Arms lists the four arms in product.md's order.
func Arms() []Arm {
	return []Arm{ArmNoMemory, ArmPayPalHistory, ArmMementoShopperOnly, ArmMementoFull}
}

// String names the arm for records and reports, in product.md's words.
func (a Arm) String() string {
	switch a {
	case ArmNoMemory:
		return "no-memory"
	case ArmPayPalHistory:
		return "paypal-history"
	case ArmMementoShopperOnly:
		return "memento-shopper-only"
	case ArmMementoFull:
		return "memento-full"
	default:
		return "unknown"
	}
}

// ErrUnknownArm is returned when a run names an arm outside the four
// round-2 conditions.
var ErrUnknownArm = errors.New("round2: unknown arm")

// memoryTool names the fetch one arm makes to build its memory block.
func (a Arm) memoryTool() string {
	switch a {
	case ArmPayPalHistory:
		return "paypal_history"
	case ArmMementoShopperOnly, ArmMementoFull:
		return "recall_for_purchase"
	default:
		return "none"
	}
}

// Case is one of the four round-2 cases: what product.md says it tests,
// why it separates the arms, and the shoppers that serve it.
type Case struct {
	// ID is the case id the shoppers, answers and run files carry.
	ID string
	// Title is product.md's name for the case.
	Title string
	// Tests is what the case is there to test.
	Tests string
	// Separates states, before any run, why the four arms' inputs differ
	// on this case and which answer each input supports or withholds.
	Separates string
	// Shoppers are the shopper ids this case runs, exactly two.
	Shoppers []string
}

// Cases returns the four cases, in product.md's order, each with its
// separation statement and its two shoppers.
func Cases() []Case {
	return []Case{
		{
			ID:       seed.Round2CaseNewToBrand,
			Title:    "New to the brand",
			Tests:    "No personal history with the candidate brand; the pool knows it runs off-size.",
			Shoppers: []string{"r2-new-plain", "r2-new-otherbrand"},
			Separates: "Both shoppers have no rows at all on the candidate line, so arm 1 (nothing), " +
				"arm 2 (PayPal outcomes with no reasons, and no candidate-line history) and arm 3 " +
				"(the shopper's own evidence for the candidate brand, which is empty) all hold no answer. " +
				"Only arm 4's pool says the line runs small, so only arm 4's input holds one rung up from " +
				"the usual size. The second shopper also carries another brand's large cut: an input that " +
				"maps sizes across brands answers down and is wrong, so the case separates own-evidence-only " +
				"arms from the pooled one and cross-brand guessing from the rule.",
		},
		{
			ID:       seed.Round2CaseUnlikeCrowd,
			Title:    "Unlike the crowd",
			Tests:    "The shopper's own evidence disagrees with the pool, and the right answer is theirs.",
			Shoppers: []string{"r2-crowd-rival", "r2-crowd-plain"},
			Separates: "The pool is Earned small at size M on the candidate line, so arm 4's pool step " +
				"alone points one rung up. Both shoppers' own fit evidence points large, and the own-evidence " +
				"step outranks the pool, so arms 3 and 4 hold S while a pool-only answer is L and wrong. " +
				"Arm 2 holds nothing: the too-large returns sit beside non-fit refunds at the same size and " +
				"the toolkit returns no refund reason, so its input cannot say which returns were about fit. " +
				"Arm 1 holds nothing. The first shopper's answer also depends on the rival-birth threshold: " +
				"below three consecutive opposites the same history would keep its earlier small belief.",
		},
		{
			ID:       seed.Round2CaseNoisyRefunds,
			Title:    "Noisy refunds",
			Tests:    "Mixed causes where a refund status without a reason cannot tell fit from the rest.",
			Shoppers: []string{"r2-noisy-up", "r2-noisy-down"},
			Separates: "Each history carries three fit returns of one direction among damage, non-arrival, " +
				"changed-mind, vague and unexplained refunds at the same size. Arm 2 sees the same refund " +
				"rows for every cause, so its input holds no answer; a reader that treats a refund's status " +
				"as a fit signal sizes the wrong way. Arms 3 and 4 hold the answer from the three classified " +
				"returns, and their pool is Claimed for these lines, so the case separates reasoned memory " +
				"from raw outcomes rather than the pool from the shopper. One shopper sizes up and one down, " +
				"so an input that only ever adjusts one way cannot pass.",
		},
		{
			ID:       seed.Round2CaseCorrection,
			Title:    "Correction, then another agent",
			Tests:    "A correction made to agent A; agent B asks next, with PayPal access but not A's conversation.",
			Shoppers: []string{"r2-corr-belief", "r2-corr-standing"},
			Separates: "The correction travels through the layer, not through a conversation, so arms 3 and " +
				"4 hold the corrected size for an agent that never saw agent A's turn. Arm 2 holds no answer: " +
				"the correction appears in no PayPal field, so an agent with the transaction history alone " +
				"cannot know it, and arm 1 holds nothing. The first shopper's belief is earned and stands " +
				"contradicted beside the correction rather than deleted; the second has no belief, so the " +
				"correction stands on its own and still decides.",
		},
	}
}

// HoldExpectation declares, before any run, whether one arm's input holds
// one shopper's answer: on the first recommendation, and after the
// correction. The fairness test proves a perfect reasoner given that
// input gets exactly this, so an arm is never asked for an answer its
// input cannot carry.
type HoldExpectation struct {
	// ShopperID names the shopper.
	ShopperID string
	// First maps each arm to whether its input holds the first answer.
	First map[Arm]bool
	// After maps each arm to whether its input holds the answer after the
	// correction. It is empty for shoppers with no correction.
	After map[Arm]bool
	// Missing names, per arm, the fact the input lacks when it holds no
	// answer. It is empty where the input holds the answer.
	Missing map[Arm]string
}

// Expectations returns the frozen hold table.
func Expectations() []HoldExpectation {
	newToBrand := func(shopper string) HoldExpectation {
		return HoldExpectation{
			ShopperID: shopper,
			First: map[Arm]bool{
				ArmNoMemory:           false,
				ArmPayPalHistory:      false,
				ArmMementoShopperOnly: false,
				ArmMementoFull:        true,
			},
			Missing: map[Arm]string{
				ArmNoMemory:           "no memory at all",
				ArmPayPalHistory:      "no history with the candidate brand at all, and no refund reason anywhere",
				ArmMementoShopperOnly: "only the shopper's general statement, which is their usual size and not the answer; no history with the candidate brand and no pool in this arm",
				ArmMementoFull:        "",
			},
		}
	}
	ownEvidence := func(shopper string) HoldExpectation {
		return HoldExpectation{
			ShopperID: shopper,
			First: map[Arm]bool{
				ArmNoMemory:           false,
				ArmPayPalHistory:      false,
				ArmMementoShopperOnly: true,
				ArmMementoFull:        true,
			},
		}
	}
	corrected := func(shopper string) HoldExpectation {
		expectation := ownEvidence(shopper)
		expectation.After = map[Arm]bool{
			ArmNoMemory:           false,
			ArmPayPalHistory:      false,
			ArmMementoShopperOnly: true,
			ArmMementoFull:        true,
		}
		return expectation
	}
	return []HoldExpectation{
		newToBrand("r2-new-plain"),
		newToBrand("r2-new-otherbrand"),
		ownEvidence("r2-crowd-rival"),
		ownEvidence("r2-crowd-plain"),
		ownEvidence("r2-noisy-up"),
		ownEvidence("r2-noisy-down"),
		corrected("r2-corr-belief"),
		corrected("r2-corr-standing"),
	}
}

// ExpectationFor returns the frozen hold table row for one shopper.
func ExpectationFor(shopperID string) (HoldExpectation, error) {
	for _, expectation := range Expectations() {
		if expectation.ShopperID == shopperID {
			return expectation, nil
		}
	}
	return HoldExpectation{}, fmt.Errorf("%w: %q", ErrNoShopper, shopperID)
}
