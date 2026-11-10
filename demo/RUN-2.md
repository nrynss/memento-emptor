# T3.6 run 2 (clean, from reset — the repeatability proof)

Session `t36-run-2-064902`, erased in-driver at the end (9
interactions, 9 concepts, 23 edges, 2 canonization events — the same
shape as run 1). Same driver commit as run 1's clean run; only the
run number differs. EXIT:0 with the built-in leak check clean.

## Outcome comparison (run 1 → run 2)

| Step | Run 1 | Run 2 | Same outcome? |
|---|---|---|---|
| 1. Three fit returns → Earned | Venerable, blast 3, guidance L | Venerable, blast 3, guidance L | yes |
| 2. Candidate + L cited | PSW8GeK, top-fit Regular | PSW8GeK, top-fit Regular | yes |
| 3. Correction → M at once | Out of true/Venerable, guidance M | Out of true/Venerable, guidance M | yes |
| 3. Agent turn 1 | M + citations + pending q, `963018cb` $0.0234 | M + citations + pending q, `7597dfbe` $0.0369 | yes (wording differs) |
| 3. Agent turn 2 (buy intent) | explicit-yes ask, `19b69d56` $0.0295 | explicit-yes ask, `343ec128` $0.0316 | yes (wording differs) |
| 3. Agent turn 3 (yes) | confirm + honest decline, no tools, `e0e01e59` $0.0159 | confirm + honest decline, no tools, `e703369e` $0.0174 | yes (wording differs) |
| 4. Purchase | COMPLETED after real approval | COMPLETED after real approval | yes |
| 4. Feedback | fits_well tied to live custom id | fits_well tied to live custom id | yes |
| 5. Dispute | reason NOT_ARRIVED, recall unchanged, seller Claimed/1 row | reason NOT_ARRIVED, recall unchanged, seller Claimed/1 row | yes |
| 6. Ledger + call log | 3 fit rows, [recall, correction] | 3 fit rows, [recall, correction] | yes |

Tool spans in order — run 2: `7597dfbe`
`[recall_for_purchase, pending_questions]` (turn 1 recalls first,
then asks the pending question through its tool), `343ec128`
`[recall_for_purchase]` (buy intent recalls first), `e703369e` `[]`
(the yes-turn moves no tool: no purchase without approval, and the
agent owns no purchase tool). Turn 1's reply again recommends M per
`corr_3b7e52`, cites `blf_8f2c41` with the 3 seeded returns, and asks
the pending question (order `ORD-7XK-05` this time — the fake's
pending-question fixture; the wording picks a different pending row,
the outcome is identical).

Jev: the same 4 phrases cost exactly the same as run 1
($0.000085596 total, build pinned) — deterministic pricing, not a
copy: each call went to the live endpoint (raw replies in `out/`).

## Differences that are expected, not drift

- New Lambo session, PayPal order/capture/custom/event ids,
  conversation and trace ids (placeholders `R2` beside run 1's `R1`).
- Reply prose differs (the model writes fresh text each turn); the
  asserted outcomes — size, citations, approval asks, tool spans —
  are identical.
- The scripted approval needed attempt 2 in run 2 (attempt 1 hit the
  transient RETRY page again; the backoff retry cleared it). Same
  mechanism as run 1's failed-then-fixed attempts, now inside one
  clean run.
- Live dispute status still `WAITING_FOR_SELLER_RESPONSE`.

## Spend (run 2)

Jev $0.000085596 · turns $0.0369 + $0.0316 + $0.0174 = $0.0859 ·
everything else free. Billing $1.15 → $1.24. Session total across
both clean runs plus the four failed attempts: $0.67 → $1.24
($0.57 new spend, under the $1.00 cap; no turn over $0.10 in either
clean run).
