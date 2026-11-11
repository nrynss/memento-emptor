// Command memento runs the memory layer: eleven frozen tools over MCP and
// plain HTTP, the call log, and payment webhook ingest. It boots on the
// recording fakes with no key and no network, so the demo story replays
// offline; the live swap replaces the fakes behind the same seams and
// touches no handler. With -seed it loads the frozen demo history first,
// so the ledger has rows before any live event arrives.
//
// The live swap is environment-driven and partial by design: every piece
// wires only when its variables are present, and the offline shape stays
// byte-identical when they are absent.
//
//   - MEMENTO_IDENTITY_SALT (64 hex characters) builds the identity keyer
//     behind the fit writer and the hub session names.
//   - ELASTIC_URL with LAMBO_ES_API_KEY opens the live fit index and the
//     pool reader over MEMENTO_FIT_INDEX (default "fit-rows").
//   - LAMBO_MCP_URL with LAMBO_MCP_TOKEN replaces the recording graph with
//     a hub-backed client: one hub MCP session per shopper, named by the
//     identity keyer, so the hub's session isolation keeps shoppers apart.
//     The layer holds one Service per shopper for exactly this reason: the
//     graph seam carries no shopper, and sharing one hub session across
//     shoppers would promote one shopper's beliefs from another's returns.
//
// With "key issue" it mints one caller key instead of serving: the token
// is printed once, on stdout, and the entry the deployment holds goes to
// stderr. The deployment keeps only the entry; the token is never stored.
//
// With "session" it prints one shopper's hub session name under the live
// salt, so the boot script can pin and read exactly the session the layer
// serves. A session name is a keyed hash, not a secret, and printing it
// reveals nothing that derives it.
//
// With -seed it loads the frozen demo history first,
// so the ledger has rows before any live event arrives.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/server"
	"github.com/nrynss/memento-emptor/internal/auth"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
	"github.com/nrynss/memento-emptor/internal/pool"
	"github.com/nrynss/memento-emptor/seed"
)

// staticContract names the embedding contract the demo store is built
// under. It carries a kind and nothing else: every session built under
// it matches or re-embeds.
type staticContract struct {
	kind string
}

// EmbeddingKind names the contract family.
func (c staticContract) EmbeddingKind() string { return c.kind }

// Validate reports whether the contract is usable.
func (c staticContract) Validate() error {
	if c.kind == "" {
		return fmt.Errorf("%w: demo contract names no kind", domain.ErrEmptyEmbedderKind)
	}
	return nil
}

// liveFitIndex is the production fit index name. A test run points the
// layer and the seeder at a scratch prefix through MEMENTO_FIT_INDEX and
// deletes it afterwards; production never takes a test prefix.
const liveFitIndex = "fit-rows"

// hubAgent is the MCP agent identity the layer writes as on the hub.
const hubAgent = "memento-layer"

// receiptWait bounds one hub write's receipt wait. The daemon applies a
// write in the background and the ack carries a receipt; the call that
// wrote it blocks until the receipt applies.
const receiptWait = 5 * time.Minute

// receiptPoll is the per-call wait carried on one lambo_stats poll.
const receiptPoll = 60000

// attachAttempts bounds how many times one call retries an attach the
// hub's bound refused, so a full hub costs a handful of requests and one
// stated wait each, never a retry storm.
const attachAttempts = 3

// attachRetryDefault is the wait used when a refusal carries no usable
// Retry-After: one second.
const attachRetryDefault = time.Second

// errAttachBound reports the hub refusing a session attach because every
// max_attached place is taken. The client retries it after the delay the
// hub stated; when the refusal persists, the last attempt returns an
// error matching errors.Is(err, errAttachBound).
var errAttachBound = errors.New("memento: hub attach bound refused the session")

// attachBoundError carries one attach refusal: the call that met it, the
// status, the delay the hub asked for and an excerpt of its body.
type attachBoundError struct {
	tool       string
	status     int
	retryAfter time.Duration
	excerpt    string
}

// Error renders the refusal with the wait the client honours.
func (e *attachBoundError) Error() string {
	return fmt.Sprintf("memento: hub %s: attach bound: HTTP %d, retry after %s: %s",
		e.tool, e.status, e.retryAfter, e.excerpt)
}

// Unwrap reports the sentinel every attach refusal matches.
func (e *attachBoundError) Unwrap() error { return errAttachBound }

