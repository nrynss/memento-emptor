package server

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/memory"
)

// The service's pooling conditions. Callers branch on these with
// errors.Is and never on message text.
var (
	// ErrTooManyLines is returned when the session's line list cannot be
	// resolved for an opt-out. The opt-out itself is recorded; the rows
	// stay until the lines resolve.
	ErrTooManyLines = errors.New("server: product lines do not resolve")
)

// fitPurchase resolves the purchase one fit row is about: the product
// line from the one catalog source, the transaction id from the row's
// own link, and the size and seeded mark from the row. Every writer call
// — the ingest-time capture and refund, the reason classification, the
// feedback — goes through here, so the writer's Record and the explicit
// Classified, Feedback and Contested calls agree on the same line for the
// same purchase. A correction never reaches the writer: it is about the
// shopper, not the product, and it names no reason category to pool.
func (s *Service) fitPurchase(ctx context.Context, row domain.Evidence) (fitindex.Purchase, error) {
	if s.lines == nil {
		return fitindex.Purchase{}, fmt.Errorf("server: fit purchase: %w", ErrPoolUnwired)
	}
	line, err := s.lines.ProductLine(ctx, row.Item)
	if err != nil {
		return fitindex.Purchase{}, fmt.Errorf("server: line for item %q: %w", row.Item.SKU, err)
	}
	txn := row.Link.CaptureID
	if txn == "" {
		txn = row.Link.OrderID
	}
	if txn == "" {
		return fitindex.Purchase{}, fmt.Errorf("server: fit purchase: %w", fitindex.ErrMissingTransaction)
	}
	return fitindex.Purchase{
		TxnID:  txn,
		Line:   line,
		Size:   row.Item.Size,
		Seeded: row.Seeded,
	}, nil
}

// recordFitRows writes the fit rows for one delivery's rows, from the
// same events that wrote the shopper's evidence. A capture creates the
// row and a refund marks it returned; a dispute writes nothing because it
// is about the seller, not how the product fits. The transaction id is
// the row's own capture id, or the order id before a capture is known:
// both legs of one purchase carry the same link, so they key the same
// row. When the records disagree about the capture id the two legs key
// two rows, which is the writer's documented two-row hazard, not a
// server decision.
func (s *Service) recordFitRows(ctx context.Context, st *shopperState, rows []domain.Evidence) error {
	if st.writer == nil {
		return nil
	}
	for _, row := range rows {
		purchase, err := s.fitPurchase(ctx, row)
		if err != nil {
			return err
		}
		switch row.Outcome.Kind {
		case domain.OutcomeCapture:
			if err := st.writer.Captured(ctx, purchase, row.OccurredAt); err != nil {
				return fmt.Errorf("server: record fit row: %w", err)
			}
		case domain.OutcomeRefund:
			if err := st.writer.Refunded(ctx, purchase, row.OccurredAt); err != nil {
				return fmt.Errorf("server: record fit row: %w", err)
			}
		default:
			continue
		}
	}
	return nil
}

// persistIfJournal saves the shopper's durable state when a journal
// exists to save it to. The evidence partition is written after every
// mutation; the standing snapshot is written when the shopper holds
// journal entries, because only those can be declared against a later
// belief. Without a standing directory both stay in memory.
func (s *Service) persistIfJournal(ctx context.Context, st *shopperState, shopper string) error {
	if st.journal.Len() == 0 {
		return s.persistEvidence(ctx, st, shopper)
	}
	return s.persistStanding(ctx, st, shopper)
}

// classifyFitRows records the shopper's classified answer for each row
// behind one pending question. A row settles whether or not a belief
// claims it: the pool counts settled purchases, not beliefs. Only the
// writer's explicit Classified path is used; the line comes from the one
// catalog source, so it agrees with the ingest-time capture and refund.
func (s *Service) classifyFitRows(ctx context.Context, st *shopperState, rows []*evidenceRecord, category domain.Category, at time.Time) error {
	if st.writer == nil {
		return nil
	}
	for _, rec := range rows {
		purchase, err := s.fitPurchase(ctx, rec.row)
		if err != nil {
			return err
		}
		if err := st.writer.Classified(ctx, purchase, category, at); err != nil {
			return fmt.Errorf("server: classify fit row: %w", err)
		}
	}
	return nil
}

// feedbackFitRow records fit feedback tied to a purchase in the fit row.
// Only a fit verdict settles a row: any other verdict returns without
// writing, and the callers reach here only for a fit verdict.
func (s *Service) feedbackFitRow(ctx context.Context, st *shopperState, rec *evidenceRecord, verdict contract.FeedbackVerdict, at time.Time) error {
	if st.writer == nil {
		return nil
	}
	var fit domain.FitVerdict
	switch verdict {
	case contract.FeedbackFitsWell:
		fit = domain.FitVerdictFitsWell
	case contract.FeedbackTooSmall:
		fit = domain.FitVerdictTooSmall
	case contract.FeedbackTooLarge:
		fit = domain.FitVerdictTooLarge
	default:
		return nil
	}
	purchase, err := s.fitPurchase(ctx, rec.row)
	if err != nil {
		return err
	}
	if err := st.writer.Feedback(ctx, purchase, fit, at); err != nil {
		return fmt.Errorf("server: feedback fit row: %w", err)
	}
	return nil
}

