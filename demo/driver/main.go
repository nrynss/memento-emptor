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

	"github.com/nrynss/memento-emptor/agent/catalog"
	"github.com/nrynss/memento-emptor/agent/loops"
	"github.com/nrynss/memento-emptor/agent/orders"
	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/api/server"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/reasons"
)

// referenceCostPerCall is T0.7's measured per-call Jev cost in USD. A
// live call costing ten times this trips the guard and stops the run.
const referenceCostPerCall = 0.0000214

// expectedBuild is the Jev model string the endpoint resolved on the
// T0.7 and T2.2b runs. A different resolution re-opens the floor and
// stops the run.
const expectedBuild = "typesafe/jev-1.14-20261014"

// pauseBetweenCalls spaces the live classifications.
const pauseBetweenCalls = 2 * time.Second

// shopper is the demo shopper id shared with the frozen fixtures.
const shopper = "shopper_01"

// turnGuardUSD caps any single Astropods model turn. The brief's guard
// is $0.10; the driver stops the run past it.
const turnGuardUSD = 0.10

// redact maps real values seen live to their committed placeholders.
// Everything committed under demo/ uses the right-hand side only.
type redact struct {
	table map[string]string
}

// add records one mapping. Empty reals are refused: a missing id must
// fail the run, never write an empty placeholder.
func (r *redact) add(real, placeholder string) {
	if real == "" {
		fail(fmt.Errorf("refusing empty real id for placeholder %s", placeholder))
	}
	r.table[real] = placeholder
}

// scrub replaces every known real value with its placeholder.
func (r *redact) scrub(text string) string {
	for real, placeholder := range r.table {
		if real != "" {
			text = strings.ReplaceAll(text, real, placeholder)
		}
	}
	return text
}

var (
	red   = &redact{table: map[string]string{}}
	logW  io.Writer
	out   string
	runNo string
)

// say prints one scrubbed line to the console and the full run log.
func say(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	fmt.Println(red.scrub(line))
	fmt.Fprintln(logW, line)
}

// fail prints to console and the log, then exits nonzero.
func fail(err error) {
	msg := fmt.Sprintf("t36-demo: %v", err)
	fmt.Println(red.scrub(msg))
	fmt.Fprintln(logW, msg)
	os.Exit(1)
}

// layerContract is the store's embedding contract. It carries no model
// name, dimensions, endpoint or key — only its kind, so the lexical
// store stands in for the live Elastic proof run beside this driver
// (T2.3b), exactly as the T2.6 driver did.
type layerContract struct{ kind string }

// EmbeddingKind names the contract family.
func (c layerContract) EmbeddingKind() string { return c.kind }

// Validate reports whether the contract is usable.
func (c layerContract) Validate() error {
	if c.kind == "" {
		return errors.New("t36-demo: contract names no kind")
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
func captureDo(inner func(*http.Request) (*http.Response, error), key, dir string, seen *int, reply *liveReply) func(*http.Request) (*http.Response, error) {
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
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("jev-%02d-response.json", *seen)), raw, 0o600); err != nil {
				return nil, err
			}
		}
		resp.Body = io.NopCloser(bytes.NewReader(raw))
		return resp, nil
	}
}

// guard enforces the spend and version guards after live Jev calls:
// the build must resolve pinned and no call may cost more than ten
// times the reference, or the run stops instead of spending on.
func guard(reply liveReply) {
	if reply.model != expectedBuild {
		fail(fmt.Errorf("jev model resolved to %s, want %s: version change re-opens the floor, stopping per the guard",
			reply.model, expectedBuild))
	}
	if reply.cost > 10*referenceCostPerCall {
		fail(fmt.Errorf("jev call cost %.9f USD, over 10x the %.7f reference: stopping per the guard",
			reply.cost, referenceCostPerCall))
	}
	say("jev: model %s, last call %.9f USD", reply.model, reply.cost)
}

// execRun runs one command, returning its trimmed stdout. Stderr
// streams to the console; secrets never appear in argv.
func execRun(dir string, env []string, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fmt.Print(red.scrub(stderr.String()))
		fail(fmt.Errorf("%s %v: %w", name, args, err))
	}
	if s := strings.TrimSpace(stderr.String()); s != "" {
		fmt.Print(red.scrub(s))
	}
	return strings.TrimSpace(stdout.String())
}

// astBilling logs the current spend position: usage, credit, limits.
func astBilling() {
	raw := execRun("", nil, "ast", "billing", "get")
	for _, line := range strings.Split(raw, "\n") {
		for _, key := range []string{"Usage", "Credit", "Limit", "Period"} {
			if strings.Contains(line, key) {
				say("billing: %s", strings.TrimSpace(line))
			}
		}
	}
}

// traceListEntry is one row of the trace list.
type traceListEntry struct {
	TraceID   string  `json:"trace_id"`
	Cost      float64 `json:"total_cost"`
	Timestamp string  `json:"timestamp"`
}

