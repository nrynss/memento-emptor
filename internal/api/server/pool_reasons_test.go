package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
)

// TestReasonCountsViewGivesEachCauseOneField proves the pool view folds
// every spelling of a cause into the domain's own field. The recorded
// population spells damage "damaged" and not-as-described
// "not_as_described"; the second counts as damage, because the goods
// arrived and were wrong, and never as non-arrival.
func TestReasonCountsViewGivesEachCauseOneField(t *testing.T) {
	got := reasonCountsView(map[domain.Category]int{
		domain.CategoryFitTooSmall:        2,
		domain.CategoryFitTooLarge:        3,
		domain.CategoryDamagedOrDefective: 5,
		"damaged":                         7,
		"not_as_described":                11,
		domain.CategoryNotArrived:         13,
		domain.CategoryChangedMind:        17,
		"too_expensive":                   19,
	})
	want := contract.ReasonCountsView{
		FitTooSmall:        2,
		FitTooLarge:        3,
		DamagedOrDefective: 23,
		NotArrived:         13,
		ChangedMind:        17,
	}
	if got != want {
		t.Fatalf("reason counts %+v, want %+v", got, want)
	}
}

// TestServedFitWriterRefusesTheLegacyPoolLabels pins the reachability of
// the view's two legacy labels. The fit writer is the only served producer
// of pool rows: it refuses a category outside domain.Categories, and a
// dispute writes no row at all, so no served path can put "damaged" or
// "not_as_described" into the index. The view keeps them as a read-side
// alias for an index that already holds the recorded population's
// spelling. A task that widens the vocabulary or writes a row around the
// writer fails here.
func TestServedFitWriterRefusesTheLegacyPoolLabels(t *testing.T) {
	ctx := context.Background()
	keyer, err := identity.NewKeyer(identity.SeedSaltHex)
	if err != nil {
		t.Fatalf("build keyer: %v", err)
	}
	index := &fakeFitIndex{}
	writer, err := fitindex.NewWriter(index, keyer, "shopper_01", fitindex.LinesFunc(staticLines))
	if err != nil {
		t.Fatalf("build fit writer: %v", err)
	}
	purchase := fitindex.Purchase{
		TxnID: "CAP-T85K-0001",
		Line:  domain.ProductLine{Brand: "brand-x-001", Category: "shirts", FitHandle: "regular"},
		Size:  "M",
	}
	at := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)

	for _, label := range []domain.Category{"damaged", "not_as_described"} {
		if err := writer.Classified(ctx, purchase, label, at); !errors.Is(err, fitindex.ErrUnknownReason) {
			t.Errorf("classify %q = %v, want %v", label, err, fitindex.ErrUnknownReason)
		}
		if err := writer.Contested(ctx, purchase, label, at); !errors.Is(err, fitindex.ErrUnknownReason) {
			t.Errorf("contest %q = %v, want %v", label, err, fitindex.ErrUnknownReason)
		}
	}
	if n := index.rowCount(); n != 0 {
		t.Fatalf("a refused label wrote %d fit rows, want none", n)
	}

	// The not-as-described dispute is the one outcome that carries this
	// cause today: it enters evidence as damage and still writes no row.
	dispute := domain.Evidence{
		Source:  domain.SourcePayment,
		Outcome: domain.Outcome{Kind: domain.OutcomeDispute},
		Item:    domain.Item{BrandID: "brand-x-001", Category: "shirts", Size: "M"},
		Link:    domain.TransactionLink{OrderID: "ORD-T85K-0001", CaptureID: "CAP-T85K-0001"},
		Reason: &domain.Reason{
			Category:   domain.CategoryDamagedOrDefective,
			Text:       string(domain.DisputeMerchandiseNotAsDescribed),
			Confidence: 1,
		},
	}
	if err := writer.Record(ctx, dispute); err != nil {
		t.Fatalf("record not-as-described dispute: %v", err)
	}
	if n := index.rowCount(); n != 0 {
		t.Fatalf("a dispute wrote %d fit rows, want none", n)
	}

	for _, label := range []domain.Category{"damaged", "not_as_described"} {
		for _, known := range domain.Categories {
			if known == label {
				t.Fatalf("domain.Categories names the legacy pool label %q: the alias is no longer read-only", label)
			}
		}
	}
}
