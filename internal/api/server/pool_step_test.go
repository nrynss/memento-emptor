package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
	"github.com/nrynss/memento-emptor/internal/pool"
)

// poolAnswer returns one index answer for a single bucket: the all-time
// counts and the same counts in the recent window, with no live rows, so a
// Building label cannot flip on the window.
func poolAnswer(all pool.Counts) pool.Result {
	return pool.Result{All: all, Seeded: all, Recent: all}
}

// fitsWellPool is four distinct shoppers whose only fit outcome is that
// the size fits well. There are no returns either way, so the pool reaches
// the Building floor and names no direction.
func fitsWellPool() pool.Result {
	return poolAnswer(pool.Counts{
		Settled:  4,
		Feedback: pool.VerdictCounts{FitsWell: 4},
		Shoppers: pool.ShopperCounts{Fit: 4},
	})
}

// levelPool is four distinct shoppers split evenly, two too small and two
// too large. It reaches the Building floor and names no direction, because
// neither direction leads.
func levelPool() pool.Result {
	return poolAnswer(pool.Counts{
		Settled:  4,
		Returns:  pool.DirectionCounts{TooSmall: 2, TooLarge: 2},
		Shoppers: pool.ShopperCounts{Fit: 4, TooSmall: 2, TooLarge: 2},
	})
}

// buildingSmallPool is the seeded population's slim line: eight distinct
// shoppers, six too-small returns, one too-large return and one fits-well.
// The small direction leads, but its six shoppers fall short of the Earned
// floor, so the pool is Building small.
func buildingSmallPool() pool.Result {
	return poolAnswer(pool.Counts{
		Settled:  8,
		Returns:  pool.DirectionCounts{TooSmall: 6, TooLarge: 1},
		Feedback: pool.VerdictCounts{FitsWell: 1},
		Shoppers: pool.ShopperCounts{Fit: 8, TooSmall: 6, TooLarge: 1},
	})
}

// assertPoolReads names the pooled label recall consulted, proving the
// regression case's premise: the same pool read product_fit serves.
func assertPoolReads(t *testing.T, svc *Service, status domain.BeliefStatus, direction *domain.FitDirection) {
	t.Helper()
	fit, err := svc.ProductFit(context.Background(), contract.ProductFitRequest{
		BrandID: "brand-x-001", Category: "shirts", FitHandle: "regular", Size: "M",
	})
	if err != nil {
		t.Fatalf("product fit: %v", err)
	}
	if fit.Pool.Status != status {
		t.Fatalf("pool reads %q, want %q", fit.Pool.Status, status)
	}
	switch {
	case direction == nil && fit.Pool.Direction != nil:
		t.Fatalf("pool names direction %q, want none", *fit.Pool.Direction)
	case direction != nil && fit.Pool.Direction == nil:
		t.Fatalf("pool names no direction, want %q", *direction)
	case direction != nil && *fit.Pool.Direction != *direction:
		t.Fatalf("pool names direction %q, want %q", *fit.Pool.Direction, *direction)
	}
}

// assertPoolNamesNoDirection proves a pool that names no direction takes
// no precedence step and guides no size: the lookup reads consulted, the
// pool object is absent, and no sentence sizes down from the gap.
func assertPoolNamesNoDirection(t *testing.T, recall contract.RecallResponse) {
	t.Helper()
	if recall.Pool != nil {
		t.Fatalf("a pool with no direction is served as %+v, want no pool object", recall.Pool)
	}
	if recall.PoolLookup != contract.PoolConsulted {
		t.Fatalf("pool lookup %q, want consulted", recall.PoolLookup)
	}
	if strings.Contains(recall.Guidance.Text, "down to") || strings.Contains(recall.Guidance.Text, "too large") {
		t.Fatalf("guidance reads a missing direction as large: %q", recall.Guidance.Text)
	}
}

// TestRecallFitsWellPoolNamesNoDirection proves the T8.5r regression: four
// distinct shoppers whose only fit outcome is fits-well reach the Building
// floor but say nothing about how the size runs off, so the pool is
// consulted, absent from the precedence order, and never tells a new
// shopper to size down from a zero count.
func TestRecallFitsWellPoolNamesNoDirection(t *testing.T) {
	ctx := context.Background()
	svc, _ := pivotHarness(t, &fakePoolIndex{result: fitsWellPool()}, &fakeFitIndex{}, "")
	assertPoolReads(t, svc, domain.StatusBuilding, nil)
	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: "new_shopper", BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	assertPoolNamesNoDirection(t, recall)
	if recall.Guidance.Basis != contract.GuidanceNone {
		t.Fatalf("guidance %+v, want none deciding", recall.Guidance)
	}
	if recall.Guidance.Text != noGuidance().Text {
		t.Fatalf("guidance text %q, want the nothing-applies sentence", recall.Guidance.Text)
	}
	if len(recall.Beliefs) != 0 {
		t.Fatalf("new shopper recall serves %d beliefs, want none", len(recall.Beliefs))
	}
}

