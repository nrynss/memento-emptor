package pool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// DefaultTimeout is the HTTP client's per-request timeout. T0.12 measured
// about 6.9 s for a cold TLS handshake and about 236 ms warm, so a client
// left at the 5 s default would fail its first read after an idle spell;
// 20 s is above the measured cold maximum and inside a recommendation's
// budget. The transport keeps connections alive, so warm reads reuse one.
const DefaultTimeout = 20 * time.Second

// maxResponseBytes bounds how much of a response is read, so a misbehaving
// deployment cannot flood the log.
const maxResponseBytes = 1 << 20

// HTTPOption configures an HTTPIndex.
type HTTPOption func(*HTTPIndex)

// WithTimeout sets the per-request timeout. A duration that is not positive
// leaves DefaultTimeout in place.
func WithTimeout(timeout time.Duration) HTTPOption {
	return func(h *HTTPIndex) {
		if timeout > 0 {
			h.client.Timeout = timeout
		}
	}
}

// HTTPIndex is the pool reader's client for one index on an HTTP-reachable
// search deployment. It runs the one line-and-size aggregation and keeps its
// connections alive, so a read after a cold start does not pay a second
// handshake.
type HTTPIndex struct {
	baseURL string
	apiKey  string
	index   string
	client  *http.Client
}

// NewHTTPIndex builds a client for one index. The base URL is the
// deployment's HTTPS endpoint with no index path; the API key is sent as the
// deployment's ApiKey authorization; the index name is the index the reader
// owns. All three are required.
func NewHTTPIndex(baseURL, apiKey, index string, opts ...HTTPOption) (*HTTPIndex, error) {
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
	h := &HTTPIndex{
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
	}
	for _, opt := range opts {
		opt(h)
	}
	return h, nil
}

// Warm opens or reuses the index connection with one HEAD request. Any
// answer, including a 404 for an index that does not exist yet, means the
// connection is up, so only a transport failure is an error.
func (h *HTTPIndex) Warm(ctx context.Context) error {
	_, _, err := h.do(ctx, http.MethodHead, "/"+url.PathEscape(h.index), nil, nil)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("pool: warm index %q: %w", h.index, ctxErr)
		}
		return fmt.Errorf("%w: warm index %q: %v", ErrUnavailable, h.index, err)
	}
	return nil
}

// Aggregate runs the line-and-size aggregation over settled rows and returns
// the counts the pool rule reads. A transport failure, including the
// per-request timeout, is ErrUnavailable; a non-200 answer is
// ErrIndexStatus; a partial or unreadable answer is ErrIncomplete or
// ErrMalformedResponse. A line with no settled rows is not an error: it is a
// valid empty pool.
func (h *HTTPIndex) Aggregate(ctx context.Context, q Query) (Result, error) {
	if err := usableQuery(q.Line, q.Size); err != nil {
		return Result{}, fmt.Errorf("pool: aggregate: %w", err)
	}
	if q.Since.IsZero() {
		return Result{}, fmt.Errorf("pool: aggregate: %w: no recent window", ErrMissingQuery)
	}
	status, body, err := h.do(ctx, http.MethodPost,
		"/"+url.PathEscape(h.index)+"/_search", nil, aggregationBody(q))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Result{}, fmt.Errorf("pool: aggregate line %q/%q/%q: %w",
				q.Line.Brand, q.Line.Category, q.Line.FitHandle, ctxErr)
		}
		return Result{}, fmt.Errorf("%w: aggregate line %q/%q/%q: %v",
			ErrUnavailable, q.Line.Brand, q.Line.Category, q.Line.FitHandle, err)
	}
	if status != http.StatusOK {
		return Result{}, fmt.Errorf("%w: aggregate line %q/%q/%q: status %d: %s",
			ErrIndexStatus, q.Line.Brand, q.Line.Category, q.Line.FitHandle, status, excerpt(body))
	}
	return parseAggregation(body)
}

