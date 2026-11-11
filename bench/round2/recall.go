package round2

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/seed"
)

// ExpectedRecall builds the recall_for_purchase response the freeze
// expects the layer to serve for one shopper, in the contract's shape. It
// is the memory the Memento arms are meant to see, built from the frozen
// history and population and never from the hidden answer, so the fairness
// check can run the reference reasoner over it before any live run exists.
//
// T10.4 records the layer's real response for each cell. A difference
// between this expectation and the served response is reported, not
// repaired: the freeze states what the memory should hold, and the layer's
// answer is the measurement.
//
// The response carries the contract's fields — beliefs with their evidence,
// precedence and status, the pool, any overriding correction, the guidance
// block (which ProjectRecall drops for both Memento arms), the pending
// count and the trace id. Belief ids and evidence ids are minted
// deterministically from the shopper id and row order; the live layer mints
// its own, so no run may compare ids.
func ExpectedRecall(shopper seed.Round2Shopper, population seed.Population) ([]byte, error) {
	beliefs, err := expectedBeliefs(shopper)
	if err != nil {
		return nil, err
	}
	pool, err := expectedPool(shopper, population, beliefs)
	if err != nil {
		return nil, err
	}
	correction := expectedCorrection(shopper)
	winner := expectedWinner(beliefs, pool, correction)
	applyPrecedence(beliefs, pool, winner, correction)
	response := map[string]any{
		"shopper_id":            shopper.ShopperID,
		"beliefs":               beliefs,
		"pool":                  pool,
		"pool_lookup":           "consulted",
		"overriding_correction": correction,
		"guidance":              expectedGuidance(shopper, winner, beliefs, pool, correction),
		"pending_count":         PendingCount(shopper),
		"trace_id":              nil,
	}
	encoded, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("round2: encode expected recall: %w", err)
	}
	return encoded, nil
}

// expectedBeliefs builds the shopper's own fit beliefs in the contract's
// shape: each carries its value, the ledger label its evidence count earns,
// its standing, and its evidence rows.
func expectedBeliefs(shopper seed.Round2Shopper) ([]map[string]any, error) {
	beliefs := ownBeliefs(shopper)
	out := make([]map[string]any, 0, len(beliefs))
	for i, belief := range beliefs {
		evidence, err := beliefEvidence(shopper, belief, i)
		if err != nil {
			return nil, err
		}
		status, standing := "Building", "Candidate"
		if len(evidence) >= 3 {
			status, standing = "Earned", "Venerable"
		}
		out = append(out, map[string]any{
			"belief_id":       fmt.Sprintf("blf_r2_%s_%02d", shopper.ShopperID, i+1),
			"kind":            string(domain.BeliefKindFit),
			"subject":         subjectOf(shopper),
			"value":           beliefValue(belief.direction, belief.size),
			"status":          status,
			"standing":        standing,
			"contradicted":    false,
			"contradicted_by": nil,
			"evidence":        evidence,
			"precedence":      nil,
			"seeded":          true,
			"updated_at":      recallTime(shopper, belief.lastDay),
		})
	}
	if len(out) == 0 {
		if feedback := FeedbackRows(shopper); len(feedback) > 0 {
			// No fit return formed a belief, so the feedback stands as
			// the emerging belief for the size the shopper said fits.
			worn := feedback[len(feedback)-1]
			out = append(out, map[string]any{
				"belief_id":       fmt.Sprintf("blf_r2_%s_fb", shopper.ShopperID),
				"kind":            string(domain.BeliefKindFit),
				"subject":         subjectOf(shopper),
				"value":           "fits well, take size " + worn.Size,
				"status":          "Building",
				"standing":        "Candidate",
				"contradicted":    false,
				"contradicted_by": nil,
				"evidence":        []map[string]any{feedbackEvidence(shopper, worn)},
				"precedence":      nil,
				"seeded":          true,
				"updated_at":      recallTime(shopper, worn.Day),
			})
		}
	}
	// A statement is served as a Claimed belief with no evidence, exactly
	// as the layer serves one.
	if stated := StatementSize(shopper); stated != "" {
		out = append(out, map[string]any{
			"belief_id":       fmt.Sprintf("blf_r2_%s_stmt", shopper.ShopperID),
			"kind":            string(domain.BeliefKindFit),
			"subject":         subjectOf(shopper),
			"value":           "shopper says they are usually size " + stated,
			"status":          "Claimed",
			"standing":        "None",
			"contradicted":    false,
			"contradicted_by": nil,
			"evidence":        []map[string]any{},
			"precedence":      nil,
			"seeded":          true,
			"updated_at":      recallTime(shopper, 0),
		})
	}
	return out, nil
}

