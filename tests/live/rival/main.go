// Live driver for T2.8b: the rival-birth rule against a real Lambo.
//
// The scripted client drives server.Service (the frozen T2.5 interface)
// over recording payments (DemoStoryInputs plus five extra refund events
// in the same shape, whose reads replay the P0 recordings), the recorded
// classifier (testdata/jev: no live classification service runs here),
// and a live Lambo 0.10.1 graph client over a fresh session with native
// embeddings. The evidence store is the in-memory lexical store; the
// live Elastic proof is the T2.3b leg, which this driver does not repeat.
//
// Two shoppers run the pinned rule from internal/domain/rival.go: N=3
// consecutive opposite-direction classified returns birth a same-kind
// opposite-direction rival, unpromoted, with the existing belief
// untouched; the count resets on any same-direction return.
//
//   - shopper_rival_birth: one too-small return, then three too-large
//     returns. The first two land unattached; the third births the rival.
//   - shopper_rival_gap: one too-small, two too-large, one too-small
//     (the interruption), one too-large. No rival is born.
//
// This is a main program, never a test: no *_test.go lives under
// tests/live/, so CI builds this package but never runs it. Run it once
// by hand with the keys loaded:
//
//	source tools/env.sh
//	LAMBO_BIN=$HOME/work/lambo/target/release/lambo \
//	  bash tests/live/rival/run.sh
//
// Only ELASTIC_URL and LAMBO_ES_API_KEY are read, for the native
// embedder. Raw inspect excerpts land in out/ (git-ignored): they carry
// only demo-fiction contents, never a key or the deployment URL. Stdout
// names fixture ids only and no secret.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/api/server"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// waitMS is the receipt wait per Lambo write, matching the T2.4b
// driver's wait.
const waitMS = 34000

// smallContent and rivalContent are the exact graph contents the frozen
// service composes for the two fit beliefs (see
// internal/api/server/content.go: beliefContent over fitBeliefValue).
// The driver repeats them verbatim so live inspect reads the same
// concepts the service wrote; a drift between the two fails loudly.
const smallContent = "memento fit brand brand-x-001 runs small, take size L"
const rivalContent = "memento fit brand brand-x-001 runs large, take size S"

// smallAnswer and largeAnswer are the two recorded answers the drive
// replays: one per direction. Both classify at 1.00, so every answer
// below lands classified.
const smallAnswer = "too tight in the shoulders"
const largeAnswer = "it swims on me, far too big"

// birthShopper proves birth at exactly three consecutive opposites.
const birthShopper = "shopper_rival_birth"

// gapShopper proves the interrupted streak births nothing.
const gapShopper = "shopper_rival_gap"

// rivalContract is the store's embedding contract. It carries no model
// name, dimensions, endpoint or key — only its kind, so the lexical
// store stands in for the live Elastic proof beside this driver.
type rivalContract struct{ kind string }

// EmbeddingKind names the contract family.
func (c rivalContract) EmbeddingKind() string { return c.kind }

// Validate reports whether the contract is usable.
func (c rivalContract) Validate() error {
	if c.kind == "" {
		return errors.New("t28b-live: contract names no kind")
	}
	return nil
}

// rpcClient speaks MCP JSON-RPC to one lambo serve child over stdio,
// the way the T2.4b Python driver does.
type rpcClient struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out io.Reader
	seq int
}

// rpcRequest is one JSON-RPC request envelope.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// rpcResponse is the envelope subset the driver reads.
type rpcResponse struct {
	Result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// startLambo starts the MCP server for session over its stdio pipes.
// The store is provisioned by run.sh before this driver runs.
func startLambo(bin, cfg, session, agent string) (*rpcClient, error) {
	cmd := exec.Command(bin, "--config", cfg, "serve", "--session", session, "--agent", agent)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("lambo stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("lambo stdout: %w", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("lambo start: %w", err)
	}
	client := &rpcClient{cmd: cmd, in: stdin, out: stdout}
	init := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05", "capabilities": map[string]any{},
			"clientInfo": map[string]any{"name": "t28b-driver", "version": "0.1.0"},
		},
	}
	raw, err := json.Marshal(init)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	if _, err := fmt.Fprintf(stdin, "%s\n", raw); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("lambo initialize: %w", err)
	}
	if _, err := client.readLine(); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("lambo initialize reply: %w", err)
	}
	client.seq = 2
	return client, nil
}