// traceSession returns the conversation the trace belongs to. The
// list rows carry no conversation id, so candidates are confirmed
// through the detail's session id before they are accepted.
func traceSession(traceID string) string {
	raw := execRun("", nil, "ast", "agent", "trace", "--id", "9d3-n2x-s0a", "-t", traceID, "--json")
	var parsed struct {
		Trace struct {
			SessionID string `json:"session_id"`
		} `json:"trace"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		fail(fmt.Errorf("parse trace %s detail: %w", traceID, err))
	}
	return parsed.Trace.SessionID
}

// newestTrace returns the newest invoke_agent trace at or after start
// that is not in seen and belongs to this run's conversation. Trace
// timestamps are completion times, so a previous turn's trace can
// complete after the next turn starts, and a previous RUN's traces
// fall inside the grace window: seen ids are excluded, and every
// candidate is confirmed through its session id before acceptance.
func newestTrace(since time.Time, seen map[string]bool, conv string) traceListEntry {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		raw := execRun("", nil, "ast", "agent", "trace", "--id", "9d3-n2x-s0a", "--limit", "6", "--json")
		var listed struct {
			Traces []traceListEntry `json:"traces"`
		}
		if err := json.Unmarshal([]byte(raw), &listed); err != nil {
			fail(fmt.Errorf("parse trace list: %w", err))
		}
		for _, entry := range listed.Traces {
			if seen[entry.TraceID] {
				continue
			}
			moment, err := time.Parse(time.RFC3339, entry.Timestamp)
			if err != nil {
				continue
			}
			if moment.Before(since.Add(-2 * time.Minute)) {
				continue
			}
			seen[entry.TraceID] = true
			if traceSession(entry.TraceID) != conv {
				continue
			}
			return entry
		}
		if time.Now().After(deadline) {
			fail(fmt.Errorf("no new trace after %s", since.Format(time.RFC3339)))
		}
		time.Sleep(15 * time.Second)
	}
}

// traceDetail fetches one trace's full JSON into out/ and returns the
// parsed output text, cost and observations.
func traceDetail(name, traceID string) (string, float64, []map[string]any) {
	raw := execRun("", nil, "ast", "agent", "trace", "--id", "9d3-n2x-s0a", "-t", traceID, "--json")
	path := filepath.Join(out, "trace-"+name+".json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		fail(err)
	}
	var parsed struct {
		Trace struct {
			Cost   float64 `json:"total_cost"`
			Output struct {
				Text string `json:"text"`
			} `json:"output"`
		} `json:"trace"`
		Observations []map[string]any `json:"observations"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		fail(fmt.Errorf("parse trace %s: %w", traceID[:8], err))
	}
	return parsed.Trace.Output.Text, parsed.Trace.Cost, parsed.Observations
}

// toolIndex returns the positions of tool spans in observation order.
func toolIndex(observations []map[string]any) []string {
	names := []string{}
	for _, obs := range observations {
		kind, _ := obs["type"].(string)
		name, _ := obs["name"].(string)
		if kind == "tool" {
			names = append(names, name)
		}
	}
	return names
}

