package pool

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// recordedFixture reads one recording from testdata/elastic-fit.
func recordedFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "elastic-fit", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

// decodeObject decodes raw JSON into an object, failing on anything else.
func decodeObject(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode JSON object: %v", err)
	}
	return value
}

// mustMarshal encodes a value the tests serve.
func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
	return raw
}

// at walks an object path and returns the value there.
func at(t *testing.T, root any, path ...string) any {
	t.Helper()
	current := root
	for _, name := range path {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("path %v: %q is not inside an object", path, name)
		}
		value, ok := object[name]
		if !ok {
			t.Fatalf("path %v: %q is missing", path, name)
		}
		current = value
	}
	return current
}

// atArray is at for a JSON array.
func atArray(t *testing.T, root any, path ...string) []any {
	t.Helper()
	value := at(t, root, path...)
	array, ok := value.([]any)
	if !ok {
		t.Fatalf("path %v is %T, want an array", path, value)
	}
	return array
}

// requireTermValue finds one term filter in a filter array and checks its
// value.
func requireTermValue(t *testing.T, filters []any, field string, want any) {
	t.Helper()
	for _, raw := range filters {
		filter, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		term, ok := filter["term"].(map[string]any)
		if !ok {
			continue
		}
		value, ok := term[field]
		if !ok {
			continue
		}
		if !reflect.DeepEqual(value, want) {
			t.Errorf("filter on %q is %v, want %v", field, value, want)
		}
		return
	}
	t.Errorf("no term filter on %q", field)
}

