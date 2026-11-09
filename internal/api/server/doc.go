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
package server