// beliefEvidence renders the rows that back one belief: the classified
// returns pointing its way, plus the fit feedback tied to a purchase in
// its direction, in event order.
func beliefEvidence(shopper seed.Round2Shopper, belief ownBelief, index int) ([]map[string]any, error) {
	out := []map[string]any{}
	for _, row := range shopper.History.Rows {
		if row.Item.BrandID != shopper.Candidate.BrandID || row.Signal != domain.SignalClassifiedOutcome || row.Reason == nil {
			continue
		}
		if row.Reason.Category.CountsToward() != domain.BeliefKindFit {
			continue
		}
		if row.Reason.Category.Direction() != belief.direction {
			continue
		}
		out = append(out, evidenceOf(shopper, row, index))
	}
	for _, row := range shopper.History.Rows {
		if row.Item.BrandID != shopper.Candidate.BrandID || row.Signal != domain.SignalFitFeedback {
			continue
		}
		if index != 0 {
			continue
		}
		out = append(out, evidenceOf(shopper, row, index))
	}
	return out, nil
}

// feedbackEvidence renders the fit feedback row that carries the emerging
// belief.
func feedbackEvidence(shopper seed.Round2Shopper, worn FitReturn) map[string]any {
	for _, row := range shopper.History.Rows {
		if row.Signal == domain.SignalFitFeedback && row.Item.Size == worn.Size {
			return evidenceOf(shopper, row, 0)
		}
	}
	return map[string]any{"outcome": map[string]any{"kind": "capture", "status": "COMPLETED"}}
}

// evidenceOf renders one history row in the contract's evidence shape.
func evidenceOf(shopper seed.Round2Shopper, row seed.Row, index int) map[string]any {
	out := map[string]any{
		"evidence_id": fmt.Sprintf("ev_r2_%06.2f_%d_%s", row.Day, index, row.Item.SKU),
		"item": map[string]any{
			"brand_id":   row.Item.BrandID,
			"brand_name": row.Item.BrandName,
			"line_name":  row.Item.LineName,
			"sku":        row.Item.SKU,
			"size":       row.Item.Size,
			"category":   row.Item.Category,
		},
		"link": map[string]any{
			"order_id":   row.Link.OrderID,
			"capture_id": row.Link.CaptureID,
			"refund_id":  row.Link.RefundID,
			"dispute_id": row.Link.DisputeID,
			"custom_id":  row.Link.CustomID,
			"event_id":   row.Link.EventID,
		},
		"seeded":      true,
		"occurred_at": recallTime(shopper, row.Day),
	}
	if row.Outcome != nil {
		out["outcome"] = map[string]any{
			"kind":         string(row.Outcome.Kind),
			"status":       row.Outcome.Status,
			"amount_minor": row.Outcome.AmountMinor,
			"currency":     row.Outcome.Currency,
		}
	}
	if row.Reason != nil {
		out["reason"] = map[string]any{
			"category":   string(row.Reason.Category),
			"text":       row.Reason.Text,
			"confidence": row.Reason.Confidence,
		}
	}
	// Fit feedback counts toward fit by its verdict, not by a reason
	// category, exactly as the layer counts it.
	counts := row.SeededEvidence(time.Time{}).CountsToward()
	if counts == domain.BeliefKindNone && row.Signal == domain.SignalFitFeedback {
		counts = domain.BeliefKindFit
	}
	out["counts_toward"] = string(counts)
	if op, err := domain.WriteFor(row.Signal); err == nil {
		out["write_op"] = string(op)
	}
	return out
}

// subjectOf names the brand subject a belief is about.
func subjectOf(shopper seed.Round2Shopper) map[string]any {
	return map[string]any{
		"kind": string(domain.SubjectBrand),
		"id":   shopper.Candidate.BrandID,
		"name": shopper.Candidate.BrandName,
	}
}

// beliefValue renders a fit belief's value, as the layer composes it: the
// direction in words and the size one rung off.
func beliefValue(direction domain.FitDirection, size string) string {
	if direction == domain.FitDirectionLarge {
		return "runs large, take size " + size
	}
	return "runs small, take size " + size
}

