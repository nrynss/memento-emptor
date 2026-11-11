package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// The REST paths the read source uses. They are the source's API, not a
// layer concept, so they stay in this package.
const (
	// OrdersPath reads one checkout order and its line items.
	OrdersPath = "/v2/checkout/orders/"
	// CapturesPath reads one capture.
	CapturesPath = "/v2/payments/captures/"
	// RefundsPath reads one refund.
	RefundsPath = "/v2/payments/refunds/"
	// DisputesPath reads one customer dispute.
	DisputesPath = "/v1/customer/disputes/"
	// WebhookEventsPath reads one stored webhook event by its id.
	WebhookEventsPath = "/v1/notifications/webhooks-events/"
)

// The event types the layer stores. A delivery of any other type is refused
// with ErrUnknownEvent.
const (
	// EventCaptureCompleted is a completed capture.
	EventCaptureCompleted = "PAYMENT.CAPTURE.COMPLETED"
	// EventCaptureRefunded is a refunded capture.
	EventCaptureRefunded = "PAYMENT.CAPTURE.REFUNDED"
	// EventDisputeCreated is a created customer dispute.
	EventDisputeCreated = "CUSTOMER.DISPUTE.CREATED"
)

// LineItem is one order line as the payment source states it: the fields
// the source carries, before the layer's catalog joins them to a product
// line and variant.
type LineItem struct {
	// Name is the line's human name.
	Name string
	// Description is the source's optional line description.
	Description string
	// SKU is the retailer's stock-keeping unit.
	SKU string
	// Category is the source's category, such as PHYSICAL_GOODS.
	Category string
	// Size is the purchased size parsed from the description or name, or
	// empty when the line states none.
	Size string
}

// Catalog joins one source line item to the layer's item. The deployment
// supplies the join, because only its catalog knows the brand id and the
// category slug a product line is keyed by. A catalog that does not know a
// line reports false and the source falls back to the line's own fields.
type Catalog interface {
	// Item returns the layer's item for one line, or false when the
	// catalog does not know it.
	Item(LineItem) (domain.Item, bool)
}

// CatalogFunc adapts a function to Catalog.
type CatalogFunc func(LineItem) (domain.Item, bool)

// Item calls f.
func (f CatalogFunc) Item(line LineItem) (domain.Item, bool) {
	return f(line)
}

// WithCatalog joins order lines through the deployment's catalog instead
// of the source's own fallback mapping.
func WithCatalog(catalog Catalog) Option {
	return func(c *Client) {
		c.catalog = catalog
	}
}

// Capture is one capture read: what was captured, from which order, and
// the custom id that joins it to the rest of the purchase.
type Capture struct {
	// CaptureID is the capture id.
	CaptureID string
	// OrderID is the checkout order the capture belongs to.
	OrderID string
	// CustomID is the shopper-scoped reference the capture carries.
	CustomID string
	// Outcome is the completed capture and its amount.
	Outcome domain.Outcome
}

// Compile-time proof the client is both the delivery gate and the payment
// source the layer reads through.
var _ fakes.PaymentSource = (*Client)(nil)

// VerifyWebhook returns the verified delivery for one event id. The event
// is read from the source by id; the caller has already verified the
// delivery's signature, so this resolves the event's outcome and links.
// An event type the layer does not store is refused with ErrUnknownEvent.
func (c *Client) VerifyWebhook(ctx context.Context, eventID string) (fakes.WebhookEvent, error) {
	if err := ctx.Err(); err != nil {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: read webhook event %q: %w", eventID, err)
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: read webhook event: %w: empty event id", ErrMalformedRecord)
	}
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	var envelope struct {
		// ID is the source's event id.
		ID string `json:"id"`
		// EventType is the delivery's type.
		EventType string `json:"event_type"`
		// Resource is the outcome the event carries.
		Resource json.RawMessage `json:"resource"`
	}
	if err := c.readJSON(ctx, WebhookEventsPath+url.PathEscape(eventID), &envelope); err != nil {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: read webhook event %q: %w", eventID, err)
	}
	if envelope.ID != "" && envelope.ID != eventID {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: read webhook event %q: %w: the source answered event %q",
			eventID, ErrMalformedRecord, envelope.ID)
	}
	switch envelope.EventType {
	case EventCaptureRefunded:
		return c.refundEvent(ctx, eventID, envelope.EventType, envelope.Resource)
	case EventCaptureCompleted:
		return c.captureEvent(ctx, eventID, envelope.EventType, envelope.Resource)
	case EventDisputeCreated:
		return c.disputeEvent(ctx, eventID, envelope.EventType, envelope.Resource)
	default:
		return fakes.WebhookEvent{}, fmt.Errorf("payments: read webhook event %q: type %q: %w",
			eventID, envelope.EventType, ErrUnknownEvent)
	}
}

