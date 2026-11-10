# Comparison results: no memory vs Memento evidence without beliefs vs Memento Emptor

## Limits — read this first

We wrote both the shoppers and the hidden answers. This comparison shows the
mechanism on constructed cases, not real-world accuracy. Five fictional
shoppers (`cmp-broken`, `cmp-changed-mind`, `cmp-never-arrived`,
`cmp-size-changed`, `cmp-unclear`), one hidden right size each, three live
turns per shopper per arm. A count like 15/15 means "right on all 15 frozen
turns", not "right on every future shopper". Nothing here transfers to
real shoppers, real catalogs, or any other model.

No arm read the hidden answers. The grader (`bench/grader/`) scored each run
against them after the fact. No hidden-answer rationale text is quoted
anywhere in this document — only the already-published right/wrong tallies.

## Round 1, relabelled

The round 1 arm this document called "raw history" is **Memento evidence
without beliefs**. Its prompt carried the layer's evidence rows, so every row
already held the classified reason, the signal strength and the write kind;
only the beliefs built over those rows were absent. Calling that "raw history"
overstated what the arm was given and made the baseline's score look like it
came from transaction data alone. Every mention here uses the honest name; the
runs, scores and freeze inputs it reports are unchanged. The committed run
files keep `arm_name: "raw-history"` and the prompt's "Memory: raw history"
line, because they are frozen T5.5 output; the label here is the report's, not
theirs.

Round 2 is the comparison `product.md` now asks for: four arms, including a
PayPal history arm that gets exactly what the Agent Toolkit returns and nothing
the layer made, against Memento shopper-only and Memento full. Its tasks and
freeze live in `dev-diary/PHASE-10-comparison-round-2.md`; this document
remains the round 1 record.

## Setup

- Frozen inputs: `bench/FROZEN.md` (25 sha256 rows: the 9 arm-facing
  histories — 5 round 1 shoppers and the 4 the T5.7 hardening added — plus
  `demo-shopper.json`, hidden answers by hash only, seed generator, harness
  prompts, grader, pilot extraction rule + run schema + driver/grade commands,
  neutral instrument).
- Arms: 1 = no memory, 2 = Memento evidence without beliefs (the layer's
  evidence rows handed to the agent directly: classified reason, signal
  strength and write kind, no beliefs over them),
  3 = Memento layer recall (`recall_for_purchase` + `pending_questions`,
  live layer, real `recall_for_purchase` responses recorded in each run file).
- Repeats: 3 live turns per shopper per arm, 45 turns total, all committed
  under `bench/runs/` in the frozen `RunFile` schema with prompt, memory
  fetch, reply, extraction, and score.
- Instrument: the neutral `pilot-baseline` deployment for all three arms (the
  reference agent is rejected as driver per the freeze record); model
  `claude-sonnet-4-6` in every arm.
- Token basis: `harness-words-v1` (whitespace words over prompt + reply,
  recorded on every score). Platform trace cost is reported beside it, never
  mixed into it.
- Caps: 4000 tokens and $0.05 per run; 0 of the 45 runs went over cap, 0 were
  contaminated. Live spend for the 45 runs was recorded by T5.5 ($0.082 of the
  $5.00 cumulative budget, including one overwritten $0.0018 shakedown turn).
  This document (T5.6) is offline: no keys, no network, no model turns,
  no spend.
- The 45-turn set's seed histories carry no corrections (pinned by
  `TestSeedHistoriesCarryNoCorrections`), so its correction measure is nil
  across all three arms — stated plainly in the measures section, not
  manufactured. The separate correction round T5.9 ran afterwards and its
  counts are in the Corrections section.

## Headline counts

| Arm | Right size first time | Wrong-cause errors | Corrections honoured | Supported / cited | Tokens (words, 15 runs) |
|---|---|---|---|---|---|
| 1 — no memory | 0 of 15 | 0 | nil (no corrections in set) | 0 of 0 | 1309 total, 83–97 per run |
| 2 — Memento evidence without beliefs | 15 of 15 | 0 | nil (no corrections in set) | 6 of 40 | 3606 total, 216–289 per run |
| 3 — Memento layer | 0 of 15 true recommendations (3 of 15 graded, all 3 an extractor artifact — see below) | 0 | nil (no corrections in set) | 0 of 0 | 2242 total, 130–215 per run |

Arm 2 beats arm 3 on right-size recommendations on this frozen set: 15 of 15
against 0 of 15 true recommendations. That is stated plainly and not spun:
on these constructed cases, the Memento evidence without beliefs arm got the
size right every time, while the layer refused to recommend without Earned
belief.
Arm 2 matches arm 3 on wrong-cause errors: 0 in both arms. Arm 1 got 0 of 15
right (12 empty replies, 3 wrong-size M recommendations on
`cmp-never-arrived` against the hidden L).

