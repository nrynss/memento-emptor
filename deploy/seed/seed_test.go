package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
	"github.com/nrynss/memento-emptor/seed"
)

// memIndex is a fake fit index: it stores each purchase's whole document
// and refuses an older version the way the deployment does, so the test
// exercises the writer's transitions without touching the network.
type memIndex struct {
	mu   sync.Mutex
	rows map[string]domain.FitRow
	puts int
}

func newMemIndex() *memIndex {
	return &memIndex{rows: map[string]domain.FitRow{}}
}

func (m *memIndex) Create(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (m *memIndex) Put(ctx context.Context, row domain.FitRow) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if held, ok := m.rows[row.TxnHash]; ok && held.Version() > row.Version() {
		return fitindex.ErrOlderEvent
	}
	m.rows[row.TxnHash] = row
	m.puts++
	return nil
}

func (m *memIndex) DeleteShopper(ctx context.Context, line domain.ProductLine, shopperHash string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := 0
	for id, row := range m.rows {
		if row.Line == line && row.ShopperHash == shopperHash {
			delete(m.rows, id)
			removed++
		}
	}
	return removed, nil
}

// poolCountsFor is a test-only read of stored rows, mirroring the pool's
// settled-only semantics: one purchase per shopper, fit returns and fit
// feedback, fits-well verdicts diluting the share. Since bounds the recent
// window; the zero time counts everything.
func poolCountsFor(rows map[string]domain.FitRow, line domain.ProductLine, size string, since time.Time) domain.PoolCounts {
	var counts domain.PoolCounts
	fit := map[string]bool{}
	small := map[string]bool{}
	large := map[string]bool{}
	for _, row := range rows {
		if row.Line != line || row.Size != size || !row.Settled {
			continue
		}
		if !since.IsZero() && row.SettledAt.Before(since) {
			continue
		}
		switch {
		case row.State == domain.FitStateFeedback && row.Verdict == domain.FitVerdictTooSmall:
			counts.TooSmall++
		case row.State == domain.FitStateFeedback && row.Verdict == domain.FitVerdictTooLarge:
			counts.TooLarge++
		case row.State == domain.FitStateFeedback && row.Verdict == domain.FitVerdictFitsWell:
			counts.FitsWell++
		case row.State == domain.FitStateReasonClassified && row.Direction == domain.FitDirectionSmall:
			counts.TooSmall++
		case row.State == domain.FitStateReasonClassified && row.Direction == domain.FitDirectionLarge:
			counts.TooLarge++
		}
		if row.State == domain.FitStateFeedback || row.Direction != domain.FitDirectionNone {
			fit[row.ShopperHash] = true
		}
		if row.Direction == domain.FitDirectionSmall {
			small[row.ShopperHash] = true
		}
		if row.Direction == domain.FitDirectionLarge {
			large[row.ShopperHash] = true
		}
	}
	counts.Shoppers = len(fit)
	counts.ShoppersTooSmall = len(small)
	counts.ShoppersTooLarge = len(large)
	return counts
}

// clearHashes blanks the fields the seed mints per run, so a stored
// document compares against its generated row. An empty reason and the
// none category index identically (the document omits both), so they
// compare equal here too.
func clearHashes(row domain.FitRow) domain.FitRow {
	row.ShopperHash = ""
	row.TxnHash = ""
	if row.Reason == "" {
		row.Reason = domain.CategoryNone
	}
	return row
}

func TestSeedRowsMatchPopulation(t *testing.T) {
	asOf := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	population, err := seed.GeneratePopulation(asOf)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	fake := newMemIndex()
	writes, _, err := seedRows(context.Background(), fake, population.Rows)
	if err != nil {
		t.Fatalf("seedRows: %v", err)
	}
	if len(fake.rows) != len(population.Rows) {
		t.Fatalf("stored %d documents, want %d (one per purchase)", len(fake.rows), len(population.Rows))
	}
	wantWrites := 0
	for _, row := range population.Rows {
		wantWrites += transitionWrites(row)
	}
	if writes != wantWrites || fake.puts != wantWrites {
		t.Fatalf("writes = %d, puts = %d, want %d", writes, fake.puts, wantWrites)
	}
	for i, want := range population.Rows {
		txnHash, err := identity.TxnHash(seedTxnID(i))
		if err != nil {
			t.Fatalf("TxnHash: %v", err)
		}
		got, ok := fake.rows[txnHash]
		if !ok {
			t.Fatalf("row %d: no document for %s", i, seedTxnID(i))
		}
		if clearHashes(got) != clearHashes(want) {
			t.Errorf("row %d: stored %+v, want %+v", i, clearHashes(got), clearHashes(want))
		}
		if !got.Seeded {
			t.Errorf("row %d: stored row lost the seeded mark", i)
		}
	}
}

// seedTxnID repeats the seeder's transaction id scheme, so the test names
// the document the seed writes for row i.
func seedTxnID(i int) string {
	return fmt.Sprintf("seedlive-%04d", i+1)
}

func TestSeededStatusesEarnHonestly(t *testing.T) {
	asOf := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	population, err := seed.GeneratePopulation(asOf)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	fake := newMemIndex()
	if _, _, err := seedRows(context.Background(), fake, population.Rows); err != nil {
		t.Fatalf("seedRows: %v", err)
	}
	since := asOf.Add(-domain.PoolRecentWindow)
	for _, want := range expectedPools {
		all := poolCountsFor(fake.rows, want.line, want.size, time.Time{})
		recent := poolCountsFor(fake.rows, want.line, want.size, since)
		got := domain.DecidePool(all, recent)
		if got.Status != want.status || got.Direction != want.direction {
			t.Errorf("%s/%s/%s size %s: status %s %s, want %s %s (all %+v, recent %+v)",
				want.line.Brand, want.line.Category, want.line.FitHandle, want.size,
				got.Status, got.Direction, want.status, want.direction, all, recent)
		}
	}
}
