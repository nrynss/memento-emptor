package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/identity"
)

// Round 2's frozen inputs, generated from one fixed seed. The round-1
// generator in seed.go is untouched: Generate still yields the round-1
// bundle byte for byte, and these builders are a separate stream with
// their own transaction ids.
//
// The four cases are product.md's ("The comparison"): new to the brand,
// unlike the crowd, noisy refunds, and correction then another agent.
// Each case has exactly two shoppers. Every shopper's hidden answer travels
// on the hidden path only, exactly as round 1's answers do.

// Round2Seed is the fixed seed every round-2 fixture is generated from.
// Regenerating with it reproduces the committed files byte for byte.
const Round2Seed uint64 = 41

// round2AsOf is the fixed event-time anchor the round-2 population's
// relative spans are measured back from. It is private so no caller can
// move it: live seeding passes the moment of seeding instead, exactly as
// round 1's does.
var round2AsOf = time.Date(2026, time.November, 11, 12, 0, 0, 0, time.UTC)

// Round2Anchor returns the fixed event-time anchor the committed round-2
// population is generated against. Regenerating against it reproduces the
// committed fixture byte for byte.
func Round2Anchor() time.Time { return round2AsOf }

// The round-2 case ids, as product.md names the four cases.
const (
	// Round2CaseNewToBrand is the shopper with no history of the brand.
	Round2CaseNewToBrand = "new-to-brand"
	// Round2CaseUnlikeCrowd is the shopper whose own evidence disagrees
	// with the pool and wins.
	Round2CaseUnlikeCrowd = "unlike-the-crowd"
	// Round2CaseNoisyRefunds is the mixed-cause history where a refund's
	// status cannot tell fit from the rest.
	Round2CaseNoisyRefunds = "noisy-refunds"
	// Round2CaseCorrection is the correction made to one agent and asked
	// about next by another.
	Round2CaseCorrection = "correction-then-another-agent"
)

// The round-2 brands. All are fictional and match nothing in
// testdata/channel3/. Every round-2 product line uses the regular fit
// handle: the served layer resolves an item to its line through
// seed.PopulationLines() and falls back to regular for a line the round-1
// population never seeded, so a line with another handle would pool
// under a line the frozen population does not name.
const (
	// Round2ValeBrand is the case-1 candidate brand, the line the pool
	// knows runs small.
	Round2ValeBrand = "brand-v-010"
	// Round2WrenBrand is the case-2 candidate brand, whose pool also runs
	// small while the shopper's own evidence runs large.
	Round2WrenBrand = "brand-w-011"
	// Round2UmberBrand is case 3's first candidate brand.
	Round2UmberBrand = "brand-u-012"
	// Round2NorthBrand is case 3's second candidate brand.
	Round2NorthBrand = "brand-n-015"
	// Round2OtterBrand is case 4's first candidate brand.
	Round2OtterBrand = "brand-o-014"
	// Round2SableBrand is case 4's second candidate brand.
	Round2SableBrand = "brand-s-013"
	// Round2TernBrand is another brand whose large cut must never leak
	// into case 1's candidate.
	Round2TernBrand = "brand-t-021"
	// Round2LoomBrand sells the tee the usual size is kept in; it is not
	// a candidate line.
	Round2LoomBrand = "brand-l-020"
)

// Round2 population line handles. Every round-2 line is regular, as the
// fallback resolver expects.
const round2FitHandle = "regular"

var (
	round2ValeLine  = domain.ProductLine{Brand: Round2ValeBrand, Category: "shirts", FitHandle: round2FitHandle}
	round2WrenLine  = domain.ProductLine{Brand: Round2WrenBrand, Category: "shirts", FitHandle: round2FitHandle}
	round2UmberLine = domain.ProductLine{Brand: Round2UmberBrand, Category: "shirts", FitHandle: round2FitHandle}
	round2NorthLine = domain.ProductLine{Brand: Round2NorthBrand, Category: "shirts", FitHandle: round2FitHandle}
	round2OtterLine = domain.ProductLine{Brand: Round2OtterBrand, Category: "shirts", FitHandle: round2FitHandle}
	round2SableLine = domain.ProductLine{Brand: Round2SableBrand, Category: "shirts", FitHandle: round2FitHandle}
	round2TernLine  = domain.ProductLine{Brand: Round2TernBrand, Category: "shirts", FitHandle: round2FitHandle}
)

// Round2PopulationLines returns the product lines the round-2 population
// seeds, in the order it builds them. A line absent from this list has no
// pool rows, so its pool reads Claimed and the pool step says nothing.
func Round2PopulationLines() []domain.ProductLine {
	return []domain.ProductLine{round2ValeLine, round2WrenLine}
}

