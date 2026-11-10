package seed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// populationDir names the committed population fixture, relative to this
// package.
const populationDir = "../testdata/seed/population"

// populationAsOf is the fixed anchor the committed population fixture is
// generated from. The live seed passes the moment of seeding instead.
var populationAsOf = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// hashPattern is a salted ShopperHash or TxnHash: 64 lowercase hex characters.
var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func populationFor(t *testing.T) Population {
	t.Helper()
	population, err := GeneratePopulation(populationAsOf)
	if err != nil {
		t.Fatalf("generate population: %v", err)
	}
	return population
}

// populationCounts reads one line and size the way the pool reader will: only
// settled rows count, and only fit-classified ones enter PoolCounts. A zero
// since means all-time; a non-zero one keeps rows settled at or after it. This
// read lives in the test because the pool reader's aggregation belongs to its
// own task; the rule it feeds is domain.DecidePool.
func populationCounts(rows []domain.FitRow, line domain.ProductLine, size string, since time.Time) domain.PoolCounts {
	var counts domain.PoolCounts
	small := map[string]bool{}
	large := map[string]bool{}
	fitShoppers := map[string]bool{}
	for _, row := range rows {
		if row.Line != line || row.Size != size || !row.Settled {
			continue
		}
		if !since.IsZero() && row.SettledAt.Before(since) {
			continue
		}
		fit := row.Direction != domain.FitDirectionNone || row.Verdict == domain.FitVerdictFitsWell
		if !fit {
			continue
		}
		fitShoppers[row.ShopperHash] = true
		switch {
		case row.Verdict == domain.FitVerdictFitsWell:
			counts.FitsWell++
		case row.Direction == domain.FitDirectionSmall:
			counts.TooSmall++
			small[row.ShopperHash] = true
		case row.Direction == domain.FitDirectionLarge:
			counts.TooLarge++
			large[row.ShopperHash] = true
		}
	}
	counts.ShoppersTooSmall = len(small)
	counts.ShoppersTooLarge = len(large)
	counts.Shoppers = len(fitShoppers)
	return counts
}

// TestPopulationFixtureMatchesTheGenerator pins the committed fixture: the
// generator must reproduce it byte for byte. Set
// UPDATE_SEED_POPULATION_FIXTURES to rewrite it deliberately.
func TestPopulationFixtureMatchesTheGenerator(t *testing.T) {
	if os.Getenv("UPDATE_SEED_POPULATION_FIXTURES") != "" {
		if err := WritePopulation(context.Background(), populationDir, populationAsOf); err != nil {
			t.Fatalf("rewrite population fixture: %v", err)
		}
	}
	population := populationFor(t)
	want, err := marshalPopulation(population)
	if err != nil {
		t.Fatalf("encode population: %v", err)
	}
	path := filepath.Join(populationDir, PopulationFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v; rerun with UPDATE_SEED_POPULATION_FIXTURES=1", path, err)
	}
	if !bytes.Equal(raw, want) {
		t.Errorf("%s does not match GeneratePopulation; rerun with UPDATE_SEED_POPULATION_FIXTURES=1", path)
	}
}

