// Live driver for T8.1b: the Lambo hub against the live Elastic recall tier.
//
// One `lambo serve --transport http` hub (the T8.1 shape: one pinned demo
// session, `max_attached = 16`, the `shopper-` on-demand credential, started
// with `--max-sessions 128 --rate-limit-rps 200`) holds three shoppers created
// on demand. Each shopper recalls only its own concepts; the ledger reader,
// started with those three sessions on its allowlist, serves them and refuses
// a session created after it started. A second hub with `max_attached = 2`
// records the 503 + `Retry-After` refusal with every place full. A
// concurrency probe and a rate probe record what `--max-sessions 128` and
// `--rate-limit-rps 200` do on the way to the 0.5 CPU starter.
//
// The embedder is the key-free fixture: vectors are computed in process, so
// no embedding endpoint is called and the embedding spend is $0.00 against
// the $0.20 budget. Only the recall tier is live: the hub mirrors every
// session's documents into test-prefixed indices on the deployment, every
// query filtered by `session_id`. The driver deletes those indices at the
// end, whether or not the checks passed, and shows the empty prefix listing.
//
// This is a main program, never a test: no *_test.go lives under
// tests/live/, so CI builds this package but never runs it. Run it by hand
// with the keys loaded:
//
//	source tools/env.sh
//	bash tests/live/hub/run.sh
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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nrynss/memento-emptor/internal/identity"
)

// The test namespace. Every index this driver creates lives under it, and
// the driver refuses to run when the prefix holds anything else, so it
// never deletes an index it did not create.
const indexPrefix = "t81b-"

// The seeded demo shopper's session, pinned by deploy/start.sh: the prefix
// plus the 64-hex HMAC-SHA256 of payer id `shopper_01` under the published
// seed salt. The driver asserts this equals what internal/identity builds,
// derives nothing in it, and leaves the deployment's demo data alone.
const demoSession = "shopper-00895719a8a9d1ba88c99984e50dd200a83b42cfab629a4919c635033e2e5caf"

// The shoppers this run creates on demand. Fixed names keep the transcript
// deterministic; the prefix guard refuses a rerun over a dirty namespace.
const (
	shopperA = "shopper-t81b-alpha"
	shopperB = "shopper-t81b-bravo"
	shopperC = "shopper-t81b-charlie"
	shopperD = "shopper-t81b-late"
)

// The 503 probe's sessions, on the second hub.
const (
	probeHot = "shopper-t81b-s1"
	probeNew = "shopper-t81b-s2"
)

// Each shopper's marker: a rare word carried by both of its concepts, so a
// recall for it is a clean own/cross signal.
const (
	markerA = "Vexilhold"
	markerB = "Brasswren"
	markerC = "Codlefirth"
)

// agentName is the MCP agent identity this driver writes as.
const agentName = "memento-t81b"

// tokenEnv is the credential's token variable, read by the hub config. The
// value is minted per run and reaches the children through the environment
// only: it is never written to a file or printed.
const tokenEnv = "LAMBO_HUB_TOKEN"

// responseBytes bounds how much of a response the probes read.
const responseBytes = 1 << 20

// fail is the driver's one error shape: a check that did not hold.
type fail struct{ msg string }

func (e *fail) Error() string { return e.msg }

func check(cond bool, format string, args ...any) error {
	if !cond {
		return &fail{fmt.Sprintf(format, args...)}
	}
	return nil
}

// scrub replaces the deployment locator and the secrets with placeholders
// before anything is printed. Tool output passes through it on the way out.
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

// es is the driver's read and cleanup client for the deployment. Writes go
// through the hub's recall tier; this client only lists, counts and deletes
// the test prefix.
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

