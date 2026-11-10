# T3.6 run 1 (clean, from reset)

Session `t36-run-1-063817` (erased after the run: 9 interactions,
9 concepts, 23 edges, 2 canonization events). Driver
`demo/driver/` at the landed commit; Lambo 0.10.1 release binary with
`store-sqlite,embed-elastic,recall-elastic`; native
`.jina-embeddings-v3` embeddings; throwaway SQLite store beside the
session, deleted with it. No Elastic indices created (store ranking
serves recall; the hybrid proof is T2.3b's).

## Step 1 — three documented fit returns → Earned (live)

Reused, nothing created: the T1.3b cycles `t13b-demo-003/004/005`
read back live — refund COMPLETED, capture settled (REFUNDED, as the
cycles were fully refunded by design), order line items "Brand X
Button Down Oxford Shirt size M" x1, custom ids join. Free.

Newly created: 4 live Jev classifications on the fixture wording
(3 fit + 1 vague), build `typesafe/jev-1.14-20261014` pinned every
call, costs 0.000021336 / 0.000021630 / 0.000021420 / 0.000021210 USD
(total $0.000085596, ~5x under the 10x-per-call guard). Three
`fit_too_small` accepted as evidence; "not great" → unclear 0.93,
ask-again, pending stays open. One return earns nothing (recall: no
belief, no guidance); after the third, live inspect reads Venerable,
blast radius 3, coverage 1.000, distinct 3. Recall serves the sizing
belief Earned/Venerable with guidance "Take size L for Brand X" and
3 evidence rows.

## Step 2 — Channel3 candidate, size-L recommendation (live)

Two free live detail calls (no searches, 0 credits): `PSW8GeK`
(Button Down Oxford Shirt, brand `SMwk`, category `dress-shirts`,
`top-fit: [Regular]`, InStock USD 195 at `us.sunspel.com`) and the
returned `tbnrl46`. The frozen match keeps the new SKU on brand +
category + equal `top-fit`, skips the returned SKU, and marks
sizes-at-the-retailer plus simulated fulfilment. The L recommendation
is served by the live layer recall of step 1 (guidance text with its
three evidence rows). The deployed agent's tools are fake-backed at
the post-correction fixture state (the known T3.1 standing
configuration: the layer is not served to the agent yet), so its
turns demonstrate the correction-honoured M; both halves are recorded
here, not conflated.

## Step 3 — shopper correction → M at once (live + agent)

Live `record_correction` ("I've lost weight, use M") against the
earned belief: Out of true, standing Venerable, guidance "Take size M
for Brand X"; recall carries the override at top level and inline,
with the belief kept beside it.

Agent turn 1 (trace `963018cb`, $0.0234, tools
`[recall_for_purchase]` — recall before the recommendation
generation): recommends M per correction `corr_3b7e52`, cites belief
`blf_8f2c41` with its 3 seeded fit returns (`ev_6d1a09`,
`ev_6d1a10`, `ev_6d1a11`), and asks the pending question (order
`ORD-7XK-03`). Full reply in `out/turn1-reply.txt` (git-ignored);
quoted in the report.

## Step 4 — approval, then a sandbox purchase of the M (live)

Agent turn 2 (trace `19b69d56`, $0.0295, tools
`[recall_for_purchase]`): buy intent for "the M in the PSW8GeK
stripe" answered with "Confirmed: Brand X Oxford Shirt, SKU PSW8GeK
(stripe), size M — guided by your correction … Shall I proceed?
Please reply yes to confirm." Agent turn 3 (trace `e0e01e59`,
$0.0159, no tool spans): "yes" → confirmation with the honest
decline (no purchase tool on the agent by design), then a pending
question.

The demo then executes the approved purchase through the frozen
orders machine (sandbox only): propose → capture-before-approval
refused with `ErrNoApproval` (transport never called) → scripted
shopper click on the real approval URL (attempt 1, order APPROVED) →
`ApproveAfterYes("yes")` with the source-side approval mirrored →
capture COMPLETED (`CAP-T36-R1`). The verified capture delivery
(`verify-webhook-signature` SUCCESS under its real event id
`EVT-T36-R1-PURCHASE`) joins the layer: 1 kept row, pending false.
Fits-well feedback ("The M fits well across the shoulders.") ties to
the live custom id (`CUSTOM-T36-R1`): verdict `fits_well`, accepted;
recall still serves the earned sizing belief (no rival).

## Step 5 — never-arrived dispute → seller only (live)

Reused, nothing created: the T1.3b OPEN dispute read back live —
reason `MERCHANDISE_OR_SERVICE_NOT_RECEIVED`, status
`WAITING_FOR_SELLER_RESPONSE` (status evolved since T0.4's OPEN; the
reason code is stable). Ingest of its recording twin: the sizing
recall is byte-identical before and after; the seller belief reads
Claimed with exactly 1 row. Sizing untouched.

## Step 6 — ledger + traces

`ledger_beliefs`: sizing Earned / Out of true / standing Venerable
with the correction beside it; seller Claimed. `ledger_evidence`
for the sizing belief: 3 rows, all counting toward fit.
`explain_belief`: 3 evidence, 1 correction, 4 timeline events.
Call log under the demo trace: `recall_for_purchase` then
`record_correction`. Turn traces: `963018cb` recall-first,
`19b69d56` recall-first, `e0e01e59` no tool spans (yes-turn claims
no purchase; the real purchase follows it in this transcript, after
the frozen approve-first gate).

## Spend (run 1)

Jev $0.000085596 · turns $0.0234 + $0.0295 + $0.0159 = $0.0688 ·
PayPal/Channel3/Lambo free. Billing $1.13 → $1.15 across the final
legs (the $0.67 → $1.15 session delta includes ~$0.40 from four
failed attempts before the clean run: stale-trace collisions, a
retryable 502, a request-id 422, a RETRY-throttled approval — each
fixed in the driver and recorded below).

## Reset procedure used

`bash demo/reset.sh 1` (fresh session, clean `out/`, spend baseline)
then `bash demo/run.sh`. The reset mints a new session id because an
erased id tombstones; PayPal history is never reset (immutable), the
run re-reads it. A leftover `PAYER_ACTION_REQUIRED` order from a
failed attempt is inert (no money moves without approval + capture)
and is superseded by the next reset's fresh order.

## What surprised (driver fixes this run)

- Capture reads for refunded cycles return `REFUNDED`, not
  `COMPLETED`: the assertion accepts either settled status.
- The orders service mints deterministic request ids when given
  none, so consecutive runs collide on `PayPal-Request-Id` (422):
  the driver passes a per-run id (`t36-<session>-propose`).
- Trace timestamps are completion times: the grace-window poll
  matched a previous run's trace. Candidates are now confirmed
  through the detail's conversation id before acceptance.
- The sandbox answered one scripted approval with a transient RETRY
  error page (the T0.4 throttle shape): the driver retries the login
  once, then fails. This run's approval succeeded on attempt 1.
- The messaging edge answered one send with a retryable 502: the
  driver backs off 70s and resends (3 attempts), then fails instead
  of polling for a trace that will never exist.
- `erase-session` refuses while the serve child holds the
  single-writer lease: the driver stops serve before erasing (this
  run's session was erased by hand after the lease expired: 9
  interactions, 9 concepts, 23 edges).
- Live dispute status reads `WAITING_FOR_SELLER_RESPONSE`, not
  `OPEN`: reason code asserted, status recorded.
