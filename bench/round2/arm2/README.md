# Arm 2: what the PayPal Agent Toolkit returns

Arm 2 in round 2 is the shopper's raw PayPal history and nothing else. This
directory holds the responses recorded from PayPal's Agent Toolkit for the demo
shopper, and the extraction that turns them into arm 2's prompt input. It holds
no reason, fit category, signal strength, belief or confidence that the layer
produced: those belong to the layer.

## What was run

The toolkit's MCP server was run, not just its source read. It needs a PayPal
access token and no model key: the model is the MCP client, which here is a
small JSON-RPC driver. Versions from the session:

| Component | Version |
|---|---|
| `@paypal/mcp` (the server entry point) | 1.8.1 |
| `@paypal/agent-toolkit` (its dependency and the tool implementation) | 1.11.0 |
| MCP protocol | `2024-11-05` (server info `PayPal` 1.11.0, `probe-initialize.json`) |

The sandbox app's token carried 37 scopes, including
`https://uri.paypal.com/services/reporting/search/read`,
`https://uri.paypal.com/services/disputes/read-seller` and
`https://uri.paypal.com/services/payments/orders/client_sdk_orders_api`.
Transaction Search is what lets `list_transactions` answer at all.

Each tool call listed below was made with `tools/call` over stdio. The MCP
server stringifies the function result into `content[0].text`; the tool itself
returns the PayPal REST response body unchanged. So each committed recording is
the REST body the tool wrapped, in the JSON text the model would receive.
`probe-mcp-result.json` and `probe-mcp-get-order.json` are untouched JSON-RPC
results: each envelope (`result.content[0].type = "text"`) and the text it
carries.

`probe-tools-list.json` is the complete `tools/list` from the session that made
the `get_order` call: 28 tools, `get_order` among them. `probe-tools-used.json`
extracts the five this directory uses: `list_transactions`, `get_order`,
`get_refund`, `list_disputes` and `get_dispute`. `get_order` was called in that
same session for `ORDER-ID-US-1`; the envelope's text is the order body, and it
matches `get-order-1.json` field for field. The other two order bodies came from
the same tool and endpoint, and are kept as recorded.

| Tool | REST call it wraps | Recordings |
|---|---|---|
| `list_transactions` | `GET /v1/reporting/transactions` | `list-transactions-succeeded.json` (`transaction_status=S`, the tool default), `list-transactions-pending.json` (`transaction_status=P`) |
| `get_order` | `GET /v2/checkout/orders/{id}` | `get-order-1.json`, `get-order-2.json`, `get-order-3.json`; `probe-mcp-get-order.json` (envelope for `-1`) |
| `get_refund` | `GET /v2/payments/refunds/{id}` | `get-refund-1.json`, `get-refund-2.json`; `probe-mcp-result.json` (envelope for `-1`) |
| `list_disputes` | `GET /v1/customer/disputes` | `probe-list-disputes.json` |
| `get_dispute` | `GET /v1/customer/disputes/{id}` | `get-dispute-1.json` |

The reporting read used a window of 31 days or less, which is the API's maximum
for one request: `2026-10-11T00:00:00-0700` to `2026-11-10T23:59:59-0700`. A
longer range returns `400 INVALID_REQUEST`.

The three order ids came from a REST `GET /v2/payments/captures/{id}` reading
`supplementary_data.related_ids.order_id`, a lookup outside the toolkit used
only to address `get_order`. The call for `ORDER-ID-US-1` is recorded as an
untouched JSON-RPC envelope (`probe-mcp-get-order.json`); the other two bodies
are the same tool's output for the other two ids, kept as recorded. The capture
read is not arm input.

## What the toolkit actually returns

1. **The transaction listing carries `transaction_info` only.** The toolkit's
   `list_transactions` passes no `fields` parameter, so PayPal returns the
   default projection: id, event code, dates, amount, fee, status, subject,
   note, custom field, instrument and protection fields. It returns no
   `cart_info`, no `payer_info` and no `shipping_info`. This is not a redaction
   here: the committed response has the same keys the tool produced. A probe
   call that passed `fields: "all"` returned the identical shape and, byte for
   byte, the same body (`probe-transactions-fields-all.json`), because the tool
   schema has `additionalProperties: false` and no `fields` property, so the
   argument is dropped before the REST call.
