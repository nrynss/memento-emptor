// Command seed plants the deployment's seeded state on first boot and then
// exits. It has two independent parts, each idempotent through its own
// marker file on the disk, so a restart reseeds nothing and loses nothing:
//
//   - the fit population: the T9.1 seeded shoppers behind the pool, written
//     through the fit index writer into the live fit index, verified back
//     through the pool reader before the marker lands;
//   - the demo shopper's live graph: the same arc the T2.4b live leg proved
//     (three fit returns take the sizing belief to Venerable, a non-arrival
//     dispute stays seller-only, the correction declares against the belief
//     without deleting it).
//
// The seeding moment is the seed's own now: T9.1's statuses are judged over
// event times measured back from asOf, so seeding later than the fixture's
// anchor would drift rows out of the recent window and change statuses.
// Every reason-bearing seed refund goes through Refunded and then Classified,
// because a refund's evidence never carries a reason through ingest and the
// row settles only when the classified reason lands.
//
// The live fit index name is "fit-rows": the production name, never a test
// prefix. Local runs must not touch it: the fit part is skipped when
// LAMBO_EMBEDDER is fixture (the documented local loop) or when
// MEMENTO_FIT_SEED is 0. A proof run against a scratch index takes
// -fit-index and -fit-only, which production never sets.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
	"github.com/nrynss/memento-emptor/internal/pool"
	"github.com/nrynss/memento-emptor/seed"
)

// liveFitIndex is the production fit index name. Test runs use their own
// prefix through -fit-index and delete it afterwards; this name is never a
// test prefix.
const liveFitIndex = "fit-rows"

// The seeded contents: demo fiction, identical in shape to the proven
// live leg. Short and keyed, so evidence cites the belief and the
// correction declares against it.
const (
	belief     = "Brand X oxford runs small for this shopper: take size L"
	assertion  = "Shopper says they are usually size M"
	kept1      = "Kept past the window: Brand Y tee, no feedback"
	kept2      = "Kept past the window: Brand Z cap, no feedback"
	ret1       = "Fit return 1: Brand X oxford size M refunded; too tight in the shoulders"
	ret2       = "Fit return 2: Brand X oxford size M refunded; too tight across the chest"
	ret3       = "Fit return 3: Brand X oxford size M refunded; sleeves too short in M"
	seller     = "The seller ships reliably for this shopper"
	dispute    = "Dispute opened: Brand X parcel never arrived; carrier lost it"
	correction = "Shopper correction: lost weight, take size M for Brand X"
	recallQ    = "Brand X oxford sizing for this shopper"
	probe      = "Seed probe concept: the embedder answers"
)

// waitMS is the receipt wait per write, from the proven driver. The
// daemon evaluates on its own cadence, so convergence polls separately.
const waitMS = 34000

// convergeTimeout bounds the promotion poll. Seeded edges join fast;
// the daemon still needs its eval cycles.
const convergeTimeout = 6 * time.Minute

// errEmbedderDown is the sentinel for a store the network cannot embed
// through. The caller falls back to the spare contract; anything else
// fails the boot seed loudly.
var errEmbedderDown = fmt.Errorf("seed: embedder unreachable from this host")

// client speaks one stdio MCP session: sequential ids, one readline
// per response. Notifications send no id and read nothing back.
type client struct {
	agent string
	seq   int
	in    io.WriteCloser
	scan  *bufio.Scanner
}

