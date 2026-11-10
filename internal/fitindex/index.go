package fitindex

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/identity"
)

// Index is the whole of the layer's interface to the fit index. The layer
// names only these operations; the store behind them stays replaceable.
type Index interface {
	// Create makes the index with the recorded mapping. It succeeds when
	// the index already exists, so a start-up may call it every time.
	Create(ctx context.Context) error
	// Put replaces one purchase's whole document at the row's version.
	// The index accepts an equal version, so a replayed event rewrites the
	// same row, and refuses an older one with ErrOlderEvent.
	Put(ctx context.Context, row domain.FitRow) error
	// DeleteShopper removes every row one shopper wrote in one line and
	// reports how many documents went. Opt-out calls it once per line.
	DeleteShopper(ctx context.Context, line domain.ProductLine, shopperHash string) (int, error)
}

// Errors returned when a row cannot be written. A missing transaction id or
// event time is a caller defect, never a defaulted value.
var (
	// ErrMissingTransaction means a purchase carries no capture or order
	// id, so its row has no stable document id.
	ErrMissingTransaction = errors.New("fitindex: purchase has no transaction id")
	// ErrMissingEventTime means an event carries no time, so its row has
	// no version to write at.
	ErrMissingEventTime = errors.New("fitindex: event time is missing")
	// ErrUnknownReason means a reason category is outside the frozen set.
	ErrUnknownReason = errors.New("fitindex: unknown reason category")
	// ErrUnknownVerdict means a feedback verdict is outside the frozen set.
	ErrUnknownVerdict = errors.New("fitindex: unknown feedback verdict")
	// ErrUnknownOutcome means a payment outcome kind is outside the set a
	// fit row tracks.
	ErrUnknownOutcome = errors.New("fitindex: unknown outcome kind")
	// ErrOlderEvent means the index already holds a newer state for the
	// row, and the write must not win.
	ErrOlderEvent = errors.New("fitindex: an older event cannot overwrite a newer state")
	// ErrIndexStatus means the index answered with an error status.
	ErrIndexStatus = errors.New("fitindex: index answered with an error status")
	// ErrMissingIndex means a writer was built without an index.
	ErrMissingIndex = errors.New("fitindex: missing index")
	// ErrMissingKeyer means a writer was built without an identity keyer.
	ErrMissingKeyer = errors.New("fitindex: missing identity keyer")
	// ErrMissingLines means a writer was built without a line resolver.
	ErrMissingLines = errors.New("fitindex: missing product line resolver")
	// ErrMalformedEndpoint means the index client was given a base URL
	// that is not an absolute http or https URL.
	ErrMalformedEndpoint = errors.New("fitindex: malformed index endpoint")
	// ErrMissingAPIKey means the index client was given no API key.
	ErrMissingAPIKey = errors.New("fitindex: missing index API key")
	// ErrMissingIndexName means the index client was given no index name.
	ErrMissingIndexName = errors.New("fitindex: missing index name")
	// ErrMissingLine means a delete was asked for a line whose brand,
	// category, fit handle or shopper hash is empty, which would match
	// more than the shopper's rows.
	ErrMissingLine = errors.New("fitindex: incomplete line or shopper hash")
)

// Purchase names the purchase a row is about: the transaction the row is
// keyed by, the product line, the size and the seeded mark. The shopper is
// the writer's, so a caller never passes a shopper id.
type Purchase struct {
	// TxnID is the purchase's transaction id: the capture id, or the order
	// id before the capture is known.
	TxnID string
	// Line is the product line: brand, category and fit handle.
	Line domain.ProductLine
	// Size is the purchased size, on the product's own label.
	Size string
	// Seeded marks generated history. Every seeded row carries the mark.
	Seeded bool
}

// Lines names the product line an order item belongs to. Ingest sees the
// item; only the catalog knows the line's fit handle, so the writer asks
// rather than guessing one from the item.
type Lines interface {
	// ProductLine returns the brand, category and fit handle for one item.
	ProductLine(ctx context.Context, item domain.Item) (domain.ProductLine, error)
}

// LinesFunc adapts a function to Lines.
type LinesFunc func(ctx context.Context, item domain.Item) (domain.ProductLine, error)

// ProductLine calls f.
func (f LinesFunc) ProductLine(ctx context.Context, item domain.Item) (domain.ProductLine, error) {
	return f(ctx, item)
}

// Pooling reports whether the writer's shopper lets their outcomes count
// toward the pool. The layer records the opt-out in the shopper's own
// memory and reads it here. A writer without one pools, because pooling is
// opt-out by default.
type Pooling func() bool

