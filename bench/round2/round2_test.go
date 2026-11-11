package round2

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/seed"
)

// fixture is the loaded frozen set every test below runs on.
type fixture struct {
	shoppers   []seed.Round2Shopper
	byID       map[string]seed.Round2Shopper
	answers    map[string]seed.Round2Answer
	population seed.Population
}

func loadFixture(t *testing.T) fixture {
	t.Helper()
	shoppers, err := LoadShoppers()
	if err != nil {
		t.Fatalf("load shoppers: %v", err)
	}
	answers, err := LoadAnswers()
	if err != nil {
		t.Fatalf("load answers: %v", err)
	}
	population, err := LoadPopulation()
	if err != nil {
		t.Fatalf("load population: %v", err)
	}
	byID := map[string]seed.Round2Shopper{}
	for _, shopper := range shoppers {
		byID[shopper.ShopperID] = shopper
	}
	return fixture{shoppers: shoppers, byID: byID, answers: answers, population: population}
}

// TestFrozenShoppersAreTheGenerator pins the committed fixtures to the
// generator, so the freeze and the generator cannot drift apart.
func TestFrozenShoppersAreTheGenerator(t *testing.T) {
	bundle, err := seed.GenerateRound2(seed.Round2Seed)
	if err != nil {
		t.Fatalf("generate round 2: %v", err)
	}
	frozen := loadFixture(t)
	if len(frozen.shoppers) != len(bundle.Shoppers) {
		t.Fatalf("frozen shoppers = %d, generator = %d", len(frozen.shoppers), len(bundle.Shoppers))
	}
	for i, shopper := range frozen.shoppers {
		if shopper.ShopperID != bundle.Shoppers[i].ShopperID {
			t.Fatalf("frozen shopper %d is %q, generator says %q", i, shopper.ShopperID, bundle.Shoppers[i].ShopperID)
		}
		if shopper.Candidate != bundle.Shoppers[i].Candidate {
			t.Errorf("%s candidate differs from the generator", shopper.ShopperID)
		}
		if len(shopper.History.Rows) != len(bundle.Shoppers[i].History.Rows) {
			t.Errorf("%s carries %d rows, generator says %d",
				shopper.ShopperID, len(shopper.History.Rows), len(bundle.Shoppers[i].History.Rows))
		}
	}
	if len(frozen.population.Rows) != len(bundle.Population.Rows) {
		t.Errorf("frozen population = %d rows, generator = %d", len(frozen.population.Rows), len(bundle.Population.Rows))
	}
	if len(frozen.answers) != len(bundle.Answers) {
		t.Errorf("frozen answers = %d, generator = %d", len(frozen.answers), len(bundle.Answers))
	}
}

// TestArmsAndCasesMatchProduct pins the four arms' numbering and names and
// the four cases' shape: product.md's table, two shoppers each.
func TestArmsAndCasesMatchProduct(t *testing.T) {
	want := map[Arm]string{
		ArmNoMemory:           "no-memory",
		ArmPayPalHistory:      "paypal-history",
		ArmMementoShopperOnly: "memento-shopper-only",
		ArmMementoFull:        "memento-full",
	}
	for _, arm := range Arms() {
		if arm.String() != want[arm] {
			t.Errorf("arm %d is %q, want %q", int(arm), arm.String(), want[arm])
		}
		if arm.memoryTool() == "" {
			t.Errorf("arm %d names no memory tool", int(arm))
		}
	}
	if arms := Arms(); len(arms) != 4 || arms[0] != 1 || arms[3] != 4 {
		t.Errorf("arms = %v, want 1..4 in order", arms)
	}
	frozen := loadFixture(t)
	cases := Cases()
	if len(cases) != 4 {
		t.Fatalf("cases = %d, want 4", len(cases))
	}
	seen := map[string]bool{}
	for _, one := range cases {
		if one.Title == "" || one.Tests == "" || one.Separates == "" {
			t.Errorf("case %s is missing its statement", one.ID)
		}
		if len(one.Shoppers) != 2 {
			t.Errorf("case %s names %d shoppers, want exactly 2", one.ID, len(one.Shoppers))
		}
		for _, id := range one.Shoppers {
			shopper, ok := frozen.byID[id]
			if !ok {
				t.Errorf("case %s names %q, which is not a frozen shopper", one.ID, id)
				continue
			}
			if shopper.CaseID != one.ID {
				t.Errorf("%s carries case %q, case %s lists it", id, shopper.CaseID, one.ID)
			}
			if seen[id] {
				t.Errorf("%s is listed by two cases", id)
			}
			seen[id] = true
			if _, ok := frozen.answers[id]; !ok {
				t.Errorf("%s has no hidden answer", id)
			}
		}
	}
	for _, shopper := range frozen.shoppers {
		if !seen[shopper.ShopperID] {
			t.Errorf("%s is in no case", shopper.ShopperID)
		}
	}
}

