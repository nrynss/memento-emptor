package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// fixtureDir holds the recorded PayPal shapes the reads replay. The tests
// read the repository's fixtures rather than copies, so a shape change is
// seen here.
const fixtureDir = "../../testdata/paypal"

// readFixture reads one recorded PayPal fixture.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

// fixtureJSON reads one fixture as a generic object.
func fixtureJSON(t *testing.T, name string) map[string]any {
	t.Helper()
	raw := readFixture(t, name)
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	return parsed
}

// fixtureEventBody reads one recorded delivery's body, which is the event
// GET's shape.
func fixtureEventBody(t *testing.T, name string) []byte {
	t.Helper()
	file := fixtureJSON(t, name)
	body, ok := file["body"]
	if !ok {
		t.Fatalf("fixture %s carries no delivery body", name)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode fixture %s body: %v", name, err)
	}
	return raw
}

// captureFromOrder returns one capture nested in a captured order.
func captureFromOrder(t *testing.T, name string) []byte {
	t.Helper()
	order := fixtureJSON(t, name)
	units, _ := order["purchase_units"].([]any)
	for _, unit := range units {
		entry, _ := unit.(map[string]any)
		payments, _ := entry["payments"].(map[string]any)
		captures, _ := payments["captures"].([]any)
		if len(captures) > 0 {
			raw, err := json.Marshal(captures[0])
			if err != nil {
				t.Fatalf("encode capture from %s: %v", name, err)
			}
			return raw
		}
	}
	t.Fatalf("fixture %s carries no capture", name)
	return nil
}

// orderWithID returns one recorded order under a different id, so the
// read's order can match the link that named it.
func orderWithID(t *testing.T, name, orderID string) []byte {
	t.Helper()
	order := fixtureJSON(t, name)
	order["id"] = orderID
	raw, err := json.Marshal(order)
	if err != nil {
		t.Fatalf("encode order %s: %v", orderID, err)
	}
	return raw
}

// captureEvent builds the event GET's envelope around a recorded capture
// resource.
func captureEvent(t *testing.T, eventID string, capture []byte) []byte {
	t.Helper()
	raw := append([]byte(`{"id":`), []byte(fmt.Sprintf("%q", eventID))...)
	raw = append(raw, `,"event_type":"PAYMENT.CAPTURE.COMPLETED","resource":`...)
	raw = append(raw, capture...)
	raw = append(raw, '}')
	return raw
}

// readSource stands in for the sandbox REST reads. Every path is an exact
// recorded fixture; an unlisted path is the source's 404.
type readSource struct {
	t       *testing.T
	server  *httptest.Server
	paths   map[string][]byte
	mu      sync.Mutex
	status  map[string]int
	repeats map[string]int
}

// newReadSource starts a stand-in source over the recorded fixtures.
func newReadSource(t *testing.T) *readSource {
	t.Helper()
	refundEvent := fixtureEventBody(t, "webhook-delivery-refunded.json")
	disputeEvent := fixtureEventBody(t, "webhook-delivery-dispute.json")
	invoiceEvent := fixtureEventBody(t, "webhook-delivery-invoice-sent.json")
	capture := readFixture(t, "capture-get.json")
	capture2 := readFixture(t, "capture2-get.json")
	capture3 := captureFromOrder(t, "capture3-completed.json")
	refund2 := func() []byte {
		file := fixtureJSON(t, "webhook-delivery-refunded.json")
		resource, _ := file["body"].(map[string]any)["resource"]
		raw, err := json.Marshal(resource)
		if err != nil {
			t.Fatalf("encode refund 2: %v", err)
		}
		return raw
	}()

	source := &readSource{
		t:       t,
		paths:   map[string][]byte{},
		status:  map[string]int{},
		repeats: map[string]int{},
	}
	source.paths[TokenPath] = []byte(`{"access_token":"token-1","expires_in":3600}`)
	source.paths[OrdersPath+"ORDER-ID-US-1"] = readFixture(t, "order-get-approved-line-items.json")
	source.paths[OrdersPath+"ORDER-ID-US-2"] = orderWithID(t, "order-get-approved-line-items.json", "ORDER-ID-US-2")
	source.paths[OrdersPath+"ORDER-ID-US-3"] = readFixture(t, "capture3-completed.json")
	source.paths[CapturesPath+"CAPTURE-ID-US-1"] = capture
	source.paths[CapturesPath+"CAPTURE-ID-US-2"] = capture2
	source.paths[CapturesPath+"CAPTURE-ID-US-3"] = capture3
	source.paths[RefundsPath+"REFUND-ID-US-1"] = readFixture(t, "refund-record.json")
	source.paths[RefundsPath+"REFUND-ID-US-2"] = refund2
	source.paths[DisputesPath+"DISPUTE-ID-SANDBOX-1"] = readFixture(t, "dispute-get.json")
	source.paths[WebhookEventsPath+"WEBHOOK-EVENT-US-1"] = refundEvent
	source.paths[WebhookEventsPath+"WEBHOOK-EVENT-DISPUTE-1"] = disputeEvent
	source.paths[WebhookEventsPath+"WEBHOOK-EVENT-ID-1"] = invoiceEvent
	source.paths[WebhookEventsPath+"WH-CAPTURE-1"] = captureEvent(t, "WH-CAPTURE-1", capture)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		source.mu.Lock()
		source.repeats[r.URL.Path]++
		status, hasStatus := source.status[r.URL.Path]
		raw, ok := source.paths[r.URL.Path]
		source.mu.Unlock()
		if hasStatus && status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == TokenPath {
			w.Header().Set("Content-Type", "application/json")
		}
		_, _ = w.Write(raw)
	})
	source.server = httptest.NewServer(mux)
	t.Cleanup(source.server.Close)
	return source
}

