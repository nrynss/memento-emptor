package memory

import (
	"context"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// The concept kinds written through Derive. A belief derives as logic so
// structural evidence can promote it; a statement or an inferred
// observation derives as a note, which never promotes however often it is
// repeated.
const (
	// ConceptBelief is the derive kind for a belief that evidence can earn.
	ConceptBelief = "logic"
	// ConceptNote is the derive kind for assertions and weak observations.
	// Notes never promote.
	ConceptNote = "observation"
)

// Insight is the graph's verdict on one belief: its promotion state, the
// evidence behind it, and the contradiction warning when one applies. A
// contradicted belief keeps its state; the warning rides beside it.
type Insight struct {
	// State is the promotion state: None, Candidate or Venerable. It
	// never reads contradicted; that overlay is Contradicted beside it.
	State domain.GraphState
	// Contradicted is true while a correction declared against the
	// belief applies.
	Contradicted bool
	// Dependents counts the structural evidence citing the belief.
	Dependents int
	// DistinctInteractions counts the distinct event-time days behind it.
	DistinctInteractions int
	// Coverage is the dependent span over the session's time extent.
	Coverage float64
	// Warning renders the contradiction, dated by the correction.
	Warning string
}

// Graph is the evidence-graph seam the writer codes against: short keyed
// beliefs, evidence-gated promotion, contradiction flags, and session
// isolation. It mirrors the fakes' EvidenceGraph so the real graph client
// implements either one; the adapter lives with the caller.
type Graph interface {
	// Derive records one belief or statement. Notes never promote,
	// however often repeated.
	Derive(ctx context.Context, content, kind string, eventTime time.Time) error
	// RecordAction records one structural write. DependsOn cites the
	// beliefs the write counts toward; Against declares the beliefs it
	// contradicts, applied at once while they stay standing.
	RecordAction(ctx context.Context, action string, dependsOn, against []string, eventTime time.Time) error
	// Inspect reports the promotion state of one belief.
	Inspect(ctx context.Context, content string) (Insight, error)
}
