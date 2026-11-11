---
title: The comparison
description: "What the first comparison round measured, honestly labelled, and the second round's arms and cases while its results are pending."
---

The project measures whether memory changes what a shopping agent recommends. It
does not measure real-world return rates, and it does not claim to.

:::caution[Read the limit first]
We wrote both the shoppers and the hidden right answers. A count like 15 of 15
means "right on all 15 frozen turns", not "right on every future shopper".
Nothing here transfers to real shoppers, real catalogs, or another model.
:::

## Round 1, honestly relabelled

Round 1 compared three arms:

1. **No memory** — nothing about past purchases.
2. **Memento evidence without beliefs** — the layer's evidence rows handed to the
   agent directly: classified reason, signal strength and write kind, with no
   beliefs built over them.
3. **Memento layer** — live `recall_for_purchase` and `pending_questions`
   responses, recorded in each run file.

The arm this round originally called "raw history" is arm 2. Its prompt carried
rows the layer had already classified, so calling it raw PayPal history
overstated what it was given. The runs, scores and freeze inputs are unchanged;
only the label is corrected. The committed run files keep the old arm name
because they are frozen output.

## Headline counts

Five constructed shoppers, three live turns per shopper per arm, 45 turns. All
runs are committed under `bench/runs/`.

| Arm | Right size first time | Wrong-cause errors | Corrections honoured | Supported / cited | Tokens (words, 15 runs) |
|---|---|---|---|---|---|
| 1 — no memory | 0 of 15 | 0 | nil (no corrections in set) | 0 of 0 | 1309 total, 83–97 per run |
| 2 — Memento evidence without beliefs | 15 of 15 | 0 | nil (no corrections in set) | 6 of 40 | 3606 total, 216–289 per run |
| 3 — Memento layer | 0 of 15 true recommendations | 0 | nil (no corrections in set) | 0 of 0 | 2242 total, 130–215 per run |

Arm 2 beat arm 3 on right-size recommendations on this frozen set: 15 of 15
against 0 of 15. That is stated plainly, not spun. On these small, clean
histories the evidence-without-beliefs arm got the size right every time, while
the layer refused to recommend without an Earned belief. Arm 1 recommended
nothing in 12 of 15 turns and recommended the wrong size in the other three.

Arm 3's raw tally was 3 of 15 because the deterministic extractor picked a size
named inside three informed refusals. Those were refusals, not recommendations;
the run files record the artifact.

## The correction round

The 45-turn set's seed histories contain no corrections, so the correction
measure is nil there, not zero. A separate round added one correction turn to
every cell afterwards: 9 shoppers × 3 arms = 27 turns.

- **Honoured on the next recommendation: 27 of 27**, 9 of 9 in every arm.
- **Wrong-cause errors: 0** across all 27 turns.
- Arm 3 routed the correction through the real `record_correction` path in 2 of 9
  cells. In the other 7, recall served no belief before or after, so there was no
  belief to contradict; the correction travelled in conversation text, the same
  path arms 1 and 2 have.

The mechanism difference is real; the outcome difference is not. Every arm
honoured every correction, so this round shows that a stated preference is
followed on the next turn, not that any arm handles corrections better.

## The hardened set

Four more constructed shoppers were added to separate the arms where the first
five could not: two long, noisy histories and two multi-brand histories. One live
turn per shopper per arm, 12 turns.

| Arm | Right size | Wrong-cause | Supported / cited | Tokens (words, 4 runs) |
|---|---|---|---|---|
| 1 — no memory | 0 of 4 | 0 | 0 of 0 | 349 |
| 2 — Memento evidence without beliefs | 4 of 4 | 0 | 9 of 10 | 2221 |
| 3 — Memento layer | 2 of 4 | 0 | 6 of 6 | 1157 |

Arm 2 kept its lead. Arm 3 recommended in the two noisy cells only; in both
multi-brand cells recall served no belief and the reply declined. Those two cells
show a refusal, not anything about cross-brand cut matching.

## What round 1 proves, and what it does not

**Proves on these constructed cases:** the refusal mechanism works as designed.
With no Earned belief the layer says "no recommendation yet" instead of guessing,
and it never once converted a non-fit outcome into a size move (0 wrong-cause in
15 arm-3 turns). It also proves that Memento evidence without beliefs is a strong
baseline on small, clean histories: arm 2 reasoned correctly about damage,
non-arrival, changed mind and vague rows 15 times out of 15 with 0 wrong-cause.

**Does not prove:** real-world accuracy; correction handling beyond one stated
correction on the next turn; citation quality at scale (34 of arm 2's 40 cites
resolve to non-fit rows); behaviour on long noisy or multi-brand histories beyond
four shoppers and one turn per cell; or anything about cost beyond the recorded
trace spend. The lead number for a video is chosen from the results, not before
them.

## Round 2: pending

:::caution[Not measured yet]
Round 2 has been designed and frozen. Its runs and results are not in this site
yet, so this section reports the design only. No counts are shown and none are
invented.
:::

The second round keeps the same agent, model and catalog in every arm, so the
memory condition is the only difference:

| # | Condition |
|---|---|
| 1 | **No memory**: nothing about past purchases |
| 2 | **PayPal history**: exactly what the Agent Toolkit returns for this shopper — transactions, line items, refund and dispute status, dispute reason codes — and nothing the layer produced. No refund reasons, no classification, no signal strength |
| 3 | **Memento, shopper only**: the shopper's own reasoned evidence, labels and corrections, with no pool. Round 1's winning arm under its honest name |
| 4 | **Memento, full**: shopper evidence, the pool, corrections, and the precedence rule |

Arm 2 is the baseline that matters: "why not just give my agent the transaction
history?" This time it gets the transaction history and only that.

The shopper set has four cases, each chosen because it separates the arms for a
stated reason:

| Case | What it tests |
|---|---|
| **New to the brand** | No personal history with the brand; the pool knows it runs off-size |
| **Unlike the crowd** | The shopper's own evidence disagrees with the pool, and the right answer is theirs. Naive pooling fails here |
| **Noisy refunds** | Mixed causes — damage, non-arrival, changed mind, fit — where a refund status without a reason cannot tell fit from the rest |
| **Correction, then another agent** | A correction made to agent A; agent B asks next, with PayPal access but not A's conversation |

The measures are counts, never percentages: right size first time against a
hidden answer for each shopper; wrong-cause errors; corrections honoured on the
very next recommendation, in the next session and by another agent; which
precedence step decided the answer; citations that really support the
recommendation; and tokens per recommendation.

The shoppers, pooled population, catalog, prompt and grader are frozen before
any arm 3 or 4 run, and every run is published. When the results land, this
section will report them beside the limits that apply.
