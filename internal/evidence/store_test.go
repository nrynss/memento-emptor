package evidence

import (
	"errors"
	"testing"

	"github.com/nrynss/memento-emptor/internal/fakes"
)

// stubContract is a test-only embedding contract: it names a kind and
// nothing else. Production code never names a model, dimensions, endpoint
// or key; the kind alone decides whether a store answers.
type stubContract struct {
	kind string
	err  error
}

// EmbeddingKind names the contract family.
func (s stubContract) EmbeddingKind() string { return s.kind }

// Validate reports whether the contract is usable.
func (s stubContract) Validate() error { return s.err }

// TestStoreContract proves the refusal rule: a store opens only under a
// usable contract, answers only under its own kind, and re-embeds
// atomically onto a new one.
func TestStoreContract(t *testing.T) {
	if _, err := NewStore(nil); !errors.Is(err, ErrInvalidContract) {
		t.Errorf("nil contract gives %v, want ErrInvalidContract", err)
	}
	if _, err := NewStore(stubContract{kind: "kind-a", err: errors.New("broken")}); !errors.Is(err, ErrInvalidContract) {
		t.Errorf("failing contract gives %v, want ErrInvalidContract", err)
	}
	if _, err := NewStore(stubContract{}); !errors.Is(err, ErrInvalidContract) {
		t.Errorf("empty kind gives %v, want ErrInvalidContract", err)
	}

	store, err := NewStore(stubContract{kind: "kind-a"})
	if err != nil {
		t.Fatal(err)
	}
	if store.ContractKind() != "kind-a" {
		t.Errorf("store kind = %q, want kind-a", store.ContractKind())
	}
	if err := store.CheckContract(stubContract{kind: "kind-a"}); err != nil {
		t.Errorf("own contract refused: %v", err)
	}
	if err := store.CheckContract(stubContract{kind: "kind-b"}); !errors.Is(err, ErrContractMismatch) {
		t.Errorf("foreign contract gives %v, want ErrContractMismatch", err)
	}

	if err := store.ReEmbed(nil); !errors.Is(err, ErrInvalidContract) {
		t.Errorf("re-embed onto nil gives %v, want ErrInvalidContract", err)
	}
	if store.ContractKind() != "kind-a" {
		t.Errorf("failed re-embed moved the kind to %q", store.ContractKind())
	}
	if err := store.ReEmbed(stubContract{kind: "kind-b"}); err != nil {
		t.Fatal(err)
	}
	if store.ContractKind() != "kind-b" {
		t.Errorf("re-embed left the kind at %q, want kind-b", store.ContractKind())
	}
	if err := store.CheckContract(stubContract{kind: "kind-a"}); !errors.Is(err, ErrContractMismatch) {
		t.Errorf("old contract still answers after re-embed: %v", err)
	}
}

// TestStoreImplementsSeam proves the store adopts the replay seam as-is:
// it serves wherever the recording fake serves.
func TestStoreImplementsSeam(t *testing.T) {
	var seam fakes.EvidenceStore
	seam, err := NewStore(stubContract{kind: "test-kind"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	id, err := seam.IndexDoc(ctx, "Brand X oxford shirt, size M")
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Error("indexed document has no id")
	}
	hits, err := seam.Search(ctx, "oxford shirt")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].DocID != id {
		t.Errorf("seam search = %+v, want the one indexed document", hits)
	}
}

// TestStoreIndexSearch proves the store's own ranking: the share of the
// query's tokens each document holds, deterministic on ties.
func TestStoreIndexSearch(t *testing.T) {
	store, err := NewStore(stubContract{kind: "test-kind"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	texts := []string{
		"Brand X classic oxford shirt, size M, regular fit",
		"Brand X slim oxford shirt, size M, tailored cut",
		"Brand Y relaxed chore coat, size L, loose fit",
	}
	var ids []string
	for _, text := range texts {
		id, err := store.IndexDoc(ctx, text)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if ids[0] != "doc-0001" || ids[1] != "doc-0002" || ids[2] != "doc-0003" {
		t.Errorf("doc ids = %v, want sequential doc-0001..3", ids)
	}

	hits, err := store.Search(ctx, "Brand X oxford shirt")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 3 {
		t.Fatalf("search holds %d hits, want 3", len(hits))
	}
	if hits[0].DocID != ids[0] || hits[1].DocID != ids[1] {
		t.Errorf("same-line shirts rank %v, want them first in index order", hits)
	}
	if hits[2].DocID != ids[2] {
		t.Errorf("distractor ranks %v, want it last", hits[2])
	}
	if hits[0].Score != 1 || hits[2].Score >= hits[0].Score {
		t.Errorf("scores = %v, want the full match at 1 above the partial",
			[]float64{hits[0].Score, hits[2].Score})
	}

	if _, err := store.Search(ctx, "   "); !errors.Is(err, ErrEmptyQuery) {
		t.Errorf("empty query gives %v, want ErrEmptyQuery", err)
	}
	if _, err := store.IndexDoc(ctx, "  "); !errors.Is(err, ErrEmptyText) {
		t.Errorf("empty text gives %v, want ErrEmptyText", err)
	}
	misses, err := store.Search(ctx, "evening gown")
	if err != nil {
		t.Fatal(err)
	}
	if len(misses) != 0 {
		t.Errorf("unmatched query returns hits: %+v", misses)
	}
}

// TestStoreLedgerSearch proves the ledger path: stored documents whose
// text holds the query, in index order.
func TestStoreLedgerSearch(t *testing.T) {
	store, err := NewStore(stubContract{kind: "test-kind"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	for _, text := range []string{
		"Brand X classic oxford shirt, size M: too tight in the shoulders",
		"Brand Y relaxed chore coat: kept by shopper",
		"Brand X slim oxford shirt, size M: too tight across the chest",
	} {
		if _, err := store.IndexDoc(ctx, text); err != nil {
			t.Fatal(err)
		}
	}
	found, err := store.LedgerSearch("TOO TIGHT")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 || found[0].DocID != "doc-0001" || found[1].DocID != "doc-0003" {
		t.Errorf("ledger search holds %v, want doc-0001 then doc-0003", found)
	}
	if _, err := store.LedgerSearch(""); !errors.Is(err, ErrEmptyQuery) {
		t.Errorf("empty ledger query gives %v, want ErrEmptyQuery", err)
	}
	if found, err := store.LedgerSearch("evening gown"); err != nil || len(found) != 0 {
		t.Errorf("ledger miss = %v, %v; want no rows and no error", found, err)
	}
}

// TestStoreKeepsDocuments proves the store returns what it holds, in
// index order, with the structured lines intact.
func TestStoreKeepsDocuments(t *testing.T) {
	store, err := NewStore(stubContract{kind: "test-kind"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	line := Line{BrandID: "brand-x-001", Category: "dress-shirts", FitHandle: "Regular"}
	doc := Document{Kind: KindProduct, Text: "Button Down Oxford Shirt, regular fit", Line: line}
	id, err := store.IndexDocument(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	kept := store.Documents()
	if len(kept) != 1 || kept[0].DocID != id || kept[0].Line != line {
		t.Errorf("stored = %+v, want the indexed document with its line", kept)
	}
	kept[0].Text = "rewritten"
	if store.Documents()[0].Text == "rewritten" {
		t.Error("Documents exposes the store's own rows")
	}
}
