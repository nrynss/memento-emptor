# T10.4 run note: the four arms

## What ran

- **Freeze.** `bench/round2/FROZEN.md` as landed by commit
  `b137c9fd0602da9a5806ebcdd11b5d3e37415928`, the commit that added it
  (`git log --diff-filter=A`). The file was amended after that commit, and
  the marker's `frozen_manifest_sha256` (`74a2ad9d0920…`) pins the amended
  file in this tree, which is the binding FROZEN section 8 describes. The
  run directory was opened with `round2.BeginRuns`; the driver runs
  `bench/round2/freeze_check.sh --allow-runs` at the start of every
  invocation, and the check is green on the committed tree.
- **Cells.** Eight frozen shoppers x four arms x two repeats = 64 first
  turns. Case 4 adds the correction, next-session and other-agent turns
  (2 shoppers x 4 arms x 2 repeats x 3 = 48). **112 turns in all.**
- **Instrument.** Deployment `pilot-baseline`, model `claude-sonnet-4-6`
  in every arm, one fresh conversation per turn, the messaging proxy's
  billing read, message, trace poll and id scrubbing as in
  `bench/pilot/cmd/drive` and `bench/runs/cmd/t55`. The prompt carries the
  arm's memory itself; no agent tool spans are involved.
- **The layer.** One Lambo 0.10.1 hub per invocation (native
  `.jina-embeddings-v3` over the live Elastic deployment), one
  `server.Service` per shopper over its own hub session, the live fit
  index `t104-fit-rows` (scratch, deleted at the end) and a temporary
  standing journal. Recall was served over the layer's own HTTP handler
  (`POST /tools/recall_for_purchase`), not read from a stub.
- **A restart reseeds; the batch ran in four invocations.** Each driver
  invocation boots a fresh hub, regenerates the 121-row population at that
  moment (see Population below) and replays the frozen histories through
  `LoadSeed`; the one-cell check was an earlier, fifth invocation. Three of
  the four batch invocations stopped early — the missing trace, then the
  two 502s below — so a turn recorded in a later invocation was served by a
  fresh seed of the same frozen inputs, not by a layer that had kept
  running. Every restart reused the turns already committed rather than
  sending them again: the two 502 turns were the only turns sent twice, and
  the trace-missing turn was recovered, not re-sent. The seed moment moves
  the absolute event times and the pool's recent window, and it moved no
  pool count: no pool difference appears in `r2-recall-compare.json`.
- **Population.** `seed.GenerateRound2Population(time.Now().UTC())`, 121
  rows, written through `fitindex.Writer`'s per-shopper transitions
  (`Captured`, `Refunded`, `Classified`, `Feedback`, `Kept`) and verified
  back through the pool reader: the Vale and Wren lines both read **Earned
  small** at M, 59 settled / 59 seeded. The fixture's `AsOf` was not
  reused.
- **Histories.** Each frozen history through `server.LoadSeed`, the
  layer's real write path: the general statement derives, classified fit
  returns, seller answers and fit feedback are structural writes citing
  the belief in `depends_on` with their own event time, an
  opposite-direction run births the rival at
  `domain.RivalBirthThreshold`, a reasonless refund is a pending question,
  and a directionless answer (changed mind, unclear) files nowhere. The
  per-shopper filing is `r2-seed-log.json`.

## Spend

| Arm | Turns | Spend |
|---|---|---|
| 1 no memory | 28 | $0.031134 |
| 2 PayPal history | 28 | $0.460746 |
| 3 Memento shopper only | 28 | $0.137073 |
| 4 Memento full | 28 | $0.160356 |
| **Total** | **112** | **$0.789309 of $5.00** |

By turn kind: first turns $0.520743, correction $0.090291, next-session
$0.089175, other-agent $0.089100.

The one-cell check, before the batch: `r2-crowd-rival` through all four
arms, repeat 1 (four turns) cost **$0.057150** — arm 1 $0.001092, arm 2
$0.036723, arm 3 $0.008199, arm 4 $0.011136. The projection at 112 turns
from that cell was **$1.6002** at the per-arm mean and **$4.1130** at the
priciest measured turn, both inside the $5.00 cap, so the batch ran. The
actual total was lower because `r2-crowd-rival`'s arm-2 extraction is the
largest in the set (2228 words).

