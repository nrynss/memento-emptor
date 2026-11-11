package orders

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// The sentinels for order failures. Match them with errors.Is.
var (
	// ErrNeedBusy is returned when a second order is proposed for the
	// same (shopper, need) while one is still open. The open order's id
	// rides in the wrapped message; the caller waits for it or cancels
	// it instead of buying twice.
	ErrNeedBusy = errors.New("orders: an open order already covers this need")

	// ErrNoApproval is returned when a capture is attempted, or a capture
	// webhook arrives, without an approval record on the order. Capture
	// fails closed: the transport is never called and no state changes.
	ErrNoApproval = errors.New("orders: capture without an approval record")

	// ErrUnknownOrder is returned when an operation names an order the
	// service has never stored.
	ErrUnknownOrder = errors.New("orders: unknown order")

	// ErrMalformed is returned when a call carries an empty id or an
	// empty shopper or need key.
	ErrMalformed = errors.New("orders: empty id, shopper or need")

	// ErrBadState is returned when an operation does not fit the order's
	// current state: approving a captured order, capturing a refunded
	// one, refunding a proposed one, or a second capture event for an
	// already terminal order.
	ErrBadState = errors.New("orders: operation does not fit the order state")

	// ErrUnknownEvent is returned when a webhook carries an event type
	// the order service does not track.
	ErrUnknownEvent = errors.New("orders: unknown webhook event type")

	// ErrLiveMerchant is returned when the service is configured against
	// anything but the sandbox. Purchases are sandbox only.
	ErrLiveMerchant = errors.New("orders: non-sandbox merchant refused")
)

// State is the durable state of one order.
type State string

// The order states. Open orders are proposed or approved; everything else
// is closed and frees the (shopper, need) slot.
const (
	// StateProposed is a created order the shopper has not approved yet.
	StateProposed State = "proposed"
	// StateApproved is an order with a recorded shopper approval,
	// awaiting capture.
	StateApproved State = "approved"
	// StateCaptured is an order whose capture completed.
	StateCaptured State = "captured"
	// StateFailed is an order that can no longer move money: PayPal
	// denied the capture or reported an approval mismatch.
	StateFailed State = "failed"
	// StateRefunded is a captured order whose capture was refunded.
	StateRefunded State = "refunded"
)

// Open reports whether the state still holds the (shopper, need) slot:
// only proposed and approved orders are open.
func (s State) Open() bool {
	return s == StateProposed || s == StateApproved
}

// Order is one durable purchase attempt. The state machine is
// proposed → approved → captured, or failed and refunded: Approve moves
// proposed to approved, Capture moves approved to captured, a denied
// capture webhook moves an open order to failed, and a refund webhook
// moves a captured order to refunded. Nothing else transitions.
type Order struct {
	// OrderID is the checkout order id from the payment source.
	OrderID string
	// ShopperID identifies the shopper.
	ShopperID string
	// Need is the opaque need key the uniqueness rule holds: one open
	// order per (ShopperID, Need).
	Need string
	// Item is the purchased thing, from the order's line items.
	Item domain.Item
	// AmountMinor is the amount in minor units, such as cents.
	AmountMinor int64
	// Currency is the ISO currency code. The demo's amounts stay USD.
	Currency string
	// CustomID is the shopper-scoped reference that propagates across
	// the order, capture and refund records, so the layer can join them.
	CustomID string
	// State is the durable state.
	State State
	// CreateRequestID is the PayPal-Request-Id sent at create, reused on
	// every retry of the same create.
	CreateRequestID string
	// CaptureRequestID is the PayPal-Request-Id sent at capture, reused
	// on every retry of the same capture.
	CaptureRequestID string
	// CaptureID is the capture id, once captured.
	CaptureID string
	// ApprovedAt is when the shopper's approval was recorded. Its zero
	// value means no approval exists, and capture fails closed.
	ApprovedAt time.Time
}

// CreateParams carries one order creation to the transport.
type CreateParams struct {
	// RequestID is the PayPal-Request-Id for the create. The service
	// always sets one and reuses it on retry.
	RequestID string
	// ShopperID identifies the shopper.
	ShopperID string
	// Need is the opaque need key.
	Need string
	// CustomID is the join key propagated across the records.
	CustomID string
	// Item is the purchased thing.
	Item domain.Item
	// AmountMinor is the amount in minor units.
	AmountMinor int64
	// Currency is the ISO currency code.
	Currency string
}

// CreateResult is what the transport returns for a create.
type CreateResult struct {
	// OrderID is the checkout order id.
	OrderID string
	// Status is the source's order status, such as PAYER_ACTION_REQUIRED.
	Status string
}

// CaptureResult is what the transport returns for a capture.
type CaptureResult struct {
	// CaptureID is the capture id.
	CaptureID string
	// Status is the source's capture status, such as COMPLETED.
	Status string
}

