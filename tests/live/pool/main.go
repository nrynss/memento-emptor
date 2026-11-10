// Live driver for T8.3b: the pool reader, against the real Elastic
// deployment.
//
// A small seeded population is written to the test-prefixed index
// t83b-fit-rows through the fit index writer, then read back through the
// pool reader's product_fit path: one line-and-size aggregation over
// settled rows, the seeded and live split, and the recent window. The run
// records a cold latency (a fresh reader with its own transport, so the
// first read pays the handshake) and warm latencies (the same reader
// reused), plus an empty size that reads as a valid empty pool. It deletes
// the index at the end, whether or not the checks passed, and shows the
// empty prefix listing.
//
// This is a main program, never a test: no *_test.go lives under
// tests/live/, so CI builds this package but never runs it. Run it by hand
// with the keys loaded:
//
//	source tools/env.sh
//	bash tests/live/pool/run.sh
//
// The deployment URL and the keys never reach stdout, a file or an error
// message: the transcript names the deployment as <deployment>, and the
// driver redacts the URL from any transport error it reports.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
	"github.com/nrynss/memento-emptor/internal/pool"
	"github.com/nrynss/memento-emptor/seed"
)

// The test namespace. Every index this driver writes is under this prefix,
// and the driver refuses to run when the prefix holds anything else, so it
// never touches an index it did not create.
const (
	indexPrefix = "t83b-"
	indexName   = indexPrefix + "fit-rows"
)

// The seeded population: eight shoppers on the demo line in one size. Five
// point small by return reason, one points large, one says the size fits,
// and one kept the purchase. Every row is seeded, so the seeded split
// carries the whole pool and the live split stays empty.
const populationSize = 8

// entry is one seeded purchase: who bought, under which transaction id,
// and how it settled.
type entry struct {
	payer  string
	txn    string
	settle string
}

// The population plan, in write order. Transaction and payer ids are the
// driver's own synthetic values.
func population() []entry {
	return []entry{
		{payer: "t83b-shopper-01", txn: "T83B-TXN-01", settle: "small"},
		{payer: "t83b-shopper-02", txn: "T83B-TXN-02", settle: "small"},
		{payer: "t83b-shopper-03", txn: "T83B-TXN-03", settle: "small"},
		{payer: "t83b-shopper-04", txn: "T83B-TXN-04", settle: "small"},
		{payer: "t83b-shopper-05", txn: "T83B-TXN-05", settle: "small"},
		{payer: "t83b-shopper-06", txn: "T83B-TXN-06", settle: "large"},
		{payer: "t83b-shopper-07", txn: "T83B-TXN-07", settle: "fitswell"},
		{payer: "t83b-shopper-08", txn: "T83B-TXN-08", settle: "kept"},
	}
}

// responseBytes bounds how much of a response the probe reads.
const responseBytes = 1 << 20

