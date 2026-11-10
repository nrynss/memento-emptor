package arm2

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNoSelector is returned when a Selector names neither a custom id nor
// a payer email, so no transaction can be identified as the shopper's.
var ErrNoSelector = errors.New("arm2: selector names no shopper")

// ErrNoRecordings is returned when a recordings directory holds no
// toolkit response this package reads.
var ErrNoRecordings = errors.New("arm2: no toolkit recordings")

// ErrMalformed is returned when a recording is not a JSON object of the
// shape the toolkit returns.
var ErrMalformed = errors.New("arm2: malformed toolkit response")

// Selector names the shopper whose history to extract. CustomID matches
// transaction_info.custom_field (the seller's custom_id carried through
// the order) and the custom id on orders, refunds and disputes. PayerEmail
// matches payer_info.email_address, order payer email and dispute buyer
// email. Both are values the toolkit returns; the selector adds nothing to
// the output.
type Selector struct {
	// CustomID is the shopper's custom id, as returned in custom_field.
	CustomID string
	// PayerEmail is the shopper's PayPal email address.
	PayerEmail string
}

// Recordings holds the decoded bytes of the toolkit responses for one
// merchant listing. Each slice holds one tool call's response body, exactly
// as the tool returned it: list_transactions, get_order, get_refund and
// get_dispute respectively.
type Recordings struct {
	// Transactions holds list_transactions responses, one per call.
	Transactions [][]byte
	// Orders holds get_order responses, one per order.
	Orders [][]byte
	// Refunds holds get_refund responses, one per refund.
	Refunds [][]byte
	// Disputes holds get_dispute responses, one per dispute.
	Disputes [][]byte
}

// Load reads the toolkit responses a directory holds. It reads four file
// patterns and ignores everything else, so probes and tool listings can
// sit beside the responses:
//
//	list-transactions-*.json  list_transactions
//	get-order-*.json          get_order
//	get-refund-*.json         get_refund
//	get-dispute-*.json        get_dispute
//
// It returns ErrNoRecordings when no pattern matches a file.
func Load(dir string) (Recordings, error) {
	var rec Recordings
	targets := []struct {
		pattern string
		dest    *[][]byte
	}{
		{"list-transactions-*.json", &rec.Transactions},
		{"get-order-*.json", &rec.Orders},
		{"get-refund-*.json", &rec.Refunds},
		{"get-dispute-*.json", &rec.Disputes},
	}
	for _, target := range targets {
		paths, err := filepath.Glob(filepath.Join(dir, target.pattern))
		if err != nil {
			return Recordings{}, fmt.Errorf("arm2: list %s in %s: %w", target.pattern, dir, err)
		}
		sort.Strings(paths)
		for _, path := range paths {
			raw, err := os.ReadFile(path)
			if err != nil {
				return Recordings{}, fmt.Errorf("arm2: read %s: %w", path, err)
			}
			*target.dest = append(*target.dest, raw)
		}
	}
	if len(rec.Transactions)+len(rec.Orders)+len(rec.Refunds)+len(rec.Disputes) == 0 {
		return Recordings{}, fmt.Errorf("arm2: load %s: %w", dir, ErrNoRecordings)
	}
	return rec, nil
}

// Extract returns arm 2's prompt input as a JSON array of records. It
// keeps the shopper's transaction details whole, trims orders, refunds and
// disputes to the fields that state what happened and what it cost, and
// drops every other shopper's rows. Every field it writes came from a
// toolkit response: it derives no reason, category, signal strength or
// belief.
func Extract(rec Recordings, sel Selector) ([]byte, error) {
	if strings.TrimSpace(sel.CustomID) == "" && strings.TrimSpace(sel.PayerEmail) == "" {
		return nil, ErrNoSelector
	}
	transactions, transactionIDs, err := selectTransactions(rec.Transactions, sel)
	if err != nil {
		return nil, err
	}
	orders, err := selectOrders(rec.Orders, sel)
	if err != nil {
		return nil, err
	}
	refunds, err := selectRefunds(rec.Refunds, sel, transactionIDs)
	if err != nil {
		return nil, err
	}
	disputes, err := selectDisputes(rec.Disputes, sel, transactionIDs)
	if err != nil {
		return nil, err
	}
	records := make([]map[string]any, 0,
		len(transactions)+len(orders)+len(refunds)+len(disputes))
	records = append(records, transactions...)
	records = append(records, orders...)
	records = append(records, refunds...)
	records = append(records, disputes...)
	encoded, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("arm2: encode input: %w", err)
	}
	return encoded, nil
}

