package server

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/pool"
)

// ErrOverrideMissing is returned when a recall would serve a contradicted
// belief without its override: a contradicted belief with no correction of
// its own inline, or contradicted beliefs served under a top-level override
// that carries none of them. The service fails the call instead of letting an
// agent recommend from a belief the shopper has overruled.
var ErrOverrideMissing = errors.New("server: contradicted belief has no overriding correction")

// ErrPoolUnavailable is returned by product_fit when the pool index cannot
// be reached. Recall never returns this: it carries on without the pool.
// product_fit has no recall to carry on with, so it refuses instead of
// reporting an empty pool.
var ErrPoolUnavailable = errors.New("server: pool index unavailable")

// assembleRecall builds the recall response from its parts. A contradicted
// belief is unrepresentable without its override: it must carry the
// correction that cites it inline, and the top-level override, when
// contradicted beliefs are served, must be one of those corrections.
// Assembly fails rather than serving a belief the shopper has overruled.
// One brand can hold two fit beliefs, one each way, and the shopper can
// correct both: the response then carries one top-level override — the
// latest, which overridingCorrection picked — and each belief keeps its
// own beside it, so every contradiction stays overridable. A present pool
// rides with a consulted lookup; an absent pool rides with whatever the
// lookup says —
// unavailable when the pool is unwired, no single line can be named for the
// request, or the index could not be reached, consulted when the wired pool
// named no direction (Claimed, or Building with the directions level or only
// fits-well outcomes) — so a recall on a pool with nothing to say serves the
// consulted lookup instead of failing. The pool and the
// lookup are always present: an absent pool is null, never a missing key.
// Every handler builds recall through here, so no code path can return the
// belief without the correction beside it, or a pool without saying whether
// it was consulted.
func assembleRecall(shopper string, beliefs []contract.BeliefView, poolView *contract.PoolView, lookup contract.PoolLookup, overriding *contract.CorrectionView, guidance contract.GuidanceView, pending int, trace *string) (contract.RecallResponse, error) {
	inline := map[string]bool{}
	for _, belief := range beliefs {
		if belief.Precedence == nil {
			return contract.RecallResponse{}, fmt.Errorf("server: recall: belief %q has no precedence step", belief.BeliefID)
		}
		if !belief.Contradicted {
			continue
		}
		if belief.ContradictedBy == nil || belief.ContradictedBy.ContradictsBeliefID != belief.BeliefID {
			return contract.RecallResponse{}, fmt.Errorf("%w: belief %q", ErrOverrideMissing, belief.BeliefID)
		}
		inline[belief.ContradictedBy.CorrectionID] = true
	}
	if len(inline) > 0 && (overriding == nil || !inline[overriding.CorrectionID]) {
		return contract.RecallResponse{}, fmt.Errorf("%w: no override for the contradicted beliefs served", ErrOverrideMissing)
	}
	if overriding != nil && guidance.Basis != contract.GuidanceCorrection {
		return contract.RecallResponse{}, fmt.Errorf("%w: override present without correction guidance", ErrOverrideMissing)
	}
	if poolView == nil && lookup != "" && lookup != contract.PoolConsulted && lookup != contract.PoolUnavailable {
		return contract.RecallResponse{}, fmt.Errorf("server: recall: pool absent with lookup %q", string(lookup))
	}
	if poolView != nil && lookup != contract.PoolConsulted {
		return contract.RecallResponse{}, fmt.Errorf("server: recall: pool present with lookup %q", string(lookup))
	}
	if beliefs == nil {
		beliefs = []contract.BeliefView{}
	}
	out := contract.RecallResponse{
		ShopperID:            shopper,
		Beliefs:              beliefs,
		OverridingCorrection: overriding,
		Guidance:             guidance,
		PendingCount:         pending,
		TraceID:              trace,
	}
	if poolView != nil || lookup != "" {
		out.Pool = poolView
		out.PoolLookup = lookup
	}
	return out, nil
}

