// Live driver for T2.6: product.md's demo through the frozen interface
// with no agent, against the real pieces.
//
// The scripted client drives server.Service (the T2.5 service) over
// recording payments (fakes.DemoStoryInputs, whose shapes the T2.1b
// live leg proves identical to the sandbox reads), a live Jev
// classifier (internal/reasons.Client with the key attached by the
// transport wrapper), and a live Lambo 0.10.1 graph client over a fresh
// session with native embeddings. The evidence store is the in-memory
// lexical store; the live Elastic proof is the T2.3b leg run by run.sh
// beside this driver.
//
// This is a main program, never a test: no *_test.go lives under
// tests/live/, so CI builds this package but never runs it. Run it once
// by hand with the keys loaded:
//
//	source tools/env.sh
//	bash tests/live/layer/run.sh
//
// The key travels only in the Authorization header. Raw Jev replies
// land in out/ (git-ignored): they carry per-request generation ids,
// never the key. Stdout names fixture ids only and no secret.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	"github.com/nrynss/memento-emptor/internal/reasons"
)

// referenceCostPerCall is T0.7's measured per-call cost in USD. A live
// call costing ten times this trips the guard and stops the run.
const referenceCostPerCall = 0.0000214

// expectedBuild is the model string the endpoint resolved on the T0.7
// and T2.2b runs. A different resolution re-opens the floor and stops
// the run.
const expectedBuild = "typesafe/jev-1.14-20261014"

// pauseBetweenCalls spaces the live classifications. No retries are
// expected.
const pauseBetweenCalls = 2 * time.Second

// waitMS is the receipt wait per Lambo write, matching the T2.4b
// driver's wait.
const waitMS = 34000

// shopper is the demo shopper id shared with the frozen fixtures.
const shopper = "shopper_01"

// layerContract is the store's embedding contract. It carries no model
// name, dimensions, endpoint or key — only its kind, so the lexical
// store stands in for the live Elastic proof run beside this driver.
type layerContract struct{ kind string }

// EmbeddingKind names the contract family.
func (c layerContract) EmbeddingKind() string { return c.kind }

// Validate reports whether the contract is usable.
func (c layerContract) Validate() error {
	if c.kind == "" {
		return errors.New("t26-live: contract names no kind")
	}
	return nil
}

// liveReply is the billing and provenance surface of one live Jev
// reply. The frozen client decodes the classification itself; this
// only records what the transcript and the guards need.
type liveReply struct {
	model string
	cost  float64
}

