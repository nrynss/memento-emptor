package grader

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/seed"
)

// TokenBasis names the word-count counter every token figure on a score
// comes from. It is recorded on the score itself so later stages cannot
// silently mix tokenizer bases: T5.1's handoff warns the live BPE count
// runs about five times hotter than this proxy.
const TokenBasis = "harness-words-v1: whitespace-separated words per harness.CountTokens"

// ErrNoHistories is returned when a run names a shopper with no history rows to check against.
var ErrNoHistories = errors.New("grader: run carries no history")

// ErrAnswerMismatch is returned when the hidden answer names a different shopper than the run.
var ErrAnswerMismatch = errors.New("grader: hidden answer is for another shopper")

// ErrNoSecondTurn is returned when a correction is given without the very next recommendation turn.
var ErrNoSecondTurn = errors.New("grader: correction needs the very next recommendation turn")

// Turn is one recommendation turn: what the model saw, what it said,
// the size it named, and the evidence refs it offered as support for
// that size. Refs are transaction ids from the history links (CustomID,
// OrderID, RefundID, DisputeID, CaptureID, EventID) or a distinctive
// substring of a row's note.
type Turn struct {
	// Prompt is what the model saw for this recommendation.
	Prompt string
	// Reply is the model's recommendation text.
	Reply string
	// Size is the size the reply recommends, such as M.
	Size string
	// Cites are the evidence refs the reply offers as support for Size.
	Cites []string
}

// Correction is the shopper's explicit correction between two
// recommendation turns: the wanted size, in the shopper's words.
type Correction struct {
	// Text is what the shopper said, such as "I've lost weight, use M".
	Text string
	// WantSize is the size the correction asks for, such as M.
	WantSize string
}

// Run is one hand-built graded run: the arm-facing history, the hidden
// right size, the first recommendation, and, when the shopper corrects,
// the very next recommendation turn.
type Run struct {
	// ShopperID names the shopper the run recommends for.
	ShopperID string
	// History is the arm-facing seeded history the evidence is checked against.
	History seed.History
	// Answer is the hidden right size for this shopper.
	Answer seed.Answer
	// First is the first recommendation turn.
	First Turn
	// Correction is the shopper's correction, or nil when there is none.
	Correction *Correction
	// Second is the very next recommendation after the correction. It is
	// required when Correction is set and must be nil otherwise.
	Second *Turn
}

// CiteVerdict is the mechanical check of one cited ref: whether it
// resolves to exactly one history row and which belief kind that row
// counts toward.
type CiteVerdict struct {
	// Ref is the cited ref as the reply gave it.
	Ref string
	// Resolved is true when the ref matches exactly one history row.
	Resolved bool
	// Kind is what the matched row counts toward, or BeliefKindNone when unresolved.
	Kind domain.BeliefKind
	// Supports is true when the cite backs a size recommendation: it
	// resolves and its row counts toward fit.
	Supports bool
	// Reason states why the cite fails, or which row it resolves to.
	Reason string
}

// Score is the graded run on the five measures, as counts. RightSize,
// WrongCause, Supported/Cited and the correction verdict are counts (0/1
// where binary); Tokens are counts of words under TokenBasis.
// Opinions carries model judgements: always empty offline, never a
// measurement.
type Score struct {
	// ShopperID names the graded shopper.
	ShopperID string
	// RightSize is true when the first recommendation names the hidden right size.
	RightSize bool
	// WrongCause counts cited non-fit evidence behind a size that moves
	// off the shopper's baseline kept size.
	WrongCause int
	// CorrectionHonoured is nil when the run carries no correction; else
	// true when the very next recommendation names the wanted size.
	CorrectionHonoured *bool
	// Supported counts cites that resolve to fit evidence. Cited counts
	// every cite offered across the recommendation turns present.
	Supported int
	// Cited counts every cite offered across the turns present.
	Cited int
	// Cites carries the per-cite verdicts in offer order.
	Cites []CiteVerdict
	// TokensFirst counts prompt plus reply words on the first turn.
	TokensFirst int
	// TokensSecond counts prompt plus reply words on the second turn, or 0.
	TokensSecond int
	// TokenBasis records which counter produced the token figures.
	TokenBasis string
	// OverCap is true when the run's tokens or spend exceed the pilot caps.
	OverCap bool
	// Opinions carries model judgements. The offline grader records none.
	Opinions []string
}

// LoadAnswers reads the hidden answers file the grader scores against.
// The harness itself must never call this; the separation is what keeps
// the arms blind while the grader sees the key.
func LoadAnswers(path string) (map[string]seed.Answer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("grader: read hidden answers: %w", err)
	}
	var answers []seed.Answer
	if err := json.Unmarshal(raw, &answers); err != nil {
		return nil, fmt.Errorf("grader: decode hidden answers: %w", err)
	}
	byShopper := make(map[string]seed.Answer, len(answers))
	for _, a := range answers {
		byShopper[a.ShopperID] = a
	}
	return byShopper, nil
}

// normalizeSize trims and upper-cases a size token so M, m and " M " compare equal.
func normalizeSize(size string) string {
	return strings.ToUpper(strings.TrimSpace(size))
}

// baselineSize is the size the shopper's history stands on: the most
// recent kept-past-window size, else the general-statement size, else
// empty when the history names none.
func baselineSize(history seed.History) string {
	kept := ""
	for _, row := range history.Rows {
		if row.Signal == domain.SignalKeptPastWindow && row.Item.Size != "" {
			kept = row.Item.Size
		}
	}
	if kept != "" {
		return kept
	}
	for _, row := range history.Rows {
		if row.Signal == domain.SignalGeneralStatement && row.Item.Size != "" {
			return row.Item.Size
		}
	}
	return ""
}

