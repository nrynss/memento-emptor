package grader

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/seed"
)

const fixtureHistoriesDir = "../../testdata/seed/histories"
const hiddenAnswersPath = "../../testdata/seed/hidden/answers.json"

func loadFixtures(t *testing.T) (map[string]seed.History, map[string]seed.Answer) {
	t.Helper()
	histories, err := harness.LoadHistories(fixtureHistoriesDir)
	if err != nil {
		t.Fatalf("load fixture histories: %v", err)
	}
	byID := make(map[string]seed.History, len(histories))
	for _, h := range histories {
		byID[h.ShopperID] = h
	}
	answers, err := LoadAnswers(hiddenAnswersPath)
	if err != nil {
		t.Fatalf("load hidden answers: %v", err)
	}
	return byID, answers
}

// goodArm3Run is the hand-built good arm-3-style run: cmp-size-changed
// recommends L, the hidden right size, citing the two classified
// too-small returns.
func goodArm3Run(histories map[string]seed.History, answers map[string]seed.Answer) Run {
	return Run{
		ShopperID: "cmp-size-changed",
		History:   histories["cmp-size-changed"],
		Answer:    answers["cmp-size-changed"],
		First: Turn{
			Prompt: "Shopper: cmp-size-changed\nRecommend one size for the Heritage oxford.",
			Reply:  "Recommend L: two newer too-small returns in M say the size moved.",
			Size:   "L",
			Cites:  []string{"seed-7-cmp-size-changed-014", "seed-7-cmp-size-changed-015"},
		},
	}
}

func TestGoodRunScoresAsExpected(t *testing.T) {
	histories, answers := loadFixtures(t)
	score, err := ScoreRun(goodArm3Run(histories, answers))
	if err != nil {
		t.Fatalf("score good run: %v", err)
	}
	if !score.RightSize {
		t.Error("good run: RightSize = false, want true")
	}
	if score.WrongCause != 0 {
		t.Errorf("good run: WrongCause = %d, want 0", score.WrongCause)
	}
	if score.CorrectionHonoured != nil {
		t.Errorf("good run without correction: CorrectionHonoured = %v, want nil", *score.CorrectionHonoured)
	}
	if score.Supported != 2 || score.Cited != 2 {
		t.Errorf("good run: Supported/Cited = %d/%d, want 2/2", score.Supported, score.Cited)
	}
	if score.OverCap {
		t.Error("good run: OverCap = true, want false")
	}
	if len(score.Opinions) != 0 {
		t.Errorf("good run: Opinions = %v, want empty offline", score.Opinions)
	}
	if score.TokenBasis != TokenBasis {
		t.Errorf("good run: TokenBasis = %q, want recorded basis", score.TokenBasis)
	}
}

func TestWrongSizeScoresAsExpected(t *testing.T) {
	histories, answers := loadFixtures(t)
	run := Run{
		ShopperID: "cmp-broken",
		History:   histories["cmp-broken"],
		Answer:    answers["cmp-broken"],
		First: Turn{
			Prompt: "Shopper: cmp-broken\nRecommend one size for the Everyday oxford.",
			Reply:  "Recommend L.",
			Size:   "L",
		},
	}
	score, err := ScoreRun(run)
	if err != nil {
		t.Fatalf("score wrong-size run: %v", err)
	}
	if score.RightSize {
		t.Error("wrong-size run: RightSize = true, want false (answer is M)")
	}
	if score.WrongCause != 0 {
		t.Errorf("wrong-size run without cites: WrongCause = %d, want 0", score.WrongCause)
	}
}

func TestWrongCauseFollowedScoresAsExpected(t *testing.T) {
	histories, answers := loadFixtures(t)
	run := Run{
		ShopperID: "cmp-broken",
		History:   histories["cmp-broken"],
		Answer:    answers["cmp-broken"],
		First: Turn{
			Prompt: "Shopper: cmp-broken\nRecommend one size for the Everyday oxford.",
			Reply:  "Recommend L: the torn-seam refund means Brand C runs small.",
			Size:   "L",
			Cites:  []string{"seed-7-cmp-broken-008"},
		},
	}
	score, err := ScoreRun(run)
	if err != nil {
		t.Fatalf("score wrong-cause run: %v", err)
	}
	if score.WrongCause != 1 {
		t.Errorf("wrong-cause run: WrongCause = %d, want 1", score.WrongCause)
	}
	if score.RightSize {
		t.Error("wrong-cause run: RightSize = true, want false (answer is M)")
	}
	if score.Supported != 0 || score.Cited != 1 {
		t.Errorf("wrong-cause run: Supported/Cited = %d/%d, want 0/1", score.Supported, score.Cited)
	}
	if got := score.Cites[0].Kind; string(got) != "seller_reliability" {
		t.Errorf("wrong-cause cite kind = %q, want seller_reliability", string(got))
	}
}

