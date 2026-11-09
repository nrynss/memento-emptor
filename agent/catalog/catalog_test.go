package catalog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureDir returns the recorded Channel3 responses beside this package.
func fixtureDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "testdata", "channel3")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("testdata/channel3 is missing at %s: %v", dir, err)
	}
	return dir
}

// loadFixture parses one recorded response file into a Product.
func loadFixture(t *testing.T, name string) Product {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir(t), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	p, err := decodeProduct(raw)
	if err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return p
}

// sunspelLine loads the three demo-line SKUs plus the two-product
// negative control, in recording order.
func sunspelLine(t *testing.T) []Product {
	t.Helper()
	return []Product{
		loadFixture(t, "product-sunspel-oxford-white-tbnrl46.json"),
		loadFixture(t, "product-sunspel-oxford-lightblue-VyAkjCA.json"),
		loadFixture(t, "product-sunspel-oxford-stripe-PSW8GeK.json"),
		loadFixture(t, "product-bonobos-everyday-oxford-stripe-V9BYM3z.json"),
		loadFixture(t, "product-bonobos-everyday-oxford-indigo-m4iRrEM.json"),
	}
}

// sunspelWant is the belief's cut for the demo line: Sunspel,
// dress-shirts, Regular top-fit.
func sunspelWant() Want {
	return Want{
		BrandID:      "SMwk",
		Category:     "dress-shirts",
		FitAttribute: "top-fit",
		FitValues:    []string{"Regular"},
	}
}

// TestMatchSunspelLine proves the demo match: the three Sunspel SKUs
// share brand, category and equal top-fit, so all three are candidates,
// and each carries the shared signals as its evidence.
func TestMatchSunspelLine(t *testing.T) {
	got := Match(sunspleWantFix(t), sunspelLine(t))
	if len(got) != 3 {
		t.Fatalf("match returned %d candidates, want 3", len(got))
	}
	wantIDs := []string{"tbnrl46", "VyAkjCA", "PSW8GeK"}
	for i, c := range got {
		if c.Product.ID != wantIDs[i] {
			t.Errorf("candidate %d is %s, want %s", i, c.Product.ID, wantIDs[i])
		}
		if len(c.Shared) != 3 {
			t.Errorf("candidate %s carries %d shared signals, want 3", c.Product.ID, len(c.Shared))
		}
		if c.Fulfilment != FulfilmentSimulated {
			t.Errorf("candidate %s fulfilment is not labelled simulated", c.Product.ID)
		}
		if c.Product.BrandName != "Sunspel" {
			t.Errorf("candidate %s brand name is %q, want Sunspel", c.Product.ID, c.Product.BrandName)
		}
	}
}

// sunspleWantFix keeps the test above readable: the belief's cut.
func sunspleWantFix(t *testing.T) Want {
	t.Helper()
	return sunspelWant()
}

// TestMatchRejectsLineNameAlone proves the negative control: the two
// Bonobos Everyday Oxfords share a line name, but their top-fit labels
// diverge, so wanting Slim matches only the Slim SKU and never the
// Slim-plus-Tailored one.
func TestMatchRejectsLineNameAlone(t *testing.T) {
	products := sunspelLine(t)[3:]
	slim := Want{BrandID: "ISZ8", Category: "dress-shirts", FitAttribute: "top-fit", FitValues: []string{"Slim"}}
	got := Match(slim, products)
	if len(got) != 1 || got[0].Product.ID != "V9BYM3z" {
		t.Fatalf("slim match returned %v, want only V9BYM3z", ids(got))
	}
	tailored := Want{BrandID: "ISZ8", Category: "dress-shirts", FitAttribute: "top-fit", FitValues: []string{"Slim", "Tailored"}}
	got = Match(tailored, products)
	if len(got) != 1 || got[0].Product.ID != "m4iRrEM" {
		t.Fatalf("slim-tailored match returned %v, want only m4iRrEM", ids(got))
	}
}

func ids(cs []Candidate) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Product.ID)
	}
	return out
}

// TestMatchIncompleteWantMatchesNothing proves an incomplete cut never
// matches: matching on a missing part would repeat the line-name mistake.
func TestMatchIncompleteWantMatchesNothing(t *testing.T) {
	products := sunspelLine(t)
	for _, want := range []Want{
		{Category: "dress-shirts", FitAttribute: "top-fit", FitValues: []string{"Regular"}},
		{BrandID: "SMwk", FitAttribute: "top-fit", FitValues: []string{"Regular"}},
		{BrandID: "SMwk", Category: "dress-shirts", FitValues: []string{"Regular"}},
		{BrandID: "SMwk", Category: "dress-shirts", FitAttribute: "top-fit"},
	} {
		if got := Match(want, products); len(got) != 0 {
			t.Errorf("incomplete want %+v matched %d products, want none", want, len(got))
		}
	}
}

// TestMatchExcludesReturnedSKUs proves the agent gets a new SKU: the
// returned ones are skipped so the candidate differs from what went back.
func TestMatchExcludesReturnedSKUs(t *testing.T) {
	want := sunspelWant()
	want.ExcludeIDs = []string{"tbnrl46", "VyAkjCA", "PSW8GeK"}
	if got := Match(want, sunspelLine(t)); len(got) != 0 {
		t.Errorf("excluded line matched %d products, want none", len(got))
	}
	want.ExcludeIDs = []string{"tbnrl46"}
	got := Match(want, sunspelLine(t))
	if len(got) != 2 || got[0].Product.ID != "VyAkjCA" {
		t.Errorf("excluded first SKU, got %v", ids(got))
	}
}

