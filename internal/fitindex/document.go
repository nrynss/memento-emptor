package fitindex

import (
	"encoding/json"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// The direction words the index stores. The domain's words are small and
// large; the recorded mapping's rows say too_small and too_large, and the
// writer maps between them.
const (
	indexDirectionTooSmall = "too_small"
	indexDirectionTooLarge = "too_large"
)

// mappingJSON is the fit index's mapping, exactly as T0.12 recorded it. The
// deployment supplies its own mode and settings, so the client sends the
// mapping and nothing else. Dynamic is strict: a field outside this list
// is refused at write time, which is how the row stays free of prose. The
// recorded mapping carries event_seq, but the row does not: T7.1 made the
// version the state rank with event time inside it.
const mappingJSON = `{
  "mappings": {
    "dynamic": "strict",
    "properties": {
      "txn_hash":         {"type": "keyword"},
      "shopper_hash":     {"type": "keyword"},
      "brand":            {"type": "keyword"},
      "category":         {"type": "keyword"},
      "fit_handle":       {"type": "keyword"},
      "size":             {"type": "keyword"},
      "purchase_state":   {"type": "keyword"},
      "settled":          {"type": "boolean"},
      "settled_kind":     {"type": "keyword"},
      "reason_category":  {"type": "keyword"},
      "fit_direction":    {"type": "keyword"},
      "feedback_verdict": {"type": "keyword"},
      "reason_contested": {"type": "boolean"},
      "seeded":           {"type": "boolean"},
      "captured_at":      {"type": "date"},
      "refunded_at":      {"type": "date"},
      "settled_at":       {"type": "date"},
      "event_seq":        {"type": "integer"}
    }
  }
}`

// Mapping returns a copy of the mapping every task creates the index with.
func Mapping() json.RawMessage {
	return append(json.RawMessage(nil), mappingJSON...)
}

// document is the wire shape of one fit row. Every field is a keyword, a
// boolean or a date; there is no prose field, no price, no order detail
// and no shopper identity. Empty fields are left out, so the row does not
// claim a reason, verdict or time it does not have.
type document struct {
	TxnHash         string `json:"txn_hash"`
	ShopperHash     string `json:"shopper_hash"`
	Brand           string `json:"brand"`
	Category        string `json:"category"`
	FitHandle       string `json:"fit_handle"`
	Size            string `json:"size"`
	PurchaseState   string `json:"purchase_state"`
	Settled         bool   `json:"settled"`
	SettledKind     string `json:"settled_kind,omitempty"`
	ReasonCategory  string `json:"reason_category,omitempty"`
	FitDirection    string `json:"fit_direction,omitempty"`
	FeedbackVerdict string `json:"feedback_verdict,omitempty"`
	ReasonContested bool   `json:"reason_contested,omitempty"`
	Seeded          bool   `json:"seeded"`
	CapturedAt      string `json:"captured_at,omitempty"`
	RefundedAt      string `json:"refunded_at,omitempty"`
	SettledAt       string `json:"settled_at,omitempty"`
}

// documentFor builds one row's document. The reason category is written as
// the domain names it, because the pool and the merchant view count the
// same categories; the direction is written as the index records it.
func documentFor(row domain.FitRow) document {
	doc := document{
		TxnHash:         row.TxnHash,
		ShopperHash:     row.ShopperHash,
		Brand:           row.Line.Brand,
		Category:        row.Line.Category,
		FitHandle:       row.Line.FitHandle,
		Size:            row.Size,
		PurchaseState:   string(row.State),
		Settled:         row.Settled,
		Seeded:          row.Seeded,
		FeedbackVerdict: string(row.Verdict),
	}
	if row.SettledKind != "" {
		doc.SettledKind = string(row.SettledKind)
	}
	if row.Reason != "" && row.Reason != domain.CategoryNone {
		doc.ReasonCategory = string(row.Reason)
	}
	doc.FitDirection = indexDirection(row.Direction)
	doc.ReasonContested = row.ReasonContested
	doc.CapturedAt = formatEventTime(row.CapturedAt)
	doc.RefundedAt = formatEventTime(row.RefundedAt)
	// A settlement time is the time the row settled, so it goes with the
	// settled flag: a row the shopper's correction left without a cause
	// does not claim one.
	if row.Settled {
		doc.SettledAt = formatEventTime(row.SettledAt)
	}
	return doc
}

// indexDirection maps a domain direction to the word the index stores.
func indexDirection(dir domain.FitDirection) string {
	switch dir {
	case domain.FitDirectionSmall:
		return indexDirectionTooSmall
	case domain.FitDirectionLarge:
		return indexDirectionTooLarge
	default:
		return ""
	}
}

// formatEventTime writes an event time the way the index stores it, and
// leaves an unknown time out.
func formatEventTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
