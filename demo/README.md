# T3.6: the full demo, live on the real stack

Product.md's six demo steps, end to end, starting from the seeded demo
shopper. One orchestration (`run.sh`) drives only frozen pieces: the
recording payment source (whose shapes T2.1b proved identical to the
live sandbox reads), the frozen interface service and write rule over a
live Lambo session with native embeddings, a live Jev classifier, the
frozen catalog match over a live Channel3 detail, the deployed reference
agent over the messaging proxy, and the frozen orders state machine over
a live sandbox REST transport.

## The six steps and where each runs live

1. **Three documented fit returns → Earned.** The three T1.3b sandbox
   cycles (`t13b-demo-003/004/005`) are read back live (refund
   COMPLETED, capture settled — REFUNDED, as the cycles were fully
   refunded by design — order with line items, free). The layer
   ingests their recording twins (`WH-2EV-01/02/03`, whose shapes T2.1b
   proved identical to the live reads); the live calls are
   verification, not the ingested bytes. Their reason words go through
   live Jev (4 classifications on the fixture wording incl. the vague
   ask-again, ~$0.0001 total, pinned build, 10x-per-call guard) and the
   writes land on a live Lambo 0.10.1 session (native
   `.jina-embeddings-v3`, throwaway SQLite store beside the session,
   erased at the end). Recall serves Earned/Venerable with guidance
   toward L.
2. **Channel3 candidate, size-L recommendation.** One live detail call
   for `PSW8GeK` (free; detail calls cost no credits, no searches are
   used) plus the returned `tbnrl46`: the frozen match keeps the new SKU
   on brand + category + equal `top-fit`, skips the returned SKU, and
   marks sizes-at-retailer and simulated fulfilment. The L
   recommendation is served by the live layer recall (guidance text with
   its three evidence rows). The deployed agent's tools are fake-backed
   at the post-correction fixture state (the known T3.1 standing
   configuration: the layer is not served to the agent yet), so its
   turns demonstrate the correction-honoured M, not the pre-correction
   L. Both halves are recorded below.
3. **Shopper correction → M at once.** Live `record_correction`
   against the earned belief: Out of true, standing Venerable, guidance
   M. Agent turn 1 (shopping question) recommends M per the correction
   as seen through its fake-backed tools (post-correction fixture
   state, not the live session), cites the belief/returns, asks the
   pending question and asks for an explicit yes. Its trace shows
   `recall_for_purchase` before the recommendation generation.
4. **Approval, then a sandbox purchase of the M.** Agent turn 2 answers
   "yes". The demo then executes the approved purchase through the
   frozen orders machine: propose → capture-before-approval refused
   (`ErrNoApproval`, transport never called) → scripted shopper click on
   the real approval URL → `ApproveAfterYes("yes")` with the
   source-side approval mirrored → capture COMPLETED. The verified
   capture delivery joins the layer under its real event id, and
   fits-well feedback ties to the live custom id. Sandbox only; the
   service constructor refuses anything else.
5. **Never-arrived dispute → seller only.** The T1.3b OPEN dispute is
   read back live (reason `MERCHANDISE_OR_SERVICE_NOT_RECEIVED`) and its
   recording twin is ingested: the sizing recall is byte-identical
   before and after, and the seller belief reads Claimed with exactly
   one row.
6. **Ledger + traces.** `ledger_beliefs`, `ledger_evidence` and
   `explain_belief` snapshots plus the demo trace's call log
   (`recall_for_purchase` then `record_correction`) and the two agent
   traces with their tool spans in order, costs and reply texts.

## Reset procedure

```sh
bash demo/reset.sh <run-number>   # fresh Lambo session (erased ids tombstone, never reused), clean out/, spend baseline
bash demo/run.sh                  # the six steps; stops on the first failure or tripped guard
```

PayPal sandbox records are immutable history: no reset deletes them,
and every run re-reads the T1.3b cycles. Each run creates exactly one
new sandbox order (the step-4 purchase) plus its capture; refunds and
disputes are never created by the demo. A second run from reset repeats
the same six outcomes with new session, order, conversation and trace
ids; only the reply *wording* may differ (the outcomes are asserted,
the prose is recorded).

## Placeholders

Committed files carry placeholders only (T0.4 table plus RUN-scoped
ids). Real ids stay in `demo/out/` (git-ignored, mode 0600).

| Placeholder | What was removed |
|---|---|
| `ORD-LIVE-R<n>-F<k>` / `CAP-…` / `RFD-…` / `EVT-…` | live fit-cycle order/capture/refund/webhook-event ids |
| `DSP-LIVE-R<n>`, `EVT-LIVE-R<n>-D1` | live dispute id and its webhook event id |
| `ORD-T36-R<n>` / `CAP-T36-R<n>` / `CUSTOM-T36-R<n>` / `EVT-T36-R<n>-PURCHASE` | the run's new purchase order/capture/custom/event ids |
| `APPROVAL-URL-REDACTED` | the payer-action approval URL (carries an order token) |
| `CONV-T36-R<n>` | the agent conversation id |
| `TRACE-T36-R<n>[-T1,-T2]` (8 hex chars in prose) | agent/platform trace ids (kept short; costs kept) |
| `BLF-T36-R<n>[-SELLER]` | the run's belief ids |

Trace ids and costs are kept (short ids in prose, full only in `out/`).
No key, token, email, shopper credential, bearer, order id or other
live value enters any committed file; the only route config is
`drive.py`'s standing deployment id and API host (the agent's address,
no secret — predates this task). The driver refuses to finish if a live
value outside that config survives into `demo/` outside `out/`.

## Spend

| Leg | Cost model | Guard |
|---|---|---|
| PayPal reads, capture, Channel3 details | sandbox free, details free | none needed |
| Jev (4 classifications/run) | ~$0.00002 each | pinned build, 10x per-call, stop |
| Lambo + Elastic native inference | no billable resources | session erased |
| Astropods turns (2/run) | ~$0.03-0.04 each | $0.10 per turn, billing before/after every turn |
| Evaluator re-run (if any) | server-side judge | billing delta watched, $0.10 cap |

Cap: stop and report if total new spend exceeds $1.00 or any single
turn exceeds $0.10. Prefer reusing existing live records (T1.3b cycles,
T2.4b session patterns) over creating new ones: only the step-4
purchase is created per run, because a purchase cannot be reused.

## Runs

Run records follow once the clean runs complete.
