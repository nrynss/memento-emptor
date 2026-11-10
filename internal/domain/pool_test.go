package domain

import (
	"testing"
	"time"
)

// TestDecidePool covers the rule's boundaries: the Building floor at three
// distinct shoppers, the Earned floor at eight, the 60% share, twice the
// opposite direction, and the recent window that flips a label to Out of
// true.
func TestDecidePool(t *testing.T) {
	small := FitDirectionSmall
	large := FitDirectionLarge
	earnedSmall := PoolCounts{
		TooSmall: 16, TooLarge: 8,
		ShoppersTooSmall: 8, ShoppersTooLarge: 8, Shoppers: 16,
	}
	cases := []struct {
		name   string
		all    PoolCounts
		recent PoolCounts
		status BeliefStatus
		way    FitDirection
	}{
		{"two shoppers say nothing", PoolCounts{TooSmall: 2, ShoppersTooSmall: 2, Shoppers: 2}, PoolCounts{}, StatusClaimed, FitDirectionNone},
		{"three shoppers build", PoolCounts{TooSmall: 3, ShoppersTooSmall: 3, Shoppers: 3}, PoolCounts{}, StatusBuilding, small},
		{"seven shoppers build", PoolCounts{TooSmall: 7, ShoppersTooSmall: 7, Shoppers: 7}, PoolCounts{}, StatusBuilding, small},
		{"eight shoppers earn", PoolCounts{TooSmall: 8, ShoppersTooSmall: 8, Shoppers: 8}, PoolCounts{}, StatusEarned, small},
		{"eight shoppers in the larger direction earn", PoolCounts{TooLarge: 9, ShoppersTooLarge: 9, Shoppers: 9}, PoolCounts{}, StatusEarned, large},
		{"59% of fit evidence builds", PoolCounts{TooSmall: 59, FitsWell: 41, ShoppersTooSmall: 8, Shoppers: 8}, PoolCounts{}, StatusBuilding, small},
		{"60% of fit evidence earns", PoolCounts{TooSmall: 60, FitsWell: 40, ShoppersTooSmall: 8, Shoppers: 8}, PoolCounts{}, StatusEarned, small},
		{"exactly twice the opposite earns", earnedSmall, PoolCounts{}, StatusEarned, small},
		{"less than twice the opposite builds", PoolCounts{TooSmall: 16, TooLarge: 9, ShoppersTooSmall: 8, ShoppersTooLarge: 9, Shoppers: 17}, PoolCounts{}, StatusBuilding, small},
		{"the recent window earning the opposite flips to out of true", earnedSmall, PoolCounts{TooLarge: 8, ShoppersTooLarge: 8, Shoppers: 8}, StatusOutOfTrue, small},
		{"the recent window failing the rule flips to out of true", earnedSmall, PoolCounts{TooSmall: 4, TooLarge: 2, ShoppersTooSmall: 4, ShoppersTooLarge: 2, Shoppers: 6}, StatusOutOfTrue, small},
		{"a recent window still earning keeps the label", earnedSmall, PoolCounts{TooSmall: 9, ShoppersTooSmall: 9, Shoppers: 9}, StatusEarned, small},
		{"a quiet window keeps the earned label", earnedSmall, PoolCounts{Shoppers: 2}, StatusEarned, small},
		{"fit evidence with no direction builds", PoolCounts{FitsWell: 4, Shoppers: 4}, PoolCounts{}, StatusBuilding, FitDirectionNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecidePool(tc.all, tc.recent)
			if got.Status != tc.status || got.Direction != tc.way {
				t.Errorf("DecidePool(%+v, %+v) = %s/%q, want %s/%q",
					tc.all, tc.recent, got.Status, got.Direction, tc.status, tc.way)
			}
		})
	}
}

// TestPoolThresholds pins the one definition of the pool's numbers: the
// document names the same values.
func TestPoolThresholds(t *testing.T) {
	if PoolBuildingShoppers != 3 {
		t.Errorf("PoolBuildingShoppers = %d, want 3", PoolBuildingShoppers)
	}
	if PoolEarnedShoppers != 8 {
		t.Errorf("PoolEarnedShoppers = %d, want 8", PoolEarnedShoppers)
	}
	if PoolEarnedSharePercent != 60 {
		t.Errorf("PoolEarnedSharePercent = %d, want 60", PoolEarnedSharePercent)
	}
	if PoolEarnedRatio != 2 {
		t.Errorf("PoolEarnedRatio = %d, want 2", PoolEarnedRatio)
	}
	if PoolRecentWindow != 90*24*time.Hour {
		t.Errorf("PoolRecentWindow = %s, want 90 days", PoolRecentWindow)
	}
}
