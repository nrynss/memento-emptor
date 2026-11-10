// Live driver for T8.5b: the served layer end to end, through the hub
// and a test-prefixed fit index.
//
// One hub (the T8.1 shape: one pinned session, `max_attached = 16`, the
// `shopper-` on-demand credential, the fixture embedder and the live
// recall tier in test-prefixed indices `t85b-*`) holds one session per
// layer shopper. One server.Service per shopper wires that shopper's
// session as its graph, the live pool reader and the live fit writer over
// the test-prefixed index `t85b-fit-rows`, the identity salt, a static
// line resolver and a recording classifier (no model spend: every answer
// replays a labelled Jev recording). The driver then decides one purchase
// per precedence step:
//
//   - correction: a standing correction recorded with brand context
//     decides the next recall, and still decides it over a fresh hub MCP
//     session to the same shopper session and after a layer restart;
//   - own evidence: three too-large returns earn a belief that beats
//     the disagreeing Earned-small pool, which rides along overridden;
//   - pool: a new shopper on the seeded Earned-small line is decided by
//     the pool, with the seeded split riding along;
//   - statement: a seeded general statement decides for a shopper with no
//     other memory;
//   - nothing: a fresh shopper on a lineless recall gets GuidanceNone,
//     never a refusal; a full-line recall on the empty line is attempted
//     too, pinning the consulted-but-empty failure the T8.5 review left
//     in the assembler.
//
// A sixth shopper pins the dangling-view behaviour the T8.5 review found:
// a declared correction survives a layer restart with its stale belief id
// and degrades to unscoped, and rebuilding the belief does not re-link
// it. The fix (re-linking by content match) needs internal/api/server/,
// outside this task's owns, so the driver pins the behaviour and names
// T8.5 as the owner.
//
// This is a main program, never a test: no *_test.go lives under
// tests/live/, so CI builds this package but never runs it. Run it by hand
// with the keys loaded:
//
//	source tools/env.sh
//	LAMBO_BIN=/path/to/lambo-0.10.1 bash tests/live/precedence/run.sh
//
// The deployment URL and the keys never reach stdout, a file or an error
// message: the transcript names the deployment as <deployment>, and the
// driver scrubs tool output before it prints.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/api/server"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
	"github.com/nrynss/memento-emptor/internal/pool"
)

// The test namespaces. Every hub recall index and the fit index live
// under them, and the driver refuses to run when either prefix holds
// anything else, so it never deletes an index it did not create.
const (
	recallPrefix = "t85b-"
	fitIndexName = recallPrefix + "fit-rows"
)

// agentName is the MCP agent identity this driver writes as, on the hub
// and in the layer's call log.
const agentName = "memento-t85b"

// tokenEnv is the hub credential's token variable, read by the hub config.
// The value is minted per run and reaches the child through the
// environment only: it is never written to a file or printed.
const tokenEnv = "LAMBO_HUB_TOKEN"

// responseBytes bounds how much of a response the driver reads.
const responseBytes = 1 << 20

// waitMS is the receipt wait per hub write: the daemon applies the write
// in the background and the ack carries a receipt.
const waitMS = 10000

// fail is the driver's one error shape: a check that did not hold.
type fail struct{ msg string }

func (e *fail) Error() string { return e.msg }

func check(cond bool, format string, args ...any) error {
	if !cond {
		return &fail{fmt.Sprintf(format, args...)}
	}
	return nil
}

// scrubber replaces the deployment locator and the secrets with
// placeholders before anything is printed.
type scrubber struct {
	secrets []string
	host    string
}

func newScrubber(env map[string]string) *scrubber {
	s := &scrubber{}
	for _, v := range env {
		if v != "" {
			s.secrets = append(s.secrets, v)
		}
	}
	if u, err := url.Parse(env["ELASTIC_URL"]); err == nil && u.Hostname() != "" {
		s.host = u.Hostname()
	}
	return s
}

func (s *scrubber) clean(text string) string {
	for _, secret := range s.secrets {
		text = strings.ReplaceAll(text, secret, "<redacted>")
	}
	if s.host != "" {
		text = strings.ReplaceAll(text, s.host, "<deployment-host>")
	}
	if u := os.Getenv("ELASTIC_URL"); u != "" {
		text = strings.ReplaceAll(text, u, "<deployment>")
	}
	return text
}

// es is the driver's read and cleanup client for the deployment. Writes
// go through the hub's recall tier and the fit index; this client only
// lists, counts and deletes the test prefixes.
type es struct {
	base   string
	apiKey string
	client *http.Client
}

func newES(baseURL, apiKey string) *es {
	return &es{
		base:   strings.TrimRight(baseURL, "/"),
		apiKey: apiKey,
		client: &http.Client{Timeout: 90 * time.Second},
	}
}

func (e *es) do(ctx context.Context, method, path string) (int, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, e.base+path, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Authorization", "ApiKey "+e.apiKey)
	response, err := e.client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, responseBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("read response: %w", err)
	}
	return response.StatusCode, raw, nil
}