// supportKind reports which belief kind a row counts toward for citation
// purposes. Classified fit reasons count toward fit through the domain
// rule; explicit fit feedback tied to a purchase counts toward fit by
// verdict (the layer's rule from the T1.2 handoff), since its row carries
// no reason category at all.
func supportKind(row seed.Row) domain.BeliefKind {
	ev := domain.Evidence{Reason: row.Reason}
	if ev.CountsToward() == domain.BeliefKindFit {
		return domain.BeliefKindFit
	}
	if row.Signal == domain.SignalFitFeedback {
		return domain.BeliefKindFit
	}
	return ev.CountsToward()
}

// matchCite resolves one cited ref to exactly one history row: first by
// exact transaction-id match across the row links, then by a
// case-insensitive substring of exactly one row's note.
func matchCite(history seed.History, ref string) (seed.Row, bool, string) {
	want := strings.TrimSpace(ref)
	if want == "" {
		return seed.Row{}, false, "empty citation"
	}
	for _, row := range history.Rows {
		link := row.Link
		for _, id := range []string{link.CustomID, link.OrderID, link.CaptureID, link.RefundID, link.DisputeID, link.EventID} {
			if id != "" && want == id {
				return row, true, "link " + id
			}
		}
	}
	var hits []seed.Row
	for _, row := range history.Rows {
		if row.Note != "" && strings.Contains(strings.ToLower(row.Note), strings.ToLower(want)) {
			hits = append(hits, row)
		}
	}
	if len(hits) == 1 {
		return hits[0], true, "note match"
	}
	if len(hits) > 1 {
		return seed.Row{}, false, fmt.Sprintf("ambiguous: matches %d rows", len(hits))
	}
	return seed.Row{}, false, "no history row carries this ref"
}

// gradeCites checks every cite of one turn and reports the verdicts, the
// supported count, and the wrong-cause count: resolved non-fit cites
// behind a size that moves off the baseline.
func gradeCites(history seed.History, turn Turn, baseline string) ([]CiteVerdict, int, int) {
	verdicts := make([]CiteVerdict, 0, len(turn.Cites))
	supported, wrong := 0, 0
	moved := baseline != "" && normalizeSize(turn.Size) != "" && normalizeSize(turn.Size) != normalizeSize(baseline)
	for _, ref := range turn.Cites {
		row, ok, how := matchCite(history, ref)
		verdict := CiteVerdict{Ref: ref, Resolved: ok}
		if !ok {
			verdict.Kind = domain.BeliefKindNone
			verdict.Reason = "unresolved: " + how
			verdicts = append(verdicts, verdict)
			continue
		}
		verdict.Kind = supportKind(row)
		if verdict.Kind == domain.BeliefKindFit {
			verdict.Supports = true
			verdict.Reason = "resolves by " + how + " to fit evidence"
			supported++
		} else {
			verdict.Reason = "resolves by " + how + " but counts toward " + quoteKind(verdict.Kind) + ", not fit"
			if moved {
				wrong++
			}
		}
		verdicts = append(verdicts, verdict)
	}
	return verdicts, supported, wrong
}

// quoteKind renders a belief kind for verdict reasons, naming the empty kind explicitly.
func quoteKind(kind domain.BeliefKind) string {
	if kind == domain.BeliefKindNone {
		return "nothing"
	}
	return string(kind)
}

// ScoreRun grades one hand-built run on the five measures. It reads only
// the run it is given: no key, no network, no model call.
func ScoreRun(run Run) (Score, error) {
	if len(run.History.Rows) == 0 {
		return Score{}, fmt.Errorf("%w: %q", ErrNoHistories, run.ShopperID)
	}
	if run.Answer.ShopperID != "" && run.Answer.ShopperID != run.ShopperID {
		return Score{}, fmt.Errorf("%w: run %q answer %q", ErrAnswerMismatch, run.ShopperID, run.Answer.ShopperID)
	}
	if run.Correction != nil && run.Second == nil {
		return Score{}, fmt.Errorf("%w: %q", ErrNoSecondTurn, run.ShopperID)
	}
	if run.Correction == nil && run.Second != nil {
		return Score{}, fmt.Errorf("grader: second turn without a correction: %q", run.ShopperID)
	}
	score := Score{ShopperID: run.ShopperID, TokenBasis: TokenBasis}
	score.RightSize = normalizeSize(run.First.Size) != "" && normalizeSize(run.First.Size) == normalizeSize(run.Answer.RightSize)
	baseline := baselineSize(run.History)
	firstVerdicts, firstSupported, firstWrong := gradeCites(run.History, run.First, baseline)
	score.Cites = append(score.Cites, firstVerdicts...)
	score.Supported += firstSupported
	score.WrongCause += firstWrong
	score.Cited += len(run.First.Cites)
	score.TokensFirst = harness.CountTokens(run.First.Prompt) + harness.CountTokens(run.First.Reply)
	if run.Correction != nil {
		honoured := normalizeSize(run.Second.Size) == normalizeSize(run.Correction.WantSize) && normalizeSize(run.Second.Size) != ""
		score.CorrectionHonoured = &honoured
		secondVerdicts, secondSupported, secondWrong := gradeCites(run.History, *run.Second, baseline)
		score.Cites = append(score.Cites, secondVerdicts...)
		score.Supported += secondSupported
		score.WrongCause += secondWrong
		score.Cited += len(run.Second.Cites)
		score.TokensSecond = harness.CountTokens(run.Second.Prompt) + harness.CountTokens(run.Second.Reply)
	}
	caps := harness.DefaultCaps()
	total := score.TokensFirst + score.TokensSecond
	score.OverCap = total > caps.MaxTokens || caps.SpendFor(total) > caps.MaxSpendUSD
	return score, nil
}
