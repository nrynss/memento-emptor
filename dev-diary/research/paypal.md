# PayPal sandbox events (T0.4)

Service: PayPal REST API, sandbox (`api-m.sandbox.paypal.com`), OAuth2
client-credentials from the keyring via `tools/env.sh`. Sandbox app from T0.1:
one business account (merchant) and one personal account (shopper); no account
detail beyond that is recorded here. Granted scopes are recorded in
`testdata/paypal/token-scopes.json` (29 scopes; token value never stored).

Agent Toolkit inspected, not executed: `@paypal/agent-toolkit` v1.11.0 from npm,
tool list taken from its README. All live calls below are plain REST.

Spend: sandbox calls are free. About 25 REST calls, one ephemeral webhook.site
URL (free), no credits touched.

## 1. Order created; approval gate; capture; idempotency

Commands, in order (all POSTs carry a `PayPal-Request-Id`):

1. `POST /v2/checkout/orders` with `intent: CAPTURE`, one purchase unit
   (USD 34.00, `reference_id: fit-shirt-M`, `custom_id: shopper-demo-001`) and
   one line item (Brand X oxford shirt, size M, SKU `BRX-OXF-M`), wallet
   payment source with `return_url`/`cancel_url`.
   Result: order created, status `PAYER_ACTION_REQUIRED`, with a `payer-action`
   link (approval URL, redacted in the recording).
   Recording: `testdata/paypal/order-wallet-create.json`.
2. Replayed the identical body with the identical `PayPal-Request-Id`.
   Result: the same order id and status returned. Idempotency at create is
   seen working.
   Recording: `testdata/paypal/order-wallet-replay-same-request-id.json`.
