package ingest

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/payments"
)

// PendingQuestion is a refund still missing its reason. The refund row is
// stored with a nil reason and counts toward nothing; this question tells
// the agent what to ask the shopper. Answering it is a later task's job.
type PendingQuestion struct {
	// EventID is the payment source's event id for the refund delivery.
	EventID string
	// Outcome is the refunded capture and its amount.
	Outcome domain.Outcome
	// Items are the order's line items the refund concerns.
	Items []domain.Item
	// Link ties the refund to its transaction across order, capture and
	// refund records.
	Link domain.TransactionLink
	// OccurredAt is when the refund happened, in event time.
	OccurredAt time.Time
}

// Result is what one delivery produced: the stored rows, the pending
// question when the delivery was a refund, and whether the delivery was
// a redelivery of an already-stored event.
type Result struct {
	// Rows are the stored evidence rows for the delivery, one per line
	// item.
	Rows []domain.Evidence
	// Pending is the pending question the delivery filed, or nil when
	// the delivery was not a refund.
	Pending *PendingQuestion
	// Duplicate is true when the event id was already stored: the rows
	// are the stored ones and nothing new was written.
	Duplicate bool
}

// FitRecorder is the second write path: the fit row one delivery writes
// beside its evidence rows, from the same event. The fit index itself sits
// behind it, so ingest names no store. A capture creates the row, a refund
// marks it returned, and a dispute writes nothing because it is about the
// seller, not the purchase. A kept outcome would settle the row, but no
// delivery emits one: Handle accepts capture, refund and dispute only, so
// no served path decides that a return window has passed.
type FitRecorder interface {
	// Record writes the fit row for one evidence row. It is idempotent on
	// the purchase's state: a redelivered event rewrites the same row.
	Record(ctx context.Context, ev domain.Evidence) error
}

// Option configures a Service.
type Option func(*Service)

// WithClock sets the event-time source, keyed by event id. The replay
// clock is the default; the live server passes the delivery clock. When
// the clock yields the zero time for an event, Handle stamps the row
// with the current UTC time instead.
func WithClock(clock func(eventID string) time.Time) Option {
	return func(s *Service) {
		s.clock = clock
	}
}

// WithFitRecorder turns on the second write path: every delivery that
// records a purchase also records that purchase's fit row, from the same
// event, so the pool is written at ingest and nothing reads a shopper's
// memory to build it. When the recorder fails, the delivery fails and the
// payment source's retry writes both again; a retry is harmless because
// the fit row is written at the purchase state's version.
func WithFitRecorder(rec FitRecorder) Option {
	return func(s *Service) {
		s.fit = rec
	}
}

// Service ingests verified deliveries into durable rows. Rows are keyed
// by the payment source's event id: ingesting the same id twice returns
// the stored rows with Duplicate set and writes nothing new. A Service
// is safe for concurrent use.
type Service struct {
	payments fakes.PaymentSource
	verifier Verifier
	clock    func(eventID string) time.Time
	fit      FitRecorder

	mu      sync.Mutex
	rows    map[string][]domain.Evidence
	pending map[string]PendingQuestion
}

