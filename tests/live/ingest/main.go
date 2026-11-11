// Live driver for T2.1b: a real sandbox refund and dispute through the
// running ingest service.
//
// The driver reads back two existing sandbox records (the T1.3b demo
// history: one fit refund, one non-arrival dispute) and creates nothing:
// no order, no capture, no refund, no dispute, no webhook change. Every
// PayPal call is a read or a signature verification, which are free.
//
// For each event the driver fetches the live delivery PayPal posted to
// the sandbox webhook's test endpoint, verifies its signature against
// the live verify API, ingests it through ingest.Service over a live
// PaymentSource (plain REST reads: refund, capture, order, dispute),
// asserts the rows are right, and proves redelivery is idempotent.
//
// Real PayPal ids go only to out/ (git-ignored) and never to stdout:
// stdout carries the redacted transcript with fixed placeholders, and
// the driver refuses to finish if a real id survives redaction.
// Credentials come from the environment (tools/env.sh) and are never
// printed, logged, or written.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/ingest"
)

// sandbox is the PayPal sandbox REST root. Sandbox only: no live
// credential or host ever enters this driver.
const sandbox = "https://api-m.sandbox.paypal.com"

// siteBase is the webhook.site API root used to read back the test
// endpoint's stored deliveries.
const siteBase = "https://webhook.site"

// outDir holds the full transcript with real ids. Git-ignored.
const outDir = "tests/live/ingest/out"

// wantEvents names the delivery the driver ingests for each outcome.
var wantEvents = map[string]string{
	"PAYMENT.CAPTURE.REFUNDED":  "refund",
	"CUSTOMER.DISPUTE.CREATED":  "dispute",
	"PAYMENT.CAPTURE.COMPLETED": "capture",
}

// demoCatalog maps the demo's SKUs to the layer's item rows. The order
// GET names the SKU and the human name; brand, line, size and category
// are the catalog's, which later tasks join in. An unknown SKU fails
// the run: the driver never invents a variant.
var demoCatalog = map[string]domain.Item{
	"BRX-OXF-WHT": {
		BrandID: "brand-x-001", BrandName: "Brand X",
		LineName: "oxford shirt", SKU: "BRX-OXF-WHT",
		Size: "M", Category: "shirts",
	},
	"BRX-OXF-STR": {
		BrandID: "brand-x-001", BrandName: "Brand X",
		LineName: "oxford shirt", SKU: "BRX-OXF-STR",
		Size: "M", Category: "shirts",
	},
}

// delivery is one webhook delivery read back from the test endpoint:
// the exact bytes PayPal posted plus the transmission headers the
// verify API needs.
type delivery struct {
	eventID   string
	eventType string
	created   time.Time
	raw       json.RawMessage
	txID      string
	txTime    string
	certURL   string
	authAlgo  string
	txSig     string
}

// resource returns the delivery's parsed resource object.
func (d *delivery) resource() map[string]any {
	var body struct {
		Resource map[string]any `json:"resource"`
	}
	if err := json.Unmarshal(d.raw, &body); err != nil {
		return nil
	}
	return body.Resource
}

// client is a minimal sandbox REST client.
type client struct {
	http      *http.Client
	token     string
	webhookID string
}