// attachRetry reads the Retry-After header as whole seconds, falling back
// to attachRetryDefault when the header is absent, names a date, or is not
// a count of seconds.
func attachRetry(header http.Header) time.Duration {
	raw := strings.TrimSpace(header.Get("Retry-After"))
	if raw == "" {
		return attachRetryDefault
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 0 {
		return attachRetryDefault
	}
	return time.Duration(seconds) * time.Second
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "key" {
		if err := runKey(os.Args[2:]); err != nil {
			log.Printf("memento: %v", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "session" {
		if err := runSession(os.Args[2:]); err != nil {
			log.Printf("memento: %v", err)
			os.Exit(1)
		}
		return
	}
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on")
	schemaDir := flag.String("schema-dir", "testdata/api", "directory holding the frozen request schemas")
	jevDir := flag.String("jev-dir", "testdata/jev", "directory holding the classifier recordings")
	seedFlag := flag.Bool("seed", false, "load the frozen demo history before serving")
	shopper := flag.String("shopper", "shopper_01", "shopper the seed loads for")
	standingDir := flag.String("standing-dir", "", "directory holding the per-shopper standing journals (empty keeps them in memory)")
	fitIndex := flag.String("fit-index", envOr("MEMENTO_FIT_INDEX", liveFitIndex), "fit index the pool reader and fit writer use")
	flag.Parse()

	if err := run(*addr, *schemaDir, *jevDir, *seedFlag, *shopper, *standingDir, *fitIndex); err != nil {
		log.Printf("memento: %v", err)
		os.Exit(1)
	}
}

// runKey mints one caller key: "key issue" with a kind, an id, a seller
// and the shoppers it may act for. The token is printed once, on stdout;
// the entry the deployment holds is printed on stderr. The binding is
// validated by the same check the edge parses, so an issued entry always
// parses.
func runKey(args []string) error {
	if len(args) == 0 || args[0] != "issue" {
		return fmt.Errorf("usage: memento key issue --kind agent|shopper|merchant --id ID --seller SELLER [--shoppers A,B]")
	}
	flags := flag.NewFlagSet("key issue", flag.ContinueOnError)
	kind := flags.String("kind", "", "caller kind: agent, shopper or merchant")
	id := flags.String("id", "", "deployment id for the key")
	seller := flags.String("seller", "", "seller whose layer issues the key")
	shoppers := flags.String("shoppers", "", "comma-separated shoppers the key may act for")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("usage: memento key issue --kind agent|shopper|merchant --id ID --seller SELLER [--shoppers A,B]")
	}
	var caller auth.CallerKind
	switch *kind {
	case "agent":
		caller = auth.CallerAgent
	case "shopper":
		caller = auth.CallerShopper
	case "merchant":
		caller = auth.CallerMerchant
	default:
		return fmt.Errorf("memento: unknown key kind %q", *kind)
	}
	var bound []string
	if *shoppers != "" {
		for _, shopper := range splitList(*shoppers) {
			if shopper != "" {
				bound = append(bound, shopper)
			}
		}
	}
	token, entry, err := auth.IssueKey(caller, *id, *seller, bound)
	if err != nil {
		return fmt.Errorf("memento: issue key: %w", err)
	}
	fmt.Println(token)
	fmt.Fprintln(os.Stderr, entry)
	return nil
}

// runSession prints one shopper's hub session name under the live salt
// from MEMENTO_IDENTITY_SALT. The boot script pins and reads exactly the
// session the layer serves. The name is a keyed hash, not the salt, so
// printing it leaks nothing that derives another name.
func runSession(args []string) error {
	flags := flag.NewFlagSet("session", flag.ContinueOnError)
	shopper := flags.String("shopper", "", "shopper id to name the hub session for")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 || *shopper == "" {
		return fmt.Errorf("usage: memento session --shopper SHOPPER")
	}
	keyer, err := identity.NewKeyer(os.Getenv("MEMENTO_IDENTITY_SALT"))
	if err != nil {
		return fmt.Errorf("memento: identity keyer: %w", err)
	}
	name, err := keyer.SessionName(*shopper)
	if err != nil {
		return fmt.Errorf("memento: session name for %q: %w", *shopper, err)
	}
	fmt.Println(name)
	return nil
}

// splitList splits a comma-separated flag value.
func splitList(raw string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i == len(raw) || raw[i] == ',' {
			out = append(out, raw[start:i])
			start = i + 1
		}
	}
	return out
}

// envOr reads one variable or returns the fallback.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// hubWired reports whether the hub client has what it needs: the hub's
// loopback URL and the credential the hub accepts. Both travel in the
// environment, never in a file, and the token is never printed.
func hubWired() bool {
	return os.Getenv("LAMBO_MCP_URL") != "" && os.Getenv("LAMBO_MCP_TOKEN") != ""
}

// elasticWired reports whether the fit and pool clients have what they
// need: the deployment URL and its API key.
func elasticWired() bool {
	return os.Getenv("ELASTIC_URL") != "" && os.Getenv("LAMBO_ES_API_KEY") != ""
}

func run(addr, schemaDir, jevDir string, seedFlag bool, shopper, standingDir, fitIndex string) error {
	if hubWired() {
		if os.Getenv("MEMENTO_IDENTITY_SALT") == "" {
			return fmt.Errorf("memento: LAMBO_MCP_URL and LAMBO_MCP_TOKEN need MEMENTO_IDENTITY_SALT: the hub sessions are named under it")
		}
		return runPivoted(addr, schemaDir, jevDir, seedFlag, shopper, standingDir, fitIndex)
	}
	if os.Getenv("MEMENTO_IDENTITY_SALT") != "" || elasticWired() {
		log.Print("memento: pool and fit stay unwired without the hub: set LAMBO_MCP_URL and LAMBO_MCP_TOKEN to serve the pivoted layer")
	}
	return runOffline(addr, schemaDir, jevDir, seedFlag, shopper, standingDir)
}

func runOffline(addr, schemaDir, jevDir string, seedFlag bool, shopper, standingDir string) error {
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir)
	if err != nil {
		return err
	}
	store, err := evidence.NewStore(staticContract{kind: "offline-demo"})
	if err != nil {
		return err
	}
	var seed server.Seed
	if seedFlag {
		loaded, err := server.SeedFromExamples(schemaDir)
		if err != nil {
			return err
		}
		loaded.ShopperID = shopper
		seed = loaded
	}
	graphContents := fakes.GraphContents()
	if seedFlag {
		graphContents = append(graphContents, server.SeedGraphContents(seed)...)
	}
	svc, err := server.NewService(server.Deps{
		Payments:   fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      fakes.NewRecordingEvidenceGraph(graphContents, fakes.StoryRecalls()),
		SchemaDir:  schemaDir,
		Clock:      func() time.Time { return time.Now().UTC() },
		// The demo leaves the pool unwired: no index, no salt and no
		// catalog adapter, so no pool fields and no fit rows. A deploy
		// wires them; the standing directory alone decides whether the
		// journals survive a restart.
		StandingDir: standingDir,
	})
	if err != nil {
		return err
	}
	if seedFlag {
		if err := svc.LoadSeed(context.Background(), seed); err != nil {
			return err
		}
	}
	httpServer := &http.Server{Addr: addr, Handler: svc.Handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("memento: listening on %s (seed=%v)", addr, seedFlag)
	return httpServer.ListenAndServe()
}

// runPivoted serves the pivoted layer: the pool reader and the fit writer
// over the live fit index, the identity keyer over the live salt, the
// static catalog resolver, the standing directory on disk, and one
// hub-backed graph session per shopper. The pool reader and the fit writer
// wire only when Elastic answers; without them recall serves memory and
// graph alone, and product_fit refuses rather than report an empty pool.
func runPivoted(addr, schemaDir, jevDir string, seedFlag bool, shopper, standingDir, fitIndex string) error {
	ctx := context.Background()
	keyer, err := identity.NewKeyer(os.Getenv("MEMENTO_IDENTITY_SALT"))
	if err != nil {
		return fmt.Errorf("memento: identity keyer: %w", err)
	}
	var poolIdx pool.Index
	var fitIdx fitindex.Index
	if elasticWired() {
		poolClient, err := pool.NewHTTPIndex(os.Getenv("ELASTIC_URL"), os.Getenv("LAMBO_ES_API_KEY"), fitIndex)
		if err != nil {
			return fmt.Errorf("memento: pool index: %w", err)
		}
		fitClient, err := fitindex.NewHTTPIndex(os.Getenv("ELASTIC_URL"), os.Getenv("LAMBO_ES_API_KEY"), fitIndex)
		if err != nil {
			return fmt.Errorf("memento: fit index: %w", err)
		}
		if err := fitClient.Create(ctx); err != nil {
			log.Printf("memento: WARNING create fit index %q: %v; reads and writes fail until it answers", fitIndex, err)
		} else if err := poolClient.Warm(ctx); err != nil {
			log.Printf("memento: WARNING warm pool connection: %v; the first recommendation pays the handshake", err)
		} else {
			log.Printf("memento: pool and fit wired over index %q", fitIndex)
		}
		poolIdx, fitIdx = poolClient, fitClient
	} else {
		log.Print("memento: WARNING no Elastic key in the environment: the pool reads nothing and no fit row is written")
	}
	lines, err := staticCatalogLines()
	if err != nil {
		return err
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir)
	if err != nil {
		return err
	}
	shared := pivotedShared{
		schemaDir:  schemaDir,
		classifier: classifier,
		pool:       poolIdx,
		fitRows:    fitIdx,
		keyer:      keyer,
		lines:      lines,
		standing:   standingDir,
	}
	director, err := newDirector(shared)
	if err != nil {
		return err
	}
	if seedFlag {
		loaded, err := server.SeedFromExamples(schemaDir)
		if err != nil {
			return err
		}
		loaded.ShopperID = shopper
		svc, err := director.forShopper(shopper)
		if err != nil {
			return err
		}
		// The seed replays through the hub session, one receipt per
		// write, so the first boot after a hub reset takes as long as
		// the writes take to apply. A hub that is not up yet fails
		// here, loudly, and the supervisor restarts the service.
		if err := svc.LoadSeed(ctx, loaded); err != nil {
			return fmt.Errorf("memento: load seed for %q: %w", shopper, err)
		}
		log.Printf("memento: seed holds for shopper %q", shopper)
	}
	httpServer := &http.Server{Addr: addr, Handler: director.handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("memento: listening on %s (seed=%v, pool=%v, fit=%v)", addr, seedFlag, poolIdx != nil, fitIdx != nil)
	return httpServer.ListenAndServe()
}

// pivotedShared carries the pieces every shopper's service builds over:
// the pool and fit indexes, the identity keyer, the line resolver and
// the recording seams. The evidence store is per service, like the hub
// session behind it, so two shoppers never share a doc id sequence.
type pivotedShared struct {
	schemaDir  string
	classifier fakes.ReasonClassifier
	pool       pool.Index
	fitRows    fitindex.Index
	keyer      *identity.Keyer
	lines      fitindex.Lines
	standing   string
}

// director routes each request to one Service per shopper, creating it on
// first use. One service per shopper is how the hub sessions stay apart:
// the graph seam carries no shopper, so sharing one service across
// shoppers would share one hub session and promote one shopper's beliefs
// from another's returns. The default service answers only what names no
// shopper: tools/list, product_fit, health and the aggregated call log.
// Its graph is a fake that nothing can reach, because none of those paths
// writes to it; a write there would fail closed instead of inventing a
// session.
type director struct {
	mu       sync.Mutex
	shared   pivotedShared
	services map[string]*server.Service
	order    []string
	defaultS *server.Service
}

// newDirector builds the router with its shopper-less default service.
func newDirector(shared pivotedShared) (*director, error) {
	d := &director{shared: shared, services: map[string]*server.Service{}}
	svc, err := d.newService("")
	if err != nil {
		return nil, err
	}
	d.defaultS = svc
	return d, nil
}

// newService builds one shopper's service over its own hub session and
// its own evidence store. The default service (empty shopper) carries a
// graph nothing can reach instead of a hub session nothing should create.
func (d *director) newService(shopper string) (*server.Service, error) {
	var graph fakes.EvidenceGraph = fakes.NewRecordingEvidenceGraph(nil, nil)
	if shopper != "" {
		session, err := d.shared.keyer.SessionName(shopper)
		if err != nil {
			return nil, fmt.Errorf("memento: session for %q: %w", shopper, err)
		}
		graph = newHubGraph(os.Getenv("LAMBO_MCP_URL"), os.Getenv("LAMBO_MCP_TOKEN"), session)
	}
	store, err := evidence.NewStore(staticContract{kind: "memento-live"})
	if err != nil {
		return nil, err
	}
	svc, err := server.NewService(server.Deps{
		Payments:     fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier:   d.shared.classifier,
		Store:        store,
		Graph:        graph,
		SchemaDir:    d.shared.schemaDir,
		Clock:        func() time.Time { return time.Now().UTC() },
		Pool:         d.shared.pool,
		FitRows:      d.shared.fitRows,
		IdentitySalt: os.Getenv("MEMENTO_IDENTITY_SALT"),
		Lines:        d.shared.lines,
		StandingDir:  d.shared.standing,
	})
	if err != nil {
		return nil, fmt.Errorf("memento: service for %q: %w", shopper, err)
	}
	return svc, nil
}

// forShopper returns the shopper's service, creating it on first use. The
// caller holds no lock; creation serializes on the director's.
func (d *director) forShopper(shopper string) (*server.Service, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if svc, ok := d.services[shopper]; ok {
		return svc, nil
	}
	svc, err := d.newService(shopper)
	if err != nil {
		return nil, err
	}
	d.services[shopper] = svc
	d.order = append(d.order, shopper)
	return svc, nil
}

// handler returns the routing handler: every request reaches exactly one
// service's frozen handler. The edge already scoped the caller to a
// shopper where one applies and says so in X-Memento-Shopper; a request
// that arrives without it (a local boot past the edge) is routed from the
// body's own shopper id. A body that names no shopper, or names one the
// route does not take, reaches the default service, which answers the
// frozen error exactly as the single service would.
func (d *director) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp", d.handleMCP)
	mux.HandleFunc("POST /tools/{tool}", d.handleTool)
	mux.HandleFunc("GET /calls", d.handleCalls)
	mux.HandleFunc("POST /webhooks/payment", d.handleWebhook)
	mux.HandleFunc("GET /healthz", d.defaultS.Handler().ServeHTTP)
	return mux
}

// route picks the service for one request: the header the guard set when
// it scoped the caller, else the body's own shopper id, else the default
// service. Creation failure answers 500: the deps are fixed at boot, so a
// failure here is a defect, not a caller error.
func (d *director) route(w http.ResponseWriter, r *http.Request, bodyShopper func([]byte) string, raw []byte) *server.Service {
	if shopper := r.Header.Get("X-Memento-Shopper"); shopper != "" {
		svc, err := d.forShopper(shopper)
		if err != nil {
			http.Error(w, "memento: no service", http.StatusInternalServerError)
			return nil
		}
		return svc
	}
	if shopper := bodyShopper(raw); shopper != "" {
		svc, err := d.forShopper(shopper)
		if err != nil {
			http.Error(w, "memento: no service", http.StatusInternalServerError)
			return nil
		}
		return svc
	}
	return d.defaultS
}

// handleMCP routes one MCP request. Tools without a shopper
// (tools/list, initialize, the initialized notification) and product_fit,
// which names a line and not a shopper, reach the default service. Every
// other tools/call routes on its arguments' shopper id.
func (d *director) handleMCP(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "memento: unreadable body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	svc := d.route(w, r, mcpShopper, raw)
	if svc == nil {
		return
	}
	svc.Handler().ServeHTTP(w, r)
}

// handleTool routes one plain tool call on its body's shopper id, or the
// default service when the tool names none.
func (d *director) handleTool(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "memento: unreadable body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if r.PathValue("tool") == "product_fit" && r.Header.Get("X-Memento-Shopper") == "" {
		d.defaultS.Handler().ServeHTTP(w, r)
		return
	}
	svc := d.route(w, r, bodyShopper, raw)
	if svc == nil {
		return
	}
	svc.Handler().ServeHTTP(w, r)
}

// handleWebhook routes one payment delivery on its resource custom id,
// which is the shopper join the reference agent sets at order creation.
func (d *director) handleWebhook(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "memento: unreadable body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	svc := d.route(w, r, webhookShopper, raw)
	if svc == nil {
		return
	}
	svc.Handler().ServeHTTP(w, r)
}

// handleCalls answers the call log across every service that has served
// one, the default first in creation order. Each service numbers its own
// log from 1; the demo reads one shopper's entries, which ride together.
func (d *director) handleCalls(w http.ResponseWriter, r *http.Request) {
	trace := r.URL.Query().Get("trace_id")
	d.mu.Lock()
	services := make([]*server.Service, 0, len(d.order)+1)
	services = append(services, d.defaultS)
	for _, shopper := range d.order {
		services = append(services, d.services[shopper])
	}
	d.mu.Unlock()
	out := []server.CallEntry{}
	for _, svc := range services {
		out = append(out, svc.Calls(trace)...)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"calls": out})
}

