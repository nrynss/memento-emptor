// Package domain holds the shared types the whole product codes against.
//
// Evidence keeps three things separate: the outcome from the payment
// evidence source, the item and variant from the order's line items, and
// the reason, which comes from a dispute reason code or from the
// shopper's classified answer. A payment outcome is not a cause: only
// evidence whose reason matches a belief counts toward it.
package domain
