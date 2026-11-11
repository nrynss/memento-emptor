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
// partition: the pooling choice, the correction views recall serves, the
// view links that pair declared views with their journal entries, and
// the journal entries a later belief may declare against. The journal
// entries are re-recorded as standings, so the graph holds their notes
// again; declared views keep their stale belief ids until a rewritten
// belief re-declares the paired entry and re-links them by entry match.
// A shopper with no snapshot starts from the defaults: pooling on, no
// corrections, an empty journal, no pairing.
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
	for _, link := range snap.Links {
		st.pairing[link.CorrectionID] = link
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
// pooling choice, the correction views, the journal entries a later
// belief may declare against, and the links pairing each declared view
// with its entry. Declared entries are saved as standings (with no
// against): the restart forgot the beliefs they were declared against,
// so they wait for the rewritten belief to declare them again. A declared
// view with no journal entry — a correction recorded straight against its
// belief — gains a synthesized cover entry from the live belief's subject,
// or it could never re-link after a restart. Entries are deduplicated by
// their words and subject, so a cover never doubles a journal entry. Each
// declared view is paired with the entry it was declared against — the
// journal entry carrying the same words against the cited belief's
// content, or the cover when the correction never journalled — so a
// rebuild re-links exactly: the same words about two subjects stay
// attached to their own beliefs. A declared view whose belief is gone
// (the restart-before-redelivery window) keeps its existing pairing, so
// recall can still honour it by subject in the meantime.
func (s *Service) persistStanding(ctx context.Context, st *shopperState, shopper string) error {
	views := make([]contract.CorrectionView, 0, len(st.corrOrder))
	for _, id := range st.corrOrder {
		if rec, ok := st.corrections[id]; ok {
			views = append(views, rec.view)
		}
	}
	journal := []memory.Correction{}
	for _, entry := range st.journal.Standing() {
		entry.Against = ""
		if !journalCoversTextSubject(journal, entry.Text, entry.Subject) {
			journal = append(journal, entry)
		}
	}
	for _, id := range st.corrOrder {
		rec, ok := st.corrections[id]
		if !ok || rec.view.ContradictsBeliefID == "" {
			continue
		}
		belief, ok := st.beliefs[rec.view.ContradictsBeliefID]
		if !ok {
			continue
		}
		if link, paired := st.pairing[id]; paired && (link.Subject.Kind != belief.subject.Kind || link.Subject.ID != belief.subject.ID) {
			// The citation is stale and its id has been reused for an
			// unrelated belief: the rewritten belief has not declared
			// the paired entry yet. Pairing the view to the stranger
			// and covering it would corrupt the journal, so the view
			// keeps its pairing until its own entry declares.
			continue
		}
		if !journalCoversTextSubject(journal, rec.view.Text, belief.subject) {
			journal = append(journal, memory.Correction{
				Text:       rec.view.Text,
				Subject:    belief.subject,
				Size:       lastSize(rec.view.Text),
				RecordedAt: rec.at,
			})
		}
		pairDeclared(st, id, rec.view.Text, belief)
	}
	links := make([]standing.ViewLink, 0, len(st.pairing))
	for _, id := range st.corrOrder {
		if link, ok := st.pairing[id]; ok {
			links = append(links, link)
		}
	}
	snap := standing.Snapshot{Pooling: st.pooling.Load(), Corrections: views, Journal: journal, Links: links}
	if err := s.standing.Save(ctx, shopper, snap); err != nil {
		return fmt.Errorf("server: persist standing for %q: %w", shopper, err)
	}
	return nil
}

// pairDeclared pairs one declared view with the journal entry it was
// declared against: the entry carrying the same words against the cited
// belief's content, or a cover over the cited belief's subject when the
// correction never journalled (a correction recorded straight against its
// belief). The cover key matches the synthesized cover entry persist
// appends beside it, so the pairing survives the round trip. The pairing
// is what the rebuild re-links by, so two views carrying the same words
// about different subjects never cross-link.
func pairDeclared(st *shopperState, id, text string, belief *beliefRecord) {
	for _, entry := range st.journal.Standing() {
		if entry.Against == belief.content && entry.Text == text {
			st.pairing[id] = standing.ViewLink{
				CorrectionID: id,
				Text:         entry.Text,
				Subject:      entry.Subject,
				Line:         entry.Line,
				Size:         entry.Size,
			}
			return
		}
	}
	st.pairing[id] = standing.ViewLink{
		CorrectionID: id,
		Text:         text,
		Subject:      belief.subject,
		Size:         lastSize(text),
	}
}

// journalCoversTextSubject reports whether any journal entry — standing or
// declared — carries the same words about the same subject.
func journalCoversTextSubject(journal []memory.Correction, text string, subject domain.Subject) bool {
	for _, entry := range journal {
		if entry.Text == text && entry.Subject == subject {
			return true
		}
	}
	return false
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
// journal just made. A paired view follows only its own entry: the entry
// that just declared against the rewritten belief re-links exactly the
// views paired to it, so the same words declared against two subjects
// stay attached to their own beliefs however the rebuild reorders the
// mints. A view from before the pairing existed falls back to the old
// words match, leaving alone a citation the journal still explains. The
// belief is touched, and its timeline gains the correction row. The
// caller holds the lock.
func (s *Service) backfillDeclaration(st *shopperState, belief *beliefRecord, now time.Time) {
	for _, entry := range st.journal.Standing() {
		if entry.Against != belief.content {
			continue
		}
		for _, id := range st.corrOrder {
			rec := st.corrections[id]
			if rec == nil {
				continue
			}
			if link, paired := st.pairing[id]; paired {
				if !standing.LinkKey(link, entry) {
					continue
				}
			} else {
				if rec.view.Text != entry.Text {
					continue
				}
				if rec.view.ContradictsBeliefID != "" {
					// A view naming a belief the layer still holds is left
					// alone when its citation is explained: a journal entry
					// with the same words about that belief's subject means
					// the citation came from this entry's lineage.
					if cited, ok := st.beliefs[rec.view.ContradictsBeliefID]; ok &&
						journalCoversTextSubject(st.journal.Standing(), rec.view.Text, cited.subject) {
						continue
					}
				}
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