// Round2Candidate names the line a shopper is asking about, and the size
// the question starts from: the shopper's usual size. It is a new SKU on
// the line with the size unchosen, so every arm recommends into the same
// open question, exactly as round 1's candidate did.
type Round2Candidate struct {
	// BrandID is the brand's id, as the contract keys a subject.
	BrandID string `json:"brand_id"`
	// BrandName is the brand's display name.
	BrandName string `json:"brand_name"`
	// LineName is the product line's display name.
	LineName string `json:"line_name"`
	// Category is the product category slug.
	Category string `json:"category"`
	// FitHandle is the line's cut. Every round-2 line is regular.
	FitHandle string `json:"fit_handle"`
	// UsualSize is the size the shopper usually takes, which recall
	// adjusts the pooled belief from.
	UsualSize string `json:"usual_size"`
}

// Line returns the candidate's product line.
func (c Round2Candidate) Line() domain.ProductLine {
	return domain.ProductLine{Brand: c.BrandID, Category: c.Category, FitHandle: c.FitHandle}
}

// Round2Correction is a shopper's explicit correction of what the memory
// says, in their own words, with the size it asks for.
type Round2Correction struct {
	// Text is what the shopper said, verbatim.
	Text string `json:"text"`
	// WantSize is the size the correction asks for.
	WantSize string `json:"want_size"`
}

// Round2Shopper is one generated round-2 shopper: the arm-facing history,
// the case it belongs to, and the line and usual size the question is
// asked from. It carries no answer: answers travel on the hidden path.
type Round2Shopper struct {
	// ShopperID names the shopper; sessions isolate shoppers.
	ShopperID string `json:"shopper_id"`
	// CaseID is which of the four cases this shopper serves.
	CaseID string `json:"case_id"`
	// Candidate is the line and usual size the shopper asks about.
	Candidate Round2Candidate `json:"candidate"`
	// Correction is the shopper's correction, or nil when the case has
	// none.
	Correction *Round2Correction `json:"correction"`
	// History is the shopper's seeded history, the only arm-facing rows.
	History History `json:"history"`
}

// Round2Answer is one shopper's hidden answer: the right size for the
// first recommendation, why, and, when the shopper corrects, the size
// right after the correction. It lives apart from the shoppers and no arm
// reads it.
type Round2Answer struct {
	// ShopperID names the shopper this answers.
	ShopperID string `json:"shopper_id"`
	// CaseID is the case the answer belongs to.
	CaseID string `json:"case_id"`
	// RightSize is the size the first recommendation grades against.
	RightSize string `json:"right_size"`
	// Why states why that size is right, and which tempting cause is not.
	Why string `json:"why"`
	// RightSizeAfterCorrection is the size right after the correction,
	// or empty when the shopper never corrects.
	RightSizeAfterCorrection string `json:"right_size_after_correction"`
}

// Round2Bundle is everything the round-2 generator produces from one seed.
type Round2Bundle struct {
	// Seed is the seed this bundle was generated from.
	Seed uint64
	// Shoppers are the arm-facing shoppers, in case order.
	Shoppers []Round2Shopper
	// Answers holds one hidden answer per shopper.
	Answers []Round2Answer
	// Population is the seeded population behind the pool, as fit rows.
	Population Population
}

// GenerateRound2 builds the round-2 bundle deterministically from seed. The
// same seed always yields byte-identical output; event-time spans are fixed
// constants, so the promotion gates hold for every seed.
func GenerateRound2(seed uint64) (Round2Bundle, error) {
	b := &builder{rng: rand.New(rand.NewSource(int64(seed))), seed: seed}
	shoppers := []Round2Shopper{
		b.newToBrandPlain(),
		b.newToBrandOtherBrand(),
		b.unlikeCrowdRival(),
		b.unlikeCrowdPlain(),
		b.noisyRefundUp(),
		b.noisyRefundDown(),
		b.correctionBelief(),
		b.correctionStanding(),
	}
	population, err := GenerateRound2Population(round2AsOf)
	if err != nil {
		return Round2Bundle{}, err
	}
	return Round2Bundle{
		Seed:       seed,
		Shoppers:   shoppers,
		Answers:    round2Answers(),
		Population: population,
	}, nil
}

// WriteRound2 generates the bundle for seed and writes the arm-facing
// shoppers to dir/shoppers, the hidden answers to dir/hidden and the
// population to dir/population. The directories are distinct, so no arm
// path can reach the answers.
func WriteRound2(ctx context.Context, dir string, seed uint64) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("seed: write round 2: %w", err)
	}
	if dir == "" {
		return fmt.Errorf("seed: write round 2: %w", ErrInvalidDir)
	}
	bundle, err := GenerateRound2(seed)
	if err != nil {
		return err
	}
	shoppers, answers, err := marshalRound2(bundle)
	if err != nil {
		return err
	}
	population, err := marshalPopulation(bundle.Population)
	if err != nil {
		return err
	}
	for _, sub := range []string{"shoppers", "hidden", "population"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return fmt.Errorf("seed: write round 2: %w", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "shoppers", "shoppers.json"), shoppers, 0o644); err != nil {
		return fmt.Errorf("seed: write round 2 shoppers: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hidden", "answers.json"), answers, 0o644); err != nil {
		return fmt.Errorf("seed: write round 2 answers: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "population", PopulationFileName), population, 0o644); err != nil {
		return fmt.Errorf("seed: write round 2 population: %w", err)
	}
	return nil
}