// aggregationBody is the one line-and-size query: T0.12's recorded query,
// with its all-settled shopper cardinality replaced by the fit-classified
// count the Building floor reads, per-direction shopper counts replaced by
// the union of fit returns and fit feedback the Earned floor reads, and a
// second aggregation over the recent window alone.
func aggregationBody(q Query) map[string]any {
	filters := lineFilters(q)
	return map[string]any{
		"size":             0,
		"track_total_hits": true,
		"query":            map[string]any{"bool": map[string]any{"filter": filters}},
		"aggs": map[string]any{
			"split": map[string]any{
				"filters": map[string]any{"filters": map[string]any{
					"all":    map[string]any{"match_all": map[string]any{}},
					"seeded": term("seeded", true),
					"live":   term("seeded", false),
				}},
				"aggs": map[string]any{
					"settled": map[string]any{
						"filter": term("settled", true),
						"aggs":   settledAggs(),
					},
				},
			},
			"recent": map[string]any{
				"filter": map[string]any{"bool": map[string]any{"filter": append(
					append([]any{}, filters...),
					term("settled", true),
					map[string]any{"range": map[string]any{"settled_at": map[string]any{
						"gte": q.Since.UTC().Format(time.RFC3339),
					}}},
				)}},
				"aggs": settledAggs(),
			},
		},
	}
}

// lineFilters names the three line fields and the size, every one a term, so
// the aggregation reads one bucket of purchases.
func lineFilters(q Query) []any {
	return []any{
		term("brand", q.Line.Brand),
		term("category", q.Line.Category),
		term("fit_handle", q.Line.FitHandle),
		term("size", q.Size),
	}
}

// The settled-kind labels the index stores. The writer takes them from the
// domain, so the reader does too: a second copy of a label here could drift
// from what the rows carry.
const (
	indexSettledReturnReason = string(domain.FitSettledByReturnReason)
	indexSettledFeedback     = string(domain.FitSettledByFeedback)
)

// settledAggs is the settled part of the query: the denominator, the seeded
// kinds, the reason categories, the fit returns by direction, the feedback
// by verdict, the fit-classified shopper count the Building floor reads,
// and the per-direction union shopper count the Earned floor reads. A
// direction is fit returns that way together with fit feedback carrying
// that verdict, counted as one distinct-shopper cardinality so a shopper
// with both kinds of evidence counts once.
func settledAggs() map[string]any {
	return map[string]any{
		"fit_shoppers": map[string]any{
			"filter": map[string]any{"bool": map[string]any{
				"should": []any{
					map[string]any{"bool": map[string]any{"filter": []any{
						term("settled_kind", indexSettledReturnReason),
						map[string]any{"terms": map[string]any{"reason_category": []any{
							string(domain.CategoryFitTooSmall),
							string(domain.CategoryFitTooLarge),
						}}},
					}}},
					term("settled_kind", indexSettledFeedback),
				},
				"minimum_should_match": 1,
			}},
			"aggs": map[string]any{"shoppers": cardinality("shopper_hash")},
		},
		"settled_kind":    map[string]any{"terms": map[string]any{"field": "settled_kind"}},
		"reason_category": map[string]any{"terms": map[string]any{"field": "reason_category"}},
		"fit_returns": map[string]any{
			"filter": term("settled_kind", indexSettledReturnReason),
			"aggs": map[string]any{"direction": map[string]any{
				"terms": map[string]any{"field": "fit_direction"},
			}},
		},
		"feedback": map[string]any{
			"filter": term("settled_kind", indexSettledFeedback),
			"aggs": map[string]any{"verdict": map[string]any{
				"terms": map[string]any{"field": "feedback_verdict"},
			}},
		},
		"direction_shoppers": map[string]any{
			"filters": map[string]any{"filters": map[string]any{
				indexDirectionTooSmall: directionFilter(indexDirectionTooSmall, string(domain.FitVerdictTooSmall)),
				indexDirectionTooLarge: directionFilter(indexDirectionTooLarge, string(domain.FitVerdictTooLarge)),
			}},
			"aggs": map[string]any{"shoppers": cardinality("shopper_hash")},
		},
	}
}

