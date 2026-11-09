package orders

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

func testItem() domain.Item {
	return domain.Item{
		BrandID: "brand-x-001", BrandName: "Brand X",
		LineName: "oxford shirt", SKU: "PSW8GeK",
		Size: "M", Category: "shirts",
	}
}

func newTestService() (*Service, *FakeCheckout, error) {
	fake := NewFakeCheckout()
	service, err := NewService(fake, Config{MerchantID: "merchant-test", Sandbox: true})
	if err != nil {
		return nil, nil, err
	}
	return service, fake, nil
}

// TestKeysAbsent keeps the suite offline: no key variable may be set
// where the replay requires them absent.
func TestKeysAbsent(t *testing.T) {
	if err := fakes.AssertKeysAbsent(); err != nil {
		t.Fatalf("keys set during offline test: %v", err)
	}
}

// TestRetriedCreateIsIdempotent covers the first Done-when clause: a
// retried create reusing the PayPal-Request-Id returns the stored order
// and never touches the transport again.
func TestRetriedCreateIsIdempotent(t *testing.T) {
	service, fake, err := newTestService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	first, duplicate, err := service.Propose(ctx, "shopper_01", "oxford-shirt-M", testItem(), 19500, "USD", "req-1")
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if duplicate {
		t.Fatal("first propose reported duplicate")
	}
	if first.CreateRequestID != "req-1" {
		t.Fatalf("create request id = %q, want req-1", first.CreateRequestID)
	}
	if fake.CreateCalls != 1 {
		t.Fatalf("transport create calls = %d, want 1", fake.CreateCalls)
	}

	second, duplicate, err := service.Propose(ctx, "shopper_01", "oxford-shirt-M", testItem(), 19500, "USD", "req-1")
	if err != nil {
		t.Fatalf("retried propose: %v", err)
	}
	if !duplicate {
		t.Fatal("retried propose did not report duplicate")
	}
	if second.OrderID != first.OrderID {
		t.Fatalf("retried order id = %q, want %q", second.OrderID, first.OrderID)
	}
	if fake.CreateCalls != 1 {
		t.Fatalf("transport create calls after retry = %d, want 1: retry reached the transport", fake.CreateCalls)
	}
}

// TestDuplicateWebhookIsIdempotent covers the second Done-when clause:
// redelivery of a capture webhook returns the stored order and
// transitions nothing twice.
func TestDuplicateWebhookIsIdempotent(t *testing.T) {
	service, fake, err := newTestService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	proposed, _, err := service.Propose(ctx, "shopper_01", "oxford-shirt-M", testItem(), 19500, "USD", "req-1")
	if err != nil {
		t.Fatal(err)
	}
	fake.ApproveOrder(proposed.OrderID)
	if _, err := service.Approve(ctx, proposed.OrderID); err != nil {
		t.Fatal(err)
	}
	captured, err := service.Capture(ctx, proposed.OrderID)
	if err != nil {
		t.Fatal(err)
	}

	first, duplicate, err := service.HandleWebhook(ctx, "WH-TEST-01", EventCaptureCompleted, proposed.OrderID, captured.CaptureID)
	if err != nil {
		t.Fatalf("webhook: %v", err)
	}
	if duplicate {
		t.Fatal("first delivery reported duplicate")
	}
	if first.State != StateCaptured {
		t.Fatalf("state = %q, want captured", first.State)
	}

	second, duplicate, err := service.HandleWebhook(ctx, "WH-TEST-01", EventCaptureCompleted, proposed.OrderID, captured.CaptureID)
	if err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if !duplicate {
		t.Fatal("redelivery did not report duplicate")
	}
	if second.State != StateCaptured || second.CaptureID != captured.CaptureID {
		t.Fatalf("redelivery changed the order: %+v", second)
	}
}