// probe is the driver's read and cleanup client. It lists the prefix,
// counts the rows and deletes the test index; the writes go through
// fitindex.HTTPIndex and the reads through pool.HTTPIndex.
type probe struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// newProbe builds the read client. A cold connection to the deployment
// took about 6.9 s in T0.12, so the timeout is above 20 s.
func newProbe(baseURL, apiKey string) *probe {
	return &probe{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// do sends one request and returns its status and body.
func (p *probe) do(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Authorization", "ApiKey "+p.apiKey)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := p.client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	raw, err := io.ReadAll(io.LimitReader(response.Body, responseBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("read response: %w", err)
	}
	return response.StatusCode, raw, nil
}

// indices lists the index names under the prefix. An empty prefix answers
// an empty list, which is how the transcript shows cleanup.
func (p *probe) indices(ctx context.Context, prefix string) ([]string, error) {
	status, body, err := p.do(ctx, http.MethodGet,
		"/_cat/indices/"+prefix+"*?format=json&h=index", nil)
	if err != nil {
		return nil, fmt.Errorf("list %s indices: %w", prefix, err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("list %s indices: status %d: %s", prefix, status, excerpt(body))
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

// count returns how many documents match one term query, or the whole
// index's count when the field is empty.
func (p *probe) count(ctx context.Context, field, value string) (int, error) {
	if field == "" {
		status, body, err := p.do(ctx, http.MethodGet, "/"+indexName+"/_count", nil)
		if err != nil {
			return 0, fmt.Errorf("count %s: %w", indexName, err)
		}
		if status != http.StatusOK {
			return 0, fmt.Errorf("count %s: status %d: %s", indexName, status, excerpt(body))
		}
		var result struct {
			Count int `json:"count"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return 0, fmt.Errorf("read %s count: %w", indexName, err)
		}
		return result.Count, nil
	}
	query := fmt.Sprintf(`{"query":{"term":{%q:%q}}}`, field, value)
	status, body, err := p.do(ctx, http.MethodPost, "/"+indexName+"/_count", []byte(query))
	if err != nil {
		return 0, fmt.Errorf("count rows for %s: %w", field, err)
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("count rows for %s: status %d: %s", field, status, excerpt(body))
	}
	var result struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("read %s count: %w", field, err)
	}
	return result.Count, nil
}

// run writes the population and reads it back through the pool reader,
// reporting the transcript on stdout.
func run(ctx context.Context) (err error) {
	elasticURL, err := required("ELASTIC_URL")
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

	p := newProbe(elasticURL, apiKey)
	created := false
	defer func() {
		if cerr := cleanup(p, created); cerr != nil {
			if err == nil {
				err = cerr
			} else {
				fmt.Fprintf(os.Stderr, "cleanup also failed: %v\n", cerr)
			}
		}
	}()

	line := seed.DemoOxfordLine()
	size := seed.DemoOxfordSize
	plan := population()

	fmt.Println("T8.3b live pool reader")
	fmt.Println("deployment: <deployment> (elastic, T0.12's project)")
	fmt.Printf("index: %s\n", indexName)
	fmt.Printf("line: %s/%s/%s size %s, %d seeded purchases\n\n",
		line.Brand, line.Category, line.FitHandle, size, len(plan))

	before, err := p.indices(ctx, indexPrefix)
	if err != nil {
		return err
	}
	fmt.Printf("-- prefix listing before: GET /_cat/indices/%s* -> %v\n", indexPrefix, before)
	if len(before) != 0 {
		return fmt.Errorf("prefix %s is not clean: %v; refusing to run so no index is deleted by mistake", indexPrefix, before)
	}

	writerIndex, err := fitindex.NewHTTPIndex(elasticURL, apiKey, indexName)
	if err != nil {
		return fmt.Errorf("build fit index client: %w", err)
	}
	if err := writerIndex.Create(ctx); err != nil {
		return fmt.Errorf("create index %s: %w", indexName, err)
	}
	created = true
	fmt.Printf("-- create: PUT /%s with the recorded mapping (no settings) -> ok\n", indexName)

	keyer, err := identity.NewKeyer(salt)
	if err != nil {
		return fmt.Errorf("build identity keyer: %w", err)
	}

	fmt.Printf("\n== 1. write %d seeded rows through the fit writer\n", len(plan))
	base := time.Now().UTC().Truncate(time.Second)
	for i, e := range plan {
		writer, err := fitindex.NewWriter(writerIndex, keyer, e.payer, fitindex.LinesFunc(fixedLines))
		if err != nil {
			return fmt.Errorf("build fit writer for %s: %w", e.payer, err)
		}
		purchase := fitindex.Purchase{TxnID: e.txn, Line: line, Size: size, Seeded: true}
		capturedAt := base.Add(-time.Hour).Add(time.Duration(i) * time.Minute)
		if err := writer.Captured(ctx, purchase, capturedAt); err != nil {
			return fmt.Errorf("capture %s: %w", e.txn, err)
		}
		switch e.settle {
		case "small", "large":
			refundedAt := capturedAt.Add(10 * time.Minute)
			if err := writer.Refunded(ctx, purchase, refundedAt); err != nil {
				return fmt.Errorf("refund %s: %w", e.txn, err)
			}
			category := domain.CategoryFitTooSmall
			if e.settle == "large" {
				category = domain.CategoryFitTooLarge
			}
			if err := writer.Classified(ctx, purchase, category, refundedAt.Add(20*time.Minute)); err != nil {
				return fmt.Errorf("classify %s: %w", e.txn, err)
			}
		case "fitswell":
			if err := writer.Feedback(ctx, purchase, domain.FitVerdictFitsWell, capturedAt.Add(30*time.Minute)); err != nil {
				return fmt.Errorf("feedback %s: %w", e.txn, err)
			}
		case "kept":
			if err := writer.Kept(ctx, purchase, capturedAt.Add(30*time.Minute)); err != nil {
				return fmt.Errorf("keep %s: %w", e.txn, err)
			}
		default:
			return fmt.Errorf("unknown settlement %q for %s", e.settle, e.txn)
		}
		fmt.Printf("   %s: captured, %s\n", e.txn, settleWord(e.settle))
	}

	total, err := p.count(ctx, "", "")
	if err != nil {
		return err
	}
	settled, err := p.count(ctx, "settled", "true")
	if err != nil {
		return err
	}
	fmt.Printf("   docs in %s: %d, settled: %d\n", indexName, total, settled)
	if total != len(plan) || settled != len(plan) {
		return fmt.Errorf("counts: total=%d settled=%d, want %d and %d", total, settled, len(plan), len(plan))
	}

	fmt.Printf("\n== 2. cold read: a fresh reader pays the handshake\n")
	coldReaderIndex, err := pool.NewHTTPIndex(elasticURL, apiKey, indexName)
	if err != nil {
		return fmt.Errorf("build cold pool client: %w", err)
	}
	coldReader, err := pool.NewReader(coldReaderIndex)
	if err != nil {
		return fmt.Errorf("build cold pool reader: %w", err)
	}
	coldStart := time.Now()
	cold, err := coldReader.Fit(ctx, line, size)
	coldElapsed := time.Since(coldStart)
	if err != nil {
		return fmt.Errorf("cold pool read: %w", err)
	}
	fmt.Printf("   cold Fit: %s in %s\n", describe(cold), coldElapsed.Round(time.Millisecond))

	fmt.Printf("\n== 3. warm reads: the same reader reuses its connection\n")
	warmReaderIndex, err := pool.NewHTTPIndex(elasticURL, apiKey, indexName)
	if err != nil {
		return fmt.Errorf("build warm pool client: %w", err)
	}
	warmReader, err := pool.NewReader(warmReaderIndex)
	if err != nil {
		return fmt.Errorf("build warm pool reader: %w", err)
	}
	warmStart := time.Now()
	if err := warmReader.Warm(ctx); err != nil {
		return fmt.Errorf("warm the pool connection: %w", err)
	}
	fmt.Printf("   Warm: %s\n", time.Since(warmStart).Round(time.Millisecond))
	var warm []time.Duration
	var warmFit pool.Fit
	for i := 1; i <= 3; i++ {
		start := time.Now()
		fit, err := warmReader.Fit(ctx, line, size)
		elapsed := time.Since(start)
		if err != nil {
			return fmt.Errorf("warm pool read %d: %w", i, err)
		}
		warm = append(warm, elapsed)
		warmFit = fit
		fmt.Printf("   warm Fit %d: %s in %s\n", i, describe(fit), elapsed.Round(time.Millisecond))
	}

	fmt.Printf("\n== 4. the counts the pool reports\n")
	if err := wantCounts(cold, warmFit); err != nil {
		return err
	}
	printCounts(cold)

	fmt.Printf("\n== 5. an empty size is a valid empty pool, not an error\n")
	empty, err := warmReader.Fit(ctx, line, "XXL")
	if err != nil {
		return fmt.Errorf("read the empty size: %w", err)
	}
	fmt.Printf("   size XXL: %s\n", describe(empty))
	if empty.Verdict.Status != domain.StatusClaimed || empty.Counts.Settled != 0 {
		return fmt.Errorf("empty size: status=%q settled=%d, want claimed with zero counts",
			empty.Verdict.Status, empty.Counts.Settled)
	}

	fmt.Printf("\n== latencies\n")
	sorted := append([]time.Duration(nil), warm...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	fmt.Printf("   cold (fresh transport, first Fit): %s\n", coldElapsed.Round(time.Millisecond))
	fmt.Printf("   warm (reused connection, 3 reads):")
	for _, d := range warm {
		fmt.Printf(" %s", d.Round(time.Millisecond))
	}
	fmt.Printf(", median %s\n", sorted[len(sorted)/2].Round(time.Millisecond))

	fmt.Println("\nmodel spend: $0 (no model call: the population is written directly through the fit writer)")
	fmt.Printf("elastic: one kilobyte-scale index, %d row writes, reads and counts; deleted below\n", 22)
	return nil
}

// fixedLines resolves every item to the demo line. Only the catalog knows
// a line's fit handle; a production resolver belongs in agent/catalog
// (T8.5 wires it), and this driver uses a function.
func fixedLines(_ context.Context, _ domain.Item) (domain.ProductLine, error) {
	return seed.DemoOxfordLine(), nil
}

// settleWord names what settled one population row.
func settleWord(settle string) string {
	switch settle {
	case "small":
		return "returned, classified fit_too_small"
	case "large":
		return "returned, classified fit_too_large"
	case "fitswell":
		return "feedback fits_well"
	case "kept":
		return "kept past the window"
	default:
		return settle
	}
}

// describe renders one pool read as its verdict and denominator.
func describe(fit pool.Fit) string {
	return fmt.Sprintf("status=%s direction=%s settled=%d seeded=%d live=%d recent=%d",
		fit.Verdict.Status, directionWord(fit.Verdict.Direction),
		fit.Counts.Settled, fit.Seeded.Settled, fit.Live.Settled, fit.Recent.Settled)
}

// directionWord renders an absent direction as a dash.
func directionWord(direction domain.FitDirection) string {
	if direction == "" || direction == domain.FitDirectionNone {
		return "-"
	}
	return string(direction)
}

// wantCounts checks the cold and warm reads agree and match the
// population: five small returns, one large return, one fits-well
// feedback and one kept purchase, all seeded and all inside the recent
// window. Seven fit-classified shoppers clear the three-shopper floor but
// five pointing small are below the eight-shopper Earned floor, so the
// pool reads Building toward small. The kept purchase settles the
// denominator without joining the fit-shopper count, which is exactly the
// honesty the denominator needs.
func wantCounts(cold, warm pool.Fit) error {
	if cold.Verdict != warm.Verdict || !countsEqual(cold.Counts, warm.Counts) {
		return fmt.Errorf("cold and warm reads disagree: cold %s, warm %s", describe(cold), describe(warm))
	}
	fit := warm
	var problems []string
	if fit.Verdict.Status != domain.StatusBuilding || fit.Verdict.Direction != domain.FitDirectionSmall {
		problems = append(problems, fmt.Sprintf("verdict = %s/%s, want building/small",
			fit.Verdict.Status, directionWord(fit.Verdict.Direction)))
	}
	if fit.Counts.Settled != populationSize {
		problems = append(problems, fmt.Sprintf("settled = %d, want %d", fit.Counts.Settled, populationSize))
	}
	if fit.Counts.Kept != 1 {
		problems = append(problems, fmt.Sprintf("kept = %d, want 1", fit.Counts.Kept))
	}
	if fit.Counts.Returns.TooSmall != 5 || fit.Counts.Returns.TooLarge != 1 {
		problems = append(problems, fmt.Sprintf("returns = small %d large %d, want 5 and 1",
			fit.Counts.Returns.TooSmall, fit.Counts.Returns.TooLarge))
	}
	if fit.Counts.Feedback.FitsWell != 1 {
		problems = append(problems, fmt.Sprintf("fits-well feedback = %d, want 1", fit.Counts.Feedback.FitsWell))
	}
	if fit.Counts.Reasons[domain.CategoryFitTooSmall] != 5 || fit.Counts.Reasons[domain.CategoryFitTooLarge] != 1 {
		problems = append(problems, fmt.Sprintf("reasons = %v, want map[fit_too_small:5 fit_too_large:1]", fit.Counts.Reasons))
	}
	if fit.Counts.Shoppers.Fit != 7 || fit.Counts.Shoppers.TooSmall != 5 || fit.Counts.Shoppers.TooLarge != 1 {
		problems = append(problems, fmt.Sprintf("shoppers = %+v, want fit 7 small 5 large 1 (the kept purchase settles the denominator but is not fit evidence)", fit.Counts.Shoppers))
	}
	if fit.Seeded.Settled != populationSize || fit.Live.Settled != 0 {
		problems = append(problems, fmt.Sprintf("seeded/live settled = %d/%d, want %d/0",
			fit.Seeded.Settled, fit.Live.Settled, populationSize))
	}
	if !countsEqual(fit.Recent, fit.Counts) {
		problems = append(problems, fmt.Sprintf("recent settled = %d, want the whole pool (%d): every row settled inside the window",
			fit.Recent.Settled, fit.Counts.Settled))
	}
	if len(problems) > 0 {
		return fmt.Errorf("pool counts: %s", strings.Join(problems, "; "))
	}
	return nil
}

// countsEqual compares two settled buckets field by field.
func countsEqual(a, b pool.Counts) bool {
	if a.Settled != b.Settled || a.Kept != b.Kept ||
		a.Returns != b.Returns || a.Feedback != b.Feedback ||
		a.Shoppers != b.Shoppers {
		return false
	}
	if len(a.Reasons) != len(b.Reasons) {
		return false
	}
	for reason, count := range a.Reasons {
		if b.Reasons[reason] != count {
			return false
		}
	}
	return true
}

// printCounts prints the settled bucket the pool reports.
func printCounts(fit pool.Fit) {
	fmt.Printf("   verdict: status=%s direction=%s\n", fit.Verdict.Status, directionWord(fit.Verdict.Direction))
	fmt.Printf("   all: settled=%d kept=%d returns small=%d large=%d feedback fits_well=%d too_small=%d too_large=%d reasons=%v shoppers=%+v\n",
		fit.Counts.Settled, fit.Counts.Kept,
		fit.Counts.Returns.TooSmall, fit.Counts.Returns.TooLarge,
		fit.Counts.Feedback.FitsWell, fit.Counts.Feedback.TooSmall, fit.Counts.Feedback.TooLarge,
		fit.Counts.Reasons, fit.Counts.Shoppers)
	fmt.Printf("   seeded settled=%d live settled=%d recent settled=%d\n",
		fit.Seeded.Settled, fit.Live.Settled, fit.Recent.Settled)
}

// cleanup deletes the test index and shows the empty prefix listing. It runs
// whether or not the checks passed, so a failed run leaves no index behind.
// It only ever deletes the one name this driver creates, after checking the
// prefix.
func cleanup(p *probe, created bool) error {
	if !created {
		return nil
	}
	if !strings.HasPrefix(indexName, indexPrefix) || indexName == indexPrefix {
		return fmt.Errorf("refusing to delete %q: it is not a %s index name", indexName, indexPrefix)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	status, body, err := p.do(ctx, http.MethodDelete, "/"+indexName, nil)
	if err != nil {
		return fmt.Errorf("delete index %s: %w", indexName, err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("delete index %s: status %d: %s", indexName, status, excerpt(body))
	}
	fmt.Printf("\n-- cleanup: DELETE /%s -> %d %s\n", indexName, status, strings.TrimSpace(string(body)))
	after, err := p.indices(ctx, indexPrefix)
	if err != nil {
		return err
	}
	fmt.Printf("-- prefix listing after: GET /_cat/indices/%s* -> %v\n", indexPrefix, after)
	if len(after) != 0 {
		return fmt.Errorf("%d index(es) remain under %s: %v", len(after), indexPrefix, after)
	}
	return nil
}

// required reads one environment variable, naming it and the loader that
// sets it when it is missing. It never echoes a value.
func required(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("%s is not set; run `source tools/env.sh` first", name)
	}
	return value, nil
}

// excerpt shortens a response body for an error message.
func excerpt(body []byte) string {
	const width = 200
	if len(body) <= width {
		return string(body)
	}
	return fmt.Sprintf("%s… (%d bytes)", body[:width], len(body))
}

// redact removes the deployment locator and the secrets from a message
// before it is printed. A transport error carries the request URL, and the
// committed transcript names the deployment as <deployment>.
func redact(message string, secrets ...string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		message = strings.ReplaceAll(message, secret, "<deployment>")
	}
	return message
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL T8.3b live pool reader: %s\n",
			redact(err.Error(), os.Getenv("ELASTIC_URL"), os.Getenv("LAMBO_ES_API_KEY"), os.Getenv("MEMENTO_IDENTITY_SALT")))
		os.Exit(1)
	}
	fmt.Println("\nPASS T8.3b live pool reader: eight seeded rows, one pool read warm and cold, index deleted")
}
