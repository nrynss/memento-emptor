# T2.1b: PayPal ingest, live (driver)

Live sibling of T2.1. It reads back one real sandbox refund and one real
sandbox dispute from the existing demo history and runs both through the
running ingest service. Nothing here runs in CI and nothing here gates
a build: there are no `*_test.go` files under `tests/live/`, so the
gate's `go test -race` finds no tests and `go vet` sees only `main.go`.

## What it proves

- Each live delivery's signature verifies through the live
  `verify-webhook-signature` API (`SUCCESS` on the exact bytes PayPal
  posted to the sandbox webhook's test endpoint).
- The refund becomes one evidence row per order line item with a nil
  reason that counts toward nothing, plus a pending question; the
  order join follows the live `refund -> capture -> order` up links.
- The dispute becomes one row per item whose reason is the live case's
  own code mapped through the frozen table, at confidence 1.0, counting
  toward whatever that category counts toward. For the current
  non-arrival case that is `not_arrived` and the seller.
- A code the table does not name is stored the way served ingest stores
  it: the code stays as the reason's text, the category is `none`, and
  the row counts toward nothing. The driver does not abort the read on
  such a code, and it makes no judgement of its own about one: the
  mapping comes from `domain.StoredCategoryForDispute`. No live sandbox
  case carries such a code, so this path rests on the domain's table
  test; no live run has taken it.
- Redelivering either event returns the stored rows with `Duplicate`
  set and writes nothing new.

## What it touches

Reads only: the webhook config, the webhook events ledger, the two
stored deliveries, the verify API, and the refund/capture/order and
dispute/capture/order GETs. It creates no order, capture, refund,
dispute, resend, or webhook change. Sandbox calls are free; spend is
$0 and no new sandbox records exist afterward.

The two events are the newest ledger entries of their types, from the
existing T1.3b history (custom ids `t13b-demo-005` for the refund and
`t13b-demo-006` for the dispute). Which ledger entry is newest can move
as later tasks run; the driver always names its choice in the
transcript.

## How to run

```sh
source tools/env.sh
bash tests/live/ingest/run.sh
```

`main.go` is the driver: a live `PaymentSource` over plain REST plus
the ingest drive and its assertions. `run.sh` runs it, saves the
redacted transcript, and checks no secret value entered any file. The
full transcript with real ids lands in `out/` (git-ignored, mode
0600); stdout carries placeholders only, and the driver refuses to
finish if a real id survives redaction.

On any 401 or throttle: stop and report, do not retry in a loop. A
missing delivery (the test endpoint expires idle tokens) is also a
stop-and-report: the resend path that replaces it is a new decision,
not a quiet fix.

## The unobserved not-as-described code

T1.1b mapped `MERCHANDISE_OR_SERVICE_NOT_AS_DESCRIBED` to
`damaged_or_defective`, but no recorded sandbox dispute carries the
code: the demo's live case is a non-arrival. The mapping's spelling
therefore rests on the API's documented reason enum, and the driver
verifies whatever code the live case carries rather than assuming one.

The sandbox cannot produce a not-as-described case through the API.
`probe_not_as_described.py` asks for one on an existing captured,
unrefunded, undisputed transaction:

```sh
source tools/env.sh
python3 tests/live/ingest/probe_not_as_described.py
```

Three request shapes were tried, all refused. The script sends the
second shape below and prints its response; the other two were tried by
hand and are quoted here, with no recording of their own.

| Shape | Response |
|---|---|
| reason only | 400, `details[].issue` = `MISSING_ISSUE_TYPE`: the item array is required before eligibility is looked at |
| reason plus one `items[]` entry | 400, `details[].issue` = `TRANSACTION_NOT_ELIGIBLE`; recorded in `testdata/paypal/dispute-create-not-as-described-not-eligible.json` |
| the capture as `seller_transaction_id` | 500 `INTERNAL_SERVER_ERROR` with no details |

The recorded body is identical to
`dispute-create-transaction-not-eligible.json`, which an earlier attempt
on an uncaptured order drew, but for a trailing newline. The API answers
the same issue either way, so the file records the refusal, and the
request it answers is the shape above plus the body in the script.

This matches the T0.4 finding for the non-arrival reason: seller-side
dispute creation through `POST /v1/customer/disputes` is gated beyond a
completed capture, so the reason enum is validated as a value but no
case is created. A not-as-described case is filed the way the demo's
non-arrival case was, as the shopper in the sandbox resolution center
(the click path in `tests/live/seed/dispute.py` starts it). That is a
new sandbox record, not a read, so this task did not file one.

## Placeholders

Real ids stay in `out/` and out of the repository. The transcript's
placeholders keep the shape:

| Placeholder | What was removed |
|---|---|
| `WEBHOOK-ID-LIVE-1` | sandbox webhook id |
| `WEBHOOK-SITE-UUID-REDACTED` | test endpoint token in the webhook URL |
| `WH-EVENT-REFUND-LIVE-1` | refund webhook event id |
| `WH-EVENT-DISPUTE-LIVE-1` | dispute webhook event id |
| `ORDER-ID-LIVE-1`, `ORDER-ID-LIVE-2` | checkout order ids (refund, dispute) |
| `CAPTURE-ID-LIVE-1`, `CAPTURE-ID-LIVE-2` | capture ids (refund, dispute) |
| `REFUND-ID-LIVE-1` | refund id |
| `DISPUTE-ID-LIVE-1` | dispute id |
| `TRANSMISSION-ID-LIVE`, `TRANSMISSION-SIG-LIVE` | delivery transmission values |
| `CERT-URL-LIVE` | webhook cert URL |
| `EMAIL-REDACTED` | any sandbox account email in a read |

Keys never enter any file: the client id and secret travel only in
the token grant's Authorization header, and `run.sh` scans the whole
directory for both values after every run.
