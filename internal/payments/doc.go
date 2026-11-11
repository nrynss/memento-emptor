// Package payments gates and reads the payment source.
//
// The verifier is the only gate on the public webhook path: a delivery is
// a request body plus the transmission headers that sign it, and nothing
// from the body is read until the source's verify-webhook-signature
// endpoint answers SUCCESS for those exact bytes. Every other answer is a
// refusal; a transport or credential failure refuses too, so an outage
// cannot open the gate.
//
// The source reads the transaction records a verified delivery names: the
// stored webhook event, the checkout order and its line items, the capture,
// the refund and the dispute. The event's outcome and links are mapped into
// the layer's own shapes, so ingest never reads a source record directly.
// Order lines are joined to the layer's product items by a deployment
// catalog; a line the catalog does not know falls back to the order's own
// name, SKU and category rather than being dropped.
//
// The adapter calls the sandbox REST API. Credentials come from the
// environment (PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET, exported by
// tools/env.sh) and are never written, logged or returned in an error. The
// webhook subscription a delivery was signed for is discovered from the
// app's subscriptions: the one whose URL matches PUBLIC_URL's
// /webhooks/paypal path when that variable is set, or every subscription the
// app has. The source answers SUCCESS only for the subscription that signed
// the delivery, so a candidate list never widens what verifies.
//
// This package moves no money: every read is a GET, and orders, approval,
// capture and double-buy protection stay in the reference agent.
package payments
