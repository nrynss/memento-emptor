package auth

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// Test tokens. Every one is invented here and hashes to a fixture digest;
// none is a live key, and none is printed by production code.
const (
	tokenAgentChat      = "test-agent-chat-token-not-a-live-key"
	tokenAgentCheckout  = "test-agent-checkout-token-not-a-live-key"
	tokenAgentRetired   = "test-agent-retired-token-not-a-live-key"
	tokenShopperDemo    = "test-shopper-demo-token-not-a-live-key"
	tokenShopperSecond  = "test-shopper-second-token-not-a-live-key"
	tokenShopperRetire  = "test-shopper-retired-token-not-a-live-key"
	tokenMerchant       = "test-merchant-brandx-token-not-a-live-key"
	tokenMerchantRetire = "test-merchant-retired-token-not-a-live-key"
	tokenUnknown        = "test-unknown-token-not-a-live-key"
)

// entry renders one environment entry from its fields.
func entry(fields ...string) string {
	return strings.Join(fields, "|")
}

// testRing is the ring every guard test runs against: two seller's agents,
// two shoppers of one seller, one revoked key of each bound kind, and the
// merchant app.
func testRing(t *testing.T) *Keyring {
	t.Helper()
	env := map[string]string{
		EnvAgentKeys: entry("agent-chat", HashToken(tokenAgentChat), "seller-demo", "shopper_01,shopper_02", "-") + "\n" +
			entry("agent-checkout", HashToken(tokenAgentCheckout), "seller-other", "shopper_09", "-") + "\n" +
			entry("agent-retired", HashToken(tokenAgentRetired), "seller-demo", "shopper_01", "revoked"),
		EnvShopperTokens: entry("shopper-demo", HashToken(tokenShopperDemo), "seller-demo", "shopper_01", "-") + "\n" +
			entry("shopper-second", HashToken(tokenShopperSecond), "seller-demo", "shopper_02", "-") + "\n" +
			entry("shopper-retired", HashToken(tokenShopperRetire), "seller-demo", "shopper_01", "revoked"),
		EnvMerchantKeys: entry("merchant-brandx", HashToken(tokenMerchant), "seller-demo", "-", "-") + "\n" +
			entry("merchant-retired", HashToken(tokenMerchantRetire), "seller-demo", "-", "revoked"),
	}
	ring, err := FromEnv(func(name string) string { return env[name] })
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	return ring
}

