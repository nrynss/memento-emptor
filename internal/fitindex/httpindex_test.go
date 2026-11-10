package fitindex

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
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

// decodeObject decodes raw JSON into a map, failing on anything else.
func decodeObject(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode JSON object: %v", err)
	}
	return value
}

// TestMappingMatchesTheRecordedIndex pins the mapping the client sends to
// the one T0.12 recorded. It is strict, every field is a keyword, boolean
// or date, and there is no vector or prose field.
func TestMappingMatchesTheRecordedIndex(t *testing.T) {
	var mapping struct {
		Create struct {
			Path    string          `json:"path"`
			Request json.RawMessage `json:"request"`
		} `json:"create"`
		Fields []string `json:"fields"`
	}
	if err := json.Unmarshal(recordedFixture(t, "mapping.json"), &mapping); err != nil {
		t.Fatalf("decode recorded mapping: %v", err)
	}
	sent := decodeObject(t, Mapping())
	recorded := decodeObject(t, mapping.Create.Request)
	if !reflect.DeepEqual(sent, recorded) {
		t.Fatalf("mapping differs from the recording:\n got: %v\nwant: %v", sent, recorded)
	}
	properties := sent["mappings"].(map[string]any)["properties"].(map[string]any)
	if dynamic := sent["mappings"].(map[string]any)["dynamic"]; dynamic != "strict" {
		t.Errorf("dynamic = %v, want strict so a prose field is refused", dynamic)
	}
	for name, property := range properties {
		kind := property.(map[string]any)["type"]
		switch kind {
		case "keyword", "boolean", "date", "integer":
		default:
			t.Errorf("field %q has type %v, want a structured type", name, kind)
		}
	}
	for _, field := range mapping.Fields {
		if _, ok := properties[field]; !ok {
			t.Errorf("recorded field %q is missing from the mapping", field)
		}
	}
}

