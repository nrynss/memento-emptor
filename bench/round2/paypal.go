package round2

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/bench/round2/arm2"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/seed"
)

// Arm 2's input, generated from the frozen shoppers. Round 2's shoppers are
// not sandbox purchases, so their PayPal history is generated in the same
// shape T10.1 recorded: a transaction listing, get_order bodies with the
// line items, get_refund bodies and get_dispute bodies. Every record is
// filtered to its shopper by one custom id, every free-text field is
// fit-neutral, and no field the toolkit does not return is added.

// PayPalCustomID is the per-shopper custom id every generated record
// carries. The toolkit's listing has no shopper filter, so this is the key
// the extraction selects on.
func PayPalCustomID(shopperID string) string {
	return "r2-" + shopperID
}

// PayPalRecords is one shopper's toolkit responses, in the shape the
// extraction reads: each slice holds one tool call's body.
type PayPalRecords struct {
	// Succeeded is the list_transactions body for successful transactions,
	// or nil when the shopper has none.
	Succeeded []byte
	// Pending is the list_transactions body for pending transactions (a
	// dispute's hold), or nil when there is none.
	Pending []byte
	// Orders holds one get_order body per purchase.
	Orders [][]byte
	// Refunds holds one get_refund body per refund.
	Refunds [][]byte
	// Disputes holds one get_dispute body per dispute.
	Disputes [][]byte
}

// BuildPayPalRecords generates one shopper's toolkit responses from the
// frozen history. A statement produces nothing: it is not a transaction.
// A kept or feedback purchase is a capture with no refund, exactly as the
// toolkit shows one; a refund is a capture, a T1107 row and a get_refund
// body; a dispute is a capture, a T1110 hold in the pending listing and a
// get_dispute body with PayPal's reason code.
func BuildPayPalRecords(shopper seed.Round2Shopper) (PayPalRecords, error) {
	customID := PayPalCustomID(shopper.ShopperID)
	var records PayPalRecords
	succeeded := []any{}
	pending := []any{}
	for index, row := range shopper.History.Rows {
		if row.Outcome == nil {
			continue
		}
		if row.Outcome.AmountMinor <= 0 {
			return PayPalRecords{}, fmt.Errorf("round2: %s row %d carries no amount", shopper.ShopperID, index)
		}
		amount := formatAmount(row.Outcome.AmountMinor)
		orderedAt := transactionTime(row.Day)
		captureID := row.Link.CaptureID
		succeeded = append(succeeded, transactionBody(map[string]any{
			"transaction_id":              captureID,
			"transaction_event_code":      "T0006",
			"transaction_initiation_date": orderedAt,
			"transaction_updated_date":    orderedAt,
			"transaction_amount":          map[string]any{"currency_code": "USD", "value": amount},
			"fee_amount":                  map[string]any{"currency_code": "USD", "value": "-" + amount},
			"transaction_status":          "S",
			"transaction_subject":         "Store purchase, 1 item",
			"custom_field":                customID,
			"protection_eligibility":      "01",
			"instrument_type":             "PayPal",
			"instrument_sub_type":         "PayPal Wallet",
		}))
		records.Orders = append(records.Orders, marshalOrder(orderBody(row, customID, amount)))
		switch row.Outcome.Kind {
		case domain.OutcomeRefund:
			refundAt := transactionTime(row.Day + 0.25)
			succeeded = append(succeeded, transactionBody(map[string]any{
				"transaction_id":              row.Link.RefundID,
				"paypal_reference_id":         captureID,
				"paypal_reference_id_type":    "TXN",
				"transaction_event_code":      "T1107",
				"transaction_initiation_date": refundAt,
				"transaction_updated_date":    refundAt,
				"transaction_amount":          map[string]any{"currency_code": "USD", "value": "-" + amount},
				"transaction_status":          "S",
				"transaction_subject":         "Refund for the returned item",
				"transaction_note":            "Refund for the returned item",
				"custom_field":                customID,
				"protection_eligibility":      "02",
				"instrument_type":             "PayPal",
				"instrument_sub_type":         "PayPal Wallet",
			}))
			records.Refunds = append(records.Refunds, marshalRefund(refundBody(row, customID, amount, refundAt)))
		case domain.OutcomeDispute:
			pending = append(pending, transactionBody(map[string]any{
				"transaction_id":              "HOLD-" + row.Link.DisputeID,
				"paypal_reference_id":         captureID,
				"paypal_reference_id_type":    "TXN",
				"transaction_event_code":      "T1110",
				"transaction_initiation_date": transactionTime(row.Day + 0.2),
				"transaction_updated_date":    transactionTime(row.Day + 0.2),
				"transaction_amount":          map[string]any{"currency_code": "USD", "value": "-" + amount},
				"transaction_status":          "P",
				"custom_field":                customID,
				"protection_eligibility":      "02",
				"instrument_type":             "PayPal",
				"instrument_sub_type":         "PayPal Wallet",
			}))
			records.Disputes = append(records.Disputes, marshalDispute(disputeBody(row, customID, amount)))
		}
	}
	if len(succeeded) > 0 {
		records.Succeeded = marshalListing(succeeded, "S", transactionTime(shopper.History.SessionDays))
	}
	if len(pending) > 0 {
		records.Pending = marshalListing(pending, "P", transactionTime(shopper.History.SessionDays))
	}
	sortBodies(records.Orders, "create_time")
	sortBodies(records.Refunds, "create_time")
	sortBodies(records.Disputes, "create_time")
	return records, nil
}