// NewService builds a Service over the payment source. The source is the
// fakes.PaymentSource interface the real client implements, so callers
// code to the interface and the swap from replay to live touches no
// caller. The delivery gate is the real payment-source adapter unless a
// caller replaces it with WithVerifier.
func NewService(source fakes.PaymentSource, opts ...Option) *Service {
	s := &Service{
		payments: source,
		verifier: payments.New(),
		clock:    fakes.OccurredAt,
		rows:     make(map[string][]domain.Evidence),
		pending:  make(map[string]PendingQuestion),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Handle verifies one delivery and stores its evidence rows, idempotent
// on the payment source's event id. Redelivery returns the stored rows
// with Duplicate set. A refund also files a pending question; a dispute
// arrives with its reason classified through the frozen dispute table, or
// with no category when the table does not name its code.
func (s *Service) Handle(ctx context.Context, eventID string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("ingest: handle event %q: %w", eventID, err)
	}
	if eventID == "" {
		return Result{}, fmt.Errorf("ingest: handle: %w: empty event id", ErrMalformedEvent)
	}
	s.mu.Lock()
	if rows, ok := s.rows[eventID]; ok {
		result := Result{Rows: copyRows(rows), Duplicate: true}
		if question, ok := s.pending[eventID]; ok {
			duplicate := question
			result.Pending = &duplicate
		}
		s.mu.Unlock()
		return result, nil
	}
	s.mu.Unlock()

	event, err := s.payments.VerifyWebhook(ctx, eventID)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: verify event %q: %w", eventID, err)
	}
	if !event.Verified {
		return Result{}, fmt.Errorf("ingest: event %q: %w", eventID, ErrBadSignature)
	}

	var result Result
	switch event.Outcome.Kind {
	case domain.OutcomeRefund:
		result, err = s.handleRefund(ctx, event)
	case domain.OutcomeDispute:
		result, err = s.handleDispute(ctx, event)
	case domain.OutcomeCapture:
		result, err = s.handleCapture(ctx, event)
	default:
		return Result{}, fmt.Errorf("ingest: event %q kind %q: %w", eventID, event.Outcome.Kind, ErrUnknownEvent)
	}
	if err != nil {
		return Result{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if stored, ok := s.rows[eventID]; ok {
		result = Result{Rows: copyRows(stored), Duplicate: true}
		if question, ok := s.pending[eventID]; ok {
			duplicate := question
			result.Pending = &duplicate
		}
		return result, nil
	}
	s.rows[eventID] = copyRows(result.Rows)
	if result.Pending != nil {
		s.pending[eventID] = *result.Pending
	}
	return result, nil
}

// handleRefund stores one row per order line item with no reason and
// files the refund as a pending question. The order read is the only
// join: the capture delivery need not have arrived first.
func (s *Service) handleRefund(ctx context.Context, event fakes.WebhookEvent) (Result, error) {
	if event.Link.RefundID == "" || event.Link.OrderID == "" {
		return Result{}, fmt.Errorf("ingest: refund event %q: %w", event.EventID, ErrMalformedEvent)
	}
	record, err := s.payments.GetRefund(ctx, event.Link.RefundID)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: read refund %q: %w", event.Link.RefundID, err)
	}
	order, err := s.payments.GetOrder(ctx, event.Link.OrderID)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: read order %q: %w", event.Link.OrderID, err)
	}
	if len(order.Items) == 0 {
		return Result{}, fmt.Errorf("ingest: refund event %q: %w", event.EventID, ErrNoLineItems)
	}
	occurred := s.stamp(event.EventID)
	link := record.Link
	link.EventID = event.EventID
	rows := make([]domain.Evidence, len(order.Items))
	for i, item := range order.Items {
		rows[i] = domain.Evidence{
			Source:     domain.SourcePayment,
			Outcome:    record.Outcome,
			Item:       item,
			Reason:     nil,
			Link:       link,
			OccurredAt: occurred,
		}
	}
	question := &PendingQuestion{
		EventID:    event.EventID,
		Outcome:    record.Outcome,
		Items:      append([]domain.Item(nil), order.Items...),
		Link:       link,
		OccurredAt: occurred,
	}
	if err := s.recordFit(ctx, rows); err != nil {
		return Result{}, err
	}
	return Result{Rows: rows, Pending: question}, nil
}