// TestFairness proves the case contract before any run: a perfect reasoner
// given one arm's input gets the answer exactly when the frozen table says
// that input holds it, and where it holds nothing it names the missing
// fact. The reasoner reads the frozen history and population, never the
// answer key.
func TestFairness(t *testing.T) {
	frozen := loadFixture(t)
	for _, shopper := range frozen.shoppers {
		expectation, err := ExpectationFor(shopper.ShopperID)
		if err != nil {
			t.Errorf("%s: %v", shopper.ShopperID, err)
			continue
		}
		answer := frozen.answers[shopper.ShopperID]
		for _, arm := range Arms() {
			first, err := Decide(shopper, arm, frozen.population)
			if err != nil {
				t.Errorf("%s arm %d: decide: %v", shopper.ShopperID, int(arm), err)
				continue
			}
			holds, declared := expectation.First[arm]
			if !declared {
				t.Errorf("%s arm %d: no frozen hold expectation", shopper.ShopperID, int(arm))
				continue
			}
			if holds {
				if !first.Hold || first.Size != answer.RightSize {
					t.Errorf("%s arm %d: input is declared to hold the answer but the reasoner decides %q (hold %v), hidden answer is %q",
						shopper.ShopperID, int(arm), first.Size, first.Hold, answer.RightSize)
				}
				if declared := expectation.Missing[arm]; declared != "" {
					t.Errorf("%s arm %d: the table holds the answer but names a missing fact %q",
						shopper.ShopperID, int(arm), declared)
				}
				continue
			}
			// An input may hold a weaker basis than the answer: the
			// shopper-only arm answers a new shopper's usual size from their
			// statement, and the case exists because that is not the right
			// size. The table still says the input does not hold the answer,
			// so a decided size must differ from it, and the table must say
			// why that size is not the answer.
			if first.Hold && first.Size == answer.RightSize {
				t.Errorf("%s arm %d: input decides the hidden answer %q, and the frozen table says it does not hold it",
					shopper.ShopperID, int(arm), answer.RightSize)
			}
			if declared := strings.TrimSpace(expectation.Missing[arm]); declared == "" {
				t.Errorf("%s arm %d: the table says the input holds no answer but names no reason", shopper.ShopperID, int(arm))
			} else if !first.Hold && first.Missing != declared {
				t.Errorf("%s arm %d: the reasoner names the missing fact %q, the frozen table says %q",
					shopper.ShopperID, int(arm), first.Missing, declared)
			}
		}
		if shopper.Correction == nil {
			if len(expectation.After) != 0 {
				t.Errorf("%s has no correction but a frozen after-correction row", shopper.ShopperID)
			}
			continue
		}
		for _, arm := range Arms() {
			after, err := DecideAfterCorrection(shopper, arm, frozen.population)
			if err != nil {
				t.Errorf("%s arm %d: decide after correction: %v", shopper.ShopperID, int(arm), err)
				continue
			}
			holds, declared := expectation.After[arm]
			if !declared {
				t.Errorf("%s arm %d: no frozen after-correction expectation", shopper.ShopperID, int(arm))
				continue
			}
			if holds {
				if !after.Hold || after.Size != answer.RightSizeAfterCorrection {
					t.Errorf("%s arm %d after correction: reasoner decides %q (hold %v), hidden answer is %q",
						shopper.ShopperID, int(arm), after.Size, after.Hold, answer.RightSizeAfterCorrection)
				}
				if declared := expectation.Missing[arm]; declared != "" {
					t.Errorf("%s arm %d after correction: the table holds the answer but names a missing fact %q",
						shopper.ShopperID, int(arm), declared)
				}
				continue
			}
			if after.Hold && after.Size == answer.RightSizeAfterCorrection {
				t.Errorf("%s arm %d: after the correction the input decides the hidden answer %q, and the frozen table says it does not hold it",
					shopper.ShopperID, int(arm), answer.RightSizeAfterCorrection)
			}
			if declared := strings.TrimSpace(expectation.Missing[arm]); declared == "" {
				t.Errorf("%s arm %d after correction: the table says the input holds no answer but names no reason", shopper.ShopperID, int(arm))
			} else if !after.Hold && after.Missing != declared {
				t.Errorf("%s arm %d after correction: the reasoner names the missing fact %q, the frozen table says %q",
					shopper.ShopperID, int(arm), after.Missing, declared)
			}
		}
	}
}

