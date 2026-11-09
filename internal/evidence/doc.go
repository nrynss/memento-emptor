// Package evidence holds the raw-evidence side of the memory layer: the
// documents the evidence store keeps, the product matching that answers
// whether a new product shares the cut of the items behind a belief, and
// the store itself.
//
// The graph holds only short, keyed stubs that carry a document id.
// Prose — shopper messages, classified reasons, product text — lives in
// the store and never enters a graph write. The store takes its embedding
// configuration as a domain.EmbedderContract and refuses a contract it was
// not built under until an explicit re-embed.
package evidence