func TestNonFitCiteWithoutSizeChangeIsExclusionNotError(t *testing.T) {
	histories, answers := loadFixtures(t)
	run := Run{
		ShopperID: "cmp-broken",
		History:   histories["cmp-broken"],
		Answer:    answers["cmp-broken"],
		First: Turn{
			Prompt: "Shopper: cmp-broken\nRecommend one size for the Everyday oxford.",
			Reply:  "Recommend M: the torn-seam refund is damage, not fit, so kept M stands.",
			Size:   "M",
			Cites:  []string{"torn seam"},
		},
	}
	score, err := ScoreRun(run)
	if err != nil {
		t.Fatalf("score exclusion run: %v", err)
	}
	if !score.RightSize {
		t.Error("exclusion run: RightSize = false, want true")
	}
	if score.WrongCause != 0 {
		t.Errorf("exclusion run: WrongCause = %d, want 0: holding the size on a non-fit cite is an exclusion, not an error", score.WrongCause)
	}
	if !score.Cites[0].Resolved {
		t.Errorf("exclusion run: note-substring cite unresolved: %q", score.Cites[0].Reason)
	}
}

func TestCorrectionHonouredScoresAsExpected(t *testing.T) {
	histories, answers := loadFixtures(t)
	run := goodArm3Run(histories, answers)
	run.Correction = &Correction{Text: "I've lost weight, use M", WantSize: "M"}
	second := Turn{
		Prompt: "Shopper: cmp-size-changed\nThe shopper corrects: use M. Recommend one size.",
		Reply:  "Recommend M per your correction.",
		Size:   "M",
	}
	run.Second = &second
	score, err := ScoreRun(run)
	if err != nil {
		t.Fatalf("score honoured run: %v", err)
	}
	if score.CorrectionHonoured == nil || !*score.CorrectionHonoured {
		t.Errorf("honoured run: CorrectionHonoured = %v, want true", score.CorrectionHonoured)
	}
	if score.TokensSecond <= 0 {
		t.Error("honoured run: second-turn tokens not counted")
	}
}

func TestCorrectionIgnoredScoresAsExpected(t *testing.T) {
	histories, answers := loadFixtures(t)
	run := goodArm3Run(histories, answers)
	run.Correction = &Correction{Text: "I've lost weight, use M", WantSize: "M"}
	second := Turn{
		Prompt: "Shopper: cmp-size-changed\nThe shopper corrects: use M. Recommend one size.",
		Reply:  "Recommend L: the evidence still says L.",
		Size:   "L",
		Cites:  []string{"seed-7-cmp-size-changed-014"},
	}
	run.Second = &second
	score, err := ScoreRun(run)
	if err != nil {
		t.Fatalf("score ignored run: %v", err)
	}
	if score.CorrectionHonoured == nil || *score.CorrectionHonoured {
		t.Errorf("ignored run: CorrectionHonoured = %v, want false", score.CorrectionHonoured)
	}
}

func TestBogusCitationScoresAsExpected(t *testing.T) {
	histories, answers := loadFixtures(t)
	run := goodArm3Run(histories, answers)
	run.First.Cites = []string{"seed-7-cmp-size-changed-014", "RFD-no-such-refund"}
	score, err := ScoreRun(run)
	if err != nil {
		t.Fatalf("score bogus-cite run: %v", err)
	}
	if score.Supported != 1 || score.Cited != 2 {
		t.Errorf("bogus-cite run: Supported/Cited = %d/%d, want 1/2", score.Supported, score.Cited)
	}
	if score.Cites[1].Resolved {
		t.Error("bogus-cite run: invented ref resolves, want unresolved")
	}
	if !score.RightSize {
		t.Error("bogus-cite run: RightSize = false, want true (size is still L)")
	}
}

