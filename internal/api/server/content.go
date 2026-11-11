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
// A dispute names a seller, but only the payment account that received the
// money, which is one account for the whole layer; so the layer keys the
// seller belief to the brand it sold through and says so in the name
// (docs/architecture.md, "The seller subject").
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
// subject, or the size alone while the correction stands on its own. The
// correction wins immediately, without waiting for evidence.
func guidanceForCorrection(correction contract.CorrectionView, subjectName string, beliefID *string) contract.GuidanceView {
	size := lastSize(correction.Text)
	text := "Follow the shopper's correction"
	switch {
	case size != "" && subjectName != "":
		text = fmt.Sprintf("Take size %s for %s", size, subjectName)
	case size != "":
		text = fmt.Sprintf("Take size %s", size)
	case subjectName != "":
		text = fmt.Sprintf("Follow the shopper's correction for %s", subjectName)
	}
	correctionID := correction.CorrectionID
	return contract.GuidanceView{
		Text:         text,
		Basis:        contract.GuidanceCorrection,
		CorrectionID: &correctionID,
		BeliefID:     beliefID,
	}
}

// guidanceForOwnEvidence tells the agent what to recommend from the
// shopper's own fit evidence: the size its value recommends, for its
// subject. When a pool rides beside the belief, the sentence also states
// what the shopper's own rows found — fits well, runs small or runs large,
// read from the belief's own value rather than assumed — with how many rows
// said it, and whether the pool recommends the same size or another one.
// With no pool there is nothing to compare: the sentence names the size and
// makes no claim about a pool.
func guidanceForOwnEvidence(view contract.BeliefView, poolView *contract.PoolView) contract.GuidanceView {
	size := lastSize(view.Value)
	name := view.Subject.Name
	if name == "" {
		name = view.Subject.ID
	}
	text := fmt.Sprintf("Take size %s for %s", size, name)
	if size == "" {
		text = fmt.Sprintf("Follow the earned belief for %s", name)
	} else if poolAgrees(size, poolView) {
		text = fmt.Sprintf("%s: %s, and the pool agrees.", text, ownEvidenceClause(len(view.Evidence), fitWords(view.Value)))
	} else if poolView != nil {
		text = fmt.Sprintf("%s: %s, ahead of the pool.", text, ownEvidenceClause(len(view.Evidence), fitWords(view.Value)))
	}
	beliefID := view.BeliefID
	return contract.GuidanceView{
		Text:     text,
		Basis:    contract.GuidanceOwnEvidence,
		BeliefID: &beliefID,
	}
}

// fitWords states what one own-evidence belief says about the fit in the
// sentence's words: runs small, runs large, or fits well. The words come
// from the belief's own value, so a fits-well belief never claims a
// direction it did not report. A value naming neither reads empty, and the
// clause then states the rows alone.
func fitWords(value string) string {
	switch domain.DirectionForValue(value) {
	case domain.FitDirectionSmall:
		return "runs small"
	case domain.FitDirectionLarge:
		return "runs large"
	}
	if strings.Contains(strings.ToLower(value), "fits") {
		return "fits well"
	}
	return ""
}

// poolAgrees reports whether the pool recommends the size the shopper's
// own evidence does. The pool names a size by stepping its consulted size
// one rung the way its direction points; a pool naming no direction names
// no size, so it never agrees.
func poolAgrees(own string, poolView *contract.PoolView) bool {
	if own == "" || poolView == nil || poolView.Direction == nil {
		return false
	}
	up := *poolView.Direction == domain.FitDirectionSmall
	return strings.EqualFold(neighbourSize(poolView.Size, up), own)
}