// TestAggregateSendsTheRecordedQuery pins the one line-and-size query: the
// recorded bucket filters, the fit-shopper count the Building floor reads,
// and the recent window on settled event time.
func TestAggregateSendsTheRecordedQuery(t *testing.T) {
	var sent struct {
		method string
		path   string
		auth   string
		raw    []byte
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		sent.method = r.Method
		sent.path = r.URL.Path
		sent.auth = r.Header.Get("Authorization")
		sent.raw = body
		_, _ = w.Write(mustMarshal(t, emptyAnswer()))
	}))
	defer srv.Close()

	index, err := NewHTTPIndex(srv.URL, "test-key", "fit-rows")
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	since := testClock().Add(-domain.PoolRecentWindow)
	if _, err := index.Aggregate(context.Background(), Query{Line: testLine, Size: "M", Since: since}); err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	body := decodeObject(t, sent.raw)
	if sent.method != http.MethodPost {
		t.Errorf("method = %s, want POST", sent.method)
	}
	if sent.path != "/fit-rows/_search" {
		t.Errorf("path = %s, want /fit-rows/_search", sent.path)
	}
	if sent.auth != "ApiKey test-key" {
		t.Errorf("authorization = %q, want the index API key", sent.auth)
	}
	if got := at(t, body, "size"); got != float64(0) {
		t.Errorf("size = %v, want 0 so no documents come back", got)
	}
	if got := at(t, body, "track_total_hits"); got != true {
		t.Errorf("track_total_hits = %v, want true", got)
	}

	filters := atArray(t, body, "query", "bool", "filter")
	requireTermValue(t, filters, "brand", testLine.Brand)
	requireTermValue(t, filters, "category", testLine.Category)
	requireTermValue(t, filters, "fit_handle", testLine.FitHandle)
	requireTermValue(t, filters, "size", "M")

	split := at(t, body, "aggs", "split")
	splitFilters := at(t, split, "filters", "filters")
	if _, ok := objectAt(t, splitFilters, "all")["match_all"]; !ok {
		t.Error("the all bucket is not match_all")
	}
	requireTermValue(t, []any{at(t, splitFilters, "seeded")}, "seeded", true)
	requireTermValue(t, []any{at(t, splitFilters, "live")}, "seeded", false)
	requireTermValue(t, []any{at(t, split, "aggs", "settled", "filter")}, "settled", true)

	settled := at(t, split, "aggs", "settled", "aggs").(map[string]any)
	for _, name := range []string{"fit_shoppers", "settled_kind", "reason_category", "fit_returns", "feedback"} {
		if _, ok := settled[name]; !ok {
			t.Errorf("settled bucket has no %q aggregation", name)
		}
	}
	if _, ok := settled["recent"]; ok {
		t.Error("the recent window is nested under the split; it must count seeded and live together")
	}

	recent := at(t, body, "aggs", "recent")
	recentFilters := atArray(t, recent, "filter", "bool", "filter")
	requireTermValue(t, recentFilters, "settled", true)
	var cutoff any
	for _, raw := range recentFilters {
		rangeFilter, ok := raw.(map[string]any)["range"].(map[string]any)
		if !ok {
			continue
		}
		cutoff = rangeFilter["settled_at"].(map[string]any)["gte"]
	}
	wantCutoff := since.UTC().Format(time.RFC3339)
	if cutoff != wantCutoff {
		t.Errorf("recent window starts %v, want %q", cutoff, wantCutoff)
	}
	for _, name := range []string{"fit_shoppers", "settled_kind", "reason_category", "fit_returns", "feedback"} {
		if _, ok := at(t, recent, "aggs").(map[string]any)[name]; !ok {
			t.Errorf("recent window has no %q aggregation", name)
		}
	}

	fitShoppers := at(t, settled, "fit_shoppers")
	if got := at(t, fitShoppers, "aggs", "shoppers", "cardinality", "field"); got != "shopper_hash" {
		t.Errorf("fit shoppers count %v, want distinct shopper_hash", got)
	}
	should := atArray(t, fitShoppers, "filter", "bool", "should")
	if len(should) != 2 {
		t.Fatalf("fit-classified outcomes have %d clauses, want 2", len(should))
	}
	if got := at(t, fitShoppers, "filter", "bool", "minimum_should_match"); got != float64(1) {
		t.Errorf("minimum_should_match = %v, want 1", got)
	}
	requireTermValue(t, []any{should[1]}, "settled_kind", "feedback")
	returnClause := atArray(t, should[0], "bool", "filter")
	requireTermValue(t, returnClause, "settled_kind", "return_reason")
	termsClause := map[string]any{}
	for _, raw := range returnClause {
		if terms, ok := raw.(map[string]any)["terms"].(map[string]any); ok {
			termsClause = terms
		}
	}
	if got := termsClause["reason_category"]; !reflect.DeepEqual(got, []any{
		string(domain.CategoryFitTooSmall), string(domain.CategoryFitTooLarge),
	}) {
		t.Errorf("fit-classified returns match %v, want the two fit categories", got)
	}

	fitReturns := at(t, settled, "fit_returns")
	requireTermValue(t, []any{at(t, fitReturns, "filter")}, "settled_kind", "return_reason")
	if got := at(t, fitReturns, "aggs", "direction", "terms", "field"); got != "fit_direction" {
		t.Errorf("fit returns group by %v, want fit_direction", got)
	}
	if _, ok := at(t, fitReturns, "aggs", "direction").(map[string]any)["aggs"]; ok {
		t.Error("fit returns carry a per-direction shopper count; the union bucket below is the source")
	}

	feedback := at(t, settled, "feedback")
	requireTermValue(t, []any{at(t, feedback, "filter")}, "settled_kind", "feedback")
	if got := at(t, feedback, "aggs", "verdict", "terms", "field"); got != "feedback_verdict" {
		t.Errorf("feedback groups by %v, want feedback_verdict", got)
	}

	// The Earned floor reads distinct shoppers pointing each way, and a
	// direction is fit returns together with fit feedback carrying that
	// verdict, so each union bucket filters both settled kinds and counts
	// each shopper once.
	unions := at(t, settled, "direction_shoppers")
	if got := at(t, unions, "aggs", "shoppers", "cardinality", "field"); got != "shopper_hash" {
		t.Errorf("union shoppers count %v, want distinct shopper_hash", got)
	}
	for _, want := range []struct {
		bucket    string
		direction string
		verdict   string
	}{
		{"too_small", "too_small", "too_small"},
		{"too_large", "too_large", "too_large"},
	} {
		bucketFilters, ok := at(t, unions, "filters", "filters").(map[string]any)[want.bucket]
		if !ok {
			t.Errorf("no union bucket %q", want.bucket)
			continue
		}
		clauses := atArray(t, bucketFilters, "bool", "should")
		if len(clauses) != 2 {
			t.Errorf("union bucket %q has %d clauses, want 2 (returns and feedback)", want.bucket, len(clauses))
			continue
		}
		if got := at(t, bucketFilters, "bool", "minimum_should_match"); got != float64(1) {
			t.Errorf("union bucket %q minimum_should_match = %v, want 1", want.bucket, got)
		}
		returnClause := atArray(t, clauses[0], "bool", "filter")
		requireTermValue(t, returnClause, "settled_kind", "return_reason")
		requireTermValue(t, returnClause, "fit_direction", want.direction)
		feedbackClause := atArray(t, clauses[1], "bool", "filter")
		requireTermValue(t, feedbackClause, "settled_kind", "feedback")
		requireTermValue(t, feedbackClause, "feedback_verdict", want.verdict)
	}
}

