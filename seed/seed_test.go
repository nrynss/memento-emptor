package seed

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// historiesDir and hiddenDir name the committed fixture paths relative to
// this package: arm-facing histories and the hidden answers, apart.
const (
	historiesDir = "../testdata/seed/histories"
	hiddenDir    = "../testdata/seed/hidden"
)

// updateFixtures rewrites the committed fixtures when UPDATE_SEED_FIXTURES
// is set. The gate runs without it and compares instead.
func updateFixtures() bool {
	return os.Getenv("UPDATE_SEED_FIXTURES") != ""
}

func fixtureBundle(t *testing.T) Bundle {
	t.Helper()
	return Generate(GoldenSeed)
}

func TestGoldenFixturesMatchTheGenerator(t *testing.T) {
	bundle := fixtureBundle(t)
	encoded, answers, err := marshalBundle(bundle)
	if err != nil {
		t.Fatalf("encode bundle: %v", err)
	}
	if updateFixtures() {
		if err := Write(context.Background(), historiesDir, hiddenDir, GoldenSeed); err != nil {
			t.Fatalf("rewrite fixtures: %v", err)
		}
	}
	for _, h := range bundle.Histories {
		path := filepath.Join(historiesDir, h.ShopperID+".json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !bytes.Equal(raw, encoded[h.ShopperID]) {
			t.Errorf("fixture %s does not match Generate(%d); rerun with UPDATE_SEED_FIXTURES=1", path, GoldenSeed)
		}
	}
	raw, err := os.ReadFile(filepath.Join(hiddenDir, "answers.json"))
	if err != nil {
		t.Fatalf("read hidden answers: %v", err)
	}
	if !bytes.Equal(raw, answers) {
		t.Errorf("hidden answers fixture does not match Generate(%d); rerun with UPDATE_SEED_FIXTURES=1", GoldenSeed)
	}
}
func TestGenerateIsDeterministic(t *testing.T) {
	first, second := Generate(GoldenSeed), Generate(GoldenSeed)
	a, _, err := marshalBundle(first)
	if err != nil {
		t.Fatalf("encode first bundle: %v", err)
	}
	b, _, err := marshalBundle(second)
	if err != nil {
		t.Fatalf("encode second bundle: %v", err)
	}
	for id, raw := range a {
		if !bytes.Equal(raw, b[id]) {
			t.Errorf("history %q differs between runs with the same seed", id)
		}
	}
	other, _, err := marshalBundle(Generate(GoldenSeed + 1))
	if err != nil {
		t.Fatalf("encode other bundle: %v", err)
	}
	same := true
	for id, raw := range a {
		if !bytes.Equal(raw, other[id]) {
			same = false
		}
	}
	if same {
		t.Error("different seeds yield identical output; the seed changes nothing")
	}
}

func TestEveryRowCarriesTheSeedMark(t *testing.T) {
	anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, h := range fixtureBundle(t).Histories {
		for i, row := range h.Rows {
			if row.SeedMark != domain.SeedMark {
				t.Errorf("%s row %d: SeedMark = %q, want %q", h.ShopperID, i, row.SeedMark, domain.SeedMark)
			}
			if ev := row.SeededEvidence(anchor); !ev.Seeded {
				t.Errorf("%s row %d: materialized evidence lost the seeded mark", h.ShopperID, i)
			}
		}
	}
}

