package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/memory"
)

// ErrAlreadySeeded is returned when a seed targets a shopper that already
// holds beliefs. Seeded history loads once; live writes follow it.
var ErrAlreadySeeded = errors.New("server: shopper already holds beliefs")

// SeedGraphContents lists every graph content a seeded service writes:
// belief contents, corrections verbatim, and the doc stubs for the
// seeded evidence plus the one live write each open question can still
// produce. The offline demo builds its graph fake over this closed
// world; anything outside it fails with no recording instead of
// inventing history.
func SeedGraphContents(seed Seed) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(content string) {
		if !seen[content] {
			seen[content] = true
			out = append(out, content)
		}
	}
	for _, belief := range seed.Beliefs {
		add(beliefContent(belief.Kind, belief.Subject, belief.Value))
		if belief.ContradictedBy != nil {
			add(belief.ContradictedBy.Text)
		}
	}
	type keyed struct {
		at  string
		key string
	}
	var keys []keyed
	for _, belief := range seed.Beliefs {
		for _, ev := range belief.Evidence {
			keys = append(keys, keyed{at: ev.OccurredAt, key: string(ev.Outcome.Kind) + " " + ev.Link.EventID})
		}
	}
	for _, question := range seed.Pending.Questions {
		keys = append(keys, keyed{at: question.AwaitingSince, key: string(question.Outcome.Kind) + " " + question.Link.EventID})
	}
	sort.SliceStable(keys, func(i, j int) bool { return keys[i].at < keys[j].at })
	seenKey := map[string]bool{}
	unique := []string{}
	for _, key := range keys {
		if !seenKey[key.key] {
			seenKey[key.key] = true
			unique = append(unique, key.key)
		}
	}
	for i := 1; i <= len(keys)+2; i++ {
		for _, key := range unique {
			add(fmt.Sprintf("evidence[doc-%04d] %s", i, key))
		}
	}
	return out
}

// Seed is one shopper's loading history: beliefs with their evidence,
// the ledger's evidence links, the open questions, the timelines, the
// seeded notices, and the verdicts behind feedback rows. The demo loader
// and the contract tests build it from the frozen fixtures; the service
// replays it through the real write path so every reading comes from the
// graph exactly like live writes.
type Seed struct {
	// ShopperID scopes the seed to one shopper.
	ShopperID string
	// Beliefs are the ledger rows with embedded evidence and warnings.
	Beliefs []contract.BeliefView
	// Links maps each evidence id to the beliefs it counts toward.
	Links map[string][]string
	// Pending is the open questions at the end of the history.
	Pending contract.PendingQuestionsResponse
	// Timelines carries each belief's history in order.
	Timelines map[string][]contract.TimelineEvent
	// Notices carries each seeded belief's notice in words.
	Notices map[string]string
	// FeedbackVerdicts carries the verdict behind each feedback row, by
	// evidence id. Feedback has no reason category; the verdict decides
	// what it counts toward.
	FeedbackVerdicts map[string]contract.FeedbackVerdict
	// FeedbackIDs carries the id the history recorded each feedback row
	// under, by evidence id. A seeded feedback row whose history names no
	// id has none, and a repeat delivery for its purchase then answers
	// with no id rather than one the layer invented.
	FeedbackIDs map[string]string
}

// parseTime parses one RFC 3339 time or fails the seed.
func parseTime(raw, what string) (time.Time, error) {
	moment, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("server: seed: bad time in %s %q: %w", what, raw, err)
	}
	return moment, nil
}

// domainRow rebuilds the domain row behind one wire evidence view.
func domainRow(view contract.EvidenceView) (domain.Evidence, error) {
	occurred, err := parseTime(view.OccurredAt, "evidence "+view.EvidenceID)
	if err != nil {
		return domain.Evidence{}, err
	}
	row := domain.Evidence{
		Outcome: domain.Outcome{
			Kind:        view.Outcome.Kind,
			Status:      view.Outcome.Status,
			AmountMinor: view.Outcome.AmountMinor,
			Currency:    view.Outcome.Currency,
		},
		Item: domain.Item{
			BrandID:   view.Item.BrandID,
			BrandName: view.Item.BrandName,
			LineName:  view.Item.LineName,
			SKU:       view.Item.SKU,
			Size:      view.Item.Size,
			Category:  view.Item.Category,
		},
		Link: domain.TransactionLink{
			OrderID:   view.Link.OrderID,
			CaptureID: view.Link.CaptureID,
			RefundID:  view.Link.RefundID,
			DisputeID: view.Link.DisputeID,
			CustomID:  view.Link.CustomID,
			EventID:   view.Link.EventID,
		},
		Seeded:     view.Seeded,
		OccurredAt: occurred,
	}
	if view.Reason != nil {
		row.Reason = &domain.Reason{
			Category:   view.Reason.Category,
			Text:       view.Reason.Text,
			Confidence: view.Reason.Confidence,
		}
	}
	return row, nil
}

