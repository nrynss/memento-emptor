package domain

import "errors"

// ErrEmptyEmbedderKind is returned when an embedding contract names no kind.
var ErrEmptyEmbedderKind = errors.New("domain: embedder contract names no kind")

// EmbedderContract is the swappable embedding configuration the evidence
// graph and the evidence store share. It carries no model name, no
// dimensions, no endpoint and no key: the project direction is native
// embeddings served by the evidence store itself, and every later session
// must match the contract it was built on or re-embed. Concrete backends
// live outside this package, behind this interface.
type EmbedderContract interface {
	// EmbeddingKind names the contract family so a session can refuse a
	// store built under a different one.
	EmbeddingKind() string
	// Validate reports whether the contract is usable.
	Validate() error
}
