// Live driver for T8.2b: the fit index writer, against the real Elastic
// deployment.
//
// One recorded sandbox purchase is driven through ingest with the fit
// writer attached, so every fit row is written from the same event that
// records the shopper's evidence. The purchase is the T0.4 cycle the
// recordings hold end to end: order ORDER-ID-US-1, captured as
// CAPTURE-ID-US-1 (testdata/paypal/capture-completed.json) and refunded as
// REFUND-ID-US-1 (testdata/paypal/refund-record.json), with the order's
// line item in testdata/paypal/order-get-approved-line-items.json. The
// capture creates the row, the refund marks it returned and unsettled, and
// the shopper's answer settles it with a classified reason. The answer is
// the recorded T0.7 one ("too tight in the shoulders"), classified by the
// recorded classifier, so no model is called.
//
// The row is written through fitindex.HTTPIndex against the deployment,
// into the test-prefixed index t82b-fit-rows, created from the recorded
// mapping. The driver shows one document per purchase, each state's
// version, a replayed delivery, a retried fit write, and a fresh writer
// whose older event the index refuses. It deletes the index at the end,
// whether or not the checks passed, and shows the empty prefix listing.
//
// This is a main program, never a test: no *_test.go lives under
// tests/live/, so CI builds this package but never runs it. Run it by hand
// with the keys loaded:
//
//	source tools/env.sh
//	bash tests/live/fitindex/run.sh
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
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
	"github.com/nrynss/memento-emptor/internal/ingest"
	"github.com/nrynss/memento-emptor/seed"
)

// The test namespace. Every index this driver writes is under this prefix,
// and the driver refuses to run when the prefix holds anything else, so it
// never touches an index it did not create.
const (
	indexPrefix = "t82b-"
	indexName   = indexPrefix + "fit-rows"
)

// The recorded purchase. The ids are the recording's placeholders: the
// fixtures redact the real sandbox ids, and these are the values committed
// with the shape.
const (
	payerID   = "shopper-demo-001" // the recorded purchase's custom id
	orderID   = "ORDER-ID-US-1"
	captureID = "CAPTURE-ID-US-1"
	refundID  = "REFUND-ID-US-1"

	// The delivery ids key ingest's idempotency. They are not purchase
	// state: the recorded deliveries T0.4 kept are the refund's
	// (WEBHOOK-EVENT-US-1, for the second cycle); these two name the
	// deliveries of this cycle so a replay can be shown.
	captureEventID = "WH-T82B-CAPTURE-1"
	refundEventID  = "WH-T82B-REFUND-1"
)

// The recorded fixtures the purchase and its answer come from.
const (
	captureRecordFixture = "testdata/paypal/capture-completed.json"
	refundRecordFixture  = "testdata/paypal/refund-record.json"
	orderFixture         = "testdata/paypal/order-get-approved-line-items.json"
	classifierDir        = "testdata/jev"
	answerFixture        = "testdata/jev/01-fit-too-small-tight-shoulders.json"
)

// The recorded event times. The capture and the refund carry them in their
// recordings; the shopper's answer arrives after the refund, at a fixed
// moment so the run is deterministic.
var (
	captureAt  = time.Date(2026, 11, 9, 14, 12, 1, 0, time.UTC) // capture-completed.json create_time
	refundAt   = time.Date(2026, 11, 9, 14, 12, 9, 0, time.UTC) // refund-record.json create_time (06:12:09-08:00)
	classifyAt = refundAt.Add(20 * time.Minute)
)

// responseBytes bounds how much of a response the probe reads.
const responseBytes = 1 << 20

// docSource is one fit row's stored fields, as the index returns them.
type docSource struct {
	TxnHash         string `json:"txn_hash"`
	ShopperHash     string `json:"shopper_hash"`
	Brand           string `json:"brand"`
	Category        string `json:"category"`
	FitHandle       string `json:"fit_handle"`
	Size            string `json:"size"`
	PurchaseState   string `json:"purchase_state"`
	Settled         bool   `json:"settled"`
	SettledKind     string `json:"settled_kind"`
	ReasonCategory  string `json:"reason_category"`
	FitDirection    string `json:"fit_direction"`
	FeedbackVerdict string `json:"feedback_verdict"`
	ReasonContested bool   `json:"reason_contested"`
	Seeded          bool   `json:"seeded"`
	CapturedAt      string `json:"captured_at"`
	RefundedAt      string `json:"refunded_at"`
	SettledAt       string `json:"settled_at"`
}