// mcpShopper returns the shopper one MCP request is about: the
// tools/call arguments' shopper id, or empty when the request names no
// tool call or no shopper. Anything unparseable routes to the default
// service, which answers the frozen error.
func mcpShopper(raw []byte) string {
	var outer struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil || outer.Method != "tools/call" {
		return ""
	}
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(outer.Params, &call); err != nil || call.Name == "product_fit" {
		return ""
	}
	return bodyShopper(call.Arguments)
}

// bodyShopper returns the top-level shopper id in one tool body, or empty
// when the body names none or does not parse. The guard already checked a
// scoped caller against X-Memento-Shopper, so this is routing, not auth.
func bodyShopper(raw []byte) string {
	var body struct {
		ShopperID string `json:"shopper_id"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return body.ShopperID
}

// webhookShopper returns the shopper one delivery belongs to from its
// resource custom id, or empty when the body does not parse. The service
// refuses a delivery without one; routing only decides which service
// answers that refusal.
func webhookShopper(raw []byte) string {
	var body struct {
		Resource struct {
			CustomID string `json:"custom_id"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return body.Resource.CustomID
}

// lineKey names one product line the static catalog knows: the brand and
// category as the order item carries them. The fit handle rides along as
// the value, never as part of the key.
type lineKey struct {
	brand    string
	category string
}

// staticLines resolves an order item to its product line from the seeded
// population's line list, the one source every writer call resolves
// through. The brand and category always come from the item itself; the
// table only supplies the fit handle the item never carries. An item on a
// line the population never seeded resolves to the same brand and
// category with the regular handle, so ingest stays total: that purchase
// pools under its own line, which can never pollute a seeded line because
// the brand and category are the item's own. An item that names no brand
// or category fails instead: a row that would match more than one line
// must never pool.
type staticLines struct {
	handles map[lineKey]string
}

// staticCatalogLines builds the resolver from the seeded population's
// lines. Two handles for one brand and category would pool one purchase
// under two lines, so the build refuses rather than pick one.
func staticCatalogLines() (*staticLines, error) {
	out := &staticLines{handles: map[lineKey]string{}}
	for _, line := range seed.PopulationLines() {
		key := lineKey{brand: line.Brand, category: line.Category}
		if known, ok := out.handles[key]; ok && known != line.FitHandle {
			return nil, fmt.Errorf("memento: catalog line %q/%q names two fit handles %q and %q",
				line.Brand, line.Category, known, line.FitHandle)
		}
		out.handles[key] = line.FitHandle
	}
	return out, nil
}

// ProductLine returns the brand, category and fit handle for one item.
func (s *staticLines) ProductLine(ctx context.Context, item domain.Item) (domain.ProductLine, error) {
	if err := ctx.Err(); err != nil {
		return domain.ProductLine{}, err
	}
	if strings.TrimSpace(item.BrandID) == "" || strings.TrimSpace(item.Category) == "" {
		return domain.ProductLine{}, fmt.Errorf("memento: catalog line for item %q names no brand or category", item.SKU)
	}
	if handle, ok := s.handles[lineKey{brand: item.BrandID, category: item.Category}]; ok {
		return domain.ProductLine{Brand: item.BrandID, Category: item.Category, FitHandle: handle}, nil
	}
	return domain.ProductLine{Brand: item.BrandID, Category: item.Category, FitHandle: "regular"}, nil
}

// hubGraph is a fakes.EvidenceGraph over one live hub session. Every write
// travels with its event time so honestly spread history promotes, and
// every write waits its receipt until the daemon applies it. The MCP
// connection opens lazily on first use and re-opens once after a failure,
// because the hub detaches an idle session and the client session with it.
type hubGraph struct {
	baseURL string
	token   string
	session string

	mu         sync.Mutex
	httpClient *http.Client
	mcpSession string
	nextID     int
	// wait spends one attach-bound delay and stops early when the context
	// ends. It is a field so a test can record the delay the client
	// honours without spending it.
	wait func(context.Context, time.Duration) error
}

// newHubGraph builds the graph client for one hub session: the hub's
// loopback URL, the credential the hub accepts, and the session name the
// identity keyer built for the shopper.
func newHubGraph(baseURL, token, session string) *hubGraph {
	return &hubGraph{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		session:    session,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		wait:       waitContext,
	}
}

// waitContext sleeps for one attach-bound delay and returns the context's
// error when the context ends first.
func waitContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Derive records one belief or statement.
func (g *hubGraph) Derive(ctx context.Context, content, kind string, eventTime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := map[string]any{
		"agent_id": hubAgent,
		"concepts": []map[string]any{{"content": content, "concept_type": kind}},
	}
	if !eventTime.IsZero() {
		args["event_time"] = eventTime.UTC().Format(time.RFC3339Nano)
	}
	receipt, err := g.callTool(ctx, "lambo_derive", args)
	if err != nil {
		return err
	}
	return g.waitApplied(ctx, receipt, fmt.Sprintf("derive %.40q", content))
}

// RecordAction records one structural write.
func (g *hubGraph) RecordAction(ctx context.Context, action string, dependsOn, against []string, eventTime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := map[string]any{"agent_id": hubAgent, "action": action}
	if len(dependsOn) > 0 {
		args["depends_on"] = dependsOn
	}
	if len(against) > 0 {
		args["against"] = against
	}
	if !eventTime.IsZero() {
		args["event_time"] = eventTime.UTC().Format(time.RFC3339Nano)
	}
	receipt, err := g.callTool(ctx, "lambo_record_action", args)
	if err != nil {
		return err
	}
	return g.waitApplied(ctx, receipt, fmt.Sprintf("record_action %.40q", action))
}

// Recall lists the session's hit labels for one query. The layer ranks
// from its stored beliefs and never calls this; it exists so the client
// satisfies the seam honestly.
func (g *hubGraph) Recall(ctx context.Context, query string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := g.call(ctx, "lambo_recall", map[string]any{"agent_id": hubAgent, "query": query})
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, line := range strings.Split(hubTexts(result), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out, nil
}

// Inspect reports the promotion state of one belief. A focus nothing
// matches reads as None with no error: the layer only inspects contents
// it derived, and an unpromoted note is None, not a failure.
func (g *hubGraph) Inspect(ctx context.Context, content string) (fakes.BeliefReport, error) {
	if err := ctx.Err(); err != nil {
		return fakes.BeliefReport{}, err
	}
	result, err := g.call(ctx, "lambo_inspect", map[string]any{
		"agent_id": hubAgent, "focus": content, "depth": 2,
	})
	if err != nil {
		if strings.Contains(err.Error(), "no concept matching") {
			return fakes.BeliefReport{Content: content, State: domain.GraphNone}, nil
		}
		return fakes.BeliefReport{}, err
	}
	if strings.Contains(hubTexts(result), "no concept matching") {
		return fakes.BeliefReport{Content: content, State: domain.GraphNone}, nil
	}
	report := fakes.BeliefReport{Content: content}
	switch strings.ToLower(fmt.Sprint(navigate(result, "structuredContent", "admission", "status"))) {
	case "venerable":
		report.State = domain.GraphVenerable
	case "candidate":
		report.State = domain.GraphCandidate
	default:
		report.State = hubStateOf(hubTexts(result))
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
		words := make([]string, 0, len(warnings))
		for _, warning := range warnings {
			words = append(words, fmt.Sprint(warning))
		}
		report.Warning = "Contradicted by " + strings.Join(words, "; ")
	}
	return report, nil
}

// hubStateOf falls back to the inspect text when the hub answers without
// a structured admission: the seeded driver waits on the same mark.
func hubStateOf(text string) domain.GraphState {
	switch {
	case strings.Contains(text, "[Logic, venerable]"):
		return domain.GraphVenerable
	case strings.Contains(text, "[Logic, candidate]"):
		return domain.GraphCandidate
	default:
		return domain.GraphNone
	}
}

// navigate walks a decoded result envelope by key, returning nil when any
// step is missing. A hub answer that omits a field reads as the zero of
// that field, never as an error.
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

// callTool makes one hub tools/call and returns its write receipt.
func (g *hubGraph) callTool(ctx context.Context, name string, args map[string]any) (string, error) {
	result, err := g.call(ctx, name, args)
	if err != nil {
		return "", err
	}
	if structured, ok := result["structuredContent"].(map[string]any); ok {
		if receipt, ok := structured["receipt"].(string); ok && receipt != "" {
			return receipt, nil
		}
	}
	return "", fmt.Errorf("memento: %s issued no receipt: %.200s", name, hubTexts(result))
}

// waitApplied blocks until the daemon applies one receipt. A write the
// daemon refused fails the call; anything else waits out the receipt.
func (g *hubGraph) waitApplied(ctx context.Context, receipt, what string) error {
	deadline := time.Now().Add(receiptWait)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		result, err := g.call(ctx, "lambo_stats", map[string]any{
			"agent_id": hubAgent, "receipt": receipt, "wait_ms": receiptPoll,
		})
		if err != nil {
			return fmt.Errorf("memento: wait %s: %w", what, err)
		}
		text := hubTexts(result)
		switch {
		case strings.Contains(text, "applied"):
			return nil
		case strings.Contains(text, "failed"), strings.Contains(text, "refused"), strings.Contains(text, "embedder unavailable"):
			return fmt.Errorf("memento: %s not applied: %.200s", what, text)
		}
	}
	return fmt.Errorf("memento: receipt for %s did not settle", what)
}

// call makes one hub tools/call, opening the MCP connection on first use,
// re-opening it once after a stale session fails, and retrying an attach
// the hub's bound refused after the delay the hub states. The hub detaches
// idle sessions, so a stale client session fails once and the retry
// carries on. A full hub answers 503 with Retry-After: only the open can
// relieve it, so the wait is followed by another attempt. attachAttempts
// bounds those waits, and a refusal that persists returns an error
// matching errAttachBound.
func (g *hubGraph) call(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for attempt := 1; ; attempt++ {
		result, err := g.callOnceLocked(ctx, name, args)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, errAttachBound) || attempt >= attachAttempts {
			return nil, err
		}
		if err := g.waitBound(ctx, err); err != nil {
			return nil, err
		}
	}
}

