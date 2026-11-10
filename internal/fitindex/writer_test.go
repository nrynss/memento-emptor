package fitindex

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/identity"
)

// The fixed event times the writer tests use. The purchase is captured,
// refunded, classified and contested on four separate moments, so event
// time is the only thing that can order the writes inside a rank.
var (
	captureTime  = time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	refundTime   = time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	classifyTime = time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	contestTime  = time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC)
)

// step is one event a test applies to a writer.
type step struct {
	name  string
	apply func(context.Context, *Writer, Purchase) error
}

// testLines resolves the two test lines from the item's category.
func testLines(_ context.Context, item domain.Item) (domain.ProductLine, error) {
	switch item.Category {
	case testLine.Category:
		return testLine, nil
	case otherLine.Category:
		return otherLine, nil
	default:
		return domain.ProductLine{}, errors.New("no line for category " + item.Category)
	}
}

// testPurchase is the purchase every writer test drives.
func testPurchase(txnID string) Purchase {
	return Purchase{TxnID: txnID, Line: testLine, Size: "M"}
}

// writerFor builds a writer over the index with the given options.
func writerFor(t *testing.T, index Index, payerID string, opts ...Option) *Writer {
	t.Helper()
	writer, err := NewWriter(index, testKeyer(t), payerID, LinesFunc(testLines), opts...)
	if err != nil {
		t.Fatalf("build writer: %v", err)
	}
	return writer
}

// permutations returns every ordering of n steps.
func permutations(n int) [][]int {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	var out [][]int
	var generate func(k int)
	generate = func(k int) {
		if k == 1 {
			out = append(out, append([]int(nil), order...))
			return
		}
		generate(k - 1)
		for i := 0; i < k-1; i++ {
			if k%2 == 0 {
				order[i], order[k-1] = order[k-1], order[i]
			} else {
				order[0], order[k-1] = order[k-1], order[0]
			}
			generate(k - 1)
		}
	}
	generate(n)
	return out
}

// applyPath applies steps in the given order to a fresh writer and returns
// the one row the index holds.
func applyPath(t *testing.T, steps []step) domain.FitRow {
	t.Helper()
	index := newFakeIndex()
	writer := writerFor(t, index, "payer-0001")
	ctx := context.Background()
	for _, s := range steps {
		if err := s.apply(ctx, writer, testPurchase("CAP-T82-0001")); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
	}
	row := index.only(t)
	if len(index.versions) != 1 {
		t.Fatalf("index holds %d versions, want 1", len(index.versions))
	}
	if row.TxnHash == "" || len(row.ShopperHash) != 64 {
		t.Fatalf("row carries txn %q and shopper %q, want the hashes", row.TxnHash, row.ShopperHash)
	}
	return row
}