// String renders the row's state without the hashes' full width, so a
// transcript line stays readable.
func (s docSource) String() string {
	return fmt.Sprintf(
		"state=%s settled=%t kind=%s reason=%s direction=%s verdict=%s contested=%t seeded=%t captured_at=%s refunded_at=%s settled_at=%s line=%s/%s/%s size=%s",
		s.PurchaseState, s.Settled, orDash(s.SettledKind), orDash(s.ReasonCategory),
		orDash(s.FitDirection), orDash(s.FeedbackVerdict), s.ReasonContested, s.Seeded,
		orDash(s.CapturedAt), orDash(s.RefundedAt), orDash(s.SettledAt),
		s.Brand, s.Category, s.FitHandle, s.Size)
}

// snapshot is one read of a purchase's document: the external version the
// index holds and the stored source, both raw and decoded.
type snapshot struct {
	Version int64
	Source  json.RawMessage
	Doc     docSource
}

// String renders one snapshot.
func (s snapshot) String() string {
	return fmt.Sprintf("version=%d %s", s.Version, s.Doc)
}

// probe is the driver's read and cleanup client. It reads the index the
// writer wrote, counts the rows and deletes the test index; the writes
// themselves go through fitindex.HTTPIndex.
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

// snapshot reads one purchase's document. A missing row is an error: every
// step of this run expects the row to be there.
func (p *probe) snapshot(ctx context.Context, txnHash string) (snapshot, error) {
	status, body, err := p.do(ctx, http.MethodGet, "/"+indexName+"/_doc/"+txnHash, nil)
	if err != nil {
		return snapshot{}, fmt.Errorf("read row %s: %w", short(txnHash), err)
	}
	if status != http.StatusOK {
		return snapshot{}, fmt.Errorf("read row %s: status %d: %s", short(txnHash), status, excerpt(body))
	}
	var envelope struct {
		Version int64           `json:"_version"`
		Source  json.RawMessage `json:"_source"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return snapshot{}, fmt.Errorf("decode row %s: %w", short(txnHash), err)
	}
	var source docSource
	if err := json.Unmarshal(envelope.Source, &source); err != nil {
		return snapshot{}, fmt.Errorf("decode row %s source: %w", short(txnHash), err)
	}
	return snapshot{Version: envelope.Version, Source: envelope.Source, Doc: source}, nil
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

// run drives the purchase and reports the transcript on stdout.
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

	fmt.Println("T8.2b live fit index writer")
	fmt.Println("deployment: <deployment> (elastic, T0.12's project)")
	fmt.Printf("index: %s\n\n", indexName)

	before, err := p.indices(ctx, indexPrefix)
	if err != nil {
		return err
	}
	fmt.Printf("-- prefix listing before: GET /_cat/indices/%s* -> %v\n", indexPrefix, before)
	if len(before) != 0 {
		return fmt.Errorf("prefix %s is not clean: %v; refusing to run so no index is deleted by mistake", indexPrefix, before)
	}

	index, err := fitindex.NewHTTPIndex(elasticURL, apiKey, indexName)
	if err != nil {
		return fmt.Errorf("build index client: %w", err)
	}
	if err := index.Create(ctx); err != nil {
		return fmt.Errorf("create index %s: %w", indexName, err)
	}
	created = true
	fmt.Printf("-- create: PUT /%s with the recorded mapping (no settings) -> ok\n", indexName)

	keyer, err := identity.NewKeyer(salt)
	if err != nil {
		return fmt.Errorf("build identity keyer: %w", err)
	}
	writer, err := fitindex.NewWriter(index, keyer, payerID, fitindex.LinesFunc(demoLines))
	if err != nil {
		return fmt.Errorf("build fit writer: %w", err)
	}
	txnHash, err := identity.TxnHash(captureID)
	if err != nil {
		return fmt.Errorf("hash the transaction: %w", err)
	}
	shopperHash, err := keyer.LineShopperHash(payerID, seed.DemoOxfordLine())
	if err != nil {
		return fmt.Errorf("hash the shopper for the line: %w", err)
	}
	fmt.Printf("-- purchase: capture %s, order %s, refund %s, %s/%s/%s size %s\n",
		captureID, orderID, refundID,
		seed.DemoOxfordBrand, seed.DemoOxfordCategory, seed.DemoOxfordFitHandle, seed.DemoOxfordSize)
	fmt.Printf("-- row id: sha256(\"txn|\"+%s) = %s\n", captureID, short(txnHash))
	fmt.Printf("-- shopper hash for the line (live salt): %s\n", short(shopperHash))

	service := ingest.NewService(
		fakes.NewRecordingPaymentSource(purchaseInputs()),
		ingest.WithClock(eventClock()),
		ingest.WithFitRecorder(writer),
	)

	fmt.Printf("\n== 1. capture delivery %s -> ingest -> writer.Captured\n", captureEventID)
	captureResult, err := service.Handle(ctx, captureEventID)
	if err != nil {
		return fmt.Errorf("ingest capture: %w", err)
	}
	fmt.Printf("   ingest: %d evidence row(s), duplicate=%v, pending question=%v\n",
		len(captureResult.Rows), captureResult.Duplicate, captureResult.Pending != nil)
	captured, err := p.snapshot(ctx, txnHash)
	if err != nil {
		return err
	}
	fmt.Printf("   doc: %s\n", captured)
	if err := wantState(captured, domain.FitStateCaptured, false, rowVersion(domain.FitStateCaptured, captureAt), "after capture"); err != nil {
		return err
	}
	if captured.Doc.CapturedAt != captureAt.Format(time.RFC3339) || captured.Doc.RefundedAt != "" || captured.Doc.SettledAt != "" {
		return fmt.Errorf("after capture: captured_at=%q refunded_at=%q settled_at=%q, want the capture time only",
			captured.Doc.CapturedAt, captured.Doc.RefundedAt, captured.Doc.SettledAt)
	}

	fmt.Printf("\n== 2. refund delivery %s -> ingest -> writer.Refunded\n", refundEventID)
	refundResult, err := service.Handle(ctx, refundEventID)
	if err != nil {
		return fmt.Errorf("ingest refund: %w", err)
	}
	fmt.Printf("   ingest: %d evidence row(s), duplicate=%v, pending question=%v\n",
		len(refundResult.Rows), refundResult.Duplicate, refundResult.Pending != nil)
	if len(refundResult.Rows) != 1 || refundResult.Pending == nil {
		return fmt.Errorf("refund delivered %d rows and pending=%v, want one row and a pending question",
			len(refundResult.Rows), refundResult.Pending != nil)
	}
	refunded, err := p.snapshot(ctx, txnHash)
	if err != nil {
		return err
	}
	fmt.Printf("   doc: %s\n", refunded)
	if err := wantState(refunded, domain.FitStateRefunded, false, rowVersion(domain.FitStateRefunded, refundAt), "after refund"); err != nil {
		return err
	}
	if refunded.Doc.CapturedAt != captureAt.Format(time.RFC3339) || refunded.Doc.RefundedAt != refundAt.Format(time.RFC3339) {
		return fmt.Errorf("after refund: captured_at=%q refunded_at=%q, want both recorded times",
			refunded.Doc.CapturedAt, refunded.Doc.RefundedAt)
	}
	if refunded.Doc.SettledAt != "" || refunded.Doc.ReasonCategory != "" || refunded.Doc.FitDirection != "" {
		return fmt.Errorf("after refund: settled_at=%q reason=%q direction=%q, want a refund stored and unsettled",
			refunded.Doc.SettledAt, refunded.Doc.ReasonCategory, refunded.Doc.FitDirection)
	}

	fmt.Printf("\n== 3. replay the refund delivery (before the reason): ingest returns the stored rows\n")
	replayResult, err := service.Handle(ctx, refundEventID)
	if err != nil {
		return fmt.Errorf("replay refund: %w", err)
	}
	fmt.Printf("   ingest: %d evidence row(s), duplicate=%v (nothing written)\n",
		len(replayResult.Rows), replayResult.Duplicate)
	if !replayResult.Duplicate {
		return fmt.Errorf("replayed refund was not a duplicate")
	}
	afterReplay, err := p.snapshot(ctx, txnHash)
	if err != nil {
		return err
	}
	fmt.Printf("   doc: %s\n", afterReplay)
	if err := wantSameRow(refunded, afterReplay, "replayed refund delivery"); err != nil {
		return err
	}

	fmt.Printf("\n== 4. shopper's answer -> recorded classifier -> writer.Classified\n")
	answer, category, confidence, provenance, err := recordedAnswer(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("   answer %q -> category %s (confidence %.2f, %s)\n", answer, category, confidence, provenance)
	purchase := fitindex.Purchase{
		TxnID: captureID,
		Line:  seed.DemoOxfordLine(),
		Size:  seed.DemoOxfordSize,
	}
	if err := writer.Classified(ctx, purchase, category, classifyAt); err != nil {
		return fmt.Errorf("classify the purchase: %w", err)
	}
	classified, err := p.snapshot(ctx, txnHash)
	if err != nil {
		return err
	}
	fmt.Printf("   doc: %s\n", classified)
	if err := wantState(classified, domain.FitStateReasonClassified, true, rowVersion(domain.FitStateReasonClassified, classifyAt), "after the reason"); err != nil {
		return err
	}
	if classified.Doc.SettledKind != string(domain.FitSettledByReturnReason) ||
		classified.Doc.ReasonCategory != string(category) ||
		classified.Doc.FitDirection != "too_small" {
		return fmt.Errorf("after the reason: kind=%q reason=%q direction=%q, want return_reason/%s/too_small",
			classified.Doc.SettledKind, classified.Doc.ReasonCategory, classified.Doc.FitDirection, category)
	}
	if classified.Doc.SettledAt != classifyAt.Format(time.RFC3339) {
		return fmt.Errorf("after the reason: settled_at=%q, want %q", classified.Doc.SettledAt, classifyAt.Format(time.RFC3339))
	}

	fmt.Printf("\n== 5. replay the refund delivery (now behind the reason): the newer state stands\n")
	replayAfter, err := service.Handle(ctx, refundEventID)
	if err != nil {
		return fmt.Errorf("replay refund after the reason: %w", err)
	}
	fmt.Printf("   ingest: duplicate=%v (nothing written)\n", replayAfter.Duplicate)
	if !replayAfter.Duplicate {
		return fmt.Errorf("replayed refund after the reason was not a duplicate")
	}
	if err := writer.Record(ctx, refundResult.Rows[0]); err != nil {
		return fmt.Errorf("retry the refund's fit write: %w", err)
	}
	afterFitReplay, err := p.snapshot(ctx, txnHash)
	if err != nil {
		return err
	}
	fmt.Printf("   fit write retried with the refund event: doc: %s\n", afterFitReplay)
	if err := wantSameRow(classified, afterFitReplay, "retried refund fit write"); err != nil {
		return err
	}

	fmt.Printf("\n== 6. retry the fit write with the capture event (an older state, same writer)\n")
	if err := writer.Record(ctx, captureResult.Rows[0]); err != nil {
		return fmt.Errorf("retry the capture's fit write: %w", err)
	}
	afterCaptureFitReplay, err := p.snapshot(ctx, txnHash)
	if err != nil {
		return err
	}
	fmt.Printf("   doc: %s\n", afterCaptureFitReplay)
	if err := wantSameRow(classified, afterCaptureFitReplay, "retried capture fit write"); err != nil {
		return err
	}

	fmt.Printf("\n== 7. a fresh writer (a restarted process) hands the index the captured event\n")
	restarted, err := fitindex.NewWriter(index, keyer, payerID, fitindex.LinesFunc(demoLines))
	if err != nil {
		return fmt.Errorf("build the restarted writer: %w", err)
	}
	if err := restarted.Record(ctx, captureResult.Rows[0]); err != nil {
		return fmt.Errorf("restarted writer records the captured event: %w", err)
	}
	afterRestart, err := p.snapshot(ctx, txnHash)
	if err != nil {
		return err
	}
	fmt.Printf("   writer returned nil (an older version is done, not an error); doc: %s\n", afterRestart)
	if err := wantSameRow(classified, afterRestart, "restarted writer's captured event"); err != nil {
		return err
	}

	fmt.Printf("\n== 8. the index refuses a write at the captured rank directly\n")
	olderPath := fmt.Sprintf("/%s/_doc/%s?version=%d&version_type=external_gte&refresh=true",
		indexName, txnHash, rowVersion(domain.FitStateCaptured, captureAt))
	status, body, err := p.do(ctx, http.MethodPut, olderPath, classified.Source)
	if err != nil {
		return fmt.Errorf("write an older version: %w", err)
	}
	fmt.Printf("   PUT at the captured rank -> %d %s\n", status, conflictReason(body))
	if status != http.StatusConflict {
		return fmt.Errorf("an older version answered %d, want 409", status)
	}
	afterOlder, err := p.snapshot(ctx, txnHash)
	if err != nil {
		return err
	}
	if err := wantSameRow(classified, afterOlder, "refused older write"); err != nil {
		return err
	}

	fmt.Printf("\n== 9. one document per purchase\n")
	forPurchase, err := p.count(ctx, "txn_hash", txnHash)
	if err != nil {
		return err
	}
	forShopper, err := p.count(ctx, "shopper_hash", shopperHash)
	if err != nil {
		return err
	}
	total, err := p.count(ctx, "", "")
	if err != nil {
		return err
	}
	fmt.Printf("   docs for the purchase: %d, docs for the shopper line: %d, docs in %s: %d\n",
		forPurchase, forShopper, indexName, total)
	if forPurchase != 1 || forShopper != 1 || total != 1 {
		return fmt.Errorf("counts: purchase=%d shopper=%d total=%d, want one row per purchase and one document",
			forPurchase, forShopper, total)
	}

	fmt.Printf("\n== states and versions\n")
	for _, step := range []struct {
		name  string
		state domain.FitPurchaseState
		at    time.Time
	}{
		{"captured", domain.FitStateCaptured, captureAt},
		{"refunded", domain.FitStateRefunded, refundAt},
		{"reason_classified", domain.FitStateReasonClassified, classifyAt},
	} {
		fmt.Printf("   %-18s rank %d, version %d (%s)\n",
			step.name, step.state.Rank(), rowVersion(step.state, step.at), step.at.Format(time.RFC3339))
	}

	fmt.Println("\nmodel spend: $0 (no model call: the answer and its category are recorded)")
	fmt.Printf("elastic: one kilobyte-scale index, 3 state writes plus 3 replayed writes, reads and counts; deleted below\n")
	return nil
}

// purchaseInputs builds the recorded purchase's payment leg: the capture
// and refund deliveries, the order read with the line item, and the refund
// read. Shapes and values come from the T0.4 recordings; the ids are the
// recordings' placeholders.
func purchaseInputs() fakes.StoryInputs {
	item := domain.Item{
		BrandID:   seed.DemoOxfordBrand,
		BrandName: "Brand X",
		LineName:  "oxford shirt",
		SKU:       "BRX-OXF-M",
		Size:      seed.DemoOxfordSize,
		Category:  seed.DemoOxfordCategory,
	}
	link := func(event string) domain.TransactionLink {
		return domain.TransactionLink{
			OrderID:   orderID,
			CaptureID: captureID,
			RefundID:  refundID,
			CustomID:  payerID,
			EventID:   event,
		}
	}
	captured := domain.Outcome{Kind: domain.OutcomeCapture, Status: "COMPLETED", AmountMinor: 3400, Currency: "USD"}
	refunded := domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 3400, Currency: "USD"}
	captureEvent := fakes.WebhookEvent{
		EventID: captureEventID, EventType: "PAYMENT.CAPTURE.COMPLETED", Verified: true,
		Outcome: captured, Link: link(captureEventID),
		Provenance: captureRecordFixture,
	}
	refundEvent := fakes.WebhookEvent{
		EventID: refundEventID, EventType: "PAYMENT.CAPTURE.REFUNDED", Verified: true,
		Outcome: refunded, Link: link(refundEventID),
		Provenance: refundRecordFixture,
	}
	order := fakes.OrderRecord{
		OrderID: orderID, Items: []domain.Item{item}, Link: link(""),
		Provenance: orderFixture,
	}
	refund := fakes.RefundRecord{
		RefundID: refundID, Outcome: refunded, Link: link(""), HasReason: false,
		Provenance: refundRecordFixture,
	}
	return fakes.StoryInputs{
		Events:  []fakes.WebhookEvent{captureEvent, refundEvent},
		Orders:  []fakes.OrderRecord{order},
		Refunds: []fakes.RefundRecord{refund},
	}
}

// eventClock gives each recorded delivery its recorded event time.
func eventClock() func(eventID string) time.Time {
	return func(eventID string) time.Time {
		switch eventID {
		case captureEventID:
			return captureAt
		case refundEventID:
			return refundAt
		default:
			return time.Time{}
		}
	}
}

// demoLines resolves the demo brand's oxford to its product line. Only the
// catalog knows a line's fit handle; a production resolver belongs in
// agent/catalog (T8.5 wires it), and this driver uses a function.
func demoLines(_ context.Context, item domain.Item) (domain.ProductLine, error) {
	if item.BrandID != seed.DemoOxfordBrand || item.Category != seed.DemoOxfordCategory {
		return domain.ProductLine{}, fmt.Errorf("driver: no catalog line for item %q (%s/%s)",
			item.SKU, item.BrandID, item.Category)
	}
	return seed.DemoOxfordLine(), nil
}

// recordedAnswer replays the recorded shopper answer through the recorded
// classifier: the answer is read from the T0.7 fixture, so the category and
// its provenance are the recording's, and no model is called.
func recordedAnswer(ctx context.Context) (answer string, category domain.Category, confidence float64, provenance string, err error) {
	raw, err := os.ReadFile(answerFixture)
	if err != nil {
		return "", "", 0, "", fmt.Errorf("read the recorded answer: %w", err)
	}
	var fixture struct {
		Request struct {
			State struct {
				Answer string `json:"answer"`
			} `json:"state"`
		} `json:"request"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		return "", "", 0, "", fmt.Errorf("decode the recorded answer: %w", err)
	}
	answer = fixture.Request.State.Answer
	if answer == "" {
		return "", "", 0, "", fmt.Errorf("the recorded answer fixture carries no answer")
	}
	classifier, err := fakes.NewRecordingReasonClassifier(classifierDir)
	if err != nil {
		return "", "", 0, "", fmt.Errorf("build the recorded classifier: %w", err)
	}
	class, err := classifier.Classify(ctx, answer)
	if err != nil {
		return "", "", 0, "", fmt.Errorf("classify the recorded answer: %w", err)
	}
	return answer, class.Category, class.Confidence, class.Provenance, nil
}