// get performs an authenticated sandbox GET and parses the response.
func (c *client) get(ctx context.Context, path string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sandbox+path, nil)
	if err != nil {
		return nil, fmt.Errorf("live: build GET %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("live: GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("live: read GET %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("live: GET %s: status %d: %s", path, resp.StatusCode, compact(body))
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("live: parse GET %s: %w", path, err)
	}
	return parsed, nil
}

// postJSON performs an authenticated sandbox POST with a JSON body.
func (c *client) postJSON(ctx context.Context, path string, payload any) (map[string]any, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("live: encode POST %s: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sandbox+path, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("live: build POST %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("live: POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("live: read POST %s: %w", path, err)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("live: POST %s: status %d: %s", path, resp.StatusCode, compact(body))
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("live: parse POST %s: %w", path, err)
	}
	return parsed, nil
}

// compact trims an error body for messages without flooding them.
func compact(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) > 300 {
		return trimmed[:300] + "..."
	}
	return trimmed
}

// grant mints a sandbox access token from the environment credentials.
func grant(ctx context.Context, hc *http.Client) (string, error) {
	id, secret := os.Getenv("PAYPAL_CLIENT_ID"), os.Getenv("PAYPAL_CLIENT_SECRET")
	if id == "" || secret == "" {
		return "", fmt.Errorf("live: PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET must come from tools/env.sh")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sandbox+"/v1/oauth2/token", strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", fmt.Errorf("live: build token request: %w", err)
	}
	req.SetBasicAuth(id, secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("live: token grant: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("live: read token grant: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("live: token grant refused (status %d): stop and report, do not retry", resp.StatusCode)
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("live: token grant status %d: %s", resp.StatusCode, compact(body))
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.AccessToken == "" {
		return "", fmt.Errorf("live: parse token grant: %w", err)
	}
	return parsed.AccessToken, nil
}

// webhookConfig returns the sandbox webhook id and the test endpoint's
// token parsed from the webhook URL, so neither value is committed.
func webhookConfig(ctx context.Context, c *client) (webhookID, siteToken string, err error) {
	got, err := c.get(ctx, "/v1/notifications/webhooks")
	if err != nil {
		return "", "", err
	}
	hooks, _ := got["webhooks"].([]any)
	for _, hook := range hooks {
		hookMap, _ := hook.(map[string]any)
		url, _ := hookMap["url"].(string)
		if !strings.HasPrefix(url, siteBase+"/") {
			continue
		}
		id, _ := hookMap["id"].(string)
		return id, strings.TrimPrefix(url, siteBase+"/"), nil
	}
	return "", "", fmt.Errorf("live: no webhook points at the test endpoint")
}

// ledgerEvent is one entry of the webhook events ledger.
type ledgerEvent struct {
	id        string
	eventType string
	created   time.Time
}

// ledger lists recent webhook events and picks the newest delivery for
// each wanted type.
func ledger(ctx context.Context, c *client) ([]ledgerEvent, error) {
	got, err := c.get(ctx, "/v1/notifications/webhooks-events?page_size=20")
	if err != nil {
		return nil, err
	}
	var out []ledgerEvent
	for _, entry := range toSlice(got["events"]) {
		item, _ := entry.(map[string]any)
		id, _ := item["id"].(string)
		eventType, _ := item["event_type"].(string)
		created, _ := item["create_time"].(string)
		moment, err := time.Parse(time.RFC3339, created)
		if err != nil {
			return nil, fmt.Errorf("live: parse event time %q: %w", created, err)
		}
		out = append(out, ledgerEvent{id: id, eventType: eventType, created: moment})
	}
	return out, nil
}

// fetchDelivery reads one stored delivery back from the test endpoint.
func fetchDelivery(ctx context.Context, hc *http.Client, siteToken, eventID string) (*delivery, error) {
	url := siteBase + "/token/" + siteToken + "/requests?sorting=newest&per_page=50"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("live: build delivery read: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("live: read deliveries: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("live: read delivery list: %w", err)
	}
	var listed struct {
		Data []struct {
			Content string              `json:"content"`
			Headers map[string][]string `json:"headers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		return nil, fmt.Errorf("live: parse delivery list: %w", err)
	}
	for _, entry := range listed.Data {
		var probe struct {
			ID         string `json:"id"`
			EventType  string `json:"event_type"`
			CreateTime string `json:"create_time"`
		}
		if err := json.Unmarshal([]byte(entry.Content), &probe); err != nil {
			continue
		}
		if probe.ID != eventID {
			continue
		}
		moment, err := time.Parse(time.RFC3339, probe.CreateTime)
		if err != nil {
			return nil, fmt.Errorf("live: parse delivery time %q: %w", probe.CreateTime, err)
		}
		first := func(name string) string {
			for key, values := range entry.Headers {
				if strings.EqualFold(key, name) && len(values) > 0 {
					return values[0]
				}
			}
			return ""
		}
		return &delivery{
			eventID: eventID, eventType: probe.EventType,
			created: moment, raw: json.RawMessage(entry.Content),
			txID:     first("paypal-transmission-id"),
			txTime:   first("paypal-transmission-time"),
			certURL:  first("paypal-cert-url"),
			authAlgo: first("paypal-auth-algo"),
			txSig:    first("paypal-transmission-sig"),
		}, nil
	}
	return nil, fmt.Errorf("live: no stored delivery for event %q", eventID)
}

// verify checks one live delivery's signature against the live verify
// API. The request body embeds the exact bytes PayPal posted.
func verify(ctx context.Context, c *client, d *delivery) error {
	if d.txID == "" || d.txTime == "" || d.certURL == "" || d.authAlgo == "" || d.txSig == "" {
		return fmt.Errorf("live: delivery %q is missing transmission headers", d.eventID)
	}
	payload := map[string]any{
		"transmission_id":   d.txID,
		"transmission_time": d.txTime,
		"cert_url":          d.certURL,
		"auth_algo":         d.authAlgo,
		"transmission_sig":  d.txSig,
		"webhook_id":        c.webhookID,
		"webhook_event":     d.raw,
	}
	got, err := c.postJSON(ctx, "/v1/notifications/verify-webhook-signature", payload)
	if err != nil {
		return err
	}
	status, _ := got["verification_status"].(string)
	if status != "SUCCESS" {
		return fmt.Errorf("live: verify delivery %q: status %q", d.eventID, status)
	}
	return nil
}

// liveSource is a PaymentSource over live sandbox reads. Deliveries are
// already verified; the reads join each outcome to its order like the
// recording source does.
type liveSource struct {
	c          *client
	deliveries map[string]*delivery
	events     map[string]fakes.WebhookEvent
	orders     map[string]fakes.OrderRecord
	refunds    map[string]fakes.RefundRecord
	disputes   map[string]fakes.DisputeRecord
	// links caches each verified event's full transaction link, keyed
	// by refund or dispute id, so the record reads below return the
	// same complete link the recording source carries.
	links map[string]domain.TransactionLink
}

// str navigates a parsed JSON object by string keys.
func str(obj map[string]any, keys ...string) string {
	var current any = obj
	for _, key := range keys {
		next, _ := current.(map[string]any)
		current = next[key]
	}
	text, _ := current.(string)
	return text
}

// toSlice coerces a parsed JSON value to a slice.
func toSlice(value any) []any {
	items, _ := value.([]any)
	return items
}

// parseMinor turns a "195.00" amount value into minor units without
// float error. Only two-decimal values are accepted.
func parseMinor(value string) (int64, error) {
	whole, frac := value, ""
	if i := strings.Index(value, "."); i >= 0 {
		whole, frac = value[:i], value[i+1:]
	}
	if len(frac) > 2 {
		return 0, fmt.Errorf("live: sub-cent amount %q", value)
	}
	for len(frac) < 2 {
		frac += "0"
	}
	digits := whole + frac
	if digits == "" {
		return 0, fmt.Errorf("live: empty amount")
	}
	minor, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("live: bad amount %q: %w", value, err)
	}
	return minor, nil
}

// captureOrder follows a capture's up link to its order id.
func (s *liveSource) captureOrder(ctx context.Context, captureID string) (orderID string, err error) {
	capture, err := s.c.get(ctx, "/v2/payments/captures/"+captureID)
	if err != nil {
		return "", err
	}
	for _, link := range toSlice(capture["links"]) {
		linkMap, _ := link.(map[string]any)
		if linkMap["rel"] == "up" {
			href, _ := linkMap["href"].(string)
			if i := strings.LastIndex(href, "/"); i >= 0 {
				return href[i+1:], nil
			}
		}
	}
	return "", fmt.Errorf("live: capture %q names no order", captureID)
}

// VerifyWebhook returns the verified delivery for one event id,
// resolving its links through live reads.
func (s *liveSource) VerifyWebhook(ctx context.Context, eventID string) (fakes.WebhookEvent, error) {
	if event, ok := s.events[eventID]; ok {
		return event, nil
	}
	d, ok := s.deliveries[eventID]
	if !ok {
		return fakes.WebhookEvent{}, fmt.Errorf("%w: webhook event %q", fakes.ErrNoRecording, eventID)
	}
	resource := d.resource()
	var event fakes.WebhookEvent
	switch d.eventType {
	case "PAYMENT.CAPTURE.REFUNDED":
		refundID := str(resource, "id")
		custom := str(resource, "custom_id")
		captureID := ""
		for _, link := range toSlice(resource["links"]) {
			linkMap, _ := link.(map[string]any)
			if linkMap["rel"] == "up" {
				href, _ := linkMap["href"].(string)
				captureID = href[strings.LastIndex(href, "/")+1:]
			}
		}
		orderID, err := s.captureOrder(ctx, captureID)
		if err != nil {
			return fakes.WebhookEvent{}, err
		}
		amount, err := parseMinor(str(resource, "amount", "value"))
		if err != nil {
			return fakes.WebhookEvent{}, err
		}
		event = fakes.WebhookEvent{
			EventID: eventID, EventType: d.eventType, Verified: true,
			Outcome:    domain.Outcome{Kind: domain.OutcomeRefund, Status: str(resource, "status"), AmountMinor: amount, Currency: str(resource, "amount", "currency_code")},
			Link:       domain.TransactionLink{OrderID: orderID, CaptureID: captureID, RefundID: refundID, CustomID: custom, EventID: eventID},
			Provenance: "live:sandbox",
		}
	case "CUSTOMER.DISPUTE.CREATED":
		disputeID := str(resource, "dispute_id")
		code := domain.DisputeCode(str(resource, "reason"))
		custom, captureID := "", ""
		for _, txn := range toSlice(resource["disputed_transactions"]) {
			txnMap, _ := txn.(map[string]any)
			custom = str(txnMap, "custom")
			captureID, _ = txnMap["seller_transaction_id"].(string)
		}
		orderID := ""
		if captureID != "" {
			resolved, resolveErr := s.captureOrder(ctx, captureID)
			if resolveErr != nil {
				return fakes.WebhookEvent{}, resolveErr
			}
			orderID = resolved
		}
		amount, err := parseMinor(str(resource, "dispute_amount", "value"))
		if err != nil {
			return fakes.WebhookEvent{}, err
		}
		event = fakes.WebhookEvent{
			EventID: eventID, EventType: d.eventType, Verified: true,
			Outcome:     domain.Outcome{Kind: domain.OutcomeDispute, Status: str(resource, "status"), AmountMinor: amount, Currency: str(resource, "dispute_amount", "currency_code")},
			Link:        domain.TransactionLink{OrderID: orderID, CaptureID: captureID, DisputeID: disputeID, CustomID: custom, EventID: eventID},
			DisputeCode: code,
			Provenance:  "live:sandbox",
		}
	default:
		return fakes.WebhookEvent{}, fmt.Errorf("live: event %q type %q is not ingested", eventID, d.eventType)
	}
	s.events[eventID] = event
	if event.Link.RefundID != "" {
		s.links[event.Link.RefundID] = event.Link
	}
	if event.Link.DisputeID != "" {
		s.links[event.Link.DisputeID] = event.Link
	}
	return event, nil
}

// GetOrder returns the order read with its line items, mapped through
// the demo catalog and checked against the order's own names.
func (s *liveSource) GetOrder(ctx context.Context, orderID string) (fakes.OrderRecord, error) {
	if record, ok := s.orders[orderID]; ok {
		return record, nil
	}
	got, err := s.c.get(ctx, "/v2/checkout/orders/"+orderID)
	if err != nil {
		return fakes.OrderRecord{}, err
	}
	var items []domain.Item
	custom := ""
	for _, unit := range toSlice(got["purchase_units"]) {
		unitMap, _ := unit.(map[string]any)
		if custom == "" {
			custom = str(unitMap, "custom_id")
		}
		for _, entry := range toSlice(unitMap["items"]) {
			itemMap, _ := entry.(map[string]any)
			sku, _ := itemMap["sku"].(string)
			name, _ := itemMap["name"].(string)
			catalog, ok := demoCatalog[sku]
			if !ok {
				return fakes.OrderRecord{}, fmt.Errorf("live: order %q carries unknown SKU %q", orderID, sku)
			}
			if !strings.Contains(strings.ToLower(name), strings.ToLower(catalog.LineName)) {
				return fakes.OrderRecord{}, fmt.Errorf("live: order %q item %q names no %q", orderID, name, catalog.LineName)
			}
			items = append(items, catalog)
		}
	}
	if len(items) == 0 {
		return fakes.OrderRecord{}, fmt.Errorf("live: order %q carries no line items", orderID)
	}
	record := fakes.OrderRecord{
		OrderID: orderID, Items: items,
		Link:       domain.TransactionLink{OrderID: orderID, CustomID: custom},
		Provenance: "live:sandbox",
	}
	s.orders[orderID] = record
	return record, nil
}

// GetRefund returns the refund read, which carries no reason: the
// reason comes from the shopper, never from this record.
func (s *liveSource) GetRefund(ctx context.Context, refundID string) (fakes.RefundRecord, error) {
	if record, ok := s.refunds[refundID]; ok {
		return record, nil
	}
	got, err := s.c.get(ctx, "/v2/payments/refunds/"+refundID)
	if err != nil {
		return fakes.RefundRecord{}, err
	}
	amount, err := parseMinor(str(got, "amount", "value"))
	if err != nil {
		return fakes.RefundRecord{}, err
	}
	captureID := ""
	for _, link := range toSlice(got["links"]) {
		linkMap, _ := link.(map[string]any)
		if linkMap["rel"] == "up" {
			href, _ := linkMap["href"].(string)
			captureID = href[strings.LastIndex(href, "/")+1:]
		}
	}
	record := fakes.RefundRecord{
		RefundID:   refundID,
		Outcome:    domain.Outcome{Kind: domain.OutcomeRefund, Status: str(got, "status"), AmountMinor: amount, Currency: str(got, "amount", "currency_code")},
		Link:       domain.TransactionLink{CaptureID: captureID, RefundID: refundID, CustomID: str(got, "custom_id")},
		HasReason:  false,
		Provenance: "live:sandbox",
	}
	if link, ok := s.links[refundID]; ok {
		record.Link = link
	}
	s.refunds[refundID] = record
	return record, nil
}

// GetDispute returns the dispute read with its reason code mapped. A
// code the frozen table does not name keeps its spelling on the row and
// carries no category: the mapping comes from
// domain.StoredCategoryForDispute, so this read makes no judgement of its
// own about an unknown code.
func (s *liveSource) GetDispute(ctx context.Context, disputeID string) (fakes.DisputeRecord, error) {
	if record, ok := s.disputes[disputeID]; ok {
		return record, nil
	}
	got, err := s.c.get(ctx, "/v1/customer/disputes/"+disputeID)
	if err != nil {
		return fakes.DisputeRecord{}, err
	}
	code := domain.DisputeCode(str(got, "reason"))
	category, err := domain.StoredCategoryForDispute(code)
	if err != nil {
		return fakes.DisputeRecord{}, err
	}
	var item domain.Item
	custom, captureID := "", ""
	for _, txn := range toSlice(got["disputed_transactions"]) {
		txnMap, _ := txn.(map[string]any)
		custom = str(txnMap, "custom")
		captureID, _ = txnMap["seller_transaction_id"].(string)
		for _, entry := range toSlice(txnMap["items"]) {
			itemMap, _ := entry.(map[string]any)
			sku, _ := itemMap["item_id"].(string)
			catalog, ok := demoCatalog[sku]
			if !ok {
				return fakes.DisputeRecord{}, fmt.Errorf("live: dispute %q carries unknown SKU %q", disputeID, sku)
			}
			item = catalog
		}
	}
	if item.SKU == "" {
		return fakes.DisputeRecord{}, fmt.Errorf("live: dispute %q names no item", disputeID)
	}
	record := fakes.DisputeRecord{
		DisputeID: disputeID, Code: code, Category: category, Item: item,
		Link:       domain.TransactionLink{CaptureID: captureID, DisputeID: disputeID, CustomID: custom},
		Provenance: "live:sandbox",
	}
	if link, ok := s.links[disputeID]; ok {
		record.Link = link
	}
	s.disputes[disputeID] = record
	return record, nil
}

// redactor maps real sandbox ids to fixed placeholders for stdout.
// The full transcript with real ids goes to out/ only.
type redactor struct {
	mapping map[string]string
	email   *regexp.Regexp
}

// add records one replacement, longest ids first at print time.
func (r *redactor) add(real, placeholder string) {
	if real != "" {
		r.mapping[real] = placeholder
	}
}

// apply replaces every known real id and every email address.
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

// check refuses to finish if a real id survived redaction.
func (r *redactor) check(text string) error {
	for real, placeholder := range r.mapping {
		if strings.Contains(text, real) {
			return fmt.Errorf("live: redaction missed %s", placeholder)
		}
	}
	return nil
}

// transcript collects the redacted stdout lines.
type transcript struct {
	lines []string
	r     *redactor
}

// say records one line, redacted.
func (t *transcript) say(format string, args ...any) {
	t.lines = append(t.lines, t.r.apply(fmt.Sprintf(format, args...)))
}

// text renders the transcript and checks every real id is gone.
func (t *transcript) text() (string, error) {
	out := strings.Join(t.lines, "\n") + "\n"
	if err := t.r.check(out); err != nil {
		return "", err
	}
	return out, nil
}

// rowLine renders one evidence row for the transcript.
func rowLine(row domain.Evidence) string {
	reason := "<nil>"
	counts := string(row.CountsToward())
	if counts == "" {
		counts = "none"
	}
	if row.Reason != nil {
		reason = fmt.Sprintf("%s %q %.2f", row.Reason.Category, row.Reason.Text, row.Reason.Confidence)
	}
	return fmt.Sprintf("outcome=%s %s %d %s custom=%s reason=%s counts_toward=%s item=%s link=%s/%s/%s/%s",
		row.Outcome.Kind, row.Outcome.Status, row.Outcome.AmountMinor, row.Outcome.Currency,
		row.Link.CustomID, reason, counts, row.Item.SKU,
		row.Link.OrderID, row.Link.CaptureID, row.Link.RefundID, row.Link.DisputeID)
}

// fail prints to stderr and exits nonzero.
func fail(err error) {
	fmt.Fprintf(os.Stderr, "live-ingest: %v\n", err)
	os.Exit(1)
}

func main() {
	ctx := context.Background()
	hc := &http.Client{Timeout: 30 * time.Second}
	clock := map[string]time.Time{}

	progress := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "[live-ingest] "+format+"\n", args...)
	}

	token, err := grant(ctx, hc)
	if err != nil {
		fail(err)
	}
	progress("token granted")
	c := &client{http: hc, token: token}

	webhookID, siteToken, err := webhookConfig(ctx, c)
	if err != nil {
		fail(err)
	}
	c.webhookID = webhookID
	progress("webhook found")

	events, err := ledger(ctx, c)
	if err != nil {
		fail(err)
	}
	progress("ledger lists %d events", len(events))
	pick := map[string]ledgerEvent{}
	for _, entry := range events {
		if _, wanted := wantEvents[entry.eventType]; !wanted {
			continue
		}
		if _, taken := pick[entry.eventType]; taken {
			continue
		}
		pick[entry.eventType] = entry
	}
	refundPick, ok := pick["PAYMENT.CAPTURE.REFUNDED"]
	if !ok {
		fail(fmt.Errorf("live: ledger holds no refund event"))
	}
	disputePick, ok := pick["CUSTOMER.DISPUTE.CREATED"]
	if !ok {
		fail(fmt.Errorf("live: ledger holds no dispute event"))
	}

	deliveries := map[string]*delivery{}
	for _, entry := range []ledgerEvent{refundPick, disputePick} {
		d, err := fetchDelivery(ctx, hc, siteToken, entry.id)
		if err != nil {
			fail(err)
		}
		progress("delivery %s read back (%s)", entry.eventType, entry.id[:8]+"...")
		if err := verify(ctx, c, d); err != nil {
			fail(err)
		}
		progress("delivery %s signature verified", entry.eventType)
		deliveries[entry.id] = d
		clock[entry.id] = d.created
	}

	src := &liveSource{c: c, deliveries: deliveries,
		events: map[string]fakes.WebhookEvent{}, orders: map[string]fakes.OrderRecord{},
		refunds: map[string]fakes.RefundRecord{}, disputes: map[string]fakes.DisputeRecord{},
		links: map[string]domain.TransactionLink{}}
	svc := ingest.NewService(src, ingest.WithClock(func(eventID string) time.Time { return clock[eventID] }))

	refundResult, err := svc.Handle(ctx, refundPick.id)
	if err != nil {
		fail(fmt.Errorf("live: ingest refund: %w", err))
	}
	refundRepeat, err := svc.Handle(ctx, refundPick.id)
	if err != nil {
		fail(fmt.Errorf("live: ingest refund redelivery: %w", err))
	}
	disputeResult, err := svc.Handle(ctx, disputePick.id)
	if err != nil {
		fail(fmt.Errorf("live: ingest dispute: %w", err))
	}
	disputeRepeat, err := svc.Handle(ctx, disputePick.id)
	if err != nil {
		fail(fmt.Errorf("live: ingest dispute redelivery: %w", err))
	}

	if len(refundResult.Rows) == 0 {
		fail(fmt.Errorf("live: refund produced no rows"))
	}
	for _, row := range refundResult.Rows {
		if row.Reason != nil {
			fail(fmt.Errorf("live: refund row carries a reason"))
		}
		if row.CountsToward() != domain.BeliefKindNone {
			fail(fmt.Errorf("live: refund row counts toward %q", row.CountsToward()))
		}
	}
	if refundResult.Pending == nil {
		fail(fmt.Errorf("live: refund filed no pending question"))
	}
	if !refundRepeat.Duplicate || len(refundRepeat.Rows) != len(refundResult.Rows) {
		fail(fmt.Errorf("live: refund redelivery was not idempotent"))
	}
	if len(disputeResult.Rows) == 0 {
		fail(fmt.Errorf("live: dispute produced no rows"))
	}
	// The dispute's own code decides the row's reason, through
	// domain.StoredCategoryForDispute: a code outside the frozen table
	// keeps its spelling, carries no category, and counts toward
	// nothing.
	disputeCode := domain.DisputeCode(str(deliveries[disputePick.id].resource(), "reason"))
	disputeWant, err := domain.StoredCategoryForDispute(disputeCode)
	if err != nil {
		fail(fmt.Errorf("live: map dispute code %q: %w", disputeCode, err))
	}
	for _, row := range disputeResult.Rows {
		switch {
		case disputeCode == "" && row.Reason != nil:
			fail(fmt.Errorf("live: dispute row carries a reason for a case with no code"))
		case disputeCode != "" && (row.Reason == nil || row.Reason.Category != disputeWant || row.Reason.Text != string(disputeCode)):
			fail(fmt.Errorf("live: dispute row has wrong reason"))
		}
		if row.CountsToward() != disputeWant.CountsToward() {
			fail(fmt.Errorf("live: dispute row counts toward %q, want %q", row.CountsToward(), disputeWant.CountsToward()))
		}
	}
	if !disputeRepeat.Duplicate || len(disputeRepeat.Rows) != len(disputeResult.Rows) {
		fail(fmt.Errorf("live: dispute redelivery was not idempotent"))
	}

	r := &redactor{mapping: map[string]string{}, email: regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.]+`)}
	r.add(webhookID, "WEBHOOK-ID-LIVE-1")
	r.add(siteToken, "WEBHOOK-SITE-UUID-REDACTED")
	refundLink := refundResult.Rows[0].Link
	disputeLink := disputeResult.Rows[0].Link
	r.add(refundPick.id, "WH-EVENT-REFUND-LIVE-1")
	r.add(disputePick.id, "WH-EVENT-DISPUTE-LIVE-1")
	r.add(refundLink.OrderID, "ORDER-ID-LIVE-1")
	r.add(refundLink.CaptureID, "CAPTURE-ID-LIVE-1")
	r.add(refundLink.RefundID, "REFUND-ID-LIVE-1")
	r.add(disputeLink.OrderID, "ORDER-ID-LIVE-2")
	r.add(disputeLink.CaptureID, "CAPTURE-ID-LIVE-2")
	r.add(disputeLink.DisputeID, "DISPUTE-ID-LIVE-1")
	for _, d := range deliveries {
		r.add(d.txID, "TRANSMISSION-ID-LIVE")
		r.add(d.txSig, "TRANSMISSION-SIG-LIVE")
		r.add(d.certURL, "CERT-URL-LIVE")
	}

	t := &transcript{r: r}
	t.say("T2.1b live ingest transcript (sandbox; placeholders -- real ids in out/transcript.json)")
	t.say("webhook WEBHOOK-ID-LIVE-1 serves CUSTOMER.DISPUTE.CREATED, PAYMENT.CAPTURE.COMPLETED, PAYMENT.CAPTURE.REFUNDED")
	t.say("ledger holds %d events; chose PAYMENT.CAPTURE.REFUNDED %s and CUSTOMER.DISPUTE.CREATED %s",
		len(events), refundPick.id, disputePick.id)
	for _, entry := range []ledgerEvent{refundPick, disputePick} {
		d := deliveries[entry.id]
		t.say("verify %s %s: SUCCESS (%s, cert CERT-URL-LIVE)", d.eventType, entry.id, d.authAlgo)
	}
	t.say("refund rows: %d", len(refundResult.Rows))
	for _, row := range refundResult.Rows {
		t.say("  %s", rowLine(row))
	}
	t.say("refund pending: event %s outcome %s %d %s",
		refundResult.Pending.EventID, refundResult.Pending.Outcome.Kind,
		refundResult.Pending.Outcome.AmountMinor, refundResult.Pending.Outcome.Currency)
	t.say("refund redelivery: duplicate, same %d row(s)", len(refundRepeat.Rows))
	t.say("dispute rows: %d", len(disputeResult.Rows))
	disputeCounts := string(disputeWant.CountsToward())
	if disputeCounts == "" {
		disputeCounts = "none"
	}
	t.say("dispute reason code %q maps to category %q (counts toward %s)",
		disputeCode, disputeWant, disputeCounts)
	for _, row := range disputeResult.Rows {
		t.say("  %s", rowLine(row))
	}
	t.say("dispute redelivery: duplicate, same %d row(s)", len(disputeRepeat.Rows))
	t.say("LIVE OK: 1 refund + 1 dispute ingested, signatures verified, no new sandbox records created")

	text, err := t.text()
	if err != nil {
		fail(err)
	}
	fmt.Print(text)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail(fmt.Errorf("live: make out dir: %w", err))
	}
	full := map[string]any{
		"webhook_id":  webhookID,
		"ledger_size": len(events),
		"refund_event": map[string]any{
			"event_id": refundPick.id, "verified": true,
			"rows": refundResult.Rows, "pending": refundResult.Pending,
			"redelivery_duplicate": refundRepeat.Duplicate,
		},
		"dispute_event": map[string]any{
			"event_id": disputePick.id, "verified": true,
			"rows":                 disputeResult.Rows,
			"redelivery_duplicate": disputeRepeat.Duplicate,
		},
	}
	encoded, err := json.MarshalIndent(full, "", "  ")
	if err != nil {
		fail(fmt.Errorf("live: encode transcript: %w", err))
	}
	if err := os.WriteFile(filepath.Join(outDir, "transcript.json"), append(encoded, '\n'), 0o600); err != nil {
		fail(fmt.Errorf("live: write transcript: %w", err))
	}
	progress("full transcript in %s/transcript.json", outDir)
}
