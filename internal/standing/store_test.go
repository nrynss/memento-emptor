package standing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/memory"
)

// testSnapshot builds one snapshot with a pooling choice, a declared
// correction, a named standing and an unscoped standing.
func testSnapshot() Snapshot {
	at := time.Date(2026, 11, 9, 10, 12, 0, 0, time.UTC)
	subject := domain.Subject{Kind: domain.SubjectBrand, ID: "brand-x-001", Name: "Brand X"}
	return Snapshot{
		Pooling: false,
		Corrections: []contract.CorrectionView{
			{CorrectionID: "corr_000001", Text: "I've lost weight, use M", ContradictsBeliefID: "blf_000001", RecordedAt: at.Format(time.RFC3339), TakesPrecedence: true},
			{CorrectionID: "corr_000002", Text: "Use M in this line", RecordedAt: at.Format(time.RFC3339), TakesPrecedence: true},
		},
		Journal: []memory.Correction{
			{Text: "Use M in this line", Subject: subject, Line: domain.ProductLine{Brand: "brand-x-001", Category: "shirts", FitHandle: "regular"}, Size: "M", RecordedAt: at},
		},
	}
}

// proveRoundTrip saves and loads one snapshot through a store and proves
// every field survives: the pooling choice, the views and the journal
// with its subject, line, size and time.
func proveRoundTrip(t *testing.T, store Store) {
	t.Helper()
	ctx := context.Background()
	want := testSnapshot()
	if err := store.Save(ctx, "shopper_01", want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, found, err := store.Load(ctx, "shopper_01")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !found {
		t.Fatal("load reports no snapshot after a save")
	}
	if got.Pooling != want.Pooling {
		t.Errorf("pooling %v, want %v", got.Pooling, want.Pooling)
	}
	if !reflect.DeepEqual(got.Corrections, want.Corrections) {
		t.Errorf("corrections differ:\nactual:   %+v\nwant: %+v", got.Corrections, want.Corrections)
	}
	if !reflect.DeepEqual(got.Journal, want.Journal) {
		t.Errorf("journal differs:\nactual:   %+v\nwant: %+v", got.Journal, want.Journal)
	}
}

// TestMemStoreRoundTrip proves the in-memory store keeps every field.
func TestMemStoreRoundTrip(t *testing.T) {
	proveRoundTrip(t, NewMemStore())
}

// TestDirStoreRoundTrip proves the file store keeps every field.
func TestDirStoreRoundTrip(t *testing.T) {
	store, err := NewDirStore(t.TempDir())
	if err != nil {
		t.Fatalf("new dir store: %v", err)
	}
	proveRoundTrip(t, store)
}

// TestLoadAbsentIsNotAnError proves a shopper with no snapshot loads as
// absent, which is how a new shopper starts from the defaults.
func TestLoadAbsentIsNotAnError(t *testing.T) {
	ctx := context.Background()
	for name, store := range map[string]Store{"mem": NewMemStore()} {
		t.Run(name, func(t *testing.T) {
			_, found, err := store.Load(ctx, "new_shopper")
			if err != nil {
				t.Fatalf("load absent: %v", err)
			}
			if found {
				t.Fatal("load reports a snapshot that was never saved")
			}
		})
	}
	dirStore, err := NewDirStore(t.TempDir())
	if err != nil {
		t.Fatalf("new dir store: %v", err)
	}
	if _, found, err := dirStore.Load(ctx, "new_shopper"); err != nil || found {
		t.Fatalf("dir load absent: found %v err %v, want false nil", found, err)
	}
}

// TestBadShopperIsRefused proves a shopper id that cannot name a file is
// refused on both paths, so no caller can escape the directory.
func TestBadShopperIsRefused(t *testing.T) {
	ctx := context.Background()
	store, err := NewDirStore(t.TempDir())
	if err != nil {
		t.Fatalf("new dir store: %v", err)
	}
	for _, shopper := range []string{"", ".", "..", "../x", "a/b", "a\\b"} {
		if err := store.Save(ctx, shopper, testSnapshot()); err == nil {
			t.Errorf("save accepts shopper %q", shopper)
		}
		if _, _, err := store.Load(ctx, shopper); err == nil {
			t.Errorf("load accepts shopper %q", shopper)
		}
	}
}

// TestCorruptFileIsRefused proves a file that does not decode, or that
// names a newer layout version, is refused rather than guessed at.
func TestCorruptFileIsRefused(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := NewDirStore(dir)
	if err != nil {
		t.Fatalf("new dir store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shopper_01.json"), []byte("{nope"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if _, _, err := store.Load(ctx, "shopper_01"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("load corrupt: %v, want %v", err, ErrCorrupt)
	}
	if err := os.WriteFile(filepath.Join(dir, "shopper_02.json"), []byte("{\"version\":99,\"pooling\":true}\n"), 0o600); err != nil {
		t.Fatalf("write future file: %v", err)
	}
	if _, _, err := store.Load(ctx, "shopper_02"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("load future version: %v, want %v", err, ErrCorrupt)
	}
}