// recallBeliefs selects the beliefs a recall serves for one brand: every
// fit belief the shopper holds for it, at any standing, strongest first.
// Status is a label, never a gate: a lone return is Claimed with one
// evidence row, and it comes back labelled Claimed with its evidence and
// its precedence step, beside the pool, so the agent weighs thin evidence
// instead of being told nothing. A fit belief with no evidence is the
// shopper's general statement and decides only when nothing stronger
// does. Seller beliefs surface through the ledger and explain reads, never
// through a sizing recall. A contradicted belief is served with its
// warning inline, never alone: the assembler beside this function
// enforces the override.
func recallBeliefs(views []contract.BeliefView, brandID string) []contract.BeliefView {
	out := []contract.BeliefView{}
	for _, view := range views {
		if view.Kind != domain.BeliefKindFit {
			continue
		}
		if brandID != "" && view.Subject.ID != brandID {
			continue
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Evidence) != len(out[j].Evidence) {
			return len(out[i].Evidence) > len(out[j].Evidence)
		}
		return out[i].BeliefID < out[j].BeliefID
	})
	return out
}

// beliefStep names the precedence step one served belief came from: own
// evidence when the shopper's history backs it, the general statement
// otherwise.
func beliefStep(view contract.BeliefView) contract.GuidanceBasis {
	if len(view.Evidence) > 0 {
		return contract.GuidanceOwnEvidence
	}
	return contract.GuidanceStatement
}

// overridingCorrection returns the correction recall must honour first:
// the latest correction declared against a served belief; else the
// latest named standing matching the brand, whose belief has not been
// written yet; else a declared correction whose belief has not been
// rewritten yet (the restart window), honoured by its paired subject the
// same way; else the latest unscoped standing, which applies to every
// brand until one is known. Specificity wins over recency: a correction
// naming this brand beats a newer correction naming none. A declared
// correction naming a belief this recall does not serve applies to
// another recall, never this one. When two served beliefs are each
// corrected, this returns the newer of the two as the top-level override
// and the older one rides inline on its own belief. Two corrections
// recorded at the same instant go to the one recorded later: arrival
// order breaks the tie, the same way latestCorrection does, so the pick
// never depends on map order or on a clock finer than the store keeps.
func overridingCorrection(st *shopperState, served []contract.BeliefView, brandID string) *contract.CorrectionView {
	servedIDs := map[string]bool{}
	for _, view := range served {
		servedIDs[view.BeliefID] = true
	}
	var best *correctionRecord
	bestClass := 3
	for _, id := range st.corrOrder {
		rec := st.corrections[id]
		if rec == nil {
			continue
		}
		class := 3
		switch cited := rec.view.ContradictsBeliefID; {
		case cited != "" && servedIDs[cited]:
			class = 0
		case cited != "":
			if _, ok := st.beliefs[cited]; ok {
				continue
			}
			subject := viewSubject(st, rec)
			switch {
			case brandID != "" && subject.Kind == domain.SubjectBrand && subject.ID == brandID:
				class = 1
			case subject == (domain.Subject{}):
				class = 2
			default:
				continue
			}
		case brandID != "" && journalSubject(st, rec.view.Text).ID == brandID:
			class = 1
		case isUnscoped(st, rec.view.Text):
			class = 2
		default:
			continue
		}
		if best == nil || class < bestClass || (class == bestClass && !rec.at.Before(best.at)) {
			best, bestClass = rec, class
		}
	}
	if best == nil {
		return nil
	}
	out := best.view
	return &out
}

// viewSubject returns the subject one correction view was declared
// against: the paired journal entry's subject when the pairing exists,
// else the journal entry's subject for the correction's words, or the
// zero subject when no entry covers it (unscoped). The pairing is exact
// where the words alone are ambiguous: the same words about two subjects
// each keep their own.
func viewSubject(st *shopperState, rec *correctionRecord) domain.Subject {
	if link, ok := st.pairing[rec.view.CorrectionID]; ok {
		return link.Subject
	}
	return journalSubject(st, rec.view.Text)
}

// journalSubject returns the journal entry's subject for one correction
// text, or the zero subject when the correction is unscoped.
func journalSubject(st *shopperState, text string) domain.Subject {
	for _, entry := range st.journal.Standing() {
		if entry.Text == text {
			return entry.Subject
		}
	}
	return domain.Subject{}
}

// isUnscoped reports whether one correction text has no journal entry:
// an unscoped standing, recorded before the layer knew any brand.
func isUnscoped(st *shopperState, text string) bool {
	for _, entry := range st.journal.Standing() {
		if entry.Text == text {
			return false
		}
	}
	return true
}