// marshalRound2 renders the shoppers and the answer key. Struct field order
// makes the encoding deterministic, so the same bundle always yields the
// same bytes.
func marshalRound2(bundle Round2Bundle) (shoppers, answers []byte, err error) {
	shoppers, err = json.MarshalIndent(bundle.Shoppers, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("seed: encode round 2 shoppers: %w", err)
	}
	answers, err = json.MarshalIndent(bundle.Answers, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("seed: encode round 2 answers: %w", err)
	}
	return append(shoppers, '\n'), append(answers, '\n'), nil
}

// GenerateRound2Population builds the pool the round-2 cases read, as fit
// rows. Two lines carry an earned belief; every other round-2 line has no
// rows, so its pool says nothing. The margins are wide enough that the
// shoppers' own fit rows — which the live layer pools as their histories
// are ingested — cannot flip either line: after the eight shoppers are in,
// Vale and Wren still both run small.
func GenerateRound2Population(asOf time.Time) (Population, error) {
	keyer, err := identity.NewKeyer(identity.SeedSaltHex)
	if err != nil {
		return Population{}, fmt.Errorf("seed: round 2 population keyer: %w", err)
	}
	anchor := asOf.UTC()
	b := &populationBuilder{keyer: keyer, asOf: anchor}
	b.seedRound2Vale()
	b.seedRound2Wren()
	if b.err != nil {
		return Population{}, b.err
	}
	return Population{AsOf: anchor, Rows: b.rows}, nil
}

// seedRound2Vale builds case 1's candidate line. Size M is Earned small
// with a wide margin: the round-2 shoppers' own rows on other lines never
// touch it.
func (b *populationBuilder) seedRound2Vale() {
	line, size := round2ValeLine, "M"
	for _, daysAgo := range []int{4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30,
		32, 34, 36, 38, 40, 42, 44, 46, 48, 50, 52, 54, 56, 58,
		62, 68, 74, 80, 86, 92,
		110, 130, 150, 180, 220, 260} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooSmall)
	}
	for _, daysAgo := range []int{25, 35, 45, 55, 70, 86, 120, 200} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooLarge)
	}
	for _, daysAgo := range []int{9, 19, 29, 39, 60, 80, 110, 140} {
		b.feedback(line, size, daysAgo, domain.FitVerdictFitsWell)
	}
	for _, daysAgo := range []int{15, 210} {
		b.kept(line, size, daysAgo)
	}
	b.returned(line, size, 65, domain.CategoryDamagedOrDefective)
	b.captured(line, size, 3)
	b.refunded(line, size, 6)
}

// seedRound2Wren builds case 2's candidate line. The crowd says size M runs
// small too, and stays saying it after the two case-2 shoppers' own
// too-large rows join the pool: that is the premise the case needs.
func (b *populationBuilder) seedRound2Wren() {
	line, size := round2WrenLine, "M"
	for _, daysAgo := range []int{4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30,
		32, 34, 36, 38, 40, 42, 44, 46, 48, 50, 52, 54, 56, 58,
		62, 68, 74, 80, 86, 92,
		110, 130, 150, 180, 220, 260} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooSmall)
	}
	for _, daysAgo := range []int{25, 35, 45, 55, 70, 86, 120, 200} {
		b.returned(line, size, daysAgo, domain.CategoryFitTooLarge)
	}
	for _, daysAgo := range []int{9, 19, 29, 39, 60, 80, 110, 140} {
		b.feedback(line, size, daysAgo, domain.FitVerdictFitsWell)
	}
	for _, daysAgo := range []int{15, 210} {
		b.kept(line, size, daysAgo)
	}
	b.returned(line, size, 68, domain.CategoryChangedMind)
	b.captured(line, size, 2)
}

