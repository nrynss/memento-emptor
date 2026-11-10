// Package payments verifies webhook deliveries against the payment
// source's signature endpoint before the layer reads them.
//
// The verifier is the only gate on the public webhook path: a delivery is
// a request body plus the transmission headers that sign it, and nothing
// from the body is read until the source's verify-webhook-signature
// endpoint answers SUCCESS for those exact bytes. Every other answer is a
// refusal; a transport or credential failure refuses too, so an outage
// cannot open the gate.
//
// The adapter calls the sandbox REST API. Credentials come from the
// environment (PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET, exported by
// tools/env.sh) and are never written, logged or returned in an error.
// The webhook subscription a delivery was signed for is discovered from the
// app's subscriptions: the one whose URL matches PUBLIC_URL's
// /webhooks/paypal path when that variable is set, or every subscription the
// app has. The source answers SUCCESS only for the subscription that signed
// the delivery, so a candidate list never widens what verifies.
//
// This package moves no money and reads no transaction. Turning a verified
// delivery into evidence rows belongs to ingest; resolving its transaction
// records belongs to the payment source that also implements those reads.
package payments
