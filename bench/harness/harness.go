package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/seed"
)

// Arm is one comparison condition. Arms differ only in the memory the
// agent is given, never in the shopper, the candidate, the base prompt
// or the model.
type Arm int

// The three comparison arms from the product spec.
const (
	// ArmNoMemory gives the agent nothing about past purchases.
	ArmNoMemory Arm = 1
	// ArmRawHistory hands the shopper's PayPal transactions and past
	// messages to the agent directly.
	ArmRawHistory Arm = 2
	// ArmMemento gives the agent the layer's beliefs, evidence and
	// corrections.
	ArmMemento Arm = 3
)

// String names the arm for records and reports.
func (a Arm) String() string {
	switch a {
	case ArmNoMemory:
		return "no-memory"
	case ArmRawHistory:
		return "raw-history"
	case ArmMemento:
		return "memento"
	default:
		return "unknown"
	}
}

// ErrUnknownArm is returned when a run names an arm outside the three
// comparison conditions.
var ErrUnknownArm = errors.New("harness: unknown arm")

// ErrNoHistories is returned when a histories directory yields no
// shopper histories to run.
var ErrNoHistories = errors.New("harness: no histories")

// ErrNoScriptedReply is returned when the scripted fixture model has no
// recorded reply for the shopper in the prompt. The harness fails rather
// than inventing a reply.
var ErrNoScriptedReply = errors.New("harness: no scripted reply")

// Caps bounds one run. Both ceilings are enforced in code before any
// model call: a run whose prompt alone exceeds either cap stops with no
// reply and no spend.
type Caps struct {
	// MaxTokens is the per-run token ceiling for prompt plus reply.
	MaxTokens int
	// MaxSpendUSD is the per-run spend ceiling in US dollars.
	MaxSpendUSD float64
	// PricePerTokenUSD is the blended price one token costs. The dry run
	// spends nothing; the price turns token counts into comparable spend
	// figures and lets the spend cap bind if prices change.
	PricePerTokenUSD float64
}

// DefaultCaps returns the pilot-scale caps. A single recommendation turn
// over the longest seeded history stays under two thousand tokens by the
// word count below, so 4000 tokens is about twice the worst case: normal
// runs pass and a looping run stops early. At the blended $5 per million
// tokens, the worst case costs about a cent, so the five-cent spend cap
// is a backstop that binds only if the price moves; the token cap is the
// ceiling that normally stops a run.
func DefaultCaps() Caps {
	return Caps{
		MaxTokens:        4000,
		MaxSpendUSD:      0.05,
		PricePerTokenUSD: 0.000005,
	}
}

// SpendFor converts a token count to dollars at the cap's price.
func (c Caps) SpendFor(tokens int) float64 {
	return float64(tokens) * c.PricePerTokenUSD
}

// Allows reports whether a prompt of promptTokens tokens may still issue
// a model call without already exceeding either ceiling.
func (c Caps) Allows(promptTokens int) bool {
	if promptTokens > c.MaxTokens {
		return false
	}
	return c.SpendFor(promptTokens) <= c.MaxSpendUSD
}

// CountTokens counts the tokens in text as whitespace-separated words.
// It is deterministic, needs no model and no network, and is the same
// counter for prompts and replies in every arm, so token figures compare
// fairly across arms.
func CountTokens(text string) int {
	return len(strings.Fields(text))
}

// ModelFunc stands in for the model. Live stages pass a real caller;
// the fixture dry run passes ScriptedByShopper. The prompt always opens
// with a "Shopper: <id>" line so the scripted model can file its reply.
type ModelFunc func(ctx context.Context, prompt string) (string, error)

// ScriptedByShopper returns a ModelFunc that answers from recorded
// replies keyed by shopper id. A shopper with no recording is an error,
// never an invented reply, and the run needs no key and no network.
func ScriptedByShopper(replies map[string]string) ModelFunc {
	return func(ctx context.Context, prompt string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("harness: scripted model: %w", err)
		}
		id := shopperFromPrompt(prompt)
		reply, ok := replies[id]
		if !ok {
			return "", fmt.Errorf("%w: %q", ErrNoScriptedReply, id)
		}
		return reply, nil
	}
}

