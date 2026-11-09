package domain

import (
	"errors"
	"fmt"
)

// Signal is one row of the evidence-strength table: what arrived and how
// strongly it counts.
type Signal string

// The signals the layer accepts.
const (
	// SignalClassifiedOutcome is a return or dispute with a classified
	// reason. It is strong: a structural edge linked to the transaction.
	SignalClassifiedOutcome Signal = "classified_outcome"
	// SignalFitFeedback is explicit fit feedback tied to a specific
	// purchase. It is strong: a structural edge linked to the transaction.
	SignalFitFeedback Signal = "fit_feedback"
	// SignalKeptPastWindow means the item was kept past the return window
	// with no feedback. It is weak and inferred: an observation only, no
	// structural edge, and it means nothing before the window closes.
	SignalKeptPastWindow Signal = "kept_past_window"
	// SignalGeneralStatement is a general statement such as being usually
	// a size M. It is an assertion: it never promotes.
	SignalGeneralStatement Signal = "general_statement"
	// SignalCorrection is the shopper's explicit, first-person correction.
	// It applies at once with precedence over the belief and is declared
	// against the belief it contradicts; the belief itself is never
	// deleted.
	SignalCorrection Signal = "correction"
)

// Signals lists every signal in the strength table.
var Signals = []Signal{
	SignalClassifiedOutcome,
	SignalFitFeedback,
	SignalKeptPastWindow,
	SignalGeneralStatement,
	SignalCorrection,
}

// WriteOp is how a signal is written to the evidence graph.
type WriteOp string

// The write operations.
const (
	// WriteRecordAction is a structural write linked to the transaction.
	// Only structural writes count as evidence toward promotion.
	WriteRecordAction WriteOp = "record_action"
	// WriteDerive is an assertion write. It never promotes, however often
	// repeated.
	WriteDerive WriteOp = "derive"
	// WriteObserve records an observation without a structural edge. It
	// never promotes.
	WriteObserve WriteOp = "observe"
)

// CanPromote reports whether the write operation can count as evidence
// toward promotion. Only structural writes can; assertions and
// observations never do.
func (o WriteOp) CanPromote() bool {
	return o == WriteRecordAction
}

// ErrUnknownSignal is returned when a signal is not in the strength table.
var ErrUnknownSignal = errors.New("domain: unknown signal")

// WriteFor maps each signal to its write operation. Statements never
// promote: only a classified outcome or feedback tied to a transaction
// is a structural write.
func WriteFor(s Signal) (WriteOp, error) {
	switch s {
	case SignalClassifiedOutcome, SignalFitFeedback, SignalCorrection:
		return WriteRecordAction, nil
	case SignalGeneralStatement:
		return WriteDerive, nil
	case SignalKeptPastWindow:
		return WriteObserve, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownSignal, string(s))
	}
}
