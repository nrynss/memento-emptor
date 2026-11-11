package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/payments"
)

// realSourcePayPal serves the recorded PayPal shapes the real source reads.
// It is the offline half of the live source: the adapter is the production
// one, pointed at a stand-in source instead of the sandbox.
type realSourcePayPal struct {
	mu     sync.Mutex
	paths  map[string][]byte
	counts map[string]int
}

// newRealSourcePayPal builds the stand-in from the repository's fixtures.
func newRealSourcePayPal(t *testing.T) *realSourcePayPal {
	t.Helper()
	read := func(name string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "paypal", name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		return raw
	}
	var refundEvent, disputeEvent, invoiceEvent map[string]any
	for name, out := range map[string]*map[string]any{
		"webhook-delivery-refunded.json":     &refundEvent,
		"webhook-delivery-dispute.json":      &disputeEvent,
		"webhook-delivery-invoice-sent.json": &invoiceEvent,
	} {
		var file struct {
			Body map[string]any `json:"body"`
		}
		if err := json.Unmarshal(read(name), &file); err != nil {
			t.Fatalf("parse fixture %s: %v", name, err)
		}
		*out = file.Body
	}
	encode := func(value any) []byte {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("encode fixture: %v", err)
		}
		return raw
	}
	capture := read("capture-get.json")
	captureEvent := encode(map[string]any{
		"id":         "WH-CAPTURE-LIVE",
		"event_type": payments.EventCaptureCompleted,
		"resource":   json.RawMessage(capture),
	})
	var order map[string]any
	if err := json.Unmarshal(read("order-get-approved-line-items.json"), &order); err != nil {
		t.Fatalf("parse order fixture: %v", err)
	}
	order2 := make(map[string]any, len(order))
	for key, value := range order {
		order2[key] = value
	}
	order2["id"] = "ORDER-ID-US-2"

	return &realSourcePayPal{
		paths: map[string][]byte{
			payments.TokenPath: []byte(`{"access_token":"token-1","expires_in":3600}`),
			payments.WebhookEventsPath + "WH-CAPTURE-LIVE":    captureEvent,
			payments.WebhookEventsPath + "WEBHOOK-EVENT-US-1": encode(refundEvent),
			payments.OrdersPath + "ORDER-ID-US-1":             read("order-get-approved-line-items.json"),
			payments.OrdersPath + "ORDER-ID-US-2":             encode(order2),
			payments.CapturesPath + "CAPTURE-ID-US-1":         capture,
			payments.CapturesPath + "CAPTURE-ID-US-2":         read("capture2-get.json"),
			payments.RefundsPath + "REFUND-ID-US-2":           encode(refundEvent["resource"]),
			payments.WebhookEventsPath + "WEBHOOK-EVENT-ID-1": encode(invoiceEvent),
		},
		counts: map[string]int{},
	}
}

// start runs the stand-in and returns its root URL.
func (s *realSourcePayPal) start(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.counts[r.URL.Path]++
		raw, ok := s.paths[r.URL.Path]
		s.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

// reads returns how many times one path was read.
func (s *realSourcePayPal) reads(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts[path]
}

// countFit records one fit row per call.
type countFit struct {
	mu   sync.Mutex
	rows int
}

// Record counts one fit row.
func (f *countFit) Record(context.Context, domain.Evidence) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows++
	return nil
}

// rowsRead returns how many fit rows were written.
func (f *countFit) rowsRead() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rows
}

// TestRealSourceIngestsOnce proves the production source maps a captured
// and a refunded delivery into rows and fit rows, and that a redelivery of
// the same event writes nothing new and reads the source once.
func TestRealSourceIngestsOnce(t *testing.T) {
	paypal := newRealSourcePayPal(t)
	source := payments.New(
		payments.WithBaseURL(paypal.start(t)),
		payments.WithCredentials("client-id", "client-secret"),
		payments.WithCatalog(payments.DemoCatalog()),
	)
	fit := &countFit{}
	service := NewService(source, WithFitRecorder(fit), WithClock(fixedClock()))
	ctx := context.Background()

	capture, err := service.Handle(ctx, "WH-CAPTURE-LIVE")
	if err != nil {
		t.Fatalf("capture delivery: %v", err)
	}
	if capture.Duplicate || len(capture.Rows) != 1 {
		t.Fatalf("capture = %+v, want one new row", capture)
	}
	if capture.Rows[0].Link.OrderID != "ORDER-ID-US-1" || capture.Rows[0].Outcome.Kind != domain.OutcomeCapture {
		t.Errorf("capture row = %+v, want the captured order", capture.Rows[0])
	}
	if fit.rowsRead() != 1 {
		t.Errorf("capture wrote %d fit rows, want 1", fit.rowsRead())
	}
	again, err := service.Handle(ctx, "WH-CAPTURE-LIVE")
	if err != nil {
		t.Fatalf("capture redelivery: %v", err)
	}
	if !again.Duplicate || len(service.Rows()) != 1 || fit.rowsRead() != 1 {
		t.Errorf("capture redelivery = duplicate %v, rows %d, fit %d; want a duplicate with nothing written",
			again.Duplicate, len(service.Rows()), fit.rowsRead())
	}
	if reads := paypal.reads(payments.WebhookEventsPath + "WH-CAPTURE-LIVE"); reads != 1 {
		t.Errorf("capture event read %d times, want once: a duplicate is answered before the source", reads)
	}

	refund, err := service.Handle(ctx, "WEBHOOK-EVENT-US-1")
	if err != nil {
		t.Fatalf("refund delivery: %v", err)
	}
	if refund.Duplicate || len(refund.Rows) != 1 || refund.Pending == nil {
		t.Fatalf("refund = %+v, want one new row and a pending question", refund)
	}
	if refund.Rows[0].Link.OrderID != "ORDER-ID-US-2" || refund.Rows[0].Outcome.Kind != domain.OutcomeRefund {
		t.Errorf("refund row = %+v, want the refunded order", refund.Rows[0])
	}
	if fit.rowsRead() != 2 {
		t.Errorf("refund wrote %d fit rows in total, want 2", fit.rowsRead())
	}
	again, err = service.Handle(ctx, "WEBHOOK-EVENT-US-1")
	if err != nil {
		t.Fatalf("refund redelivery: %v", err)
	}
	if !again.Duplicate || len(service.Rows()) != 2 || len(service.PendingQuestions()) != 1 || fit.rowsRead() != 2 {
		t.Errorf("refund redelivery = duplicate %v, rows %d, questions %d, fit %d; want nothing new",
			again.Duplicate, len(service.Rows()), len(service.PendingQuestions()), fit.rowsRead())
	}
}