// TestOnePurchaseEndsAtOneRow drives a purchase through every state a fit
// row has: captured, refunded, reason classified, then contested, plus the
// feedback and kept ways of settling. Each path is applied in event order
// and then in every possible shuffled order, and every run must end at the
// same row: the highest state's content, one document, with the purchase's
// facts filled in.
func TestOnePurchaseEndsAtOneRow(t *testing.T) {
	paths := []struct {
		name  string
		steps []step
		want  func(t *testing.T, row domain.FitRow)
	}{
		{
			name: "return then contest",
			steps: []step{
				{"captured", func(ctx context.Context, w *Writer, p Purchase) error {
					return w.Captured(ctx, p, captureTime)
				}},
				{"refunded", func(ctx context.Context, w *Writer, p Purchase) error {
					return w.Refunded(ctx, p, refundTime)
				}},
				{"classified", func(ctx context.Context, w *Writer, p Purchase) error {
					return w.Classified(ctx, p, domain.CategoryFitTooSmall, classifyTime)
				}},
				{"contested", func(ctx context.Context, w *Writer, p Purchase) error {
					return w.Contested(ctx, p, domain.CategoryFitTooLarge, contestTime)
				}},
			},
			want: func(t *testing.T, row domain.FitRow) {
				t.Helper()
				if row.State != domain.FitStateContested {
					t.Errorf("state = %q, want contested", row.State)
				}
				if row.Reason != domain.CategoryFitTooLarge || row.Direction != domain.FitDirectionLarge {
					t.Errorf("contested reason = %q/%q, want fit_too_large/large", row.Reason, row.Direction)
				}
				if !row.ReasonContested {
					t.Error("contested mark is not set")
				}
				if !row.Settled || row.SettledKind != domain.FitSettledByReturnReason {
					t.Errorf("settled = %v/%q, want true/return_reason", row.Settled, row.SettledKind)
				}
				if !row.SettledAt.Equal(classifyTime) {
					t.Errorf("settled_at = %s, want the classification time %s", row.SettledAt, classifyTime)
				}
			},
		},
		{
			name: "fit feedback settles",
			steps: []step{
				{"captured", func(ctx context.Context, w *Writer, p Purchase) error {
					return w.Captured(ctx, p, captureTime)
				}},
				{"feedback", func(ctx context.Context, w *Writer, p Purchase) error {
					return w.Feedback(ctx, p, domain.FitVerdictTooSmall, classifyTime)
				}},
			},
			want: func(t *testing.T, row domain.FitRow) {
				t.Helper()
				if row.State != domain.FitStateFeedback {
					t.Errorf("state = %q, want feedback", row.State)
				}
				if row.Verdict != domain.FitVerdictTooSmall || row.Direction != domain.FitDirectionSmall {
					t.Errorf("verdict = %q/%q, want too_small/small", row.Verdict, row.Direction)
				}
				if !row.Settled || row.SettledKind != domain.FitSettledByFeedback {
					t.Errorf("settled = %v/%q, want true/feedback", row.Settled, row.SettledKind)
				}
			},
		},
		{
			name: "kept past the window settles",
			steps: []step{
				{"captured", func(ctx context.Context, w *Writer, p Purchase) error {
					return w.Captured(ctx, p, captureTime)
				}},
				{"kept", func(ctx context.Context, w *Writer, p Purchase) error {
					return w.Kept(ctx, p, classifyTime)
				}},
			},
			want: func(t *testing.T, row domain.FitRow) {
				t.Helper()
				if row.State != domain.FitStateKept {
					t.Errorf("state = %q, want kept", row.State)
				}
				if row.Verdict != "" || row.Reason != "" || row.Direction != domain.FitDirectionNone {
					t.Errorf("kept row carries reason %q, verdict %q, direction %q; a kept row carries none",
						row.Reason, row.Verdict, row.Direction)
				}
				if !row.Settled || row.SettledKind != domain.FitSettledByKept {
					t.Errorf("settled = %v/%q, want true/kept", row.Settled, row.SettledKind)
				}
			},
		},
	}

	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			inOrder := applyPath(t, path.steps)
			path.want(t, inOrder)
			if !inOrder.CapturedAt.Equal(captureTime) {
				t.Errorf("captured_at = %s, want %s", inOrder.CapturedAt, captureTime)
			}
			for _, order := range permutations(len(path.steps)) {
				shuffled := make([]step, len(order))
				for i, at := range order {
					shuffled[i] = path.steps[at]
				}
				got := applyPath(t, shuffled)
				if !reflect.DeepEqual(got, inOrder) {
					t.Fatalf("order %v ends at a different row:\n got: %+v\nwant: %+v", order, got, inOrder)
				}
			}
		})
	}
}

