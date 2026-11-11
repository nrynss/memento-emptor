// Command sourcecheck reads the sandbox's real capture, refund and dispute
// records through the production payment source and shows each one mapped
// into the layer's event shapes.
//
// It is read-only: every PayPal call is a GET (plus the OAuth token grant),
// nothing is created or refunded, no webhook is sent, and no model is
// called. The in-process ingest drive writes only to memory.
//
// Real ids stay in the git-ignored out/ directory. stdout carries the
// redacted transcript, and the driver refuses to finish if a credential or
// a real id survives redaction.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/ingest"
	"github.com/nrynss/memento-emptor/internal/payments"
)

// sandbox is the PayPal sandbox REST root. Sandbox only.
const sandbox = "https://api-m.sandbox.paypal.com"

// outDir holds the unredacted transcript. Git-ignored.
const outDir = "internal/payments/cmd/sourcecheck/out"

// wanted names the event types the live proof needs, newest first.
var wanted = []string{
	payments.EventCaptureCompleted,
	payments.EventCaptureRefunded,
	payments.EventDisputeCreated,
}

// ledgerEvent is one entry of the webhook events ledger.
type ledgerEvent struct {
	ID        string
	EventType string
	Created   time.Time
}

// fail prints to stderr and exits nonzero.
func fail(err error) {
	fmt.Fprintf(os.Stderr, "sourcecheck: %v\n", err)
	os.Exit(1)
}

// progress reports one step on stderr without any value.
func progress(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[sourcecheck] "+format+"\n", args...)
}

