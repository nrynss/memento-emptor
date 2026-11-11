package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// unknownDisputeCode is a dispute reason the frozen table does not name:
// ingest stores the outcome with the code as its text and no category.
const unknownDisputeCode domain.DisputeCode = "NOT_A_FROZEN_DISPUTE_CODE"

// unattachedDisputeInputs extends the demo story with two disputes the
// dispute table cannot classify: one whose code is outside the table, and
// one that carries no code at all.
func unattachedDisputeInputs() fakes.StoryInputs {
	inputs := fakes.DemoStoryInputs()
	for i, code := range []domain.DisputeCode{unknownDisputeCode, ""} {
		n := fmt.Sprintf("%02d", 7+i)
		event := "WH-2EV-" + n
		order := "ORD-7XK-" + n
		disputeID := "PP-D-900" + n
		link := domain.TransactionLink{
			OrderID:   order,
			CaptureID: "CAP-7XK-" + n,
			DisputeID: disputeID,
			CustomID:  "memento-shopper01-" + n,
			EventID:   event,
		}
		item := domain.Item{
			BrandID: "brand-x-001", BrandName: "Brand X", LineName: "chino",
			SKU: "chino-khaki-32", Size: "32", Category: "pants",
		}
		outcome := domain.Outcome{Kind: domain.OutcomeDispute, Status: "OPEN", AmountMinor: 8900, Currency: "USD"}
		inputs.Events = append(inputs.Events, fakes.WebhookEvent{
			EventID:     event,
			EventType:   "CUSTOMER.DISPUTE.CREATED",
			Verified:    true,
			Outcome:     outcome,
			Link:        link,
			DisputeCode: code,
			Provenance:  "testdata/paypal/webhook-delivery-dispute.json",
		})
		inputs.Orders = append(inputs.Orders, fakes.OrderRecord{
			OrderID: order, Items: []domain.Item{item}, Link: link,
			Provenance: "testdata/paypal/order-get-approved-line-items.json",
		})
		inputs.Disputes = append(inputs.Disputes, fakes.DisputeRecord{
			DisputeID: disputeID, Code: code, Category: domain.CategoryNone, Item: item, Link: link,
			Provenance: "testdata/paypal/dispute-get.json",
		})
	}
	return inputs
}