// LoadSeed replays one shopper's history through the real write path:
// beliefs derive, evidence indexes with its doc id in the graph stub,
// corrections declare against, and timelines and notices load verbatim.
// Readings then come from the graph, so seeded and live rows obey the
// same bars.
func (s *Service) LoadSeed(ctx context.Context, seed Seed) error {
	if seed.ShopperID == "" {
		return fmt.Errorf("server: load seed: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.state(ctx, seed.ShopperID)
	if err != nil {
		return fmt.Errorf("server: load seed: %w", err)
	}
	if len(st.beliefs) > 0 {
		return fmt.Errorf("server: load seed for %q: %w", seed.ShopperID, ErrAlreadySeeded)
	}

	type pendingEvidence struct {
		belief *beliefRecord
		rec    *evidenceRecord
	}
	var ordered []*pendingEvidence

	for _, view := range seed.Beliefs {
		updated, err := parseTime(view.UpdatedAt, "belief "+view.BeliefID)
		if err != nil {
			return err
		}
		earliest := updated
		consider := func(at time.Time) {
			if at.Before(earliest) {
				earliest = at
			}
		}
		for _, ev := range view.Evidence {
			at, err := parseTime(ev.OccurredAt, "evidence "+ev.EvidenceID)
			if err != nil {
				return err
			}
			consider(at)
		}
		if view.ContradictedBy != nil {
			at, err := parseTime(view.ContradictedBy.RecordedAt, "correction "+view.ContradictedBy.CorrectionID)
			if err != nil {
				return err
			}
			consider(at)
		}
		content := beliefContent(view.Kind, view.Subject, view.Value)
		rec := &beliefRecord{
			id:        view.BeliefID,
			kind:      view.Kind,
			subject:   view.Subject,
			value:     view.Value,
			direction: domain.DirectionForValue(view.Value),
			seeded:    view.Seeded,
			createdAt: earliest,
			updatedAt: updated,
			content:   content,
		}
		if len(view.Evidence) == 0 {
			if err := memory.RecordStatement(ctx, s.graph, content, earliest); err != nil {
				return fmt.Errorf("server: load seed: record statement: %w", err)
			}
		} else if _, err := memory.EnsureBeliefDeclaring(ctx, s.graph, st.journal, memory.Belief{
			Content: content,
			Kind:    rec.kind,
			Subject: rec.subject,
		}, earliest); err != nil {
			return fmt.Errorf("server: load seed: ensure belief: %w", err)
		}
		st.beliefs[rec.id] = rec
		for _, ev := range view.Evidence {
			row, err := domainRow(ev)
			if err != nil {
				return err
			}
			counts := domain.BeliefKindNone
			if ev.CountsToward != nil {
				counts = *ev.CountsToward
			}
			evRec := &evidenceRecord{
				id:        ev.EvidenceID,
				row:       row,
				counts:    counts,
				writeOp:   ev.WriteOp,
				beliefIDs: append([]string(nil), seed.Links[ev.EvidenceID]...),
			}
			if len(evRec.beliefIDs) == 0 {
				evRec.beliefIDs = []string{rec.id}
			}
			st.evidence[evRec.id] = evRec
			rec.evIDs = append(rec.evIDs, evRec.id)
			if row.Outcome.Kind == domain.OutcomeCapture {
				st.captures[row.Link.CustomID] = append(st.captures[row.Link.CustomID], evRec.id)
			}
			ordered = append(ordered, &pendingEvidence{belief: rec, rec: evRec})
		}
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].rec.row.OccurredAt.Before(ordered[j].rec.row.OccurredAt)
	})
	for _, item := range ordered {
		docID, err := s.indexEvidence(ctx, item.rec.row, "")
		if err != nil {
			return err
		}
		stub, err := evidence.GraphStub(docID, stubKey(item.rec.row))
		if err != nil {
			return err
		}
		item.rec.docID = docID
		if item.rec.row.Outcome.Kind == domain.OutcomeCapture {
			verdict, ok := seed.FeedbackVerdicts[item.rec.id]
			if !ok {
				return fmt.Errorf("server: load seed: no verdict for feedback %q", item.rec.id)
			}
			counts, err := memory.VerdictCountsToward(verdict)
			if err != nil {
				return err
			}
			if counts != domain.BeliefKindNone {
				if _, err := memory.WriteFeedback(ctx, s.graph, item.belief.content, item.belief.kind, stub, verdict, item.rec.row.OccurredAt); err != nil {
					return fmt.Errorf("server: load seed: write feedback: %w", err)
				}
				// The history holds this purchase's feedback already, so
				// the row carries its identity: a later delivery for the
				// purchase is a repeat, not that purchase's first
				// feedback.
				item.rec.feedback = &recordedFeedback{
					id:       seed.FeedbackIDs[item.rec.id],
					verdict:  verdict,
					beliefID: item.belief.id,
				}
			}
			continue
		}
		if _, err := memory.WriteEvidence(ctx, s.graph, item.belief.content, item.belief.kind, item.belief.direction, stub, item.rec.row); err != nil {
			return fmt.Errorf("server: load seed: write evidence: %w", err)
		}
	}

	type seedCorrection struct {
		view contract.CorrectionView
		at   time.Time
	}
	seen := map[string]bool{}
	corrections := []seedCorrection{}
	for _, view := range seed.Beliefs {
		if view.ContradictedBy == nil || seen[view.ContradictedBy.CorrectionID] {
			continue
		}
		seen[view.ContradictedBy.CorrectionID] = true
		at, err := parseTime(view.ContradictedBy.RecordedAt, "correction "+view.ContradictedBy.CorrectionID)
		if err != nil {
			return err
		}
		corrections = append(corrections, seedCorrection{view: *view.ContradictedBy, at: at})
	}
	sort.Slice(corrections, func(i, j int) bool { return corrections[i].at.Before(corrections[j].at) })
	for _, corr := range corrections {
		belief, ok := st.beliefs[corr.view.ContradictsBeliefID]
		if !ok {
			return fmt.Errorf("server: load seed: %w %q", ErrUnknownBelief, corr.view.ContradictsBeliefID)
		}
		if err := memory.RecordCorrection(ctx, s.graph, corr.view.Text, belief.content, corr.at); err != nil {
			return fmt.Errorf("server: load seed: record correction: %w", err)
		}
		st.takeCorrection(corr.view.CorrectionID, &correctionRecord{view: corr.view, at: corr.at})
	}

	for id, timeline := range seed.Timelines {
		st.timelines[id] = append([]contract.TimelineEvent(nil), timeline...)
	}
	for id, notice := range seed.Notices {
		st.notices[id] = notice
	}
	for _, question := range seed.Pending.Questions {
		awaiting, err := parseTime(question.AwaitingSince, "question "+question.QuestionID)
		if err != nil {
			return err
		}
		var lastAsked *time.Time
		if question.LastAskedAt != nil {
			at, err := parseTime(*question.LastAskedAt, "question "+question.QuestionID)
			if err != nil {
				return err
			}
			lastAsked = &at
		}
		st.pending[question.QuestionID] = &pendingRecord{
			id:      question.QuestionID,
			eventID: question.Link.EventID,
			outcome: domain.Outcome{
				Kind:        question.Outcome.Kind,
				Status:      question.Outcome.Status,
				AmountMinor: question.Outcome.AmountMinor,
				Currency:    question.Outcome.Currency,
			},
			items: []domain.Item{{
				BrandID:   question.Item.BrandID,
				BrandName: question.Item.BrandName,
				LineName:  question.Item.LineName,
				SKU:       question.Item.SKU,
				Size:      question.Item.Size,
				Category:  question.Item.Category,
			}},
			link: domain.TransactionLink{
				OrderID:   question.Link.OrderID,
				CaptureID: question.Link.CaptureID,
				RefundID:  question.Link.RefundID,
				DisputeID: question.Link.DisputeID,
				CustomID:  question.Link.CustomID,
				EventID:   question.Link.EventID,
			},
			seeded:        question.Seeded,
			askedCount:    question.AskedCount,
			lastAskedAt:   lastAsked,
			awaitingSince: awaiting,
		}
	}
	return nil
}

