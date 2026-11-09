# T1.3b: the demo shopper in the sandbox (live driver)

Live sibling of T1.3. It creates the demo shopper's history as real
sandbox orders, captures, refunds and one dispute under the current
keyring app (US-home business account; credentials load via
`source tools/env.sh`). Nothing here runs in CI and nothing here gates
a build: there are no `*_test.go` files under `tests/live/`, so the
gate's `go test -race` has no tests to run and `go vet` sees only
`approve.py`/`dispute.py` (not Go) plus this README and `run.sh`.

## What it creates

Six wallet orders, one per demo-shopper evidence row, joined by
`custom_id` (`t13b-demo-001` … `t13b-demo-006`), each approved by the
scripted shopper login (`approve.py`) before capture:

| Cycle | custom_id | Seeded day | Item | Amount | Then |
|---|---|---|---|---|---|
| kept-1 | t13b-demo-001 | day 1 | Brand Y Everyday Tee, M | USD 25.00 | capture, no refund |
| kept-2 | t13b-demo-002 | day 2 | Brand Z Canvas Cap, OS | USD 30.00 | capture, no refund |
| fit-1 | t13b-demo-003 | day 3 | Brand X oxford, M | USD 195.00 | capture, full refund, note names the reason in words |
| fit-2 | t13b-demo-004 | day 6 | Brand X oxford, M | USD 195.00 | capture, full refund, note names the reason in words |
| fit-3 | t13b-demo-005 | day 9 | Brand X oxford, M | USD 195.00 | capture, full refund, note names the reason in words |
| dispute-1 | t13b-demo-006 | day 11 | Brand X oxford, M | USD 195.00 | capture, then `dispute.py` files MERCHANDISE_OR_SERVICE_NOT_RECEIVED in the resolution center |

Line items live on the order GET; captures carry no items, so the
ledger reads order + capture joined on `custom_id` (proven in T0.4).
Refund records carry no reason field (proven twice in T0.4), so each
refund's `note_to_payer` names the fit reason in words and the
classified reason itself is recorded by the layer from the shopper's
answer, exactly as the product's evidence model requires.

## Where seeded time comes from

The sandbox cannot backdate: every capture settles at wall-clock now.
Relative spacing lives on the layer's side. `seed.Row.SeededEvidence`
maps a row's `Day` (days since session start) to an absolute time with
`anchor.Add(day * 24h)`, so the live run documents one anchor per
cycle and the layer applies the fixture's day offsets (1, 2, 3, 6, 9,
11 of an 11-day session) onto it. Seeded rows stay marked seeded: the
live PayPal records are the evidence source, and the layer marks every
row the generator's mark would mark.

The real order-id → seeded-day mapping, with ids in placeholder form,
is recorded in the task report (handoff text), never here: `out/` holds
real ids and is git-ignored.

## How to run

```sh
source tools/env.sh
export SHOPPER_EMAIL="$(secret-tool lookup service paypal-sandbox account memento-emptor kind shopper-email)"
export SHOPPER_PASSWORD="$(secret-tool lookup service paypal-sandbox account memento-emptor kind shopper-password)"
export APPROVAL_URL="<approval url from run.sh>"
bash tests/live/seed/run.sh            # one cycle: create, wait for approval, capture, read back
python3 tests/live/seed/approve.py     # scripted shopper approval for APPROVAL_URL
python3 tests/live/seed/dispute.py <order-or-transaction-id>   # resolution-center filing
```

Values travel only in the browser session and in `Authorization`
headers. `run.sh` saves response bodies to `out/` (git-ignored); scan
`out/` for the loaded values before sharing anything from it. If
approvals throttle (seen once in T0.4 as a transient sandbox risk
throttle), wait and retry one cycle at a time.
