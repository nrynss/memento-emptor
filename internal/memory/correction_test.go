package memory_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/memory"
)

// The standing-correction fixture's graph contents. The story's contents
// are the base; the later beliefs, the rival belief and the later
// correction are this fixture's own additions, allowlisted the way a
// recording covers them.
const (
	// beliefRunsSmallLater is the belief written after the correction has
	// already been recorded, about the same brand.
	beliefRunsSmallLater = "runs small in the shoulders, take size L"
	// beliefRunsLargeRival is a second belief about the same brand,
	// pointing the other way: a second match that must not re-declare.
	beliefRunsLargeRival = "runs large across the chest, take size S"
	// beliefRunsLargerLater is the belief written after the later
	// correction, so that correction declares exactly once.
	beliefRunsLargerLater = "runs very small, take size XL"
	// correctionGrew is a correction recorded after the first one has been
	// declared. It must not revive the first declaration.
	correctionGrew = "I've grown, use size XL"
)

// graphCall is one write the writer made through the graph seam.
type graphCall struct {
	content   string
	kind      string
	dependsOn []string
	against   []string
	at        time.Time
}

// countingGraph records every derive and structural write, so a fixture
// test can prove a standing correction has no structural edge and a
// declaration happens exactly once.
type countingGraph struct {
	memory.Graph
	derives []graphCall
	actions []graphCall
}

func (c *countingGraph) Derive(ctx context.Context, content, kind string, at time.Time) error {
	c.derives = append(c.derives, graphCall{content: content, kind: kind, at: at})
	return c.Graph.Derive(ctx, content, kind, at)
}

func (c *countingGraph) RecordAction(ctx context.Context, action string, dependsOn, against []string, at time.Time) error {
	c.actions = append(c.actions, graphCall{content: action, dependsOn: dependsOn, against: against, at: at})
	return c.Graph.RecordAction(ctx, action, dependsOn, against, at)
}

// standingGraph builds the writer's graph over the story's contents plus
// the standing-correction fixture's own.
func standingGraph() *countingGraph {
	allowed := append(append([]string{}, fakes.GraphContents()...),
		beliefRunsSmallLater, beliefRunsLargeRival, beliefRunsLargerLater, correctionGrew)
	return &countingGraph{Graph: adapter{inner: fakes.NewRecordingEvidenceGraph(allowed, fakes.StoryRecalls())}}
}

// brandX is the subject every standing-correction case is about.
func brandX() domain.Subject {
	return domain.Subject{Kind: domain.SubjectBrand, ID: "brand-x-001", Name: "Brand X"}
}

// shirtsRegular is the product line the correction names. Its category and
// cut are the only line fields the match reads; the brand rides on the
// subject.
var shirtsRegular = domain.ProductLine{Brand: "brand-x-001", Category: "shirts", FitHandle: "regular"}

// earnBelief replays the three documented small returns against one
// belief, so a test starts from an Earned belief of its own content.
func earnBelief(t *testing.T, ctx context.Context, g memory.Graph, content string) {
	t.Helper()
	seededAt := fakes.OccurredAt("WH-2EV-01")
	if err := memory.EnsureBelief(ctx, g, content, seededAt); err != nil {
		t.Fatalf("ensure belief %q: %v", content, err)
	}
	for i, ret := range []struct {
		event  string
		action string
		text   string
	}{
		{"WH-2EV-01", fakes.ReturnOne, "too tight in the shoulders"},
		{"WH-2EV-02", fakes.ReturnTwo, "too tight across the chest"},
		{"WH-2EV-03", fakes.ReturnThree, "sleeves too short in M"},
	} {
		written, err := memory.WriteEvidence(ctx, g, content, domain.BeliefKindFit, domain.FitDirectionSmall, ret.action, fitReturn(fakes.OccurredAt(ret.event), ret.text))
		if err != nil {
			t.Fatalf("write return %d: %v", i+1, err)
		}
		if !written {
			t.Fatalf("write return %d reported nothing written", i+1)
		}
	}
	reading := readStatus(t, ctx, g, content)
	if reading.Status != domain.StatusEarned || reading.Dependents != 3 {
		t.Fatalf("earned belief %q reads status %q dependents %d, want Earned 3", content, reading.Status, reading.Dependents)
	}
}

