// Package main drives the T3.6 full demo: product.md's six steps end to
// end, live on the real stack, from the seeded demo shopper.
//
// The driver composes only frozen pieces: the recording payment source
// (whose shapes T2.1b proved identical to the live sandbox reads), the
// frozen interface service, the frozen write rule over a live Lambo
// session, a live Jev classifier, the frozen catalog match over a live
// Channel3 detail, and the frozen orders state machine over a live
// sandbox REST transport. Nothing here changes those pieces.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/agent/orders"
)

// sandbox is the PayPal sandbox REST root. Sandbox only: no live
// credential or host ever enters this driver.
const sandbox = "https://api-m.sandbox.paypal.com"

// siteBase is the webhook.site API root holding the test endpoint's
// stored deliveries.
const siteBase = "https://webhook.site"

// restClient is a minimal sandbox REST client: token grant, GET reads,
// order create and capture. Reads are free; create and capture move only
// sandbox money through the project's own merchant.
type restClient struct {
	http  *http.Client
	token string
	id    string
}

// grant mints a sandbox access token from the environment credentials,
// which arrive via tools/env.sh and are never printed, logged or written.
func grant(ctx context.Context, hc *http.Client) (string, error) {
	id, secret := os.Getenv("PAYPAL_CLIENT_ID"), os.Getenv("PAYPAL_CLIENT_SECRET")
	if id == "" || secret == "" {
		return "", fmt.Errorf("demo: PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET must come from tools/env.sh")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sandbox+"/v1/oauth2/token", strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(id, secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("demo: token grant: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("demo: token grant status %d", resp.StatusCode)
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed.AccessToken == "" {
		return "", fmt.Errorf("demo: parse token grant")
	}
	return parsed.AccessToken, nil
}

// get performs an authenticated sandbox GET and parses the response.
func (c *restClient) get(ctx context.Context, path string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sandbox+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("demo: GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("demo: GET %s: status %d: %.200s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("demo: parse GET %s: %w", path, err)
	}
	return parsed, nil
}

// post performs an authenticated sandbox POST with a JSON body. The
// caller supplies the PayPal-Request-Id so retries stay idempotent.
func (c *restClient) post(ctx context.Context, path string, requestID string, payload any) (int, map[string]any, error) {
	var encoded []byte
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		encoded = raw
	} else {
		encoded = []byte("{}")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sandbox+path, bytes.NewReader(encoded))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	if requestID != "" {
		req.Header.Set("PayPal-Request-Id", requestID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("demo: POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return resp.StatusCode, nil, fmt.Errorf("demo: parse POST %s status %d: %.200s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return resp.StatusCode, parsed, nil
}

// restTransport is an orders.CheckoutTransport over live sandbox REST.
// Construction refuses a non-sandbox merchant in NewService itself; this
// transport additionally pins the sandbox root in code.
type restTransport struct {
	c       *restClient
	approve map[string]string
}

// CreateOrder creates the wallet order: intent CAPTURE, one purchase
// unit with line items, idempotent on params.RequestID. The approval
// URL rides beside the order id for the scripted shopper approval.
func (t *restTransport) CreateOrder(ctx context.Context, params orders.CreateParams) (orders.CreateResult, error) {
	amount := fmt.Sprintf("%.2f", float64(params.AmountMinor)/100)
	body := map[string]any{
		"intent": "CAPTURE",
		"purchase_units": []map[string]any{{
			"reference_id": params.CustomID,
			"custom_id":    params.CustomID,
			"description":  params.Item.LineName + " size " + params.Item.Size,
			"amount": map[string]any{
				"currency_code": params.Currency, "value": amount,
				"breakdown": map[string]any{
					"item_total": map[string]any{"currency_code": params.Currency, "value": amount},
				},
			},
			"items": []map[string]any{{
				"name":        "Brand X Button Down Oxford Shirt size " + params.Item.Size,
				"sku":         params.Item.SKU,
				"quantity":    "1",
				"unit_amount": map[string]any{"currency_code": params.Currency, "value": amount},
			}},
		}},
		"payment_source": map[string]any{"paypal": map[string]any{
			"return_url": "https://example.com/return",
			"cancel_url": "https://example.com/cancel",
		}},
	}
	status, parsed, err := t.c.post(ctx, "/v2/checkout/orders", params.RequestID, body)
	if err != nil {
		return orders.CreateResult{}, err
	}
	if status/100 != 2 {
		detail := ""
		if parsed != nil {
			if raw, jerr := json.Marshal(parsed); jerr == nil {
				detail = string(raw)
				if len(detail) > 300 {
					detail = detail[:300]
				}
			}
		}
		return orders.CreateResult{}, fmt.Errorf("demo: create order status %d: %s", status, detail)
	}
	id, _ := parsed["id"].(string)
	orderStatus, _ := parsed["status"].(string)
	if id == "" {
		return orders.CreateResult{}, fmt.Errorf("demo: create returned no order id")
	}
	for _, link := range toSlice(parsed["links"]) {
		linkMap, _ := link.(map[string]any)
		if linkMap["rel"] == "payer-action" {
			href, _ := linkMap["href"].(string)
			t.approve[id] = href
		}
	}
	if t.approve[id] == "" {
		return orders.CreateResult{}, fmt.Errorf("demo: create returned no approval url")
	}
	return orders.CreateResult{OrderID: id, Status: orderStatus}, nil
}

// CaptureOrder captures an approved order, idempotent on requestID. A
// payment-source approval mismatch wraps orders.ErrNoApproval, so the
// service marks the order failed exactly as the frozen rule requires.
func (t *restTransport) CaptureOrder(ctx context.Context, orderID, requestID string) (orders.CaptureResult, error) {
	status, parsed, err := t.c.post(ctx, "/v2/checkout/orders/"+orderID+"/capture", requestID, nil)
	if err != nil {
		return orders.CaptureResult{}, err
	}
	if status == 422 {
		return orders.CaptureResult{}, fmt.Errorf("demo: capture refused: %w", orders.ErrNoApproval)
	}
	if status/100 != 2 {
		return orders.CaptureResult{}, fmt.Errorf("demo: capture status %d", status)
	}
	captureID := ""
	for _, pu := range toSlice(parsed["purchase_units"]) {
		puMap, _ := pu.(map[string]any)
		payments, _ := puMap["payments"].(map[string]any)
		for _, cap := range toSlice(payments["captures"]) {
			capMap, _ := cap.(map[string]any)
			id, _ := capMap["id"].(string)
			st, _ := capMap["status"].(string)
			if id != "" && st == "COMPLETED" {
				captureID = id
			}
		}
	}
	if captureID == "" {
		return orders.CaptureResult{}, fmt.Errorf("demo: capture recorded no COMPLETED capture")
	}
	return orders.CaptureResult{CaptureID: captureID, Status: "COMPLETED"}, nil
}

// approvalMirror records the shopper approval on the payment transport:
// it reads the source-side status and records only an APPROVED order,
// so the local record and the payment source agree before any capture.
type approvalMirror struct {
	c        *restClient
	approved map[string]bool
}

// MarkApproved implements loops.ApprovalMirror.
func (m *approvalMirror) MarkApproved(orderID string) {
	got, err := m.c.get(context.Background(), "/v2/checkout/orders/"+orderID)
	if err != nil {
		fail(fmt.Errorf("mirror read order: %w", err))
	}
	status, _ := got["status"].(string)
	if status != "APPROVED" {
		fail(fmt.Errorf("mirror: order status %q, want APPROVED: the shopper approval is not on the payment source", status))
	}
	m.approved[orderID] = true
}

// toSlice coerces a parsed JSON value to a slice.
func toSlice(value any) []any {
	items, _ := value.([]any)
	return items
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

// runApprove shells out to the scripted shopper approval for one
// approval URL, then polls the order to APPROVED. Credentials travel
// only in the browser session and the environment; nothing secret is
// printed, and screenshots land in out/ (git-ignored).
func runApprove(ctx context.Context, c *restClient, out, approvalURL, orderID string) {
	// The sandbox sometimes answers a scripted login with a transient
	// RETRY error page instead of completing (seen in T0.4 as a risk
	// throttle that a short wait clears). The approval is attempted
	// twice with a poll between them before the run fails: the order
	// stays PAYER_ACTION_REQUIRED throughout, so no money can move on a
	// partial approval.
	for attempt := 1; attempt <= 2; attempt++ {
		cmd := exec.CommandContext(ctx, "python3", "tests/live/seed/approve.py")
		cmd.Env = append(os.Environ(), "OUT="+out, "APPROVAL_URL="+approvalURL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fail(fmt.Errorf("scripted approval: %w", err))
		}
		deadline := time.Now().Add(4 * time.Minute)
		for i := 0; ; i++ {
			got, err := c.get(ctx, "/v2/checkout/orders/"+orderID)
			if err != nil {
				fail(fmt.Errorf("poll order: %w", err))
			}
			status, _ := got["status"].(string)
			if i%4 == 0 {
				say("approval poll (attempt %d): order status %s", attempt, status)
			}
			if status == "APPROVED" {
				return
			}
			if status != "PAYER_ACTION_REQUIRED" {
				fail(fmt.Errorf("order status %q, want APPROVED", status))
			}
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(15 * time.Second)
		}
		say("approval attempt %d did not reach APPROVED; retrying the scripted login", attempt)
	}
	fail(fmt.Errorf("order never left PAYER_ACTION_REQUIRED"))
}
