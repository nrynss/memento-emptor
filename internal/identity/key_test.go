package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// testKeyer returns a Keyer over the published seed salt, so every test
// runs offline and no live value is read.
func testKeyer(t *testing.T) *Keyer {
	t.Helper()
	k, err := NewKeyer(SeedSaltHex)
	if err != nil {
		t.Fatalf("NewKeyer(SeedSaltHex): %v", err)
	}
	return k
}

// testSaltPhrase derives a second salt for the tests that need two. The
// derived value is never committed.
func testSaltPhrase(t *testing.T, phrase string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(phrase))
	return hex.EncodeToString(sum[:])
}

func TestNewKeyer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		salt string
		want error
	}{
		{name: "seed salt", salt: SeedSaltHex},
		{name: "uppercase hex", salt: strings.ToUpper(SeedSaltHex)},
		{name: "missing", salt: "", want: ErrMissingSalt},
		{name: "too short", salt: strings.Repeat("a", 63), want: ErrMalformedSalt},
		{name: "too long", salt: strings.Repeat("a", 65), want: ErrMalformedSalt},
		{name: "31 bytes", salt: strings.Repeat("ab", 31), want: ErrMalformedSalt},
		{name: "33 bytes", salt: strings.Repeat("ab", 33), want: ErrMalformedSalt},
		{name: "not hex", salt: strings.Repeat("z", 64), want: ErrMalformedSalt},
		{name: "trailing newline", salt: SeedSaltHex + "\n", want: ErrMalformedSalt},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k, err := NewKeyer(tc.salt)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("NewKeyer error = %v, want %v", err, tc.want)
				}
				if k != nil {
					t.Fatalf("NewKeyer returned a keyer beside error %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewKeyer: %v", err)
			}
			if k == nil {
				t.Fatal("NewKeyer returned a nil keyer")
			}
		})
	}
}

func TestNewKeyerErrorDoesNotEchoTheSalt(t *testing.T) {
	t.Parallel()

	const probe = "supersecretvalue"
	_, err := NewKeyer(probe)
	if !errors.Is(err, ErrMalformedSalt) {
		t.Fatalf("NewKeyer error = %v, want ErrMalformedSalt", err)
	}
	if strings.Contains(err.Error(), probe) {
		t.Fatalf("NewKeyer error repeats the supplied salt: %v", err)
	}
}

func TestNilKeyerFailsClearly(t *testing.T) {
	t.Parallel()

	var k *Keyer
	if _, err := k.SessionName("PAYER-1"); !errors.Is(err, ErrMissingSalt) {
		t.Errorf("nil SessionName error = %v, want ErrMissingSalt", err)
	}
	if _, err := k.LineShopperHash("PAYER-1", domain.ProductLine{Brand: "b", Category: "c", FitHandle: "f"}); !errors.Is(err, ErrMissingSalt) {
		t.Errorf("nil LineShopperHash error = %v, want ErrMissingSalt", err)
	}
}