## Counts (frozen grader)

| | first | correction | next-session | other-agent |
|---|---|---|---|---|
| Arm 1 right size | 0/16 | 4/4 | 0/4 | 0/4 |
| Arm 2 right size | 4/16 | 4/4 | 2/4 | 2/4 |
| Arm 3 right size | 10/16 | 4/4 | 4/4 | 4/4 |
| Arm 4 right size | 12/16 | 4/4 | 4/4 | 4/4 |

Wrong-cause errors: 0 in every arm. Over-cap turns: 0. Contaminated
turns: 7 (the round-1 string rule flags demo-shopper tokens; every hit is
this arm's own live layer id — six `corr_` and one `blf_` — not another
shopper's memory).
Every run file carries its own `score` (the frozen `round2.Score`), and
`r2-scores.json` collects the same 112 rows.

## Differences between the live recall and the frozen expectation

`r2-recall-compare.json` compares every Memento turn with
`bench/round2/recall.go`'s expectation for its phase. Ids, the absolute
event times and the guidance prose are excluded; every other field
compares. **28 of 56 turns differ, in two groups; the other 28 match
field for field** — including all of `r2-new-plain`,
`r2-new-otherbrand`, `r2-crowd-plain`, `r2-noisy-up`, `r2-noisy-down` and
`r2-corr-belief`'s first turn.

`r2-crowd-rival`, first turn (4 runs, arms 3 and 4):

- The one-evidence runs-small belief is **Claimed/None** live where the
  freeze labels it Building/Candidate. Lambo's Candidate gate needs two
  dependent interactions; the one pre-birth too-small return is the only
  dependent. The rival is served first and decides, so the answer is not
  affected.
- The rival's evidence is **three rows live, five in the fixture**. The
  fixture collects every same-direction return; the layer attaches only
  the rows that cite the rival — the birth row and the two after it. The
  two pre-birth opposite returns stayed unattached, exactly as
  `beliefForReason` files them (FROZEN §5). The rival's status
  (Earned/Venerable), the answer S and the own-evidence rung are the same;
  the evidence ids and row identities differ as a consequence.

After the correction, `r2-corr-belief` and `r2-corr-standing` (24 runs,
the correction/next-session/other-agent turns):

- The live layer reads a contradicted belief's **status** as `Out of
  true`; the fixture keeps `Earned`/`Claimed` and carries
  `contradicted: true` beside it. Both name the same correction.
- The fixture marks **every served belief** contradicted when the
  correction wins; the layer marks only the belief the correction is
  declared against. On `r2-corr-belief` the statement is not contradicted
  live.
- On `r2-corr-standing` the layer declares the correction against the
  shopper's statement (the only belief there), so the served override
  carries a `contradicts_belief_id` and the guidance names that belief;
  the fixture says the correction stands alone. The corrected size (S) and
  the correction-first precedence are the same in both.

## Limits and deviations

- **The statement is pre-derived as a Logic concept.** Lambo never
  resolves an `against` reference to an Observation, and the layer's
  standing-correction path declares against the shopper's statement.
  Without a workaround `record_correction` answers 500 for
  `r2-corr-standing`. The driver derives each statement's content as a
  belief before `LoadSeed`; the layer's own `RecordStatement` then derives
  the same content, matches the existing Logic concept, and the served
  reading is unchanged (no evidence, Claimed). This is a layer gap: the
  statement a standing correction must name is filed as an Observation the
  graph refuses to cite. Owner: a layer task.
- **The fixture's event times are not the live event times.** Seeding the
  frozen anchor's absolute times puts every history row in the future (the
  anchor is the freeze day at noon and the rows run up to +18 days), and
  an unaged dependent never counts, so promotion never happened. Each
  shopper's relative days are seeded in real past event time instead
  (anchor = run start - (session days + 1) days). The comparison excludes
  absolute times and keeps their order.
- **The pool is the seeded population.** `LoadSeed` writes no fit rows, so
  the live pool's counts are the 121 generated rows exactly, matching the
  fixture; a webhook-driven ingest would add the shoppers' own rows, which
  is why the population's margin is wide (FROZEN §5).
- **Kept rows are not seeded as evidence.** They settle the pool's fit row
  and otherwise stay out of recall, as in the live path. The feedback row
  carries its verdict through `FeedbackVerdicts`, not reason words,
  because the frozen history names none.
- **The other-agent turn is a fresh `pilot-baseline` conversation.** The
  deployed second agent (`memento-second`) holds an agent key scoped to
  `shopper_01`; it cannot address a round-2 shopper without minting keys
  and redeploying, which the brief forbids. The turn is the same neutral
  instrument with no shared conversation — the structural property T9.5's
  second agent demonstrates — and the run file labels it agent B.
- **The extraction rule's explicit-size step misreads six Memento
  replies.** The frozen rule reads the first `size X` mention before a
  standalone token, and these replies open with the recommendation but
  quote the returned size later: `r2-a3-noisy-up-r1-first` and `-r2`
  (L read as M), `r2-a4-crowd-rival-r1-first` and `-r2` (S read as M),
  `r2-a4-new-plain-r1-first` (L read as M), `r2-a4-corr-belief-r2-first`
  (L read as M). No human override was applied, so the committed scores
  are the frozen rule's. Re-read by hand, arm 3's first-turn count would
  be 12/16 and arm 4's 16/16. How to present that is T10.5's call: it owns
  `bench/round2/README.md`, and this note does not choose between the
  deterministic counts, the hand re-read, or both.
- **Transport failures.** Two message POSTs answered 502 and were not
  recorded (no message id, no reply): `r2-a3-corr-standing-r1-first` and
  `r2-a2-corr-standing-r1-other-agent`. Each stopped its invocation; the
  turn was sent again in the next invocation after a fresh seed, and both
  retries succeeded. The billing reads across those gaps rose by at most
  $0.02, platform overhead included; any orphaned turn cost is below one
  arm-2 turn and is not in the scores.
- **The correction is recorded again in each invocation, and the
  corrections file holds the last call.** A fresh hub holds no correction,
  so every invocation that reaches a case-4 shopper records it again
  (`recordCorrection`) before that invocation's later turns.
  `r2-corrections.json` is the final invocation's call for each shopper,
  and its `recorded_at` is later than the correction memory the committed
  correction and next-session turns were served (each run file's
  `memory_response` is what that turn saw). The text and the ids agree
  across instances, and `recorded_at` is excluded from the recall
  comparison.
- **One turn's trace arrived late.** `r2-a1-noisy-up-r2-first` was sent
  and paid for, but its trace was not listed within the deadline. The
  failure was recorded, not retried, and the batch stopped there — the
  first of the three early stops above. The trace appeared afterwards
  (short id `c39c1881`, $0.001080) and was recovered from the platform in
  recover mode and recorded with a note. This turn's recovery is why the
  scores carry 112 rows.

## Review (fresh eyes)

Re-verified from the committed files, no live call, $0. The 112 run files
are the exact 8 shoppers x 4 arms x 2 repeats cross product plus case 4's
later turns, with no missing or duplicated id. Every score recomputes with
`round2.GradeRunFile` from the frozen shopper and hidden answer, and
`r2-scores.json`, `r2-summary.json` and `r2-recall-compare.json` recompute
from the run files. Every prompt is `BaseSection` plus `MemorySection` over
the memory the file records (plus the conversation on a correction turn),
and arm 2's memory is `ArmTwoInput` byte for byte. Spend re-sums to
$0.789309; the one-cell figure and both projections re-derive. The frozen
inputs are untouched: `freeze_check.sh --allow-runs` is green at 116 rows
and the run directory is the only addition.

Review fixes, in this file and the driver; no frozen input and no run
file's measured content changed. (1) The earlier text said the layer ran
continuously with no restart; the artifacts say otherwise — the correction
`recorded_at` in `r2-corrections.json` (19:26) is later than the one the
correction and next-session turns carry (18:14 and 18:58), and the seeded
event times and the correction times group into four batch instances.
"What ran" now records the four invocations, the three early stops and
what a restart means. (2) The contaminated count named only `blf_`; six of
the seven replies quote `corr_`. (3) "Three groups" was two, and the
corrections file's note now says it holds the final invocation's calls.
(4) The driver now reuses an existing marker only when it still names this
freeze (it used to rewrite `FREEZE.json` at the start of every invocation,
which would have silently re-bound a directory to an amended freeze), and
it refuses to reuse a committed turn that carries no reply (the
trace-missing failure used to be skipped as "already committed" on
resume).