// callOnceLocked runs one call over the held client session, opening it
// when none is held and re-opening it once when a request fails for a
// reason other than the attach bound.
func (g *hubGraph) callOnceLocked(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	if err := g.ensureInitLocked(ctx); err != nil {
		return nil, err
	}
	result, err := g.postLocked(ctx, name, args)
	if err == nil {
		return result, nil
	}
	g.mcpSession = ""
	if errors.Is(err, errAttachBound) {
		return nil, err
	}
	if err := g.ensureInitLocked(ctx); err != nil {
		return nil, err
	}
	return g.postLocked(ctx, name, args)
}

// waitBound spends the delay one attach refusal stated.
func (g *hubGraph) waitBound(ctx context.Context, err error) error {
	delay := attachRetryDefault
	var bound *attachBoundError
	if errors.As(err, &bound) {
		delay = bound.retryAfter
	}
	if g.wait == nil {
		return waitContext(ctx, delay)
	}
	return g.wait(ctx, delay)
}

// ensureInitLocked runs the MCP open when no client session is held.
func (g *hubGraph) ensureInitLocked(ctx context.Context) error {
	if g.mcpSession != "" {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	g.nextID++
	status, headers, raw, err := g.post(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      g.nextID,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": hubAgent, "version": "1"},
		},
	})
	if err != nil {
		return fmt.Errorf("memento: hub initialize: %w", err)
	}
	if status == http.StatusServiceUnavailable {
		return &attachBoundError{
			tool:       "initialize",
			status:     status,
			retryAfter: attachRetry(headers),
			excerpt:    hubExcerpt(raw),
		}
	}
	if status != http.StatusOK {
		return fmt.Errorf("memento: hub initialize: HTTP %d", status)
	}
	session := headers.Get("Mcp-Session-Id")
	if session == "" {
		return fmt.Errorf("memento: hub initialize: no Mcp-Session-Id header")
	}
	if _, err := hubDecode(headers.Get("Content-Type"), raw); err != nil {
		return fmt.Errorf("memento: hub initialize: %w", err)
	}
	g.mcpSession = session
	g.nextID++
	status, _, _, err = g.post(ctx, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	if err != nil {
		g.mcpSession = ""
		return fmt.Errorf("memento: hub initialized notification: %w", err)
	}
	if status != http.StatusOK && status != http.StatusAccepted && status != http.StatusNoContent {
		g.mcpSession = ""
		return fmt.Errorf("memento: hub initialized notification: HTTP %d", status)
	}
	return nil
}

