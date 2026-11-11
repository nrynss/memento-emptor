package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/payments"
)

// acceptDelivery is the delivery gate the route tests inject: every
// delivery verifies, because these tests are about the shopper join and
// the stored rows, not the signature.
type acceptDelivery struct{}

// Verify accepts one delivery.
func (acceptDelivery) Verify(context.Context, payments.Delivery) error { return nil }

// webhookRouteInputs is one recorded event per outcome, each on its own
// purchase and with its own shopper join.
func webhookRouteInputs() fakes.StoryInputs {
	disputeLink := domain.TransactionLink{
		OrderID: "ORD-WH-D", CaptureID: "CAP-WH-D", DisputeID: "PP-D-WH",
		CustomID: "dispute_shopper", EventID: "WH-DISPUTE-ROUTE",
	}
	disputeItem := domain.Item{
		BrandID: "brand-x-001", BrandName: "Brand X", LineName: "chino",
		SKU: "chino-khaki-32", Size: "32", Category: "pants",
	}
	disputeEvent := fakes.WebhookEvent{
		EventID: "WH-DISPUTE-ROUTE", EventType: "CUSTOMER.DISPUTE.CREATED", Verified: true,
		Outcome: domain.Outcome{Kind: domain.OutcomeDispute, Status: "OPEN", AmountMinor: 8900, Currency: "USD"},
		Link:    disputeLink, DisputeCode: domain.DisputeMerchandiseNotReceived,
	}
	disputeRecord := fakes.DisputeRecord{
		DisputeID: "PP-D-WH", Code: domain.DisputeMerchandiseNotReceived,
		Category: domain.CategoryNotArrived, Item: disputeItem, Link: disputeLink,
	}
	disputeOrder := fakes.OrderRecord{OrderID: "ORD-WH-D", Items: []domain.Item{disputeItem}, Link: disputeLink}

	captureLink := domain.TransactionLink{
		OrderID: "ORD-WH-C", CaptureID: "CAP-WH-C",
		CustomID: "capture_shopper", EventID: "WH-CAPTURE-ROUTE",
	}
	captureItem := domain.Item{
		BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt",
		SKU: "PSW8GeK", Size: "M", Category: "shirts",
	}
	captureEvent := fakes.WebhookEvent{
		EventID: "WH-CAPTURE-ROUTE", EventType: "PAYMENT.CAPTURE.COMPLETED", Verified: true,
		Outcome: domain.Outcome{Kind: domain.OutcomeCapture, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"},
		Link:    captureLink,
	}
	captureOrder := fakes.OrderRecord{OrderID: "ORD-WH-C", Items: []domain.Item{captureItem}, Link: captureLink}

	refundLink := domain.TransactionLink{
		OrderID: "ORD-WH-R", CaptureID: "CAP-WH-R", RefundID: "RFD-WH-R",
		CustomID: "refund_shopper", EventID: "WH-REFUND-ROUTE",
	}
	refundEvent := fakes.WebhookEvent{
		EventID: "WH-REFUND-ROUTE", EventType: "PAYMENT.CAPTURE.REFUNDED", Verified: true,
		Outcome: domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"},
		Link:    refundLink,
	}
	refundRecord := fakes.RefundRecord{
		RefundID: "RFD-WH-R", Outcome: refundEvent.Outcome, Link: refundLink,
	}
	refundOrder := fakes.OrderRecord{OrderID: "ORD-WH-R", Items: []domain.Item{captureItem}, Link: refundLink}

	return fakes.StoryInputs{
		Events:   []fakes.WebhookEvent{disputeEvent, captureEvent, refundEvent},
		Orders:   []fakes.OrderRecord{disputeOrder, captureOrder, refundOrder},
		Refunds:  []fakes.RefundRecord{refundRecord},
		Disputes: []fakes.DisputeRecord{disputeRecord},
	}
}

// webhookRouteService builds one service over the recorded events with the
// accepting gate and a pinned clock.
func webhookRouteService(t *testing.T) *Service {
	t.Helper()
	store, err := evidence.NewStore(testContract{kind: "webhook-route"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	seller := domain.Subject{Kind: domain.SubjectSeller, ID: "seller-for-brand-x-001", Name: "Brand X seller"}
	graph := fakes.NewRecordingEvidenceGraph([]string{
		beliefContent(domain.BeliefKindSeller, seller, sellerBeliefValue),
		"evidence[doc-0001] dispute WH-DISPUTE-ROUTE",
	}, nil)
	fixed := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:   fakes.NewRecordingPaymentSource(webhookRouteInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      graph,
		SchemaDir:  apiDir(t),
		Clock:      func() time.Time { return fixed },
		Verifier:   acceptDelivery{},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

// postDelivery posts one body to the layer's webhook route.
func postDelivery(svc *Service, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/webhooks/payment", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	svc.Handler().ServeHTTP(recorder, request)
	return recorder
}

// decodeResult decodes one webhook answer.
func decodeResult(t *testing.T, recorder *httptest.ResponseRecorder) WebhookResult {
	t.Helper()
	var result WebhookResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode webhook result: %v (body %s)", err, recorder.Body.String())
	}
	return result
}

// wireMessage reads the frozen error envelope's message.
func wireMessage(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v (body %s)", err, recorder.Body.String())
	}
	return envelope.Error.Message
}

// TestWebhookRouteDisputeJoinsPerTransactionCustom proves a dispute
// delivery carries its shopper join per disputed transaction: the route
// resolves that custom, the evidence lands in that shopper's partition,
// and the row counts toward the seller.
func TestWebhookRouteDisputeJoinsPerTransactionCustom(t *testing.T) {
	svc := webhookRouteService(t)
	body := `{"id":"WH-DISPUTE-ROUTE","event_type":"CUSTOMER.DISPUTE.CREATED","resource":{"dispute_id":"PP-D-WH","reason":"MERCHANDISE_OR_SERVICE_NOT_RECEIVED","disputed_transactions":[{"custom":"dispute_shopper","seller_transaction_id":"CAP-WH-D"}]}}`

	if custom, err := webhookShopper([]byte(body)); err != nil || custom != "dispute_shopper" {
		t.Fatalf("dispute shopper = %q, %v; want dispute_shopper", custom, err)
	}
	recorder := postDelivery(svc, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("dispute delivery = %d: %s", recorder.Code, recorder.Body.String())
	}
	result := decodeResult(t, recorder)
	if result.Rows != 1 || result.Duplicate || result.Pending {
		t.Fatalf("dispute result = %+v, want one new row, no question", result)
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.shoppers) != 1 {
		t.Fatalf("service holds %d partitions, want the one dispute shopper", len(svc.shoppers))
	}
	st, ok := svc.shoppers["dispute_shopper"]
	if !ok {
		t.Fatalf("dispute did not land in dispute_shopper: %v", svc.shoppers)
	}
	if len(st.evidence) != 1 {
		t.Fatalf("dispute shopper holds %d evidence rows, want 1", len(st.evidence))
	}
	for _, rec := range st.evidence {
		if rec.row.Outcome.Kind != domain.OutcomeDispute {
			t.Errorf("row outcome = %q, want dispute", rec.row.Outcome.Kind)
		}
		if rec.row.Link.DisputeID != "PP-D-WH" || rec.row.Link.CustomID != "dispute_shopper" {
			t.Errorf("row link = %+v, want the disputed transaction", rec.row.Link)
		}
		if rec.counts != domain.BeliefKindSeller {
			t.Errorf("row counts toward %q, want seller_reliability", rec.counts)
		}
	}
}

// TestWebhookRouteDisputeWithoutCustomIsRefused proves a dispute that
// names no shopper is refused with its own sentinel and stores nothing,
// rather than being defaulted to a partition.
func TestWebhookRouteDisputeWithoutCustomIsRefused(t *testing.T) {
	svc := webhookRouteService(t)
	body := `{"id":"WH-DISPUTE-ROUTE","event_type":"CUSTOMER.DISPUTE.CREATED","resource":{"disputed_transactions":[{"custom":""}]}}`

	_, err := webhookShopper([]byte(body))
	if !errors.Is(err, ErrDisputeWithoutShopper) {
		t.Fatalf("dispute without custom = %v, want ErrDisputeWithoutShopper", err)
	}
	if !errors.Is(err, ErrWebhookShopper) {
		t.Errorf("dispute refusal does not wrap ErrWebhookShopper: %v", err)
	}
	recorder := postDelivery(svc, body)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("dispute without custom = %d, want 400", recorder.Code)
	}
	if message := wireMessage(t, recorder); message != ErrDisputeWithoutShopper.Error() {
		t.Errorf("refusal message = %q, want %q", message, ErrDisputeWithoutShopper.Error())
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.shoppers) != 0 {
		t.Errorf("refused dispute created %d partitions", len(svc.shoppers))
	}
}

// TestWebhookRouteCaptureAndRefundKeepTopLevelCustom proves the capture and
// refund path is unchanged: the resource's top-level custom id is the join,
// its missing-custom refusal keeps the old wire text, and the rows land in
// the named shopper's partition.
func TestWebhookRouteCaptureAndRefundKeepTopLevelCustom(t *testing.T) {
	svc := webhookRouteService(t)
	captureBody := `{"id":"WH-CAPTURE-ROUTE","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{"custom_id":"capture_shopper"}}`
	refundBody := `{"id":"WH-REFUND-ROUTE","event_type":"PAYMENT.CAPTURE.REFUNDED","resource":{"custom_id":"refund_shopper"}}`

	if custom, err := webhookShopper([]byte(captureBody)); err != nil || custom != "capture_shopper" {
		t.Fatalf("capture shopper = %q, %v; want capture_shopper", custom, err)
	}
	if custom, err := webhookShopper([]byte(refundBody)); err != nil || custom != "refund_shopper" {
		t.Fatalf("refund shopper = %q, %v; want refund_shopper", custom, err)
	}

	captureRecorder := postDelivery(svc, captureBody)
	if captureRecorder.Code != http.StatusOK {
		t.Fatalf("capture delivery = %d: %s", captureRecorder.Code, captureRecorder.Body.String())
	}
	if result := decodeResult(t, captureRecorder); result.Rows != 1 || result.Pending {
		t.Fatalf("capture result = %+v, want one row and no question", result)
	}
	refundRecorder := postDelivery(svc, refundBody)
	if refundRecorder.Code != http.StatusOK {
		t.Fatalf("refund delivery = %d: %s", refundRecorder.Code, refundRecorder.Body.String())
	}
	if result := decodeResult(t, refundRecorder); result.Rows != 1 || !result.Pending {
		t.Fatalf("refund result = %+v, want one row and a question", result)
	}

	svc.mu.Lock()
	if _, ok := svc.shoppers["capture_shopper"]; !ok {
		t.Errorf("capture did not land in capture_shopper: %v", svc.shoppers)
	}
	if _, ok := svc.shoppers["refund_shopper"]; !ok {
		t.Errorf("refund did not land in refund_shopper: %v", svc.shoppers)
	}
	svc.mu.Unlock()

	noCustomCapture := `{"id":"WH-CAPTURE-ROUTE","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{}}`
	if _, err := webhookShopper([]byte(noCustomCapture)); !errors.Is(err, ErrWebhookShopper) {
		t.Fatalf("capture without custom = %v, want ErrWebhookShopper", err)
	} else if errors.Is(err, ErrDisputeWithoutShopper) {
		t.Errorf("capture without custom also matches the dispute sentinel: %v", err)
	}
	refused := postDelivery(svc, noCustomCapture)
	if refused.Code != http.StatusBadRequest {
		t.Fatalf("capture without custom = %d, want 400", refused.Code)
	}
	if message := wireMessage(t, refused); message != "delivery names no shopper" {
		t.Errorf("capture refusal message = %q, want the old text", message)
	}
}

// TestWebhookShopperKeepsTopLevelCustomFirst proves a dispute that carries
// both forms still joins on the top-level custom id, so the order of
// resolution is the capture/refund path first.
func TestWebhookShopperKeepsTopLevelCustomFirst(t *testing.T) {
	body := `{"event_type":"CUSTOMER.DISPUTE.CREATED","resource":{"custom_id":"top_level","disputed_transactions":[{"custom":"per_transaction"}]}}`
	custom, err := webhookShopper([]byte(body))
	if err != nil || custom != "top_level" {
		t.Fatalf("dispute shopper = %q, %v; want top_level", custom, err)
	}
}