// client builds the read source over the stand-in.
func (s *readSource) client(t *testing.T, opts ...Option) *Client {
	t.Helper()
	all := []Option{
		WithBaseURL(s.server.URL),
		WithCredentials("client-id", "client-secret"),
		WithCatalog(DemoCatalog()),
	}
	all = append(all, opts...)
	return New(all...)
}

// setStatus changes one path's answer between calls.
func (s *readSource) setStatus(path string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status[path] = status
}

// TestGetOrderMapsRecordedLineItems reads the recorded approved order and
// checks the line item is joined through the demo catalog.
func TestGetOrderMapsRecordedLineItems(t *testing.T) {
	source := newReadSource(t)
	order, err := source.client(t).GetOrder(context.Background(), "ORDER-ID-US-1")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.OrderID != "ORDER-ID-US-1" {
		t.Errorf("order id = %q", order.OrderID)
	}
	if order.Link.CustomID != "shopper-demo-001" {
		t.Errorf("custom id = %q, want shopper-demo-001", order.Link.CustomID)
	}
	if len(order.Items) != 1 {
		t.Fatalf("order carries %d items, want 1", len(order.Items))
	}
	item := order.Items[0]
	want := domain.Item{
		BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt",
		SKU: "BRX-OXF-M", Size: "M", Category: "shirts",
	}
	if item != want {
		t.Errorf("item = %+v, want %+v", item, want)
	}
}

// TestGetCaptureMapsRecordedCapture reads the recorded capture and checks
// its order join and outcome.
func TestGetCaptureMapsRecordedCapture(t *testing.T) {
	source := newReadSource(t)
	capture, err := source.client(t).GetCapture(context.Background(), "CAPTURE-ID-US-1")
	if err != nil {
		t.Fatalf("get capture: %v", err)
	}
	if capture.CaptureID != "CAPTURE-ID-US-1" || capture.OrderID != "ORDER-ID-US-1" {
		t.Errorf("capture = %+v, want CAPTURE-ID-US-1 on ORDER-ID-US-1", capture)
	}
	if capture.CustomID != "shopper-demo-001" {
		t.Errorf("custom id = %q, want shopper-demo-001", capture.CustomID)
	}
	want := domain.Outcome{Kind: domain.OutcomeCapture, Status: "COMPLETED", AmountMinor: 3400, Currency: "USD"}
	if capture.Outcome != want {
		t.Errorf("outcome = %+v, want %+v", capture.Outcome, want)
	}
}

// TestGetRefundMapsRecordedRefund reads the recorded refund and checks it
// carries no reason and joins its capture and order.
func TestGetRefundMapsRecordedRefund(t *testing.T) {
	source := newReadSource(t)
	refund, err := source.client(t).GetRefund(context.Background(), "REFUND-ID-US-1")
	if err != nil {
		t.Fatalf("get refund: %v", err)
	}
	if refund.RefundID != "REFUND-ID-US-1" || refund.HasReason {
		t.Errorf("refund = %+v, want REFUND-ID-US-1 with no reason", refund)
	}
	if refund.Link.OrderID != "ORDER-ID-US-1" || refund.Link.CaptureID != "CAPTURE-ID-US-1" {
		t.Errorf("refund link = %+v, want its capture and order", refund.Link)
	}
	if refund.Link.CustomID != "shopper-demo-001" {
		t.Errorf("custom id = %q, want shopper-demo-001", refund.Link.CustomID)
	}
	want := domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 3400, Currency: "USD"}
	if refund.Outcome != want {
		t.Errorf("outcome = %+v, want %+v", refund.Outcome, want)
	}
}

