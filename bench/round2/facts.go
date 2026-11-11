package round2

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/seed"
)

// The frozen fixture layout, relative to this package: the arm-facing
// shoppers, the hidden answers apart from them, the population behind the
// pool, and the arm-2 PayPal records.
const (
	// ShoppersFile is the arm-facing shopper file.
	ShoppersFile = "shoppers/shoppers.json"
	// AnswersFile is the hidden answer key. No arm reads it.
	AnswersFile = "hidden/answers.json"
	// PopulationFile is the seeded population behind the pool.
	PopulationFile = "population/population.json"
	// PayPalDir holds one directory per shopper of toolkit-shaped records.
	PayPalDir = "paypal"
)

// Errors returned when the frozen fixtures cannot answer.
var (
	// ErrNoShopper means no frozen shopper carries the id.
	ErrNoShopper = errors.New("round2: no shopper")
	// ErrNoFixtures means a fixture file is missing or malformed.
	ErrNoFixtures = errors.New("round2: frozen fixture missing")
)

// Shoppers reads the frozen arm-facing shoppers. It reads only the file it
// is given: there is no path joining to the answer key.
func Shoppers(path string) ([]seed.Round2Shopper, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoFixtures, err)
	}
	var shoppers []seed.Round2Shopper
	if err := json.Unmarshal(raw, &shoppers); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoFixtures, err)
	}
	if len(shoppers) == 0 {
		return nil, fmt.Errorf("%w: %s holds no shoppers", ErrNoFixtures, path)
	}
	return shoppers, nil
}

// LoadShoppers reads the frozen shoppers from this package's fixture path.
func LoadShoppers() ([]seed.Round2Shopper, error) {
	return Shoppers(fixturePath(ShoppersFile))
}

// Shopper returns one frozen shopper by id.
func Shopper(shoppers []seed.Round2Shopper, shopperID string) (seed.Round2Shopper, error) {
	for _, shopper := range shoppers {
		if shopper.ShopperID == shopperID {
			return shopper, nil
		}
	}
	return seed.Round2Shopper{}, fmt.Errorf("%w: %q", ErrNoShopper, shopperID)
}

// Answers reads the hidden answer key. Only the grader and the fairness
// check call it; the arms never do.
func Answers(path string) (map[string]seed.Round2Answer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoFixtures, err)
	}
	var list []seed.Round2Answer
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoFixtures, err)
	}
	byShopper := make(map[string]seed.Round2Answer, len(list))
	for _, answer := range list {
		byShopper[answer.ShopperID] = answer
	}
	return byShopper, nil
}

// Population reads the frozen population.
func Population(path string) (seed.Population, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return seed.Population{}, fmt.Errorf("%w: %v", ErrNoFixtures, err)
	}
	var population seed.Population
	if err := json.Unmarshal(raw, &population); err != nil {
		return seed.Population{}, fmt.Errorf("%w: %v", ErrNoFixtures, err)
	}
	return population, nil
}

// UsualSize applies the layer's usual-size rule to one shopper's rows: the
// size most often kept or said to fit well in any brand, failing that the
// size their general statement names. It returns the size and the basis.
func UsualSize(shopper seed.Round2Shopper) (string, domain.UsualSizeBasis) {
	tallies := map[string]*domain.SizeTally{}
	order := []string{}
	tally := func(size string) *domain.SizeTally {
		if _, ok := tallies[size]; !ok {
			tallies[size] = &domain.SizeTally{Size: size}
			order = append(order, size)
		}
		return tallies[size]
	}
	stated := ""
	for _, row := range shopper.History.Rows {
		if row.Item.Size == "" {
			continue
		}
		switch {
		case row.Signal == domain.SignalGeneralStatement:
			if stated == "" {
				stated = row.Item.Size
			}
		case row.Signal == domain.SignalKeptPastWindow:
			tally(row.Item.Size).Kept++
		case row.Signal == domain.SignalFitFeedback:
			tally(row.Item.Size).FitsWell++
		}
	}
	list := make([]domain.SizeTally, 0, len(order))
	for _, size := range order {
		list = append(list, *tallies[size])
	}
	usual := domain.UsualSizeFor(list, stated)
	return usual.Size, usual.Basis
}

// RecallTarget is the recall_for_purchase request a Memento arm sends for
// one shopper. The pool read needs all four of the brand, category, fit
// handle and size: a request missing any of them answers "unavailable" and
// the pool step never decides, which would silently remove the case-1 and
// case-2 premise.
type RecallTarget struct {
	// ShopperID is the shopper making the purchase.
	ShopperID string `json:"shopper_id"`
	// BrandID is the candidate brand.
	BrandID string `json:"brand_id"`
	// Category is the product category slug.
	Category string `json:"category"`
	// FitHandle is the line's cut.
	FitHandle string `json:"fit_handle"`
	// SizeConsidered is the shopper's usual size.
	SizeConsidered string `json:"size_considered"`
}

