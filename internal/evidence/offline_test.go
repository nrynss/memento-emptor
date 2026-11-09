package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/fakes"
)

// TestEvidenceSourceHasNoKeysOrNetwork proves the seam can hold: this
// package names no key variable, shells out to no keyring, imports no HTTP
// client or service hostname, and bakes in no model, endpoint or
// dimensions. A store that phones home cannot pass this.
func TestEvidenceSourceHasNoKeysOrNetwork(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(".", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		text := string(contents)
		for _, banned := range fakes.KeyVariables() {
			if strings.Contains(text, banned) {
				t.Errorf("%s names %q", entry.Name(), banned)
			}
		}
		for _, banned := range []string{
			"secret-tool",
			"net/http",
			"http.Client",
			"http.Get",
			"http.Post",
			"openrouter.ai",
			"trychannel3.com",
			"baai/",
			"bge",
			"api-m.sandbox.paypal.com",
		} {
			if strings.Contains(text, banned) {
				t.Errorf("%s names %q", entry.Name(), banned)
			}
		}
	}
}

// TestStoreRunsOffline proves the store answers with no key and no
// network: keys unset, proxies broken, and the whole path — index,
// candidate search, ledger search and line matching — still passes.
func TestStoreRunsOffline(t *testing.T) {
	for _, name := range fakes.KeyVariables() {
		t.Setenv(name, "")
	}
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("ALL_PROXY", "http://127.0.0.1:1")
	if err := fakes.AssertKeysAbsent(); err != nil {
		t.Fatalf("offline precondition failed: %v", err)
	}

	store, err := NewStore(stubContract{kind: "test-kind"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	line := Line{BrandID: "brand-x-001", Category: "dress-shirts", FitHandle: "Regular"}
	returnDoc := Document{Kind: KindReason, Text: "Brand X oxford shirt, size M: too tight in the shoulders", Line: line}
	id, err := store.IndexDocument(ctx, returnDoc)
	if err != nil {
		t.Fatal(err)
	}
	hits, err := store.Search(ctx, "Brand X oxford shirt")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].DocID != id {
		t.Fatalf("offline search = %+v, want the one indexed document", hits)
	}
	rows, err := store.LedgerSearch("tight")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].DocID != id {
		t.Fatalf("offline ledger search = %+v, want the one indexed row", rows)
	}
	report := MatchCandidate(Candidate{Line: line, SKU: "new-sku"}, store.Documents())
	if !report.Matches || len(report.MatchedDocIDs) != 1 || report.MatchedDocIDs[0] != id {
		t.Errorf("offline match = %+v, want the indexed return", report)
	}
}