func main() {
	lamboBin := flag.String("lambo", "/app/bin/lambo", "lambo release binary")
	config := flag.String("config", "", "lambo.toml written at boot (required)")
	session := flag.String("session", "demo-shopper", "session to seed")
	agent := flag.String("agent", "memento-seed", "agent id the seed writes under")
	marker := flag.String("marker", "", "marker file proving a finished graph seed (required)")
	fitIndexName := flag.String("fit-index", liveFitIndex, "fit index the seeded population is written to")
	fitOnly := flag.Bool("fit-only", false, "seed only the fit population, not the demo graph")
	flag.Parse()

	if *config == "" || *marker == "" {
		log.Fatal("seed: -config and -marker are required")
	}
	if *fitIndexName == "" {
		log.Fatal("seed: -fit-index names no index")
	}

	if err := seedFit(*fitIndexName, *marker); err != nil {
		if strings.Contains(err.Error(), errFitSkipped.Error()) {
			log.Printf("seed: fit population skipped: %v", err)
		} else {
			log.Printf("seed: fit population: %v", err)
			os.Exit(1)
		}
	}
	if *fitOnly {
		log.Print("seed: fit-only run, demo graph untouched")
		return
	}
	if _, err := os.Stat(*marker); err == nil {
		log.Printf("seed: marker %s present, nothing to do", *marker)
		return
	}
	if up, err := probeOnly(*lamboBin, *config, *session+"-probe", *agent); err != nil {
		log.Printf("seed: probe: %v", err)
		os.Exit(1)
	} else if !up {
		log.Printf("seed: %v", errEmbedderDown)
		os.Exit(3)
	}
	if err := run(*lamboBin, *config, *session, *agent); err != nil {
		if strings.Contains(err.Error(), errEmbedderDown.Error()) {
			log.Printf("seed: %v", err)
			os.Exit(3)
		}
		log.Printf("seed: %v", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*marker, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		log.Printf("seed: write marker: %v", err)
		os.Exit(1)
	}
	log.Printf("seed: finished, marker %s written", *marker)
}

// errFitSkipped reports a fit seed the run chose not to do: the local
// loop or an explicit opt-out. It is not a failure and writes no marker.
var errFitSkipped = fmt.Errorf("seed: fit seeding skipped for this run")

// fitMarker names the marker file proving a finished population seed for
// one index. It sits beside the graph marker on the same disk, so the two
// parts retry independently.
func fitMarker(marker, index string) string {
	return filepath.Join(filepath.Dir(marker), ".seeded-fit-"+index+"-v1")
}

// seedFit writes the T9.1 seeded population into one fit index, once per
// index, and reads every line back through the pool reader before the
// marker lands. The seeding moment is this run's now: the fixture's as-of
// anchor would drift the recent window if it were reused later.
func seedFit(indexName, marker string) error {
	if os.Getenv("MEMENTO_FIT_SEED") == "0" {
		return fmt.Errorf("%w: MEMENTO_FIT_SEED=0", errFitSkipped)
	}
	if os.Getenv("LAMBO_EMBEDDER") == "fixture" {
		return fmt.Errorf("%w: LAMBO_EMBEDDER=fixture never touches the live index", errFitSkipped)
	}
	markPath := fitMarker(marker, indexName)
	if _, err := os.Stat(markPath); err == nil {
		log.Printf("seed: fit marker %s present, population stands", markPath)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	asOf := time.Now().UTC().Truncate(time.Second)
	population, err := seed.GeneratePopulation(asOf)
	if err != nil {
		return err
	}
	baseURL := os.Getenv("ELASTIC_URL")
	apiKey := os.Getenv("LAMBO_ES_API_KEY")
	if baseURL == "" || apiKey == "" {
		return fmt.Errorf("seed: fit seeding needs ELASTIC_URL and LAMBO_ES_API_KEY in the environment")
	}
	writerIndex, err := fitindex.NewHTTPIndex(baseURL, apiKey, indexName)
	if err != nil {
		return fmt.Errorf("seed: fit index client: %w", err)
	}
	if err := writerIndex.Create(ctx); err != nil {
		return fmt.Errorf("seed: create fit index %q: %w", indexName, err)
	}
	writes, elapsed, err := seedRows(ctx, writerIndex, population.Rows)
	if err != nil {
		return err
	}
	log.Printf("seed: population holds: %d rows, %d index writes in %s (%.1f ms per write, refresh forced)",
		len(population.Rows), writes, elapsed.Round(time.Millisecond),
		float64(elapsed.Microseconds())/float64(writes)/1000.0)
	if err := verifyPopulation(ctx, baseURL, apiKey, indexName, asOf); err != nil {
		return err
	}
	if err := os.WriteFile(markPath, []byte(asOf.Format(time.RFC3339)+"\n"), 0o644); err != nil {
		return fmt.Errorf("seed: write fit marker: %w", err)
	}
	log.Printf("seed: fit population holds, marker %s written", markPath)
	return nil
}

// seedRows drives every population row through one shopper's writer, one
// purchase per shopper. A refund's evidence never carries a reason, so a
// reason-bearing refund is written Refunded and then Classified: the row
// settles only when the classified reason lands. The ids are deterministic
// in row order, so a rerun before the marker lands rewrites the same
// documents instead of doubling them.
func seedRows(ctx context.Context, writerIndex fitindex.Index, rows []domain.FitRow) (int, time.Duration, error) {
	keyer, err := identity.NewKeyer(identity.SeedSaltHex)
	if err != nil {
		return 0, 0, fmt.Errorf("seed: population keyer: %w", err)
	}
	start := time.Now()
	writes := 0
	for i, row := range rows {
		if err := ctx.Err(); err != nil {
			return writes, time.Since(start), fmt.Errorf("seed: population row %d: %w", i, err)
		}
		payer := fmt.Sprintf("seedshopper-live-%04d", i+1)
		writer, err := fitindex.NewWriter(writerIndex, keyer, payer, fitindex.LinesFunc(noLineResolution))
		if err != nil {
			return writes, time.Since(start), fmt.Errorf("seed: writer for row %d: %w", i, err)
		}
		before := writes
		if err := seedRow(ctx, writer, row, fmt.Sprintf("seedlive-%04d", i+1)); err != nil {
			return before, time.Since(start), fmt.Errorf("seed: population row %d: %w", i, err)
		}
		writes = before + transitionWrites(row)
	}
	return writes, time.Since(start), nil
}

// transitionWrites counts the index writes one row's transitions perform:
// a capture plus one write per further transition the row carries.
func transitionWrites(row domain.FitRow) int {
	writes := 1
	switch row.State {
	case domain.FitStateCaptured:
	case domain.FitStateRefunded:
		writes++
	case domain.FitStateReasonClassified:
		writes += 2
	case domain.FitStateFeedback, domain.FitStateKept:
		writes++
	}
	return writes
}

// noLineResolution never resolves a catalog line: the population drives
// writer transitions directly with the row's own line, so a resolution
// would mean the seed took the wrong path.
func noLineResolution(ctx context.Context, item domain.Item) (domain.ProductLine, error) {
	return domain.ProductLine{}, fmt.Errorf("seed: population rows carry their own line, item %q must not resolve", item.SKU)
}

// seedRow writes one population row through its shopper's writer, in
// state order. Every transition carries the row's own event time, so the
// stored document converges to the generated row.
func seedRow(ctx context.Context, writer *fitindex.Writer, row domain.FitRow, txnID string) error {
	purchase := fitindex.Purchase{TxnID: txnID, Line: row.Line, Size: row.Size, Seeded: true}
	if row.CapturedAt.IsZero() {
		return fmt.Errorf("seed: purchase %q has no capture time", txnID)
	}
	if err := writer.Captured(ctx, purchase, row.CapturedAt); err != nil {
		return fmt.Errorf("capture: %w", err)
	}
	switch row.State {
	case domain.FitStateCaptured:
		return nil
	case domain.FitStateRefunded:
		if row.RefundedAt.IsZero() {
			return fmt.Errorf("seed: purchase %q refunded with no refund time", txnID)
		}
		return writer.Refunded(ctx, purchase, row.RefundedAt)
	case domain.FitStateReasonClassified:
		if row.RefundedAt.IsZero() {
			return fmt.Errorf("seed: purchase %q classified with no refund time", txnID)
		}
		if err := writer.Refunded(ctx, purchase, row.RefundedAt); err != nil {
			return fmt.Errorf("refund: %w", err)
		}
		at := row.EventAt
		if row.Settled && !row.SettledAt.IsZero() {
			at = row.SettledAt
		}
		if err := writer.Classified(ctx, purchase, row.Reason, at); err != nil {
			return fmt.Errorf("classify: %w", err)
		}
		return nil
	case domain.FitStateFeedback:
		if row.Verdict == "" {
			return fmt.Errorf("seed: purchase %q has feedback state with no verdict", txnID)
		}
		return writer.Feedback(ctx, purchase, row.Verdict, row.SettledAt)
	case domain.FitStateKept:
		return writer.Kept(ctx, purchase, row.SettledAt)
	default:
		return fmt.Errorf("seed: purchase %q has unknown state %q", txnID, string(row.State))
	}
}

// expectedPools is the status table T9.1 pins: the verdict the pool
// reader must return for every seeded line and size after the seed.
var expectedPools = []struct {
	line      domain.ProductLine
	size      string
	status    domain.BeliefStatus
	direction domain.FitDirection
}{
	{line: seed.DemoOxfordLine(), size: seed.DemoOxfordSize, status: domain.StatusEarned, direction: domain.FitDirectionSmall},
	{line: seed.DemoOxfordLine(), size: "L", status: domain.StatusBuilding, direction: domain.FitDirectionLarge},
	{line: domain.ProductLine{Brand: "brand-y-002", Category: "shirts", FitHandle: "slim"}, size: "M", status: domain.StatusBuilding, direction: domain.FitDirectionSmall},
	{line: domain.ProductLine{Brand: "brand-z-003", Category: "shirts", FitHandle: "regular"}, size: "M", status: domain.StatusBuilding, direction: domain.FitDirectionSmall},
	{line: domain.ProductLine{Brand: "brand-v-004", Category: "shirts", FitHandle: "relaxed"}, size: "M", status: domain.StatusClaimed, direction: domain.FitDirectionNone},
	{line: domain.ProductLine{Brand: "brand-w-005", Category: "shirts", FitHandle: "regular"}, size: "M", status: domain.StatusOutOfTrue, direction: domain.FitDirectionSmall},
}

// verifyPopulation reads every seeded line back through the pool reader
// the merchant app and recall use, and refuses the seed when any verdict
// differs from the table T9.1 pins. The seeded and live split rides along
// so the ledger's counts are proved too.
func verifyPopulation(ctx context.Context, baseURL, apiKey, indexName string, asOf time.Time) error {
	readerIndex, err := pool.NewHTTPIndex(baseURL, apiKey, indexName)
	if err != nil {
		return fmt.Errorf("seed: pool client: %w", err)
	}
	reader, err := pool.NewReader(readerIndex)
	if err != nil {
		return fmt.Errorf("seed: pool reader: %w", err)
	}
	if err := readerIndex.Warm(ctx); err != nil {
		return fmt.Errorf("seed: warm pool connection: %w", err)
	}
	for _, want := range expectedPools {
		fit, err := reader.Fit(ctx, want.line, want.size)
		if err != nil {
			return fmt.Errorf("seed: read pool for %s/%s/%s size %s: %w",
				want.line.Brand, want.line.Category, want.line.FitHandle, want.size, err)
		}
		if fit.Verdict.Status != want.status || fit.Verdict.Direction != want.direction {
			return fmt.Errorf("seed: pool for %s/%s/%s size %s reads %s %s, want %s %s",
				want.line.Brand, want.line.Category, want.line.FitHandle, want.size,
				fit.Verdict.Status, fit.Verdict.Direction, want.status, want.direction)
		}
		log.Printf("seed: pool %s/%s/%s size %s: %s %s (settled %d, seeded %d, live %d)",
			want.line.Brand, want.line.Category, want.line.FitHandle, want.size,
			fit.Verdict.Status, fit.Verdict.Direction,
			fit.Counts.Settled, fit.Seeded.Settled, fit.Live.Settled)
	}
	return nil
}

func run(lamboBin, config, session, agent string) error {
	proc := exec.Command(lamboBin, "--config", config, "serve", "--session", session, "--agent", agent)
	stdin, err := proc.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	proc.Stderr = os.Stderr
	if err := proc.Start(); err != nil {
		return fmt.Errorf("start serve: %w", err)
	}
	defer func() {
		_ = proc.Wait()
	}()
	client := &client{agent: agent, seq: 0, in: stdin, scan: bufio.NewScanner(stdout)}
	client.scan.Buffer(make([]byte, 1<<20), 1<<24)

	finish := func(fail error) error {
		_ = client.in.Close()
		if fail != nil {
			return fail
		}
		return nil
	}

	if err := client.handshake(); err != nil {
		return finish(fmt.Errorf("handshake: %w", err))
	}

	anchor := time.Now().UTC().Add(-11 * 24 * time.Hour)
	at := func(day int) string {
		return anchor.Add(time.Duration(day) * 24 * time.Hour).Format(time.RFC3339Nano)
	}

	steps := []struct {
		what    string
		derive  bool
		content string
		kind    string
		depends []string
		against []string
		when    string
	}{
		{"sizing belief", true, belief, "logic", nil, nil, at(0)},
		{"general statement", true, assertion, "observation", nil, nil, at(0)},
		{"kept Brand Y", true, kept1, "observation", nil, nil, at(1)},
		{"kept Brand Z", true, kept2, "observation", nil, nil, at(2)},
		{"fit return 1", false, ret1, "", []string{belief}, nil, at(3)},
		{"fit return 2", false, ret2, "", []string{belief}, nil, at(6)},
		{"fit return 3", false, ret3, "", []string{belief}, nil, at(9)},
	}
	for _, step := range steps {
		var receipt string
		var err error
		if step.derive {
			receipt, err = client.derive(step.content, step.kind, step.when)
		} else {
			receipt, err = client.recordAction(step.content, step.depends, nil, step.when)
		}
		if err != nil {
			return finish(fmt.Errorf("%s: %w", step.what, err))
		}
		if err := client.waitApplied(receipt, step.what); err != nil {
			return finish(err)
		}
	}

	if err := client.waitFor(belief, "[Logic, venerable]"); err != nil {
		return finish(fmt.Errorf("promotion: %w", err))
	}
	log.Print("seed: sizing belief is Venerable")

	if sellerReceipt, err := client.derive(seller, "logic", at(11)); err != nil {
		return finish(fmt.Errorf("seller belief: %w", err))
	} else if err := client.waitApplied(sellerReceipt, "seller belief"); err != nil {
		return finish(err)
	}
	disputeReceipt, err := client.recordAction(dispute, []string{seller}, nil, at(11))
	if err != nil {
		return finish(fmt.Errorf("dispute: %w", err))
	}
	if err := client.waitApplied(disputeReceipt, "dispute"); err != nil {
		return finish(err)
	}

	correctionReceipt, err := client.recordAction(correction, nil, []string{belief}, "")
	if err != nil {
		return finish(fmt.Errorf("correction: %w", err))
	}
	if err := client.waitApplied(correctionReceipt, "correction"); err != nil {
		return finish(err)
	}

	seen, err := client.inspect(belief, 2)
	if err != nil {
		return finish(fmt.Errorf("final inspect: %w", err))
	}
	if !strings.Contains(seen, "[Logic, venerable]") || !strings.Contains(seen, "Contradicts") {
		return finish(fmt.Errorf("final inspect misses Venerable-plus-Contradicts: %.200s", seen))
	}
	recalled, err := client.recall(recallQ)
	if err != nil {
		return finish(fmt.Errorf("final recall: %w", err))
	}
	if !strings.Contains(recalled, "Contradicted") || !strings.Contains(recalled, belief) {
		return finish(fmt.Errorf("final recall misses the warned belief: %.200s", recalled))
	}
	log.Print("seed: correction declared, warning intact")
	return finish(nil)
}

// send writes one framed request.
func (c *client) send(message map[string]any) error {
	line, err := json.Marshal(message)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	if _, err := c.in.Write(line); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// recv reads one response line into a generic map.
func (c *client) recv() (map[string]any, error) {
	if !c.scan.Scan() {
		if err := c.scan.Err(); err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		return nil, fmt.Errorf("serve closed stdout mid-run")
	}
	var out map[string]any
	decoder := json.NewDecoder(bytes.NewReader(c.scan.Bytes()))
	decoder.UseNumber()
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return out, nil
}

// call sends one tools/call and returns the result object.
func (c *client) call(name string, args map[string]any) (map[string]any, error) {
	c.seq++
	if err := c.send(map[string]any{
		"jsonrpc": "2.0", "id": c.seq,
		"method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	}); err != nil {
		return nil, err
	}
	resp, err := c.recv()
	if err != nil {
		return nil, err
	}
	result, _ := resp["result"].(map[string]any)
	if result == nil {
		return nil, fmt.Errorf("%s: no result: %.200s", name, c.scan.Bytes())
	}
	return result, nil
}

// textOf concatenates a result's content texts.
func textOf(result map[string]any) string {
	var parts []string
	items, _ := result["content"].([]any)
	for _, item := range items {
		block, _ := item.(map[string]any)
		if block["type"] == "text" {
			text, _ := block["text"].(string)
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

// receiptOf reads the write receipt: structured content first, then a
// receipt field inside the first JSON text block.
func receiptOf(result map[string]any) string {
	if structured, _ := result["structuredContent"].(map[string]any); structured != nil {
		if receipt, _ := structured["receipt"].(string); receipt != "" {
			return receipt
		}
	}
	items, _ := result["content"].([]any)
	for _, item := range items {
		block, _ := item.(map[string]any)
		text, _ := block["text"].(string)
		var body map[string]any
		if err := json.Unmarshal([]byte(text), &body); err == nil {
			if receipt, _ := body["receipt"].(string); receipt != "" {
				return receipt
			}
		}
	}
	return ""
}

// handshake runs the MCP open: initialize, the initialized
// notification (no reply), and a tools/list to prove the surface.
func (c *client) handshake() error {
	c.seq = 1
	if err := c.send(map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05", "capabilities": map[string]any{},
			"clientInfo": map[string]any{"name": "memento-seed", "version": "0.1.0"},
		},
	}); err != nil {
		return err
	}
	if _, err := c.recv(); err != nil {
		return err
	}
	if err := c.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		return err
	}
	c.seq = 2
	if err := c.send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}); err != nil {
		return err
	}
	if _, err := c.recv(); err != nil {
		return err
	}
	return nil
}

// derive writes one belief or statement, returning its receipt.
func (c *client) derive(content, kind, eventTime string) (string, error) {
	args := map[string]any{
		"agent_id": c.agent,
		"concepts": []any{map[string]any{"content": content, "concept_type": kind}},
	}
	if eventTime != "" {
		args["event_time"] = eventTime
	}
	result, err := c.call("lambo_derive", args)
	if err != nil {
		return "", err
	}
	receipt := receiptOf(result)
	if receipt == "" {
		return "", fmt.Errorf("derive issued no receipt: %.200s", textOf(result))
	}
	return receipt, nil
}

// recordAction writes one structural edge, returning its receipt.
func (c *client) recordAction(action string, dependsOn, against []string, eventTime string) (string, error) {
	args := map[string]any{"agent_id": c.agent, "action": action}
	if dependsOn != nil {
		args["depends_on"] = dependsOn
	}
	if against != nil {
		args["against"] = against
	}
	if eventTime != "" {
		args["event_time"] = eventTime
	}
	result, err := c.call("lambo_record_action", args)
	if err != nil {
		return "", err
	}
	receipt := receiptOf(result)
	if receipt == "" {
		return "", fmt.Errorf("record_action issued no receipt: %.200s", textOf(result))
	}
	return receipt, nil
}

// waitApplied waits for one receipt to apply. An unreachable embedder
// reports the sentinel so the caller can fall back; any other miss is
// a hard failure.
func (c *client) waitApplied(receipt, what string) error {
	result, err := c.call("lambo_stats", map[string]any{
		"agent_id": c.agent, "receipt": receipt, "wait_ms": waitMS,
	})
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	text := textOf(result)
	switch {
	case strings.Contains(text, "embedder unavailable") || strings.Contains(text, "error sending request"):
		return fmt.Errorf("%s: %w: %.200s", what, errEmbedderDown, text)
	case strings.Contains(text, "applied"):
		return nil
	default:
		return fmt.Errorf("%s: receipt never applied: %.200s", what, text)
	}
}

// probeOnly opens a scratch session, derives one concept and waits
// for its receipt. It reports false (not an error) when the embedder
// is down, so the caller can try the spare contract before failing.
// The scratch session stays in the store: sessions isolate shoppers,
// and one observation per boot is the cheapest honest probe.
func probeOnly(lamboBin, config, session, agent string) (bool, error) {
	proc := exec.Command(lamboBin, "--config", config, "serve", "--session", session, "--agent", agent)
	stdin, err := proc.StdinPipe()
	if err != nil {
		return false, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return false, fmt.Errorf("stdout pipe: %w", err)
	}
	proc.Stderr = os.Stderr
	if err := proc.Start(); err != nil {
		return false, fmt.Errorf("start serve: %w", err)
	}
	defer func() {
		_ = proc.Wait()
	}()
	probeClient := &client{agent: agent, in: stdin, scan: bufio.NewScanner(stdout)}
	probeClient.scan.Buffer(make([]byte, 1<<20), 1<<24)
	defer func() {
		_ = probeClient.in.Close()
	}()
	if err := probeClient.handshake(); err != nil {
		return false, fmt.Errorf("handshake: %w", err)
	}
	return probeClient.probeEmbedder()
}

// probeEmbedder derives one scratch concept and waits for its receipt.
// It reports false (not an error) when the embedder is down, so the
// caller can try the spare contract before failing.
func (c *client) probeEmbedder() (bool, error) {
	receipt, err := c.derive(probe, "observation", "")
	if err != nil {
		return false, fmt.Errorf("probe derive: %w", err)
	}
	if err := c.waitApplied(receipt, "probe"); err != nil {
		if strings.Contains(err.Error(), errEmbedderDown.Error()) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// inspect returns the neighbourhood text around one concept.
func (c *client) inspect(focus string, depth int) (string, error) {
	result, err := c.call("lambo_inspect", map[string]any{
		"agent_id": c.agent, "focus": focus, "depth": depth,
	})
	if err != nil {
		return "", err
	}
	return textOf(result), nil
}

// recall returns the recall text for one query.
func (c *client) recall(query string) (string, error) {
	result, err := c.call("lambo_recall", map[string]any{"agent_id": c.agent, "query": query})
	if err != nil {
		return "", err
	}
	return textOf(result), nil
}

// waitFor polls inspect until the focus text contains the wanted mark.
// Promotion is eventual: the daemon evaluates on its own cadence, so
// the verdict right after the last receipt can predate the flush.
func (c *client) waitFor(focus, want string) error {
	deadline := time.Now().Add(convergeTimeout)
	var seen string
	for time.Now().Before(deadline) {
		text, err := c.inspect(focus, 2)
		if err != nil {
			return err
		}
		seen = text
		if strings.Contains(text, want) {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timed out waiting for %q: %.200s", want, seen)
}