// round2Answers is the hidden key, one answer per shopper, in shopper order.
func round2Answers() []Round2Answer {
	return []Round2Answer{
		{
			ShopperID: "r2-new-plain",
			CaseID:    Round2CaseNewToBrand,
			RightSize: "L",
			Why:       "The shopper has no history with the candidate brand, so their own evidence says nothing about it. The pool is Earned small at size M, so the pooled step adjusts one rung up from the usual M to L.",
		},
		{
			ShopperID: "r2-new-otherbrand",
			CaseID:    Round2CaseNewToBrand,
			RightSize: "L",
			Why:       "Another brand's large cut is that brand's, never the candidate's: cross-brand size mapping is out, so the own-evidence step has nothing to say about the candidate. The pool is Earned small at size M, so the right size is one rung up from the usual M, L. S is the temptation and it is wrong.",
		},
		{
			ShopperID: "r2-crowd-rival",
			CaseID:    Round2CaseUnlikeCrowd,
			RightSize: "S",
			Why:       "One early too-small return in M starts a runs-small belief. Three consecutive too-large returns then reach the rival-birth threshold, so the layer births the opposite-direction belief, and two more too-large returns back it; the newer, better-supported belief is the shopper's own and beats the pool's small direction. One rung down from M is S. Without the threshold the repeated opposites stay unattached, own evidence still says L, and the case would grade the mirror instead of the layer.",
		},
		{
			ShopperID: "r2-crowd-plain",
			CaseID:    Round2CaseUnlikeCrowd,
			RightSize: "S",
			Why:       "The first too-large return in M births a runs-large belief and two more back it, and the fits-well verdict at S is fit evidence for S. Own evidence beats the pool's small direction, so the right size is S. L is what naive pooling says, and it is wrong.",
		},
		{
			ShopperID: "r2-noisy-up",
			CaseID:    Round2CaseNoisyRefunds,
			RightSize: "L",
			Why:       "Three fit-classified too-small returns in M accumulate to size up to L. The damaged return and the lost parcel count toward the seller, the changed-mind and vague answers count toward nothing, the refund with no reason yet is a pending question, and the kept rows are weak inference; none of them moves the size.",
		},
		{
			ShopperID: "r2-noisy-down",
			CaseID:    Round2CaseNoisyRefunds,
			RightSize: "S",
			Why:       "Three fit-classified too-large returns in M accumulate to size down to S. The damaged and lost-parcel rows count toward the seller, the changed-mind and vague answers count toward nothing, the refund with no reason yet is pending, and the kept rows are weak inference; none of them moves the size.",
		},
		{
			ShopperID:                "r2-corr-belief",
			CaseID:                   Round2CaseCorrection,
			RightSize:                "L",
			RightSizeAfterCorrection: "M",
			Why:                      "Two fit-classified too-small returns in M earn the shopper's own runs-small belief, so the first recommendation is one rung up from M, L. The shopper's correction then takes precedence immediately: the size after it is M, while the belief stands declared against it, not deleted.",
		},
		{
			ShopperID:                "r2-corr-standing",
			CaseID:                   Round2CaseCorrection,
			RightSize:                "M",
			RightSizeAfterCorrection: "S",
			Why:                      "The only fit trace is a kept M, which is weak inference, so the first recommendation rests on the shopper's stated usual size, M, labelled Claimed. The correction stands on its own with no belief to contradict, and the size after it is S.",
		},
	}
}

// New round-2 builders. Each shopper's history carries its own hidden
// answer above; the builders know nothing of it.

func (b *builder) valeItem(size string) domain.Item {
	return domain.Item{BrandID: Round2ValeBrand, BrandName: "Brand Vale", LineName: "Vale Oxford Shirt", SKU: "VAL-OXF-" + size, Size: size, Category: "shirts"}
}

func (b *builder) wrenItem(size string) domain.Item {
	return domain.Item{BrandID: Round2WrenBrand, BrandName: "Brand Wren", LineName: "Wren Oxford Shirt", SKU: "WRN-OXF-" + size, Size: size, Category: "shirts"}
}

func (b *builder) ternItem(size string) domain.Item {
	return domain.Item{BrandID: Round2TernBrand, BrandName: "Brand Tern", LineName: "Tern Oxford Shirt", SKU: "TRN-OXF-" + size, Size: size, Category: "shirts"}
}

func (b *builder) loomItem(size string) domain.Item {
	return domain.Item{BrandID: Round2LoomBrand, BrandName: "Loom", LineName: "Loom Tee", SKU: "LOM-TEE-" + size, Size: size, Category: "t-shirts"}
}

// newToBrandPlain is case 1's first shopper: nothing about the candidate
// brand at all, and a usual size kept in another brand.
func (b *builder) newToBrandPlain() Round2Shopper {
	const shopper = "r2-new-plain"
	return Round2Shopper{
		ShopperID: shopper,
		CaseID:    Round2CaseNewToBrand,
		Candidate: Round2Candidate{
			BrandID: Round2ValeBrand, BrandName: "Brand Vale", LineName: "Vale Oxford Shirt",
			Category: "shirts", FitHandle: round2FitHandle, UsualSize: "M",
		},
		History: History{
			ShopperID:   shopper,
			SessionDays: 5,
			Rows: []Row{
				b.statement(shopper, 0, b.loomItem("M"), "Shopper says they are usually size M"),
				b.kept(shopper, 2, b.loomItem("M"), 2600, "Kept past the window: Loom tee size M, no feedback"),
			},
		},
	}
}