// TestTheCasesSeparateTheArms pins each case's separation in the
// reasoner, so the stated reason is a measured property of the inputs and
// not a claim in prose.
func TestTheCasesSeparateTheArms(t *testing.T) {
	frozen := loadFixture(t)
	for _, shopper := range frozen.shoppers {
		answer := frozen.answers[shopper.ShopperID]
		switch shopper.CaseID {
		case seed.Round2CaseNewToBrand:
			// The pool is the only step that answers; the shopper-only arm
			// has nothing and says so.
			shopperOnly, err := Decide(shopper, ArmMementoShopperOnly, frozen.population)
			if err != nil {
				t.Fatalf("%s: %v", shopper.ShopperID, err)
			}
			full, err := Decide(shopper, ArmMementoFull, frozen.population)
			if err != nil {
				t.Fatalf("%s: %v", shopper.ShopperID, err)
			}
			if shopperOnly.Hold && shopperOnly.Size == answer.RightSize {
				t.Errorf("%s: arm 3 decides %q from an input with no candidate-brand evidence; the case needs it to miss",
					shopper.ShopperID, shopperOnly.Size)
			}
			if !full.Hold || full.Rung != RungPool || full.Size != answer.RightSize {
				t.Errorf("%s: arm 4 decides %q on %q, want the pool's %q",
					shopper.ShopperID, full.Size, full.Rung, answer.RightSize)
			}
			pool, err := PoolFor(shopper, frozen.population)
			if err != nil {
				t.Fatalf("%s: %v", shopper.ShopperID, err)
			}
			if pool.Status != "Earned" || pool.Direction != "small" {
				t.Errorf("%s: the pool reads %q/%q, the case needs Earned/small",
					shopper.ShopperID, pool.Status, pool.Direction)
			}
		case seed.Round2CaseUnlikeCrowd:
			// Own evidence decides; the pool alone points the other way.
			poolOnly, err := DecidePoolOnly(shopper, frozen.population)
			if err != nil {
				t.Fatalf("%s: %v", shopper.ShopperID, err)
			}
			if !poolOnly.Hold || poolOnly.Size == answer.RightSize {
				t.Errorf("%s: naive pooling answers %q, the case needs it wrong against %q",
					shopper.ShopperID, poolOnly.Size, answer.RightSize)
			}
			for _, arm := range []Arm{ArmMementoShopperOnly, ArmMementoFull} {
				decision, err := Decide(shopper, arm, frozen.population)
				if err != nil {
					t.Fatalf("%s: %v", shopper.ShopperID, err)
				}
				if !decision.Hold || decision.Rung != RungOwnEvidence || decision.Size != answer.RightSize {
					t.Errorf("%s arm %d decides %q on %q, want own evidence's %q",
						shopper.ShopperID, int(arm), decision.Size, decision.Rung, answer.RightSize)
				}
			}
		case seed.Round2CaseNoisyRefunds:
			// Both Memento arms decide from reasons; the PayPal input is
			// cause-blind; the pool says nothing, so the arms that hold
			// the answer hold it for one reason.
			pool, err := PoolFor(shopper, frozen.population)
			if err != nil {
				t.Fatalf("%s: %v", shopper.ShopperID, err)
			}
			if pool.Status != "Claimed" {
				t.Errorf("%s: the pool reads %q, the case needs it to say nothing", shopper.ShopperID, pool.Status)
			}
			for _, arm := range []Arm{ArmMementoShopperOnly, ArmMementoFull} {
				decision, err := Decide(shopper, arm, frozen.population)
				if err != nil {
					t.Fatalf("%s: %v", shopper.ShopperID, err)
				}
				if !decision.Hold || decision.Rung != RungOwnEvidence {
					t.Errorf("%s arm %d decides on %q, want own evidence", shopper.ShopperID, int(arm), decision.Rung)
				}
			}
			fits := OwnFitRows(shopper)
			directions := map[string]int{}
			for _, fit := range fits {
				directions[string(fit.Direction)]++
			}
			if len(directions) != 1 || len(fits) < 3 {
				t.Errorf("%s: %d fit returns across %d directions, want at least 3 in one direction",
					shopper.ShopperID, len(fits), len(directions))
			}
		case seed.Round2CaseCorrection:
			before, err := Decide(shopper, ArmMementoFull, frozen.population)
			if err != nil {
				t.Fatalf("%s: %v", shopper.ShopperID, err)
			}
			after, err := DecideAfterCorrection(shopper, ArmMementoFull, frozen.population)
			if err != nil {
				t.Fatalf("%s: %v", shopper.ShopperID, err)
			}
			if !after.Hold || after.Rung != RungCorrection || after.Size != answer.RightSizeAfterCorrection {
				t.Errorf("%s: after the correction the layer decides %q on %q, want %q",
					shopper.ShopperID, after.Size, after.Rung, answer.RightSizeAfterCorrection)
			}
			if before.Size == answer.RightSizeAfterCorrection && before.Rung == RungCorrection {
				t.Errorf("%s: the correction is already in force before it was made", shopper.ShopperID)
			}
			for _, arm := range []Arm{ArmNoMemory, ArmPayPalHistory} {
				missing, err := DecideAfterCorrection(shopper, arm, frozen.population)
				if err != nil {
					t.Fatalf("%s: %v", shopper.ShopperID, err)
				}
				if missing.Hold {
					t.Errorf("%s arm %d holds the answer after the correction; the correction is in no PayPal field",
						shopper.ShopperID, int(arm))
				}
			}
		}
	}
}