// captureDo sends through inner with the key attached, saves the raw
// reply body to out/, and reports the reply's model and cost. The body
// is re-wrapped so the frozen client decodes the same bytes.
func captureDo(inner func(*http.Request) (*http.Response, error), key, out string, seen *int, reply *liveReply) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := inner(req)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		*seen++
		var parsed struct {
			Model string `json:"model"`
			Usage struct {
				Cost float64 `json:"cost"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(raw, &parsed); err == nil {
			*reply = liveReply{model: parsed.Model, cost: parsed.Usage.Cost}
			if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("%02d-response.json", *seen)), raw, 0o600); err != nil {
				return nil, err
			}
		}
		resp.Body = io.NopCloser(bytes.NewReader(raw))
		return resp, nil
	}
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

// startLambo provisions the store under cfg and starts the MCP server
// for session over its stdio pipes.
func startLambo(bin, cfg, session, agent string) (*rpcClient, error) {
	provision := exec.Command(bin, "--config", cfg, "provision")
	if raw, err := provision.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("lambo provision: %w: %s", err, strings.TrimSpace(string(raw)))
	}
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
			"clientInfo": map[string]any{"name": "t26-driver", "version": "0.1.0"},
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
var (
	_ fakes.EvidenceGraph = (*lamboGraph)(nil)
)

// waitApplied blocks until the daemon applies one receipt.
func (g *lamboGraph) waitApplied(receipt, what string) error {
	resp, err := g.client.call("lambo_stats", map[string]any{
		"agent_id": g.agent, "receipt": receipt, "wait_ms": waitMS,
	})
	if err != nil {
		return fmt.Errorf("t26-live: wait %s: %w", what, err)
	}
	text := textOf(resp)
	if strings.Contains(text, "embedder unavailable") || strings.Contains(text, "error sending request") {
		return fmt.Errorf("t26-live: embedder down after %s: %s", what, lastLine(text))
	}
	if !strings.Contains(text, "applied") {
		return fmt.Errorf("t26-live: receipt for %s never applied: %.200s", what, text)
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
		return "", fmt.Errorf("t26-live: decode receipt: %w", err)
	}
	if structured.Receipt == "" {
		return "", fmt.Errorf("t26-live: write issued no receipt")
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
		return fakes.BeliefReport{}, fmt.Errorf("t26-live: decode inspect: %w", err)
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

// liveChild is the running Lambo server, stopped by fail so a failed
// run never leaves a writer holding the session.
var liveChild *rpcClient

// eraseSession removes one session and reports the store's answer.
// It is safe to repeat: an absent session reports already_absent.
func eraseSession(bin, cfg, session string) (string, error) {
	raw, err := exec.Command(bin, "--config", cfg, "erase-session", "--session", session, "--confirm", session).CombinedOutput()
	text := strings.TrimSpace(string(raw))
	if err != nil {
		return text, fmt.Errorf("lambo erase-session %s: %w: %s", session, err, text)
	}
	return text, nil
}

// fail prints to stderr and exits nonzero.
func fail(err error) {
	if liveChild != nil {
		liveChild.stop()
		liveChild = nil
	}
	fmt.Fprintf(os.Stderr, "t26-live: %v\n", err)
	os.Exit(1)
}

// must ingests one webhook event and asserts its stored shape.
func must(ctx context.Context, svc *server.Service, eventID string, wantRows int, wantPending bool) {
	res, err := svc.IngestWebhook(ctx, shopper, eventID)
	if err != nil {
		fail(fmt.Errorf("ingest %s: %w", eventID, err))
	}
	if res.Rows != wantRows || res.Pending != wantPending || res.Duplicate {
		fail(fmt.Errorf("ingest %s: rows %d pending %t duplicate %t, want %d %t false",
			eventID, res.Rows, res.Pending, res.Duplicate, wantRows, wantPending))
	}
	fmt.Printf("ingest %s: %d row(s), pending=%t\n", eventID, res.Rows, res.Pending)
}

// questionForEvent returns the open question filed by one webhook event.
func questionForEvent(ctx context.Context, svc *server.Service, eventID string) string {
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

// answerReason records one classified answer for one event's question.
func answerReason(ctx context.Context, svc *server.Service, eventID, answer string) contract.RecordReasonResponse {
	res, err := svc.RecordReason(ctx, contract.RecordReasonRequest{
		ShopperID: shopper, QuestionID: questionForEvent(ctx, svc, eventID), AnswerText: answer,
	})
	if err != nil {
		fail(fmt.Errorf("record reason for %s: %w", eventID, err))
	}
	belief := "<nil>"
	if res.BeliefID != nil {
		belief = *res.BeliefID
	}
	fmt.Printf("reason %s %q: category=%s conf=%.2f accepted=%t ask_again=%t belief=%s\n",
		eventID, answer, res.Category, res.Confidence, res.Accepted, res.AskAgain, belief)
	return res
}

// recallJSON renders a recall response without volatile fields for
// step-to-step comparison.
func recallJSON(res contract.RecallResponse) string {
	raw, err := json.Marshal(res)
	if err != nil {
		fail(fmt.Errorf("encode recall: %w", err))
	}
	return string(raw)
}

func main() {
	out := os.Getenv("OUT")
	if out == "" {
		out = "tests/live/layer/out"
	}
	bin := os.Getenv("LAMBO_BIN")
	if bin == "" {
		bin = filepath.Join(os.Getenv("HOME"), "work/lambo/target/release/lambo")
	}
	session := os.Getenv("SESSION")
	if session == "" {
		session = "t26-layer"
	}
	agent := "memento-t26"
	if err := run(bin, session, agent, out); err != nil {
		fail(err)
	}
}

func run(bin, session, agent, out string) error {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		return fmt.Errorf("OPENROUTER_API_KEY is empty: source tools/env.sh first")
	}
	elasticURL := os.Getenv("ELASTIC_URL")
	elasticKey := os.Getenv("LAMBO_ES_API_KEY")
	if elasticURL == "" || elasticKey == "" {
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

	cfg := filepath.Join(out, "lambo.toml")
	config := "# T2.6 live run config, generated by the driver (not committed).\n" +
		"[store]\nkind = \"sqlite\"\npath = \"" + filepath.Join(out, "lambo.db") + "\"\n\n" +
		"[embedder]\nkind = \"elastic\"\nurl = \"" + elasticURL + "\"\n" +
		"model = \".jina-embeddings-v3\"\ndim = 1024\napi_key_env = \"LAMBO_ES_API_KEY\"\n" +
		"connect_timeout_ms = 30000\ntimeout_ms = 60000\ncanary = \"verify\"\n\n" +
		"# Cadence only, from the proven T0.3/T2.4b toml; the bars live in code.\n" +
		"[daemon]\ngc_interval = 1\ngc_idle_floor = 1\ngc_max_interval_secs = 3600\n" +
		"canonization_eval_interval_secs = 5\n"
	if err := os.WriteFile(cfg, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write lambo config: %w", err)
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

	httpClient := &http.Client{Timeout: 90 * time.Second}
	seen := 0
	var reply liveReply
	classifier := reasons.NewClient(reasons.DefaultModel, reasons.DefaultEndpoint,
		captureDo(httpClient.Do, key, out, &seen, &reply))

	store, err := evidence.NewStore(layerContract{kind: "server-story"})
	if err != nil {
		return fmt.Errorf("open evidence store: %w", err)
	}
	svc, err := server.NewService(server.Deps{
		Payments:   fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      graph,
		SchemaDir:  filepath.Join("testdata", "api"),
	})
	if err != nil {
		return fmt.Errorf("new service: %w", err)
	}

	ctx := context.Background()
	fmt.Println("--- refunds in, pending questions out ---")
	for _, event := range []string{"WH-2EV-01", "WH-2EV-02", "WH-2EV-03"} {
		must(ctx, svc, event, 1, true)
	}
	pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		return fmt.Errorf("pending questions: %w", err)
	}
	if len(pending.Questions) != 3 {
		return fmt.Errorf("pending holds %d questions, want 3", len(pending.Questions))
	}
	fmt.Printf("pending: %d open questions\n", len(pending.Questions))

	fmt.Println("--- live reasons in: two too-small, one too-large ---")
	first := answerReason(ctx, svc, "WH-2EV-01", "too tight in the shoulders")
	if !first.Accepted || first.AskAgain {
		return fmt.Errorf("first return: accepted %v ask again %v, want true false", first.Accepted, first.AskAgain)
	}
	guard(reply)
	recall, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		return fmt.Errorf("recall after one return: %w", err)
	}
	if len(recall.Beliefs) != 0 || recall.Guidance.Basis != contract.GuidanceNone {
		return fmt.Errorf("one return earns nothing, recall serves %d beliefs with %q guidance",
			len(recall.Beliefs), recall.Guidance.Basis)
	}
	fmt.Println("recall after one return: no belief, no guidance (vote 1 of 3)")

	answerReason(ctx, svc, "WH-2EV-02", "the sleeves are way too short and I can't button it")
	third := answerReason(ctx, svc, "WH-2EV-03", "it swims on me, far too big")
	if !third.Accepted {
		return fmt.Errorf("third return was not accepted as evidence")
	}
	guard(reply)
	sizingID := ""
	if third.BeliefID != nil {
		sizingID = *third.BeliefID
	}
	fmt.Println("--- Earned on the live graph ---")
	if err := waitVenerable(graph); err != nil {
		return err
	}
	earned, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		return fmt.Errorf("recall after three returns: %w", err)
	}
	if len(earned.Beliefs) != 1 {
		return fmt.Errorf("recall serves %d beliefs, want the 1 earned sizing belief", len(earned.Beliefs))
	}
	sizing := earned.Beliefs[0]
	if sizing.Status != domain.StatusEarned || sizing.Standing != domain.GraphVenerable {
		return fmt.Errorf("sizing reads %q/%q, want Earned/Venerable", sizing.Status, sizing.Standing)
	}
	if sizing.Contradicted || earned.OverridingCorrection != nil {
		return fmt.Errorf("no correction applies yet, but recall contradicts or overrides")
	}
	if earned.Guidance.Basis != contract.GuidanceBelief || earned.Guidance.Text != "Take size L for Brand X" {
		return fmt.Errorf("guidance %+v, want belief guidance toward size L", earned.Guidance)
	}
	if earned.PendingCount != 0 {
		return fmt.Errorf("pending count %d, want 0 before the fourth refund", earned.PendingCount)
	}
	fmt.Printf("recall: belief %s Earned/Venerable, guidance %q, evidence %d rows\n",
		sizing.BeliefID, earned.Guidance.Text, len(sizing.Evidence))

	fmt.Println("--- vague answer stays pending ---")
	must(ctx, svc, "WH-2EV-05", 1, true)
	vague := answerReason(ctx, svc, "WH-2EV-05", "not great")
	if vague.Accepted || !vague.AskAgain || vague.BeliefID != nil {
		return fmt.Errorf("vague answer: accepted %v ask again %v belief %v, want false true nil",
			vague.Accepted, vague.AskAgain, vague.BeliefID)
	}
	guard(reply)

	fmt.Println("--- dispute counts only against the seller ---")
	before, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		return fmt.Errorf("recall before dispute: %w", err)
	}
	must(ctx, svc, "WH-2EV-04", 1, false)
	after, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		return fmt.Errorf("recall after dispute: %w", err)
	}
	if recallJSON(before) != recallJSON(after) {
		return fmt.Errorf("the non-arrival dispute moved the sizing recall")
	}
	fmt.Println("recall unchanged by the dispute")
	ledger, err := svc.LedgerBeliefs(ctx, contract.LedgerBeliefsRequest{ShopperID: shopper})
	if err != nil {
		return fmt.Errorf("ledger beliefs: %w", err)
	}
	sellerFound := false
	for _, row := range ledger.Beliefs {
		if row.Belief.Kind == domain.BeliefKindSeller {
			sellerFound = true
			if row.Belief.Status != domain.StatusClaimed || row.EvidenceCount != 1 {
				return fmt.Errorf("seller belief reads %q with %d rows, want Claimed with 1",
					row.Belief.Status, row.EvidenceCount)
			}
			fmt.Printf("seller belief %s: Claimed, 1 row (one dispute never earns)\n", row.Belief.BeliefID)
		}
	}
	if !sellerFound {
		return fmt.Errorf("the dispute wrote no seller belief")
	}

	fmt.Println("--- correction wins at once, belief stays ---")
	traced, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001", TraceID: "trace-R"})
	if err != nil {
		return fmt.Errorf("traced recall: %w", err)
	}
	if len(traced.Beliefs) != 1 {
		return fmt.Errorf("traced recall serves %d beliefs, want 1", len(traced.Beliefs))
	}
	correction, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: "I've lost weight, use M",
		ContradictsBeliefID: sizingID, TraceID: "trace-R",
	})
	if err != nil {
		return fmt.Errorf("record correction: %w", err)
	}
	if correction.Status != domain.StatusOutOfTrue || correction.Standing != domain.GraphVenerable || !correction.Contradicted {
		return fmt.Errorf("correction leaves %q/%q contradicted %v, want Out of true/Venerable/true",
			correction.Status, correction.Standing, correction.Contradicted)
	}
	if correction.Guidance.Basis != contract.GuidanceCorrection || correction.Guidance.Text != "Take size M for Brand X" {
		return fmt.Errorf("correction guidance %+v, want M at once", correction.Guidance)
	}
	overridden, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		return fmt.Errorf("recall after correction: %w", err)
	}
	if len(overridden.Beliefs) != 1 || !overridden.Beliefs[0].Contradicted {
		return fmt.Errorf("recall after correction serves no contradicted belief")
	}
	inline := overridden.Beliefs[0].ContradictedBy
	if overridden.OverridingCorrection == nil || inline == nil ||
		overridden.OverridingCorrection.CorrectionID != inline.CorrectionID {
		return fmt.Errorf("the override does not ride both at the top level and inline")
	}
	if overridden.Guidance.Basis != contract.GuidanceCorrection ||
		overridden.Guidance.CorrectionID == nil ||
		*overridden.Guidance.CorrectionID != overridden.OverridingCorrection.CorrectionID {
		return fmt.Errorf("guidance does not point at the overriding correction")
	}
	fmt.Printf("recall after correction: Out of true, standing Venerable, guidance %q\n", overridden.Guidance.Text)

	fmt.Println("--- purchase feedback grows the rival, recall holds ---")
	must(ctx, svc, "WH-2EV-06", 1, false)
	feedback, err := svc.RecordFeedback(ctx, contract.RecordFeedbackRequest{
		ShopperID: shopper, CustomID: "memento-shopper01-06",
		SKU: "PSW8GeK", Size: "M", FeedbackText: "The M fits well across the shoulders.",
	})
	if err != nil {
		return fmt.Errorf("record feedback: %w", err)
	}
	if !feedback.Accepted || feedback.Verdict != contract.FeedbackFitsWell {
		return fmt.Errorf("feedback: accepted %v verdict %q, want true fits_well", feedback.Accepted, feedback.Verdict)
	}
	final, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		return fmt.Errorf("final recall: %w", err)
	}
	if len(final.Beliefs) != 1 || final.Beliefs[0].BeliefID != sizingID {
		return fmt.Errorf("feedback grew a rival belief but recall still serves only the earned sizing belief")
	}
	fmt.Printf("feedback: verdict %s, recall still serves the earned sizing belief\n", feedback.Verdict)

	calls := svc.Calls("trace-R")
	if len(calls) != 2 || calls[0].Tool != string(contract.ToolRecallForPurchase) || calls[1].Tool != string(contract.ToolRecordCorrection) {
		return fmt.Errorf("trace-R log shows %v, want recall before the correction it caused", calls)
	}
	fmt.Println("call log: recall_for_purchase then record_correction under trace-R")

	explained, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: sizingID})
	if err != nil {
		return fmt.Errorf("explain belief: %w", err)
	}
	fmt.Printf("explain: %d evidence, %d correction(s), %d timeline events\n",
		len(explained.Belief.Evidence), len(explained.Corrections), len(explained.Timeline))

	fmt.Printf("LIVE OK: story end to end on live Lambo + live Jev (%d Jev calls)\n", seen)

	liveChild.stop()
	liveChild = nil
	if text, err := eraseSession(bin, cfg, session); err != nil {
		return fmt.Errorf("erase session after run: %s", text)
	} else {
		fmt.Printf("erased session %s: %s\n", session, text)
	}
	return nil
}

// guard enforces the spend and version guards after live Jev calls:
// the build must resolve pinned and no call may cost more than ten
// times the reference, or the run stops instead of spending on.
func guard(reply liveReply) {
	if reply.model != expectedBuild {
		fail(fmt.Errorf("model resolved to %s, want %s: version change re-opens the floor, stopping per the guard",
			reply.model, expectedBuild))
	}
	if reply.cost > 10*referenceCostPerCall {
		fail(fmt.Errorf("call cost %.9f USD, over 10x the %.7f reference: stopping per the guard",
			reply.cost, referenceCostPerCall))
	}
	fmt.Printf("jev: model %s, last call %.9f USD\n", reply.model, reply.cost)
}

// waitVenerable polls the live graph until the sizing belief reaches
// Venerable. Promotion is eventual: the daemon evaluates on its own
// cadence, so the verdict right after the last receipt can predate the
// final flush.
func waitVenerable(g *lamboGraph) error {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		report, err := pollSizing(g)
		if err != nil {
			return err
		}
		if report.State == domain.GraphVenerable {
			fmt.Printf("live inspect: Venerable, blast radius %d, coverage %.3f, distinct %d\n",
				report.Dependents, report.Coverage, report.DistinctInteractions)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for Venerable (state %q)", string(report.State))
		}
		time.Sleep(5 * time.Second)
	}
}

// pollSizing inspects the sizing belief. The service keys its content
// from the brand and the first classified direction (see
// internal/api/server/content.go: beliefContent over fitBeliefValue);
// the poll repeats that key verbatim, and the Venerable assertion
// below fails loudly if the key ever drifts from the service's.
func pollSizing(g *lamboGraph) (fakes.BeliefReport, error) {
	const content = "memento fit brand brand-x-001 runs small, take size L"
	report, err := g.Inspect(context.Background(), content)
	if err != nil {
		return fakes.BeliefReport{}, fmt.Errorf("inspect sizing belief: %w", err)
	}
	fmt.Printf("poll: state=%s dependents=%d\n", string(report.State), report.Dependents)
	return report, nil
}
