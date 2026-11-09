package ingest

import "errors"

// The sentinels for ingest failures. Match them with errors.Is.
var (
	// ErrBadSignature is returned when a delivery fails the payment
	// source's verification. The delivery is rejected and nothing is
	// stored.
	ErrBadSignature = errors.New("ingest: delivery failed verification")

	// ErrUnknownEvent is returned when a verified delivery carries an
	// outcome kind ingest does not store: only refunds, disputes and
	// captures become evidence rows.
	ErrUnknownEvent = errors.New("ingest: unknown event type")

	// ErrMalformedEvent is returned when a delivery names no transaction
	// to read: the join id its outcome kind needs is empty.
	ErrMalformedEvent = errors.New("ingest: delivery names no transaction")

	// ErrNoLineItems is returned when the order read behind a delivery
	// carries no line items. Evidence needs the item the outcome is
	// about; a delivery without one stores nothing.
	ErrNoLineItems = errors.New("ingest: order carries no line items")
)
