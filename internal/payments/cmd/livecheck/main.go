// Command livecheck records four webhook deliveries against the sandbox
// verify API and runs each one through the ingest gate.
//
// It reads one delivery PayPal already posted to the sandbox test endpoint
// (the bytes and transmission headers are stored there), verifies it live,
// and then records the four cases the task calls for: the good delivery,
// the same body tampered with after signing, the good delivery replayed,
// and a delivery PayPal never issued. Each case goes through
// ingest.Service with no verifier injected, so the real adapter is the gate;
// the reads behind a verified delivery are the recorded demo story keyed to
// the live event id, because live transaction reads are T2.1b's subject.
//
// Nothing is created: the run reads one stored delivery and calls the verify
// endpoint a handful of times. Sandbox calls are free.
//
// Real ids stay in the git-ignored out/ directory. stdout carries the
// redacted transcript, and the driver refuses to finish if a credential or a
// real id survives redaction.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/ingest"
	"github.com/nrynss/memento-emptor/internal/payments"
)

// sandbox is the PayPal sandbox REST root. Sandbox only.
const sandbox = "https://api-m.sandbox.paypal.com"

// siteBase is the webhook test endpoint that stores deliveries.
const siteBase = "https://webhook.site"

// outDir holds the unredacted transcript. Git-ignored.
const outDir = "internal/payments/cmd/livecheck/out"

// storedDelivery is one delivery read back from the test endpoint: the
// exact bytes PayPal posted plus the headers that sign them.
type storedDelivery struct {
	eventID string
	raw     []byte
	header  http.Header
}

// recordedDeliveryFile is the on-disk recording of one delivery: its exact
// body bytes and the headers that sign them.
type recordedDeliveryFile struct {
	Headers map[string][]string `json:"headers"`
	Body    json.RawMessage     `json:"body"`
}

// writeRecording saves one delivery under out/ with the body byte for byte,
// so the recorded copy can be re-verified later.
func writeRecording(dir string, d Delivery) error {
	header, err := json.Marshal(d.header)
	if err != nil {
		return fmt.Errorf("encode headers: %w", err)
	}
	var raw []byte
	raw = append(raw, `{"headers":`...)
	raw = append(raw, header...)
	raw = append(raw, `,"body":`...)
	raw = append(raw, d.raw...)
	raw = append(raw, '}')
	return os.WriteFile(filepath.Join(dir, "delivery.json"), append(raw, '\n'), 0o600)
}

// readRecording loads the delivery saved under out/.
func readRecording(dir string) (Delivery, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "delivery.json"))
	if err != nil {
		return Delivery{}, fmt.Errorf("read recording: %w", err)
	}
	var file recordedDeliveryFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return Delivery{}, fmt.Errorf("parse recording: %w", err)
	}
	header := http.Header{}
	for name, values := range file.Headers {
		for _, value := range values {
			header.Add(name, value)
		}
	}
	var probe struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(file.Body, &probe); err != nil || probe.ID == "" {
		return Delivery{}, fmt.Errorf("recorded body names no event")
	}
	return Delivery{eventID: probe.ID, raw: append([]byte(nil), file.Body...), header: header}, nil
}

// fail prints to stderr and exits nonzero.
func fail(err error) {
	fmt.Fprintf(os.Stderr, "livecheck: %v\n", err)
	os.Exit(1)
}

// progress reports one step on stderr without any value.
func progress(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[livecheck] "+format+"\n", args...)
}

