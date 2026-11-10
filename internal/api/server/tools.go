package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/memory"
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
func (s *Service) IngestWebhook(ctx context.Context, shopper, eventID string) (WebhookResult, error) {
	if strings.TrimSpace(shopper) == "" {
		return WebhookResult{}, fmt.Errorf("server: ingest: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log("webhook_ingest", shopper, nil)
	st := s.state(shopper)
	res, err := st.ingest.Handle(ctx, eventID)
	if err != nil {
		return WebhookResult{}, fmt.Errorf("server: ingest webhook %q: %w", eventID, err)
	}
	out := WebhookResult{EventID: eventID, Rows: len(res.Rows), Duplicate: res.Duplicate}
	if res.Duplicate {
		out.Pending = res.Pending != nil
		return out, nil
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
func (s *Service) newBelief(ctx context.Context, st *shopperState, kind domain.BeliefKind, subject domain.Subject, value string, direction domain.FitDirection, seeded bool, at time.Time) (*beliefRecord, error) {
	content := beliefContent(kind, subject, value)
	if err := memory.EnsureBelief(ctx, s.graph, content, at); err != nil {
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
	return rec, nil
}

// Recall returns the beliefs that apply to a purchase, each with its
// status and evidence, plus any correction that overrides them. The
// correction ships both at the top level and inline, with the guidance
// pointing at it, so an agent cannot recommend from the belief alone.
func (s *Service) Recall(ctx context.Context, req contract.RecallRequest) (contract.RecallResponse, error) {
	if strings.TrimSpace(req.ShopperID) == "" {
		return contract.RecallResponse{}, fmt.Errorf("server: recall: %w", ErrMissingShopper)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log(string(contract.ToolRecallForPurchase), req.ShopperID, traceOf(req.TraceID))
	st := s.state(req.ShopperID)
	views := []contract.BeliefView{}
	for _, rec := range st.beliefs {
		view, err := s.beliefView(ctx, st, rec)
		if err != nil {
			return contract.RecallResponse{}, fmt.Errorf("server: recall: %w", err)
		}
		views = append(views, view)
	}
	served := recallBeliefs(views, req.BrandID)
	var overriding *contract.CorrectionView
	for _, view := range served {
		if view.ContradictedBy != nil {
			latest := *view.ContradictedBy
			if overriding == nil || latest.RecordedAt > overriding.RecordedAt {
				overriding = &latest
			}
		}
	}
	var guidance contract.GuidanceView
	switch {
	case overriding != nil:
		belief := servedBelief(served, overriding.ContradictsBeliefID)
		guidance = guidanceForCorrection(*overriding, belief)
	case len(served) > 0:
		belief := servedBelief(served, served[0].BeliefID)
		guidance = guidanceForBelief(belief)
	default:
		guidance = noGuidance()
	}
	return assembleRecall(req.ShopperID, served, overriding, guidance, len(st.pending), traceOf(req.TraceID))
}

// servedBelief finds the stored record behind one served view. The caller
// serves only beliefs from its own state, so the record always exists.
func servedBelief(served []contract.BeliefView, id string) *beliefRecord {
	for _, view := range served {
		if view.BeliefID == id {
			return &beliefRecord{id: view.BeliefID, subject: view.Subject, value: view.Value}
		}
	}
	return &beliefRecord{id: id}
}

// RecordCorrection records the shopper's explicit correction at once,
// declared against the belief it contradicts. The same words against the
// same belief return the stored correction: a repeated correction is one
// correction, not new evidence.
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
	st := s.state(req.ShopperID)
	// A correction is shopper-driven and resets every opposite-direction
	// run for the shopper: the shopper's word breaks all streaks.
	clear(st.streaks)
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
	return s.correctionResponse(ctx, st, belief, rec, traceOf(req.TraceID))
}

// correctionResponse renders the confirmation that a correction applies at
// once while the contradicted belief keeps its status and standing.
func (s *Service) correctionResponse(ctx context.Context, st *shopperState, belief *beliefRecord, rec *correctionRecord, trace *string) (contract.RecordCorrectionResponse, error) {
	reading, err := s.readBelief(ctx, belief.content)
	if err != nil {
		return contract.RecordCorrectionResponse{}, fmt.Errorf("server: read belief: %w", err)
	}
	return contract.RecordCorrectionResponse{
		Correction:   rec.view,
		BeliefID:     belief.id,
		Status:       reading.Status,
		Standing:     reading.Standing,
		Contradicted: true,
		Guidance:     guidanceForCorrection(rec.view, belief),
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
	st := s.state(req.ShopperID)
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
	st := s.state(req.ShopperID)
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
	st := s.state(req.ShopperID)
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
	st := s.state(req.ShopperID)
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
	st := s.state(req.ShopperID)
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
	st := s.state(req.ShopperID)
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
	st := s.state(req.ShopperID)
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