// CheckoutTransport is the seam the real PayPal REST client implements:
// order create and capture, each carrying its PayPal-Request-Id. Every
// method takes ctx first. The fake in fake.go replays the recorded
// sandbox shapes offline.
type CheckoutTransport interface {
	// CreateOrder creates the checkout order, idempotent on
	// params.RequestID: the same request id returns the same order id.
	CreateOrder(ctx context.Context, params CreateParams) (CreateResult, error)
	// CaptureOrder captures an approved order, idempotent on requestID.
	// It reports an approval mismatch wrapping ErrNoApproval when the
	// order was never approved on the payment source's side.
	CaptureOrder(ctx context.Context, orderID, requestID string) (CaptureResult, error)
}

// Configures a Service.
type Config struct {
	// MerchantID is the project's own sandbox merchant the purchases
	// run through.
	MerchantID string
	// Sandbox must be true. Construction refuses anything else, so no
	// live credential can enter this path by accident.
	Sandbox bool
}

// Service holds durable order state: the orders, the create request ids
// behind retried creates, the webhook event ids behind redeliveries, and
// the open (shopper, need) slots behind the uniqueness rule. A Service is
// safe for concurrent use.
//
// Durability here is in-process maps behind a mutex. The production swap
// is a row store with the same keys: order id, create request id,
// capture request id, webhook event id, and a unique constraint on open
// (shopper, need). The keys are the correctness; the maps are the stand-in.
type Service struct {
	transport CheckoutTransport

	mu        sync.Mutex
	orders    map[string]*Order
	byNeed    map[string]string
	byRequest map[string]string
	seen      map[string]string
	seq       int
}

// NewService builds a Service over the transport. It refuses a
// non-sandbox configuration with ErrLiveMerchant.
func NewService(transport CheckoutTransport, cfg Config) (*Service, error) {
	if !cfg.Sandbox {
		return nil, fmt.Errorf("%w", ErrLiveMerchant)
	}
	if cfg.MerchantID == "" {
		return nil, fmt.Errorf("%w: empty merchant id", ErrMalformed)
	}
	return &Service{
		transport: transport,
		orders:    make(map[string]*Order),
		byNeed:    make(map[string]string),
		byRequest: make(map[string]string),
		seen:      make(map[string]string),
	}, nil
}

// needKey joins the uniqueness scope: one open order per shopper need.
func needKey(shopper, need string) string {
	return shopper + "\x00" + need
}

// Propose creates the checkout order and stores it as proposed. A retried
// create that reuses a stored PayPal-Request-Id returns the stored order
// with duplicate set and never touches the transport again, so PayPal and
// the local store agree on idempotency. A second order for the same
// (shopper, need) while one is open is refused with ErrNeedBusy. An
// empty request id is minted once and reported back on the order, so the
// caller can retry with the stored id.
func (s *Service) Propose(ctx context.Context, shopper, need string, item domain.Item, amountMinor int64, currency, requestID string) (*Order, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, fmt.Errorf("orders: propose: %w", err)
	}
	if shopper == "" || need == "" {
		return nil, false, fmt.Errorf("orders: propose: %w", ErrMalformed)
	}
	s.mu.Lock()
	if requestID == "" {
		s.seq++
		requestID = fmt.Sprintf("memento-%s-%s-%d", shopper, need, s.seq)
	}
	// This pre-check fails the common sequential case early, without a
	// transport call. The binding check reruns inside create under the
	// store lock, which is what concurrent proposes serialize on: two
	// concurrent proposes with distinct request ids can still reach the
	// payment source twice and leave a stray proposed order there, but
	// the loser never approves or captures it, so no money moves.
	if id, ok := s.byRequest[requestID]; ok {
		stored := *s.orders[id]
		s.mu.Unlock()
		return &stored, true, nil
	}
	if id, ok := s.byNeed[needKey(shopper, need)]; ok {
		if s.orders[id].State.Open() {
			s.mu.Unlock()
			return nil, false, fmt.Errorf("orders: propose need %q for shopper %q: %w: open order %q", need, shopper, ErrNeedBusy, id)
		}
	}
	s.seq++
	custom := fmt.Sprintf("memento-%s-%d", shopper, s.seq)
	params := CreateParams{
		RequestID: requestID, ShopperID: shopper, Need: need,
		CustomID: custom, Item: item,
		AmountMinor: amountMinor, Currency: currency,
	}
	s.mu.Unlock()
	return s.create(ctx, params)
}