// t012Counts is one recorded bucket of the T0.12 aggregation.
type t012Counts struct {
	Docs                        int            `json:"docs"`
	Settled                     int            `json:"settled"`
	SettledKind                 map[string]int `json:"settled_kind"`
	ReasonCategory              map[string]int `json:"reason_category"`
	FitReturnsByDirection       map[string]int `json:"fit_returns_by_direction"`
	FeedbackByVerdict           map[string]int `json:"feedback_by_verdict"`
	DistinctShoppersByDirection map[string]int `json:"distinct_shoppers_by_direction"`
	DistinctShoppersSettled     int            `json:"distinct_shoppers_settled"`
}

// t012Recording is the recorded population: the query and its answer, the
// hand-written truth, and the counts the harness read back.
type t012Recording struct {
	Query struct {
		Response json.RawMessage `json:"response"`
	} `json:"query"`
	GotAll          t012Counts `json:"got_all"`
	GotSeeded       t012Counts `json:"got_seeded"`
	GotLive         t012Counts `json:"got_live"`
	PopulationCount int        `json:"population_count"`
	LineSizeCount   int        `json:"line_size_count"`
}

// fitOutcomesIn counts the fit-classified settled outcomes in one recorded
// bucket: fit returns and every feedback row.
func fitOutcomesIn(got t012Counts) int {
	return got.FitReturnsByDirection["too_small"] +
		got.FitReturnsByDirection["too_large"] +
		got.FeedbackByVerdict["fits_well"] +
		got.FeedbackByVerdict["too_small"] +
		got.FeedbackByVerdict["too_large"]
}

// fitShoppersIn derives the fit-classified shopper count the recording does
// not carry. The recorded population gives a shopper one kind of settled
// outcome, so every settled shopper is either fit-classified, a non-fit
// return, or a kept purchase, and the subtraction is exact.
func fitShoppersIn(t *testing.T, got t012Counts) int {
	t.Helper()
	nonFit := got.ReasonCategory["changed_mind"] + got.ReasonCategory["damaged"] + got.ReasonCategory["not_as_described"]
	fit := got.DistinctShoppersSettled - nonFit - got.SettledKind["kept"]
	if fit < 0 {
		t.Fatalf("recorded shopper counts do not add up: %d settled, %d non-fit, %d kept",
			got.DistinctShoppersSettled, nonFit, got.SettledKind["kept"])
	}
	return fit
}

