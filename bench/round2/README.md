# Round 2 results: four arms, four constructed cases, as counts

This is the record of T10.4's run: every shopper through every arm, two repeats, plus case 4's
correction turns. The counts below are read from the committed run files and the scores built from
them:

- `bench/round2/runs/` — 112 run files, one per turn.
- `bench/round2/runs/r2-scores.json` — the frozen `round2.Score` for each of the 112 turns.
- `bench/round2/runs/r2-summary.json` — turns, spend, contamination and the recall-comparison ids.
- `bench/round2/runs/r2-recall-compare.json` — the live recall against the frozen expectation.
- `bench/round2/runs/r2-corrections.json` — the case-4 corrections as the layer recorded them.
- `bench/round2/runs/r2-seed-log.json` — what each frozen history filed in the live layer.
- `bench/round2/runs/RUN-NOTE.md` — what ran, in what order, and the deviations.

The frozen inputs and the rules are in `bench/round2/FROZEN.md`; the schedule is
`dev-diary/PHASE-10-comparison-round-2.md`. The round 1 record is `bench/README.md` and is not
changed by this document.

## Limits — read this first

We wrote the shoppers, the hidden answers, the population, the prompts and the grader. This shows
the mechanism on eight constructed shoppers in four cases, not real-world accuracy. A count like
12/16 means "right on 12 of these 16 frozen turns", not "right on 12 of 16 shoppers" and not "right
on every future shopper". Nothing here transfers to real shoppers, another catalog or another model.

- **Eight shoppers, two per case, each built to separate the arms for a stated reason** (case
  statements: `bench/round2/cases.go`; the frozen reasoning: `bench/round2/FROZEN.md` §3). A case
  measures only what its two histories carry; a behaviour none of them exercises is unmeasured.
- **Two repeats per shopper per arm.** A stable answer shows as the same size twice; a lucky one
  shows once.
- **One instrument, one model, one prompt template.** `pilot-baseline` on `claude-sonnet-4-6` in
  every arm (RUN-NOTE, "What ran"); the four prompts differ only in the memory block
  (`bench/round2/prompt.go`). No arm read the hidden answers: the grader
  (`bench/round2/grade.go`) scored each turn against `bench/round2/hidden/answers.json` after the
  fact.
- **The Memento layer ran live, over a seeded population.** Each Memento run file records the
  `recall_for_purchase` response its turn actually saw. The pool is the 121-row seeded population,
  not real crowd traffic (`bench/round2/runs/RUN-NOTE.md`, "Population";
  `bench/round2/runs/r2-seed-log.json`; FROZEN §4 and §10).
- **Arm 2's input is PayPal-shaped.** It gets exactly the T10.1 extraction of the shopper's toolkit
  records, and nothing the layer produced.

## The extraction artifact, before any count