// TestExpectedRecallIsWhatTheMementoArmsSee pins the recall fixture's
// shape and the projection in both phases: the pool is present for arm 4
// and null for arm 3, the guidance block is dropped for both, and the
// correction reaches both Memento arms only in the phase after it.
func TestExpectedRecallIsWhatTheMementoArmsSee(t *testing.T) {
	frozen := loadFixture(t)
	for _, shopper := range frozen.shoppers {
		for _, phase := range Phases() {
			raw, err := ExpectedRecall(shopper, frozen.population, phase)
			if err != nil {
				t.Fatalf("%s %s: expected recall: %v", shopper.ShopperID, phase, err)
			}
			var response map[string]any
			if err := json.Unmarshal(raw, &response); err != nil {
				t.Fatalf("%s %s: decode expected recall: %v", shopper.ShopperID, phase, err)
			}
			for _, key := range []string{"shopper_id", "beliefs", "pool", "pool_lookup", "overriding_correction", "guidance", "pending_count"} {
				if _, ok := response[key]; !ok {
					t.Errorf("%s %s: expected recall has no %q", shopper.ShopperID, phase, key)
				}
			}
			if response["shopper_id"] != shopper.ShopperID {
				t.Errorf("%s %s: expected recall is for %v", shopper.ShopperID, phase, response["shopper_id"])
			}
			full, err := ProjectRecall(raw, ArmMementoFull)
			if err != nil {
				t.Fatalf("%s %s: project arm 4: %v", shopper.ShopperID, phase, err)
			}
			shopperOnly, err := ProjectRecall(raw, ArmMementoShopperOnly)
			if err != nil {
				t.Fatalf("%s %s: project arm 3: %v", shopper.ShopperID, phase, err)
			}
			for _, projected := range [][]byte{full, shopperOnly} {
				if strings.Contains(string(projected), "\"guidance\"") {
					t.Errorf("%s %s: the projection kept the guidance block", shopper.ShopperID, phase)
				}
			}
			var fullView, shopperView map[string]any
			if err := json.Unmarshal(full, &fullView); err != nil {
				t.Fatalf("%s %s: decode arm 4 memory: %v", shopper.ShopperID, phase, err)
			}
			if err := json.Unmarshal(shopperOnly, &shopperView); err != nil {
				t.Fatalf("%s %s: decode arm 3 memory: %v", shopper.ShopperID, phase, err)
			}
			if shopperView["pool"] != nil {
				t.Errorf("%s %s: arm 3's memory carries a pool", shopper.ShopperID, phase)
			}
			if shopper.CaseID == seed.Round2CaseNewToBrand || shopper.CaseID == seed.Round2CaseUnlikeCrowd {
				if fullView["pool"] == nil {
					t.Errorf("%s %s: arm 4's memory carries no pool, and the case needs one", shopper.ShopperID, phase)
				}
			}
			// The two Memento arms differ only in the pool: null it on
			// arm 4's view and the two memories must be byte-identical, so
			// no other field can drift between the conditions.
			fullView["pool"] = nil
			encodedFull, err := json.Marshal(fullView)
			if err != nil {
				t.Fatalf("%s %s: encode arm 4 memory: %v", shopper.ShopperID, phase, err)
			}
			encodedShopper, err := json.Marshal(shopperView)
			if err != nil {
				t.Fatalf("%s %s: encode arm 3 memory: %v", shopper.ShopperID, phase, err)
			}
			if !bytes.Equal(encodedFull, encodedShopper) {
				t.Errorf("%s %s: arms 3 and 4 carry memories that differ in more than the pool", shopper.ShopperID, phase)
			}
			if shopper.Correction == nil {
				continue
			}
			wantCorrection := phase == PhaseAfterCorrection
			if got := strings.Contains(string(shopperOnly), shopper.Correction.Text); got != wantCorrection {
				t.Errorf("%s %s: arm 3's memory carries the correction = %v, want %v",
					shopper.ShopperID, phase, got, wantCorrection)
			}
			if got := strings.Contains(string(full), shopper.Correction.Text); got != wantCorrection {
				t.Errorf("%s %s: arm 4's memory carries the correction = %v, want %v",
					shopper.ShopperID, phase, got, wantCorrection)
			}
		}
	}
}

// TestPromptsDifferOnlyInMemory pins the instrument: the base section is
// byte-identical across the four arms and the memory block is the only
// difference, and arms 3 and 4 differ only in the pool.
func TestPromptsDifferOnlyInMemory(t *testing.T) {
	frozen := loadFixture(t)
	for _, shopper := range frozen.shoppers {
		base := BaseSection(shopper)
		prompts := map[Arm]string{}
		memories := map[Arm]string{}
		for _, arm := range Arms() {
			prompt, fetch, err := PromptFor(shopper, arm, frozen.population, PhaseBeforeCorrection)
			if err != nil {
				t.Fatalf("%s arm %d: prompt: %v", shopper.ShopperID, int(arm), err)
			}
			if !strings.HasPrefix(prompt, base) {
				t.Errorf("%s arm %d: prompt does not open with the shared base", shopper.ShopperID, int(arm))
			}
			memory, err := MemorySection(arm, fetch)
			if err != nil {
				t.Fatalf("%s arm %d: memory: %v", shopper.ShopperID, int(arm), err)
			}
			if prompt != base+"\n"+memory {
				t.Errorf("%s arm %d: prompt is not base plus memory", shopper.ShopperID, int(arm))
			}
			prompts[arm] = prompt
			memories[arm] = memory
		}
		if !strings.HasPrefix(memories[ArmNoMemory], "Memory: none") {
			t.Errorf("%s: arm 1's memory does not say it has none", shopper.ShopperID)
		}
		if prompts[ArmNoMemory] == prompts[ArmMementoFull] {
			t.Errorf("%s: arm 1 and arm 4 carry the same prompt", shopper.ShopperID)
		}
		if strings.Contains(memories[ArmNoMemory], shopper.Candidate.BrandName) {
			t.Errorf("%s: arm 1's memory names the brand", shopper.ShopperID)
		}
		if memories[ArmMementoShopperOnly] == memories[ArmMementoFull] {
			t.Errorf("%s: arms 3 and 4 carry the same memory", shopper.ShopperID)
		}
	}
}