2. **Line items come from `get_order`, not the listing.** `get_order` returns
   `purchase_units[].items` with name, SKU, quantity, unit amount, description
   and category. A caller can only ask for an order it already has an id for;
   the listing does not name one. Arm 2 therefore gets line items only for
   orders whose ids the agent already holds.
3. **A pending dispute hold needs the `P` status to reach the listing.** The
   tool defaults `transaction_status` to `S`, so the `T1110` hold placed by the
   open dispute appears in the transaction listing only in a second call with
   `transaction_status=P`. The recorded pending row is that hold. A caller that
   never lists pending transactions can still meet it in one other place:
   `get_order` for the held capture's order carries the same hold as a pending
   refund under `purchase_units[].payments.refunds`
   (`HOLD-TRANSACTION-ID-1`, amount `32.32`). The extraction drops `payments`
   from orders, so arm 2's input carries the hold only as the `P` transaction
   row.
4. **Refund status is a row and a record.** A refund is a `T1107` row in the
   transaction listing, and the same id answers `get_refund`, which adds the
   refund status (`COMPLETED`, where the row carries the reporting `S`), the
   `seller_payable_breakdown` with its platform fees, and the explicit
   `custom_id`. Neither carries a reason for the refund. The seller's free text
   is in both: `get_refund`'s `note_to_payer` repeats the listing row's
   `transaction_note` and `transaction_subject`.
5. **Dispute reason codes are on `get_dispute`.** `list_disputes` returns the
   dispute summary (id, reason, status, amount, disputed transaction); it now
   returns this merchant's disputes. `get_dispute` adds the per-item reason,
   the dispute state and life-cycle stage, the held transaction and the fund
   movement. `reason` is a PayPal reason code (here
   `MERCHANDISE_OR_SERVICE_NOT_RECEIVED`), not a fit classification.
6. **The listing is merchant-wide.** `list_transactions` returns every
   transaction on the merchant account in the window. The tool has no shopper
   or custom-id filter; the only per-shopper key in its output is
   `transaction_info.custom_field` (the seller's `custom_id`). Differentiating
   shoppers therefore depends on the integration setting `custom_id`. Rows for
   other shoppers were removed from the committed copies, and so was the one
   other dispute in `probe-list-disputes.json`, because they carry other
   people's data; `total_items` (18 here, against 5 kept rows) and every other
   field are left as returned. The extraction emits only
   `transaction_details[]` entries, so the counts do not reach arm 2.

## The extraction

`Extract` takes `Recordings` (the raw tool response bodies) and a `Selector`
(`CustomID` matching `custom_field`/`custom_id`, or `PayerEmail` as a fallback)
and returns arm 2's prompt input as a JSON array. The harness embeds that array
verbatim; the package adds no header text.

Each output record comes from one toolkit response:

- **Transaction:** the whole `transaction_details[]` entry, unchanged, free
  text included (`transaction_subject`, `transaction_note`).
- **Order:** `id`, `status`, `intent`, `create_time`, and each purchase unit's
  `reference_id`, `custom_id`, `description`, `amount` and `items`. The
  purchase unit's `payments` — capture status, refunds and the dispute hold —
  is dropped.
- **Refund:** `id`, `amount`, `status`, `custom_id`, `create_time`,
  `update_time`, `note_to_payer`, `seller_payable_breakdown`.
- **Dispute:** `dispute_id`, `reason`, `status`, `dispute_state`,
  `dispute_life_cycle_stage`, `dispute_channel`, `dispute_amount`, `create_time`,
  `update_time`, `seller_response_due_date`, and each disputed transaction's
  `buyer_transaction_id`, `seller_transaction_id`, `create_time`,
  `transaction_status`, `gross_amount`, `custom`, `items`,
  `seller_protection_eligible`.

Everything else on an order, refund or dispute is dropped: buyer and seller
identities, messages, evidence bodies and API links are not needed to state
what happened. Trimming removes fields; it never names a new one. Records are
grouped by kind — transactions, orders, refunds, disputes — and each group is
ordered by its date and then its id, so the same responses always encode to the
same bytes. The array is not one chronological list. The test
`TestExtractCarriesWhatTheToolkitReturned` finds the one recording each output
record came from and fails on any output field path or value that recording did
not return, empty objects and arrays included. A reason category, signal or
confidence would have no source path to match. `TestExtractionInventsNoReason`
checks the layer's vocabulary by name as well, and `TestExtractDemoShopper` pins
what the demo shopper gets: 6 transactions (5 successful, 1 pending hold), 3
orders with line items, 2 refunds and 1 dispute with its reason code.

`Load` reads four file patterns and ignores the probes:
`list-transactions-*.json`, `get-order-*.json`, `get-refund-*.json`,
`get-dispute-*.json`. Round 2's generated shoppers can write the same shapes.

## Redaction

Identifiers and personal data were replaced with fixed placeholders, as in
`dev-diary/research/paypal.md`. The table lists the ones these files use;
`HOLD-TRANSACTION-ID-1` and `REFERENCE-ID-US-1/2` are new here because T0.4 did
not record those rows. Two values are left as returned, as T0.4 left them: the
merchant's generic sandbox business name (`Test Store`, also inside
`soft_descriptor`) and the shopper's `country_code` (`IN` on the first cycle,
`US` on the other two). Neither identifies a person.

