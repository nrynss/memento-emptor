package evidence

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/fakes"
)

// TestLineMatching proves the catalog rule: brand plus category plus
// equal fit handle matches; the line name alone, a divergent handle, a
// different category, or a missing part never does.
func TestLineMatching(t *testing.T) {
	line := Line{BrandID: "SMwk", Category: "dress-shirts", FitHandle: "Regular"}
	same := Candidate{Line: line, SKU: "new-sku", Title: "Button Down Oxford Shirt"}
	if !line.Matches(same) {
		t.Error("same brand, category and fit handle does not match")
	}

	divergent := Candidate{
		Line: Line{BrandID: "SMwk", Category: "dress-shirts", FitHandle: "Slim"},
		SKU:  "other-sku",
	}
	if line.Matches(divergent) {
		t.Error("same line name with a divergent fit handle matches")
	}

	otherCategory := Candidate{
		Line: Line{BrandID: "SMwk", Category: "pants", FitHandle: "Regular"},
	}
	if line.Matches(otherCategory) {
		t.Error("same brand and fit handle in another category matches")
	}

	otherBrand := Candidate{
		Line: Line{BrandID: "other", Category: "dress-shirts", FitHandle: "Regular"},
	}
	if line.Matches(otherBrand) {
		t.Error("another brand with the same cut words matches")
	}

	for _, incomplete := range []Line{
		{Category: "dress-shirts", FitHandle: "Regular"},
		{BrandID: "SMwk", FitHandle: "Regular"},
		{BrandID: "SMwk", Category: "dress-shirts"},
		{},
	} {
		if incomplete.Matches(same) {
			t.Errorf("incomplete line %+v matches", incomplete)
		}
		if line.Matches(Candidate{Line: incomplete}) {
			t.Errorf("incomplete candidate %+v matches", incomplete)
		}
	}
}

// TestMatchCandidate proves the belief question: a new product matches
// the items behind a belief when it shares a line with their documents,
// and bare messages never match.
func TestMatchCandidate(t *testing.T) {
	line := Line{BrandID: "brand-x-001", Category: "dress-shirts", FitHandle: "Regular"}
	docs := []Document{
		{DocID: "doc-0001", Kind: KindReason, Text: "return one", Line: line},
		{DocID: "doc-0002", Kind: KindReason, Text: "return two", Line: line},
		{DocID: "doc-0003", Kind: KindMessage, Text: "too tight in the shoulders"},
		{DocID: "doc-0004", Kind: KindReason, Text: "slim cut return",
			Line: Line{BrandID: "brand-x-001", Category: "dress-shirts", FitHandle: "Slim"}},
	}
	candidate := Candidate{Line: line, SKU: "candidate-sku", Title: "Button Down Oxford Shirt"}
	report := MatchCandidate(candidate, docs)
	if !report.Matches {
		t.Fatal("candidate sharing the returns' line does not match")
	}
	if len(report.MatchedDocIDs) != 2 || report.MatchedDocIDs[0] != "doc-0001" || report.MatchedDocIDs[1] != "doc-0002" {
		t.Errorf("matched = %v, want the two agreeing returns in order", report.MatchedDocIDs)
	}

	slim := Candidate{
		Line: Line{BrandID: "brand-x-001", Category: "dress-shirts", FitHandle: "Slim"},
		SKU:  "slim-sku",
	}
	slimReport := MatchCandidate(slim, docs)
	if !slimReport.Matches || len(slimReport.MatchedDocIDs) != 1 || slimReport.MatchedDocIDs[0] != "doc-0004" {
		t.Errorf("slim candidate matched %v, want only doc-0004", slimReport.MatchedDocIDs)
	}

	if report := MatchCandidate(candidate, nil); report.Matches {
		t.Error("candidate matches with no belief documents")
	}
	if report := MatchCandidate(candidate, docs[:0]); report.Matches {
		t.Error("candidate matches an empty belief")
	}
}

// TestGraphStubKeepsProseOut proves the layer's core separation: the
// graph write carries the document id and no prose body.
func TestGraphStubKeepsProseOut(t *testing.T) {
	prose := "too tight in the shoulders, pulls at the buttons"
	doc := ForMessage(prose, false)
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(stubContract{kind: "test-kind"})
	if err != nil {
		t.Fatal(err)
	}
	docID, err := store.IndexDocument(t.Context(), doc)
	if err != nil {
		t.Fatal(err)
	}

	stub, err := GraphStub(docID, "fit return 1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stub, docID) {
		t.Errorf("graph stub %q carries no document id", stub)
	}
	if strings.Contains(stub, prose) {
		t.Errorf("graph stub %q carries the prose body", stub)
	}
	for _, words := range []string{"too tight", "pulls at the buttons"} {
		if strings.Contains(stub, words) {
			t.Errorf("graph stub %q leaks %q", stub, words)
		}
	}
	if len(stub) > stubMaxLen {
		t.Errorf("graph stub %q is %d characters, longer than the short bound", stub, len(stub))
	}

	// The stub writes to the graph exactly as built: it cites the belief
	// and carries the id, never the prose.
	const belief = "runs small, take size L"
	graph := fakes.NewRecordingEvidenceGraph([]string{belief, stub}, nil)
	ctx := t.Context()
	if err := graph.Derive(ctx, belief, "logic", time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := graph.RecordAction(ctx, stub, []string{belief}, nil, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("graph write of the stub failed: %v", err)
	}
	report, err := graph.Inspect(ctx, belief)
	if err != nil {
		t.Fatal(err)
	}
	if report.Dependents != 1 {
		t.Errorf("belief holds %d dependents, want the stub write to count as one", report.Dependents)
	}
	if got, err := StubDocID(stub); err != nil || got != docID {
		t.Errorf("stub round-trips to %q, %v; want %q", got, err, docID)
	}
}

// TestGraphStubErrors proves malformed stubs fail loudly: empty fields,
// prose-length keys, and content no stub builder produced.
func TestGraphStubErrors(t *testing.T) {
	if _, err := GraphStub("", "fit return 1"); !errors.Is(err, ErrEmptyText) {
		t.Errorf("empty doc id gives %v, want ErrEmptyText", err)
	}
	if _, err := GraphStub("doc-0001", "  "); !errors.Is(err, ErrEmptyText) {
		t.Errorf("empty key gives %v, want ErrEmptyText", err)
	}
	if _, err := GraphStub("doc-0001", strings.Repeat("word ", 30)); !errors.Is(err, ErrStubTooLong) {
		t.Errorf("prose-length key does not fail with ErrStubTooLong")
	}
	for _, bad := range []string{"", "just words", "evidence[", "evidence[] key"} {
		if _, err := StubDocID(bad); !errors.Is(err, ErrNoDocID) {
			t.Errorf("StubDocID(%q) gives %v, want ErrNoDocID", bad, err)
		}
	}
}
