package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/identity"
)

// The demo brand's oxford, named as the contract names a product line: the
// brand id, the category slug and the fit handle. The demo's new shopper asks
// about DemoOxfordSize, where the seeded pool is Earned in the small
// direction, so the agent sizes up from the shopper's usual size.
const (
	DemoOxfordBrand     = "brand-x-001"
	DemoOxfordCategory  = "shirts"
	DemoOxfordFitHandle = "regular"
	DemoOxfordSize      = "M"
)

// PopulationFileName is the population fixture's file name inside its
// directory. The fixture is one JSON document: the as-of anchor and every
// row.
const PopulationFileName = "population.json"

// The spans between one seeded purchase's events. A return settles days after
// capture, feedback soon after capture, an unexplained refund days after
// capture, and a kept purchase settles once its return window has closed.
const (
	populationReturnLead   = 14 * 24 * time.Hour
	populationFeedbackLead = 7 * 24 * time.Hour
	populationRefundLead   = 5 * 24 * time.Hour
	populationKeptLead     = 30 * 24 * time.Hour
)

// The population's product lines. Only the demo line is exported by name: the
// demo and its pool answer for that line, while the other lines exist to pin
// Building, Claimed and Out of true.
var (
	demoOxfordLine = domain.ProductLine{
		Brand:     DemoOxfordBrand,
		Category:  DemoOxfordCategory,
		FitHandle: DemoOxfordFitHandle,
	}
	slimLine       = domain.ProductLine{Brand: "brand-y-002", Category: "shirts", FitHandle: "slim"}
	dilutedLine    = domain.ProductLine{Brand: "brand-z-003", Category: "shirts", FitHandle: "regular"}
	belowFloorLine = domain.ProductLine{Brand: "brand-v-004", Category: "shirts", FitHandle: "relaxed"}
	reversedLine   = domain.ProductLine{Brand: "brand-w-005", Category: "shirts", FitHandle: "regular"}
)

// DemoOxfordLine returns the demo brand's oxford product line: the line
// product.md's demo step 0 asks about. Its DemoOxfordSize pool is Earned in
// the small direction.
func DemoOxfordLine() domain.ProductLine {
	return demoOxfordLine
}

// PopulationLines returns the product lines the seeded population covers, in
// the order it builds them: the demo brand's oxford, a slim line that stays
// Building, a line whose fits-well verdicts hold it at Building, a line below
// the Building floor, and one that is Out of true. product_fit answers for one
// line and size at a time and does not enumerate lines, so a caller listing
// them takes them from here.
func PopulationLines() []domain.ProductLine {
	return []domain.ProductLine{demoOxfordLine, slimLine, dilutedLine, belowFloorLine, reversedLine}
}

// Population is the seeded population of shoppers behind the pool, as fit
// rows. It is a truth table, not a random sample: the same as-of anchor always
// yields the same rows.
type Population struct {
	// AsOf is the moment the rows' event times are measured back from. It is
	// the moment the population is "as of", and the recent window is judged
	// against it.
	AsOf time.Time
	// Rows are the population's fit rows. Every row carries the seeded mark,
	// the salted per-line shopper hash and the transaction hash, and no raw
	// shopper or transaction id appears anywhere in it.
	Rows []domain.FitRow
}

// GeneratePopulation builds the seeded population of shoppers behind the
// pool, as fit rows written through the domain's FitRow shape. The rows carry
// identity.SeedSaltHex's per-line shopper hashes, the transaction hash as the
// row id, and the seeded mark on every row.
//
// Event times are measured back from asOf, so seeding against the moment of
// seeding keeps the recent window meaningful. The same asOf always yields
// byte-identical rows.
//
// The demo brand's oxford in DemoOxfordSize is Earned in the small direction;
// a slim line and a line diluted by fits-well verdicts stay Building, a line
// below the Building floor reads Claimed, and one line is Out of true because
// its recent window points the other way.
func GeneratePopulation(asOf time.Time) (Population, error) {
	keyer, err := identity.NewKeyer(identity.SeedSaltHex)
	if err != nil {
		return Population{}, fmt.Errorf("seed: population keyer: %w", err)
	}
	anchor := asOf.UTC()
	b := &populationBuilder{keyer: keyer, asOf: anchor}
	b.seedDemoOxford()
	b.seedSlim()
	b.seedDiluted()
	b.seedBelowFloor()
	b.seedReversed()
	if b.err != nil {
		return Population{}, b.err
	}
	return Population{AsOf: anchor, Rows: b.rows}, nil
}