// Option configures a Writer.
type Option func(*Writer)

// WithPooling sets the shopper's consent check. When it reports false,
// every event writes no row: opting out stops new fit rows for that
// shopper.
func WithPooling(pooling Pooling) Option {
	return func(w *Writer) {
		w.pooling = pooling
	}
}

// Writer writes one shopper's fit rows. One writer serves one shopper,
// because the per-line shopper hash is salted over that shopper's payer
// id; keeping a writer per shopper keeps two shoppers' rows apart.
//
// The writer holds each purchase's facts for the life of the process, so
// it can replace the whole document on every event: a capture's time is
// still there when the reason arrives. A writer that starts after the
// purchase knows only the events it sees, and fills the facts in as they
// arrive. A Writer is safe for concurrent use and writes one shopper's
// events in the order they reach it.
type Writer struct {
	index   Index
	keyer   *identity.Keyer
	payerID string
	lines   Lines
	pooling Pooling

	mu   sync.Mutex
	rows map[string]*domain.FitRow
}

// NewWriter builds the fit row writer for one shopper over one index. The
// keyer supplies the identity salt; lines resolves each order item's
// product line.
func NewWriter(index Index, keyer *identity.Keyer, payerID string, lines Lines, opts ...Option) (*Writer, error) {
	if index == nil {
		return nil, ErrMissingIndex
	}
	if keyer == nil {
		return nil, ErrMissingKeyer
	}
	if payerID == "" {
		return nil, fmt.Errorf("fitindex: new writer: %w", identity.ErrEmptyPayerID)
	}
	if lines == nil {
		return nil, ErrMissingLines
	}
	w := &Writer{
		index:   index,
		keyer:   keyer,
		payerID: payerID,
		lines:   lines,
		rows:    make(map[string]*domain.FitRow),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w, nil
}

// Record writes the fit row for one ingested evidence row, from the same
// event that wrote the shopper's evidence. A capture creates the row, a
// refund marks it returned, and a kept outcome settles it. A dispute
// writes nothing: it says the purchase did not arrive, which is about the
// seller, not how the product fits. The shopper's own words write nothing
// either, because a correction is about the shopper and never enters the
// pool.
func (w *Writer) Record(ctx context.Context, ev domain.Evidence) error {
	if ev.Source != domain.SourcePayment {
		return nil
	}
	switch ev.Outcome.Kind {
	case domain.OutcomeCapture, domain.OutcomeRefund, domain.OutcomeKept:
	case domain.OutcomeDispute:
		return nil
	default:
		return fmt.Errorf("fitindex: record event %q: %w: %q", ev.Link.EventID, ErrUnknownOutcome, string(ev.Outcome.Kind))
	}
	purchase, err := w.purchaseFor(ctx, ev)
	if err != nil {
		return err
	}
	switch ev.Outcome.Kind {
	case domain.OutcomeCapture:
		return w.Captured(ctx, purchase, ev.OccurredAt)
	case domain.OutcomeRefund:
		return w.Refunded(ctx, purchase, ev.OccurredAt)
	default:
		return w.Kept(ctx, purchase, ev.OccurredAt)
	}
}

// Captured writes the purchase's row at the captured state. It creates the
// row on first sight. A capture that arrives after a later state fills the
// capture time in without lowering the state.
func (w *Writer) Captured(ctx context.Context, p Purchase, at time.Time) error {
	return w.record(ctx, p, change{state: domain.FitStateCaptured, at: at, capturedAt: at})
}

// Refunded marks the purchase returned, with no reason yet. The row is
// stored and unsettled: a refund says the money came back, not why.
func (w *Writer) Refunded(ctx context.Context, p Purchase, at time.Time) error {
	return w.record(ctx, p, change{state: domain.FitStateRefunded, at: at, refundedAt: at})
}

// Classified records the shopper's classified answer for a returned
// purchase. A reason that names a cause settles the purchase as a return
// with that reason and its direction. A reason of unclear or none is
// recorded and leaves the row unsettled: the refund still has no cause, so
// the agent asks again.
func (w *Writer) Classified(ctx context.Context, p Purchase, reason domain.Category, at time.Time) error {
	if !validCategory(reason) {
		return fmt.Errorf("fitindex: classify purchase %q: %w: %q", p.TxnID, ErrUnknownReason, string(reason))
	}
	ch := change{state: domain.FitStateReasonClassified, at: at, reason: reason}
	if hasCause(reason) {
		ch.settled = true
		ch.kind = domain.FitSettledByReturnReason
		ch.settledAt = at
	}
	return w.record(ctx, p, ch)
}

// Feedback records fit feedback tied to the purchase. Every recorded
// verdict settles the purchase: fits well says the size was right, and the
// other two say which way it ran.
func (w *Writer) Feedback(ctx context.Context, p Purchase, verdict domain.FitVerdict, at time.Time) error {
	if !validVerdict(verdict) {
		return fmt.Errorf("fitindex: feedback purchase %q: %w: %q", p.TxnID, ErrUnknownVerdict, string(verdict))
	}
	return w.record(ctx, p, change{
		state:     domain.FitStateFeedback,
		at:        at,
		verdict:   verdict,
		settled:   true,
		kind:      domain.FitSettledByFeedback,
		settledAt: at,
	})
}

// Kept records the purchase kept past its return window with no feedback.
// It settles the purchase as kept: a weak, inferred signal that the item
// was not returned, never a claim that it was liked.
func (w *Writer) Kept(ctx context.Context, p Purchase, at time.Time) error {
	return w.record(ctx, p, change{
		state:     domain.FitStateKept,
		at:        at,
		settled:   true,
		kind:      domain.FitSettledByKept,
		settledAt: at,
	})
}

// Contested records the shopper's contested reason. The corrected category
// and direction replace the old values in place, and the row stays settled
// when the correction names a cause. A correction that names no cause
// leaves the row unsettled, because the purchase has no cause left.
func (w *Writer) Contested(ctx context.Context, p Purchase, reason domain.Category, at time.Time) error {
	if !validCategory(reason) {
		return fmt.Errorf("fitindex: contest purchase %q: %w: %q", p.TxnID, ErrUnknownReason, string(reason))
	}
	ch := change{state: domain.FitStateContested, at: at, reason: reason, contested: true}
	if hasCause(reason) {
		ch.settled = true
		ch.kind = domain.FitSettledByReturnReason
		ch.settledAt = at
	}
	return w.record(ctx, p, ch)
}

// Forget deletes every row the writer's shopper wrote in the given lines,
// one per-line hash per line, and reports how many documents went. The
// caller records the opt-out in the shopper's own memory first, so the
// Pooling check stops new rows; Forget removes what is already written. A
// line the shopper never bought in deletes nothing.
func (w *Writer) Forget(ctx context.Context, lines []domain.ProductLine) (int, error) {
	removed := 0
	for _, line := range lines {
		if err := ctx.Err(); err != nil {
			return removed, fmt.Errorf("fitindex: forget: %w", err)
		}
		shopperHash, err := w.keyer.LineShopperHash(w.payerID, line)
		if err != nil {
			return removed, fmt.Errorf("fitindex: forget line %q/%q/%q: %w",
				line.Brand, line.Category, line.FitHandle, err)
		}
		deleted, err := w.index.DeleteShopper(ctx, line, shopperHash)
		if err != nil {
			return removed, fmt.Errorf("fitindex: forget line %q/%q/%q: %w",
				line.Brand, line.Category, line.FitHandle, err)
		}
		removed += deleted
		w.forgetRows(shopperHash)
	}
	return removed, nil
}

// forgetRows drops the purchase facts held for one per-line hash, so a row
// written after an opt-back-in starts from the events that follow it.
func (w *Writer) forgetRows(shopperHash string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for txnHash, row := range w.rows {
		if row.ShopperHash == shopperHash {
			delete(w.rows, txnHash)
		}
	}
}

// change is one event's effect on a purchase: the state it moves to, the
// content that state carries, and the event time it is written at.
type change struct {
	state      domain.FitPurchaseState
	at         time.Time
	reason     domain.Category
	verdict    domain.FitVerdict
	settled    bool
	kind       domain.FitSettledKind
	contested  bool
	capturedAt time.Time
	refundedAt time.Time
	settledAt  time.Time
}

// record applies one event to the purchase's row and replaces the whole
// document at the row's version. Facts about the purchase are recorded
// whenever an event carries them; the state moves only when the event is
// at least as new as the state held. A write the index refuses as older
// leaves the newer state standing and is not an error: the delivery is
// done and its retry must stay harmless.
func (w *Writer) record(ctx context.Context, p Purchase, ch change) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fitindex: record purchase: %w", err)
	}
	if ch.at.IsZero() {
		return fmt.Errorf("fitindex: record purchase %q: %w", p.TxnID, ErrMissingEventTime)
	}
	if w.pooling != nil && !w.pooling() {
		return nil
	}
	row, err := w.rowFor(p)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	held, ok := w.rows[row.TxnHash]
	if !ok {
		held = &row
		w.rows[row.TxnHash] = held
	}
	merge(held, ch)
	if err := w.index.Put(ctx, *held); err != nil {
		if errors.Is(err, ErrOlderEvent) {
			return nil
		}
		return fmt.Errorf("fitindex: write row %q: %w", row.TxnHash, err)
	}
	return nil
}

