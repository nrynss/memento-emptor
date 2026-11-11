# The demo, after the pivot

Product.md's demo (steps 0-7) against the hosted services. One shopper,
one layer, two agents and two read-only apps. The run record is
[`RUN-6.md`](RUN-6.md) and its evidence is in [`evidence/run-6/`](evidence/run-6/).

## What runs where

| Piece | Address | What the demo reads from it |
|---|---|---|
| The layer's edge | `https://memento-emptor.onrender.com` | `recall_for_purchase`, `product_fit`, the ledger reads, `record_correction`, `record_feedback`, `set_pooling` |
| The reference agent | Astropods deployment `MEMENTO-REF` | two conversations: the correction, then the feedback |
| The second agent | Astropods deployment `MEMENTO-SECOND` | one fresh conversation: does it honour the correction it never saw? |
| The evidence ledger | `https://memento-emptor.onrender.com/belief` and `/ledger` | the page as deployed (a fixture replay; the live reads are the API) |
| The merchant app | `https://memento-merchant.onrender.com` | the live pool through `product_fit`, and the "why" for the demo line |

Keys come from the system keyring (`source tools/env.sh`, plus the caller
tokens in `memento-tokens`). The scripts read them into the process and never
print or write one. Raw answers and full trace ids stay in `demo/out/`
(git-ignored, mode 0600); the committed evidence is redacted and uses short
trace ids.

## The eight steps and each one's artifact

| Step | Label | What happened | Artifact |
|---|---|---|---|
| **0. Day one, from the pool** | live (pool read) | `product_fit` for brand-x-001 / shirts / regular, size M reads **Earned, small**: 22 settled, 22 seeded, 12 too-small returns against 3 too-large and 4 fits-well, 19 fit shoppers. The empty-shopper recall is **blocked**: the agent key binds `shopper_01` only, so `shopper_02` answers `401 unauthenticated`. Recorded, not faked. | [`00-pool-and-new-shopper.json`](evidence/run-6/00-pool-and-new-shopper.json) |
| **1. Three returns** | live read of seeded rows | Live recall and `explain_belief` for `blf_8f2c41`: three seeded refunds of the M oxford, every reason `fit_too_small` ("too tight in the shoulders", "too tight across the chest", "sleeves too short in M"), standing **Venerable**; its status is **Out of true** because a correction was already in force before the run (see **reset**). | [`01-three-returns-and-belief.json`](evidence/run-6/01-three-returns-and-belief.json), [`02-belief-explained.json`](evidence/run-6/02-belief-explained.json) |
| **2. The belief they earn** | live read; no Channel3 call | The belief's value is "runs small, take size L" and the pool agrees (also L). No live Channel3 candidate call exists in the reference agent, so the candidate SKU (`PSW8GeK`) appears only in the captured purchase in step 4 (see **gaps**). | same as step 1 |
| **3. The correction, through the reference agent** | live (three agent turns) | Two turns in one conversation. Trace `ded28ebc` (ask) recalls first; trace `c94448ee` recalls then calls `record_correction` and the layer mints `corr_000004`; trace `82d6512f` recalls and recommends M under it. The belief stays, **Out of true**, with its three rows. | [`03-correction-through-the-agent.json`](evidence/run-6/03-correction-through-the-agent.json) |
| **4. The purchase and feedback** | existing capture + live turn | The purchase is the capture `ev_51c2d8`, custom id `memento-shopper01-06`, SKU `PSW8GeK`, size M, recorded in an earlier sandbox cycle; a new purchase cannot be ingested (see **gaps**). Trace `11d83a4f` calls `record_feedback`; the fit row settles live and the pool moves 22 → 23 settled, 0 → 1 live, fits-well 4 → 5. | [`04-purchase-and-feedback.json`](evidence/run-6/04-purchase-and-feedback.json) |
| **5. The dispute that leaves sizing alone** | live read of an existing row | Live reads: seller belief `blf_1a5e09` is **Claimed** with exactly one row, the not-arrived dispute `PP-D-51713` recorded in an earlier sandbox cycle; the sizing belief still holds exactly its three fit rows and no dispute row, and recall still decides on the correction, not on the dispute. | [`05-dispute-leaves-sizing-alone.json`](evidence/run-6/05-dispute-leaves-sizing-alone.json) |
| **6. The ledger** | live API reads + fixture-replay page | Live `ledger_beliefs`, `ledger_evidence` and recall: the decision is the correction, the pool is overridden by it (`pool.precedence`), the belief rows carry statuses, seeded marks and counts. The deployed page is a **fixture replay** (60 settled / 48 seeded in the screenshot, not the live 23 / 22 / 1) and has no live transport yet; it is shown and labelled, never counted as the live read. | [`06-ledger.json`](evidence/run-6/06-ledger.json), `06-ledger-belief.png`, `06-ledger-table.png` |
| **7. The second agent and the merchant** | live (one agent turn; live merchant read) | Trace `9d4893e7` (fresh conversation, no correction in its prompt) recalls first and recommends M citing `corr_000004`, quotes its words, and shows the overridden pool and beliefs with the live/seeded split. The merchant app reads **Earned small, 23 settled (22 seeded, 1 live)** through its proxy; its "why" panel names the counts and the rule. | [`07-second-agent-and-merchant.json`](evidence/run-6/07-second-agent-and-merchant.json), `07-merchant.png` |

