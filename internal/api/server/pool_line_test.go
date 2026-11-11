package server

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// TestRecallResolvesTheOneLineForBrandAndCategory proves the live why view's
// shape: the page names the brand and category it honestly knows and no fit
// handle, and recall resolves the one line the population seeds for them,
// consults the pool with it, and serves the pool.
func TestRecallResolvesTheOneLineForBrandAndCategory(t *testing.T) {
	ctx := context.Background()
	poolIdx := &fakePoolIndex{result: earnedSmallPool()}
	svc, _ := pivotHarness(t, poolIdx, &fakeFitIndex{}, "")

	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: "new_shopper", BrandID: "brand-x-001", Category: "shirts", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if recall.PoolLookup != contract.PoolConsulted {
		t.Fatalf("pool lookup %q, want consulted: the brand and category name one population line", recall.PoolLookup)
	}
	if recall.Pool == nil {
		t.Fatal("recall carries no pool, want the Earned small answer")
	}
	if recall.Pool.Status != domain.StatusEarned || recall.Pool.Direction == nil ||
		*recall.Pool.Direction != domain.FitDirectionSmall {
		t.Fatalf("pool reads %+v, want Earned small", recall.Pool)
	}
	if recall.Pool.Line.FitHandle != "regular" {
		t.Fatalf("pool line names handle %q, want the population's regular", recall.Pool.Line.FitHandle)
	}
	want := domain.ProductLine{Brand: "brand-x-001", Category: "shirts", FitHandle: "regular"}
	if query := poolIdx.lastQuery(); query.Line != want || query.Size != "M" {
		t.Fatalf("pool query %+v, want line %+v at size M", query, want)
	}
	if recall.Guidance.Basis != contract.GuidancePool {
		t.Fatalf("guidance %+v, want the pool deciding", recall.Guidance)
	}
}

// TestRecallLeavesThePoolUnavailableWhenNoLineMatches proves a brand and
// category the population never seeded does not invent a line: the lookup
// reads unavailable, no pool rides, and the index is never asked.
func TestRecallLeavesThePoolUnavailableWhenNoLineMatches(t *testing.T) {
	ctx := context.Background()
	poolIdx := &fakePoolIndex{result: earnedSmallPool()}
	svc, _ := pivotHarness(t, poolIdx, &fakeFitIndex{}, "")

	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: "new_shopper", BrandID: "brand-unseeded-999", Category: "shirts", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if recall.PoolLookup != contract.PoolUnavailable {
		t.Fatalf("pool lookup %q, want unavailable", recall.PoolLookup)
	}
	if recall.Pool != nil {
		t.Fatalf("recall serves pool %+v, want none", recall.Pool)
	}
	if len(poolIdx.queries) != 0 {
		t.Fatalf("the index was asked %d times, want none: there is no line to read", len(poolIdx.queries))
	}
}

// TestRecallLeavesThePoolUnavailableWhenTwoLinesMatch proves an ambiguous
// brand and category does not pick a line: with two handles seeded for it,
// the lookup reads unavailable, no pool rides, and the index is never
// asked.
func TestRecallLeavesThePoolUnavailableWhenTwoLinesMatch(t *testing.T) {
	ctx := context.Background()
	poolIdx := &fakePoolIndex{result: earnedSmallPool()}
	svc, _ := pivotHarness(t, poolIdx, &fakeFitIndex{}, "")
	svc.population = []domain.ProductLine{
		{Brand: "brand-x-001", Category: "shirts", FitHandle: "regular"},
		{Brand: "brand-x-001", Category: "shirts", FitHandle: "slim"},
	}

	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: "new_shopper", BrandID: "brand-x-001", Category: "shirts", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if recall.PoolLookup != contract.PoolUnavailable {
		t.Fatalf("pool lookup %q, want unavailable", recall.PoolLookup)
	}
	if recall.Pool != nil {
		t.Fatalf("recall serves pool %+v, want none", recall.Pool)
	}
	if len(poolIdx.queries) != 0 {
		t.Fatalf("the index was asked %d times, want none: two lines are not one line", len(poolIdx.queries))
	}
}