// indices lists the index names under the test prefix.
func (e *es) indices(ctx context.Context) ([]string, error) {
	status, body, err := e.do(ctx, http.MethodGet, "/_cat/indices/"+indexPrefix+"*?format=json&h=index&expand_wildcards=all")
	if err != nil {
		return nil, fmt.Errorf("list %s indices: %w", indexPrefix, err)
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("list %s indices: status %d", indexPrefix, status)
	}
	var rows []struct {
		Index string `json:"index"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("read %s index list: %w", indexPrefix, err)
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

func rssMB(pid int) float64 {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			var kb float64
			if _, err := fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "VmRSS:")), "%f", &kb); err == nil {
				return kb / 1024
			}
		}
	}
	return -1
}

// mcp is one JSON-RPC session over streamable HTTP at /mcp/s/<session>.
// rawCall surfaces the HTTP status and headers so the 503 probe can read
// the refusal; call fails on anything but 200 like the checks do.
type mcp struct {
	host       string
	port       int
	path       string
	token      string
	sessionID  string
	nextID     int
	mu         sync.Mutex
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

func (c *mcp) post(body map[string]any, notify bool) (int, http.Header, []byte, error) {
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
		return 0, nil, nil, err
	}
	return response.StatusCode, response.Header, raw, nil
}

// decode unwraps a JSON-RPC reply from a JSON or an SSE body.
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

func (c *mcp) initialize() error {
	c.nextID++
	status, headers, raw, err := c.post(map[string]any{
		"jsonrpc": "2.0",
		"id":      c.nextID,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "memento-t81b", "version": "1"},
		},
	}, false)
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
	}, true)
	if err != nil {
		return fmt.Errorf("initialized notification: %w", err)
	}
	if status != 200 && status != 202 && status != 204 {
		return fmt.Errorf("initialized notification: HTTP %d", status)
	}
	return nil
}

// rawCall makes one tools/call and returns the HTTP status with it, so the
// 503 probe can name the refusal exactly. It holds the client's lock: one
// MCP session may be shared by several burst workers.
func (c *mcp) rawCall(name string, args map[string]any) (int, http.Header, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	status, headers, raw, err := c.post(map[string]any{
		"jsonrpc": "2.0",
		"id":      c.nextID,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	}, false)
	if err != nil {
		return 0, nil, "", err
	}
	if status != http.StatusOK {
		return status, headers, excerpt(raw), nil
	}
	reply, err := decode(headers.Get("Content-Type"), raw)
	if err != nil {
		return status, headers, "", err
	}
	if failure, ok := reply["error"]; ok {
		return status, headers, "", fmt.Errorf("%s: %v", name, failure)
	}
	result, _ := reply["result"].(map[string]any)
	var texts []string
	if content, ok := result["content"].([]any); ok {
		for _, item := range content {
			if block, ok := item.(map[string]any); ok {
				if text, ok := block["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
	}
	return status, headers, strings.Join(texts, "\n"), nil
}

func (c *mcp) call(name string, args map[string]any) (string, error) {
	args = mapsCopy(args)
	args["agent_id"] = agentName
	status, _, text, err := c.rawCall(name, args)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("%s: HTTP %d: %s", name, status, text)
	}
	return text, nil
}

func mapsCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}

var receiptRE = regexp.MustCompile(`receipt (lwr[0-9a-z.]+)`)

// derive writes concepts and waits every receipt until it is applied.
func (c *mcp) derive(contents ...map[string]string) error {
	concepts := make([]any, 0, len(contents))
	for _, concept := range contents {
		concepts = append(concepts, map[string]any{
			"content": concept["content"], "concept_type": concept["kind"],
		})
	}
	text, err := c.call("lambo_derive", map[string]any{"concepts": concepts})
	if err != nil {
		return fmt.Errorf("derive: %w", err)
	}
	for _, receipt := range receiptRE.FindAllStringSubmatch(text, -1) {
		if err := c.waitReceipt(receipt[1]); err != nil {
			return err
		}
	}
	return nil
}

func (c *mcp) waitReceipt(receipt string) error {
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		text, err := c.call("lambo_stats", map[string]any{
			"receipt": receipt, "wait_ms": 5000,
		})
		if err != nil {
			return fmt.Errorf("wait receipt: %w", err)
		}
		switch {
		case strings.Contains(text, "applied"):
			return nil
		case strings.Contains(text, "failed"), strings.Contains(text, "refused"):
			return &fail{fmt.Sprintf("receipt %s not applied: %s", receipt, excerptStr(text))}
		}
	}
	return &fail{fmt.Sprintf("receipt %s did not settle", receipt)}
}

func (c *mcp) recall(query string) (string, error) {
	return c.call("lambo_recall", map[string]any{"query": query})
}

func (c *mcp) inspect(focus string) (string, error) {
	return c.call("lambo_inspect", map[string]any{"focus": focus, "depth": 2})
}

func (c *mcp) stats() (string, error) {
	return c.call("lambo_stats", map[string]any{})
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

// hubConfig writes the T8.1 serve table: one pinned session, the on-demand
// credential, the fixture embedder and the live recall tier.
func hubConfig(path, store, pinned string, maxAttached int, elasticURL string) error {
	text := "# Written at boot by tests/live/hub: the T8.1 shape, test-scoped.\n" +
		"[store]\nkind = \"sqlite\"\npath = \"" + store + "\"\n\n" +
		"[embedder]\nkind = \"fixture\"\ndim = 1024\ncalibration = \"off\"\n\n" +
		"[daemon]\ngc_interval = 1\ngc_idle_floor = 1\ngc_max_interval_secs = 3600\n" +
		"canonization_eval_interval_secs = 5\n\n" +
		"[recall]\nkind = \"elastic\"\nurl = \"" + elasticURL + "\"\n" +
		"api_key = { env = \"LAMBO_ES_API_KEY\" }\nindex_prefix = \"" + indexPrefix + "\"\n" +
		"refresh = \"true\"\ntimeout_ms = 20000\n\n" +
		"[serve]\nsessions = [\"" + pinned + "\"]\ndefault_session = \"" + pinned + "\"\n" +
		fmt.Sprintf("max_attached = %d\n", maxAttached) +
		"attach_concurrency = 1\nidle_detach_secs = 900\n\n" +
		"[[serve.credential]]\nname = \"memento\"\ntoken_env = \"" + tokenEnv + "\"\n" +
		"session_prefix = \"shopper-\"\ncreate = true\n\n" +
		"[web]\nlist_sessions = true\n"
	return os.WriteFile(path, []byte(text), 0600)
}

// driver holds the run's shared state.
type driver struct {
	ctx     context.Context
	binary  string
	elastic string
	store   *es
	token   string
	scrub   *scrubber
	runDir  string
	out     *os.File
}

func (d *driver) printf(format string, args ...any) {
	fmt.Fprintf(d.out, format+"\n", args...)
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

func (d *driver) startHub(name, store string, maxAttached int, rateLimit int) (*proc, error) {
	cfg := filepath.Join(d.runDir, name+".toml")
	if err := hubConfig(cfg, store, demoSession, maxAttached, d.elastic); err != nil {
		return nil, err
	}
	argv := []string{
		"--config", cfg, "serve",
		"--transport", "http", "--bind", "127.0.0.1", "--port", "0",
		"--agent", "memento-layer", "--max-sessions", "128",
		"--rate-limit-rps", fmt.Sprint(rateLimit),
	}
	if out, err := exec.Command(d.binary, "--config", cfg, "provision").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("provision: %v: %s", err, excerpt(out))
	}
	env := append(os.Environ(), tokenEnv+"="+d.token)
	hub, err := startProc(d.binary, argv, env, filepath.Join(d.runDir, name+".log"))
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}
	return hub, nil
}

func (d *driver) startReader(sessions []string, name string) (*proc, error) {
	cfg := filepath.Join(d.runDir, "hub.toml")
	argv := []string{"--config", cfg, "serve-web", "--port", "0", "--bind", "127.0.0.1"}
	for _, s := range sessions {
		argv = append(argv, "--session", s)
	}
	env := append(os.Environ(), tokenEnv+"="+d.token)
	web, err := startProc(d.binary, argv, env, filepath.Join(d.runDir, name+".log"))
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}
	return web, nil
}

func (d *driver) get(port int, path string) (int, http.Header, []byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	request, err := http.NewRequestWithContext(d.ctx, http.MethodGet,
		fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
	if err != nil {
		return 0, nil, nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, nil, err
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, responseBytes))
	if err != nil {
		return 0, nil, nil, err
	}
	return response.StatusCode, response.Header, raw, nil
}

// belief is one shopper's own sentence, carrying its marker.
func belief(session, marker string) string {
	return fmt.Sprintf("%s oxford in Brand V runs one size small for %s", marker, session)
}

func note(session, marker string) string {
	return fmt.Sprintf("%s keeps %s as its own brand record", session, marker)
}

// attach derives a shopper's two concepts and waits them applied.
func (d *driver) attach(port int, session, marker string) (*mcp, error) {
	client := newMCP(port, session, d.token)
	if err := client.initialize(); err != nil {
		return nil, fmt.Errorf("attach %s: %w", session, err)
	}
	if err := client.derive(
		map[string]string{"content": belief(session, marker), "kind": "logic"},
		map[string]string{"content": note(session, marker), "kind": "observation"},
	); err != nil {
		return nil, fmt.Errorf("attach %s: %w", session, err)
	}
	return client, nil
}

func run(ctx context.Context) (err error) {
	out := os.Stdout
	d := &driver{ctx: ctx, out: out}
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
	d.binary = binary
	d.elastic = elastic
	d.scrub = newScrubber(map[string]string{
		"ELASTIC_URL":      elastic,
		"LAMBO_ES_API_KEY": apiKey,
	})
	d.store = newES(elastic, apiKey)

	version, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return fmt.Errorf("read the hub binary version: %w", err)
	}
	d.printf("T8.1b live hub: one process for every shopper, against the live recall tier")
	d.printf("hub binary: %s", strings.TrimSpace(string(version)))
	d.printf("deployment: <deployment> (elastic, T0.12's project)")
	d.printf("embedder: fixture (in-process vectors; the tier mirrors them live)")
	d.printf("machine: %s, %d CPUs", runtime.GOOS+"/"+runtime.GOARCH, runtime.NumCPU())
	d.printf("flags under measure: --max-sessions 128 --rate-limit-rps 200")
	d.printf("budget: under $0.20 of embedding spend; test sessions erased afterwards")

	// The run directory holds the store, the configs and the child logs. It
	// is outside the repo and removed at the end, so no URL, key or token
	// survives the run.
	raw := make([]byte, 3)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("mint the run id: %w", err)
	}
	runDir := filepath.Join(os.TempDir(), "t81b-hub-"+hex.EncodeToString(raw))
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return fmt.Errorf("make the run dir: %w", err)
	}
	d.runDir = runDir
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

	d.printf("\n== 0. the test namespace is clean, and the demo session is the package's")
	before, err := d.store.indices(ctx)
	if err != nil {
		return err
	}
	d.printf("-- prefix listing before: GET /_cat/indices/%s* -> %v", indexPrefix, before)
	if err := check(len(before) == 0, "prefix %s is not clean: %v; refusing so no index is deleted by mistake", indexPrefix, before); err != nil {
		return err
	}
	keyer, err := identity.NewKeyer(identity.SeedSaltHex)
	if err != nil {
		return fmt.Errorf("build the seed keyer: %w", err)
	}
	built, err := keyer.SessionName("shopper_01")
	if err != nil {
		return fmt.Errorf("build the demo session name: %w", err)
	}
	if err := check(built == demoSession, "the demo session drifted: package builds %s", built); err != nil {
		return err
	}
	if err := check(identity.ValidSessionName(demoSession), "the pinned demo session is not addressable"); err != nil {
		return err
	}
	d.printf("-- demo session: package builds %s; start.sh pins the same 72 bytes", shortHash(built))

	hub, err := d.startHub("hub", filepath.Join(runDir, "lambo.db"), 16, 200)
	if err != nil {
		return err
	}
	d.printf("\n== hub up: addr=127.0.0.1:%d (one pinned session, max_attached=16, credential prefix shopper-, create=true)", hub.port)
	base := float64(0)
	if rss := rssMB(hub.cmd.Process.Pid); rss >= 0 {
		base = rss
		d.printf("-- hub RSS at start: %.1f MiB", rss)
	}

	d.printf("\n== 1. one shopper first, then its cost")
	clientA, err := d.attach(hub.port, shopperA, markerA)
	if err != nil {
		return err
	}
	own, err := clientA.recall(markerA)
	if err != nil {
		return fmt.Errorf("recall the first shopper's marker: %w", err)
	}
	if err := check(strings.Contains(own, markerA), "the first shopper's recall missed its own marker: %s", excerptStr(d.scrub.clean(own))); err != nil {
		return err
	}
	d.say("-- %s recalls %q: found its own marker", shopperA, markerA)
	d.printf("-- spend after one shopper: $0.00 of embedding (fixture computes in process; no embedding endpoint called)")
	d.printf("-- under the $0.20 budget; continuing with the other two shoppers")

	d.printf("\n== 2. two more shoppers, each recalling only its own concepts")
	clientB, err := d.attach(hub.port, shopperB, markerB)
	if err != nil {
		return err
	}
	clientC, err := d.attach(hub.port, shopperC, markerC)
	if err != nil {
		return err
	}
	clients := map[string]*mcp{shopperA: clientA, shopperB: clientB, shopperC: clientC}
	markers := map[string]string{shopperA: markerA, shopperB: markerB, shopperC: markerC}
	for _, session := range []string{shopperA, shopperB, shopperC} {
		own, err := clients[session].recall(markers[session])
		if err != nil {
			return fmt.Errorf("recall %s: %w", session, err)
		}
		if err := check(strings.Contains(own, markers[session]), "%s missed its own marker", session); err != nil {
			return err
		}
		var leaks []string
		for _, other := range []string{shopperA, shopperB, shopperC} {
			if other == session {
				continue
			}
			stranger, err := clients[session].recall(markers[other] + " oxford Brand V")
			if err != nil {
				return fmt.Errorf("cross-recall %s for %s: %w", session, other, err)
			}
			if strings.Contains(stranger, markers[other]) {
				leaks = append(leaks, other)
			}
		}
		if err := check(len(leaks) == 0, "recall crossed sessions: %s read %v", session, leaks); err != nil {
			return err
		}
		d.say("-- %s: own marker %q found; no other session's marker in its recalls", session, markers[session])
	}
	for _, session := range []string{shopperA, shopperB, shopperC} {
		other := shopperA
		if other == session {
			other = shopperB
		}
		text, err := clients[session].inspect(belief(other, markers[other]))
		if err != nil {
			return fmt.Errorf("cross-inspect %s for %s: %w", session, other, err)
		}
		if err := check(strings.Contains(text, "no concept matching"), "%s inspected %s's belief: %s", session, other, excerptStr(d.scrub.clean(text))); err != nil {
			return err
		}
	}
	d.printf("-- every cross-session exact-content inspect: no concept matching")

	d.printf("\n== 3. the seeded demo session holds none of their markers")
	demo, err := func() (*mcp, error) {
		c := newMCP(hub.port, demoSession, d.token)
		if err := c.initialize(); err != nil {
			return nil, err
		}
		return c, nil
	}()
	if err != nil {
		return fmt.Errorf("attach the demo session: %w", err)
	}
	for _, session := range []string{shopperA, shopperB, shopperC} {
		found, err := demo.recall(markers[session] + " oxford Brand V")
		if err != nil {
			return fmt.Errorf("recall %s's marker in the demo session: %w", markers[session], err)
		}
		if err := check(!strings.Contains(found, markers[session]), "the demo session recalled %s's marker", session); err != nil {
			return err
		}
		text, err := demo.inspect(belief(session, markers[session]))
		if err != nil {
			return fmt.Errorf("inspect %s's belief in the demo session: %w", session, err)
		}
		if err := check(strings.Contains(text, "no concept matching"), "the demo session inspected %s's belief", session); err != nil {
			return err
		}
	}
	stats, err := demo.stats()
	if err != nil {
		return fmt.Errorf("read the demo session stats: %w", err)
	}
	d.say("-- demo session: no test marker recalled, no test belief inspected; stats: %s", excerptStr(stats))

	d.printf("\n== 4. the ledger reader serves the allowlisted ones")
	// The allowlist mirrors the deploy: the demo shopper first (so the
	// unscoped alias reads it, as start.sh's reader does), then the test
	// shoppers.
	allowlist := []string{demoSession, shopperA, shopperB, shopperC}
	web, err := d.startReader(allowlist, "web")
	if err != nil {
		return err
	}
	d.printf("-- reader on 127.0.0.1:%d with the demo shopper plus [%s %s %s]", web.port, shopperA, shopperB, shopperC)
	// The reader loads the durable store, not the hub's in-flight writes, so
	// poll until the first belief is servable.
	if err := d.pollInspect(web.port, shopperA, belief(shopperA, markerA), true); err != nil {
		return err
	}
	for _, session := range []string{shopperA, shopperB, shopperC} {
		if err := d.pollInspect(web.port, session, belief(session, markers[session]), true); err != nil {
			return err
		}
		d.printf("-- GET /s/%s/api/inspect for its own belief: 200 found=true", shortSession(session))
	}
	found, err := d.readFound(web.port, shopperB, belief(shopperA, markerA))
	if err != nil {
		return err
	}
	if err := check(!found, "the reader served %s's belief in %s's window", shortSession(shopperA), shortSession(shopperB)); err != nil {
		return err
	}
	d.printf("-- GET /s/%s/api/inspect for %s's belief: 200 found=false", shortSession(shopperB), shortSession(shopperA))
	// The unscoped alias reads the default session, the pinned demo shopper,
	// which holds none of the test markers.
	alias, err := d.readFound(web.port, "", belief(shopperA, markerA))
	if err != nil {
		return err
	}
	if err := check(!alias, "the unscoped alias served a test belief (the default is not the demo)"); err != nil {
		return err
	}
	d.printf("-- GET /api/inspect for a test belief: 200 found=false (the default session is the demo shopper)")
	status, _, body, err := d.get(web.port, "/api/sessions")
	if err != nil {
		return fmt.Errorf("list the reader sessions: %w", err)
	}
	if err := check(status == 200, "GET /api/sessions: status %d", status); err != nil {
		return err
	}
	var listing struct {
		Sessions []string `json:"sessions"`
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		return fmt.Errorf("decode the session listing: %w", err)
	}
	if err := check(equalStrings(listing.Sessions, allowlist),
		"the reader listing is not the allowlist: %v", listing.Sessions); err != nil {
		return err
	}
	d.printf("-- GET /api/sessions -> %v", listing.Sessions)

	// A shopper created after the reader started is uniformly refused until
	// a restart names it (Lambo issue 241).
	late, err := d.attach(hub.port, shopperD, "Latefirth")
	if err != nil {
		return fmt.Errorf("create the late shopper: %w", err)
	}
	if _, err := late.recall("Latefirth"); err != nil {
		return fmt.Errorf("recall the late shopper's marker on the hub: %w", err)
	}
	d.say("-- late shopper %s attached on the hub after the reader started", shopperD)
	status, _, _, err = d.get(web.port, "/s/"+shopperD+"/api/inspect?focus="+url.QueryEscape(belief(shopperD, "Latefirth")))
	if err != nil {
		return fmt.Errorf("read the late session on the reader: %w", err)
	}
	if err := check(status == 404, "a session outside the allowlist answered %d, want the uniform 404", status); err != nil {
		return err
	}
	d.printf("-- GET /s/%s/api/inspect: 404 (uniform; the allowlist is start-time state)", shortSession(shopperD))
	if err := web.stop(); err != nil {
		return fmt.Errorf("stop the reader: %w", err)
	}
	d.printf("-- reader stopped")

	d.printf("\n== 5. the 503 + Retry-After refusal with every place full")
	if err := d.refusalProbe(); err != nil {
		return err
	}

	d.printf("\n== 6. --max-sessions 128 and --rate-limit-rps 200")
	if err := d.sessionCeilingProbe(hub.port); err != nil {
		return err
	}
	// The burst reuses the isolation phase's sessions: the ceiling probe
	// may have left the hub near its session bound, and opening new
	// sessions would measure that bound again instead of the rate budget.
	if err := d.rateProbe([]*mcp{clientA, clientB, clientC, demo}); err != nil {
		return err
	}
	if rss := rssMB(hub.cmd.Process.Pid); rss >= 0 {
		d.printf("-- hub RSS with 5 attached sessions: %.1f MiB (started at %.1f MiB)", rss, base)
	}

	if err := hub.stop(); err != nil {
		return fmt.Errorf("stop the hub: %w", err)
	}
	d.printf("\n-- hub stopped (one SIGTERM, then waited, so the write-behind log drains)")
	cleaned = true

	d.printf("\nmodel spend: $0.00 (no model call: the fixture embedder computes vectors in process)")
	d.printf("elastic: test-prefixed indices only, deleted below; no billable resource beyond the existing project")
	return nil
}

// pollInspect waits until the reader serves (or refuses) a focus.
func (d *driver) pollInspect(port int, session, focus string, want bool) error {
	path := "/s/" + session + "/api/inspect?focus=" + url.QueryEscape(focus)
	if session == "" {
		path = "/api/inspect?focus=" + url.QueryEscape(focus)
	}
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		status, _, body, err := d.get(port, path)
		if err == nil && status == 200 {
			var answer struct {
				Found bool `json:"found"`
			}
			if err := json.Unmarshal(body, &answer); err == nil && answer.Found == want {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return &fail{fmt.Sprintf("the reader never served %s with found=%v", shortSession(session), want)}
}

func (d *driver) readFound(port int, session, focus string) (bool, error) {
	path := "/s/" + session + "/api/inspect?focus=" + url.QueryEscape(focus)
	if session == "" {
		path = "/api/inspect?focus=" + url.QueryEscape(focus)
	}
	status, _, body, err := d.get(port, path)
	if err != nil {
		return false, fmt.Errorf("read the reader: %w", err)
	}
	if status != 200 {
		return false, &fail{fmt.Sprintf("reader answered %d, want 200", status)}
	}
	var answer struct {
		Found bool `json:"found"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return false, fmt.Errorf("decode the reader answer: %w", err)
	}
	return answer.Found, nil
}