// TestGetDisputeMapsRecordedDispute reads the recorded dispute and checks
// its reason code, category, item and transaction links.
func TestGetDisputeMapsRecordedDispute(t *testing.T) {
	source := newReadSource(t)
	dispute, err := source.client(t).GetDispute(context.Background(), "DISPUTE-ID-SANDBOX-1")
	if err != nil {
		t.Fatalf("get dispute: %v", err)
	}
	if dispute.DisputeID != "DISPUTE-ID-SANDBOX-1" {
		t.Errorf("dispute id = %q", dispute.DisputeID)
	}
	if dispute.Code != domain.DisputeMerchandiseNotReceived || dispute.Category != domain.CategoryNotArrived {
		t.Errorf("dispute code/category = %q/%q, want not-received/not_arrived", dispute.Code, dispute.Category)
	}
	if dispute.Link.OrderID != "ORDER-ID-US-3" || dispute.Link.CaptureID != "CAPTURE-ID-US-3" {
		t.Errorf("dispute link = %+v, want its capture and order", dispute.Link)
	}
	if dispute.Item.SKU != "BRX-OXF-M" || dispute.Item.BrandID != "brand-x-001" || dispute.Item.Size != "M" {
		t.Errorf("dispute item = %+v, want the catalog's Brand X oxford in M", dispute.Item)
	}
}

// TestVerifyWebhookMapsRefund maps a recorded refund delivery to the
// layer's event shape, including the order reached through its capture.
func TestVerifyWebhookMapsRefund(t *testing.T) {
	source := newReadSource(t)
	event, err := source.client(t).VerifyWebhook(context.Background(), "WEBHOOK-EVENT-US-1")
	if err != nil {
		t.Fatalf("verify webhook: %v", err)
	}
	if !event.Verified || event.EventType != EventCaptureRefunded {
		t.Errorf("event = %+v, want a verified refund", event)
	}
	if event.Outcome.Kind != domain.OutcomeRefund || event.Outcome.AmountMinor != 3400 || event.Outcome.Status != "COMPLETED" {
		t.Errorf("outcome = %+v, want a completed 34.00 refund", event.Outcome)
	}
	if event.Link.OrderID != "ORDER-ID-US-2" || event.Link.CaptureID != "CAPTURE-ID-US-2" || event.Link.RefundID != "REFUND-ID-US-2" {
		t.Errorf("link = %+v, want the refund's capture and order", event.Link)
	}
	if event.Link.CustomID != "shopper-demo-001" {
		t.Errorf("custom id = %q, want shopper-demo-001", event.Link.CustomID)
	}
}

// TestVerifyWebhookMapsCapture maps a capture delivery built around the
// recorded capture shape.
func TestVerifyWebhookMapsCapture(t *testing.T) {
	source := newReadSource(t)
	event, err := source.client(t).VerifyWebhook(context.Background(), "WH-CAPTURE-1")
	if err != nil {
		t.Fatalf("verify webhook: %v", err)
	}
	if event.Outcome.Kind != domain.OutcomeCapture || event.Outcome.AmountMinor != 3400 {
		t.Errorf("outcome = %+v, want a completed 34.00 capture", event.Outcome)
	}
	if event.Link.OrderID != "ORDER-ID-US-1" || event.Link.CaptureID != "CAPTURE-ID-US-1" {
		t.Errorf("link = %+v, want the capture and its order", event.Link)
	}
	if event.Link.CustomID != "shopper-demo-001" {
		t.Errorf("custom id = %q, want shopper-demo-001", event.Link.CustomID)
	}
}

// TestVerifyWebhookMapsDispute maps a recorded dispute delivery to the
// layer's event shape, reason code included.
func TestVerifyWebhookMapsDispute(t *testing.T) {
	source := newReadSource(t)
	event, err := source.client(t).VerifyWebhook(context.Background(), "WEBHOOK-EVENT-DISPUTE-1")
	if err != nil {
		t.Fatalf("verify webhook: %v", err)
	}
	if event.Outcome.Kind != domain.OutcomeDispute || event.Outcome.AmountMinor != 3400 {
		t.Errorf("outcome = %+v, want a 34.00 dispute", event.Outcome)
	}
	if event.DisputeCode != domain.DisputeMerchandiseNotReceived {
		t.Errorf("dispute code = %q, want not-received", event.DisputeCode)
	}
	if event.Link.DisputeID != "DISPUTE-ID-SANDBOX-1" || event.Link.CaptureID != "CAPTURE-ID-US-3" {
		t.Errorf("link = %+v, want the dispute and its capture", event.Link)
	}
	if event.Link.OrderID != "ORDER-ID-US-3" {
		t.Errorf("order = %q, want ORDER-ID-US-3", event.Link.OrderID)
	}
}

