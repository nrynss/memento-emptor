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

Currency finding (follow-up): the first USD order's approval was refused with
"This seller doesn't accept payments in your currency", so the merchant does
not receive USD as configured. No API available to this app reveals the
merchant's home currency: `GET /v1/identity/oauth2/userinfo` with the merchant
token returns only the app identity (`sub`); `GET /v1/reporting/balances`
returns `NOT_AUTHORIZED` like the transaction listing. The already-recorded
catalog event carries an Indian business phone (country code redacted with the
rest), which suggests INR, but recreating the identical shape in INR fails at
creation with `CURRENCY_NOT_SUPPORTED` — so INR is not the answer either, and
order creation cannot confirm receivability anyway (USD created fine and was
refused only at approval). Open: read the home currency in the developer
dashboard (Sandbox, Accounts, the business account's profile) and either add
a USD balance / allow receiving USD, or recreate the app under a USD-home
sandbox business account. The demo's amounts stay USD either way.

## 2. Refund, and what it carries

No live refund exists: a refund needs a capture, which is blocked on the
approval above.

Surface probe: `POST /v2/payments/captures/{nonexistent}/refund` with
`{amount, note_to_payer}`.
Result: 404 `INVALID_RESOURCE_ID` on `capture_id`. The endpoint is reachable
and authenticated; the body shape was accepted and only the address was wrong.
Recording: `testdata/paypal/refund-nonexistent-capture.json`.

On the brief's claim that a refund has no reason field: partly verified. The
refund request took `amount` plus `note_to_payer` with no reason demanded, and
the toolkit's `create_refund` (v1.11.0 source) posts the same shape to the
same endpoint. What the refund *record* and its webhook carry (whether a GET
shows any reason-like field) is unverified and needs the capture.

What this means for the layer, as far as the sandbox shows: a refund arrives
as an outcome without a reason, so the reason has to come from the shopper
(`pending_questions` / `record_reason`). Nothing seen contradicts that.

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

So the sandbox produces disputes through this API (seller-side create is the
sandbox testing path; in live, buyers file through the resolution center), but
a dispute with a reason code is one capture away, not zero. Nothing was
fabricated as a stand-in; dispute evidence is pending the same one-click
approval. For the demo's use (a never-arrived item mapping to seller
reliability, never to sizing), the reason-code half is verified at the
validation layer only.

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
resend/retry rather than polling. The transport is identical for
capture/refund/dispute events; only their payloads are still unverified.

## 5. Line items

- `GET /v2/checkout/orders/{id}` returns `purchase_units[].items` with name,
  SKU, quantity, description and unit amount.
  Recording: `testdata/paypal/order-get-line-items.json`.
- `GET /v2/invoicing/invoices/{id}` returns `items[]` with name and quantity.
  Recording: `testdata/paypal/invoice-get-line-items.json`.
- Line items from a capture or a transaction-detail read are pending the
  approval. The reporting API is not a fallback: listing transactions returns
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

This matches product.md's "What PayPal's tools cover" against v1.11.0.

## Placeholders

Recordings replace secrets, account detail and transient ids with fixed
placeholders that keep the shape:

| Placeholder | What was removed |
|---|---|
| `ORDER-ID-SANDBOX-1` | wallet order id |
| `WEBHOOK-ID-SANDBOX-1` | webhook id |
| `WEBHOOK-EVENT-ID-1`, `-2` | webhook event ids |
| `INVOICE-ID-SANDBOX-1` | invoice id and URL fragment |
| `PRODUCT-ID-SANDBOX-1` | catalog product id |
| `ITEM-ID-REDACTED`, `TEMPLATE-ID-REDACTED` | invoice item / template ids |
| `MERCHANT-EMAIL-REDACTED`, `MERCHANT-ID-REDACTED`, `PHONE-REDACTED` | sandbox merchant contact and account id from PayPal responses |
| `PAYER-EMAIL-REDACTED` | invoice recipient address (a placeholder-shaped address) |
| `APPROVAL-URL-REDACTED` | payer-action approval URL (contains the order token) |
| `WEBHOOK-SITE-UUID-REDACTED`, `DELIVERY-UUID-REDACTED`, `TRANSMISSION-ID-REDACTED`, `TRANSMISSION-SIG-REDACTED` | ephemeral capture URL, delivery and signature values |
| `DEBUG-ID-REDACTED` | PayPal debug ids |
| `CAPTURE-ID-NONEXISTENT-PLACEHOLDER` | the fake capture id in the refund probe |

Keys never entered any recording: the client id/secret travelled only in
Authorization headers, and a filename scan for both values returns nothing.

## Open: one human click finishes the rest

After the sandbox shopper approves the wallet order (order id and approval
URL are in the task report, not committed here), run in order: capture the
order; GET the order and the capture (line items); refund the capture with a
`note_to_payer` and GET the refund (record shape); create the dispute with a
reason code on the capture id (dispute shape). Each call reuses the commands
above with the same headers. If the order has expired by then, recreate it
with the same body and a fresh `PayPal-Request-Id`.