// directionFilter matches settled rows pointing one way: a fit return in
// that direction, or fit feedback carrying that verdict. Either clause is
// enough; the cardinality above counts each shopper once.
func directionFilter(direction, verdict string) map[string]any {
	return map[string]any{"bool": map[string]any{
		"should": []any{
			map[string]any{"bool": map[string]any{"filter": []any{
				term("settled_kind", indexSettledReturnReason),
				term("fit_direction", direction),
			}}},
			map[string]any{"bool": map[string]any{"filter": []any{
				term("settled_kind", indexSettledFeedback),
				term("feedback_verdict", verdict),
			}}},
		},
		"minimum_should_match": 1,
	}}
}

// term builds one term filter.
func term(field string, value any) map[string]any {
	return map[string]any{"term": map[string]any{field: value}}
}

// cardinality counts distinct values under one bucket.
func cardinality(field string) map[string]any {
	return map[string]any{"cardinality": map[string]any{"field": field}}
}

// searchResponse is the part of the index's answer the reader trusts. Every
// count the pool reports comes from a bucket named here.
type searchResponse struct {
	TimedOut bool `json:"timed_out"`
	Shards   struct {
		Failed int `json:"failed"`
	} `json:"_shards"`
	Aggregations struct {
		Split struct {
			Buckets map[string]*splitBucket `json:"buckets"`
		} `json:"split"`
		Recent *settledBucket `json:"recent"`
	} `json:"aggregations"`
}

// splitBucket is one seeded or live bucket of the split aggregation.
type splitBucket struct {
	DocCount int            `json:"doc_count"`
	Settled  *settledBucket `json:"settled"`
}

// settledBucket is the settled aggregation over one bucket of rows.
type settledBucket struct {
	DocCount          int               `json:"doc_count"`
	FitShoppers       *filterCount      `json:"fit_shoppers"`
	SettledKind       *termsCount       `json:"settled_kind"`
	ReasonCategory    *termsCount       `json:"reason_category"`
	FitReturns        *filterCount      `json:"fit_returns"`
	Feedback          *filterCount      `json:"feedback"`
	DirectionShoppers *directionBuckets `json:"direction_shoppers"`
}

// directionBuckets is the per-direction union shopper count: one bucket per
// direction, each a filter of fit returns that way together with fit
// feedback carrying that verdict, with its distinct-shopper cardinality.
type directionBuckets struct {
	Buckets map[string]*filterCount `json:"buckets"`
}

// filterCount is a filter aggregation with its own counts. FitReturns and
// Feedback carry a terms aggregation beside their doc count, named by the
// caller through termsCount.
type filterCount struct {
	DocCount  int         `json:"doc_count"`
	Shoppers  *countValue `json:"shoppers"`
	Direction *termsCount `json:"direction"`
	Verdict   *termsCount `json:"verdict"`
}

// termsCount is one terms aggregation.
type termsCount struct {
	Buckets []termCount `json:"buckets"`
}

// termCount is one terms bucket, with the distinct-shopper count when the
// query asked for one.
type termCount struct {
	Key      string      `json:"key"`
	DocCount int         `json:"doc_count"`
	Shoppers *countValue `json:"shoppers"`
}

// countValue is a cardinality aggregation's value.
type countValue struct {
	Value int `json:"value"`
}

// parseAggregation reads the index's answer into the counts the pool
// reports. A partial answer is refused rather than reported, and so is an
// answer missing a bucket the query asked for: a zero in place of an answer
// would let the pool pretend a line is quiet.
func parseAggregation(body []byte) (Result, error) {
	var answer searchResponse
	if err := json.Unmarshal(body, &answer); err != nil {
		return Result{}, fmt.Errorf("%w: decode answer: %v", ErrMalformedResponse, err)
	}
	if answer.TimedOut {
		return Result{}, fmt.Errorf("%w: the query timed out", ErrIncomplete)
	}
	if answer.Shards.Failed > 0 {
		return Result{}, fmt.Errorf("%w: %d shard(s) failed", ErrIncomplete, answer.Shards.Failed)
	}
	all, err := bucketCounts("all", answer.Aggregations.Split.Buckets)
	if err != nil {
		return Result{}, err
	}
	seeded, err := bucketCounts("seeded", answer.Aggregations.Split.Buckets)
	if err != nil {
		return Result{}, err
	}
	live, err := bucketCounts("live", answer.Aggregations.Split.Buckets)
	if err != nil {
		return Result{}, err
	}
	if answer.Aggregations.Recent == nil {
		return Result{}, fmt.Errorf("%w: the recent window is missing", ErrMalformedResponse)
	}
	recent, err := countsIn("recent", answer.Aggregations.Recent)
	if err != nil {
		return Result{}, err
	}
	return Result{All: all, Seeded: seeded, Live: live, Recent: recent}, nil
}