// GetOrder returns one order read and its line items. The items are joined
// through the deployment's catalog when it knows the line, and otherwise
// mapped from the order's own name, SKU and category.
func (c *Client) GetOrder(ctx context.Context, orderID string) (fakes.OrderRecord, error) {
	if err := ctx.Err(); err != nil {
		return fakes.OrderRecord{}, fmt.Errorf("payments: read order %q: %w", orderID, err)
	}
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return fakes.OrderRecord{}, fmt.Errorf("payments: read order: %w: empty order id", ErrMalformedRecord)
	}
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	var order struct {
		// ID is the source's order id.
		ID string `json:"id"`
		// PurchaseUnits carry the line items and the custom id.
		PurchaseUnits []struct {
			// CustomID is the shopper-scoped reference.
			CustomID string `json:"custom_id"`
			// Description is the purchase unit's description.
			Description string `json:"description"`
			// Items are the order's line items.
			Items []struct {
				// Name is the line's human name.
				Name string `json:"name"`
				// Description is the line's optional description.
				Description string `json:"description"`
				// SKU is the retailer's stock-keeping unit.
				SKU string `json:"sku"`
				// Category is the source's category.
				Category string `json:"category"`
			} `json:"items"`
		} `json:"purchase_units"`
	}
	if err := c.readJSON(ctx, OrdersPath+url.PathEscape(orderID), &order); err != nil {
		return fakes.OrderRecord{}, fmt.Errorf("payments: read order %q: %w", orderID, err)
	}
	record := fakes.OrderRecord{
		OrderID: orderID,
		Link:    domain.TransactionLink{OrderID: orderID},
	}
	for _, unit := range order.PurchaseUnits {
		if record.Link.CustomID == "" {
			record.Link.CustomID = unit.CustomID
		}
		for _, entry := range unit.Items {
			line := LineItem{
				Name:        entry.Name,
				Description: strings.TrimSpace(entry.Description + " " + unit.Description),
				SKU:         entry.SKU,
				Category:    entry.Category,
			}
			line.Size = sizeOf(line)
			record.Items = append(record.Items, c.itemFor(line))
		}
	}
	return record, nil
}

// GetCapture returns one capture read: the amount, status and order the
// capture belongs to. A refund or a dispute names its capture, so this is
// the join the source follows to reach the order.
func (c *Client) GetCapture(ctx context.Context, captureID string) (Capture, error) {
	if err := ctx.Err(); err != nil {
		return Capture{}, fmt.Errorf("payments: read capture %q: %w", captureID, err)
	}
	captureID = strings.TrimSpace(captureID)
	if captureID == "" {
		return Capture{}, fmt.Errorf("payments: read capture: %w: empty capture id", ErrMalformedRecord)
	}
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	record, err := c.capture(ctx, captureID)
	if err != nil {
		return Capture{}, fmt.Errorf("payments: read capture %q: %w", captureID, err)
	}
	amount, err := record.Amount.minor()
	if err != nil {
		return Capture{}, fmt.Errorf("payments: read capture %q: %w", captureID, err)
	}
	orderID := record.orderID()
	if orderID == "" {
		return Capture{}, fmt.Errorf("payments: read capture %q: %w: the capture names no order", captureID, ErrMalformedRecord)
	}
	return Capture{
		CaptureID: record.ID,
		OrderID:   orderID,
		CustomID:  record.CustomID,
		Outcome: domain.Outcome{
			Kind:        domain.OutcomeCapture,
			Status:      record.Status,
			AmountMinor: amount,
			Currency:    record.Amount.CurrencyCode,
		},
	}, nil
}