// Target builds the recall request for one shopper.
func Target(shopper seed.Round2Shopper) RecallTarget {
	usual, _ := UsualSize(shopper)
	return RecallTarget{
		ShopperID:      shopper.ShopperID,
		BrandID:        shopper.Candidate.BrandID,
		Category:       shopper.Candidate.Category,
		FitHandle:      shopper.Candidate.FitHandle,
		SizeConsidered: usual,
	}
}

// JSON renders the request as the tool body.
func (t RecallTarget) JSON() []byte {
	encoded, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil
	}
	return encoded
}

// PoolFact is the pooled product fit for one shopper's question, reduced to
// what decides the size: the line and size queried, the label the rule
// gives, the direction it rates and the counts behind it.
type PoolFact struct {
	// Line is the product line the pool was read for.
	Line domain.ProductLine
	// Size is the size the pool was read at: the shopper's usual size.
	Size string
	// Status is the pooled label.
	Status domain.BeliefStatus
	// Direction is the direction the label rates, or none.
	Direction domain.FitDirection
	// Counts are the all-time settled counts.
	Counts domain.PoolCounts
}

// PoolFor builds the pool fact for one shopper from the frozen population.
// The reader queries the shopper's usual size, exactly as recall does, and
// the recent window is judged against the population's as-of moment.
func PoolFor(shopper seed.Round2Shopper, population seed.Population) (PoolFact, error) {
	usual, _ := UsualSize(shopper)
	if usual == "" {
		return PoolFact{Status: domain.StatusClaimed}, nil
	}
	line := shopper.Candidate.Line()
	all := seed.Round2PoolCounts(population.Rows, line, usual, time.Time{})
	recent := seed.Round2PoolCounts(population.Rows, line, usual, population.AsOf.Add(-domain.PoolRecentWindow))
	verdict := domain.DecidePool(all, recent)
	return PoolFact{Line: line, Size: usual, Status: verdict.Status, Direction: verdict.Direction, Counts: all}, nil
}

// FitReturn is one classified fit return from a shopper's own history, in
// event order.
type FitReturn struct {
	// Day is days since the session start.
	Day float64
	// Size is the returned item's size.
	Size string
	// Direction is the way the reason points.
	Direction domain.FitDirection
}

// OwnFitRows returns the shopper's classified fit returns for the
// candidate brand, in event order. A seller-classified row and a
// directionless answer are not fit evidence for sizing.
func OwnFitRows(shopper seed.Round2Shopper) []FitReturn {
	var out []FitReturn
	for _, row := range shopper.History.Rows {
		if row.Item.BrandID != shopper.Candidate.BrandID || row.Signal != domain.SignalClassifiedOutcome || row.Reason == nil {
			continue
		}
		if row.Reason.Category.CountsToward() != domain.BeliefKindFit {
			continue
		}
		out = append(out, FitReturn{Day: row.Day, Size: row.Item.Size, Direction: row.Reason.Category.Direction()})
	}
	return out
}

// FeedbackRows returns the shopper's fit-feedback rows for the candidate
// brand, in event order, with the worn size.
func FeedbackRows(shopper seed.Round2Shopper) []FitReturn {
	var out []FitReturn
	for _, row := range shopper.History.Rows {
		if row.Item.BrandID != shopper.Candidate.BrandID || row.Signal != domain.SignalFitFeedback {
			continue
		}
		out = append(out, FitReturn{Day: row.Day, Size: row.Item.Size, Direction: domain.FitDirectionNone})
	}
	return out
}

// StatementSize returns the size the shopper's general statement names, or
// empty when the history carries none.
func StatementSize(shopper seed.Round2Shopper) string {
	for _, row := range shopper.History.Rows {
		if row.Signal == domain.SignalGeneralStatement && row.Item.Size != "" {
			return row.Item.Size
		}
	}
	return ""
}

// PendingCount counts the refunds still missing a reason: what
// pending_questions would answer for this shopper.
func PendingCount(shopper seed.Round2Shopper) int {
	pending := 0
	for _, row := range shopper.History.Rows {
		if row.Outcome == nil || row.Reason != nil {
			continue
		}
		if row.Outcome.Kind == domain.OutcomeRefund || row.Outcome.Kind == domain.OutcomeDispute {
			pending++
		}
	}
	return pending
}

// LoadAnswers reads the frozen answer key from this package's fixture
// path.
func LoadAnswers() (map[string]seed.Round2Answer, error) {
	return Answers(fixturePath(AnswersFile))
}

// LoadPopulation reads the frozen population from this package's fixture
// path.
func LoadPopulation() (seed.Population, error) {
	return Population(fixturePath(PopulationFile))
}

// fixturePath resolves a frozen path against this package's directory. The
// source file's own directory is used rather than the working directory, so
// the loader answers the same from the package's tests and from a driver
// started at the repository root.
func fixturePath(relative string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.FromSlash(relative)
	}
	return filepath.Join(filepath.Dir(file), filepath.FromSlash(relative))
}
