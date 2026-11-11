// Package ingest turns verified payment-source deliveries into durable
// evidence rows.
//
// The payment source calls back once per outcome: a refund, a dispute or
// a capture. Every delivery carries the source's event id, and every
// delivery is verified before it is read. Ingest keys its rows on that
// event id, so a redelivered event returns the stored rows instead of
// writing them twice.
//
// Line items live on the order read, never on the delivery: capture
// records carry no items, so ingest joins each outcome to its order and
// writes one row per line item. A refund record carries no reason, so a
// refund row keeps a nil reason and is also filed as a pending question
// for the shopper to answer; it counts toward nothing meanwhile. A
// dispute carries the source's reason code, which the frozen dispute table
// maps to a reason category on arrival; a code the table does not name, or
// no code at all, keeps no category and counts toward nothing.
//
// Each delivery stands alone: a refund read needs only its order and
// refund reads, never the capture delivery, so a refund that arrives
// before its capture loses nothing and a later capture adds its own row
// without double-counting.
//
// A delivery that touches a purchase also writes that purchase's fit row
// through a FitRecorder, from the same event: a capture creates the row
// and a refund marks it returned. A dispute writes no fit row, because it
// is about the seller and not how the product fits.
//
// Verification itself happens before a delivery changes anything.
// HandleDelivery takes the transport's raw bytes, parses the event id so
// malformed bodies are refused without a call, asks the delivery gate
// (the real signature verifier by default) to check them, and only then
// stores rows. A delivery that does not verify is
// refused with ErrBadSignature and stores nothing, whether its body was
// tampered with, its signature belongs to no delivery, or the verifier
// could not answer. Handle names an event id directly and is the recorded
// path; the HTTP transport that builds a Delivery belongs to the server
// layer, and this package owns verification-to-row.
package ingest
