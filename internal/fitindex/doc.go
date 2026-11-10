// Package fitindex writes the pooled product-fit rows: one structured
// document per purchase, beside the write that records the shopper's own
// memory, from the same event.
//
// The row is the only input to the pooled product-fit belief. Nothing
// reads a shopper's memory to build the pool, and the pool carries counts,
// sizes and reason categories, never a shopper. So a row carries the
// product line, the size, the purchase state, the classified reason and
// its direction, the feedback verdict, the settled rule's answer, a salted
// per-line shopper hash, the transaction hash, the seeded mark and the
// event times. It carries no prose, no order detail and no shopper
// identity.
//
// One purchase is one document, keyed by the transaction hash, and each
// event replaces the whole document at the purchase state's rank with the
// event time inside the rank. A later state always wins, a late webhook
// cannot undo it, and a replay carries the same version so it rewrites the
// same row. Only settled purchases count: a return with a classified
// reason, fit feedback, or an item kept past its return window. A refund
// with no reason is stored and unsettled.
//
// The store behind the index sits behind Index. The layer names only that
// interface; this package carries the adapter.
package fitindex