// refusalProbe fills a max_attached=2 hub (the pinned demo plus one hot
// shopper) and records the refusal a further attach gets.
func (d *driver) refusalProbe() error {
	hub, err := d.startHub("hub503", filepath.Join(d.runDir, "lambo503.db"), 2, 200)
	if err != nil {
		return err
	}
	defer func() { _ = hub.stop() }()
	hot, err := d.attach(hub.port, probeHot, "Hotbrass")
	if err != nil {
		return fmt.Errorf("attach the hot session: %w", err)
	}
	d.say("-- second hub on 127.0.0.1:%d (max_attached=2: the pinned demo plus one shopper)", hub.port)
	// Keep the one place hot so the newcomer cannot evict it as idle; the
	// only honest answer is then the refusal.
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = hot.stats()
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()
	start := time.Now()
	status, headers, raw, callErr := func() (int, http.Header, []byte, error) {
		c := newMCP(hub.port, probeNew, d.token)
		c.nextID++
		return c.post(map[string]any{
			"jsonrpc": "2.0",
			"id":      c.nextID,
			"method":  "initialize",
			"params": map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{},
				"clientInfo":      map[string]any{"name": "memento-t81b", "version": "1"},
			},
		}, false)
	}()
	elapsed := time.Since(start)
	close(stop)
	wg.Wait()
	if callErr != nil {
		return fmt.Errorf("probe the full hub: %w", callErr)
	}
	retryAfter := headers.Get("Retry-After")
	d.printf("-- newcomer attach with every place full: HTTP %d in %s, Retry-After=%q", status, elapsed.Truncate(time.Microsecond), retryAfter)
	if err := check(status == 503, "a full hub answered %d (%s), want 503", status, excerpt(raw)); err != nil {
		return err
	}
	if err := check(retryAfter != "", "the 503 refusal carries no Retry-After"); err != nil {
		return err
	}
	// The hot session was not evicted to make room.
	if _, err := hot.stats(); err != nil {
		return fmt.Errorf("the hot session did not survive: %w", err)
	}
	d.printf("-- the hot session still answers; a caller must retry after %s seconds", retryAfter)
	return nil
}

