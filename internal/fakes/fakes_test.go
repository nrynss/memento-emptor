package fakes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// fixtureRoot returns the repository's testdata directory beside this package.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "testdata")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("testdata is missing at %s: %v", dir, err)
	}
	return dir
}

// TestAssertKeysAbsent proves the offline guard: empty means absent, any
// value fails loudly naming the variable.
func TestAssertKeysAbsent(t *testing.T) {
	for _, name := range KeyVariables() {
		t.Setenv(name, "")
	}
	if err := AssertKeysAbsent(); err != nil {
		t.Fatalf("keys unset but guard failed: %v", err)
	}
	t.Setenv("OPENROUTER_API_KEY", "anything")
	if err := AssertKeysAbsent(); !errors.Is(err, ErrKeysPresent) {
		t.Fatalf("set key did not fail with ErrKeysPresent: %v", err)
	}
}

// TestClassifierReplaysLabelledSet proves the fake serves the thirteen
// recorded Jev answers with their recorded choices and confidences. The
// expectations below are the research table, not the loader's output.
func TestClassifierReplaysLabelledSet(t *testing.T) {
	fake, err := NewRecordingReasonClassifier(filepath.Join(fixtureRoot(t), "jev"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		answer     string
		category   domain.Category
		confidence float64
	}{
		{"too tight in the shoulders", domain.CategoryFitTooSmall, 1},
		{"the sleeves are way too short and I can't button it", domain.CategoryFitTooSmall, 1},
		{"it swims on me, far too big", domain.CategoryFitTooLarge, 1},
		{"arrived broken", domain.CategoryDamagedOrDefective, 1},
		{"the screen was cracked when I opened the box", domain.CategoryDamagedOrDefective, 1},
		{"it never arrived, tracking has been stuck for weeks", domain.CategoryNotArrived, 1},
		{"changed my mind", domain.CategoryChangedMind, 1},
		{"just don't want it anymore", domain.CategoryChangedMind, 1},
		{"not great", domain.CategoryUnclear, 0.94},
		{"it wasn't what I wanted", domain.CategoryChangedMind, 0.96},
		{"meh, sending it back", domain.CategoryUnclear, 0.68},
		{"sleeves too short in M", domain.CategoryFitTooSmall, 1},
		{"too tight across the chest", domain.CategoryFitTooSmall, 1},
	}
	if got := fake.RecordedAnswers(); len(got) != len(cases) {
		t.Fatalf("fake covers %d answers, want %d", len(got), len(cases))
	}
	ctx := context.Background()
	for _, tc := range cases {
		class, err := fake.Classify(ctx, tc.answer)
		if err != nil {
			t.Errorf("classify %q: %v", tc.answer, err)
			continue
		}
		if class.Category != tc.category || class.Confidence != tc.confidence {
			t.Errorf("classify %q = (%v, %v), want (%v, %v)",
				tc.answer, class.Category, class.Confidence, tc.category, tc.confidence)
		}
		if !strings.HasPrefix(class.Provenance, "testdata/jev/") {
			t.Errorf("classify %q has no recording provenance: %q", tc.answer, class.Provenance)
		}
	}
	// The floor rule from the research: the vague answers ask again, the
	// clear ones do not.
	for _, answer := range []string{"not great", "meh, sending it back", "it wasn't what I wanted"} {
		class, err := fake.Classify(ctx, answer)
		if err != nil {
			t.Fatal(err)
		}
		if !AskAgain(class) {
			t.Errorf("answer %q should ask again", answer)
		}
	}
	for _, answer := range []string{"too tight in the shoulders", "changed my mind"} {
		class, err := fake.Classify(ctx, answer)
		if err != nil {
			t.Fatal(err)
		}
		if AskAgain(class) {
			t.Errorf("answer %q should not ask again", answer)
		}
	}
	if _, err := fake.Classify(ctx, "too tight around the waist"); !errors.Is(err, ErrNoRecording) {
		t.Errorf("unrecorded answer did not fail loudly: %v", err)
	}
}

// TestGraphReplaysSeededPromotion replays the seeded-session transcript's
// contents through the fake: three fit returns earn Venerable over an
// honest span, the correction warns beside the kept state, the dispute
// leaves sizing untouched, and one supporting point promotes nothing.
func TestGraphReplaysSeededPromotion(t *testing.T) {
	const (
		belief      = "Brand X oxford runs small for this shopper: take size L"
		statement   = "Shopper says they are usually size M"
		returnOne   = "Fit return 1: Brand X oxford shirt size M refunded; reason too tight in the shoulders"
		returnTwo   = "Fit return 2: Brand X oxford shirt size M refunded; reason too tight across the chest"
		returnThree = "Fit return 3: Brand X oxford shirt size M refunded; reason sleeves too short in M"
		correction  = "Shopper correction: lost weight, take size M for Brand X"
		seller      = "Seller Acme ships reliably for this shopper"
		dispute     = "Dispute opened: Brand X parcel never arrived; carrier lost it"
		rival       = "Brand X oxford in size M fits this shopper now"
		feedback    = "Fit feedback: Brand X oxford shirt size M fits well"
	)
	day := func(n int) time.Time {
		return time.Date(2026, 1, 1+n, 0, 0, 0, 0, time.UTC)
	}
	fake := NewRecordingEvidenceGraph(
		[]string{belief, statement, returnOne, returnTwo, returnThree, correction, seller, dispute, rival, feedback},
		map[string][]string{"sizing": {belief, statement}},
	)
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(fake.Derive(ctx, belief, "logic", day(0)))
	must(fake.Derive(ctx, statement, "observation", day(0)))
	must(fake.RecordAction(ctx, returnOne, []string{belief}, nil, day(3)))
	must(fake.RecordAction(ctx, returnTwo, []string{belief}, nil, day(6)))
	must(fake.RecordAction(ctx, returnThree, []string{belief}, nil, day(9)))

	report, err := fake.Inspect(ctx, belief)
	if err != nil {
		t.Fatal(err)
	}
	if report.State != domain.GraphVenerable {
		t.Errorf("after three returns state = %v, want Venerable", report.State)
	}
	if report.Dependents != 3 || report.DistinctInteractions != 3 {
		t.Errorf("after three returns dependents = %d/%d, want 3/3", report.Dependents, report.DistinctInteractions)
	}
	if diff := report.Coverage - 6.0/9.0; diff < -0.01 || diff > 0.01 {
		t.Errorf("after three returns coverage = %v, want 0.667", report.Coverage)
	}

	must(fake.RecordAction(ctx, correction, nil, []string{belief}, day(10)))
	report, err = fake.Inspect(ctx, belief)
	if err != nil {
		t.Fatal(err)
	}
	if report.State != domain.GraphVenerable || !report.Contradicted || report.Warning == "" {
		t.Errorf("after correction state = %v contradicted = %v warning = %q",
			report.State, report.Contradicted, report.Warning)
	}

	must(fake.Derive(ctx, seller, "logic", day(11)))
	must(fake.RecordAction(ctx, dispute, []string{seller}, nil, day(11)))
	sizing, err := fake.Inspect(ctx, belief)
	if err != nil {
		t.Fatal(err)
	}
	if sizing.Dependents != 3 {
		t.Errorf("after dispute sizing dependents = %d, want 3", sizing.Dependents)
	}
	sellerReport, err := fake.Inspect(ctx, seller)
	if err != nil {
		t.Fatal(err)
	}
	if sellerReport.State != domain.GraphNone {
		t.Errorf("one dispute promotes the seller belief to %v, want None", sellerReport.State)
	}

	must(fake.Derive(ctx, rival, "logic", day(12)))
	must(fake.RecordAction(ctx, feedback, []string{rival}, nil, day(12)))
	rivalReport, err := fake.Inspect(ctx, rival)
	if err != nil {
		t.Fatal(err)
	}
	if rivalReport.State != domain.GraphNone {
		t.Errorf("one feedback promotes the rival to %v, want None", rivalReport.State)
	}
	statementReport, err := fake.Inspect(ctx, statement)
	if err != nil {
		t.Fatal(err)
	}
	if statementReport.State != domain.GraphNone {
		t.Errorf("statement promotes to %v, want None", statementReport.State)
	}
}

// TestGraphFailLoudly proves the graph fake invents nothing: uncovered
// contents, uncited beliefs and uncovered queries all fail with
// ErrNoRecording.
func TestGraphFailLoudly(t *testing.T) {
	fake := NewRecordingEvidenceGraph([]string{BeliefRunsSmall}, StoryRecalls())
	ctx := context.Background()
	if err := fake.Derive(ctx, "a belief no recording covers", "logic", time.Now()); !errors.Is(err, ErrNoRecording) {
		t.Errorf("uncovered derive did not fail loudly: %v", err)
	}
	if err := fake.RecordAction(ctx, "an action no recording covers", []string{BeliefRunsSmall}, nil, time.Now()); !errors.Is(err, ErrNoRecording) {
		t.Errorf("uncovered action did not fail loudly: %v", err)
	}
	if err := fake.RecordAction(ctx, ReturnOne, []string{"a belief never derived"}, nil, time.Now()); !errors.Is(err, ErrNoRecording) {
		t.Errorf("uncited depends_on did not fail loudly: %v", err)
	}
	if _, err := fake.Inspect(ctx, "a belief never derived"); !errors.Is(err, ErrNoRecording) {
		t.Errorf("uncovered inspect did not fail loudly: %v", err)
	}
	if _, err := fake.Recall(ctx, "a query no recording covers"); !errors.Is(err, ErrNoRecording) {
		t.Errorf("uncovered recall did not fail loudly: %v", err)
	}
	if _, err := fake.Recall(ctx, StoryRecallQuery); err != nil {
		t.Errorf("recorded recall failed: %v", err)
	}
}

// TestPaymentFailLoudly proves the payment fake invents no transaction.
func TestPaymentFailLoudly(t *testing.T) {
	fake := NewRecordingPaymentSource(DemoStoryInputs())
	ctx := context.Background()
	if _, err := fake.VerifyWebhook(ctx, "WH-NOPE"); !errors.Is(err, ErrNoRecording) {
		t.Errorf("unknown event did not fail loudly: %v", err)
	}
	if _, err := fake.GetOrder(ctx, "ORD-NOPE"); !errors.Is(err, ErrNoRecording) {
		t.Errorf("unknown order did not fail loudly: %v", err)
	}
	if _, err := fake.GetRefund(ctx, "RFD-NOPE"); !errors.Is(err, ErrNoRecording) {
		t.Errorf("unknown refund did not fail loudly: %v", err)
	}
	if _, err := fake.GetDispute(ctx, "PP-D-NOPE"); !errors.Is(err, ErrNoRecording) {
		t.Errorf("unknown dispute did not fail loudly: %v", err)
	}
}

// TestPaymentShapes proves the story's payment leg replays the P0 shapes:
// line items live on the order read, refunds carry no reason, and the
// dispute's reason routes to the seller, never to sizing.
func TestPaymentShapes(t *testing.T) {
	fake := NewRecordingPaymentSource(DemoStoryInputs())
	ctx := context.Background()
	for _, refundID := range []string{"RFD-7XK-01", "RFD-7XK-02", "RFD-7XK-03", "RFD-7XK-05"} {
		record, err := fake.GetRefund(ctx, refundID)
		if err != nil {
			t.Fatal(err)
		}
		if record.HasReason {
			t.Errorf("refund %s carries a reason; recordings carry none", refundID)
		}
	}
	order, err := fake.GetOrder(ctx, "ORD-7XK-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(order.Items) == 0 {
		t.Error("order read carries no line items; the ledger cannot join without them")
	}
	event, err := fake.VerifyWebhook(ctx, "WH-2EV-01")
	if err != nil {
		t.Fatal(err)
	}
	if !event.Verified {
		t.Error("recorded webhook delivery does not verify")
	}
	dispute, err := fake.GetDispute(ctx, "PP-D-51713")
	if err != nil {
		t.Fatal(err)
	}
	if dispute.Category != domain.CategoryNotArrived {
		t.Errorf("dispute category = %v, want not_arrived", dispute.Category)
	}
	if dispute.Category.CountsToward() != domain.BeliefKindSeller {
		t.Errorf("dispute counts toward %v, want seller reliability", dispute.Category.CountsToward())
	}
	if domain.CategoryFitTooSmall.CountsToward() != domain.BeliefKindFit {
		t.Error("fit reasons do not count toward fit")
	}
}

// TestStoreReplaysRecordings proves the store serves the recorded kNN
// rankings: the same-line candidate ranks first, and the fresh snug-chest
// query puts the recorded reason phrases on top. Unknown queries fail
// loudly.
func TestStoreReplaysRecordings(t *testing.T) {
	fake, err := NewRecordingEvidenceStore(filepath.Join(fixtureRoot(t), "elastic"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := fake.IndexDoc(ctx, "Brand X oxford shirt, size L"); err != nil {
		t.Fatal(err)
	}
	hits, err := fake.Search(ctx, "Brand X oxford shirt")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].DocID != "p6" {
		t.Errorf("candidate search ranks %v first, want p6", hits)
	}
	hits, err = fake.Search(ctx, "snug across the chest, pulls at the buttons")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 2 || hits[0].DocID != "r2" || hits[1].DocID != "r1" {
		t.Errorf("reason search ranks %v first, want r2 then r1", hits)
	}
	if _, err := fake.Search(ctx, "a query no recording covers"); !errors.Is(err, ErrNoRecording) {
		t.Errorf("unrecorded query did not fail loudly: %v", err)
	}
}