// readExample reads one frozen fixture file.
func readExample(apiDir, name string) (json.RawMessage, error) {
	raw, err := os.ReadFile(filepath.Join(apiDir, name))
	if err != nil {
		return nil, fmt.Errorf("server: read example %s: %w", name, err)
	}
	return raw, nil
}

// SeedFromExamples builds a seed from the frozen contract fixtures: the
// ledger rows, their evidence links, the open questions, the explained
// timeline, and the seeded notice. Feedback rows carry the fits-well
// verdict behind the fixture's shape: a capture counting toward fit is
// the worn size working out. A feedback row also takes the id the
// fixtures record for it: the record_feedback example pair names the
// purchase and the feedback id, and the explained timeline repeats that
// id, so one purchase's feedback has one identity across the set.
func SeedFromExamples(apiDir string) (Seed, error) {
	var seed Seed
	decode := func(name string, target any) error {
		raw, err := readExample(apiDir, name)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return fmt.Errorf("server: parse example %s: %w", name, err)
		}
		return nil
	}
	var beliefs contract.LedgerBeliefsResponse
	if err := decode("ledger_beliefs.response.example.json", &beliefs); err != nil {
		return Seed{}, err
	}
	var ledgerEvidence contract.LedgerEvidenceResponse
	if err := decode("ledger_evidence.response.example.json", &ledgerEvidence); err != nil {
		return Seed{}, err
	}
	var pending contract.PendingQuestionsResponse
	if err := decode("pending_questions.response.example.json", &pending); err != nil {
		return Seed{}, err
	}
	var explained contract.ExplainBeliefResponse
	if err := decode("explain_belief.response.example.json", &explained); err != nil {
		return Seed{}, err
	}
	var ledgerBelief contract.LedgerBeliefResponse
	if err := decode("ledger_belief.response.example.json", &ledgerBelief); err != nil {
		return Seed{}, err
	}
	var feedbackReq contract.RecordFeedbackRequest
	if err := decode("record_feedback.request.example.json", &feedbackReq); err != nil {
		return Seed{}, err
	}
	var feedbackRes contract.RecordFeedbackResponse
	if err := decode("record_feedback.response.example.json", &feedbackRes); err != nil {
		return Seed{}, err
	}
	seed.ShopperID = beliefs.ShopperID
	seed.Beliefs = make([]contract.BeliefView, 0, len(beliefs.Beliefs))
	for _, row := range beliefs.Beliefs {
		seed.Beliefs = append(seed.Beliefs, row.Belief)
	}
	seed.Links = map[string][]string{}
	for _, row := range ledgerEvidence.Evidence {
		seed.Links[row.Evidence.EvidenceID] = append([]string(nil), row.BeliefIDs...)
	}
	seed.Pending = pending
	seed.Timelines = map[string][]contract.TimelineEvent{
		explained.Belief.BeliefID: append([]contract.TimelineEvent(nil), explained.Timeline...),
	}
	seed.Notices = map[string]string{}
	if ledgerBelief.SeededNotice != nil {
		seed.Notices[ledgerBelief.Belief.BeliefID] = *ledgerBelief.SeededNotice
	}
	seed.FeedbackVerdicts = map[string]contract.FeedbackVerdict{}
	seed.FeedbackIDs = map[string]string{}
	for _, belief := range seed.Beliefs {
		for _, ev := range belief.Evidence {
			if ev.Outcome.Kind != domain.OutcomeCapture || ev.CountsToward == nil || *ev.CountsToward != domain.BeliefKindFit {
				continue
			}
			seed.FeedbackVerdicts[ev.EvidenceID] = contract.FeedbackFitsWell
			if feedbackNames(feedbackReq, seed.ShopperID, ev) {
				seed.FeedbackIDs[ev.EvidenceID] = feedbackRes.FeedbackID
			}
		}
	}
	return seed, nil
}

// feedbackNames reports whether the record_feedback example is about one
// seeded evidence row: the same shopper, the same purchase and the same
// worn size. It is the same join record_feedback itself makes, so the
// fixture names the feedback of the row its request describes.
func feedbackNames(req contract.RecordFeedbackRequest, shopper string, ev contract.EvidenceView) bool {
	return req.ShopperID == shopper &&
		req.CustomID == ev.Link.CustomID &&
		req.SKU == ev.Item.SKU &&
		req.Size == ev.Item.Size
}
