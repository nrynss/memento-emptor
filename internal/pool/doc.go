// Package pool reads the pooled product-fit belief: the settled counts for
// one product line and size, and the status the rule decides from them.
//
// The pool is an aggregation over the fit rows the layer writes at ingest,
// not a belief in the shopper's graph. So a read is one line-and-size
// aggregation over settled rows, and it answers in counts, sizes and reason
// categories, never a shopper, a row or free text. A purchase still inside
// its return window, and a refund with no reason yet, are stored but settle
// nothing, so they count toward neither the denominator nor a direction.
//
// The status is not computed here. The counts are handed to the rule in
// domain, so the reader, the ledger and the merchant app all read one
// definition of Building, Earned and Out of true. The reader also reads the
// last window of event time, because Out of true is judged on that window
// alone.
//
// The search deployment behind the index sits behind Index. The layer names
// only that interface; this package carries the HTTP adapter, keeps its
// connections alive and allows a cold connection the 20 s handshake T0.12
// measured. When the index cannot be reached, the error says so with
// ErrUnavailable: recall carries on without the pool and never reports an
// empty one.
package pool