// newToBrandOtherBrand is case 1's second shopper: no candidate-brand
// history, and another brand's large cut sitting there to tempt a size
// down that the candidate never earned.
func (b *builder) newToBrandOtherBrand() Round2Shopper {
	const shopper = "r2-new-otherbrand"
	return Round2Shopper{
		ShopperID: shopper,
		CaseID:    Round2CaseNewToBrand,
		Candidate: Round2Candidate{
			BrandID: Round2ValeBrand, BrandName: "Brand Vale", LineName: "Vale Oxford Shirt",
			Category: "shirts", FitHandle: round2FitHandle, UsualSize: "M",
		},
		History: History{
			ShopperID:   shopper,
			SessionDays: 9,
			Rows: []Row{
				b.statement(shopper, 0, b.ternItem("M"), "Shopper says they are usually size M"),
				b.refund(shopper, 3, b.ternItem("M"), 4400, "swims on the shopper", domain.CategoryFitTooLarge, 1, "Fit return 1: Brand Tern oxford size M refunded; swims on the shopper"),
				b.refund(shopper, 6, b.ternItem("M"), 4400, "sleeves too long in M", domain.CategoryFitTooLarge, 1, "Fit return 2: Brand Tern oxford size M refunded; sleeves too long in M"),
				b.kept(shopper, 8, b.loomItem("M"), 2600, "Kept past the window: Loom tee size M, no feedback"),
			},
		},
	}
}

// unlikeCrowdRival is case 2's first shopper: one early too-small return
// starts a runs-small belief, then three consecutive too-large returns
// reach domain.RivalBirthThreshold and birth the opposite belief. The
// answer depends on that rule, so a driver that mirrors only the direction
// rule grades the mirror, not the layer.
func (b *builder) unlikeCrowdRival() Round2Shopper {
	const shopper = "r2-crowd-rival"
	return Round2Shopper{
		ShopperID: shopper,
		CaseID:    Round2CaseUnlikeCrowd,
		Candidate: Round2Candidate{
			BrandID: Round2WrenBrand, BrandName: "Brand Wren", LineName: "Wren Oxford Shirt",
			Category: "shirts", FitHandle: round2FitHandle, UsualSize: "M",
		},
		History: History{
			ShopperID:   shopper,
			SessionDays: 18,
			Rows: []Row{
				b.statement(shopper, 0, b.wrenItem("M"), "Shopper says they are usually size M"),
				b.refund(shopper, 2, b.wrenItem("M"), 4600, "too tight in the shoulders", domain.CategoryFitTooSmall, 1, "Fit return 1: Brand Wren oxford size M refunded; too tight in the shoulders"),
				b.refund(shopper, 4, b.wrenItem("M"), 4600, "changed my mind about the colour", domain.CategoryChangedMind, 1, "Refund: Brand Wren oxford size M; shopper changed their mind about the colour"),
				b.refund(shopper, 5, b.wrenItem("M"), 4600, "too loose across the chest", domain.CategoryFitTooLarge, 1, "Fit return 1: Brand Wren oxford size M refunded; too loose across the chest"),
				b.refund(shopper, 7, b.wrenItem("M"), 4600, "arrived with a torn seam", domain.CategoryDamagedOrDefective, 1, "Refund: Brand Wren oxford size M; arrived with a torn seam"),
				b.refund(shopper, 8, b.wrenItem("M"), 4600, "swims on the shopper", domain.CategoryFitTooLarge, 1, "Fit return 2: Brand Wren oxford size M refunded; swims on the shopper"),
				b.refund(shopper, 10, b.wrenItem("M"), 4600, "sleeves too long in M", domain.CategoryFitTooLarge, 1, "Fit return 3: Brand Wren oxford size M refunded; sleeves too long in M"),
				b.dispute(shopper, 12, b.wrenItem("M"), 4600, domain.DisputeMerchandiseNotReceived, domain.CategoryNotArrived, "Dispute opened: Brand Wren parcel never arrived; carrier lost it"),
				b.refund(shopper, 13, b.wrenItem("M"), 4600, "buttons arrived cracked", domain.CategoryDamagedOrDefective, 1, "Refund: Brand Wren oxford size M; buttons arrived cracked"),
				b.refund(shopper, 14, b.wrenItem("M"), 4600, "too loose in the body", domain.CategoryFitTooLarge, 1, "Fit return 4: Brand Wren oxford size M refunded; too loose in the body"),
				b.refund(shopper, 16, b.wrenItem("M"), 4600, "sleeves too long again", domain.CategoryFitTooLarge, 1, "Fit return 5: Brand Wren oxford size M refunded; sleeves too long again"),
				b.kept(shopper, 18, b.loomItem("M"), 2600, "Kept past the window: Loom tee size M, no feedback"),
			},
		},
	}
}

