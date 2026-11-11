// The live layer for the T10.4 run: one Lambo hub, one service per shopper
// over that shopper's own hub session, the live fit index behind the pool,
// the round-2 population written through the fit writer's per-shopper
// transitions, and every shopper's frozen history loaded through the layer's
// real write path (server.LoadSeed).
//
// The layer holds its beliefs, evidence and views in memory; only the hub's
// own store and the standing journal survive a process exit. T10.4 therefore
// seeds and runs in one continuous process and never restarts the layer
// between the first seed and the last turn.
//
// The hub is the deploy's shape (deploy/start.sh): one `lambo serve
// --transport http` over a sqlite store with the native Elastic embedder,
// `--max-sessions 128 --rate-limit-rps 200`, `max_attached = 16`, one pinned
// session and a `shopper-` on-demand credential. It listens on loopback
// only; the config and the store live in a temporary directory removed at
// the end, and the credential is minted per process and passed through the
// environment only.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/nrynss/memento-emptor/bench/round2"
	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/api/server"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
	"github.com/nrynss/memento-emptor/internal/memory"
	"github.com/nrynss/memento-emptor/internal/pool"
	"github.com/nrynss/memento-emptor/seed"
)

// hubAgent is the MCP agent identity the layer writes as on the hub.
const hubAgent = "memento-layer"

// receiptPollMS is the per-poll receipt wait; the daemon applies writes in
// the background and the ack carries a receipt.
const receiptPollMS = 60000

// settleMin is how long the seeded beliefs must read unchanged before the
// first turn. Promotion is eventual: the daemon evaluates on its own cadence,
// and a reading taken before that evaluation would misstate the standing.
const settleMin = 65 * time.Second

// settleMax bounds the settle poll.
const settleMax = 6 * time.Minute

// seedSalt is the published salt the population fixture is built under. The
// round-2 population rows are generated with it, so writing them through the
// per-shopper writer must hash with it too.
const seedSalt = identity.SeedSaltHex

// staticContract names the embedding contract the evidence store is built
// under. It carries a kind and nothing else.
type staticContract struct{ kind string }

// EmbeddingKind names the contract family.
func (c staticContract) EmbeddingKind() string { return c.kind }

// Validate reports whether the contract is usable.
func (c staticContract) Validate() error {
	if c.kind == "" {
		return errors.New("t104: contract names no kind")
	}
	return nil
}

// closedClassifier refuses every classification: seeding replays frozen,
// already-classified rows and never asks a model.
type closedClassifier struct{}

// Classify always fails.
func (closedClassifier) Classify(context.Context, string) (fakes.Classification, error) {
	return fakes.Classification{}, errors.New("t104: classifier must never be called during seeding")
}

// closedPayments refuses every payment read: seeding replays frozen
// histories, not webhooks.
type closedPayments struct{}

// VerifyWebhook always fails.
func (closedPayments) VerifyWebhook(context.Context, string) (fakes.WebhookEvent, error) {
	return fakes.WebhookEvent{}, errors.New("t104: payment source must never be read during seeding")
}

// GetOrder always fails.
func (closedPayments) GetOrder(context.Context, string) (fakes.OrderRecord, error) {
	return fakes.OrderRecord{}, errors.New("t104: payment source must never be read during seeding")
}

// GetRefund always fails.
func (closedPayments) GetRefund(context.Context, string) (fakes.RefundRecord, error) {
	return fakes.RefundRecord{}, errors.New("t104: payment source must never be read during seeding")
}

// GetDispute always fails.
func (closedPayments) GetDispute(context.Context, string) (fakes.DisputeRecord, error) {
	return fakes.DisputeRecord{}, errors.New("t104: payment source must never be read during seeding")
}

// lineResolver resolves an order item to its product line from the round-1
// population, falling back to the regular handle for a line it never seeded,
// exactly as the deployed layer's static catalog does. The round-2 lines are
// all regular on purpose (FROZEN.md section 10).
type lineResolver struct {
	handles map[lineKey]string
}

// lineKey keys one brand and category to its fit handle.
type lineKey struct{ brand, category string }

// newLineResolver builds the resolver.
func newLineResolver() *lineResolver {
	out := &lineResolver{handles: map[lineKey]string{}}
	for _, line := range seed.PopulationLines() {
		out.handles[lineKey{brand: line.Brand, category: line.Category}] = line.FitHandle
	}
	return out
}

// ProductLine returns the brand and category from the item and the handle
// the seeded population names, or regular when it names none.
func (r *lineResolver) ProductLine(ctx context.Context, item domain.Item) (domain.ProductLine, error) {
	if err := ctx.Err(); err != nil {
		return domain.ProductLine{}, err
	}
	if strings.TrimSpace(item.BrandID) == "" || strings.TrimSpace(item.Category) == "" {
		return domain.ProductLine{}, fmt.Errorf("t104: catalog line for item %q names no brand or category", item.SKU)
	}
	handle := "regular"
	if known, ok := r.handles[lineKey{brand: item.BrandID, category: item.Category}]; ok {
		handle = known
	}
	return domain.ProductLine{Brand: item.BrandID, Category: item.Category, FitHandle: handle}, nil
}

// layer is the live layer one driver process runs against.
type layer struct {
	workDir    string
	ownWorkDir bool
	hubCmd     *exec.Cmd
	hubLog     *os.File
	hubToken   string
	baseURL    string
	layerURL   string
	graphs     map[string]*hubGraph
	services   map[string]*server.Service
	defaultSvc *server.Service
	srv        *http.Server
	listener   net.Listener
	indexName  string
	elasticURL string
	elasticKey string
}

// stop shuts the layer down: the loopback server, then one SIGTERM to the
// hub so its write-behind log drains, then the temporary directory.
func (l *layer) stop() {
	if l.listener != nil {
		_ = l.srv.Close()
	}
	if l.hubCmd != nil && l.hubCmd.Process != nil {
		_ = l.hubCmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() {
			_ = l.hubCmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			_ = l.hubCmd.Process.Kill()
			<-done
		}
	}
	if l.hubLog != nil {
		_ = l.hubLog.Close()
	}
	if l.ownWorkDir {
		_ = os.RemoveAll(l.workDir)
	}
}

