package pilot

import (
	"fmt"
	"strings"

	"github.com/nrynss/memento-emptor/bench/grader"
	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/seed"
)

// Run Labels and placeholder conventions for committed records.
//
// Platform ids (deployment, conversation, trace, user, session) are
// scrubbed from committed files per the research scrub rule: they identify
// platform objects under the author's account, not pilot data. The driver
// replaces each with a fixed placeholder and prints the real mapping to
// stdout, which stays in the operator's transcript and the task report,
// never in the repo. Timestamps, costs and latencies are pilot data and
// are kept.
const (
	// PlaceholderDeployment stands in for the deployment id.
	PlaceholderDeployment = "<deployment-id>"
	// PlaceholderConversation stands in for the conversation id.
	PlaceholderConversation = "<conv-id>"
	// PlaceholderTrace stands in for the trace id.
	PlaceholderTrace = "<trace-id>"
	// PlaceholderUser stands in for the platform user id.
	PlaceholderUser = "<user-id>"
	// PlaceholderSession stands in for the trace session id.
	PlaceholderSession = "<session-id>"
)

// Model is the comparison model: gateway Sonnet, the T3.1 pick, the same
// string in all three arms.
const Model = "claude-sonnet-4-6"

// ToolSpan is one agent tool span the trace shows, without platform ids.
type ToolSpan struct {
	// Name is the observation name, such as "tool recall_for_purchase".
	Name string `json:"name"`
	// Type is the observation type, such as "tool" or "generation".
	Type string `json:"type"`
	// LatencyMs is the span latency in milliseconds.
	LatencyMs int64 `json:"latency_ms"`
}

// HumanRef is one prose-to-evidence mapping from the human extraction
// step: the ref as the grader resolves it, the reply quote it came from,
// and why it maps to exactly one row.
type HumanRef struct {
	// Ref is the note-substring ref the grader resolves.
	Ref string `json:"ref"`
	// Quote is the reply passage the ref came from.
	Quote string `json:"quote"`
	// Reason states why the passage maps to exactly one history row.
	Reason string `json:"reason"`
}

// Extraction is the reply-to-grader reduction for one run. Deterministic
// fields come from ExtractSize and ExtractIDRefs; human fields are set by
// the pilot author when reviewing the reply and win over the deterministic
// values where set, with the reason recorded beside them.
type Extraction struct {
	// Size is the deterministic recommended size, or empty.
	Size string `json:"size"`
	// SizeRule names the deterministic rule that fired.
	SizeRule SizeRule `json:"size_rule"`
	// SizeQuote is the reply passage the deterministic size came from.
	SizeQuote string `json:"size_quote"`
	// HumanSize overrides Size where the deterministic pick is judged
	// not to be the reply's recommendation.
	HumanSize string `json:"human_size,omitempty"`
	// HumanSizeReason records why the override was made.
	HumanSizeReason string `json:"human_size_reason,omitempty"`
	// IDRefs are the transaction ids appearing verbatim in the reply.
	IDRefs []string `json:"id_refs"`
	// HumanRefs are the prose-to-evidence mappings.
	HumanRefs []HumanRef `json:"human_refs,omitempty"`
	// Notes carries anything the rules above do not cover.
	Notes string `json:"notes,omitempty"`
}

// EffectiveCites returns the cites the run grades on: verbatim ids plus
// every human-mapped ref, in that order.
func (e Extraction) EffectiveCites() []string {
	out := append([]string{}, e.IDRefs...)
	for _, h := range e.HumanRefs {
		out = append(out, h.Ref)
	}
	return out
}

