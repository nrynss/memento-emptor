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

// round2Dir is the committed fixture directory relative to this package.
const round2Dir = "../bench/round2"

// updateRound2Fixtures rewrites the committed round-2 fixtures when
// UPDATE_ROUND2_FIXTURES is set. The gate runs without it and compares.
func updateRound2Fixtures() bool {
	return os.Getenv("UPDATE_ROUND2_FIXTURES") != ""
}

func round2Bundle(t *testing.T) Round2Bundle {
	t.Helper()
	bundle, err := GenerateRound2(Round2Seed)
	if err != nil {
		t.Fatalf("generate round 2: %v", err)
	}
	return bundle
}

func TestRound2FixturesMatchTheGenerator(t *testing.T) {
	bundle := round2Bundle(t)
	shoppers, answers, err := marshalRound2(bundle)
	if err != nil {
		t.Fatalf("encode round 2: %v", err)
	}
	population, err := marshalPopulation(bundle.Population)
	if err != nil {
		t.Fatalf("encode round 2 population: %v", err)
	}
	if updateRound2Fixtures() {
		if err := WriteRound2(context.Background(), round2Dir, Round2Seed); err != nil {
			t.Fatalf("rewrite round 2 fixtures: %v", err)
		}
	}
	for path, want := range map[string][]byte{
		filepath.Join(round2Dir, "shoppers", "shoppers.json"):      shoppers,
		filepath.Join(round2Dir, "hidden", "answers.json"):         answers,
		filepath.Join(round2Dir, "population", PopulationFileName): population,
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !bytes.Equal(raw, want) {
			t.Errorf("fixture %s does not match GenerateRound2(%d); rerun with UPDATE_ROUND2_FIXTURES=1", path, Round2Seed)
		}
	}
}

func TestRound2GenerateIsDeterministic(t *testing.T) {
	first, second := round2Bundle(t), round2Bundle(t)
	a, b, err := marshalRound2(first)
	if err != nil {
		t.Fatalf("encode first: %v", err)
	}
	other, otherAnswers, err := marshalRound2(second)
	if err != nil {
		t.Fatalf("encode second: %v", err)
	}
	if !bytes.Equal(a, other) || !bytes.Equal(b, otherAnswers) {
		t.Error("two generations with the same seed differ")
	}
	different, err := GenerateRound2(Round2Seed + 1)
	if err != nil {
		t.Fatalf("generate a different seed: %v", err)
	}
	c, _, err := marshalRound2(different)
	if err != nil {
		t.Fatalf("encode a different seed: %v", err)
	}
	if bytes.Equal(a, c) {
		t.Error("a different seed yields identical shoppers; the seed changes nothing")
	}
}

// TestRound2CasesAreTwoShoppersEach pins the scale decision recorded in
// the freeze: exactly two shoppers for each of the four cases, eight in
// all, each with its own hidden answer.
func TestRound2CasesAreTwoShoppersEach(t *testing.T) {
	bundle := round2Bundle(t)
	cases := map[string]int{}
	answered := map[string]Round2Answer{}
	for _, answer := range bundle.Answers {
		if _, seen := answered[answer.ShopperID]; seen {
			t.Errorf("two hidden answers for %s", answer.ShopperID)
		}
		answered[answer.ShopperID] = answer
	}
	for _, shopper := range bundle.Shoppers {
		cases[shopper.CaseID]++
		if shopper.History.ShopperID != shopper.ShopperID {
			t.Errorf("%s carries a history for %q", shopper.ShopperID, shopper.History.ShopperID)
		}
		answer, ok := answered[shopper.ShopperID]
		if !ok {
			t.Errorf("%s has no hidden answer", shopper.ShopperID)
			continue
		}
		if answer.CaseID != shopper.CaseID {
			t.Errorf("%s answer names case %q, shopper names %q", shopper.ShopperID, answer.CaseID, shopper.CaseID)
		}
		if answer.RightSize == "" || answer.Why == "" {
			t.Errorf("%s answer is incomplete", shopper.ShopperID)
		}
		if (shopper.Correction == nil) != (answer.RightSizeAfterCorrection == "") {
			t.Errorf("%s correction and its answer disagree", shopper.ShopperID)
		}
		if shopper.Correction != nil && shopper.Correction.WantSize != answer.RightSizeAfterCorrection {
			t.Errorf("%s correction asks for %q, answer after it is %q",
				shopper.ShopperID, shopper.Correction.WantSize, answer.RightSizeAfterCorrection)
		}
		if shopper.CaseID != Round2CaseCorrection && shopper.Correction != nil {
			t.Errorf("%s carries a correction outside case 4", shopper.ShopperID)
		}
	}
	for _, id := range []string{Round2CaseNewToBrand, Round2CaseUnlikeCrowd, Round2CaseNoisyRefunds, Round2CaseCorrection} {
		if cases[id] != 2 {
			t.Errorf("case %s has %d shoppers, want exactly 2", id, cases[id])
		}
	}
	if len(bundle.Answers) != len(bundle.Shoppers) {
		t.Errorf("%d answers for %d shoppers", len(bundle.Answers), len(bundle.Shoppers))
	}
}

