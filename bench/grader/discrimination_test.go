package grader

import (
	"testing"
)

// TestHonouredCorrectionWithBogusCiteScoresSplit pins that the correction
// and citation measures stay separate: honouring the correction does not
// excuse an invented cite, and the invented cite withholds support without
// counting as a wrong cause.
func TestHonouredCorrectionWithBogusCiteScoresSplit(t *testing.T) {
	histories, answers := loadFixtures(t)
	run := goodArm3Run(histories, answers)
	run.Correction = &Correction{Text: "I've lost weight, use M", WantSize: "M"}
	second := Turn{
		Prompt: "Shopper: cmp-size-changed\nThe shopper corrects: use M. Recommend one size.",
		Reply:  "Recommend M per your correction, see RFD-invented.",
		Size:   "M",
		Cites:  []string{"RFD-invented"},
	}
	run.Second = &second
	score, err := ScoreRun(run)
	if err != nil {
		t.Fatalf("score honoured-with-bogus-cite run: %v", err)
	}
	if score.CorrectionHonoured == nil || !*score.CorrectionHonoured {
		t.Errorf("CorrectionHonoured = %v, want true: the size is honoured even though the cite is bogus", score.CorrectionHonoured)
	}
	if score.Supported != 2 || score.Cited != 3 {
		t.Errorf("Supported/Cited = %d/%d, want 2/3: the invented second-turn cite is unsupported", score.Supported, score.Cited)
	}
	if score.WrongCause != 0 {
		t.Errorf("WrongCause = %d, want 0: an unresolved cite never counts as a wrong cause", score.WrongCause)
	}
}

// TestRightSizeForWrongReasonScoresSplit pins that a right size does not
// excuse a wrong cause: recommending the hidden right size off a non-fit
// cite that moves off the baseline scores RightSize true and WrongCause 1.
func TestRightSizeForWrongReasonScoresSplit(t *testing.T) {
	histories, answers := loadFixtures(t)
	run := Run{
		ShopperID: "cmp-size-changed",
		History:   histories["cmp-size-changed"],
		Answer:    answers["cmp-size-changed"],
		First: Turn{
			Prompt: "Shopper: cmp-size-changed\nRecommend one size for the Heritage oxford.",
			Reply:  "Recommend L: the usual-size statement says so.",
			Size:   "L",
			Cites:  []string{"usually size M"},
		},
	}
	score, err := ScoreRun(run)
	if err != nil {
		t.Fatalf("score right-size-wrong-reason run: %v", err)
	}
	if !score.RightSize {
		t.Error("RightSize = false, want true: L is the hidden right size")
	}
	if score.Supported != 0 || score.Cited != 1 {
		t.Errorf("Supported/Cited = %d/%d, want 0/1: the statement counts toward nothing", score.Supported, score.Cited)
	}
	if score.WrongCause != 1 {
		t.Errorf("WrongCause = %d, want 1: a non-fit cite behind a move off the baseline is a wrong cause even when the size is right", score.WrongCause)
	}
}
