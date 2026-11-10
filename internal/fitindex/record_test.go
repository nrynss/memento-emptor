package fitindex

import (
	"context"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/identity"
	"github.com/nrynss/memento-emptor/internal/ingest"
)

// storyLines resolves the recorded story's line from its item: the brand id
// and category the order carries, with the line's cut.
func storyLines(_ context.Context, item domain.Item) (domain.ProductLine, error) {
	return domain.ProductLine{Brand: item.BrandID, Category: item.Category, FitHandle: "regular"}, nil
}

// TestIngestWritesTheFitRowBesideTheEvidence drives the recorded story's
// capture and refund through ingest with the writer as its recorder. Each
// delivery writes the shopper's evidence and the purchase's fit row from
// one event, keyed by the transaction hash, and a redelivered webhook
// writes neither twice.
func TestIngestWritesTheFitRowBesideTheEvidence(t *testing.T) {
	index := newFakeIndex()
	writer, err := NewWriter(index, testKeyer(t), "payer-0001", LinesFunc(storyLines))
	if err != nil {
		t.Fatalf("build writer: %v", err)
	}
	service := ingest.NewService(fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		ingest.WithFitRecorder(writer))
	ctx := context.Background()

	if _, err := service.Handle(ctx, "WH-2EV-06"); err != nil {
		t.Fatalf("ingest capture: %v", err)
	}
	if len(service.Rows()) != 1 {
		t.Fatalf("ingest stored %d evidence rows, want the capture's 1", len(service.Rows()))
	}
	captured, ok := index.docs[mustTxnHash(t, "CAP-7XK-06")]
	if !ok {
		t.Fatalf("index holds no row for the captured purchase; it holds %d rows", len(index.docs))
	}
	if captured.State != domain.FitStateCaptured || captured.Settled {
		t.Errorf("captured row = %q settled %v, want captured and unsettled", captured.State, captured.Settled)
	}
	if !captured.CapturedAt.Equal(fakes.OccurredAt("WH-2EV-06")) {
		t.Errorf("captured_at = %s, want the delivery's event time", captured.CapturedAt)
	}
	if captured.Line.Brand != "brand-x-001" || captured.Line.Category != "shirts" || captured.Line.FitHandle != "regular" {
		t.Errorf("line = %+v, want the catalog's line for the item", captured.Line)
	}
	if captured.Seeded {
		t.Error("an ingested row is marked seeded; lived rows are not")
	}

	if _, err := service.Handle(ctx, "WH-2EV-01"); err != nil {
		t.Fatalf("ingest refund: %v", err)
	}
	refunded, ok := index.docs[mustTxnHash(t, "CAP-7XK-01")]
	if !ok {
		t.Fatalf("index holds no row for the refunded purchase; it holds %d rows", len(index.docs))
	}
	if refunded.State != domain.FitStateRefunded || refunded.Settled {
		t.Errorf("refunded row = %q settled %v, want refunded and unsettled", refunded.State, refunded.Settled)
	}
	if !refunded.RefundedAt.Equal(fakes.OccurredAt("WH-2EV-01")) {
		t.Errorf("refunded_at = %s, want the delivery's event time", refunded.RefundedAt)
	}

	writes := index.writes
	rows := len(service.Rows())
	if _, err := service.Handle(ctx, "WH-2EV-01"); err != nil {
		t.Fatalf("replay refund: %v", err)
	}
	if index.writes != writes {
		t.Errorf("a replayed webhook wrote %d fit rows, want 0 new writes", index.writes-writes)
	}
	if len(service.Rows()) != rows {
		t.Errorf("a replayed webhook stored %d evidence rows, want the %d already stored", len(service.Rows()), rows)
	}
	if len(index.docs) != 2 {
		t.Errorf("index holds %d rows, want one per purchase", len(index.docs))
	}
}

// mustTxnHash is the row id for one transaction id.
func mustTxnHash(t *testing.T, txnID string) string {
	t.Helper()
	hash, err := identity.TxnHash(txnID)
	if err != nil {
		t.Fatalf("hash transaction %q: %v", txnID, err)
	}
	return hash
}