// TestRound2EveryRowCarriesTheSeedMark pins product.md's seeded rule on
// every round-2 row.
func TestRound2EveryRowCarriesTheSeedMark(t *testing.T) {
	anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, shopper := range round2Bundle(t).Shoppers {
		for i, row := range shopper.History.Rows {
			if row.SeedMark != domain.SeedMark {
				t.Errorf("%s row %d: SeedMark = %q, want %q", shopper.ShopperID, i, row.SeedMark, domain.SeedMark)
			}
			if evidence := row.SeededEvidence(anchor); !evidence.Seeded {
				t.Errorf("%s row %d: materialized evidence lost the seeded mark", shopper.ShopperID, i)
			}
		}
	}
}

// TestRound2ShoppersAreFair pins the structural invariants every case
// rests on: rows ordered inside a session extent, fit evidence spread
// across it, non-fit causes never counting toward fit, and each case's
// own premise (no candidate-brand history where the case says so, a rival
// birth where the answer depends on it).
func TestRound2ShoppersAreFair(t *testing.T) {
	bundle := round2Bundle(t)
	for _, shopper := range bundle.Shoppers {
		history := shopper.History
		if len(history.Rows) == 0 {
			t.Errorf("%s has no rows", shopper.ShopperID)
			continue
		}
		for i := 1; i < len(history.Rows); i++ {
			if history.Rows[i].Day < history.Rows[i-1].Day {
				t.Errorf("%s row %d precedes row %d: history must stay in event-time order",
					shopper.ShopperID, i, i-1)
			}
		}
		fitDays := map[float64]bool{}
		var firstFit, lastFit float64
		fitCount := 0
		candidateRows := 0
		for i, row := range history.Rows {
			if row.Day < 0 || row.Day > history.SessionDays {
				t.Errorf("%s row %d: day %.0f outside session extent %.0f",
					shopper.ShopperID, i, row.Day, history.SessionDays)
			}
			if row.Item.BrandID == shopper.Candidate.BrandID {
				candidateRows++
			}
			evidence := row.SeededEvidence(time.Now().UTC())
			if row.Signal == domain.SignalClassifiedOutcome && evidence.CountsToward() == domain.BeliefKindFit {
				if row.Reason == nil {
					t.Errorf("%s row %d: fit-counting row carries no reason", shopper.ShopperID, i)
					continue
				}
				if fitCount == 0 {
					firstFit = row.Day
				}
				lastFit = row.Day
				fitDays[row.Day] = true
				fitCount++
			}
			if row.Signal == domain.SignalClassifiedOutcome && row.Reason != nil &&
				row.Reason.Category.CountsToward() != domain.BeliefKindFit &&
				evidence.CountsToward() == domain.BeliefKindFit {
				t.Errorf("%s row %q: non-fit reason counts toward fit", shopper.ShopperID, row.Note)
			}
		}
		if len(fitDays) != fitCount {
			t.Errorf("%s shares an event day across fit rows; each needs its own interaction", shopper.ShopperID)
		}
		if fitCount >= 3 && (lastFit-firstFit)/history.SessionDays < 0.3 {
			t.Errorf("%s fit coverage = %.3f, want at least 0.3 over the session extent",
				shopper.ShopperID, (lastFit-firstFit)/history.SessionDays)
		}
		switch shopper.CaseID {
		case Round2CaseNewToBrand:
			if candidateRows != 0 {
				t.Errorf("%s has %d rows on the candidate brand; case 1 needs a shopper new to it",
					shopper.ShopperID, candidateRows)
			}
		default:
			if candidateRows == 0 {
				t.Errorf("%s has no rows on the candidate brand", shopper.ShopperID)
			}
		}
	}
}

