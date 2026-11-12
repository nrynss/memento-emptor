---
title: The comparison
description: "What the first and second comparison rounds measured, honestly labelled, with the limits and the counts behind every claim."
---

The project measures whether memory changes what a shopping agent recommends. It
does not measure real-world return rates, and it does not claim to.

:::caution[Read the limits first]
We wrote the shoppers, the hidden right answers and the pooled population. A
count like 15 of 15 means "right on these frozen turns", not "right on every
future shopper". Nothing here transfers to real shoppers, another catalog or
another model.
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

## Round 1: headline counts

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

## Round 1: the correction round

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

## Round 1: the hardened set

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

## Round 2: measured

:::caution[Read these limits first]
We wrote the shoppers, the hidden answers, the population, the prompts and the
grader. This shows the mechanism on eight constructed shoppers in four cases,
not real-world accuracy. A count like 12/16 means "right on 12 of these 16
frozen turns", not "right on 12 of 16 shoppers" and not "right on every future
shopper". Nothing here transfers to real shoppers, another catalog or another
model.
:::

Round 2 is a separate run with its own frozen inputs; none of it is folded into
round 1 above. The full record is
[bench/round2/README.md](https://github.com/nrynss/memento-emptor/blob/main/bench/round2/README.md).
Every count below links its committed file under `bench/round2/runs/` in the
repository: the scores, the summary, the seed log, the recall comparison, or the
run file it came from.

- **Eight shoppers, two per case**, each built to separate the arms for a stated
  reason. A case measures only what its two histories carry.
- **Two repeats per shopper per arm.** A stable answer shows as the same size
  twice; a lucky one shows once.
- **One instrument, one model, one prompt template.** `pilot-baseline` on
  `claude-sonnet-4-6` in every arm; the four prompts differ only in the memory
  block. No arm read the hidden answers: the grader scored each turn against
  them after the fact.
- **The Memento layer ran live, over a seeded population.** The pool is the
  121-row seeded population, not real crowd traffic ([seed log][seed-log]).
- **Arm 2's input is PayPal-shaped.** It gets exactly the T10.1 extraction of the
  shopper's toolkit records, and nothing the layer produced.

The arms keep the same agent, model and catalog, so the memory condition is the
only difference:

| # | Condition |
|---|---|
| 1 | **No memory**: nothing about past purchases |
| 2 | **PayPal history**: exactly what the Agent Toolkit returns for this shopper — transactions, line items, refund and dispute status, dispute reason codes — and nothing the layer produced. No refund reasons, no classification, no signal strength |
| 3 | **Memento, shopper only**: the shopper's own reasoned evidence, labels and corrections, with no pool |
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

The run is [112 turns][summary] in all: [64][scores] first turns plus [48][scores]
later turns in case 4. [Five driver invocations][run-note] each booted a fresh
hub and replayed the frozen histories, so a later turn was served by a fresh
seed, not by a layer that kept running. No correction crosses an invocation boundary. The
other-agent turn is a fresh neutral-instrument conversation, not the deployed
second agent; it measures the structural property the second agent demonstrates.
[0 of 112][summary] turns went over cap. [7][summary] carry round 1's
contamination flag, every hit this turn's own live layer id, and all seven are
graded.

### Right size first time

Mechanical rule: the extracted size equals the hidden answer. Each cell is two
shoppers × two repeats = [4 turns][scores]. The pairs are
`r2-new-plain`/`r2-new-otherbrand`, `r2-crowd-rival`/`r2-crowd-plain`,
`r2-noisy-up`/`r2-noisy-down` and `r2-corr-belief`/`r2-corr-standing`.

| Case | Arm 1 no memory | Arm 2 PayPal history | Arm 3 Memento shopper only | Arm 4 Memento full |
|---|---|---|---|---|
| New to the brand | [0/4][scores] | [0/4][scores] | [0/4][scores] | [3/4][scores] |
| Unlike the crowd | [0/4][scores] | [2/4][scores] | [4/4][scores] | [2/4][scores] |
| Noisy refunds | [0/4][scores] | [0/4][scores] | [2/4][scores] | [4/4][scores] |
| Correction, first turn | [0/4][scores] | [2/4][scores] | [4/4][scores] | [3/4][scores] |
| **All four cases** | **[0/16][scores]** | **[4/16][scores]** | **[10/16][scores]** | **[12/16][scores]** |

What the counts carry:

- **Arm 1 named no size in [16 of 16][scores] first turns.** Its 0/16 is 16
  refusals, not 16 wrong sizes; the commonest opening is "no sizing information
  … was provided" ([example][run-a1-new-plain]).
- **Arm 2 named a size in [16 of 16][scores] first turns**, M on [14][scores] and
  S on [2][scores]; [4 of 16][scores] are right. The 4 are two things, two
  repeats each: `r2-corr-standing`'s first answer M, the one shopper whose
  answer is in PayPal's records alone through its kept M, and `r2-crowd-plain`'s
  S, where the reference reasoner holds no answer because the kept tally ties
  and the arm picked the more recent unreturned purchase
  ([run][run-a2-crowd-plain]).
- **Arm 3's 0/4 in new-to-brand is the statement deciding**, as frozen: the
  shopper-only arm answers the usual size M on a brand it has never bought, and
  the hidden answer is L. The four runs are
  [r2-a3-new-plain-r1][run-a3-new-plain] and its siblings.
- **Arm 4's new-to-brand [3/4][scores] is [4/4][run-a4-new-plain-r1] by hand**:
  r1's [r2-a4-new-plain-r1][run-a4-new-plain-r1] opens with L and is one of the
  six artifact turns; [r2][run-a4-new-plain-r2] and the two
  [r2-a4-new-otherbrand][run-a4-new-otherbrand-r1] turns are right by the rule
  as well.
- **Arm 4's unlike-the-crowd [2/4][scores] is [4/4][run-a4-crowd-rival-r1] by
  hand**: both [r2-a4-crowd-rival][run-a4-crowd-rival-r1] replies open with S
  and both are artifacts; `r2-a4-crowd-plain` is right twice by the rule.

Where the arms match: arms 1 and 2 are both 0/4 on new-to-brand and noisy
refunds. Arms 3 and 4 are both 4/4 on unlike-the-crowd by hand (arm 4
[2/4][scores] committed) and both 4/4 on noisy refunds by hand (arm 3
[2/4][scores] committed). Arm 4 is the only arm that got new-to-brand right
([3/4][scores] committed, [4/4][run-a4-new-plain-r1] by hand); every other arm
is [0/4][scores] there.

**Wrong-cause errors:** [0 in all 112 turns][scores], in every arm and every
case. The measure did not separate the arms.

**Citations are a limit of the measure:** the rule resolves only the shopper's
own PayPal link ids, so only arm 2 can offer a cite it resolves —
[16 cited, 4 supported][scores] across its [28 turns][scores]. Arms 1, 3 and 4
cite nothing under this rule. That cannot separate the Memento arms, because their
memory carries belief and evidence ids, not link ids.

### Corrections

Case 4 only: two shoppers, one correction each, recorded through the layer's
`record_correction` after the first turn and before every later turn. The
corrections are "Actually I have lost a size since then, use M now."
(`r2-corr-belief`, L → M) and "I was measured last week, take S now."
(`r2-corr-standing`, M → S). Each row is two shoppers × two repeats =
[4 turns][scores] per arm.

| Turn | Arm 1 no memory | Arm 2 PayPal history | Arm 3 shopper only | Arm 4 full |
|---|---|---|---|---|
| Next turn, same conversation | [4/4][scores] | [4/4][scores] | [4/4][scores] | [4/4][scores] |
| Next session, no conversation | [0/4][scores] | [2/4][scores] | [4/4][scores] | [4/4][scores] |
| Another agent, no conversation | [0/4][scores] | [2/4][scores] | [4/4][scores] | [4/4][scores] |
| **All three later turns** | **[4/12][scores]** | **[8/12][scores]** | **[12/12][scores]** | **[12/12][scores]** |

- **The correction turn is 4/4 in every arm, and it measures the conversation,
  not memory.** Its prompt carries the arm's first reply and the shopper's
  correction ([example][run-a1-corr]); every arm follows a stated instruction it
  can see. Arm 1's 4/4 there is the clearest case: with no memory it answers the
  corrected size on that turn, then refuses in the next session.
- **The next session and the other agent are where memory separates the arms.**
  Arms 3 and 4 are [4/4][scores] and [4/4][scores] on both; arm 1 is
  [0/4][scores] and [0/4][scores] (four refusals in each); arm 2 is
  [2/4][scores] and [2/4][scores].
- **Arm 2's 2/4 is not the correction travelling.** It answers M for both
  shoppers in every later turn. That is right for `r2-corr-belief`, whose
  corrected size is M, and wrong for `r2-corr-standing`, whose corrected size is
  S. Its correction turn did say S
  ([r2-a2-corr-standing-r1-correction][run-a2-corr-correction]), and the next
  session says M ([r2-a2-corr-standing-r1-next-session][run-a2-corr-next]) — the
  correction was in the conversation and gone in the next session.
- **The belief survives the correction in both Memento arms.** On
  `r2-corr-belief` the Venerable belief stands contradicted beside the
  correction rather than deleted; on `r2-corr-standing` the statement stands
  contradicted and the correction decides on its own. Every post-correction
  memory response carries the contradicted belief beside the correction, and
  [r2-corrections.json][corrections] records the named belief as
  `"status": "Out of true"` with the correction at `"takes_precedence": true`.

### Spend

[Spend][summary] was $0.789309 of the $5.00 cap. The same table carries the word
counts, so the cost of arm 2's PayPal-shaped prompt is visible beside its size:

| Arm | Turns | Words, all turns | Spend |
|---|---|---|---|
| 1 no memory | [28][scores] | [2589][scores] | [$0.031134][summary] |
| 2 PayPal history | [28][scores] | [28197][scores] | [$0.460746][summary] |
| 3 shopper only | [28][scores] | [9482][scores] | [$0.137073][summary] |
| 4 full | [28][scores] | [11263][scores] | [$0.160356][summary] |

Arm 2's prompt is dominated by the extracted records: its `r2-crowd-rival`
extraction is the largest in the set, and that turn's prompt is the largest at
[2228 words][run-a2-crowd-rival]. Token figures are words, not model tokens;
spend is the platform's own trace cost, reported beside them and never mixed in.

### The extraction artifact

The frozen size rule is round 1's, unchanged: the first `size X` mention in the
reply, then the first standalone size token, then none. Six Memento replies open
with the recommendation but later name another size — the size that came back
("returned the … Shirt in size M") or the size the pool reports — so the rule
reads that size instead of the recommendation. All six are first turns:

| Run | Reply opens with | Frozen rule read | Hidden answer |
|---|---|---|---|
| [r2-a3-noisy-up-r1-first][run-a3-noisy-up-r1] | L | M | L |
| [r2-a3-noisy-up-r2-first][run-a3-noisy-up-r2] | L | M | L |
| [r2-a4-crowd-rival-r1-first][run-a4-crowd-rival-r1] | S | M | S |
| [r2-a4-crowd-rival-r2-first][run-a4-crowd-rival-r2] | S | M | S |
| [r2-a4-new-plain-r1-first][run-a4-new-plain-r1] | L | M | L |
| [r2-a4-corr-belief-r2-first][run-a4-corr-belief-r2] | L | M | L |

No human override was recorded on any of them, so the committed scores are the
frozen rule's. Beside them, a hand reading of the same [64][scores] replies is
labelled as a reading, not a measurement: no score is overridden and none is
changed.

| Reading | Arm 3 shopper only | Arm 4 full |
|---|---|---|
| Committed scores | [10/16][scores] | [12/16][scores] |
| Hand reading (not a score) | [12/16][run-a3-noisy-up-r1] | [16/16][run-a4-crowd-rival-r1] |

The hand reading adds exactly the six turns above. Per case, arm 3's
noisy-refunds [2/4][scores] becomes [4/4][run-a3-noisy-up-r1], and arm 4's
new-to-brand [3/4][scores], unlike-the-crowd [2/4][scores] and correction
[3/4][scores] each become [4/4][run-a4-crowd-rival-r1] by hand. Arm 3's
new-to-brand stays [0/4][scores] under either reading. The committed first-turn
counts therefore understate the Memento arms by [2][scores] and [4][scores]
turns respectively; both are shown so a reader can see the difference.

### The recall difference, in one sentence

[28 of 56][recall-compare] Memento turns differ from the freeze's expected
`recall_for_purchase` response in belief status and standing labels,
contradiction marks, and which of the rival's returns the belief carries, while
the answers and the deciding precedence step agree.

### The headline: right on day one for a new shopper

The project's headline is **right on day one for a new shopper**. The pool answers
the new-to-brand case that no shopper's history can:
[3/4 by the committed scores][scores] ([4/4 by hand][run-a4-new-plain-r1]),
against [0/4 for Memento shopper-only][scores],
[0/4 for PayPal history][scores] and [four refusals for no memory][scores].
Correction carrying to the next session and to another agent,
[4/4 and 4/4][scores] in both Memento arms, is the supporting point.

What it does not show: case 1 is constructed so the pool is the only input that
holds the answer, two shoppers share one product line, and the pool is the
seeded population rather than live traffic. It shows the mechanism, not
real-world accuracy. The decision and its limits are recorded in
[product.md](https://github.com/nrynss/memento-emptor/blob/main/product.md), and
`bench/round2/README.md` keeps its four candidates as the frozen record of the
run.

### The other three candidates

:::note[These are measured results, not a second choice]
Each stays a candidate the project could use. The exact counts are linked, and
each carries its limit.
:::

1. **The correction crosses sessions and agents for both Memento arms:
   [4/4 in the next session and 4/4 for another agent][scores], where
   [PayPal history is 2/4 and 2/4][scores] and
   [no memory is 0/4 and 0/4][scores].** The correction turn itself is
   [4/4][scores] in every arm, which is what makes the later turns the measure
   of memory rather than instruction-following. Proves the layer carries a
   stated correction to an agent that never saw the conversation. Does not prove
   the deployed second agent would, or that a correction to anything but size
   travels (arm 2's 2/4 is a coincidence: it repeats its own M, right for one
   shopper and wrong for the other).
2. **Own evidence beats a wrong crowd in both Memento arms:
   [4/4 + 4/4 by hand][run-a3-crowd-rival] ([4/4 + 2/4 committed][scores]),
   where the pool alone points the other way and is wrong.** The runs are the
   two repeats of [r2-a3-crowd-rival][run-a3-crowd-rival],
   [r2-a3-crowd-plain][run-a3-crowd-plain],
   [r2-a4-crowd-rival][run-a4-crowd-rival-r1] and
   [r2-a4-crowd-plain][run-a4-crowd-plain]; `r2-crowd-plain` is the shopper whose
   own evidence carries it past the pool. Arm 2 is [2/4][scores] (crowd-plain
   only), arm 1 [0/4][scores]. The pool is Earned small at M, so a pool-only
   answer is L and wrong. Does not prove the rival-birth threshold on other
   histories: one shopper depends on it, and the pool's "wrong" is constructed,
   not observed.
3. **PayPal history costs [2.9 times what Memento full costs][summary] and
   answers [4 of 16 first turns against 12 of 16][scores].** The same files give
   [28197 words against 11263][scores]. Proves the input size difference is not
   free in this comparison. Does not prove a general cost ratio: the
   extraction's size is a property of the generated record shape, and the model
   and instrument are single.

[scores]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-scores.json
[summary]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-summary.json
[run-note]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/RUN-NOTE.md
[seed-log]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-seed-log.json
[recall-compare]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-recall-compare.json
[corrections]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-corrections.json
[run-a1-new-plain]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a1-new-plain-r1-first.json
[run-a1-corr]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a1-corr-belief-r1-correction.json
[run-a2-crowd-plain]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a2-crowd-plain-r1-first.json
[run-a2-crowd-rival]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a2-crowd-rival-r1-first.json
[run-a2-corr-correction]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a2-corr-standing-r1-correction.json
[run-a2-corr-next]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a2-corr-standing-r1-next-session.json
[run-a3-new-plain]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a3-new-plain-r1-first.json
[run-a3-noisy-up-r1]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a3-noisy-up-r1-first.json
[run-a3-noisy-up-r2]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a3-noisy-up-r2-first.json
[run-a3-crowd-rival]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a3-crowd-rival-r1-first.json
[run-a3-crowd-plain]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a3-crowd-plain-r1-first.json
[run-a4-crowd-rival-r1]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a4-crowd-rival-r1-first.json
[run-a4-crowd-rival-r2]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a4-crowd-rival-r2-first.json
[run-a4-crowd-plain]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a4-crowd-plain-r1-first.json
[run-a4-new-plain-r1]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a4-new-plain-r1-first.json
[run-a4-new-plain-r2]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a4-new-plain-r2-first.json
[run-a4-new-otherbrand-r1]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a4-new-otherbrand-r1-first.json
[run-a4-corr-belief-r2]: https://github.com/nrynss/memento-emptor/blob/main/bench/round2/runs/r2-a4-corr-belief-r2-first.json