// TestRecallLevelPoolNamesNoDirection proves the second T8.5r regression:
// a level Building pool (two too small, two too large) leads nowhere, so
// it takes no precedence step and guides no size instead of reading the
// tie as runs large.
func TestRecallLevelPoolNamesNoDirection(t *testing.T) {
	ctx := context.Background()
	svc, _ := pivotHarness(t, &fakePoolIndex{result: levelPool()}, &fakeFitIndex{}, "")
	assertPoolReads(t, svc, domain.StatusBuilding, nil)
	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: "new_shopper", BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	assertPoolNamesNoDirection(t, recall)
	if recall.Guidance.Basis != contract.GuidanceNone {
		t.Fatalf("guidance %+v, want none deciding", recall.Guidance)
	}
	if recall.Guidance.Text != noGuidance().Text {
		t.Fatalf("guidance text %q, want the nothing-applies sentence", recall.Guidance.Text)
	}
}

// TestRecallBuildingPoolWithDirectionDecides proves the gate is only on a
// missing direction, not on the status label: a Building pool whose small
// direction leads still takes precedence step 3 and sizes up from the
// size under consideration, citing the pool's own counts.
func TestRecallBuildingPoolWithDirectionDecides(t *testing.T) {
	ctx := context.Background()
	small := domain.FitDirectionSmall
	svc, _ := pivotHarness(t, &fakePoolIndex{result: buildingSmallPool()}, &fakeFitIndex{}, "")
	assertPoolReads(t, svc, domain.StatusBuilding, &small)
	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: "new_shopper", BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if recall.Pool == nil || recall.Pool.Status != domain.StatusBuilding ||
		recall.Pool.Direction == nil || *recall.Pool.Direction != domain.FitDirectionSmall {
		t.Fatalf("pool reads %+v, want a served Building small pool", recall.Pool)
	}
	if recall.Pool.Precedence == nil || recall.Pool.Precedence.Step != contract.GuidancePool || recall.Pool.Precedence.Overridden {
		t.Fatalf("pool precedence %+v, want the deciding pool step", recall.Pool.Precedence)
	}
	if recall.Guidance.Basis != contract.GuidancePool {
		t.Fatalf("guidance %+v, want the pool deciding", recall.Guidance)
	}
	want := "Size up to L for brand-x-001: 6 of the 8 fit-classified settled purchases at size M came back too small, against 1 too large."
	if recall.Guidance.Text != want {
		t.Fatalf("guidance text %q, want %q", recall.Guidance.Text, want)
	}
}

// TestRecallOwnEvidenceUnaffectedByPoolWithNoDirection proves a pool that
// names no direction is not a pool the shopper's evidence is ahead of: own
// evidence decides and the guidance omits the pool clause, because there is
// no pool side to disagree with.
func TestRecallOwnEvidenceUnaffectedByPoolWithNoDirection(t *testing.T) {
	ctx := context.Background()
	svc, _ := pivotHarness(t, &fakePoolIndex{result: fitsWellPool()}, &fakeFitIndex{}, "")
	assertPoolReads(t, svc, domain.StatusBuilding, nil)
	shopper := "own_shopper"
	answerTwoLarge(t, svc, shopper)
	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: shopper, BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	assertPoolNamesNoDirection(t, recall)
	if len(recall.Beliefs) != 1 {
		t.Fatalf("recall serves %d beliefs, want the one own-evidence belief", len(recall.Beliefs))
	}
	belief := recall.Beliefs[0]
	if belief.Precedence == nil || belief.Precedence.Step != contract.GuidanceOwnEvidence || belief.Precedence.Overridden {
		t.Fatalf("own belief precedence %+v, want the deciding own-evidence step", belief.Precedence)
	}
	if recall.Guidance.Basis != contract.GuidanceOwnEvidence {
		t.Fatalf("guidance %+v, want own_evidence deciding", recall.Guidance)
	}
	want := "Take size S for Brand X"
	if recall.Guidance.Text != want {
		t.Fatalf("guidance text %q, want %q", recall.Guidance.Text, want)
	}
}