// startLayer boots the hub, writes the population, builds one service per
// shopper and loads each shopper's frozen history through it.
func startLayer(ctx context.Context, opts options, shoppers []seed.Round2Shopper) (*layer, error) {
	l := &layer{
		elasticURL: os.Getenv("ELASTIC_URL"),
		elasticKey: os.Getenv("LAMBO_ES_API_KEY"),
		indexName:  opts.fitIndex,
		graphs:     map[string]*hubGraph{},
		services:   map[string]*server.Service{},
	}
	if !strings.HasPrefix(l.indexName, "t104") {
		return nil, fmt.Errorf("t104: -fit-index %q must start with t104: the run writes a scratch index and deletes it", l.indexName)
	}
	if l.elasticURL == "" || l.elasticKey == "" {
		return nil, errors.New("t104: ELASTIC_URL and LAMBO_ES_API_KEY are empty: source tools/env.sh first")
	}
	if len(shoppers) == 0 {
		return nil, errors.New("t104: no shopper to serve")
	}
	l.workDir = opts.work
	if l.workDir == "" {
		var err error
		l.workDir, err = os.MkdirTemp("", "t104-layer-")
		if err != nil {
			return nil, fmt.Errorf("t104: temp layer dir: %w", err)
		}
		l.ownWorkDir = true
	}
	if err := os.MkdirAll(l.workDir, 0o700); err != nil {
		return nil, fmt.Errorf("t104: layer work dir: %w", err)
	}
	keyer, err := identity.NewKeyer(os.Getenv("MEMENTO_IDENTITY_SALT"))
	if err != nil {
		return nil, fmt.Errorf("t104: identity keyer: %w", err)
	}
	firstSession, err := keyer.SessionName(shoppers[0].ShopperID)
	if err != nil {
		return nil, fmt.Errorf("t104: session name: %w", err)
	}
	if err := l.startHub(ctx, opts, firstSession); err != nil {
		l.stop()
		return nil, err
	}
	fmt.Printf("hub: %s (one pinned session, shopper- on-demand credential)\n", l.baseURL)
	if err := l.seedPopulation(ctx, opts); err != nil {
		l.stop()
		return nil, err
	}
	if err := l.buildServices(ctx, opts, shoppers, keyer); err != nil {
		l.stop()
		return nil, err
	}
	if err := l.loadHistories(ctx, shoppers); err != nil {
		l.stop()
		return nil, err
	}
	if err := l.settle(ctx, shoppers); err != nil {
		l.stop()
		return nil, err
	}
	if err := l.serveLoopback(); err != nil {
		l.stop()
		return nil, err
	}
	return l, nil
}

// startHub writes the deploy-shaped config, provisions the store, starts the
// hub on a loopback port and waits for its listen address.
func (l *layer) startHub(ctx context.Context, opts options, pinned string) error {
	token, err := randomHex(32)
	if err != nil {
		return fmt.Errorf("t104: mint hub token: %w", err)
	}
	store := filepath.Join(l.workDir, "lambo.db")
	cfg := filepath.Join(l.workDir, "lambo.toml")
	config := "# Written by the T10.4 driver; removed with the run directory.\n" +
		"[store]\nkind = \"sqlite\"\npath = \"" + store + "\"\n\n" +
		"[embedder]\nkind = \"elastic\"\nurl = \"" + l.elasticURL + "\"\n" +
		"model = \".jina-embeddings-v3\"\ndim = 1024\napi_key_env = \"LAMBO_ES_API_KEY\"\n" +
		"connect_timeout_ms = 30000\ntimeout_ms = 60000\ncanary = \"verify\"\n\n" +
		"# Cadence only; the bars live in code.\n" +
		"[daemon]\ngc_interval = 1\ngc_idle_floor = 1\ngc_max_interval_secs = 3600\n" +
		"canonization_eval_interval_secs = 5\n\n" +
		"[serve]\nsessions = [\"" + pinned + "\"]\ndefault_session = \"" + pinned + "\"\n" +
		"max_attached = 16\nattach_concurrency = 1\nidle_detach_secs = 900\n\n" +
		"[[serve.credential]]\nname = \"memento\"\ntoken_env = \"LAMBO_MCP_TOKEN\"\n" +
		"session_prefix = \"shopper-\"\ncreate = true\n"
	if err := os.WriteFile(cfg, []byte(config), 0o600); err != nil {
		return fmt.Errorf("t104: write hub config: %w", err)
	}
	provision := exec.CommandContext(ctx, opts.lamboBin, "--config", cfg, "provision")
	if raw, err := provision.CombinedOutput(); err != nil {
		return fmt.Errorf("t104: lambo provision: %w: %s", err, strings.TrimSpace(string(raw)))
	}
	log, err := os.Create(filepath.Join(l.workDir, "hub.log"))
	if err != nil {
		return fmt.Errorf("t104: hub log: %w", err)
	}
	cmd := exec.Command(opts.lamboBin, "--config", cfg, "serve",
		"--transport", "http", "--bind", "127.0.0.1", "--port", "0",
		"--agent", hubAgent, "--max-sessions", "128", "--rate-limit-rps", "200")
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.Env = append(os.Environ(), "LAMBO_MCP_TOKEN="+token)
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		return fmt.Errorf("t104: start hub: %w", err)
	}
	l.hubCmd = cmd
	l.hubLog = log
	port, err := waitForPort(log.Name(), 60*time.Second)
	if err != nil {
		return fmt.Errorf("t104: hub never answered: %w", err)
	}
	l.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	l.hubToken = token
	return nil
}

// hubToken is kept out of the layer struct's printed form; it is set here.
// (declared as a field through the setter below to keep the struct readable)

// waitForPort scans a child's log for its listen address.
var portPattern = regexp.MustCompile(`127\.0\.0\.1:(\d+)`)

