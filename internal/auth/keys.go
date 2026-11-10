package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// CallerKind names who one key belongs to.
type CallerKind string

// The three caller kinds, one per audience in product.md.
const (
	// CallerAgent is one of the seller's shopping agents. It reaches the
	// tools the shopper's agent calls, scoped to the shoppers it serves.
	CallerAgent CallerKind = "agent"
	// CallerShopper is a shopper signed in to their own ledger. It reaches
	// that shopper's ledger and that shopper's own writes.
	CallerShopper CallerKind = "shopper"
	// CallerMerchant is the seller's merchant app. It reaches product_fit
	// and the MCP handshake, which reads no memory, and nothing else.
	CallerMerchant CallerKind = "merchant"
)

// CallerKinds lists every kind, in the order this package's docs name them.
var CallerKinds = []CallerKind{CallerAgent, CallerShopper, CallerMerchant}

// Token prefixes. A token starts with the prefix of its kind, so a person
// reading a configuration can tell one from another. Authentication hashes
// the whole token, prefix included, and the entry's digest decides: the
// prefix is a label, never the check.
const (
	// AgentTokenPrefix starts an agent key's token.
	AgentTokenPrefix = "memento_agent_"
	// ShopperTokenPrefix starts a shopper link token.
	ShopperTokenPrefix = "memento_shop_"
	// MerchantTokenPrefix starts a merchant key's token.
	MerchantTokenPrefix = "memento_merchant_"
)

// TokenSecretBytes is the random entropy in a minted token. Thirty-two bytes
// is why the stored digest may go unsalted.
const TokenSecretBytes = 32

// The environment variables the ring is built from. Each holds zero or more
// entries in the grammar this package's docs state; a deployment fills them
// from its platform secrets and a workstation from the system keyring.
const (
	// EnvAgentKeys holds the agent keys.
	EnvAgentKeys = "MEMENTO_AGENT_KEYS"
	// EnvShopperTokens holds the shopper link tokens.
	EnvShopperTokens = "MEMENTO_SHOPPER_TOKENS"
	// EnvMerchantKeys holds the merchant app keys.
	EnvMerchantKeys = "MEMENTO_MERCHANT_KEYS"
)

// The conditions a key ring or an entry can violate. Each is one sentinel
// so a caller branches with errors.Is and never on message text. No error
// carries a token, a digest or a binding, so a bad configuration is safe to
// log.
var (
	// ErrBadEntry means one environment entry is not in the documented
	// grammar: wrong field count, a digest that is not 64 hex characters,
	// or an unknown flag.
	ErrBadEntry = errors.New("auth: malformed key entry")
	// ErrEmptyKeyID means an entry names no key id.
	ErrEmptyKeyID = errors.New("auth: empty key id")
	// ErrBadName means a key id, seller or shopper id holds a character
	// the grammar would split on, or a space.
	ErrBadName = errors.New("auth: key name has an invalid character")
	// ErrDuplicateKeyID means two entries share a key id.
	ErrDuplicateKeyID = errors.New("auth: duplicate key id")
	// ErrDuplicateDigest means two entries hold the same token digest, so
	// one token would name two callers.
	ErrDuplicateDigest = errors.New("auth: duplicate key digest")
	// ErrUnknownKind means an entry names no caller kind.
	ErrUnknownKind = errors.New("auth: unknown caller kind")
	// ErrEmptySeller means an entry names no seller.
	ErrEmptySeller = errors.New("auth: empty seller")
	// ErrEmptyShopper means a binding list holds an empty shopper id.
	ErrEmptyShopper = errors.New("auth: empty shopper id")
	// ErrNoAgentShoppers means an agent key binds no shopper, so it could
	// reach no memory and every call it made would refuse.
	ErrNoAgentShoppers = errors.New("auth: agent key binds no shopper")
	// ErrShopperBinding means a shopper token binds more, or fewer, than
	// exactly one shopper.
	ErrShopperBinding = errors.New("auth: shopper token must bind one shopper")
	// ErrMerchantShoppers means a merchant key binds a shopper. The
	// merchant app reads the pool, which carries no shopper.
	ErrMerchantShoppers = errors.New("auth: merchant key binds a shopper")
)

// Key is one issued credential as the deployment holds it: the caller it
// names, the seller it belongs to, the shoppers it may act for, and the
// digest of the token it was issued. A revoked key keeps its place, so
// revocation is auditable without deleting the caller's binding.
type Key struct {
	// ID names the key in logs, headers and revocation notes. It is unique
	// across the whole ring.
	ID string
	// Kind is the caller kind the key was issued to.
	Kind CallerKind
	// Seller is the seller whose layer issued the key.
	Seller string
	// Shoppers are the shoppers the key may act for: one or more for an
	// agent, exactly one for a shopper token, none for a merchant key.
	Shoppers []string
	// Hash is the SHA-256 digest of the token. The token itself is never
	// stored.
	Hash [32]byte
	// Revoked marks a key that must stop working but keep its record.
	Revoked bool
}

