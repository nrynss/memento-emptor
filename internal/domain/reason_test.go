package domain

import (
	"errors"
	"testing"
)

func TestCategoryCountsToward(t *testing.T) {
	cases := []struct {
		category Category
		want     BeliefKind
	}{
		{CategoryFitTooSmall, BeliefKindFit},
		{CategoryFitTooLarge, BeliefKindFit},
		{CategoryDamagedOrDefective, BeliefKindSeller},
		{CategoryNotArrived, BeliefKindSeller},
		{CategoryChangedMind, BeliefKindNone},
		{CategoryUnclear, BeliefKindNone},
		{CategoryNone, BeliefKindNone},
	}
	for _, tc := range cases {
		if got := tc.category.CountsToward(); got != tc.want {
			t.Errorf("Category(%q).CountsToward() = %q, want %q", tc.category, got, tc.want)
		}
	}
}

func TestCategoriesCoversAll(t *testing.T) {
	if len(Categories) != 7 {
		t.Fatalf("len(Categories) = %d, want 7", len(Categories))
	}
	seen := map[Category]bool{}
	for _, c := range Categories {
		if seen[c] {
			t.Errorf("duplicate category %q", c)
		}
		seen[c] = true
	}
}

func TestCategoryForDispute(t *testing.T) {
	cases := []struct {
		code     DisputeCode
		category Category
		kind     BeliefKind
	}{
		{DisputeMerchandiseNotReceived, CategoryNotArrived, BeliefKindSeller},
		{DisputeItemNotReceived, CategoryNotArrived, BeliefKindSeller},
		// The not-as-described code files under damage: the goods arrived
		// and were wrong, so non-arrival would state a false fact about
		// the delivery.
		{DisputeMerchandiseNotAsDescribed, CategoryDamagedOrDefective, BeliefKindSeller},
		{DisputeUnauthorizedTransaction, CategoryNone, BeliefKindNone},
	}
	for _, tc := range cases {
		got, err := CategoryForDispute(tc.code)
		if err != nil {
			t.Errorf("CategoryForDispute(%q) returned error: %v", tc.code, err)
			continue
		}
		if got != tc.category {
			t.Errorf("CategoryForDispute(%q) = %q, want %q", tc.code, got, tc.category)
		}
		if kind := got.CountsToward(); kind != tc.kind {
			t.Errorf("CategoryForDispute(%q) counts toward %q, want %q", tc.code, kind, tc.kind)
		}
	}
}

// TestNotAsDescribedCountsAgainstTheSeller pins the disposition product.md
// promises: a not-as-described dispute is a seller-reliability signal, so
// it must never count toward sizing and must never read as non-arrival.
func TestNotAsDescribedCountsAgainstTheSeller(t *testing.T) {
	got, err := CategoryForDispute(DisputeMerchandiseNotAsDescribed)
	if err != nil {
		t.Fatalf("CategoryForDispute(%q) returned error: %v", DisputeMerchandiseNotAsDescribed, err)
	}
	if got == CategoryNotArrived {
		t.Errorf("not-as-described maps to %q; the parcel arrived, so that states the wrong fact", got)
	}
	if kind := got.CountsToward(); kind != BeliefKindSeller {
		t.Errorf("not-as-described counts toward %q, want seller reliability", kind)
	}
}

func TestCategoryForDisputeUnknown(t *testing.T) {
	got, err := CategoryForDispute("NOT_A_REAL_CODE")
	if !errors.Is(err, ErrUnknownDisputeCode) {
		t.Fatalf("CategoryForDispute(unknown) error = %v, want ErrUnknownDisputeCode", err)
	}
	if got != CategoryNone {
		t.Errorf("CategoryForDispute(unknown) = %q, want none", got)
	}
	if kind := got.CountsToward(); kind != BeliefKindNone {
		t.Errorf("unknown dispute code counts toward %q, want nothing", kind)
	}
}

// TestStoredCategoryForDispute pins the disposition the store takes: a code
// outside the frozen table is not an error, because the outcome is stored
// with no classified reason rather than dropped. Every table code maps as
// CategoryForDispute maps it.
func TestStoredCategoryForDispute(t *testing.T) {
	for _, code := range DisputeCodes {
		want, err := CategoryForDispute(code)
		if err != nil {
			t.Fatalf("CategoryForDispute(%q) returned error: %v", code, err)
		}
		got, err := StoredCategoryForDispute(code)
		if err != nil {
			t.Errorf("StoredCategoryForDispute(%q) returned error: %v", code, err)
			continue
		}
		if got != want {
			t.Errorf("StoredCategoryForDispute(%q) = %q, want %q", code, got, want)
		}
	}
	got, err := StoredCategoryForDispute("NOT_A_REAL_CODE")
	if err != nil {
		t.Fatalf("StoredCategoryForDispute(unknown) returned error: %v", err)
	}
	if got != CategoryNone {
		t.Errorf("StoredCategoryForDispute(unknown) = %q, want none", got)
	}
	if kind := got.CountsToward(); kind != BeliefKindNone {
		t.Errorf("stored unknown dispute code counts toward %q, want nothing", kind)
	}
}

func TestDisputeCodesListsAll(t *testing.T) {
	for _, code := range DisputeCodes {
		if _, err := CategoryForDispute(code); err != nil {
			t.Errorf("DisputeCodes entry %q has no mapping: %v", code, err)
		}
	}
}
