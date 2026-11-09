package domain

import "time"

// SeedMark is the ledger mark carried by every seeded row. Seeded history
// is generated, not lived, and the ledger shows the mark beside it.
const SeedMark = "seeded"

// Source names where a piece of evidence came from.
type Source string

// The sources evidence arrives from.
const (
	// SourcePayment is the payment evidence source: refunds, disputes and
	// captures arrive without the agent doing anything.
	SourcePayment Source = "payment"
	// SourceShopper is the shopper's own words, reported through the agent:
	// answers, feedback, corrections and general statements.
	SourceShopper Source = "shopper"
)

// OutcomeKind names what happened to the money.
type OutcomeKind string

// The outcomes the layer accepts as evidence.
const (
	// OutcomeRefund is a refunded capture. It arrives as an outcome without
	// a reason; the reason comes from the shopper when asked.
	OutcomeRefund OutcomeKind = "refund"
	// OutcomeDispute is a dispute filed against a transaction. It carries
	// the evidence source's reason code.
	OutcomeDispute OutcomeKind = "dispute"
	// OutcomeCapture is a completed capture. On its own it proves the
	// purchase happened; it proves nothing about fit or the seller.
	OutcomeCapture OutcomeKind = "capture"
	// OutcomeKept means the item was kept past the return window with no
	// feedback. It is a weak, inferred signal, never satisfaction.
	OutcomeKept OutcomeKind = "kept"
)

// Outcome is what happened to the money: the payment side of evidence,
// kept separate from the item and the reason.
type Outcome struct {
	// Kind is what happened: refund, dispute, capture or kept.
	Kind OutcomeKind
	// Status is the source's status for the outcome, such as COMPLETED or OPEN.
	Status string
	// AmountMinor is the amount in minor units, such as cents.
	AmountMinor int64
	// Currency is the ISO currency code, such as USD.
	Currency string
}

// Item is the purchased thing the evidence is about: the order's line
// item joined to the catalog's variant data. Brand and line identify the
// cut; size identifies the variant the shopper actually wore.
type Item struct {
	// BrandID identifies the brand. Fit beliefs are keyed per brand.
	BrandID string
	// BrandName is the brand's display name, such as Brand X.
	BrandName string
	// LineName is the product line, such as the oxford shirt line.
	LineName string
	// SKU is the retailer's stock-keeping unit for the purchased variant.
	SKU string
	// Size is the purchased size, such as M.
	Size string
	// Category is the product category slug, such as dress-shirts.
	Category string
}

// Reason is why the outcome happened: the evidence source's dispute code
// translated to a category, or the shopper's classified answer. A nil
// reason means the outcome has no known cause yet and counts toward
// nothing.
type Reason struct {
	// Category is the classified reason.
	Category Category
	// Text is the shopper's words, or the dispute code for a dispute.
	Text string
	// Confidence is the classifier's confidence in the category, from 0 to 1.
	Confidence float64
}

// TransactionLink ties an outcome to its transaction across reads. The
// custom ID propagates from order to capture to refund and dispute, so it
// is the join key; the event ID keys webhook idempotency.
type TransactionLink struct {
	// OrderID is the checkout order id.
	OrderID string
	// CaptureID is the capture id the refund or dispute refers to.
	CaptureID string
	// RefundID is the refund id, when the outcome is a refund.
	RefundID string
	// DisputeID is the dispute id, when the outcome is a dispute.
	DisputeID string
	// CustomID is the shopper-scoped reference that propagates across the
	// order, capture, refund and dispute records.
	CustomID string
	// EventID is the webhook event id, for idempotent ingest.
	EventID string
}

// Evidence is one unit of memory input: an outcome, the item it concerns,
// the reason if one is known, and the links that join them. The three
// stay separate so no code path can turn a refund or dispute into a
// belief without a matching reason.
type Evidence struct {
	// Source names where the evidence came from.
	Source Source
	// Outcome is what happened to the money.
	Outcome Outcome
	// Item is the purchased thing the evidence is about.
	Item Item
	// Reason is why it happened, or nil when no reason is known yet.
	Reason *Reason
	// Link ties the outcome to its transaction.
	Link TransactionLink
	// Seeded marks generated history. Every seeded row carries the mark.
	Seeded bool
	// OccurredAt is when the evidence happened, in event time, so seeded
	// history counts as spread out.
	OccurredAt time.Time
}

// CountsToward reports which belief kind this evidence counts toward. An
// outcome without a reason, or with a reason matching no belief kind,
// counts toward nothing.
func (e Evidence) CountsToward() BeliefKind {
	if e.Reason == nil {
		return BeliefKindNone
	}
	return e.Reason.Category.CountsToward()
}