// TestStandingCorrectionStandsThenDeclaresOnce walks the standing
// correction's whole life: recorded before any belief about its subject,
// never promoted, declared once when the matching belief arrives, left
// alone by a second matching belief, and not revived by a later
// correction.
func TestStandingCorrectionStandsThenDeclaresOnce(t *testing.T) {
	ctx := context.Background()
	g := standingGraph()
	journal := &memory.StandingCorrections{}
	correctionAt := mustParse(t, "2026-11-05T14:02:11Z")
	correction := memory.Correction{
		Text:       fakes.CorrectionLostWeight,
		Subject:    brandX(),
		Line:       shirtsRegular,
		Size:       "M",
		RecordedAt: correctionAt,
	}

	// (a) The correction arrives before any belief about its subject.
	if err := journal.Record(ctx, g, correction); err != nil {
		t.Fatalf("record standing correction: %v", err)
	}
	if len(g.actions) != 0 {
		t.Fatalf("standing correction wrote %d structural edges, want 0", len(g.actions))
	}
	if len(g.derives) != 1 || g.derives[0].content != correction.Text || g.derives[0].kind != memory.ConceptNote {
		t.Fatalf("standing correction derives %+v, want one %q note", g.derives, memory.ConceptNote)
	}
	standing := readStatus(t, ctx, g, correction.Text)
	if standing.Standing != domain.GraphNone || standing.Status != domain.StatusClaimed || standing.Dependents != 0 || standing.Contradicted {
		t.Errorf("standing correction reads standing %q status %q dependents %d contradicted %v, want None Claimed 0 false",
			standing.Standing, standing.Status, standing.Dependents, standing.Contradicted)
	}
	if standing.Warning != "" {
		t.Errorf("standing correction carries warning %q, want none", standing.Warning)
	}
	if got := journal.Standing(); len(got) != 1 || got[0].Against != "" {
		t.Fatalf("journal holds %+v, want one correction with no against", got)
	}

	// (b) The matching belief arrives later and the correction is declared
	// against it once.
	declared, err := memory.EnsureBeliefDeclaring(ctx, g, journal, memory.Belief{
		Content: beliefRunsSmallLater,
		Kind:    domain.BeliefKindFit,
		Subject: brandX(),
	}, correctionAt)
	if err != nil {
		t.Fatalf("write matching belief: %v", err)
	}
	if declared != 1 {
		t.Fatalf("matching belief declared %d corrections, want 1", declared)
	}
	if len(g.actions) != 1 {
		t.Fatalf("matching belief wrote %d structural edges, want 1", len(g.actions))
	}
	action := g.actions[0]
	if action.content != correction.Text {
		t.Errorf("declaration writes %q, want the correction %q", action.content, correction.Text)
	}
	if len(action.dependsOn) != 0 {
		t.Errorf("declaration depends on %v, want none: it adds no evidence", action.dependsOn)
	}
	if len(action.against) != 1 || action.against[0] != beliefRunsSmallLater {
		t.Errorf("declaration is against %v, want [%q]", action.against, beliefRunsSmallLater)
	}
	if !action.at.Equal(correctionAt) {
		t.Errorf("declaration dated %s, want the correction's time %s", action.at, correctionAt)
	}
	contradicted := readStatus(t, ctx, g, beliefRunsSmallLater)
	if !contradicted.Contradicted || contradicted.Dependents != 0 || contradicted.Standing != domain.GraphNone {
		t.Errorf("matched belief reads contradicted %v dependents %d standing %q, want true 0 None",
			contradicted.Contradicted, contradicted.Dependents, contradicted.Standing)
	}
	if count := strings.Count(contradicted.Warning, correction.Text); count != 1 {
		t.Errorf("matched belief warning %q names the correction %d times, want 1", contradicted.Warning, count)
	}
	if got := journal.Standing(); got[0].Against != beliefRunsSmallLater {
		t.Errorf("journal records against %q, want %q", got[0].Against, beliefRunsSmallLater)
	}

	// (c) A second matching belief does not re-declare it.
	declared, err = memory.EnsureBeliefDeclaring(ctx, g, journal, memory.Belief{
		Content: beliefRunsLargeRival,
		Kind:    domain.BeliefKindFit,
		Subject: brandX(),
	}, mustParse(t, "2026-11-06T11:30:00Z"))
	if err != nil {
		t.Fatalf("write rival belief: %v", err)
	}
	if declared != 0 {
		t.Errorf("second matching belief declared %d corrections, want 0", declared)
	}
	if len(g.actions) != 1 {
		t.Errorf("second matching belief wrote %d structural edges, want 1 total", len(g.actions))
	}
	rival := readStatus(t, ctx, g, beliefRunsLargeRival)
	if rival.Contradicted || rival.Warning != "" {
		t.Errorf("second matching belief reads contradicted %v warning %q, want false and empty", rival.Contradicted, rival.Warning)
	}

	// A later correction does not revive or duplicate the first one's
	// declaration.
	before := readStatus(t, ctx, g, beliefRunsSmallLater)
	if err := journal.Record(ctx, g, memory.Correction{
		Text:       correctionGrew,
		Subject:    brandX(),
		Line:       shirtsRegular,
		Size:       "XL",
		RecordedAt: mustParse(t, "2026-11-07T09:15:00Z"),
	}); err != nil {
		t.Fatalf("record later correction: %v", err)
	}
	if after := readStatus(t, ctx, g, beliefRunsSmallLater); after != before {
		t.Errorf("later correction moved the first belief: %+v, want unchanged %+v", after, before)
	}
	if journal.Len() != 2 {
		t.Fatalf("journal holds %d corrections, want 2", journal.Len())
	}
	if len(g.actions) != 1 {
		t.Errorf("later correction wrote %d structural edges, want 1 total: it stands on its own", len(g.actions))
	}

	// The later correction declares once when its own matching belief
	// arrives, and the first correction is not declared again.
	declared, err = memory.EnsureBeliefDeclaring(ctx, g, journal, memory.Belief{
		Content: beliefRunsLargerLater,
		Kind:    domain.BeliefKindFit,
		Subject: brandX(),
	}, mustParse(t, "2026-11-08T20:10:00Z"))
	if err != nil {
		t.Fatalf("write belief after later correction: %v", err)
	}
	if declared != 1 {
		t.Errorf("later correction declared %d times, want 1", declared)
	}
	later := readStatus(t, ctx, g, beliefRunsLargerLater)
	if count := strings.Count(later.Warning, correctionGrew); count != 1 {
		t.Errorf("later belief warning %q names the later correction %d times, want 1", later.Warning, count)
	}
	if strings.Contains(later.Warning, correction.Text) {
		t.Errorf("later belief warning %q carries the earlier correction again", later.Warning)
	}
	first := readStatus(t, ctx, g, beliefRunsSmallLater)
	if count := strings.Count(first.Warning, correction.Text); count != 1 {
		t.Errorf("first belief warning %q names its correction %d times, want 1", first.Warning, count)
	}
	if strings.Contains(first.Warning, correctionGrew) {
		t.Errorf("first belief warning %q now carries the later correction", first.Warning)
	}
	// The correction itself never earned anything either way.
	if own := readStatus(t, ctx, g, correction.Text); own.Standing != domain.GraphNone || own.Dependents != 0 {
		t.Errorf("declared correction reads standing %q dependents %d, want None 0", own.Standing, own.Dependents)
	}
}

