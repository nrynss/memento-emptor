package identity

// SeedSaltHex is the published identity salt for seeded rows and seeded
// shoppers. It is safe to commit: it keys only generated history, never a
// live shopper, and every row it produces carries the domain's seeded mark.
// It fixes the seeded population's session names and per-line hashes, so
// the population replays byte for byte.
//
// The value is the lowercase hex SHA-256 of the published phrase
// "memento-emptor seed identity salt v1". Anyone can recompute it:
//
//	printf '%s' 'memento-emptor seed identity salt v1' | sha256sum
//
// It is not the live salt, and code building keys for a live shopper must
// use a Keyer made from MEMENTO_IDENTITY_SALT instead.
const SeedSaltHex = "577f023802abd2fbab409f4024d218520d258097d441a28f301600fb9490a1d4"