// selectTransactions picks the shopper's transaction details out of every
// list_transactions response, keeps the first copy of each transaction id,
// and returns the ids it kept so refunds and disputes can be tied to them.
func selectTransactions(raws [][]byte, sel Selector) ([]map[string]any, map[string]bool, error) {
	seen := map[string]bool{}
	ids := map[string]bool{}
	var out []map[string]any
	for _, raw := range raws {
		root, err := decodeObject(raw, "list_transactions")
		if err != nil {
			return nil, nil, err
		}
		details, ok := root["transaction_details"]
		if !ok {
			continue
		}
		items, ok := details.([]any)
		if !ok {
			return nil, nil, fmt.Errorf("%w: transaction_details is not an array", ErrMalformed)
		}
		for _, item := range items {
			detail, ok := item.(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("%w: transaction detail is not an object", ErrMalformed)
			}
			if !transactionMatches(detail, sel) {
				continue
			}
			id := nestedString(detail, "transaction_info", "transaction_id")
			if id != "" {
				if seen[id] {
					continue
				}
				seen[id] = true
				ids[id] = true
			}
			out = append(out, detail)
		}
	}
	sortByDateThenID(out,
		[]string{"transaction_info", "transaction_initiation_date"},
		[]string{"transaction_info", "transaction_id"})
	return out, ids, nil
}

// transactionMatches reports whether a transaction detail belongs to the
// shopper. A custom id wins; a payer email is the fallback for a reporting
// response that carried payer_info.
func transactionMatches(detail map[string]any, sel Selector) bool {
	if sel.CustomID != "" &&
		strings.EqualFold(nestedString(detail, "transaction_info", "custom_field"), sel.CustomID) {
		return true
	}
	return sel.PayerEmail != "" &&
		strings.EqualFold(nestedString(detail, "payer_info", "email_address"), sel.PayerEmail)
}

// selectOrders picks the shopper's orders and trims each to the fields
// that name the purchase and its line items.
func selectOrders(raws [][]byte, sel Selector) ([]map[string]any, error) {
	var out []map[string]any
	for _, raw := range raws {
		order, err := decodeObject(raw, "get_order")
		if err != nil {
			return nil, err
		}
		if !orderMatches(order, sel) {
			continue
		}
		out = append(out, trimOrder(order))
	}
	sortByDateThenID(out, []string{"create_time"}, []string{"id"})
	return out, nil
}

// orderMatches reports whether any purchase unit on the order carries the
// shopper's custom id, or the payer email matches.
func orderMatches(order map[string]any, sel Selector) bool {
	for _, unit := range nestedSlice(order, "purchase_units") {
		fields, ok := unit.(map[string]any)
		if !ok {
			continue
		}
		if sel.CustomID != "" &&
			strings.EqualFold(nestedString(fields, "custom_id"), sel.CustomID) {
			return true
		}
	}
	return sel.PayerEmail != "" &&
		strings.EqualFold(nestedString(order, "payer", "email_address"), sel.PayerEmail)
}

// trimOrder keeps the order fields that state the order and its items.
func trimOrder(order map[string]any) map[string]any {
	trimmed := map[string]any{}
	copyKeys(trimmed, order, "id", "status", "intent", "create_time")
	units, ok := order["purchase_units"].([]any)
	if !ok {
		return trimmed
	}
	unitOut := make([]any, 0, len(units))
	for _, unit := range units {
		fields, ok := unit.(map[string]any)
		if !ok {
			continue
		}
		kept := map[string]any{}
		copyKeys(kept, fields, "reference_id", "custom_id", "description", "amount", "items")
		unitOut = append(unitOut, kept)
	}
	trimmed["purchase_units"] = unitOut
	return trimmed
}

// selectRefunds picks the shopper's refunds. A refund belongs to the
// shopper when its custom_id matches, when its id is one of the selected
// transactions (a refund row and the refund record share the id), or when
// its "up" link names a selected capture.
func selectRefunds(raws [][]byte, sel Selector, transactionIDs map[string]bool) ([]map[string]any, error) {
	seen := map[string]bool{}
	var out []map[string]any
	for _, raw := range raws {
		refund, err := decodeObject(raw, "get_refund")
		if err != nil {
			return nil, err
		}
		if !refundMatches(refund, sel, transactionIDs) {
			continue
		}
		id := nestedString(refund, "id")
		if id != "" {
			if seen[id] {
				continue
			}
			seen[id] = true
		}
		out = append(out, trimRefund(refund))
	}
	sortByDateThenID(out, []string{"create_time"}, []string{"id"})
	return out, nil
}

// refundMatches reports whether a refund record belongs to the shopper.
func refundMatches(refund map[string]any, sel Selector, transactionIDs map[string]bool) bool {
	if sel.CustomID != "" &&
		strings.EqualFold(nestedString(refund, "custom_id"), sel.CustomID) {
		return true
	}
	if transactionIDs[nestedString(refund, "id")] {
		return true
	}
	for _, link := range nestedSlice(refund, "links") {
		fields, ok := link.(map[string]any)
		if !ok || nestedString(fields, "rel") != "up" {
			continue
		}
		if transactionIDs[pathTail(nestedString(fields, "href"))] {
			return true
		}
	}
	return false
}

// trimRefund keeps the refund fields that state its status, amount and
// the seller's note. get_refund returns no reason field; none is added.
func trimRefund(refund map[string]any) map[string]any {
	trimmed := map[string]any{}
	copyKeys(trimmed, refund, "id", "amount", "status", "custom_id",
		"create_time", "update_time", "note_to_payer", "seller_payable_breakdown")
	return trimmed
}