// ownEvidenceClause states the shopper's own rows behind a recommendation,
// with the verb agreeing with the count: one return says, several say. The
// count itself comes from evidenceCount, so the phrasing has one home. The
// fit words follow the pronoun when the belief names a direction, and the
// clause says "so" when it names none.
func ownEvidenceClause(n int, fit string) string {
	verb := "say"
	if n == 1 {
		verb = "says"
	}
	if fit == "" {
		return fmt.Sprintf("your own %s %s so", evidenceCount(n), verb)
	}
	return fmt.Sprintf("your own %s %s it %s", evidenceCount(n), verb, fit)
}

// evidenceCount states how many evidence rows back a recommendation, in
// the words the guidance uses.
func evidenceCount(n int) string {
	switch n {
	case 1:
		return "one return"
	case 2:
		return "two returns"
	default:
		return fmt.Sprintf("%d returns", n)
	}
}

// guidanceForPool tells the agent what to recommend from the pooled
// product fit, adjusted from the shopper's usual size: up when the pool
// says the product runs small, down when it runs large. direction is the
// direction the pool named when recall took its step: a direction that
// names neither way has no size to recommend, so it guides nothing rather
// than reading the gap as large. The counts ride along, so the agent
// cites the pool instead of the shopper.
func guidanceForPool(view contract.PoolView, direction domain.FitDirection, brand string) contract.GuidanceView {
	size := view.Size
	small := view.Counts.Returns.TooSmall + view.Counts.Feedback.TooSmall
	large := view.Counts.Returns.TooLarge + view.Counts.Feedback.TooLarge
	fit := small + large + view.Counts.Feedback.FitsWell
	var up bool
	switch direction {
	case domain.FitDirectionSmall:
		up = true
	case domain.FitDirectionLarge:
		up = false
	default:
		return noGuidance()
	}
	recommend := neighbourSize(size, up)
	verb, blamed, excused := "up to", "too small", "too large"
	if !up {
		verb, blamed, excused = "down to", "too large", "too small"
	}
	if recommend == size {
		return contract.GuidanceView{
			Text:  fmt.Sprintf("Stay with size %s for %s: %d of the %d fit-classified settled purchases at size %s came back %s, against %d %s.", size, brand, blamedCount(small, large, up), fit, size, blamed, excusedCount(small, large, up), excused),
			Basis: contract.GuidancePool,
		}
	}
	return contract.GuidanceView{
		Text:  fmt.Sprintf("Size %s %s for %s: %d of the %d fit-classified settled purchases at size %s came back %s, against %d %s.", verb, recommend, brand, blamedCount(small, large, up), fit, size, blamed, excusedCount(small, large, up), excused),
		Basis: contract.GuidancePool,
	}
}

// blamedCount reports the direction's outcomes for guidance: the small
// count when the pool blames small, the large count otherwise.
func blamedCount(small, large int, up bool) int {
	if up {
		return small
	}
	return large
}

// excusedCount reports the opposite direction's outcomes for guidance.
func excusedCount(small, large int, up bool) int {
	if up {
		return large
	}
	return small
}

// guidanceForStatement tells the agent what to recommend from the
// shopper's general statement: the size they said they usually take. A
// statement is Claimed and never promotes; it decides only when no
// evidence and no pool speak.
func guidanceForStatement(view contract.BeliefView) contract.GuidanceView {
	size := lastSize(view.Value)
	name := view.Subject.Name
	if name == "" {
		name = view.Subject.ID
	}
	text := fmt.Sprintf("Follow the shopper's usual size for %s", name)
	if size != "" {
		text = fmt.Sprintf("Take size %s for %s: the shopper's usual size.", size, name)
	}
	beliefID := view.BeliefID
	return contract.GuidanceView{
		Text:     text,
		Basis:    contract.GuidanceStatement,
		BeliefID: &beliefID,
	}
}

// noGuidance applies when nothing is earned and no correction applies.
// The agent asks about the gaps instead of guessing.
func noGuidance() contract.GuidanceView {
	return contract.GuidanceView{
		Text:  "No memory applies yet: no correction, no fit evidence, no pool and no general statement. Ask the shopper which size they usually take.",
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