// sessionCeilingProbe opens more concurrent MCP sessions than the default
// ceiling of 32, proving the raised --max-sessions 128 holds them. The five
// sessions the earlier phases opened stay live, so the bound engages part
// way through the burst; the probe names that split exactly.
func (d *driver) sessionCeilingProbe(port int) error {
	const alreadyLive = 5 // alpha, bravo, charlie, demo, late
	const want = 130
	type outcome struct {
		status int
		err    error
	}
	results := make([]outcome, want)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < want; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			session := []string{shopperA, shopperB, shopperC}[i%3]
			c := newMCP(port, session, d.token)
			if err := c.initialize(); err != nil {
				if strings.Contains(err.Error(), "HTTP 503") {
					results[i] = outcome{status: 503}
					return
				}
				results[i] = outcome{err: err}
				return
			}
			results[i] = outcome{status: 200}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)
	var ok, refused, other int
	var firstErr error
	for _, r := range results {
		switch {
		case r.status == 200:
			ok++
		case r.status == 503:
			refused++
		default:
			other++
			if firstErr == nil && r.err != nil {
				firstErr = r.err
			}
		}
	}
	d.printf("-- %d concurrent initialize with %d already live: %dx200, %dx503, %d other in %s", want, alreadyLive, ok, refused, other, elapsed.Round(time.Millisecond))
	if other > 0 && firstErr != nil {
		return fmt.Errorf("the ceiling probe errored: %w", d.scrubErr(firstErr))
	}
	if err := check(ok > 32, "only %d concurrent sessions held; the default ceiling of 32 still binds", ok); err != nil {
		return err
	}
	if refused > 0 {
		d.printf("-- the 503s past the ceiling are the bound working: %d refused of %d (ceiling 128)", refused, want)
	} else {
		d.printf("-- no refusal at %d concurrent sessions: the ceiling holds at least this many (flag is 128)", want)
	}
	return nil
}