// usualSize computes the size recall adjusts the pool from: the size the
// shopper most often kept or said fits well, in any brand, else the size
// their general statement names. Kept rows and fits-well feedback feed
// the tallies; a classified refund never does.
func usualSize(st *shopperState, served []contract.BeliefView) domain.UsualSize {
	tallies := map[string]*domain.SizeTally{}
	for _, rec := range st.evidence {
		size := rec.row.Item.Size
		if size == "" {
			continue
		}
		tally, ok := tallies[size]
		if !ok {
			tally = &domain.SizeTally{Size: size}
			tallies[size] = tally
		}
		switch {
		case rec.row.Outcome.Kind == domain.OutcomeKept:
			tally.Kept++
		case rec.row.Reason != nil && verdictForFeedback(rec.row.Reason.Text) == contract.FeedbackFitsWell:
			tally.FitsWell++
		}
	}
	list := make([]domain.SizeTally, 0, len(tallies))
	for _, tally := range tallies {
		if tally.Kept+tally.FitsWell > 0 {
			list = append(list, *tally)
		}
	}
	stated := ""
	for _, view := range served {
		if len(view.Evidence) == 0 {
			if size := lastSize(view.Value); size != "" {
				stated = size
				break
			}
		}
	}
	return domain.UsualSizeFor(list, stated)
}

// poolQuery resolves the pool read for one recall: the line and the size
// to read, and whether a line could be named at all. The size is the usual
// size when one is known, else the size under consideration; without one
// there is nothing to read.
func poolQuery(req contract.RecallRequest, usual domain.UsualSize, lines []domain.ProductLine) (domain.ProductLine, string, bool) {
	line, ok := requestLine(req, lines)
	if !ok {
		return domain.ProductLine{}, "", false
	}
	size := usual.Size
	if size == "" {
		size = req.SizeConsidered
	}
	if size == "" {
		return domain.ProductLine{}, "", false
	}
	return line, size, true
}

// requestLine names the product line one recall asks about. A request that
// carries a fit handle names its line outright and it is used as given: the
// caller is the authority on the line it asks about. A request that names
// the brand and category but no fit handle is resolved against the layer's
// own population, so a caller that can honestly name only what it knows —
// such as the why view, whose belief carries the brand and whose evidence
// rows carry the category — still reads the right line instead of
// inventing a cut. The population must name exactly one line: none and
// more than one both leave the pool unconsulted rather than pick a line or
// make one up.
func requestLine(req contract.RecallRequest, lines []domain.ProductLine) (domain.ProductLine, bool) {
	if req.BrandID == "" || req.Category == "" {
		return domain.ProductLine{}, false
	}
	if req.FitHandle != "" {
		return domain.ProductLine{Brand: req.BrandID, Category: req.Category, FitHandle: req.FitHandle}, true
	}
	return populationLine(lines, req.BrandID, req.Category)
}

// populationLine returns the one line the population seeds for a brand and
// category. It reports false when none matches or more than one does: two
// handles for one brand and category are two lines, and recall never
// chooses between them.
func populationLine(lines []domain.ProductLine, brand, category string) (domain.ProductLine, bool) {
	found := domain.ProductLine{}
	matches := 0
	for _, line := range lines {
		if line.Brand != brand || line.Category != category {
			continue
		}
		found = line
		matches++
	}
	if matches != 1 {
		return domain.ProductLine{}, false
	}
	return found, true
}

// poolStep is one recall's pool contribution: the view recall serves and
// the direction the pool names. Only a pool that names a direction has a
// step, because a step adjusts the recommendation and only an off-size
// direction can adjust it: a Claimed pool, a Building pool with the two
// directions level, and a Building pool whose only fit outcomes are
// fits-well all name none, so they take no step and guide no size.
type poolStep struct {
	// view is the pooled product fit as recall serves it.
	view contract.PoolView
	// direction is the direction the pool rates, small or large.
	direction domain.FitDirection
}