// shopperFromPrompt reads the shopper id back off the prompt's opening
// line. Prompts are built by PromptFor, which always writes it.
func shopperFromPrompt(prompt string) string {
	line, _, _ := strings.Cut(prompt, "\n")
	id, found := strings.CutPrefix(line, "Shopper: ")
	if !found {
		return ""
	}
	return strings.TrimSpace(id)
}

// ToolCall records one memory-fetch the run made: what was asked and
// what came back. Arm 1 makes none; arm 2 reads the raw history; arm 3
// recalls from the layer fixture.
type ToolCall struct {
	// Name is the stub tool called: read_history or recall_for_purchase.
	Name string
	// Request states what was asked for: the shopper and brand ids.
	Request string
	// Response is what the stub returned: the memory the arm carries.
	Response string
}

// RunRecord is the complete record of one shopper through one arm: the
// prompt, every tool call, the reply, and the token and spend counts.
type RunRecord struct {
	// ShopperID names the shopper the run drove.
	ShopperID string
	// Arm names the condition the run drove.
	Arm Arm
	// Prompt is the full prompt the model saw: base plus memory.
	Prompt string
	// ToolCalls is every memory fetch the run made, in order.
	ToolCalls []ToolCall
	// Reply is the model's reply, or empty when the cap stopped the run
	// before any model call.
	Reply string
	// PromptTokens counts the prompt by CountTokens.
	PromptTokens int
	// ReplyTokens counts the reply by CountTokens.
	ReplyTokens int
	// TotalTokens is prompt plus reply tokens.
	TotalTokens int
	// SpendUSD converts the total at the run's token price.
	SpendUSD float64
	// Capped is true when the run exceeded its caps and stopped. A run
	// capped before the model call carries no reply.
	Capped bool
	// Complete is true when the run holds a reply within both caps.
	Complete bool
}

// anchor is the arbitrary origin event-time spans count from when the
// harness materializes seeded rows as evidence. Only the spans between
// rows matter, never the origin.
var anchor = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

// RunOne drives one shopper through one arm and returns the complete
// record. It enforces caps in code: when the prompt alone exceeds the
// token or spend ceiling the run stops before any model call, so it never
// spends through. A reply that pushes the total over either ceiling is
// recorded and marked capped.
func RunOne(ctx context.Context, history seed.History, arm Arm, model ModelFunc, caps Caps) (RunRecord, error) {
	if err := ctx.Err(); err != nil {
		return RunRecord{}, fmt.Errorf("harness: run %s: %w", history.ShopperID, err)
	}
	switch arm {
	case ArmNoMemory, ArmRawHistory, ArmMemento:
	default:
		return RunRecord{}, fmt.Errorf("%w: %d", ErrUnknownArm, int(arm))
	}
	prompt, calls := PromptFor(history, arm)
	record := RunRecord{
		ShopperID: history.ShopperID,
		Arm:       arm,
		Prompt:    prompt,
		ToolCalls: calls,
	}
	record.PromptTokens = CountTokens(prompt)
	if !caps.Allows(record.PromptTokens) {
		record.SpendUSD = caps.SpendFor(record.PromptTokens)
		record.TotalTokens = record.PromptTokens
		record.Capped = true
		return record, nil
	}
	reply, err := model(ctx, prompt)
	if err != nil {
		return RunRecord{}, fmt.Errorf("harness: run %s arm %s: %w", history.ShopperID, arm, err)
	}
	record.Reply = reply
	record.ReplyTokens = CountTokens(reply)
	record.TotalTokens = record.PromptTokens + record.ReplyTokens
	record.SpendUSD = caps.SpendFor(record.TotalTokens)
	if record.TotalTokens > caps.MaxTokens || record.SpendUSD > caps.MaxSpendUSD {
		record.Capped = true
		return record, nil
	}
	record.Complete = reply != ""
	return record, nil
}

