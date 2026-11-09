package memory

import (
	"context"
	"fmt"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// Reading is what the layer remembers about one belief: the ledger status
// for display, the underlying graph standing with the overlay removed,
// and the warning inline. A contradicted belief reports status Out of
// true while standing stays Venerable: both travel together, so the
// correction wins the decision at once while the belief stands.
type Reading struct {
	// Content is the belief's exact content.
	Content string
	// Status is the ledger status. It reads Out of true while a
	// correction applies.
	Status domain.BeliefStatus
	// Standing is the underlying graph state with the overlay removed:
	// None, Candidate or Venerable. It never reads contradicted; that
	// overlay is the Contradicted flag beside it.
	Standing domain.GraphState
	// Contradicted is true while a correction applies to this belief.
	Contradicted bool
	// Dependents counts the structural evidence citing the belief.
	Dependents int
	// Warning renders the contradiction, dated by the correction.
	Warning string
}

// LedgerStatus maps a graph state plus the contradiction overlay to the
// ledger's status through the frozen round trip. A contradicted belief
// reads Out of true whatever its standing; anything else follows the
// table exactly.
func LedgerStatus(state domain.GraphState, contradicted bool) (domain.BeliefStatus, error) {
	status, err := domain.StatusForGraph(state)
	if err != nil {
		return "", err
	}
	if contradicted {
		return domain.StatusOutOfTrue, nil
	}
	return status, nil
}

// ReadBelief inspects one belief and maps it to the ledger's reading. The
// correction wins here: a contradicted belief returns Out of true with
// the warning inline, while its standing stays where the evidence earned
// it.
func ReadBelief(ctx context.Context, g Graph, content string) (Reading, error) {
	insight, err := g.Inspect(ctx, content)
	if err != nil {
		return Reading{}, fmt.Errorf("memory: read belief %q: %w", content, err)
	}
	status, err := LedgerStatus(insight.State, insight.Contradicted)
	if err != nil {
		return Reading{}, fmt.Errorf("memory: read belief %q: %w", content, err)
	}
	return Reading{
		Content:      content,
		Status:       status,
		Standing:     insight.State,
		Contradicted: insight.Contradicted,
		Dependents:   insight.Dependents,
		Warning:      insight.Warning,
	}, nil
}
