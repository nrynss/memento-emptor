package domain

import "strings"

// SizeTally is how often one size label was kept or said to fit well. The
// usual-size rule reads the label the shopper actually wore across every
// brand; it never maps one brand's size onto another's.
type SizeTally struct {
	// Size is the size label, such as M.
	Size string
	// Kept is how many purchases of this size were kept past the return
	// window with no feedback.
	Kept int
	// FitsWell is how many purchases of this size got fit feedback saying
	// it fits.
	FitsWell int
}

// UsualSizeBasis names where the usual size came from.
type UsualSizeBasis string

// The usual-size bases.
const (
	// UsualSizeFromOutcomes is the size most often kept or said to fit
	// well, in any brand.
	UsualSizeFromOutcomes UsualSizeBasis = "kept_fits_well"
	// UsualSizeFromStatement is the shopper's own general statement. It is
	// Claimed and never promotes.
	UsualSizeFromStatement UsualSizeBasis = "statement"
	// UsualSizeUnknown is no usual size: no outcome and no statement.
	UsualSizeUnknown UsualSizeBasis = "none"
)

// UsualSize is the size recall adjusts the pooled belief from. It is never
// mapped across brands: only the size the shopper wore is carried.
type UsualSize struct {
	// Size is the size label, or empty when there is no usual size.
	Size string
	// Basis names what the size came from.
	Basis UsualSizeBasis
	// Outcomes is how many kept or fits-well purchases back an
	// outcome-derived size, and zero for the statement and unknown bases.
	Outcomes int
}

// UsualSizeFor applies the usual-size rule: the size most often kept or
// said to fit well in any brand; failing that, the size the layer read
// from the shopper's general statement; failing that, no size. Two sizes
// level at the top are not "most often", so a tie falls through to the
// statement.
func UsualSizeFor(tallies []SizeTally, statedSize string) UsualSize {
	best := SizeTally{}
	tied := false
	for _, tally := range tallies {
		count := tally.Kept + tally.FitsWell
		if count <= 0 || tally.Size == "" {
			continue
		}
		switch top := best.Kept + best.FitsWell; {
		case count > top:
			best = tally
			tied = false
		case count == top && tally.Size != best.Size:
			tied = true
		}
	}
	if best.Size != "" && !tied {
		return UsualSize{
			Size:     best.Size,
			Basis:    UsualSizeFromOutcomes,
			Outcomes: best.Kept + best.FitsWell,
		}
	}
	if stated := strings.TrimSpace(statedSize); stated != "" {
		return UsualSize{Size: stated, Basis: UsualSizeFromStatement}
	}
	return UsualSize{Basis: UsualSizeUnknown}
}