// recordedAnswer loads the recording, injects the fit-shopper count, the
// per-direction union shopper counts and the recent window, and returns the
// answer the fake index serves.
//
// The recording predates the three buckets the reader needs. The
// fit-shopper count is derived from the recorded counts above. The union
// counts add each direction's feedback-verdict rows to its return-distinct
// shoppers, exact because the harness gives feedback rows their own
// shoppers. The recent window is the recorded settled bucket itself,
// because every settled row in the population falls inside the 90 days
// before the pinned clock; that is what makes the population's own settled
// counts its recent counts.
func recordedAnswer(t *testing.T) (t012Recording, map[string]any) {
	t.Helper()
	var rec t012Recording
	if err := json.Unmarshal(recordedFixture(t, "aggregations.json"), &rec); err != nil {
		t.Fatalf("decode recorded aggregation: %v", err)
	}
	answer := decodeObject(t, rec.Query.Response)
	all := objectAt(t, answer, "aggregations", "split", "buckets", "all")
	allSettled := objectAt(t, all, "settled")
	if got, want := fitShoppersIn(t, rec.GotAll), 63; got != want {
		t.Fatalf("derived fit shoppers = %d, want %d", got, want)
	}
	allSettled["fit_shoppers"] = map[string]any{
		"doc_count": fitOutcomesIn(rec.GotAll),
		"shoppers":  map[string]any{"value": fitShoppersIn(t, rec.GotAll)},
	}
	allSettled["direction_shoppers"] = unionAnswer(t, rec.GotAll, 36, 15)
	for _, split := range []struct {
		name      string
		counts    t012Counts
		want      int
		smallWant int
		largeWant int
	}{
		{"seeded", rec.GotSeeded, 55, 31, 14},
		{"live", rec.GotLive, 8, 5, 1},
	} {
		if got := fitShoppersIn(t, split.counts); got != split.want {
			t.Fatalf("derived %s fit shoppers = %d, want %d", split.name, got, split.want)
		}
		settled := objectAt(t, objectAt(t, answer, "aggregations", "split", "buckets", split.name), "settled")
		settled["fit_shoppers"] = map[string]any{
			"doc_count": fitOutcomesIn(split.counts),
			"shoppers":  map[string]any{"value": fitShoppersIn(t, split.counts)},
		}
		settled["direction_shoppers"] = unionAnswer(t, split.counts, split.smallWant, split.largeWant)
	}
	aggs := objectAt(t, answer, "aggregations")
	aggs["recent"] = mustDecode(t, mustMarshal(t, allSettled))
	return rec, answer
}

// unionShoppersIn derives the per-direction union shopper count the Earned
// floor reads: fit returns that way together with fit feedback carrying
// that verdict. The harness gives every feedback row its own shopper id in
// a namespace disjoint from the return shoppers (feedback ids are
// l1m-fb-..., returns are l1m-small-... and l1m-large-...), so the union is
// the recorded return-distinct count plus the feedback verdict count.
func unionShoppersIn(got t012Counts, direction, verdict string) int {
	return got.DistinctShoppersByDirection[direction] + got.FeedbackByVerdict[verdict]
}

// unionAnswer builds the direction_shoppers bucket the reader needs: one
// bucket per direction, each with its union doc count and distinct-shopper
// cardinality. The wants pin the new numbers the fixture then carries: the
// old 31/12 were fit-return shoppers only, and the union adds the
// feedback-only shoppers (5 small and 3 large all-time).
func unionAnswer(t *testing.T, got t012Counts, smallWant, largeWant int) map[string]any {
	t.Helper()
	small := unionShoppersIn(got, "too_small", "too_small")
	large := unionShoppersIn(got, "too_large", "too_large")
	if small != smallWant || large != largeWant {
		t.Fatalf("derived union shoppers = %d/%d, want %d/%d", small, large, smallWant, largeWant)
	}
	bucket := func(docs, shoppers int) map[string]any {
		return map[string]any{
			"doc_count": docs,
			"shoppers":  map[string]any{"value": shoppers},
		}
	}
	return map[string]any{"buckets": map[string]any{
		"too_small": bucket(
			got.FitReturnsByDirection["too_small"]+got.FeedbackByVerdict["too_small"], small),
		"too_large": bucket(
			got.FitReturnsByDirection["too_large"]+got.FeedbackByVerdict["too_large"], large),
	}}
}
func objectAt(t *testing.T, root any, path ...string) map[string]any {
	t.Helper()
	object, ok := at(t, root, path...).(map[string]any)
	if !ok {
		t.Fatalf("path %v is not an object", path)
	}
	return object
}

// mustDecode decodes one JSON object the tests built.
func mustDecode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	return decodeObject(t, raw)
}

// wantCounts is one bucket's expected settled counts.
type wantCounts struct {
	settled  int
	kept     int
	returns  DirectionCounts
	feedback VerdictCounts
	reasons  map[domain.Category]int
	shoppers ShopperCounts
}

