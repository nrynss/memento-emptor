# T10.4 run note: the four arms

## What ran

- **Freeze.** `bench/round2/FROZEN.md` as landed by commit
  `b137c9fd0602da9a5806ebcdd11b5d3e37415928`; the run directory was opened
  with `round2.BeginRuns` and `bench/round2/freeze_check.sh --allow-runs`
  ran green before the first turn and again after the last.
- **Cells.** Eight frozen shoppers x four arms x two repeats = 64 first
  turns. Case 4 adds the correction, next-session and other-agent turns
  (2 shoppers x 4 arms x 2 repeats x 3 = 48). **112 turns in all.**
- **Instrument.** Deployment `pilot-baseline`, model `claude-sonnet-4-6`
  in every arm, one fresh conversation per turn, the messaging proxy's
  billing read, message, trace poll and id scrubbing as in
  `bench/pilot/cmd/drive` and `bench/runs/cmd/t55`. The prompt carries the
  arm's memory itself; no agent tool spans are involved.
- **The layer.** One Lambo 0.10.1 hub (native `.jina-embeddings-v3` over
  the live Elastic deployment), one `server.Service` per shopper over its
  own hub session, the live fit index `t104-fit-rows` (scratch, deleted at
  the end) and a temporary standing journal. The layer ran continuously
  from the population seed to the last turn; nothing restarted it
  mid-run. Recall was served over the layer's own HTTP handler
  (`POST /tools/recall_for_purchase`), not read from a stub.
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
turns: 7 (the round-1 string rule flags a reply that quotes its own live
belief id — `blf_`; the memory is the arm's own, not another shopper's).
Every run file carries its own `score` (the frozen `round2.Score`), and
`r2-scores.json` collects the same 112 rows.

## Differences between the live recall and the frozen expectation

`r2-recall-compare.json` compares every Memento turn with
`bench/round2/recall.go`'s expectation for its phase. Ids, the absolute
event times and the guidance prose are excluded; every other field
compares. **28 of 56 turns differ, in three groups; the other 28 match
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
  be 12/16 and arm 4's 16/16; T10.5 should decide whether to report the
  deterministic counts, the hand re-read, or both.
- **Transport failures.** Two message POSTs answered 502 and were not
  recorded (no message id, no reply): `r2-a3-corr-standing-r1-first` and
  `r2-a2-corr-standing-r1-other-agent`. The batch stopped each time and
  the turn was sent again after a fresh seed; both retries succeeded. The
  billing reads across those gaps rose by at most $0.02, platform
  overhead included; any orphaned turn cost is below one arm-2 turn and
  is not in the scores.
- **One turn's trace arrived late.** `r2-a1-noisy-up-r2-first` was sent
  and paid for, but its trace was not listed within the deadline. The
  failure was recorded, not retried; the trace appeared afterwards
  (short id `c39c1881`, $0.001080) and was recovered from the platform and
  recorded with a note. This turn's recovery is why the scores carry 112
  rows.
