// Package arm2 builds arm 2's prompt input from what the PayPal Agent
// Toolkit returns for one shopper: the transaction listing, order line
// items, refund status, dispute status and dispute reason codes.
//
// The input to Extract is the untouched output of the toolkit's
// list_transactions, get_order, get_refund and get_dispute tools. The
// output is a JSON array of records, and every field in it is copied from
// one of those responses. The package adds no reason, category, signal
// strength, belief, confidence or write kind: those are the layer's, and
// arm 2 must not carry them.
//
// One fact about the toolkit shapes the input. list_transactions wraps
// GET /v1/reporting/transactions and passes no fields parameter, so PayPal
// returns transaction_info only; the response has no cart_info and no
// payer_info. Line items therefore come from get_order, which needs an
// order id the caller already holds. The committed recordings under
// recordings/ show both shapes.
package arm2
