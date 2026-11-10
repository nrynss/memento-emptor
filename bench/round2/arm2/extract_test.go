package arm2

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// demoCustomID is the custom id the demo shopper's orders, captures,
// refunds and disputes carry through every toolkit response.
const demoCustomID = "shopper-demo-001"

// loadDemo loads the committed toolkit recordings.
func loadDemo(t *testing.T) Recordings {
	t.Helper()
	rec, err := Load(filepath.Join("recordings"))
	if err != nil {
		t.Fatalf("load recordings: %v", err)
	}
	return rec
}

// extractDemo extracts the demo shopper's arm-2 input and decodes it.
func extractDemo(t *testing.T) []map[string]any {
	t.Helper()
	encoded, err := Extract(loadDemo(t), Selector{CustomID: demoCustomID})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	var records []map[string]any
	if err := json.Unmarshal(encoded, &records); err != nil {
		t.Fatalf("decode extraction: %v", err)
	}
	return records
}

// recordKind names what one output record is, from the keys the toolkit
// returned.
type recordKind int

const (
	kindUnknown recordKind = iota
	kindTransaction
	kindOrder
	kindRefund
	kindDispute
)

// kindOf classifies a record without looking at values, only at the keys
// the matching toolkit call returns.
func kindOf(record map[string]any) recordKind {
	switch {
	case record["transaction_info"] != nil:
		return kindTransaction
	case record["dispute_id"] != nil:
		return kindDispute
	case record["purchase_units"] != nil:
		return kindOrder
	case record["id"] != nil:
		return kindRefund
	default:
		return kindUnknown
	}
}

// TestExtractCarriesWhatTheToolkitReturned walks the extraction record by
// record, finds the toolkit response each record came from, and fails on
// any field path or value that response did not carry. This is the proof
// that arm 2's input holds nothing the layer produced: a reason category,
// signal strength, belief or confidence would have no source path to
// match.
func TestExtractCarriesWhatTheToolkitReturned(t *testing.T) {
	rec := loadDemo(t)
	sources := sourceRecords(t, rec)
	records := extractDemo(t)
	if len(records) == 0 {
		t.Fatal("extraction is empty")
	}
	for index, record := range records {
		source, err := sourceFor(record, sources)
		if err != nil {
			t.Fatalf("record %d: %v", index, err)
		}
		sourceFields := scalarFields(source)
		for path, values := range scalarFields(record) {
			sourceValues, ok := sourceFields[path]
			if !ok {
				t.Errorf("record %d: field %q was not returned by the Agent Toolkit", index, path)
				continue
			}
			for _, value := range values {
				if !containsValue(sourceValues, value) {
					t.Errorf("record %d: field %q value %v was not returned by the Agent Toolkit",
						index, path, value)
				}
			}
		}
	}
}

// TestExtractionInventsNoReason checks the fields the layer owns are
// absent by name, beside the per-record path proof above.
func TestExtractionInventsNoReason(t *testing.T) {
	banned := []string{"signal", "confidence", "belief", "classif",
		"counts_toward", "seed_mark", "write", "precedence", "pending"}
	for index, record := range extractDemo(t) {
		for path := range scalarFields(record) {
			leaf := path
			if dot := strings.LastIndex(leaf, "."); dot >= 0 {
				leaf = leaf[dot+1:]
			}
			for _, word := range banned {
				if strings.Contains(strings.ToLower(leaf), word) {
					t.Errorf("record %d: field %q is layer vocabulary, not a toolkit field", index, path)
				}
			}
		}
		if kindOf(record) == kindRefund {
			if _, ok := record["reason"]; ok {
				t.Errorf("record %d: refund carries a reason field the toolkit never returns", index)
			}
		}
	}
}

