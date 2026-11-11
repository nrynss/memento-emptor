package catalog

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubTransport serves recorded response bodies by request path, so the
// client test replays fixtures with no network and no key on the wire.
type stubTransport struct {
	t      *testing.T
	bodies map[string]string
	last   *http.Request
}

func (s *stubTransport) roundTrip(req *http.Request) (*http.Response, error) {
	s.last = req
	key := req.Method + " " + req.URL.Path
	body, ok := s.bodies[key]
	if !ok {
		s.t.Fatalf("unexpected live-shaped request %s: no recorded fixture", key)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

// mustReadFixture returns one recorded response body beside this package.
func mustReadFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir(t), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}

// stubClient wires the client to the recorded search and detail
// responses.
func stubClient(t *testing.T) (*stubTransport, *Client) {
	t.Helper()
	read := func(name string) string {
		return mustReadFixture(t, name)
	}
	stub := &stubTransport{t: t, bodies: map[string]string{
		"POST /v1/search":          read("search-oxford-shirts.json"),
		"GET /v1/products/tbnrl46": read("product-sunspel-oxford-white-tbnrl46.json"),
		"GET /v1/products/VWvTIPF": read("product-puma-darter-pro-2-VWvTIPF.json"),
	}}
	return stub, NewClient("fixture-key", "", stub.roundTrip)
}

// TestClientSearchReplaysFixture proves the client parses a recorded
// search: 10 products, the demo SKU first under Sunspel.
func TestClientSearchReplaysFixture(t *testing.T) {
	stub, client := stubClient(t)
	got, err := client.Search(context.Background(), "mens oxford button-down shirt", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 10 {
		t.Fatalf("search returned %d products, want 10", len(got))
	}
	first := got[0]
	if first.ID != "tbnrl46" || first.BrandID != "SMwk" || first.Category != "dress-shirts" {
		t.Errorf("first product is %+v, want the sunspel white oxford", first)
	}
	if h := stub.last.Header.Get("x-api-key"); h == "" {
		t.Error("search sent no x-api-key header")
	}
}

// TestClientSearchPumaReplaysFixture replays the second recorded search,
// which no other test read: the running-shoe page carries ten products
// and the Puma Darter Pro 2 with its category-specific footwear handle.
func TestClientSearchPumaReplaysFixture(t *testing.T) {
	stub := &stubTransport{t: t, bodies: map[string]string{
		"POST /v1/search": mustReadFixture(t, "search-puma-running-shoes.json"),
	}}
	client := NewClient("fixture-key", "", stub.roundTrip)
	got, err := client.Search(context.Background(), "puma running shoes", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 10 {
		t.Fatalf("search returned %d products, want 10", len(got))
	}
	var puma *Product
	for i := range got {
		if got[i].ID == "VWvTIPF" {
			puma = &got[i]
		}
	}
	if puma == nil {
		t.Fatalf("search holds no VWvTIPF: %v", got)
	}
	if puma.BrandID != "T0QO" || puma.Category != "athletic-shoes" {
		t.Errorf("puma is brand %q category %q", puma.BrandID, puma.Category)
	}
	if cut := puma.Fit["footwear-cut"]; len(cut) != 1 || cut[0] != "Low cut" {
		t.Errorf("footwear-cut is %v, want [Low cut]: the fit handle is per category", cut)
	}
	if puma.Variants == nil {
		t.Error("puma variants is nil, want the recorded Size option")
	}
}

// TestClientFetchReplaysFixture proves the client parses a recorded
// product detail: brand, category, Regular top-fit, one InStock retailer
// offer, and nil variants on apparel.
func TestClientFetchReplaysFixture(t *testing.T) {
	_, client := stubClient(t)
	p, err := client.Fetch(context.Background(), "tbnrl46")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if p.Title != "Men's Button Down Oxford Shirt" {
		t.Errorf("title is %q", p.Title)
	}
	if p.BrandName != "Sunspel" || p.Category != "dress-shirts" {
		t.Errorf("product is brand %q category %q", p.BrandName, p.Category)
	}
	if fit := p.Fit["top-fit"]; len(fit) != 1 || fit[0] != "Regular" {
		t.Errorf("top-fit is %v, want [Regular]", fit)
	}
	if len(p.Offers) != 1 || p.Offers[0].Availability != "InStock" {
		t.Errorf("offers are %+v, want one InStock offer", p.Offers)
	}
	if p.Offers[0].Currency != "USD" || p.Offers[0].Price != 195 {
		t.Errorf("offer price is %+v, want USD 195", p.Offers[0])
	}
	if p.Variants != nil {
		t.Error("variants is modelled, want nil: sizes live at the retailer")
	}
}

// TestClientRejectsBadInput proves the client fails loudly on empty
// queries, non-positive limits, empty ids and a missing key, instead of
// sending a request that invents products.
func TestClientRejectsBadInput(t *testing.T) {
	_, client := stubClient(t)
	ctx := context.Background()
	if _, err := client.Search(ctx, "  ", 10); err != ErrEmptyQuery {
		t.Errorf("empty query error is %v, want ErrEmptyQuery", err)
	}
	if _, err := client.Search(ctx, "oxford", 0); err != ErrBadLimit {
		t.Errorf("zero limit error is %v, want ErrBadLimit", err)
	}
	if _, err := client.Fetch(ctx, ""); err != ErrEmptyProductID {
		t.Errorf("empty id error is %v, want ErrEmptyProductID", err)
	}
	keyless := NewClient("", "", func(*http.Request) (*http.Response, error) {
		t.Fatal("request went out with no key")
		return nil, nil
	})
	if _, err := keyless.Search(ctx, "oxford", 10); err != ErrMissingAPIKey {
		t.Errorf("keyless search error is %v, want ErrMissingAPIKey", err)
	}
	if _, err := keyless.Fetch(ctx, "tbnrl46"); err != ErrMissingAPIKey {
		t.Errorf("keyless fetch error is %v, want ErrMissingAPIKey", err)
	}
}

// TestCatalogHoldsNoKeyMaterial proves the seam can hold: non-test files
// name no key variable, shell out to no keyring, and bake in no key. The
// key travels in from the environment through NewClient and goes out in
// the request header only.
func TestCatalogHoldsNoKeyMaterial(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), "_test.go") || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(".", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"CHANNEL3_API_KEY", "secret-tool", "keyring", "REDACTED"} {
			if strings.Contains(string(contents), banned) {
				t.Errorf("%s names %q: keys never enter the source", entry.Name(), banned)
			}
		}
	}
}
