package payments

import "errors"

// The sentinels for verification failures. Match them with errors.Is.
var (
	// ErrMalformedDelivery is returned when a delivery cannot be checked
	// at all: a required transmission header is missing, or the body is
	// empty or not JSON. The verifier refuses without calling the source.
	ErrMalformedDelivery = errors.New("payments: delivery is missing its signed fields")

	// ErrNotVerified is returned when the source's verify endpoint answers
	// anything but SUCCESS. A tampered body, a signature that belongs to
	// no delivery, and a delivery for another webhook subscription all
	// land here, and the caller refuses them alike.
	ErrNotVerified = errors.New("payments: the delivery did not verify")

	// ErrUnavailable is returned when verification could not be asked for:
	// credentials are not set, the token grant or the verify call failed,
	// the reply was not readable, or the endpoint answered with an error
	// status. The caller refuses the delivery: a verifier that cannot
	// answer does not open the gate.
	ErrUnavailable = errors.New("payments: verification is unavailable")

	// ErrNoWebhook is returned when no subscription can be named: the
	// configured public URL matches none, or the app has no subscription.
	// Verification needs at least one subscription id, so the adapter
	// refuses rather than guess.
	ErrNoWebhook = errors.New("payments: no webhook subscription")
)
