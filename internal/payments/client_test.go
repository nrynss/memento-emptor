package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// goodBody is one delivery body, deliberately carrying spaces after its
// colons: verification covers the exact bytes, so the request must carry
// them unchanged.
const goodBody = `{"id":"WH-TEST-1", "event_type":"PAYMENT.CAPTURE.REFUNDED", "resource": {"amount": {"value":"34.00"}}}`

// stubSource stands in for the payment source's REST API. It records what
// the adapter sent and answers with the configured verdict.
type stubSource struct {
	mu       sync.Mutex
	grants   int
	verifies int
	lists    int
	requests []string
	payloads [][]byte
	// sentHooks names the webhook ids the verify calls carried, in order.
	sentHooks []string
	hooks     []webhook
	// verdicts answers verify calls in order; the last one repeats.
	verdicts []int
	answer   string
	// accept, when set, answers SUCCESS only for these webhook ids.
	accept map[string]bool
}

// start runs the stub on a test server and returns its root URL.
func (s *stubSource) start(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(TokenPath, func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.grants++
		s.mu.Unlock()
		_, _ = w.Write([]byte(`{"access_token":"token-1","expires_in":3600}`))
	})
	mux.HandleFunc(WebhooksPath, func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.lists++
		s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"webhooks": s.hooks})
	})
	mux.HandleFunc(VerifyPath, func(w http.ResponseWriter, r *http.Request) {
		raw := new(bytes.Buffer)
		_, _ = raw.ReadFrom(r.Body)
		var probe struct {
			WebhookID string `json:"webhook_id"`
		}
		_ = json.Unmarshal(raw.Bytes(), &probe)
		s.mu.Lock()
		s.verifies++
		s.requests = append(s.requests, r.Header.Get("Authorization"))
		s.payloads = append(s.payloads, raw.Bytes())
		s.sentHooks = append(s.sentHooks, probe.WebhookID)
		status := http.StatusOK
		if len(s.verdicts) > 0 {
			status = s.verdicts[0]
			if len(s.verdicts) > 1 {
				s.verdicts = s.verdicts[1:]
			}
		}
		answer := s.answer
		if s.accept != nil {
			answer = "FAILURE"
			if s.accept[probe.WebhookID] {
				answer = "SUCCESS"
			}
		}
		s.mu.Unlock()
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = fmt.Fprintf(w, `{"verification_status":%q}`, answer)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

// delivery builds one delivery with the five signature headers.
func delivery(body string) Delivery {
	header := http.Header{}
	header.Set(headerTransmissionID, "transmission-1")
	header.Set(headerTransmissionTime, "2026-11-10T06:57:39Z")
	header.Set(headerCertURL, "https://api.sandbox.paypal.com/v1/notifications/certs/CERT-TEST")
	header.Set(headerAuthAlgo, "SHA256withRSA")
	header.Set(headerTransmissionSig, "signature-1")
	return Delivery{Header: header, Body: []byte(body)}
}

// client builds a client over the stub with a named subscription.
func client(t *testing.T, stub *stubSource) *Client {
	t.Helper()
	return New(
		WithBaseURL(stub.start(t)),
		WithCredentials("client-id", "client-secret"),
		WithWebhookID("WEBHOOK-TEST-1"),
	)
}

// TestVerifiedDeliverySendsTheExactBytes proves a good delivery verifies
// and that the verify request carries every transmission field and the
// body byte for byte.
func TestVerifiedDeliverySendsTheExactBytes(t *testing.T) {
	stub := &stubSource{answer: "SUCCESS"}
	c := client(t, stub)
	if err := c.Verify(context.Background(), delivery(goodBody)); err != nil {
		t.Fatalf("good delivery refused: %v", err)
	}
	if stub.verifies != 1 {
		t.Fatalf("verify calls = %d, want 1", stub.verifies)
	}
	payload := stub.payloads[0]
	if !bytes.Contains(payload, []byte(goodBody)) {
		t.Error("verify request does not carry the delivered body byte for byte")
	}
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(payload, &sent); err != nil {
		t.Fatalf("verify request is not JSON: %v", err)
	}
	want := map[string]string{
		"transmission_id":   `"transmission-1"`,
		"transmission_time": `"2026-11-10T06:57:39Z"`,
		"cert_url":          `"https://api.sandbox.paypal.com/v1/notifications/certs/CERT-TEST"`,
		"auth_algo":         `"SHA256withRSA"`,
		"transmission_sig":  `"signature-1"`,
		"webhook_id":        `"WEBHOOK-TEST-1"`,
	}
	for field, value := range want {
		if string(sent[field]) != value {
			t.Errorf("verify request %s = %s, want %s", field, sent[field], value)
		}
	}
}

// TestFailureVerdictRefuses proves a delivery whose body was tampered
// with is refused with ErrNotVerified.
func TestFailureVerdictRefuses(t *testing.T) {
	stub := &stubSource{answer: "FAILURE"}
	c := client(t, stub)
	err := c.Verify(context.Background(), delivery(goodBody))
	if !errors.Is(err, ErrNotVerified) {
		t.Fatalf("tampered delivery = %v, want ErrNotVerified", err)
	}
}

// TestAnythingButSuccessRefuses proves an empty or unknown verdict is a
// refusal, never a pass.
func TestAnythingButSuccessRefuses(t *testing.T) {
	for _, answer := range []string{"", "PENDING", "success"} {
		stub := &stubSource{answer: answer}
		c := client(t, stub)
		err := c.Verify(context.Background(), delivery(goodBody))
		if !errors.Is(err, ErrNotVerified) {
			t.Errorf("verdict %q = %v, want ErrNotVerified", answer, err)
		}
	}
}

// TestSourceErrorFailsClosed proves an unavailable source refuses the
// delivery instead of opening the gate.
func TestSourceErrorFailsClosed(t *testing.T) {
	stub := &stubSource{verdicts: []int{http.StatusInternalServerError}}
	c := client(t, stub)
	err := c.Verify(context.Background(), delivery(goodBody))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("source error = %v, want ErrUnavailable", err)
	}
}

// TestMalformedDeliveryIsRefusedWithoutCalling proves a delivery missing
// a signature field is refused locally.
func TestMalformedDeliveryIsRefusedWithoutCalling(t *testing.T) {
	stub := &stubSource{answer: "SUCCESS"}
	c := client(t, stub)
	d := delivery(goodBody)
	d.Header.Del(headerTransmissionSig)
	err := c.Verify(context.Background(), d)
	if !errors.Is(err, ErrMalformedDelivery) {
		t.Fatalf("missing signature = %v, want ErrMalformedDelivery", err)
	}
	if stub.verifies != 0 || stub.grants != 0 {
		t.Errorf("malformed delivery reached the source: %d verify calls, %d grants", stub.verifies, stub.grants)
	}

	empty := delivery(goodBody)
	empty.Body = nil
	if err := c.Verify(context.Background(), empty); !errors.Is(err, ErrMalformedDelivery) {
		t.Errorf("empty body = %v, want ErrMalformedDelivery", err)
	}
	bad := delivery("{not json")
	if err := c.Verify(context.Background(), bad); !errors.Is(err, ErrMalformedDelivery) {
		t.Errorf("non-JSON body = %v, want ErrMalformedDelivery", err)
	}
}

// TestMissingCredentialsFailClosed proves an unconfigured verifier refuses
// rather than skip the check.
func TestMissingCredentialsFailClosed(t *testing.T) {
	stub := &stubSource{answer: "SUCCESS"}
	c := New(WithBaseURL(stub.start(t)), WithCredentials("", ""), WithWebhookID("WEBHOOK-TEST-1"))
	err := c.Verify(context.Background(), delivery(goodBody))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing credentials = %v, want ErrUnavailable", err)
	}
	if stub.verifies != 0 {
		t.Errorf("unconfigured verifier called the source %d times", stub.verifies)
	}
}