// lookupPool reads the pooled product fit for one recall. Without a wired
// pool index, or without a line and size to read, there is nothing to
// consult: the lookup reads unavailable and recall carries on without the
// pool, never pretending an empty one. The line comes from the request's
// own fit handle when it names one, else from the one line the layer's
// population seeds for the request's brand and category. Any index error
// answers unavailable the same way: recall never refuses for the pool's
// sake. A pool that names no direction says nothing about how the size
// runs off, so it is consulted but yields no step: recall's pool
// precedence rung applies only when the pool says the product runs
// off-size.
func (s *Service) lookupPool(ctx context.Context, req contract.RecallRequest, usual domain.UsualSize) (*poolStep, contract.PoolLookup) {
	if s.pool == nil {
		return nil, contract.PoolUnavailable
	}
	line, size, ok := poolQuery(req, usual, s.population)
	if !ok {
		return nil, contract.PoolUnavailable
	}
	fit, err := s.pool.Fit(ctx, line, size)
	if err != nil {
		return nil, contract.PoolUnavailable
	}
	if fit.Verdict.Status == domain.StatusClaimed || fit.Verdict.Direction == domain.FitDirectionNone {
		return nil, contract.PoolConsulted
	}
	return &poolStep{view: poolViewOf(fit), direction: fit.Verdict.Direction}, contract.PoolConsulted
}

// poolViewOf maps one pool read to its wire form. The precedence step is
// left out: recall sets it, and product_fit never carries it.
func poolViewOf(fit pool.Fit) contract.PoolView {
	var direction *domain.FitDirection
	if fit.Verdict.Direction != domain.FitDirectionNone {
		dir := fit.Verdict.Direction
		direction = &dir
	}
	return contract.PoolView{
		Line:      contract.ProductLineView{BrandID: fit.Line.Brand, Category: fit.Line.Category, FitHandle: fit.Line.FitHandle},
		Size:      fit.Size,
		Status:    fit.Verdict.Status,
		Direction: direction,
		Counts:    poolCountsView(fit.Counts),
		Seeded:    poolCountsView(fit.Seeded),
		Live:      poolCountsView(fit.Live),
		Recent:    poolCountsView(fit.Recent),
	}
}

// poolCountsView maps one bucket of settled counts to its wire form. The
// reasons arrive in the index's labels, which predate the domain's words
// for the two non-fit causes, so both spellings count to the same field.
func poolCountsView(counts pool.Counts) contract.PoolCountsView {
	return contract.PoolCountsView{
		Settled: counts.Settled,
		Kept:    counts.Kept,
		Returns: contract.DirectionCountsView{
			TooSmall: counts.Returns.TooSmall,
			TooLarge: counts.Returns.TooLarge,
		},
		Feedback: contract.VerdictCountsView{
			FitsWell: counts.Feedback.FitsWell,
			TooSmall: counts.Feedback.TooSmall,
			TooLarge: counts.Feedback.TooLarge,
		},
		Reasons: reasonCountsView(counts.Reasons),
		Shoppers: contract.ShopperCountsView{
			Fit:      counts.Shoppers.Fit,
			TooSmall: counts.Shoppers.TooSmall,
			TooLarge: counts.Shoppers.TooLarge,
		},
	}
}

// reasonCountsView folds the settled returns by reason category. The index
// can carry the recorded population's labels beside the domain's words:
// "damaged" spells damage, and "not_as_described" spells a cause the
// domain files under damage too, because the goods arrived and were
// wrong. Both count with damage and neither counts as non-arrival.
// Unknown categories are dropped: the merchant's "why it comes back"
// names only the frozen set.
//
// The two labels are a read-side spelling only. No served write path can
// emit one: the classifier refuses a choice outside domain.Categories,
// and the fit writer validates every category it stores against the same
// set, so an index the layer fills carries domain words only.
func reasonCountsView(reasons map[domain.Category]int) contract.ReasonCountsView {
	out := contract.ReasonCountsView{}
	for category, n := range reasons {
		switch category {
		case domain.CategoryFitTooSmall:
			out.FitTooSmall += n
		case domain.CategoryFitTooLarge:
			out.FitTooLarge += n
		case domain.CategoryDamagedOrDefective, "damaged", "not_as_described":
			out.DamagedOrDefective += n
		case domain.CategoryNotArrived:
			out.NotArrived += n
		case domain.CategoryChangedMind:
			out.ChangedMind += n
		}
	}
	return out
}

// brandName names the brand for guidance: the subject name from the
// shopper's own beliefs when one names it, else the brand id.
func brandName(st *shopperState, brandID string) string {
	for _, rec := range st.beliefs {
		if rec.subject.ID == brandID && rec.subject.Name != "" {
			return rec.subject.Name
		}
	}
	return brandID
}