// unlikeCrowdPlain is case 2's second shopper: the simplest form of the
// same disagreement. Three too-large returns in M with the first birthing
// the belief, plus a fits-well verdict at S, against a pool that says
// small.
func (b *builder) unlikeCrowdPlain() Round2Shopper {
	const shopper = "r2-crowd-plain"
	return Round2Shopper{
		ShopperID: shopper,
		CaseID:    Round2CaseUnlikeCrowd,
		Candidate: Round2Candidate{
			BrandID: Round2WrenBrand, BrandName: "Brand Wren", LineName: "Wren Oxford Shirt",
			Category: "shirts", FitHandle: round2FitHandle, UsualSize: "M",
		},
		History: History{
			ShopperID:   shopper,
			SessionDays: 13,
			Rows: []Row{
				b.statement(shopper, 0, b.wrenItem("M"), "Shopper says they are usually size M"),
				b.refund(shopper, 2, b.wrenItem("M"), 4600, "too loose across the chest", domain.CategoryFitTooLarge, 1, "Fit return 1: Brand Wren oxford size M refunded; too loose across the chest"),
				b.refund(shopper, 4, b.wrenItem("M"), 4600, "changed my mind about the colour", domain.CategoryChangedMind, 1, "Refund: Brand Wren oxford size M; shopper changed their mind about the colour"),
				b.refund(shopper, 6, b.wrenItem("M"), 4600, "swims on the shopper", domain.CategoryFitTooLarge, 1, "Fit return 2: Brand Wren oxford size M refunded; swims on the shopper"),
				b.refund(shopper, 8, b.wrenItem("M"), 4600, "buttons arrived cracked", domain.CategoryDamagedOrDefective, 1, "Refund: Brand Wren oxford size M; buttons arrived cracked"),
				b.refund(shopper, 10, b.wrenItem("M"), 4600, "sleeves too long in M", domain.CategoryFitTooLarge, 1, "Fit return 3: Brand Wren oxford size M refunded; sleeves too long in M"),
				b.kept(shopper, 11, b.wrenItem("M"), 4600, "Kept past the window: Brand Wren oxford size M, no feedback"),
				b.feedback(shopper, 13, b.wrenItem("S"), 4600, "Fit feedback: Brand Wren oxford size S fits well, tied to this purchase"),
			},
		},
	}
}

// noisyRefundUp is case 3's first shopper: three too-small fit returns in
// M buried in four other causes and an unexplained refund. A refund status
// without its reason cannot say which rows are which; the reasons can.
func (b *builder) noisyRefundUp() Round2Shopper {
	const shopper = "r2-noisy-up"
	item := func(size string) domain.Item {
		return domain.Item{BrandID: Round2UmberBrand, BrandName: "Brand Umber", LineName: "Umber Oxford Shirt", SKU: "UMB-OXF-" + size, Size: size, Category: "shirts"}
	}
	return Round2Shopper{
		ShopperID: shopper,
		CaseID:    Round2CaseNoisyRefunds,
		Candidate: Round2Candidate{
			BrandID: Round2UmberBrand, BrandName: "Brand Umber", LineName: "Umber Oxford Shirt",
			Category: "shirts", FitHandle: round2FitHandle, UsualSize: "M",
		},
		History: History{
			ShopperID:   shopper,
			SessionDays: 18,
			Rows: []Row{
				b.statement(shopper, 0, item("M"), "Shopper says they are usually size M"),
				b.kept(shopper, 1, b.loomItem("M"), 2600, "Kept past the window: Loom tee size M, no feedback"),
				b.refund(shopper, 2, item("M"), 5000, "too tight in the shoulders", domain.CategoryFitTooSmall, 1, "Fit return 1: Brand Umber oxford size M refunded; too tight in the shoulders"),
				b.kept(shopper, 3, domain.Item{BrandID: Round2LoomBrand, BrandName: "Loom", LineName: "Loom Beanie", SKU: "LOM-BEA-OS", Size: "OS", Category: "hats"}, 2800, "Kept past the window: Loom beanie, no feedback"),
				b.refund(shopper, 4, item("M"), 5000, "arrived with a torn seam", domain.CategoryDamagedOrDefective, 1, "Refund: Brand Umber oxford size M; arrived with a torn seam"),
				b.dispute(shopper, 6, item("M"), 5000, domain.DisputeMerchandiseNotReceived, domain.CategoryNotArrived, "Dispute opened: Brand Umber parcel never arrived; tracking stuck for weeks"),
				b.refund(shopper, 7, item("M"), 5000, "changed my mind about the colour", domain.CategoryChangedMind, 1, "Refund: Brand Umber oxford size M; shopper changed their mind about the colour"),
				b.refund(shopper, 9, item("M"), 5000, "too tight across the chest", domain.CategoryFitTooSmall, 1, "Fit return 2: Brand Umber oxford size M refunded; too tight across the chest"),
				b.refund(shopper, 11, item("M"), 5000, "not great", domain.CategoryUnclear, 0.94, "Refund: Brand Umber oxford size M; shopper answer too vague to classify, ask again"),
				b.pendingRefund(shopper, 13, item("M"), 5000, "Refund: Brand Umber oxford size M; no reason yet, the agent has not asked"),
				b.refund(shopper, 15, item("M"), 5000, "sleeves too short in M", domain.CategoryFitTooSmall, 1, "Fit return 3: Brand Umber oxford size M refunded; sleeves too short in M"),
			},
		},
	}
}