// expectedPool renders the pooled product fit for the shopper's question,
// or nil when the pool has nothing to say. The counts are the contract's
// full count shape: all event time, the seeded share (every population row
// is seeded), the live share (empty before any live run) and the recent
// window the Out of true rule judges.
func expectedPool(shopper seed.Round2Shopper, population seed.Population, beliefs []map[string]any) (any, error) {
	pool, err := PoolFor(shopper, population)
	if err != nil {
		return nil, err
	}
	if pool.Status == domain.StatusClaimed || pool.Status == "" {
		return nil, nil
	}
	direction := any(nil)
	if pool.Direction != domain.FitDirectionNone {
		direction = string(pool.Direction)
	}
	all := poolCountsView(population.Rows, pool.Line, pool.Size, time.Time{})
	recent := poolCountsView(population.Rows, pool.Line, pool.Size, population.AsOf.Add(-domain.PoolRecentWindow))
	return map[string]any{
		"line": map[string]any{
			"brand_id":   pool.Line.Brand,
			"category":   pool.Line.Category,
			"fit_handle": pool.Line.FitHandle,
		},
		"size":       pool.Size,
		"status":     string(pool.Status),
		"direction":  direction,
		"counts":     all,
		"seeded":     all,
		"live":       poolCountsView(nil, pool.Line, pool.Size, time.Time{}),
		"recent":     recent,
		"precedence": nil,
	}, nil
}

// poolCountsView aggregates one line and size into the contract's count
// shape: settled rows of every kind, the two directions, the fits-well
// share that dilutes them, the reason categories behind the returns, and
// distinct shoppers by their per-line hash. A zero since means all event
// time.
func poolCountsView(rows []domain.FitRow, line domain.ProductLine, size string, since time.Time) map[string]any {
	kept := 0
	small, large, fitsWell := 0, 0, 0
	reasons := map[domain.Category]int{}
	shoppers, shoppersSmall, shoppersLarge := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, row := range rows {
		if row.Line != line || row.Size != size || !row.Settled {
			continue
		}
		if !since.IsZero() && row.EventAt.Before(since) {
			continue
		}
		if row.SettledKind == domain.FitSettledByKept {
			kept++
		}
		if row.SettledKind == domain.FitSettledByReturnReason {
			reasons[row.Reason]++
		}
		switch {
		case row.Direction == domain.FitDirectionSmall:
			small++
			shoppersSmall[row.ShopperHash] = true
		case row.Direction == domain.FitDirectionLarge:
			large++
			shoppersLarge[row.ShopperHash] = true
		case row.Verdict == domain.FitVerdictFitsWell:
			fitsWell++
		default:
			continue
		}
		shoppers[row.ShopperHash] = true
	}
	return map[string]any{
		"settled": small + large + fitsWell + kept + reasons[domain.CategoryDamagedOrDefective] + reasons[domain.CategoryNotArrived] + reasons[domain.CategoryChangedMind],
		"kept":    kept,
		"returns": map[string]any{
			"too_small": small,
			"too_large": large,
		},
		"feedback": map[string]any{
			"fits_well": fitsWell,
			"too_small": 0,
			"too_large": 0,
		},
		"reasons": map[string]any{
			"fit_too_small":        reasons[domain.CategoryFitTooSmall],
			"fit_too_large":        reasons[domain.CategoryFitTooLarge],
			"damaged_or_defective": reasons[domain.CategoryDamagedOrDefective],
			"not_arrived":          reasons[domain.CategoryNotArrived],
			"changed_mind":         reasons[domain.CategoryChangedMind],
		},
		"shoppers": map[string]any{
			"fit":       len(shoppers),
			"too_small": len(shoppersSmall),
			"too_large": len(shoppersLarge),
		},
	}
}

// expectedCorrection renders the shopper's correction in the contract's
// shape, declared against the strongest own-evidence belief when there is
// one and standing on its own when there is not.
func expectedCorrection(shopper seed.Round2Shopper) any {
	if shopper.Correction == nil {
		return nil
	}
	correction := map[string]any{
		"correction_id":    fmt.Sprintf("corr_r2_%s", shopper.ShopperID),
		"text":             shopper.Correction.Text,
		"recorded_at":      recallTime(shopper, shopper.History.SessionDays),
		"takes_precedence": true,
	}
	if beliefs := ownBeliefs(shopper); len(beliefs) > 0 {
		correction["contradicts_belief_id"] = fmt.Sprintf("blf_r2_%s_01", shopper.ShopperID)
	}
	return correction
}