// create calls the transport and stores the proposed order. It is the
// single write path for new orders.
func (s *Service) create(ctx context.Context, params CreateParams) (*Order, bool, error) {
	result, err := s.transport.CreateOrder(ctx, params)
	if err != nil {
		return nil, false, fmt.Errorf("orders: create: %w", err)
	}
	if result.OrderID == "" {
		return nil, false, fmt.Errorf("orders: create: %w: transport returned no order id", ErrMalformed)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.byRequest[params.RequestID]; ok {
		stored := *s.orders[id]
		return &stored, true, nil
	}
	// The open-need check runs again under this lock, so two concurrent
	// proposes for the same need cannot both store: the loser sees the
	// winner's open slot and reports ErrNeedBusy.
	if id, ok := s.byNeed[needKey(params.ShopperID, params.Need)]; ok {
		if s.orders[id].State.Open() {
			return nil, false, fmt.Errorf("orders: propose need %q for shopper %q: %w: open order %q", params.Need, params.ShopperID, ErrNeedBusy, id)
		}
	}
	order := &Order{
		OrderID: result.OrderID, ShopperID: params.ShopperID,
		Need: params.Need, Item: params.Item,
		AmountMinor: params.AmountMinor, Currency: params.Currency,
		CustomID: params.CustomID, State: StateProposed,
		CreateRequestID: params.RequestID,
	}
	s.orders[result.OrderID] = order
	s.byRequest[params.RequestID] = result.OrderID
	s.byNeed[needKey(params.ShopperID, params.Need)] = result.OrderID
	stored := *order
	return &stored, false, nil
}

// Approve records the shopper's approval for a proposed order. Only a
// proposed order can be approved; anything else reports ErrBadState.
func (s *Service) Approve(ctx context.Context, orderID string) (*Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("orders: approve: %w", err)
	}
	if orderID == "" {
		return nil, fmt.Errorf("orders: approve: %w", ErrMalformed)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("orders: approve %q: %w", orderID, ErrUnknownOrder)
	}
	if order.State != StateProposed {
		return nil, fmt.Errorf("orders: approve %q in state %q: %w", orderID, order.State, ErrBadState)
	}
	order.State = StateApproved
	order.ApprovedAt = time.Now().UTC()
	stored := *order
	return &stored, nil
}

// Capture captures an approved order. Without an approval record it
// fails closed with ErrNoApproval: the transport is never called and the
// order does not move. The capture PayPal-Request-Id is minted once per
// order and reused on retry, so a retried capture is one capture. A
// payment-source approval mismatch marks the order failed and reports
// ErrNoApproval.
func (s *Service) Capture(ctx context.Context, orderID string) (*Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("orders: capture: %w", err)
	}
	if orderID == "" {
		return nil, fmt.Errorf("orders: capture: %w", ErrMalformed)
	}
	s.mu.Lock()
	order, ok := s.orders[orderID]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("orders: capture %q: %w", orderID, ErrUnknownOrder)
	}
	if order.State != StateApproved || order.ApprovedAt.IsZero() {
		s.mu.Unlock()
		return nil, fmt.Errorf("orders: capture %q without approval: %w", orderID, ErrNoApproval)
	}
	if order.CaptureRequestID == "" {
		s.seq++
		order.CaptureRequestID = fmt.Sprintf("memento-capture-%s-%d", orderID, s.seq)
	}
	requestID := order.CaptureRequestID
	s.mu.Unlock()

	result, err := s.transport.CaptureOrder(ctx, orderID, requestID)
	if err != nil {
		if errors.Is(err, ErrNoApproval) {
			s.mu.Lock()
			defer s.mu.Unlock()
			if current, ok := s.orders[orderID]; ok && current.State == StateApproved {
				current.State = StateFailed
			}
		}
		return nil, fmt.Errorf("orders: capture %q: %w", orderID, err)
	}
	if result.CaptureID == "" {
		return nil, fmt.Errorf("orders: capture %q: %w: transport returned no capture id", orderID, ErrMalformed)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("orders: capture %q: %w", orderID, ErrUnknownOrder)
	}
	if current.State != StateApproved {
		return nil, fmt.Errorf("orders: capture %q in state %q: %w", orderID, current.State, ErrBadState)
	}
	current.State = StateCaptured
	current.CaptureID = result.CaptureID
	stored := *current
	return &stored, nil
}

// Webhook outcomes the order service tracks.
const (
	// EventCaptureCompleted is the completed-capture delivery.
	EventCaptureCompleted = "PAYMENT.CAPTURE.COMPLETED"
	// EventCaptureDenied is the denied-capture delivery.
	EventCaptureDenied = "PAYMENT.CAPTURE.DENIED"
	// EventCaptureRefunded is the refunded-capture delivery.
	EventCaptureRefunded = "PAYMENT.CAPTURE.REFUNDED"
	// EventDisputeCreated is the dispute-opened delivery. It changes no
	// order state; it is recorded so redelivery stays idempotent.
	EventDisputeCreated = "CUSTOMER.DISPUTE.CREATED"
)