// waitForPort blocks until the log carries a listen address.
func waitForPort(logPath string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(logPath)
		if err == nil {
			if match := portPattern.FindStringSubmatch(string(raw)); match != nil {
				var port int
				if _, err := fmt.Sscanf(match[1], "%d", &port); err == nil && port > 0 {
					return port, nil
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return 0, fmt.Errorf("no listen address in %s", logPath)
}

// seedPopulation deletes any leftover scratch index, creates it, writes the
// population generated at the moment of seeding through the fit writer's
// per-shopper transitions, and verifies the two seeded lines through the
// pool reader before any turn.
func (l *layer) seedPopulation(ctx context.Context, opts options) error {
	if opts.skipPopulation {
		fmt.Println("population: skipped (debug reuse of the scratch index)")
		population, err := seed.GenerateRound2Population(time.Now().UTC())
		if err != nil {
			return fmt.Errorf("t104: generate round-2 population: %w", err)
		}
		return l.verifyPopulation(ctx, population)
	}
	if err := l.deleteIndex(ctx); err != nil {
		return err
	}
	fitIdx, err := fitindex.NewHTTPIndex(l.elasticURL, l.elasticKey, l.indexName)
	if err != nil {
		return fmt.Errorf("t104: fit index client: %w", err)
	}
	if err := fitIdx.Create(ctx); err != nil {
		return fmt.Errorf("t104: create fit index: %w", err)
	}
	asOf := time.Now().UTC()
	population, err := seed.GenerateRound2Population(asOf)
	if err != nil {
		return fmt.Errorf("t104: generate round-2 population: %w", err)
	}
	keyer, err := identity.NewKeyer(seedSalt)
	if err != nil {
		return fmt.Errorf("t104: population keyer: %w", err)
	}
	noLines := fitindex.LinesFunc(func(context.Context, domain.Item) (domain.ProductLine, error) {
		return domain.ProductLine{}, errors.New("t104: population rows carry their own line")
	})
	start := time.Now()
	for i, row := range population.Rows {
		payer := fmt.Sprintf("seedshopper-%s-%04d", row.Line.Brand, i+1)
		writer, err := fitindex.NewWriter(fitIdx, keyer, payer, noLines)
		if err != nil {
			return fmt.Errorf("t104: population writer %d: %w", i+1, err)
		}
		txn := fmt.Sprintf("seedtxn-%04d", i+1)
		if err := writePopulationRow(ctx, writer, row, txn); err != nil {
			return fmt.Errorf("t104: population row %d: %w", i+1, err)
		}
	}
	fmt.Printf("population: %d rows written at %s in %s\n", len(population.Rows), asOf.Format(time.RFC3339), time.Since(start).Round(time.Second))
	if err := l.verifyPopulation(ctx, population); err != nil {
		return err
	}
	return nil
}

// writePopulationRow drives one generated row's transition chain.
func writePopulationRow(ctx context.Context, writer *fitindex.Writer, row domain.FitRow, txnID string) error {
	purchase := fitindex.Purchase{TxnID: txnID, Line: row.Line, Size: row.Size, Seeded: true}
	if row.CapturedAt.IsZero() {
		return errors.New("t104: population row has no capture time")
	}
	if err := writer.Captured(ctx, purchase, row.CapturedAt); err != nil {
		return fmt.Errorf("capture: %w", err)
	}
	switch row.State {
	case domain.FitStateCaptured:
		return nil
	case domain.FitStateRefunded:
		return writer.Refunded(ctx, purchase, row.RefundedAt)
	case domain.FitStateReasonClassified:
		if err := writer.Refunded(ctx, purchase, row.RefundedAt); err != nil {
			return fmt.Errorf("refund: %w", err)
		}
		at := row.EventAt
		if row.Settled && !row.SettledAt.IsZero() {
			at = row.SettledAt
		}
		return writer.Classified(ctx, purchase, row.Reason, at)
	case domain.FitStateFeedback:
		return writer.Feedback(ctx, purchase, row.Verdict, row.SettledAt)
	case domain.FitStateKept:
		return writer.Kept(ctx, purchase, row.SettledAt)
	default:
		return fmt.Errorf("unknown state %q", row.State)
	}
}

// verifyPopulation reads the two seeded lines back through the pool reader
// and requires the freeze's verdicts: Earned small at M on the Vale and Wren
// lines.
func (l *layer) verifyPopulation(ctx context.Context, population seed.Population) error {
	poolIdx, err := pool.NewHTTPIndex(l.elasticURL, l.elasticKey, l.indexName)
	if err != nil {
		return fmt.Errorf("t104: pool index client: %w", err)
	}
	if err := poolIdx.Warm(ctx); err != nil {
		return fmt.Errorf("t104: warm pool: %w", err)
	}
	reader, err := pool.NewReader(poolIdx)
	if err != nil {
		return fmt.Errorf("t104: pool reader: %w", err)
	}
	for _, line := range seed.Round2PopulationLines() {
		all := seed.Round2PoolCounts(population.Rows, line, "M", time.Time{})
		recent := seed.Round2PoolCounts(population.Rows, line, "M", population.AsOf.Add(-domain.PoolRecentWindow))
		want := domain.DecidePool(all, recent)
		fit, err := reader.Fit(ctx, line, "M")
		if err != nil {
			return fmt.Errorf("t104: pool read %s/%s: %w", line.Brand, line.Category, err)
		}
		if fit.Verdict.Status != want.Status || fit.Verdict.Direction != want.Direction {
			return fmt.Errorf("t104: pool %s size M reads %s/%s, want %s/%s",
				line.Brand, fit.Verdict.Status, fit.Verdict.Direction, want.Status, want.Direction)
		}
		fmt.Printf("pool: %s/%s/%s M reads %s %s (%d settled, %d seeded)\n",
			line.Brand, line.Category, line.FitHandle, fit.Verdict.Status, fit.Verdict.Direction,
			fit.Counts.Settled, fit.Seeded.Settled)
	}
	return nil
}

// deleteIndex removes the scratch index and refuses any name outside the
// t104 prefix. A leftover index would double the counts.
func (l *layer) deleteIndex(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		strings.TrimRight(l.elasticURL, "/")+"/"+l.indexName, nil)
	if err != nil {
		return fmt.Errorf("t104: delete index request: %w", err)
	}
	request.Header.Set("Authorization", "ApiKey "+l.elasticKey)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("t104: delete index: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}

// closeIndex deletes the scratch index at the end of the run.
func (l *layer) closeIndex(ctx context.Context) {
	if err := l.deleteIndex(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "t104: delete scratch index: %v\n", err)
	}
}

// buildServices builds one service per shopper over its own hub session.
func (l *layer) buildServices(ctx context.Context, opts options, shoppers []seed.Round2Shopper, keyer *identity.Keyer) error {
	poolIdx, err := pool.NewHTTPIndex(l.elasticURL, l.elasticKey, l.indexName)
	if err != nil {
		return fmt.Errorf("t104: pool index: %w", err)
	}
	fitIdx, err := fitindex.NewHTTPIndex(l.elasticURL, l.elasticKey, l.indexName)
	if err != nil {
		return fmt.Errorf("t104: fit index: %w", err)
	}
	lines := newLineResolver()
	standingDir := filepath.Join(l.workDir, "standing")
	if err := os.MkdirAll(standingDir, 0o700); err != nil {
		return fmt.Errorf("t104: standing dir: %w", err)
	}
	for _, shopper := range shoppers {
		session, err := keyer.SessionName(shopper.ShopperID)
		if err != nil {
			return fmt.Errorf("t104: session for %s: %w", shopper.ShopperID, err)
		}
		graph := newHubGraph(l.baseURL, l.hubToken, session)
		l.graphs[shopper.ShopperID] = graph
		store, err := evidence.NewStore(staticContract{kind: "t104-layer"})
		if err != nil {
			return fmt.Errorf("t104: evidence store: %w", err)
		}
		svc, err := server.NewService(server.Deps{
			Payments:     closedPayments{},
			Classifier:   closedClassifier{},
			Store:        store,
			Graph:        graph,
			SchemaDir:    "testdata/api",
			Clock:        func() time.Time { return time.Now().UTC() },
			Pool:         poolIdx,
			FitRows:      fitIdx,
			IdentitySalt: os.Getenv("MEMENTO_IDENTITY_SALT"),
			Lines:        lines,
			StandingDir:  standingDir,
		})
		if err != nil {
			return fmt.Errorf("t104: service for %s: %w", shopper.ShopperID, err)
		}
		l.services[shopper.ShopperID] = svc
	}
	defaultStore, err := evidence.NewStore(staticContract{kind: "t104-default"})
	if err != nil {
		return fmt.Errorf("t104: default evidence store: %w", err)
	}
	defaultSvc, err := server.NewService(server.Deps{
		Payments:    closedPayments{},
		Classifier:  closedClassifier{},
		Store:       defaultStore,
		Graph:       fakes.NewRecordingEvidenceGraph(nil, nil),
		SchemaDir:   "testdata/api",
		Clock:       func() time.Time { return time.Now().UTC() },
		Pool:        poolIdx,
		FitRows:     fitIdx,
		Lines:       lines,
		StandingDir: standingDir,
	})
	if err != nil {
		return fmt.Errorf("t104: default service: %w", err)
	}
	l.defaultSvc = defaultSvc
	return nil
}

// loadHistories loads every selected shopper's frozen history through the
// layer's real write path.
func (l *layer) loadHistories(ctx context.Context, shoppers []seed.Round2Shopper) error {
	for _, shopper := range shoppers {
		s, logEntry, err := buildSeed(shopper)
		if err != nil {
			return fmt.Errorf("t104: seed %s: %w", shopper.ShopperID, err)
		}
		// A general statement derives as a note (an Observation), and
		// Lambo never lets an against reference resolve to an Observation
		// (canonicalize deliberately skips them). A standing correction
		// the layer then declares against the statement would be refused,
		// so the statement's concept is planted as a belief first: the
		// layer's own RecordStatement derives the same content, which
		// matches the existing concept and leaves it a Logic one. The
		// served reading is unchanged (no evidence, Claimed), and the
		// correction can land. The run note records this deviation.
		if err := prederiveStatements(ctx, l.graphs[shopper.ShopperID], s); err != nil {
			return fmt.Errorf("t104: pre-derive statements for %s: %w", shopper.ShopperID, err)
		}
		if err := l.services[shopper.ShopperID].LoadSeed(ctx, s); err != nil {
			return fmt.Errorf("t104: load seed for %s: %w", shopper.ShopperID, err)
		}
		seedLogs = append(seedLogs, logEntry)
		fmt.Printf("seeded %s: %d beliefs, %d evidence rows, %d pending\n",
			shopper.ShopperID, len(s.Beliefs), logEntry.Evidence, len(s.Pending.Questions))
	}
	return nil
}

// seedLogs collects the per-shopper seed summary the run commits.
var seedLogs []SeedLog

// prederiveStatements plants each zero-evidence belief's concept as a Logic
// belief before LoadSeed derives it as a note. The note then matches the
// existing concept, so the served reading is unchanged and the concept can
// be the target of the layer's own correction declaration.
func prederiveStatements(ctx context.Context, graph *hubGraph, s server.Seed) error {
	for _, view := range s.Beliefs {
		if len(view.Evidence) != 0 {
			continue
		}
		at, err := time.Parse(time.RFC3339, view.UpdatedAt)
		if err != nil {
			return fmt.Errorf("statement %q has no readable event time: %w", view.BeliefID, err)
		}
		content := beliefContent(view.Kind, view.Subject, view.Value)
		if err := graph.Derive(ctx, content, memory.ConceptBelief, at); err != nil {
			return fmt.Errorf("derive statement %q: %w", view.BeliefID, err)
		}
	}
	return nil
}

// settle polls every seeded belief until the readings hold unchanged for
// settleMin, or the deadline fails the run loudly.
func (l *layer) settle(ctx context.Context, shoppers []seed.Round2Shopper) error {
	type content struct {
		shopper string
		text    string
	}
	var contents []content
	for _, shopper := range shoppers {
		s, _, err := buildSeed(shopper)
		if err != nil {
			return err
		}
		for _, belief := range s.Beliefs {
			contents = append(contents, content{
				shopper: shopper.ShopperID,
				text:    beliefContent(belief.Kind, belief.Subject, belief.Value),
			})
		}
	}
	start := time.Now()
	last := map[string]string{}
	polls := 0
	for {
		polls++
		stable := time.Since(start) >= settleMin
		states := map[string]int{}
		changed := []string{}
		for _, item := range contents {
			report, err := l.graphs[item.shopper].Inspect(ctx, item.text)
			if err != nil {
				return fmt.Errorf("t104: settle inspect %s: %w", item.shopper, err)
			}
			key := fmt.Sprintf("%s/%d/%.3f", report.State, report.Dependents, report.Coverage)
			states[string(report.State)]++
			slot := item.shopper + "\x00" + item.text
			if prev, ok := last[slot]; !ok || prev != key {
				stable = false
				changed = append(changed, fmt.Sprintf("%s %s %s/%d", item.shopper, shortText(item.text), report.State, report.Dependents))
			}
			last[slot] = key
		}
		fmt.Printf("settle poll %d (%ds): venerable=%d candidate=%d none=%d changed=[%s]\n",
			polls, int(time.Since(start).Seconds()), states["Venerable"], states["Candidate"], states["None"], strings.Join(changed, "; "))
		if stable {
			fmt.Printf("settle: %d beliefs stable after %s\n", len(contents), time.Since(start).Round(time.Second))
			return nil
		}
		if time.Since(start) > settleMax {
			fmt.Printf("settle: readings still moving after %s; the run records the states it reads and the note says so\n", time.Since(start).Round(time.Second))
			return nil
		}
		time.Sleep(5 * time.Second)
	}
}

// serveLoopback serves the layer's frozen handler on loopback so recall is
// called over the wire path the deployment serves, not in process.
func (l *layer) serveLoopback() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("t104: layer listener: %w", err)
	}
	l.listener = listener
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tools/{tool}", l.handleTool)
	mux.HandleFunc("POST /mcp", l.handleTool)
	mux.HandleFunc("GET /healthz", l.defaultSvc.Handler().ServeHTTP)
	l.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := l.srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "t104: layer server: %v\n", err)
		}
	}()
	l.layerURL = "http://" + listener.Addr().String()
	fmt.Printf("layer: served on %s (health and recall over the wire)\n", l.layerURL)
	return nil
}