func (d *driver) scrubErr(err error) error {
	return fmt.Errorf("%s", d.scrub.clean(err.Error()))
}

// rateProbe fires a concurrent burst at the hub over already-live MCP
// sessions (opening new ones would spend the session ceiling, not the rate
// budget) and records the service rate and any throttling under
// --rate-limit-rps 200 (burst 2x).
func (d *driver) rateProbe(shared []*mcp) error {
	const workers = 8
	const perWorker = 75
	type outcome struct {
		ok, throttled, other int
	}
	results := make([]outcome, workers)
	var wg sync.WaitGroup
	start := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			c := shared[w%len(shared)]
			for i := 0; i < perWorker; i++ {
				status, _, _, err := c.rawCall("lambo_stats", map[string]any{"agent_id": agentName})
				switch {
				case err != nil:
					results[w].other++
				case status == 200:
					results[w].ok++
				case status == 429:
					results[w].throttled++
				default:
					results[w].other++
				}
			}
		}(w)
	}
	wg.Wait()
	elapsed := time.Since(start)
	var ok, throttled, other int
	for _, r := range results {
		ok += r.ok
		throttled += r.throttled
		other += r.other
	}
	total := float64(ok + throttled + other)
	rps := total / elapsed.Seconds()
	d.printf("-- %d stats calls over %d workers: %d ok, %d throttled (429), %d other in %s = %.0f req/s served",
		int(total), workers, ok, throttled, other, elapsed.Round(time.Millisecond), rps)
	if other > 0 {
		return &fail{fmt.Sprintf("%d calls failed outside throttling", other)}
	}
	if throttled > 0 {
		d.printf("-- %d calls met the 200rps ceiling with burst 2x: the limiter is the bound, not the machine", throttled)
	} else {
		d.printf("-- no call throttled: the hub served %.0f req/s without touching the 200rps ceiling", rps)
	}
	return nil
}

