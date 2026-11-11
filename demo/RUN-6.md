# Run 6: the pivot demo, live

The recorded run of product.md's steps 0-7 against the hosted layer, both
deployed agents and both read-only apps. Raw answers and full trace ids are
in `out/run-6/` (git-ignored); this record cites the redacted evidence under
[`evidence/run-6/`](evidence/run-6/).

Recorded in one pass, then a transient `401` from the agent platform stopped
step 7; `demo/run.py --resume` re-read the free layer state and drove that one
turn. No paid turn was repeated: the recorded run's five turns are the ones
below. Spend is in the last section.

## The run

| Step | Label | Outcome | Evidence |
|---|---|---|---|
| 0 | live (pool read) | Pool `Earned small`, size M: 22 settled (22 seeded, 0 live), 12 too-small returns / 3 too-large / 4 fits-well, 2 kept, 19 fit shoppers. `shopper_02` recall: **401 `unauthenticated`** (the agent key binds `shopper_01` only). | `00-pool-and-new-shopper.json` |
| 1 | live read of seeded rows | Three seeded refunds of the M oxford, all `fit_too_small`: "too tight in the shoulders" (`ev_6d1a09`), "too tight across the chest" (`ev_6d1a10`), "sleeves too short in M" (`ev_6d1a11`). Belief `blf_8f2c41`, value "runs small, take size L", standing **Venerable**. | `01-three-returns-and-belief.json`, `02-belief-explained.json` |
| 2 | live read; no Channel3 call | The pool independently reads small and also points L. The belief is **Out of true** because a correction is in force (it was already in force at the start; see the reset note). The reference agent has no Channel3 search, so no live candidate call exists; the candidate SKU `PSW8GeK` is visible in the step-4 purchase only. | `01-…`, `02-…` |
| 3 | live (three agent turns) | Trace `ded28ebc` (ask): `recall_for_purchase` → M under the pre-run correction `corr_000003`. Trace `c94448ee` (correct): `recall_for_purchase` → `record_correction`; the layer mints **`corr_000004`** with the shopper's words. Trace `82d6512f` (next): `recall_for_purchase` → M under `corr_000004`. Belief `blf_8f2c41` stays with its three rows, **Out of true**. | `03-correction-through-the-agent.json` |
| 4 | existing capture + live turn | Purchase: the capture `ev_51c2d8`, custom id `memento-shopper01-06`, SKU `PSW8GeK`, size M, recorded in an earlier sandbox cycle. Trace `11d83a4f`: `record_feedback` ("fits well across the shoulders and in the sleeves") → verdict `fits_well`, belief `blf_2b9d60`. Fit row settles live: pool 22 → **23 settled**, 0 → **1 live**, fits-well 4 → 5. A new purchase cannot be ingested: the app's one webhook subscription does not point at the layer, and the served layer reads verified events from the recording demo source; the unsigned-delivery probe answers 400 at the gate. | `04-purchase-and-feedback.json`, `04-purchase-blockers.json` |
| 5 | live read of an existing row | Seller belief `blf_1a5e09` **Claimed**, one row: dispute `PP-D-51713` (`ev_9d4c22`, recorded in an earlier cycle), `not_arrived`, counts `seller_reliability`. Sizing belief still exactly `ev_6d1a09/10/11`; no dispute row; recall still decides on the correction, not the dispute. | `05-dispute-leaves-sizing-alone.json` |
| 6 | live API reads + fixture-replay page | Live ledger reads: `blf_8f2c41` Out of true (3), `blf_2b9d60` Building (3, with the same purchase repeated — see gaps), `blf_1a5e09` Claimed (1), `blf_77c310` Claimed (0). Recall: basis `correction` over `corr_000004`, pool precedence `{step: pool, overridden_by: correction}`. The deployed page is a fixture replay (`06-ledger-belief.png`, `06-ledger-table.png`; 60 settled / 48 seeded in the screenshot); the live numbers are the JSON. | `06-ledger.json`, two screens |
| 7 | live (one agent turn; live merchant read) | Second agent, fresh conversation, trace `9d4893e7`: `recall_for_purchase` first, then **M** citing `corr_000004`, quoting the correction's words, and showing the overridden pool and beliefs (23 settled: 22 seeded, 1 live). The merchant proxy reads **Earned small, 23 settled (22 seeded, 1 live)**; the why panel names the counts and the rule. | `07-second-agent-and-merchant.json`, `07-merchant.png` |