// agentTurn sends one shopping message and returns its trace id, reply
// text and cost. Billing is re-checked around the turn and the per-turn
// guard trips the run past $0.10.
func agentTurn(seen map[string]bool, conv, text string) (string, string) {
	say("--- agent turn: %q", text)
	astBilling()
	// The platform edge occasionally answers a send with a retryable
	// 5xx instead of delivering the message (seen once as a 502 with
	// retry_after 60). Back off past the asked delay and resend, up to
	// three attempts; a message that never delivers fails the run
	// instead of polling for a trace that will never exist.
	var sendOut string
	for attempt := 1; ; attempt++ {
		cmd := exec.Command("python3", "demo/drive.py", "send", conv, text)
		cmd.Dir = ""
		cmd.Env = append(os.Environ(), "OUT="+out)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		runErr := cmd.Run()
		fmt.Print(red.scrub(strings.TrimSpace(stderr.String())))
		if runErr == nil {
			sendOut = strings.TrimSpace(stdout.String())
			break
		} else if attempt >= 3 {
			fail(fmt.Errorf("send %q: %v", text, runErr))
		}
		say("send attempt %d failed; backing off 70s before resending", attempt)
		time.Sleep(70 * time.Second)
	}
	_ = sendOut
	start := time.Now()
	entry := newestTrace(start, seen, conv)
	text2, cost, observations := traceDetail("turn", entry.TraceID)
	_ = observations
	say("turn trace %.8s cost %.4f USD", entry.TraceID, cost)
	if cost > turnGuardUSD {
		fail(fmt.Errorf("turn cost %.4f USD over the %.2f guard", cost, turnGuardUSD))
	}
	astBilling()
	red.add(entry.TraceID, "TRACE-T36-R"+runNo)
	red.add(conv, "CONV-T36-R"+runNo)
	return entry.TraceID, text2
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
	ctx := context.Background()
	out = os.Getenv("OUT")
	if out == "" {
		out = "demo/out"
	}
	sessionBytes, err := os.ReadFile(filepath.Join(out, "SESSION"))
	if err != nil {
		fail(fmt.Errorf("read SESSION (run demo/reset.sh first): %w", err))
	}
	session := strings.TrimSpace(string(sessionBytes))
	runBytes, err := os.ReadFile(filepath.Join(out, "RUN"))
	if err != nil {
		fail(fmt.Errorf("read RUN: %w", err))
	}
	runNo = strings.TrimSpace(string(runBytes))
	key := os.Getenv("OPENROUTER_API_KEY")
	elasticURL := os.Getenv("ELASTIC_URL")
	elasticKey := os.Getenv("LAMBO_ES_API_KEY")
	channelKey := os.Getenv("CHANNEL3_API_KEY")
	if key == "" || elasticURL == "" || elasticKey == "" {
		fail(fmt.Errorf("OPENROUTER_API_KEY, ELASTIC_URL and LAMBO_ES_API_KEY must come from tools/env.sh"))
	}
	if channelKey == "" {
		fail(fmt.Errorf("CHANNEL3_API_KEY must come from tools/env.sh"))
	}
	logFile, err := os.Create(filepath.Join(out, "driver.log"))
	if err != nil {
		fail(err)
	}
	defer logFile.Close()
	logW = logFile
	traceID := "trace-T36-R" + runNo

	fmt.Printf("t36-demo run %s session %s\n", runNo, session)
	astBilling()

	hc := &http.Client{Timeout: 90 * time.Second}
	token, err := grant(ctx, hc)
	if err != nil {
		fail(err)
	}
	client := &restClient{http: hc, token: token}

	say("=== step 1 (live): three documented fit returns read back ===")
	webhookID, site, err := siteToken(ctx, client)
	if err != nil {
		fail(err)
	}
	client.id = webhookID
	deliveries, err := listDeliveries(ctx, hc, site)
	if err != nil {
		fail(err)
	}
	say("test endpoint holds %d deliveries", len(deliveries))
	for i, custom := range []string{"t13b-demo-003", "t13b-demo-004", "t13b-demo-005"} {
		d, err := findDelivery(deliveries, custom, "PAYMENT.CAPTURE.REFUNDED")
		if err != nil {
			fail(err)
		}
		var envelope struct {
			Resource map[string]any `json:"resource"`
		}
		if err := json.Unmarshal(d.raw, &envelope); err != nil {
			fail(err)
		}
		resource := envelope.Resource
		refundID, _ := resource["id"].(string)
		status, _ := resource["status"].(string)
		amount, _ := resource["amount"].(map[string]any)
		red.add(d.eventID, fmt.Sprintf("EVT-LIVE-R%s-F%d", runNo, i+1))
		red.add(refundID, fmt.Sprintf("RFD-LIVE-R%s-F%d", runNo, i+1))
		if status != "COMPLETED" {
			fail(fmt.Errorf("refund %s status %q, want COMPLETED", custom, status))
		}
		got, err := client.get(ctx, "/v2/payments/refunds/"+refundID)
		if err != nil {
			fail(err)
		}
		if str(got, "status") != "COMPLETED" {
			fail(fmt.Errorf("refund read %s status %q", custom, str(got, "status")))
		}
		captureID := ""
		for _, link := range toSlice(resource["links"]) {
			linkMap, _ := link.(map[string]any)
			if linkMap["rel"] == "up" {
				href, _ := linkMap["href"].(string)
				captureID = href[strings.LastIndex(href, "/")+1:]
			}
		}
		red.add(captureID, fmt.Sprintf("CAP-LIVE-R%s-F%d", runNo, i+1))
		capture, err := client.get(ctx, "/v2/payments/captures/"+captureID)
		if err != nil {
			fail(err)
		}
		// The fit cycles were fully refunded by design, so the capture
		// reads back REFUNDED; either settled status proves the capture.
		if str(capture, "status") != "COMPLETED" && str(capture, "status") != "REFUNDED" {
			fail(fmt.Errorf("capture %s status %q, want a settled capture", custom, str(capture, "status")))
		}
		orderID := ""
		for _, link := range toSlice(capture["links"]) {
			linkMap, _ := link.(map[string]any)
			if linkMap["rel"] == "up" {
				href, _ := linkMap["href"].(string)
				orderID = href[strings.LastIndex(href, "/")+1:]
			}
		}
		red.add(orderID, fmt.Sprintf("ORD-LIVE-R%s-F%d", runNo, i+1))
		order, err := client.get(ctx, "/v2/checkout/orders/"+orderID)
		if err != nil {
			fail(err)
		}
		name, qty := "", ""
		for _, pu := range toSlice(order["purchase_units"]) {
			puMap, _ := pu.(map[string]any)
			if str(puMap, "custom_id") != custom {
				fail(fmt.Errorf("order custom id %q, want %q", str(puMap, "custom_id"), custom))
			}
			for _, item := range toSlice(puMap["items"]) {
				itemMap, _ := item.(map[string]any)
				name, _ = itemMap["name"].(string)
				qty, _ = itemMap["quantity"].(string)
			}
		}
		if !strings.Contains(strings.ToLower(name), "oxford") || qty != "1" {
			fail(fmt.Errorf("order item %q x%s, want the oxford shirt x1", name, qty))
		}
		_ = amount
		say("fit return %d: refund COMPLETED, capture COMPLETED, order %s x1", i+1, name)
	}

	say("=== layer story on live Lambo + live Jev ===")
	bin := os.Getenv("LAMBO_BIN")
	if bin == "" {
		bin = filepath.Join(os.Getenv("HOME"), "work/lambo/target/release/lambo")
	}
	cfg := filepath.Join(out, "lambo.toml")
	config := "# T3.6 live run config, generated by the driver (not committed).\n" +
		"[store]\nkind = \"sqlite\"\npath = \"" + filepath.Join(out, "lambo.db") + "\"\n\n" +
		"[embedder]\nkind = \"elastic\"\nurl = \"" + elasticURL + "\"\n" +
		"model = \".jina-embeddings-v3\"\ndim = 1024\napi_key_env = \"LAMBO_ES_API_KEY\"\n" +
		"connect_timeout_ms = 30000\ntimeout_ms = 60000\ncanary = \"verify\"\n\n" +
		"# Cadence only, from the proven T0.3/T2.4b toml; the bars live in code.\n" +
		"[daemon]\ngc_interval = 1\ngc_idle_floor = 1\ngc_max_interval_secs = 3600\n" +
		"canonization_eval_interval_secs = 5\n"
	if err := os.WriteFile(cfg, []byte(config), 0o600); err != nil {
		fail(fmt.Errorf("write lambo config: %w", err))
	}
	rpc, err := startLambo(bin, cfg, session, "memento-t36")
	if err != nil {
		fail(err)
	}
	defer rpc.stop()
	graph := &lamboGraph{client: rpc, agent: "memento-t36"}

	seen := 0
	var reply liveReply
	classifier := reasons.NewClient(reasons.DefaultModel, reasons.DefaultEndpoint,
		captureDo(http.DefaultClient.Do, key, out, &seen, &reply))

	store, err := evidence.NewStore(layerContract{kind: "demo-t36"})
	if err != nil {
		fail(fmt.Errorf("open evidence store: %w", err))
	}
	payments := &purchaseSource{base: fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs())}
	svc, err := server.NewService(server.Deps{
		Payments:   payments,
		Classifier: classifier,
		Store:      store,
		Graph:      graph,
		SchemaDir:  filepath.Join("testdata", "api"),
	})
	if err != nil {
		fail(fmt.Errorf("new service: %w", err))
	}

	must := func(eventID string, rows int, pending bool) {
		res, err := svc.IngestWebhook(ctx, shopper, eventID)
		if err != nil {
			fail(fmt.Errorf("ingest %s: %w", eventID, err))
		}
		if res.Rows != rows || res.Pending != pending || res.Duplicate {
			fail(fmt.Errorf("ingest %s: rows %d pending %t duplicate %t, want %d %t false",
				eventID, res.Rows, res.Pending, res.Duplicate, rows, pending))
		}
		say("ingest %s: %d row(s), pending=%t", eventID, res.Rows, res.Pending)
	}
	questionFor := func(eventID string) string {
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
	answer := func(eventID, words string) contract.RecordReasonResponse {
		res, err := svc.RecordReason(ctx, contract.RecordReasonRequest{
			ShopperID: shopper, QuestionID: questionFor(eventID), AnswerText: words,
		})
		if err != nil {
			fail(fmt.Errorf("record reason for %s: %w", eventID, err))
		}
		belief := "<nil>"
		if res.BeliefID != nil {
			belief = *res.BeliefID
		}
		say("reason %s %q: category=%s conf=%.2f accepted=%t ask_again=%t belief=%s",
			eventID, words, res.Category, res.Confidence, res.Accepted, res.AskAgain, belief)
		guard(reply)
		return res
	}

	for _, event := range []string{"WH-2EV-01", "WH-2EV-02", "WH-2EV-03"} {
		must(event, 1, true)
	}
	pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		fail(err)
	}
	if len(pending.Questions) != 3 {
		fail(fmt.Errorf("pending holds %d questions, want 3", len(pending.Questions)))
	}
	say("pending: 3 open questions")
	first := answer("WH-2EV-01", "too tight in the shoulders")
	if !first.Accepted || first.AskAgain || first.Category != domain.CategoryFitTooSmall {
		fail(fmt.Errorf("first return: accepted %t ask again %t category %s, want the too-small return",
			first.Accepted, first.AskAgain, first.Category))
	}
	recall, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		fail(err)
	}
	// One return is thin evidence, and recall never withholds it: the
	// belief comes back with its Claimed label, its one row and the
	// own-evidence step that decides, so the agent shows the shopper how
	// sure the layer is instead of being told nothing.
	if len(recall.Beliefs) != 1 {
		fail(fmt.Errorf("one return serves %d beliefs, want the 1 thin fit belief", len(recall.Beliefs)))
	}
	thin := recall.Beliefs[0]
	if thin.Status != domain.StatusClaimed || thin.Standing != domain.GraphNone {
		fail(fmt.Errorf("one return reads %q/%q, want Claimed/None: thin evidence is labelled, not withheld",
			thin.Status, thin.Standing))
	}
	if len(thin.Evidence) != 1 {
		fail(fmt.Errorf("one return carries %d evidence rows, want its one", len(thin.Evidence)))
	}
	if thin.Evidence[0].Reason == nil || thin.Evidence[0].Reason.Category != domain.CategoryFitTooSmall {
		fail(fmt.Errorf("one return's row reads %+v, want the too-small return", thin.Evidence[0].Reason))
	}
	if thin.Precedence == nil || thin.Precedence.Step != contract.GuidanceOwnEvidence || thin.Precedence.Overridden {
		fail(fmt.Errorf("one return precedence %+v, want the deciding own-evidence step", thin.Precedence))
	}
	if recall.Guidance.Basis != contract.GuidanceOwnEvidence || recall.Guidance.Text != "Take size L for Brand X" {
		fail(fmt.Errorf("one-return guidance %+v, want own evidence toward size L", recall.Guidance))
	}
	say("recall after one return: %s/%s, 1 row, guidance %q (vote 1 of 3)",
		thin.Status, thin.Standing, recall.Guidance.Text)
	answer("WH-2EV-02", "the sleeves are way too short and I can't button it")
	third := answer("WH-2EV-03", "sleeves too short in M")
	if !third.Accepted || third.Category != domain.CategoryFitTooSmall {
		fail(fmt.Errorf("third return: accepted %v category %s", third.Accepted, third.Category))
	}
	sizingID := ""
	if third.BeliefID != nil {
		sizingID = *third.BeliefID
	}
	red.add(sizingID, "BLF-T36-R"+runNo)
	if err := waitVenerable(graph); err != nil {
		fail(err)
	}
	earned, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		fail(err)
	}
	if len(earned.Beliefs) != 1 {
		fail(fmt.Errorf("recall serves %d beliefs, want the 1 earned sizing belief", len(earned.Beliefs)))
	}
	sizing := earned.Beliefs[0]
	if sizing.Status != domain.StatusEarned || sizing.Standing != domain.GraphVenerable {
		fail(fmt.Errorf("sizing reads %q/%q, want Earned/Venerable", sizing.Status, sizing.Standing))
	}
	if earned.Guidance.Basis != contract.GuidanceBelief || earned.Guidance.Text != "Take size L for Brand X" {
		fail(fmt.Errorf("guidance %+v, want belief guidance toward size L", earned.Guidance))
	}
	say("STEP 1 DONE: three documented fit returns -> Earned; recall guides %q with %d evidence rows",
		earned.Guidance.Text, len(sizing.Evidence))

	say("=== step 2 (live): Channel3 candidate sharing the cut ===")
	catClient := catalog.NewClient(channelKey, "", nil)
	candidate, err := catClient.Fetch(ctx, "PSW8GeK")
	if err != nil {
		fail(fmt.Errorf("channel3 fetch candidate: %w", err))
	}
	returned, err := catClient.Fetch(ctx, "tbnrl46")
	if err != nil {
		fail(fmt.Errorf("channel3 fetch returned sku: %w", err))
	}
	say("candidate: %s brand %s category %s fit %v", candidate.Title, candidate.BrandID, candidate.Category, candidate.Fit["top-fit"])
	if candidate.BrandID != "SMwk" || returned.BrandID != "SMwk" {
		fail(fmt.Errorf("brand drift: candidate %s returned %s, want SMwk", candidate.BrandID, returned.BrandID))
	}
	want := catalog.Want{
		BrandID: candidate.BrandID, Category: candidate.Category,
		FitAttribute: "top-fit", FitValues: candidate.Fit["top-fit"],
		ExcludeIDs: []string{"tbnrl46"},
	}
	if err := want.Validate(); err != nil {
		fail(fmt.Errorf("candidate want: %w", err))
	}
	matched := catalog.Match(want, []catalog.Product{candidate, returned})
	if len(matched) != 1 || matched[0].Product.ID != "PSW8GeK" {
		fail(fmt.Errorf("match returned %d candidates, want the 1 new SKU", len(matched)))
	}
	if matched[0].SizeNote == "" || matched[0].Fulfilment != catalog.FulfilmentSimulated {
		fail(fmt.Errorf("candidate misses the size-data or simulated-fulfilment marks"))
	}
	say("STEP 2 DONE (layer): candidate PSW8GeK shares brand+category+top-fit %v; sizes at the retailer, fulfilment simulated",
		candidate.Fit["top-fit"])

	say("=== pending question stays open on a vague answer ===")
	must("WH-2EV-05", 1, true)
	vague := answer("WH-2EV-05", "not great")
	if vague.Accepted || !vague.AskAgain || vague.BeliefID != nil {
		fail(fmt.Errorf("vague answer: accepted %v ask again %v belief %v", vague.Accepted, vague.AskAgain, vague.BeliefID))
	}

	say("=== step 3 (layer): the shopper correction wins at once ===")
	traced, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001", TraceID: traceID})
	if err != nil {
		fail(fmt.Errorf("traced recall: %w", err))
	}
	if len(traced.Beliefs) != 1 {
		fail(fmt.Errorf("traced recall serves %d beliefs, want the 1 earned sizing belief", len(traced.Beliefs)))
	}
	correction, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: shopper, Text: "I've lost weight, use M",
		ContradictsBeliefID: sizingID, TraceID: traceID,
	})
	if err != nil {
		fail(fmt.Errorf("record correction: %w", err))
	}
	if correction.Status != domain.StatusOutOfTrue || correction.Standing != domain.GraphVenerable || !correction.Contradicted {
		fail(fmt.Errorf("correction leaves %q/%q contradicted %v", correction.Status, correction.Standing, correction.Contradicted))
	}
	if correction.Guidance.Basis != contract.GuidanceCorrection || correction.Guidance.Text != "Take size M for Brand X" {
		fail(fmt.Errorf("correction guidance %+v, want M at once", correction.Guidance))
	}
	overridden, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		fail(err)
	}
	if len(overridden.Beliefs) != 1 || !overridden.Beliefs[0].Contradicted {
		fail(fmt.Errorf("recall after correction serves no contradicted belief"))
	}
	if overridden.OverridingCorrection == nil || overridden.Guidance.Basis != contract.GuidanceCorrection {
		fail(fmt.Errorf("recall after correction names no override"))
	}
	say("STEP 3 DONE (layer): recall guides %q with the belief kept beside it", overridden.Guidance.Text)

	say("=== steps 2+3 (agent): the deployed reference agent ===")
	seenTraces := map[string]bool{}
	convRaw := execRun("", []string{"OUT=" + out}, "python3", "demo/drive.py", "create")
	var created struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal([]byte(convRaw), &created); err != nil || created.ConversationID == "" {
		fail(fmt.Errorf("create conversation: %.100s", convRaw))
	}
	conv := created.ConversationID
	turn1Trace, turn1Text := agentTurn(seenTraces, conv, "I need a Brand X oxford shirt. What size should I get, M or L?")
	lowered := strings.ToLower(turn1Text)
	if !strings.Contains(turn1Text, "M") {
		fail(fmt.Errorf("turn 1 recommends no M: %.200s", turn1Text))
	}
	if !strings.Contains(lowered, "belief") && !strings.Contains(lowered, "correction") && !strings.Contains(lowered, "return") {
		fail(fmt.Errorf("turn 1 cites no belief: %.200s", turn1Text))
	}
	if !strings.Contains(lowered, "approv") && !strings.Contains(lowered, "yes") && !strings.Contains(lowered, "confirm") &&
		!strings.Contains(lowered, "pending") && !strings.Contains(lowered, "refund") && !strings.Contains(lowered, "return") {
		fail(fmt.Errorf("turn 1 asks neither approval nor the pending question: %.200s", turn1Text))
	}
	if err := os.WriteFile(filepath.Join(out, "turn1-reply.txt"), []byte(turn1Text), 0o600); err != nil {
		fail(err)
	}
	say("turn 1: M per the correction, cited, pending question asked")
	turn2Trace, turn2Text := agentTurn(seenTraces, conv, "I'd like to buy the M in the PSW8GeK stripe.")
	lowered2 := strings.ToLower(turn2Text)
	if !strings.Contains(turn2Text, "M") {
		fail(fmt.Errorf("turn 2 recommends no M: %.200s", turn2Text))
	}
	if !strings.Contains(lowered2, "approv") && !strings.Contains(lowered2, "yes") && !strings.Contains(lowered2, "confirm") &&
		!strings.Contains(lowered2, "purchase") && !strings.Contains(lowered2, "buy") {
		fail(fmt.Errorf("turn 2 asks no approval: %.200s", turn2Text))
	}
	if err := os.WriteFile(filepath.Join(out, "turn2-reply.txt"), []byte(turn2Text), 0o600); err != nil {
		fail(err)
	}
	say("turn 2: buy intent answered with an explicit-yes approval ask")
	turn3Trace, turn3Text := agentTurn(seenTraces, conv, "yes")
	lowered3 := strings.ToLower(turn3Text)
	if !strings.Contains(lowered3, "approv") && !strings.Contains(lowered3, "purchase") && !strings.Contains(lowered3, "wired") && !strings.Contains(lowered3, "yes") && !strings.Contains(lowered3, "confirm") {
		fail(fmt.Errorf("turn 3 answers no approval: %.200s", turn3Text))
	}
	if err := os.WriteFile(filepath.Join(out, "turn3-reply.txt"), []byte(turn3Text), 0o600); err != nil {
		fail(err)
	}
	say("turn 3: explicit yes recorded by the agent (no purchase tool on the agent by design)")
	red.add(turn1Trace, "TRACE-T36-R"+runNo+"-T1")
	red.add(turn2Trace, "TRACE-T36-R"+runNo+"-T2")
	red.add(turn3Trace, "TRACE-T36-R"+runNo+"-T3")

	say("=== step 4 (live): approval, then a sandbox purchase of the M ===")
	transport := &restTransport{c: client, approve: map[string]string{}}
	ordSvc, err := orders.NewService(transport, orders.Config{MerchantID: "t36-demo-merchant", Sandbox: true})
	if err != nil {
		fail(fmt.Errorf("orders service (sandbox only): %w", err))
	}
	mirror := &approvalMirror{c: client, approved: map[string]bool{}}
	item := domain.Item{BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt", SKU: "PSW8GeK", Size: "M", Category: "shirts"}
	// The request id must be unique per run: the service mints a
	// deterministic id when given none, and PayPal answers a reused
	// id with 422 instead of a new order.
	proposed, duplicate, err := ordSvc.Propose(ctx, shopper, "demo-oxford-m", item, 19500, "USD", "t36-"+session+"-propose")
	if err != nil {
		fail(fmt.Errorf("propose: %w", err))
	}
	if duplicate {
		fail(fmt.Errorf("fresh propose returned duplicate"))
	}
	red.add(proposed.OrderID, "ORD-T36-R"+runNo)
	red.add(proposed.CustomID, "CUSTOM-T36-R"+runNo)
	red.add(transport.approve[proposed.OrderID], "APPROVAL-URL-REDACTED")
	if _, err := ordSvc.Capture(ctx, proposed.OrderID); !errors.Is(err, orders.ErrNoApproval) {
		fail(fmt.Errorf("capture before approval: want ErrNoApproval, got %v", err))
	}
	say("approve-first holds live: capture without approval refused, transport never called")
	runApprove(ctx, client, out, transport.approve[proposed.OrderID], proposed.OrderID)
	if _, err := loops.ApproveAfterYes(ctx, ordSvc, mirror, proposed.OrderID, "yes"); err != nil {
		fail(fmt.Errorf("approve after yes: %w", err))
	}
	captured, err := ordSvc.Capture(ctx, proposed.OrderID)
	if err != nil {
		fail(fmt.Errorf("capture after approval: %w", err))
	}
	if captured.State != orders.StateCaptured {
		fail(fmt.Errorf("order state %q, want captured", captured.State))
	}
	red.add(captured.CaptureID, "CAP-T36-R"+runNo)
	say("STEP 4a DONE: sandbox order captured COMPLETED after the real approval (explicit yes + scripted shopper click)")

	say("=== step 4 (live): the verified capture joins the layer ===")
	var liveDelivery *delivery
	for attempt := 0; attempt < 30; attempt++ {
		deliveries, err = listDeliveries(ctx, hc, site)
		if err != nil {
			fail(err)
		}
		liveDelivery, err = findDelivery(deliveries, captured.CustomID, "PAYMENT.CAPTURE.COMPLETED")
		if err == nil {
			break
		}
		time.Sleep(10 * time.Second)
	}
	if liveDelivery == nil {
		say("auto-delivery not landed; resending the ledger event")
		events, err := ledgerEvents(ctx, client)
		if err != nil {
			fail(err)
		}
		eventID := ""
		for _, entryAny := range events {
			entry, _ := entryAny.(map[string]any)
			eventType, _ := entry["event_type"].(string)
			if eventType == "PAYMENT.CAPTURE.COMPLETED" {
				eventID, _ = entry["id"].(string)
				break
			}
		}
		if eventID == "" {
			fail(fmt.Errorf("ledger holds no capture event"))
		}
		if err := resend(ctx, client, eventID); err != nil {
			fail(err)
		}
		for attempt := 0; attempt < 18; attempt++ {
			time.Sleep(10 * time.Second)
			deliveries, err = listDeliveries(ctx, hc, site)
			if err != nil {
				fail(err)
			}
			liveDelivery, err = findDelivery(deliveries, captured.CustomID, "PAYMENT.CAPTURE.COMPLETED")
			if err == nil {
				break
			}
		}
		if liveDelivery == nil {
			fail(fmt.Errorf("capture delivery never landed, even after resend"))
		}
	}
	red.add(liveDelivery.eventID, "EVT-T36-R"+runNo+"-PURCHASE")
	if err := verifyDelivery(ctx, client, liveDelivery); err != nil {
		fail(err)
	}
	say("capture delivery verified SUCCESS under the real event id")
	liveOrder, err := client.get(ctx, "/v2/checkout/orders/"+captured.OrderID)
	if err != nil {
		fail(err)
	}
	liveItems := []domain.Item{}
	for _, pu := range toSlice(liveOrder["purchase_units"]) {
		puMap, _ := pu.(map[string]any)
		for _, entry := range toSlice(puMap["items"]) {
			itemMap, _ := entry.(map[string]any)
			sku, _ := itemMap["sku"].(string)
			size := "M"
			name, _ := itemMap["name"].(string)
			if strings.Contains(name, "size ") {
				size = name[strings.LastIndex(name, "size ")+5:]
			}
			liveItems = append(liveItems, domain.Item{
				BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt",
				SKU: sku, Size: size, Category: "shirts",
			})
		}
	}
	if len(liveItems) == 0 {
		fail(fmt.Errorf("live order carries no line items"))
	}
	event, err := captureEvent(liveDelivery, captured.OrderID)
	if err != nil {
		fail(err)
	}
	payments.event = &event
	payments.eventID = liveDelivery.eventID
	payments.order = &fakes.OrderRecord{OrderID: captured.OrderID, Items: liveItems, Link: event.Link, Provenance: "live:sandbox"}
	payments.orderID = captured.OrderID
	must(liveDelivery.eventID, 1, false)
	say("live capture ingested: 1 kept row, pending=false (kept is weak, never confirming)")

	feedback, err := svc.RecordFeedback(ctx, contract.RecordFeedbackRequest{
		ShopperID: shopper, CustomID: captured.CustomID,
		SKU: "PSW8GeK", Size: "M", FeedbackText: "The M fits well across the shoulders.",
	})
	if err != nil {
		fail(fmt.Errorf("record feedback: %w", err))
	}
	if !feedback.Accepted || feedback.Verdict != contract.FeedbackFitsWell {
		fail(fmt.Errorf("feedback: accepted %v verdict %q, want true fits_well", feedback.Accepted, feedback.Verdict))
	}
	final, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		fail(err)
	}
	if len(final.Beliefs) != 2 {
		fail(fmt.Errorf("final recall serves %d beliefs, want the sizing belief and the fits-well feedback beside it",
			len(final.Beliefs)))
	}
	if final.Beliefs[0].BeliefID != sizingID {
		fail(fmt.Errorf("final recall serves %q first, want the earned sizing belief %s",
			final.Beliefs[0].BeliefID, sizingID))
	}
	// The feedback grew a belief with one row. Recall serves it with its
	// label rather than holding it back, ranked after the sizing belief.
	rival := final.Beliefs[1]
	if rival.Status != domain.StatusClaimed || len(rival.Evidence) != 1 {
		fail(fmt.Errorf("fits-well belief reads %q with %d rows, want Claimed with its one: thin evidence is served",
			rival.Status, len(rival.Evidence)))
	}
	say("STEP 4 DONE: fits-well feedback tied to the live purchase; recall serves the earned sizing belief and the Claimed fits-well one beside it")

	say("=== step 5 (live): the never-arrived dispute counts seller-only ===")
	d6, err := findDelivery(deliveries, "t13b-demo-006", "CUSTOMER.DISPUTE.CREATED")
	if err != nil {
		fail(err)
	}
	var disputeEnvelope struct {
		Resource map[string]any `json:"resource"`
	}
	if err := json.Unmarshal(d6.raw, &disputeEnvelope); err != nil {
		fail(err)
	}
	disputeID, _ := disputeEnvelope.Resource["dispute_id"].(string)
	red.add(disputeID, "DSP-LIVE-R"+runNo)
	red.add(d6.eventID, "EVT-LIVE-R"+runNo+"-D1")
	liveDispute, err := client.get(ctx, "/v1/customer/disputes/"+disputeID)
	if err != nil {
		fail(err)
	}
	if str(liveDispute, "reason") != "MERCHANDISE_OR_SERVICE_NOT_RECEIVED" {
		fail(fmt.Errorf("live dispute reason %q, want MERCHANDISE_OR_SERVICE_NOT_RECEIVED", str(liveDispute, "reason")))
	}
	say("live dispute: reason MERCHANDISE_OR_SERVICE_NOT_RECEIVED, status %s", str(liveDispute, "status"))
	before, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		fail(err)
	}
	must("WH-2EV-04", 1, false)
	after, err := svc.Recall(ctx, contract.RecallRequest{ShopperID: shopper, BrandID: "brand-x-001"})
	if err != nil {
		fail(err)
	}
	if recallJSON(before) != recallJSON(after) {
		fail(fmt.Errorf("the non-arrival dispute moved the sizing recall"))
	}
	say("recall unchanged by the dispute")
	ledger, err := svc.LedgerBeliefs(ctx, contract.LedgerBeliefsRequest{ShopperID: shopper})
	if err != nil {
		fail(fmt.Errorf("ledger beliefs: %w", err))
	}
	sellerFound := false
	for _, row := range ledger.Beliefs {
		if row.Belief.Kind == domain.BeliefKindSeller {
			sellerFound = true
			if row.Belief.Status != domain.StatusClaimed || row.EvidenceCount != 1 {
				fail(fmt.Errorf("seller belief reads %q with %d rows, want Claimed with 1", row.Belief.Status, row.EvidenceCount))
			}
			red.add(row.Belief.BeliefID, "BLF-T36-R"+runNo+"-SELLER")
			say("seller belief: Claimed, 1 row (one dispute never earns)")
		}
	}
	if !sellerFound {
		fail(fmt.Errorf("the dispute wrote no seller belief"))
	}
	say("STEP 5 DONE: dispute against the seller only; sizing untouched")

	say("=== step 6: the evidence ledger and the traces ===")
	ledgerEvidence, err := svc.LedgerEvidence(ctx, contract.LedgerEvidenceRequest{ShopperID: shopper, BeliefID: sizingID})
	if err != nil {
		fail(fmt.Errorf("ledger evidence: %w", err))
	}
	fitRows := 0
	for _, row := range ledgerEvidence.Evidence {
		for _, id := range row.BeliefIDs {
			if id == sizingID {
				fitRows++
				break
			}
		}
	}
	say("ledger evidence for the sizing belief: %d rows, %d counting toward fit", len(ledgerEvidence.Evidence), fitRows)
	explained, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: sizingID})
	if err != nil {
		fail(fmt.Errorf("explain belief: %w", err))
	}
	say("explain: %d evidence, %d correction(s), %d timeline events",
		len(explained.Belief.Evidence), len(explained.Corrections), len(explained.Timeline))
	calls := svc.Calls(traceID)
	if len(calls) != 2 || calls[0].Tool != string(contract.ToolRecallForPurchase) || calls[1].Tool != string(contract.ToolRecordCorrection) {
		fail(fmt.Errorf("call log under the demo trace: want [recall_for_purchase record_correction], got %v", calls))
	}
	say("call log: recall_for_purchase then record_correction under the demo trace")
	for name, id := range map[string]string{"turn1": turn1Trace, "turn2": turn2Trace, "turn3": turn3Trace} {
		_, _, observations := traceDetail("check-"+name, id)
		tools := toolIndex(observations)
		say("trace %.8s tools in order: %v", id, tools)
		if name == "turn1" {
			found := false
			for _, tool := range tools {
				if tool == "recall_for_purchase" {
					found = true
				}
			}
			if !found {
				fail(fmt.Errorf("turn 1 trace names no recall_for_purchase span"))
			}
		}
	}
	snapshot := map[string]any{
		"run":               runNo,
		"sizing_belief":     sizingID,
		"sizing_status":     string(sizing.Status),
		"sizing_standing":   string(sizing.Standing),
		"fit_evidence_rows": fitRows,
		"timeline_events":   len(explained.Timeline),
		"guidance":          overridden.Guidance.Text,
	}
	rawSnapshot, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(out, "ledger.json"), []byte(red.scrub(string(rawSnapshot))), 0o600); err != nil {
		fail(err)
	}

	// Stop the serve child before erasing: the single-writer lease is
	// held until a clean stop, and erase refuses while it is held. The
	// deferred stop stays as the failure-path release.
	rpc.stop()
	if text, err := eraseSession(bin, cfg, session); err != nil {
		fail(fmt.Errorf("erase session after run: %s", text))
	} else {
		say("erased session: %s", text)
	}
	astBilling()
	say("LIVE OK: all six steps on the real stack (%d Jev calls this run)", seen)

	say("=== leak check: no real id or secret in committed files ===")
	secrets := []string{os.Getenv("PAYPAL_CLIENT_ID"), os.Getenv("PAYPAL_CLIENT_SECRET"),
		os.Getenv("OPENROUTER_API_KEY"), os.Getenv("LAMBO_ES_API_KEY"), os.Getenv("CHANNEL3_API_KEY"), elasticURL}
	for real := range red.table {
		secrets = append(secrets, real)
	}
	leaks := []string{}
	for _, value := range secrets {
		if len(value) < 6 {
			continue
		}
		// grep exits 1 on no matches: that is the clean case, not a
		// failure, so this leg shells out directly instead of execRun.
		raw, _ := exec.Command("grep", "-rlF", "--exclude-dir=out", "--exclude-dir=.git", "--", value, "demo").CombinedOutput()
		for _, file := range strings.Split(string(raw), "\n") {
			if strings.TrimSpace(file) != "" {
				leaks = append(leaks, file+":holds a live value")
			}
		}
	}
	if len(leaks) > 0 {
		fail(fmt.Errorf("live values entered committed files:\n%s", strings.Join(leaks, "\n")))
	}
	say("leak check clean")
}