// readLine reads one newline-terminated JSON-RPC message.
func (c *rpcClient) readLine() ([]byte, error) {
	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := c.out.Read(buf)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		if buf[0] == '\n' {
			return line, nil
		}
		line = append(line, buf[0])
	}
}

// call invokes one MCP tool and returns its result envelope.
func (c *rpcClient) call(name string, args map[string]any) (rpcResponse, error) {
	var zero rpcResponse
	c.seq++
	req := rpcRequest{JSONRPC: "2.0", ID: c.seq, Method: "tools/call",
		Params: map[string]any{"name": name, "arguments": args}}
	raw, err := json.Marshal(req)
	if err != nil {
		return zero, err
	}
	if _, err := fmt.Fprintf(c.in, "%s\n", raw); err != nil {
		return zero, fmt.Errorf("lambo call %s: %w", name, err)
	}
	line, err := c.readLine()
	if err != nil {
		return zero, fmt.Errorf("lambo call %s reply: %w", name, err)
	}
	var resp rpcResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return zero, fmt.Errorf("lambo call %s decode: %w", name, err)
	}
	if resp.Error != nil {
		return zero, fmt.Errorf("lambo call %s: %s", name, resp.Error.Message)
	}
	return resp, nil
}

// text joins the result's text content blocks.
func textOf(resp rpcResponse) string {
	parts := []string{}
	for _, block := range resp.Result.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// stop closes the child's stdin and waits for it to exit.
func (c *rpcClient) stop() {
	_ = c.in.Close()
	_ = c.cmd.Wait()
}

// lamboGraph is a Graph and EvidenceGraph over one live Lambo session.
// Event times travel with every write so honestly spread history
// promotes; corrections cite against, never depends_on.
type lamboGraph struct {
	client *rpcClient
	agent  string
}

// Compile-time proof the live client satisfies the seams the service
// builds on.
var _ fakes.EvidenceGraph = (*lamboGraph)(nil)

// waitApplied blocks until the daemon applies one receipt.
func (g *lamboGraph) waitApplied(receipt, what string) error {
	resp, err := g.client.call("lambo_stats", map[string]any{
		"agent_id": g.agent, "receipt": receipt, "wait_ms": waitMS,
	})
	if err != nil {
		return fmt.Errorf("t28b-live: wait %s: %w", what, err)
	}
	text := textOf(resp)
	if strings.Contains(text, "embedder unavailable") || strings.Contains(text, "error sending request") {
		return fmt.Errorf("t28b-live: embedder down after %s: %s", what, lastLine(text))
	}
	if !strings.Contains(text, "applied") {
		return fmt.Errorf("t28b-live: receipt for %s never applied: %.200s", what, text)
	}
	return nil
}

// lastLine returns the final non-empty line of a report.
func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

// receiptOf reads the write receipt from a tools/call envelope.
func receiptOf(resp rpcResponse) (string, error) {
	var structured struct {
		Receipt string `json:"receipt"`
	}
	if err := json.Unmarshal(resp.Result.StructuredContent, &structured); err != nil {
		return "", fmt.Errorf("t28b-live: decode receipt: %w", err)
	}
	if structured.Receipt == "" {
		return "", fmt.Errorf("t28b-live: write issued no receipt")
	}
	return structured.Receipt, nil
}

// Derive records one belief or statement.
func (g *lamboGraph) Derive(ctx context.Context, content, kind string, eventTime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := map[string]any{
		"agent_id": g.agent,
		"concepts": []map[string]any{{"content": content, "concept_type": kind}},
	}
	if !eventTime.IsZero() {
		args["event_time"] = eventTime.UTC().Format(time.RFC3339Nano)
	}
	resp, err := g.client.call("lambo_derive", args)
	if err != nil {
		return err
	}
	receipt, err := receiptOf(resp)
	if err != nil {
		return err
	}
	return g.waitApplied(receipt, fmt.Sprintf("derive %.40q", content))
}

// RecordAction records one structural write.
func (g *lamboGraph) RecordAction(ctx context.Context, action string, dependsOn, against []string, eventTime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := map[string]any{"agent_id": g.agent, "action": action}
	if len(dependsOn) > 0 {
		args["depends_on"] = dependsOn
	}
	if len(against) > 0 {
		args["against"] = against
	}
	if !eventTime.IsZero() {
		args["event_time"] = eventTime.UTC().Format(time.RFC3339Nano)
	}
	resp, err := g.client.call("lambo_record_action", args)
	if err != nil {
		return err
	}
	receipt, err := receiptOf(resp)
	if err != nil {
		return err
	}
	return g.waitApplied(receipt, fmt.Sprintf("record_action %.40q", action))
}

// Recall lists the live session's hit labels for one query. The
// service ranks from its stored beliefs and never calls this; it
// exists so the client satisfies the seam honestly.
func (g *lamboGraph) Recall(ctx context.Context, query string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resp, err := g.client.call("lambo_recall", map[string]any{"agent_id": g.agent, "query": query})
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, line := range strings.Split(textOf(resp), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out, nil
}

// inspectShape is the structured subset of lambo_inspect the driver
// maps to a belief report.
type inspectShape struct {
	Admission struct {
		Status string `json:"status"`
		Checks []struct {
			Gate    string  `json:"gate"`
			Current float64 `json:"current"`
		} `json:"checks"`
	} `json:"admission"`
	Focus struct {
		BlastRadius int `json:"blast_radius"`
	} `json:"focus"`
	Warnings []string `json:"warnings"`
}

// Inspect reports the promotion state of one belief.
func (g *lamboGraph) Inspect(ctx context.Context, content string) (fakes.BeliefReport, error) {
	if err := ctx.Err(); err != nil {
		return fakes.BeliefReport{}, err
	}
	resp, err := g.client.call("lambo_inspect", map[string]any{
		"agent_id": g.agent, "focus": content, "depth": 2,
	})
	if err != nil {
		return fakes.BeliefReport{}, err
	}
	var shape inspectShape
	if err := json.Unmarshal(resp.Result.StructuredContent, &shape); err != nil {
		return fakes.BeliefReport{}, fmt.Errorf("t28b-live: decode inspect: %w", err)
	}
	report := fakes.BeliefReport{Content: content, Dependents: shape.Focus.BlastRadius}
	switch strings.ToLower(shape.Admission.Status) {
	case "venerable":
		report.State = domain.GraphVenerable
	case "candidate":
		report.State = domain.GraphCandidate
	default:
		report.State = domain.GraphNone
	}
	for _, check := range shape.Admission.Checks {
		switch check.Gate {
		case "distinct_interactions":
			report.DistinctInteractions = int(check.Current)
		case "coverage":
			report.Coverage = check.Current
		}
	}
	if len(shape.Warnings) > 0 {
		report.Contradicted = true
		report.Warning = "Contradicted by " + strings.Join(shape.Warnings, "; ")
	}
	return report, nil
}

// inspectRaw returns the inspect text untouched, for the birth proof:
// the transcript shows the live session holding the rival. An error
// means the focus names no concept in the session.
func (g *lamboGraph) inspectRaw(content string) (string, error) {
	resp, err := g.client.call("lambo_inspect", map[string]any{
		"agent_id": g.agent, "focus": content, "depth": 2,
	})
	if err != nil {
		return "", err
	}
	return textOf(resp), nil
}

// liveChild is the running Lambo server, stopped by fail so a failed
// run never leaves a writer holding the session.
var liveChild *rpcClient

// fail prints to stderr and exits nonzero.
func fail(err error) {
	if liveChild != nil {
		liveChild.stop()
		liveChild = nil
	}
	fmt.Fprintf(os.Stderr, "t28b-live: %v\n", err)
	os.Exit(1)
}

// extraRefund builds one more refund event with its order and refund
// records, shaped exactly like the frozen story's refunds: the order
// read carries the line item, the refund record carries no reason, and
// the custom id propagates across the link. These events live only in
// this driver's inputs.
func extraRefund(n, eventID string) (fakes.WebhookEvent, fakes.OrderRecord, fakes.RefundRecord) {
	orderID := "ORD-7XK-" + n
	link := domain.TransactionLink{
		OrderID:   orderID,
		CaptureID: "CAP-7XK-" + n,
		RefundID:  "RFD-7XK-" + n,
		CustomID:  "memento-shopper01-" + n,
		EventID:   eventID,
	}
	item := domain.Item{
		BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt",
		SKU: "rival-tee-" + n, Size: "M", Category: "shirts",
	}
	outcome := domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"}
	event := fakes.WebhookEvent{
		EventID: eventID, EventType: "PAYMENT.CAPTURE.REFUNDED", Verified: true,
		Outcome: outcome, Link: link,
		Provenance: "testdata/paypal/webhook-delivery-refunded.json",
	}
	order := fakes.OrderRecord{
		OrderID: orderID, Items: []domain.Item{item}, Link: link,
		Provenance: "testdata/paypal/order-get-approved-line-items.json",
	}
	refund := fakes.RefundRecord{
		RefundID: link.RefundID, Outcome: outcome, Link: link,
		Provenance: "testdata/paypal/refund-record.json",
	}
	return event, order, refund
}

// driverInputs extends the frozen story with five extra refunds, so the
// interrupted-streak shopper answers five questions on its own events.
func driverInputs() fakes.StoryInputs {
	inputs := fakes.DemoStoryInputs()
	for i, event := range []string{"WH-2EV-07", "WH-2EV-08", "WH-2EV-09", "WH-2EV-10", "WH-2EV-11"} {
		n := fmt.Sprintf("%02d", 7+i)
		e, o, r := extraRefund(n, event)
		inputs.Events = append(inputs.Events, e)
		inputs.Orders = append(inputs.Orders, o)
		inputs.Refunds = append(inputs.Refunds, r)
	}
	return inputs
}

// must ingests one webhook event and asserts its stored shape.
func must(ctx context.Context, svc *server.Service, shopper, eventID string) {
	res, err := svc.IngestWebhook(ctx, shopper, eventID)
	if err != nil {
		fail(fmt.Errorf("ingest %s: %w", eventID, err))
	}
	if res.Rows != 1 || !res.Pending || res.Duplicate {
		fail(fmt.Errorf("ingest %s: rows %d pending %t duplicate %t, want 1 true false",
			eventID, res.Rows, res.Pending, res.Duplicate))
	}
	fmt.Printf("ingest %s for %s: 1 row, pending\n", eventID, shopper)
}

// questionForEvent returns the open question filed by one webhook event.
func questionForEvent(ctx context.Context, svc *server.Service, shopper, eventID string) string {
	pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		fail(fmt.Errorf("pending questions: %w", err))
	}
	for _, question := range pending.Questions {
		if question.Link.EventID == eventID {
			return question.QuestionID
		}
	}
	fail(fmt.Errorf("no pending question for event %s", eventID))
	return ""
}

// answer records one classified answer for one event's question and
// reports what the frozen service decided.
func answer(ctx context.Context, svc *server.Service, shopper, eventID, text string) contract.RecordReasonResponse {
	res, err := svc.RecordReason(ctx, contract.RecordReasonRequest{
		ShopperID: shopper, QuestionID: questionForEvent(ctx, svc, shopper, eventID), AnswerText: text,
	})
	if err != nil {
		fail(fmt.Errorf("record reason for %s: %w", eventID, err))
	}
	belief := "<nil>"
	if res.BeliefID != nil {
		belief = *res.BeliefID
	}
	fmt.Printf("reason %s %q: category=%s conf=%.2f accepted=%t ask_again=%t belief=%s\n",
		eventID, text, res.Category, res.Confidence, res.Accepted, res.AskAgain, belief)
	return res
}

// wantUnattached answers one opposite-direction return and asserts it
// lands unattached: classified, closed, counting toward nothing.
func wantUnattached(ctx context.Context, svc *server.Service, shopper, eventID string) {
	res := answer(ctx, svc, shopper, eventID, largeAnswer)
	if res.Accepted || res.AskAgain || res.BeliefID != nil {
		fail(fmt.Errorf("opposite %s: accepted %v ask again %v belief %v, want false false nil",
			eventID, res.Accepted, res.AskAgain, res.BeliefID))
	}
	if res.Category != domain.CategoryFitTooLarge {
		fail(fmt.Errorf("opposite %s: category %q, want fit_too_large", eventID, string(res.Category)))
	}
	if res.CountsToward != nil {
		fail(fmt.Errorf("opposite %s counts toward %q, want nothing", eventID, string(*res.CountsToward)))
	}
}

// fitBeliefs returns one shopper's fit belief rows.
func fitBeliefs(ctx context.Context, svc *server.Service, shopper string) []contract.LedgerBeliefRow {
	ledger, err := svc.LedgerBeliefs(ctx, contract.LedgerBeliefsRequest{ShopperID: shopper, IncludeSeeded: true})
	if err != nil {
		fail(fmt.Errorf("ledger beliefs: %w", err))
	}
	out := []contract.LedgerBeliefRow{}
	for _, row := range ledger.Beliefs {
		if row.Belief.Kind == domain.BeliefKindFit {
			out = append(out, row)
		}
	}
	return out
}

// unattachedLarge counts the ledger evidence rows that carry a
// too-large reason with no belief link.
func unattachedLarge(ctx context.Context, svc *server.Service, shopper string) int {
	ledger, err := svc.LedgerEvidence(ctx, contract.LedgerEvidenceRequest{ShopperID: shopper})
	if err != nil {
		fail(fmt.Errorf("ledger evidence: %w", err))
	}
	out := 0
	for _, row := range ledger.Evidence {
		if row.Evidence.Reason != nil && row.Evidence.Reason.Category == domain.CategoryFitTooLarge {
			if len(row.BeliefIDs) != 0 {
				fail(fmt.Errorf("too-large row links %v, want unattached", row.BeliefIDs))
			}
			out++
		}
	}
	return out
}

// save writes one inspect excerpt under out/ for the handoff record.
func save(out, name, text string) {
	if err := os.WriteFile(filepath.Join(out, name), []byte(text+"\n"), 0o600); err != nil {
		fail(fmt.Errorf("save %s: %w", name, err))
	}
}

func main() {
	out := os.Getenv("OUT")
	if out == "" {
		out = "tests/live/rival/out"
	}
	bin := os.Getenv("LAMBO_BIN")
	if bin == "" {
		bin = filepath.Join(os.Getenv("HOME"), "work/lambo/target/release/lambo")
	}
	cfg := os.Getenv("LAMBO_CFG")
	if cfg == "" {
		cfg = filepath.Join(out, "lambo.toml")
	}
	session := os.Getenv("SESSION")
	if session == "" {
		session = "t28b-rival"
	}
	agent := "memento-t28b"
	if err := run(bin, cfg, session, agent, out); err != nil {
		fail(err)
	}
}

func run(bin, cfg, session, agent, out string) error {
	if os.Getenv("ELASTIC_URL") == "" || os.Getenv("LAMBO_ES_API_KEY") == "" {
		return fmt.Errorf("ELASTIC_URL and LAMBO_ES_API_KEY are empty: source tools/env.sh first")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("make out dir: %w", err)
	}
	if raw, err := exec.Command(bin, "--version").CombinedOutput(); err != nil {
		return fmt.Errorf("lambo binary: %w: %s", err, strings.TrimSpace(string(raw)))
	} else {
		fmt.Printf("lambo: %s\n", strings.TrimSpace(string(raw)))
	}

	// No pre-erase here: an erased session id is tombstoned and can
	// never be served again, so erasing before serving would brick
	// this run's session on a reused store. For a re-run, use a fresh
	// OUT directory or a new SESSION value (see README).
	client, err := startLambo(bin, cfg, session, agent)
	if err != nil {
		return err
	}
	liveChild = client
	graph := &lamboGraph{client: client, agent: agent}

	classifier, err := fakes.NewRecordingReasonClassifier(filepath.Join("testdata", "jev"))
	if err != nil {
		return fmt.Errorf("open classifier fake: %w", err)
	}
	store, err := evidence.NewStore(rivalContract{kind: "server-rival-live"})
	if err != nil {
		return fmt.Errorf("open evidence store: %w", err)
	}
	svc, err := server.NewService(server.Deps{
		Payments:   fakes.NewRecordingPaymentSource(driverInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      graph,
		SchemaDir:  filepath.Join("testdata", "api"),
	})
	if err != nil {
		return fmt.Errorf("new service: %w", err)
	}

	ctx := context.Background()

	// One case per session: belief contents carry no shopper id, so
	// the two cases would share concepts in one session and the
	// interrupted case could never prove absence. run.sh drives one
	// case per session and erases both after.
	switch os.Getenv("CASE") {
	case "gap":
		if err := runGap(ctx, svc, graph, out); err != nil {
			return err
		}
	case "birth":
		if err := runBirth(ctx, svc, graph, out); err != nil {
			return err
		}
	default:
		return fmt.Errorf("CASE is %q: want birth or gap", os.Getenv("CASE"))
	}

	fmt.Printf("LIVE OK: case %s on live Lambo + native inference ($0)\n", os.Getenv("CASE"))
	liveChild.stop()
	liveChild = nil
	return nil
}

// runBirth proves birth at exactly three consecutive opposites: the
// first two land unattached and the third births the rival,
// unpromoted, with the existing belief untouched.
func runBirth(ctx context.Context, svc *server.Service, graph *lamboGraph, out string) error {
	fmt.Println("--- birth: three consecutive opposites birth the rival ---")
	must(ctx, svc, birthShopper, "WH-2EV-01")
	first := answer(ctx, svc, birthShopper, "WH-2EV-01", smallAnswer)
	if !first.Accepted || first.AskAgain || first.BeliefID == nil {
		return fmt.Errorf("first return: accepted %v ask again %v belief %v, want true false attached",
			first.Accepted, first.AskAgain, first.BeliefID)
	}
	smallID := *first.BeliefID

	must(ctx, svc, birthShopper, "WH-2EV-02")
	wantUnattached(ctx, svc, birthShopper, "WH-2EV-02")
	must(ctx, svc, birthShopper, "WH-2EV-03")
	wantUnattached(ctx, svc, birthShopper, "WH-2EV-03")
	if rows := fitBeliefs(ctx, svc, birthShopper); len(rows) != 1 {
		return fmt.Errorf("below the threshold the shopper holds %d fit beliefs, want 1: no rival yet", len(rows))
	}
	fmt.Println("below the threshold: 1 fit belief, both opposites unattached")

	must(ctx, svc, birthShopper, "WH-2EV-05")
	birth := answer(ctx, svc, birthShopper, "WH-2EV-05", largeAnswer)
	if !birth.Accepted || birth.AskAgain || birth.BeliefID == nil {
		return fmt.Errorf("third opposite: accepted %v ask again %v belief %v, want true false attached",
			birth.Accepted, birth.AskAgain, birth.BeliefID)
	}
	if birth.Category != domain.CategoryFitTooLarge {
		return fmt.Errorf("birth answer: category %q, want fit_too_large", string(birth.Category))
	}
	rivalID := *birth.BeliefID
	if rivalID == smallID {
		return fmt.Errorf("the third opposite attached to the runs-small belief, want a new rival")
	}
	rows := fitBeliefs(ctx, svc, birthShopper)
	if len(rows) != 2 {
		return fmt.Errorf("after the birth the shopper holds %d fit beliefs, want 2", len(rows))
	}
	byID := map[string]contract.LedgerBeliefRow{}
	for _, row := range rows {
		byID[row.Belief.BeliefID] = row
	}
	rival, ok := byID[rivalID]
	if !ok {
		return fmt.Errorf("no ledger row for the rival %q", rivalID)
	}
	if rival.Belief.Value != "runs large, take size S" {
		return fmt.Errorf("rival value %q, want the reason's direction in words", rival.Belief.Value)
	}
	if rival.Belief.Status != domain.StatusClaimed || rival.Belief.Standing != domain.GraphNone {
		return fmt.Errorf("rival reads %q/%q, want Claimed/None: birth is not promotion",
			rival.Belief.Status, rival.Belief.Standing)
	}
	if rival.EvidenceCount != 1 {
		return fmt.Errorf("rival holds %d rows, want the 1 birth return", rival.EvidenceCount)
	}
	if rival.Belief.Contradicted {
		return fmt.Errorf("the newborn rival reads contradicted: nothing declared against it")
	}
	small, ok := byID[smallID]
	if !ok {
		return fmt.Errorf("no ledger row for the existing belief %q", smallID)
	}
	if small.EvidenceCount != 1 {
		return fmt.Errorf("existing belief holds %d rows, want 1: the opposites added no support", small.EvidenceCount)
	}
	if small.Belief.Contradicted || small.Belief.Standing != domain.GraphNone {
		return fmt.Errorf("existing belief reads contradicted %v standing %q, want false None: it is untouched",
			small.Belief.Contradicted, small.Belief.Standing)
	}

	fmt.Println("--- birth: live inspect shows the rival born, the old belief untouched ---")
	rivalSeen, err := graph.inspectRaw(rivalContent)
	if err != nil {
		return fmt.Errorf("inspect rival: %w", err)
	}
	save(out, "inspect-rival.txt", rivalSeen)
	if !strings.Contains(rivalSeen, rivalContent) {
		return fmt.Errorf("rival inspect names no rival concept: %.300s", rivalSeen)
	}
	if strings.Contains(rivalSeen, "Contradicts") {
		return fmt.Errorf("rival inspect declares against: nothing declared it")
	}
	if !strings.Contains(rivalSeen, "blast radius: 1") {
		return fmt.Errorf("rival blast radius is not 1: %.300s", rivalSeen)
	}
	if !strings.Contains(rivalSeen, "None -> Candidate refused") {
		return fmt.Errorf("rival is not unpromoted on the live session: %.300s", rivalSeen)
	}
	for _, line := range strings.Split(rivalSeen, "\n") {
		if strings.Contains(line, smallContent) && strings.Contains(line, "Dependency") {
			return fmt.Errorf("rival inspect supports the old belief: %q", strings.TrimSpace(line))
		}
	}
	fmt.Printf("rival inspect: holds %.60q, no against edge\n", rivalContent)
	smallSeen, err := graph.inspectRaw(smallContent)
	if err != nil {
		return fmt.Errorf("inspect existing belief: %w", err)
	}
	save(out, "inspect-small.txt", smallSeen)
	if !strings.Contains(smallSeen, smallContent) {
		return fmt.Errorf("existing-belief inspect lost the belief: %.300s", smallSeen)
	}
	if strings.Contains(smallSeen, "Contradicts") {
		return fmt.Errorf("existing-belief inspect declares against: birth declares nothing")
	}
	// The daemon links near-duplicate wordings with a symmetric
	// Semantic edge (its own similarity linking, visible in both
	// neighbourhoods). That edge is neither support nor
	// contradiction: the rule's "untouched" means no Dependency edge
	// to the rival, no Contradicts edge, and an unchanged blast
	// radius. Assert exactly that, line by line.
	if !strings.Contains(smallSeen, "blast radius: 1") {
		return fmt.Errorf("existing-belief blast radius moved: %.300s", smallSeen)
	}
	for _, line := range strings.Split(smallSeen, "\n") {
		if strings.Contains(line, rivalContent) && strings.Contains(line, "Dependency") {
			return fmt.Errorf("existing-belief inspect supports the rival: %q", strings.TrimSpace(line))
		}
	}
	fmt.Println("existing-belief inspect: still present, no contradiction, rival linked only by the daemon's Semantic edge")

	fmt.Println("birth case holds: rival born at 3, unpromoted, existing belief untouched")
	return nil
}

// runGap proves the interrupted streak births nothing, in a session
// that never saw the birth case: the rival content must name no
// concept at all, since the service is the only minter and it minted
// none here.
func runGap(ctx context.Context, svc *server.Service, graph *lamboGraph, out string) error {
	fmt.Println("--- interrupted streak: opposite, opposite, same, opposite births nothing ---")
	must(ctx, svc, gapShopper, "WH-2EV-07")
	gapFirst := answer(ctx, svc, gapShopper, "WH-2EV-07", smallAnswer)
	if !gapFirst.Accepted || gapFirst.BeliefID == nil {
		return fmt.Errorf("gap first return: accepted %v belief %v, want true attached",
			gapFirst.Accepted, gapFirst.BeliefID)
	}
	gapSmallID := *gapFirst.BeliefID
	must(ctx, svc, gapShopper, "WH-2EV-08")
	wantUnattached(ctx, svc, gapShopper, "WH-2EV-08")
	must(ctx, svc, gapShopper, "WH-2EV-09")
	wantUnattached(ctx, svc, gapShopper, "WH-2EV-09")
	must(ctx, svc, gapShopper, "WH-2EV-10")
	matched := answer(ctx, svc, gapShopper, "WH-2EV-10", smallAnswer)
	if !matched.Accepted || matched.BeliefID == nil || *matched.BeliefID != gapSmallID {
		return fmt.Errorf("gap same-direction answer attaches to %v, want the runs-small %q",
			matched.BeliefID, gapSmallID)
	}
	must(ctx, svc, gapShopper, "WH-2EV-11")
	wantUnattached(ctx, svc, gapShopper, "WH-2EV-11")
	if rows := fitBeliefs(ctx, svc, gapShopper); len(rows) != 1 {
		return fmt.Errorf("after opposite, opposite, same, opposite the shopper holds %d fit beliefs, want 1",
			len(rows))
	}
	if got := unattachedLarge(ctx, svc, gapShopper); got != 3 {
		return fmt.Errorf("ledger shows %d unattached too-large rows, want 3", got)
	}
	gapSmall := fitBeliefs(ctx, svc, gapShopper)[0]
	if gapSmall.EvidenceCount != 2 {
		return fmt.Errorf("gap belief holds %d rows, want 2: only the same-direction return joined it",
			gapSmall.EvidenceCount)
	}
	if gapSmall.Belief.Contradicted {
		return fmt.Errorf("gap belief reads contradicted: nothing declared against it")
	}

	fmt.Println("--- interrupted streak: live inspect shows no rival ---")
	absent, err := graph.inspectRaw(rivalContent)
	if err == nil {
		save(out, "inspect-gap-rival.txt", absent)
		if strings.Contains(absent, "blast radius:") {
			return fmt.Errorf("rival concept is admitted after the interrupted streak: %.300s", absent)
		}
		fmt.Println("rival inspect returned no admitted concept (see out/inspect-gap-rival.txt)")
	} else {
		save(out, "inspect-gap-rival.txt", "inspect refused: "+err.Error())
		fmt.Println("rival inspect refused: no such concept in the session")
	}

	fmt.Println("gap case holds: opposite, opposite, same, opposite births nothing")
	return nil
}