// indices lists the index names under one test prefix.
func (e *es) indices(ctx context.Context, prefix string) ([]string, error) {
	status, body, err := e.do(ctx, http.MethodGet, "/_cat/indices/"+prefix+"*?format=json&h=index&expand_wildcards=all")
	if err != nil {
		return nil, fmt.Errorf("list %s indices: %w", prefix, err)
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("list %s indices: status %d", prefix, status)
	}
	var rows []struct {
		Index string `json:"index"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("read %s index list: %w", prefix, err)
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Index)
	}
	return names, nil
}

// deleteIndex removes one index by name.
func (e *es) deleteIndex(ctx context.Context, name string) error {
	status, _, err := e.do(ctx, http.MethodDelete, "/"+name)
	if err != nil {
		return fmt.Errorf("delete index: %w", err)
	}
	if status != http.StatusOK && status != http.StatusNotFound {
		return fmt.Errorf("delete index: status %d", status)
	}
	return nil
}

// proc is a supervised lambo child whose stderr is scanned for its port.
type proc struct {
	cmd     *exec.Cmd
	logPath string
	log     *os.File
	port    int
}

var portRE = regexp.MustCompile(`127\.0\.0\.1:(\d+)`)

func startProc(binary string, argv []string, env []string, logPath string) (*proc, error) {
	log, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	cmd := exec.Command(binary, argv...)
	cmd.Env = env
	cmd.Stdin = nil
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		return nil, fmt.Errorf("start %s: %w", argv[0], err)
	}
	p := &proc{cmd: cmd, logPath: logPath, log: log}
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			p.log.Sync()
			return nil, fmt.Errorf("process exited before listening: %s", tail(logPath))
		}
		if m := portRE.FindStringSubmatch(string(readFile(logPath))); m != nil {
			var port int
			if _, err := fmt.Sscanf(m[1], "%d", &port); err == nil {
				p.port = port
				return p, nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = p.stop()
	return nil, fmt.Errorf("no listen address in %s", tail(logPath))
}

func readFile(path string) []byte {
	raw, _ := os.ReadFile(path)
	return raw
}

func tail(path string) string {
	raw := readFile(path)
	if len(raw) > 2000 {
		raw = raw[len(raw)-2000:]
	}
	return string(raw)
}

func (p *proc) stop() error {
	if p.cmd.Process == nil {
		return nil
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(45 * time.Second):
		_ = p.cmd.Process.Kill()
		<-done
	}
	_ = p.log.Close()
	return nil
}

// mcp is one JSON-RPC session over streamable HTTP at /mcp/s/<session>.
type mcp struct {
	host       string
	port       int
	path       string
	token      string
	sessionID  string
	nextID     int
	httpClient *http.Client
}

func newMCP(port int, session, token string) *mcp {
	return &mcp{
		host:       "127.0.0.1",
		port:       port,
		path:       "/mcp/s/" + session,
		token:      token,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *mcp) post(body map[string]any) (int, http.Header, []byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, nil, nil, err
	}
	request, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("http://%s:%d%s", c.host, c.port, c.path),
		bytes.NewReader(payload))
	if err != nil {
		return 0, nil, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer "+c.token)
	if c.sessionID != "" {
		request.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, nil, nil, err
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, responseBytes))
	if err != nil {
		return 0, nil, nil, fmt.Errorf("read response: %w", err)
	}
	return response.StatusCode, response.Header, raw, nil
}

// decode unwraps a JSON-RPC reply from a JSON or an SSE body into the
// result envelope.
func decode(contentType string, raw []byte) (map[string]any, error) {
	text := strings.TrimSpace(string(raw))
	if strings.Contains(contentType, "text/event-stream") {
		var frames []string
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "data:") {
				frame := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if frame != "" && frame != "[DONE]" {
					frames = append(frames, frame)
				}
			}
		}
		if len(frames) == 0 {
			return nil, fmt.Errorf("no data frame in SSE body")
		}
		var reply map[string]any
		if err := json.Unmarshal([]byte(frames[len(frames)-1]), &reply); err != nil {
			return nil, fmt.Errorf("decode SSE frame: %w", err)
		}
		return reply, nil
	}
	var reply map[string]any
	if err := json.Unmarshal(raw, &reply); err != nil {
		return nil, fmt.Errorf("decode reply: %w", err)
	}
	return reply, nil
}

// resultOf returns the result envelope of a tools/call reply, failing on
// a protocol error.
func resultOf(reply map[string]any, tool string) (map[string]any, error) {
	if failure, ok := reply["error"]; ok {
		return nil, fmt.Errorf("%s: %v", tool, failure)
	}
	result, _ := reply["result"].(map[string]any)
	if result == nil {
		return nil, fmt.Errorf("%s: no result envelope", tool)
	}
	return result, nil
}

// texts joins the result's text content blocks.
func texts(result map[string]any) string {
	var out []string
	content, _ := result["content"].([]any)
	for _, item := range content {
		if block, ok := item.(map[string]any); ok {
			if text, ok := block["text"].(string); ok {
				out = append(out, text)
			}
		}
	}
	return strings.Join(out, "\n")
}

func (c *mcp) initialize() error {
	c.nextID++
	status, headers, raw, err := c.post(map[string]any{
		"jsonrpc": "2.0",
		"id":      c.nextID,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "memento-t85b", "version": "1"},
		},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("initialize: HTTP %d", status)
	}
	c.sessionID = headers.Get("Mcp-Session-Id")
	if c.sessionID == "" {
		return fmt.Errorf("initialize: no Mcp-Session-Id header")
	}
	if _, err := decode(headers.Get("Content-Type"), raw); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	c.nextID++
	status, _, _, err = c.post(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	if err != nil {
		return fmt.Errorf("initialized notification: %w", err)
	}
	if status != 200 && status != 202 && status != 204 {
		return fmt.Errorf("initialized notification: HTTP %d", status)
	}
	return nil
}

// call makes one tools/call and returns its result envelope.
func (c *mcp) call(name string, args map[string]any) (map[string]any, error) {
	args = mapsCopy(args)
	args["agent_id"] = agentName
	c.nextID++
	status, headers, raw, err := c.post(map[string]any{
		"jsonrpc": "2.0",
		"id":      c.nextID,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d: %s", name, status, excerpt(raw))
	}
	reply, err := decode(headers.Get("Content-Type"), raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return resultOf(reply, name)
}

func mapsCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}

func excerpt(raw []byte) string {
	const width = 200
	if len(raw) <= width {
		return string(raw)
	}
	return string(raw[:width]) + fmt.Sprintf("… (%d bytes)", len(raw))
}

func excerptStr(text string) string {
	const width = 300
	if len(text) <= width {
		return text
	}
	return text[:width] + "…"
}

var receiptRE = regexp.MustCompile(`receipt (lwr[0-9a-z.]+)`)

// receiptOf reads the write receipt from a derive or record_action
// result: the structured receipt first, else the text the hub prints.
func receiptOf(result map[string]any) (string, error) {
	if structured, ok := result["structuredContent"].(map[string]any); ok {
		if receipt, ok := structured["receipt"].(string); ok && receipt != "" {
			return receipt, nil
		}
	}
	if m := receiptRE.FindStringSubmatch(texts(result)); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("write issued no receipt")
}

// lamboGraph is a fakes.EvidenceGraph over one live hub session. Event
// times travel with every write so honestly spread history promotes;
// corrections cite against, never depends_on. It waits every write's
// receipt until the daemon applies it.
type lamboGraph struct {
	client *mcp
}

// Compile-time proof the live client satisfies the seam the service
// builds on.
var _ fakes.EvidenceGraph = (*lamboGraph)(nil)

// waitApplied blocks until the daemon applies one receipt.
func (g *lamboGraph) waitApplied(receipt, what string) error {
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		result, err := g.client.call("lambo_stats", map[string]any{
			"receipt": receipt, "wait_ms": waitMS,
		})
		if err != nil {
			return fmt.Errorf("wait %s: %w", what, err)
		}
		text := texts(result)
		switch {
		case strings.Contains(text, "applied"):
			return nil
		case strings.Contains(text, "failed"), strings.Contains(text, "refused"):
			return &fail{fmt.Sprintf("receipt for %s not applied: %.300s", what, text)}
		}
	}
	return &fail{fmt.Sprintf("receipt for %s did not settle", what)}
}

// Derive records one belief or statement.
func (g *lamboGraph) Derive(ctx context.Context, content, kind string, eventTime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := map[string]any{
		"concepts": []map[string]any{{"content": content, "concept_type": kind}},
	}
	if !eventTime.IsZero() {
		args["event_time"] = eventTime.UTC().Format(time.RFC3339Nano)
	}
	result, err := g.client.call("lambo_derive", args)
	if err != nil {
		return err
	}
	receipt, err := receiptOf(result)
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
	args := map[string]any{"action": action}
	if len(dependsOn) > 0 {
		args["depends_on"] = dependsOn
	}
	if len(against) > 0 {
		args["against"] = against
	}
	if !eventTime.IsZero() {
		args["event_time"] = eventTime.UTC().Format(time.RFC3339Nano)
	}
	result, err := g.client.call("lambo_record_action", args)
	if err != nil {
		return err
	}
	receipt, err := receiptOf(result)
	if err != nil {
		return err
	}
	return g.waitApplied(receipt, fmt.Sprintf("record_action %.40q", action))
}

// Recall lists the live session's hit labels for one query. The service
// ranks from its stored beliefs and never calls this; it exists so the
// client satisfies the seam honestly.
func (g *lamboGraph) Recall(ctx context.Context, query string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := g.client.call("lambo_recall", map[string]any{"query": query})
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, line := range strings.Split(texts(result), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out, nil
}

// navigate walks a decoded result envelope by key.
func navigate(node any, keys ...string) any {
	for _, key := range keys {
		obj, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		node = obj[key]
	}
	return node
}

// Inspect reports the promotion state of one belief. A focus nothing
// matches reads as None with no error: the layer only inspects contents
// it derived, and an unpromoted note is None, not a failure.
func (g *lamboGraph) Inspect(ctx context.Context, content string) (fakes.BeliefReport, error) {
	if err := ctx.Err(); err != nil {
		return fakes.BeliefReport{}, err
	}
	result, err := g.client.call("lambo_inspect", map[string]any{
		"focus": content, "depth": 2,
	})
	if err != nil {
		if strings.Contains(err.Error(), "no concept matching") {
			return fakes.BeliefReport{Content: content}, nil
		}
		return fakes.BeliefReport{}, err
	}
	if strings.Contains(texts(result), "no concept matching") {
		return fakes.BeliefReport{Content: content}, nil
	}
	report := fakes.BeliefReport{Content: content}
	switch strings.ToLower(fmt.Sprint(navigate(result, "structuredContent", "admission", "status"))) {
	case "venerable":
		report.State = domain.GraphVenerable
	case "candidate":
		report.State = domain.GraphCandidate
	default:
		report.State = domain.GraphNone
	}
	if checks, ok := navigate(result, "structuredContent", "admission", "checks").([]any); ok {
		for _, item := range checks {
			check, _ := item.(map[string]any)
			gate, _ := check["gate"].(string)
			current, _ := check["current"].(float64)
			switch gate {
			case "distinct_interactions":
				report.DistinctInteractions = int(current)
			case "coverage":
				report.Coverage = current
			}
		}
	}
	if radius, ok := navigate(result, "structuredContent", "focus", "blast_radius").(float64); ok {
		report.Dependents = int(radius)
	}
	if warnings, ok := navigate(result, "structuredContent", "warnings").([]any); ok && len(warnings) > 0 {
		report.Contradicted = true
		var words []string
		for _, warning := range warnings {
			words = append(words, fmt.Sprint(warning))
		}
		report.Warning = "Contradicted by " + strings.Join(words, "; ")
	}
	return report, nil
}

// hubConfig writes the T8.1 serve table: one pinned session, the
// on-demand credential, the fixture embedder and the live recall tier.
func hubConfig(path, store, pinned string, maxAttached int, elasticURL string) error {
	text := "# Written at boot by tests/live/precedence: the T8.1 shape, test-scoped.\n" +
		"[store]\nkind = \"sqlite\"\npath = \"" + store + "\"\n\n" +
		"[embedder]\nkind = \"fixture\"\ndim = 1024\ncalibration = \"off\"\n\n" +
		"[daemon]\ngc_interval = 1\ngc_idle_floor = 1\ngc_max_interval_secs = 3600\n" +
		"canonization_eval_interval_secs = 5\n\n" +
		"[recall]\nkind = \"elastic\"\nurl = \"" + elasticURL + "\"\n" +
		"api_key = { env = \"LAMBO_ES_API_KEY\" }\nindex_prefix = \"" + recallPrefix + "\"\n" +
		"refresh = \"true\"\ntimeout_ms = 20000\n\n" +
		"[serve]\nsessions = [\"" + pinned + "\"]\ndefault_session = \"" + pinned + "\"\n" +
		fmt.Sprintf("max_attached = %d\n", maxAttached) +
		"attach_concurrency = 1\nidle_detach_secs = 900\n\n" +
		"[[serve.credential]]\nname = \"memento\"\ntoken_env = \"" + tokenEnv + "\"\n" +
		"session_prefix = \"shopper-\"\ncreate = true\n\n" +
		"[web]\nlist_sessions = true\n"
	return os.WriteFile(path, []byte(text), 0600)
}

// layerContract is the store's embedding contract. It carries no model
// name, dimensions, endpoint or key — only its kind, so the lexical
// store stands in for the live Elastic proof beside this driver.
type layerContract struct{ kind string }

// EmbeddingKind names the contract family.
func (c layerContract) EmbeddingKind() string { return c.kind }

// Validate reports whether the contract is usable.
func (c layerContract) Validate() error {
	if c.kind == "" {
		return errors.New("t85b-live: contract names no kind")
	}
	return nil
}

// driver holds the run's shared state.
type driver struct {
	ctx        context.Context
	binary     string
	elastic    string
	apiKey     string
	salt       string
	store      *es
	token      string
	scrub      *scrubber
	runDir     string
	standing   string
	fitRows    fitindex.Index
	poolIdx    pool.Index
	keyer      *identity.Keyer
	classifier *fakes.RecordingReasonClassifier
	hubPort    int
}

func (d *driver) printf(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

// say prints a transcript line with tool output scrubbed.
func (d *driver) say(format string, args ...any) {
	d.printf(d.scrub.clean(fmt.Sprintf(format, args...)))
}

func required(name string) (string, error) {
	if value := os.Getenv(name); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("%s is not set; run `source tools/env.sh` first", name)
}

// attach opens one hub MCP session, creating the shopper's session on
// demand through the prefix credential.
func (d *driver) attach(session string) (*mcp, error) {
	client := newMCP(d.hubPort, session, d.token)
	if err := client.initialize(); err != nil {
		return nil, fmt.Errorf("attach %s: %w", session, err)
	}
	return client, nil
}

// staticLines resolves every order item to its brand and category with
// the regular fit handle: the tests' catalog, which the demo path leaves
// unwired. One source for every writer call, as T8.5 requires.
func staticLines(_ context.Context, item domain.Item) (domain.ProductLine, error) {
	return domain.ProductLine{Brand: item.BrandID, Category: item.Category, FitHandle: "regular"}, nil
}

// service builds one shopper's served layer over one hub session: the
// live pool reader and the live fit writer, the identity salt, the
// static line resolver, the recording classifier and the recording
// payment source. The standing directory is shared, so a rebuilt service
// over the same directory restores the shopper's journal.
func (d *driver) service(client *mcp, shopper string) (*server.Service, error) {
	store, err := evidence.NewStore(layerContract{kind: "t85b-live"})
	if err != nil {
		return nil, fmt.Errorf("open evidence store: %w", err)
	}
	svc, err := server.NewService(server.Deps{
		Payments:     fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier:   d.classifier,
		Store:        store,
		Graph:        &lamboGraph{client: client},
		SchemaDir:    filepath.Join("testdata", "api"),
		Pool:         d.poolIdx,
		FitRows:      d.fitRows,
		IdentitySalt: d.salt,
		Lines:        fitindex.LinesFunc(staticLines),
		StandingDir:  d.standing,
	})
	if err != nil {
		return nil, fmt.Errorf("new service for %s: %w", shopper, err)
	}
	_ = shopper
	return svc, nil
}

// questionForEvent returns the open question filed by one webhook event.
func questionForEvent(ctx context.Context, svc *server.Service, shopper, eventID string) (string, error) {
	pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: shopper})
	if err != nil {
		return "", fmt.Errorf("pending questions: %w", err)
	}
	for _, question := range pending.Questions {
		if question.Link.EventID == eventID {
			return question.QuestionID, nil
		}
	}
	return "", &fail{fmt.Sprintf("no pending question for event %s", eventID)}
}

// answerReason records one classified answer for one event's question,
// replaying the labelled recording (no model call).
func (d *driver) answerReason(svc *server.Service, shopper, eventID, answer string) error {
	question, err := questionForEvent(d.ctx, svc, shopper, eventID)
	if err != nil {
		return err
	}
	res, err := svc.RecordReason(d.ctx, contract.RecordReasonRequest{
		ShopperID: shopper, QuestionID: question, AnswerText: answer,
	})
	if err != nil {
		return fmt.Errorf("record reason for %s: %w", eventID, err)
	}
	if !res.Accepted || res.AskAgain {
		return &fail{fmt.Sprintf("reason for %s: accepted=%t ask_again=%t, want true false (recording %q)",
			eventID, res.Accepted, res.AskAgain, answer)}
	}
	d.say("-- reason %s %q: category=%s conf=%.2f accepted", eventID, answer, res.Category, res.Confidence)
	return nil
}

// recallFor polls recall until the guidance basis is the want step, so
// eventual graph promotion does not race the assertion.
func (d *driver) recallFor(svc *server.Service, req contract.RecallRequest, want contract.GuidanceBasis) (contract.RecallResponse, error) {
	deadline := time.Now().Add(15 * time.Minute)
	for {
		recall, err := svc.Recall(d.ctx, req)
		if err != nil {
			return contract.RecallResponse{}, fmt.Errorf("recall: %w", err)
		}
		if recall.Guidance.Basis == want {
			return recall, nil
		}
		if time.Now().After(deadline) {
			return contract.RecallResponse{}, &fail{fmt.Sprintf("recall never decided %q: basis %q, guidance %q",
				string(want), string(recall.Guidance.Basis), recall.Guidance.Text)}
		}
		time.Sleep(15 * time.Second)
	}
}

// poolLine is the seeded line the pool steps read on: the demo oxford,
// so the pool step and the own-evidence step meet on one line. The
// seeded population says it runs small; the own-evidence shopper's own
// three too-large returns disagree with it live, which is exactly the
// pair the precedence order resolves.
func poolLine() domain.ProductLine {
	return domain.ProductLine{Brand: "brand-x-001", Category: "shirts", FitHandle: "regular"}
}

// seedPool writes the Earned-small population the pool step reads: eight
// settled small returns (eight distinct shoppers: the 8-shopper floor),
// one large return and one fits-well feedback. Eight of ten fit outcomes
// point small (the 60% floor) at eight times the opposite (the 2x rule).
func (d *driver) seedPool() error {
	type entry struct {
		payer  string
		txn    string
		settle string
	}
	plan := []entry{}
	for i := 1; i <= 8; i++ {
		plan = append(plan, entry{payer: fmt.Sprintf("t85b-seed-%02d", i), txn: fmt.Sprintf("T85B-TXN-%02d", i), settle: "small"})
	}
	plan = append(plan,
		entry{payer: "t85b-seed-09", txn: "T85B-TXN-09", settle: "large"},
		entry{payer: "t85b-seed-10", txn: "T85B-TXN-10", settle: "fitswell"},
	)
	base := time.Now().UTC().Truncate(time.Second)
	for i, e := range plan {
		writer, err := fitindex.NewWriter(d.fitRows, d.keyer, e.payer, fitindex.LinesFunc(staticLines))
		if err != nil {
			return fmt.Errorf("build fit writer for %s: %w", e.payer, err)
		}
		purchase := fitindex.Purchase{TxnID: e.txn, Line: poolLine(), Size: "M", Seeded: true}
		capturedAt := base.Add(-3 * time.Hour).Add(time.Duration(i) * time.Minute)
		if err := writer.Captured(d.ctx, purchase, capturedAt); err != nil {
			return fmt.Errorf("capture %s: %w", e.txn, err)
		}
		switch e.settle {
		case "small", "large":
			refundedAt := capturedAt.Add(10 * time.Minute)
			if err := writer.Refunded(d.ctx, purchase, refundedAt); err != nil {
				return fmt.Errorf("refund %s: %w", e.txn, err)
			}
			category := domain.CategoryFitTooSmall
			if e.settle == "large" {
				category = domain.CategoryFitTooLarge
			}
			if err := writer.Classified(d.ctx, purchase, category, refundedAt.Add(20*time.Minute)); err != nil {
				return fmt.Errorf("classify %s: %w", e.txn, err)
			}
		case "fitswell":
			if err := writer.Feedback(d.ctx, purchase, domain.FitVerdictFitsWell, capturedAt.Add(30*time.Minute)); err != nil {
				return fmt.Errorf("feedback %s: %w", e.txn, err)
			}
		}
	}
	return nil
}

// statementSeed is one shopper's loading history holding nothing but a
// general statement: no evidence, no questions. The statement derives as
// a note through the live graph, so the serving rule is what the
// statement step reads live.
func statementSeed(shopper string) server.Seed {
	return server.Seed{
		ShopperID: shopper,
		Beliefs: []contract.BeliefView{{
			BeliefID:  "seed_stmt_001",
			Kind:      domain.BeliefKindFit,
			Subject:   domain.Subject{Kind: domain.SubjectBrand, ID: "brand-s-001", Name: "Brand S"},
			Value:     "shopper says they are usually size M",
			Seeded:    false,
			UpdatedAt: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		}},
		Links:            map[string][]string{},
		Pending:          contract.PendingQuestionsResponse{ShopperID: shopper, Questions: []contract.QuestionView{}},
		Timelines:        map[string][]contract.TimelineEvent{},
		Notices:          map[string]string{},
		FeedbackVerdicts: map[string]contract.FeedbackVerdict{},
	}
}

func run(ctx context.Context) (err error) {
	d := &driver{ctx: ctx}
	binary, err := required("LAMBO_BIN")
	if err != nil {
		return err
	}
	elastic, err := required("ELASTIC_URL")
	if err != nil {
		return err
	}
	apiKey, err := required("LAMBO_ES_API_KEY")
	if err != nil {
		return err
	}
	salt, err := required("MEMENTO_IDENTITY_SALT")
	if err != nil {
		return err
	}
	d.binary = binary
	d.elastic = elastic
	d.apiKey = apiKey
	d.salt = salt
	d.scrub = newScrubber(map[string]string{
		"ELASTIC_URL":           elastic,
		"LAMBO_ES_API_KEY":      apiKey,
		"MEMENTO_IDENTITY_SALT": salt,
	})
	d.store = newES(elastic, apiKey)

	version, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return fmt.Errorf("read the hub binary version: %w", err)
	}
	d.printf("T8.5b live layer: recall never refuses, and says which step decided")
	d.printf("hub binary: %s", strings.TrimSpace(string(version)))
	d.printf("deployment: <deployment> (elastic, T0.12's project)")
	d.printf("embedder: fixture (in-process vectors; no embedding endpoint called)")
	d.printf("classifier: recording replay (no model call)")
	d.printf("budget: under $0.50 of model spend; test sessions and indices removed afterwards")

	raw := make([]byte, 3)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("mint the run id: %w", err)
	}
	runDir := filepath.Join(os.TempDir(), "t85b-precedence-"+hex.EncodeToString(raw))
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return fmt.Errorf("make the run dir: %w", err)
	}
	d.runDir = runDir
	d.standing = filepath.Join(runDir, "standing")
	if err := os.MkdirAll(d.standing, 0700); err != nil {
		return fmt.Errorf("make the standing dir: %w", err)
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("mint the hub token: %w", err)
	}
	d.token = hex.EncodeToString(tokenBytes)
	d.scrub.secrets = append(d.scrub.secrets, d.token)

	cleaned := false
	defer func() {
		if cerr := d.cleanup(cleaned); cerr != nil {
			if err == nil {
				err = cerr
			} else {
				fmt.Fprintf(os.Stderr, "cleanup also failed: %v\n", d.scrub.clean(cerr.Error()))
			}
		}
	}()

	d.printf("\n== 0. the test namespaces are clean")
	before, err := d.store.indices(ctx, recallPrefix)
	if err != nil {
		return err
	}
	d.printf("-- prefix listing before: GET /_cat/indices/%s* -> %v", recallPrefix, before)
	if err := check(len(before) == 0, "prefix %s is not clean: %v; refusing so no index is deleted by mistake", recallPrefix, before); err != nil {
		return err
	}

	cfg := filepath.Join(runDir, "hub.toml")
	if err := hubConfig(cfg, filepath.Join(runDir, "lambo.db"), "shopper-t85b-pinned", 16, elastic); err != nil {
		return err
	}
	if out, err := exec.Command(binary, "--config", cfg, "provision").CombinedOutput(); err != nil {
		return fmt.Errorf("provision: %v: %s", err, excerpt(out))
	}
	hub, err := startProc(binary, []string{
		"--config", cfg, "serve",
		"--transport", "http", "--bind", "127.0.0.1", "--port", "0",
		"--agent", "memento-layer", "--max-sessions", "128",
		"--rate-limit-rps", "200",
	}, append(os.Environ(), tokenEnv+"="+d.token), filepath.Join(runDir, "hub.log"))
	if err != nil {
		return fmt.Errorf("start the hub: %w", err)
	}
	d.hubPort = hub.port
	d.printf("\n-- hub up: addr=127.0.0.1:%d (T8.1 shape, recall prefix %s)", hub.port, recallPrefix)
	defer func() {
		_ = hub.stop()
		d.printf("\n-- hub stopped (one SIGTERM, then waited, so the write-behind log drains)")
	}()

	d.printf("\n== 1. the test fit index")
	fitRows, err := fitindex.NewHTTPIndex(elastic, apiKey, fitIndexName)
	if err != nil {
		return fmt.Errorf("build fit index client: %w", err)
	}
	if err := fitRows.Create(ctx); err != nil {
		return fmt.Errorf("create index %s: %w", fitIndexName, err)
	}
	d.fitRows = fitRows
	d.printf("-- create: PUT /%s with the recorded mapping -> ok", fitIndexName)
	poolIdx, err := pool.NewHTTPIndex(elastic, apiKey, fitIndexName)
	if err != nil {
		return fmt.Errorf("build pool index client: %w", err)
	}
	d.poolIdx = poolIdx
	keyer, err := identity.NewKeyer(salt)
	if err != nil {
		return fmt.Errorf("build identity keyer: %w", err)
	}
	d.keyer = keyer
	classifier, err := fakes.NewRecordingReasonClassifier(filepath.Join("testdata", "jev"))
	if err != nil {
		return fmt.Errorf("open classifier recordings: %w", err)
	}
	d.classifier = classifier
	if err := d.seedPool(); err != nil {
		return err
	}
	d.printf("-- seeded pool population: 10 rows on brand-x-001/shirts/regular size M (8 small, 1 large, 1 fits-well)")

	d.printf("\n== 2. one case first, then its cost")
	correctionSvc, correctionClient, err := d.openShopper("t85b-correction")
	if err != nil {
		return err
	}
	correctionReq := contract.RecallRequest{ShopperID: "t85b-correction", BrandID: "brand-x-001"}
	// A capture first: it files no question and earns no belief, but it
	// names the brand, so the standing correction is recorded under
	// "Brand X" rather than the bare brand id. The record must come
	// before any recall: a recall with no beliefs would clobber the
	// brand name back to the id.
	if _, err := correctionSvc.IngestWebhook(ctx, "t85b-correction", "WH-2EV-06"); err != nil {
		return fmt.Errorf("ingest the correction shopper's capture: %w", err)
	}
	correction, err := correctionSvc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: "t85b-correction", Text: "I've lost weight, use M",
	})
	if err != nil {
		return fmt.Errorf("record standing correction: %w", err)
	}
	if correction.BeliefID != "" || correction.Contradicted {
		return &fail{"standing correction with no belief contradicts one"}
	}
	if correction.Guidance.Text != "Take size M for Brand X" {
		return &fail{fmt.Sprintf("standing guidance %q, want %q", correction.Guidance.Text, "Take size M for Brand X")}
	}
	d.say("-- standing correction %s recorded with no belief: Claimed, uncontradicted", correction.Correction.CorrectionID)
	decided, err := d.recallFor(correctionSvc, correctionReq, contract.GuidanceCorrection)
	if err != nil {
		return err
	}
	if decided.OverridingCorrection == nil {
		return &fail{"standing correction does not override the recall"}
	}
	if decided.Guidance.Text != "Take size M for Brand X" {
		return &fail{fmt.Sprintf("correction guidance %q, want %q", decided.Guidance.Text, "Take size M for Brand X")}
	}
	d.printf("-- correction decides: guidance %q", decided.Guidance.Text)
	d.printf("-- spend after one case: $0.00 of model spend (recording replay; fixture embedder); elastic is test-prefixed churn only")
	d.printf("-- under the $0.50 budget; continuing with the other steps")

	d.printf("\n== 3. the cross-session correction holds")
	freshClient, err := d.attach("shopper-t85b-correction")
	if err != nil {
		return err
	}
	_ = correctionClient
	freshSvc, err := d.service(freshClient, "t85b-correction")
	if err != nil {
		return err
	}
	again, err := d.recallFor(freshSvc, correctionReq, contract.GuidanceCorrection)
	if err != nil {
		return fmt.Errorf("recall over a fresh hub MCP session: %w", err)
	}
	if again.OverridingCorrection == nil ||
		again.OverridingCorrection.CorrectionID != decided.OverridingCorrection.CorrectionID {
		return &fail{"the correction did not survive a fresh hub MCP session"}
	}
	d.printf("-- fresh hub MCP session to the same shopper session: same correction %s still decides",
		again.OverridingCorrection.CorrectionID)
	restartedSvc, err := d.service(freshClient, "t85b-correction")
	if err != nil {
		return err
	}
	afterRestart, err := d.recallFor(restartedSvc, correctionReq, contract.GuidanceCorrection)
	if err != nil {
		return fmt.Errorf("recall after a layer restart: %w", err)
	}
	if afterRestart.OverridingCorrection == nil ||
		afterRestart.OverridingCorrection.CorrectionID != decided.OverridingCorrection.CorrectionID {
		return &fail{"the standing correction lapsed across a layer restart"}
	}
	d.printf("-- rebuilt layer over the same standing dir: standing %s re-recorded, still decides",
		afterRestart.OverridingCorrection.CorrectionID)

	d.printf("\n== 4. the pool decides for a new shopper")
	poolSvc, _, err := d.openShopper("t85b-pool")
	if err != nil {
		return err
	}
	pooled, err := d.recallFor(poolSvc, contract.RecallRequest{ShopperID: "t85b-pool",
		BrandID: "brand-x-001", Category: "shirts", FitHandle: "regular", SizeConsidered: "M"},
		contract.GuidancePool)
	if err != nil {
		return err
	}
	if len(pooled.Beliefs) != 0 {
		return &fail{fmt.Sprintf("new-shopper recall serves %d beliefs, want none", len(pooled.Beliefs))}
	}
	if pooled.Pool == nil || pooled.PoolLookup != contract.PoolConsulted {
		return &fail{"new-shopper recall carries no consulted pool"}
	}
	if pooled.Pool.Status != domain.StatusEarned || pooled.Pool.Direction == nil ||
		*pooled.Pool.Direction != domain.FitDirectionSmall {
		return &fail{fmt.Sprintf("pool reads %+v, want Earned small", pooled.Pool)}
	}
	if pooled.Pool.Precedence == nil || pooled.Pool.Precedence.Step != contract.GuidancePool ||
		pooled.Pool.Precedence.Overridden {
		return &fail{fmt.Sprintf("pool precedence %+v, want the deciding pool step", pooled.Pool.Precedence)}
	}
	if pooled.Pool.Seeded.Settled != 10 || pooled.Pool.Live.Settled != 0 {
		return &fail{fmt.Sprintf("pool split seeded %d live %d, want the 10 seeded rows alone",
			pooled.Pool.Seeded.Settled, pooled.Pool.Live.Settled)}
	}
	want := "Size up to L for brand-x-001: 8 of the 10 fit-classified settled purchases at size M came back too small, against 1 too large."
	if pooled.Guidance.Text != want {
		return &fail{fmt.Sprintf("pool guidance %q, want %q", pooled.Guidance.Text, want)}
	}
	d.printf("-- pool decides: Earned small, seeded %d live %d, guidance %q",
		pooled.Pool.Seeded.Settled, pooled.Pool.Live.Settled, pooled.Guidance.Text)

	d.printf("\n== 5. own evidence beats a disagreeing pool")
	evidenceSvc, _, err := d.openShopper("t85b-evidence")
	if err != nil {
		return err
	}
	for _, event := range []string{"WH-2EV-01", "WH-2EV-02", "WH-2EV-03"} {
		if _, err := evidenceSvc.IngestWebhook(ctx, "t85b-evidence", event); err != nil {
			return fmt.Errorf("ingest %s: %w", event, err)
		}
		if err := d.answerReason(evidenceSvc, "t85b-evidence", event, "it swims on me, far too big"); err != nil {
			return err
		}
	}
	evidenceReq := contract.RecallRequest{ShopperID: "t85b-evidence", BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M"}
	own, err := d.recallFor(evidenceSvc, evidenceReq, contract.GuidanceOwnEvidence)
	if err != nil {
		return err
	}
	if len(own.Beliefs) != 1 {
		return &fail{fmt.Sprintf("own-evidence recall serves %d beliefs, want the 1 Building belief", len(own.Beliefs))}
	}
	if own.Beliefs[0].Precedence == nil || own.Beliefs[0].Precedence.Step != contract.GuidanceOwnEvidence ||
		own.Beliefs[0].Precedence.Overridden {
		return &fail{fmt.Sprintf("own belief precedence %+v, want the deciding own step", own.Beliefs[0].Precedence)}
	}
	if own.Pool == nil || own.Pool.Precedence == nil || !own.Pool.Precedence.Overridden ||
		*own.Pool.Precedence.OverriddenBy != contract.GuidanceOwnEvidence {
		return &fail{fmt.Sprintf("pool precedence %+v, want overridden by own_evidence", own.Pool.Precedence)}
	}
	if own.Pool.Seeded.Settled != 10 || own.Pool.Live.Settled != 3 {
		return &fail{fmt.Sprintf("pool split seeded %d live %d, want 10 seeded and the shopper's 3 live rows",
			own.Pool.Seeded.Settled, own.Pool.Live.Settled)}
	}
	if own.Guidance.Text != "Take size S for Brand X: your own 3 returns say it runs large, ahead of the pool." {
		return &fail{fmt.Sprintf("own-evidence guidance %q, want the ahead-of-the-pool text", own.Guidance.Text)}
	}
	d.printf("-- three too-large returns: belief %s %s/%s, guidance %q",
		own.Beliefs[0].BeliefID, own.Beliefs[0].Status, own.Beliefs[0].Standing, own.Guidance.Text)
	d.printf("-- the Earned-small pool rides along overridden: seeded %d live %d",
		own.Pool.Seeded.Settled, own.Pool.Live.Settled)

	d.printf("\n== 6. a general statement decides with nothing else")
	statementSvc, _, err := d.openShopper("t85b-statement")
	if err != nil {
		return err
	}
	if err := statementSvc.LoadSeed(ctx, statementSeed("t85b-statement")); err != nil {
		return fmt.Errorf("load the statement seed: %w", err)
	}
	stated, err := d.recallFor(statementSvc, contract.RecallRequest{ShopperID: "t85b-statement",
		BrandID: "brand-s-001"},
		contract.GuidanceStatement)
	if err != nil {
		return err
	}
	if stated.Guidance.Text != "Take size M for Brand S: the shopper's usual size." {
		return &fail{fmt.Sprintf("statement guidance %q, want the usual-size text", stated.Guidance.Text)}
	}
	d.printf("-- statement decides: guidance %q", stated.Guidance.Text)

	d.printf("\n== 7. nothing decides, and recall still answers")
	nothingSvc, _, err := d.openShopper("t85b-nothing")
	if err != nil {
		return err
	}
	empty, err := d.recallFor(nothingSvc, contract.RecallRequest{ShopperID: "t85b-nothing",
		BrandID: "brand-n-001"},
		contract.GuidanceNone)
	if err != nil {
		return err
	}
	if len(empty.Beliefs) != 0 || empty.Pool != nil || empty.OverridingCorrection != nil {
		return &fail{"an empty recall carries a belief, a pool or a correction"}
	}
	d.printf("-- nothing decides: GuidanceNone, no refusal: %q", empty.Guidance.Text)

	// A wired pool consulted on an empty line returns Claimed, which the
	// lookup reports as consulted-but-absent — the frozen contract's
	// example-none shape. The served assembler rejects exactly that pair,
	// so the recall fails instead of serving the empty pool. Pinned live:
	// the fix is the guard in internal/api/server/recall.go, owner T8.5.
	d.printf("\n-- probe: the consulted-but-empty pool, pinned")
	_, probeErr := nothingSvc.Recall(ctx, contract.RecallRequest{ShopperID: "t85b-nothing",
		BrandID: "brand-n-001", Category: "shirts", FitHandle: "regular", SizeConsidered: "M"})
	if probeErr == nil {
		return &fail{"the consulted-empty-pool pin is stale: the guard now serves the empty pool; update the driver (owner T8.5)"}
	}
	if !strings.Contains(probeErr.Error(), "pool absent with lookup") {
		return fmt.Errorf("empty-line recall failed differently: %w", probeErr)
	}
	d.say("-- full-line recall on the empty line fails as pinned: %s", excerptStr(probeErr.Error()))

	d.printf("\n== 8. the dangling view, pinned")
	if err := d.danglingView(); err != nil {
		return err
	}

	cleaned = true
	d.printf("\nmodel spend: $0.00 (no model call: every answer replays a labelled recording)")
	d.printf("elastic: test-prefixed index churn only (one hub prefix, one fit index); deleted below")
	return nil
}

// openShopper attaches one hub session and builds that shopper's served
// layer over it.
func (d *driver) openShopper(shopper string) (*server.Service, *mcp, error) {
	client, err := d.attach("shopper-" + shopper)
	if err != nil {
		return nil, nil, err
	}
	svc, err := d.service(client, shopper)
	if err != nil {
		return nil, nil, err
	}
	return svc, client, nil
}

// danglingView reproduces the T8.5 review finding live and pins it: a
// declared correction survives a layer restart with its stale belief id
// and matches nothing afterwards, so the correction drops out of recall
// entirely; rebuilding the belief does not re-link it. The fix is
// re-linking by content match inside internal/api/server/, outside this
// task's owns, so T8.5 owns it.
//
// The declared shopper reuses the recorded purchases 01-03, whose fit
// rows therefore share document ids with the evidence shopper's rows and
// overwrite them (one purchase, one row: in production a purchase
// belongs to one shopper). The pool step and the own-evidence step run
// first and assert before this overwrite, and this step asserts no pool
// counts.
func (d *driver) danglingView() error {
	ctx := d.ctx
	svc, _, err := d.openShopper("t85b-declared")
	if err != nil {
		return err
	}
	for _, event := range []string{"WH-2EV-01", "WH-2EV-02", "WH-2EV-03"} {
		if _, err := svc.IngestWebhook(ctx, "t85b-declared", event); err != nil {
			return fmt.Errorf("ingest %s: %w", event, err)
		}
		if err := d.answerReason(svc, "t85b-declared", event, "too tight in the shoulders"); err != nil {
			return err
		}
	}
	req := contract.RecallRequest{ShopperID: "t85b-declared", BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M"}
	served, err := d.recallFor(svc, req, contract.GuidanceOwnEvidence)
	if err != nil {
		return fmt.Errorf("declared shopper's own evidence: %w", err)
	}
	if len(served.Beliefs) != 1 {
		return &fail{"the declared shopper serves no own belief to contradict"}
	}
	beliefID := served.Beliefs[0].BeliefID
	declared, err := svc.RecordCorrection(ctx, contract.RecordCorrectionRequest{
		ShopperID: "t85b-declared", Text: "I've lost weight, use M", ContradictsBeliefID: beliefID,
	})
	if err != nil {
		return fmt.Errorf("record declared correction: %w", err)
	}
	if !declared.Contradicted || declared.BeliefID != beliefID {
		return &fail{"the declared correction does not contradict its belief at once"}
	}
	first, err := d.recallFor(svc, req, contract.GuidanceCorrection)
	if err != nil {
		return err
	}
	if first.OverridingCorrection == nil ||
		first.OverridingCorrection.ContradictsBeliefID != beliefID {
		return &fail{"the declared correction does not name its belief before the restart"}
	}
	if len(first.Beliefs) != 1 || !first.Beliefs[0].Contradicted {
		return &fail{"the belief carries no contradiction before the restart"}
	}
	d.say("-- before restart: correction %s declared against belief %s, guidance %q",
		first.OverridingCorrection.CorrectionID, beliefID, first.Guidance.Text)

	// The restart: a rebuilt layer over the same standing directory and
	// the same hub session. The hub keeps the graph; the layer mints
	// fresh belief ids, so the restored view cites a belief that no
	// longer exists and matches no precedence class. The correction
	// loses all force: the Earned-small pool decides instead.
	client, err := d.attach("shopper-t85b-declared")
	if err != nil {
		return err
	}
	restarted, err := d.service(client, "t85b-declared")
	if err != nil {
		return err
	}
	after, err := d.recallFor(restarted, req, contract.GuidancePool)
	if err != nil {
		return fmt.Errorf("recall after restart: %w", err)
	}
	if after.OverridingCorrection != nil {
		return &fail{"the declared correction still overrides after the restart: this pin is stale (owner T8.5)"}
	}
	if len(after.Beliefs) != 0 {
		return &fail{fmt.Sprintf("the restarted layer serves %d beliefs, want none (beliefs are rebuilt from new writes)", len(after.Beliefs))}
	}
	d.say("-- after restart: the declared view cites stale belief %s and matches nothing; the pool decides: %q",
		first.OverridingCorrection.ContradictsBeliefID, after.Guidance.Text)

	// Rebuilding the belief does not re-link the view: the journal
	// dropped the declared entry at persist time, so the new belief
	// declares nothing. What the recall then shows depends on the mint
	// order, which is the fragility. The rebuilt partition replays the
	// same mint sequence, so the new belief reuses the old id and the
	// stale view matches by id coincidence, not by content re-linking:
	// the belief reads contradicted against a view that was never
	// re-declared. Any divergence in mint order would leave the view
	// dangling (or attached to an unrelated belief), and a hub that
	// forgot the old against edge would surface the dangling
	// declaration as a hard refusal instead. All three shapes are
	// pinned as observed; the fix is re-linking by content match in
	// internal/api/server/, owner T8.5.
	for _, event := range []string{"WH-2EV-01", "WH-2EV-02", "WH-2EV-03"} {
		if _, err := restarted.IngestWebhook(ctx, "t85b-declared", event); err != nil {
			return fmt.Errorf("re-ingest %s: %w", event, err)
		}
		if err := d.answerReason(restarted, "t85b-declared", event, "too tight in the shoulders"); err != nil {
			return err
		}
	}
	rebuilt, err := restarted.Recall(ctx, req)
	if err != nil {
		if !strings.Contains(err.Error(), "contradicted belief has no overriding correction") {
			return fmt.Errorf("rebuilt recall failed differently: %w", err)
		}
		d.say("-- after rebuild: recall fails as pinned (the graph remembers the declaration, the layer lost its mapping): %s",
			excerptStr(d.scrub.clean(err.Error())))
		return nil
	}
	if len(rebuilt.Beliefs) != 1 {
		return &fail{fmt.Sprintf("rebuilt recall serves %d beliefs, want the 1 rebuilt belief", len(rebuilt.Beliefs))}
	}
	if rebuilt.Beliefs[0].BeliefID != beliefID {
		if rebuilt.Beliefs[0].Contradicted || rebuilt.OverridingCorrection != nil {
			return &fail{"the divergent rebuild contradicts or overrides without a re-link: this pin is stale (owner T8.5)"}
		}
		d.say("-- after rebuild: new belief %s served uncontradicted, the stale view (id %s) silent: pinned, owner T8.5",
			rebuilt.Beliefs[0].BeliefID, first.OverridingCorrection.ContradictsBeliefID)
		return nil
	}
	if !rebuilt.Beliefs[0].Contradicted || rebuilt.Beliefs[0].ContradictedBy == nil ||
		rebuilt.Beliefs[0].ContradictedBy.CorrectionID != first.OverridingCorrection.CorrectionID {
		return &fail{"the id-coincident rebuild does not carry the stale declaration: this pin is stale (owner T8.5)"}
	}
	d.say("-- after rebuild: belief %s reuses the old id, so the stale view %s matches by id coincidence, not by re-linking: pinned, owner T8.5",
		rebuilt.Beliefs[0].BeliefID, rebuilt.Beliefs[0].ContradictedBy.CorrectionID)
	return nil
}

// cleanup always deletes the test indices, shows the empty listings and
// removes the run directory.
func (d *driver) cleanup(done bool) error {
	_ = done
	if d.runDir == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	for _, prefix := range []string{recallPrefix} {
		before, err := d.store.indices(ctx, prefix)
		if err != nil {
			return err
		}
		for _, name := range before {
			if !strings.HasPrefix(name, prefix) {
				return &fail{fmt.Sprintf("refusing to delete %q: it is not a %s index", name, prefix)}
			}
		}
		for _, name := range before {
			if err := d.store.deleteIndex(ctx, name); err != nil {
				return err
			}
		}
		d.printf("\n-- cleanup: deleted %d index(es) under %s", len(before), prefix)
		after, err := d.store.indices(ctx, prefix)
		if err != nil {
			return err
		}
		d.printf("-- prefix listing after: GET /_cat/indices/%s* -> %v", prefix, after)
		if err := check(len(after) == 0, "%d index(es) remain under %s: %v", len(after), prefix, after); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(d.runDir); err != nil {
		return fmt.Errorf("remove the run dir: %w", err)
	}
	d.printf("-- run dir removed (store, configs, standing dir, child logs); nothing keyed survives locally")
	return nil
}

func main() {
	if err := run(context.Background()); err != nil {
		redacted := err.Error()
		for _, secret := range []string{os.Getenv("ELASTIC_URL"), os.Getenv("LAMBO_ES_API_KEY"),
			os.Getenv("MEMENTO_IDENTITY_SALT")} {
			if secret != "" {
				redacted = strings.ReplaceAll(redacted, secret, "<redacted>")
			}
		}
		fmt.Fprintf(os.Stderr, "FAIL T8.5b live layer: %s\n", redacted)
		os.Exit(1)
	}
	fmt.Println("\nPASS T8.5b live layer: every step decided, the cross-session correction holds, the dangle pinned, the namespace erased")
}
