package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// ErrIncompleteCorrection is returned when a correction carries no words,
// names no subject, or names a product line belonging to another brand.
// Without words and one named subject the layer cannot key the correction
// or match a belief to it later, so the write stops.
var ErrIncompleteCorrection = errors.New("memory: correction needs words and one named subject")

// ErrCorrectionDeclared is returned when a correction already declared
// against a belief is offered as a standing one. A declared correction
// keeps its declaration: it never becomes standing again.
var ErrCorrectionDeclared = errors.New("memory: correction already stands against a belief")

// ErrNoStandingJournal is returned when a standing correction is offered
// with nowhere to keep it. A standing correction that is not journalled
// would be lost, so the write stops rather than discard it.
var ErrNoStandingJournal = errors.New("memory: no journal for a standing correction")

// Correction is one shopper correction in the domain's shapes: the words
// the shopper said, the brand or seller they are about, the product line
// and size they name, and the belief they are declared against. A
// correction with no Against stands on its own: it contradicts nothing
// yet, and a matching belief written later is declared against then.
type Correction struct {
	// Text is the shopper's verbatim first-person words, and the content
	// the graph keys the correction by, exactly as a correction against
	// an existing belief is written.
	Text string
	// Subject is the brand or seller the correction is about.
	Subject domain.Subject
	// Line is the product line the correction names, or the zero line
	// when it names none. Its brand, when set, is the subject's brand.
	Line domain.ProductLine
	// Size is the size the correction names, or empty when it names
	// none. It is kept with the correction; it is not a match key (see
	// SameSubject).
	Size string
	// RecordedAt is when the shopper said it, in event time. A
	// declaration made later is dated by it, so the warning dates the
	// shopper's words.
	RecordedAt time.Time
	// Against is the belief content the correction has been declared
	// against, or empty while the correction stands on its own.
	Against string
}

// validate checks a correction names words and one subject.
func (c Correction) validate() error {
	if strings.TrimSpace(c.Text) == "" {
		return fmt.Errorf("%w: no words", ErrIncompleteCorrection)
	}
	if !namesSubject(c.Subject) {
		return fmt.Errorf("%w: kind %q id %q", ErrIncompleteCorrection, string(c.Subject.Kind), c.Subject.ID)
	}
	if c.Line.Brand != "" && c.Line.Brand != c.Subject.ID {
		return fmt.Errorf("%w: line brand %q is not subject %q", ErrIncompleteCorrection, c.Line.Brand, c.Subject.ID)
	}
	return nil
}

// Belief is one belief about to be written: the content that keys it in
// the graph, and the subject it is about in the domain's shapes.
type Belief struct {
	// Content is the belief's exact graph content.
	Content string
	// Kind is whether the belief is about fit or seller reliability.
	Kind domain.BeliefKind
	// Subject is the brand or seller the belief is about.
	Subject domain.Subject
	// Line is the product line when the belief is about one line only,
	// or the zero line when it spans its whole subject.
	Line domain.ProductLine
}

// StandingCorrections is a shopper's journal of corrections recorded
// while no belief contradicted them. The graph holds each correction as a
// note, which never promotes; the journal holds the subject and the one
// declaration, which a derive cannot carry. The caller keeps one journal
// per shopper, beside the graph, and serialises that shopper's writes: a
// journal is not safe for concurrent use.
type StandingCorrections struct {
	corrections []Correction
}

// Record writes one standing correction. It derives the shopper's words as
// a note, the kind that never promotes, so the record has no structural
// edge, no score and no canonization contribution, and adds it to the
// journal with no Against. The same words about the same subject, line and
// size are one correction: recording them again keeps the entry already
// held, standing or declared, so a repeat never adds a second declaration
// later. The same words about another line or size are their own entry.
func (j *StandingCorrections) Record(ctx context.Context, g Graph, c Correction) error {
	if j == nil {
		return ErrNoStandingJournal
	}
	if err := c.validate(); err != nil {
		return err
	}
	if c.Against != "" {
		return fmt.Errorf("%w: %q against %q", ErrCorrectionDeclared, c.Text, c.Against)
	}
	for _, existing := range j.corrections {
		if existing.Text == c.Text && existing.Subject == c.Subject && existing.Line == c.Line && existing.Size == c.Size {
			return nil
		}
	}
	if err := g.Derive(ctx, c.Text, ConceptNote, c.RecordedAt); err != nil {
		return fmt.Errorf("memory: record standing correction %q: %w", c.Text, err)
	}
	j.corrections = append(j.corrections, c)
	return nil
}

// Declare declares every standing correction about the same subject as b
// against b, once each, and reports how many it declared. A correction
// already declared is never declared again: a second matching belief
// finds it declared and leaves it alone, and a later correction is its
// own entry, so the earlier declaration is neither repeated nor revived.
// The declaration is a structural action citing the belief in against,
// never in depends_on, so it adds no evidence and the belief keeps its
// standing. The belief's content must already be written to the graph, as
// EnsureBeliefDeclaring writes it. A nil journal declares nothing.
func (j *StandingCorrections) Declare(ctx context.Context, g Graph, b Belief) (int, error) {
	if j == nil {
		return 0, nil
	}
	declared := 0
	for i := range j.corrections {
		correction := &j.corrections[i]
		if correction.Against != "" || !SameSubject(*correction, b) {
			continue
		}
		if err := structuralOp(domain.SignalCorrection); err != nil {
			return declared, err
		}
		if err := g.RecordAction(ctx, correction.Text, nil, []string{b.Content}, correction.RecordedAt); err != nil {
			return declared, fmt.Errorf("memory: declare correction %q against %q: %w", correction.Text, b.Content, err)
		}
		correction.Against = b.Content
		declared++
	}
	return declared, nil
}

// Len reports how many corrections the journal holds, standing and
// declared alike.
func (j *StandingCorrections) Len() int {
	if j == nil {
		return 0
	}
	return len(j.corrections)
}

// Standing returns the journal's corrections, oldest first, as a copy. A
// correction's Against is set once it has been declared against a belief;
// an empty Against means it still stands on its own.
func (j *StandingCorrections) Standing() []Correction {
	if j == nil {
		return nil
	}
	out := make([]Correction, len(j.corrections))
	copy(out, j.corrections)
	return out
}

// EnsureBeliefDeclaring derives one belief so structural evidence can
// promote it, then declares each standing correction about the same
// subject against it, once each, and reports how many it declared. A
// belief written through it declares a correction that was recorded
// before the belief existed, so a second belief about the same subject
// never re-declares that correction.
func EnsureBeliefDeclaring(ctx context.Context, g Graph, j *StandingCorrections, b Belief, at time.Time) (int, error) {
	if err := EnsureBelief(ctx, g, b.Content, at); err != nil {
		return 0, err
	}
	return j.Declare(ctx, g, b)
}
