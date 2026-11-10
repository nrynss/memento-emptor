package pilot

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/nrynss/memento-emptor/bench/grader"
	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/seed"
)

// Extraction rule (T5.3 pilot; freeze input for T5.4).
//
// The grader scores Size and Cites, but live replies are free text, so a
// reply must first be reduced to those fields. The reduction has two steps.
// The deterministic step runs in code (ExtractSize, ExtractIDRefs) and is
// tested offline. The human step maps prose references to evidence refs and
// is recorded verbatim in the run file with the reply quote it came from,
// so the freeze can replay or replace it. Extraction never consults the
// hidden answers; it reads only the reply and the arm-facing history.
//
// Deterministic Size, first match wins:
//  1. explicit: the first "size X" mention (case-insensitive, optional
//     colon or dash) whose token is in the size vocabulary below;
//  2. standalone: the first whitespace-separated token, stripped of
//     surrounding punctuation, that is in the size vocabulary;
//  3. none: no size named (the agent asked a question instead of
//     recommending, or recommended without a parseable token).
// A human override (HumanSize plus HumanSizeReason) wins over the
// deterministic value only when the deterministic pick is judged not to be
// the reply's recommendation, and the reason is recorded.
//
// Deterministic Cites: every transaction id from the history's links
// (CustomID, OrderID, CaptureID, RefundID, DisputeID, EventID) that appears
// verbatim in the reply. SKU or size mentions are never cites: the grader
// resolves cites by link id first and then by note substring, and neither
// path resolves a bare SKU.
// Human Cites: prose references ("the torn-seam refund") mapped by the
// pilot author to a note-substring ref that resolves to exactly one history
// row, each recorded with the reply quote and the reason. Prose too vague
// to map to one row is not a cite; the run file says so instead of
// inventing one.
//
// Contamination: for comparison shoppers the layer holds nothing, and the
// agent's recall returns no memory. A reply that names demo-shopper facts
// (CheckContamination) proves the run read another shopper's memory and is
// discarded, not graded.

// sizeVocabulary is the closed set of recommendable size tokens. Waist
// numbers cover the comparison lines; OS covers one-size lines.
var sizeVocabulary = map[string]bool{
	"XS": true, "S": true, "M": true, "L": true,
	"XL": true, "XXL": true, "2XL": true, "3XL": true,
	"OS": true,
	"28": true, "30": true, "32": true, "34": true,
	"36": true, "38": true, "40": true,
}

// sizeMention matches an explicit "size X" recommendation.
var sizeMention = regexp.MustCompile(`(?i)\bsizes?\s*[:\-–—]?\s*([A-Za-z0-9]+)`)

// SizeRule names which deterministic rule produced a size.
type SizeRule string

// Deterministic size rules, in precedence order.
const (
	// SizeExplicit names a size from a "size X" mention.
	SizeExplicit SizeRule = "explicit-size"
	// SizeStandalone names a size from a standalone vocabulary token.
	SizeStandalone SizeRule = "standalone-token"
	// SizeNone means the reply names no vocabulary size.
	SizeNone SizeRule = "none"
)

// cleanToken upper-cases a candidate size token and strips surrounding
// punctuation so "M." and "(L)" compare equal to M and L.
func cleanToken(raw string) string {
	return strings.ToUpper(strings.Trim(raw, ".,;:!?*\"'()[]{}"))
}

// ExtractSize reduces a free-text reply to the recommended size by the
// deterministic rule above. It returns the size (or empty) and the rule
// that fired.
func ExtractSize(reply string) (string, SizeRule) {
	for _, match := range sizeMention.FindAllStringSubmatch(reply, -1) {
		if candidate := cleanToken(match[1]); sizeVocabulary[candidate] {
			return candidate, SizeExplicit
		}
	}
	for _, field := range strings.Fields(reply) {
		if candidate := cleanToken(field); sizeVocabulary[candidate] {
			return candidate, SizeStandalone
		}
	}
	return "", SizeNone
}

// ExtractIDRefs returns the history's transaction ids that appear verbatim
// in the reply, in history row order, deduplicated. These are the cites
// the deterministic step can defend without judgement.
func ExtractIDRefs(history seed.History, reply string) []string {
	var refs []string
	seen := map[string]bool{}
	for _, row := range history.Rows {
		link := row.Link
		for _, id := range []string{
			link.CustomID, link.OrderID, link.CaptureID,
			link.RefundID, link.DisputeID, link.EventID,
		} {
			if id == "" || seen[id] || !strings.Contains(reply, id) {
				continue
			}
			seen[id] = true
			refs = append(refs, id)
		}
	}
	return refs
}

// demoTokens are facts only the demo shopper's memory holds. A comparison
// run whose reply names any of them read another shopper's memory.
var demoTokens = []string{
	"7XK", "Brand X", "blf_", "corr_", "ev_", "PSW8GeK", "tbnrl46",
	"memento-shopper01", "shopper_01", "shopper 01",
}

// CheckContamination reports whether the reply names demo-shopper facts.
// A contaminated run is discarded, never graded: it measured the wrong
// memory, not the arm.
func CheckContamination(reply string) (bool, []string) {
	var hits []string
	for _, token := range demoTokens {
		if strings.Contains(reply, token) {
			hits = append(hits, token)
		}
	}
	return len(hits) > 0, hits
}

// ErrNoHistory is returned when grading names a shopper with no history.
var ErrNoHistory = errors.New("pilot: no history for shopper")

// GradeFirst scores one pilot run's first (and only) recommendation turn
// with the frozen grader. size and cites are the effective extraction: the
// human values where the run file sets them, else the deterministic ones.
func GradeFirst(history seed.History, answer seed.Answer, prompt, reply, size string, cites []string) (grader.Score, error) {
	if len(history.Rows) == 0 {
		return grader.Score{}, fmt.Errorf("%w: %q", ErrNoHistory, history.ShopperID)
	}
	return grader.ScoreRun(grader.Run{
		ShopperID: history.ShopperID,
		History:   history,
		Answer:    answer,
		First: grader.Turn{
			Prompt: prompt,
			Reply:  reply,
			Size:   size,
			Cites:  cites,
		},
	})
}

// EffectiveSize returns the size the run grades on: the human override
// where set, else the deterministic extraction.
func EffectiveSize(deterministic, human string) string {
	if human != "" {
		return human
	}
	return deterministic
}

// HarnessSpend converts word-count tokens to dollars at the harness price.
// It exists so the pilot record carries the same comparable figure the dry
// run reports; it is never the live spend, which the trace costs carry.
func HarnessSpend(tokens int) float64 {
	return harness.DefaultCaps().SpendFor(tokens)
}