// ProductFit reads the pooled product fit for one product line and size:
// its status, the direction it rates, the settled counts by direction,
// feedback and reason, distinct shoppers by direction, and the seeded
// split. It carries no shopper identity, no row and no free text, so the
// merchant app reads this and nothing else. Without a wired pool index
// it refuses rather than report an empty pool.
func (s *Service) ProductFit(ctx context.Context, req contract.ProductFitRequest) (contract.ProductFitResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolProductFit), "", nil)
	if s.pool == nil {
		return contract.ProductFitResponse{}, fmt.Errorf("server: product fit: %w", ErrPoolUnavailable)
	}
	line := domain.ProductLine{Brand: req.BrandID, Category: req.Category, FitHandle: req.FitHandle}
	fit, err := s.pool.Fit(ctx, line, req.Size)
	if err != nil {
		return contract.ProductFitResponse{}, fmt.Errorf("server: product fit: %w", ErrPoolUnavailable)
	}
	return contract.ProductFitResponse{Pool: poolViewOf(fit)}, nil
}

// SetPooling records the shopper's pooling choice. Opting out stops new
// fit rows and removes the rows already written, across every line the
// session knows; opting back in removes nothing. The choice itself is
// recorded in the shopper's own memory. A partial delete is not done:
// the opt-out retries the session's full line list, and a delete that
// still reports failures fails the call so the agent can try again.
func (s *Service) SetPooling(ctx context.Context, req contract.SetPoolingRequest) (contract.SetPoolingResponse, error) {
	if strings.TrimSpace(req.ShopperID) == "" {
		return contract.SetPoolingResponse{}, fmt.Errorf("server: set pooling: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolSetPooling), req.ShopperID, traceOf(req.TraceID))
	st, err := s.state(ctx, req.ShopperID)
	if err != nil {
		return contract.SetPoolingResponse{}, fmt.Errorf("server: partition for %q: %w", req.ShopperID, err)
	}
	now := s.deps.Clock()
	if err := memory.RecordStatement(ctx, s.graph, poolingContent(req.Pooling), now); err != nil {
		return contract.SetPoolingResponse{}, fmt.Errorf("server: set pooling: %w", err)
	}
	st.pooling.Store(req.Pooling)
	removed := 0
	if !req.Pooling && st.writer != nil {
		lines, err := s.sessionLines(ctx, st)
		if err != nil {
			return contract.SetPoolingResponse{}, err
		}
		removed, err = forgetWithRetry(ctx, st.writer, lines)
		if err != nil {
			return contract.SetPoolingResponse{}, err
		}
	}
	if err := s.persistStanding(ctx, st, req.ShopperID); err != nil {
		return contract.SetPoolingResponse{}, err
	}
	return contract.SetPoolingResponse{
		ShopperID:   req.ShopperID,
		Pooling:     req.Pooling,
		RowsRemoved: removed,
		TraceID:     traceOf(req.TraceID),
	}, nil
}

// poolingContent states the shopper's pooling choice in words for their
// own memory. It is a note, never evidence: the choice constrains the
// pool, it does not support a belief.
func poolingContent(pooling bool) string {
	if pooling {
		return "shopper opted back in to pooling"
	}
	return "shopper opted out of pooling"
}

// sessionLines lists every product line the shopper's session knows:
// the distinct lines across its stored evidence rows, resolved through
// the one catalog source. Opt-out deletes the shopper's rows line by
// line, so this list is what Forget must cover.
func (s *Service) sessionLines(ctx context.Context, st *shopperState) ([]domain.ProductLine, error) {
	seen := map[domain.ProductLine]bool{}
	out := []domain.ProductLine{}
	for _, rec := range st.evidence {
		line, err := s.lines.ProductLine(ctx, rec.row.Item)
		if err != nil {
			return nil, fmt.Errorf("%w: item %q: %v", ErrTooManyLines, rec.row.Item.SKU, err)
		}
		if !seen[line] {
			seen[line] = true
			out = append(out, line)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Brand != out[j].Brand {
			return out[i].Brand < out[j].Brand
		}
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].FitHandle < out[j].FitHandle
	})
	return out, nil
}

// forgetAttempts is how many times an opt-out retries a partial delete
// before it reports not done.
const forgetAttempts = 3

// forgetWithRetry removes the shopper's rows across the session's full
// line list. An incomplete delete is not done: the whole list is
// retried, since deletes are idempotent, and the removed counts add up
// across the attempts. A delete that still reports failures after the
// retries fails the call.
func forgetWithRetry(ctx context.Context, writer *fitindex.Writer, lines []domain.ProductLine) (int, error) {
	total := 0
	var err error
	for attempt := 0; attempt < forgetAttempts; attempt++ {
		var removed int
		removed, err = writer.Forget(ctx, lines)
		total += removed
		if err == nil {
			return total, nil
		}
		if !errors.Is(err, fitindex.ErrIncompleteDelete) {
			return total, fmt.Errorf("server: forget fit rows: %w", err)
		}
	}
	return total, fmt.Errorf("server: forget fit rows: %w", err)
}