// RunAll drives every history through one arm with the same model and
// caps, in shopper-id order, and returns one record per shopper. It stops
// at the first model error; capped runs are records, not errors, so they
// never stop the set.
func RunAll(ctx context.Context, histories []seed.History, arm Arm, model ModelFunc, caps Caps) ([]RunRecord, error) {
	if len(histories) == 0 {
		return nil, fmt.Errorf("harness: run arm %s: %w", arm, ErrNoHistories)
	}
	ordered := append([]seed.History(nil), histories...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ShopperID < ordered[j].ShopperID })
	records := make([]RunRecord, 0, len(ordered))
	for _, h := range ordered {
		record, err := RunOne(ctx, h, arm, model, caps)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// LoadHistories reads the arm-facing shopper histories from dir, one
// JSON file per shopper, sorted by file name. It reads only the histories
// directory it is given: there is no path joining to anywhere else.
func LoadHistories(dir string) ([]seed.History, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("harness: list histories in %s: %w", dir, err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("harness: load %s: %w", dir, ErrNoHistories)
	}
	histories := make([]seed.History, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("harness: read %s: %w", path, err)
		}
		var history seed.History
		if err := json.Unmarshal(raw, &history); err != nil {
			return nil, fmt.Errorf("harness: decode %s: %w", path, err)
		}
		histories = append(histories, history)
	}
	return histories, nil
}

// candidateLine picks the line under consideration for a shopper: the
// brand and line of the first row carrying an item. The candidate is a
// new SKU on that line with the size unchosen, so every arm recommends
// into the same open question.
func candidateLine(history seed.History) (brand, line string) {
	for _, row := range history.Rows {
		if row.Item.BrandName != "" && row.Item.LineName != "" {
			return row.Item.BrandName, row.Item.LineName
		}
	}
	return "unknown brand", "unknown line"
}

// BaseSection builds the part of the prompt every arm shares: who the
// shopper is, the candidate, and the task. It carries no memory, so it is
// identical for all three arms of the same shopper.
func BaseSection(history seed.History) string {
	brand, line := candidateLine(history)
	var out strings.Builder
	out.WriteString("Shopper: " + history.ShopperID + "\n")
	out.WriteString("Candidate: a new " + line + " from " + brand)
	out.WriteString(" (new SKU, size unchosen). The shopper wants another one.\n")
	out.WriteString("Task: recommend one size (a single token such as M or L)")
	out.WriteString(" and one sentence of justification citing only what you were given.")
	out.WriteString(" Do not invent purchases, reasons, or corrections.")
	return out.String()
}

// MemorySection builds the part of the prompt that differs by arm, and
// records the stub tool call that fetched it. The three sections carry
// the three memories and nothing else, so prompts differ only here.
func MemorySection(arm Arm, history seed.History) (string, []ToolCall) {
	switch arm {
	case ArmRawHistory:
		raw := RawHistoryContext(history)
		section := "Memory: raw history. The shopper's transactions and past messages, uninterpreted:\n" + raw
		calls := []ToolCall{{
			Name:     "read_history",
			Request:  "shopper_id=" + history.ShopperID,
			Response: raw,
		}}
		return section, calls
	case ArmMemento:
		layer := LayerContext(history)
		brand, _ := candidateLine(history)
		calls := []ToolCall{{
			Name:     "recall_for_purchase",
			Request:  "shopper_id=" + history.ShopperID + " brand=" + brand,
			Response: layer,
		}}
		return "Memory: Memento layer. Beliefs, evidence and corrections for this shopper:\n" + layer, calls
	default:
		return "Memory: none. No past purchases were given for this recommendation. Recommend from the candidate alone.", nil
	}
}

// PromptFor builds the full prompt for one shopper through one arm: the
// shared base plus the arm's memory. Arm 1 carries no memory, arm 2
// carries the raw history, arm 3 carries the layer summary.
func PromptFor(history seed.History, arm Arm) (string, []ToolCall) {
	base := BaseSection(history)
	memory, calls := MemorySection(arm, history)
	return base + "\n" + memory, calls
}

// outcomeText renders a row's outcome, or notes a bare statement with no
// outcome to render.
func outcomeText(row seed.Row) string {
	if row.Outcome == nil {
		return "statement (no outcome)"
	}
	return string(row.Outcome.Kind) + " " + row.Outcome.Status
}