// checkCounts compares one bucket of a fit to the T0.12 truth.
func checkCounts(t *testing.T, name string, got Counts, want wantCounts) {
	t.Helper()
	if got.Settled != want.settled {
		t.Errorf("%s settled = %d, want %d", name, got.Settled, want.settled)
	}
	if got.Kept != want.kept {
		t.Errorf("%s kept = %d, want %d", name, got.Kept, want.kept)
	}
	if got.Returns != want.returns {
		t.Errorf("%s returns = %+v, want %+v", name, got.Returns, want.returns)
	}
	if got.Feedback != want.feedback {
		t.Errorf("%s feedback = %+v, want %+v", name, got.Feedback, want.feedback)
	}
	if !reflect.DeepEqual(got.Reasons, want.reasons) {
		t.Errorf("%s reasons = %v, want %v", name, got.Reasons, want.reasons)
	}
	if got.Shoppers != want.shoppers {
		t.Errorf("%s shoppers = %+v, want %+v", name, got.Shoppers, want.shoppers)
	}
}

// TestReadsTheRecordedPopulation reproduces T0.12's 600-purchase population
// exactly on the probe line and size: the settled denominator, the fit
// returns by direction, the feedback by verdict, the distinct shoppers and
// the seeded split. The 120 purchases in the line and size are the query's
// total; the pool's denominator is the 100 settled purchases among them.
func TestReadsTheRecordedPopulation(t *testing.T) {
	rec, answer := recordedAnswer(t)
	if rec.PopulationCount != 600 {
		t.Fatalf("recorded population = %d purchases, want 600", rec.PopulationCount)
	}
	if rec.LineSizeCount != 120 || rec.GotAll.Docs != 120 {
		t.Fatalf("recorded line and size = %d purchases (%d docs), want 120",
			rec.LineSizeCount, rec.GotAll.Docs)
	}
	if rec.GotAll.Settled != 100 {
		t.Fatalf("recorded settled = %d, want 100", rec.GotAll.Settled)
	}

	answerBody := mustMarshal(t, answer)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(answerBody)
	}))
	defer srv.Close()

	index, err := NewHTTPIndex(srv.URL, "test-key", "fit-pop")
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	// The pinned clock is inside 90 days of every recorded settled row, which
	// is why the recorded settled bucket is also the recent one.
	reader, err := NewReader(index, WithClock(func() time.Time {
		return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	}))
	if err != nil {
		t.Fatalf("build reader: %v", err)
	}
	fit, err := reader.Fit(context.Background(), testLine, "M")
	if err != nil {
		t.Fatalf("fit: %v", err)
	}

	wantAll := wantCounts{
		settled:  100,
		kept:     10,
		returns:  DirectionCounts{TooSmall: 40, TooLarge: 15},
		feedback: VerdictCounts{FitsWell: 12, TooSmall: 5, TooLarge: 3},
		reasons: map[domain.Category]int{
			"fit_too_small":    40,
			"fit_too_large":    15,
			"changed_mind":     8,
			"damaged":          5,
			"not_as_described": 2,
		},
		shoppers: ShopperCounts{Fit: 63, TooSmall: 36, TooLarge: 15},
	}
	wantSeeded := wantCounts{
		settled:  85,
		kept:     8,
		returns:  DirectionCounts{TooSmall: 35, TooLarge: 13},
		feedback: VerdictCounts{FitsWell: 10, TooSmall: 4, TooLarge: 3},
		reasons: map[domain.Category]int{
			"fit_too_small":    35,
			"fit_too_large":    13,
			"changed_mind":     6,
			"damaged":          4,
			"not_as_described": 2,
		},
		shoppers: ShopperCounts{Fit: 55, TooSmall: 31, TooLarge: 14},
	}
	wantLive := wantCounts{
		settled:  15,
		kept:     2,
		returns:  DirectionCounts{TooSmall: 5, TooLarge: 2},
		feedback: VerdictCounts{FitsWell: 2, TooSmall: 1, TooLarge: 0},
		reasons: map[domain.Category]int{
			"fit_too_small": 5,
			"fit_too_large": 2,
			"changed_mind":  2,
			"damaged":       1,
		},
		shoppers: ShopperCounts{Fit: 8, TooSmall: 5, TooLarge: 1},
	}
	checkCounts(t, "all", fit.Counts, wantAll)
	checkCounts(t, "seeded", fit.Seeded, wantSeeded)
	checkCounts(t, "live", fit.Live, wantLive)
	checkCounts(t, "recent", fit.Recent, wantAll)

	if fit.Verdict.Status != domain.StatusEarned || fit.Verdict.Direction != domain.FitDirectionSmall {
		t.Errorf("verdict = %s/%q, want Earned/small: 45 small outcomes of 75 fit outcomes is the 60%% floor, 36 shoppers in the direction, and 45 against 18 is more than twice",
			fit.Verdict.Status, fit.Verdict.Direction)
	}
	if fit.Line != testLine || fit.Size != "M" {
		t.Errorf("fit names %+v size %q, want %+v size M", fit.Line, fit.Size, testLine)
	}
}