// Recordings returns the records in the shape the extraction takes.
func (r PayPalRecords) Recordings() arm2.Recordings {
	rec := arm2.Recordings{Orders: r.Orders, Refunds: r.Refunds, Disputes: r.Disputes}
	if len(r.Succeeded) > 0 {
		rec.Transactions = append(rec.Transactions, r.Succeeded)
	}
	if len(r.Pending) > 0 {
		rec.Transactions = append(rec.Transactions, r.Pending)
	}
	return rec
}

// ArmTwoInput builds arm 2's prompt input for one shopper: the T10.1
// extraction over that shopper's generated toolkit responses. It carries
// transactions, line items, refund statuses and dispute reason codes, and
// no refund reason, classification or belief.
func ArmTwoInput(shopper seed.Round2Shopper) ([]byte, error) {
	records, err := BuildPayPalRecords(shopper)
	if err != nil {
		return nil, err
	}
	input, err := arm2.Extract(records.Recordings(), arm2.Selector{CustomID: PayPalCustomID(shopper.ShopperID)})
	if err != nil {
		return nil, fmt.Errorf("round2: extract arm 2 input for %q: %w", shopper.ShopperID, err)
	}
	return input, nil
}

// WritePayPal writes every shopper's toolkit responses under dir, one
// directory per shopper, with the file names arm2.Load reads. A shopper
// with no pending hold gets no pending listing.
func WritePayPal(dir string) error {
	shoppers, err := LoadShoppers()
	if err != nil {
		return err
	}
	for _, shopper := range shoppers {
		records, err := BuildPayPalRecords(shopper)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, PayPalDir, shopper.ShopperID)
		if err := os.MkdirAll(target, 0o755); err != nil {
			return fmt.Errorf("round2: write paypal records: %w", err)
		}
		if err := writeRecordings(target, records); err != nil {
			return err
		}
	}
	return nil
}