func TestNewKeyringValidatesEveryKind(t *testing.T) {
	t.Parallel()

	digest := HashToken("test-validate-token-not-a-live-key")
	hash := testDigest(t, digest)
	otherHash := testDigest(t, HashToken("test-validate-other-token-not-a-live-key"))

	tests := []struct {
		name string
		keys []Key
		want error
	}{
		{
			name: "agent with two shoppers",
			keys: []Key{{ID: "a", Kind: CallerAgent, Seller: "s", Shoppers: []string{"one", "two"}, Hash: hash}},
		},
		{
			name: "shopper with one",
			keys: []Key{{ID: "b", Kind: CallerShopper, Seller: "s", Shoppers: []string{"one"}, Hash: hash}},
		},
		{
			name: "merchant with none",
			keys: []Key{{ID: "c", Kind: CallerMerchant, Seller: "s", Hash: hash}},
		},
		{
			name: "empty ring",
			keys: nil,
		},
		{
			name: "agent with none",
			keys: []Key{{ID: "a", Kind: CallerAgent, Seller: "s", Hash: hash}},
			want: ErrNoAgentShoppers,
		},
		{
			name: "shopper with two",
			keys: []Key{{ID: "b", Kind: CallerShopper, Seller: "s", Shoppers: []string{"one", "two"}, Hash: hash}},
			want: ErrShopperBinding,
		},
		{
			name: "shopper with none",
			keys: []Key{{ID: "b", Kind: CallerShopper, Seller: "s", Hash: hash}},
			want: ErrShopperBinding,
		},
		{
			name: "merchant with one",
			keys: []Key{{ID: "c", Kind: CallerMerchant, Seller: "s", Shoppers: []string{"one"}, Hash: hash}},
			want: ErrMerchantShoppers,
		},
		{
			name: "empty id",
			keys: []Key{{Kind: CallerMerchant, Seller: "s", Hash: hash}},
			want: ErrEmptyKeyID,
		},
		{
			name: "id with a separator",
			keys: []Key{{ID: "a,b", Kind: CallerMerchant, Seller: "s", Hash: hash}},
			want: ErrBadName,
		},
		{
			name: "empty seller",
			keys: []Key{{ID: "a", Kind: CallerMerchant, Hash: hash}},
			want: ErrEmptySeller,
		},
		{
			name: "empty shopper id",
			keys: []Key{{ID: "a", Kind: CallerAgent, Seller: "s", Shoppers: []string{"one", ""}, Hash: hash}},
			want: ErrEmptyShopper,
		},
		{
			name: "unknown kind",
			keys: []Key{{ID: "a", Kind: "platform", Seller: "s", Hash: hash}},
			want: ErrUnknownKind,
		},
		{
			name: "duplicate id",
			keys: []Key{
				{ID: "a", Kind: CallerMerchant, Seller: "s", Hash: hash},
				{ID: "a", Kind: CallerMerchant, Seller: "t", Hash: otherHash},
			},
			want: ErrDuplicateKeyID,
		},
		{
			name: "duplicate digest",
			keys: []Key{
				{ID: "a", Kind: CallerMerchant, Seller: "s", Hash: hash},
				{ID: "b", Kind: CallerMerchant, Seller: "s", Hash: hash},
			},
			want: ErrDuplicateDigest,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ring, err := NewKeyring(tc.keys)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("NewKeyring error = %v, want %v", err, tc.want)
				}
				if ring != nil {
					t.Fatalf("NewKeyring returned a ring beside error %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewKeyring: %v", err)
			}
			if ring.Len() != len(tc.keys) {
				t.Fatalf("Len() = %d, want %d", ring.Len(), len(tc.keys))
			}
		})
	}
}

func TestFromEnvGrammar(t *testing.T) {
	t.Parallel()

	digest := HashToken("test-grammar-token-not-a-live-key")
	upper := strings.ToUpper(digest)

	tests := []struct {
		name string
		env  map[string]string
		want error
	}{
		{name: "empty environment"},
		{name: "blank values", env: map[string]string{EnvAgentKeys: "  \n\t ", EnvShopperTokens: "", EnvMerchantKeys: ""}}, {
			name: "uppercase digest",
			env:  map[string]string{EnvMerchantKeys: entry("m", upper, "s", "-", "-")},
		},
		{
			name: "revoked flag",
			env:  map[string]string{EnvMerchantKeys: entry("m", digest, "s", "-", "revoked")},
		},
		{
			name: "no flag field",
			env:  map[string]string{EnvMerchantKeys: entry("m", digest, "s", "-")},
		},
		{
			name: "agent shoppers with no flag field",
			env:  map[string]string{EnvAgentKeys: entry("a", digest, "s", "one,two")},
		},
		{
			name: "too few fields",
			env:  map[string]string{EnvMerchantKeys: entry("m", digest, "s")},
			want: ErrBadEntry,
		},
		{
			name: "too many fields",
			env:  map[string]string{EnvMerchantKeys: entry("m", digest, "s", "-", "-", "extra")},
			want: ErrBadEntry,
		},
		{
			name: "short digest",
			env:  map[string]string{EnvMerchantKeys: entry("m", "abc", "s", "-", "-")},
			want: ErrBadEntry,
		},
		{
			name: "digest not hex",
			env:  map[string]string{EnvMerchantKeys: entry("m", strings.Repeat("z", 64), "s", "-", "-")},
			want: ErrBadEntry,
		},
		{
			name: "unknown flag",
			env:  map[string]string{EnvMerchantKeys: entry("m", digest, "s", "-", "expired")},
			want: ErrBadEntry,
		},
		{
			name: "empty id",
			env:  map[string]string{EnvMerchantKeys: entry("", digest, "s", "-", "-")},
			want: ErrEmptyKeyID,
		},
		{
			name: "a space in a name splits the entry",
			env:  map[string]string{EnvMerchantKeys: entry("m", digest, "two sellers", "-", "-")},
			want: ErrBadEntry,
		},
		{
			name: "agent binds nothing",
			env:  map[string]string{EnvAgentKeys: entry("a", digest, "s", "-", "-")},
			want: ErrNoAgentShoppers,
		},
		{
			name: "shopper binds two",
			env:  map[string]string{EnvShopperTokens: entry("s", digest, "seller", "one,two", "-")},
			want: ErrShopperBinding,
		},
		{
			name: "shopper binds an empty id",
			env:  map[string]string{EnvShopperTokens: entry("s", digest, "seller", "one,", "-")},
			want: ErrEmptyShopper,
		},
		{
			name: "merchant binds a shopper",
			env:  map[string]string{EnvMerchantKeys: entry("m", digest, "s", "one", "-")},
			want: ErrMerchantShoppers,
		},
		{
			name: "duplicate id across variables",
			env: map[string]string{
				EnvMerchantKeys:  entry("same", digest, "s", "-", "-"),
				EnvShopperTokens: entry("same", HashToken("test-grammar-other-token"), "s", "one", "-"),
			},
			want: ErrDuplicateKeyID,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ring, err := FromEnv(func(name string) string { return tc.env[name] })
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("FromEnv error = %v, want %v", err, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("FromEnv: %v", err)
			}
			if ring == nil {
				t.Fatal("FromEnv returned a nil ring")
			}
		})
	}
}