// GetRefund returns one refund read. The refund carries no reason, twice
// confirmed live, so HasReason is always false; the reason comes from the
// shopper. The order is read through the refund's capture.
func (c *Client) GetRefund(ctx context.Context, refundID string) (fakes.RefundRecord, error) {
	if err := ctx.Err(); err != nil {
		return fakes.RefundRecord{}, fmt.Errorf("payments: read refund %q: %w", refundID, err)
	}
	refundID = strings.TrimSpace(refundID)
	if refundID == "" {
		return fakes.RefundRecord{}, fmt.Errorf("payments: read refund: %w: empty refund id", ErrMalformedRecord)
	}
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	var refund struct {
		// ID is the refund id.
		ID string `json:"id"`
		// Status is the refund's status, such as COMPLETED.
		Status string `json:"status"`
		// Amount is the refunded amount.
		Amount amount `json:"amount"`
		// CustomID is the shopper-scoped reference.
		CustomID string `json:"custom_id"`
		// Links join the refund to its capture.
		Links []link `json:"links"`
	}
	if err := c.readJSON(ctx, RefundsPath+url.PathEscape(refundID), &refund); err != nil {
		return fakes.RefundRecord{}, fmt.Errorf("payments: read refund %q: %w", refundID, err)
	}
	if refund.ID == "" {
		return fakes.RefundRecord{}, fmt.Errorf("payments: read refund %q: %w: no refund id", refundID, ErrMalformedRecord)
	}
	captureID := linkID(refund.Links, "up")
	if captureID == "" {
		return fakes.RefundRecord{}, fmt.Errorf("payments: read refund %q: %w: the refund names no capture", refundID, ErrMalformedRecord)
	}
	orderID, err := c.captureOrder(ctx, captureID)
	if err != nil {
		return fakes.RefundRecord{}, fmt.Errorf("payments: read refund %q: %w", refundID, err)
	}
	amount, err := refund.Amount.minor()
	if err != nil {
		return fakes.RefundRecord{}, fmt.Errorf("payments: read refund %q: %w", refundID, err)
	}
	return fakes.RefundRecord{
		RefundID: refund.ID,
		Outcome: domain.Outcome{
			Kind:        domain.OutcomeRefund,
			Status:      refund.Status,
			AmountMinor: amount,
			Currency:    refund.Amount.CurrencyCode,
		},
		Link: domain.TransactionLink{
			OrderID:   orderID,
			CaptureID: captureID,
			RefundID:  refund.ID,
			CustomID:  refund.CustomID,
		},
		HasReason: false,
	}, nil
}

// GetDispute returns one dispute read with its reason code and mapped
// category, and the item the dispute names. The order is read through the
// disputed capture when the dispute names one.
func (c *Client) GetDispute(ctx context.Context, disputeID string) (fakes.DisputeRecord, error) {
	if err := ctx.Err(); err != nil {
		return fakes.DisputeRecord{}, fmt.Errorf("payments: read dispute %q: %w", disputeID, err)
	}
	disputeID = strings.TrimSpace(disputeID)
	if disputeID == "" {
		return fakes.DisputeRecord{}, fmt.Errorf("payments: read dispute: %w: empty dispute id", ErrMalformedRecord)
	}
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	record, err := c.dispute(ctx, disputeID)
	if err != nil {
		return fakes.DisputeRecord{}, fmt.Errorf("payments: read dispute %q: %w", disputeID, err)
	}
	code := domain.DisputeCode(record.Reason)
	category, err := domain.StoredCategoryForDispute(code)
	if err != nil {
		return fakes.DisputeRecord{}, fmt.Errorf("payments: read dispute %q: %w", disputeID, err)
	}
	link, item, err := c.disputeLink(ctx, record)
	if err != nil {
		return fakes.DisputeRecord{}, fmt.Errorf("payments: read dispute %q: %w", disputeID, err)
	}
	return fakes.DisputeRecord{
		DisputeID: record.DisputeID,
		Code:      code,
		Category:  category,
		Item:      item,
		Link:      link,
	}, nil
}