Step 1's and step 2's evidence was re-read during the step-7 resume, after
the feedback, so the pool block inside `01-three-returns-and-belief.json`
shows the post-feedback counts (23 settled, 1 live). The belief, its three
returns and the correction are unchanged by the re-read.

The run was recorded in one pass, then a transient `401` from the agent
platform's conversation create stopped step 7; `demo/run.py --resume`
re-read the free layer state and drove that one turn, so the paid turns are
the ones the record cites. The summary in the evidence says so.

## Reset

```sh
bash demo/reset.sh <run-number>
```

It reads the demo line and restores the starting pool counts without touching
the seeded pool or the seller's other lines. When an earlier run left a live
settled row, it opts the demo shopper out of pooling (the layer removes that
shopper's own fit rows and reports how many) and back in; it opts in even when
no row was left, so an interrupted reset cannot leave the shopper out. It then
asserts the seeded state: the pool Earned small at 22 settled / 22 seeded / 0
live, the sizing belief with its three seeded returns, the latest capture and
dispute in memory, and a spend baseline.

What a reset cannot do: **delete a correction**. Corrections are append-only
and live in the layer's standing journal, so each run records a new one and
the newest wins. The reset records the correction in force at the start so
the run's own correction can be told apart from it. It also cannot take back
feedback already written: opting out removes the pool row, but the fit belief
keeps every copy the feedback appended, so a second run starts with more
copies than the first.

## Run

```sh
MEMENTO_SECOND_DEPLOYMENT=<id> bash demo/run.sh
```

The second agent's deployment id never lives in this repository; it comes
from the operator's environment. Every step writes its raw answer under
`demo/out/run-<n>/` and its redacted evidence under `demo/evidence/run-<n>/`.

If the platform drops a step, `python3 demo/run.py --resume` re-reads the
free layer state and drives the missing turn without repeating a paid turn;
`--summary-only` rebuilds the summary from the recorded evidence with no live
call at all.

## Spend

This task spent **$0.69** of usage (billing $4.95 → $5.64; credit $5.05 →
$4.36). The recorded run's five turns are **$0.2377**; the rehearsal step-0
turn, the eight turns across three failed attempts and the two superseded
step-7 turns come to **$0.396**, so the sixteen traces behind the task total
**$0.634** and the rest of the billing delta is the platform's compute. The
layer, PayPal sandbox, both read-only apps and the Elastic pool reads cost
nothing. Budget was $1.00; the task fits inside it.

## What is not live, and who owns it

The run's summary carries gaps 1, 3, 4, 5 and 7; gap 2's probe and
subscription facts are in
[`04-purchase-blockers.json`](evidence/run-6/04-purchase-blockers.json); gaps
6, 8, 9 and 10 were read from the traces and the served layer while this
record was written. None is filled in from a fixture:

1. **Step 0's empty shopper** is refused by the edge: the agent key binds
   `shopper_01` only. Until an operator adds the day-one shopper to the key's
   binding (a Render secret change) or a new task mints a second agent key,
   the pool read is live and the empty-shopper recall is recorded as `401`.
   Owner: the operator (the secret) or a new task (a second key).
2. **A new purchase and a new dispute cannot be ingested.** The PayPal app's
   only webhook subscription points at an external test receiver, not at the
   layer, and the served layer's ingest reads verified events through the
   recording demo source (`cmd/memento` passes `fakes.NewRecordingPaymentSource`),
   so a genuine delivery would verify and then find no event to read. The
   run records the existing capture and writes the fit row through
   `record_feedback`; the probe and the subscription facts are in
   [`04-purchase-blockers.json`](evidence/run-6/04-purchase-blockers.json).
   Owner: a layer/deploy task (proposed T9.10).
3. **The ledger page is a fixture replay.** It has no live transport; the
   live reads are the API answers in step 6. Owner: a `web/` task (proposed
   T9.11).
4. **The seller belief never reaches recall**, and the agent key is refused
   on ledger reads, so no live agent turn can mention the dispute; the live
   reads prove sizing is untouched. Owner: T9.9 (surface alignment) or a
   layer recall extension.
5. **`record_feedback` is not idempotent per purchase** in the shopper's
   belief: a second feedback for the same purchase appends the same evidence
   again. The seeded arc's one copy plus this run's two feedback calls left
   `ev_51c2d8` three times on `blf_2b9d60` (which lifted it from Claimed to
   Building). The pool row is idempotent; the belief is not. Owner: a layer
   task (proposed T9.10).
6. **A correction can reuse an id after a restart.** The served layer
   restores the standing journal but not the id sequence, so the next mint
   overwrote the restored `corr_000001` during an early attempt (the run's
   own `corr_000004` is unaffected). Recorded, not worked around. Owner: the
   same layer task (proposed T9.10).
7. **The reference agent skipped recall** on the correction turn in one
   attempt and on the recorded feedback turn; recall-first holds on the ask
   and next-recommendation turns. The traces show it as it happened. Owner:
   T9.9.
8. **Both deployed apps render their own source comments** as visible text
   above the heading (the `// ...` blocks), and the belief page prints its
   standing-correction empty-state sentence twice. Presentation defects
   outside this task's `owns`; owner a presentation task under `web/` and
   `merchant/` (proposed T9.11).
9. **`explain_belief` lists a correction twice.** The live answer for
   `blf_8f2c41` carries seven corrections for five ids: `corr_000001` and
   `corr_3b7e52` each appear twice
   ([`02-belief-explained.json`](evidence/run-6/02-belief-explained.json)).
   `restoreStanding` appends the journal's ids to the correction order and
   `LoadSeed` adds the seeded one again, and a reused id is appended a second
   time; the read does not de-duplicate. Owner: a layer task (proposed
   T9.10).
10. **Step 2's Channel3 candidate is not live.** The reference agent has no
    product-search tool, so no Channel3 call exists in the run and the
    candidate SKU appears only in step 4's captured purchase. Owner: T9.9
    (agent tool surface) or a new task the orchestrator opens.

## The pre-pivot record

`RUN-1.md`, `RUN-2.md`, `drive.py` and `driver/` are the T3.6 demo on the
pre-pivot stack (local Lambo session, recording payments, the layer not yet
served to the agents). They are kept as history and are not used by the
pivot run above.