// emptyAnswer is a valid aggregation answer for a line with no settled rows:
// every bucket present and every count zero.
func emptyAnswer() map[string]any {
	union := func() map[string]any {
		bucket := func() map[string]any {
			return map[string]any{"doc_count": 0, "shoppers": map[string]any{"value": 0}}
		}
		return map[string]any{"buckets": map[string]any{
			"too_small": bucket(),
			"too_large": bucket(),
		}}
	}
	settled := func() map[string]any {
		return map[string]any{
			"doc_count":          0,
			"fit_shoppers":       map[string]any{"doc_count": 0, "shoppers": map[string]any{"value": 0}},
			"settled_kind":       map[string]any{"buckets": []any{}},
			"reason_category":    map[string]any{"buckets": []any{}},
			"fit_returns":        map[string]any{"doc_count": 0, "direction": map[string]any{"buckets": []any{}}},
			"feedback":           map[string]any{"doc_count": 0, "verdict": map[string]any{"buckets": []any{}}},
			"direction_shoppers": union(),
		}
	}
	return map[string]any{
		"timed_out": false,
		"_shards":   map[string]any{"failed": 0},
		"aggregations": map[string]any{
			"split": map[string]any{"buckets": map[string]any{
				"all":    map[string]any{"doc_count": 0, "settled": settled()},
				"seeded": map[string]any{"doc_count": 0, "settled": settled()},
				"live":   map[string]any{"doc_count": 0, "settled": settled()},
			}},
			"recent": settled(),
		},
	}
}

// TestRefusesAnAnswerItCannotTrust makes partial or unreadable answers an
// error rather than a zero count. A zero in place of an answer would let the
// pool pretend a line is quiet.
func TestRefusesAnAnswerItCannotTrust(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   func(t *testing.T) map[string]any
		raw    []byte
		want   error
	}{
		{
			name:   "an error status",
			status: http.StatusServiceUnavailable,
			raw:    []byte(`{"error":"unavailable"}`),
			want:   ErrIndexStatus,
		},
		{
			name:   "a body that is not JSON",
			status: http.StatusOK,
			raw:    []byte(`not the aggregation`),
			want:   ErrMalformedResponse,
		},
		{
			name:   "a timed-out query",
			status: http.StatusOK,
			body: func(t *testing.T) map[string]any {
				answer := emptyAnswer()
				answer["timed_out"] = true
				return answer
			},
			want: ErrIncomplete,
		},
		{
			name:   "a failed shard",
			status: http.StatusOK,
			body: func(t *testing.T) map[string]any {
				answer := emptyAnswer()
				answer["_shards"] = map[string]any{"failed": 1}
				return answer
			},
			want: ErrIncomplete,
		},
		{
			name:   "no recent window",
			status: http.StatusOK,
			body: func(t *testing.T) map[string]any {
				answer := emptyAnswer()
				delete(objectAt(t, answer, "aggregations"), "recent")
				return answer
			},
			want: ErrMalformedResponse,
		},
		{
			name:   "no fit-shopper count",
			status: http.StatusOK,
			body: func(t *testing.T) map[string]any {
				answer := emptyAnswer()
				delete(settledAt(t, answer, "all"), "fit_shoppers")
				return answer
			},
			want: ErrMalformedResponse,
		},
		{
			name:   "a union direction with no shopper count",
			status: http.StatusOK,
			body: func(t *testing.T) map[string]any {
				answer := emptyAnswer()
				union := objectAt(t, settledAt(t, answer, "all"), "direction_shoppers", "buckets")
				union["too_small"] = map[string]any{"doc_count": 1}
				return answer
			},
			want: ErrMalformedResponse,
		},
		{
			name:   "no live split",
			status: http.StatusOK,
			body: func(t *testing.T) map[string]any {
				answer := emptyAnswer()
				delete(objectAt(t, answer, "aggregations", "split", "buckets"), "live")
				return answer
			},
			want: ErrMalformedResponse,
		},
		{
			name:   "no settled bucket",
			status: http.StatusOK,
			body: func(t *testing.T) map[string]any {
				answer := emptyAnswer()
				delete(objectAt(t, answer, "aggregations", "split", "buckets", "all"), "settled")
				return answer
			},
			want: ErrMalformedResponse,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.raw
			if tc.body != nil {
				body = mustMarshal(t, tc.body(t))
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write(body)
			}))
			defer srv.Close()
			index, err := NewHTTPIndex(srv.URL, "test-key", "fit-rows")
			if err != nil {
				t.Fatalf("build index: %v", err)
			}
			result, err := index.Aggregate(context.Background(), Query{
				Line: testLine, Size: "M", Since: testClock().Add(-domain.PoolRecentWindow),
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("aggregate error = %v, want %v", err, tc.want)
			}
			if !reflect.DeepEqual(result, Result{}) {
				t.Errorf("refused answer returned counts %+v, want none", result)
			}
		})
	}
}