// TestPopulationIsDeterministic pins that a second run is byte-identical and
// that the as-of anchor is the only input that moves the rows.
func TestPopulationIsDeterministic(t *testing.T) {
	first, err := marshalPopulation(populationFor(t))
	if err != nil {
		t.Fatalf("encode first population: %v", err)
	}
	second, err := marshalPopulation(populationFor(t))
	if err != nil {
		t.Fatalf("encode second population: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("two runs with the same anchor differ; the population is not deterministic")
	}

	shifted, err := GeneratePopulation(populationAsOf.Add(time.Hour))
	if err != nil {
		t.Fatalf("generate shifted population: %v", err)
	}
	shiftedBytes, err := marshalPopulation(shifted)
	if err != nil {
		t.Fatalf("encode shifted population: %v", err)
	}
	if bytes.Equal(first, shiftedBytes) {
		t.Error("a different anchor yields identical bytes; event times do not follow the anchor")
	}
}

// TestPopulationRowsCarryOnlyHashesAndTheSeededMark pins the row contract:
// every row is marked seeded, both identifiers are hashes, and no raw shopper
// or transaction id reaches the encoded population.
func TestPopulationRowsCarryOnlyHashesAndTheSeededMark(t *testing.T) {
	population := populationFor(t)
	if len(population.Rows) == 0 {
		t.Fatal("the population has no rows")
	}
	txnHashes := map[string]bool{}
	for i, row := range population.Rows {
		if !row.Seeded {
			t.Errorf("row %d: Seeded is false; every seeded row carries the mark", i)
		}
		if row.Line.Brand == "" || row.Line.Category == "" || row.Line.FitHandle == "" {
			t.Errorf("row %d: incomplete product line %+v", i, row.Line)
		}
		if row.Size == "" {
			t.Errorf("row %d: empty size", i)
		}
		if row.State.Rank() == 0 {
			t.Errorf("row %d: unknown purchase state %q", i, row.State)
		}
		if !hashPattern.MatchString(row.ShopperHash) {
			t.Errorf("row %d: ShopperHash %q is not a 64-character lowercase hex hash", i, row.ShopperHash)
		}
		if !hashPattern.MatchString(row.TxnHash) {
			t.Errorf("row %d: TxnHash %q is not a 64-character lowercase hex hash", i, row.TxnHash)
		}
		if row.ShopperHash == row.TxnHash {
			t.Errorf("row %d: the shopper hash equals the transaction hash", i)
		}
		if txnHashes[row.TxnHash] {
			t.Errorf("row %d: transaction hash repeats; one purchase is one row", i)
		}
		txnHashes[row.TxnHash] = true
	}
	encoded, err := json.Marshal(population)
	if err != nil {
		t.Fatalf("encode population: %v", err)
	}
	for _, raw := range []string{"seedshopper-", "seedtxn-"} {
		if bytes.Contains(encoded, []byte(raw)) {
			t.Errorf("encoded population contains the raw id prefix %q; only hashes may reach a row", raw)
		}
	}
}

// TestSettledRowsCarryTheirSettlingEventTime pins that a settled row's event
// time is the moment it settled. The index mapping T0.12 recorded carries no
// separate event time, so the pool's 90-day window reads settled_at: a row
// whose event time drifted from it would give the fixture's recent counts and
// the pool reader different windows.
func TestSettledRowsCarryTheirSettlingEventTime(t *testing.T) {
	for i, row := range populationFor(t).Rows {
		if !row.Settled {
			continue
		}
		if row.SettledAt.IsZero() {
			t.Errorf("row %d: settled with no SettledAt", i)
		}
		if !row.EventAt.Equal(row.SettledAt) {
			t.Errorf("row %d: event time %s is not the settling time %s", i, row.EventAt, row.SettledAt)
		}
	}
}

// TestPopulationPairsAreExactlyTheIntendedOnes pins that the population covers
// the intended line-and-size pairs and no others, so a changed status cannot
// hide behind a new pair.
func TestPopulationPairsAreExactlyTheIntendedOnes(t *testing.T) {
	type pair struct {
		line domain.ProductLine
		size string
	}
	want := []pair{
		{line: DemoOxfordLine(), size: DemoOxfordSize},
		{line: DemoOxfordLine(), size: "L"},
		{line: slimLine, size: "M"},
		{line: dilutedLine, size: "M"},
		{line: belowFloorLine, size: "M"},
		{line: reversedLine, size: "M"},
	}
	got := map[pair]int{}
	for _, row := range populationFor(t).Rows {
		got[pair{line: row.Line, size: row.Size}]++
	}
	if len(got) != len(want) {
		t.Errorf("population covers %d line-and-size pairs, want %d", len(got), len(want))
	}
	for _, p := range want {
		if got[p] == 0 {
			t.Errorf("population covers no rows for %+v size %q", p.line, p.size)
		}
	}
	for p := range got {
		known := false
		for _, w := range want {
			if p == w {
				known = true
			}
		}
		if !known {
			t.Errorf("population covers the unexpected pair %+v size %q", p.line, p.size)
		}
	}
}

// TestPopulationCountsArePinned pins the population's size and the row mix, so
// a change to the truth table has to update this test on purpose.
func TestPopulationCountsArePinned(t *testing.T) {
	population := populationFor(t)
	states := map[domain.FitPurchaseState]int{}
	settled := 0
	shoppers := map[string]bool{}
	for _, row := range population.Rows {
		states[row.State]++
		if row.Settled {
			settled++
		}
		shoppers[row.ShopperHash] = true
	}
	if len(population.Rows) != 69 {
		t.Errorf("rows = %d, want 69", len(population.Rows))
	}
	if settled != 66 {
		t.Errorf("settled rows = %d, want 66", settled)
	}
	if len(shoppers) != len(population.Rows) {
		t.Errorf("distinct shopper hashes = %d, want one per row (%d)", len(shoppers), len(population.Rows))
	}
	wantStates := map[domain.FitPurchaseState]int{
		domain.FitStateReasonClassified: 51,
		domain.FitStateFeedback:         13,
		domain.FitStateKept:             2,
		domain.FitStateCaptured:         2,
		domain.FitStateRefunded:         1,
	}
	for state, want := range wantStates {
		if states[state] != want {
			t.Errorf("state %q rows = %d, want %d", state, states[state], want)
		}
	}
	if len(states) != len(wantStates) {
		t.Errorf("population carries %d states, want %d", len(states), len(wantStates))
	}
}

// TestPoolStatusesOverThePopulation runs domain.DecidePool over the generated
// rows for every line and size: the demo oxford is Earned small, the slim and
// diluted lines stay Building, the thin line reads Claimed, and the reversed
// line is Out of true.
func TestPoolStatusesOverThePopulation(t *testing.T) {
	population := populationFor(t)
	recentSince := population.AsOf.Add(-domain.PoolRecentWindow)
	cases := []struct {
		name      string
		line      domain.ProductLine
		size      string
		all       domain.PoolCounts
		recent    domain.PoolCounts
		status    domain.BeliefStatus
		direction domain.FitDirection
	}{
		{
			name: "demo oxford M",
			line: DemoOxfordLine(), size: DemoOxfordSize,
			all:    domain.PoolCounts{TooSmall: 12, TooLarge: 3, FitsWell: 4, ShoppersTooSmall: 12, ShoppersTooLarge: 3, Shoppers: 19},
			recent: domain.PoolCounts{TooSmall: 8, FitsWell: 1, ShoppersTooSmall: 8, Shoppers: 9},
			status: domain.StatusEarned, direction: domain.FitDirectionSmall,
		},
		{
			name: "demo oxford L",
			line: DemoOxfordLine(), size: "L",
			all:    domain.PoolCounts{TooLarge: 4, ShoppersTooLarge: 4, Shoppers: 4},
			recent: domain.PoolCounts{},
			status: domain.StatusBuilding, direction: domain.FitDirectionLarge,
		},
		{
			name: "slim M",
			line: slimLine, size: "M",
			all:    domain.PoolCounts{TooSmall: 6, TooLarge: 1, FitsWell: 1, ShoppersTooSmall: 6, ShoppersTooLarge: 1, Shoppers: 8},
			recent: domain.PoolCounts{TooSmall: 5, ShoppersTooSmall: 5, Shoppers: 5},
			status: domain.StatusBuilding, direction: domain.FitDirectionSmall,
		},
		{
			name: "diluted M",
			line: dilutedLine, size: "M",
			all:    domain.PoolCounts{TooSmall: 8, FitsWell: 8, ShoppersTooSmall: 8, Shoppers: 16},
			recent: domain.PoolCounts{TooSmall: 8, FitsWell: 8, ShoppersTooSmall: 8, Shoppers: 16},
			status: domain.StatusBuilding, direction: domain.FitDirectionSmall,
		},
		{
			name: "below floor M",
			line: belowFloorLine, size: "M",
			all:    domain.PoolCounts{TooSmall: 1, TooLarge: 1, ShoppersTooSmall: 1, ShoppersTooLarge: 1, Shoppers: 2},
			recent: domain.PoolCounts{TooSmall: 1, TooLarge: 1, ShoppersTooSmall: 1, ShoppersTooLarge: 1, Shoppers: 2},
			status: domain.StatusClaimed, direction: domain.FitDirectionNone,
		},
		{
			name: "reversed M",
			line: reversedLine, size: "M",
			all:    domain.PoolCounts{TooSmall: 10, TooLarge: 4, ShoppersTooSmall: 10, ShoppersTooLarge: 4, Shoppers: 14},
			recent: domain.PoolCounts{TooSmall: 2, TooLarge: 4, ShoppersTooSmall: 2, ShoppersTooLarge: 4, Shoppers: 6},
			status: domain.StatusOutOfTrue, direction: domain.FitDirectionSmall,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			all := populationCounts(population.Rows, tc.line, tc.size, time.Time{})
			if all != tc.all {
				t.Errorf("all-time counts = %+v, want %+v", all, tc.all)
			}
			recent := populationCounts(population.Rows, tc.line, tc.size, recentSince)
			if recent != tc.recent {
				t.Errorf("recent counts = %+v, want %+v", recent, tc.recent)
			}
			verdict := domain.DecidePool(all, recent)
			if verdict.Status != tc.status || verdict.Direction != tc.direction {
				t.Errorf("DecidePool = {%s %q}, want {%s %q}", verdict.Status, verdict.Direction, tc.status, tc.direction)
			}
		})
	}
}