// TestExtractDemoShopper pins what arm 2 actually gets for the demo
// shopper: the transaction listing, order line items, refund status and
// the dispute's status and reason code.
func TestExtractDemoShopper(t *testing.T) {
	counts := map[recordKind]int{}
	for _, record := range extractDemo(t) {
		counts[kindOf(record)]++
	}
	if counts[kindTransaction] != 6 {
		t.Errorf("transactions = %d, want 6 (five successful rows and one pending dispute hold)",
			counts[kindTransaction])
	}
	if counts[kindOrder] != 3 {
		t.Errorf("orders = %d, want 3", counts[kindOrder])
	}
	if counts[kindRefund] != 2 {
		t.Errorf("refunds = %d, want 2", counts[kindRefund])
	}
	if counts[kindDispute] != 1 {
		t.Errorf("disputes = %d, want 1", counts[kindDispute])
	}

	var pendingHold, refunded, disputed bool
	for _, record := range extractDemo(t) {
		switch kindOf(record) {
		case kindTransaction:
			status := nestedString(record, "transaction_info", "transaction_status")
			if status == "P" {
				pendingHold = true
			}
			// list_transactions passes no fields parameter, so the
			// reporting response carries transaction_info only. Line
			// items must not appear here.
			if _, ok := record["cart_info"]; ok {
				t.Error("transaction carries cart_info the toolkit did not return")
			}
			if _, ok := record["payer_info"]; ok {
				t.Error("transaction carries payer_info the toolkit did not return")
			}
		case kindOrder:
			// purchase_units is an array, so read the first unit before
			// looking for its line items.
			units := nestedSlice(record, "purchase_units")
			if len(units) == 0 {
				t.Fatal("order carries no purchase unit")
			}
			unit, ok := units[0].(map[string]any)
			if !ok {
				t.Fatal("order purchase unit is not an object")
			}
			items := nestedSlice(unit, "items")
			if len(items) == 0 {
				t.Fatal("order carries no line items")
			}
			item, ok := items[0].(map[string]any)
			if !ok {
				t.Fatal("order line item is not an object")
			}
			if sku := nestedString(item, "sku"); sku != "BRX-OXF-M" {
				t.Errorf("line item sku = %q, want BRX-OXF-M", sku)
			}
		case kindRefund:
			if status := nestedString(record, "status"); status != "COMPLETED" {
				t.Errorf("refund status = %q, want COMPLETED", status)
			}
			refunded = true
		case kindDispute:
			if reason := nestedString(record, "reason"); reason != "MERCHANDISE_OR_SERVICE_NOT_RECEIVED" {
				t.Errorf("dispute reason = %q, want MERCHANDISE_OR_SERVICE_NOT_RECEIVED", reason)
			}
			if status := nestedString(record, "status"); status != "WAITING_FOR_SELLER_RESPONSE" {
				t.Errorf("dispute status = %q, want WAITING_FOR_SELLER_RESPONSE", status)
			}
			disputed = true
		}
	}
	if !pendingHold {
		t.Error("pending dispute hold is missing from the transaction listing")
	}
	if !refunded {
		t.Error("refund status is missing")
	}
	if !disputed {
		t.Error("dispute status and reason are missing")
	}
}

// TestExtractSelectsOneShopper proves the selector drops another
// shopper's rows rather than widening the history.
func TestExtractSelectsOneShopper(t *testing.T) {
	rec := Recordings{
		Transactions: [][]byte{[]byte(`{
			"transaction_details": [
				{"transaction_info": {"transaction_id": "TX-A", "custom_field": "shopper-a", "transaction_initiation_date": "2026-01-01T00:00:00Z"}},
				{"transaction_info": {"transaction_id": "TX-B", "custom_field": "shopper-b", "transaction_initiation_date": "2026-01-02T00:00:00Z"}}
			]
		}`)},
		Orders: [][]byte{
			[]byte(`{"id": "ORDER-A", "create_time": "2026-01-01T00:00:00Z", "purchase_units": [{"custom_id": "shopper-a", "items": [{"sku": "SKU-A"}]}]}`),
			[]byte(`{"id": "ORDER-B", "create_time": "2026-01-02T00:00:00Z", "purchase_units": [{"custom_id": "shopper-b", "items": [{"sku": "SKU-B"}]}]}`),
		},
		Refunds: [][]byte{
			[]byte(`{"id": "TX-A", "status": "COMPLETED", "custom_id": "shopper-a"}`),
			[]byte(`{"id": "TX-B", "status": "COMPLETED", "custom_id": "shopper-b"}`),
		},
		Disputes: [][]byte{
			[]byte(`{"dispute_id": "DSP-A", "reason": "MERCHANDISE_OR_SERVICE_NOT_RECEIVED", "status": "OPEN", "create_time": "2026-01-01T00:00:00Z", "disputed_transactions": [{"custom": "shopper-a", "seller_transaction_id": "TX-A"}]}`),
			[]byte(`{"dispute_id": "DSP-B", "reason": "MERCHANDISE_OR_SERVICE_NOT_RECEIVED", "status": "OPEN", "create_time": "2026-01-02T00:00:00Z", "disputed_transactions": [{"custom": "shopper-b", "seller_transaction_id": "TX-B"}]}`),
		},
	}
	encoded, err := Extract(rec, Selector{CustomID: "shopper-a"})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	var records []map[string]any
	if err := json.Unmarshal(encoded, &records); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("records = %d, want 4", len(records))
	}
	text := string(encoded)
	if strings.Contains(text, "TX-B") || strings.Contains(text, "ORDER-B") ||
		strings.Contains(text, "DSP-B") || strings.Contains(text, "shopper-b") {
		t.Error("another shopper's rows survived the selector")
	}
}

