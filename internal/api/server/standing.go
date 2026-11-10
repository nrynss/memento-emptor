package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/memory"
	"github.com/nrynss/memento-emptor/internal/standing"
)

// restoreStanding loads one shopper's durable state into a fresh
// partition: the pooling choice, the correction views recall serves, and
// the undeclared journal entries a later belief may declare against. The
// journal entries are re-recorded, so the graph holds their notes again;
// declared corrections need no re-declaration because the restart forgot
// the beliefs they were declared against too. A shopper with no snapshot
// starts from the defaults: pooling on, no corrections, an empty journal.
func (s *Service) restoreStanding(ctx context.Context, st *shopperState, shopper string) error {
	snap, found, err := s.standing.Load(ctx, shopper)
	if err != nil {
		return fmt.Errorf("server: restore standing for %q: %w", shopper, err)
	}
	if !found {
		return nil
	}
	st.pooling.Store(snap.Pooling)
	for _, view := range snap.Corrections {
		at, err := time.Parse(time.RFC3339, view.RecordedAt)
		if err != nil {
			return fmt.Errorf("server: restore standing for %q: bad correction time: %w", shopper, err)
		}
		if _, ok := st.corrections[view.CorrectionID]; !ok {
			st.corrOrder = append(st.corrOrder, view.CorrectionID)
		}
		st.corrections[view.CorrectionID] = &correctionRecord{view: view, at: at}
	}
	for _, entry := range snap.Journal {
		standing := entry
		standing.Against = ""
		if err := st.journal.Record(ctx, s.graph, standing); err != nil {
			return fmt.Errorf("server: restore standing for %q: %w", shopper, err)
		}
	}
	return nil
}

// persistStanding saves one shopper's durable state after a mutation: the
// pooling choice, the correction views, and the undeclared journal
// entries. Declared entries are history and travel with the views that
// cite them; only standings that a later belief may declare against are
// journalled.
func (s *Service) persistStanding(ctx context.Context, st *shopperState, shopper string) error {
	views := make([]contract.CorrectionView, 0, len(st.corrOrder))
	for _, id := range st.corrOrder {
		if rec, ok := st.corrections[id]; ok {
			views = append(views, rec.view)
		}
	}
	journal := []memory.Correction{}
	for _, entry := range st.journal.Standing() {
		if entry.Against == "" {
			journal = append(journal, entry)
		}
	}
	snap := standing.Snapshot{Pooling: st.pooling.Load(), Corrections: views, Journal: journal}
	if err := s.standing.Save(ctx, shopper, snap); err != nil {
		return fmt.Errorf("server: persist standing for %q: %w", shopper, err)
	}
	return nil
}

// standingCorrection builds the journal entry for a standing correction:
// the shopper's words, the most recent brand context as its subject, the
// size it names, and its event time. A shopper with no brand context yet
// records an unscoped correction: it applies to every brand in recall
// until one is known, and it is never declared because the match needs a
// named subject on both sides.
func standingCorrection(text string, subject domain.Subject, size string, at time.Time) memory.Correction {
	return memory.Correction{
		Text:       text,
		Subject:    subject,
		Size:       size,
		RecordedAt: at,
	}
}

// correctionSubject returns the subject a standing correction is recorded
// about: the shopper's most recent brand context. It is empty while the
// layer knows no brand for the shopper, and the correction is then
// unscoped.
func correctionSubject(st *shopperState) domain.Subject {
	if strings.TrimSpace(st.lastBrand.ID) == "" {
		return domain.Subject{}
	}
	return st.lastBrand
}

// memoryBelief renders one stored belief in the shapes the journal
// matches on: the graph content, the kind, the subject, and the zero
// line, which spans every line of its subject.
func memoryBelief(rec *beliefRecord) memory.Belief {
	return memory.Belief{
		Content: rec.content,
		Kind:    rec.kind,
		Subject: rec.subject,
	}
}

// backfillDeclaration links the correction views to a declaration the
// journal just made: every journal entry declared against the belief's
// content names the belief in the matching view, the belief is touched,
// and its timeline gains the correction row. The caller holds the lock.
func (s *Service) backfillDeclaration(st *shopperState, belief *beliefRecord, now time.Time) {
	for _, entry := range st.journal.Standing() {
		if entry.Against != belief.content {
			continue
		}
		for _, id := range st.corrOrder {
			rec := st.corrections[id]
			if rec == nil || rec.view.ContradictsBeliefID != "" || rec.view.Text != entry.Text {
				continue
			}
			rec.view.ContradictsBeliefID = belief.id
			rec.at = entry.RecordedAt
		}
		belief.touch(now)
		st.timeline(belief.id, contract.TimelineEvent{
			At:      now.Format(time.RFC3339),
			Kind:    contract.TimelineCorrection,
			Summary: fmt.Sprintf("Shopper correction: %s.", entry.Text),
			RefID:   belief.id,
		})
	}
}