// noisyRefundDown mirrors case 3 the other way: three too-large fit
// returns accumulate to S, so a driver that only ever sizes up cannot pass
// the case.
func (b *builder) noisyRefundDown() Round2Shopper {
	const shopper = "r2-noisy-down"
	item := func(size string) domain.Item {
		return domain.Item{BrandID: Round2NorthBrand, BrandName: "Brand North", LineName: "Northwind Oxford Shirt", SKU: "NOR-OXF-" + size, Size: size, Category: "shirts"}
	}
	return Round2Shopper{
		ShopperID: shopper,
		CaseID:    Round2CaseNoisyRefunds,
		Candidate: Round2Candidate{
			BrandID: Round2NorthBrand, BrandName: "Brand North", LineName: "Northwind Oxford Shirt",
			Category: "shirts", FitHandle: round2FitHandle, UsualSize: "M",
		},
		History: History{
			ShopperID:   shopper,
			SessionDays: 19,
			Rows: []Row{
				b.statement(shopper, 0, item("M"), "Shopper says they are usually size M"),
				b.kept(shopper, 1, b.loomItem("M"), 2500, "Kept past the window: Loom tee size M, no feedback"),
				b.kept(shopper, 3, domain.Item{BrandID: Round2LoomBrand, BrandName: "Loom", LineName: "Loom Beanie", SKU: "LOM-BEA-OS", Size: "OS", Category: "hats"}, 2800, "Kept past the window: Loom beanie, no feedback"),
				b.refund(shopper, 4, item("M"), 5200, "swims on the shopper", domain.CategoryFitTooLarge, 1, "Fit return 1: Brand North oxford size M refunded; swims on the shopper"),
				b.refund(shopper, 6, item("M"), 5200, "arrived with a loose hem", domain.CategoryDamagedOrDefective, 1, "Refund: Brand North oxford size M; arrived with a loose hem"),
				b.dispute(shopper, 8, item("M"), 5200, domain.DisputeMerchandiseNotReceived, domain.CategoryNotArrived, "Dispute opened: Brand North parcel never arrived; carrier lost it"),
				b.refund(shopper, 10, item("M"), 5200, "too loose across the chest", domain.CategoryFitTooLarge, 1, "Fit return 2: Brand North oxford size M refunded; too loose across the chest"),
				b.refund(shopper, 12, item("M"), 5200, "ordered two colours, kept the other", domain.CategoryChangedMind, 1, "Refund: Brand North oxford size M; ordered two colours, kept the other"),
				b.refund(shopper, 14, item("M"), 5200, "did not love it", domain.CategoryUnclear, 0.94, "Refund: Brand North oxford size M; shopper answer too vague to classify, ask again"),
				b.pendingRefund(shopper, 16, item("M"), 5200, "Refund: Brand North oxford size M; no reason yet, the agent has not asked"),
				b.refund(shopper, 17, item("M"), 5200, "sleeves too long in M", domain.CategoryFitTooLarge, 1, "Fit return 3: Brand North oxford size M refunded; sleeves too long in M"),
			},
		},
	}
}