// TestRecallStatementDecidesWhenPoolNamesNoDirection proves the next step
// decides when the pool cannot: a shopper whose only memory is a general
// statement gets the statement step, with the pool consulted, absent and
// unmarked rather than overridden.
func TestRecallStatementDecidesWhenPoolNamesNoDirection(t *testing.T) {
	ctx := context.Background()
	svc := statementOnlyHarness(t, &fakePoolIndex{result: fitsWellPool()})
	assertPoolReads(t, svc, domain.StatusBuilding, nil)
	recall, err := svc.Recall(ctx, contract.RecallRequest{
		ShopperID: "statement_shopper", BrandID: "brand-x-001",
		Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	assertPoolNamesNoDirection(t, recall)
	if len(recall.Beliefs) != 1 {
		t.Fatalf("recall serves %d beliefs, want the one general statement", len(recall.Beliefs))
	}
	statement := recall.Beliefs[0]
	if statement.Precedence == nil || statement.Precedence.Step != contract.GuidanceStatement || statement.Precedence.Overridden {
		t.Fatalf("statement precedence %+v, want the deciding statement step", statement.Precedence)
	}
	if recall.Guidance.Basis != contract.GuidanceStatement {
		t.Fatalf("guidance %+v, want the statement deciding", recall.Guidance)
	}
	want := "Take size M for Brand X: the shopper's usual size."
	if recall.Guidance.Text != want {
		t.Fatalf("guidance text %q, want %q", recall.Guidance.Text, want)
	}
}

// TestGuidanceForPoolCannotReadMissingDirection proves the guidance
// builder takes its direction as an argument: a view whose direction is
// absent, handed the no-direction value, guides nothing instead of sizing
// down from the missing direction.
func TestGuidanceForPoolCannotReadMissingDirection(t *testing.T) {
	view := contract.PoolView{
		Size: "M",
		Counts: contract.PoolCountsView{
			Returns: contract.DirectionCountsView{TooSmall: 2, TooLarge: 2},
		},
	}
	guidance := guidanceForPool(view, domain.FitDirectionNone, "brand-x-001")
	if guidance.Basis != contract.GuidanceNone {
		t.Fatalf("guidance %+v, want none", guidance)
	}
	if strings.Contains(guidance.Text, "down to") || strings.Contains(guidance.Text, "too large") {
		t.Fatalf("guidance reads a missing direction as large: %q", guidance.Text)
	}
}

// TestProductFitServesDirectionlessBuildingPool proves the merchant read
// still answers for a Building pool that names no direction: the label and
// the counts are the merchant's answer, and only recall's precedence step
// requires a direction. The direction is null because none leads, and the
// precedence step is null because product_fit never carries one.
func TestProductFitServesDirectionlessBuildingPool(t *testing.T) {
	svc, _ := pivotHarness(t, &fakePoolIndex{result: fitsWellPool()}, &fakeFitIndex{}, "")
	fit, err := svc.ProductFit(context.Background(), contract.ProductFitRequest{
		BrandID: "brand-x-001", Category: "shirts", FitHandle: "regular", Size: "M",
	})
	if err != nil {
		t.Fatalf("product fit: %v", err)
	}
	if fit.Pool.Status != domain.StatusBuilding {
		t.Fatalf("pool status %q, want Building", fit.Pool.Status)
	}
	if fit.Pool.Direction != nil {
		t.Fatalf("pool direction %q, want null", *fit.Pool.Direction)
	}
	if fit.Pool.Counts.Settled != 4 || fit.Pool.Counts.Feedback.FitsWell != 4 {
		t.Fatalf("pool counts %+v, want four settled fits-well rows", fit.Pool.Counts)
	}
	if fit.Pool.Precedence != nil {
		t.Fatalf("pool precedence %+v, want none from product_fit", fit.Pool.Precedence)
	}
}

// statementOnlyHarness builds a service whose one seeded belief is a
// general statement, with a pool index wired. The statement has no
// evidence, which is what makes it the statement step.
func statementOnlyHarness(t *testing.T, poolIdx pool.Index) *Service {
	t.Helper()
	seed := Seed{
		ShopperID: "statement_shopper",
		Beliefs: []contract.BeliefView{{
			BeliefID:  "blf_77c310",
			Kind:      domain.BeliefKindFit,
			Subject:   domain.Subject{Kind: domain.SubjectBrand, ID: "brand-x-001", Name: "Brand X"},
			Value:     "shopper says they are usually size M",
			Status:    domain.StatusClaimed,
			Standing:  domain.GraphNone,
			Evidence:  []contract.EvidenceView{},
			Seeded:    true,
			UpdatedAt: "2026-11-08T09:20:00Z",
		}},
	}
	store, err := evidence.NewStore(testContract{kind: "pool-step"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	fixed := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:     fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier:   classifier,
		Store:        store,
		Graph:        fakes.NewRecordingEvidenceGraph(SeedGraphContents(seed), nil),
		SchemaDir:    apiDir(t),
		Clock:        func() time.Time { return fixed },
		Pool:         poolIdx,
		FitRows:      &fakeFitIndex{},
		Lines:        fitindex.LinesFunc(staticLines),
		IdentitySalt: identity.SeedSaltHex,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if err := svc.LoadSeed(context.Background(), seed); err != nil {
		t.Fatalf("load statement seed: %v", err)
	}
	return svc
}
