package server

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/memory"
)

// beliefContent composes the graph content for one belief: short, keyed,
// and stable for the same belief, so evidence cites it and corrections
// declare against it.
func beliefContent(kind domain.BeliefKind, subject domain.Subject, value string) string {
	return fmt.Sprintf("memento %s %s %s %s", string(kind), string(subject.Kind), subject.ID, value)
}

// stubKey composes the short key inside a graph stub: the outcome and the
// event that carried it. It names the row, never the shopper's words.
func stubKey(row domain.Evidence) string {
	return fmt.Sprintf("%s %s", string(row.Outcome.Kind), row.Link.EventID)
}

// ensureKind maps a belief kind to its derive kind. Fit and seller beliefs
// derive as logic so structural evidence can promote them.
func ensureKind() string {
	return memory.ConceptBelief
}

// sizeLadder orders apparel sizes for recommendations. A return blaming a
// too-small size recommends one rung up; too-large recommends one down.
var sizeLadder = []string{"XXS", "XS", "S", "M", "L", "XL", "XXL"}

// neighbourSize moves one rung on the ladder in the given direction. An
// unknown size or the ladder's edge stays where it is: the service never
// invents a size it cannot name.
func neighbourSize(size string, up bool) string {
	for i, rung := range sizeLadder {
		if !strings.EqualFold(rung, size) {
			continue
		}
		if up && i+1 < len(sizeLadder) {
			return sizeLadder[i+1]
		}
		if !up && i-1 >= 0 {
			return sizeLadder[i-1]
		}
		return sizeLadder[i]
	}
	return size
}

// fitBeliefValue composes the value for a new fit belief from the first
// classified direction: too small sizes up, too large sizes down. The
// value carries the worn size's verdict, not the shopper's words.
func fitBeliefValue(category domain.Category, brandName, size string) string {
	switch category {
	case domain.CategoryFitTooSmall:
		return fmt.Sprintf("runs small, take size %s", neighbourSize(size, true))
	default:
		return fmt.Sprintf("runs large, take size %s", neighbourSize(size, false))
	}
}

// sellerSubject derives the seller subject behind a brand's transaction.
// Dispute records carry no seller identity, so the layer keys the seller
// belief to the brand it sold through and says so in the name.
func sellerSubject(item domain.Item) domain.Subject {
	name := strings.TrimSpace(item.BrandName)
	if name == "" {
		name = strings.TrimSpace(item.BrandID)
	}
	return domain.Subject{
		Kind: domain.SubjectSeller,
		ID:   "seller-for-" + strings.TrimSpace(item.BrandID),
		Name: name + " seller",
	}
}

// feedbackBeliefValue composes the value for the emerging belief that fit
// feedback supports: the worn size fits this shopper now.
func feedbackBeliefValue(size string) string {
	return fmt.Sprintf("size %s fits this shopper now", size)
}

// sizeToken finds size mentions in free text: apparel rungs or numeric
// waist sizes. The match is case-insensitive; the returned token keeps
// the ladder's casing.
var sizeToken = regexp.MustCompile(`(?i)\b(XXS|XS|S|M|L|XL|XXL|[2-5][0-9](?:\.\d)?)\b`)

// lastSize returns the last size token in free text, or empty when none
// applies. Corrections name the size that works now; the recommendation
// follows the shopper's latest word.
func lastSize(text string) string {
	found := sizeToken.FindAllString(text, -1)
	if len(found) == 0 {
		return ""
	}
	last := found[len(found)-1]
	upper := strings.ToUpper(last)
	for _, rung := range sizeLadder {
		if rung == upper {
			return rung
		}
	}
	return last
}

// guidanceForCorrection tells the agent what to recommend from the
// shopper's correction: the named size for the contradicted belief's
// subject. The correction wins immediately, without waiting for evidence.
func guidanceForCorrection(correction contract.CorrectionView, belief *beliefRecord) contract.GuidanceView {
	size := lastSize(correction.Text)
	text := fmt.Sprintf("Take size %s for %s", size, belief.subject.Name)
	if size == "" {
		text = fmt.Sprintf("Follow the shopper's correction for %s", belief.subject.Name)
	}
	correctionID := correction.CorrectionID
	beliefID := belief.id
	return contract.GuidanceView{
		Text:         text,
		Basis:        contract.GuidanceCorrection,
		CorrectionID: &correctionID,
		BeliefID:     &beliefID,
	}
}

// guidanceForBelief tells the agent what to recommend from the strongest
// belief: the size its value recommends, for its subject.
func guidanceForBelief(rec *beliefRecord) contract.GuidanceView {
	size := lastSize(rec.value)
	text := fmt.Sprintf("Take size %s for %s", size, rec.subject.Name)
	if size == "" {
		text = fmt.Sprintf("Follow the earned belief for %s", rec.subject.Name)
	}
	beliefID := rec.id
	return contract.GuidanceView{
		Text:     text,
		Basis:    contract.GuidanceBelief,
		BeliefID: &beliefID,
	}
}

// noGuidance applies when nothing is earned and no correction applies.
// The agent asks about the gaps instead of guessing.
func noGuidance() contract.GuidanceView {
	return contract.GuidanceView{
		Text:  "No recommendation yet: too little evidence.",
		Basis: contract.GuidanceNone,
	}
}

// verdictForFeedback reads explicit fit feedback tied to a purchase and
// returns the layer's verdict. It reads the shopper's words about the
// worn size, never a reason category: a fits-well verdict has no reason
// category yet counts toward fit. Anything it cannot read is unclear,
// and the agent asks again.
func verdictForFeedback(text string) contract.FeedbackVerdict {
	lowered := strings.ToLower(text)
	switch {
	case strings.Contains(lowered, "fits well") || strings.Contains(lowered, "fits great") ||
		strings.Contains(lowered, "perfect fit") || strings.Contains(lowered, "fits perfectly"):
		return contract.FeedbackFitsWell
	case strings.Contains(lowered, "too tight") || strings.Contains(lowered, "too small") ||
		strings.Contains(lowered, "too short") || strings.Contains(lowered, "cannot button") ||
		strings.Contains(lowered, "can't button"):
		return contract.FeedbackTooSmall
	case strings.Contains(lowered, "too big") || strings.Contains(lowered, "too loose") ||
		strings.Contains(lowered, "too large") || strings.Contains(lowered, "swims"):
		return contract.FeedbackTooLarge
	case strings.Contains(lowered, "broken") || strings.Contains(lowered, "defective") ||
		strings.Contains(lowered, "damaged") || strings.Contains(lowered, "cracked"):
		return contract.FeedbackDamagedOrDefective
	case strings.Contains(lowered, "never arrived") || strings.Contains(lowered, "didn't arrive") ||
		strings.Contains(lowered, "did not arrive") || strings.Contains(lowered, "not arrived"):
		return contract.FeedbackNotArrived
	case strings.Contains(lowered, "changed my mind") || strings.Contains(lowered, "don't want"):
		return contract.FeedbackChangedMind
	default:
		return contract.FeedbackUnclear
	}
}