// correctionBelief is case 4's first shopper: own evidence earns a
// runs-small belief, and the shopper then corrects the size it implies.
func (b *builder) correctionBelief() Round2Shopper {
	const shopper = "r2-corr-belief"
	item := func(size string) domain.Item {
		return domain.Item{BrandID: Round2OtterBrand, BrandName: "Brand Otter", LineName: "Otter Oxford Shirt", SKU: "OTT-OXF-" + size, Size: size, Category: "shirts"}
	}
	return Round2Shopper{
		ShopperID: shopper,
		CaseID:    Round2CaseCorrection,
		Candidate: Round2Candidate{
			BrandID: Round2OtterBrand, BrandName: "Brand Otter", LineName: "Otter Oxford Shirt",
			Category: "shirts", FitHandle: round2FitHandle, UsualSize: "M",
		},
		Correction: &Round2Correction{
			Text:     "Actually I have lost a size since then, use M now.",
			WantSize: "M",
		},
		History: History{
			ShopperID:   shopper,
			SessionDays: 8,
			Rows: []Row{
				b.statement(shopper, 0, item("M"), "Shopper says they are usually size M"),
				b.refund(shopper, 2, item("M"), 4700, "too tight in the shoulders", domain.CategoryFitTooSmall, 1, "Fit return 1: Brand Otter oxford size M refunded; too tight in the shoulders"),
				b.refund(shopper, 5, item("M"), 4700, "sleeves too short in M", domain.CategoryFitTooSmall, 1, "Fit return 2: Brand Otter oxford size M refunded; sleeves too short in M"),
				b.refund(shopper, 6, item("M"), 4700, "arrived with a torn seam", domain.CategoryDamagedOrDefective, 1, "Refund: Brand Otter oxford size M; arrived with a torn seam"),
				b.refund(shopper, 7, item("M"), 4700, "too tight across the chest", domain.CategoryFitTooSmall, 1, "Fit return 3: Brand Otter oxford size M refunded; too tight across the chest"),
				b.kept(shopper, 8, b.loomItem("M"), 2600, "Kept past the window: Loom tee size M, no feedback"),
			},
		},
	}
}

// correctionStanding is case 4's second shopper: no fit belief to
// contradict, so the correction stands on its own.
func (b *builder) correctionStanding() Round2Shopper {
	const shopper = "r2-corr-standing"
	item := func(size string) domain.Item {
		return domain.Item{BrandID: Round2SableBrand, BrandName: "Brand Sable", LineName: "Sable Oxford Shirt", SKU: "SAB-OXF-" + size, Size: size, Category: "shirts"}
	}
	return Round2Shopper{
		ShopperID: shopper,
		CaseID:    Round2CaseCorrection,
		Candidate: Round2Candidate{
			BrandID: Round2SableBrand, BrandName: "Brand Sable", LineName: "Sable Oxford Shirt",
			Category: "shirts", FitHandle: round2FitHandle, UsualSize: "M",
		},
		Correction: &Round2Correction{
			Text:     "I was measured last week, take S now.",
			WantSize: "S",
		},
		History: History{
			ShopperID:   shopper,
			SessionDays: 6,
			Rows: []Row{
				b.statement(shopper, 0, item("M"), "Shopper says they are usually size M"),
				b.kept(shopper, 2, item("M"), 4300, "Kept past the window: Brand Sable oxford size M, no feedback"),
				b.dispute(shopper, 4, item("M"), 4300, domain.DisputeMerchandiseNotReceived, domain.CategoryNotArrived, "Dispute opened: Brand Sable parcel never arrived; tracking stuck for weeks"),
			},
		},
	}
}

// Round2PoolCounts aggregates one line and size the way the pool reader
// does: settled rows only, a direction from a return's reason or a
// feedback verdict, distinct shoppers counted by their per-line hash. A
// zero since means all event time; any other since keeps the rows at or
// after it, which is how the recent window is judged.
func Round2PoolCounts(rows []domain.FitRow, line domain.ProductLine, size string, since time.Time) domain.PoolCounts {
	counts := domain.PoolCounts{}
	small, large, shoppers := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, row := range rows {
		if row.Line != line || row.Size != size || !row.Settled {
			continue
		}
		if !since.IsZero() && row.EventAt.Before(since) {
			continue
		}
		switch {
		case row.Direction == domain.FitDirectionSmall:
			counts.TooSmall++
			small[row.ShopperHash] = true
		case row.Direction == domain.FitDirectionLarge:
			counts.TooLarge++
			large[row.ShopperHash] = true
		case row.Verdict == domain.FitVerdictFitsWell:
			counts.FitsWell++
		default:
			continue
		}
		shoppers[row.ShopperHash] = true
	}
	counts.ShoppersTooSmall = len(small)
	counts.ShoppersTooLarge = len(large)
	counts.Shoppers = len(shoppers)
	return counts
}

// pendingRefund records a refund still missing its reason: an outcome that
// counts toward nothing until the shopper says why. It is what a reasonless
// refund looks like, and arm 2 sees it exactly as it sees a reasoned one.
func (b *builder) pendingRefund(shopper string, day float64, item domain.Item, amountMinor int64, note string) Row {
	return Row{
		Day:    day,
		Signal: domain.SignalClassifiedOutcome,
		Source: domain.SourcePayment,
		Outcome: &domain.Outcome{
			Kind:        domain.OutcomeRefund,
			Status:      "COMPLETED",
			AmountMinor: amountMinor,
			Currency:    "USD",
		},
		Item:     item,
		Reason:   nil,
		Link:     b.link(shopper, domain.OutcomeRefund),
		Note:     note,
		SeedMark: domain.SeedMark,
	}
}