// Principal is an authenticated caller: the key it presented, the kind of
// caller it is, the seller it belongs to, and the shoppers it may act for.
type Principal struct {
	// KeyID is the deployment's id for the key, never the token.
	KeyID string
	// Kind is the caller's kind.
	Kind CallerKind
	// Seller is the seller whose layer issued the key.
	Seller string
	// Shoppers are the shoppers the caller may act for, sorted. An agent
	// has one or more; a shopper has exactly one; a merchant has none.
	Shoppers []string
}

// MayActFor reports whether the caller may name one shopper in a request.
// An empty id is never one of the caller's shoppers.
func (p Principal) MayActFor(shopperID string) bool {
	if shopperID == "" {
		return false
	}
	for _, bound := range p.Shoppers {
		if bound == shopperID {
			return true
		}
	}
	return false
}

// Keyring is the set of keys a deployment was configured with. It is built
// once at boot and read-only afterwards, so it is safe for concurrent use.
type Keyring struct {
	keys []Key
}

// Summary is a boot-time count of a ring, for a log line. It carries no key
// id, binding, digest or token.
type Summary struct {
	// Agents is how many agent keys the ring holds, revoked ones included.
	Agents int
	// Shoppers is how many shopper link tokens the ring holds.
	Shoppers int
	// Merchants is how many merchant keys the ring holds.
	Merchants int
	// Revoked is how many keys of any kind are revoked.
	Revoked int
}

// NewKeyring validates keys and returns a ring over them. It copies every
// binding, so a caller may reuse or change its slice afterwards.
func NewKeyring(keys []Key) (*Keyring, error) {
	ring := &Keyring{keys: make([]Key, 0, len(keys))}
	ids := make(map[string]bool, len(keys))
	digests := make(map[[32]byte]bool, len(keys))
	for _, key := range keys {
		if err := validateKey(key); err != nil {
			return nil, err
		}
		if ids[key.ID] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateKeyID, key.ID)
		}
		if digests[key.Hash] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateDigest, key.ID)
		}
		ids[key.ID] = true
		digests[key.Hash] = true
		copied := key
		copied.Shoppers = append([]string(nil), key.Shoppers...)
		sort.Strings(copied.Shoppers)
		ring.keys = append(ring.keys, copied)
	}
	return ring, nil
}

// FromEnv builds the ring from the deployment's environment. getenv is
// os.Getenv in production and a map lookup in tests. A missing variable is
// not an error: the ring then holds no key of that kind. A malformed entry
// is an error, and the caller refuses to start.
func FromEnv(getenv func(string) string) (*Keyring, error) {
	if getenv == nil {
		return nil, fmt.Errorf("auth: nil getenv")
	}
	var keys []Key
	for _, source := range []struct {
		kind CallerKind
		name string
	}{
		{CallerAgent, EnvAgentKeys},
		{CallerShopper, EnvShopperTokens},
		{CallerMerchant, EnvMerchantKeys},
	} {
		parsed, err := parseEntries(source.kind, getenv(source.name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", source.name, err)
		}
		keys = append(keys, parsed...)
	}
	return NewKeyring(keys)
}

// parseEntries reads one environment variable: zero or more entries
// separated by whitespace, each in the documented grammar.
func parseEntries(kind CallerKind, raw string) ([]Key, error) {
	if !validKind(kind) {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKind, kind)
	}
	var keys []Key
	for i, entry := range strings.Fields(raw) {
		key, err := parseEntry(kind, entry)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i+1, err)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// parseEntry reads one entry: id, digest, seller, shoppers, and an optional
// revoked flag. The error names the failed field, never the entry's value.
func parseEntry(kind CallerKind, entry string) (Key, error) {
	fields := strings.Split(entry, "|")
	if len(fields) < 4 || len(fields) > 5 {
		return Key{}, ErrBadEntry
	}
	key := Key{ID: fields[0], Kind: kind, Seller: fields[2]}
	hash, err := parseDigest(fields[1])
	if err != nil {
		return Key{}, err
	}
	key.Hash = hash
	key.Shoppers = parseBindings(fields[3])
	if len(fields) == 5 {
		switch fields[4] {
		case "", "-":
		case "revoked":
			key.Revoked = true
		default:
			return Key{}, ErrBadEntry
		}
	}
	return key, nil
}

// parseDigest reads one lowercase or uppercase hex SHA-256 digest.
func parseDigest(raw string) ([32]byte, error) {
	var digest [32]byte
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) != len(digest) {
		return digest, ErrBadEntry
	}
	copy(digest[:], decoded)
	return digest, nil
}

// parseBindings reads the shoppers field: a comma-separated list, with - or
// an empty field meaning none.
func parseBindings(raw string) []string {
	if raw == "" || raw == "-" {
		return nil
	}
	return strings.Split(raw, ",")
}

