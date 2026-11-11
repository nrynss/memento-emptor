package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultBaseURL is the Channel3 catalog API root. Calls send the key in
// the x-api-key header read from the environment; the key appears nowhere
// in this repository or in the recorded fixtures.
const DefaultBaseURL = "https://api.trychannel3.com/v1"

// Source is the seam a builder replaces: search and fetch over any
// product catalog. The Channel3 client below is one implementation; the
// matcher takes products from any source, so swapping the catalog changes
// no matching rule.
type Source interface {
	// Search returns up to limit products for the query.
	Search(ctx context.Context, query string, limit int) ([]Product, error)
	// Fetch returns the product detail for one catalog id.
	Fetch(ctx context.Context, id string) (Product, error)
}

// Compile-time proof the live client satisfies the seam a builder
// replaces.
var _ Source = (*Client)(nil)

// Client is the Channel3 Source: search and product detail over HTTP with
// an injected transport, so tests replay the recorded fixtures offline
// and only the re-verification call is live. The key travels in from the
// caller — read from the environment, never written to a file — and goes
// out in the x-api-key header.
type Client struct {
	apiKey string
	base   string
	do     func(*http.Request) (*http.Response, error)
}

// NewClient returns a Client sending to baseURL with apiKey through do.
// An empty baseURL selects DefaultBaseURL; a nil do sends through the
// default transport.
func NewClient(apiKey, baseURL string, do func(*http.Request) (*http.Response, error)) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if do == nil {
		do = http.DefaultClient.Do
	}
	return &Client{apiKey: apiKey, base: strings.TrimSuffix(baseURL, "/"), do: do}
}

// roundTrip sends one request and returns its body. A missing key, a
// non-200 status and an undecodable body all fail loudly instead of
// inventing products.
func (c *Client) roundTrip(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if c.apiKey == "" {
		return nil, ErrMissingAPIKey
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: context: %v", ErrBadResponse, err)
	}
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", ErrBadResponse, err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadResponse, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrBadResponse, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrBadResponse, resp.StatusCode)
	}
	return raw, nil
}

// searchReply is the wire shape of a search answer: the products plus the
// paging token for the next page. Source has no paging seam, so a search
// reads its first page and the token is not carried further.
type searchReply struct {
	Products      []json.RawMessage `json:"products"`
	NextPageToken string            `json:"next_page_token"`
}

// Search returns up to limit products for the query. A search costs one
// credit; detail calls are free. The guard is one search plus a couple of
// details: run one case first, check its cost, and stop past the budget.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Product, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrEmptyQuery
	}
	if limit <= 0 {
		return nil, ErrBadLimit
	}
	body, err := json.Marshal(map[string]any{"query": query, "limit": limit})
	if err != nil {
		return nil, fmt.Errorf("%w: encode search: %v", ErrBadResponse, err)
	}
	raw, err := c.roundTrip(ctx, http.MethodPost, "/search", body)
	if err != nil {
		return nil, err
	}
	var reply searchReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return nil, fmt.Errorf("%w: decode search: %v", ErrBadResponse, err)
	}
	out := make([]Product, 0, len(reply.Products))
	for _, item := range reply.Products {
		p, err := decodeProduct(item)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// Fetch returns the product detail for one catalog id.
func (c *Client) Fetch(ctx context.Context, id string) (Product, error) {
	if strings.TrimSpace(id) == "" {
		return Product{}, ErrEmptyProductID
	}
	raw, err := c.roundTrip(ctx, http.MethodGet, "/products/"+id, nil)
	if err != nil {
		return Product{}, err
	}
	return decodeProduct(raw)
}

// Catalog is the agent's shelf: one Source plus the match rule. The agent
// searches the catalog, then keeps only the products that share the
// belief's cut, so a recommendation cites the shared signals behind it.
type Catalog struct {
	// Source is the product catalog behind the shelf. A builder replaces
	// it by implementing Search and Fetch over their own catalog.
	Source Source
}

// Candidates searches for query and returns the products that share the
// want's cut, in catalog order. An invalid want matches nothing.
func (c Catalog) Candidates(ctx context.Context, query string, limit int, want Want) ([]Candidate, error) {
	products, err := c.Source.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	return Match(want, products), nil
}