// TestRound2ArmTwoCannotAttributeTheReturns pins the fairness premise arm
// 2 rests on: for every fit return on the candidate line, the same line
// and size also carries a non-fit outcome, so a reader holding only
// PayPal's outcomes cannot tell which returns were about fit. A
// fits-well feedback row is fit evidence too, but it is a purchase that
// was not returned: PayPal shows it exactly as it shows a kept purchase,
// so it reveals no cause and needs no partner row. Where the shopper has
// no candidate-line history at all, the premise holds vacuously.
func TestRound2ArmTwoCannotAttributeTheReturns(t *testing.T) {
	for _, shopper := range round2Bundle(t).Shoppers {
		perSize := map[string]struct{ fitReturns, nonFit int }{}
		for _, row := range shopper.History.Rows {
			if row.Item.BrandID != shopper.Candidate.BrandID || row.Outcome == nil {
				continue
			}
			if row.Signal == domain.SignalFitFeedback {
				continue
			}
			counts := perSize[row.Item.Size]
			if row.SeededEvidence(time.Now().UTC()).CountsToward() == domain.BeliefKindFit {
				counts.fitReturns++
			} else if row.Outcome.Kind == domain.OutcomeRefund || row.Outcome.Kind == domain.OutcomeDispute {
				counts.nonFit++
			}
			perSize[row.Item.Size] = counts
		}
		for size, counts := range perSize {
			if counts.fitReturns > 0 && counts.nonFit == 0 {
				t.Errorf("%s: size %s carries %d fit returns and no non-fit outcome; arm 2 could attribute them",
					shopper.ShopperID, size, counts.fitReturns)
			}
		}
	}
}

// TestRound2RivalShopperReachesTheThreshold pins that the case-2 shopper
// whose answer depends on the rival-birth rule really reaches it under the
// layer's own counting: one belief direction, then
// domain.RivalBirthThreshold consecutive opposite-direction fit returns.
func TestRound2RivalShopperReachesTheThreshold(t *testing.T) {
	bundle := round2Bundle(t)
	var rival *Round2Shopper
	for i := range bundle.Shoppers {
		if bundle.Shoppers[i].ShopperID == "r2-crowd-rival" {
			rival = &bundle.Shoppers[i]
			break
		}
	}
	if rival == nil {
		t.Fatal("no r2-crowd-rival shopper")
	}
	var streak domain.OppositeStreak
	beliefDirection := domain.FitDirectionNone
	births := 0
	for _, row := range rival.History.Rows {
		if row.Signal != domain.SignalClassifiedOutcome || row.Reason == nil {
			continue
		}
		direction := row.Reason.Category.Direction()
		if direction == domain.FitDirectionNone {
			// A directionless fit answer is not opposite-direction and
			// moves no run, exactly as the layer's rule says.
			continue
		}
		if beliefDirection == domain.FitDirectionNone {
			beliefDirection = direction
			continue
		}
		if direction == beliefDirection {
			streak.NoteMatched()
			continue
		}
		if streak.NoteOpposite() {
			births++
			beliefDirection = direction
		}
	}
	if births != 1 {
		t.Errorf("r2-crowd-rival births %d rivals, want exactly 1 at the threshold", births)
	}
	if beliefDirection != domain.FitDirectionLarge {
		t.Errorf("r2-crowd-rival ends pointing %q, want large", beliefDirection)
	}
}