// handleDispute stores one row per line item with the reason the frozen
// dispute table maps the source's code to. The items come from the order
// read; the dispute record's item is the fallback when the order read
// carries none.
//
// A code the table does not name stores the row with no classified
// reason: the disposition comes from domain.StoredCategoryForDispute, so
// the outcome is stored instead of dropped and no belief is earned from
// it.
func (s *Service) handleDispute(ctx context.Context, event fakes.WebhookEvent) (Result, error) {
	if event.Link.DisputeID == "" {
		return Result{}, fmt.Errorf("ingest: dispute event %q: %w", event.EventID, ErrMalformedEvent)
	}
	dispute, err := s.payments.GetDispute(ctx, event.Link.DisputeID)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: read dispute %q: %w", event.Link.DisputeID, err)
	}
	category, err := domain.StoredCategoryForDispute(dispute.Code)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: dispute event %q: %w", event.EventID, err)
	}
	items := []domain.Item{}
	if event.Link.OrderID != "" {
		order, err := s.payments.GetOrder(ctx, event.Link.OrderID)
		if err != nil {
			return Result{}, fmt.Errorf("ingest: read order %q: %w", event.Link.OrderID, err)
		}
		items = append(items, order.Items...)
	}
	if len(items) == 0 {
		items = append(items, dispute.Item)
	}
	if len(items) == 0 || items[0].SKU == "" {
		return Result{}, fmt.Errorf("ingest: dispute event %q: %w", event.EventID, ErrNoLineItems)
	}
	occurred := s.stamp(event.EventID)
	link := dispute.Link
	link.EventID = event.EventID
	var reason *domain.Reason
	if code := string(dispute.Code); code != "" {
		reason = &domain.Reason{
			Category:   category,
			Text:       code,
			Confidence: 1.0,
		}
	}
	rows := make([]domain.Evidence, len(items))
	for i, item := range items {
		rows[i] = domain.Evidence{
			Source:     domain.SourcePayment,
			Outcome:    event.Outcome,
			Item:       item,
			Reason:     reason,
			Link:       link,
			OccurredAt: occurred,
		}
	}
	// A dispute writes no fit row: it says the purchase did not arrive or
	// was not as described, which is about the seller, not how the
	// product fits.
	return Result{Rows: rows}, nil
}

// handleCapture stores one row per order line item with no reason. A
// capture proves the purchase happened; it proves nothing about fit or
// the seller, so it counts toward nothing on its own.
func (s *Service) handleCapture(ctx context.Context, event fakes.WebhookEvent) (Result, error) {
	if event.Link.OrderID == "" {
		return Result{}, fmt.Errorf("ingest: capture event %q: %w", event.EventID, ErrMalformedEvent)
	}
	order, err := s.payments.GetOrder(ctx, event.Link.OrderID)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: read order %q: %w", event.Link.OrderID, err)
	}
	if len(order.Items) == 0 {
		return Result{}, fmt.Errorf("ingest: capture event %q: %w", event.EventID, ErrNoLineItems)
	}
	occurred := s.stamp(event.EventID)
	link := event.Link
	rows := make([]domain.Evidence, len(order.Items))
	for i, item := range order.Items {
		rows[i] = domain.Evidence{
			Source:     domain.SourcePayment,
			Outcome:    event.Outcome,
			Item:       item,
			Reason:     nil,
			Link:       link,
			OccurredAt: occurred,
		}
	}
	if err := s.recordFit(ctx, rows); err != nil {
		return Result{}, err
	}
	return Result{Rows: rows}, nil
}

// recordFit writes each row's fit row from the same event. A delivery
// without a recorder, or one that touches no purchase, writes nothing.
func (s *Service) recordFit(ctx context.Context, rows []domain.Evidence) error {
	if s.fit == nil {
		return nil
	}
	for _, row := range rows {
		if err := s.fit.Record(ctx, row); err != nil {
			return fmt.Errorf("ingest: record fit row for event %q: %w", row.Link.EventID, err)
		}
	}
	return nil
}

// stamp returns the event time for one event id: the clock's answer, or
// the current UTC time when the clock yields the zero time.
func (s *Service) stamp(eventID string) time.Time {
	if moment := s.clock(eventID); !moment.IsZero() {
		return moment
	}
	return time.Now().UTC()
}

// Rows returns every stored evidence row, ordered by event time and then
// by event id. The rows are copies; later deliveries never mutate them.
func (s *Service) Rows() []domain.Evidence {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Evidence
	for _, rows := range s.rows {
		out = append(out, copyRows(rows)...)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].OccurredAt.Equal(out[j].OccurredAt) {
			return out[i].OccurredAt.Before(out[j].OccurredAt)
		}
		if out[i].Link.EventID != out[j].Link.EventID {
			return out[i].Link.EventID < out[j].Link.EventID
		}
		return out[i].Item.SKU < out[j].Item.SKU
	})
	return out
}

// PendingQuestions returns every refund still missing its reason,
// ordered by event id.
func (s *Service) PendingQuestions() []PendingQuestion {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]PendingQuestion, 0, len(s.pending))
	for _, question := range s.pending {
		out = append(out, question)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EventID < out[j].EventID })
	return out
}

// copyRows copies one event's rows so stored rows never alias a result.
func copyRows(rows []domain.Evidence) []domain.Evidence {
	return append([]domain.Evidence(nil), rows...)
}