// TestArmTwoInputCarriesNoCause pins arm 2's fairness: every record is the
// shopper's own, no free text names a cause, the correction appears
// nowhere, every order carries its line item, and the committed records
// extract to the same bytes as the generated ones.
func TestArmTwoInputCarriesNoCause(t *testing.T) {
	frozen := loadFixture(t)
	for _, shopper := range frozen.shoppers {
		input, err := ArmTwoInput(shopper)
		if err != nil {
			t.Fatalf("%s: arm 2 input: %v", shopper.ShopperID, err)
		}
		committed, err := LoadArmTwoInput(fixturePath("."), shopper.ShopperID)
		if err != nil {
			t.Fatalf("%s: committed arm 2 input: %v", shopper.ShopperID, err)
		}
		if !bytes.Equal(input, committed) {
			t.Errorf("%s: the committed records extract differently from the generated ones", shopper.ShopperID)
		}
		if words := fitWordsIn(string(input)); len(words) > 0 {
			t.Errorf("%s: arm 2's input names a cause: %v", shopper.ShopperID, words)
		}
		if shopper.Correction != nil && strings.Contains(string(input), shopper.Correction.Text) {
			t.Errorf("%s: arm 2's input carries the correction", shopper.ShopperID)
		}
		var records []map[string]any
		if err := json.Unmarshal(input, &records); err != nil {
			t.Fatalf("%s: decode arm 2 input: %v", shopper.ShopperID, err)
		}
		if len(records) == 0 {
			t.Errorf("%s: arm 2's input is empty", shopper.ShopperID)
		}
		orders := 0
		for _, record := range records {
			info, isTransaction := record["transaction_info"].(map[string]any)
			if isTransaction {
				if info["custom_field"] != PayPalCustomID(shopper.ShopperID) {
					t.Errorf("%s: a transaction record carries custom field %v", shopper.ShopperID, info["custom_field"])
				}
				continue
			}
			if units, isOrder := record["purchase_units"].([]any); isOrder {
				orders++
				if len(units) != 1 {
					t.Errorf("%s: an order carries %d purchase units, want exactly 1", shopper.ShopperID, len(units))
				}
				for _, unit := range units {
					fields, ok := unit.(map[string]any)
					if !ok {
						continue
					}
					if fields["custom_id"] != PayPalCustomID(shopper.ShopperID) {
						t.Errorf("%s: an order unit carries custom id %v", shopper.ShopperID, fields["custom_id"])
					}
					items, _ := fields["items"].([]any)
					if len(items) == 0 {
						t.Errorf("%s: an order carries no line item, so arm 2 has no item", shopper.ShopperID)
					}
					if len(items) > 1 {
						t.Errorf("%s: an order carries %d line items; multi-item orders are excluded because the fit row is keyed by the capture", shopper.ShopperID, len(items))
					}
				}
				continue
			}
			if _, isRefund := record["status"].(string); isRefund && record["id"] != nil {
				if record["custom_id"] != PayPalCustomID(shopper.ShopperID) {
					t.Errorf("%s: a refund record carries custom id %v", shopper.ShopperID, record["custom_id"])
				}
				continue
			}
			if disputed, isDispute := record["disputed_transactions"].([]any); isDispute {
				for _, transaction := range disputed {
					fields, ok := transaction.(map[string]any)
					if !ok {
						continue
					}
					if fields["custom"] != PayPalCustomID(shopper.ShopperID) {
						t.Errorf("%s: a disputed transaction carries custom %v", shopper.ShopperID, fields["custom"])
					}
				}
				continue
			}
		}
		// Every capture the listing shows must have its order, so line
		// items are supplied for every order and none is withheld.
		captures := 0
		for _, record := range records {
			if info, ok := record["transaction_info"].(map[string]any); ok && info["transaction_event_code"] == "T0006" {
				captures++
			}
		}
		if captures != orders {
			t.Errorf("%s: %d captures and %d orders; every order needs its line items", shopper.ShopperID, captures, orders)
		}
	}
}

// TestArmTwoCannotReadTheAnswers pins the separation of the two paths: no
// prompt any arm can be given carries a hidden answer or its rationale.
func TestArmTwoCannotReadTheAnswers(t *testing.T) {
	frozen := loadFixture(t)
	for _, shopper := range frozen.shoppers {
		answer := frozen.answers[shopper.ShopperID]
		for _, arm := range Arms() {
			prompt, _, err := PromptFor(shopper, arm, frozen.population, PhaseBeforeCorrection)
			if err != nil {
				t.Fatalf("%s arm %d: %v", shopper.ShopperID, int(arm), err)
			}
			if strings.Contains(prompt, answer.Why) {
				t.Errorf("%s arm %d: the prompt carries the hidden rationale", shopper.ShopperID, int(arm))
			}
			if strings.Contains(prompt, "right_size") || strings.Contains(prompt, "RightSize") {
				t.Errorf("%s arm %d: the prompt carries an answer key", shopper.ShopperID, int(arm))
			}
		}
	}
}