// LoadArmTwoInput reads one shopper's committed toolkit responses and runs
// the frozen extraction over them, exactly as the arm-2 driver does.
func LoadArmTwoInput(dir, shopperID string) ([]byte, error) {
	records, err := arm2.Load(filepath.Join(dir, PayPalDir, shopperID))
	if err != nil {
		return nil, fmt.Errorf("round2: load arm 2 records for %q: %w", shopperID, err)
	}
	input, err := arm2.Extract(records, arm2.Selector{CustomID: PayPalCustomID(shopperID)})
	if err != nil {
		return nil, fmt.Errorf("round2: extract arm 2 input for %q: %w", shopperID, err)
	}
	return input, nil
}

// writeRecordings writes one shopper's responses under the names
// arm2.Load reads.
func writeRecordings(dir string, records PayPalRecords) error {
	write := func(name string, body []byte) error {
		if len(body) == 0 {
			return nil
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
			return fmt.Errorf("round2: write %s: %w", path, err)
		}
		return nil
	}
	if err := write("list-transactions-succeeded.json", records.Succeeded); err != nil {
		return err
	}
	if err := write("list-transactions-pending.json", records.Pending); err != nil {
		return err
	}
	for i, body := range records.Orders {
		if err := write(fmt.Sprintf("get-order-%d.json", i+1), body); err != nil {
			return err
		}
	}
	for i, body := range records.Refunds {
		if err := write(fmt.Sprintf("get-refund-%d.json", i+1), body); err != nil {
			return err
		}
	}
	for i, body := range records.Disputes {
		if err := write(fmt.Sprintf("get-dispute-%d.json", i+1), body); err != nil {
			return err
		}
	}
	return nil
}

// transactionTime renders one row's day as a timestamp measured from the
// round-2 anchor, at a fixed time of day so the fixture is byte-stable.
func transactionTime(day float64) string {
	return seed.Round2Anchor().Add(time.Duration(day * float64(24*time.Hour))).UTC().Format(time.RFC3339)
}

// formatAmount renders minor units as the toolkit's decimal string.
func formatAmount(minor int64) string {
	if minor < 0 {
		minor = -minor
	}
	return fmt.Sprintf("%d.%02d", minor/100, minor%100)
}

// transactionBody wraps one transaction_info object in a listing entry.
func transactionBody(info map[string]any) any {
	return map[string]any{"transaction_info": info}
}

// orderBody builds one get_order body: the order, its purchase unit, and
// one line item on the candidate or purchased line.
func orderBody(row seed.Row, customID, amount string) map[string]any {
	created := transactionTime(row.Day)
	return map[string]any{
		"id":          row.Link.OrderID,
		"intent":      "CAPTURE",
		"status":      "COMPLETED",
		"create_time": created,
		"purchase_units": []any{map[string]any{
			"reference_id": "r2-" + row.Item.SKU,
			"custom_id":    customID,
			"description":  itemDescription(row.Item),
			"amount":       map[string]any{"currency_code": "USD", "value": amount},
			"items": []any{map[string]any{
				"name":        itemDescription(row.Item),
				"unit_amount": map[string]any{"currency_code": "USD", "value": amount},
				"quantity":    "1",
				"description": "Size " + row.Item.Size,
				"sku":         row.Item.SKU,
				"category":    itemCategory(row.Item),
			}},
		}},
	}
}

// refundBody builds one get_refund body. Its note repeats the listing
// row's subject and note, as the toolkit's does, and says nothing about
// why the item went back.
func refundBody(row seed.Row, customID, amount, at string) map[string]any {
	return map[string]any{
		"id":            row.Link.RefundID,
		"amount":        map[string]any{"currency_code": "USD", "value": amount},
		"note_to_payer": "Refund for the returned item",
		"seller_payable_breakdown": map[string]any{
			"gross_amount":          map[string]any{"currency_code": "USD", "value": amount},
			"paypal_fee":            map[string]any{"currency_code": "USD", "value": "0.00"},
			"net_amount":            map[string]any{"currency_code": "USD", "value": amount},
			"total_refunded_amount": map[string]any{"currency_code": "USD", "value": amount},
		},
		"custom_id":   customID,
		"status":      "COMPLETED",
		"create_time": at,
		"update_time": at,
		"links": []any{
			map[string]any{"href": "https://api.sandbox.paypal.com/v2/payments/refunds/" + row.Link.RefundID, "rel": "self", "method": "GET"},
			map[string]any{"href": "https://api.sandbox.paypal.com/v2/payments/captures/" + row.Link.CaptureID, "rel": "up", "method": "GET"},
		},
	}
}

