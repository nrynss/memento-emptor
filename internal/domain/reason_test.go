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

func TestDisputeCodesListsAll(t *testing.T) {
	for _, code := range DisputeCodes {
		if _, err := CategoryForDispute(code); err != nil {
			t.Errorf("DisputeCodes entry %q has no mapping: %v", code, err)
		}
	}
}
