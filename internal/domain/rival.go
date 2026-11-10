package domain

// RivalBirthThreshold is how many consecutive opposite-direction
// classified returns birth a same-kind opposite-direction rival belief
// instead of landing unattached. It mirrors the Earned bar: three returns
// earn a belief, so three consecutive opposites earn the other way a
// hearing. Below the threshold every opposite lands unattached exactly
// as before.
const RivalBirthThreshold = 3

// OppositeStreak counts consecutive opposite-direction classified returns
// toward one rival birth for one (kind, subject). The count is runtime
// streak state, not a belief: it lives in the service's per-shopper
// state and never reaches the graph. A same-direction return breaks the
// run; a shopper correction clears every run.
type OppositeStreak struct {
	// count is the current run of consecutive opposite-direction returns.
	count int
}

// NoteOpposite records one opposite-direction return and reports whether
// the run reached the birth threshold. Reaching it consumes the run, so
// counting starts over after the birth.
func (s *OppositeStreak) NoteOpposite() bool {
	s.count++
	if s.count >= RivalBirthThreshold {
		s.count = 0
		return true
	}
	return false
}

// NoteMatched records a same-direction return: the run breaks and
// counting starts over.
func (s *OppositeStreak) NoteMatched() {
	s.count = 0
}

// Count reports the current run of consecutive opposite-direction
// returns, always below the birth threshold: reaching it consumes the
// run.
func (s *OppositeStreak) Count() int {
	return s.count
}