// TestRefundWithoutReasonIsStoredNotSettled captures a purchase and
// refunds it with no reason. The row exists, says returned, and counts
// toward nothing: a refund says the money came back, not why.
func TestRefundWithoutReasonIsStoredNotSettled(t *testing.T) {
	index := newFakeIndex()
	writer := writerFor(t, index, "payer-0001")
	ctx := context.Background()
	purchase := testPurchase("CAP-T82-0002")

	if err := writer.Captured(ctx, purchase, captureTime); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if err := writer.Refunded(ctx, purchase, refundTime); err != nil {
		t.Fatalf("refund: %v", err)
	}
	row := index.only(t)
	if row.State != domain.FitStateRefunded {
		t.Errorf("state = %q, want refunded", row.State)
	}
	if row.Settled || row.SettledKind != "" || !row.SettledAt.IsZero() {
		t.Errorf("row settled = %v/%q at %s; a refund with no reason counts toward nothing",
			row.Settled, row.SettledKind, row.SettledAt)
	}
	if !row.RefundedAt.Equal(refundTime) {
		t.Errorf("refunded_at = %s, want %s", row.RefundedAt, refundTime)
	}
	if !row.CapturedAt.Equal(captureTime) {
		t.Errorf("captured_at = %s, want the capture that preceded it %s", row.CapturedAt, captureTime)
	}
}

// TestReplayedWebhookIsHarmless applies the same event twice. The second
// write carries the same version, so the index accepts it, the row is
// unchanged and there is still one document.
func TestReplayedWebhookIsHarmless(t *testing.T) {
	index := newFakeIndex()
	writer := writerFor(t, index, "payer-0001")
	ctx := context.Background()
	purchase := testPurchase("CAP-T82-0003")

	if err := writer.Captured(ctx, purchase, captureTime); err != nil {
		t.Fatalf("capture: %v", err)
	}
	first := index.only(t)
	if err := writer.Captured(ctx, purchase, captureTime); err != nil {
		t.Fatalf("replayed capture: %v", err)
	}
	second := index.only(t)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("replay changed the row:\n got: %+v\nwant: %+v", second, first)
	}
	if index.writes != 2 {
		t.Errorf("index took %d writes, want 2: the replay is accepted and changes nothing", index.writes)
	}
	if len(index.versions) != 1 {
		t.Errorf("index holds %d versions, want 1", len(index.versions))
	}
}

// TestOlderStateNeverOverwritesNewer applies the later state first and
// then replays the earlier ones. The row keeps the newer state while the
// earlier events fill their facts in, and the index refuses an older write
// outright.
func TestOlderStateNeverOverwritesNewer(t *testing.T) {
	index := newFakeIndex()
	writer := writerFor(t, index, "payer-0001")
	ctx := context.Background()
	purchase := testPurchase("CAP-T82-0004")

	if err := writer.Classified(ctx, purchase, domain.CategoryFitTooSmall, classifyTime); err != nil {
		t.Fatalf("classified: %v", err)
	}
	want := index.only(t)
	if err := writer.Refunded(ctx, purchase, refundTime); err != nil {
		t.Fatalf("late refund: %v", err)
	}
	if err := writer.Captured(ctx, purchase, captureTime); err != nil {
		t.Fatalf("late capture: %v", err)
	}
	got := index.only(t)
	if got.State != domain.FitStateReasonClassified {
		t.Errorf("state = %q after older events, want reason_classified", got.State)
	}
	if got.Reason != domain.CategoryFitTooSmall {
		t.Errorf("reason = %q after older events, want the classified reason", got.Reason)
	}
	if got.Version() != want.Version() {
		t.Errorf("version = %d after older events, want the newer state's %d", got.Version(), want.Version())
	}
	if !got.CapturedAt.Equal(captureTime) || !got.RefundedAt.Equal(refundTime) {
		t.Errorf("older events filled no facts: captured_at %s, refunded_at %s",
			got.CapturedAt, got.RefundedAt)
	}

	older := got
	older.State = domain.FitStateCaptured
	older.EventAt = captureTime
	if err := index.Put(ctx, older); !errors.Is(err, ErrOlderEvent) {
		t.Fatalf("older write error = %v, want ErrOlderEvent", err)
	}
	if held := index.only(t); held.State != domain.FitStateReasonClassified {
		t.Errorf("index state = %q after a refused older write, want reason_classified", held.State)
	}
}

