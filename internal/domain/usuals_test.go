package domain

import "testing"

// TestUsualSizeFor covers the rule: the size most often kept or said to fit
// well, in any brand; the statement when outcomes do not name one; and
// nothing when neither does.
func TestUsualSizeFor(t *testing.T) {
	cases := []struct {
		name      string
		tallies   []SizeTally
		statement string
		size      string
		basis     UsualSizeBasis
		outcomes  int
	}{
		{
			name:     "the most often kept size wins",
			tallies:  []SizeTally{{Size: "M", Kept: 2}, {Size: "L", Kept: 1}},
			size:     "M",
			basis:    UsualSizeFromOutcomes,
			outcomes: 2,
		},
		{
			name:     "kept and fits well count together",
			tallies:  []SizeTally{{Size: "M", Kept: 1, FitsWell: 1}, {Size: "L", Kept: 1}},
			size:     "M",
			basis:    UsualSizeFromOutcomes,
			outcomes: 2,
		},
		{
			name:     "the most often fits-well size wins",
			tallies:  []SizeTally{{Size: "S", FitsWell: 3}, {Size: "M", FitsWell: 1}},
			size:     "S",
			basis:    UsualSizeFromOutcomes,
			outcomes: 3,
		},
		{
			name:      "a tie falls through to the statement",
			tallies:   []SizeTally{{Size: "M", Kept: 2}, {Size: "L", Kept: 2}},
			statement: "S",
			size:      "S",
			basis:     UsualSizeFromStatement,
		},
		{
			name:    "a tie with no statement is no usual size",
			tallies: []SizeTally{{Size: "M", Kept: 2}, {Size: "L", Kept: 2}},
			basis:   UsualSizeUnknown,
		},
		{
			name:      "a statement stands when no outcome exists",
			statement: "M",
			size:      "M",
			basis:     UsualSizeFromStatement,
		},
		{
			name:  "nothing at all is no usual size",
			basis: UsualSizeUnknown,
		},
		{
			name:     "an empty size label and a zero count count for nothing",
			tallies:  []SizeTally{{Size: "", Kept: 5}, {Size: "M", Kept: 0}, {Size: "L", Kept: 1}},
			size:     "L",
			basis:    UsualSizeFromOutcomes,
			outcomes: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := UsualSizeFor(tc.tallies, tc.statement)
			if got.Size != tc.size || got.Basis != tc.basis || got.Outcomes != tc.outcomes {
				t.Errorf("UsualSizeFor(%v, %q) = %q/%q/%d, want %q/%q/%d",
					tc.tallies, tc.statement, got.Size, got.Basis, got.Outcomes,
					tc.size, tc.basis, tc.outcomes)
			}
		})
	}
}