// WritePopulation generates the population for asOf and writes it as one JSON
// file under dir. The file holds no shopper identity: only salted per-line
// shopper hashes and transaction hashes.
func WritePopulation(ctx context.Context, dir string, asOf time.Time) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("seed: write population: %w", err)
	}
	if dir == "" {
		return fmt.Errorf("seed: write population: %w", ErrInvalidDir)
	}
	population, err := GeneratePopulation(asOf)
	if err != nil {
		return err
	}
	encoded, err := marshalPopulation(population)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("seed: write population: %w", err)
	}
	path := filepath.Join(dir, PopulationFileName)
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("seed: write population: %w", err)
	}
	return nil
}

// marshalPopulation renders the population. The struct fields and the row
// order make the encoding deterministic, so the same population always yields
// the same bytes.
func marshalPopulation(population Population) ([]byte, error) {
	raw, err := json.MarshalIndent(population, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("seed: encode population: %w", err)
	}
	return append(raw, '\n'), nil
}

// populationBuilder appends one population's rows in a fixed order. Every row
// is one shopper's one purchase, so each row mints a fresh shopper id; the id
// never reaches the row, only its salted per-line hash does.
type populationBuilder struct {
	keyer    *identity.Keyer
	asOf     time.Time
	sequence int
	shoppers int
	rows     []domain.FitRow
	err      error
}

// add fills in the line, the size, the per-line shopper hash, the transaction
// hash and the seeded mark, then appends the row. The first error stops the
// build and is reported by GeneratePopulation.
func (b *populationBuilder) add(line domain.ProductLine, size string, row domain.FitRow) {
	if b.err != nil {
		return
	}
	b.shoppers++
	b.sequence++
	payerID := fmt.Sprintf("seedshopper-%s-%04d", line.Brand, b.shoppers)
	shopperHash, err := b.keyer.LineShopperHash(payerID, line)
	if err != nil {
		b.err = fmt.Errorf("seed: population shopper hash: %w", err)
		return
	}
	txnID := fmt.Sprintf("seedtxn-%04d", b.sequence)
	txnHash, err := identity.TxnHash(txnID)
	if err != nil {
		b.err = fmt.Errorf("seed: population transaction hash: %w", err)
		return
	}
	row.Line = line
	row.Size = size
	row.ShopperHash = shopperHash
	row.TxnHash = txnHash
	row.Seeded = true
	b.rows = append(b.rows, row)
}

// daysAgo returns the event time of something that happened daysAgo days
// before the population's as-of moment.
func (b *populationBuilder) daysAgo(daysAgo int) time.Time {
	return b.asOf.Add(-time.Duration(daysAgo) * 24 * time.Hour)
}

// returned appends a purchase returned with a classified reason. A fit reason
// points the row small or large; damage points nowhere and counts toward the
// seller, never toward sizing.
func (b *populationBuilder) returned(line domain.ProductLine, size string, daysAgo int, category domain.Category) {
	settledAt := b.daysAgo(daysAgo)
	b.add(line, size, domain.FitRow{
		State:       domain.FitStateReasonClassified,
		Reason:      category,
		Direction:   category.Direction(),
		Settled:     true,
		SettledKind: domain.FitSettledByReturnReason,
		CapturedAt:  settledAt.Add(-populationReturnLead),
		RefundedAt:  settledAt,
		SettledAt:   settledAt,
		EventAt:     settledAt,
	})
}

// feedback appends a purchase settled by explicit fit feedback tied to it. A
// fits-well verdict points nowhere but is fit evidence, so it dilutes a
// direction's share without counting for it.
func (b *populationBuilder) feedback(line domain.ProductLine, size string, daysAgo int, verdict domain.FitVerdict) {
	settledAt := b.daysAgo(daysAgo)
	b.add(line, size, domain.FitRow{
		State:       domain.FitStateFeedback,
		Reason:      domain.CategoryNone,
		Direction:   verdict.Direction(),
		Verdict:     verdict,
		Settled:     true,
		SettledKind: domain.FitSettledByFeedback,
		CapturedAt:  settledAt.Add(-populationFeedbackLead),
		SettledAt:   settledAt,
		EventAt:     settledAt,
	})
}

// kept appends a purchase kept past its return window with no feedback: a weak
// inferred signal that settles the purchase without naming a verdict.
func (b *populationBuilder) kept(line domain.ProductLine, size string, daysAgo int) {
	settledAt := b.daysAgo(daysAgo)
	b.add(line, size, domain.FitRow{
		State:       domain.FitStateKept,
		Reason:      domain.CategoryNone,
		Settled:     true,
		SettledKind: domain.FitSettledByKept,
		CapturedAt:  settledAt.Add(-populationKeptLead),
		SettledAt:   settledAt,
		EventAt:     settledAt,
	})
}

