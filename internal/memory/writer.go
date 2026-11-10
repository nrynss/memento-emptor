package memory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// ErrUnknownVerdict is returned when a feedback verdict is not in the
// frozen set.
var ErrUnknownVerdict = errors.New("memory: unknown feedback verdict")

// ErrWriteRuleChanged is returned when the domain's write table no longer
// maps a structural signal to a structural write. The table is frozen, so
// this means the writer and the domain disagree and the write must stop
// rather than guess.
var ErrWriteRuleChanged = errors.New("memory: write rule changed under the writer")

// ErrKindMismatch is returned when evidence is offered to a belief of a
// different kind than its reason counts toward: a non-arrival dispute
// offered to the sizing belief, or fit evidence offered to the seller
// belief. The write stops and nothing touches the graph.
var ErrKindMismatch = errors.New("memory: evidence counts toward a different belief kind")

// ErrDirectionMismatch is returned when fit evidence is offered to a fit
// belief pointing the other way: a too-large return offered to a
// runs-small belief, or a too-small return offered to a runs-large
// belief. A reason's direction must match the belief's direction to count
// toward it. The write stops and nothing touches the graph: the evidence
// neither supports nor contradicts the belief, so no against edge is
// declared here. Contradictions stay shopper-driven.
var ErrDirectionMismatch = errors.New("memory: evidence direction does not match the belief")

// VerdictCountsToward reports which belief kind explicit fit feedback
// counts toward. It reads the verdict, not the reason category: a
// fits-well verdict has no reason category yet counts toward fit as
// supporting evidence for the worn size.
func VerdictCountsToward(v contract.FeedbackVerdict) (domain.BeliefKind, error) {
	switch v {
	case contract.FeedbackFitsWell, contract.FeedbackTooSmall, contract.FeedbackTooLarge:
		return domain.BeliefKindFit, nil
	case contract.FeedbackDamagedOrDefective, contract.FeedbackNotArrived:
		return domain.BeliefKindSeller, nil
	case contract.FeedbackChangedMind, contract.FeedbackUnclear:
		return domain.BeliefKindNone, nil
	default:
		return domain.BeliefKindNone, fmt.Errorf("%w: %q", ErrUnknownVerdict, string(v))
	}
}

// EnsureBelief derives the belief content as logic so structural evidence
// can promote it. It carries no evidence itself.
func EnsureBelief(ctx context.Context, g Graph, content string, at time.Time) error {
	if err := g.Derive(ctx, content, ConceptBelief, at); err != nil {
		return fmt.Errorf("memory: ensure belief %q: %w", content, err)
	}
	return nil
}

// RecordStatement derives a general statement as a note: an assertion. It
// never promotes, however often repeated.
func RecordStatement(ctx context.Context, g Graph, content string, at time.Time) error {
	if err := g.Derive(ctx, content, ConceptNote, at); err != nil {
		return fmt.Errorf("memory: record statement %q: %w", content, err)
	}
	return nil
}

// RecordKept records a kept-past-the-window observation as a note. It is
// weak and inferred: an observation only, with no structural edge, so it
// never promotes.
func RecordKept(ctx context.Context, g Graph, content string, at time.Time) error {
	if err := g.Derive(ctx, content, ConceptNote, at); err != nil {
		return fmt.Errorf("memory: record kept %q: %w", content, err)
	}
	return nil
}

// structuralOp checks the frozen write table still maps the signal to a
// structural write before the writer touches the graph.
func structuralOp(signal domain.Signal) error {
	op, err := domain.WriteFor(signal)
	if err != nil {
		return err
	}
	if op != domain.WriteRecordAction {
		return fmt.Errorf("%w: signal %q maps to %q", ErrWriteRuleChanged, string(signal), string(op))
	}
	return nil
}

// WriteEvidence writes one reasoned outcome as its own structural write
// citing the belief it counts toward in depends_on. Evidence whose reason
// matches no belief kind — a refund the shopper never explained, a
// changed mind, an unclear answer — writes nothing to any belief and
// reports written false. Evidence offered to a belief of a different kind
// than its reason counts toward is refused with ErrKindMismatch, and
// evidence whose direction does not match the belief's direction is
// refused with ErrDirectionMismatch; both refuse without touching the
// graph, declaring nothing against the belief. The event time travels
// with the write so seeded history counts as spread out.
func WriteEvidence(ctx context.Context, g Graph, beliefContent string, kind domain.BeliefKind, beliefDir domain.FitDirection, actionContent string, ev domain.Evidence) (bool, error) {
	counts := ev.CountsToward()
	if counts == domain.BeliefKindNone {
		return false, nil
	}
	if counts != kind {
		return false, fmt.Errorf("%w: evidence counts toward %q, offered to %q", ErrKindMismatch, string(counts), string(kind))
	}
	if dir := ev.Reason.Category.Direction(); dir != domain.FitDirectionNone && beliefDir != dir {
		return false, fmt.Errorf("%w: evidence points %q, offered to a belief pointing %q", ErrDirectionMismatch, string(dir), string(beliefDir))
	}
	if err := structuralOp(domain.SignalClassifiedOutcome); err != nil {
		return false, err
	}
	if err := g.RecordAction(ctx, actionContent, []string{beliefContent}, nil, ev.OccurredAt); err != nil {
		return false, fmt.Errorf("memory: write evidence %q: %w", actionContent, err)
	}
	return true, nil
}

// WriteFeedback writes explicit fit feedback tied to a specific purchase
// as its own structural write citing the belief the verdict counts
// toward. A verdict that counts toward nothing writes nothing and reports
// written false; a verdict offered to a belief of another kind is refused
// with ErrKindMismatch.
func WriteFeedback(ctx context.Context, g Graph, beliefContent string, kind domain.BeliefKind, actionContent string, verdict contract.FeedbackVerdict, at time.Time) (bool, error) {
	counts, err := VerdictCountsToward(verdict)
	if err != nil {
		return false, err
	}
	if counts == domain.BeliefKindNone {
		return false, nil
	}
	if counts != kind {
		return false, fmt.Errorf("%w: feedback counts toward %q, offered to %q", ErrKindMismatch, string(counts), string(kind))
	}
	if err := structuralOp(domain.SignalFitFeedback); err != nil {
		return false, err
	}
	if err := g.RecordAction(ctx, actionContent, []string{beliefContent}, nil, at); err != nil {
		return false, fmt.Errorf("memory: write feedback %q: %w", actionContent, err)
	}
	return true, nil
}

// RecordCorrection declares the shopper's explicit correction against the
// belief it contradicts. The correction applies at once with precedence
// in the read path while the belief stays standing: the write cites the
// belief in against, never in depends_on, so it adds no evidence to the
// belief it contradicts. A correction with no belief to name is a standing
// correction: StandingCorrections.Record keeps it until EnsureBeliefDeclaring
// writes the belief it matches.
func RecordCorrection(ctx context.Context, g Graph, correctionContent, beliefContent string, at time.Time) error {
	if err := structuralOp(domain.SignalCorrection); err != nil {
		return err
	}
	if err := g.RecordAction(ctx, correctionContent, nil, []string{beliefContent}, at); err != nil {
		return fmt.Errorf("memory: record correction %q: %w", correctionContent, err)
	}
	return nil
}