// TestCorrectionAgainstExistingBeliefUnchanged pins today's path: a
// correction naming an existing belief ships at once, declares against it,
// adds no evidence, leaves the belief standing, and never becomes a
// standing record that a later belief could re-declare.
func TestCorrectionAgainstExistingBeliefUnchanged(t *testing.T) {
	ctx := context.Background()
	g := standingGraph()
	journal := &memory.StandingCorrections{}
	correctionAt := mustParse(t, "2026-11-05T14:02:11Z")

	earnBelief(t, ctx, g, beliefRunsSmallLater)
	beforeActions := len(g.actions)

	if err := memory.RecordCorrection(ctx, g, fakes.CorrectionLostWeight, beliefRunsSmallLater, correctionAt); err != nil {
		t.Fatalf("record correction against existing belief: %v", err)
	}
	if len(g.actions) != beforeActions+1 {
		t.Fatalf("correction wrote %d structural edges, want 1", len(g.actions)-beforeActions)
	}
	action := g.actions[len(g.actions)-1]
	if action.content != fakes.CorrectionLostWeight {
		t.Errorf("correction writes %q, want %q", action.content, fakes.CorrectionLostWeight)
	}
	if len(action.dependsOn) != 0 {
		t.Errorf("correction depends on %v, want none: it adds no evidence", action.dependsOn)
	}
	if len(action.against) != 1 || action.against[0] != beliefRunsSmallLater {
		t.Errorf("correction is against %v, want [%q]", action.against, beliefRunsSmallLater)
	}
	if !action.at.Equal(correctionAt) {
		t.Errorf("correction dated %s, want %s", action.at, correctionAt)
	}
	reading := readStatus(t, ctx, g, beliefRunsSmallLater)
	if reading.Status != domain.StatusOutOfTrue || reading.Standing != domain.GraphVenerable {
		t.Errorf("contradicted belief reads status %q standing %q, want Out of true Venerable: the belief stays",
			reading.Status, reading.Standing)
	}
	if !reading.Contradicted || reading.Dependents != 3 {
		t.Errorf("contradicted belief reads contradicted %v dependents %d, want true 3", reading.Contradicted, reading.Dependents)
	}
	if count := strings.Count(reading.Warning, fakes.CorrectionLostWeight); count != 1 {
		t.Errorf("belief warning %q names the correction %d times, want 1", reading.Warning, count)
	}
	if journal.Len() != 0 {
		t.Errorf("today's path journalled %d standing corrections, want 0", journal.Len())
	}

	// A belief written later does not repeat the declaration: the
	// correction stands against the belief it named, not against a new
	// one.
	declared, err := memory.EnsureBeliefDeclaring(ctx, g, journal, memory.Belief{
		Content: beliefRunsLargeRival,
		Kind:    domain.BeliefKindFit,
		Subject: brandX(),
	}, mustParse(t, "2026-11-06T11:30:00Z"))
	if err != nil {
		t.Fatalf("write later belief: %v", err)
	}
	if declared != 0 {
		t.Errorf("later belief declared %d corrections, want 0", declared)
	}
	if len(g.actions) != beforeActions+1 {
		t.Errorf("later belief wrote %d structural edges, want the correction alone", len(g.actions)-beforeActions)
	}
	if rival := readStatus(t, ctx, g, beliefRunsLargeRival); rival.Contradicted || rival.Warning != "" {
		t.Errorf("later belief reads contradicted %v warning %q, want false and empty", rival.Contradicted, rival.Warning)
	}
	if kept := readStatus(t, ctx, g, beliefRunsSmallLater); kept != reading {
		t.Errorf("after the later belief the contradicted belief reads %+v, want unchanged %+v", kept, reading)
	}
}

