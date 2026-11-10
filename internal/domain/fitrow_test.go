package domain

import (
	"testing"
	"time"
)

// TestFitPurchaseStateRank pins the version order: captured, then
// refunded, then reason classified, then settled or contested. An unknown
// state ranks zero.
func TestFitPurchaseStateRank(t *testing.T) {
	cases := []struct {
		state FitPurchaseState
		rank  int
	}{
		{FitStateCaptured, 1},
		{FitStateRefunded, 2},
		{FitStateReasonClassified, 3},
		{FitStateFeedback, 4},
		{FitStateKept, 4},
		{FitStateContested, 4},
		{FitPurchaseState("unknown"), 0},
		{FitPurchaseState(""), 0},
	}
	for _, tc := range cases {
		if got := tc.state.Rank(); got != tc.rank {
			t.Errorf("FitPurchaseState(%q).Rank() = %d, want %d", tc.state, got, tc.rank)
		}
	}
}

// TestFitRowVersion pins the write version: state rank first, event time
// second. A later state wins over an earlier one whatever the event times,
// and inside one rank the later event wins.
func TestFitRowVersion(t *testing.T) {
	early := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	late := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	captured := FitRow{State: FitStateCaptured, EventAt: late}
	refunded := FitRow{State: FitStateRefunded, EventAt: early}
	classified := FitRow{State: FitStateReasonClassified, EventAt: early}
	feedback := FitRow{State: FitStateFeedback, EventAt: early}
	contested := FitRow{State: FitStateContested, EventAt: late}
	kept := FitRow{State: FitStateKept, EventAt: late}
	cases := []struct {
		name string
		low  FitRow
		high FitRow
	}{
		{"time orders one rank", FitRow{State: FitStateCaptured, EventAt: early}, captured},
		{"rank beats time", captured, refunded},
		{"rank beats time again", refunded, classified},
		{"settled follows the classified reason", classified, feedback},
		{"inside the settled rank the later event wins", feedback, contested},
		{"a later keep wins the same rank", feedback, kept},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !(tc.low.Version() < tc.high.Version()) {
				t.Errorf("version(%s/%s) = %d, want below version(%s/%s) = %d",
					tc.low.State, tc.low.EventAt, tc.low.Version(),
					tc.high.State, tc.high.EventAt, tc.high.Version())
			}
		})
	}
	retry := FitRow{State: FitStateReasonClassified, EventAt: early}
	if retry.Version() != classified.Version() {
		t.Errorf("a retry of one event changed the version: %d then %d", classified.Version(), retry.Version())
	}
}

// TestFitRowMapping pins the vocabulary the fit index stores, as T0.12
// recorded it: every purchase state, settled kind and feedback verdict.
func TestFitRowMapping(t *testing.T) {
	states := map[FitPurchaseState]string{
		FitStateCaptured:         "captured",
		FitStateRefunded:         "refunded",
		FitStateReasonClassified: "reason_classified",
		FitStateFeedback:         "feedback",
		FitStateKept:             "kept",
		FitStateContested:        "contested",
	}
	for state, want := range states {
		if string(state) != want {
			t.Errorf("purchase state = %q, want %q", state, want)
		}
	}
	settled := map[FitSettledKind]string{
		FitSettledByReturnReason: "return_reason",
		FitSettledByFeedback:     "feedback",
		FitSettledByKept:         "kept",
	}
	for kind, want := range settled {
		if string(kind) != want {
			t.Errorf("settled kind = %q, want %q", kind, want)
		}
	}
	verdicts := []struct {
		verdict   FitVerdict
		value     string
		direction FitDirection
	}{
		{FitVerdictFitsWell, "fits_well", FitDirectionNone},
		{FitVerdictTooSmall, "too_small", FitDirectionSmall},
		{FitVerdictTooLarge, "too_large", FitDirectionLarge},
	}
	for _, tc := range verdicts {
		if string(tc.verdict) != tc.value {
			t.Errorf("verdict = %q, want %q", tc.verdict, tc.value)
		}
		if got := tc.verdict.Direction(); got != tc.direction {
			t.Errorf("FitVerdict(%q).Direction() = %q, want %q", tc.verdict, got, tc.direction)
		}
	}
	row := FitRow{
		Line:        ProductLine{Brand: "brand-x", Category: "shirts", FitHandle: "regular"},
		Size:        "M",
		State:       FitStateFeedback,
		Reason:      CategoryNone,
		Direction:   FitDirectionNone,
		Verdict:     FitVerdictFitsWell,
		Settled:     true,
		SettledKind: FitSettledByFeedback,
		ShopperHash: "shopper-hash",
		TxnHash:     "txn-hash",
		Seeded:      true,
	}
	if row.Line.Brand != "brand-x" || row.Line.Category != "shirts" || row.Line.FitHandle != "regular" {
		t.Errorf("product line = %+v, want brand, category and fit handle", row.Line)
	}
	if row.Version() <= 0 {
		t.Errorf("version = %d, want a positive version", row.Version())
	}
}