// refundEvent maps a PAYMENT.CAPTURE.REFUNDED delivery to the layer's
// event: the refund's amount and links, with the order resolved through
// the refunded capture.
func (c *Client) refundEvent(ctx context.Context, eventID, eventType string, raw json.RawMessage) (fakes.WebhookEvent, error) {
	var refund struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Amount   amount `json:"amount"`
		CustomID string `json:"custom_id"`
		Links    []link `json:"links"`
	}
	if err := unmarshalResource(raw, &refund); err != nil {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: refund event %q: %w", eventID, err)
	}
	if refund.ID == "" {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: refund event %q: %w: no refund id", eventID, ErrMalformedRecord)
	}
	captureID := linkID(refund.Links, "up")
	if captureID == "" {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: refund event %q: %w: the refund names no capture", eventID, ErrMalformedRecord)
	}
	orderID, err := c.captureOrder(ctx, captureID)
	if err != nil {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: refund event %q: %w", eventID, err)
	}
	amount, err := refund.Amount.minor()
	if err != nil {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: refund event %q: %w", eventID, err)
	}
	return fakes.WebhookEvent{
		EventID:   eventID,
		EventType: eventType,
		Verified:  true,
		Outcome: domain.Outcome{
			Kind:        domain.OutcomeRefund,
			Status:      refund.Status,
			AmountMinor: amount,
			Currency:    refund.Amount.CurrencyCode,
		},
		Link: domain.TransactionLink{
			OrderID:   orderID,
			CaptureID: captureID,
			RefundID:  refund.ID,
			CustomID:  refund.CustomID,
			EventID:   eventID,
		},
	}, nil
}

// captureEvent maps a PAYMENT.CAPTURE.COMPLETED delivery to the layer's
// event: the capture's amount, its order and its custom id.
func (c *Client) captureEvent(ctx context.Context, eventID, eventType string, raw json.RawMessage) (fakes.WebhookEvent, error) {
	var capture captureRecord
	if err := unmarshalResource(raw, &capture); err != nil {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: capture event %q: %w", eventID, err)
	}
	if capture.ID == "" {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: capture event %q: %w: no capture id", eventID, ErrMalformedRecord)
	}
	orderID := capture.orderID()
	if orderID == "" {
		resolved, err := c.captureOrder(ctx, capture.ID)
		if err != nil {
			return fakes.WebhookEvent{}, fmt.Errorf("payments: capture event %q: %w", eventID, err)
		}
		orderID = resolved
	}
	amount, err := capture.Amount.minor()
	if err != nil {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: capture event %q: %w", eventID, err)
	}
	return fakes.WebhookEvent{
		EventID:   eventID,
		EventType: eventType,
		Verified:  true,
		Outcome: domain.Outcome{
			Kind:        domain.OutcomeCapture,
			Status:      capture.Status,
			AmountMinor: amount,
			Currency:    capture.Amount.CurrencyCode,
		},
		Link: domain.TransactionLink{
			OrderID:   orderID,
			CaptureID: capture.ID,
			CustomID:  capture.CustomID,
			EventID:   eventID,
		},
	}, nil
}

// disputeEvent maps a CUSTOMER.DISPUTE.CREATED delivery to the layer's
// event: the source's reason code, the disputed amount and the transaction
// links.
func (c *Client) disputeEvent(ctx context.Context, eventID, eventType string, raw json.RawMessage) (fakes.WebhookEvent, error) {
	var record disputeRecord
	if err := unmarshalResource(raw, &record); err != nil {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: dispute event %q: %w", eventID, err)
	}
	if record.DisputeID == "" {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: dispute event %q: %w: no dispute id", eventID, ErrMalformedRecord)
	}
	link, err := c.disputeTransaction(ctx, record)
	if err != nil {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: dispute event %q: %w", eventID, err)
	}
	link.DisputeID = record.DisputeID
	link.EventID = eventID
	amount, err := record.DisputeAmount.minor()
	if err != nil {
		return fakes.WebhookEvent{}, fmt.Errorf("payments: dispute event %q: %w", eventID, err)
	}
	return fakes.WebhookEvent{
		EventID:   eventID,
		EventType: eventType,
		Verified:  true,
		Outcome: domain.Outcome{
			Kind:        domain.OutcomeDispute,
			Status:      record.Status,
			AmountMinor: amount,
			Currency:    record.DisputeAmount.CurrencyCode,
		},
		Link:        link,
		DisputeCode: domain.DisputeCode(record.Reason),
	}, nil
}

