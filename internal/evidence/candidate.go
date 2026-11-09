package evidence

import "strings"

// Candidate is a new product the agent considers recommending: the line
// it belongs to, backed by the catalog's own words.
type Candidate struct {
	// Line is the cut the product belongs to: brand, category and the
	// category-specific fit handle.
	Line Line
	// SKU identifies the product variant in the catalog.
	SKU string
	// Title is the catalog title.
	Title string
	// Description is the catalog description, which usually states the fit
	// in words.
	Description string
}

// Text returns the catalog's own words for the candidate: the title backed
// by the description. The structural line gate decides the match; this
// text is the lexical leg behind it.
func (c Candidate) Text() string {
	return strings.Join(strings.Fields(strings.TrimSpace(c.Title+" "+c.Description)), " ")
}

// MatchReport answers whether a new product matches the items behind a
// belief: it shares their cut, using what the catalog data supports.
type MatchReport struct {
	// Candidate is the product the question was asked about.
	Candidate Candidate
	// Matches is true when the candidate shares a line with at least one
	// of the belief's evidence documents.
	Matches bool
	// MatchedDocIDs are the belief documents sharing the candidate's
	// line, in the order given.
	MatchedDocIDs []string
}

// MatchCandidate reports whether the candidate shares a line with any of
// the belief's evidence documents: same brand, same category, same fit
// handle. Documents with no line — bare shopper messages — never match:
// the line name alone proves nothing, and a message alone names no line.
func MatchCandidate(candidate Candidate, docs []Document) MatchReport {
	report := MatchReport{Candidate: candidate}
	for _, doc := range docs {
		if doc.DocID == "" {
			continue
		}
		if doc.Line.Matches(candidate) {
			report.Matches = true
			report.MatchedDocIDs = append(report.MatchedDocIDs, doc.DocID)
		}
	}
	return report
}