// TestRecallUsesTheFitHandleTheRequestNames proves a request that states its
// line outright is still read as given: an explicit handle is not replaced
// by the population's line for the brand and category.
func TestRecallUsesTheFitHandleTheRequestNames(t *testing.T) {
	ctx := context.Background()
	poolIdx := &fakePoolIndex{result: earnedSmallPool()}
	svc, _ := pivotHarness(t, poolIdx, &fakeFitIndex{}, "")

	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: "new_shopper", BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "slim", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if recall.PoolLookup != contract.PoolConsulted {
		t.Fatalf("pool lookup %q, want consulted", recall.PoolLookup)
	}
	want := domain.ProductLine{Brand: "brand-x-001", Category: "shirts", FitHandle: "slim"}
	if query := poolIdx.lastQuery(); query.Line != want {
		t.Fatalf("pool query line %+v, want the request's own %+v", query.Line, want)
	}
}

// TestRecallWithoutALineLeavesThePoolUnavailable proves the pre-pivot shape
// is unchanged for a request that names no brand or category: no line can
// be named, the lookup reads unavailable, no pool rides, and the index is
// never asked.
func TestRecallWithoutALineLeavesThePoolUnavailable(t *testing.T) {
	ctx := context.Background()
	poolIdx := &fakePoolIndex{result: earnedSmallPool()}
	svc, _ := pivotHarness(t, poolIdx, &fakeFitIndex{}, "")

	for _, req := range []contract.RecallRequest{
		{ShopperID: "new_shopper", SizeConsidered: "M"},
		{ShopperID: "new_shopper", BrandID: "brand-x-001", SizeConsidered: "M"},
		{ShopperID: "new_shopper", Category: "shirts", SizeConsidered: "M"},
	} {
		recall, err := svc.Recall(ctx, req)
		if err != nil {
			t.Fatalf("recall %+v: %v", req, err)
		}
		if recall.PoolLookup != contract.PoolUnavailable || recall.Pool != nil {
			t.Fatalf("recall %+v reads %q/%+v, want unavailable with no pool", req, recall.PoolLookup, recall.Pool)
		}
	}
	if len(poolIdx.queries) != 0 {
		t.Fatalf("the index was asked %d times, want none: no request named a line", len(poolIdx.queries))
	}
}

// TestResolvedLineRecallKeepsTheFrozenShape proves a recall that resolved its
// line still carries exactly the frozen response keys: the resolution changes
// nothing about the wire shape the contract fixes.
func TestResolvedLineRecallKeepsTheFrozenShape(t *testing.T) {
	ctx := context.Background()
	svc, _ := pivotHarness(t, &fakePoolIndex{result: earnedSmallPool()}, &fakeFitIndex{}, "")

	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: "new_shopper", BrandID: "brand-x-001", Category: "shirts", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	raw, err := json.Marshal(recall)
	if err != nil {
		t.Fatalf("marshal recall: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal recall: %v", err)
	}
	keys := make([]string, 0, len(doc))
	for key := range doc {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	want := []string{
		"beliefs", "guidance", "overriding_correction", "pending_count",
		"pool", "pool_lookup", "shopper_id", "trace_id",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("recall response keys %v, want the frozen %v", keys, want)
	}
}

// TestPopulationLineNamesOnlyAnExactSingleMatch pins the resolver on its own:
// one match is the line, no match and two matches are nothing.
func TestPopulationLineNamesOnlyAnExactSingleMatch(t *testing.T) {
	lines := []domain.ProductLine{
		{Brand: "brand-a", Category: "shirts", FitHandle: "regular"},
		{Brand: "brand-b", Category: "shirts", FitHandle: "slim"},
		{Brand: "brand-b", Category: "shirts", FitHandle: "relaxed"},
		{Brand: "brand-c", Category: "jeans", FitHandle: "regular"},
	}
	cases := []struct {
		name     string
		brand    string
		category string
		want     domain.ProductLine
		wantOK   bool
	}{
		{"one match", "brand-a", "shirts", lines[0], true},
		{"no brand match", "brand-z", "shirts", domain.ProductLine{}, false},
		{"no category match", "brand-a", "jeans", domain.ProductLine{}, false},
		{"two handles", "brand-b", "shirts", domain.ProductLine{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := populationLine(lines, tc.brand, tc.category)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("populationLine(%q, %q) = %+v, %v; want %+v, %v",
					tc.brand, tc.category, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
