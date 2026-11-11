// Package fakes is the offline seam every later phase builds on.
//
// The real clients for the payment source, the reason classifier, the
// evidence store and the evidence graph arrive in P2. They implement the
// interfaces declared here (PaymentSource, ReasonClassifier,
// EvidenceStore, EvidenceGraph); callers code to those interfaces, so the
// swap from fake to real touches no caller. This package owns the
// interfaces until P2 lands, and P2 adopts them as-is or records the
// difference in its handoff.
//
// Every fake replays a P0 recording, never the network. The payment fake
// serves the ledger story's ids in the shapes proved by the PayPal
// recordings; the classifier serves the thirteen labelled Jev answers
// verbatim; the store serves the hybrid search legs measured on the live
// deployment; the graph applies the promotion bars proved on the seeded
// session. A request that names nothing any recording or the committed
// story covers fails with ErrNoRecording: a fake never invents an answer.
//
// The replay in replay.go walks the demo shopper's whole story, from the
// first refund webhook to the final ledger state, with no key and no
// network. TestTestReplayOffline proves it: keys unset, proxy broken, and
// it still passes in seconds.
package fakes