// TestApparelMarksSizesOutOfBand proves where the data ends: variants is
// null on every apparel product fetched, so the candidate says size
// availability lives at the retailer's offer URL, recorded out-of-band.
func TestApparelMarksSizesOutOfBand(t *testing.T) {
	p := loadFixture(t, "product-sunspel-oxford-white-tbnrl46.json")
	if p.Variants != nil {
		t.Fatalf("sunspel variants is modelled, want nil: sizes live at the retailer")
	}
	got := Match(sunspleWantFix(t), []Product{p})
	if len(got) != 1 {
		t.Fatalf("match returned %d candidates, want 1", len(got))
	}
	c := got[0]
	if len(c.AvailableSizes) != 0 {
		t.Errorf("candidate lists sizes %v, want none from the catalog", c.AvailableSizes)
	}
	if !strings.Contains(c.SizeNote, "retailer offer URL") || !strings.Contains(c.SizeNote, "out-of-band") {
		t.Errorf("size note does not point out-of-band: %q", c.SizeNote)
	}
	if len(p.Offers) == 0 || p.Offers[0].Domain != "us.sunspel.com" {
		t.Errorf("offer is not the sunspel retailer page: %+v", p.Offers)
	}
}

// TestFootwearListsInStockSizes proves the variant shape where the
// catalog models it: the Puma shoe lists its InStock sizes, and a
// narrowed selection echoes back.
func TestFootwearListsInStockSizes(t *testing.T) {
	p := loadFixture(t, "product-puma-darter-pro-2-VWvTIPF.json")
	if p.Variants == nil {
		t.Fatal("puma variants is null, want the modelled Size option")
	}
	sizes := p.InStockSizes()
	if len(sizes) != 5 {
		t.Fatalf("instock sizes are %v, want the 5 recorded InStock labels", sizes)
	}
	if sizes[0] != "7.5" || sizes[len(sizes)-1] != "10.5" {
		t.Errorf("instock sizes are %v, want 7.5 through 10.5", sizes)
	}
	narrowed := loadFixture(t, "product-puma-darter-pro-2-size10-VWvTIPF.json")
	if narrowed.Selected["Size"] != "10" {
		t.Errorf("narrowed selection is %v, want Size 10 echoed", narrowed.Selected)
	}
	if len(narrowed.Offers) != 1 {
		t.Errorf("narrowed offers are %d, want the 1 matching offer", len(narrowed.Offers))
	}
}

// TestSizeParamIgnoredOnNullVariants pins the research finding: the
// option_Size parameter changes nothing on a product with null variants,
// so the demo never reads sizes from Channel3 on apparel.
func TestSizeParamIgnoredOnNullVariants(t *testing.T) {
	plain := loadFixture(t, "product-sunspel-oxford-white-tbnrl46.json")
	sized := loadFixture(t, "product-sunspel-oxford-white-sizeM-ignored-tbnrl46.json")
	if sized.ID != plain.ID || sized.Variants != nil {
		t.Errorf("size-M request changed the product: id %s variants-modelled %v", sized.ID, sized.Variants != nil)
	}
}

// fixtureSource serves the recorded search file behind the Source seam,
// so the shelf test replays fixtures with no network.
type fixtureSource struct {
	t        *testing.T
	products []Product
}

func (s fixtureSource) Search(_ context.Context, query string, limit int) ([]Product, error) {
	if query == "" {
		return nil, ErrEmptyQuery
	}
	if limit <= 0 {
		return nil, ErrBadLimit
	}
	if limit > len(s.products) {
		limit = len(s.products)
	}
	return s.products[:limit], nil
}

func (s fixtureSource) Fetch(_ context.Context, id string) (Product, error) {
	for _, p := range s.products {
		if p.ID == id {
			return p, nil
		}
	}
	return Product{}, ErrEmptyProductID
}

// TestCatalogCandidatesEndToEnd proves the shelf: search the recorded
// oxford results, keep the belief's cut, skip the returned SKUs, and the
// candidate is a new SKU with its evidence and its marks.
func TestCatalogCandidatesEndToEnd(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(fixtureDir(t), "search-oxford-shirts.json"))
	if err != nil {
		t.Fatalf("read search fixture: %v", err)
	}
	var generic struct {
		Products []json.RawMessage `json:"products"`
	}
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("decode search fixture: %v", err)
	}
	products := make([]Product, 0, len(generic.Products))
	for _, item := range generic.Products {
		p, err := decodeProduct(item)
		if err != nil {
			t.Fatalf("decode search product: %v", err)
		}
		products = append(products, p)
	}
	if len(products) != 10 {
		t.Fatalf("search fixture holds %d products, want 10", len(products))
	}
	shelf := Catalog{Source: fixtureSource{t: t, products: products}}
	want := sunspelWant()
	want.ExcludeIDs = []string{"tbnrl46"}
	got, err := shelf.Candidates(context.Background(), "mens oxford button-down shirt", 10, want)
	if err != nil {
		t.Fatalf("candidates: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("candidates are %v, want the two remaining sunspel SKUs", ids(got))
	}
	if got[0].Product.ID != "VyAkjCA" || got[1].Product.ID != "PSW8GeK" {
		t.Errorf("candidates are %v, want VyAkjCA then PSW8GeK", ids(got))
	}
}