// TestCorrectionWritesNothing records a shopper's words through the same
// recorder ingest uses. A correction is about the shopper, never the
// product, so it writes no row.
func TestCorrectionWritesNothing(t *testing.T) {
	index := newFakeIndex()
	writer := writerFor(t, index, "payer-0001")
	correction := domain.Evidence{
		Source: domain.SourceShopper,
		Item:   domain.Item{BrandID: "brand-x-001", Category: "shirts", Size: "M", SKU: "PSW8GeK"},
		Link:   domain.TransactionLink{CaptureID: "CAP-T82-0005"},
		Reason: &domain.Reason{Category: domain.CategoryFitTooLarge},
	}
	if err := writer.Record(context.Background(), correction); err != nil {
		t.Fatalf("record correction: %v", err)
	}
	if len(index.docs) != 0 || index.writes != 0 {
		t.Errorf("index took %d writes for a correction, want 0", index.writes)
	}
	if index.created != 0 {
		t.Error("writer created the index; the layer creates it, not a write path")
	}
}

// TestDisputeWritesNoRow records a dispute. It says the purchase did not
// arrive, which is about the seller and not about how the product fits.
func TestDisputeWritesNoRow(t *testing.T) {
	index := newFakeIndex()
	writer := writerFor(t, index, "payer-0001")
	dispute := domain.Evidence{
		Source:  domain.SourcePayment,
		Outcome: domain.Outcome{Kind: domain.OutcomeDispute},
		Item:    domain.Item{BrandID: "brand-x-001", Category: "shirts", Size: "M"},
		Link:    domain.TransactionLink{OrderID: "ORD-T82-0006", CaptureID: "CAP-T82-0006"},
		Reason:  &domain.Reason{Category: domain.CategoryNotArrived},
	}
	if err := writer.Record(context.Background(), dispute); err != nil {
		t.Fatalf("record dispute: %v", err)
	}
	if index.writes != 0 {
		t.Errorf("index took %d writes for a dispute, want 0", index.writes)
	}
}

// TestRecordMapsTheKeptOutcome drives a kept outcome through the recorder
// ingest calls. Kept is a purchase state the recorder must reach, and it
// settles the row as kept without claiming a reason or a verdict.
func TestRecordMapsTheKeptOutcome(t *testing.T) {
	index := newFakeIndex()
	writer := writerFor(t, index, "payer-0001")
	keptAt := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)
	kept := domain.Evidence{
		Source:     domain.SourcePayment,
		Outcome:    domain.Outcome{Kind: domain.OutcomeKept, Status: "PAST_WINDOW"},
		Item:       domain.Item{BrandID: testLine.Brand, Category: testLine.Category, Size: "M", SKU: "SKU-KEPT"},
		Link:       domain.TransactionLink{OrderID: "ORD-T82-KEPT", CaptureID: "CAP-T82-KEPT"},
		OccurredAt: keptAt,
	}
	if err := writer.Record(context.Background(), kept); err != nil {
		t.Fatalf("record kept outcome: %v", err)
	}
	row := index.only(t)
	if row.State != domain.FitStateKept {
		t.Errorf("state = %q, want kept", row.State)
	}
	if !row.Settled || row.SettledKind != domain.FitSettledByKept {
		t.Errorf("settled = %v/%q, want true/kept", row.Settled, row.SettledKind)
	}
	if !row.SettledAt.Equal(keptAt) {
		t.Errorf("settled_at = %s, want the kept event's time %s", row.SettledAt, keptAt)
	}
	if row.Reason != "" || row.Verdict != "" || row.Direction != domain.FitDirectionNone {
		t.Errorf("kept row carries reason %q, verdict %q, direction %q; it carries none",
			row.Reason, row.Verdict, row.Direction)
	}
	if !row.CapturedAt.IsZero() || !row.RefundedAt.IsZero() {
		t.Errorf("kept-only row claims captured_at %s and refunded_at %s", row.CapturedAt, row.RefundedAt)
	}
	if got, want := row.TxnHash, mustTxnHash(t, "CAP-T82-KEPT"); got != want {
		t.Errorf("row id = %s, want the capture's %s", got, want)
	}
}

