package memory

import (
	"strings"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// SameSubject reports whether a correction and a belief are about the same
// subject, so the correction is declared against the belief when the belief
// is written later. It reads the domain's shapes only — the subject kind
// and id, the belief kind, and the product line — and never the
// correction's words or the belief's value:
//
//   - Both sides must name a subject, with the same kind (brand or seller)
//     and the same id.
//   - The belief's kind must agree with its subject: a fit belief is about
//     a brand and a seller belief about a seller.
//   - The product line matches where both sides name one: the brand,
//     category and cut each compare only when both sides set them, so a
//     side that names no line spans every line of its subject.
//   - The size is not a match key. A correction and a belief that name
//     different sizes are exactly the pair a correction contradicts, so
//     requiring equal sizes would never declare the correction that
//     disagrees.
func SameSubject(c Correction, b Belief) bool {
	if !namesSubject(c.Subject) || !namesSubject(b.Subject) {
		return false
	}
	if c.Subject.Kind != b.Subject.Kind || c.Subject.ID != b.Subject.ID {
		return false
	}
	if !beliefKindAgrees(b.Kind, b.Subject.Kind) {
		return false
	}
	return lineSpans(c.Line, b.Line)
}

// namesSubject reports whether a subject is one of the layer's two: a
// brand or a seller, with an id. Anything else names nothing.
func namesSubject(s domain.Subject) bool {
	if strings.TrimSpace(s.ID) == "" {
		return false
	}
	return s.Kind == domain.SubjectBrand || s.Kind == domain.SubjectSeller
}

// beliefKindAgrees reports whether a belief kind can be about a subject
// kind: fit beliefs are about a brand, seller beliefs about a seller, and
// a belief counting toward nothing is about neither.
func beliefKindAgrees(kind domain.BeliefKind, subject domain.SubjectKind) bool {
	switch kind {
	case domain.BeliefKindFit:
		return subject == domain.SubjectBrand
	case domain.BeliefKindSeller:
		return subject == domain.SubjectSeller
	default:
		return false
	}
}

// lineSpans reports whether two product lines can be the same line, where
// an unset field names nothing and therefore spans every value.
func lineSpans(a, b domain.ProductLine) bool {
	return fieldSpans(a.Brand, b.Brand) &&
		fieldSpans(a.Category, b.Category) &&
		fieldSpans(a.FitHandle, b.FitHandle)
}

// fieldSpans reports whether two named fields agree, where an empty field
// names nothing and so agrees with anything.
func fieldSpans(a, b string) bool {
	return a == "" || b == "" || a == b
}