// layerURL is the loopback base address the driver calls.
// (declared with the other fields through the assignment above)

// handleTool routes one tool call to the service that names its shopper.
func (l *layer) handleTool(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "t104: unreadable body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	svc := l.defaultSvc
	if shopper := bodyShopper(raw); shopper != "" {
		if named, ok := l.services[shopper]; ok {
			svc = named
		}
	}
	svc.Handler().ServeHTTP(w, r)
}

// bodyShopper reads the shopper_id of a tool request.
func bodyShopper(raw []byte) string {
	var body struct {
		ShopperID string `json:"shopper_id"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return body.ShopperID
}

// recall calls the layer's recall_for_purchase over the wire and returns the
// response bytes verbatim.
func (l *layer) recall(ctx context.Context, shopper seed.Round2Shopper) ([]byte, error) {
	target := round2.Target(shopper)
	return l.tool(ctx, "recall_for_purchase", target.JSON())
}

// tool calls one frozen tool over the layer's loopback handler.
func (l *layer) tool(ctx context.Context, name string, body []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, l.layerURL+"/tools/"+name, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("t104: %s request: %w", name, err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("t104: %s: %w", name, err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("t104: read %s: %w", name, err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("t104: %s: status %d: %s", name, response.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

// ---------------------------------------------------------------------------
// Hub graph client: one MCP session over the hub's /mcp/s/<session> path.
// ---------------------------------------------------------------------------

// hubGraph is a fakes.EvidenceGraph over one live hub session. Every write
// travels with its event time so honestly spread history promotes, and every
// write waits its receipt until the daemon applies it.
type hubGraph struct {
	baseURL string
	token   string
	session string

	httpClient *http.Client
	mcpSession string
	nextID     int
}

// Compile-time proof the live client satisfies the seam the service builds
// on.
var _ fakes.EvidenceGraph = (*hubGraph)(nil)

// newHubGraph builds the graph client for one hub session.
func newHubGraph(baseURL, token, session string) *hubGraph {
	return &hubGraph{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		session:    session,
		httpClient: &http.Client{Timeout: 180 * time.Second},
	}
}

// Derive records one belief or statement.
func (g *hubGraph) Derive(ctx context.Context, content, kind string, eventTime time.Time) error {
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

// Recall lists the session's hit labels for one query. The layer ranks from
// its stored beliefs and never calls this; it exists so the client satisfies
// the seam honestly.
func (g *hubGraph) Recall(ctx context.Context, query string) ([]string, error) {
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

// Inspect reports the promotion state of one belief. A focus nothing matches
// reads as None with no error.
func (g *hubGraph) Inspect(ctx context.Context, content string) (fakes.BeliefReport, error) {
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
	if status, ok := navigate(result, "structuredContent", "admission", "status").(string); ok {
		// The structured admission is authoritative: its "None" is a real
		// reading, never a reason to sniff the view text. The text carries
		// every neighbour too, so a fallback there would attribute a
		// neighbour's state to the focus.
		switch strings.ToLower(status) {
		case "venerable":
			report.State = domain.GraphVenerable
		case "candidate":
			report.State = domain.GraphCandidate
		default:
			report.State = domain.GraphNone
		}
	} else {
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

// hubStateOf falls back to the inspect text when the hub answers without a
// structured admission.
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
	return "", fmt.Errorf("t104: %s issued no receipt: %.200s", name, hubTexts(result))
}

// waitApplied blocks until the daemon applies one receipt.
func (g *hubGraph) waitApplied(ctx context.Context, receipt, what string) error {
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		result, err := g.call(ctx, "lambo_stats", map[string]any{
			"agent_id": hubAgent, "receipt": receipt, "wait_ms": receiptPollMS,
		})
		if err != nil {
			return fmt.Errorf("t104: wait %s: %w", what, err)
		}
		text := hubTexts(result)
		switch {
		case strings.Contains(text, "applied"):
			return nil
		case strings.Contains(text, "failed"), strings.Contains(text, "refused"), strings.Contains(text, "embedder unavailable"):
			return fmt.Errorf("t104: %s not applied: %.200s", what, text)
		}
	}
	return fmt.Errorf("t104: receipt for %s did not settle", what)
}

// call makes one hub tools/call, opening the MCP connection on first use and
// re-opening it once when the hub detached the idle client session.
func (g *hubGraph) call(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	result, err := g.callOnce(ctx, name, args)
	if err == nil {
		return result, nil
	}
	g.mcpSession = ""
	return g.callOnce(ctx, name, args)
}

// callOnce opens the client session when none is held and posts the call.
func (g *hubGraph) callOnce(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	if g.mcpSession == "" {
		if err := g.initialize(ctx); err != nil {
			return nil, err
		}
	}
	withAgent := make(map[string]any, len(args)+1)
	for key, value := range args {
		withAgent[key] = value
	}
	if _, ok := withAgent["agent_id"]; !ok {
		withAgent["agent_id"] = hubAgent
	}
	g.nextID++
	result, status, err := g.post(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      g.nextID,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": withAgent},
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("t104: hub %s: HTTP %d", name, status)
	}
	reply, err := hubDecode(result)
	if err != nil {
		return nil, fmt.Errorf("t104: hub %s: %w", name, err)
	}
	if failure, ok := reply["error"]; ok {
		return nil, fmt.Errorf("t104: hub %s: %v", name, failure)
	}
	envelope, _ := reply["result"].(map[string]any)
	if envelope == nil {
		return nil, fmt.Errorf("t104: hub %s: no result envelope", name)
	}
	return envelope, nil
}

// initialize opens one MCP client session on the shopper's hub path.
func (g *hubGraph) initialize(ctx context.Context) error {
	g.nextID++
	reply, status, err := g.post(ctx, map[string]any{
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
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("t104: hub initialize: HTTP %d", status)
	}
	session := reply.header.Get("Mcp-Session-Id")
	if session == "" {
		return errors.New("t104: hub initialize: no Mcp-Session-Id header")
	}
	if _, err := hubDecode(reply); err != nil {
		return fmt.Errorf("t104: hub initialize: %w", err)
	}
	g.mcpSession = session
	g.nextID++
	if _, status, err := g.postRaw(ctx, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}); err != nil {
		g.mcpSession = ""
		return err
	} else if status != http.StatusOK && status != http.StatusAccepted && status != http.StatusNoContent {
		g.mcpSession = ""
		return fmt.Errorf("t104: hub initialized notification: HTTP %d", status)
	}
	return nil
}

// hubReply carries one HTTP answer: headers and body.
type hubReply struct {
	header http.Header
	body   []byte
}

// post makes one request to the shopper's hub path.
func (g *hubGraph) post(ctx context.Context, message map[string]any) (hubReply, int, error) {
	reply, status, err := g.postRaw(ctx, message)
	return reply, status, err
}

// postRaw sends one JSON-RPC message and reads the answer.
func (g *hubGraph) postRaw(ctx context.Context, message map[string]any) (hubReply, int, error) {
	payload, err := json.Marshal(message)
	if err != nil {
		return hubReply{}, 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.baseURL+"/mcp/s/"+g.session, bytes.NewReader(payload))
	if err != nil {
		return hubReply{}, 0, fmt.Errorf("t104: hub request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer "+g.token)
	if g.mcpSession != "" {
		request.Header.Set("Mcp-Session-Id", g.mcpSession)
	}
	response, err := g.httpClient.Do(request)
	if err != nil {
		return hubReply{}, 0, fmt.Errorf("t104: hub request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return hubReply{}, 0, fmt.Errorf("t104: hub response: %w", err)
	}
	return hubReply{header: response.Header, body: body}, response.StatusCode, nil
}

// hubDecode unwraps one hub reply from a JSON or an SSE body.
func hubDecode(reply hubReply) (map[string]any, error) {
	text := strings.TrimSpace(string(reply.body))
	if strings.Contains(reply.header.Get("Content-Type"), "text/event-stream") {
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
			return nil, errors.New("no data frame in SSE body")
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(frames[len(frames)-1]), &out); err != nil {
			return nil, fmt.Errorf("decode SSE frame: %w", err)
		}
		return out, nil
	}
	var out map[string]any
	if err := json.Unmarshal(reply.body, &out); err != nil {
		return nil, fmt.Errorf("decode reply: %w", err)
	}
	return out, nil
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

// ---------------------------------------------------------------------------
// History to seed: the filing the layer's own write path performs.
// ---------------------------------------------------------------------------

// SeedLog is the committed summary of one shopper's seed: what the driver
// filed where, so a reader can see which rows the layer never receives.
type SeedLog struct {
	ShopperID string `json:"shopper_id"`
	Beliefs   int    `json:"beliefs"`
	Evidence  int    `json:"evidence"`
	Pending   int    `json:"pending"`
	// Statements, Kept, Feedback, Seller, Fit, Rival, Unattached and
	// ReasonOnly count the history rows by the filing they took.
	Statements int      `json:"statements"`
	Kept       int      `json:"kept"`
	Feedback   int      `json:"feedback"`
	Seller     int      `json:"seller"`
	Fit        int      `json:"fit"`
	Rival      int      `json:"rival"`
	Unattached int      `json:"unattached"`
	ReasonOnly int      `json:"reason_only"`
	PendingRow int      `json:"pending_rows"`
	Skipped    []string `json:"skipped,omitempty"`
}

// runNow is the moment the driver started. Every shopper's history is
// measured back from it, so its event times are in the past when the layer
// writes them: a future event time never ages, and an unaged dependent never
// counts toward promotion. The relative spans are the frozen ones.
var runNow = time.Now().UTC()

// seedAnchorFor places one shopper's session so its last seeded day sits
// about a day before the run. The relative spans are the frozen ones; only
// the anchor moves, which is what makes the history a live past rather than
// a fixture's fixed date.
func seedAnchorFor(shopper seed.Round2Shopper) time.Time {
	days := time.Duration(shopper.History.SessionDays + 1)
	return runNow.Add(-days * 24 * time.Hour)
}

// eventAt materializes a seed day from one shopper's live anchor.
func eventAt(anchor time.Time, day float64) time.Time {
	return anchor.Add(time.Duration(day * float64(24*time.Hour))).UTC()
}

// atString formats one event time the way the wire does.
func atString(anchor time.Time, day float64) string { return eventAt(anchor, day).Format(time.RFC3339) }

// beliefContent is the layer's graph content for one belief.
func beliefContent(kind domain.BeliefKind, subject domain.Subject, value string) string {
	return fmt.Sprintf("memento %s %s %s %s", string(kind), string(subject.Kind), subject.ID, value)
}

// fitValue composes a fit belief's value exactly as the layer does: a
// too-small return sizes one rung up, anything else one rung down.
func fitValue(category domain.Category, size string) string {
	if category == domain.CategoryFitTooSmall {
		return "runs small, take size " + round2.SizeForDirection(size, domain.FitDirectionSmall)
	}
	return "runs large, take size " + round2.SizeForDirection(size, domain.FitDirectionLarge)
}

// beliefDraft is one belief being built from the history.
type beliefDraft struct {
	view     *contract.BeliefView
	dir      domain.FitDirection
	evidence int
	lastDay  float64
}

// buildSeed files one frozen history the way the layer's own write path
// files it, then hands the result to server.LoadSeed:
//
//   - the general statement derives as a note (memory.RecordStatement), so
//     it is served as a Claimed belief and never promotes;
//   - a classified fit return births or joins a same-direction fit belief
//     (memory.EnsureBeliefDeclaring + WriteEvidence, which cites the belief
//     in depends_on with the row's event time);
//   - an opposite-direction return lands unattached below
//     domain.RivalBirthThreshold and births the opposite-direction rival at
//     the threshold, exactly as beliefForReason does;
//   - a seller-classified answer writes the brand's seller belief;
//   - fit feedback joins the strongest fit belief for the brand (or births
//     the directionless one) through WriteFeedback, citing the belief with
//     its event time;
//   - a kept-past-window row is an observation with no structural edge, the
//     same as the layer's live path;
//   - a reasonless refund is a pending question, and a directionless answer
//     (changed mind, unclear) is filed nowhere: no code path turns it into
//     a belief or a question.
func buildSeed(shopper seed.Round2Shopper) (server.Seed, SeedLog, error) {
	brand := shopper.Candidate.BrandID
	brandName := shopper.Candidate.BrandName
	s := server.Seed{
		ShopperID:        shopper.ShopperID,
		Beliefs:          []contract.BeliefView{},
		Links:            map[string][]string{},
		Pending:          contract.PendingQuestionsResponse{ShopperID: shopper.ShopperID, Questions: []contract.QuestionView{}},
		Timelines:        map[string][]contract.TimelineEvent{},
		Notices:          map[string]string{},
		FeedbackVerdicts: map[string]contract.FeedbackVerdict{},
		FeedbackIDs:      map[string]string{},
	}
	logEntry := SeedLog{ShopperID: shopper.ShopperID}
	anchor := seedAnchorFor(shopper)

	rows := append([]seed.Row(nil), shopper.History.Rows...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Day < rows[j].Day })

	drafts := []*beliefDraft{}
	evSeq, qSeq, blfSeq := 0, 0, 0
	mintEv := func() string {
		evSeq++
		return fmt.Sprintf("ev_%06d", evSeq)
	}
	mintBelief := func() string {
		blfSeq++
		return fmt.Sprintf("blf_%06d", blfSeq)
	}
	fitBeliefs := map[string][]*beliefDraft{}
	sellerBeliefs := map[string]*beliefDraft{}
	streaks := map[string]*domain.OppositeStreak{}
	streakFor := func(brand string) *domain.OppositeStreak {
		streak, ok := streaks[brand]
		if !ok {
			streak = &domain.OppositeStreak{}
			streaks[brand] = streak
		}
		return streak
	}
	newDraft := func(kind domain.BeliefKind, subject domain.Subject, value string, dir domain.FitDirection, day float64) *beliefDraft {
		draft := &beliefDraft{
			view: &contract.BeliefView{
				BeliefID:  mintBelief(),
				Kind:      kind,
				Subject:   subject,
				Value:     value,
				Seeded:    true,
				UpdatedAt: atString(anchor, day),
				Evidence:  []contract.EvidenceView{},
			},
			dir:     dir,
			lastDay: day,
		}
		drafts = append(drafts, draft)
		logEntry.Beliefs++
		return draft
	}
	newFit := func(brandID, brandDisplay string, dir domain.FitDirection, value string, day float64, rival bool) *beliefDraft {
		draft := newDraft(domain.BeliefKindFit,
			domain.Subject{Kind: domain.SubjectBrand, ID: brandID, Name: brandDisplay},
			value, dir, day)
		fitBeliefs[brandID] = append(fitBeliefs[brandID], draft)
		if rival {
			logEntry.Rival++
		}
		return draft
	}
	addEvidence := func(draft *beliefDraft, view contract.EvidenceView) {
		draft.view.Evidence = append(draft.view.Evidence, view)
		draft.evidence++
		if rowDay := rowDayOf(view, anchor); rowDay > draft.lastDay {
			draft.lastDay = rowDay
			draft.view.UpdatedAt = atString(anchor, draft.lastDay)
		}
		s.Links[view.EvidenceID] = []string{draft.view.BeliefID}
		logEntry.Evidence++
	}
	fitEvidence := func(evID string, row seed.Row) contract.EvidenceView {
		counts := domain.BeliefKindFit
		return contract.EvidenceView{
			EvidenceID:   evID,
			Outcome:      outcomeOf(row),
			Item:         itemOf(row),
			Reason:       reasonOf(row),
			CountsToward: &counts,
			WriteOp:      domain.WriteRecordAction,
			Link:         linkOf(row),
			Seeded:       true,
			OccurredAt:   atString(anchor, row.Day),
		}
	}
	bestFit := func(brandID string, want domain.FitDirection) *beliefDraft {
		var best *beliefDraft
		for _, draft := range fitBeliefs[brandID] {
			if draft.dir != want {
				continue
			}
			if best == nil || draft.evidence > best.evidence {
				best = draft
			}
		}
		return best
	}
	strongestFit := func(brandID string) *beliefDraft {
		var best *beliefDraft
		for _, draft := range fitBeliefs[brandID] {
			if best == nil || draft.evidence > best.evidence {
				best = draft
			}
		}
		return best
	}

	for _, row := range rows {
		switch row.Signal {
		case domain.SignalGeneralStatement:
			// The statement is memory about the shopper, so it is served
			// against the candidate's brand: the frozen recall fixture
			// builds it that way, and recall filters beliefs by brand.
			if row.Item.Size == "" {
				return server.Seed{}, logEntry, fmt.Errorf("statement row on day %.0f names no size", row.Day)
			}
			newDraft(domain.BeliefKindFit,
				domain.Subject{Kind: domain.SubjectBrand, ID: brand, Name: brandName},
				"shopper says they are usually size "+row.Item.Size, domain.FitDirectionNone, row.Day)
			logEntry.Statements++
		case domain.SignalKeptPastWindow:
			// No live ingest writes kept rows to a belief: the observation
			// stays in the pool's fit row and never reaches recall.
			logEntry.Kept++
		case domain.SignalFitFeedback:
			if !strings.Contains(strings.ToLower(row.Note), "fits well") {
				return server.Seed{}, logEntry, fmt.Errorf("feedback row on day %.0f without fits-well: %q", row.Day, row.Note)
			}
			draft := strongestFit(row.Item.BrandID)
			if draft == nil {
				// The layer's beliefForFeedback births the directionless
				// belief when no fit belief exists yet.
				draft = newFit(row.Item.BrandID, row.Item.BrandName, domain.FitDirectionNone,
					"size "+row.Item.Size+" fits this shopper now", row.Day, false)
			}
			evID := mintEv()
			counts := domain.BeliefKindFit
			// The frozen history names no reason for a feedback row; the
			// verdict is what WriteFeedback counts on. A live feedback
			// delivery would attach the shopper's words here, but this
			// replay invents none.
			addEvidence(draft, contract.EvidenceView{
				EvidenceID:   evID,
				Outcome:      outcomeOf(row),
				Item:         itemOf(row),
				Reason:       nil,
				CountsToward: &counts,
				WriteOp:      domain.WriteRecordAction,
				Link:         linkOf(row),
				Seeded:       true,
				OccurredAt:   atString(anchor, row.Day),
			})
			s.FeedbackVerdicts[evID] = contract.FeedbackFitsWell
			logEntry.Feedback++
		case domain.SignalClassifiedOutcome:
			if row.Outcome == nil {
				return server.Seed{}, logEntry, fmt.Errorf("classified row on day %.0f carries no outcome", row.Day)
			}
			if row.Reason == nil {
				// A reasonless refund is the one open question.
				qSeq++
				s.Pending.Questions = append(s.Pending.Questions, contract.QuestionView{
					QuestionID:    fmt.Sprintf("q_%06d", qSeq),
					Outcome:       outcomeOf(row),
					Item:          itemOf(row),
					Link:          linkOf(row),
					Seeded:        true,
					AskedCount:    0,
					LastAskedAt:   nil,
					AwaitingSince: atString(anchor, row.Day),
				})
				logEntry.PendingRow++
				continue
			}
			switch row.Reason.Category.CountsToward() {
			case domain.BeliefKindSeller:
				draft := sellerBeliefs[row.Item.BrandID]
				if draft == nil {
					draft = newDraft(domain.BeliefKindSeller, domain.Subject{
						Kind: domain.SubjectSeller,
						ID:   "seller-for-" + row.Item.BrandID,
						Name: strings.TrimSpace(row.Item.BrandName) + " seller",
					}, "ships reliably for this shopper", domain.FitDirectionNone, row.Day)
					sellerBeliefs[row.Item.BrandID] = draft
				}
				evID := mintEv()
				counts := domain.BeliefKindSeller
				addEvidence(draft, contract.EvidenceView{
					EvidenceID:   evID,
					Outcome:      outcomeOf(row),
					Item:         itemOf(row),
					Reason:       reasonOf(row),
					CountsToward: &counts,
					WriteOp:      domain.WriteRecordAction,
					Link:         linkOf(row),
					Seeded:       true,
					OccurredAt:   atString(anchor, row.Day),
				})
				logEntry.Seller++
			case domain.BeliefKindFit:
				want := row.Reason.Category.Direction()
				if want == domain.FitDirectionNone {
					logEntry.Unattached++
					continue
				}
				draft := bestFit(row.Item.BrandID, want)
				if draft != nil {
					streakFor(row.Item.BrandID).NoteMatched()
					addEvidence(draft, fitEvidence(mintEv(), row))
					logEntry.Fit++
					continue
				}
				if len(fitBeliefs[row.Item.BrandID]) == 0 {
					draft = newFit(row.Item.BrandID, row.Item.BrandName, want,
						fitValue(row.Reason.Category, row.Item.Size), row.Day, false)
					addEvidence(draft, fitEvidence(mintEv(), row))
					logEntry.Fit++
					continue
				}
				if streakFor(row.Item.BrandID).NoteOpposite() {
					// The run reached domain.RivalBirthThreshold: the
					// opposite-direction belief is born and this row
					// attaches to it.
					draft = newFit(row.Item.BrandID, row.Item.BrandName, want,
						fitValue(row.Reason.Category, row.Item.Size), row.Day, true)
					addEvidence(draft, fitEvidence(mintEv(), row))
					logEntry.Fit++
					continue
				}
				logEntry.Unattached++
			default:
				// Changed mind, an unclear answer: no code path turns it
				// into a belief or a pending question.
				logEntry.ReasonOnly++
			}
		default:
			return server.Seed{}, logEntry, fmt.Errorf("unknown signal %q on day %.0f", string(row.Signal), row.Day)
		}
	}
	for _, draft := range drafts {
		s.Beliefs = append(s.Beliefs, *draft.view)
	}
	logEntry.Pending = len(s.Pending.Questions)
	return s, logEntry, nil
}

// shortText trims a belief content for the settle log.
func shortText(text string) string {
	if len(text) > 48 {
		return text[len(text)-48:]
	}
	return text
}

// rowDayOf parses one evidence view's event time back to its day offset from
// the anchor. It is used only to keep a belief's updated_at in event order.
func rowDayOf(view contract.EvidenceView, anchor time.Time) float64 {
	at, err := time.Parse(time.RFC3339, view.OccurredAt)
	if err != nil {
		return 0
	}
	return at.Sub(anchor).Hours() / 24
}

// outcomeOf renders one row's outcome in wire form.
func outcomeOf(row seed.Row) contract.OutcomeView {
	if row.Outcome == nil {
		return contract.OutcomeView{}
	}
	return contract.OutcomeView{
		Kind:        row.Outcome.Kind,
		Status:      row.Outcome.Status,
		AmountMinor: row.Outcome.AmountMinor,
		Currency:    row.Outcome.Currency,
	}
}

// itemOf renders one row's item in wire form.
func itemOf(row seed.Row) contract.ItemView {
	return contract.ItemView{
		BrandID:   row.Item.BrandID,
		BrandName: row.Item.BrandName,
		LineName:  row.Item.LineName,
		SKU:       row.Item.SKU,
		Size:      row.Item.Size,
		Category:  row.Item.Category,
	}
}

// reasonOf renders one row's reason in wire form.
func reasonOf(row seed.Row) *contract.ReasonView {
	if row.Reason == nil {
		return nil
	}
	return &contract.ReasonView{
		Category:   row.Reason.Category,
		Text:       row.Reason.Text,
		Confidence: row.Reason.Confidence,
	}
}

// linkOf renders one row's link in wire form.
func linkOf(row seed.Row) contract.TransactionLinkView {
	return contract.TransactionLinkView{
		OrderID:   row.Link.OrderID,
		CaptureID: row.Link.CaptureID,
		RefundID:  row.Link.RefundID,
		DisputeID: row.Link.DisputeID,
		CustomID:  row.Link.CustomID,
		EventID:   row.Link.EventID,
	}
}

// randomHex mints n random bytes as lowercase hex.
func randomHex(n int) (string, error) {
	raw := make([]byte, n)
	file, err := os.Open("/dev/urandom")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := io.ReadFull(file, raw); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", raw), nil
}
