package fakes

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// Promotion bars replayed from the seeded-session research. Three
// interactions citing the belief take it to Venerable when their span
// covers at least this share of the session's time extent; an edge younger
// than the floor does not count, which is why the story dates every write
// in event time.
const (
	// venerableDependents is the distinct-interaction bar for Venerable.
	venerableDependents = 3
	// venerableCoverage is the time-extent bar for Venerable.
	venerableCoverage = 0.3
	// candidateSweeps is the GC-sweep bar for Candidate. The file cadence
	// sweeps on every mutation, so one write is one sweep here.
	candidateSweeps = 3
	// candidateInteractions is the distinct-interaction bar for Candidate.
	candidateInteractions = 2
)

// BeliefReport is the graph's verdict on one belief: its promotion state,
// the evidence behind it, and the contradiction warning when one applies.
// A contradicted belief keeps its state; the warning rides beside it.
type BeliefReport struct {
	// Content is the belief's exact content.
	Content string
	// State is the promotion state: None, Candidate or Venerable. It
	// never reads contradicted; that overlay is Contradicted beside it.
	State domain.GraphState
	// Dependents counts the structural evidence citing the belief.
	Dependents int
	// DistinctInteractions counts the distinct event-time days behind it.
	DistinctInteractions int
	// Coverage is the dependent span over the session's time extent.
	Coverage float64
	// Contradicted is true while a correction declared against the
	// belief applies.
	Contradicted bool
	// Warning renders the contradiction, dated by the correction.
	Warning string
}

// EvidenceGraph is the seam the real graph writer implements in P2: short
// keyed beliefs, evidence-gated promotion, contradiction flags, and
// session isolation. Every content must be covered by the committed story
// or the seeded transcript set; anything else fails with ErrNoRecording.
type EvidenceGraph interface {
	// Derive records one belief or statement. Statements never promote,
	// however often repeated.
	Derive(ctx context.Context, content, kind string, eventTime time.Time) error
	// RecordAction records one structural write. DependsOn cites the
	// beliefs the write counts toward; Against declares the beliefs it
	// contradicts, applied at once while they stay standing.
	RecordAction(ctx context.Context, action string, dependsOn, against []string, eventTime time.Time) error
	// Recall replays the recorded hits for one recorded query.
	Recall(ctx context.Context, query string) ([]string, error)
	// Inspect reports the promotion state of one belief.
	Inspect(ctx context.Context, content string) (BeliefReport, error)
}

// RecordingEvidenceGraph is an EvidenceGraph that applies the researched
// promotion bars in memory. The allowed contents are the committed
// story's plus the seeded transcript's; every other content fails with
// ErrNoRecording.
type RecordingEvidenceGraph struct {
	allowed  map[string]bool
	concepts map[string]graphConcept
	actions  []graphAction
	sweeps   int
	recalls  map[string][]string
}

type graphConcept struct {
	kind      string
	eventTime time.Time
}

type graphAction struct {
	content   string
	dependsOn []string
	against   []string
	eventTime time.Time
}

// NewRecordingEvidenceGraph builds the graph fake over an allowlist of
// exact contents and a table of recorded recall hits. Recall serves only
// covered queries; anything else fails with ErrNoRecording.
func NewRecordingEvidenceGraph(allowed []string, recalls map[string][]string) *RecordingEvidenceGraph {
	set := make(map[string]bool, len(allowed))
	for _, content := range allowed {
		set[content] = true
	}
	table := make(map[string][]string, len(recalls))
	for query, hits := range recalls {
		out := make([]string, len(hits))
		copy(out, hits)
		table[query] = out
	}
	return &RecordingEvidenceGraph{
		allowed:  set,
		concepts: map[string]graphConcept{},
		recalls:  table,
	}
}

// checkContent fails with ErrNoRecording when the content is covered by
// nothing recorded or committed.
func (f *RecordingEvidenceGraph) checkContent(content string) error {
	if !f.allowed[content] {
		return fmt.Errorf("%w: graph content %q", ErrNoRecording, content)
	}
	return nil
}

// Derive records one belief or statement, or ErrNoRecording for
// uncovered content.
func (f *RecordingEvidenceGraph) Derive(ctx context.Context, content, kind string, eventTime time.Time) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fakes: derive %q: %w", content, err)
	}
	if err := f.checkContent(content); err != nil {
		return err
	}
	f.concepts[content] = graphConcept{kind: kind, eventTime: eventTime}
	f.sweeps++
	return nil
}