// TestDocumentFitsTheMapping builds a row through the writer and checks
// its document against the recording's contested row and the mapping's
// field list. Every field is in the mapping, there is no event_seq (T7.1
// made the state rank the version) and no prose.
func TestDocumentFitsTheMapping(t *testing.T) {
	row := contestedRow(t)
	doc := documentFor(row)

	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("encode document: %v", err)
	}
	document := decodeObject(t, encoded)
	properties := decodeObject(t, Mapping())["mappings"].(map[string]any)["properties"].(map[string]any)
	for name := range document {
		if _, ok := properties[name]; !ok {
			t.Errorf("document field %q is not in the mapping; strict indexing would refuse it", name)
		}
	}
	if _, ok := document["event_seq"]; ok {
		t.Error("document carries event_seq; the version is the state rank with event time inside it")
	}

	var states struct {
		Steps []struct {
			Request map[string]any `json:"request"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(recordedFixture(t, "states.json"), &states); err != nil {
		t.Fatalf("decode recorded states: %v", err)
	}
	var recorded map[string]any
	for _, step := range states.Steps {
		if step.Request["purchase_state"] == "contested" {
			recorded = step.Request
			break
		}
	}
	if recorded == nil {
		t.Fatal("the recording holds no contested row")
	}
	// The recorded row is a different purchase in another line, so the
	// line and the event times differ by design. Every other field must
	// say what the recording's contested row says.
	for _, field := range []string{
		"size", "purchase_state", "settled", "settled_kind",
		"reason_category", "fit_direction", "reason_contested",
	} {
		if document[field] != recorded[field] {
			t.Errorf("%s = %v, want the recorded row's %v", field, document[field], recorded[field])
		}
	}
	for _, field := range []string{"captured_at", "refunded_at", "settled_at"} {
		moment, ok := document[field].(string)
		if !ok || moment == "" {
			t.Fatalf("%s = %v, want an event time", field, document[field])
		}
		if _, err := time.Parse(time.RFC3339, moment); err != nil {
			t.Errorf("%s = %q, want an ISO 8601 event time: %v", field, moment, err)
		}
	}
}

// TestPutReplacesTheWholeDocumentAtTheVersion points the client at a local
// server and asserts the write: a whole-document PUT to the transaction
// hash, at the row's version, with external_gte and a refresh so a read
// after the purchase sees the row.
func TestPutReplacesTheWholeDocumentAtTheVersion(t *testing.T) {
	row := contestedRow(t)
	server, recorded := recordingServer(t, http.StatusOK, `{"result":"updated"}`)
	index, err := NewHTTPIndex(server.URL, "test-api-key", "test-fit-rows")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	if err := index.Put(context.Background(), row); err != nil {
		t.Fatalf("put: %v", err)
	}

	if recorded.method != http.MethodPut {
		t.Errorf("method = %s, want PUT", recorded.method)
	}
	if want := "/test-fit-rows/_doc/" + row.TxnHash; recorded.path != want {
		t.Errorf("path = %s, want %s", recorded.path, want)
	}
	query := recorded.query
	if got := query.Get("version_type"); got != "external_gte" {
		t.Errorf("version_type = %q, want external_gte", got)
	}
	if got, want := query.Get("version"), strconv.FormatInt(row.Version(), 10); got != want {
		t.Errorf("version = %q, want the row's %s", got, want)
	}
	if got := query.Get("refresh"); got != "true" {
		t.Errorf("refresh = %q, want true", got)
	}
	if got := recorded.header.Get("Authorization"); got != "ApiKey test-api-key" {
		t.Errorf("authorization = %q, want the ApiKey header", got)
	}
	if recorded.header.Get("Content-Type") != "application/json" {
		t.Errorf("content type = %q, want application/json", recorded.header.Get("Content-Type"))
	}
	body := decodeObject(t, recorded.body)
	if body["txn_hash"] != row.TxnHash {
		t.Errorf("body txn_hash = %v, want %s", body["txn_hash"], row.TxnHash)
	}
	if body["purchase_state"] != "contested" {
		t.Errorf("body purchase_state = %v, want contested", body["purchase_state"])
	}
	if body["fit_direction"] != "too_large" {
		t.Errorf("body fit_direction = %v, want the index's word too_large", body["fit_direction"])
	}
	if body["reason_category"] != "fit_too_large" {
		t.Errorf("body reason_category = %v, want the domain category", body["reason_category"])
	}
	if _, ok := body["note"]; ok {
		t.Error("body carries prose")
	}
}

// TestPutRefusesAnOlderVersion maps the index's conflict answer to
// ErrOlderEvent, the writer's signal that a newer state stands.
func TestPutRefusesAnOlderVersion(t *testing.T) {
	server, _ := recordingServer(t, http.StatusConflict,
		`{"error":{"type":"version_conflict_engine_exception"},"status":409}`)
	index, err := NewHTTPIndex(server.URL, "test-api-key", "test-fit-rows")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	err = index.Put(context.Background(), contestedRow(t))
	if !errors.Is(err, ErrOlderEvent) {
		t.Fatalf("conflict error = %v, want ErrOlderEvent", err)
	}
}

// TestCreateSendsTheMappingAndAcceptsAnExistingIndex asserts that create
// sends the recorded mapping to the index's own path, and that an index
// that already exists is not an error.
func TestCreateSendsTheMappingAndAcceptsAnExistingIndex(t *testing.T) {
	server, recorded := recordingServer(t, http.StatusOK, `{"acknowledged":true}`)
	index, err := NewHTTPIndex(server.URL, "test-api-key", "test-fit-rows")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	if err := index.Create(context.Background()); err != nil {
		t.Fatalf("create: %v", err)
	}
	if recorded.method != http.MethodPut || recorded.path != "/test-fit-rows" {
		t.Errorf("create sent %s %s, want PUT /test-fit-rows", recorded.method, recorded.path)
	}
	if got, want := decodeObject(t, recorded.body), decodeObject(t, Mapping()); !reflect.DeepEqual(got, want) {
		t.Errorf("create body = %v, want the mapping", got)
	}

	exists, _ := recordingServer(t, http.StatusBadRequest,
		`{"error":{"type":"resource_already_exists_exception"},"status":400}`)
	again, err := NewHTTPIndex(exists.URL, "test-api-key", "test-fit-rows")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	if err := again.Create(context.Background()); err != nil {
		t.Errorf("create over an existing index = %v, want no error", err)
	}
}

// TestDeleteShopperFiltersEveryLineFieldAndTheShopperHash asserts opt-out's
// delete: the three line fields and the per-line shopper hash, so only that
// shopper's rows in that line go, and the count comes back.
func TestDeleteShopperFiltersEveryLineFieldAndTheShopperHash(t *testing.T) {
	server, recorded := recordingServer(t, http.StatusOK, `{"deleted":3,"failures":[]}`)
	index, err := NewHTTPIndex(server.URL, "test-api-key", "test-fit-rows")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	removed, err := index.DeleteShopper(context.Background(), testLine, "abc123")
	if err != nil {
		t.Fatalf("delete shopper: %v", err)
	}
	if removed != 3 {
		t.Errorf("removed = %d, want the deleted count 3", removed)
	}
	if recorded.method != http.MethodPost || recorded.path != "/test-fit-rows/_delete_by_query" {
		t.Errorf("delete sent %s %s, want POST /test-fit-rows/_delete_by_query", recorded.method, recorded.path)
	}
	if recorded.query.Get("conflicts") != "proceed" {
		t.Errorf("conflicts = %q, want proceed so an opt-out removes every row", recorded.query.Get("conflicts"))
	}
	body := decodeObject(t, recorded.body)
	filters := body["query"].(map[string]any)["bool"].(map[string]any)["filter"].([]any)
	want := map[string]string{
		"brand":        testLine.Brand,
		"category":     testLine.Category,
		"fit_handle":   testLine.FitHandle,
		"shopper_hash": "abc123",
	}
	if len(filters) != len(want) {
		t.Fatalf("delete has %d filters, want %d", len(filters), len(want))
	}
	for _, filter := range filters {
		for field, value := range filter.(map[string]any)["term"].(map[string]any) {
			if want[field] != value {
				t.Errorf("filter %s = %v, want %v", field, value, want[field])
			}
			delete(want, field)
		}
	}
	if len(want) != 0 {
		t.Errorf("delete misses filters for %v", want)
	}
}

// TestDeleteShopperRefusesAnIncompleteFilter refuses a delete whose filter
// would match more than the shopper's rows.
func TestDeleteShopperRefusesAnIncompleteFilter(t *testing.T) {
	server, _ := recordingServer(t, http.StatusOK, `{"deleted":100}`)
	index, err := NewHTTPIndex(server.URL, "test-api-key", "test-fit-rows")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	if _, err := index.DeleteShopper(context.Background(), testLine, ""); !errors.Is(err, ErrMissingLine) {
		t.Errorf("empty hash error = %v, want ErrMissingLine", err)
	}
	if _, err := index.DeleteShopper(context.Background(), domain.ProductLine{Brand: "brand-x"}, "abc123"); !errors.Is(err, ErrMissingLine) {
		t.Errorf("partial line error = %v, want ErrMissingLine", err)
	}
}

// TestHTTPIndexRefusesBadConfiguration fails closed: no endpoint, no key
// and no index name are each refused rather than defaulted.
func TestHTTPIndexRefusesBadConfiguration(t *testing.T) {
	cases := []struct {
		name string
		url  string
		key  string
		idx  string
		want error
	}{
		{"relative endpoint", "/fit", "key", "fit", ErrMalformedEndpoint},
		{"wrong scheme", "ftp://example.invalid", "key", "fit", ErrMalformedEndpoint},
		{"no key", "https://example.invalid", "", "fit", ErrMissingAPIKey},
		{"no index", "https://example.invalid", "key", "", ErrMissingIndexName},
	}
	for _, tc := range cases {
		if _, err := NewHTTPIndex(tc.url, tc.key, tc.idx); !errors.Is(err, tc.want) {
			t.Errorf("%s error = %v, want %v", tc.name, err, tc.want)
		}
	}
}

// contestedRow builds one purchase's row through the writer: captured,
// refunded, classified too small, then contested too large by the shopper.
func contestedRow(t *testing.T) domain.FitRow {
	t.Helper()
	index := newFakeIndex()
	writer := writerFor(t, index, "payer-0001")
	ctx := context.Background()
	purchase := testPurchase("CAP-T82-1000")
	for _, apply := range []func() error{
		func() error { return writer.Captured(ctx, purchase, captureTime) },
		func() error { return writer.Refunded(ctx, purchase, refundTime) },
		func() error { return writer.Classified(ctx, purchase, domain.CategoryFitTooSmall, classifyTime) },
		func() error { return writer.Contested(ctx, purchase, domain.CategoryFitTooLarge, contestTime) },
	} {
		if err := apply(); err != nil {
			t.Fatalf("build contested row: %v", err)
		}
	}
	return index.only(t)
}

// recorded is one request the local server received.
type recorded struct {
	method string
	path   string
	query  url.Values
	header http.Header
	body   []byte
}

// recordingServer starts a local server that answers one request with the
// given status and body, and returns the request it saw.
func recordingServer(t *testing.T, status int, response string) (*httptest.Server, *recorded) {
	t.Helper()
	seen := &recorded{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		seen.method = r.Method
		seen.path = r.URL.Path
		seen.query = r.URL.Query()
		seen.header = r.Header.Clone()
		seen.body = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	return server, seen
}
