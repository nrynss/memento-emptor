package pool

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// Errors returned when a pool read cannot be answered. Each is a distinct
// condition: a caller that must carry on without the pool distinguishes an
// unreachable index from an index that answered with an error, and both from
// a valid empty pool, which is an answer.
var (
	// ErrMissingIndex means a reader was built without an index.
	ErrMissingIndex = errors.New("pool: missing index")
	// ErrMissingQuery means a line or size field is empty, or the recent
	// window has no start: an empty field would match more than one line
	// or size, and a window with no start would count every settled row.
	ErrMissingQuery = errors.New("pool: incomplete product line, size or window")
	// ErrUnavailable means the index could not be reached: the connection
	// was refused, the request timed out, or the transport failed. Recall
	// carries on without the pool; it must not report an empty pool.
	ErrUnavailable = errors.New("pool: index unavailable")
	// ErrIndexStatus means the index answered with an error status.
	ErrIndexStatus = errors.New("pool: index answered with an error status")
	// ErrIncomplete means the index answered but said its answer is
	// partial: the query timed out or a shard failed. Partial counts are
	// not counts, so the reader refuses them rather than reporting them.
	ErrIncomplete = errors.New("pool: index answer is incomplete")
	// ErrMalformedResponse means the index's answer is not the aggregation
	// the query asked for, so no count can be trusted.
	ErrMalformedResponse = errors.New("pool: index answer is not the expected aggregation")
	// ErrMalformedEndpoint means the index client was given a base URL that
	// is not an absolute http or https URL.
	ErrMalformedEndpoint = errors.New("pool: malformed index endpoint")
	// ErrMissingAPIKey means the index client was given no API key.
	ErrMissingAPIKey = errors.New("pool: missing index API key")
	// ErrMissingIndexName means the index client was given no index name.
	ErrMissingIndexName = errors.New("pool: missing index name")
)

// DirectionCounts counts settled returns by the direction they point. A
// return with a non-fit reason is in the denominator but in neither count.
type DirectionCounts struct {
	// TooSmall counts settled returns whose reason says the item ran small.
	TooSmall int
	// TooLarge counts settled returns whose reason says the item ran large.
	TooLarge int
}

// VerdictCounts counts settled fit feedback by verdict. A fits-well verdict
// points nowhere; it is fit evidence, so it dilutes a direction's share.
type VerdictCounts struct {
	// FitsWell counts feedback saying the size fits.
	FitsWell int
	// TooSmall counts feedback saying the size was too small.
	TooSmall int
	// TooLarge counts feedback saying the size was too large.
	TooLarge int
}

// ShopperCounts counts distinct shoppers, never identities. Fit is the
// Building floor and is not the sum of the two directions: a shopper who
// returned one size each way counts once here.
type ShopperCounts struct {
	// Fit counts distinct shoppers with any fit-classified settled outcome.
	Fit int
	// TooSmall counts distinct shoppers pointing small.
	TooSmall int
	// TooLarge counts distinct shoppers pointing large.
	TooLarge int
}

// Counts is one bucket's settled counts for a product line and size. The
// denominator is Settled, every settled purchase, including a return with a
// non-fit reason and a purchase kept past the window. Directions are returns
// and feedback, and Reasons is the merchant's "why it comes back".
type Counts struct {
	// Settled is the denominator: every settled purchase.
	Settled int
	// Kept counts purchases kept past the return window with no feedback.
	Kept int
	// Returns counts settled returns by direction.
	Returns DirectionCounts
	// Feedback counts settled feedback by verdict.
	Feedback VerdictCounts
	// Reasons counts settled returns by the index's reason category. It is
	// never nil, and a category the index reports that this build does not
	// name is kept rather than dropped.
	Reasons map[domain.Category]int
	// Shoppers counts distinct shoppers.
	Shoppers ShopperCounts
}

// poolCounts converts one bucket to the shape the status rule reads. A
// direction is returns and feedback together, whichever way the shopper
// told us; a fits-well verdict points nowhere but dilutes the share.
func (c Counts) poolCounts() domain.PoolCounts {
	return domain.PoolCounts{
		TooSmall:         c.Returns.TooSmall + c.Feedback.TooSmall,
		TooLarge:         c.Returns.TooLarge + c.Feedback.TooLarge,
		FitsWell:         c.Feedback.FitsWell,
		ShoppersTooSmall: c.Shoppers.TooSmall,
		ShoppersTooLarge: c.Shoppers.TooLarge,
		Shoppers:         c.Shoppers.Fit,
	}
}

