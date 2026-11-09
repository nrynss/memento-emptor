package evidence

import (
	"errors"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// TestDocumentBuilders proves the store keeps three kinds of raw text:
// the shopper's words, a reason joined to its outcome and item, and
// catalog title backed by description.
func TestDocumentBuilders(t *testing.T) {
	message := ForMessage("too tight in the shoulders", false)
	if message.Kind != KindMessage {
		t.Errorf("message kind = %q, want message", message.Kind)
	}
	if message.Text != "too tight in the shoulders" {
		t.Errorf("message text = %q", message.Text)
	}
	if message.Line.Complete() {
		t.Errorf("bare message names a line: %+v", message.Line)
	}

	ev := domain.Evidence{
		Source:  domain.SourceShopper,
		Outcome: domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED"},
		Item: domain.Item{
			BrandID: "brand-x-001", BrandName: "Brand X",
			LineName: "oxford shirt", Size: "M", Category: "dress-shirts",
		},
		Reason: &domain.Reason{
			Category: domain.CategoryFitTooSmall,
			Text:     "too tight in the shoulders",
		},
		Seeded: true,
	}
	reason := ForReason(ev, "Regular")
	if reason.Kind != KindReason {
		t.Errorf("reason kind = %q, want reason", reason.Kind)
	}
	for _, want := range []string{"refund", "Brand X", "oxford shirt", "M", "too tight in the shoulders"} {
		if !strings.Contains(reason.Text, want) {
			t.Errorf("reason text %q misses %q", reason.Text, want)
		}
	}
	if !reason.Seeded {
		t.Error("reason document drops the seed mark")
	}
	wantLine := Line{BrandID: "brand-x-001", Category: "dress-shirts", FitHandle: "Regular"}
	if reason.Line != wantLine {
		t.Errorf("reason line = %+v, want %+v", reason.Line, wantLine)
	}

	// A reasonless outcome still stores, and says so: it becomes a
	// pending question and counts toward nothing here.
	ev.Reason = nil
	pending := ForReason(ev, "Regular")
	if !strings.Contains(pending.Text, "no reason given yet") {
		t.Errorf("reasonless text %q does not say so", pending.Text)
	}

	product := ForProduct("brand-x-001", "dress-shirts", "Regular",
		"Button Down Oxford Shirt", "It features a regular fit.", false)
	if product.Kind != KindProduct {
		t.Errorf("product kind = %q, want product", product.Kind)
	}
	if !strings.Contains(product.Text, "regular fit") {
		t.Errorf("product text %q loses the description's fit words", product.Text)
	}
	if product.Line != wantLine {
		t.Errorf("product line = %+v, want %+v", product.Line, wantLine)
	}
}

// TestDocumentValidate proves empty text stores nothing.
func TestDocumentValidate(t *testing.T) {
	for _, doc := range []Document{
		ForMessage("   ", false),
		ForProduct("b", "c", "f", "", "  ", false),
	} {
		if err := doc.Validate(); !errors.Is(err, ErrEmptyText) {
			t.Errorf("empty document validates: %+v", doc)
		}
	}
	if err := ForMessage("words", false).Validate(); err != nil {
		t.Errorf("storables text fails validation: %v", err)
	}
}