// grant mints a sandbox access token from the environment.
func grant(ctx context.Context, hc *http.Client) (string, error) {
	id, secret := os.Getenv("PAYPAL_CLIENT_ID"), os.Getenv("PAYPAL_CLIENT_SECRET")
	if id == "" || secret == "" {
		return "", fmt.Errorf("PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET must come from tools/env.sh")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sandbox+payments.TokenPath,
		strings.NewReader("grant_type=client_credentials"))
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

// listEvents reads the webhook events ledger, newest first.
func listEvents(ctx context.Context, hc *http.Client, token string) ([]ledgerEvent, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		sandbox+"/v1/notifications/webhooks-events?page_size=20", nil)
	if err != nil {
		return nil, fmt.Errorf("build event list: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("read event list: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read event list: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("event list answered %d", resp.StatusCode)
	}
	var listed struct {
		Events []struct {
			ID         string `json:"id"`
			EventType  string `json:"event_type"`
			CreateTime string `json:"create_time"`
		} `json:"events"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return nil, fmt.Errorf("parse event list: %w", err)
	}
	var out []ledgerEvent
	for _, entry := range listed.Events {
		moment, err := time.Parse(time.RFC3339, entry.CreateTime)
		if err != nil {
			return nil, fmt.Errorf("parse event time %q: %w", entry.CreateTime, err)
		}
		if entry.ID == "" || entry.EventType == "" {
			continue
		}
		out = append(out, ledgerEvent{ID: entry.ID, EventType: entry.EventType, Created: moment})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}

// pick returns the newest event of one type.
func pick(events []ledgerEvent, eventType string) (ledgerEvent, error) {
	for _, event := range events {
		if event.EventType == eventType {
			return event, nil
		}
	}
	return ledgerEvent{}, fmt.Errorf("the sandbox ledger holds no %s event", eventType)
}

// countingFit records the fit rows the ingest drive would write, without an
// index. It proves the capture and refund legs reach the second write path;
// it is not the live index.
type countingFit struct {
	rows int
}

// Record counts one fit row.
func (f *countingFit) Record(ctx context.Context, ev domain.Evidence) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.rows++
	return nil
}

// redactor replaces real ids with placeholders on stdout.
type redactor struct {
	mapping map[string]string
	email   *regexp.Regexp
}

// add records one replacement. The first placeholder for a value wins, so
// one real id reads the same way on every line even when several records
// carry it.
func (r *redactor) add(real, placeholder string) {
	if strings.TrimSpace(real) == "" {
		return
	}
	if _, ok := r.mapping[real]; ok {
		return
	}
	r.mapping[real] = placeholder
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

// itemLine renders one item without its identity fields.
func itemLine(item domain.Item) string {
	return fmt.Sprintf("brand=%s name=%s line=%s sku=%s size=%s category=%s",
		item.BrandID, item.BrandName, item.LineName, item.SKU, item.Size, item.Category)
}

// linkLine renders one transaction link.
func linkLine(link domain.TransactionLink) string {
	return fmt.Sprintf("order=%s capture=%s refund=%s dispute=%s custom=%s",
		link.OrderID, link.CaptureID, link.RefundID, link.DisputeID, link.CustomID)
}

// outcomeLine renders one outcome.
func outcomeLine(outcome domain.Outcome) string {
	return fmt.Sprintf("%s/%s/%d/%s", outcome.Kind, outcome.Status, outcome.AmountMinor, outcome.Currency)
}

// resultLine renders one ingest result.
func resultLine(result ingest.Result, fit int) string {
	pending := result.Pending != nil
	return fmt.Sprintf("rows=%d pending=%v duplicate=%v fit_rows=%d",
		len(result.Rows), pending, result.Duplicate, fit)
}

// main runs the read-only proof.
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	hc := &http.Client{Timeout: 30 * time.Second}

	token, err := grant(ctx, hc)
	if err != nil {
		fail(err)
	}
	progress("access token granted")
	events, err := listEvents(ctx, hc, token)
	if err != nil {
		fail(err)
	}
	captureEvent, err := pick(events, payments.EventCaptureCompleted)
	if err != nil {
		fail(err)
	}
	refundEvent, err := pick(events, payments.EventCaptureRefunded)
	if err != nil {
		fail(err)
	}
	disputeEvent, err := pick(events, payments.EventDisputeCreated)
	if err != nil {
		fail(err)
	}
	progress("ledger holds a capture, a refund and a dispute event")

	source := payments.New(
		payments.WithCatalog(payments.DemoCatalog()),
		payments.WithHTTPClient(hc),
	)

	capture, err := source.VerifyWebhook(ctx, captureEvent.ID)
	if err != nil {
		fail(fmt.Errorf("capture event: %w", err))
	}
	captureOrder, err := source.GetOrder(ctx, capture.Link.OrderID)
	if err != nil {
		fail(fmt.Errorf("capture order: %w", err))
	}
	captureRead, err := source.GetCapture(ctx, capture.Link.CaptureID)
	if err != nil {
		fail(fmt.Errorf("capture read: %w", err))
	}

	refund, err := source.VerifyWebhook(ctx, refundEvent.ID)
	if err != nil {
		fail(fmt.Errorf("refund event: %w", err))
	}
	refundRead, err := source.GetRefund(ctx, refund.Link.RefundID)
	if err != nil {
		fail(fmt.Errorf("refund read: %w", err))
	}
	refundOrder, err := source.GetOrder(ctx, refund.Link.OrderID)
	if err != nil {
		fail(fmt.Errorf("refund order: %w", err))
	}

	dispute, err := source.VerifyWebhook(ctx, disputeEvent.ID)
	if err != nil {
		fail(fmt.Errorf("dispute event: %w", err))
	}
	disputeRead, err := source.GetDispute(ctx, dispute.Link.DisputeID)
	if err != nil {
		fail(fmt.Errorf("dispute read: %w", err))
	}
	disputeOrder, err := source.GetOrder(ctx, dispute.Link.OrderID)
	if err != nil {
		fail(fmt.Errorf("dispute order: %w", err))
	}

	fit := &countingFit{}
	service := ingest.NewService(source,
		ingest.WithFitRecorder(fit),
		ingest.WithClock(func(string) time.Time { return time.Now().UTC() }))
	captureFirst, err := service.Handle(ctx, capture.EventID)
	if err != nil {
		fail(fmt.Errorf("ingest capture: %w", err))
	}
	captureFit := fit.rows
	captureAgain, err := service.Handle(ctx, capture.EventID)
	if err != nil {
		fail(fmt.Errorf("ingest capture again: %w", err))
	}
	refundFirst, err := service.Handle(ctx, refund.EventID)
	if err != nil {
		fail(fmt.Errorf("ingest refund: %w", err))
	}
	refundFit := fit.rows
	refundAgain, err := service.Handle(ctx, refund.EventID)
	if err != nil {
		fail(fmt.Errorf("ingest refund again: %w", err))
	}
	disputeFirst, err := service.Handle(ctx, dispute.EventID)
	if err != nil {
		fail(fmt.Errorf("ingest dispute: %w", err))
	}
	if len(service.Rows()) != 3 {
		fail(fmt.Errorf("ingest stored %d rows, want 3", len(service.Rows())))
	}
	if len(service.PendingQuestions()) != 1 {
		fail(fmt.Errorf("ingest filed %d pending questions, want 1", len(service.PendingQuestions())))
	}
	progress("read and ingest drive complete")

	redact := &redactor{mapping: map[string]string{}, email: regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.]+`)}
	redact.add(captureEvent.ID, "WH-CAPTURE-LIVE")
	redact.add(refundEvent.ID, "WH-REFUND-LIVE")
	redact.add(disputeEvent.ID, "WH-DISPUTE-LIVE")
	redact.add(capture.Link.OrderID, "ORD-CAPTURE")
	redact.add(capture.Link.CaptureID, "CAP-CAPTURE")
	redact.add(capture.Link.CustomID, "CUSTOM-CAPTURE")
	redact.add(refund.Link.OrderID, "ORD-REFUND")
	redact.add(refund.Link.CaptureID, "CAP-REFUND")
	redact.add(refund.Link.RefundID, "RFD-REFUND")
	redact.add(refund.Link.CustomID, "CUSTOM-REFUND")
	redact.add(dispute.Link.OrderID, "ORD-DISPUTE")
	redact.add(dispute.Link.CaptureID, "CAP-DISPUTE")
	redact.add(dispute.Link.DisputeID, "DISPUTE-LIVE")
	redact.add(dispute.Link.CustomID, "CUSTOM-DISPUTE")
	redact.add(os.Getenv("PAYPAL_CLIENT_ID"), "CLIENT-ID-REDACTED")
	redact.add(os.Getenv("PAYPAL_CLIENT_SECRET"), "CLIENT-SECRET-REDACTED")

	t := &transcript{r: redact}
	t.say("T8.7b live payment source (sandbox; GET only; real ids in out/transcript.json)")
	t.say("events: capture %s, refund %s, dispute %s", captureEvent.ID, refundEvent.ID, disputeEvent.ID)
	t.say("capture event: type=%s verified=%v outcome=%s link={%s}",
		capture.EventType, capture.Verified, outcomeLine(capture.Outcome), linkLine(capture.Link))
	t.say("capture read: capture=%s order=%s custom=%s outcome=%s",
		captureRead.CaptureID, captureRead.OrderID, captureRead.CustomID, outcomeLine(captureRead.Outcome))
	t.say("capture order: order=%s custom=%s items=%d", captureOrder.OrderID, captureOrder.Link.CustomID, len(captureOrder.Items))
	for _, item := range captureOrder.Items {
		t.say("  capture item: %s", itemLine(item))
	}
	t.say("refund event: type=%s verified=%v outcome=%s link={%s}",
		refund.EventType, refund.Verified, outcomeLine(refund.Outcome), linkLine(refund.Link))
	t.say("refund read: refund=%s order=%s capture=%s custom=%s outcome=%s has_reason=%v",
		refundRead.RefundID, refundRead.Link.OrderID, refundRead.Link.CaptureID,
		refundRead.Link.CustomID, outcomeLine(refundRead.Outcome), refundRead.HasReason)
	t.say("refund order: order=%s custom=%s items=%d", refundOrder.OrderID, refundOrder.Link.CustomID, len(refundOrder.Items))
	for _, item := range refundOrder.Items {
		t.say("  refund item: %s", itemLine(item))
	}
	t.say("dispute event: type=%s verified=%v outcome=%s code=%s link={%s}",
		dispute.EventType, dispute.Verified, outcomeLine(dispute.Outcome), dispute.DisputeCode, linkLine(dispute.Link))
	t.say("dispute read: dispute=%s code=%s category=%s item={%s} link={%s}",
		disputeRead.DisputeID, disputeRead.Code, disputeRead.Category, itemLine(disputeRead.Item), linkLine(disputeRead.Link))
	t.say("dispute order: order=%s custom=%s items=%d", disputeOrder.OrderID, disputeOrder.Link.CustomID, len(disputeOrder.Items))
	for _, item := range disputeOrder.Items {
		t.say("  dispute item: %s", itemLine(item))
	}
	t.say("ingest capture: %s", resultLine(captureFirst, captureFit))
	t.say("ingest capture again: %s", resultLine(captureAgain, captureFit))
	t.say("ingest refund: %s", resultLine(refundFirst, refundFit))
	t.say("ingest refund again: %s", resultLine(refundAgain, refundFit))
	t.say("ingest dispute: %s", resultLine(disputeFirst, fit.rows))
	t.say("LIVE READ OK: three events mapped and ingested, redelivery is a duplicate; no money moved, no webhook sent, no model spend")

	text, err := t.text()
	if err != nil {
		fail(err)
	}
	fmt.Print(text)

	if err := os.MkdirAll(outDir, 0o700); err != nil {
		fail(fmt.Errorf("make out dir: %w", err))
	}
	full := map[string]any{
		"capture_event":  capture,
		"capture_order":  captureOrder,
		"capture_read":   captureRead,
		"refund_event":   refund,
		"refund_read":    refundRead,
		"refund_order":   refundOrder,
		"dispute_event":  dispute,
		"dispute_read":   disputeRead,
		"dispute_order":  disputeOrder,
		"ingest_rows":    len(service.Rows()),
		"ingest_pending": len(service.PendingQuestions()),
		"ingest": map[string]any{
			"capture":       captureFirst,
			"capture_again": captureAgain,
			"refund":        refundFirst,
			"refund_again":  refundAgain,
			"dispute":       disputeFirst,
			"fit_rows":      fit.rows,
		},
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
