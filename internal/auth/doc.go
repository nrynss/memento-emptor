// Package auth authenticates every caller of the layer and bounds what each
// one may reach. It sits in front of every route the edge serves, and it
// changes nothing about the tools' contract: a caller proves itself with an
// Authorization: Bearer header, and a tool request keeps exactly the fields
// its schema pins.
//
// # Caller kinds
//
// Three kinds exist, one per audience in product.md.
//
//   - An agent key belongs to one of the seller's shopping agents. It is
//     bound to the shoppers that agent serves. A tool call that names a
//     shopper outside that list is refused with the same answer as an
//     unknown key, so the endpoint cannot be used to find out who exists.
//   - A shopper link token belongs to one shopper, and reaches only that
//     shopper's ledger and that shopper's own writes: a correction,
//     feedback, a reason, their pending questions, and the pooling choice.
//     It never reaches another shopper's memory.
//   - A merchant key belongs to the seller's merchant app. It reaches
//     product_fit and the MCP handshake, which reads no memory, and nothing
//     else.
//
// The webhook route is public and carries no key: the layer accepts only a
// delivery its payment source verifies, as ingest already built it.
// /healthz is public too, and answers no state.
//
// # Sign-in for shoppers, and its limits
//
// For the hackathon a shopper signs in with a per-shopper link token that
// the seller issues: the ledger link carries the token, the ledger app puts
// it in the Authorization header, and the layer treats it as that shopper's
// identity. The demo shopper's token is a sandbox credential and is safe to
// publish on the judges' page. The limits, stated plainly:
//
//   - It is a bearer credential. Anyone holding the link can read that
//     shopper's ledger and write that shopper's correction, feedback and
//     pooling choice until the token is revoked.
//   - It does not expire. Revocation is a configuration change: remove the
//     entry or mark it revoked, then restart, because the ring is read once
//     at boot.
//   - It is not the shopper's payment identity and proves nothing about it.
//     A seller running the layer for real shoppers replaces this with the
//     seller's own sign-in and issues the same principal from it. The
//     stored keys still follow the shopper's payment identity through
//     internal/identity; this package decides who may ask, not whose rows
//     are written.
//   - One token names one shopper. A shopper with two identities holds two
//     tokens.
//
// # Keys are stored hashed
//
// The deployment holds the SHA-256 digest of each token, never the token
// itself, so a leaked environment cannot be replayed as a key. A token is
// minted by NewToken from 32 random bytes and carries a kind prefix; the
// seller hands the token to the caller once and puts HashToken(token) in
// the deployment's configuration. Authentication hashes the whole token it
// is offered, prefix included, and compares digests in constant time, so a
// token's kind prefix is a label and never the decision.
//
// The digest is unsalted because it hashes a high-entropy secret, not a
// guessable password: 32 random bytes have no offline dictionary. A token
// typed by hand at lower entropy loses that property, so IssueKey is the
// only supported way to mint one.
//
// # Where the keys come from
//
// The ring is built once at boot from three environment variables, which a
// deployment fills from its platform secrets and a workstation fills from
// the system keyring. The values are never written to a file in this
// repository and never printed.
//
//	MEMENTO_AGENT_KEYS      agent keys
//	MEMENTO_SHOPPER_TOKENS  shopper link tokens
//	MEMENTO_MERCHANT_KEYS   merchant app keys
//
// Each variable holds zero or more entries separated by whitespace, each
// entry with four or five |-separated fields:
//
//		<key id>|<sha256 hex>|<seller>|<shoppers>|<flags>
//
//	  - The key id names the key in logs, headers and revocation notes. It is
//	    unique across the whole ring, so an agent key and a shopper token
//	    cannot share one.
//	  - The digest is the lowercase hex SHA-256 of the token, 64 characters.
//	    Uppercase is accepted and means the same digest.
//	  - The seller is the seller whose layer issued the key. It travels to
//	    the layer as a header and is part of what the key means.
//	  - The shoppers field is a comma-separated list for an agent (at least
//	    one), exactly one shopper for a shopper token, and - for a merchant.
//	  - The flags field is - or absent for a live key, and revoked for a key
//	    that must stop working but keep its record. A revoked key is refused
//	    with the same answer as an unknown one.
//
// A missing variable is not an error: the ring then holds no key of that
// kind and refuses every caller of that kind. A malformed entry is an
// error, and the edge refuses to start, because a typo in a binding must
// not silently widen or narrow access.
//
// # Refusals
//
// Every failed authentication answers with one fixed 401: no key, an
// unknown key, a revoked key, a bad scheme, and a key naming a shopper it
// is not bound to. The body, status and headers are identical, so the
// answer never says whether a key exists, whether a shopper exists, or
// which of the two was wrong. A caller that is authenticated but asks for a
// tool its kind may not use gets a 403 instead; that tells the holder of
// the key only that the key it already holds is valid.
//
// # The surface
//
// Surface lists every route the edge serves, and the guard is mounted on
// all of them. Everything but /healthz and the webhook path refuses a
// request without a key.
//
// The MCP handshake (initialize, tools/list and notifications) names no
// shopper and reads no memory, so any authenticated caller may ask it. Every
// tool call is bound by the table in surface.go, and a tool the contract does
// not name is passed through for the layer to answer as unknown; the guard
// never reveals whether a tool exists.
//
// The layer behind the edge is reached on loopback, so it trusts the guard
// and nothing else. The guard strips any X-Memento-* header a client sent,
// then sets the authenticated caller's kind, key id, seller and, when the
// request is scoped to one shopper, that shopper id on the request. It also
// strips the Authorization header, so the token reaches nothing behind the
// edge.
package auth