The frozen size rule is round 1's, unchanged (`bench/pilot/pilot.go`): the first `size X` mention
in the reply, then the first standalone size token, then none. Six Memento replies open with the
recommendation but later name another size — the size that came back ("returned the … Shirt in
size M") or the size the pool reports — so the rule reads that size instead of the recommendation.
All six are first turns:

| Run | Reply opens with | Frozen rule read | Hidden answer |
|---|---|---|---|
| `bench/round2/runs/r2-a3-noisy-up-r1-first.json` | L | M | L |
| `bench/round2/runs/r2-a3-noisy-up-r2-first.json` | L | M | L |
| `bench/round2/runs/r2-a4-crowd-rival-r1-first.json` | S | M | S |
| `bench/round2/runs/r2-a4-crowd-rival-r2-first.json` | S | M | S |
| `bench/round2/runs/r2-a4-new-plain-r1-first.json` | L | M | L |
| `bench/round2/runs/r2-a4-corr-belief-r2-first.json` | L | M | L |

No human override was recorded on any of them, so the committed scores are the frozen rule's:

- **Committed, measured: arm 3 10/16 first turns, arm 4 12/16 first turns**
  (`bench/round2/runs/r2-scores.json`).
- **A hand reading of the same 64 replies, beside the scores: arm 3 12/16, arm 4 16/16.** This is a
  reading, not the measurement; no score is overridden and none is changed. It adds exactly the six
  turns above: arm 3's two `r2-noisy-up` turns and arm 4's two `r2-crowd-rival` turns, one
  `r2-new-plain` turn and one `r2-corr-belief` turn.

The rest of this document quotes the committed counts and says where the hand reading differs. The
per-case effect of the reading is four cells: arm 3's noisy-refunds 2/4 becomes 4/4, and arm 4's
new-to-brand 3/4, unlike-the-crowd 2/4 and correction 3/4 each become 4/4. Arm 3's new-to-brand
stays 0/4 under either reading.

## What ran, and the shape of the run

- **Turns.** Eight shoppers × four arms × two repeats = 64 first turns, plus case 4's
  2 shoppers × 4 arms × 2 repeats × 3 later turns = 48. **112 turns in all**
  (`bench/round2/runs/r2-summary.json`, `turns: 112`).
- **Five driver invocations.** The one-cell check ran first (`r2-crowd-rival`, four arms, repeat 1,
  four turns, $0.057150 in RUN-NOTE, "Spend"), then four batch invocations. Every invocation booted
  a fresh hub,
  regenerated the 121-row population and replayed the frozen histories, so a turn committed later
  was served by a fresh seed of the same frozen inputs, not by a layer that had kept running
  (RUN-NOTE, "What ran", which groups the seeded event times and the correction times into those
  four batch instances).
- **No correction crosses an invocation boundary.** Every invocation that reached case 4 records
  the correction again after its first turns and before its later turns; each of the 24
  post-correction Memento turns carries the correction in its own `memory_response` with
  `takes_precedence: true`. `r2-corrections.json` holds the final invocation's calls, and its
  `recorded_at` is later than the correction memory an earlier turn committed with — so the file
  is not the source of any turn's memory; the run file is.
- **The other-agent turn is a fresh neutral-instrument conversation, not the deployed second
  agent.** The deployed `memento-second` is keyed to the round-1 demo shopper and cannot address a
  round-2 shopper without minting keys and redeploying, which T10.4's brief forbids. The turn
  measures the structural property T9.5's second agent demonstrates — another agent, no shared
  conversation, the same memory — and the run file labels its agent B (RUN-NOTE, "Limits and
  deviations").
- **Three early stops, all recorded.** Two message POSTs answered 502 and were re-sent after a
  fresh seed (`r2-a3-corr-standing-r1-first`, `r2-a2-corr-standing-r1-other-agent`). One paid turn's
  trace arrived late and was recovered from the platform and committed (`r2-a1-noisy-up-r2-first`,
  $0.001080) rather than re-sent. The failed sends carry no reply and are not scored.
- **Caps.** 0 of 112 turns over cap (`r2-summary.json`, `capped: 0`; `over_cap` false on every
  score).
- **Contamination.** 7 turns are flagged by the round-1 string rule, whose demo-shopper tokens
  include `corr_` and `blf_` (`r2-summary.json`, `contaminated: 7`):
  `r2-a3-corr-belief-r1-next-session`,
  `r2-a3-corr-belief-r2-next-session`, `r2-a3-corr-belief-r2-other-agent`,
  `r2-a4-corr-belief-r1-next-session`, `r2-a4-corr-standing-r1-next-session`,
  `r2-a4-corr-standing-r2-next-session`, `r2-a4-crowd-rival-r1-first`. Every hit is this turn's
  own live layer id (six `corr_`, one `blf_`), not another shopper's memory; the round-1 rule was
  written when those prefixes appeared only in demo memory. The seven are graded, with
  `contaminated: true` on the record.
- **Spend.** $0.789309 of the $5.00 cap (`r2-summary.json`): arm 1 $0.031134, arm 2 $0.460746,
  arm 3 $0.137073, arm 4 $0.160356. The one-cell projection was $1.6002 at the per-arm mean and
  $4.1130 at the priciest turn, both under the cap, so the batch ran (RUN-NOTE, "Spend").

## The live recall against the frozen expectation

**28 of 56 Memento turns differ from the freeze's expected `recall_for_purchase` response, in
belief status and standing labels, contradiction marks, and which of the rival's returns the
belief carries; the answers and the deciding precedence step agree.** The
other 28 match field for field, including every turn of `r2-new-plain`, `r2-new-otherbrand`,
`r2-crowd-plain`, `r2-noisy-up`, `r2-noisy-down`, and the first turns of `r2-corr-belief` and
`r2-corr-standing` — all of it in `bench/round2/runs/r2-recall-compare.json`
(`recall_turns_differing: 28` in `r2-summary.json`). Ids, absolute event times and the guidance
prose are excluded from the comparison; they are not part of the freeze (RUN-NOTE, "Differences
between the live recall and the frozen expectation").

The 28 fall in two groups:

1. **`r2-crowd-rival`, first turn — 4 runs (arms 3 and 4, both repeats).** The one-return
   runs-small belief is **Claimed/None** live where the freeze labels it **Building/Candidate**,
   and the rival's evidence is **three rows live, five in the fixture**. The fixture collects every
   same-direction return; the layer attaches only the rows that cite the rival — its birth row and
   the two after it — and the two pre-birth opposite returns stay unattached, as FROZEN §5
   describes. The rival's standing (Earned/Venerable), the answer S and the own-evidence step are
   the same. Runs: `bench/round2/runs/r2-a3-crowd-rival-r1-first.json`,
   `...-r2-first.json`, `bench/round2/runs/r2-a4-crowd-rival-r1-first.json`,
   `...-r2-first.json`.
2. **Case 4 after the correction — 24 runs (two shoppers × two Memento arms × two repeats × three
   later turns).** The live layer reads a contradicted belief's status as **Out of true** where the
   fixture keeps `Earned`/`Claimed` and carries `contradicted: true` beside it. The fixture marks
   every served belief contradicted when the correction wins; the layer marks only the belief the
   correction is declared against, so on `r2-corr-belief` the statement is not contradicted live.
   On **`r2-corr-standing`** the layer declares the correction against the shopper's statement
   (the only belief there), so the served override carries a `contradicts_belief_id` and the
   guidance names that belief, where the fixture has the correction stand alone. The corrected
   size and the correction-first precedence agree in both.

**Answers on the 28 differing turns.** The recommendation equals the hidden answer in 26 of the
28 by the committed scores, and in all 28 by the hand reading above: the two committed misses are
`r2-a4-crowd-rival-r1-first` and `-r2`, which open with S and are two of the six artifact runs.
**Precedence on the 28 differing turns.** In group 1 own evidence decides in both the live memory
and the expectation; in group 2 the correction decides in both (every belief's precedence entry
reads `overridden_by: correction`). Nothing in the differences moves a size.

## Right size first time

Mechanical rule: the extracted size equals the hidden answer. Each cell is 2 shoppers × 2 repeats
= 4 turns. The pairs are `r2-new-plain`/`r2-new-otherbrand`, `r2-crowd-rival`/`r2-crowd-plain`,
`r2-noisy-up`/`r2-noisy-down` and `r2-corr-belief`/`r2-corr-standing`; why each case separates the
arms is FROZEN §3 and `bench/round2/cases.go`. Committed scores,
`bench/round2/runs/r2-scores.json`:

| Case | Arm 1 no memory | Arm 2 PayPal history | Arm 3 Memento shopper only | Arm 4 Memento full |
|---|---|---|---|---|
| New to the brand | 0/4 | 0/4 | 0/4 | 3/4 |
| Unlike the crowd | 0/4 | 2/4 | 4/4 | 2/4 |
| Noisy refunds | 0/4 | 0/4 | 2/4 | 4/4 |
| Correction, first turn | 0/4 | 2/4 | 4/4 | 3/4 |
| **All four cases** | **0/16** | **4/16** | **10/16** | **12/16** |

A hand reading of the same 64 replies, kept beside the scores and not substituted for them:

| Case | Arm 1 | Arm 2 | Arm 3 | Arm 4 |
|---|---|---|---|---|
| New to the brand | 0/4 | 0/4 | 0/4 | 4/4 |
| Unlike the crowd | 0/4 | 2/4 | 4/4 | 4/4 |
| Noisy refunds | 0/4 | 0/4 | 4/4 | 4/4 |
| Correction, first turn | 0/4 | 2/4 | 4/4 | 4/4 |
| **All four cases** | **0/16** | **4/16** | **12/16** | **16/16** |

What the counts carry:

- **Arm 1 named no size at all in 16 of 16 first turns.** Its 0/16 is 16 refusals — the commonest
  opening is "no sizing information … was provided" — not 16 wrong sizes:
  `bench/round2/runs/r2-a1-*-first.json`.
- **Arm 2 named a size in 16 of 16 first turns**, M on 14 and S on 2; 4 of the 16 are right
  (`bench/round2/runs/r2-a2-*-first.json`). The 4 are two things, two repeats each:
  `r2-corr-standing`'s first answer M, the one shopper whose answer is in PayPal's records alone
  through its kept M (FROZEN §4), and `r2-crowd-plain`'s S, where the reference reasoner holds no
  answer because the kept tally ties, and the arm picked the more recent unreturned purchase
  (`bench/round2/runs/r2-a2-crowd-plain-r1-first.json`). Arm 2's 4/16 is therefore not four
  independently held answers.
- **Arm 3's 0/4 in new-to-brand is the statement deciding**, as frozen: the shopper-only arm
  answers the usual size M on a brand it has never bought, and the hidden answer is L (FROZEN §4,
  "a decoy, not an answer"). Its four case-1 runs are `bench/round2/runs/r2-a3-new-plain-r*-first.json`
  and `r2-a3-new-otherbrand-r*-first.json`.
- **Arm 4's new-to-brand 3/4 is 4/4 by hand**: `r2-a4-new-plain-r1-first` opens with L and is the
  artifact; `...-r2-first` and the two `r2-a4-new-otherbrand` turns are right by the rule as well.
- **Arm 4's unlike-the-crowd 2/4 is 4/4 by hand**: both `r2-a4-crowd-rival` replies open with S and
  both are artifacts; `r2-a4-crowd-plain` is right twice by the rule.

Where the arms match on this measure: arms 1 and 2 are both 0/4 on new-to-brand and on noisy
refunds. Arms 3 and 4 are both 4/4 on unlike-the-crowd (arm 4 by hand, 2/4 committed) and both
4/4 on noisy refunds by hand (arm 3 2/4 committed). Arm 4 is the only arm that got new-to-brand
right (3/4 committed, 4/4 hand); every other arm is 0/4 there, arm 2 answering the usual-size M
each time and arm 1 refusing each time.

## Corrections

Case 4 only: two shoppers, one correction each, recorded through the layer's `record_correction`
after the first turn and before every later turn. The correction texts are the frozen ones:
"Actually I have lost a size since then, use M now." (`r2-corr-belief`, L → M) and "I was measured
last week, take S now." (`r2-corr-standing`, M → S). Each row is 2 shoppers × 2 repeats = 4 turns
per arm. Counts from `bench/round2/runs/r2-scores.json`, `correction_honoured`:

| Turn | Arm 1 no memory | Arm 2 PayPal history | Arm 3 shopper only | Arm 4 full |
|---|---|---|---|---|
| Next turn, same conversation | 4/4 | 4/4 | 4/4 | 4/4 |
| Next session, no conversation | 0/4 | 2/4 | 4/4 | 4/4 |
| Another agent, no conversation | 0/4 | 2/4 | 4/4 | 4/4 |
| **All three later turns** | **4/12** | **8/12** | **12/12** | **12/12** |

- **The correction turn is 4/4 in every arm, and it measures the conversation, not memory.** That
  turn's prompt carries the arm's first reply and the shopper's correction
  (`bench/round2/runs/r2-a1-corr-belief-r1-correction.json` and its siblings); every arm follows a
  stated instruction it can see. Arm 1's 4/4 there is the clearest case: with no memory at all it
  answers the corrected size on the turn the correction is in front of it, then refuses in the next
  session.
- **The next session and the other agent are where memory separates the arms.** Arms 3 and 4 are
  4/4 and 4/4 on both; arm 1 is 0/4 and 0/4 (four refusals in each); arm 2 is 2/4 and 2/4.
- **Arm 2's 2/4 is not the correction travelling.** It answers M for both shoppers in every later
  turn. That is right for `r2-corr-belief`, whose corrected size is M — the same M it gave, wrongly,
  before the correction — and wrong for `r2-corr-standing`, whose corrected size is S, where it
  keeps saying M. Its correction turn did say S (`r2-a2-corr-standing-r1-correction.json`,
  `...-r2-correction.json`), so the correction was in the conversation and gone in the next
  session. Per shopper: `r2-corr-belief` 2/2 and 2/2, `r2-corr-standing` 0/2 and 0/2.
- **The belief survives the correction in both Memento arms.** On `r2-corr-belief` the Venerable
  belief stands contradicted beside the correction rather than deleted; on `r2-corr-standing` the
  statement stands contradicted and the correction decides on its own. Every post-correction
  `memory_response` carries the contradicted belief beside the correction, and
  `bench/round2/runs/r2-corrections.json`'s two responses record the named belief as
  `"status": "Out of true"` and the correction with `"takes_precedence": true`.

## Which precedence step decided

The frozen score records the precedence step the served memory put first (`rung`), read from the
live response (`bench/round2/runs/r2-scores.json`). It is empty for arms 1 and 2 in all 56 of their
turns: they have no precedence view at all. The first-turn counts, both Memento arms (the two arms
record the same live response, so the column is the same for both):

| Case | Recorded rung | Turns | Right by committed score | Right by hand |
|---|---|---|---|---|
| New to the brand | `pool` | 8 (4 per arm) | 3 | 4 |
| Unlike the crowd | `own_evidence` | 8 (4 per arm) | 6 | 8 |
| Noisy refunds | `own_evidence` | 8 (4 per arm) | 6 | 8 |
| Correction, first turn | `own_evidence` 4, `statement` 4 | 8 | 7 | 8 |

Read by step:

- **Own evidence decided 20 first turns** (10 in each Memento arm): 15 right by the committed
  scores, 20 by hand. The five committed misses are the artifact runs in `r2-noisy-up` (arm 3),
  `r2-crowd-rival` (arm 4) and `r2-corr-belief` (arm 4, repeat 2).
- **The statement decided 4 first turns** (both arms, `r2-corr-standing`), and all 4 are right: the
  shopper says M and the hidden first answer is M.
- **The pool decides only in arm 4, and only in case 1.** Its four turns are right 3 of 4 by the
  committed score and 4 of 4 by hand (`r2-a4-new-plain-r1-first` is the artifact). The pool's
  recorded step is also the row for arm 3's four case-1 turns, but **arm 3's prompt carries no pool
  block** (`ProjectRecall` nulls it, `bench/round2/prompt.go`), so the step that could decide
  there was the statement — whose precedence entry still names the pool as the overriding step,
  with no pool beside it. Arm 3 answered the statement's M, 0/4. The score's `rung` field reads the
  unprojected live response, which is why the recorded rung and the step present in the arm's
  prompt disagree for those four turns only. The run files are
  `bench/round2/runs/r2-a3-new-plain-r*-first.json` and
  `r2-a3-new-otherbrand-r*-first.json`; the prompt text is in each file.
- **After the correction, the correction decides in 24 of 24 Memento later turns.** Every belief's
  precedence entry reads `overridden_by: correction` and the override carries
  `takes_precedence: true`, so the frozen `rung` field is empty there: it only reports an
  un-overridden step, and after a correction no step is un-overridden. The correction-honoured
  counts above are the outcome.

## Citations that support

Mechanical rule (round 1's, unchanged): a cite resolves to exactly one history row and supports
when that row counts toward fit. Only the shopper's own PayPal link ids are matched, so only arm 2
can offer a cite the rule resolves. Counts from `bench/round2/runs/r2-scores.json`:

| Arm | First turns cited / supported | Correction | Next session | Other agent | All 28 turns |
|---|---|---|---|---|---|
| 1 no memory | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 |
| 2 PayPal history | 8 / 4 | 0 / 0 | 4 / 0 | 4 / 0 | 16 / 4 |
| 3 shopper only | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 |
| 4 full | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 |

- **All 4 supported cites are arm 2's, on unlike-the-crowd, 2 per repeat**
  (`bench/round2/runs/r2-a2-crowd-plain-r1-first.json`, `...-r2-first.json`): the custom id and
  the order id of the fit-feedback purchase, both resolving to fit evidence. The reply's reasoning
  is the recency tie-break described above, not the layer's usual-size rule.
- **Arm 2's 8 later cites resolve to nothing fit**: `r2-corr-standing`'s kept M purchase is cited
  by custom id and order id in every next-session and other-agent turn (8 cites, 0 supported); the
  kept row is a reason-free positive signal, and the grader counts it toward nothing
  (`bench/round2/runs/r2-a2-corr-standing-r*-next-session.json`, `...-other-agent.json`).
- **Arms 1, 3 and 4 cite nothing in 28 turns each.** The Memento memory carries belief and
  evidence ids, not PayPal link ids, so this rule cannot resolve anything a Memento arm would
  cite. That is a limit of the measure, not a finding about those arms: the citation column cannot
  separate the two Memento arms from each other or from arm 2 on this set.
- **Wrong-cause errors: 0 in all 112 turns, in every arm and every case**
  (`r2-scores.json`, `wrong_cause` 0 on every row). No arm moved a size off the kept baseline on
  the back of a non-fit cause, so this measure did not discriminate.

## Tokens

`harness-words-v1` (whitespace words over prompt + reply), recorded on every score
(`token_basis` in `bench/round2/runs/r2-scores.json`). First-turn words are the sum over that
case's four turns; the last column is the platform trace spend (`r2-summary.json`,
`cost_by_arm`):

| Arm | Turns | Words, all turns | Mean per turn | First turns: new / unlike / noisy / correction | Spend |
|---|---|---|---|---|---|
| 1 no memory | 28 | 2589 | 92.5 | 343 / 342 / 341 / 338 | $0.031134 |
| 2 PayPal history | 28 | 28197 | 1007.0 | 1636 / 7162 / 7738 / 2864 | $0.460746 |
| 3 shopper only | 28 | 9482 | 338.6 | 595 / 1983 / 1695 / 1124 | $0.137073 |
| 4 full | 28 | 11263 | 402.3 | 1495 / 2805 / 1690 / 1144 | $0.160356 |

Arm 2's prompt is dominated by the extracted records: `r2-crowd-rival`'s 2152-word extraction is
the largest in the set, and that turn's prompt is the largest at 2228 words
(`bench/round2/runs/r2-a2-crowd-rival-r1-first.json`, `prompt_words`). Arm 4's prompt adds the
pool block to arm 3's in case 1 and case 2. In cases 3 and 4 the live response carries no pool
(`"pool": null` in those prompts), so the two arms' memory JSON is identical there; the header
line above it states the arm's condition and differs in every case (`bench/round2/prompt.go`).
Token figures are words, not model tokens; spend is the platform's own trace cost, reported beside
them and never mixed in.

## Where the arms match

Stated as plainly as the differences. Counts in this section are from
`bench/round2/runs/r2-scores.json` and the run files named above.

- **Wrong-cause errors are 0 in every arm** across all 112 turns. The measure did not separate the
  arms on these cases.
- **The correction turn is 4/4 in every arm**, because every arm has the correction in the
  conversation on that turn.
- **Arms 3 and 4 agree by hand on every case but new-to-brand; by the committed scores they differ
  in every case.** They give the same first-turn size in 12 of 16 cells by hand and 8 of 16 by the
  committed extraction. The four hand differences are all in case 1, the one case the pool answers;
  five more cells differ by the extraction alone, all artifact runs in cases 2, 3 and 4. Both are
  12/12 on the three later turns, 0 cited, 0 wrong-cause, and they record the identical live memory
  in all 28 cells.
- **Arms 1 and 2 match on new-to-brand and noisy refunds**, both 0/4 first turns, and both 4/4 on
  the correction turn. They differ in unlike-the-crowd and in the correction case, where PayPal's
  records carry something the reference reasoner can read as an answer (corr-standing's kept M) or
  the model can read as a tie-break (crowd-plain).
- **No precedence view exists in arms 1 and 2** — 56 turns with an empty `rung` — so "which step
  decided" is a Memento-arms-only measure here.

## What this does and does not prove

Proves on these constructed cases: the layer's memory carried a correction into a new session and
to another agent with no shared conversation, for both Memento arms, on both shoppers (4/4 and
4/4); the pool answered a case no shopper's own history could answer, where arm 3's four turns,
arm 2's four turns and arm 1's four refusals did not (arm 4 3/4 committed, 4/4 hand); and own
evidence outranked a pool that pointed the wrong way in both Memento arms (8/8 by hand, 6/8 by the
committed extraction). It also shows the layer served status labels the freeze did not expect on 28
of 56 turns without moving an answer or a deciding step.

Does not prove: real-world accuracy (we wrote the shoppers, answers and population); a general
correction capability (two constructed shoppers, one correction each, to a size, made outside a
longer conversation); the deployed second agent's behaviour (the other-agent turn is the neutral
instrument, not `memento-second`); the pool on live crowd traffic (the pool is the 121-row seeded
population); cross-brand size mapping (the case-1 second shopper is built to punish it, and one
shopper is not a measurement); or the citation quality of the Memento arms (the rule cannot read
their ids, so the column is empty for structural reasons). The extraction artifact above means the
first-turn counts understate the Memento arms by 2 and 4 turns respectively; the hand reading is
given so a reader can see both.

## Headline candidates (the user picks one — none is chosen here)

1. **"The correction crosses sessions and agents for both Memento arms: 4/4 in the next session
   and 4/4 for another agent, where PayPal history is 2/4 and 2/4 and no memory is 0/4 and 0/4."**
   Exact counts: `bench/round2/runs/r2-scores.json`, `correction_honoured` on the next-session and
   other-agent rows — arm 3 4/4 + 4/4, arm 4 4/4 + 4/4, arm 2 2/4 + 2/4, arm 1 0/4 + 0/4. The
   correction turn itself is 4/4 in every arm, which is what makes the later turns the measure of
   memory rather than of instruction-following. Proves the layer carries a stated correction to an
   agent that never saw the conversation. Does not prove the deployed second agent would, or that
   a correction to anything but size travels (arm 2's 2/4 is a coincidence: it repeats its own M,
   right for one shopper and wrong for the other).
2. **"The pool answers the new-to-brand case that no shopper's history can: 3/4 by the committed
   scores (4/4 by hand), against 0/4 for Memento shopper-only, 0/4 for PayPal history and 4
   refusals for no memory."** Exact counts: `bench/round2/runs/r2-a4-new-*-first.json` (3 right of
   4), `bench/round2/runs/r2-a3-new-*-first.json` (0 of 4), `bench/round2/runs/r2-a2-new-*-first.json`
   (0 of 4), `bench/round2/runs/r2-a1-new-*-first.json` (0 of 4, no size given; arm 1 refuses in all
   16 of its first turns). Case 1 is built so
   the pool is the only input that holds the answer (FROZEN §3). Does not prove the pool is right
   in general: two shoppers on one line, and the pool is the seeded population rather than live
   traffic.
3. **"Own evidence beats a wrong crowd in both Memento arms: 4/4 + 4/4 by hand (4/4 + 2/4
   committed), where the pool alone points the other way and is wrong."** Exact counts:
   `bench/round2/runs/r2-a3-crowd-*-first.json` and `r2-a4-crowd-*-first.json` — 8 of 8 by hand,
   6 of 8 by the committed scores (`r2-a4-crowd-rival-r{1,2}-first.json` are the artifact);
   `r2-crowd-plain` is the shopper whose own evidence carries it past the pool. Arm 2 is 2/4
   (crowd-plain only), arm 1 0/4. The pool is Earned small at M, so a pool-only answer is L and
   wrong (FROZEN §3). Does not prove the rival-birth threshold on other histories: one shopper
   (`r2-crowd-rival`) depends on it, and the pool's "wrong" is constructed, not observed.
4. **"PayPal history costs 2.9 times what Memento full costs and answers 4 of 16 first turns
   against 12 of 16."** Exact counts: `bench/round2/runs/r2-summary.json`,
   `cost_by_arm` arm 2 $0.460746 against arm 4 $0.160356, and 28197 words against 11263;
   `r2-scores.json` first turns 4/16 against 12/16. Proves the input size difference is not free
   in this comparison. Does not prove a general cost ratio: the extraction's size is a property of
   the generated record shape, and the model and instrument are single.

## Reproduce (offline)

From the repository root, no keys, no network, no spend:

```sh
python3 - <<'PY'
import json
scores = json.load(open('bench/round2/runs/r2-scores.json'))['runs']
cases = ['new-to-brand', 'unlike-the-crowd', 'noisy-refunds', 'correction-then-another-agent']
for case in cases:
    row = []
    for arm in (1, 2, 3, 4):
        rs = [r for r in scores if r['case_id'] == case and r['arm'] == arm and r['turn'] == 'first']
        row.append(f"a{arm}={sum(r['right_size'] for r in rs)}/{len(rs)}")
    print(f"{case:34s}", " ".join(row))
for arm in (1, 2, 3, 4):
    rs = [r for r in scores if r['arm'] == arm]
    print(f"a{arm}: words={sum(r['tokens'] for r in rs)} wrong_cause={sum(r['wrong_cause'] for r in rs)}")
PY
```

That prints the committed first-turn counts, the token totals and the wrong-cause totals. The hand
reading is the six replies named above, read as they open; `bench/round2/runs/RUN-NOTE.md` records
the rule's behavior on them. The freeze check is green on this tree:
`bench/round2/freeze_check.sh --allow-runs` verifies all 116 hashes, and `./tools/check.sh` passes
the flag because `bench/round2/runs/FREEZE.json` exists.

## The run files

Every turn is one file named `r2-a<arm>-<shopper>-r<repeat>-<turn>.json`, with the full prompt, the
memory the arm fetched (empty for arm 1), the reply, the extraction, the trace cost and the frozen
score.
The aggregate files are `r2-scores.json`, `r2-summary.json`, `r2-recall-compare.json`,
`r2-corrections.json`, `r2-seed-log.json` and the `FREEZE.json` run marker, all listed at the top
with `RUN-NOTE.md`. The driver is `bench/round2/runs/cmd/`; the protocol for opening the directory
is `bench/round2/runs.go` and FROZEN §8. No frozen input was changed by the run: the manifest is
still 116 rows, and `bench/round2/freeze_check.sh --allow-runs` is green.