// RawHistoryContext assembles arm 2's input from the seed history: the
// shopper's transactions and past messages, handed to the agent directly
// with no interpretation. Every row carries its seeded mark.
func RawHistoryContext(history seed.History) string {
	var out strings.Builder
	fmt.Fprintf(&out, "Raw history for %s: %d rows over %.0f days.\n",
		history.ShopperID, len(history.Rows), history.SessionDays)
	for _, row := range history.Rows {
		fmt.Fprintf(&out, "day %.0f | %s | signal %s | source %s | ",
			row.Day, outcomeText(row), string(row.Signal), string(row.Source))
		if row.Item.BrandName != "" {
			fmt.Fprintf(&out, "%s %s SKU %s size %s (%s) | ",
				row.Item.BrandName, row.Item.LineName, row.Item.SKU, row.Item.Size, row.Item.Category)
		} else {
			out.WriteString("no item | ")
		}
		if row.Reason != nil {
			fmt.Fprintf(&out, "reason %s %q conf %.2f | ",
				string(row.Reason.Category), row.Reason.Text, row.Reason.Confidence)
		} else {
			out.WriteString("reason none | ")
		}
		writeOp, err := domain.WriteFor(row.Signal)
		if err != nil {
			out.WriteString("write unknown | ")
		} else {
			fmt.Fprintf(&out, "write %s | ", string(writeOp))
		}
		fmt.Fprintf(&out, "%s | %s\n", row.SeedMark, row.Note)
	}
	return out.String()
}

// LayerContext assembles arm 3's input from the same seed history: the
// beliefs, evidence and corrections the layer would serve, derived only
// from the rows the layer ingested. It states what the evidence is and
// what each row counts toward; it never states a right size, which lives
// only in the answer key no arm may read.
func LayerContext(history seed.History) string {
	type brandRows struct {
		name string
		rows []seed.Row
	}
	byBrand := map[string]*brandRows{}
	order := []string{}
	for _, row := range history.Rows {
		key := row.Item.BrandID
		if key == "" {
			key = "no-brand"
		}
		br, ok := byBrand[key]
		if !ok {
			br = &brandRows{name: row.Item.BrandName}
			byBrand[key] = br
			order = append(order, key)
		}
		br.rows = append(br.rows, row)
	}
	sort.Strings(order)
	var out strings.Builder
	fmt.Fprintf(&out, "Layer memory for %s: beliefs with evidence, derived from %d ingested rows.\n",
		history.ShopperID, len(history.Rows))
	for _, key := range order {
		br := byBrand[key]
		fit, seller, pending, statements := 0, 0, 0, 0
		for _, row := range br.rows {
			ev := row.SeededEvidence(anchor)
			switch ev.CountsToward() {
			case domain.BeliefKindFit:
				fit++
			case domain.BeliefKindSeller:
				seller++
			case domain.BeliefKindNone:
				if row.Outcome != nil &&
					(row.Outcome.Kind == domain.OutcomeRefund || row.Outcome.Kind == domain.OutcomeDispute) {
					pending++
				} else if row.Signal == domain.SignalGeneralStatement {
					statements++
				}
			}
		}
		name := br.name
		if name == "" {
			name = "no brand"
		}
		fmt.Fprintf(&out, "Brand %s (%s): %d fit evidence rows, %d seller rows, %d pending with no reason, %d general statements (never promote).\n",
			name, key, fit, seller, pending, statements)
		for _, row := range br.rows {
			ev := row.SeededEvidence(anchor)
			writeOp, err := domain.WriteFor(row.Signal)
			op := "unknown"
			if err == nil {
				op = string(writeOp)
			}
			fmt.Fprintf(&out, "- day %.0f | %s | counts toward %q | write %s | %s | %s\n",
				row.Day, outcomeText(row), string(ev.CountsToward()), op, row.SeedMark, row.Note)
		}
	}
	out.WriteString("Corrections: none recorded. No correction overrides any belief above.\n")
	return out.String()
}