// grant mints a sandbox access token from the environment.
func grant(ctx context.Context, hc *http.Client) (string, error) {
	id, secret := os.Getenv("PAYPAL_CLIENT_ID"), os.Getenv("PAYPAL_CLIENT_SECRET")
	if id == "" || secret == "" {
		return "", fmt.Errorf("PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET must come from tools/env.sh")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sandbox+payments.TokenPath, strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.SetBasicAuth(id, secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("token grant: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read token grant: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token grant refused with status %d: stop and report, do not retry", resp.StatusCode)
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed.AccessToken == "" {
		return "", fmt.Errorf("token grant carried no token")
	}
	return parsed.AccessToken, nil
}

// testEndpoint returns the app's webhook subscription that points at the
// delivery store, with the store's token parsed out of the URL.
func testEndpoint(ctx context.Context, hc *http.Client, token string) (webhookID, siteToken string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sandbox+payments.WebhooksPath, nil)
	if err != nil {
		return "", "", fmt.Errorf("build webhook list: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := hc.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("list webhooks: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", fmt.Errorf("read webhook list: %w", err)
	}
	var listed struct {
		Webhooks []struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		} `json:"webhooks"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return "", "", fmt.Errorf("parse webhook list: %w", err)
	}
	for _, hook := range listed.Webhooks {
		if strings.HasPrefix(hook.URL, siteBase+"/") && hook.ID != "" {
			return hook.ID, strings.TrimPrefix(hook.URL, siteBase+"/"), nil
		}
	}
	return "", "", fmt.Errorf("no webhook subscription points at the delivery store")
}

// readDeliveries fetches the stored deliveries the test endpoint holds.
func readDeliveries(ctx context.Context, hc *http.Client, siteToken string) ([]storedDelivery, error) {
	url := siteBase + "/token/" + siteToken + "/requests?sorting=newest&per_page=50"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build delivery read: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("read deliveries: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read delivery list: %w", err)
	}
	var listed struct {
		Data []struct {
			Content string              `json:"content"`
			Headers map[string][]string `json:"headers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		return nil, fmt.Errorf("parse delivery list: %w", err)
	}
	var out []storedDelivery
	for _, entry := range listed.Data {
		var probe struct {
			ID        string `json:"id"`
			EventType string `json:"event_type"`
		}
		if err := json.Unmarshal([]byte(entry.Content), &probe); err != nil || probe.ID == "" {
			continue
		}
		header := http.Header{}
		for name, values := range entry.Headers {
			for _, value := range values {
				header.Add(name, value)
			}
		}
		out = append(out, storedDelivery{eventID: probe.ID, raw: []byte(entry.Content), header: header})
	}
	return out, nil
}

// pickRefund returns the newest stored refund delivery.
func pickRefund(deliveries []storedDelivery) (storedDelivery, error) {
	for _, d := range deliveries {
		var probe struct {
			EventType string `json:"event_type"`
		}
		if err := json.Unmarshal(d.raw, &probe); err != nil {
			continue
		}
		if probe.EventType == "PAYMENT.CAPTURE.REFUNDED" {
			return d, nil
		}
	}
	return storedDelivery{}, fmt.Errorf("the delivery store holds no refund event")
}

// tampered returns the delivery with its refunded amount changed after
// signing, as an attacker would.
func tampered(d Delivery) ([]byte, error) {
	var body map[string]any
	if err := json.Unmarshal(d.raw, &body); err != nil {
		return nil, fmt.Errorf("parse body: %w", err)
	}
	resource, _ := body["resource"].(map[string]any)
	amount, _ := resource["amount"].(map[string]any)
	if amount == nil {
		return nil, fmt.Errorf("delivery carries no refund amount")
	}
	amount["value"] = "1.00"
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode tampered body: %w", err)
	}
	return encoded, nil
}

// unissued returns the good delivery with a transmission id and signature
// from no delivery PayPal ever sent.
func unissued(d Delivery) Delivery {
	header := d.header.Clone()
	header.Set("Paypal-Transmission-Id", "00000000-0000-0000-0000-000000000000")
	header.Set("Paypal-Transmission-Sig", strings.Repeat("A", 344))
	return Delivery{eventID: d.eventID, raw: append([]byte(nil), d.raw...), header: header}
}

// Delivery is one case's delivery: the event id its body names, the exact
// body bytes, and the headers that sign them.
type Delivery struct {
	eventID string
	raw     []byte
	header  http.Header
}

// asDelivery converts a stored delivery to a gate delivery.
func (d Delivery) asDelivery() payments.Delivery {
	return payments.Delivery{Header: d.header.Clone(), Body: append([]byte(nil), d.raw...)}
}

// readSource builds the read leg behind the live event: the recorded demo
// refund story keyed to the live event id. The gate and idempotency are
// this run's subject; live transaction reads are T2.1b's.
func readSource(eventID string) fakes.PaymentSource {
	inputs := fakes.DemoStoryInputs()
	link := inputs.Events[0].Link
	link.EventID = eventID
	event := inputs.Events[0]
	event.EventID = eventID
	event.Link = link
	order := inputs.Orders[0]
	order.Link = link
	refund := inputs.Refunds[0]
	refund.Link = link
	return fakes.NewRecordingPaymentSource(fakes.StoryInputs{
		Events:  []fakes.WebhookEvent{event},
		Orders:  []fakes.OrderRecord{order},
		Refunds: []fakes.RefundRecord{refund},
	})
}

// redactor replaces real ids with placeholders on stdout.
type redactor struct {
	mapping map[string]string
	email   *regexp.Regexp
}

// add records one replacement.
func (r *redactor) add(real, placeholder string) {
	if real != "" {
		r.mapping[real] = placeholder
	}
}