func TestSessionName(t *testing.T) {
	t.Parallel()

	k := testKeyer(t)
	tests := []struct {
		name    string
		payerID string
		want    string
		wantErr error
	}{
		{
			name:    "sandbox payer",
			payerID: "PAYER-1",
			want:    "shopper-8ad89b95e260756509aa5ed7263362b25ba9dbf9fd8a9c12b3c00b3ceeb2c030",
		},
		{
			name:    "ledger shopper id",
			payerID: "shopper_01",
			want:    "shopper-00895719a8a9d1ba88c99984e50dd200a83b42cfab629a4919c635033e2e5caf",
		},
		{
			name:    "dashed shopper id",
			payerID: "shopper-01",
			want:    "shopper-8e1b01f26ad22db3b3aa538e68073563d125bedeea333fd0c6eaa40cb32b4b16",
		},
		{
			name:    "non-ASCII payer is hashed, never copied",
			payerID: "päyer-é",
			want:    "shopper-6bb6e40ceaece171cae1c47272516329691e9897348406821d31abb9ce1e6d6b",
		},
		{
			name:    "space is not trimmed from an opaque id",
			payerID: " ",
			want:    "shopper-6e079727ec5f71b42322db5187ff68a16d6edede634d433fd2be63b765deee0d",
		},
		{
			name:    "payer at the addressed-name limit",
			payerID: strings.Repeat("a", MaxAddressedNameBytes),
			want:    "shopper-824794b10539ae6277aed7a55ff491f63a64e3b013fdf0c818d152f9e3e5faaa",
		},
		{
			name:    "empty payer",
			payerID: "",
			wantErr: ErrEmptyPayerID,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := k.SessionName(tc.payerID)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("SessionName error = %v, want %v", err, tc.wantErr)
				}
				if got != "" {
					t.Fatalf("SessionName = %q beside error %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("SessionName: %v", err)
			}
			if got != tc.want {
				t.Errorf("SessionName = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSessionNameLengthAndAddressRule(t *testing.T) {
	t.Parallel()

	k := testKeyer(t)

	// Every payer yields the same fixed width: 8-byte prefix plus 64 hex.
	name, err := k.SessionName(strings.Repeat("a", MaxAddressedNameBytes))
	if err != nil {
		t.Fatalf("SessionName: %v", err)
	}
	if want := len(SessionNamePrefix) + 64; len(name) != want {
		t.Errorf("session name is %d bytes, want %d", len(name), want)
	}
	if !ValidSessionName(name) {
		t.Errorf("session name %q fails the addressed-name rule", name)
	}

	// The rule's own boundary: exactly 128 bytes is addressable, 129 is not.
	atLimit := SessionNamePrefix + strings.Repeat("a", MaxAddressedNameBytes-len(SessionNamePrefix))
	if len(atLimit) != MaxAddressedNameBytes {
		t.Fatalf("test name is %d bytes, want %d", len(atLimit), MaxAddressedNameBytes)
	}
	if !ValidSessionName(atLimit) {
		t.Errorf("name of exactly %d bytes should be addressable", MaxAddressedNameBytes)
	}
	if ValidSessionName(atLimit + "a") {
		t.Errorf("name of %d bytes should not be addressable", MaxAddressedNameBytes+1)
	}
}

func TestLineShopperHash(t *testing.T) {
	t.Parallel()

	k := testKeyer(t)
	tests := []struct {
		name    string
		payerID string
		line    domain.ProductLine
		want    string
		wantErr error
	}{
		{
			name:    "regular cut",
			payerID: "PAYER-1",
			line:    domain.ProductLine{Brand: "sunspel", Category: "shirt", FitHandle: "regular"},
			want:    "4c38f5c5150a37bafa623d43f99c8765f93f0ccf90591b71ba313dcbc4c134b8",
		},
		{
			name:    "slim cut, same payer and brand",
			payerID: "PAYER-1",
			line:    domain.ProductLine{Brand: "sunspel", Category: "shirt", FitHandle: "slim"},
			want:    "a6b8e9a1a802ded5a6b9f51e24300d6242fdd079677448282e464328699d68d0",
		},
		{
			name:    "line fields are case sensitive",
			payerID: "PAYER-1",
			line:    domain.ProductLine{Brand: "Sunspel", Category: "Shirt", FitHandle: "Regular"},
			want:    "629e6fdd0e890970cc0cd6d53e900a840ddd498726d88f2157a81733df7f825a",
		},
		{
			name:    "another payer, same line",
			payerID: "PAYER-2",
			line:    domain.ProductLine{Brand: "sunspel", Category: "shirt", FitHandle: "regular"},
			want:    "c30eeb062af211ddc227de3d77ddaba7b02513075de6ad32462a998f1892d65f",
		},
		{
			name:    "length prefixes keep field splits apart",
			payerID: "e",
			line:    domain.ProductLine{Brand: "ab", Category: "c", FitHandle: "d"},
			want:    "72c5959432294d70fe3e41f279d966589d4e59a90b0001a9feba3196414883f5",
		},
		{
			name:    "same field bytes, different split",
			payerID: "e",
			line:    domain.ProductLine{Brand: "a", Category: "bc", FitHandle: "d"},
			want:    "29adcce638068e4efc35bd208e0c2b077db282b4a9e8ee6532db5b9fc27705e5",
		},
		{
			name:    "empty payer",
			payerID: "",
			line:    domain.ProductLine{Brand: "sunspel", Category: "shirt", FitHandle: "regular"},
			wantErr: ErrEmptyPayerID,
		},
		{
			name:    "empty brand",
			payerID: "PAYER-1",
			line:    domain.ProductLine{Category: "shirt", FitHandle: "regular"},
			wantErr: ErrEmptyLine,
		},
		{
			name:    "empty category",
			payerID: "PAYER-1",
			line:    domain.ProductLine{Brand: "sunspel", FitHandle: "regular"},
			wantErr: ErrEmptyLine,
		},
		{
			name:    "empty fit handle",
			payerID: "PAYER-1",
			line:    domain.ProductLine{Brand: "sunspel", Category: "shirt"},
			wantErr: ErrEmptyLine,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := k.LineShopperHash(tc.payerID, tc.line)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("LineShopperHash error = %v, want %v", err, tc.wantErr)
				}
				if got != "" {
					t.Fatalf("LineShopperHash = %q beside error %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LineShopperHash: %v", err)
			}
			if got != tc.want {
				t.Errorf("LineShopperHash = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLineShopperHashStableWithinOneLineAndUnrelatedAcrossLines(t *testing.T) {
	t.Parallel()

	k := testKeyer(t)
	regular := domain.ProductLine{Brand: "sunspel", Category: "shirt", FitHandle: "regular"}
	slim := domain.ProductLine{Brand: "sunspel", Category: "shirt", FitHandle: "slim"}

	first, err := k.LineShopperHash("PAYER-1", regular)
	if err != nil {
		t.Fatalf("first hash: %v", err)
	}
	second, err := k.LineShopperHash("PAYER-1", regular)
	if err != nil {
		t.Fatalf("second hash: %v", err)
	}
	if first != second {
		t.Errorf("same payer and line produced %q then %q", first, second)
	}

	otherLine, err := k.LineShopperHash("PAYER-1", slim)
	if err != nil {
		t.Fatalf("other line hash: %v", err)
	}
	if first == otherLine {
		t.Errorf("one payer's hashes in two lines are equal: %q", first)
	}
}

func TestSessionNameAndLineHashAreUnrelated(t *testing.T) {
	t.Parallel()

	k := testKeyer(t)
	name, err := k.SessionName("PAYER-1")
	if err != nil {
		t.Fatalf("SessionName: %v", err)
	}
	line, err := k.LineShopperHash("PAYER-1", domain.ProductLine{Brand: "sunspel", Category: "shirt", FitHandle: "regular"})
	if err != nil {
		t.Fatalf("LineShopperHash: %v", err)
	}
	if strings.TrimPrefix(name, SessionNamePrefix) == line {
		t.Error("session name and per-line hash carry the same digest")
	}
}

func TestTxnHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		txnID   string
		want    string
		wantErr error
	}{
		{
			name:  "capture id",
			txnID: "CAP-1",
			want:  "34b5415820cd936cdc1d7dc79bb1e74a982ec826e20abe3b8e179d8d572586a4",
		},
		{
			name:  "sandbox capture id",
			txnID: "5O190127TN364715T",
			want:  "6fe8b2876ad49de4a3c77c86e354e58b1534e1cd2e4a7a6a212665fd76216168",
		},
		{
			name:    "empty transaction",
			txnID:   "",
			wantErr: ErrEmptyTransactionID,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := TxnHash(tc.txnID)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("TxnHash error = %v, want %v", err, tc.wantErr)
				}
				if got != "" {
					t.Fatalf("TxnHash = %q beside error %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("TxnHash: %v", err)
			}
			if got != tc.want {
				t.Errorf("TxnHash = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidSessionName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want bool
	}{
		{name: SessionNamePrefix + strings.Repeat("a", 64), want: true},
		{name: SessionNamePrefix + strings.Repeat("a", 120), want: true}, // 128 bytes
		{name: SessionNamePrefix + strings.Repeat("a", 121), want: false},
		{name: "", want: false},
		{name: ".shopper-leading-dot", want: false},
		{name: "shopper-a b", want: false},
		{name: "shopper-a+b", want: false},
		{name: "shopper-%2Dencoded", want: false},
		{name: "shopper-ä", want: false},
		{name: "shopper/slash", want: false},
		{name: "shopper-Shopper:01_A.B-2", want: true},
	}
	for _, tc := range tests {
		if got := ValidSessionName(tc.name); got != tc.want {
			t.Errorf("ValidSessionName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSeedSaltIsThePublishedValue(t *testing.T) {
	t.Parallel()

	if len(SeedSaltHex) != 64 {
		t.Fatalf("SeedSaltHex is %d characters, want 64", len(SeedSaltHex))
	}
	if strings.ToLower(SeedSaltHex) != SeedSaltHex {
		t.Errorf("SeedSaltHex is not lowercase: %q", SeedSaltHex)
	}
	decoded, err := hex.DecodeString(SeedSaltHex)
	if err != nil {
		t.Fatalf("SeedSaltHex is not hex: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("SeedSaltHex decodes to %d bytes, want 32", len(decoded))
	}

	sum := sha256.Sum256([]byte("memento-emptor seed identity salt v1"))
	if want := hex.EncodeToString(sum[:]); SeedSaltHex != want {
		t.Errorf("SeedSaltHex = %q, want sha256 of the published phrase %q", SeedSaltHex, want)
	}
}

func TestDifferentSaltsProduceDifferentKeys(t *testing.T) {
	t.Parallel()

	other, err := NewKeyer(testSaltPhrase(t, "memento-emptor test alternate salt"))
	if err != nil {
		t.Fatalf("alternate keyer: %v", err)
	}
	k := testKeyer(t)
	line := domain.ProductLine{Brand: "sunspel", Category: "shirt", FitHandle: "regular"}

	seedName, err := k.SessionName("PAYER-1")
	if err != nil {
		t.Fatalf("seed session name: %v", err)
	}
	otherName, err := other.SessionName("PAYER-1")
	if err != nil {
		t.Fatalf("alternate session name: %v", err)
	}
	if seedName == otherName {
		t.Error("two salts produced the same session name")
	}

	seedLine, err := k.LineShopperHash("PAYER-1", line)
	if err != nil {
		t.Fatalf("seed line hash: %v", err)
	}
	otherLine, err := other.LineShopperHash("PAYER-1", line)
	if err != nil {
		t.Fatalf("alternate line hash: %v", err)
	}
	if seedLine == otherLine {
		t.Error("two salts produced the same per-line hash")
	}
}
