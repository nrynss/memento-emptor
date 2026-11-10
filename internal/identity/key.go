package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// SessionNamePrefix is the graph credential's session prefix. Every session
// name starts with it, so the hub can attach a shopper's session on demand.
const SessionNamePrefix = "shopper-"

// MaxAddressedNameBytes is the graph's addressed-name limit: a session name
// is at most this many bytes. The constructed name is 72 bytes, inside it.
const MaxAddressedNameBytes = 128

// saltHexLen and saltBytes are the identity salt's two lengths: 64 hex
// characters decode to the 32 key bytes.
const (
	saltHexLen = 64
	saltBytes  = 32
)

// The fixed version tags every HMAC message starts with. A tag keeps the
// purpose of a message in the message itself, so a session hash can never
// be confused with a per-line hash even under the same salt.
const (
	sessionMessagePrefix = "memento/session/v1|"
	lineMessagePrefix    = "memento/line-shopper/v1|"
	txnMessagePrefix     = "txn|"
)

// Errors returned when a key cannot be built. A missing or malformed salt
// is never replaced by a default: the caller must supply the keyring's
// value, or SeedSaltHex for seeded rows.
var (
	// ErrMissingSalt means no salt was supplied.
	ErrMissingSalt = errors.New("identity: identity salt is missing")
	// ErrMalformedSalt means the salt is not 64 hexadecimal characters
	// decoding to 32 bytes.
	ErrMalformedSalt = errors.New("identity: identity salt is malformed")
	// ErrEmptyPayerID means the payer id is empty.
	ErrEmptyPayerID = errors.New("identity: payer id is empty")
	// ErrEmptyLine means a product line field is empty. Every fit row
	// carries all three fields.
	ErrEmptyLine = errors.New("identity: product line field is empty")
	// ErrEmptyTransactionID means the transaction id is empty.
	ErrEmptyTransactionID = errors.New("identity: transaction id is empty")
)

// Keyer builds the salted keys the layer stores: the shopper's session name
// and the per-line shopper hash. It holds one identity salt for its whole
// life and never reads a global or the environment. A Keyer is safe for
// concurrent use.
type Keyer struct {
	salt []byte
}

// NewKeyer returns a Keyer for the given identity salt, as 64 hexadecimal
// characters decoding to 32 bytes. The keyring holds the live salt and the
// value is never written down; SeedSaltHex is the published salt for seeded
// rows. An empty salt returns ErrMissingSalt and anything malformed returns
// ErrMalformedSalt; neither falls back to a default. The error never
// repeats the supplied value.
func NewKeyer(hexSalt string) (*Keyer, error) {
	if hexSalt == "" {
		return nil, ErrMissingSalt
	}
	if len(hexSalt) != saltHexLen {
		return nil, fmt.Errorf("%w: need %d hexadecimal characters, got %d", ErrMalformedSalt, saltHexLen, len(hexSalt))
	}
	salt, err := hex.DecodeString(hexSalt)
	if err != nil {
		return nil, fmt.Errorf("%w: need %d hexadecimal characters", ErrMalformedSalt, saltHexLen)
	}
	if len(salt) != saltBytes {
		return nil, fmt.Errorf("%w: need %d key bytes, got %d", ErrMalformedSalt, saltBytes, len(salt))
	}
	return &Keyer{salt: salt}, nil
}

// SessionName returns the graph session name for a payer id:
// SessionNamePrefix followed by the lowercase hex HMAC-SHA256 of
// "memento/session/v1|" plus the length-prefixed payer id, keyed with the
// identity salt. The result is 72 bytes, inside the addressed-name rule.
// An empty payer id returns ErrEmptyPayerID.
func (k *Keyer) SessionName(payerID string) (string, error) {
	if k == nil || len(k.salt) == 0 {
		return "", ErrMissingSalt
	}
	if payerID == "" {
		return "", ErrEmptyPayerID
	}
	return SessionNamePrefix + hex.EncodeToString(k.mac(sessionMessagePrefix, payerID)), nil
}

// LineShopperHash returns the per-line shopper hash for a payer id and a
// product line: the lowercase hex HMAC-SHA256 of
// "memento/line-shopper/v1|" plus the length-prefixed brand, category, fit
// handle and payer id, in that order, keyed with the identity salt. The same
// payer id in the same line is stable; the same payer id in another line is
// unrelated, so the hash counts distinct shoppers within one line without
// linking a shopper across lines. An empty payer id returns ErrEmptyPayerID
// and an empty line field returns ErrEmptyLine.
func (k *Keyer) LineShopperHash(payerID string, line domain.ProductLine) (string, error) {
	if k == nil || len(k.salt) == 0 {
		return "", ErrMissingSalt
	}
	if payerID == "" {
		return "", ErrEmptyPayerID
	}
	switch {
	case line.Brand == "":
		return "", fmt.Errorf("%w: brand", ErrEmptyLine)
	case line.Category == "":
		return "", fmt.Errorf("%w: category", ErrEmptyLine)
	case line.FitHandle == "":
		return "", fmt.Errorf("%w: fit handle", ErrEmptyLine)
	}
	return hex.EncodeToString(k.mac(lineMessagePrefix, line.Brand, line.Category, line.FitHandle, payerID)), nil
}

// TxnHash returns a purchase's transaction hash: the lowercase hex SHA-256
// of "txn|" followed by the transaction id, exactly as the fit index pins
// it. The hash is unsalted, so the row id stays stable when the identity
// salt rotates. An empty transaction id returns ErrEmptyTransactionID.
func TxnHash(txnID string) (string, error) {
	if txnID == "" {
		return "", ErrEmptyTransactionID
	}
	sum := sha256.Sum256([]byte(txnMessagePrefix + txnID))
	return hex.EncodeToString(sum[:]), nil
}

// ValidSessionName reports whether name satisfies the graph's addressed-name
// rule: 1 to 128 bytes, each in [A-Za-z0-9._:-], with no leading dot. It
// judges the rule, not the credential's prefix; a session this package
// builds always begins with SessionNamePrefix.
func ValidSessionName(name string) bool {
	if len(name) == 0 || len(name) > MaxAddressedNameBytes {
		return false
	}
	if name[0] == '.' {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == ':' || c == '-':
		default:
			return false
		}
	}
	return true
}

// mac writes the version tag and each length-prefixed field to an
// HMAC-SHA256 keyed with the identity salt, and returns the digest.
func (k *Keyer) mac(prefix string, fields ...string) []byte {
	h := hmac.New(sha256.New, k.salt)
	h.Write([]byte(prefix))
	var buf []byte
	for _, field := range fields {
		buf = strconv.AppendInt(buf[:0], int64(len(field)), 10)
		buf = append(buf, ':')
		h.Write(buf)
		h.Write([]byte(field))
	}
	return h.Sum(nil)
}