// TestSecondOrderWhileOpenRefused covers the third Done-when clause: a
// second order for the same (shopper, need) while one is open is
// refused, and the slot frees once the first order closes.
func TestSecondOrderWhileOpenRefused(t *testing.T) {
	service, _, err := newTestService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	first, _, err := service.Propose(ctx, "shopper_01", "oxford-shirt-M", testItem(), 19500, "USD", "req-1")
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := service.Propose(ctx, "shopper_01", "oxford-shirt-M", testItem(), 19500, "USD", "req-2"); !errors.Is(err, ErrNeedBusy) {
		t.Fatalf("second open propose err = %v, want %v", err, ErrNeedBusy)
	}

	if _, _, err := service.Propose(ctx, "shopper_01", "chino-32", testItem(), 8900, "USD", "req-3"); err != nil {
		t.Fatalf("different need propose: %v", err)
	}

	if id, open := service.InFlight("shopper_01", "oxford-shirt-M"); !open || id != first.OrderID {
		t.Fatalf("InFlight = %q, %v, want %q, true", id, open, first.OrderID)
	}

	if _, err := service.Approve(ctx, first.OrderID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.HandleWebhook(ctx, "WH-TEST-DENIED", EventCaptureDenied, first.OrderID, ""); err != nil {
		t.Fatal(err)
	}
	if _, open := service.InFlight("shopper_01", "oxford-shirt-M"); open {
		t.Fatal("InFlight still reports the need after the order failed")
	}

	if _, duplicate, err := service.Propose(ctx, "shopper_01", "oxford-shirt-M", testItem(), 19500, "USD", "req-4"); err != nil || duplicate {
		t.Fatalf("propose after close err = %v, duplicate = %v; want a fresh order", err, duplicate)
	}
}

// TestCaptureWithoutApprovalFailsClosed covers the fourth Done-when
// clause: capturing a proposed order fails with ErrNoApproval, the
// transport is never called, and the order does not move. The same
// fail-closed rule holds for a capture webhook on an unapproved order.
func TestCaptureWithoutApprovalFailsClosed(t *testing.T) {
	service, fake, err := newTestService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	proposed, _, err := service.Propose(ctx, "shopper_01", "oxford-shirt-M", testItem(), 19500, "USD", "req-1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.Capture(ctx, proposed.OrderID); !errors.Is(err, ErrNoApproval) {
		t.Fatalf("capture err = %v, want %v", err, ErrNoApproval)
	}
	if fake.CaptureCalls != 0 {
		t.Fatalf("transport capture calls = %d, want 0: fail-closed capture reached PayPal", fake.CaptureCalls)
	}
	stored, err := service.Get(ctx, proposed.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != StateProposed {
		t.Fatalf("state after refused capture = %q, want proposed", stored.State)
	}

	if _, _, err := service.HandleWebhook(ctx, "WH-TEST-EARLY", EventCaptureCompleted, proposed.OrderID, "CAPTURE-TEST-9"); !errors.Is(err, ErrNoApproval) {
		t.Fatalf("webhook err = %v, want %v", err, ErrNoApproval)
	}
}

// TestSourceMismatchFailsOrder covers the payment source disagreeing
// with the local approval record: the service recorded approval but the
// source reports ORDER_NOT_APPROVED, so the order fails and capture
// reports ErrNoApproval.
func TestSourceMismatchFailsOrder(t *testing.T) {
	service, _, err := newTestService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	proposed, _, err := service.Propose(ctx, "shopper_01", "oxford-shirt-M", testItem(), 19500, "USD", "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Approve(ctx, proposed.OrderID); err != nil {
		t.Fatal(err)
	}
	// No fake.ApproveOrder: the source never saw the shopper approve.

	if _, err := service.Capture(ctx, proposed.OrderID); !errors.Is(err, ErrNoApproval) {
		t.Fatalf("capture err = %v, want %v", err, ErrNoApproval)
	}
	stored, err := service.Get(ctx, proposed.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != StateFailed {
		t.Fatalf("state after source mismatch = %q, want failed", stored.State)
	}
}

// TestRefundClosesCapturedOrder walks the happy path to captured and
// then to refunded, freeing nothing twice: a refund webhook on a
// non-captured order is refused.
func TestRefundClosesCapturedOrder(t *testing.T) {
	service, fake, err := newTestService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	proposed, _, err := service.Propose(ctx, "shopper_01", "oxford-shirt-M", testItem(), 19500, "USD", "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.HandleWebhook(ctx, "WH-TEST-REF", EventCaptureRefunded, proposed.OrderID, "RFD-TEST-1"); !errors.Is(err, ErrBadState) {
		t.Fatalf("refund of proposed order err = %v, want %v", err, ErrBadState)
	}

	fake.ApproveOrder(proposed.OrderID)
	if _, err := service.Approve(ctx, proposed.OrderID); err != nil {
		t.Fatal(err)
	}
	captured, err := service.Capture(ctx, proposed.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if captured.State != StateCaptured || captured.CaptureID == "" {
		t.Fatalf("captured order: %+v", captured)
	}
	if captured.CaptureRequestID == "" {
		t.Fatal("capture stored no PayPal-Request-Id")
	}

	refunded, duplicate, err := service.HandleWebhook(ctx, "WH-TEST-REF", EventCaptureRefunded, proposed.OrderID, "RFD-TEST-1")
	if err != nil {
		t.Fatal(err)
	}
	if duplicate || refunded.State != StateRefunded {
		t.Fatalf("refunded = %+v, duplicate = %v", refunded, duplicate)
	}

	if _, duplicate, err := service.HandleWebhook(ctx, "WH-TEST-REF", EventCaptureRefunded, proposed.OrderID, "RFD-TEST-1"); err != nil || !duplicate {
		t.Fatalf("refund redelivery err = %v, duplicate = %v; want stored duplicate", err, duplicate)
	}
}

// TestSandboxOnly proves the service refuses a non-sandbox merchant, so
// no live credential can enter the purchase path by accident.
func TestSandboxOnly(t *testing.T) {
	if _, err := NewService(NewFakeCheckout(), Config{MerchantID: "merchant-test"}); !errors.Is(err, ErrLiveMerchant) {
		t.Fatalf("non-sandbox config err = %v, want %v", err, ErrLiveMerchant)
	}
	if _, err := NewService(NewFakeCheckout(), Config{MerchantID: "merchant-test", Sandbox: true}); err != nil {
		t.Fatalf("sandbox config: %v", err)
	}
}

// TestFixtureShapes ties the fake to the T0.4 recordings it replays: the
// two create recordings carry the same order id under the same
// PayPal-Request-Id, and the early-capture recording carries the
// ORDER_NOT_APPROVED issue the fail-closed paths surface.
func TestFixtureShapes(t *testing.T) {
	read := func(name string) map[string]any {
		raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "paypal", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		return decoded
	}

	create := read("order-wallet-create.json")
	replay := read("order-wallet-replay-same-request-id.json")
	if create["id"] != replay["id"] {
		t.Fatalf("replayed create id = %v, want %v: the recording pair no longer shows request-id idempotency", replay["id"], create["id"])
	}

	early := read("capture-before-approval.json")
	details, ok := early["details"].([]any)
	if !ok || len(details) == 0 {
		t.Fatal("capture-before-approval.json carries no details")
	}
	first, ok := details[0].(map[string]any)
	if !ok || first["issue"] != "ORDER_NOT_APPROVED" {
		t.Fatalf("capture-before-approval issue = %v, want ORDER_NOT_APPROVED", details[0])
	}
}