// TestTokenIsCached proves two deliveries share one access token.
func TestTokenIsCached(t *testing.T) {
	stub := &stubSource{answer: "SUCCESS"}
	c := client(t, stub)
	for range 2 {
		if err := c.Verify(context.Background(), delivery(goodBody)); err != nil {
			t.Fatal(err)
		}
	}
	if stub.grants != 1 {
		t.Errorf("token grants = %d, want 1 cached token", stub.grants)
	}
}

// TestRejectedTokenIsRenewedOnce proves a cached token the source no longer
// honours is renewed, and a second refusal refuses the delivery.
func TestRejectedTokenIsRenewedOnce(t *testing.T) {
	stub := &stubSource{answer: "SUCCESS", verdicts: []int{http.StatusUnauthorized, http.StatusOK}}
	c := client(t, stub)
	if err := c.Verify(context.Background(), delivery(goodBody)); err != nil {
		t.Fatalf("delivery after token renewal refused: %v", err)
	}
	if stub.grants != 2 {
		t.Errorf("token grants = %d, want 2 after one renewal", stub.grants)
	}

	stub = &stubSource{answer: "SUCCESS", verdicts: []int{http.StatusUnauthorized, http.StatusUnauthorized}}
	c = client(t, stub)
	if err := c.Verify(context.Background(), delivery(goodBody)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("renewed token refused again = %v, want ErrUnavailable", err)
	}
	if stub.verifies != 2 {
		t.Errorf("verify calls = %d, want exactly 2 (one renewal, no loop)", stub.verifies)
	}
}

// TestSubscriptionIsDiscovered proves the subscription comes from the
// app's single subscription when none is named.
func TestSubscriptionIsDiscovered(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	stub := &stubSource{answer: "SUCCESS", hooks: []webhook{{ID: "WEBHOOK-ONLY", URL: "https://example.test/webhooks/paypal"}}}
	c := New(WithBaseURL(stub.start(t)), WithCredentials("client-id", "client-secret"))
	if err := c.Verify(context.Background(), delivery(goodBody)); err != nil {
		t.Fatalf("delivery refused: %v", err)
	}
	if stub.lists != 1 {
		t.Errorf("webhook lists = %d, want 1", stub.lists)
	}
	if !bytes.Contains(stub.payloads[0], []byte(`"webhook_id":"WEBHOOK-ONLY"`)) {
		t.Errorf("verify request does not name the discovered subscription: %s", stub.payloads[0])
	}
}