// postLocked sends one tools/call over the held client session.
func (g *hubGraph) postLocked(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	withAgent := make(map[string]any, len(args)+1)
	for key, value := range args {
		withAgent[key] = value
	}
	if _, ok := withAgent["agent_id"]; !ok {
		withAgent["agent_id"] = hubAgent
	}
	g.nextID++
	status, headers, raw, err := g.post(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      g.nextID,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": withAgent},
	})
	if err != nil {
		return nil, err
	}
	if status == http.StatusServiceUnavailable {
		return nil, &attachBoundError{
			tool:       name,
			status:     status,
			retryAfter: attachRetry(headers),
			excerpt:    hubExcerpt(raw),
		}
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("memento: hub %s: HTTP %d: %s", name, status, hubExcerpt(raw))
	}
	reply, err := hubDecode(headers.Get("Content-Type"), raw)
	if err != nil {
		return nil, fmt.Errorf("memento: hub %s: %w", name, err)
	}
	if failure, ok := reply["error"]; ok {
		return nil, fmt.Errorf("memento: hub %s: %v", name, failure)
	}
	result, _ := reply["result"].(map[string]any)
	if result == nil {
		return nil, fmt.Errorf("memento: hub %s: no result envelope", name)
	}
	return result, nil
}

// post sends one JSON-RPC request to the shopper's hub session path. The
// tools take no session argument; the path carries it, and the bearer
// token carries the credential.
func (g *hubGraph) post(ctx context.Context, message map[string]any) (int, http.Header, []byte, error) {
	payload, err := json.Marshal(message)
	if err != nil {
		return 0, nil, nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.baseURL+"/mcp/s/"+g.session, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, nil, fmt.Errorf("memento: hub request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer "+g.token)
	if g.mcpSession != "" {
		request.Header.Set("Mcp-Session-Id", g.mcpSession)
	}
	response, err := g.httpClient.Do(request)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("memento: hub request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return 0, nil, nil, fmt.Errorf("memento: hub response: %w", err)
	}
	return response.StatusCode, response.Header, raw, nil
}

// hubDecode unwraps one hub reply from a JSON or an SSE body into the
// result envelope.
func hubDecode(contentType string, raw []byte) (map[string]any, error) {
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

// hubTexts joins one hub result's text content blocks.
func hubTexts(result map[string]any) string {
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

// hubExcerpt shortens a hub response for an error message.
func hubExcerpt(raw []byte) string {
	const width = 200
	if len(raw) <= width {
		return string(raw)
	}
	return fmt.Sprintf("%s… (%d bytes)", raw[:width], len(raw))
}