// TestDemoOxfordPoolIsEarnedOnDayOne pins product.md's demo step 0: a shopper
// new to the brand is sized up from the pool, because the demo line's size M
// is Earned small.
func TestDemoOxfordPoolIsEarnedOnDayOne(t *testing.T) {
	population := populationFor(t)
	all := populationCounts(population.Rows, DemoOxfordLine(), DemoOxfordSize, time.Time{})
	recent := populationCounts(population.Rows, DemoOxfordLine(), DemoOxfordSize, population.AsOf.Add(-domain.PoolRecentWindow))
	verdict := domain.DecidePool(all, recent)
	if verdict.Status != domain.StatusEarned {
		t.Fatalf("demo oxford size %s pool = %s, want %s", DemoOxfordSize, verdict.Status, domain.StatusEarned)
	}
	if verdict.Direction != domain.FitDirectionSmall {
		t.Fatalf("demo oxford size %s direction = %q, want %q so the agent sizes up", DemoOxfordSize, verdict.Direction, domain.FitDirectionSmall)
	}
	if all.ShoppersTooSmall < domain.PoolEarnedShoppers {
		t.Errorf("demo oxford distinct too-small shoppers = %d, want at least %d", all.ShoppersTooSmall, domain.PoolEarnedShoppers)
	}
}

// TestWritePopulation pins the writer: it writes the fixture bytes it encodes,
// refuses an empty directory, and refuses a cancelled context.
func TestWritePopulation(t *testing.T) {
	dir := t.TempDir()
	if err := WritePopulation(context.Background(), dir, populationAsOf); err != nil {
		t.Fatalf("write population: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, PopulationFileName))
	if err != nil {
		t.Fatalf("read written population: %v", err)
	}
	population := populationFor(t)
	want, err := marshalPopulation(population)
	if err != nil {
		t.Fatalf("encode population: %v", err)
	}
	if !bytes.Equal(raw, want) {
		t.Error("written population differs from the generator's encoding")
	}
	if err := WritePopulation(context.Background(), "", populationAsOf); !errors.Is(err, ErrInvalidDir) {
		t.Errorf("empty directory error = %v, want ErrInvalidDir", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := WritePopulation(ctx, dir, populationAsOf); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled context error = %v, want context.Canceled", err)
	}
}