// TestVerifyWebhookRefusesUnknownEvent proves an event type the layer does
// not store is refused with its own sentinel, not mapped into a row.
func TestVerifyWebhookRefusesUnknownEvent(t *testing.T) {
	source := newReadSource(t)
	_, err := source.client(t).VerifyWebhook(context.Background(), "WEBHOOK-EVENT-ID-1")
	if !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("unknown event = %v, want ErrUnknownEvent", err)
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrMalformedRecord) {
		t.Errorf("unknown event also matches a read sentinel: %v", err)
	}
}

// TestReadsMapSourceAnswers proves a missing record, a refused credential
// and a source failure each keep their own sentinel.
func TestReadsMapSourceAnswers(t *testing.T) {
	source := newReadSource(t)
	client := source.client(t)
	if _, err := client.GetOrder(context.Background(), "ORDER-NOT-THERE"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing order = %v, want ErrNotFound", err)
	}
	source.setStatus(OrdersPath+"ORDER-ID-US-1", http.StatusUnauthorized)
	if _, err := client.GetOrder(context.Background(), "ORDER-ID-US-1"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("refused order = %v, want ErrUnauthorized", err)
	}
	source.mu.Lock()
	source.status[OrdersPath+"ORDER-ID-US-1"] = http.StatusInternalServerError
	source.mu.Unlock()
	if _, err := client.GetOrder(context.Background(), "ORDER-ID-US-1"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("failed order = %v, want ErrUnavailable", err)
	}
}

// TestReadsAreBounded proves a read that hangs is cut off by the client's
// timeout instead of holding the delivery forever.
func TestReadsAreBounded(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(slow.Close)
	client := New(
		WithBaseURL(slow.URL),
		WithCredentials("client-id", "client-secret"),
		WithHTTPClient(&http.Client{Timeout: 20 * time.Millisecond}),
	)
	start := time.Now()
	_, err := client.GetOrder(context.Background(), "ORDER-SLOW")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("slow read = %v, want ErrUnavailable", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("slow read took %s, want a bounded failure", elapsed)
	}
}

// TestReadRefusalNamesNoCredential proves a failed read never carries the
// app's credentials.
func TestReadRefusalNamesNoCredential(t *testing.T) {
	source := newReadSource(t)
	client := source.client(t, WithCredentials("client-id-secret", "super-secret-value"))
	_, err := client.GetOrder(context.Background(), "ORDER-NOT-THERE")
	if err == nil {
		t.Fatal("missing order answered no error")
	}
	if strings.Contains(err.Error(), "super-secret-value") || strings.Contains(err.Error(), "client-id-secret") {
		t.Errorf("refusal names a credential: %v", err)
	}
}

// TestDemoCatalogMapsAndFallsBack proves the demo catalog joins its own
// SKUs and the source's fallback still maps a line the catalog never saw.
func TestDemoCatalogMapsAndFallsBack(t *testing.T) {
	catalog := DemoCatalog()
	item, ok := catalog.Item(LineItem{Name: "Brand X oxford shirt", SKU: "BRX-OXF-M"})
	if !ok {
		t.Fatal("demo catalog does not know BRX-OXF-M")
	}
	if item.BrandID != "brand-x-001" || item.Size != "M" || item.Category != "shirts" {
		t.Errorf("demo item = %+v, want Brand X/shirts/M", item)
	}
	if _, ok := catalog.Item(LineItem{Name: "Acme Field Jacket", SKU: "ACM-JKT-L"}); ok {
		t.Error("demo catalog claims an unknown SKU")
	}
	derived := derivedItem(LineItem{Name: "Acme Field Jacket size L", SKU: "ACM-JKT-L", Category: "PHYSICAL_GOODS", Size: "L"})
	if derived.BrandID != "acme" || derived.Size != "L" || derived.Category != "physical-goods" {
		t.Errorf("derived item = %+v, want a stable Acme line", derived)
	}
}

// TestAmountParserRejectsSubCentAndNegative proves a source amount that
// cannot be minor units is refused rather than rounded.
func TestAmountParserRejectsSubCentAndNegative(t *testing.T) {
	for _, value := range []string{"", "-1.00", "1.001", "not-a-number"} {
		if _, err := (amount{Value: value}).minor(); !errors.Is(err, ErrMalformedRecord) {
			t.Errorf("amount %q = %v, want ErrMalformedRecord", value, err)
		}
	}
	got, err := (amount{Value: "195"}).minor()
	if err != nil || got != 19500 {
		t.Errorf("195 = %d, %v, want 19500", got, err)
	}
}
