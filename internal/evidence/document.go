package evidence

import (
	"fmt"
	"strings"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// Kind names what source text a stored document was built from.
type Kind string

// The document kinds the store keeps.
const (
	// KindMessage is the shopper's own words, reported through the agent.
	KindMessage Kind = "message"
	// KindReason is a classified reason joined to its outcome and item: a
	// refund or dispute with the shopper's answer or the source's reason
	// code, in words.
	KindReason Kind = "reason"
	// KindProduct is catalog text for one product: its title and
	// description, keyed by the line it belongs to.
	KindProduct Kind = "product"
)

// Kinds lists every document kind the store keeps.
var Kinds = []Kind{KindMessage, KindReason, KindProduct}

// Line identifies a product cut: the brand, the category, and the
// category-specific fit handle. A new product matches returned items only
// when all three agree; the line name alone proves nothing, since one
// line can carry divergent fit labels per product.
type Line struct {
	// BrandID identifies the brand. Fit beliefs are keyed per brand.
	BrandID string `json:"brand_id"`
	// Category is the product category slug, such as dress-shirts.
	Category string `json:"category"`
	// FitHandle is the category-specific fit label from the catalog's
	// structured attributes, such as top-fit on shirts.
	FitHandle string `json:"fit_handle"`
}

// Complete reports whether the line names all three parts. An incomplete
// line never matches: matching on a missing part would repeat the
// line-name mistake the catalog data warns against.
func (l Line) Complete() bool {
	return l.BrandID != "" && l.Category != "" && l.FitHandle != ""
}

// Matches reports whether the candidate shares this line's cut: same
// brand, same category, same fit handle, all present.
func (l Line) Matches(candidate Candidate) bool {
	return l.Complete() && candidate.Line.Complete() &&
		l.BrandID == candidate.Line.BrandID &&
		l.Category == candidate.Line.Category &&
		l.FitHandle == candidate.Line.FitHandle
}

// Document is one raw-evidence text in the store: the prose the graph
// never holds, keyed by the line it is about when one is known.
type Document struct {
	// DocID is the store-assigned id. Empty until the document is indexed;
	// graph stubs carry this id so prose stays out of the graph.
	DocID string `json:"doc_id"`
	// Kind names which source text the document was built from.
	Kind Kind `json:"kind"`
	// Text is the storable prose: a message, a reason joined to its
	// outcome and item, or a product title with its description.
	Text string `json:"text"`
	// Line is the cut the text is about. Zero when the text names none,
	// such as a bare shopper message.
	Line Line `json:"line"`
	// Seeded marks generated history. Every seeded row carries the mark.
	Seeded bool `json:"seeded"`
}

// Validate reports whether the document is storable: it must carry text.
func (d Document) Validate() error {
	if strings.TrimSpace(d.Text) == "" {
		return ErrEmptyText
	}
	return nil
}

// ForMessage builds a document from the shopper's own words. A bare
// message names no line, so the line stays zero.
func ForMessage(text string, seeded bool) Document {
	return Document{
		Kind:   KindMessage,
		Text:   strings.TrimSpace(text),
		Seeded: seeded,
	}
}

// ForReason builds a document from one piece of evidence: the outcome in
// words, the item it concerns, and the reason when one is known. A
// reasonless outcome still stores — it becomes a pending question — but
// the text says so, and no code path here turns it into a belief. The fit
// handle arrives with the product text, never with the payment record, so
// the caller passes it in.
func ForReason(ev domain.Evidence, fitHandle string) Document {
	var text strings.Builder
	fmt.Fprintf(&text, "%s %s", ev.Outcome.Kind, ev.Outcome.Status)
	item := ev.Item
	name := strings.TrimSpace(strings.TrimSpace(item.BrandName) + " " + strings.TrimSpace(item.LineName))
	if name != "" {
		fmt.Fprintf(&text, ": %s", name)
	}
	if item.Size != "" {
		fmt.Fprintf(&text, ", size %s", item.Size)
	}
	if ev.Reason != nil && strings.TrimSpace(ev.Reason.Text) != "" {
		fmt.Fprintf(&text, ": %s", strings.TrimSpace(ev.Reason.Text))
	} else {
		text.WriteString(": no reason given yet")
	}
	return Document{
		Kind: KindReason,
		Text: text.String(),
		Line: Line{
			BrandID:   item.BrandID,
			Category:  item.Category,
			FitHandle: fitHandle,
		},
		Seeded: ev.Seeded,
	}
}

// ForProduct builds a document from catalog text for one product: the
// title backed by the description, keyed by the line they describe.
func ForProduct(brandID, category, fitHandle, title, description string, seeded bool) Document {
	text := strings.TrimSpace(strings.TrimSpace(title) + " " + strings.TrimSpace(description))
	return Document{
		Kind: KindProduct,
		Text: strings.Join(strings.Fields(text), " "),
		Line: Line{
			BrandID:   brandID,
			Category:  category,
			FitHandle: fitHandle,
		},
		Seeded: seeded,
	}
}
