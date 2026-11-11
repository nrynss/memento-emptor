package server

import (
	"context"
	"fmt"
	"sort"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// documentStore is the optional capability of an evidence store that can
// hand its documents to the journal and take them back at boot. A store
// without it still gets the ledger rows; the raw texts behind the doc ids
// are then not restored.
type documentStore interface {
	// Documents returns the stored documents in index order.
	Documents() []evidence.Document
	// RestoreDocuments installs documents under the ids they were
	// stored with.
	RestoreDocuments([]evidence.Document) error
}

// restoreEvidence loads one shopper's evidence journal into a fresh
// partition, before the boot seed replays. The rows come back with their
// ids, reasons, links, write operations, feedback markers and doc ids;
// the documents come back into the store under the same ids; the open
// questions, mint counters and brand context come back with them. A
// shopper with no journal starts empty, and a service with no standing
// directory keeps everything in memory as before.
func (s *Service) restoreEvidence(ctx context.Context, st *shopperState, shopper string) error {
	if s.evidenceJournal == nil {
		return nil
	}
	snap, found, err := s.evidenceJournal.Load(ctx, shopper)
	if err != nil {
		return fmt.Errorf("server: restore evidence for %q: %w", shopper, err)
	}
	if !found {
		return nil
	}
	if err := st.restoreEvidenceSnapshot(snap, s.deps.Store); err != nil {
		return fmt.Errorf("server: restore evidence for %q: %w", shopper, err)
	}
	return nil
}

// restoreEvidenceSnapshot rebuilds one partition from its journal
// snapshot. The mint counters go back first, so a later write never
// reuses an id the journal names; then the rows and the open questions.
// The belief citations and timelines wait in the partition until the boot
// seed has rebuilt the beliefs they belong to.
func (st *shopperState) restoreEvidenceSnapshot(snap evidence.Snapshot, store fakes.EvidenceStore) error {
	for prefix, n := range snap.Seq {
		if n > st.seq[prefix] {
			st.seq[prefix] = n
		}
	}
	for _, row := range snap.Rows {
		rec := &evidenceRecord{
			id:        row.ID,
			row:       row.Evidence,
			counts:    row.CountsThatToward,
			writeOp:   row.WriteOp,
			docID:     row.DocID,
			beliefIDs: append([]string{}, row.BeliefIDs...),
		}
		if row.Feedback != nil {
			rec.feedback = &recordedFeedback{
				id:       row.Feedback.ID,
				verdict:  row.Feedback.Verdict,
				beliefID: row.Feedback.BeliefID,
			}
		}
		st.evidence[rec.id] = rec
		st.reserveID(rec.id)
		if rec.row.Outcome.Kind == domain.OutcomeCapture {
			st.addCapture(rec.row.Link.CustomID, rec.id)
		}
	}
	for _, question := range snap.Pending {
		rec := &pendingRecord{
			id:            question.ID,
			eventID:       question.EventID,
			outcome:       question.Outcome,
			items:         append([]domain.Item{}, question.Items...),
			link:          question.Link,
			seeded:        question.Seeded,
			askedCount:    question.AskedCount,
			lastAskedAt:   question.LastAskedAt,
			awaitingSince: question.AwaitingSince,
		}
		st.pending[rec.id] = rec
		st.reserveID(rec.id)
	}
	st.restoredLinks = snap.Links
	st.restoredTimelines = snap.Timelines
	st.lastBrand = snap.LastBrand
	if restorer, ok := store.(documentStore); ok {
		if err := restorer.RestoreDocuments(snap.Documents); err != nil {
			return fmt.Errorf("server: restore evidence documents: %w", err)
		}
	}
	return nil
}

// persistEvidence saves one shopper's evidence partition when a journal
// exists to save it to. Without a standing directory the evidence stays
// in memory, which is the demo's offline shape.
func (s *Service) persistEvidence(ctx context.Context, st *shopperState, shopper string) error {
	if s.evidenceJournal == nil {
		return nil
	}
	docs := []evidence.Document{}
	if store, ok := s.deps.Store.(documentStore); ok {
		docs = store.Documents()
	}
	snap := st.evidenceSnapshot(docs)
	if len(snap.Rows) == 0 && len(snap.Documents) == 0 {
		return nil
	}
	if err := s.evidenceJournal.Save(ctx, shopper, snap); err != nil {
		return fmt.Errorf("server: persist evidence for %q: %w", shopper, err)
	}
	return nil
}

// evidenceSnapshot renders one partition in its durable form. Rows,
// questions, beliefs and timelines are ordered by id where a map has no
// order of its own, so the same state saves the same bytes; a belief's
// citations keep the order the live writes produced. Documents are kept
// only when a row cites them: an orphan left by a failed graph write is
// not evidence.
func (st *shopperState) evidenceSnapshot(docs []evidence.Document) evidence.Snapshot {
	snap := evidence.Snapshot{
		Seq:       make(map[string]int, len(st.seq)),
		LastBrand: st.lastBrand,
	}
	for prefix, n := range st.seq {
		snap.Seq[prefix] = n
	}
	referenced := map[string]bool{}
	for id, rec := range st.evidence {
		row := evidence.Row{
			ID:               id,
			Evidence:         rec.row,
			CountsThatToward: rec.counts,
			WriteOp:          rec.writeOp,
			DocID:            rec.docID,
			BeliefIDs:        append([]string{}, rec.beliefIDs...),
		}
		if rec.feedback != nil {
			row.Feedback = &evidence.Feedback{
				ID:       rec.feedback.id,
				Verdict:  rec.feedback.verdict,
				BeliefID: rec.feedback.beliefID,
			}
		}
		if rec.docID != "" {
			referenced[rec.docID] = true
		}
		snap.Rows = append(snap.Rows, row)
	}
	sort.Slice(snap.Rows, func(i, j int) bool { return snap.Rows[i].ID < snap.Rows[j].ID })
	for _, doc := range docs {
		if referenced[doc.DocID] {
			snap.Documents = append(snap.Documents, doc)
		}
	}
	questionIDs := make([]string, 0, len(st.pending))
	for id := range st.pending {
		questionIDs = append(questionIDs, id)
	}
	sort.Strings(questionIDs)
	for _, id := range questionIDs {
		rec := st.pending[id]
		snap.Pending = append(snap.Pending, evidence.Pending{
			ID:            rec.id,
			EventID:       rec.eventID,
			Outcome:       rec.outcome,
			Items:         append([]domain.Item{}, rec.items...),
			Link:          rec.link,
			Seeded:        rec.seeded,
			AskedCount:    rec.askedCount,
			LastAskedAt:   rec.lastAskedAt,
			AwaitingSince: rec.awaitingSince,
		})
	}
	beliefIDs := make([]string, 0, len(st.beliefs))
	for id := range st.beliefs {
		beliefIDs = append(beliefIDs, id)
	}
	sort.Strings(beliefIDs)
	for _, id := range beliefIDs {
		rec := st.beliefs[id]
		snap.Links = append(snap.Links, evidence.BeliefEvidence{
			BeliefID:    id,
			EvidenceIDs: append([]string{}, rec.evIDs...),
			UpdatedAt:   rec.updatedAt,
		})
	}
	timelineIDs := make([]string, 0, len(st.timelines))
	for id := range st.timelines {
		timelineIDs = append(timelineIDs, id)
	}
	sort.Strings(timelineIDs)
	for _, id := range timelineIDs {
		snap.Timelines = append(snap.Timelines, evidence.BeliefTimeline{
			BeliefID: id,
			Events:   append([]contract.TimelineEvent{}, st.timelines[id]...),
		})
	}
	return snap
}

// applyRestoredEvidence merges the journal's belief-facing state into the
// partition the boot seed rebuilt. A belief's citations gain every
// restored row it does not already name, in the order the live writes
// appended them, so an id the seed also restores is named once; the
// belief keeps the later change time the evidence wrote. A timeline gains
// every restored event it does not already hold, so explain_belief keeps
// the live history beside the replayed one. A citation naming a row or a
// belief this boot does not hold is left out rather than rendered empty.
func (st *shopperState) applyRestoredEvidence() {
	for _, link := range st.restoredLinks {
		rec, ok := st.beliefs[link.BeliefID]
		if !ok {
			continue
		}
		held := make(map[string]bool, len(rec.evIDs))
		for _, id := range rec.evIDs {
			held[id] = true
		}
		for _, id := range link.EvidenceIDs {
			if _, known := st.evidence[id]; !known || held[id] {
				continue
			}
			rec.evIDs = append(rec.evIDs, id)
			held[id] = true
		}
		if link.UpdatedAt.After(rec.updatedAt) {
			rec.updatedAt = link.UpdatedAt
		}
	}
	for _, timeline := range st.restoredTimelines {
		if _, ok := st.beliefs[timeline.BeliefID]; !ok {
			continue
		}
		held := make(map[contract.TimelineEvent]bool, len(st.timelines[timeline.BeliefID]))
		for _, event := range st.timelines[timeline.BeliefID] {
			held[event] = true
		}
		for _, event := range timeline.Events {
			if held[event] {
				continue
			}
			st.timelines[timeline.BeliefID] = append(st.timelines[timeline.BeliefID], event)
			held[event] = true
		}
	}
}

// addCapture records one capture row under its purchase's custom id,
// once. Both the journal restore and the boot replay route through it, so
// a restored capture is not listed twice and record_feedback joins the
// same way after a restart.
func (st *shopperState) addCapture(customID, id string) {
	if customID == "" {
		return
	}
	for _, held := range st.captures[customID] {
		if held == id {
			return
		}
	}
	st.captures[customID] = append(st.captures[customID], id)
}
