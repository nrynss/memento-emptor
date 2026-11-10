package fitindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// DefaultTimeout is the HTTP client's per-request timeout. The recorded
// cold-connection handshake took about 6.9 s, so a client left at the 5 s
// default would fail its first write after an idle spell; 20 s is above
// the measured cold maximum and inside a recommendation's budget.
const DefaultTimeout = 20 * time.Second

// maxResponseBytes bounds how much of an error response is read, so a
// misbehaving deployment cannot flood the log.
const maxResponseBytes = 1 << 20

// HTTPIndex is the fit index client for one index on an HTTP-reachable
// search deployment. It sends the recorded mapping on create, replaces
// whole documents at external versions, and deletes one shopper's rows by
// query. The deployment behind it is the one T0.12 recorded: index mode and
// settings come from the deployment, so the client sends no settings.
type HTTPIndex struct {
	baseURL string
	apiKey  string
	index   string
	client  *http.Client
}

// NewHTTPIndex builds a client for one index. The base URL is the
// deployment's HTTPS endpoint with no index path; the API key is sent as
// the deployment's ApiKey authorization; the index name is the index this
// client owns. All three are required.
func NewHTTPIndex(baseURL, apiKey, index string) (*HTTPIndex, error) {
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedEndpoint, err)
	}
	if (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" {
		return nil, fmt.Errorf("%w: %q is not an absolute http or https URL", ErrMalformedEndpoint, baseURL)
	}
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}
	if index == "" {
		return nil, ErrMissingIndexName
	}
	return &HTTPIndex{
		baseURL: endpoint.String(),
		apiKey:  apiKey,
		index:   index,
		client: &http.Client{
			Timeout: DefaultTimeout,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				MaxIdleConnsPerHost: 4,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}, nil
}

// Create makes the index with the recorded mapping. An index that already
// exists is not an error, so every start-up may call it.
func (h *HTTPIndex) Create(ctx context.Context) error {
	status, body, err := h.do(ctx, http.MethodPut, "/"+url.PathEscape(h.index), nil, Mapping())
	if err != nil {
		return fmt.Errorf("fitindex: create index %q: %w", h.index, err)
	}
	if status == http.StatusOK || status == http.StatusCreated {
		return nil
	}
	if status == http.StatusBadRequest && bytes.Contains(body, []byte("resource_already_exists_exception")) {
		return nil
	}
	return fmt.Errorf("%w: create index %q: status %d: %s", ErrIndexStatus, h.index, status, excerpt(body))
}

// Put replaces one purchase's whole document at the row's version with
// version_type external_gte, so a replayed event rewrites the same row and
// an older one is refused with ErrOlderEvent. The write asks for a refresh,
// so a read that follows a purchase sees its row.
func (h *HTTPIndex) Put(ctx context.Context, row domain.FitRow) error {
	if row.TxnHash == "" {
		return fmt.Errorf("fitindex: put row: %w", ErrMissingTransaction)
	}
	version := strconv.FormatInt(row.Version(), 10)
	query := url.Values{
		"version":      {version},
		"version_type": {"external_gte"},
		"refresh":      {"true"},
	}
	status, body, err := h.do(ctx, http.MethodPut,
		"/"+url.PathEscape(h.index)+"/_doc/"+url.PathEscape(row.TxnHash), query, documentFor(row))
	if err != nil {
		return fmt.Errorf("fitindex: put row %q: %w", row.TxnHash, err)
	}
	if status == http.StatusOK || status == http.StatusCreated {
		return nil
	}
	if status == http.StatusConflict {
		return fmt.Errorf("%w: row %q at version %s", ErrOlderEvent, row.TxnHash, version)
	}
	return fmt.Errorf("%w: put row %q: status %d: %s", ErrIndexStatus, row.TxnHash, status, excerpt(body))
}

// DeleteShopper removes every row one shopper wrote in one line, matching
// the three line fields and the per-line shopper hash, and reports how many
// documents went. Conflicting concurrent writes are retried by the
// deployment rather than aborting the delete, because an opt-out must
// remove every row it names. The deployment reports per-document failures
// in the same answer, so a delete that removed only part of a shopper's
// rows fails with ErrIncompleteDelete and the caller knows the opt-out did
// not fully happen.
func (h *HTTPIndex) DeleteShopper(ctx context.Context, line domain.ProductLine, shopperHash string) (int, error) {
	switch {
	case line.Brand == "" || line.Category == "" || line.FitHandle == "" || shopperHash == "":
		return 0, ErrMissingLine
	}
	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{
					map[string]any{"term": map[string]any{"brand": line.Brand}},
					map[string]any{"term": map[string]any{"category": line.Category}},
					map[string]any{"term": map[string]any{"fit_handle": line.FitHandle}},
					map[string]any{"term": map[string]any{"shopper_hash": shopperHash}},
				},
			},
		},
	}
	params := url.Values{"refresh": {"true"}, "conflicts": {"proceed"}}
	status, body, err := h.do(ctx, http.MethodPost,
		"/"+url.PathEscape(h.index)+"/_delete_by_query", params, query)
	if err != nil {
		return 0, fmt.Errorf("fitindex: delete rows for line %q/%q/%q: %w",
			line.Brand, line.Category, line.FitHandle, err)
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("%w: delete rows for line %q/%q/%q: status %d: %s",
			ErrIndexStatus, line.Brand, line.Category, line.FitHandle, status, excerpt(body))
	}
	var result struct {
		Deleted  int               `json:"deleted"`
		Failures []json.RawMessage `json:"failures"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("fitindex: read delete response: %w", err)
	}
	if len(result.Failures) > 0 {
		return result.Deleted, fmt.Errorf("%w: line %q/%q/%q: %d rows deleted, %d failed",
			ErrIncompleteDelete, line.Brand, line.Category, line.FitHandle,
			result.Deleted, len(result.Failures))
	}
	return result.Deleted, nil
}

// do sends one request and returns the status and body. A transport
// failure is returned as an error; an error status is the caller's to
// classify.
func (h *HTTPIndex) do(ctx context.Context, method, path string, query url.Values, body any) (int, []byte, error) {
	target := h.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var payload io.Reader
	switch value := body.(type) {
	case nil:
	case json.RawMessage:
		payload = bytes.NewReader(value)
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return 0, nil, fmt.Errorf("encode request: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, payload)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Authorization", "ApiKey "+h.apiKey)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := h.client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("read response: %w", err)
	}
	return response.StatusCode, responseBody, nil
}

// excerpt shortens a response body for an error message. It reports how
// much was left out rather than truncating silently.
func excerpt(body []byte) string {
	const width = 200
	if len(body) <= width {
		return string(body)
	}
	return fmt.Sprintf("%s… (%d bytes)", body[:width], len(body))
}
