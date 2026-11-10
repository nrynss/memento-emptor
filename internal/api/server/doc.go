// Package server is the memory layer's running service. It serves the
// frozen interface in internal/api/contract over MCP and over plain HTTP,
// composed from the real layer pieces: ingest for payment outcomes,
// reasons for classification, the evidence store for raw texts, and the
// memory writer and reader for beliefs. Behind the fakes' seams it runs
// offline: tests drive it on the recording fakes with no key and no
// network, and the live swap replaces the fakes behind the same seams.
//
// Every interface call is logged with its shopper and trace id, so the
// ledger can show that a recommendation followed a recall. A contradicted
// belief never leaves recall without its override: the response assembler
// fails closed instead of serving one.
//
// Recall resolves the pivot's five steps — correction, own evidence, the
// pooled product fit adjusted from the usual size, statement, nothing —
// in internal/precedence, and serves every item with the step it came
// from, overridden or not. Corrections with no belief stand on their own
// in a per-shopper journal (internal/memory), persisted across restarts
// by internal/standing and declared against a matching belief when one
// is written later. The per-shopper fit writers behind product_fit and
// set_pooling resolve every purchase's line through the one catalog
// source, so the ingest-time capture and the later classification agree
// on the row they pool. The layer trusts the edge's X-Memento-* headers
// and never re-reads Authorization; the call log stays agent-only.
package server