// ingestWriteOpHarness builds a service over the extended inputs with a
// pinned clock and a graph that must see no write: neither dispute counts
// toward a belief, so ingest writes no structural edge for either.
func ingestWriteOpHarness(t *testing.T) *Service {
	t.Helper()
	store, err := evidence.NewStore(testContract{kind: "server-ingest-writeop"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	fixed := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:   fakes.NewRecordingPaymentSource(unattachedDisputeInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      fakes.NewRecordingEvidenceGraph(nil, nil),
		SchemaDir:  apiDir(t),
		Clock:      func() time.Time { return fixed },
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

// frozenWriteOps returns the write operations the frozen evidence schema
// admits. The web decoder's writeOps list carries the same three; any
// other value, including the zero value, fails both.
func frozenWriteOps(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(apiDir(t), "common.schema.json"))
	if err != nil {
		t.Fatalf("read common.schema.json: %v", err)
	}
	var schema struct {
		Defs map[string]struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse common.schema.json: %v", err)
	}
	evidence, ok := schema.Defs["Evidence"]
	if !ok {
		t.Fatal("common.schema.json has no Evidence definition")
	}
	writeOp, ok := evidence.Properties["write_op"]
	if !ok || len(writeOp.Enum) == 0 {
		t.Fatal("common.schema.json pins no write_op values")
	}
	out := map[string]bool{}
	for _, op := range writeOp.Enum {
		out[op] = true
	}
	return out
}

// TestUnknownDisputeCodeRidesAsObservation proves the served layer emits an
// unknown-code dispute as an observation: it is stored, it keeps its code
// as the reason's text, it counts toward nothing and links no belief, and
// its emitted write_op is one the frozen schema and the web decoder admit.
func TestUnknownDisputeCodeRidesAsObservation(t *testing.T) {
	ctx := context.Background()
	svc := ingestWriteOpHarness(t)
	shopper := "writeop_unknown_code"
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-07"); err != nil {
		t.Fatalf("ingest unknown-code dispute: %v", err)
	}

	ledger, err := svc.LedgerEvidence(ctx, contract.LedgerEvidenceRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("ledger evidence: %v", err)
	}
	if len(ledger.Evidence) != 1 {
		t.Fatalf("ledger shows %d rows, want the stored unknown-code dispute", len(ledger.Evidence))
	}
	row := ledger.Evidence[0]
	if row.Evidence.Link.EventID != "WH-2EV-07" {
		t.Fatalf("ledger row is %q, want the unknown-code dispute", row.Evidence.Link.EventID)
	}
	if row.Evidence.Reason == nil || row.Evidence.Reason.Text != string(unknownDisputeCode) {
		t.Fatalf("ledger row reason %+v, want the unknown code as its text", row.Evidence.Reason)
	}
	if row.Evidence.Reason.Category != domain.CategoryNone {
		t.Errorf("reason category = %q, want none: the code matches no belief", row.Evidence.Reason.Category)
	}
	if row.Evidence.CountsToward != nil {
		t.Errorf("counts_toward = %q, want null", *row.Evidence.CountsToward)
	}
	if row.Evidence.WriteOp != domain.WriteObserve {
		t.Errorf("write_op = %q, want observe: the row is an observation, not a structural write",
			row.Evidence.WriteOp)
	}
	if len(row.BeliefIDs) != 0 {
		t.Errorf("belief link = %v, want none", row.BeliefIDs)
	}

	// The emitted row must decode in the web client: the frozen schema
	// admits only its three write operations, and the decoder requires
	// belief_ids to be an array, so an unattached row marshals [] and
	// never null.
	raw, err := json.Marshal(ledger)
	if err != nil {
		t.Fatalf("marshal ledger: %v", err)
	}
	var wire struct {
		Evidence []struct {
			Evidence struct {
				WriteOp string `json:"write_op"`
			} `json:"evidence"`
			BeliefIDs json.RawMessage `json:"belief_ids"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("decode ledger JSON: %v", err)
	}
	if len(wire.Evidence) != 1 {
		t.Fatalf("wire holds %d rows, want 1", len(wire.Evidence))
	}
	allowed := frozenWriteOps(t)
	if !allowed[wire.Evidence[0].Evidence.WriteOp] {
		t.Errorf("emitted write_op %q is not one the frozen schema admits %v",
			wire.Evidence[0].Evidence.WriteOp, allowed)
	}
	if got := string(wire.Evidence[0].BeliefIDs); got != "[]" {
		t.Errorf("belief_ids marshals %s, want [] so the decoder accepts it", got)
	}
}

// TestCodeLessDisputeStaysAuditOnly pins the decided shape of a dispute
// that arrived with no code: the outcome is stored, carries no reason and
// no belief link, and rides as an observation, but it never reaches the
// ledger list. There is nothing to show beside the outcome and no question
// to ask, so it stays an audit row instead of a silent omission from the
// store.
func TestCodeLessDisputeStaysAuditOnly(t *testing.T) {
	ctx := context.Background()
	svc := ingestWriteOpHarness(t)
	shopper := "writeop_no_code"
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-08"); err != nil {
		t.Fatalf("ingest code-less dispute: %v", err)
	}

	ledger, err := svc.LedgerEvidence(ctx, contract.LedgerEvidenceRequest{ShopperID: shopper})
	if err != nil {
		t.Fatalf("ledger evidence: %v", err)
	}
	if len(ledger.Evidence) != 0 {
		t.Fatalf("ledger shows %d rows, want none: the code-less dispute is audit-only", len(ledger.Evidence))
	}

	svc.mu.Lock()
	st := svc.shoppers[shopper]
	if st == nil {
		svc.mu.Unlock()
		t.Fatalf("no partition for %q", shopper)
	}
	var stored *evidenceRecord
	for _, rec := range st.evidence {
		if rec.row.Link.EventID == "WH-2EV-08" {
			stored = rec
		}
	}
	beliefs := len(st.beliefs)
	var writeOp domain.WriteOp
	var beliefIDs int
	var counts domain.BeliefKind
	var reason *domain.Reason
	if stored != nil {
		writeOp = stored.writeOp
		beliefIDs = len(stored.beliefIDs)
		counts = stored.counts
		reason = stored.row.Reason
	}
	svc.mu.Unlock()

	if stored == nil {
		t.Fatal("the code-less dispute was not stored: audit rows are kept, not dropped")
	}
	if reason != nil {
		t.Fatalf("stored reason %+v, want none: the dispute carried no code", reason)
	}
	if counts != domain.BeliefKindNone {
		t.Errorf("counts = %q, want none", counts)
	}
	if writeOp != domain.WriteObserve {
		t.Errorf("stored write_op = %q, want observe", writeOp)
	}
	if beliefIDs != 0 {
		t.Errorf("stored belief links = %d, want none", beliefIDs)
	}
	if beliefs != 0 {
		t.Errorf("the shopper holds %d beliefs, want none: neither dispute earned one", beliefs)
	}
}