The correction column's `nil` is the 45-turn frozen set only: those seed
histories contain no corrections. T5.9 ran one correction turn in every cell
afterwards; its counts are under Corrections.

## Measure by measure

### 1. Right size first time (mechanical: recommended size equals hidden answer)

- Arm 1: 0 of 15. Runs `bench/runs/t55-a1-*.json` (trace id = run id in each
  file). Twelve turns recommended no size at all; the three
  `t55-a1-never-arrived-r{1,2,3}.json` turns recommended M against the hidden
  L.
- Arm 2: 15 of 15. Runs `bench/runs/t55-a2-*.json`:
  `t55-a2-broken-r{1,2,3}.json`, `t55-a2-changed-mind-r{1,2,3}.json`,
  `t55-a2-never-arrived-r{1,2,3}.json`, `t55-a2-size-changed-r{1,2,3}.json`,
  `t55-a2-unclear-r{1,2,3}.json` — every turn names the hidden size.
- Arm 3: 0 of 15 true recommendations. Twelve turns are outright refusals
  (empty extracted size, no recommendation): `t55-a3-broken-r{1,2,3}.json`,
  `t55-a3-changed-mind-r{1,2,3}.json`,
  `t55-a3-never-arrived-r{1,2,3}.json`,
  `t55-a3-size-changed-r{1,2,3}.json` — each reply states the layer returned
  no beliefs and declines to recommend. The remaining three,
  `t55-a3-unclear-r{1,2,3}.json`, are informed refusals: each reply says it
  cannot make a confident recommendation because the only signal is a refund
  of unknown reason (the layer returned zero beliefs with pending count 1).
  The frozen extraction rule has no override-to-empty, so the deterministic
  extractor picked the M named inside the refusal prose and the grader scored
  3 of 15 — an extractor artifact, recorded in each run file's
  `extraction.notes`, not a recommendation the arm made. True arm-3
  recommendations: 0 of 15.

### 2. Wrong-cause errors (mechanical: a cited non-fit row behind a size that moves off the kept baseline)

0 in all three arms, across all 45 runs (`bench/runs/scores.json`,
`wrong_cause` is 0 on every row). Arm 2 matches arm 3 here: neither arm ever
moved the size off baseline on the back of a damage, never-arrived,
changed-mind, or vague row. Arm 2's exclusion behavior is visible in the
cites: its `cmp-unclear` turns cite non-fit rows while holding M (the
baseline), which the grader counts as a legitimate exclusion, not an error
(`t55-a2-unclear-r{1,2,3}.json`, 3 cites each, 0 supported, 0 wrong-cause).

### 3. Corrections honoured on the very next recommendation

Nil for the 45 frozen turns: every score in `bench/runs/scores.json` carries
`correction_honoured: null`, because those seed histories contain no
corrections. It is nil, not 0, and no correction behaviour is claimed for any
arm from those runs. The correction round T5.9 measured this separately; its
counts are in Corrections below.

### 4. Citations that really support the recommendation (mechanical: cite resolves to exactly one history row that counts toward fit)

- Arm 1: 0 cited, 0 supported (15 runs, `bench/runs/t55-a1-*.json`).
- Arm 2: 40 cited, 6 supported. All 6 supported cites sit in the three
  `cmp-size-changed` turns (`t55-a2-size-changed-r{1,2,3}.json`, 2 of 2 each:
  the two fit-return note substrings). The other 34 cites resolve to non-fit
  rows (damage, never-arrived, changed-mind, kept, general statements) via
  human prose-mapping and do not support a size recommendation — including 9
  cites across `t55-a2-unclear-r{1,2,3}.json` that resolve but count toward
  nothing fit-relevant. The pilot already flagged this shape: arm-2 cites
  need human prose-mapping and mostly land on non-fit rows.
- Arm 3: 0 cited, 0 supported (15 runs, `bench/runs/t55-a3-*.json`). An arm
  that refuses to recommend offers no cites; there is nothing to grade as
  supported or misleading.

### 5. Tokens per recommendation (mechanical: prompt + reply words, basis `harness-words-v1`)

- Arm 1: 1309 words over 15 runs, 83–97 per run.
- Arm 2: 3606 words over 15 runs, 216–289 per run.
- Arm 3: 2242 words over 15 runs, 130–215 per run (the 3 longer turns are the
  `t55-a3-unclear-r{1,2,3}.json` informed refusals at 193–215 words; the 12
  outright refusals run 130–141 words).

Per-run figures are the `tokens_first` column of `bench/runs/scores.json`;
run files carry `prompt_words` + `reply_words` for the same turn. The ~5x
live-BPE factor from the freeze record applies equally to all arms, so
cross-arm token comparisons hold on the frozen basis.

## Corrections