func TestDemoShopperMeetsPromotionGates(t *testing.T) {
	var bundle = fixtureBundle(t)
	var demo *History
	for i := range bundle.Histories {
		if bundle.Histories[i].ShopperID == "demo-shopper" {
			demo = &bundle.Histories[i]
		}
	}
	if demo == nil {
		t.Fatal("no demo-shopper history in the bundle")
	}
	var first, last float64
	fitDays := map[float64]bool{}
	fitCount := 0
	for _, row := range demo.Rows {
		ev := row.SeededEvidence(time.Now().UTC())
		switch ev.CountsToward() {
		case domain.BeliefKindFit:
			if row.Signal != domain.SignalClassifiedOutcome {
				t.Errorf("fit row %q uses signal %q, want classified outcome", row.Note, row.Signal)
			}
			if fitCount == 0 {
				first = row.Day
			}
			last = row.Day
			fitDays[row.Day] = true
			fitCount++
		case domain.BeliefKindSeller:
			if row.Signal != domain.SignalClassifiedOutcome {
				t.Errorf("seller row %q uses signal %q, want classified outcome", row.Note, row.Signal)
			}
		}
	}
	if fitCount != 3 {
		t.Errorf("demo fit evidence rows = %d, want exactly 3", fitCount)
	}
	if len(fitDays) != fitCount {
		t.Error("demo fit returns share an event day; each return needs its own interaction")
	}
	coverage := (last - first) / demo.SessionDays
	if coverage < 0.3 {
		t.Errorf("demo fit coverage = %.3f, want at least 0.3 over the session extent", coverage)
	}
	var sawSellerDispute bool
	for _, row := range demo.Rows {
		ev := row.SeededEvidence(time.Now().UTC())
		if ev.Outcome.Kind == domain.OutcomeDispute && ev.CountsToward() == domain.BeliefKindSeller {
			sawSellerDispute = true
			if ev.CountsToward() == domain.BeliefKindFit {
				t.Error("demo dispute counts toward fit; a never-arrived parcel is seller evidence only")
			}
		}
	}
	if !sawSellerDispute {
		t.Error("demo history has no seller-reliability dispute")
	}
}

func TestComparisonRowsNeverCountTheWrongCauseTowardFit(t *testing.T) {
	bundle := fixtureBundle(t)
	byID := map[string]History{}
	for _, h := range bundle.Histories {
		byID[h.ShopperID] = h
	}
	for _, id := range []string{"cmp-broken", "cmp-never-arrived", "cmp-changed-mind", "cmp-unclear"} {
		h, ok := byID[id]
		if !ok {
			t.Fatalf("no %s history in the bundle", id)
		}
		for _, row := range h.Rows {
			ev := row.SeededEvidence(time.Now().UTC())
			if row.Signal == domain.SignalClassifiedOutcome && ev.CountsToward() == domain.BeliefKindFit {
				t.Errorf("%s row %q counts toward fit; the tempting cause must not", id, row.Note)
			}
		}
	}
}