// rowFor builds the row's stable parts: the document id from the
// transaction hash, and the salted per-line shopper hash.
func (w *Writer) rowFor(p Purchase) (domain.FitRow, error) {
	if p.TxnID == "" {
		return domain.FitRow{}, fmt.Errorf("fitindex: row: %w", ErrMissingTransaction)
	}
	txnHash, err := identity.TxnHash(p.TxnID)
	if err != nil {
		return domain.FitRow{}, fmt.Errorf("fitindex: hash transaction: %w", err)
	}
	shopperHash, err := w.keyer.LineShopperHash(w.payerID, p.Line)
	if err != nil {
		return domain.FitRow{}, fmt.Errorf("fitindex: hash shopper for line: %w", err)
	}
	return domain.FitRow{
		Line:        p.Line,
		Size:        p.Size,
		ShopperHash: shopperHash,
		TxnHash:     txnHash,
		Seeded:      p.Seeded,
	}, nil
}

// purchaseFor reads the purchase an evidence row is about. The transaction
// id is the capture's, or the order's before a capture is known, and the
// line comes from the catalog through Lines.
func (w *Writer) purchaseFor(ctx context.Context, ev domain.Evidence) (Purchase, error) {
	line, err := w.lines.ProductLine(ctx, ev.Item)
	if err != nil {
		return Purchase{}, fmt.Errorf("fitindex: line for item %q: %w", ev.Item.SKU, err)
	}
	return Purchase{
		TxnID:  transactionID(ev.Link),
		Line:   line,
		Size:   ev.Item.Size,
		Seeded: ev.Seeded,
	}, nil
}

