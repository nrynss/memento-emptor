package round2

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nrynss/memento-emptor/bench/grader"
	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/bench/pilot"
	"github.com/nrynss/memento-emptor/seed"
)

// The round-2 run record and grader. Both are frozen: the record is what
// every committed turn must carry, and the grader reduces a turn to the
// measures product.md names, as counts.
//
// The extraction rule is round 1's, unchanged: pilot.ExtractSize and
// pilot.ExtractIDRefs, with the same documented human override. The grader
// is round 1's for four of the five measures (right size, wrong cause,
// supported cites, tokens), because the rules are the same and reusing them
// keeps one definition. Round 2 adds the turn a cell records and the
// precedence step the memory served.

// TurnKind names which turn of one cell a run file records. Every cell
// runs a first turn; a correction cell then runs the correction and, for
// arms 3 and 4, the next-session and other-agent turns.
type TurnKind string

// The round-2 turn kinds.
const (
	// TurnFirst is the recommendation before any correction.
	TurnFirst TurnKind = "first"
	// TurnCorrection is the same agent's next recommendation, with the
	// correction in the conversation.
	TurnCorrection TurnKind = "correction"
	// TurnNextSession is the same agent's next recommendation in a new
	// session, with no conversation carried over.
	TurnNextSession TurnKind = "next-session"
	// TurnOtherAgent is another agent's first recommendation for the same
	// shopper, with no conversation carried over.
	TurnOtherAgent TurnKind = "other-agent"
)

// TurnKinds lists the turn kinds in the order a cell runs them.
func TurnKinds() []TurnKind {
	return []TurnKind{TurnFirst, TurnCorrection, TurnNextSession, TurnOtherAgent}
}

// Model is the comparison model string, the same in every arm.
const Model = "claude-sonnet-4-6"

// TokenBasis records which counter produced every token figure, so later
// stages cannot mix bases.
const TokenBasis = "harness-words-v1: whitespace-separated words per harness.CountTokens"

// RunFile is the committed record of one round-2 turn. It carries the
// prompt verbatim, the memory fetch the arm made, the reply and the
// extraction; the score is added by the grader. The hidden answer never
// enters this file.
type RunFile struct {
	// RunID is the local run label, such as r2-a4-crowd-rival-r1-first.
	RunID string `json:"run_id"`
	// ShopperID names the frozen shopper.
	ShopperID string `json:"shopper_id"`
	// CaseID names the case the shopper serves.
	CaseID string `json:"case_id"`
	// Arm is the round-2 arm number, 1 to 4.
	Arm int `json:"arm"`
	// ArmName is the arm's name.
	ArmName string `json:"arm_name"`
	// Repeat is the repetition index, 1 to the frozen repeat count.
	Repeat int `json:"repeat"`
	// Turn names which turn this record is.
	Turn TurnKind `json:"turn"`
	// Agent names the agent that made the recommendation, A or B.
	Agent string `json:"agent"`
	// Model is the comparison model string.
	Model string `json:"model"`
	// Prompt is the full prompt the turn sent, verbatim.
	Prompt string `json:"prompt"`
	// MemoryTool names the arm's memory fetch.
	MemoryTool string `json:"memory_tool"`
	// MemoryResponse is what the fetch returned verbatim, or empty for arm
	// 1.
	MemoryResponse string `json:"memory_response"`
	// Reply is the recommendation, verbatim.
	Reply string `json:"reply"`
	// PromptWords and ReplyWords count the prompt and the reply by the
	// harness counter.
	PromptWords int `json:"prompt_words"`
	ReplyWords  int `json:"reply_words"`
	// TraceCostUSD is the platform cost of the turn.
	TraceCostUSD float64 `json:"trace_cost_usd"`
	// Contaminated is true when the reply names another shopper's facts.
	Contaminated bool `json:"contaminated"`
	// Capped is true when the turn exceeded the frozen caps.
	Capped bool `json:"capped"`
	// Extraction is the reply-to-grader reduction.
	Extraction pilot.Extraction `json:"extraction"`
	// Score is set by the grader; nil before grading.
	Score *Score `json:"score,omitempty"`
}

// NewRunFile assembles a run record from one turn and reduces the reply
// with the frozen extraction rule. It never consults the hidden answer, and
// it marks the turn capped exactly as the harness does: a prompt over the
// ceiling, or a prompt plus reply over the token or spend ceiling.
func NewRunFile(runID string, shopper seed.Round2Shopper, arm Arm, repeat int, turn TurnKind, agent, prompt string, fetch MemoryFetch, reply string, cost float64) (RunFile, error) {
	size, rule := pilot.ExtractSize(reply)
	contaminated, _ := pilot.CheckContamination(reply)
	caps := harness.DefaultCaps()
	promptWords := harness.CountTokens(prompt)
	replyWords := harness.CountTokens(reply)
	total := promptWords + replyWords
	record := RunFile{
		RunID:          runID,
		ShopperID:      shopper.ShopperID,
		CaseID:         shopper.CaseID,
		Arm:            int(arm),
		ArmName:        arm.String(),
		Repeat:         repeat,
		Turn:           turn,
		Agent:          agent,
		Model:          Model,
		Prompt:         prompt,
		MemoryTool:     fetch.Tool,
		MemoryResponse: string(fetch.Response),
		Reply:          reply,
		PromptWords:    promptWords,
		ReplyWords:     replyWords,
		TraceCostUSD:   cost,
		Contaminated:   contaminated,
		Capped:         !caps.Allows(promptWords) || total > caps.MaxTokens || caps.SpendFor(total) > caps.MaxSpendUSD,
		Extraction: pilot.Extraction{
			Size:     size,
			SizeRule: rule,
			IDRefs:   pilot.ExtractIDRefs(shopper.History, reply),
		},
	}
	return record, nil
}