// selectDisputes picks the shopper's disputes. A dispute belongs to the
// shopper when a disputed transaction carries their custom id, when its
// seller transaction is one of the selected transactions (the held
// capture), or when the buyer email matches.
func selectDisputes(raws [][]byte, sel Selector, transactionIDs map[string]bool) ([]map[string]any, error) {
	seen := map[string]bool{}
	var out []map[string]any
	for _, raw := range raws {
		dispute, err := decodeObject(raw, "get_dispute")
		if err != nil {
			return nil, err
		}
		if !disputeMatches(dispute, sel, transactionIDs) {
			continue
		}
		id := nestedString(dispute, "dispute_id")
		if id != "" {
			if seen[id] {
				continue
			}
			seen[id] = true
		}
		out = append(out, trimDispute(dispute))
	}
	sortByDateThenID(out, []string{"create_time"}, []string{"dispute_id"})
	return out, nil
}

// disputeMatches reports whether a dispute record belongs to the shopper.
func disputeMatches(dispute map[string]any, sel Selector, transactionIDs map[string]bool) bool {
	for _, transaction := range nestedSlice(dispute, "disputed_transactions") {
		fields, ok := transaction.(map[string]any)
		if !ok {
			continue
		}
		if sel.CustomID != "" &&
			strings.EqualFold(nestedString(fields, "custom"), sel.CustomID) {
			return true
		}
		if transactionIDs[nestedString(fields, "seller_transaction_id")] {
			return true
		}
		if sel.PayerEmail != "" &&
			strings.EqualFold(nestedString(fields, "buyer", "email"), sel.PayerEmail) {
			return true
		}
	}
	return false
}

// trimDispute keeps the dispute fields that state its status, reason,
// amounts and disputed items. Buyer and seller identities and the message
// and evidence bodies are left out.
func trimDispute(dispute map[string]any) map[string]any {
	trimmed := map[string]any{}
	copyKeys(trimmed, dispute, "dispute_id", "reason", "status", "dispute_state",
		"dispute_life_cycle_stage", "dispute_channel", "dispute_amount",
		"create_time", "update_time", "seller_response_due_date")
	transactions, ok := dispute["disputed_transactions"].([]any)
	if ok {
		transactionOut := make([]any, 0, len(transactions))
		for _, transaction := range transactions {
			fields, ok := transaction.(map[string]any)
			if !ok {
				continue
			}
			kept := map[string]any{}
			copyKeys(kept, fields, "buyer_transaction_id", "seller_transaction_id",
				"create_time", "transaction_status", "gross_amount", "custom",
				"items", "seller_protection_eligible")
			transactionOut = append(transactionOut, kept)
		}
		trimmed["disputed_transactions"] = transactionOut
	}
	return trimmed
}

// decodeObject decodes one toolkit response body into a JSON object.
func decodeObject(raw []byte, tool string) (map[string]any, error) {
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrMalformed, tool, err)
	}
	if object == nil {
		return nil, fmt.Errorf("%w: %s: not an object", ErrMalformed, tool)
	}
	return object, nil
}

// copyKeys copies the named keys that exist and are not nil.
func copyKeys(dst, src map[string]any, keys ...string) {
	for _, key := range keys {
		value, ok := src[key]
		if !ok || value == nil {
			continue
		}
		dst[key] = value
	}
}

// nestedString walks a chain of object keys and returns the string it
// finds, or the empty string.
func nestedString(object map[string]any, keys ...string) string {
	value, ok := nestedValue(object, keys...)
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

// nestedSlice walks a chain of object keys and returns the array it
// finds, or nil.
func nestedSlice(object map[string]any, keys ...string) []any {
	value, ok := nestedValue(object, keys...)
	if !ok {
		return nil
	}
	items, _ := value.([]any)
	return items
}

// nestedValue walks a chain of object keys.
func nestedValue(object map[string]any, keys ...string) (any, bool) {
	var current any = object
	for _, key := range keys {
		fields, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = fields[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// pathTail returns the last path segment of a URL, which is the resource
// id on a PayPal link.
func pathTail(url string) string {
	url = strings.TrimSuffix(url, "/")
	if index := strings.LastIndex(url, "/"); index >= 0 {
		return url[index+1:]
	}
	return url
}

// sortByDateThenID orders records by a nested date key, then by a nested
// id key, so the input is deterministic across runs.
func sortByDateThenID(records []map[string]any, dateKeys, idKeys []string) {
	sort.SliceStable(records, func(i, j int) bool {
		leftDate, _ := nestedValue(records[i], dateKeys...)
		rightDate, _ := nestedValue(records[j], dateKeys...)
		left, leftOK := leftDate.(string)
		right, rightOK := rightDate.(string)
		if leftOK && rightOK && left != right {
			return left < right
		}
		leftID, _ := nestedValue(records[i], idKeys...)
		rightID, _ := nestedValue(records[j], idKeys...)
		leftText, _ := leftID.(string)
		rightText, _ := rightID.(string)
		return leftText < rightText
	})
}
