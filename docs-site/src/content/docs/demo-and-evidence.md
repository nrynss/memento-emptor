---
title: The demo and evidence
description: "What the reference demo runs, which parts are live, which are replays or seeded, and what the run record does not prove."
---

The demo follows one shopper through PayPal's sandbox: a new shopper with no
history, three documented returns, one correction, a purchase, a dispute that
does not count toward sizing, the evidence ledger, and a second agent that
honours the correction it never saw. Then the merchant app shows the seller why
the product line comes back.

The committed run record is `demo/RUN-6.md`, with its evidence under
`demo/evidence/run-6/`. This page summarises it for a builder. Where the two
disagree, the run record wins.

## What runs where

| Piece | Where it runs | What the demo reads from it |
|---|---|---|
| The layer's edge | `https://memento-emptor.onrender.com` | Recall, `product_fit`, the ledger reads, corrections, feedback, pooling |
| The reference agent | A hosted agent deployment | The correction turn, then the feedback turn |
| The second agent | A separate hosted deployment | One fresh conversation: does it honour a correction it never saw? |
| The evidence ledger | `https://memento-emptor.onrender.com/ledger` and `/belief` | With a shopper link token the page reads the layer live and labels its source; without one it replays the demo story, labelled as a replay |
| The merchant app | `https://memento-merchant.onrender.com` | The live pool through `product_fit`, and the counts behind a status |

Keys come from the system keyring. Raw answers and full trace ids stay in
`demo/out/`, which is git-ignored; the committed evidence is redacted and uses
short trace ids.

## The eight steps

| Step | What it shows | Was it live in the recorded run? |
|---|---|---|
| **0. Day one, from the pool** | A new shopper has no history with the brand. The pool says the line runs small, so the agent can size up and cite the counts. | The pool read is live. The empty-shopper recall is blocked: the agent key binds one shopper, so a second shopper answers `401`. Recorded as it happened, not worked around. |
| **1. Three returns** | Three seeded refunds of the same size-M shirt, each with a classified fit reason, stand behind one belief at Venerable. | A live read of seeded rows. The belief reads Out of true because an earlier correction was already in force before this run. |
| **2. The belief they earn** | The belief says the brand runs small and to take L; the pool agrees. | The belief and pool are live reads. In this run the reference agent made no live catalog call for the candidate, so the SKU appears only in the later captured purchase. |
| **3. The correction, through the agent** | The shopper says "I've lost weight, use M". The next recommendation is M; the L belief stays, contradicted, with its three rows. | Live: three agent turns, with recall before the recommendation. |
| **4. The purchase and feedback** | Fit feedback tied to a specific purchase settles a live pool row: 22 settled becomes 23, and 1 becomes live. | The purchase is an existing sandbox capture; the feedback turn is live. A new purchase cannot be ingested in this shape. |
| **5. The dispute that leaves sizing alone** | A not-arrived dispute sits on the seller belief with exactly one row. The sizing belief keeps exactly its three fit rows. | A live read of an existing row. The seller belief does not reach recall in this shape, so no agent turn mentions it. |
| **6. The ledger** | Beliefs, the evidence behind each, the correction, and every status and seeded mark. | Live API reads plus the fixture-replay page. The run predates the ledger's live transport, so its screenshots are labelled as the replay and are never counted as the live read. |
| **7. The second agent and the merchant** | A fresh conversation recalls first and recommends M, citing the correction it never saw. The merchant app reads the live pool. | Live: one agent turn and one merchant read. |

## What is live, replayed, or seeded

- **Live:** the layer's reads and writes through the hosted edge, the agent turns,
  the merchant read, and the pool row the feedback settles.
- **Seeded:** the three October returns and the population behind the pool. The
  ledger marks every seeded row and shows the seeded count.
- **Replayed:** the ledger screenshots in the run record. The page replays the
  demo story when no shopper token is present, and labels it as a replay.
- **Recorded from an earlier sandbox cycle:** the purchase capture and the
  not-arrived dispute. They are real sandbox events, but this run did not create
  them.

## What the run does not prove

The record lists ten gaps. Five have since closed in the tree, one is covered by
a sequencing rule, and four remain open:

1. **The day-one empty shopper is refused by the edge.** The agent key binds the
   demo shopper only, so the pool read is live while the empty-shopper recall is
   recorded as `401`.
2. **A new purchase and a new dispute cannot be ingested.** The PayPal app's only
   webhook subscription points at an external test receiver, and the served
   ingest reads verified events through a recording payment source. The run uses
   the existing capture and writes the fit row through `record_feedback`.
3. **The seller belief never reaches recall.** The live reads prove sizing is
   untouched by the dispute; no live agent turn can mention it.
4. **Step 2's catalog turn named a SKU the shopper's memory already carries.**
   After a sequencing fix, recall answered before the search, but the model
   dropped the returned-SKU exclusion when the filtered search came back empty
   and named a SKU from its own evidence rows. The step-2 "new candidate" clause
   is therefore the one part of the story the run record does not close.

None of this is filled in from a fixture or smoothed over in the record. The
ledger's replay mode exists so the page can be shown without a token; it is
always labelled.

## Running it

From the repository root, with the keys loaded from the keyring:

```sh
bash demo/reset.sh <run-number>
MEMENTO_SECOND_DEPLOYMENT=<deployment-id> bash demo/run.sh
```

The reset restores the starting pool counts without touching the seeded pool or
the seller's other lines, and asserts the starting state. It cannot delete a
correction, because corrections are append-only; each run records a new one and
the newest wins.

If a step fails, the runner can resume without repeating a paid turn:

```sh
python3 demo/run.py --resume
```

`--summary-only` rebuilds the summary from recorded evidence with no live call at
all.

## Where the evidence lives

- `demo/evidence/run-6/` — the redacted run: pool and recall reads, the belief
  explained, the correction turns, feedback, the dispute, the ledger reads, the
  second agent, and the merchant app.
- `demo/evidence/step2-catalog/` — the catalog turns, with their ordering,
  searches, credit use and the candidate each named.
- `bench/` — the comparison harness and its committed runs; see
  [The comparison](/comparison/).
