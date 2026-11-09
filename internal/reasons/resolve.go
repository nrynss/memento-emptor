package reasons

import (
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// Settle maps one classification to the reason attached to the refund.
// When the choice is unclear or the confidence is under the floor it
// returns a nil reason with askAgain true: the refund stays a pending
// question and the agent asks again. A nil reason counts toward nothing,
// so neither path can turn a vague answer into a belief.
func Settle(answer string, class fakes.Classification) (reason *domain.Reason, askAgain bool) {
	if fakes.AskAgain(class) {
		return nil, true
	}
	return &domain.Reason{
		Category:   class.Category,
		Text:       answer,
		Confidence: class.Confidence,
	}, false
}