// bucketCounts reads one bucket of the split aggregation.
func bucketCounts(name string, buckets map[string]*splitBucket) (Counts, error) {
	bucket, ok := buckets[name]
	if !ok || bucket == nil {
		return Counts{}, fmt.Errorf("%w: the %s split bucket is missing", ErrMalformedResponse, name)
	}
	if bucket.Settled == nil {
		return Counts{}, fmt.Errorf("%w: the %s settled bucket is missing", ErrMalformedResponse, name)
	}
	return countsIn(name, bucket.Settled)
}

// countsIn reads one settled bucket into the counts the pool reports.
func countsIn(name string, bucket *settledBucket) (Counts, error) {
	if bucket.FitShoppers == nil || bucket.FitShoppers.Shoppers == nil {
		return Counts{}, fmt.Errorf("%w: the %s fit-shopper count is missing", ErrMalformedResponse, name)
	}
	if bucket.SettledKind == nil || bucket.ReasonCategory == nil ||
		bucket.FitReturns == nil || bucket.FitReturns.Direction == nil ||
		bucket.Feedback == nil || bucket.Feedback.Verdict == nil ||
		bucket.DirectionShoppers == nil {
		return Counts{}, fmt.Errorf("%w: the %s settled counts are incomplete", ErrMalformedResponse, name)
	}
	counts := Counts{
		Settled: bucket.DocCount,
		Reasons: make(map[domain.Category]int, len(bucket.ReasonCategory.Buckets)),
		Shoppers: ShopperCounts{
			Fit: bucket.FitShoppers.Shoppers.Value,
		},
	}
	for _, kind := range bucket.SettledKind.Buckets {
		if kind.Key == string(domain.FitSettledByKept) {
			counts.Kept = kind.DocCount
		}
	}
	for _, reason := range bucket.ReasonCategory.Buckets {
		counts.Reasons[domain.Category(reason.Key)] = reason.DocCount
	}
	for _, direction := range bucket.FitReturns.Direction.Buckets {
		switch direction.Key {
		case indexDirectionTooSmall:
			counts.Returns.TooSmall = direction.DocCount
		case indexDirectionTooLarge:
			counts.Returns.TooLarge = direction.DocCount
		}
	}
	for _, verdict := range bucket.Feedback.Verdict.Buckets {
		switch verdict.Key {
		case string(domain.FitVerdictFitsWell):
			counts.Feedback.FitsWell = verdict.DocCount
		case string(domain.FitVerdictTooSmall):
			counts.Feedback.TooSmall = verdict.DocCount
		case string(domain.FitVerdictTooLarge):
			counts.Feedback.TooLarge = verdict.DocCount
		}
	}
	for _, direction := range []struct {
		key   string
		place *int
	}{
		{indexDirectionTooSmall, &counts.Shoppers.TooSmall},
		{indexDirectionTooLarge, &counts.Shoppers.TooLarge},
	} {
		union, ok := bucket.DirectionShoppers.Buckets[direction.key]
		if !ok || union == nil || union.Shoppers == nil {
			return Counts{}, fmt.Errorf("%w: the %s direction %q has no union shopper count",
				ErrMalformedResponse, name, direction.key)
		}
		*direction.place = union.Shoppers.Value
	}
	return counts, nil
}

// The direction words the index stores. The domain's words are small and
// large; the recorded mapping's rows say too_small and too_large.
const (
	indexDirectionTooSmall = "too_small"
	indexDirectionTooLarge = "too_large"
)

// do sends one request and returns the status and body. A transport failure
// is returned as an error; an error status is the caller's to classify.
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

// excerpt shortens a response body for an error message. It reports how much
// was left out rather than truncating silently.
func excerpt(body []byte) string {
	const width = 200
	if len(body) <= width {
		return string(body)
	}
	return fmt.Sprintf("%s… (%d bytes)", body[:width], len(body))
}