// RunFile is the complete committed record of one pilot run: the prompt,
// the arm's memory fetch, the agent tool spans the trace shows, the reply,
// word-count tokens, the trace cost, the extraction, and (after grading)
// the score. The hidden answer never enters this file; the grade command
// writes scores beside it in scores.json.
type RunFile struct {
	// RunID is the local run label, such as pilot-a1-broken.
	RunID string `json:"run_id"`
	// ShopperID names the comparison shopper.
	ShopperID string `json:"shopper_id"`
	// Arm is the harness arm number, 1 or 2 in the pilot.
	Arm int `json:"arm"`
	// ArmName is the harness arm name.
	ArmName string `json:"arm_name"`
	// Model is the comparison model string.
	Model string `json:"model"`
	// Deployment is the reference agent deployment name.
	Deployment string `json:"deployment"`
	// Prompt is the full prompt the run sent, verbatim.
	Prompt string `json:"prompt"`
	// ArmMemoryTool names the arm's memory fetch: none for arm 1,
	// read_history for arm 2.
	ArmMemoryTool string `json:"arm_memory_tool"`
	// ArmMemoryResponse is what the arm's memory fetch returned: the raw
	// history the prompt already carries, summarized here by length.
	ArmMemoryResponse string `json:"arm_memory_response"`
	// AgentToolSpans are the tool spans the live trace shows, in order.
	AgentToolSpans []ToolSpan `json:"agent_tool_spans"`
	// RecallObserved is true when the trace shows a recall_for_purchase span.
	RecallObserved bool `json:"recall_observed"`
	// Reply is the agent's recommendation text, verbatim.
	Reply string `json:"reply"`
	// Contaminated is true when the reply names demo-shopper facts.
	Contaminated bool `json:"contaminated"`
	// ContaminationHits are the demo tokens found in the reply.
	ContaminationHits []string `json:"contamination_hits,omitempty"`
	// TraceCostUSD is the platform cost of the turn.
	TraceCostUSD float64 `json:"trace_cost_usd"`
	// PromptWords and ReplyWords count the prompt and reply by the
	// harness counter, the same basis the grader records.
	PromptWords int `json:"prompt_words"`
	// ReplyWords counts the reply by the harness counter.
	ReplyWords int `json:"reply_words"`
	// Capped is true when the run exceeded its caps and stopped.
	Capped bool `json:"capped"`
	// SentAt and TraceAt order the run against the billing reads.
	SentAt  string `json:"sent_at"`
	TraceAt string `json:"trace_at"`
	// Extraction is the reply-to-grader reduction.
	Extraction Extraction `json:"extraction"`
	// Score is set by the grade command; nil before grading.
	Score *grader.Score `json:"score,omitempty"`
}

// NewRunFile assembles the committed record from a live turn. toolCalls
// are the harness arm memory fetches; spans are the trace observations;
// cost is the trace cost. Ids are scrubbed to the placeholders above.
func NewRunFile(runID string, history seed.History, arm harness.Arm, toolCalls []harness.ToolCall, spans []ToolSpan, prompt, reply string, cost float64, sentAt, traceAt string, scrub func(string) string) RunFile {
	size, rule := ExtractSize(reply)
	quote := sizeQuote(reply, size)
	contaminated, hits := CheckContamination(reply)
	recall := false
	for _, span := range spans {
		if span.Name == "tool recall_for_purchase" {
			recall = true
		}
	}
	memoryTool, memoryRaw := "", ""
	for _, call := range toolCalls {
		memoryTool = call.Name
		memoryRaw = call.Response
	}
	return RunFile{
		RunID:             runID,
		ShopperID:         history.ShopperID,
		Arm:               int(arm),
		ArmName:           arm.String(),
		Model:             Model,
		Deployment:        "memento-agent",
		Prompt:            scrub(prompt),
		ArmMemoryTool:     memoryTool,
		ArmMemoryResponse: summarizeMemory(memoryRaw),
		AgentToolSpans:    spans,
		RecallObserved:    recall,
		Reply:             scrub(reply),
		Contaminated:      contaminated,
		ContaminationHits: hits,
		TraceCostUSD:      cost,
		PromptWords:       harness.CountTokens(prompt),
		ReplyWords:        harness.CountTokens(reply),
		SentAt:            sentAt,
		TraceAt:           traceAt,
		Extraction: Extraction{
			Size:      size,
			SizeRule:  rule,
			SizeQuote: quote,
			IDRefs:    ExtractIDRefs(history, reply),
		},
	}
}

// sizeQuote returns the reply line carrying the extracted size, so the
// freeze can see what the rule fired on without re-reading the reply.
func sizeQuote(reply, size string) string {
	if size == "" {
		return ""
	}
	for _, line := range strings.Split(reply, "\n") {
		for _, field := range strings.Fields(line) {
			if cleanToken(field) == size {
				return strings.TrimSpace(line)
			}
		}
	}
	return ""
}

// summarizeMemory keeps the arm memory fetch honest without duplicating
// the prompt: the response already sits inside the prompt verbatim, so the
// record carries its length and first line only.
func summarizeMemory(response string) string {
	first := ""
	if lines := strings.Split(response, "\n"); len(lines) > 0 {
		first = strings.TrimSpace(lines[0])
	}
	return fmt.Sprintf("response %d chars, first line: %s", len(response), first)
}