// settledBucket returns one split bucket's settled aggregation.
func settledAt(t *testing.T, answer map[string]any, name string) map[string]any {
	t.Helper()
	return objectAt(t, answer, "aggregations", "split", "buckets", name, "settled")
}

// TestOneWarmConnectionServesEveryRead proves the reader keeps one
// connection: warming it and then reading twice opens the connection once.
func TestOneWarmConnectionServesEveryRead(t *testing.T) {
	answer := mustMarshal(t, emptyAnswer())
	var mu sync.Mutex
	opened := 0
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write(answer)
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			mu.Lock()
			opened++
			mu.Unlock()
		}
	}
	srv.Start()
	defer srv.Close()

	index, err := NewHTTPIndex(srv.URL, "test-key", "fit-rows")
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	reader, err := NewReader(index, WithClock(testClock))
	if err != nil {
		t.Fatalf("build reader: %v", err)
	}
	ctx := context.Background()
	if err := reader.Warm(ctx); err != nil {
		t.Fatalf("warm: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := reader.Fit(ctx, testLine, "M"); err != nil {
			t.Fatalf("fit %d: %v", i, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if opened != 1 {
		t.Errorf("%d connections opened, want one warm connection", opened)
	}
}

// TestWarmAcceptsAnAbsentIndex keeps warming about the connection: an index
// that does not exist yet answers 404, which is still a warm connection.
func TestWarmAcceptsAnAbsentIndex(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	index, err := NewHTTPIndex(srv.URL, "test-key", "fit-rows")
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	if err := index.Warm(context.Background()); err != nil {
		t.Fatalf("warm an absent index: %v", err)
	}
	if method != http.MethodHead || path != "/fit-rows" {
		t.Errorf("warm sent %s %s, want HEAD /fit-rows", method, path)
	}
}

// TestTimeoutClearsTheColdHandshake pins the timeout above T0.12's measured
// cold TLS handshake.
func TestTimeoutClearsTheColdHandshake(t *testing.T) {
	if DefaultTimeout != 20*time.Second {
		t.Errorf("DefaultTimeout = %s, want 20s", DefaultTimeout)
	}
	if DefaultTimeout <= 7*time.Second {
		t.Errorf("DefaultTimeout = %s, want above the 6.9s cold handshake", DefaultTimeout)
	}
}

// TestNewHTTPIndexRejectsBadArguments pins the constructor's refusals.
func TestNewHTTPIndexRejectsBadArguments(t *testing.T) {
	cases := []struct {
		name string
		url  string
		key  string
		idx  string
		want error
	}{
		{"not an absolute URL", "search.example/fit", "key", "fit-rows", ErrMalformedEndpoint},
		{"not http or https", "ftp://search.example", "key", "fit-rows", ErrMalformedEndpoint},
		{"no host", "http://", "key", "fit-rows", ErrMalformedEndpoint},
		{"no API key", "https://search.example", "", "fit-rows", ErrMissingAPIKey},
		{"no index name", "https://search.example", "key", "", ErrMissingIndexName},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewHTTPIndex(tc.url, tc.key, tc.idx); !errors.Is(err, tc.want) {
				t.Fatalf("NewHTTPIndex = %v, want %v", err, tc.want)
			}
		})
	}
}