// TestRecordRefusesAnUnknownOutcome keeps a payment outcome the fit row
// does not track from writing anything or being defaulted to another state.
func TestRecordRefusesAnUnknownOutcome(t *testing.T) {
	index := newFakeIndex()
	writer := writerFor(t, index, "payer-0001")
	unknown := domain.Evidence{
		Source:  domain.SourcePayment,
		Outcome: domain.Outcome{Kind: "chargeback"},
		Link:    domain.TransactionLink{CaptureID: "CAP-T82-UNKNOWN"},
	}
	err := writer.Record(context.Background(), unknown)
	if !errors.Is(err, ErrUnknownOutcome) {
		t.Fatalf("unknown outcome error = %v, want ErrUnknownOutcome", err)
	}
	if index.writes != 0 || len(index.docs) != 0 {
		t.Errorf("index took %d writes for an unknown outcome, want 0", index.writes)
	}
}

// TestTransactionIDPrefersTheCapture pins the purchase's row id: the
// capture's id when the link carries one, and the order's id only when it
// does not.
func TestTransactionIDPrefersTheCapture(t *testing.T) {
	cases := []struct {
		name string
		link domain.TransactionLink
		want string
	}{
		{"capture and order", domain.TransactionLink{OrderID: "ORD-T82", CaptureID: "CAP-T82"}, "CAP-T82"},
		{"capture only", domain.TransactionLink{CaptureID: "CAP-T82"}, "CAP-T82"},
		{"order only", domain.TransactionLink{OrderID: "ORD-T82"}, "ORD-T82"},
		{"neither", domain.TransactionLink{}, ""},
	}
	for _, tc := range cases {
		if got := transactionID(tc.link); got != tc.want {
			t.Errorf("%s: id = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestClassifiedWithoutACauseStaysUnsettled classifies a returned purchase
// with the two answers that name no cause. Each is stored and leaves the
// row unsettled, and the document claims no settlement time, so the refund
// stays a pending question.
func TestClassifiedWithoutACauseStaysUnsettled(t *testing.T) {
	for _, reason := range []domain.Category{domain.CategoryUnclear, domain.CategoryNone} {
		t.Run(string(reason), func(t *testing.T) {
			index := newFakeIndex()
			writer := writerFor(t, index, "payer-0001")
			ctx := context.Background()
			purchase := testPurchase("CAP-T82-0011")
			for _, apply := range []func() error{
				func() error { return writer.Captured(ctx, purchase, captureTime) },
				func() error { return writer.Refunded(ctx, purchase, refundTime) },
				func() error { return writer.Classified(ctx, purchase, reason, classifyTime) },
			} {
				if err := apply(); err != nil {
					t.Fatalf("drive purchase: %v", err)
				}
			}
			row := index.only(t)
			if row.State != domain.FitStateReasonClassified {
				t.Errorf("state = %q, want reason_classified", row.State)
			}
			if row.Reason != reason {
				t.Errorf("reason = %q, want the stored %q", row.Reason, reason)
			}
			if row.Settled || row.SettledKind != "" || !row.SettledAt.IsZero() {
				t.Errorf("row settled = %v/%q at %s; a reason without a cause counts toward nothing",
					row.Settled, row.SettledKind, row.SettledAt)
			}
			encoded, err := json.Marshal(documentFor(row))
			if err != nil {
				t.Fatalf("encode document: %v", err)
			}
			document := decodeObject(t, encoded)
			if _, ok := document["settled_at"]; ok {
				t.Error("document claims a settlement time for an unsettled row")
			}
			category, hasCategory := document["reason_category"]
			if reason == domain.CategoryNone && hasCategory {
				t.Errorf("document claims reason_category %v for no reason", category)
			}
			if reason != domain.CategoryNone && category != string(reason) {
				t.Errorf("document reason_category = %v, want %q", category, reason)
			}
		})
	}
}

// TestOptedOutShopperWritesNoRow turns pooling off and records a capture
// and a refund. Neither writes a row, and the row written before the
// opt-out was removed by Forget.
func TestOptedOutShopperWritesNoRow(t *testing.T) {
	index := newFakeIndex()
	pooling := true
	writer := writerFor(t, index, "payer-0001", WithPooling(func() bool { return pooling }))
	ctx := context.Background()
	purchase := testPurchase("CAP-T82-0007")

	if err := writer.Captured(ctx, purchase, captureTime); err != nil {
		t.Fatalf("capture: %v", err)
	}
	pooling = false
	if removed, err := writer.Forget(ctx, []domain.ProductLine{testLine}); err != nil || removed != 1 {
		t.Fatalf("forget = %d, %v; want 1 removed and no error", removed, err)
	}
	if err := writer.Captured(ctx, purchase, captureTime); err != nil {
		t.Fatalf("opted-out capture: %v", err)
	}
	if err := writer.Refunded(ctx, purchase, refundTime); err != nil {
		t.Fatalf("opted-out refund: %v", err)
	}
	if len(index.docs) != 0 {
		t.Errorf("index holds %d documents for an opted-out shopper, want 0", len(index.docs))
	}
	if index.writes != 1 {
		t.Errorf("index took %d writes, want 1: only the row written before the opt-out", index.writes)
	}
}

// TestForgetRemovesEveryLineAndLeavesOthers writes two shoppers' rows in
// two lines each, and opts the first shopper out. Their rows go from every
// line their session knows; the other shopper's rows stay.
func TestForgetRemovesEveryLineAndLeavesOthers(t *testing.T) {
	index := newFakeIndex()
	ctx := context.Background()
	first := writerFor(t, index, "payer-0001")
	second := writerFor(t, index, "payer-0002")

	for _, line := range []domain.ProductLine{testLine, otherLine} {
		for _, writer := range []*Writer{first, second} {
			purchase := Purchase{
				TxnID: "CAP-" + line.Category + "-" + writer.payerID,
				Line:  line,
				Size:  "M",
			}
			if err := writer.Captured(ctx, purchase, captureTime); err != nil {
				t.Fatalf("capture in %s: %v", line.Category, err)
			}
		}
	}
	if len(index.docs) != 4 {
		t.Fatalf("index holds %d documents, want 4", len(index.docs))
	}

	removed, err := first.Forget(ctx, []domain.ProductLine{testLine, otherLine})
	if err != nil {
		t.Fatalf("forget: %v", err)
	}
	if removed != 2 {
		t.Errorf("forget removed %d rows, want 2: one per line", removed)
	}
	keyer := testKeyer(t)
	for _, line := range []domain.ProductLine{testLine, otherLine} {
		firstHash, err := keyer.LineShopperHash("payer-0001", line)
		if err != nil {
			t.Fatalf("hash first shopper: %v", err)
		}
		secondHash, err := keyer.LineShopperHash("payer-0002", line)
		if err != nil {
			t.Fatalf("hash second shopper: %v", err)
		}
		for _, row := range index.docs {
			if row.Line != line {
				continue
			}
			if row.ShopperHash == firstHash {
				t.Errorf("line %s still holds a row for the opted-out shopper", line.Category)
			}
			if row.ShopperHash != secondHash {
				t.Errorf("line %s holds a row for an unexpected shopper", line.Category)
			}
		}
	}
	if len(index.docs) != 2 {
		t.Errorf("index holds %d documents after the opt-out, want the other shopper's 2", len(index.docs))
	}
}

// TestSeededRowsCarryTheMark writes a seeded capture. The row and the
// document both carry the seeded mark, so the ledger can split on it.
func TestSeededRowsCarryTheMark(t *testing.T) {
	index := newFakeIndex()
	writer := writerFor(t, index, "seeded-payer")
	purchase := testPurchase("CAP-T82-0008")
	purchase.Seeded = true
	if err := writer.Captured(context.Background(), purchase, captureTime); err != nil {
		t.Fatalf("capture: %v", err)
	}
	row := index.only(t)
	if !row.Seeded {
		t.Error("row is not marked seeded")
	}
	if doc := documentFor(row); !doc.Seeded {
		t.Error("document is not marked seeded")
	}
}

// TestContestWithNoCauseUnsettles contests a classified reason with an
// unclear answer. The corrected reason replaces the old one in place and
// leaves the row unsettled, because the purchase has no cause left to
// count; the document then claims no settlement time.
func TestContestWithNoCauseUnsettles(t *testing.T) {
	index := newFakeIndex()
	writer := writerFor(t, index, "payer-0001")
	ctx := context.Background()
	purchase := testPurchase("CAP-T82-0010")

	for _, apply := range []func() error{
		func() error { return writer.Captured(ctx, purchase, captureTime) },
		func() error { return writer.Refunded(ctx, purchase, refundTime) },
		func() error { return writer.Classified(ctx, purchase, domain.CategoryFitTooSmall, classifyTime) },
		func() error { return writer.Contested(ctx, purchase, domain.CategoryUnclear, contestTime) },
	} {
		if err := apply(); err != nil {
			t.Fatalf("drive purchase: %v", err)
		}
	}
	row := index.only(t)
	if row.State != domain.FitStateContested {
		t.Errorf("state = %q, want contested", row.State)
	}
	if row.Reason != domain.CategoryUnclear {
		t.Errorf("reason = %q, want the corrected unclear answer", row.Reason)
	}
	if row.Settled || row.SettledKind != "" {
		t.Errorf("settled = %v/%q, want unsettled: an unclear answer is not a cause", row.Settled, row.SettledKind)
	}
	encoded, err := json.Marshal(documentFor(row))
	if err != nil {
		t.Fatalf("encode document: %v", err)
	}
	if _, ok := decodeObject(t, encoded)["settled_at"]; ok {
		t.Error("document claims a settlement time for an unsettled row")
	}
}

// TestNewWriterRefusesMissingParts fails closed: a writer needs an index, a
// keyer, a shopper and a line resolver, and none is defaulted.
func TestNewWriterRefusesMissingParts(t *testing.T) {
	keyer := testKeyer(t)
	lines := LinesFunc(testLines)
	cases := []struct {
		name  string
		index Index
		keyer *identity.Keyer
		payer string
		lines Lines
		want  error
	}{
		{"no index", nil, keyer, "payer-0001", lines, ErrMissingIndex},
		{"no keyer", newFakeIndex(), nil, "payer-0001", lines, ErrMissingKeyer},
		{"no shopper", newFakeIndex(), keyer, "", lines, identity.ErrEmptyPayerID},
		{"no lines", newFakeIndex(), keyer, "payer-0001", nil, ErrMissingLines},
	}
	for _, tc := range cases {
		if _, err := NewWriter(tc.index, tc.keyer, tc.payer, tc.lines); !errors.Is(err, tc.want) {
			t.Errorf("%s error = %v, want %v", tc.name, err, tc.want)
		}
	}
}

// TestWriterRefusesBadInput covers the caller defects the writer refuses
// rather than defaulting: no transaction id, no event time, an unknown
// reason and an unknown verdict.
func TestWriterRefusesBadInput(t *testing.T) {
	writer := writerFor(t, newFakeIndex(), "payer-0001")
	ctx := context.Background()
	cases := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "no transaction id",
			err:  writer.Captured(ctx, Purchase{Line: testLine, Size: "M"}, captureTime),
			want: ErrMissingTransaction,
		},
		{
			name: "no event time",
			err:  writer.Captured(ctx, testPurchase("CAP-T82-0009"), time.Time{}),
			want: ErrMissingEventTime,
		},
		{
			name: "unknown reason",
			err:  writer.Classified(ctx, testPurchase("CAP-T82-0009"), "runs_odd", classifyTime),
			want: ErrUnknownReason,
		},
		{
			name: "unknown verdict",
			err:  writer.Feedback(ctx, testPurchase("CAP-T82-0009"), "fits_great", classifyTime),
			want: ErrUnknownVerdict,
		},
	}
	for _, tc := range cases {
		if !errors.Is(tc.err, tc.want) {
			t.Errorf("%s error = %v, want %v", tc.name, tc.err, tc.want)
		}
	}
}