// disputeBody builds one get_dispute body. The reason is PayPal's code,
// which classifies the dispute for the seller, not for fit.
func disputeBody(row seed.Row, customID, amount string) map[string]any {
	code := string(domain.DisputeMerchandiseNotReceived)
	if row.Reason != nil {
		if _, err := domain.CategoryForDispute(domain.DisputeCode(row.Reason.Text)); err == nil {
			code = row.Reason.Text
		}
	}
	return map[string]any{
		"dispute_id":               row.Link.DisputeID,
		"create_time":              transactionTime(row.Day),
		"update_time":              transactionTime(row.Day + 0.1),
		"reason":                   code,
		"status":                   "WAITING_FOR_SELLER_RESPONSE",
		"dispute_state":            "REQUIRED_ACTION",
		"dispute_life_cycle_stage": "INQUIRY",
		"dispute_channel":          "INTERNAL",
		"dispute_amount":           map[string]any{"currency_code": "USD", "value": amount},
		"seller_response_due_date": transactionTime(row.Day + 20),
		"disputed_transactions": []any{map[string]any{
			"buyer_transaction_id":  "BUYER-" + row.Link.CaptureID,
			"seller_transaction_id": row.Link.CaptureID,
			"create_time":           transactionTime(row.Day),
			"transaction_status":    "HELD",
			"gross_amount":          map[string]any{"currency_code": "USD", "value": amount},
			"custom":                customID,
			"items": []any{map[string]any{
				"item_id":       row.Item.SKU,
				"item_name":     itemDescription(row.Item),
				"item_quantity": "1",
				"reason":        code,
				"item_type":     "PRODUCT",
			}},
			"seller_protection_eligible": true,
		}},
	}
}

// itemDescription names the purchased item without saying anything about
// why it went back or how it fitted.
func itemDescription(item domain.Item) string {
	name := strings.TrimSpace(item.BrandName + " " + item.LineName)
	if name == "" {
		name = item.SKU
	}
	return name
}

// itemCategory maps a seeded category slug onto the toolkit's category.
func itemCategory(item domain.Item) string {
	switch item.Category {
	case "hats":
		return "DIGITAL_GOODS"
	default:
		return "PHYSICAL_GOODS"
	}
}

// marshalListing encodes one list_transactions body with the fields the
// tool returns beside transaction_details.
func marshalListing(entries []any, status, end string) []byte {
	body := map[string]any{
		"transaction_details":     entries,
		"start_date":              transactionTime(0),
		"end_date":                end,
		"last_refreshed_datetime": end,
		"page":                    1,
		"total_items":             len(entries),
		"total_pages":             1,
	}
	encoded, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return nil
	}
	return encoded
}

// marshalOrder, marshalRefund and marshalDispute encode one body each.
func marshalOrder(body map[string]any) []byte {
	encoded, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return nil
	}
	return encoded
}

func marshalRefund(body map[string]any) []byte { return marshalOrder(body) }

func marshalDispute(body map[string]any) []byte { return marshalOrder(body) }

// sortBodies orders record bodies by their create_time so the committed
// files and the extraction's own ordering agree.
func sortBodies(bodies [][]byte, key string) {
	sort.SliceStable(bodies, func(i, j int) bool {
		return bodyField(bodies[i], key) < bodyField(bodies[j], key)
	})
}

// bodyField reads one top-level string field from an encoded body.
func bodyField(body []byte, key string) string {
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ""
	}
	value, _ := decoded[key].(string)
	return value
}
