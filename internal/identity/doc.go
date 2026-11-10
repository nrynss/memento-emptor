// Package identity builds the keys the layer stores for a shopper and a
// purchase. It is the one place these constructions live, so the graph's
// session names, the fit index's per-line shopper hashes and its row ids
// cannot drift apart.
//
// Three constructions exist, each reproducible from this package's doc
// comments alone:
//
//   - The session name is SessionNamePrefix followed by the lowercase hex
//     HMAC-SHA256 of the payer id under the identity salt. It is 72 bytes:
//     inside the graph's addressed-name rule of 1 to 128 bytes of
//     [A-Za-z0-9._:-] with no leading dot.
//   - The per-line shopper hash is the lowercase hex HMAC-SHA256 of the
//     product line (brand, category and fit handle) and the payer id under
//     the same salt. It counts distinct shoppers within one line, and the
//     same shopper's hashes in two lines share nothing, so no stored value
//     links a shopper across lines.
//   - The transaction hash is the lowercase hex SHA-256 of "txn|" followed
//     by the transaction id. It holds no salt, so the row id stays stable
//     when the salt rotates.
//
// Every HMAC message is length-prefixed: each field is its byte length in
// decimal, a colon, then the field's bytes, with nothing between fields.
// For example, a payer id "P-1" gives the session message
// "memento/session/v1|3:P-1". The length prefix makes a field boundary
// unambiguous, so two different tuples never encode to the same message.
//
// Which salt applies, and where it comes from:
//
//   - Live rows and live sessions use the layer's identity salt. It is a key
//     held in the system keyring at service identity, account memento-emptor,
//     attribute kind salt, read through tools/env.sh as
//     MEMENTO_IDENTITY_SALT: 32 random bytes as 64 hexadecimal characters.
//     The value is never written to a file or printed, and must reach a
//     deployed service as a platform secret. Rotating it changes every
//     session name and per-line hash, which orphans existing sessions and
//     breaks opt-out's recompute, so it is generated once.
//   - Seeded rows and seeded shoppers use SeedSaltHex, a fixed published
//     value committed to this repository, so the seeded population replays
//     byte for byte. Every row it produces carries the domain's seeded mark.
//     A caller building keys for a row marked seeded passes SeedSaltHex;
//     every other caller passes MEMENTO_IDENTITY_SALT.
//
// The salt is a parameter, never a package global: a Keyer is built from an
// explicit salt value, so tests and fixture generation run offline and a
// missing salt fails instead of falling back to a default.
package identity