Steps 1 and 2 were re-read during the step-7 resume, after the feedback, so
`01-three-returns-and-belief.json` carries the post-feedback pool (23
settled, 1 live) beside the belief and its three returns. Nothing in step 1
or 2 is quoted from that pool block.

## Step 0 in the shopper's words

The pool read is live and complete: the day-one sizing source is Earned from
22 seeded settled purchases, 12 of them too small. The shopper half is not
showable: the only agent key is bound to `shopper_01`, so any other shopper
is refused before the pool is reached. The reset and the pool read are the
proof that the layer would size a new shopper from the pool; a live turn for
one needs an operator to widen the key binding. The decision belongs to the
orchestrator, not to this run.

## Spend

Billing usage $4.95 → $5.64 = **$0.69** of the $1.00 budget (credit $5.05 →
$4.36). The recorded run's five turns, from their traces:

| Turn | Trace | Cost |
|---|---|---|
| step 3 ask | `ded28ebc` | $0.0337 |
| step 3 correct | `c94448ee` | $0.0759 |
| step 3 next | `82d6512f` | $0.0658 |
| step 4 feedback | `11d83a4f` | $0.0239 |
| step 7 second agent | `9d4893e7` | $0.0385 |
| **run total** | | **$0.2377** |

The rest of the billing is agent turns plus the platform's compute: the
rehearsal step-0 turn ($0.032), eight turns across three failed attempts
before the recorded pass ($0.290) and two step-7 turns superseded while the
record was fixed ($0.074). The sixteen traces behind the task come to
$0.634; the remaining $0.056 of the billing delta is the platform's compute.

The reset immediately before this run removed one live settled row from an
earlier attempt (the shopper's `set_pooling` opt-out and back in), and the
step-0 pool read came back 22 settled / 22 seeded / 0 live: the starting
state in the evidence is the reset's answer, not a remembered one.

## Gaps, with owners

Fixed since this run: 3, 5, 6, 8 and 9 are closed in the tree (8 also on the
served page after its next deploy); 7 is covered by T9.22's sequencing rule,
proved by T9.23's re-record; 10's catalog tool is live (T9.20) and its
candidate gap is T9.22/T9.23. Open: 1, 2 and 4.

1. **Day-one shopper recall refused** (401, `shopper_02`). Owner: **T9.15**
   (second shopper key); the demo cannot change a Render secret.
2. **No live purchase or dispute ingest.** The PayPal app's only webhook
   subscription points at an external test receiver, not the layer, and the
   served layer reads verified events through `fakes.NewRecordingPaymentSource`
   in `cmd/memento`. Owner: **T8.7b** (live payment events).
3. **Ledger page had no live transport** (fixture-only). **Closed by T4.8**:
   `/ledger?t=` reads live and labels the source; without a token the demo
   story replays, labelled.
4. **Seller belief never reaches recall**, and agent keys are refused on
   ledger reads, so no agent turn can mention the dispute. Owner: **T9.24**
   (a live path for the dispute).
5. **`record_feedback` duplicated a purchase in the belief.** **Closed by
   T8.5l**: a repeat answers the feedback on record without minting, appending
   or moving the status.
6. **Correction id reused after a restart.** **Closed by T8.5m**: the mint
   sequence resumes past every restored id.
7. **Reference agent skipped recall** on the recorded feedback turn. **Closed
   by T9.9's retry rule and T9.22's recall-before-catalog sequencing**; T9.23
   proves the live turn.
8. **Both apps rendered their own source comments** as visible text, and the
   belief page printed its standing-correction sentence twice. **Closed by the
   P4 header guard and T4.6**; the served page shows it after its next deploy.
9. **`explain_belief` listed a correction twice.** **Closed by T8.5m**: each
   correction is served once.
10. **Step 2's Channel3 candidate.** T9.20 made the catalog live; the recorded
    turn named the returned SKU. T9.22 sequences recall before the search and
    excludes the returned SKUs; **T9.23** owns the redeploy and the re-record,
    and T9.17 stays in review until a fresh candidate is named.
