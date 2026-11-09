package orders

import (
	"context"
	"fmt"
	"sync"
)

// FakeCheckout is an offline CheckoutTransport for tests. It replays the
// recorded sandbox shapes from testdata/paypal/ with fixed placeholder
// ids: create is idempotent on the request id (the replayed recording
// returns the same order id for the same PayPal-Request-Id), and capture
// before approval fails with the recorded ORDER_NOT_APPROVED shape. It
// makes no network call, reads no key, and spends nothing.
type FakeCheckout struct {
	mu sync.Mutex
	// byRequest maps a create PayPal-Request-Id to its order id, so a
	// retried create returns the same order.
	byRequest map[string]string
	// approved marks the orders whose shopper completed the approval
	// step. Capture on any other order fails closed.
	approved map[string]bool
	// byCapture maps a capture PayPal-Request-Id to its capture id, so
	// a retried capture returns the same capture.
	byCapture map[string]string
	// CreateCalls counts CreateOrder calls, so tests can prove a
	// fail-closed capture never reached the transport.
	CreateCalls int
	// CaptureCalls counts CaptureOrder calls.
	CaptureCalls int
	seq          int
}

// NewFakeCheckout builds an empty FakeCheckout.
func NewFakeCheckout() *FakeCheckout {
	return &FakeCheckout{
		byRequest: make(map[string]string),
		approved:  make(map[string]bool),
		byCapture: make(map[string]string),
	}
}

// CreateOrder replays the recorded wallet-order create: the first call
// for a request id mints an order in status PAYER_ACTION_REQUIRED; a
// retry with the same request id returns the same order id, mirroring
// testdata/paypal/order-wallet-replay-same-request-id.json.
func (f *FakeCheckout) CreateOrder(ctx context.Context, params CreateParams) (CreateResult, error) {
	if err := ctx.Err(); err != nil {
		return CreateResult{}, fmt.Errorf("orders fake: create: %w", err)
	}
	if params.RequestID == "" {
		return CreateResult{}, fmt.Errorf("orders fake: create: %w: empty request id", ErrMalformed)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CreateCalls++
	if id, ok := f.byRequest[params.RequestID]; ok {
		return CreateResult{OrderID: id, Status: "PAYER_ACTION_REQUIRED"}, nil
	}
	f.seq++
	id := fmt.Sprintf("ORDER-TEST-%d", f.seq)
	f.byRequest[params.RequestID] = id
	return CreateResult{OrderID: id, Status: "PAYER_ACTION_REQUIRED"}, nil
}

// ApproveOrder marks the order approved, modelling the shopper's click
// on the approval URL. No code in the service reads the sandbox site
// logins; the fake stands in for the human step.
func (f *FakeCheckout) ApproveOrder(orderID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.approved[orderID] = true
}

// CaptureOrder replays the recorded capture shapes: an unapproved order
// fails with the ORDER_NOT_APPROVED mismatch from
// testdata/paypal/capture-before-approval.json, wrapping ErrNoApproval
// so the service marks the order failed; an approved order completes,
// idempotent on the capture request id.
func (f *FakeCheckout) CaptureOrder(ctx context.Context, orderID, requestID string) (CaptureResult, error) {
	if err := ctx.Err(); err != nil {
		return CaptureResult{}, fmt.Errorf("orders fake: capture: %w", err)
	}
	if orderID == "" || requestID == "" {
		return CaptureResult{}, fmt.Errorf("orders fake: capture: %w: empty order or request id", ErrMalformed)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CaptureCalls++
	if id, ok := f.byCapture[requestID]; ok {
		return CaptureResult{CaptureID: id, Status: "COMPLETED"}, nil
	}
	if !f.approved[orderID] {
		return CaptureResult{}, fmt.Errorf("orders fake: capture %q: %w: ORDER_NOT_APPROVED: payer has not yet approved the order", orderID, ErrNoApproval)
	}
	f.seq++
	id := fmt.Sprintf("CAPTURE-TEST-%d", f.seq)
	f.byCapture[requestID] = id
	return CaptureResult{CaptureID: id, Status: "COMPLETED"}, nil
}
