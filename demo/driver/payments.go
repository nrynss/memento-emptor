package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// delivery is one stored webhook delivery from the test endpoint.
type delivery struct {
	eventID  string
	raw      json.RawMessage
	txID     string
	txTime   string
	certURL  string
	authAlgo string
	txSig    string
}

// siteToken returns the test endpoint's token parsed from the sandbox
// webhook URL, so neither value is committed.
func siteToken(ctx context.Context, c *restClient) (webhookID, token string, err error) {
	got, err := c.get(ctx, "/v1/notifications/webhooks")
	if err != nil {
		return "", "", err
	}
	for _, hook := range toSlice(got["webhooks"]) {
		hookMap, _ := hook.(map[string]any)
		u, _ := hookMap["url"].(string)
		if !strings.HasPrefix(u, siteBase+"/") {
			continue
		}
		id, _ := hookMap["id"].(string)
		return id, strings.TrimPrefix(u, siteBase+"/"), nil
	}
	return "", "", fmt.Errorf("demo: no webhook points at the test endpoint")
}

// listDeliveries reads the test endpoint's stored deliveries, newest
// first.
func listDeliveries(ctx context.Context, hc *http.Client, token string) ([]delivery, error) {
	u := siteBase + "/token/" + token + "/requests?sorting=newest&per_page=50"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var listed struct {
		Data []struct {
			Content string              `json:"content"`
			Headers map[string][]string `json:"headers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return nil, fmt.Errorf("demo: parse delivery list: %w", err)
	}
	out := []delivery{}
	for _, entry := range listed.Data {
		var probe struct {
			ID   string `json:"id"`
			Type string `json:"event_type"`
		}
		if err := json.Unmarshal([]byte(entry.Content), &probe); err != nil {
			continue
		}
		first := func(name string) string {
			for key, values := range entry.Headers {
				if strings.EqualFold(key, name) && len(values) > 0 {
					return values[0]
				}
			}
			return ""
		}
		out = append(out, delivery{
			eventID: probe.ID, raw: json.RawMessage(entry.Content),
			txID: first("paypal-transmission-id"), txTime: first("paypal-transmission-time"),
			certURL: first("paypal-cert-url"), authAlgo: first("paypal-auth-algo"),
			txSig: first("paypal-transmission-sig"),
		})
	}
	return out, nil
}

// resourceCustomID reads the join key out of a delivery resource: a
// refund/capture carries custom_id; a dispute carries it per item.
func resourceCustomID(resource map[string]any) string {
	if custom, _ := resource["custom_id"].(string); custom != "" {
		return custom
	}
	for _, txn := range toSlice(resource["disputed_transactions"]) {
		txnMap, _ := txn.(map[string]any)
		if custom, _ := txnMap["custom"].(string); custom != "" {
			return custom
		}
	}
	return ""
}

// findDelivery returns the newest delivery for one (custom id, event
// type) pair.
func findDelivery(deliveries []delivery, customID, eventType string) (*delivery, error) {
	for i := range deliveries {
		var probe struct {
			Type string `json:"event_type"`
		}
		if err := json.Unmarshal(deliveries[i].raw, &probe); err != nil {
			continue
		}
		if probe.Type != eventType {
			continue
		}
		var resource map[string]any
		var envelope struct {
			Resource map[string]any `json:"resource"`
		}
		if err := json.Unmarshal(deliveries[i].raw, &envelope); err != nil {
			continue
		}
		resource = envelope.Resource
		if resourceCustomID(resource) == customID {
			return &deliveries[i], nil
		}
	}
	return nil, fmt.Errorf("demo: no %s delivery for custom id %q", eventType, customID)
}

// verifyDelivery checks one live delivery's signature against the live
// verify API. The request body embeds the exact bytes PayPal posted.
func verifyDelivery(ctx context.Context, c *restClient, d *delivery) error {
	if d.txID == "" || d.txTime == "" || d.certURL == "" || d.authAlgo == "" || d.txSig == "" {
		return fmt.Errorf("demo: delivery %q is missing transmission headers", d.eventID)
	}
	payload := map[string]any{
		"transmission_id":   d.txID,
		"transmission_time": d.txTime,
		"cert_url":          d.certURL,
		"auth_algo":         d.authAlgo,
		"transmission_sig":  d.txSig,
		"webhook_id":        c.id,
		"webhook_event":     json.RawMessage(d.raw),
	}
	_, parsed, err := c.post(ctx, "/v1/notifications/verify-webhook-signature", "", payload)
	if err != nil {
		return err
	}
	status, _ := parsed["verification_status"].(string)
	if status != "SUCCESS" {
		return fmt.Errorf("demo: verify delivery: status %q", status)
	}
	return nil
}

// parseMinor turns a "195.00" amount value into minor units without
// float error. Only two-decimal values are accepted.
func parseMinor(value string) (int64, error) {
	whole, frac := value, ""
	if i := strings.Index(value, "."); i >= 0 {
		whole, frac = value[:i], value[i+1:]
	}
	if len(frac) > 2 {
		return 0, fmt.Errorf("demo: sub-cent amount %q", value)
	}
	for len(frac) < 2 {
		frac += "0"
	}
	var minor int64
	if _, err := fmt.Sscanf(whole+frac, "%d", &minor); err != nil {
		return 0, fmt.Errorf("demo: bad amount %q", value)
	}
	if strings.HasPrefix(whole, "-") {
		// Sscanf reads the sign; nothing more to do.
	}
	return minor, nil
}

// captureEvent builds the verified webhook event for one live capture
// delivery: outcome from the resource, items resolved through the live
// order read.
func captureEvent(d *delivery, orderID string) (fakes.WebhookEvent, error) {
	var envelope struct {
		Resource map[string]any `json:"resource"`
	}
	if err := json.Unmarshal(d.raw, &envelope); err != nil {
		return fakes.WebhookEvent{}, err
	}
	resource := envelope.Resource
	captureID, _ := resource["id"].(string)
	status, _ := resource["status"].(string)
	amount, _ := resource["amount"].(map[string]any)
	value, _ := amount["value"].(string)
	currency, _ := amount["currency_code"].(string)
	minor, err := parseMinor(value)
	if err != nil {
		return fakes.WebhookEvent{}, err
	}
	custom, _ := resource["custom_id"].(string)
	return fakes.WebhookEvent{
		EventID: d.eventID, EventType: "PAYMENT.CAPTURE.COMPLETED", Verified: true,
		Outcome:    domain.Outcome{Kind: domain.OutcomeCapture, Status: status, AmountMinor: minor, Currency: currency},
		Link:       domain.TransactionLink{OrderID: orderID, CaptureID: captureID, CustomID: custom, EventID: d.eventID},
		Provenance: "live:sandbox",
	}, nil
}

// purchaseSource is a PaymentSource that replays the frozen story for
// every fixture id and serves the demo run's own live purchase through
// the same seam. Unknown ids fail with fakes.ErrNoRecording, exactly
// like the recording source: the wrapper never invents a transaction,
// it only exposes the live reads the run just created.
type purchaseSource struct {
	base    fakes.PaymentSource
	event   *fakes.WebhookEvent
	order   *fakes.OrderRecord
	eventID string
	orderID string
}

// Compile-time proof the wrapper satisfies the seam the service builds
// on.
var _ fakes.PaymentSource = (*purchaseSource)(nil)

// VerifyWebhook returns the verified delivery for one event id.
func (s *purchaseSource) VerifyWebhook(ctx context.Context, eventID string) (fakes.WebhookEvent, error) {
	if s.event != nil && eventID == s.eventID {
		return *s.event, nil
	}
	return s.base.VerifyWebhook(ctx, eventID)
}

// GetOrder returns the order read with its line items.
func (s *purchaseSource) GetOrder(ctx context.Context, orderID string) (fakes.OrderRecord, error) {
	if s.order != nil && orderID == s.orderID {
		return *s.order, nil
	}
	return s.base.GetOrder(ctx, orderID)
}

// GetRefund returns the refund read.
func (s *purchaseSource) GetRefund(ctx context.Context, refundID string) (fakes.RefundRecord, error) {
	return s.base.GetRefund(ctx, refundID)
}

// GetDispute returns the dispute read.
func (s *purchaseSource) GetDispute(ctx context.Context, disputeID string) (fakes.DisputeRecord, error) {
	return s.base.GetDispute(ctx, disputeID)
}

// ledgerEvents lists recent webhook events, newest first.
func ledgerEvents(ctx context.Context, c *restClient) ([]any, error) {
	got, err := c.get(ctx, "/v1/notifications/webhooks-events?page_size=20")
	if err != nil {
		return nil, err
	}
	return toSlice(got["events"]), nil
}

// resend asks PayPal to redeliver one webhook event to the test
// endpoint. Redelivery is proven in T0.4; this is the fallback when the
// automatic delivery has not landed yet.
func resend(ctx context.Context, c *restClient, eventID string) error {
	status, _, err := c.post(ctx, "/v1/notifications/webhooks-events/"+url.PathEscape(eventID)+"/resend", "", nil)
	if err != nil {
		return err
	}
	if status != 200 && status != 202 {
		return fmt.Errorf("demo: resend status %d", status)
	}
	return nil
}