// HandleWebhook applies one verified webhook delivery to its order,
// idempotent on the payment source's event id: redelivery returns the
// stored order with duplicate set and transitions nothing twice.
//
// A capture-completed delivery for an order with no approval record is
// refused with ErrNoApproval and changes nothing: approve-first is
// enforced on the webhook path exactly as on the capture path.
func (s *Service) HandleWebhook(ctx context.Context, eventID, eventType, orderID, transactionID string) (*Order, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, fmt.Errorf("orders: webhook: %w", err)
	}
	if eventID == "" || orderID == "" {
		return nil, false, fmt.Errorf("orders: webhook: %w", ErrMalformed)
	}
	switch eventType {
	case EventCaptureCompleted, EventCaptureDenied, EventCaptureRefunded, EventDisputeCreated:
	default:
		return nil, false, fmt.Errorf("orders: webhook event %q type %q: %w", eventID, eventType, ErrUnknownEvent)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.seen[eventID]; ok {
		stored := *s.orders[id]
		return &stored, true, nil
	}
	order, ok := s.orders[orderID]
	if !ok {
		return nil, false, fmt.Errorf("orders: webhook event %q for order %q: %w", eventID, orderID, ErrUnknownOrder)
	}

	switch eventType {
	case EventDisputeCreated:
		s.seen[eventID] = orderID
		stored := *order
		return &stored, false, nil
	case EventCaptureDenied:
		if !order.State.Open() {
			return nil, false, fmt.Errorf("orders: webhook event %q for order %q in state %q: %w", eventID, orderID, order.State, ErrBadState)
		}
		order.State = StateFailed
		delete(s.byNeed, needKey(order.ShopperID, order.Need))
		s.seen[eventID] = orderID
		stored := *order
		return &stored, false, nil
	case EventCaptureCompleted:
		if order.ApprovedAt.IsZero() {
			return nil, false, fmt.Errorf("orders: webhook event %q for unapproved order %q: %w", eventID, orderID, ErrNoApproval)
		}
		if order.State == StateCaptured {
			// The payment source confirms what Capture already stored:
			// the local capture call and the webhook agree, so the
			// delivery is recorded and changes nothing.
			if transactionID != "" && order.CaptureID != "" && transactionID != order.CaptureID {
				return nil, false, fmt.Errorf("orders: webhook event %q for order %q names capture %q, stored %q: %w", eventID, orderID, transactionID, order.CaptureID, ErrBadState)
			}
			if transactionID != "" {
				order.CaptureID = transactionID
			}
			s.seen[eventID] = orderID
			stored := *order
			return &stored, false, nil
		}
		if order.State != StateApproved {
			return nil, false, fmt.Errorf("orders: webhook event %q for order %q in state %q: %w", eventID, orderID, order.State, ErrBadState)
		}
		order.State = StateCaptured
		if transactionID != "" {
			order.CaptureID = transactionID
		}
		delete(s.byNeed, needKey(order.ShopperID, order.Need))
		s.seen[eventID] = orderID
		stored := *order
		return &stored, false, nil
	case EventCaptureRefunded:
		if order.State == StateRefunded {
			// Redundant confirmation of a stored refund: recorded,
			// nothing transitions twice.
			s.seen[eventID] = orderID
			stored := *order
			return &stored, false, nil
		}
		if order.State != StateCaptured {
			return nil, false, fmt.Errorf("orders: webhook event %q for order %q in state %q: %w", eventID, orderID, order.State, ErrBadState)
		}
		order.State = StateRefunded
		s.seen[eventID] = orderID
		stored := *order
		return &stored, false, nil
	}
	return nil, false, fmt.Errorf("orders: webhook event %q type %q: %w", eventID, eventType, ErrUnknownEvent)
}

// InFlight reports whether an open order already covers the shopper's
// need, and if so which order id. It is a coordination aid: it lets a
// second agent notice a purchase in flight and wait instead of racing
// it. It is NEVER the correctness mechanism. The uniqueness rule in
// Propose is what prevents the second purchase, and it holds whether
// or not any caller consulted InFlight first.
func (s *Service) InFlight(shopper, need string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byNeed[needKey(shopper, need)]
	if !ok {
		return "", false
	}
	order, ok := s.orders[id]
	if !ok || !order.State.Open() {
		return "", false
	}
	return id, true
}

// Get returns a copy of the stored order, or ErrUnknownOrder.
func (s *Service) Get(ctx context.Context, orderID string) (*Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("orders: get: %w", err)
	}
	if orderID == "" {
		return nil, fmt.Errorf("orders: get: %w", ErrMalformed)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("orders: get %q: %w", orderID, ErrUnknownOrder)
	}
	stored := *order
	return &stored, nil
}