// Fit is the pooled product-fit belief for one product line and one size:
// the counts over all settled rows, the seeded and live parts of them, the
// last window of event time, and the label the rule decided. It carries no
// shopper identity, no row and no free text.
type Fit struct {
	// Line is the product line the counts are for.
	Line domain.ProductLine
	// Size is the size the counts are for.
	Size string
	// Verdict is the label and direction the rule decided from Counts and
	// Recent.
	Verdict domain.PoolVerdict
	// Counts is every settled purchase in the line and size, seeded and
	// live.
	Counts Counts
	// Seeded is the seeded part of Counts. Every seeded row carries the
	// mark, and the ledger shows the split.
	Seeded Counts
	// Live is the lived part of Counts.
	Live Counts
	// Recent is the last domain.PoolRecentWindow of event time alone, the
	// window the Out of true rule reads.
	Recent Counts
}

// Query names the one bucket a read is about: the product line, the size,
// and the first event time the recent window includes.
type Query struct {
	// Line is the product line: brand, category and fit handle.
	Line domain.ProductLine
	// Size is the size the counts are for.
	Size string
	// Since is the first event time the recent window includes. A settled
	// row whose settlement is at or after it is in the window.
	Since time.Time
}

// Result is one index answer: the counts over all settled rows, the seeded
// and live parts of them, and the recent window. Recent is empty when no
// settled row falls inside the window, never absent.
type Result struct {
	// All is every settled row in the line and size.
	All Counts
	// Seeded is the seeded part of All.
	Seeded Counts
	// Live is the lived part of All.
	Live Counts
	// Recent is the settled part of the recent window alone.
	Recent Counts
}

// Index is the whole of the pool reader's interface to the index. The layer
// names only these operations; the store behind them stays replaceable, and
// no sponsor name reaches the layer's code.
type Index interface {
	// Warm opens or reuses the index connection, so a cold handshake lands
	// on a start-up rather than on a recommendation.
	Warm(ctx context.Context) error
	// Aggregate runs the line-and-size aggregation for one query and
	// returns the settled counts. It reports ErrUnavailable when the index
	// cannot be reached.
	Aggregate(ctx context.Context, q Query) (Result, error)
}

// Option configures a Reader.
type Option func(*Reader)

// WithClock sets the clock the reader uses to place the recent window's
// start. Tests use it to pin the window; production reads the wall clock. A
// nil clock leaves the wall clock in place.
func WithClock(now func() time.Time) Option {
	return func(r *Reader) {
		if now != nil {
			r.now = now
		}
	}
}

// Reader reads the pooled product-fit belief for a line and size. It adds
// the recent window's start to the query and hands the counts to the domain's
// rule; it holds no copy of the thresholds or the rule itself.
type Reader struct {
	index Index
	now   func() time.Time
}

// NewReader builds a pool reader over one index. The index is required.
func NewReader(index Index, opts ...Option) (*Reader, error) {
	if index == nil {
		return nil, ErrMissingIndex
	}
	r := &Reader{index: index, now: time.Now}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// Fit reads the pooled belief for one product line and size. The recent
// window starts one domain.PoolRecentWindow before now, in event time, and
// the status comes from domain.DecidePool over the all-time and recent
// counts. A line with no settled rows is an empty pool and reads Claimed; an
// index that cannot be reached is ErrUnavailable, never an empty pool.
func (r *Reader) Fit(ctx context.Context, line domain.ProductLine, size string) (Fit, error) {
	if err := usableQuery(line, size); err != nil {
		return Fit{}, err
	}
	query := Query{
		Line:  line,
		Size:  size,
		Since: r.now().UTC().Add(-domain.PoolRecentWindow),
	}
	result, err := r.index.Aggregate(ctx, query)
	if err != nil {
		return Fit{}, fmt.Errorf("pool: read fit for line %q/%q/%q size %q: %w",
			line.Brand, line.Category, line.FitHandle, size, err)
	}
	return Fit{
		Line:    line,
		Size:    size,
		Verdict: domain.DecidePool(result.All.poolCounts(), result.Recent.poolCounts()),
		Counts:  result.All,
		Seeded:  result.Seeded,
		Live:    result.Live,
		Recent:  result.Recent,
	}, nil
}

// Warm opens or reuses the index connection. A caller runs it when the layer
// starts, so the first recommendation after a cold start reads a warm
// connection instead of paying the handshake.
func (r *Reader) Warm(ctx context.Context) error {
	if err := r.index.Warm(ctx); err != nil {
		return fmt.Errorf("pool: warm index connection: %w", err)
	}
	return nil
}

// usableQuery refuses a query that would match more than one line or size.
func usableQuery(line domain.ProductLine, size string) error {
	if line.Brand == "" || line.Category == "" || line.FitHandle == "" || size == "" {
		return fmt.Errorf("%w: brand %q, category %q, fit handle %q, size %q",
			ErrMissingQuery, line.Brand, line.Category, line.FitHandle, size)
	}
	return nil
}