func TestFromEnvErrorDoesNotEchoTheEntry(t *testing.T) {
	t.Parallel()

	const probe = "probe-secret-token-value"
	_, err := FromEnv(func(name string) string {
		if name == EnvMerchantKeys {
			return entry("m", HashToken("probe"), "s", probe+",", "-")
		}
		return ""
	})
	if err == nil {
		t.Fatal("FromEnv accepted an empty shopper binding")
	}
	if strings.Contains(err.Error(), HashToken("probe")) {
		t.Fatalf("the error echoes the digest: %v", err)
	}
	if strings.Contains(err.Error(), probe) {
		t.Fatalf("the error echoes a binding value: %v", err)
	}
}

func TestAuthenticateAndRevocation(t *testing.T) {
	t.Parallel()

	ring := testRing(t)

	principal, ok := ring.Authenticate(tokenAgentChat)
	if !ok {
		t.Fatal("the agent's own token did not authenticate")
	}
	if principal.KeyID != "agent-chat" || principal.Kind != CallerAgent || principal.Seller != "seller-demo" {
		t.Fatalf("principal = %+v, want agent-chat of seller-demo", principal)
	}
	if len(principal.Shoppers) != 2 || principal.Shoppers[0] != "shopper_01" || principal.Shoppers[1] != "shopper_02" {
		t.Fatalf("shoppers = %v, want shopper_01 and shopper_02 sorted", principal.Shoppers)
	}
	if !principal.MayActFor("shopper_02") {
		t.Fatal("the agent may not act for a bound shopper")
	}
	if principal.MayActFor("shopper_09") || principal.MayActFor("") {
		t.Fatal("the agent may act for a shopper it is not bound to")
	}

	tests := []struct {
		name  string
		token string
	}{
		{name: "empty token", token: ""},
		{name: "unknown token", token: tokenUnknown},
		{name: "revoked agent", token: tokenAgentRetired},
		{name: "revoked shopper", token: tokenShopperRetire},
		{name: "revoked merchant", token: tokenMerchantRetire},
		{name: "token with trailing space", token: tokenAgentChat + " "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if principal, ok := ring.Authenticate(tc.token); ok {
				t.Fatalf("Authenticate(%q) = %+v, want no principal", tc.token, principal)
			}
		})
	}

	if got, want := ring.Summary(), (Summary{Agents: 3, Shoppers: 3, Merchants: 2, Revoked: 3}); got != want {
		t.Fatalf("Summary() = %+v, want %+v", got, want)
	}
}

