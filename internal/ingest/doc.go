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
// dispute carries the source's reason code, which maps through the frozen
// dispute table to a reason category on arrival.
//
// Each delivery stands alone: a refund read needs only its order and
// refund reads, never the capture delivery, so a refund that arrives
// before its capture loses nothing and a later capture adds its own row
// without double-counting.
//
// A delivery that touches a purchase also writes that purchase's fit row
// through a FitRecorder, from the same event: a capture creates the row
// and a refund marks it returned. A dispute writes no fit row, because it
// says the item did not arrive, which is about the seller and not how the
// product fits.
//
// Verification itself happens in the payment source's VerifyWebhook
// call, which replays the source's signature check. The HTTP transport
// that extracts the event id from a delivery body belongs to the server
// layer; this package owns verification-to-row.
package ingest