// TestRunsBeginOnlyAfterTheFreeze pins the run-directory protocol. A run
// directory that does not exist is the pre-freeze state and passes. Once it
// exists, its marker must open this freeze: the marker names the commit
// that landed the freeze and pins the manifest by hash, and the hash must
// match the file in this tree. A run directory without a marker, or with a
// marker from an earlier freeze, fails here — and the freeze check refuses
// the directory too unless the gate passes --allow-runs, which it does only
// when the marker exists.
func TestRunsBeginOnlyAfterTheFreeze(t *testing.T) {
	dir := fixturePath("runs")
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("read runs: %v", err)
	}
	marker, err := ReadRunsMarker(dir)
	if err != nil {
		t.Fatalf("%v: write bench/round2/runs/%s with round2.BeginRuns before the first turn (FROZEN.md section 8)", err, RunsMarkerFile)
	}
	if !commitPattern.MatchString(marker.FreezeCommit) {
		t.Errorf("the runs marker names freeze commit %q, want a full commit id", marker.FreezeCommit)
	}
}

// TestRunsMarkerBindsTheFreeze proves the marker protocol's own code in a
// temporary directory: BeginRuns writes a marker that opens this freeze, a
// short commit id is refused, and a marker written against another manifest
// no longer opens it.
func TestRunsMarkerBindsTheFreeze(t *testing.T) {
	dir := t.TempDir()
	const commit = "0123456789abcdef0123456789abcdef01234567"
	if _, err := BeginRuns(dir, "not-a-commit", "test"); !errors.Is(err, ErrNoRunsMarker) {
		t.Errorf("a short commit id was accepted: %v", err)
	}
	marker, err := BeginRuns(dir, commit, "test marker")
	if err != nil {
		t.Fatalf("open runs: %v", err)
	}
	manifest, err := FrozenManifestSHA256()
	if err != nil {
		t.Fatalf("freeze manifest hash: %v", err)
	}
	if marker.FreezeCommit != commit || marker.FrozenManifestSHA256 != manifest {
		t.Errorf("marker = %+v, want commit %s and manifest %s", marker, commit, manifest)
	}
	if _, err := ReadRunsMarker(dir); err != nil {
		t.Errorf("a marker just written does not open the freeze: %v", err)
	}
	// A marker from an earlier freeze must not open this one.
	stale := RunsMarker{FreezeCommit: commit, FrozenManifestSHA256: "00" + manifest[2:]}
	raw, err := json.MarshalIndent(stale, "", "  ")
	if err != nil {
		t.Fatalf("encode stale marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, RunsMarkerFile), raw, 0o644); err != nil {
		t.Fatalf("write stale marker: %v", err)
	}
	if _, err := ReadRunsMarker(dir); !errors.Is(err, ErrNoRunsMarker) {
		t.Errorf("a marker from another freeze opened this one: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, RunsMarkerFile)); err != nil {
		t.Fatalf("remove marker: %v", err)
	}
	if _, err := ReadRunsMarker(dir); !errors.Is(err, ErrNoRunsMarker) {
		t.Errorf("an empty run directory opened the freeze: %v", err)
	}
}

// TestFirstTurnPrecedesTheCorrection pins the phase rule. The first turn's
// Memento prompt carries no correction, because the layer has not been told
// yet; every later turn carries it. A first turn rendered from the
// post-correction response would answer after the shopper corrected, which
// is the opposite of what case 4 measures.
func TestFirstTurnPrecedesTheCorrection(t *testing.T) {
	frozen := loadFixture(t)
	corrected := 0
	for _, shopper := range frozen.shoppers {
		if TurnPhase(TurnFirst) != PhaseBeforeCorrection {
			t.Fatalf("the first turn runs in phase %q, want %q", TurnPhase(TurnFirst), PhaseBeforeCorrection)
		}
		for _, turn := range []TurnKind{TurnCorrection, TurnNextSession, TurnOtherAgent} {
			if TurnPhase(turn) != PhaseAfterCorrection {
				t.Fatalf("%q runs in phase %q, want %q", turn, TurnPhase(turn), PhaseAfterCorrection)
			}
		}
		for _, arm := range []Arm{ArmMementoShopperOnly, ArmMementoFull} {
			before, beforeFetch, err := PromptFor(shopper, arm, frozen.population, TurnPhase(TurnFirst))
			if err != nil {
				t.Fatalf("%s arm %d: first prompt: %v", shopper.ShopperID, int(arm), err)
			}
			after, afterFetch, err := PromptFor(shopper, arm, frozen.population, TurnPhase(TurnOtherAgent))
			if err != nil {
				t.Fatalf("%s arm %d: later prompt: %v", shopper.ShopperID, int(arm), err)
			}
			if shopper.Correction == nil {
				if before != after || !bytes.Equal(beforeFetch.Response, afterFetch.Response) {
					t.Errorf("%s arm %d: the phases differ for a shopper with no correction", shopper.ShopperID, int(arm))
				}
				continue
			}
			if strings.Contains(before, shopper.Correction.Text) {
				t.Errorf("%s arm %d: the first-turn prompt carries the correction text", shopper.ShopperID, int(arm))
			}
			if strings.Contains(before, `"overriding_correction": {`) {
				t.Errorf("%s arm %d: the first-turn memory carries an overriding correction", shopper.ShopperID, int(arm))
			}
			if !strings.Contains(after, shopper.Correction.Text) {
				t.Errorf("%s arm %d: the later-turn prompt carries no correction text", shopper.ShopperID, int(arm))
			}
		}
		if shopper.Correction == nil {
			continue
		}
		corrected++
	}
	if corrected != 2 {
		t.Errorf("%d shoppers have a correction, want 2", corrected)
	}
}

// TestOwnEvidenceCanBeServed pins the promotion premise every own-evidence
// case rests on: recall serves a belief only at Candidate or better, which
// needs dependents from more than one interaction, and Earned needs three
// spread over at least 0.3 of the session extent. Each case's own-evidence
// belief therefore carries at least three returns spanning that much, so a
// live cell cannot refuse for want of standing and silently turn a
// measurement into a no-answer.
func TestOwnEvidenceCanBeServed(t *testing.T) {
	frozen := loadFixture(t)
	for _, shopper := range frozen.shoppers {
		decision, err := Decide(shopper, ArmMementoShopperOnly, frozen.population)
		if err != nil {
			t.Fatalf("%s: %v", shopper.ShopperID, err)
		}
		if decision.Rung != RungOwnEvidence {
			continue
		}
		beliefs := ownBeliefs(shopper)
		if len(beliefs) == 0 {
			// The answer came from fit feedback, which is strong evidence
			// on its own and needs no promotion of a return belief.
			if len(FeedbackRows(shopper)) == 0 {
				t.Errorf("%s: own evidence decides but the history carries neither a belief nor feedback", shopper.ShopperID)
			}
			continue
		}
		top := beliefs[0]
		if len(top.days) < 3 {
			t.Errorf("%s: the deciding belief has %d evidence rows, want at least 3 so it can be served",
				shopper.ShopperID, len(top.days))
			continue
		}
		span := top.days[len(top.days)-1] - top.days[0]
		if coverage := span / shopper.History.SessionDays; coverage < 0.3 {
			t.Errorf("%s: the deciding belief's evidence covers %.2f of the session, want at least 0.3",
				shopper.ShopperID, coverage)
		}
		if top.direction != domain.FitDirectionSmall && top.direction != domain.FitDirectionLarge {
			t.Errorf("%s: the deciding belief points %q", shopper.ShopperID, top.direction)
		}
	}
}

// TestExpectedRecallDecodesAsTheContract pins the expected recall against
// the wire type the layer serves, so T10.4 can compare a live response with
// it field for field: an unknown field, a wrong type or a missing label
// fails here rather than after a paid run.
func TestExpectedRecallDecodesAsTheContract(t *testing.T) {
	frozen := loadFixture(t)
	for _, shopper := range frozen.shoppers {
		for _, phase := range Phases() {
			raw, err := ExpectedRecall(shopper, frozen.population, phase)
			if err != nil {
				t.Fatalf("%s %s: expected recall: %v", shopper.ShopperID, phase, err)
			}
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			var response contract.RecallResponse
			if err := decoder.Decode(&response); err != nil {
				t.Errorf("%s: expected recall does not decode as recall_for_purchase: %v", shopper.ShopperID, err)
				continue
			}
			if response.ShopperID != shopper.ShopperID {
				t.Errorf("%s: response names %q", shopper.ShopperID, response.ShopperID)
			}
			if response.PoolLookup != contract.PoolConsulted {
				t.Errorf("%s: pool_lookup is %q, want consulted", shopper.ShopperID, response.PoolLookup)
			}
			if response.Guidance.Text == "" {
				t.Errorf("%s: the expected response carries no guidance block to project away", shopper.ShopperID)
			}
			if response.PendingCount != PendingCount(shopper) {
				t.Errorf("%s: pending count is %d, history has %d", shopper.ShopperID, response.PendingCount, PendingCount(shopper))
			}
			for _, belief := range response.Beliefs {
				if belief.BeliefID == "" || belief.Value == "" || belief.Status == "" {
					t.Errorf("%s: a belief is unnamed, unvalued or unlabelled: %+v", shopper.ShopperID, belief)
				}
				for _, evidence := range belief.Evidence {
					if evidence.EvidenceID == "" || evidence.Item.Size == "" || evidence.WriteOp == "" {
						t.Errorf("%s: an evidence row is incomplete: %+v", shopper.ShopperID, evidence)
					}
				}
			}
			if response.Pool != nil {
				counts := response.Pool.Counts
				sum := counts.Returns.TooSmall + counts.Returns.TooLarge + counts.Feedback.FitsWell +
					counts.Reasons.DamagedOrDefective + counts.Reasons.NotArrived + counts.Reasons.ChangedMind + counts.Kept
				if counts.Settled != sum {
					t.Errorf("%s: pool settled count %d is not the sum %d", shopper.ShopperID, counts.Settled, sum)
				}
				if response.Pool.Status == domain.StatusClaimed {
					t.Errorf("%s: a Claimed pool must be null so the pool step says nothing", shopper.ShopperID)
				}
			}
		}
	}
}

// TestRecallTargetsCarryThePoolQuery pins the recall request every
// Memento arm sends: the pool read needs the brand, category, fit handle
// and size, and a request missing any of them answers "unavailable", which
// would quietly drop the pool the case-1 and case-2 premises need.
func TestRecallTargetsCarryThePoolQuery(t *testing.T) {
	frozen := loadFixture(t)
	for _, shopper := range frozen.shoppers {
		target := Target(shopper)
		if target.ShopperID != shopper.ShopperID {
			t.Errorf("%s: target names %q", shopper.ShopperID, target.ShopperID)
		}
		for field, value := range map[string]string{
			"brand_id":        target.BrandID,
			"category":        target.Category,
			"fit_handle":      target.FitHandle,
			"size_considered": target.SizeConsidered,
		} {
			if value == "" {
				t.Errorf("%s: recall target carries no %s, so the pool read answers unavailable", shopper.ShopperID, field)
			}
		}
		if !json.Valid(target.JSON()) {
			t.Errorf("%s: recall target is not valid JSON", shopper.ShopperID)
		}
		pool, err := PoolFor(shopper, frozen.population)
		if err != nil {
			t.Fatalf("%s: pool: %v", shopper.ShopperID, err)
		}
		if pool.Line.Brand != target.BrandID || pool.Line.Category != target.Category ||
			pool.Line.FitHandle != target.FitHandle || pool.Size != target.SizeConsidered {
			t.Errorf("%s: the pool was read for %v/%s, the target asks %v/%s",
				shopper.ShopperID, pool.Line, pool.Size, target.BrandID, target.SizeConsidered)
		}
	}
}

// TestGraderScoresGoodAndBadTurns pins the grader: a reply naming the
// right size with a fit cite scores right, a reply naming another size
// scores wrong, and a correction turn grades against the corrected size.
func TestGraderScoresGoodAndBadTurns(t *testing.T) {
	frozen := loadFixture(t)
	shopper, ok := frozen.byID["r2-noisy-up"]
	if !ok {
		t.Fatal("no r2-noisy-up shopper")
	}
	answer := frozen.answers[shopper.ShopperID]
	fitRow := seed.Row{}
	for _, row := range shopper.History.Rows {
		if row.Reason != nil && row.Reason.Category.Direction() == "small" {
			fitRow = row
			break
		}
	}
	if fitRow.Link.CustomID == "" {
		t.Fatal("no fit return with a link to cite")
	}
	goodRecord, err := NewRunFile("r2-test-good", shopper, ArmMementoShopperOnly, 1, TurnFirst, "A", "prompt", MemoryFetch{Tool: "recall_for_purchase", Response: []byte("{}")}, "Take size L. It is the fit return "+fitRow.Link.CustomID+".", 0)
	if err != nil {
		t.Fatalf("build good run: %v", err)
	}
	goodScore, err := GradeRunFile(goodRecord, shopper, answer)
	if err != nil {
		t.Fatalf("grade good run: %v", err)
	}
	if !goodScore.RightSize {
		t.Errorf("a reply naming the hidden size %q scored wrong", answer.RightSize)
	}
	if goodScore.Supported == 0 {
		t.Error("a cite to a fit row scored unsupported")
	}
	badRecord, err := NewRunFile("r2-test-bad", shopper, ArmMementoShopperOnly, 1, TurnFirst, "A", "prompt", MemoryFetch{Tool: "recall_for_purchase", Response: []byte("{}")}, "Take size S.", 0)
	if err != nil {
		t.Fatalf("build bad run: %v", err)
	}
	badScore, err := GradeRunFile(badRecord, shopper, answer)
	if err != nil {
		t.Fatalf("grade bad run: %v", err)
	}
	if badScore.RightSize {
		t.Error("a reply naming another size scored right")
	}

	corrected, ok := frozen.byID["r2-corr-belief"]
	if !ok {
		t.Fatal("no r2-corr-belief shopper")
	}
	correctedAnswer := frozen.answers[corrected.ShopperID]
	correctionRecord, err := NewRunFile("r2-test-corr", corrected, ArmMementoFull, 1, TurnCorrection, "A", "prompt", MemoryFetch{Tool: "recall_for_purchase", Response: []byte(`{"beliefs":[],"pool":null}`)}, "Take size M.", 0)
	if err != nil {
		t.Fatalf("build correction run: %v", err)
	}
	correctionScore, err := GradeRunFile(correctionRecord, corrected, correctedAnswer)
	if err != nil {
		t.Fatalf("grade correction run: %v", err)
	}
	if correctionScore.CorrectionHonoured == nil || !*correctionScore.CorrectionHonoured {
		t.Error("a correction turn naming the corrected size was not honoured")
	}
	if correctionScore.ExpectedSize != correctedAnswer.RightSizeAfterCorrection {
		t.Errorf("correction turn grades against %q, want %q", correctionScore.ExpectedSize, correctedAnswer.RightSizeAfterCorrection)
	}
}
