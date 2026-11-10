package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/payments"
)

// recordedDelivery is one row of the delivered cases recorded against the
// sandbox verify API: the delivery, the verdict the live endpoint gave it,
// and what ingest is to do with it.
type recordedDelivery struct {
	// Name identifies the case.
	Name string `json:"name"`
	// Why states what makes the delivery the case it is.
	Why string `json:"why"`
	// Headers are the transmitted signature headers, placeholders only.
	Headers map[string][]string `json:"headers"`
	// Body is the delivered event, a stand-in keeping the recorded shape.
	Body json.RawMessage `json:"body"`
	// RecordedVerdict is what the live verify endpoint answered.
	RecordedVerdict string `json:"recorded_verdict"`
	// RecordedIngest is what ingest did with the delivery.
	RecordedIngest string `json:"recorded_ingest"`
}

// loadRecordedDeliveries reads the recorded cases beside this package.
func loadRecordedDeliveries(t *testing.T) []recordedDelivery {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "recorded-deliveries.json"))
	if err != nil {
		t.Fatalf("read recorded deliveries: %v", err)
	}
	var file struct {
		// Note explains how the cases were recorded.
		Note string `json:"note"`
		// Deliveries are the recorded cases, in arrival order.
		Deliveries []recordedDelivery `json:"deliveries"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse recorded deliveries: %v", err)
	}
	if file.Note == "" {
		t.Error("recorded deliveries carry no note saying how they were recorded")
	}
	return file.Deliveries
}

// recordedVerifier replays the recorded verification verdicts in order and
// records what it was asked to check. It is the fake a test injects where
// production runs the real adapter.
type recordedVerifier struct {
	verdicts []error
	seen     []payments.Delivery
}

// Verify returns the next recorded verdict, refusing once the recordings
// run out rather than inventing a pass.
func (v *recordedVerifier) Verify(ctx context.Context, d payments.Delivery) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("stub verifier: %w", err)
	}
	v.seen = append(v.seen, d)
	if len(v.verdicts) == 0 {
		return payments.ErrNotVerified
	}
	err := v.verdicts[0]
	v.verdicts = v.verdicts[1:]
	return err
}

// delivery turns one recorded case into a delivery.
func (r recordedDelivery) delivery() Delivery {
	header := http.Header{}
	for name, values := range r.Headers {
		for _, value := range values {
			header.Add(name, value)
		}
	}
	return Delivery{Header: header, Body: append([]byte(nil), r.Body...)}
}

// TestRecordedDeliveries drives the four recorded deliveries through the
// ingest gate in their recorded order and checks every outcome against the
// recording. It is the offline half of the live recording: the verdicts are
// what the sandbox verify endpoint answered, replayed through a fake so the
// suite never reaches the network.
func TestRecordedDeliveries(t *testing.T) {
	recorded := loadRecordedDeliveries(t)
	want := []string{"good", "tampered", "replayed", "unknown"}
	if len(recorded) != len(want) {
		t.Fatalf("recorded %d deliveries, want %d", len(recorded), len(want))
	}
	verifier := &recordedVerifier{}
	for i, delivery := range recorded {
		if delivery.Name != want[i] {
			t.Fatalf("delivery %d is %q, want %q", i, delivery.Name, want[i])
		}
		switch delivery.RecordedVerdict {
		case "SUCCESS":
			verifier.verdicts = append(verifier.verdicts, nil)
		case "FAILURE":
			verifier.verdicts = append(verifier.verdicts, payments.ErrNotVerified)
		default:
			t.Fatalf("delivery %q recorded unknown verdict %q", delivery.Name, delivery.RecordedVerdict)
		}
	}

	service := NewService(fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		WithVerifier(verifier), WithClock(fixedClock()))
	ctx := context.Background()

	var goodRows int
	var tamperedRefusal, unknownRefusal string
	for _, delivery := range recorded {
		result, err := service.HandleDelivery(ctx, delivery.delivery())
		switch delivery.RecordedIngest {
		case "rows":
			if err != nil {
				t.Fatalf("%s: verified delivery refused: %v", delivery.Name, err)
			}
			if result.Duplicate {
				t.Fatalf("%s: first delivery reports Duplicate", delivery.Name)
			}
			if len(result.Rows) == 0 {
				t.Fatalf("%s: verified delivery stored no rows", delivery.Name)
			}
			if result.Pending == nil {
				t.Fatalf("%s: refund filed no pending question", delivery.Name)
			}
			goodRows = len(service.Rows())
		case "duplicate":
			if err != nil {
				t.Fatalf("%s: replayed delivery refused: %v", delivery.Name, err)
			}
			if !result.Duplicate {
				t.Fatalf("%s: replayed delivery does not report Duplicate", delivery.Name)
			}
			if len(result.Rows) != goodRows || len(service.Rows()) != goodRows {
				t.Fatalf("%s: replayed delivery stored %d rows, store holds %d, want the %d already stored",
					delivery.Name, len(result.Rows), len(service.Rows()), goodRows)
			}
		case "refused":
			if !errors.Is(err, ErrBadSignature) {
				t.Fatalf("%s: delivery = %v, want ErrBadSignature", delivery.Name, err)
			}
			if result.Duplicate || len(result.Rows) != 0 {
				t.Fatalf("%s: refused delivery reported %d rows (duplicate %v)", delivery.Name, len(result.Rows), result.Duplicate)
			}
			if len(service.Rows()) != goodRows {
				t.Fatalf("%s: refused delivery changed the store: %d rows, want %d", delivery.Name, len(service.Rows()), goodRows)
			}
			if delivery.Name == "tampered" {
				tamperedRefusal = err.Error()
			}
			if delivery.Name == "unknown" {
				unknownRefusal = err.Error()
			}
		default:
			t.Fatalf("%s: recorded unknown ingest outcome %q", delivery.Name, delivery.RecordedIngest)
		}
	}

	if tamperedRefusal == "" || unknownRefusal == "" {
		t.Fatal("tampered and unknown deliveries were not both refused")
	}
	if tamperedRefusal != unknownRefusal {
		t.Errorf("a tampered body and an unverified delivery get different refusals:\n tampered: %s\n  unknown: %s", tamperedRefusal, unknownRefusal)
	}
	if len(verifier.seen) != len(recorded) {
		t.Errorf("verifier checked %d deliveries, want %d: every delivery is checked, replay included", len(verifier.seen), len(recorded))
	}
	if len(service.Rows()) != goodRows {
		t.Errorf("store holds %d rows, want the %d one verified delivery wrote", len(service.Rows()), goodRows)
	}
	if len(service.PendingQuestions()) != 1 {
		t.Errorf("store holds %d pending questions, want 1", len(service.PendingQuestions()))
	}
}

// TestVerificationPrecedesIdempotency proves a forged body naming an
// already stored event id is refused instead of answered as a duplicate.
func TestVerificationPrecedesIdempotency(t *testing.T) {
	recorded := loadRecordedDeliveries(t)
	good := recorded[0]
	verifier := &recordedVerifier{verdicts: []error{nil, payments.ErrNotVerified}}
	service := NewService(fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		WithVerifier(verifier), WithClock(fixedClock()))
	ctx := context.Background()

	if _, err := service.HandleDelivery(ctx, good.delivery()); err != nil {
		t.Fatalf("good delivery refused: %v", err)
	}
	forged := good.delivery()
	forged.Body = []byte(`{"id":"WH-2EV-01","event_type":"PAYMENT.CAPTURE.REFUNDED","resource":{"amount":{"value":"1.00"}}}`)
	result, err := service.HandleDelivery(ctx, forged)
	if !errors.Is(err, ErrBadSignature) {
		t.Fatalf("forged delivery naming a stored event = %v, want ErrBadSignature", err)
	}
	if result.Duplicate {
		t.Error("forged delivery was answered as a duplicate, so its signature was never checked")
	}
	if len(service.Rows()) != 1 {
		t.Errorf("store holds %d rows, want the 1 verified row", len(service.Rows()))
	}
}

// TestDefaultGateFailsClosed proves the default gate is the real adapter
// and that it refuses when no credentials are configured.
func TestDefaultGateFailsClosed(t *testing.T) {
	t.Setenv("PAYPAL_CLIENT_ID", "")
	t.Setenv("PAYPAL_CLIENT_SECRET", "")
	recorded := loadRecordedDeliveries(t)
	service := NewService(fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()))
	_, err := service.HandleDelivery(context.Background(), recorded[0].delivery())
	if !errors.Is(err, ErrBadSignature) {
		t.Fatalf("unconfigured verifier = %v, want ErrBadSignature", err)
	}
	if !errors.Is(err, payments.ErrUnavailable) {
		t.Errorf("unconfigured verifier did not report ErrUnavailable: %v", err)
	}
	if len(service.Rows()) != 0 {
		t.Errorf("unconfigured verifier stored %d rows", len(service.Rows()))
	}
}

// TestMalformedDeliveryIsRefused proves a delivery with no event id or no
// body is refused before the verifier is asked.
func TestMalformedDeliveryIsRefused(t *testing.T) {
	verifier := &recordedVerifier{verdicts: []error{nil}}
	service := NewService(fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()), WithVerifier(verifier))
	cases := []Delivery{
		{Body: []byte(`{"event_type":"PAYMENT.CAPTURE.REFUNDED"}`)},
		{Body: []byte(`not json`)},
		{},
	}
	for _, d := range cases {
		if _, err := service.HandleDelivery(context.Background(), d); !errors.Is(err, ErrMalformedEvent) {
			t.Errorf("malformed delivery = %v, want ErrMalformedEvent", err)
		}
	}
	if len(verifier.seen) != 0 {
		t.Errorf("malformed deliveries reached the verifier: %d", len(verifier.seen))
	}
}