// RecordAction records one structural write citing its beliefs, or
// ErrNoRecording for uncovered content on either side of the citation.
func (f *RecordingEvidenceGraph) RecordAction(ctx context.Context, action string, dependsOn, against []string, eventTime time.Time) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fakes: record action %q: %w", action, err)
	}
	if err := f.checkContent(action); err != nil {
		return err
	}
	for _, cited := range dependsOn {
		if err := f.checkContent(cited); err != nil {
			return err
		}
		if _, ok := f.concepts[cited]; !ok {
			return fmt.Errorf("%w: depends_on %q was never derived", ErrNoRecording, cited)
		}
	}
	for _, cited := range against {
		if err := f.checkContent(cited); err != nil {
			return err
		}
		if _, ok := f.concepts[cited]; !ok {
			return fmt.Errorf("%w: against %q was never derived", ErrNoRecording, cited)
		}
	}
	f.actions = append(f.actions, graphAction{content: action, dependsOn: dependsOn, against: against, eventTime: eventTime})
	f.sweeps++
	return nil
}

// Recall replays the recorded hits for one recorded query, or
// ErrNoRecording for an uncovered query.
func (f *RecordingEvidenceGraph) Recall(ctx context.Context, query string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fakes: recall %q: %w", query, err)
	}
	hits, ok := f.recalls[query]
	if !ok {
		return nil, fmt.Errorf("%w: recall query %q", ErrNoRecording, query)
	}
	out := make([]string, len(hits))
	copy(out, hits)
	return out, nil
}

// Inspect reports the promotion state of one belief from the researched
// bars: three distinct citing interactions covering at least 0.3 of the
// session's extent reach Venerable; a declared contradiction warns beside
// the kept state; assertions never promote.
func (f *RecordingEvidenceGraph) Inspect(_ context.Context, content string) (BeliefReport, error) {
	concept, ok := f.concepts[content]
	if !ok {
		return BeliefReport{}, fmt.Errorf("%w: inspect %q was never derived", ErrNoRecording, content)
	}
	var dependents []graphAction
	for _, action := range f.actions {
		for _, cited := range action.dependsOn {
			if cited == content {
				dependents = append(dependents, action)
				break
			}
		}
	}
	days := map[string]bool{}
	var earliest, latest time.Time
	for _, action := range dependents {
		day := action.eventTime.UTC().Format("2006-01-02")
		days[day] = true
		if earliest.IsZero() || action.eventTime.Before(earliest) {
			earliest = action.eventTime
		}
		if latest.IsZero() || action.eventTime.After(latest) {
			latest = action.eventTime
		}
	}
	var sessionStart, sessionEnd time.Time
	consider := func(moment time.Time) {
		if moment.IsZero() {
			return
		}
		if sessionStart.IsZero() || moment.Before(sessionStart) {
			sessionStart = moment
		}
		if sessionEnd.IsZero() || moment.After(sessionEnd) {
			sessionEnd = moment
		}
	}
	for _, other := range f.concepts {
		consider(other.eventTime)
	}
	for _, action := range f.actions {
		consider(action.eventTime)
	}
	coverage := 0.0
	if len(dependents) > 0 && !sessionEnd.Equal(sessionStart) {
		coverage = latest.Sub(earliest).Hours() / sessionEnd.Sub(sessionStart).Hours()
	}
	report := BeliefReport{
		Content:              content,
		State:                domain.GraphNone,
		Dependents:           len(dependents),
		DistinctInteractions: len(days),
		Coverage:             coverage,
	}
	if concept.kind != "observation" {
		if len(days) >= venerableDependents && coverage >= venerableCoverage {
			report.State = domain.GraphVenerable
		} else if f.sweeps >= candidateSweeps && len(days) >= candidateInteractions {
			report.State = domain.GraphCandidate
		}
	}
	var warnings []string
	for _, action := range f.actions {
		for _, cited := range action.against {
			if cited == content {
				warnings = append(warnings, action.content)
			}
		}
	}
	sort.Strings(warnings)
	if len(warnings) > 0 {
		report.Contradicted = true
		report.Warning = "Contradicted by " + strings.Join(warnings, "; ")
	}
	return report, nil
}
