package evidence

import "errors"

// ErrEmptyText is returned when a document carries no text. An empty
// message, reason or product description stores nothing.
var ErrEmptyText = errors.New("evidence: document carries no text")

// ErrEmptyQuery is returned when a search or ledger lookup names no
// query. An empty query matches everything, so it matches nothing loudly
// instead.
var ErrEmptyQuery = errors.New("evidence: search names no query")

// ErrNoDocID is returned when a graph stub carries no document id. A stub
// without an id points at no stored prose.
var ErrNoDocID = errors.New("evidence: graph stub carries no document id")

// ErrStubTooLong is returned when a graph stub exceeds the short keyed
// bound. A stub that long is prose trying to enter the graph. Match it
// with errors.Is.
var ErrStubTooLong = errors.New("evidence: graph stub exceeds the short keyed bound")

// ErrContractMismatch is returned when the caller presents an embedding
// contract the store was not built under. Index and search refuse until
// an explicit re-embed. Match it with errors.Is.
var ErrContractMismatch = errors.New("evidence: embedding contract does not match the store")

// ErrInvalidContract is returned when the embedding contract is nil,
// fails its own validation, or names no kind. Match it with errors.Is.
var ErrInvalidContract = errors.New("evidence: embedding contract is not usable")

// ErrBadJournalShopper is returned when a shopper id cannot name a
// journal file: it is empty or carries a character outside the file-safe
// set. Match it with errors.Is.
var ErrBadJournalShopper = errors.New("evidence: shopper id cannot name a journal file")

// ErrBadJournalDir is returned when the journal directory is empty, missing
// or not a directory. Match it with errors.Is.
var ErrBadJournalDir = errors.New("evidence: missing journal directory")

// ErrCorruptJournal is returned when a stored evidence journal does not
// decode or names a layout version this code does not know. It is refused
// rather than guessed at. Match it with errors.Is.
var ErrCorruptJournal = errors.New("evidence: stored journal does not decode")