// TestRealSourceRefusesUnknownEvent proves an event type the source does not
// map is refused with the source's own sentinel, distinct from ingest's
// unknown-outcome sentinel, and stores nothing.
func TestRealSourceRefusesUnknownEvent(t *testing.T) {
	paypal := newRealSourcePayPal(t)
	source := payments.New(
		payments.WithBaseURL(paypal.start(t)),
		payments.WithCredentials("client-id", "client-secret"),
	)
	service := NewService(source, WithClock(fixedClock()))
	_, err := service.Handle(context.Background(), "WEBHOOK-EVENT-ID-1")
	if !errors.Is(err, payments.ErrUnknownEvent) {
		t.Fatalf("unknown event = %v, want payments.ErrUnknownEvent", err)
	}
	if errors.Is(err, ErrUnknownEvent) {
		t.Errorf("unknown event also matches ingest.ErrUnknownEvent: %v", err)
	}
	if len(service.Rows()) != 0 {
		t.Errorf("unknown event stored %d rows", len(service.Rows()))
	}
}

// TestRealSourceDeliveryPathVerifiesThenReads proves the deployed path's
// order: the delivery gate runs first, then the real source resolves the
// verified event, and a redelivery is answered as a duplicate.
func TestRealSourceDeliveryPathVerifiesThenReads(t *testing.T) {
	paypal := newRealSourcePayPal(t)
	source := payments.New(
		payments.WithBaseURL(paypal.start(t)),
		payments.WithCredentials("client-id", "client-secret"),
		payments.WithCatalog(payments.DemoCatalog()),
	)
	verifier := &recordedVerifier{verdicts: []error{nil, nil}}
	service := NewService(source, WithVerifier(verifier), WithClock(fixedClock()))
	ctx := context.Background()
	delivery := Delivery{Body: []byte(`{"id":"WH-CAPTURE-LIVE"}`)}

	first, err := service.HandleDelivery(ctx, delivery)
	if err != nil {
		t.Fatalf("verified delivery: %v", err)
	}
	if first.Duplicate || len(first.Rows) != 1 {
		t.Fatalf("verified delivery = %+v, want one row", first)
	}
	again, err := service.HandleDelivery(ctx, delivery)
	if err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if !again.Duplicate || len(service.Rows()) != 1 {
		t.Errorf("redelivery = duplicate %v, rows %d; want a duplicate with one row", again.Duplicate, len(service.Rows()))
	}
	if len(verifier.seen) != 2 {
		t.Errorf("verifier checked %d deliveries, want 2: a redelivery is verified too", len(verifier.seen))
	}
}

// TestRealSourceMalformedEventFailsImport proves a source record without
// the fields the layer needs is refused rather than stored half-formed.
func TestRealSourceMalformedEventFailsImport(t *testing.T) {
	paypal := newRealSourcePayPal(t)
	paypal.paths[payments.WebhookEventsPath+"WH-BROKEN"] = []byte(`{"id":"WH-BROKEN","event_type":"PAYMENT.CAPTURE.REFUNDED","resource":{}}`)
	source := payments.New(
		payments.WithBaseURL(paypal.start(t)),
		payments.WithCredentials("client-id", "client-secret"),
	)
	service := NewService(source, WithClock(fixedClock()))
	_, err := service.Handle(context.Background(), "WH-BROKEN")
	if !errors.Is(err, payments.ErrMalformedRecord) {
		t.Fatalf("broken refund = %v, want payments.ErrMalformedRecord", err)
	}
	if len(service.Rows()) != 0 {
		t.Errorf("broken refund stored %d rows", len(service.Rows()))
	}
}