// TestPublicURLNamesTheSubscription proves a configured public URL picks
// the matching subscription out of several.
func TestPublicURLNamesTheSubscription(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://demo.test/")
	stub := &stubSource{answer: "SUCCESS", hooks: []webhook{
		{ID: "WEBHOOK-OLD", URL: "https://old.test/webhooks/paypal"},
		{ID: "WEBHOOK-DEMO", URL: "https://demo.test/webhooks/paypal"},
	}}
	c := New(WithBaseURL(stub.start(t)), WithCredentials("client-id", "client-secret"))
	if err := c.Verify(context.Background(), delivery(goodBody)); err != nil {
		t.Fatalf("delivery refused: %v", err)
	}
	if !bytes.Contains(stub.payloads[0], []byte(`"webhook_id":"WEBHOOK-DEMO"`)) {
		t.Errorf("verify request names the wrong subscription: %s", stub.payloads[0])
	}
}

// TestEverySubscriptionIsTried proves that without a public URL every
// subscription the app has is a candidate, so a delivery keeps verifying
// when a second subscription is added.
func TestEverySubscriptionIsTried(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	stub := &stubSource{accept: map[string]bool{"WEBHOOK-B": true}, hooks: []webhook{
		{ID: "WEBHOOK-A", URL: "https://a.test/webhooks/paypal"},
		{ID: "WEBHOOK-B", URL: "https://b.test/webhooks/paypal"},
	}}
	c := New(WithBaseURL(stub.start(t)), WithCredentials("client-id", "client-secret"))
	if err := c.Verify(context.Background(), delivery(goodBody)); err != nil {
		t.Fatalf("delivery refused: %v", err)
	}
	if len(stub.sentHooks) != 2 || stub.sentHooks[0] != "WEBHOOK-A" || stub.sentHooks[1] != "WEBHOOK-B" {
		t.Errorf("verify calls named %v, want the candidates in order", stub.sentHooks)
	}
}

// TestPublicURLNarrowsToItsSubscription proves a configured public URL
// refuses a delivery signed for another subscription.
func TestPublicURLNarrowsToItsSubscription(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://demo.test")
	stub := &stubSource{accept: map[string]bool{"WEBHOOK-OLD": true}, hooks: []webhook{
		{ID: "WEBHOOK-OLD", URL: "https://old.test/webhooks/paypal"},
		{ID: "WEBHOOK-DEMO", URL: "https://demo.test/webhooks/paypal"},
	}}
	c := New(WithBaseURL(stub.start(t)), WithCredentials("client-id", "client-secret"))
	err := c.Verify(context.Background(), delivery(goodBody))
	if !errors.Is(err, ErrNotVerified) {
		t.Fatalf("delivery signed for another subscription = %v, want ErrNotVerified", err)
	}
	if len(stub.sentHooks) != 1 || stub.sentHooks[0] != "WEBHOOK-DEMO" {
		t.Errorf("verify calls named %v, want only the configured subscription", stub.sentHooks)
	}
}

// TestNoSubscriptionsRefuse proves the adapter refuses to guess when the
// app has no subscription at all.
func TestNoSubscriptionsRefuse(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	stub := &stubSource{answer: "SUCCESS"}
	c := New(WithBaseURL(stub.start(t)), WithCredentials("client-id", "client-secret"))
	err := c.Verify(context.Background(), delivery(goodBody))
	if !errors.Is(err, ErrNoWebhook) {
		t.Fatalf("no subscriptions = %v, want ErrNoWebhook", err)
	}
	if stub.verifies != 0 {
		t.Errorf("no subscription still verified: %d calls", stub.verifies)
	}
}

// TestRefusalNamesNoCredential proves errors never carry the secret.
func TestRefusalNamesNoCredential(t *testing.T) {
	stub := &stubSource{answer: "FAILURE"}
	c := New(WithBaseURL(stub.start(t)), WithCredentials("client-id", "super-secret-value"), WithWebhookID("WEBHOOK-TEST-1"))
	err := c.Verify(context.Background(), delivery(goodBody))
	if err == nil {
		t.Fatal("failed delivery verified")
	}
	if strings.Contains(err.Error(), "super-secret-value") || strings.Contains(err.Error(), "client-id") {
		t.Errorf("refusal names a credential: %v", err)
	}
}

// TestCancelledContextIsRefused proves a cancelled request refuses the
// delivery rather than storing it.
func TestCancelledContextIsRefused(t *testing.T) {
	stub := &stubSource{answer: "SUCCESS"}
	c := client(t, stub)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Verify(ctx, delivery(goodBody)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled verification = %v, want context.Canceled", err)
	}
	if stub.verifies != 0 {
		t.Errorf("cancelled verification called the source %d times", stub.verifies)
	}
}