func TestAuthenticateReturnsCallsOwnBindings(t *testing.T) {
	t.Parallel()

	ring := testRing(t)
	principal, ok := ring.Authenticate(tokenAgentChat)
	if !ok {
		t.Fatal("the agent's own token did not authenticate")
	}
	principal.Shoppers[0] = "shopper_09"
	again, ok := ring.Authenticate(tokenAgentChat)
	if !ok {
		t.Fatal("the second authentication failed")
	}
	if again.MayActFor("shopper_09") {
		t.Fatal("changing a returned binding changed the ring")
	}
}

func TestIssueKeyRoundTripsThroughTheGrammar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		kind     CallerKind
		prefix   string
		shoppers []string
	}{
		{name: "agent", kind: CallerAgent, prefix: AgentTokenPrefix, shoppers: []string{"shopper_01"}},
		{name: "shopper", kind: CallerShopper, prefix: ShopperTokenPrefix, shoppers: []string{"shopper_01"}},
		{name: "merchant", kind: CallerMerchant, prefix: MerchantTokenPrefix},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token, line, err := IssueKey(tc.kind, "issued", "seller-demo", tc.shoppers)
			if err != nil {
				t.Fatalf("IssueKey: %v", err)
			}
			if !strings.HasPrefix(token, tc.prefix) {
				t.Fatalf("token does not start with %q", tc.prefix)
			}
			if len(token) <= len(tc.prefix) {
				t.Fatalf("token carries no secret beside its prefix: %q", token)
			}
			if strings.Contains(line, token) {
				t.Fatal("the entry holds the token instead of its digest")
			}
			ring, err := NewKeyring(mustParse(t, tc.kind, line))
			if err != nil {
				t.Fatalf("the issued entry does not parse: %v", err)
			}
			principal, ok := ring.Authenticate(token)
			if !ok {
				t.Fatal("the issued token does not authenticate")
			}
			if principal.KeyID != "issued" || principal.Kind != tc.kind {
				t.Fatalf("principal = %+v, want issued of kind %q", principal, tc.kind)
			}
			for _, shopper := range tc.shoppers {
				if !principal.MayActFor(shopper) {
					t.Fatalf("the issued key lost its binding to %q", shopper)
				}
			}
		})
	}

	if _, _, err := IssueKey("platform", "x", "seller-demo", nil); !errors.Is(err, ErrUnknownKind) {
		t.Fatalf("IssueKey for an unknown kind = %v, want ErrUnknownKind", err)
	}
}

func TestNewTokenIsUniqueAndPrefixed(t *testing.T) {
	t.Parallel()

	first, err := NewToken(CallerShopper)
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	second, err := NewToken(CallerShopper)
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if first == second {
		t.Fatal("two minted tokens are equal")
	}
	if TokenPrefix(CallerShopper) != ShopperTokenPrefix || TokenPrefix("platform") != "" {
		t.Fatal("TokenPrefix does not map the kinds")
	}
}

// testDigest decodes one hex digest into the bytes a Key carries.
func testDigest(t *testing.T, digest string) [32]byte {
	t.Helper()
	var out [32]byte
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != len(out) {
		t.Fatalf("test fixture digest is not 32 hex-encoded bytes")
	}
	copy(out[:], decoded)
	return out
}

// mustParse parses one entry through the environment grammar.
func mustParse(t *testing.T, kind CallerKind, raw string) []Key {
	t.Helper()
	keys, err := parseEntries(kind, raw)
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}
	return keys
}

// TestRenderEntryRoundTripsRevocation proves an entry the issuer renders for
// a revoked key parses back as revoked, so the grammar has one writer.
func TestRenderEntryRoundTripsRevocation(t *testing.T) {
	t.Parallel()

	key := Key{
		ID:      "retired",
		Kind:    CallerMerchant,
		Seller:  "seller-demo",
		Hash:    testDigest(t, HashToken("test-render-token-not-a-live-key")),
		Revoked: true,
	}
	parsed := mustParse(t, CallerMerchant, renderEntry(key))
	if len(parsed) != 1 {
		t.Fatalf("parsed %d entries, want 1", len(parsed))
	}
	if !parsed[0].Revoked {
		t.Fatalf("the rendered entry lost the revoked flag: %+v", parsed[0])
	}
	if parsed[0].ID != key.ID || parsed[0].Hash != key.Hash {
		t.Fatalf("the rendered entry changed the key: %+v", parsed[0])
	}
}