// validateKey checks one key's binding. It is the same check an entry gets,
// so a key issued by IssueKey and a key parsed from the environment cannot
// drift apart.
func validateKey(key Key) error {
	if !validKind(key.Kind) {
		return fmt.Errorf("%w: %q", ErrUnknownKind, key.Kind)
	}
	if key.ID == "" {
		return ErrEmptyKeyID
	}
	if !validName(key.ID) {
		return fmt.Errorf("%w: key id", ErrBadName)
	}
	if key.Seller == "" {
		return ErrEmptySeller
	}
	if !validName(key.Seller) {
		return fmt.Errorf("%w: seller", ErrBadName)
	}
	for _, shopper := range key.Shoppers {
		if shopper == "" {
			return ErrEmptyShopper
		}
		if !validName(shopper) {
			return fmt.Errorf("%w: shopper id", ErrBadName)
		}
	}
	switch key.Kind {
	case CallerAgent:
		if len(key.Shoppers) == 0 {
			return ErrNoAgentShoppers
		}
	case CallerShopper:
		if len(key.Shoppers) != 1 {
			return ErrShopperBinding
		}
	case CallerMerchant:
		if len(key.Shoppers) != 0 {
			return ErrMerchantShoppers
		}
	}
	return nil
}

// validKind reports whether a caller kind is one of the three.
func validKind(kind CallerKind) bool {
	switch kind {
	case CallerAgent, CallerShopper, CallerMerchant:
		return true
	}
	return false
}

// validName reports whether a key id, seller or shopper id survives the
// entry grammar: no whitespace, no field or list separator.
func validName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "|, \t\r\n")
}

// TokenPrefix returns the prefix a kind's tokens start with, or an empty
// string for a kind this package does not know.
func TokenPrefix(kind CallerKind) string {
	switch kind {
	case CallerAgent:
		return AgentTokenPrefix
	case CallerShopper:
		return ShopperTokenPrefix
	case CallerMerchant:
		return MerchantTokenPrefix
	}
	return ""
}

// NewToken mints one token for a caller kind: the kind's prefix followed by
// 32 random bytes as unpadded base64url. The returned token is the only copy
// that will ever exist; HashToken is what the deployment stores.
func NewToken(kind CallerKind) (string, error) {
	prefix := TokenPrefix(kind)
	if prefix == "" {
		return "", fmt.Errorf("%w: %q", ErrUnknownKind, kind)
	}
	secret := make([]byte, TokenSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("auth: mint token: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(secret), nil
}

// HashToken returns the lowercase hex SHA-256 digest of one token, which is
// the value an environment entry holds. The digest is unsalted because a
// minted token carries 32 random bytes; a token typed by hand at lower
// entropy loses that property.
func HashToken(token string) string {
	digest := digestOf(token)
	return hex.EncodeToString(digest[:])
}

// digestOf returns the SHA-256 digest of one token's bytes.
func digestOf(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}

// IssueKey mints one token and renders the environment entry the deployment
// holds for it. The seller receives the token once and hands it to the
// caller; the deployment receives only the entry. The binding is validated
// by the same check the parser applies, so an issued entry always parses.
func IssueKey(kind CallerKind, id, seller string, shoppers []string) (string, string, error) {
	token, err := NewToken(kind)
	if err != nil {
		return "", "", err
	}
	key := Key{ID: id, Kind: kind, Seller: seller, Shoppers: shoppers, Hash: digestOf(token)}
	if err := validateKey(key); err != nil {
		return "", "", err
	}
	return token, renderEntry(key), nil
}

// renderEntry renders one key in the grammar parseEntries reads.
func renderEntry(key Key) string {
	bindings := "-"
	if len(key.Shoppers) > 0 {
		bindings = strings.Join(key.Shoppers, ",")
	}
	flags := "-"
	if key.Revoked {
		flags = "revoked"
	}
	digest := key.Hash
	return strings.Join([]string{key.ID, hex.EncodeToString(digest[:]), key.Seller, bindings, flags}, "|")
}

// Authenticate returns the principal for a presented token, or false when
// the token is empty, unknown or revoked. Every key is compared in constant
// time and no comparison stops the scan, so the answer's timing does not
// depend on which entry matched or whether one did.
func (k *Keyring) Authenticate(token string) (Principal, bool) {
	if k == nil || token == "" {
		return Principal{}, false
	}
	digest := digestOf(token)
	match := -1
	for i := range k.keys {
		if subtle.ConstantTimeCompare(digest[:], k.keys[i].Hash[:]) == 1 {
			match = i
		}
	}
	if match < 0 || k.keys[match].Revoked {
		return Principal{}, false
	}
	key := k.keys[match]
	return Principal{
		KeyID:    key.ID,
		Kind:     key.Kind,
		Seller:   key.Seller,
		Shoppers: append([]string(nil), key.Shoppers...),
	}, true
}

// Len reports how many keys the ring holds, revoked keys included.
func (k *Keyring) Len() int {
	if k == nil {
		return 0
	}
	return len(k.keys)
}

// Summary counts the ring by kind and by revocation, for a boot log line.
func (k *Keyring) Summary() Summary {
	var summary Summary
	if k == nil {
		return summary
	}
	for _, key := range k.keys {
		switch key.Kind {
		case CallerAgent:
			summary.Agents++
		case CallerShopper:
			summary.Shoppers++
		case CallerMerchant:
			summary.Merchants++
		}
		if key.Revoked {
			summary.Revoked++
		}
	}
	return summary
}
