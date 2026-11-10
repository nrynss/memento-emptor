package fitindex

import (
	"context"
	"fmt"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/identity"
)

// testSalt is a fixed identity salt for the tests. It is not the keyring's
// salt and no live value appears here.
const testSalt = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

// testLine is the line every writer test writes in.
var testLine = domain.ProductLine{Brand: "brand-x-001", Category: "shirts", FitHandle: "regular"}

// otherLine is a second line in the same brand, so one shopper's two lines
// share nothing.
var otherLine = domain.ProductLine{Brand: "brand-x-001", Category: "pants", FitHandle: "slim"}

// testKeyer builds a keyer over the fixed test salt.
func testKeyer(t testingT) *identity.Keyer {
	t.Helper()
	keyer, err := identity.NewKeyer(testSalt)
	if err != nil {
		t.Fatalf("build keyer: %v", err)
	}
	return keyer
}

// testingT is the part of testing.T the helpers use.
type testingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// fakeIndex is the fit index in memory, with the recorded write rules: a
// whole document per purchase, an equal version accepted and an older one
// refused. It records what reached it, so a test can assert that a path
// wrote nothing.
type fakeIndex struct {
	created  int
	docs     map[string]domain.FitRow
	versions map[string]int64
	writes   int
	deleted  int
	putErr   error
}

// newFakeIndex builds an empty index.
func newFakeIndex() *fakeIndex {
	return &fakeIndex{
		docs:     make(map[string]domain.FitRow),
		versions: make(map[string]int64),
	}
}

// Create records that the index was created from the mapping.
func (f *fakeIndex) Create(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.created++
	return nil
}

// Put replaces one row at its version, accepting an equal version and
// refusing an older one, as external_gte does.
func (f *fakeIndex) Put(ctx context.Context, row domain.FitRow) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.putErr != nil {
		return f.putErr
	}
	version := row.Version()
	if held, ok := f.versions[row.TxnHash]; ok && version < held {
		return fmt.Errorf("%w: row %q holds version %d, write is %d", ErrOlderEvent, row.TxnHash, held, version)
	}
	f.docs[row.TxnHash] = row
	f.versions[row.TxnHash] = version
	f.writes++
	return nil
}

// DeleteShopper removes one shopper's rows in one line and reports how
// many went.
func (f *fakeIndex) DeleteShopper(ctx context.Context, line domain.ProductLine, shopperHash string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	removed := 0
	for txnHash, row := range f.docs {
		if row.Line == line && row.ShopperHash == shopperHash {
			delete(f.docs, txnHash)
			delete(f.versions, txnHash)
			removed++
		}
	}
	f.deleted += removed
	return removed, nil
}

// only returns the one row the index holds, failing when it holds another
// number of documents.
func (f *fakeIndex) only(t testingT) domain.FitRow {
	t.Helper()
	if len(f.docs) != 1 {
		t.Fatalf("index holds %d documents, want 1", len(f.docs))
	}
	for _, row := range f.docs {
		return row
	}
	return domain.FitRow{}
}
