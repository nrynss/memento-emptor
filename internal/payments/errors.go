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

	// ErrUnavailable is returned when the source could not be asked:
	// credentials are not set, the token grant failed, a verify or read
	// call could not be made, the reply was not readable, or the endpoint
	// answered with an error status. The caller refuses the delivery or
	// the read: a source that cannot answer does not open the gate and
	// does not invent a record.
	ErrUnavailable = errors.New("payments: the source is unavailable")

	// ErrNoWebhook is returned when no subscription can be named: the
	// configured public URL matches none, or the app has no subscription.
	// Verification needs at least one subscription id, so the adapter
	// refuses rather than guess.
	ErrNoWebhook = errors.New("payments: no webhook subscription")

	// ErrUnknownEvent is returned when a verified delivery names an event
	// type the source does not map to an outcome. Only completed captures,
	// refunded captures and created disputes become evidence; every other
	// type is refused instead of stored.
	ErrUnknownEvent = errors.New("payments: unknown webhook event type")

	// ErrNotFound is returned when a read names a record the source does
	// not hold. The caller refuses the delivery or the lookup; it never
	// invents a record.
	ErrNotFound = errors.New("payments: the source has no such record")

	// ErrUnauthorized is returned when the source refuses the app's
	// credentials for a read. The failure is the source's answer, never a
	// reason to retry with a different identity.
	ErrUnauthorized = errors.New("payments: the source refused the credentials")

	// ErrMalformedRecord is returned when the source answers with a record
	// the adapter cannot map: unreadable JSON, a missing id, or a link the
	// record needs and does not carry. Nothing is stored from a record the
	// adapter cannot describe.
	ErrMalformedRecord = errors.New("payments: the source returned an unreadable record")
)