// expectedWinner applies product.md's precedence order to the presence of
// each step.
func expectedWinner(beliefs []map[string]any, pool any, correction any) Rung {
	if correction != nil {
		return RungCorrection
	}
	for _, belief := range beliefs {
		if evidence, ok := belief["evidence"].([]map[string]any); ok && len(evidence) > 0 {
			return RungOwnEvidence
		}
	}
	if pool != nil {
		return RungPool
	}
	for range beliefs {
		return RungStatement
	}
	return RungNone
}

// applyPrecedence marks each served step with the step it came from and
// whether the winner overrode it, exactly as the layer's precedence view
// does.
func applyPrecedence(beliefs []map[string]any, pool any, winner Rung, correction any) {
	for _, belief := range beliefs {
		step := RungStatement
		if evidence, ok := belief["evidence"].([]map[string]any); ok && len(evidence) > 0 {
			step = RungOwnEvidence
		}
		belief["precedence"] = precedenceOf(step, winner)
		if winner == RungCorrection && step != RungCorrection {
			belief["contradicted"] = true
			belief["contradicted_by"] = correction
		}
	}
	if poolView, ok := pool.(map[string]any); ok {
		poolView["precedence"] = precedenceOf(RungPool, winner)
	}
}

// precedenceOf renders one item's precedence: the step it came from, and
// the winner that overrode it when there is one.
func precedenceOf(step, winner Rung) map[string]any {
	if step == winner {
		return map[string]any{"step": string(step), "overridden": false, "overridden_by": nil}
	}
	return map[string]any{"step": string(step), "overridden": true, "overridden_by": string(winner)}
}

// expectedGuidance renders the guidance block the projection drops. It
// exists so the fixture is the whole response and the projection is tested
// against a real one.
func expectedGuidance(shopper seed.Round2Shopper, winner Rung, beliefs []map[string]any, pool any, correction any) map[string]any {
	guidance := map[string]any{
		"text":          "No memory applies yet: ask the shopper which size they usually take.",
		"basis":         string(RungNone),
		"correction_id": nil,
		"belief_id":     nil,
	}
	switch winner {
	case RungCorrection:
		if view, ok := correction.(map[string]any); ok {
			guidance["text"] = "Take size " + shopper.Correction.WantSize + " for " + shopper.Candidate.BrandName + ": the shopper's correction."
			guidance["basis"] = string(RungCorrection)
			guidance["correction_id"] = view["correction_id"]
			if id, ok := view["contradicts_belief_id"]; ok {
				guidance["belief_id"] = id
			}
		}
	case RungOwnEvidence:
		size := ""
		if beliefs := ownBeliefs(shopper); len(beliefs) > 0 {
			size = beliefs[0].size
		} else if feedback := FeedbackRows(shopper); len(feedback) > 0 {
			size = feedback[len(feedback)-1].Size
		}
		guidance["text"] = "Take size " + size + " for " + shopper.Candidate.BrandName + "."
		guidance["basis"] = string(RungOwnEvidence)
		if len(beliefs) > 0 {
			guidance["belief_id"] = beliefs[0]["belief_id"]
		}
	case RungPool:
		if view, ok := pool.(map[string]any); ok {
			size := SizeForDirection(view["size"].(string), directionOf(view["direction"]))
			guidance["text"] = "Size " + size + " for " + shopper.Candidate.BrandName + "."
			guidance["basis"] = string(RungPool)
		}
	case RungStatement:
		guidance["text"] = "Take size " + StatementSize(shopper) + " for " + shopper.Candidate.BrandName + ": the shopper's usual size."
		guidance["basis"] = string(RungStatement)
		for _, belief := range beliefs {
			if evidence, ok := belief["evidence"].([]map[string]any); ok && len(evidence) == 0 {
				guidance["belief_id"] = belief["belief_id"]
			}
		}
	}
	return guidance
}

// directionOf reads a pool view's direction back.
func directionOf(value any) domain.FitDirection {
	text, _ := value.(string)
	return domain.FitDirection(text)
}

// recallTime renders one row's event time as an RFC3339 timestamp,
// measured from the round-2 anchor. Only the order of the times matters;
// the anchor keeps the fixture byte-stable.
func recallTime(shopper seed.Round2Shopper, day float64) string {
	return seed.Round2Anchor().Add(time.Duration(day * float64(24*time.Hour))).UTC().Format(time.RFC3339)
}