// TestHardenedShoppersAreFair pins the T5.7 hardening contract: the long
// noisy histories bury their fit evidence in at least ten mixed-signal
// rows with the fit rows spread honestly across the session, the
// multi-brand histories carry distinct cuts per brand, every non-fit
// classified row counts toward the seller or nothing (never fit), and each
// history's event times stay ordered inside its session extent.
func TestHardenedShoppersAreFair(t *testing.T) {
	bundle := fixtureBundle(t)
	byID := map[string]History{}
	for _, h := range bundle.Histories {
		byID[h.ShopperID] = h
	}
	for _, id := range []string{"cmp-noisy-small", "cmp-noisy-large", "cmp-brands-steady", "cmp-brands-split"} {
		h, ok := byID[id]
		if !ok {
			t.Fatalf("no %s history in the bundle", id)
		}
		if len(h.Rows) == 0 {
			t.Fatalf("%s has no rows", id)
		}
		for i := 1; i < len(h.Rows); i++ {
			if h.Rows[i].Day < h.Rows[i-1].Day {
				t.Errorf("%s row %d (day %.0f) precedes row %d (day %.0f): history must stay in event-time order",
					id, i, h.Rows[i].Day, i-1, h.Rows[i-1].Day)
			}
		}
		for i, row := range h.Rows {
			if row.Day < 0 || row.Day > h.SessionDays {
				t.Errorf("%s row %d: day %.0f outside session extent %.0f", id, i, row.Day, h.SessionDays)
			}
			if ev := row.SeededEvidence(time.Now().UTC()); ev.CountsToward() == domain.BeliefKindFit &&
				row.Signal == domain.SignalClassifiedOutcome && row.Reason == nil {
				t.Errorf("%s row %d: fit-counting row carries no reason", id, i)
			}
			if row.Signal == domain.SignalClassifiedOutcome && row.Reason != nil &&
				row.Reason.Category.CountsToward() != domain.BeliefKindFit &&
				row.SeededEvidence(time.Now().UTC()).CountsToward() == domain.BeliefKindFit {
				t.Errorf("%s row %q: non-fit reason %q counts toward fit", id, row.Note, row.Reason.Category)
			}
		}
	}
	for _, id := range []string{"cmp-noisy-small", "cmp-noisy-large"} {
		h := byID[id]
		if len(h.Rows) < 10 {
			t.Errorf("%s has %d rows, want at least 10 mixed-signal rows", id, len(h.Rows))
		}
		var first, last float64
		fitDays := map[float64]bool{}
		fitCount := 0
		for _, row := range h.Rows {
			ev := row.SeededEvidence(time.Now().UTC())
			if ev.CountsToward() != domain.BeliefKindFit {
				continue
			}
			if row.Signal != domain.SignalClassifiedOutcome {
				t.Errorf("%s fit row %q uses signal %q, want classified outcome", id, row.Note, row.Signal)
			}
			if fitCount == 0 {
				first = row.Day
			}
			last = row.Day
			fitDays[row.Day] = true
			fitCount++
		}
		if fitCount < 3 {
			t.Errorf("%s has %d fit rows, want at least 3 accumulated beliefs", id, fitCount)
		}
		if len(fitDays) != fitCount {
			t.Errorf("%s shares an event day across fit rows; each needs its own interaction", id)
		}
		if fitCount > 0 && (last-first)/h.SessionDays < 0.3 {
			t.Errorf("%s fit coverage = %.3f, want at least 0.3 over the session extent", id, (last-first)/h.SessionDays)
		}
	}
	for _, id := range []string{"cmp-brands-steady", "cmp-brands-split"} {
		h := byID[id]
		brands := map[string]bool{}
		fitDirByBrand := map[string]domain.FitDirection{}
		for _, row := range h.Rows {
			if row.Item.BrandID != "" {
				brands[row.Item.BrandID] = true
			}
			ev := row.SeededEvidence(time.Now().UTC())
			if ev.CountsToward() != domain.BeliefKindFit || row.Reason == nil {
				continue
			}
			dir := row.Reason.Category.Direction()
			if prev, seen := fitDirByBrand[row.Item.BrandID]; seen && prev != dir {
				t.Errorf("%s brand %s points both %q and %q: one cut, one direction",
					id, row.Item.BrandID, prev, dir)
			}
			fitDirByBrand[row.Item.BrandID] = dir
		}
		if len(brands) < 2 {
			t.Errorf("%s spans %d brands, want at least 2 distinct cuts", id, len(brands))
		}
		if len(fitDirByBrand) == 0 {
			t.Errorf("%s has no brand-keyed fit evidence to separate", id)
		}
	}
}

func TestHiddenAnswersStayOffTheArmPath(t *testing.T) {
	bundle := fixtureBundle(t)
	historyFiles, err := filepath.Glob(filepath.Join(historiesDir, "*.json"))
	if err != nil {
		t.Fatalf("list history fixtures: %v", err)
	}
	if len(historyFiles) != len(bundle.Histories) {
		t.Fatalf("history fixtures = %d, want %d", len(historyFiles), len(bundle.Histories))
	}
	var armText strings.Builder
	for _, path := range historyFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		armText.Write(raw)
	}
	arm := armText.String()
	if strings.Contains(arm, "right_size") || strings.Contains(arm, "RightSize") {
		t.Error("arm-facing histories contain an answer key")
	}
	for _, a := range bundle.Answers {
		if strings.Contains(arm, a.Why) {
			t.Errorf("arm-facing histories contain the hidden rationale for %s", a.ShopperID)
		}
	}
	raw, err := os.ReadFile(filepath.Join(hiddenDir, "answers.json"))
	if err != nil {
		t.Fatalf("read hidden answers: %v", err)
	}
	var stored []Answer
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("decode hidden answers: %v", err)
	}
	if len(stored) != len(bundle.Answers) {
		t.Fatalf("stored answers = %d, want %d", len(stored), len(bundle.Answers))
	}
	answered := map[string]bool{}
	for _, a := range stored {
		answered[a.ShopperID] = true
		if a.RightSize == "" || a.Why == "" {
			t.Errorf("answer for %s is incomplete", a.ShopperID)
		}
	}
	for _, h := range bundle.Histories {
		if strings.HasPrefix(h.ShopperID, "cmp-") && !answered[h.ShopperID] {
			t.Errorf("comparison shopper %s has no hidden answer", h.ShopperID)
		}
		if h.ShopperID == "demo-shopper" && answered[h.ShopperID] {
			t.Error("demo shopper needs no hidden answer; the demo story is public")
		}
	}
}