// capture reads one capture record.
func (c *Client) capture(ctx context.Context, captureID string) (captureRecord, error) {
	var record captureRecord
	if err := c.readJSON(ctx, CapturesPath+url.PathEscape(captureID), &record); err != nil {
		return captureRecord{}, err
	}
	if record.ID == "" {
		return captureRecord{}, fmt.Errorf("%w: capture %q names no id", ErrMalformedRecord, captureID)
	}
	return record, nil
}

// dispute reads one dispute record.
func (c *Client) dispute(ctx context.Context, disputeID string) (disputeRecord, error) {
	var record disputeRecord
	if err := c.readJSON(ctx, DisputesPath+url.PathEscape(disputeID), &record); err != nil {
		return disputeRecord{}, err
	}
	if record.DisputeID == "" {
		return disputeRecord{}, fmt.Errorf("%w: dispute %q names no id", ErrMalformedRecord, disputeID)
	}
	return record, nil
}

// captureOrder follows one capture to its order id.
func (c *Client) captureOrder(ctx context.Context, captureID string) (string, error) {
	if captureID == "" {
		return "", fmt.Errorf("%w: no capture to follow", ErrMalformedRecord)
	}
	record, err := c.capture(ctx, captureID)
	if err != nil {
		return "", err
	}
	if orderID := record.orderID(); orderID != "" {
		return orderID, nil
	}
	return "", fmt.Errorf("%w: capture %q names no order", ErrMalformedRecord, captureID)
}

// disputeTransaction returns the dispute's transaction link and its item
// from the first disputed transaction. The order is resolved through the
// disputed capture when the transaction names one.
func (c *Client) disputeTransaction(ctx context.Context, record disputeRecord) (domain.TransactionLink, error) {
	var link domain.TransactionLink
	if len(record.DisputedTransactions) == 0 {
		return link, fmt.Errorf("%w: dispute %q names no transaction", ErrMalformedRecord, record.DisputeID)
	}
	transaction := record.DisputedTransactions[0]
	link.CaptureID = transaction.SellerTransactionID
	link.CustomID = transaction.Custom
	if link.CaptureID != "" {
		orderID, err := c.captureOrder(ctx, link.CaptureID)
		if err != nil {
			return link, err
		}
		link.OrderID = orderID
	}
	return link, nil
}

// disputeLink returns the dispute's link and item for the record read. The
// item comes from the disputed transaction; the order's richer line items
// stay the order read's business.
func (c *Client) disputeLink(ctx context.Context, record disputeRecord) (domain.TransactionLink, domain.Item, error) {
	link, err := c.disputeTransaction(ctx, record)
	if err != nil {
		return link, domain.Item{}, err
	}
	link.DisputeID = record.DisputeID
	var item domain.Item
	if len(record.DisputedTransactions) > 0 {
		items := record.DisputedTransactions[0].Items
		if len(items) > 0 {
			line := LineItem{Name: items[0].ItemName, SKU: items[0].ItemID}
			line.Size = sizeOf(line)
			item = c.itemFor(line)
		}
	}
	return link, item, nil
}

// itemFor maps one line through the deployment's catalog, falling back to
// the line's own fields when the catalog does not know it. The fallback
// derives a stable brand key from the line's name or SKU so a purchase the
// catalog has never seen still becomes evidence under its own line.
func (c *Client) itemFor(line LineItem) domain.Item {
	if c.catalog != nil {
		if item, ok := c.catalog.Item(line); ok {
			return item
		}
	}
	return derivedItem(line)
}

// readJSON performs one authenticated read and decodes its record. It maps
// the source's answers to the package's sentinels: 404 is ErrNotFound, a
// credential refusal is ErrUnauthorized, any other failure is
// ErrUnavailable, and an unreadable body is ErrMalformedRecord.
func (c *Client) readJSON(ctx context.Context, path string, out any) error {
	if c.clientID == "" || c.secret == "" {
		return fmt.Errorf("%w: PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET are not set", ErrUnavailable)
	}
	status, raw, err := c.read(ctx, path)
	if err != nil {
		return err
	}
	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	default:
		return fmt.Errorf("%w: the source answered %d", ErrUnavailable, status)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformedRecord, err)
	}
	return nil
}

