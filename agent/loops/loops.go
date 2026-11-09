package loops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nrynss/memento-emptor/agent/orders"
)

// The sentinels for loop failures. Match them with errors.Is.
var (
	// ErrNotApproved is returned when the shopper's reply is anything
	// but an explicit yes. The order does not move and the transport is
	// never mirrored: an approval is a yes, never an inference.
	ErrNotApproved = errors.New("loops: shopper has not approved")

	// ErrNoPending is returned when the agent is asked to build the one
	// short question but no refund is pending.
	ErrNoPending = errors.New("loops: no pending refund to ask about")

	// ErrMalformed is returned when a loop step carries an empty id or
	// an empty shopper reply where one is required.
	ErrMalformed = errors.New("loops: empty id or empty reply")
)

// PendingQuestion is the smallest refund description the one-question
// builder needs: which question, what went back, and the transaction
// the answer must tie to.
type PendingQuestion struct {
	// QuestionID is the pending_questions id the answer returns to.
	QuestionID string
	// LineName and Size name the item in the shopper's words.
	LineName string
	Size     string
	// CustomID joins the refund to its order and capture.
	CustomID string
}

// AskOne builds the single short question for the first pending refund:
// one item, one size, no lecture. It reports ErrNoPending when there is
// nothing to ask, and ErrMalformed when the first entry names no
// question, so a reason can never attach to the wrong transaction.
func AskOne(pending []PendingQuestion) (string, error) {
	if len(pending) == 0 {
		return "", fmt.Errorf("%w", ErrNoPending)
	}
	first := pending[0]
	if first.QuestionID == "" || first.CustomID == "" {
		return "", fmt.Errorf("%w: first pending refund names no question or transaction", ErrMalformed)
	}
	item := first.LineName
	if item == "" {
		item = "item"
	}
	size := first.Size
	if size == "" {
		size = "that"
	}
	return fmt.Sprintf("Your %s in size %s came back — what went wrong with the fit?", item, size), nil
}

// IsApprovalYes reports whether the shopper's reply is an explicit yes.
// Only a bare yes counts: "yes", "approve" and "confirm" with optional
// case and trailing punctuation. Anything longer ("yes but...", "maybe",
// silence) is not an approval, and the caller must ask again.
func IsApprovalYes(reply string) bool {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(reply), ".!")) {
	case "yes", "y", "approve", "approved", "confirm", "confirmed":
		return true
	default:
		return false
	}
}

// ApprovalMirror records a shopper approval on the payment transport, so
// the source side agrees the order was approved before any capture. The
// fake's ApproveOrder stands in for the human click offline; the live
// REST client implements the same one-method seam.
type ApprovalMirror interface {
	// MarkApproved records the approval for the order id.
	MarkApproved(orderID string)
}

// ApproveAfterYes records the shopper's approval for a proposed order,
// but only after a real explicit yes. A non-yes reply returns
// ErrNotApproved and changes nothing: the order stays proposed and the
// transport is never mirrored. On a yes it calls Service.Approve and
// then mirrors into the transport, so the local record and the payment
// source agree before capture. Capture without this step fails closed
// in the orders package.
func ApproveAfterYes(ctx context.Context, svc *orders.Service, mirror ApprovalMirror, orderID, reply string) (*orders.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("loops: approve: %w", err)
	}
	if svc == nil || mirror == nil || orderID == "" {
		return nil, fmt.Errorf("loops: approve: %w", ErrMalformed)
	}
	if !IsApprovalYes(reply) {
		return nil, fmt.Errorf("loops: approve order %q: %w: reply %q is not an explicit yes", orderID, ErrNotApproved, reply)
	}
	approved, err := svc.Approve(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("loops: approve order %q: %w", orderID, err)
	}
	mirror.MarkApproved(orderID)
	return approved, nil
}

// RecommendationFor picks the size the very next recommendation must
// carry. A live correction always wins: the returned text is the
// correction, and the belief value rides beside it as context, never as
// the decision. With no correction the belief value stands.
func RecommendationFor(correctionText, beliefValue string) string {
	if strings.TrimSpace(correctionText) != "" {
		return correctionText
	}
	return beliefValue
}

// Feedback is explicit fit feedback tied to one purchase. Every field is
// required: feedback without its transaction counts toward nothing.
type Feedback struct {
	// CustomID joins the feedback to its order and capture.
	CustomID string
	// SKU and Size name exactly what was worn.
	SKU  string
	Size string
	// Text carries the shopper's verbatim words about the fit.
	Text string
}

// Validate reports ErrMalformed when any field is empty. The layer
// refuses untied feedback; this refuses to build it.
func (f Feedback) Validate() error {
	if f.CustomID == "" || f.SKU == "" || f.Size == "" || strings.TrimSpace(f.Text) == "" {
		return fmt.Errorf("%w: feedback must carry its custom id, sku, size and words", ErrMalformed)
	}
	return nil
}