// TestExtractRequiresSelector proves a selector that names no shopper
// fails rather than returning the whole merchant listing.
func TestExtractRequiresSelector(t *testing.T) {
	if _, err := Extract(loadDemo(t), Selector{}); !errors.Is(err, ErrNoSelector) {
		t.Fatalf("error = %v, want ErrNoSelector", err)
	}
}

// TestLoadWithoutRecordings proves an empty directory is an error, not an
// empty history.
func TestLoadWithoutRecordings(t *testing.T) {
	if _, err := Load(t.TempDir()); !errors.Is(err, ErrNoRecordings) {
		t.Fatalf("error = %v, want ErrNoRecordings", err)
	}
}

// TestExtractRejectsMalformedResponse proves a body that is not a toolkit
// response is reported, not guessed at.
func TestExtractRejectsMalformedResponse(t *testing.T) {
	rec := Recordings{Transactions: [][]byte{[]byte(`not json`)}}
	if _, err := Extract(rec, Selector{CustomID: "shopper-a"}); !errors.Is(err, ErrMalformed) {
		t.Fatalf("error = %v, want ErrMalformed", err)
	}
}

// sourceIndex holds the toolkit responses keyed by the id each response
// carries.
type sourceIndex struct {
	transactions map[string]map[string]any
	orders       map[string]map[string]any
	refunds      map[string]map[string]any
	disputes     map[string]map[string]any
}

// sourceRecords indexes every committed recording by its returned id.
func sourceRecords(t *testing.T, rec Recordings) sourceIndex {
	t.Helper()
	index := sourceIndex{
		transactions: map[string]map[string]any{},
		orders:       map[string]map[string]any{},
		refunds:      map[string]map[string]any{},
		disputes:     map[string]map[string]any{},
	}
	for _, raw := range rec.Transactions {
		root, err := decodeObject(raw, "list_transactions")
		if err != nil {
			t.Fatalf("decode transaction recording: %v", err)
		}
		for _, detail := range nestedSlice(root, "transaction_details") {
			fields, ok := detail.(map[string]any)
			if !ok {
				t.Fatalf("transaction detail is not an object")
			}
			id := nestedString(fields, "transaction_info", "transaction_id")
			if id != "" {
				index.transactions[id] = fields
			}
		}
	}
	for _, raw := range rec.Orders {
		order := decodeRecording(t, raw)
		index.orders[nestedString(order, "id")] = order
	}
	for _, raw := range rec.Refunds {
		refund := decodeRecording(t, raw)
		index.refunds[nestedString(refund, "id")] = refund
	}
	for _, raw := range rec.Disputes {
		dispute := decodeRecording(t, raw)
		index.disputes[nestedString(dispute, "dispute_id")] = dispute
	}
	return index
}

// decodeRecording decodes one recording or fails the test.
func decodeRecording(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("decode recording: %v", err)
	}
	return object
}

// sourceFor finds the one toolkit response an output record came from.
func sourceFor(record map[string]any, sources sourceIndex) (map[string]any, error) {
	switch kindOf(record) {
	case kindTransaction:
		id := nestedString(record, "transaction_info", "transaction_id")
		if source, ok := sources.transactions[id]; ok {
			return source, nil
		}
		return nil, fmt.Errorf("no transaction response with id %q", id)
	case kindOrder:
		id := nestedString(record, "id")
		if source, ok := sources.orders[id]; ok {
			return source, nil
		}
		return nil, fmt.Errorf("no order response with id %q", id)
	case kindRefund:
		id := nestedString(record, "id")
		if source, ok := sources.refunds[id]; ok {
			return source, nil
		}
		return nil, fmt.Errorf("no refund response with id %q", id)
	case kindDispute:
		id := nestedString(record, "dispute_id")
		if source, ok := sources.disputes[id]; ok {
			return source, nil
		}
		return nil, fmt.Errorf("no dispute response with id %q", id)
	default:
		return nil, errors.New("record carries no key any toolkit response returns")
	}
}

// scalarFields maps every scalar path in a value to the scalar values at
// that path. Array elements share their parent's path, so only the field
// names matter.
func scalarFields(value any) map[string][]any {
	fields := map[string][]any{}
	var walk func(any, string)
	walk = func(current any, path string) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				next := key
				if path != "" {
					next = path + "." + key
				}
				walk(child, next)
			}
		case []any:
			for _, child := range typed {
				walk(child, path)
			}
		default:
			fields[path] = append(fields[path], typed)
		}
	}
	walk(value, "")
	return fields
}

// containsValue reports whether values holds value.
func containsValue(values []any, value any) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
