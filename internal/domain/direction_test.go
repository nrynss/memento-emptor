package domain

import "testing"

func TestCategoryDirection(t *testing.T) {
	cases := []struct {
		category Category
		want     FitDirection
	}{
		{CategoryFitTooSmall, FitDirectionSmall},
		{CategoryFitTooLarge, FitDirectionLarge},
		{CategoryDamagedOrDefective, FitDirectionNone},
		{CategoryNotArrived, FitDirectionNone},
		{CategoryChangedMind, FitDirectionNone},
		{CategoryUnclear, FitDirectionNone},
		{CategoryNone, FitDirectionNone},
	}
	for _, tc := range cases {
		if got := tc.category.Direction(); got != tc.want {
			t.Errorf("Category(%q).Direction() = %q, want %q", tc.category, got, tc.want)
		}
	}
}

func TestDirectionForValue(t *testing.T) {
	cases := []struct {
		value string
		want  FitDirection
	}{
		{"runs small, take size L", FitDirectionSmall},
		{"runs large, take size S", FitDirectionLarge},
		{"size M fits this shopper now", FitDirectionNone},
		{"shopper says they are usually size M", FitDirectionNone},
		{"ships reliably for this shopper", FitDirectionNone},
		{"", FitDirectionNone},
	}
	for _, tc := range cases {
		if got := DirectionForValue(tc.value); got != tc.want {
			t.Errorf("DirectionForValue(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}