// cleanup stops nothing (children are stopped inline) and always deletes the
// test indices, shows the empty listing, and removes the run directory.
func (d *driver) cleanup(hubStopped bool) error {
	_ = hubStopped
	if d.runDir == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	before, err := d.store.indices(ctx)
	if err != nil {
		return err
	}
	for _, name := range before {
		if !strings.HasPrefix(name, indexPrefix) {
			return &fail{fmt.Sprintf("refusing to delete %q: it is not a %s index", name, indexPrefix)}
		}
	}
	for _, name := range before {
		if err := d.store.deleteIndex(ctx, name); err != nil {
			return err
		}
	}
	d.printf("\n-- cleanup: deleted %d index(es) under %s", len(before), indexPrefix)
	after, err := d.store.indices(ctx)
	if err != nil {
		return err
	}
	d.printf("-- prefix listing after: GET /_cat/indices/%s* -> %v", indexPrefix, after)
	if err := check(len(after) == 0, "%d index(es) remain under %s: %v", len(after), indexPrefix, after); err != nil {
		return err
	}
	if err := os.RemoveAll(d.runDir); err != nil {
		return fmt.Errorf("remove the run dir: %w", err)
	}
	d.printf("-- run dir removed (store, configs, child logs); nothing keyed survives locally")
	return nil
}

func shortHash(hash string) string {
	if len(hash) <= 16 {
		return hash
	}
	return hash[:16] + "…"
}

func shortSession(session string) string {
	return strings.TrimPrefix(session, "shopper-t81b-")
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func main() {
	if err := run(context.Background()); err != nil {
		redacted := err.Error()
		for _, secret := range []string{os.Getenv("ELASTIC_URL"), os.Getenv("LAMBO_ES_API_KEY"), os.Getenv("MEMENTO_IDENTITY_SALT")} {
			if secret != "" {
				redacted = strings.ReplaceAll(redacted, secret, "<redacted>")
			}
		}
		fmt.Fprintf(os.Stderr, "FAIL T8.1b live hub: %s\n", redacted)
		os.Exit(1)
	}
	fmt.Println("\nPASS T8.1b live hub: three shoppers isolated, the reader allowlisted, the 503 recorded, the ceilings measured, the namespace erased")
}