// read performs one authenticated GET, renewing a cached token the source
// rejects once. A second rejection is the source's answer.
func (c *Client) read(ctx context.Context, path string) (int, []byte, error) {
	token, err := c.bearer(ctx)
	if err != nil {
		return 0, nil, err
	}
	status, raw, err := c.get(ctx, path, token)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if status != http.StatusUnauthorized {
		return status, raw, nil
	}
	c.forgetToken()
	token, err = c.bearer(ctx)
	if err != nil {
		return 0, nil, err
	}
	status, raw, err = c.get(ctx, path, token)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return status, raw, nil
}

// amount is one source amount as its records carry it.
type amount struct {
	// CurrencyCode is the ISO currency code.
	CurrencyCode string `json:"currency_code"`
	// Value is the decimal amount, such as 34.00.
	Value string `json:"value"`
}

// minor converts the decimal amount to minor units without floating point.
// A sub-cent value, a negative value or an empty value is refused.
func (a amount) minor() (int64, error) {
	value := strings.TrimSpace(a.Value)
	if value == "" {
		return 0, fmt.Errorf("%w: amount carries no value", ErrMalformedRecord)
	}
	if strings.HasPrefix(value, "-") {
		return 0, fmt.Errorf("%w: amount %q is negative", ErrMalformedRecord, value)
	}
	whole, fraction := value, ""
	if i := strings.Index(value, "."); i >= 0 {
		whole, fraction = value[:i], value[i+1:]
	}
	if len(fraction) > 2 {
		return 0, fmt.Errorf("%w: amount %q has sub-cent precision", ErrMalformedRecord, value)
	}
	if whole == "" {
		whole = "0"
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	minor, err := strconv.ParseInt(whole+fraction, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: amount %q is not a decimal amount", ErrMalformedRecord, value)
	}
	return minor, nil
}

// link is one source link.
type link struct {
	// Href is the link's target.
	Href string `json:"href"`
	// Rel names the link's relation, such as up.
	Rel string `json:"rel"`
}

// linkID returns the trailing id of the first link with one relation.
func linkID(links []link, rel string) string {
	for _, entry := range links {
		if entry.Rel == rel {
			return idFromHref(entry.Href)
		}
	}
	return ""
}

// idFromHref returns the last path segment of one link target.
func idFromHref(href string) string {
	trimmed := strings.TrimRight(href, "/")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		return trimmed[i+1:]
	}
	return trimmed
}

// captureRecord is one capture as the source states it.
type captureRecord struct {
	// ID is the capture id.
	ID string `json:"id"`
	// Status is the capture's status.
	Status string `json:"status"`
	// Amount is the captured amount.
	Amount amount `json:"amount"`
	// CustomID is the shopper-scoped reference.
	CustomID string `json:"custom_id"`
	// SupplementaryData carries the related order id.
	SupplementaryData struct {
		// RelatedIDs joins the capture to its order.
		RelatedIDs struct {
			// OrderID is the checkout order id.
			OrderID string `json:"order_id"`
		} `json:"related_ids"`
	} `json:"supplementary_data"`
	// Links join the capture to its order.
	Links []link `json:"links"`
}

// orderID returns the capture's order id from its supplementary data or
// its up link.
func (c captureRecord) orderID() string {
	if c.SupplementaryData.RelatedIDs.OrderID != "" {
		return c.SupplementaryData.RelatedIDs.OrderID
	}
	return linkID(c.Links, "up")
}

// disputeRecord is one dispute as the source states it.
type disputeRecord struct {
	// DisputeID is the dispute id.
	DisputeID string `json:"dispute_id"`
	// Reason is the source's reason code.
	Reason string `json:"reason"`
	// Status is the dispute's status.
	Status string `json:"status"`
	// DisputeAmount is the disputed amount.
	DisputeAmount amount `json:"dispute_amount"`
	// DisputedTransactions are the transactions the dispute concerns.
	DisputedTransactions []struct {
		// Custom is the shopper-scoped reference.
		Custom string `json:"custom"`
		// SellerTransactionID is the disputed capture.
		SellerTransactionID string `json:"seller_transaction_id"`
		// Items are the disputed line items.
		Items []struct {
			// ItemID is the SKU.
			ItemID string `json:"item_id"`
			// ItemName is the line's human name.
			ItemName string `json:"item_name"`
		} `json:"items"`
	} `json:"disputed_transactions"`
	// ID is the event resource's id on some dispute deliveries.
	ID string `json:"id"`
}