// TestSameSubject pins the subject match: the domain's shapes decide, and
// the size never does.
func TestSameSubject(t *testing.T) {
	brand := brandX()
	seller := domain.Subject{Kind: domain.SubjectSeller, ID: "seller-for-brand-x-001", Name: "Brand X seller"}
	otherBrand := domain.Subject{Kind: domain.SubjectBrand, ID: "brand-y-002", Name: "Brand Y"}
	cases := []struct {
		name string
		cor  memory.Correction
		bel  memory.Belief
		want bool
	}{
		{
			name: "same brand, no line named",
			cor:  memory.Correction{Subject: brand},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: brand},
			want: true,
		},
		{
			name: "different brand",
			cor:  memory.Correction{Subject: brand},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: otherBrand},
			want: false,
		},
		{
			name: "correction names a line, belief spans the brand",
			cor:  memory.Correction{Subject: brand, Line: shirtsRegular},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: brand},
			want: true,
		},
		{
			name: "both name the same line",
			cor:  memory.Correction{Subject: brand, Line: shirtsRegular},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: brand, Line: shirtsRegular},
			want: true,
		},
		{
			name: "different category",
			cor:  memory.Correction{Subject: brand, Line: shirtsRegular},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: brand, Line: domain.ProductLine{Brand: brand.ID, Category: "pants", FitHandle: "regular"}},
			want: false,
		},
		{
			name: "different cut",
			cor:  memory.Correction{Subject: brand, Line: shirtsRegular},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: brand, Line: domain.ProductLine{Brand: brand.ID, Category: "shirts", FitHandle: "slim"}},
			want: false,
		},
		{
			name: "line brand disagrees with the subject",
			cor:  memory.Correction{Subject: brand, Line: shirtsRegular},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: brand, Line: domain.ProductLine{Brand: otherBrand.ID, Category: "shirts", FitHandle: "regular"}},
			want: false,
		},
		{
			name: "different sizes still match",
			cor:  memory.Correction{Subject: brand, Size: "M"},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: brand},
			want: true,
		},
		{
			name: "seller correction against a seller belief",
			cor:  memory.Correction{Subject: seller},
			bel:  memory.Belief{Kind: domain.BeliefKindSeller, Subject: seller},
			want: true,
		},
		{
			name: "fit belief about a seller",
			cor:  memory.Correction{Subject: seller},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: seller},
			want: false,
		},
		{
			name: "seller correction against a brand belief",
			cor:  memory.Correction{Subject: seller},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: brand},
			want: false,
		},
		{
			name: "belief counting toward nothing",
			cor:  memory.Correction{Subject: brand},
			bel:  memory.Belief{Kind: domain.BeliefKindNone, Subject: brand},
			want: false,
		},
		{
			name: "belief names no id",
			cor:  memory.Correction{Subject: brand},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: domain.Subject{Kind: domain.SubjectBrand}},
			want: false,
		},
		{
			name: "correction names an unknown subject kind",
			cor:  memory.Correction{Subject: domain.Subject{Kind: "merchant", ID: brand.ID}},
			bel:  memory.Belief{Kind: domain.BeliefKindFit, Subject: brand},
			want: false,
		},
	}
	for _, tc := range cases {
		if got := memory.SameSubject(tc.cor, tc.bel); got != tc.want {
			t.Errorf("%s: SameSubject = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestStandingCorrectionRefusals proves a standing correction needs words
// and one named subject, needs somewhere to be kept, and that the same
// words about the same subject stay one correction.
func TestStandingCorrectionRefusals(t *testing.T) {
	ctx := context.Background()
	g := standingGraph()
	brand := brandX()
	at := mustParse(t, "2026-11-05T14:02:11Z")

	var absent *memory.StandingCorrections
	if err := absent.Record(ctx, g, memory.Correction{Text: "use M", Subject: brand, RecordedAt: at}); !errors.Is(err, memory.ErrNoStandingJournal) {
		t.Errorf("record with no journal: error = %v, want ErrNoStandingJournal", err)
	}
	if declared, err := absent.Declare(ctx, g, memory.Belief{Content: fakes.BeliefRunsSmall, Kind: domain.BeliefKindFit, Subject: brand}); declared != 0 || err != nil {
		t.Errorf("declaring with no journal = (%d, %v), want (0, nil)", declared, err)
	}

	journal := &memory.StandingCorrections{}
	for _, tc := range []struct {
		name string
		c    memory.Correction
	}{
		{"no words", memory.Correction{Subject: brand, RecordedAt: at}},
		{"no subject", memory.Correction{Text: "use M", RecordedAt: at}},
		{"unknown subject kind", memory.Correction{Text: "use M", Subject: domain.Subject{Kind: "merchant", ID: "m-1"}, RecordedAt: at}},
		{"line of another brand", memory.Correction{Text: "use M", Subject: brand, Line: domain.ProductLine{Brand: "brand-y-002", Category: "shirts"}, RecordedAt: at}},
	} {
		if err := journal.Record(ctx, g, tc.c); !errors.Is(err, memory.ErrIncompleteCorrection) {
			t.Errorf("%s: error = %v, want ErrIncompleteCorrection", tc.name, err)
		}
	}
	if journal.Len() != 0 {
		t.Errorf("refused corrections left %d in the journal, want 0", journal.Len())
	}
	if len(g.derives) != 0 || len(g.actions) != 0 {
		t.Errorf("refused corrections wrote %d derives and %d actions, want none", len(g.derives), len(g.actions))
	}

	if err := journal.Record(ctx, g, memory.Correction{Text: "use M", Subject: brand, Against: fakes.BeliefRunsSmall, RecordedAt: at}); !errors.Is(err, memory.ErrCorrectionDeclared) {
		t.Errorf("declared correction offered as standing: error = %v, want ErrCorrectionDeclared", err)
	}

	c := memory.Correction{Text: fakes.CorrectionLostWeight, Subject: brand, Size: "M", RecordedAt: at}
	for i := 0; i < 2; i++ {
		if err := journal.Record(ctx, g, c); err != nil {
			t.Fatalf("record standing correction %d: %v", i+1, err)
		}
	}
	if journal.Len() != 1 {
		t.Errorf("the same correction recorded twice holds %d entries, want 1", journal.Len())
	}
	if len(g.derives) != 1 {
		t.Errorf("the same correction recorded twice derives %d times, want 1", len(g.derives))
	}
	if len(g.actions) != 0 {
		t.Errorf("the same correction recorded twice wrote %d structural edges, want 0", len(g.actions))
	}
}
