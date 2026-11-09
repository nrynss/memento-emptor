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
- The dispute becomes one row per item with the reason the frozen
  table maps `MERCHANDISE_OR_SERVICE_NOT_RECEIVED` to (`not_arrived`,
  confidence 1.0), counting toward the seller.
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
