package domain

import (
	"errors"
	"fmt"
)

// Category is a classified reason for an outcome. The set is exclusive:
// one shopper answer maps to exactly one category, and a vague answer
// maps to Unclear so the agent asks again instead of guessing.
type Category string

// The reason categories.
const (
	// CategoryFitTooSmall means too small or too tight: tight shoulders,
	// short sleeves, cannot button. It counts toward fit.
	CategoryFitTooSmall Category = "fit_too_small"
	// CategoryFitTooLarge means too big or too loose: swims on the
	// shopper, runs large. It counts toward fit.
	CategoryFitTooLarge Category = "fit_too_large"
	// CategoryDamagedOrDefective means the goods were wrong when they
	// arrived: broken, not working, or materially unlike the description.
	// It counts toward seller reliability, never toward sizing.
	CategoryDamagedOrDefective Category = "damaged_or_defective"
	// CategoryNotArrived means the item never arrived or delivery failed.
	// It counts toward seller reliability, never toward sizing.
	CategoryNotArrived Category = "not_arrived"
	// CategoryChangedMind means the shopper changed their mind with no
	// product fault. It counts toward nothing.
	CategoryChangedMind Category = "changed_mind"
	// CategoryUnclear means no usable reason: vague, ambiguous or
	// off-topic. It counts toward nothing and the agent asks again.
	CategoryUnclear Category = "unclear"
	// CategoryNone is the zero value: no classified reason. Evidence with
	// this category counts toward nothing.
	CategoryNone Category = "none"
)

// Categories lists every classified category: the six from the reason
// classifier plus the none value for unclassified outcomes.
var Categories = []Category{
	CategoryFitTooSmall,
	CategoryFitTooLarge,
	CategoryDamagedOrDefective,
	CategoryNotArrived,
	CategoryChangedMind,
	CategoryUnclear,
	CategoryNone,
}

// CountsToward reports which belief kind evidence with this category
// counts toward. Fit reasons count toward fit. Non-arrival and damage
// count toward seller reliability. A reason matching neither counts
// toward nothing.
func (c Category) CountsToward() BeliefKind {
	switch c {
	case CategoryFitTooSmall, CategoryFitTooLarge:
		return BeliefKindFit
	case CategoryDamagedOrDefective, CategoryNotArrived:
		return BeliefKindSeller
	default:
		return BeliefKindNone
	}
}

// DisputeCode is a dispute reason or dispute-category code from the
// payment evidence source.
type DisputeCode string

// The dispute codes observed in the recorded evidence.
const (
	// DisputeMerchandiseNotReceived is the dispute reason carried
	// top-level and per item on a live non-arrival dispute.
	DisputeMerchandiseNotReceived DisputeCode = "MERCHANDISE_OR_SERVICE_NOT_RECEIVED"
	// DisputeMerchandiseNotAsDescribed is the dispute reason for goods that
	// arrived but were materially unlike the description: the wrong item,
	// the wrong variant, or a different specification. It counts against
	// the seller, in the same class as damage, and never toward sizing.
	DisputeMerchandiseNotAsDescribed DisputeCode = "MERCHANDISE_OR_SERVICE_NOT_AS_DESCRIBED"
	// DisputeItemNotReceived is the seller-protection dispute category
	// carried on capture records.
	DisputeItemNotReceived DisputeCode = "ITEM_NOT_RECEIVED"
	// DisputeUnauthorizedTransaction is the seller-protection dispute
	// category for account-takeover payments. It matches no belief kind:
	// it says the payment was not the shopper's, not that the seller is
	// unreliable, so it counts toward nothing.
	DisputeUnauthorizedTransaction DisputeCode = "UNAUTHORIZED_TRANSACTION"
)

// DisputeCodes lists every dispute code in the fixed table.
var DisputeCodes = []DisputeCode{
	DisputeMerchandiseNotReceived,
	DisputeMerchandiseNotAsDescribed,
	DisputeItemNotReceived,
	DisputeUnauthorizedTransaction,
}

// ErrUnknownDisputeCode is returned when a dispute code is not in the fixed table.
var ErrUnknownDisputeCode = errors.New("domain: unknown dispute code")

// CategoryForDispute maps a dispute code to its reason category. Codes
// outside the table are an error and count toward nothing: no code path
// may turn a dispute into a belief without a matching reason. A caller
// that must keep the outcome takes StoredCategoryForDispute, which stores
// it with no classified reason instead of dropping it.
//
// The not-as-described code files under the damage category: the goods
// arrived and were wrong, which is the seller signal damage already
// carries, and the reason-counts shape the ledger and merchant app share
// has no field of its own for it.
func CategoryForDispute(code DisputeCode) (Category, error) {
	switch code {
	case DisputeMerchandiseNotReceived, DisputeItemNotReceived:
		return CategoryNotArrived, nil
	case DisputeMerchandiseNotAsDescribed:
		return CategoryDamagedOrDefective, nil
	case DisputeUnauthorizedTransaction:
		return CategoryNone, nil
	default:
		return CategoryNone, fmt.Errorf("%w: %q", ErrUnknownDisputeCode, string(code))
	}
}

// StoredCategoryForDispute maps a dispute code to the category stored with
// it. A code outside the fixed table is not an error here: the outcome
// exists, so it is stored with no classified reason, which is CategoryNone,
// and counts toward nothing instead of being dropped. Any other mapping
// error is returned.
//
// This is the one place the unknown-code disposition is decided. Served
// ingest and the live driver both take it from here, so a change to the
// disposition cannot reach one and miss the other.
func StoredCategoryForDispute(code DisputeCode) (Category, error) {
	category, err := CategoryForDispute(code)
	switch {
	case err == nil:
		return category, nil
	case errors.Is(err, ErrUnknownDisputeCode):
		return CategoryNone, nil
	default:
		return CategoryNone, err
	}
}