| Placeholder | What was removed |
|---|---|
| `ORDER-ID-US-1`, `-2`, `-3` | the three order ids |
| `CAPTURE-ID-US-1`, `-2`, `-3` | the three capture transaction ids |
| `REFUND-ID-US-1`, `-2` | the two refund ids (also their `T1107` row ids) |
| `HOLD-TRANSACTION-ID-1` | the pending `T1110` hold row id (also its refund record on the order) |
| `REFERENCE-ID-US-1`, `-2` | the reporting `paypal_reference_id` on two capture rows |
| `DISPUTE-ID-SANDBOX-1` | the dispute id |
| `BUYER-TRANSACTION-ID-SANDBOX-1` | the buyer-side transaction id on the dispute, and the third capture row's reporting reference |
| `PAYER-ID-REDACTED` | payer account ids (including the reporting `paypal_account_id`) and payer ids on orders and disputes |
| `MERCHANT-ID-REDACTED` | merchant ids and the reporting account number |
| `BUYER-EMAIL-REDACTED` | shopper email addresses |
| `MERCHANT-EMAIL-REDACTED` | merchant email address |
| `BUYER-NAME-REDACTED` | shopper name in every shape it appeared |
| `SHOPPER-ADDRESS-REDACTED`, `SHOPPER-CITY-REDACTED`, `SHOPPER-STATE-REDACTED`, `SHOPPER-POSTAL-REDACTED` | shopper street, city, state and postal code |

No phone number appears in these recordings, so `PHONE-REDACTED` from the
research table is not used here.

No key, token or site login was written to any file. A scan for the removed
values over this directory and the recordings returns nothing.

## Handoff to T10.3

Round 2's shoppers must be built so this extraction sees them and so the arm
stays fair:

- Give every order, and the capture, refund and dispute behind it, the
  shopper's `custom_id`. It is the only per-shopper key the listing returns.
  `PayerEmail` is a fallback the default listing can never satisfy: it returns
  no `payer_info`.
- Supply `get_order` responses for the line items. The listing names no order
  id, so without them arm 2 gets amounts and no items.
- Keep the seller's free text fit-neutral. A refund carries no reason, but its
  note does: `note_to_payer` on `get_refund` is the same text the `T1107` row
  shows as `transaction_note` and `transaction_subject`, and the extraction
  passes the listing row whole. A note like the demo's "fit return" hands arm 2
  the reason and breaks case 3's premise.
- The listing window is 31 days and `page_size` defaults to 100, so a longer
  history is several calls. Disputes are listed as summaries and read by id.
- Arm 2's input carries ids and amounts, which is what round 1's F3 finding
  asked the freeze to settle. The extraction's ordering and field lists above
  are the input contract to freeze.

## Reproduce

Offline, from the repository root:

```sh
go test ./bench/round2/arm2/
```

The test reads the committed recordings; no key, network or model is involved.
To record again, source `tools/env.sh`, obtain a sandbox access token, and call
the MCP server without a model:

```sh
npx -y @paypal/mcp --tools=all --paypal-environment=sandbox
```

The token travels in `PAYPAL_ACCESS_TOKEN`; it is never printed or written.
Then call `list_transactions` for the window, `get_order` for known order ids,
`get_refund` for each refund id and `get_dispute` for each dispute id. The
probe files show the full tool list, the five-tool subset, the `get_order` and
refund envelopes, and the `fields` attempt. Sandbox calls are free, and this
task made no model call.
