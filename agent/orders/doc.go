// Package orders is the only code in this repository that moves money.
//
// The reference agent makes the approved sandbox purchase here; the memory
// layer never does (developer rule 5). Correctness lives in the payment
// path, never in coordination aids:
//
//   - durable order state: proposed → approved → captured on the happy
//     path, or failed and refunded on the sad paths;
//   - a uniqueness rule on open orders per (shopper, need): a second order
//     for the same need while one is open is refused;
//   - a PayPal-Request-Id on every create and capture, stored on the order
//     and reused on retry, so PayPal dedupes retried calls;
//   - webhook handling idempotent on PayPal's event id: redelivery returns
//     the stored outcome and transitions nothing twice;
//   - approve-first enforced in code: Capture fails closed with
//     ErrNoApproval when no approval record exists, without calling the
//     transport, and a capture webhook for an unapproved order is refused
//     the same way. Approval is never assumed from PayPal state.
//
// Soft locks (see Service.InFlight) let a second agent notice a purchase
// already in flight. They are coordination aids only and are NEVER the
// correctness mechanism: the uniqueness rule above is what prevents the
// second purchase, with or without any lock.
//
// Purchases run through the project's own sandbox merchant. Service
// construction refuses a non-sandbox configuration, so no live credential
// can enter this path by accident.
package orders
