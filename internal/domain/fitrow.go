package domain

import "time"

// ProductLine names the product line a fit row is about: the brand, the
// category and the fit handle. All three are present on every row, and a
// query for a line filters all three.
type ProductLine struct {
	// Brand is the brand the line belongs to.
	Brand string
	// Category is the product category slug, such as shirts.
	Category string
	// FitHandle is the line's cut, such as regular or slim.
	FitHandle string
}

// FitPurchaseState is where a purchase stands in its life. The state's
// rank is the write version, so a later state always wins and a late
// webhook can never undo it.
type FitPurchaseState string

// The purchase states a fit row moves through.
const (
	// FitStateCaptured means the purchase was captured.
	FitStateCaptured FitPurchaseState = "captured"
	// FitStateRefunded means the purchase was refunded, with no reason yet.
	FitStateRefunded FitPurchaseState = "refunded"
	// FitStateReasonClassified means the refund's reason is classified.
	FitStateReasonClassified FitPurchaseState = "reason_classified"
	// FitStateFeedback means fit feedback settled the purchase.
	FitStateFeedback FitPurchaseState = "feedback"
	// FitStateKept means the purchase was kept past the return window.
	FitStateKept FitPurchaseState = "kept"
	// FitStateContested means the shopper contested the purchase's reason;
	// the corrected reason and direction replace the old values.
	FitStateContested FitPurchaseState = "contested"
)

// Rank is the state's place in the version order: captured, then
// refunded, then reason classified, then settled or contested. Event time
// breaks ties inside a rank.
func (s FitPurchaseState) Rank() int {
	switch s {
	case FitStateCaptured:
		return 1
	case FitStateRefunded:
		return 2
	case FitStateReasonClassified:
		return 3
	case FitStateFeedback, FitStateKept, FitStateContested:
		return 4
	default:
		return 0
	}
}

// FitSettledKind names what settled a purchase. Only settled purchases
// count toward the pool.
type FitSettledKind string

// What settles a fit row.
const (
	// FitSettledByReturnReason is a return with a classified reason.
	FitSettledByReturnReason FitSettledKind = "return_reason"
	// FitSettledByFeedback is fit feedback tied to the purchase.
	FitSettledByFeedback FitSettledKind = "feedback"
	// FitSettledByKept is a purchase kept past the return window.
	FitSettledByKept FitSettledKind = "kept"
)

// FitVerdict is how fit feedback read the purchased size, as a fit row
// records it. The wire's feedback verdicts carry the same three values.
type FitVerdict string

// The fit verdicts a pooled row can carry.
const (
	// FitVerdictFitsWell means the size fits.
	FitVerdictFitsWell FitVerdict = "fits_well"
	// FitVerdictTooSmall means the size was too small or too tight.
	FitVerdictTooSmall FitVerdict = "too_small"
	// FitVerdictTooLarge means the size was too big or too loose.
	FitVerdictTooLarge FitVerdict = "too_large"
)

// Direction reports which way a verdict points. A fits-well verdict points
// nowhere: it says the size was right, not that it ran off.
func (v FitVerdict) Direction() FitDirection {
	switch v {
	case FitVerdictTooSmall:
		return FitDirectionSmall
	case FitVerdictTooLarge:
		return FitDirectionLarge
	default:
		return FitDirectionNone
	}
}

// FitRow is one purchase's row in the fit index. The pooled product-fit
// belief is an aggregation over these rows and nothing else, so a row
// carries no shopper identity, no order detail and no free text. The index
// stores the direction as too_small or too_large; the domain's direction
// words are small and large.
type FitRow struct {
	// Line is the product line: brand, category and fit handle.
	Line ProductLine
	// Size is the purchased size, on the product's own label.
	Size string
	// State is where the purchase stands now.
	State FitPurchaseState
	// Reason is the classified reason, or CategoryNone while none is known.
	Reason Category
	// Direction is which way the reason or verdict points, or
	// FitDirectionNone when it points nowhere.
	Direction FitDirection
	// Verdict is the fit feedback's reading, or empty when none arrived.
	Verdict FitVerdict
	// Settled is true once the purchase counts toward the pool: returned
	// with a classified reason, fit feedback, or kept past the window.
	Settled bool
	// SettledKind names what settled it, or empty while unsettled.
	SettledKind FitSettledKind
	// ReasonContested is true once the shopper contests this purchase's
	// reason; the corrected category and direction replace the old values.
	ReasonContested bool
	// ShopperHash is the salted per-line shopper hash: it counts distinct
	// shoppers within one line and cannot link one shopper across lines.
	ShopperHash string
	// TxnHash is the transaction hash and the row's id.
	TxnHash string
	// Seeded marks generated history. Every seeded row carries the mark.
	Seeded bool
	// CapturedAt is when the purchase was captured, or the zero time.
	CapturedAt time.Time
	// RefundedAt is when the purchase was refunded, or the zero time.
	RefundedAt time.Time
	// SettledAt is when the purchase settled, or the zero time.
	SettledAt time.Time
	// EventAt is the event time of the write this row carries. It breaks
	// ties inside a state rank.
	EventAt time.Time
}

// fitRankStride is the width of one state rank in the version. It is wider
// than any event time's Unix seconds, so version order is state rank first
// and event time second.
const fitRankStride int64 = 1_000_000_000_000

// Version is the row's write version: the purchase state's rank, with the
// event time breaking ties inside a rank. A higher state always wins, a
// late event inside one rank cannot overwrite a newer one, and a retry of
// the same event carries the same version so it rewrites the same row.
func (r FitRow) Version() int64 {
	return int64(r.State.Rank())*fitRankStride + r.EventAt.Unix()
}
