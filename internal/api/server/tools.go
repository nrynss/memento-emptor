package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/ingest"
	"github.com/nrynss/memento-emptor/internal/memory"
	"github.com/nrynss/memento-emptor/internal/precedence"
	"github.com/nrynss/memento-emptor/internal/reasons"
)

// sellerBeliefValue is the value of a new seller-reliability belief. One
// dispute never promotes it; the ledger shows it as Claimed.
const sellerBeliefValue = "ships reliably for this shopper"

// WebhookResult reports what one delivery stored.
type WebhookResult struct {
	// EventID is the payment source's event id.
	EventID string `json:"event_id"`
	// Rows counts the stored evidence rows for the delivery.
	Rows int `json:"rows"`
	// Pending is true when the delivery filed a pending question.
	Pending bool `json:"pending"`
	// Duplicate is true when the event id was already stored.
	Duplicate bool `json:"duplicate"`
}

// IngestWebhook verifies one delivery and stores its evidence rows,
// idempotent on the payment source's event id. Refunds file a pending
// question; disputes arrive classified and write to the seller belief at
// once; captures store for the feedback join. Every delivery is logged.
// Purchases that touch the pool write their fit rows before anything is
// stored, from the same event, so a failed fit write fails the delivery
// and the retry writes both. A redelivery after a failed first attempt
// repairs the fit rows idempotently; a redelivery of a stored event
// writes nothing new.
func (s *Service) IngestWebhook(ctx context.Context, shopper, eventID string) (WebhookResult, error) {
	if strings.TrimSpace(shopper) == "" {
		return WebhookResult{}, fmt.Errorf("server: ingest: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log("webhook_ingest", shopper, nil)
	st, err := s.state(ctx, shopper)
	if err != nil {
		return WebhookResult{}, fmt.Errorf("server: partition for %q: %w", shopper, err)
	}
	res, err := st.ingest.Handle(ctx, eventID)
	if err != nil {
		return WebhookResult{}, fmt.Errorf("server: ingest webhook %q: %w", eventID, err)
	}
	return s.storeIngestResult(ctx, st, shopper, eventID, res)
}

// IngestDelivery verifies one raw webhook delivery and stores its
// evidence rows, idempotent on the payment source's event id. This is
// the deployed route's path: verification runs before the idempotency
// check, so a forged body naming a stored event id is refused, never
// answered as a duplicate.
func (s *Service) IngestDelivery(ctx context.Context, shopper string, delivery ingest.Delivery) (WebhookResult, error) {
	if strings.TrimSpace(shopper) == "" {
		return WebhookResult{}, fmt.Errorf("server: ingest: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log("webhook_ingest", shopper, nil)
	st, err := s.state(ctx, shopper)
	if err != nil {
		return WebhookResult{}, fmt.Errorf("server: partition for %q: %w", shopper, err)
	}
	res, err := st.ingest.HandleDelivery(ctx, delivery)
	if err != nil {
		return WebhookResult{}, fmt.Errorf("server: ingest delivery: %w", err)
	}
	var envelope struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(delivery.Body, &envelope); err != nil || strings.TrimSpace(envelope.ID) == "" {
		return WebhookResult{}, fmt.Errorf("server: ingest delivery: %w", ingest.ErrMalformedEvent)
	}
	return s.storeIngestResult(ctx, st, shopper, envelope.ID, res)
}

// storeIngestResult stores one delivery's rows in the shopper's partition
// after the fit rows are written. The caller holds the service lock.
func (s *Service) storeIngestResult(ctx context.Context, st *shopperState, shopper, eventID string, res ingest.Result) (WebhookResult, error) {
	out := WebhookResult{EventID: eventID, Rows: len(res.Rows), Duplicate: res.Duplicate}
	if res.Duplicate && st.webhooks[eventID] {
		out.Pending = res.Pending != nil
		return out, nil
	}
	if err := s.recordFitRows(ctx, st, res.Rows); err != nil {
		return WebhookResult{}, err
	}
	for _, row := range res.Rows {
		rec := &evidenceRecord{id: st.mint("ev"), row: row, counts: row.CountsToward()}
		st.evidence[rec.id] = rec
		if row.Outcome.Kind == domain.OutcomeCapture {
			st.captures[row.Link.CustomID] = append(st.captures[row.Link.CustomID], rec.id)
		}
		if row.Reason != nil && row.CountsToward() != domain.BeliefKindNone {
			if err := s.attachRow(ctx, st, rec); err != nil {
				return WebhookResult{}, err
			}
		}
	}
	if res.Pending != nil {
		question := res.Pending
		id := st.mint("q")
		st.pending[id] = &pendingRecord{
			id:            id,
			eventID:       question.EventID,
			outcome:       question.Outcome,
			items:         question.Items,
			link:          question.Link,
			seeded:        false,
			awaitingSince: question.OccurredAt,
		}
		out.Pending = true
	}
	for _, row := range res.Rows {
		if strings.TrimSpace(row.Item.BrandID) != "" {
			st.lastBrand = domain.Subject{Kind: domain.SubjectBrand, ID: row.Item.BrandID, Name: row.Item.BrandName}
			break
		}
	}
	st.webhooks[eventID] = true
	if err := s.persistIfJournal(ctx, st, shopper); err != nil {
		return WebhookResult{}, err
	}
	return out, nil
}

// attachRow writes one reasoned row to its belief, creating the belief on
// first sight. Only outcomes with a matching reason reach here: the
// writer refuses cross-kind writes, and reasonless rows never call it.
func (s *Service) attachRow(ctx context.Context, st *shopperState, rec *evidenceRecord) error {
	counts := rec.row.CountsToward()
	var belief *beliefRecord
	var err error
	if counts == domain.BeliefKindSeller {
		belief, err = s.ensureSellerBelief(ctx, st, rec.row.Item, rec.row.OccurredAt)
	} else {
		belief, err = s.ensureFitBelief(ctx, st, rec.row.Item, rec.row.Reason.Category, rec.row.OccurredAt)
	}
	if err != nil {
		return err
	}
	prev, err := s.readBelief(ctx, belief.content)
	if err != nil {
		return err
	}
	if err := s.writeEvidenceRow(ctx, st, belief, rec, stubKey(rec.row)); err != nil {
		return err
	}
	s.appendStatusRow(ctx, st, belief, prev)
	st.timeline(belief.id, contract.TimelineEvent{
		At:      rec.row.OccurredAt.Format(time.RFC3339),
		Kind:    contract.TimelineEvidence,
		Summary: evidenceSummary(rec.row, len(belief.evIDs)),
		RefID:   rec.id,
	})
	return nil
}

// appendStatusRow records a status change when a write moves a belief.
func (s *Service) appendStatusRow(ctx context.Context, st *shopperState, belief *beliefRecord, prev memory.Reading) {
	next, err := s.readBelief(ctx, belief.content)
	if err != nil || next.Status == prev.Status {
		return
	}
	st.timeline(belief.id, contract.TimelineEvent{
		At:      s.deps.Clock().Format(time.RFC3339),
		Kind:    contract.TimelineStatus,
		Summary: fmt.Sprintf("Belief reached %s.", string(next.Status)),
		RefID:   belief.id,
	})
}

// evidenceSummary states one evidence row in one line for the timeline.
func evidenceSummary(row domain.Evidence, n int) string {
	line := strings.TrimSpace(row.Item.LineName)
	size := strings.TrimSpace(row.Item.Size)
	reason := ""
	if row.Reason != nil {
		reason = strings.TrimSpace(row.Reason.Text)
	}
	switch row.Outcome.Kind {
	case domain.OutcomeDispute:
		return fmt.Sprintf("Dispute for %s: %s.", line, reason)
	case domain.OutcomeCapture:
		return fmt.Sprintf("Feedback on %s size %s: %s.", line, size, reason)
	default:
		return fmt.Sprintf("Fit return %d: %s size %s refunded; %s.", n, line, size, reason)
	}
}

// ensureFitBelief returns the fit belief for a brand, creating it from
// the first classified direction when nothing exists yet. The new belief
// points the creating category's way.
func (s *Service) ensureFitBelief(ctx context.Context, st *shopperState, item domain.Item, category domain.Category, at time.Time) (*beliefRecord, error) {
	for _, rec := range st.beliefs {
		if rec.kind == domain.BeliefKindFit && rec.subject.ID == item.BrandID {
			return rec, nil
		}
	}
	subject := domain.Subject{Kind: domain.SubjectBrand, ID: item.BrandID, Name: item.BrandName}
	return s.newBelief(ctx, st, domain.BeliefKindFit, subject, fitBeliefValue(category, item.BrandName, item.Size), category.Direction(), false, at)
}

// ensureSellerBelief returns the seller belief behind a brand's
// transaction, creating it on first sight.
func (s *Service) ensureSellerBelief(ctx context.Context, st *shopperState, item domain.Item, at time.Time) (*beliefRecord, error) {
	subject := sellerSubject(item)
	for _, rec := range st.beliefs {
		if rec.kind == domain.BeliefKindSeller && rec.subject.ID == subject.ID {
			return rec, nil
		}
	}
	return s.newBelief(ctx, st, domain.BeliefKindSeller, subject, sellerBeliefValue, domain.FitDirectionNone, false, at)
}

// newBelief derives one belief in the graph and stores its record. The
// event time travels with the write so honestly spread history promotes.
// Every belief written this way declares the matching standing
// corrections against it, once each, so a correction recorded before its
// belief still contradicts it when the belief arrives.
func (s *Service) newBelief(ctx context.Context, st *shopperState, kind domain.BeliefKind, subject domain.Subject, value string, direction domain.FitDirection, seeded bool, at time.Time) (*beliefRecord, error) {
	content := beliefContent(kind, subject, value)
	declared, err := memory.EnsureBeliefDeclaring(ctx, s.graph, st.journal, memory.Belief{
		Content: content,
		Kind:    kind,
		Subject: subject,
	}, at)
	if err != nil {
		return nil, fmt.Errorf("server: ensure belief: %w", err)
	}
	id := st.mint("blf")
	rec := &beliefRecord{
		id:        id,
		kind:      kind,
		subject:   subject,
		value:     value,
		direction: direction,
		seeded:    seeded,
		createdAt: at,
		updatedAt: at,
		content:   content,
	}
	st.beliefs[id] = rec
	if declared > 0 {
		s.backfillDeclaration(st, rec, at)
	}
	return rec, nil
}

// Recall returns what applies to a purchase: the shopper's own beliefs
// with their status and precedence step, the pooled product fit when the
// pool has something to say, and any correction that overrides them. The
// correction ships both at the top level and inline, with the guidance
// pointing at it, so an agent cannot recommend from the belief alone.
// Recall never refuses: thin evidence comes back labelled thin, and a
// pool with nothing to say leaves a null pool with an unavailable lookup
// rather than an empty pool.
func (s *Service) Recall(ctx context.Context, req contract.RecallRequest) (contract.RecallResponse, error) {
	if strings.TrimSpace(req.ShopperID) == "" {
		return contract.RecallResponse{}, fmt.Errorf("server: recall: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolRecallForPurchase), req.ShopperID, traceOf(req.TraceID))
	st, err := s.state(ctx, req.ShopperID)
	if err != nil {
		return contract.RecallResponse{}, fmt.Errorf("server: partition for %q: %w", req.ShopperID, err)
	}
	if strings.TrimSpace(req.BrandID) != "" {
		st.lastBrand = domain.Subject{Kind: domain.SubjectBrand, ID: req.BrandID, Name: brandName(st, req.BrandID)}
	}
	views := []contract.BeliefView{}
	for _, rec := range st.beliefs {
		view, err := s.beliefView(ctx, st, rec)
		if err != nil {
			return contract.RecallResponse{}, fmt.Errorf("server: recall: %w", err)
		}
		views = append(views, view)
	}
	served := recallBeliefs(views, req.BrandID)
	usual := usualSize(st, served)
	poolView, lookup := s.lookupPool(ctx, req, usual)
	overriding := overridingCorrection(st, served, req.BrandID)
	presence := precedence.Presence{
		Correction: overriding != nil,
		Pool:       poolView != nil,
	}
	for _, view := range served {
		if beliefStep(view) == contract.GuidanceOwnEvidence {
			presence.OwnEvidence = true
		} else {
			presence.Statement = true
		}
	}
	outcome := precedence.Decide(presence)
	for i := range served {
		step := beliefStep(served[i])
		view := precedence.View(step, outcome.Winner)
		served[i].Precedence = &view
	}
	if poolView != nil {
		view := precedence.View(contract.GuidancePool, outcome.Winner)
		poolView.Precedence = &view
	}
	name := brandName(st, req.BrandID)
	if strings.TrimSpace(req.BrandID) != "" && name == "" {
		name = req.BrandID
	}
	var guidance contract.GuidanceView
	switch outcome.Winner {
	case contract.GuidanceCorrection:
		guidance = guidanceForCorrection(*overriding, correctionBrand(st, overriding), correctionBelief(st, overriding))
	case contract.GuidanceOwnEvidence:
		guidance = guidanceForOwnEvidence(strongestOwn(served), poolView != nil && outcome.Overridden[contract.GuidancePool] == contract.GuidanceOwnEvidence)
	case contract.GuidancePool:
		guidance = guidanceForPool(*poolView, name)
	case contract.GuidanceStatement:
		guidance = guidanceForStatement(strongestStatement(served))
	default:
		guidance = noGuidance()
	}
	return assembleRecall(req.ShopperID, served, poolView, lookup, overriding, guidance, len(st.pending), traceOf(req.TraceID))
}

// correctionBrand names the subject a correction was recorded about: the
// contradicted belief's subject for a declared correction, the journal
// entry's subject for a named standing, and empty for an unscoped one. A
// declared correction whose belief has not been rewritten yet reports its
// paired subject, so the guidance still names the brand.
func correctionBrand(st *shopperState, correction *contract.CorrectionView) string {
	if correction.ContradictsBeliefID != "" {
		if belief, ok := st.beliefs[correction.ContradictsBeliefID]; ok {
			return belief.subject.Name
		}
		if link, ok := st.pairing[correction.CorrectionID]; ok && strings.TrimSpace(link.Subject.Name) != "" {
			return link.Subject.Name
		}
	}
	return string(journalSubject(st, correction.Text).Name)
}

// correctionBelief identifies the belief a correction contradicts, or nil
// while the correction stands on its own — including the restart window
// where the cited belief has not been rewritten yet. The guidance then
// recommends from the correction alone instead of pointing at a stale id.
func correctionBelief(st *shopperState, correction *contract.CorrectionView) *string {
	if correction.ContradictsBeliefID == "" {
		return nil
	}
	if _, ok := st.beliefs[correction.ContradictsBeliefID]; !ok {
		return nil
	}
	id := correction.ContradictsBeliefID
	return &id
}

// strongestOwn returns the strongest served own-evidence belief: the
// first served, which the serving order already ranked.
func strongestOwn(served []contract.BeliefView) contract.BeliefView {
	for _, view := range served {
		if beliefStep(view) == contract.GuidanceOwnEvidence {
			return view
		}
	}
	return contract.BeliefView{}
}

// strongestStatement returns the first served general statement.
func strongestStatement(served []contract.BeliefView) contract.BeliefView {
	for _, view := range served {
		if beliefStep(view) == contract.GuidanceStatement {
			return view
		}
	}
	return contract.BeliefView{}
}

// RecordCorrection records the shopper's explicit correction at once,
// declared against the belief it contradicts. With no belief id the
// correction is a standing record: it is journalled under the shopper's
// most recent brand context, declared against a matching belief when one
// appears later, and honoured in recall until then. An unscoped standing
// — recorded before the layer knows any brand — applies to every brand
// and is never declared. The same words about the same subject return the
// stored correction: a repeated correction is one correction, not new
// evidence, and it does not refresh its event time.
func (s *Service) RecordCorrection(ctx context.Context, req contract.RecordCorrectionRequest) (contract.RecordCorrectionResponse, error) {
	if strings.TrimSpace(req.ShopperID) == "" {
		return contract.RecordCorrectionResponse{}, fmt.Errorf("server: record correction: %w", ErrMissingShopper)
	}
	if strings.TrimSpace(req.Text) == "" {
		return contract.RecordCorrectionResponse{}, fmt.Errorf("server: record correction: %w", ErrEmptyCorrection)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolRecordCorrection), req.ShopperID, traceOf(req.TraceID))
	st, err := s.state(ctx, req.ShopperID)
	if err != nil {
		return contract.RecordCorrectionResponse{}, fmt.Errorf("server: partition for %q: %w", req.ShopperID, err)
	}
	// A correction is shopper-driven and resets every opposite-direction
	// run for the shopper: the shopper's word breaks all streaks.
	clear(st.streaks)
	if strings.TrimSpace(req.ContradictsBeliefID) == "" {
		return s.recordStanding(ctx, st, req)
	}
	belief, ok := st.beliefs[req.ContradictsBeliefID]
	if !ok {
		return contract.RecordCorrectionResponse{}, fmt.Errorf("server: record correction: %w %q", ErrUnknownBelief, req.ContradictsBeliefID)
	}
	for _, id := range st.corrOrder {
		existing := st.corrections[id]
		if existing.view.ContradictsBeliefID == req.ContradictsBeliefID && existing.view.Text == req.Text {
			return s.correctionResponse(ctx, st, belief, existing, traceOf(req.TraceID))
		}
	}
	now := s.deps.Clock()
	if err := memory.RecordCorrection(ctx, s.graph, req.Text, belief.content, now); err != nil {
		return contract.RecordCorrectionResponse{}, fmt.Errorf("server: record correction: %w", err)
	}
	id := st.mint("corr")
	rec := &correctionRecord{
		view: contract.CorrectionView{
			CorrectionID:        id,
			Text:                req.Text,
			ContradictsBeliefID: belief.id,
			RecordedAt:          now.Format(time.RFC3339),
			TakesPrecedence:     true,
		},
		at: now,
	}
	st.corrections[id] = rec
	st.corrOrder = append(st.corrOrder, id)
	belief.touch(now)
	st.timeline(belief.id, contract.TimelineEvent{
		At:      now.Format(time.RFC3339),
		Kind:    contract.TimelineCorrection,
		Summary: fmt.Sprintf("Shopper correction: %s.", req.Text),
		RefID:   id,
	})
	if err := s.persistStanding(ctx, st, req.ShopperID); err != nil {
		return contract.RecordCorrectionResponse{}, err
	}
	return s.correctionResponse(ctx, st, belief, rec, traceOf(req.TraceID))
}

// recordStanding records a correction with no belief as a standing
// record. A correction naming the shopper's brand context is journalled
// and declared against a matching belief at once when one already
// exists; an unscoped one waits in the views alone. Either way the
// correction applies in recall from this call on.
func (s *Service) recordStanding(ctx context.Context, st *shopperState, req contract.RecordCorrectionRequest) (contract.RecordCorrectionResponse, error) {
	now := s.deps.Clock()
	size := lastSize(req.Text)
	subject := correctionSubject(st)
	for _, id := range st.corrOrder {
		existing := st.corrections[id]
		if existing == nil || existing.view.ContradictsBeliefID != "" || existing.view.Text != req.Text {
			continue
		}
		if journalSubject(st, req.Text) != subject {
			continue
		}
		return standingResponse(st, existing, traceOf(req.TraceID)), nil
	}
	id := st.mint("corr")
	rec := &correctionRecord{
		view: contract.CorrectionView{
			CorrectionID:    id,
			Text:            req.Text,
			RecordedAt:      now.Format(time.RFC3339),
			TakesPrecedence: true,
		},
		at: now,
	}
	st.corrections[id] = rec
	st.corrOrder = append(st.corrOrder, id)
	if namesSubject(subject) {
		entry := standingCorrection(req.Text, subject, size, now)
		if err := st.journal.Record(ctx, s.graph, entry); err != nil {
			return contract.RecordCorrectionResponse{}, fmt.Errorf("server: record standing correction: %w", err)
		}
		// A matching belief may already exist: the agent recorded the
		// correction without naming it. Declare the standing against one
		// match, so the warning renders at once. The match is the
		// strongest belief — most evidence, then earliest, then smallest
		// id — so two beliefs on one subject cite deterministically
		// instead of by map order, and a later rival keeps its own
		// standing: the entry declares only once.
		matches := []*beliefRecord{}
		for _, belief := range st.beliefs {
			if memory.SameSubject(entry, memoryBelief(belief)) {
				matches = append(matches, belief)
			}
		}
		sort.Slice(matches, func(i, j int) bool {
			if len(matches[i].evIDs) != len(matches[j].evIDs) {
				return len(matches[i].evIDs) > len(matches[j].evIDs)
			}
			if !matches[i].createdAt.Equal(matches[j].createdAt) {
				return matches[i].createdAt.Before(matches[j].createdAt)
			}
			return matches[i].id < matches[j].id
		})
		for _, belief := range matches {
			if _, err := st.journal.Declare(ctx, s.graph, memoryBelief(belief)); err != nil {
				return contract.RecordCorrectionResponse{}, fmt.Errorf("server: declare standing correction: %w", err)
			}
			if rec.view.ContradictsBeliefID != "" {
				continue
			}
			rec.view.ContradictsBeliefID = belief.id
			rec.at = now
			belief.touch(now)
			st.timeline(belief.id, contract.TimelineEvent{
				At:      now.Format(time.RFC3339),
				Kind:    contract.TimelineCorrection,
				Summary: fmt.Sprintf("Shopper correction: %s.", req.Text),
				RefID:   id,
			})
		}
	}
	if err := s.persistStanding(ctx, st, req.ShopperID); err != nil {
		return contract.RecordCorrectionResponse{}, err
	}
	if rec.view.ContradictsBeliefID != "" {
		belief := st.beliefs[rec.view.ContradictsBeliefID]
		return s.correctionResponse(ctx, st, belief, rec, traceOf(req.TraceID))
	}
	return standingResponse(st, rec, traceOf(req.TraceID)), nil
}

// namesSubject reports whether a subject names the layer's brand or
// seller scope for a standing correction's journal entry.
func namesSubject(subject domain.Subject) bool {
	if strings.TrimSpace(subject.ID) == "" {
		return false
	}
	return subject.Kind == domain.SubjectBrand || subject.Kind == domain.SubjectSeller
}

// standingResponse renders the confirmation for a correction that stands
// on its own: no belief is contradicted yet, so there is no belief to
// report, and the guidance recommends from the correction alone.
func standingResponse(st *shopperState, rec *correctionRecord, trace *string) contract.RecordCorrectionResponse {
	return contract.RecordCorrectionResponse{
		Correction:   rec.view,
		Contradicted: false,
		Status:       domain.StatusClaimed,
		Standing:     domain.GraphNone,
		Guidance:     guidanceForCorrection(rec.view, string(journalSubject(st, rec.view.Text).Name), nil),
		TraceID:      trace,
	}
}

// correctionResponse renders the confirmation that a correction applies at
// once while the contradicted belief keeps its status and standing.
func (s *Service) correctionResponse(ctx context.Context, st *shopperState, belief *beliefRecord, rec *correctionRecord, trace *string) (contract.RecordCorrectionResponse, error) {
	reading, err := s.readBelief(ctx, belief.content)
	if err != nil {
		return contract.RecordCorrectionResponse{}, fmt.Errorf("server: read belief: %w", err)
	}
	beliefID := belief.id
	return contract.RecordCorrectionResponse{
		Correction:   rec.view,
		BeliefID:     belief.id,
		Status:       reading.Status,
		Standing:     reading.Standing,
		Contradicted: true,
		Guidance:     guidanceForCorrection(rec.view, belief.subject.Name, &beliefID),
		TraceID:      trace,
	}, nil
}

// PendingQuestions lists the refunds still missing a reason, oldest
// first. Listing never resolves a question: only an answered reason or
// the question's absence does.
func (s *Service) PendingQuestions(ctx context.Context, req contract.PendingQuestionsRequest) (contract.PendingQuestionsResponse, error) {
	if strings.TrimSpace(req.ShopperID) == "" {
		return contract.PendingQuestionsResponse{}, fmt.Errorf("server: pending questions: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolPendingQuestions), req.ShopperID, nil)
	st, err := s.state(ctx, req.ShopperID)
	if err != nil {
		return contract.PendingQuestionsResponse{}, fmt.Errorf("server: partition for %q: %w", req.ShopperID, err)
	}
	questions := make([]contract.QuestionView, 0, len(st.pending))
	for _, rec := range st.pending {
		item := domain.Item{}
		if len(rec.items) > 0 {
			item = rec.items[0]
		}
		var lastAsked *string
		if rec.lastAskedAt != nil {
			asked := rec.lastAskedAt.Format(time.RFC3339)
			lastAsked = &asked
		}
		questions = append(questions, contract.QuestionView{
			QuestionID: rec.id,
			Outcome: contract.OutcomeView{
				Kind:        rec.outcome.Kind,
				Status:      rec.outcome.Status,
				AmountMinor: rec.outcome.AmountMinor,
				Currency:    rec.outcome.Currency,
			},
			Item: contract.ItemView{
				BrandID:   item.BrandID,
				BrandName: item.BrandName,
				LineName:  item.LineName,
				SKU:       item.SKU,
				Size:      item.Size,
				Category:  item.Category,
			},
			Link: contract.TransactionLinkView{
				OrderID:   rec.link.OrderID,
				CaptureID: rec.link.CaptureID,
				RefundID:  rec.link.RefundID,
				DisputeID: rec.link.DisputeID,
				CustomID:  rec.link.CustomID,
				EventID:   rec.link.EventID,
			},
			Seeded:        rec.seeded,
			AskedCount:    rec.askedCount,
			LastAskedAt:   lastAsked,
			AwaitingSince: rec.awaitingSince.Format(time.RFC3339),
		})
	}
	sort.Slice(questions, func(i, j int) bool {
		if questions[i].AwaitingSince != questions[j].AwaitingSince {
			return questions[i].AwaitingSince < questions[j].AwaitingSince
		}
		return questions[i].QuestionID < questions[j].QuestionID
	})
	if req.Limit > 0 && len(questions) > req.Limit {
		questions = questions[:req.Limit]
	}
	return contract.PendingQuestionsResponse{ShopperID: req.ShopperID, Questions: questions}, nil
}

// RecordReason records the shopper's answer to one pending question. The
// layer classifies it: a vague answer or a low-confidence classification
// stays pending and the agent asks again. A clear answer becomes evidence
// for the belief its reason counts toward; the question closes either
// way an answer lands.
func (s *Service) RecordReason(ctx context.Context, req contract.RecordReasonRequest) (contract.RecordReasonResponse, error) {
	if strings.TrimSpace(req.ShopperID) == "" {
		return contract.RecordReasonResponse{}, fmt.Errorf("server: record reason: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolRecordReason), req.ShopperID, traceOf(req.TraceID))
	st, err := s.state(ctx, req.ShopperID)
	if err != nil {
		return contract.RecordReasonResponse{}, fmt.Errorf("server: partition for %q: %w", req.ShopperID, err)
	}
	question, ok := st.pending[req.QuestionID]
	if !ok {
		return contract.RecordReasonResponse{}, fmt.Errorf("server: record reason: %w %q", ErrUnknownQuestion, req.QuestionID)
	}
	class, err := s.deps.Classifier.Classify(ctx, req.AnswerText)
	if err != nil {
		return contract.RecordReasonResponse{}, fmt.Errorf("server: record reason: classify: %w", err)
	}
	now := s.deps.Clock()
	question.askedCount++
	question.lastAskedAt = &now
	reason, askAgain := reasons.Settle(req.AnswerText, class)
	id := st.mint("rsn")
	if askAgain {
		return contract.RecordReasonResponse{
			ReasonID:     id,
			QuestionID:   question.id,
			Category:     class.Category,
			Confidence:   class.Confidence,
			CountsToward: countsTowardView(domain.BeliefKindNone),
			Accepted:     false,
			AskAgain:     true,
			BeliefID:     nil,
			TraceID:      traceOf(req.TraceID),
		}, nil
	}
	counts := reason.Category.CountsToward()
	rows := st.rowsForEvent(question.eventID)
	if len(rows) == 0 && len(question.items) > 0 {
		rows = []*evidenceRecord{{
			id: st.mint("ev"),
			row: domain.Evidence{
				Source:     domain.SourcePayment,
				Outcome:    question.outcome,
				Item:       question.items[0],
				Link:       question.link,
				Seeded:     question.seeded,
				OccurredAt: question.awaitingSince,
			},
			counts: counts,
		}}
		st.evidence[rows[0].id] = rows[0]
	}
	for _, rec := range rows {
		rec.row.Reason = reason
		rec.counts = counts
	}
	// Every classified answer settles its purchase's fit row, whether or
	// not a belief claims it: the pool counts settled purchases, not
	// beliefs. An unclear answer never reaches here; it stays pending.
	if err := s.classifyFitRows(ctx, st, rows, reason.Category, now); err != nil {
		return contract.RecordReasonResponse{}, err
	}
	if len(question.items) > 0 && strings.TrimSpace(question.items[0].BrandID) != "" {
		item := question.items[0]
		st.lastBrand = domain.Subject{Kind: domain.SubjectBrand, ID: item.BrandID, Name: item.BrandName}
	}
	if counts == domain.BeliefKindNone {
		markUnattached(rows)
		delete(st.pending, question.id)
		return contract.RecordReasonResponse{
			ReasonID:     id,
			QuestionID:   question.id,
			Category:     reason.Category,
			Confidence:   reason.Confidence,
			CountsToward: countsTowardView(counts),
			Accepted:     false,
			AskAgain:     false,
			BeliefID:     nil,
			TraceID:      traceOf(req.TraceID),
		}, nil
	}
	belief, err := s.beliefForReason(ctx, st, question, reason)
	if err != nil {
		return contract.RecordReasonResponse{}, err
	}
	if belief == nil {
		// Direction mismatch: the answer is classified and the question
		// closes, but no fit belief points its way, so the rows count
		// toward nothing. They stay stored with their reason — visible
		// as unattached, never silent — and no against edge is
		// declared: contradictions stay shopper-driven.
		markUnattached(rows)
		delete(st.pending, question.id)
		if err := s.persistIfJournal(ctx, st, req.ShopperID); err != nil {
			return contract.RecordReasonResponse{}, err
		}
		return contract.RecordReasonResponse{
			ReasonID:     id,
			QuestionID:   question.id,
			Category:     reason.Category,
			Confidence:   reason.Confidence,
			CountsToward: countsTowardView(domain.BeliefKindNone),
			Accepted:     false,
			AskAgain:     false,
			BeliefID:     nil,
			TraceID:      traceOf(req.TraceID),
		}, nil
	}
	for _, rec := range rows {
		if err := s.writeEvidenceRow(ctx, st, belief, rec, stubKey(rec.row)); err != nil {
			return contract.RecordReasonResponse{}, err
		}
		st.timeline(belief.id, contract.TimelineEvent{
			At:      rec.row.OccurredAt.Format(time.RFC3339),
			Kind:    contract.TimelineEvidence,
			Summary: evidenceSummary(rec.row, len(belief.evIDs)),
			RefID:   rec.id,
		})
	}
	delete(st.pending, question.id)
	beliefID := belief.id
	if err := s.persistIfJournal(ctx, st, req.ShopperID); err != nil {
		return contract.RecordReasonResponse{}, err
	}
	return contract.RecordReasonResponse{
		ReasonID:     id,
		QuestionID:   question.id,
		Category:     reason.Category,
		Confidence:   reason.Confidence,
		CountsToward: countsTowardView(counts),
		Accepted:     true,
		AskAgain:     false,
		BeliefID:     &beliefID,
		TraceID:      traceOf(req.TraceID),
	}, nil
}

// rowsForEvent returns the stored rows for one webhook event, oldest item
// first.
func (st *shopperState) rowsForEvent(eventID string) []*evidenceRecord {
	out := []*evidenceRecord{}
	for _, rec := range st.evidence {
		if rec.row.Link.EventID == eventID {
			out = append(out, rec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].row.Item.SKU < out[j].row.Item.SKU })
	return out
}

// markUnattached stores classified rows that count toward nothing as
// observations with no belief link: the reason stays visible on the row
// while no structural edge is written. The slice is never nil-linked, so
// the ledger renders an empty belief list instead of a missing one.
func markUnattached(rows []*evidenceRecord) {
	for _, rec := range rows {
		rec.counts = domain.BeliefKindNone
		rec.writeOp = domain.WriteObserve
		rec.beliefIDs = []string{}
	}
}

// beliefForReason returns the belief a classified answer counts toward:
// the most-supported fit belief for the brand pointing the reason's way,
// or the seller belief behind the transaction. It creates the belief on
// first sight. When fit beliefs exist for the brand but none points the
// reason's way, the answer joins the brand's opposite-direction run:
// below the birth threshold it counts toward nothing and the caller
// stores it unattached, while the run reaching the threshold births a
// same-kind opposite-direction rival belief the answer attaches to. A
// same-direction answer that attaches to an existing belief breaks the
// run. Birth is not promotion: the rival derives and earns exactly like
// any new belief, and the existing belief is untouched — no against edge
// is declared on the way out. A nil belief is never an error.
func (s *Service) beliefForReason(ctx context.Context, st *shopperState, question *pendingRecord, reason *domain.Reason) (*beliefRecord, error) {
	item := domain.Item{}
	if len(question.items) > 0 {
		item = question.items[0]
	}
	if reason.Category.CountsToward() == domain.BeliefKindSeller {
		return s.ensureSellerBelief(ctx, st, item, question.awaitingSince)
	}
	want := reason.Category.Direction()
	subject := domain.Subject{Kind: domain.SubjectBrand, ID: item.BrandID, Name: item.BrandName}
	var best *beliefRecord
	anyFit := false
	for _, rec := range st.beliefs {
		if rec.kind != domain.BeliefKindFit || rec.subject.ID != item.BrandID {
			continue
		}
		anyFit = true
		if rec.direction != want {
			continue
		}
		if best == nil || len(rec.evIDs) > len(best.evIDs) {
			best = rec
		}
	}
	if best != nil {
		// A same-direction return breaks the brand's opposite run.
		st.streakFor(domain.BeliefKindFit, item.BrandID).NoteMatched()
		return best, nil
	}
	if anyFit {
		// Fit beliefs exist but none points the answer's way. A
		// directionless fit answer is not opposite-direction, so it
		// lands unattached without moving the run.
		if want == domain.FitDirectionNone {
			return nil, nil
		}
		if st.streakFor(domain.BeliefKindFit, item.BrandID).NoteOpposite() {
			return s.newBelief(ctx, st, domain.BeliefKindFit, subject,
				fitBeliefValue(reason.Category, item.BrandName, item.Size), want, false, question.awaitingSince)
		}
		return nil, nil
	}
	return s.newBelief(ctx, st, domain.BeliefKindFit, subject,
		fitBeliefValue(reason.Category, item.BrandName, item.Size), want, false, question.awaitingSince)
}

// RecordFeedback records explicit fit feedback tied to a specific
// purchase, joined on the transaction's custom id. A fits-well verdict
// counts toward fit as supporting evidence for the worn size; an unclear
// verdict counts toward nothing.
func (s *Service) RecordFeedback(ctx context.Context, req contract.RecordFeedbackRequest) (contract.RecordFeedbackResponse, error) {
	if strings.TrimSpace(req.ShopperID) == "" {
		return contract.RecordFeedbackResponse{}, fmt.Errorf("server: record feedback: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolRecordFeedback), req.ShopperID, traceOf(req.TraceID))
	st, err := s.state(ctx, req.ShopperID)
	if err != nil {
		return contract.RecordFeedbackResponse{}, fmt.Errorf("server: partition for %q: %w", req.ShopperID, err)
	}
	ids := st.captures[req.CustomID]
	var matched *evidenceRecord
	for _, id := range ids {
		rec := st.evidence[id]
		if rec == nil {
			continue
		}
		if rec.row.Item.SKU == req.SKU && rec.row.Item.Size == req.Size {
			matched = rec
			break
		}
	}
	if matched == nil {
		if len(ids) == 0 {
			return contract.RecordFeedbackResponse{}, fmt.Errorf("server: record feedback: %w %q", ErrUnknownPurchase, req.CustomID)
		}
		return contract.RecordFeedbackResponse{}, fmt.Errorf("server: record feedback: %w", ErrFeedbackMismatch)
	}
	verdict := verdictForFeedback(req.FeedbackText)
	counts, err := memory.VerdictCountsToward(verdict)
	if err != nil {
		return contract.RecordFeedbackResponse{}, fmt.Errorf("server: record feedback: %w", err)
	}
	id := st.mint("fb")
	if counts == domain.BeliefKindNone {
		return contract.RecordFeedbackResponse{
			FeedbackID:   id,
			Verdict:      verdict,
			CountsToward: countsTowardView(counts),
			BeliefID:     nil,
			Accepted:     false,
			TraceID:      traceOf(req.TraceID),
		}, nil
	}
	now := s.deps.Clock()
	matched.row.Reason = &domain.Reason{Category: domain.CategoryNone, Text: req.FeedbackText, Confidence: 1.0}
	matched.counts = counts
	belief, err := s.beliefForFeedback(ctx, st, matched.row.Item, now)
	if err != nil {
		return contract.RecordFeedbackResponse{}, err
	}
	// Fit feedback settles its purchase's fit row, from the same words
	// that wrote the belief. Seller verdicts never reach here: a damaged
	// or missing item is about the seller, not the fit.
	if err := s.feedbackFitRow(ctx, st, matched, verdict, now); err != nil {
		return contract.RecordFeedbackResponse{}, err
	}
	if strings.TrimSpace(matched.row.Item.BrandID) != "" {
		item := matched.row.Item
		st.lastBrand = domain.Subject{Kind: domain.SubjectBrand, ID: item.BrandID, Name: item.BrandName}
	}
	docID, err := s.indexEvidence(ctx, matched.row, "")
	if err != nil {
		return contract.RecordFeedbackResponse{}, err
	}
	stub, err := evidence.GraphStub(docID, stubKey(matched.row))
	if err != nil {
		return contract.RecordFeedbackResponse{}, err
	}
	written, err := memory.WriteFeedback(ctx, s.graph, belief.content, belief.kind, stub, verdict, now)
	if err != nil {
		return contract.RecordFeedbackResponse{}, fmt.Errorf("server: write feedback: %w", err)
	}
	if !written {
		return contract.RecordFeedbackResponse{}, fmt.Errorf("server: feedback wrote nothing")
	}
	matched.docID = docID
	matched.writeOp = domain.WriteRecordAction
	matched.beliefIDs = append(matched.beliefIDs, belief.id)
	belief.evIDs = append(belief.evIDs, matched.id)
	belief.touch(now)
	st.timeline(belief.id, contract.TimelineEvent{
		At:      now.Format(time.RFC3339),
		Kind:    contract.TimelineFeedback,
		Summary: evidenceSummary(matched.row, len(belief.evIDs)),
		RefID:   id,
	})
	beliefID := belief.id
	if err := s.persistIfJournal(ctx, st, req.ShopperID); err != nil {
		return contract.RecordFeedbackResponse{}, err
	}
	return contract.RecordFeedbackResponse{
		FeedbackID:   id,
		Verdict:      verdict,
		CountsToward: countsTowardView(counts),
		BeliefID:     &beliefID,
		Accepted:     true,
		TraceID:      traceOf(req.TraceID),
	}, nil
}

// beliefForFeedback returns the fit belief feedback supports: the
// strongest belief the shopper has not overruled, or a new emerging
// belief when every fit belief stands contradicted.
func (s *Service) beliefForFeedback(ctx context.Context, st *shopperState, item domain.Item, at time.Time) (*beliefRecord, error) {
	var best *beliefRecord
	for _, rec := range st.beliefs {
		if rec.kind != domain.BeliefKindFit || rec.subject.ID != item.BrandID {
			continue
		}
		contradicted := false
		for _, id := range st.corrOrder {
			if corr := st.corrections[id]; corr != nil && corr.view.ContradictsBeliefID == rec.id {
				contradicted = true
				break
			}
		}
		if contradicted {
			continue
		}
		if best == nil || len(rec.evIDs) > len(best.evIDs) {
			best = rec
		}
	}
	if best != nil {
		return best, nil
	}
	return s.newBelief(ctx, st, domain.BeliefKindFit,
		domain.Subject{Kind: domain.SubjectBrand, ID: item.BrandID, Name: item.BrandName},
		feedbackBeliefValue(item.Size), domain.FitDirectionNone, false, at)
}

// ExplainBelief answers why the layer believes something: every piece of
// evidence with its reason, every correction, and the history in order.
func (s *Service) ExplainBelief(ctx context.Context, req contract.ExplainBeliefRequest) (contract.ExplainBeliefResponse, error) {
	if strings.TrimSpace(req.ShopperID) == "" {
		return contract.ExplainBeliefResponse{}, fmt.Errorf("server: explain belief: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolExplainBelief), req.ShopperID, nil)
	st, err := s.state(ctx, req.ShopperID)
	if err != nil {
		return contract.ExplainBeliefResponse{}, fmt.Errorf("server: partition for %q: %w", req.ShopperID, err)
	}
	rec, ok := st.beliefs[req.BeliefID]
	if !ok {
		return contract.ExplainBeliefResponse{}, fmt.Errorf("server: explain belief: %w %q", ErrUnknownBelief, req.BeliefID)
	}
	view, err := s.beliefView(ctx, st, rec)
	if err != nil {
		return contract.ExplainBeliefResponse{}, fmt.Errorf("server: explain belief: %w", err)
	}
	corrections := []contract.CorrectionView{}
	for i := len(st.corrOrder) - 1; i >= 0; i-- {
		corr := st.corrections[st.corrOrder[i]]
		if corr != nil && corr.view.ContradictsBeliefID == rec.id {
			corrections = append(corrections, corr.view)
		}
	}
	if corrections == nil {
		corrections = []contract.CorrectionView{}
	}
	timeline := append([]contract.TimelineEvent(nil), st.timelines[rec.id]...)
	sort.SliceStable(timeline, func(i, j int) bool { return timeline[i].At < timeline[j].At })
	if timeline == nil {
		timeline = []contract.TimelineEvent{}
	}
	return contract.ExplainBeliefResponse{Belief: view, Corrections: corrections, Timeline: timeline}, nil
}

// beliefRank orders ledger rows: contradicted first, then Earned,
// Building, then Claimed; ties break on evidence, recency and id.
func beliefRank(status domain.BeliefStatus, contradicted bool) int {
	if contradicted {
		return 0
	}
	switch status {
	case domain.StatusEarned:
		return 1
	case domain.StatusBuilding:
		return 2
	default:
		return 3
	}
}

// LedgerBeliefs lists the ledger's belief rows, strongest first, each
// with its ledger status, underlying standing, contradiction warning,
// seeded mark and evidence count.
func (s *Service) LedgerBeliefs(ctx context.Context, req contract.LedgerBeliefsRequest) (contract.LedgerBeliefsResponse, error) {
	if strings.TrimSpace(req.ShopperID) == "" {
		return contract.LedgerBeliefsResponse{}, fmt.Errorf("server: ledger beliefs: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolLedgerBeliefs), req.ShopperID, nil)
	st, err := s.state(ctx, req.ShopperID)
	if err != nil {
		return contract.LedgerBeliefsResponse{}, fmt.Errorf("server: partition for %q: %w", req.ShopperID, err)
	}
	rows := []contract.LedgerBeliefRow{}
	for _, rec := range st.beliefs {
		if !req.IncludeSeeded && rec.seeded {
			continue
		}
		view, err := s.beliefView(ctx, st, rec)
		if err != nil {
			return contract.LedgerBeliefsResponse{}, fmt.Errorf("server: ledger beliefs: %w", err)
		}
		if req.StatusFilter != "" && view.Status != req.StatusFilter {
			continue
		}
		rows = append(rows, contract.LedgerBeliefRow{
			Belief:        view,
			EvidenceCount: len(rec.evIDs),
			LastChangeAt:  rec.updatedAt.Format(time.RFC3339),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		ri, rj := beliefRank(rows[i].Belief.Status, rows[i].Belief.Contradicted), beliefRank(rows[j].Belief.Status, rows[j].Belief.Contradicted)
		if ri != rj {
			return ri < rj
		}
		if rows[i].EvidenceCount != rows[j].EvidenceCount {
			return rows[i].EvidenceCount > rows[j].EvidenceCount
		}
		if rows[i].LastChangeAt != rows[j].LastChangeAt {
			return rows[i].LastChangeAt > rows[j].LastChangeAt
		}
		return rows[i].Belief.BeliefID < rows[j].Belief.BeliefID
	})
	if rows == nil {
		rows = []contract.LedgerBeliefRow{}
	}
	return contract.LedgerBeliefsResponse{ShopperID: req.ShopperID, Beliefs: rows}, nil
}

// LedgerBelief reads one ledger row in full, with its corrections and its
// seeded notice. The notice carries the seeded mark in words for
// generated rows and is null for lived ones.
func (s *Service) LedgerBelief(ctx context.Context, req contract.LedgerBeliefRequest) (contract.LedgerBeliefResponse, error) {
	if strings.TrimSpace(req.ShopperID) == "" {
		return contract.LedgerBeliefResponse{}, fmt.Errorf("server: ledger belief: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolLedgerBelief), req.ShopperID, nil)
	st, err := s.state(ctx, req.ShopperID)
	if err != nil {
		return contract.LedgerBeliefResponse{}, fmt.Errorf("server: partition for %q: %w", req.ShopperID, err)
	}
	rec, ok := st.beliefs[req.BeliefID]
	if !ok {
		return contract.LedgerBeliefResponse{}, fmt.Errorf("server: ledger belief: %w %q", ErrUnknownBelief, req.BeliefID)
	}
	view, err := s.beliefView(ctx, st, rec)
	if err != nil {
		return contract.LedgerBeliefResponse{}, fmt.Errorf("server: ledger belief: %w", err)
	}
	corrections := []contract.CorrectionView{}
	for i := len(st.corrOrder) - 1; i >= 0; i-- {
		corr := st.corrections[st.corrOrder[i]]
		if corr != nil && corr.view.ContradictsBeliefID == rec.id {
			corrections = append(corrections, corr.view)
		}
	}
	if corrections == nil {
		corrections = []contract.CorrectionView{}
	}
	var notice *string
	if text, ok := st.notices[rec.id]; ok {
		notice = &text
	}
	return contract.LedgerBeliefResponse{Belief: view, Corrections: corrections, SeededNotice: notice}, nil
}

// LedgerEvidence lists the ledger's evidence rows, newest first: outcomes
// with their reasons, what each counts toward, seeded marks and belief
// links. Rows that count toward a belief carry its link. Classified rows
// that count toward nothing — a changed mind, or a fit answer pointing
// nowhere the shopper's beliefs point — still appear, unattached but with
// their reason, so nothing the shopper said goes silent. Only an
// unexplained outcome stays out: with no reason it counts toward nothing
// and stays in pending questions.
func (s *Service) LedgerEvidence(ctx context.Context, req contract.LedgerEvidenceRequest) (contract.LedgerEvidenceResponse, error) {
	if strings.TrimSpace(req.ShopperID) == "" {
		return contract.LedgerEvidenceResponse{}, fmt.Errorf("server: ledger evidence: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolLedgerEvidence), req.ShopperID, nil)
	st, err := s.state(ctx, req.ShopperID)
	if err != nil {
		return contract.LedgerEvidenceResponse{}, fmt.Errorf("server: partition for %q: %w", req.ShopperID, err)
	}
	rows := []contract.LedgerEvidenceRow{}
	for _, rec := range st.evidence {
		if len(rec.beliefIDs) == 0 && rec.row.Reason == nil {
			continue
		}
		if req.BeliefID != "" {
			linked := false
			for _, id := range rec.beliefIDs {
				if id == req.BeliefID {
					linked = true
					break
				}
			}
			if !linked {
				continue
			}
		}
		rows = append(rows, contract.LedgerEvidenceRow{
			Evidence:  evidenceView(rec),
			BeliefIDs: append([]string(nil), rec.beliefIDs...),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Evidence.OccurredAt != rows[j].Evidence.OccurredAt {
			return rows[i].Evidence.OccurredAt > rows[j].Evidence.OccurredAt
		}
		return rows[i].Evidence.EvidenceID < rows[j].Evidence.EvidenceID
	})
	if rows == nil {
		rows = []contract.LedgerEvidenceRow{}
	}
	return contract.LedgerEvidenceResponse{ShopperID: req.ShopperID, Evidence: rows}, nil
}