// transactionID names the transaction a purchase is keyed by: the capture
// id, or the order id when the capture is not known.
func transactionID(link domain.TransactionLink) string {
	if link.CaptureID != "" {
		return link.CaptureID
	}
	return link.OrderID
}

// merge records an event's facts and, when the event is at least as new as
// the state held, moves the row to the event's state. Event time breaks
// ties inside a rank, so an older event cannot undo a newer state.
func merge(held *domain.FitRow, ch change) {
	held.CapturedAt = earliest(held.CapturedAt, ch.capturedAt)
	held.RefundedAt = earliest(held.RefundedAt, ch.refundedAt)
	held.SettledAt = earliest(held.SettledAt, ch.settledAt)

	candidate := domain.FitRow{State: ch.state, EventAt: ch.at}
	if candidate.Version() < held.Version() {
		return
	}
	held.State = ch.state
	held.EventAt = ch.at
	held.Reason = ch.reason
	held.Direction = directionFor(ch)
	held.Verdict = ch.verdict
	held.Settled = ch.settled
	held.SettledKind = ch.kind
	if ch.contested {
		held.ReasonContested = true
	}
}

// directionFor reports which way one event points. A classified reason's
// category decides it; a feedback verdict decides it when there is no
// reason.
func directionFor(ch change) domain.FitDirection {
	if dir := ch.reason.Direction(); dir != domain.FitDirectionNone {
		return dir
	}
	return ch.verdict.Direction()
}

// earliest returns the earlier of two event times, treating the zero time
// as unknown.
func earliest(held, at time.Time) time.Time {
	if at.IsZero() {
		return held
	}
	if held.IsZero() || at.Before(held) {
		return at
	}
	return held
}

// hasCause reports whether a reason names a cause. No reason, and an
// unclear answer, name none: the refund stays a pending question.
func hasCause(c domain.Category) bool {
	return c != "" && c != domain.CategoryNone && c != domain.CategoryUnclear
}

// validCategory reports whether a category is in the frozen set.
func validCategory(c domain.Category) bool {
	for _, known := range domain.Categories {
		if c == known {
			return true
		}
	}
	return false
}

// validVerdict reports whether a verdict is one of the three fit readings.
func validVerdict(v domain.FitVerdict) bool {
	switch v {
	case domain.FitVerdictFitsWell, domain.FitVerdictTooSmall, domain.FitVerdictTooLarge:
		return true
	default:
		return false
	}
}