// unmarshalResource decodes one event resource, refusing one that is not
// an object.
func unmarshalResource(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		return fmt.Errorf("%w: the event carries no resource", ErrMalformedRecord)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformedRecord, err)
	}
	return nil
}

// sizePattern finds the size a line states in words, such as "size M".
var sizePattern = regexp.MustCompile(`(?i)\bsize\s+([A-Za-z0-9]+)`)

// sizeOf returns the size a line states, or empty when it states none. The
// description is read before the name, because a line's description carries
// the variant note ("Size M, runs small") and its name carries the product.
func sizeOf(line LineItem) string {
	for _, text := range []string{line.Description, line.Name} {
		if match := sizePattern.FindStringSubmatch(text); len(match) == 2 {
			return strings.ToUpper(match[1])
		}
		if strings.HasSuffix(strings.ToUpper(strings.TrimSpace(text)), " OS") {
			return "OS"
		}
	}
	return ""
}

// brandName returns the leading brand words of a line name. The demo's
// names start with a brand ("Brand X ...", "Loom ..."); the first two words
// are the brand when the first is "Brand", and the first word otherwise.
func brandName(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	if strings.EqualFold(fields[0], "brand") && len(fields) > 1 {
		return fields[0] + " " + fields[1]
	}
	return fields[0]
}

// categorySlug normalises a source category to a layer category slug.
func categorySlug(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	var out strings.Builder
	lastDash := false
	for _, r := range category {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
			lastDash = false
		case !lastDash:
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

// slug lowercases a display name into a stable key.
func slug(name string) string {
	return categorySlug(name)
}

// derivedItem maps a line the catalog does not know from the line's own
// fields. The brand key is derived from the name, or the SKU when the name
// states no brand, so a purchase never pools under an empty line.
func derivedItem(line LineItem) domain.Item {
	name := strings.TrimSpace(line.Name)
	brand := brandName(name)
	brandID := slug(brand)
	if brandID == "" {
		brandID = slug(line.SKU)
	}
	if brandID == "" {
		brandID = "unknown"
	}
	category := categorySlug(line.Category)
	if category == "" {
		category = "goods"
	}
	return domain.Item{
		BrandID:   brandID,
		BrandName: brand,
		LineName:  name,
		SKU:       line.SKU,
		Size:      line.Size,
		Category:  category,
	}
}

// DemoCatalog is the sandbox demo's catalog join: the SKUs the reference
// agent's orders carry, joined to the layer's brand ids, line names and
// category slugs. It is deployment data for the sandbox demo, not a PayPal
// API fact; a line it does not know falls back to the order's own fields.
func DemoCatalog() Catalog {
	return CatalogFunc(func(line LineItem) (domain.Item, bool) {
		sku := strings.ToUpper(strings.TrimSpace(line.SKU))
		size := line.Size
		if size == "" {
			size = sizeFromSKU(line.SKU)
		}
		switch {
		case strings.HasPrefix(sku, "BRX-OXF"), sku == "PSW8GEK":
			return domain.Item{
				BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt",
				SKU: line.SKU, Size: size, Category: "shirts",
			}, true
		case strings.HasPrefix(sku, "BRY-TEE"):
			return domain.Item{
				BrandID: "brand-y-002", BrandName: "Brand Y", LineName: "everyday tee",
				SKU: line.SKU, Size: size, Category: "shirts",
			}, true
		case strings.HasPrefix(sku, "BRZ-CAP"):
			if size == "" {
				size = "OS"
			}
			return domain.Item{
				BrandID: "brand-z-003", BrandName: "Brand Z", LineName: "canvas cap",
				SKU: line.SKU, Size: size, Category: "hats",
			}, true
		default:
			return domain.Item{}, false
		}
	})
}

// sizeFromSKU returns a size the SKU ends with, such as BRX-OXF-M, or empty
// when its last segment is a colour or has no size spelling.
func sizeFromSKU(sku string) string {
	trimmed := strings.TrimSpace(sku)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(strings.ToUpper(trimmed), "-")
	switch last := parts[len(parts)-1]; last {
	case "S", "M", "L", "XL", "XXL", "OS":
		return last
	}
	return ""
}