// rowVersion is the external version a state writes at: the state's rank
// with the event time inside it.
func rowVersion(state domain.FitPurchaseState, at time.Time) int64 {
	return domain.FitRow{State: state, EventAt: at}.Version()
}

// wantState checks the row's state, settlement and version after a step.
func wantState(got snapshot, state domain.FitPurchaseState, settled bool, version int64, what string) error {
	var problems []string
	if got.Doc.PurchaseState != string(state) {
		problems = append(problems, fmt.Sprintf("state = %q, want %q", got.Doc.PurchaseState, state))
	}
	if got.Version != version {
		problems = append(problems, fmt.Sprintf("version = %d, want %d", got.Version, version))
	}
	if got.Doc.Settled != settled {
		problems = append(problems, fmt.Sprintf("settled = %t, want %t", got.Doc.Settled, settled))
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s: %s", what, strings.Join(problems, "; "))
	}
	return nil
}

// wantSameRow checks that a write left the row exactly as it was: same
// version, same stored source.
func wantSameRow(before, after snapshot, what string) error {
	if after.Version != before.Version {
		return fmt.Errorf("%s: version = %d, want the unchanged %d", what, after.Version, before.Version)
	}
	if !bytes.Equal(before.Source, after.Source) {
		return fmt.Errorf("%s: stored source changed;\n before: %s\n  after: %s", what, before.Source, after.Source)
	}
	return nil
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

// conflictReason names the conflict when an old write is refused, so the
// transcript shows why the status was 409.
func conflictReason(body []byte) string {
	var answer struct {
		Error struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &answer) == nil && answer.Error.Type != "" {
		return answer.Error.Type + ": " + answer.Error.Reason
	}
	return excerpt(body)
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

// orDash renders an absent value as a dash, so a state line shows what the
// row does not carry.
func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// short keeps a hash readable in a transcript line.
func short(hash string) string {
	if len(hash) <= 16 {
		return hash
	}
	return hash[:16] + "…"
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
		fmt.Fprintf(os.Stderr, "FAIL T8.2b live fit index: %s\n",
			redact(err.Error(), os.Getenv("ELASTIC_URL"), os.Getenv("LAMBO_ES_API_KEY"), os.Getenv("MEMENTO_IDENTITY_SALT")))
		os.Exit(1)
	}
	fmt.Println("\nPASS T8.2b live fit index: one purchase, one row, every state, replayed writes harmless, index deleted")
}