T5.9 added one correction turn to every cell after the T5.5 and T5.8 sweeps: 9
shoppers × 3 arms = 27 turns. Each reuses the published first turn
(`source_run_id` in the run file), then the shopper names a different size than
the arm just gave ("Actually I take L now", or M where the arm had already
named L, so the correction always contradicts), then one re-ask. Run files and
grades: `bench/runs/t59/`, scores in `bench/runs/t59/t59-scores.json`.

- Honoured on the next recommendation: 27 of 27, 9 of 9 in every arm.
- Wrong-cause errors: 0 across all 27 turns.
- Arm 3 routed the correction through the real `record_correction` path in 2
  of 9 cells (`t59-a3-noisy-large.json`, `t59-a3-noisy-small.json`): the
  Venerable belief was kept standing with its evidence, the correction
  recorded beside it as the override, and the next guidance given on the
  correction's basis. In the other 7 of 9 arm-3 cells recall served no belief
  before or after, so there was no `record_correction` call to make and the
  correction travelled in conversation text only — the same path arms 1 and 2
  have. Each of those 7 arm-3 run files records `correction_recorded: false`
  and the reason.

The mechanism difference is real; the outcome difference is not. Arms 1 and 2
have no graph, so the correction exists only as text in the conversation and
the next turn honours it because that text is still in the prompt. Arm 3 can
contradict a belief and keep it, and did so in 2 of 9 cells. Every arm honoured
every correction, so corrections did not discriminate the arms: this round
shows a stated preference is followed on the next turn, not that any arm
handles corrections better.

## What this does and does not prove

Proves on these constructed cases: the refusal mechanism works as designed —
with no Earned belief the layer says "no recommendation yet" instead of
guessing, and it never once converted a non-fit outcome into a size move
(0 wrong-cause in 15 arm-3 turns). Also proves: Memento evidence without
beliefs is a strong baseline on small, clean histories — arm 2 reasoned
correctly about damage, never-arrived, changed-mind, and vague rows 15 times
out of 15 with 0 wrong-cause.

Does not prove: real-world accuracy (we wrote the shoppers and the answers);
correction handling beyond one stated correction on the next turn (T5.9's 27
turns are in Corrections; a correction that changes something other than a
size, or arrives mid-conversation, is unmeasured); citation quality at scale
(arm-2 cites are mostly human-mapped prose, and 34 of 40 resolve to non-fit
rows); behavior on long histories where signal drowns in noise or on
multi-brand histories (recorded as hardening candidates in the freeze, not
built); anything about cost beyond the recorded trace spend.

## Candidate lead numbers (the user picks one — none is chosen here)

1. **"15 of 15 vs 0 of 15: Memento evidence without beliefs got the size right every time; the layer refused rather than guess."** Exact counts: arm 2 right-size 15 of 15 runs (`bench/runs/t55-a2-*.json`); arm 3 true recommendations 0 of 15 (`bench/runs/t55-a3-*.json`, 12 outright + 3 informed refusals). Proves the baseline is strong on clean histories and the layer is conservative by design. Does not prove the layer is better — on this set it is worse at naming sizes.
2. **"0 wrong-cause errors in 45 runs — neither arm ever blamed the wrong reason for a size."** Exact counts: `wrong_cause` 0 on all 45 rows of `bench/runs/scores.json` (15 per arm). Proves the causal-exclusion behavior held for both memory conditions on these cases. Does not prove it holds on noisy or multi-brand histories.
3. **"6 of 40 cites from Memento evidence without beliefs actually support the size; the layer's 15 turns cite nothing instead of citing loosely."** Exact counts: arm 2 `supported_cites` 6 of `cited` 40 in `bench/runs/scores.json` (all 6 in `t55-a2-size-changed-r{1,2,3}.json`); arm 3 cited 0 across all 15 runs (12 outright + 3 informed refusals). Proves arm-2 justifications mostly lean on non-fit rows even when the size is right, while the layer withholds rather than over-cites. Does not prove shoppers prefer refusal — that is a product judgement, not a measured one.

## LEAD NUMBER: [user picks]

The user chooses which number the video leads with, after seeing these
results. This document is complete with the candidates above marked but
unchosen.

## Reproduce (offline)

From the repository root, no keys, no network, no spend:

```sh
python3 -c "
import json
s = json.load(open('bench/runs/scores.json'))
by = {}
for r in s['runs']:
    by.setdefault((r['arm'], r['shopper_id']), []).append(r)
for (arm, shop) in sorted(by):
    rs = by[(arm, shop)]
    print(arm, shop,
          'right:', sum(1 for r in rs if r['right_size_first']),
          'wrong:', sum(r['wrong_cause'] for r in rs),
          'cited:', sum(r['cited'] for r in rs),
          'supported:', sum(r['supported_cites'] for r in rs))
"
```

Frozen hashes still verify via `bench/freeze_check.sh` (run from
`./tools/check.sh`); the 45 run files it does not cover are the T5.5-published
measurement this document reports.
