package domain

import "testing"

// TestRivalBirthThresholdMirrorsEarnedBar pins the user decision: three
// consecutive opposite-direction returns birth the rival, mirroring the
// three returns that earn a belief.
func TestRivalBirthThresholdMirrorsEarnedBar(t *testing.T) {
	if RivalBirthThreshold != 3 {
		t.Fatalf("rival birth threshold is %d, want 3", RivalBirthThreshold)
	}
}

// TestOppositeStreakBirthsAtExactlyThree proves the counting: the first
// two opposites report no birth, the third reports birth and consumes the
// run, and the next run births again at three.
func TestOppositeStreakBirthsAtExactlyThree(t *testing.T) {
	var streak OppositeStreak
	if streak.NoteOpposite() {
		t.Fatal("first opposite reported birth, want no birth below 3")
	}
	if streak.NoteOpposite() {
		t.Fatal("second opposite reported birth, want no birth below 3")
	}
	if got := streak.Count(); got != 2 {
		t.Fatalf("streak count is %d, want 2", got)
	}
	if !streak.NoteOpposite() {
		t.Fatal("third consecutive opposite reported no birth, want birth at 3")
	}
	if got := streak.Count(); got != 0 {
		t.Fatalf("birth left count %d, want 0: the run is consumed", got)
	}
	var second OppositeStreak
	second.NoteOpposite()
	second.NoteOpposite()
	if !second.NoteOpposite() {
		t.Fatal("next run of three reported no birth, want birth at 3 again")
	}
}

// TestOppositeStreakResetsOnMatchedReturn proves the interruption rule: a
// same-direction return breaks the run, so opposite, opposite, matched,
// opposite, opposite reports no birth.
func TestOppositeStreakResetsOnMatchedReturn(t *testing.T) {
	var streak OppositeStreak
	streak.NoteOpposite()
	streak.NoteOpposite()
	streak.NoteMatched()
	if got := streak.Count(); got != 0 {
		t.Fatalf("matched return left count %d, want 0", got)
	}
	if streak.NoteOpposite() {
		t.Fatal("first opposite after reset reported birth")
	}
	if streak.NoteOpposite() {
		t.Fatal("opposite, opposite, matched, opposite, opposite reported birth, want none")
	}
	if got := streak.Count(); got != 2 {
		t.Fatalf("streak count is %d, want 2 after the reset run", got)
	}
}
