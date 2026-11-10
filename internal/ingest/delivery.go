package ingest

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nrynss/memento-emptor/internal/payments"
)

// Delivery is one raw webhook delivery as the transport received it: the
// headers that sign it and the exact body bytes that were posted. Ingest
// hands it to the verifier before it reads the event from the body.
type Delivery = payments.Delivery

// Verifier checks one raw delivery's signature before ingest reads it.
// WithVerifier replaces the default, which is the real payment-source
// adapter; tests inject a fake so no test reaches the network.
type Verifier interface {
	// Verify returns nil only when the delivery's signature verified.
	// Every other answer is an error and ingest refuses the delivery.
	Verify(ctx context.Context, d Delivery) error
}

// WithVerifier replaces the delivery gate. The default is the real
// payment-source adapter built from the environment; a test injects its
// own so the suite stays offline.
func WithVerifier(v Verifier) Option {
	return func(s *Service) {
		if v != nil {
			s.verifier = v
		}
	}
}

// HandleDelivery verifies one raw webhook delivery and ingests the event
// it carries, idempotent on the event id in its body. The event id is
// parsed first so malformed bodies are refused without a call; verification
// runs before the idempotency check, so a forged body that names an already
// stored event id is refused rather than answered as a duplicate.
//
// Every way a delivery can fail verification is one refusal: a tampered
// body, a signature that belongs to no delivery, a delivery signed for
// another webhook, and a verifier that cannot answer all return
// ErrBadSignature. The cause stays matchable underneath, with errors.Is.
func (s *Service) HandleDelivery(ctx context.Context, d Delivery) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("ingest: handle delivery: %w", err)
	}
	eventID, err := deliveryEventID(d.Body)
	if err != nil {
		return Result{}, err
	}
	if err := s.verifier.Verify(ctx, d); err != nil {
		// The refusal names neither the delivery nor the reason it failed:
		// a tampered body and a delivery PayPal never issued must not be
		// told apart by the answer. The cause stays matchable underneath.
		return Result{}, fmt.Errorf("%w: %w", ErrBadSignature, err)
	}
	return s.Handle(ctx, eventID)
}

// deliveryEventID reads the event id from a delivery body. It reads the id
// only; nothing else from the body reaches the layer.
func deliveryEventID(body []byte) (string, error) {
	var envelope struct {
		// ID is the payment source's event id, which keys idempotency.
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("ingest: delivery body: %w: %v", ErrMalformedEvent, err)
	}
	if envelope.ID == "" {
		return "", fmt.Errorf("ingest: delivery body: %w: no event id", ErrMalformedEvent)
	}
	return envelope.ID, nil
}