// captured appends a purchase still inside its return window. It settles
// nothing, so it counts toward nothing until a reason or the window decides.
func (b *populationBuilder) captured(line domain.ProductLine, size string, daysAgo int) {
	capturedAt := b.daysAgo(daysAgo)
	b.add(line, size, domain.FitRow{
		State:      domain.FitStateCaptured,
		Reason:     domain.CategoryNone,
		CapturedAt: capturedAt,
		EventAt:    capturedAt,
	})
}

// refunded appends a refund still missing its reason: a pending question that
// settles nothing.
func (b *populationBuilder) refunded(line domain.ProductLine, size string, daysAgo int) {
	refundedAt := b.daysAgo(daysAgo)
	b.add(line, size, domain.FitRow{
		State:      domain.FitStateRefunded,
		Reason:     domain.CategoryNone,
		CapturedAt: refundedAt.Add(-populationRefundLead),
		RefundedAt: refundedAt,
		EventAt:    refundedAt,
	})
}

// seedDemoOxford builds the demo brand's oxford. Size M is Earned small:
// twelve too-small returns against three too-large ones and four fits-well
// verdicts, with eight of the too-small returns inside the recent window so
// the recent window earns the same direction. A damaged return, two kept
// purchases, two captures still inside their window and an unexplained refund
// round out the line without moving the pooled belief. Size L is Building
// large. The line is the demo's own, so its M pool is the one step 0 cites.
func (b *populationBuilder) seedDemoOxford() {
	line, size := demoOxfordLine, DemoOxfordSize
	for _, daysAgo := range []int{10, 14, 18, 22, 26, 30, 35, 40} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooSmall)
	}
	for _, daysAgo := range []int{120, 140, 160, 180} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooSmall)
	}
	for _, daysAgo := range []int{130, 150, 170} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooLarge)
	}
	for _, daysAgo := range []int{55, 95, 110, 125} {
		b.feedback(line, size, daysAgo, domain.FitVerdictFitsWell)
	}
	for _, daysAgo := range []int{105, 200} {
		b.kept(line, size, daysAgo)
	}
	b.returned(line, size, 60, domain.CategoryDamagedOrDefective)
	b.captured(line, size, 3)
	b.captured(line, size, 4)
	b.refunded(line, size, 7)

	for _, daysAgo := range []int{100, 130, 160, 190} {
		b.returned(line, "L", daysAgo, domain.CategoryFitTooLarge)
	}
}

// seedSlim builds a young line that stays Building: six too-small returns, one
// too-large return and one fits-well verdict are enough to say which way it
// runs but not enough distinct shoppers to earn the belief.
func (b *populationBuilder) seedSlim() {
	line, size := slimLine, "M"
	for _, daysAgo := range []int{15, 30, 45, 60, 75, 200} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooSmall)
	}
	b.returned(line, size, 91, domain.CategoryFitTooLarge)
	b.feedback(line, size, 120, domain.FitVerdictFitsWell)
}

// seedDiluted builds a line with eight too-small shoppers that still stays
// Building: eight fits-well verdicts hold the direction's share under the
// Earned floor, so the line says Building even though the shoppers are there.
func (b *populationBuilder) seedDiluted() {
	line, size := dilutedLine, "M"
	for _, daysAgo := range []int{10, 20, 30, 40, 50, 60, 70, 80} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooSmall)
	}
	for _, daysAgo := range []int{5, 15, 25, 35, 45, 55, 65, 75} {
		b.feedback(line, size, daysAgo, domain.FitVerdictFitsWell)
	}
}

// seedBelowFloor builds a line under the Building floor: two shoppers, one
// each way, so the pool says nothing and reads Claimed.
func (b *populationBuilder) seedBelowFloor() {
	line, size := belowFloorLine, "M"
	b.returned(line, size, 40, domain.CategoryFitTooSmall)
	b.returned(line, size, 70, domain.CategoryFitTooLarge)
}

// seedReversed builds the line that is Out of true: ten too-small returns
// against four too-large ones earn the belief overall, while the recent window
// carries more too-large returns than too-small ones and earns neither.
func (b *populationBuilder) seedReversed() {
	line, size := reversedLine, "M"
	for _, daysAgo := range []int{120, 140, 160, 180, 200, 220, 240, 260} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooSmall)
	}
	for _, daysAgo := range []int{30, 60} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooSmall)
	}
	for _, daysAgo := range []int{5, 15, 45, 75} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooLarge)
	}
}