// Score is the graded turn, as counts. RightSize and CorrectionHonoured
// answer the same question for different turns: whether the recommendation
// names the size that turn's memory holds. Rung is the precedence step the
// served memory put first, which is how the comparison reports which step
// decided the answer.
type Score struct {
	// ShopperID, CaseID, Arm and Turn name the cell and turn graded.
	ShopperID string   `json:"shopper_id"`
	CaseID    string   `json:"case_id"`
	Arm       int      `json:"arm"`
	Turn      TurnKind `json:"turn"`
	// ExpectedSize is the size that turn is graded against.
	ExpectedSize string `json:"expected_size"`
	// RightSize is true when the extracted size equals ExpectedSize.
	RightSize bool `json:"right_size"`
	// WrongCause counts cited non-fit evidence behind a size that moves
	// off the shopper's baseline kept size.
	WrongCause int `json:"wrong_cause"`
	// CorrectionHonoured is nil for the first turn; for every correction
	// turn it says whether the recommendation followed the correction.
	CorrectionHonoured *bool `json:"correction_honoured"`
	// Supported counts cites that resolve to fit evidence; Cited counts
	// every cite offered.
	Supported int `json:"supported_cites"`
	Cited     int `json:"cited"`
	// Cites carries the per-cite verdicts.
	Cites []grader.CiteVerdict `json:"cites,omitempty"`
	// Rung is the precedence step the memory served first, or empty when
	// the arm served no precedence view.
	Rung string `json:"rung"`
	// Tokens counts the prompt and reply words; TokenBasis names the
	// counter.
	Tokens     int    `json:"tokens"`
	TokenBasis string `json:"token_basis"`
	// OverCap is true when the turn exceeded the frozen caps.
	OverCap bool `json:"over_cap"`
	// Opinions carries model judgements. The offline grader records none.
	Opinions []string `json:"opinions,omitempty"`
}

// ExpectedSize returns the size one turn is graded against: the hidden
// answer's first recommendation, or the size right after the correction
// for every correction turn.
func ExpectedSize(answer seed.Round2Answer, turn TurnKind) string {
	if turn == TurnFirst {
		return answer.RightSize
	}
	return answer.RightSizeAfterCorrection
}

// GradeRunFile scores one committed turn with the frozen rules. The
// extraction it grades on is the record's effective extraction: the
// deterministic size and cites, with the recorded human override where one
// was made.
func GradeRunFile(record RunFile, shopper seed.Round2Shopper, answer seed.Round2Answer) (Score, error) {
	if record.ShopperID != shopper.ShopperID || answer.ShopperID != shopper.ShopperID {
		return Score{}, fmt.Errorf("%w: run %q, shopper %q, answer %q", ErrNoShopper, record.ShopperID, shopper.ShopperID, answer.ShopperID)
	}
	expected := ExpectedSize(answer, record.Turn)
	score, err := grader.ScoreRun(grader.Run{
		ShopperID: shopper.ShopperID,
		History:   shopper.History,
		Answer:    seed.Answer{ShopperID: shopper.ShopperID, RightSize: expected},
		First: grader.Turn{
			Prompt: record.Prompt,
			Reply:  record.Reply,
			Size:   pilot.EffectiveSize(record.Extraction.Size, record.Extraction.HumanSize),
			Cites:  record.Extraction.EffectiveCites(),
		},
	})
	if err != nil {
		return Score{}, err
	}
	out := Score{
		ShopperID:    record.ShopperID,
		CaseID:       record.CaseID,
		Arm:          record.Arm,
		Turn:         record.Turn,
		ExpectedSize: expected,
		RightSize:    score.RightSize,
		WrongCause:   score.WrongCause,
		Supported:    score.Supported,
		Cited:        score.Cited,
		Cites:        score.Cites,
		Rung:         servedRung(record.MemoryResponse),
		Tokens:       harness.CountTokens(record.Prompt) + harness.CountTokens(record.Reply),
		TokenBasis:   TokenBasis,
		OverCap:      record.Capped,
	}
	if record.Turn != TurnFirst {
		honoured := score.RightSize && strings.TrimSpace(expected) != ""
		out.CorrectionHonoured = &honoured
	}
	return out, nil
}

// servedRung reads the precedence step one memory response put first: the
// un-overridden step of a belief or of the pool. It returns empty when the
// response is not a recall view (arms 1 and 2), so the report can say which
// arms had a precedence step at all.
func servedRung(memory string) string {
	if strings.TrimSpace(memory) == "" {
		return ""
	}
	var response struct {
		Beliefs []struct {
			Precedence *struct {
				Step       string `json:"step"`
				Overridden bool   `json:"overridden"`
			} `json:"precedence"`
		} `json:"beliefs"`
		Pool *struct {
			Precedence *struct {
				Step       string `json:"step"`
				Overridden bool   `json:"overridden"`
			} `json:"precedence"`
		} `json:"pool"`
	}
	if err := json.Unmarshal([]byte(memory), &response); err != nil {
		return ""
	}
	for _, belief := range response.Beliefs {
		if belief.Precedence != nil && !belief.Precedence.Overridden {
			return belief.Precedence.Step
		}
	}
	if response.Pool != nil && response.Pool.Precedence != nil && !response.Pool.Precedence.Overridden {
		return response.Pool.Precedence.Step
	}
	return ""
}