func TestAmbiguousCitationIsUnsupported(t *testing.T) {
	histories, answers := loadFixtures(t)
	run := goodArm3Run(histories, answers)
	run.First.Cites = []string{"Brand F oxford"}
	score, err := ScoreRun(run)
	if err != nil {
		t.Fatalf("score ambiguous-cite run: %v", err)
	}
	if score.Cites[0].Resolved {
		t.Error("ambiguous cite: resolves, want unresolved")
	}
	if score.Supported != 0 {
		t.Errorf("ambiguous cite: Supported = %d, want 0", score.Supported)
	}
}

func TestTokenBlowoutScoresAsExpected(t *testing.T) {
	histories, answers := loadFixtures(t)
	run := goodArm3Run(histories, answers)
	run.First.Reply = strings.Repeat("word ", 5000)
	score, err := ScoreRun(run)
	if err != nil {
		t.Fatalf("score blowout run: %v", err)
	}
	if !score.OverCap {
		t.Errorf("blowout run: OverCap = false with %d tokens, want true", score.TokensFirst)
	}
	if score.TokenBasis != TokenBasis {
		t.Errorf("blowout run: TokenBasis = %q, want recorded basis", score.TokenBasis)
	}
}

func TestTokensReuseHarnessCounter(t *testing.T) {
	histories, answers := loadFixtures(t)
	run := goodArm3Run(histories, answers)
	score, err := ScoreRun(run)
	if err != nil {
		t.Fatalf("score run: %v", err)
	}
	want := harness.CountTokens(run.First.Prompt) + harness.CountTokens(run.First.Reply)
	if score.TokensFirst != want {
		t.Errorf("TokensFirst = %d, want harness.CountTokens sum %d", score.TokensFirst, want)
	}
}

func TestFitFeedbackCountsAsFitSupport(t *testing.T) {
	histories, _ := loadFixtures(t)
	history := histories["cmp-size-changed"]
	row, ok, _ := matchCite(history, "seed-7-cmp-size-changed-013")
	if !ok {
		t.Fatal("feedback row does not resolve by CustomID")
	}
	if got := supportKind(row); string(got) != "fit" {
		t.Errorf("feedback supportKind = %q, want fit: the layer counts feedback by verdict", string(got))
	}
}

func TestRunValidation(t *testing.T) {
	histories, answers := loadFixtures(t)
	good := goodArm3Run(histories, answers)

	empty := good
	empty.History = seed.History{ShopperID: "cmp-size-changed"}
	if _, err := ScoreRun(empty); !errors.Is(err, ErrNoHistories) {
		t.Errorf("empty history: err = %v, want ErrNoHistories", err)
	}

	mismatch := good
	mismatch.Answer = answers["cmp-broken"]
	if _, err := ScoreRun(mismatch); !errors.Is(err, ErrAnswerMismatch) {
		t.Errorf("answer mismatch: err = %v, want ErrAnswerMismatch", err)
	}

	missing := good
	missing.Correction = &Correction{Text: "use M", WantSize: "M"}
	if _, err := ScoreRun(missing); !errors.Is(err, ErrNoSecondTurn) {
		t.Errorf("correction without second: err = %v, want ErrNoSecondTurn", err)
	}

	dangling := good
	second := Turn{Prompt: "p", Reply: "Recommend M.", Size: "M"}
	dangling.Second = &second
	if _, err := ScoreRun(dangling); err == nil {
		t.Error("second without correction: err = nil, want an error")
	}
}

func TestHiddenAnswersShape(t *testing.T) {
	_, answers := loadFixtures(t)
	if len(answers) != 5 {
		t.Fatalf("hidden answers = %d shoppers, want 5", len(answers))
	}
	if answers["cmp-broken"].RightSize != "M" {
		t.Errorf("cmp-broken RightSize = %q, want M", answers["cmp-broken"].RightSize)
	}
}

// TestGraderNeverTouchesNetwork pins the offline rule statically: the
// grader's non-test sources must not import network, key or cloud
// packages. The hidden answers are the one exception: the grader scores
// against them, unlike the harness.
func TestGraderNeverTouchesNetwork(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list grader sources: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, banned := range []string{"net/http", "\"net\"", "openrouter", "elastic", "api_key", "secret-tool", "OPENROUTER"} {
			if strings.Contains(string(raw), banned) {
				t.Errorf("%s references %q: grader code must run fully offline", file, banned)
			}
		}
	}
}