// TestRound2PopulationGivesTheCasesThePoolsTheyNeed pins both halves of
// the pool premise: the case-1 and case-2 candidate lines read Earned
// small, every other round-2 line reads Claimed, and the case-2 line stays
// Earned small after the case-2 shoppers' own fit rows join it, which is
// what the live layer does at ingest.
func TestRound2PopulationGivesTheCasesThePoolsTheyNeed(t *testing.T) {
	bundle := round2Bundle(t)
	asOf := bundle.Population.AsOf
	recent := asOf.Add(-domain.PoolRecentWindow)
	for _, line := range []domain.ProductLine{round2ValeLine, round2WrenLine} {
		all := Round2PoolCounts(bundle.Population.Rows, line, "M", time.Time{})
		window := Round2PoolCounts(bundle.Population.Rows, line, "M", recent)
		verdict := domain.DecidePool(all, window)
		if verdict.Status != domain.StatusEarned || verdict.Direction != domain.FitDirectionSmall {
			t.Errorf("line %s size M reads %q/%q, want Earned/small", line.Brand, verdict.Status, verdict.Direction)
		}
	}
	for _, line := range []domain.ProductLine{round2UmberLine, round2NorthLine, round2OtterLine, round2SableLine} {
		all := Round2PoolCounts(bundle.Population.Rows, line, "M", time.Time{})
		if verdict := domain.DecidePool(all, domain.PoolCounts{}); verdict.Status != domain.StatusClaimed {
			t.Errorf("line %s reads %q, want Claimed so the pool step says nothing", line.Brand, verdict.Status)
		}
	}

	// The live layer pools every shopper's own classified rows as their
	// history is ingested. Add the case-2 shoppers' fit rows as rows for
	// one new shopper hash each, and the crowd must still say small.
	rows := append([]domain.FitRow(nil), bundle.Population.Rows...)
	sequence := 0
	for _, shopper := range bundle.Shoppers {
		if shopper.CaseID != Round2CaseUnlikeCrowd {
			continue
		}
		sequence++
		for _, row := range shopper.History.Rows {
			evidence := row.SeededEvidence(asOf)
			if row.Item.BrandID != shopper.Candidate.BrandID {
				continue
			}
			if evidence.CountsToward() != domain.BeliefKindFit && row.Signal != domain.SignalFitFeedback {
				continue
			}
			rows = append(rows, domain.FitRow{
				Line:        shopper.Candidate.Line(),
				Size:        row.Item.Size,
				Direction:   fitDirectionOf(row),
				Verdict:     fitVerdictOf(row),
				Settled:     true,
				SettledKind: domain.FitSettledByReturnReason,
				ShopperHash: "round2-case2-shopper-" + string(rune('a'+sequence)),
				EventAt:     evidence.OccurredAt,
			})
		}
	}
	line := round2WrenLine
	all := Round2PoolCounts(rows, line, "M", time.Time{})
	window := Round2PoolCounts(rows, line, "M", recent)
	if verdict := domain.DecidePool(all, window); verdict.Status != domain.StatusEarned || verdict.Direction != domain.FitDirectionSmall {
		t.Errorf("case-2 line reads %q/%q after the shoppers' own rows join the pool, want Earned/small",
			verdict.Status, verdict.Direction)
	}
}

// fitDirectionOf reports the direction one history row's reason points,
// and none for a row with no fit reason.
func fitDirectionOf(row Row) domain.FitDirection {
	if row.Reason == nil {
		return domain.FitDirectionNone
	}
	return row.Reason.Category.Direction()
}

// fitVerdictOf reports the verdict one feedback row carries, and empty for
// every other row.
func fitVerdictOf(row Row) domain.FitVerdict {
	switch {
	case row.Signal != domain.SignalFitFeedback:
		return ""
	case fitDirectionOf(row) == domain.FitDirectionNone:
		return domain.FitVerdictFitsWell
	default:
		return ""
	}
}

// TestRound2HiddenAnswersStayOffTheArmPath proves the arm-facing shoppers
// file carries no answer and no rationale, and that every shopper's answer
// is in the hidden file instead.
func TestRound2HiddenAnswersStayOffTheArmPath(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(round2Dir, "shoppers", "shoppers.json"))
	if err != nil {
		t.Fatalf("read shoppers: %v", err)
	}
	arm := string(raw)
	if strings.Contains(arm, "right_size") || strings.Contains(arm, "RightSize") {
		t.Error("arm-facing shoppers contain an answer key")
	}
	bundle := round2Bundle(t)
	for _, answer := range bundle.Answers {
		if strings.Contains(arm, answer.Why) {
			t.Errorf("arm-facing shoppers contain the hidden rationale for %s", answer.ShopperID)
		}
	}
	answersRaw, err := os.ReadFile(filepath.Join(round2Dir, "hidden", "answers.json"))
	if err != nil {
		t.Fatalf("read hidden answers: %v", err)
	}
	var stored []Round2Answer
	if err := json.Unmarshal(answersRaw, &stored); err != nil {
		t.Fatalf("decode hidden answers: %v", err)
	}
	if len(stored) != len(bundle.Shoppers) {
		t.Errorf("%d stored answers for %d shoppers", len(stored), len(bundle.Shoppers))
	}
}