// apply replaces every known real value, longest first, then emails.
func (r *redactor) apply(text string) string {
	ordered := make([]string, 0, len(r.mapping))
	for real := range r.mapping {
		ordered = append(ordered, real)
	}
	sort.Slice(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	for _, real := range ordered {
		text = strings.ReplaceAll(text, real, r.mapping[real])
	}
	return r.email.ReplaceAllString(text, "EMAIL-REDACTED")
}

// check refuses to finish if a real value survived redaction.
func (r *redactor) check(text string) error {
	for real, placeholder := range r.mapping {
		if strings.Contains(text, real) {
			return fmt.Errorf("redaction missed %s", placeholder)
		}
	}
	return nil
}

// transcript collects redacted stdout lines.
type transcript struct {
	lines []string
	r     *redactor
}

// say records one line, redacted.
func (t *transcript) say(format string, args ...any) {
	t.lines = append(t.lines, t.r.apply(fmt.Sprintf(format, args...)))
}

// text renders the transcript and checks every real value is gone.
func (t *transcript) text() (string, error) {
	out := strings.Join(t.lines, "\n") + "\n"
	if err := t.r.check(out); err != nil {
		return "", err
	}
	return out, nil
}

// caseResult records one case's verification and ingest outcome.
type caseResult struct {
	Name      string `json:"name"`
	Verdict   string `json:"verdict"`
	Ingest    string `json:"ingest"`
	Rows      int    `json:"rows"`
	Duplicate bool   `json:"duplicate"`
	Refusal   string `json:"refusal,omitempty"`
}

// main runs the recording.
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	hc := &http.Client{Timeout: 30 * time.Second}

	token, err := grant(ctx, hc)
	if err != nil {
		fail(err)
	}
	progress("access token granted")
	webhookID, siteToken, err := testEndpoint(ctx, hc, token)
	if err != nil {
		fail(err)
	}
	progress("webhook subscription found")
	deliveries, err := readDeliveries(ctx, hc, siteToken)
	if err != nil {
		fail(err)
	}
	chosen, err := pickRefund(deliveries)
	if err != nil {
		fail(err)
	}
	progress("refund delivery read back from the test endpoint")

	good := Delivery{eventID: chosen.eventID, raw: chosen.raw, header: chosen.header}
	tamperedBody, err := tampered(good)
	if err != nil {
		fail(err)
	}
	unknown := unissued(good)

	// The verifier is the real adapter, built from the environment exactly
	// as production builds it; no fake is injected.
	verifier := payments.New()

	// Record the delivery and verify the recorded copy live, so the
	// evidence is that the saved bytes verify, not only the fetched ones.
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		fail(fmt.Errorf("make out dir: %w", err))
	}
	if err := writeRecording(outDir, good); err != nil {
		fail(err)
	}
	recorded, err := readRecording(outDir)
	if err != nil {
		fail(err)
	}
	if err := verifier.Verify(ctx, recorded.asDelivery()); err != nil {
		fail(fmt.Errorf("the recorded delivery did not verify live: %w", err))
	}
	service := ingest.NewService(readSource(good.eventID))

	results := make([]caseResult, 0, 4)
	verifyCase := func(name string, d Delivery) string {
		if err := verifier.Verify(ctx, d.asDelivery()); err != nil {
			if !errors.Is(err, payments.ErrNotVerified) {
				fail(fmt.Errorf("%s: verification did not answer: %w", name, err))
			}
			return "FAILURE"
		}
		return "SUCCESS"
	}

	// 1. The good delivery.
	goodVerdict := verifyCase("good", good)
	if goodVerdict != "SUCCESS" {
		fail(fmt.Errorf("the stored delivery did not verify"))
	}
	first, err := service.HandleDelivery(ctx, good.asDelivery())
	if err != nil {
		fail(fmt.Errorf("good delivery refused: %w", err))
	}
	if len(first.Rows) == 0 || first.Pending == nil {
		fail(fmt.Errorf("good delivery stored no rows or filed no pending question"))
	}
	rows := len(service.Rows())
	results = append(results, caseResult{Name: "good", Verdict: goodVerdict, Ingest: "rows", Rows: len(first.Rows)})

	// 2. The same delivery with its body tampered with after signing.
	tamperedDelivery := Delivery{eventID: good.eventID, raw: tamperedBody, header: good.header.Clone()}
	tamperedVerdict := verifyCase("tampered", tamperedDelivery)
	if tamperedVerdict != "FAILURE" {
		fail(fmt.Errorf("a tampered body verified"))
	}
	if _, err := service.HandleDelivery(ctx, tamperedDelivery.asDelivery()); !errors.Is(err, ingest.ErrBadSignature) {
		fail(fmt.Errorf("tampered delivery = %v, want ErrBadSignature", err))
	} else {
		results = append(results, caseResult{Name: "tampered", Verdict: tamperedVerdict, Ingest: "refused", Refusal: err.Error()})
	}
	if len(service.Rows()) != rows {
		fail(fmt.Errorf("tampered delivery changed the store"))
	}

	// 3. The good delivery replayed.
	replayVerdict := verifyCase("replayed", good)
	if replayVerdict != "SUCCESS" {
		fail(fmt.Errorf("the replayed delivery did not verify"))
	}
	replay, err := service.HandleDelivery(ctx, good.asDelivery())
	if err != nil {
		fail(fmt.Errorf("replayed delivery refused: %w", err))
	}
	if !replay.Duplicate || len(service.Rows()) != rows {
		fail(fmt.Errorf("replayed delivery was not idempotent"))
	}
	results = append(results, caseResult{Name: "replayed", Verdict: replayVerdict, Ingest: "duplicate", Rows: len(replay.Rows), Duplicate: true})

	// 4. A delivery PayPal never issued.
	unknownVerdict := verifyCase("unknown", unknown)
	if unknownVerdict != "FAILURE" {
		fail(fmt.Errorf("a delivery PayPal never issued verified"))
	}
	unknownErr := func() error {
		_, err := service.HandleDelivery(ctx, unknown.asDelivery())
		if !errors.Is(err, ingest.ErrBadSignature) {
			fail(fmt.Errorf("unknown delivery = %v, want ErrBadSignature", err))
		}
		return err
	}()
	results = append(results, caseResult{Name: "unknown", Verdict: unknownVerdict, Ingest: "refused", Refusal: unknownErr.Error()})
	if len(service.Rows()) != rows {
		fail(fmt.Errorf("unknown delivery changed the store"))
	}
	if results[1].Refusal != results[3].Refusal {
		fail(fmt.Errorf("a tampered body and an unissued delivery get different refusals: %q vs %q", results[1].Refusal, results[3].Refusal))
	}

	redact := &redactor{mapping: map[string]string{}, email: regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.]+`)}
	redact.add(webhookID, "WEBHOOK-ID-LIVE-1")
	redact.add(siteToken, "WEBHOOK-SITE-TOKEN-REDACTED")
	redact.add(good.eventID, "WH-EVENT-LIVE-1")
	redact.add(good.header.Get("Paypal-Transmission-Id"), "TRANSMISSION-ID-LIVE")
	redact.add(good.header.Get("Paypal-Transmission-Sig"), "TRANSMISSION-SIG-LIVE")
	redact.add(good.header.Get("Paypal-Cert-Url"), "CERT-URL-LIVE")
	redact.add(os.Getenv("PAYPAL_CLIENT_ID"), "CLIENT-ID-REDACTED")
	redact.add(os.Getenv("PAYPAL_CLIENT_SECRET"), "CLIENT-SECRET-REDACTED")
	redact.add(os.Getenv("PAYPAL_MERCHANT_EMAIL"), "EMAIL-REDACTED")

	t := &transcript{r: redact}
	t.say("T8.7 verified webhook deliveries (sandbox; placeholders; real values in out/transcript.json)")
	t.say("recorded: one PAYMENT.CAPTURE.REFUNDED delivery WH-EVENT-LIVE-1 and its transmission headers")
	t.say("gate: ingest.Service with the real adapter, no verifier injected; read leg: recorded demo refund keyed to the live event id")
	t.say("recorded delivery re-verified live from out/delivery.json: SUCCESS")
	for _, result := range results {
		switch result.Ingest {
		case "rows":
			t.say("%s: verify %s; ingest stored %d row(s), pending question filed", result.Name, result.Verdict, result.Rows)
		case "duplicate":
			t.say("%s: verify %s; ingest returned the %d stored row(s) with duplicate set", result.Name, result.Verdict, result.Rows)
		default:
			t.say("%s: verify %s; ingest refused: %s", result.Name, result.Verdict, result.Refusal)
		}
	}
	t.say("tampered and unknown refusals are identical: %v", results[1].Refusal == results[3].Refusal)
	t.say("verification is live: each verdict above is the sandbox verify-webhook-signature endpoint's answer")
	t.say("LIVE OK: four deliveries recorded, nothing created, no model spend")

	text, err := t.text()
	if err != nil {
		fail(err)
	}
	fmt.Print(text)

	if err := os.MkdirAll(outDir, 0o700); err != nil {
		fail(fmt.Errorf("make out dir: %w", err))
	}
	full := map[string]any{
		"webhook_id":        webhookID,
		"event_id":          good.eventID,
		"recorded_verified": true,
		"deliveries":        results,
		"body_bytes":        len(good.raw),
		"headers":           good.header,
		"rows_stored":       len(service.Rows()),
	}
	encoded, err := json.MarshalIndent(full, "", "  ")
	if err != nil {
		fail(fmt.Errorf("encode transcript: %w", err))
	}
	if err := os.WriteFile(filepath.Join(outDir, "transcript.json"), append(encoded, '\n'), 0o600); err != nil {
		fail(fmt.Errorf("write transcript: %w", err))
	}
	progress("full transcript in %s/transcript.json", outDir)
}