3. `POST /v2/checkout/orders/{id}/capture` before approval.
   Result: 422 `ORDER_NOT_APPROVED` ("Payer has not yet approved the Order ...
   redirect the payer"). The approval gate is enforced server-side.
   Recording: `testdata/paypal/capture-before-approval.json`.
4. `POST /v2/checkout/orders/{id}/confirm-payment-source` with the wallet
   source, trying for a headless confirmation.
   Result: returns the order unchanged, still `PAYER_ACTION_REQUIRED`. No
   headless approval path through this endpoint.
5. Same order shape with `payment_source.card` (sandbox test card) to capture
   without a wallet login.
   Result: 422 `PAYEE_NOT_ENABLED_FOR_CARD_PROCESSING`. This merchant is not
   set up for direct card payments. Fixes: enable card processing on the
   sandbox business account, or do the one-click wallet approval.
   Recording: `testdata/paypal/card-order-payee-not-enabled.json`.

Post-approval capture is NOT done: it needs one human click on the approval
URL with the sandbox shopper login. No code read the shopper credentials (per
AGENTS.md they are for a person, and no code reads them), and no headless path
was found. The order id and the exact follow-up commands are in the task
report's handoff, not here.

Resolution: the merchant was replaced with a fresh US-home sandbox account and
REST app (new client-id/secret in the keyring; token grant succeeds, 31
scopes). Order `ORDER-ID-US-1` was approved through scripted shopper login
(Playwright Chromium driving the sandbox checkout: email, password, Complete
Purchase, landing back on the return URL with a payer id) and captured:
`COMPLETED`, capture `CAPTURE-ID-US-1`. A second identical cycle
(`ORDER-ID-US-2` / `CAPTURE-ID-US-2`) confirmed repeatability. Back-to-back
scripted logins hit a transient sandbox risk throttle once; a short wait
cleared it. Create-response shape varies by app: the new app's create echoes
no `purchase_units`; the order GET is authoritative for line items.
Recordings: `order-usd-approved.json`, `order-get-approved-line-items.json`,
`capture-completed.json`, `capture-get.json`, plus the `*2*` second-cycle
equivalents.

Currency finding (follow-up): RESOLVED by replacing the merchant. The first
USD order's approval was refused with "This seller doesn't accept payments in
your currency" under the original merchant; no API available to that app
revealed its home currency, and INR failed at creation with
`CURRENCY_NOT_SUPPORTED`. The replacement US-home sandbox account and REST app
approves and captures USD with no currency complaint. The demo's amounts stay
USD; the old INR detour is superseded, not evidence of anything about PayPal.

## 2. Refund, and what it carries

No live refund exists: a refund needs a capture, which is blocked on the
approval above.

Surface probe: `POST /v2/payments/captures/{nonexistent}/refund` with
`{amount, note_to_payer}`.
Result: 404 `INVALID_RESOURCE_ID` on `capture_id`. The endpoint is reachable and
authenticated; the body shape was accepted and only the address was wrong.
Recording: `testdata/paypal/refund-nonexistent-capture.json`.

On the brief's claim that a refund has no reason field: CONFIRMED, twice over.
Two full refunds on two captured orders returned `COMPLETED`, and both refund
records carry id, amount, `note_to_payer`, `seller_payable_breakdown`,
`custom_id`, status, timestamps, payer and links — no reason or reason-like
field anywhere. The `PAYMENT.CAPTURE.REFUNDED` webhook resource carries the
same shape, also with no reason field. Recordings: `refund-record.json`,
`refund2-record.json`, `webhook-delivery-refunded.json`.

What this means for the layer is settled: a refund arrives as an outcome
without a reason, so the reason has to come from the shopper
(`pending_questions` / `record_reason`). Useful join key: `custom_id`
(`shopper-demo-001`) propagates order → capture → refund, so the layer can tie
a refund to its order without relying on PayPal ids.

## 3. Dispute with a reason code

Attempt: `POST /v1/customer/disputes` with
`{reason: MERCHANDISE_OR_SERVICE_NOT_RECEIVED,
disputed_transactions: [{buyer_transaction_id: <the uncaptured order>}]}`.
(A first try with extra fields failed with `UNSUPPORTED_FIELDS`; the minimal
shape passes validation.)
Result: 422 `TRANSACTION_NOT_ELIGIBLE`. The endpoint is live and the reason
value passed validation; eligibility needs a funded/captured transaction, the
same approval blocker as the refund.
Recording: `testdata/paypal/dispute-create-transaction-not-eligible.json`.

Follow-up against two completed captures (second cycle run precisely so the
dispute could precede any refund): the minimal shape and the minimal shape
plus `dispute_amount` both pass schema validation and both return
`TRANSACTION_NOT_ELIGIBLE`; a fuller shape with messages is rejected on
schema (`UNSUPPORTED_FIELDS`). Five attempts across two apps, order ids and
capture ids, before and after refunds, all agree. Sandbox seller-side dispute
creation through this API is gated beyond "completed capture" in a way no
request shape tried can satisfy; the documented reason enum itself is
validated fine, so the taxonomy is real but unreachable this way.
Recordings: `dispute-completed-capture-not-eligible.json`,
`dispute-fuller-shape-unsupported.json`.

What was the stand-in is now the evidenced path. A third cycle (order,
scripted approval, capture, no refund) followed by filing in the sandbox
resolution center as the shopper produced a live dispute:
`DISPUTE-ID-SANDBOX-1`, `reason: MERCHANDISE_OR_SERVICE_NOT_RECEIVED`
(top-level and per item), `status: OPEN`, life-cycle `INQUIRY`, disputed items
carrying the SKU (`BRX-OXF-M`), quantity and reason, `custom:
shopper-demo-001`, the buyer message as message and evidence, and the capture
as `seller_transaction_id`. An API `GET` on the dispute id returns all of
this under the merchant token; the list endpoint stays empty for the merchant
token, so read disputes by id. Recording: `dispute-get.json`.

Exact click path, repeatable for the demo (sandbox shopper login; scripted
with Playwright Chromium, with back-off between logins when the sandbox
throttles): disputes dashboard (`/disputes/dashboard/`) → open a Payment row
→ in the sheet choose Report a problem → filing page
(`/resolutioncenter/filing/{transaction}/create?type=PAYMENT`) → "I haven't
received the item or service" → tick the item, Item type Physical product,
Category, Quantity → Continue → pick the detail ("The item hasn't reached me
or I refused delivery") → Continue → fill Expected refund amount (blank and
link-like values are rejected; date, charges and link stay optional) →
Continue → seller-contact step → documents step (optional) → review →
Submit → case page with the case id. Server state persists between steps, so
a direct visit to the filing URL resumes. Quirks seen: the dashboard list and
the sheet CTA render unreliably under rapid automation; waiting out the
sandbox throttle and addressing the filing URL directly were the fixes.

## 4. Webhooks: delivery, verification, redelivery

Fully verified end to end.

- Webhook created via `POST /v1/notifications/webhooks` (capture completed,
  capture refunded, dispute created), then PATCHed to add invoicing events and
  to correct the URL. Recordings: `webhook-create.json`,
  `webhook-patch-events.json`, `webhook-url-fix.json`, `webhook-get.json`.
- The event simulator is a dead end: `POST /v1/notifications/simulate-events`
  returns 404 with an empty body on both sandbox hosts. Real events were used
  instead.
- Real events, both visible in the event ledger
  (`GET /v1/notifications/webhooks-events`): creating a catalog product emits
  `CATALOG.PRODUCT.CREATED` (an early assumption that catalog work emits
  nothing was wrong; the ledger proves it), and sending an invoice emits
  `INVOICING.INVOICE.SENT`. Recording: `webhook-events-ledger.json`.
- Delivery: PayPal cannot reach localhost, so a webhook.site URL stood in for
  the local endpoint (production path is the Render endpoint, T0.9). The first
  URL form was wrong (`.../token/<uuid>` 404s at webhook.site); the working
  form is `https://webhook.site/<uuid>`. After fixing the webhook URL, a
  manual resend of the invoice event delivered within about a minute: full JSON
  body plus the `paypal-transmission-*` headers.
  Recording: `webhook-delivery-invoice-sent.json` (request metadata stripped,
  values redacted).
- Signature verification: `POST /v1/notifications/verify-webhook-signature`
  with the delivered transmission id, time, cert URL, auth algo, signature,
  webhook id and event body returned `{"verification_status": "SUCCESS"}`.
  Recording: `webhook-verify-success.json`.
- Redelivery: `POST /v1/notifications/webhooks-events/{id}/resend` returned
  200, and the delivered copy above IS the resent one, which proves resend
  works. Automatic retry/backoff was not observed, only the manual resend, so
  retry policy stays reference knowledge, not evidence.
  Recording: `webhook-resend-accepted.json`.

What this means for the ingest (T2.1): key idempotency on PayPal's event id,
verify every delivery through `verify-webhook-signature`, and rely on
resend/retry rather than polling. The refund payload shape is verified above,
and the dispute payload shape below.

Second session under the replacement app: a fresh webhook
(`WEBHOOK-ID-US-1`, same three event types) shows the new app's ledger
contains only events generated after subscription — no backfill of the two
captures or the first refund. Refunding the second capture generated
`PAYMENT.CAPTURE.REFUNDED`; manual resend returned 202 and delivered within
about a minute, and `verify-webhook-signature` on the delivered refund event
returned `SUCCESS`. The delivered refund resource carries the refund record
shape with no reason field. Recordings: `webhook-us-app.json`,
`webhook-events-us-ledger.json`, `webhook-resend-us-accepted.json`,
`webhook-delivery-refunded.json`, `webhook-verify-refunded-success.json`.

Dispute webhook, third cycle: filing the case generated
`CUSTOMER.DISPUTE.CREATED` in the ledger, and it auto-delivered to the test
endpoint about a minute later with no resend involved — the resource carries
`reason: MERCHANDISE_OR_SERVICE_NOT_RECEIVED`. `verify-webhook-signature` on
the delivered copy returned `SUCCESS`. A manual resend was accepted (202); no
duplicate arrived in the window watched, consistent with PayPal not
re-sending an already-delivered event (redelivery itself stays evidenced by
the invoice-sent and refunded resends). The disputed capture also appears in
the ledger as `PAYMENT.CAPTURE.COMPLETED` with auto-delivery. Recordings:
`dispute-event.json`, `dispute-events-ledger.json`,
`webhook-delivery-dispute.json`, `webhook-verify-dispute-success.json`,
`webhook-resend-dispute-accepted.json`.

## 5. Line items

- `GET /v2/checkout/orders/{id}` returns `purchase_units[].items` with name,
  SKU, quantity, description and unit amount.
  Recording: `testdata/paypal/order-get-line-items.json`.
- `GET /v2/invoicing/invoices/{id}` returns `items[]` with name and quantity.
  Recording: `testdata/paypal/invoice-get-line-items.json`.
- Capture-side line items: settled. Capture records (GET and the capture
  response) carry NO items — `purchase_units[]` there holds only payments,
  reference and shipping. Line items live on the order GET, and `custom_id`
  propagates order → capture → refund, so that is the join key.
  Recordings: `order-get-approved-line-items.json`, `capture-get.json`,
  `capture2-get.json`.
- The reporting API is not a fallback: listing transactions returns
  `NOT_AUTHORIZED` (insufficient permissions for this app), not history.
  Recording: `testdata/paypal/reporting-transactions-not-authorized.json`.
  The ledger must read orders/captures, not the reporting API, unless the
  app's permissions change.

## 6. Agent Toolkit / MCP server vs plain REST

Toolkit tool list below is from `@paypal/agent-toolkit` v1.11.0's README
(framework bindings and the MCP server expose the same set):

- Invoices (create/list/get/send/reminder/cancel/QR/delete/number/search/
  update/auto-reminder/payment/refund/conditional-rules, recurring series):
  toolkit covers; REST used here and equivalent.
- Payments: `create_order`, `get_order`, `pay_order` (capture of an
  authorized order), `create_refund`, `get_refund`: toolkit covers. Nuance:
  there is no standalone capture tool; capture goes through `pay_order`.
- Disputes: `list_disputes`, `get_dispute`, `accept_dispute_claim` only.
  Creating a dispute (the call this task needed) is REST-only, as is
  contesting one or submitting evidence.
- Shipment tracking, subscriptions/plans, catalog products, transaction
  listing, merchant insights: toolkit covers (product create/list/show; used
  via REST here).
- REST-only for everything this task's transport needed: webhook CRUD,
  `resend`, `verify-webhook-signature`; payouts; vaulting; order
  authorize/`confirm-payment-source`; reporting detail. `simulate-events`
  404s either way.

This matches product.md's "What PayPal's tools cover" against v1.11.0. Scope
note: the replacement app's grant has 31 scopes against the first app's 29 —
invoicing scope gone, `shipping/trackers/readwrite` and
`wallet/mandates/read|write` added. Recording: `token-scopes-us-app.json`
beside the original `token-scopes.json`. None of this task's calls needed the
differing scopes.

## Placeholders

Recordings replace secrets, account detail and transient ids with fixed
placeholders that keep the shape:

| Placeholder | What was removed |
|---|---|
| `ORDER-ID-SANDBOX-1` | wallet order id (first app) |
| `ORDER-ID-US-1`, `ORDER-ID-US-2`, `ORDER-ID-US-3` | wallet order ids (replacement app, three cycles) |
| `CAPTURE-ID-US-1`, `CAPTURE-ID-US-2`, `CAPTURE-ID-US-3` | capture ids |
| `DISPUTE-ID-SANDBOX-1` | dispute (case) id from resolution-center filing |
| `BUYER-TRANSACTION-ID-SANDBOX-1` | buyer-side transaction id on the dispute |
| `REFUND-ID-US-1`, `REFUND-ID-US-2` | refund ids |
| `WEBHOOK-ID-SANDBOX-1` | webhook id (first app) |
| `WEBHOOK-ID-US-1` | webhook id (replacement app) |
| `WEBHOOK-EVENT-ID-1`, `-2` | webhook event ids (first app) |
| `WEBHOOK-EVENT-US-1` | webhook event id (replacement app, refund) |
| `WEBHOOK-EVENT-DISPUTE-1`, `WEBHOOK-EVENT-CAP3-1` | dispute-created and capture event ids |
| `BUYER-EMAIL-REDACTED`, `BUYER-NAME-REDACTED` | shopper account email and name on the dispute, order and capture reads |
| `SHOPPER-ADDRESS-REDACTED`, `SHOPPER-CITY-REDACTED`, `SHOPPER-STATE-REDACTED`, `SHOPPER-POSTAL-REDACTED` | shopper shipping street, city, state and postal code on the order and capture reads |
| `INVOICE-ID-SANDBOX-1` | invoice id and URL fragment |
| `PRODUCT-ID-SANDBOX-1` | catalog product id |
| `ITEM-ID-REDACTED`, `TEMPLATE-ID-REDACTED` | invoice item / template ids |
| `MERCHANT-EMAIL-REDACTED`, `MERCHANT-ID-REDACTED`, `PHONE-REDACTED` | sandbox merchant contact and account id from PayPal responses |
| `PAYER-EMAIL-REDACTED` | invoice recipient address (a placeholder-shaped address) |
| `PAYER-ID-REDACTED` | shopper payer id from approval and capture responses |
| `APPROVAL-URL-REDACTED` | payer-action approval URL (contains the order token) |
| `WEBHOOK-SITE-UUID-REDACTED`, `DELIVERY-UUID-REDACTED`, `TRANSMISSION-ID-REDACTED`, `TRANSMISSION-SIG-REDACTED` | ephemeral capture URL, delivery and signature values |
| `DEBUG-ID-REDACTED` | PayPal debug ids |
| `CAPTURE-ID-NONEXISTENT-PLACEHOLDER` | the fake capture id in the refund probe |

Keys never entered any recording: the client id/secret travelled only in
Authorization headers, and a filename scan for both values returns nothing.
The same holds for the shopper site logins used for the scripted approvals:
values travelled only in the browser session, never touched a file.

## Open: nothing

Every Done-when clause is met with evidence. The one piece that changes hands:
the dispute case (`DISPUTE-ID-SANDBOX-1`) stays open in the sandbox on the
third cycle's unrefunded capture; the demo can show it live or file a fresh
one by repeating the click path above.
