# P10: Comparison, round 2

The comparison product.md now asks for: what a PayPal-only agent can do, against the layer with and without the pool, on cases chosen because they separate the arms for a stated reason. Round 1 (P5) stays published, relabelled honestly.

Read first: [`../product.md`](../product.md) ("The comparison"), [`PHASE-5-comparison.md`](PHASE-5-comparison.md) in full (the freeze, the publish-everything rule, the neutral instrument), and `bench/README.md`.

The rules from P5 carry over unchanged: the shoppers, population, catalog, prompts and grader are frozen before any arm 3 or 4 run; every run is published; counts, never percentages; if one arm matches another, the document says so; the headline is picked after the runs.

---

### T10.1: What arm 2 actually gets
```yaml
requires:   []
fixture-ok: no
size:       S · mid
owns:       bench/round2/arm2/
status:     done
```
Record exactly what the Agent Toolkit returns for a shopper's history: transaction listing, line items, refund and dispute status, dispute reason codes. Start from T0.4's recordings (`research/paypal.md`) and fill any gap with sandbox reads. Transaction Search is enabled on the sandbox app and the token carries `https://uri.paypal.com/services/reporting/search/read`; the reporting read must use a date range of 31 days or less (`400` otherwise). Stop if the scope disappears before the run. Write the extraction that turns that, and only that, into arm 2's prompt input: no refund reason, no classification, no signal strength, nothing the layer produced. No model spend.

**Done when:** the recorded responses and the extraction, with a test, are in `bench/round2/arm2/`. A test proves the extraction's output has no field the Agent Toolkit didn't return.

Notes: the toolkit was run, not only read. `@paypal/mcp` 1.8.1 on `@paypal/agent-toolkit` 1.11.0 served the tools over stdio with an access token and no model key. The complete `tools/list` from the order session is `bench/round2/arm2/recordings/probe-tools-list.json` (28 tools, `get_order` among them), and `probe-tools-used.json` extracts the five used: `list_transactions`, `get_order`, `get_refund`, `list_disputes` and `get_dispute`. Remediation for review F1: a fresh sandbox session called `get_order` for `ORDER-ID-US-1`, and its untouched JSON-RPC envelope is `probe-mcp-get-order.json`; the body it carries matches `get-order-1.json` field for field. The other two order bodies are the same tool's output for the other two ids, kept as recorded. What it returns is narrower than the product wording suggests: `list_transactions` wraps the reporting API with no `fields` parameter, so the default projection is `transaction_info` only — no line items, no payer info — and the tool schema drops a `fields` argument. Line items therefore come from `get_order` (recorded for the three demo orders), which needs an order id the caller already holds. The tool's default `transaction_status` is `S`, so the pending dispute hold reaches the listing only on a second call with `P` (the held order's `payments.refunds` also carries it, and the extraction drops that block). Refunds are a `T1107` row plus `get_refund`, and the seller's free text sits in both, so generated shoppers must keep it fit-neutral; disputes are `list_disputes` plus `get_dispute`, whose `reason` is PayPal's code. The extraction (`bench/round2/arm2`) filters by the seller's `custom_id` (the only per-shopper key in the listing), emits a JSON array of the returned records it needs, and adds no field: the test finds each output record's source response and fails on any path or value that response did not carry, empty objects included. Sandbox calls only; no model spend, no key in any file.

---

### T10.2: Round 1, relabelled
```yaml
requires:   []
fixture-ok: yes
size:       XS · light
owns:       bench/README.md
status:     done
```
In `bench/README.md`, the round 1 arm called "raw history" is renamed **Memento evidence without beliefs**, with one paragraph saying why: its rows carried the layer's classified reasons, signal strength and write kind. No count changes, and no run file is touched. A pointer to round 2 is added.

**Done when:** the relabel and the pointer are in, and the freeze check still passes.

Worker notes (in-review): `bench/README.md` gains a "Round 1, relabelled" section between Limits and Setup that names the arm **Memento evidence without beliefs**, says why (its prompt carried the layer's evidence rows, so each row already held the classified reason, the signal strength and the write kind; only the beliefs over them were absent), and points at round 2 (`product.md` "The comparison" and this phase document). Every other mention uses the new name: title, Setup arms bullet, headline table row, headline prose, the "what this does and does not prove" paragraph, and lead-number candidates 1 and 3. No count changed, and no run file, scores file or freeze input was touched; `bench/freeze_check.sh` green (25 rows) and `./tools/check.sh` green.

Surprises for the next agent: the committed round 1 run files keep `arm_name: "raw-history"` and the prompt line "Memory: raw history", because they are frozen T5.5 output and the manifest pins `bench/harness/harness.go`; the report label cannot be pushed into them, so a reader comparing this document to a run file will meet both names and should change neither. `bench/pilot/README.md` (the T5.3 record) and `dev-diary/PHASE-5-comparison.md` (its header and T5.9's task text) still say raw history; both are outside this task's owns. Separately, this round 1 record still reports the correction measure as nil and does not point at T5.9's 27 correction turns (9 of 9 honoured in every arm, `bench/runs/t59/t59-scores.json`); fixing that adds counts, which this task forbids, so it needs a follow-up task owning `bench/README.md` to add the correction-round pointer.

Review fixes: the Setup bullet's manifest description was stale since T5.6 — it said 21 sha256 rows and 5 arm-facing histories while `bench/freeze_check.sh` pins 25 and the T5.7 amendment added 4 histories — so it now reads 25 and 9. The relabel paragraph and the Setup arms bullet say "evidence rows" rather than "per-purchase rows" (some rows are shopper statements), and the relabel paragraph states that the frozen run files keep the old `arm_name` and prompt line, so the report is not mistaken for a rewrite of them. The round 2 pointer says "tasks and freeze" rather than "frozen inputs", because no round 2 freeze exists yet (T10.3 owns it). The gate and the freeze check were re-run green at the review tip.

### T10.2b: Round 1 corrections, reported
```yaml
requires:   [T10.2]
fixture-ok: yes
size:       XS · light
owns:       bench/README.md
status:     done
```
The round 1 record reports the correction measure as nil and stops at T5.6, which reads as if corrections were never measured. T5.9 published 27 correction turns (9 shoppers × 3 arms): 27/27 honoured, 0 wrong-cause, and arm 3 routed 2 of 9 through the real `record_correction` path. Add a short "Corrections" subsection to `bench/README.md` with those counts, the mechanism difference, and a pointer to `bench/runs/t59/t59-scores.json`; state plainly that corrections did not discriminate the arms. No other count and no run file changes.

**Done when:** the T5.9 counts are in the record with their evidence file; `bench/freeze_check.sh` and the gate pass; no run file or frozen input changed.

Worker notes (in-review): `bench/README.md` gains a `## Corrections` section after the five measures and before "What this does and does not prove": 9 shoppers × 3 arms = 27 turns, honoured 27 of 27 (9 of 9 in every arm), wrong-cause 0 across all 27, with the evidence file `bench/runs/t59/t59-scores.json` and the per-run directory `bench/runs/t59/`. It states the mechanism difference (arms 1 and 2 carry the correction as conversation text; arm 3 made the real `record_correction` call in 2 of 9 cells, `t59-a3-noisy-large.json` and `t59-a3-noisy-small.json`, keeping the Venerable belief with the override beside it, and had no belief to declare against in the other 7 of 9), and says plainly that corrections did not discriminate the arms. Three existing nil statements now point at that section instead of implying the measure was never taken: the Setup seed bullet, the sentence under the headline table, and measure 3. The "does not prove" paragraph now names what T5.9 leaves unmeasured (a correction to anything but size, or one arriving mid-conversation) rather than saying corrections were absent. No other count changed, and no run, scores, fixture or freeze input was touched; `bench/freeze_check.sh` green (25 rows) and `./tools/check.sh` green.

Surprises for the next agent: the T5.9 scores carry `correction_honoured` as a real boolean on all 27 rows, so the round 1 table's `nil` and the corrections section's `27 of 27` describe different sets (the 45 frozen turns and the 27 correction turns). A reader collapsing them into one column would overstate the T5.5 set. The HEADLINE TABLE is deliberately left unchanged, with the pointer sitting under it, because its rows are the 45-turn counts. `correction_recorded` exists only on the arm-3 score rows (9 of 9; arms 1 and 2 have no such field), so the 2 of 9 is arm 3's denominator alone and must not be read as 2 of 27.

Review fixes: the corrections rule said the shopper names "a different size than the arm just gave ... so the correction always contradicts", which the run files make too strong for 15 of the 27 cells: 14 carry an empty effective first size (8 arm 1, 6 arm 3) and `cmp-unclear`'s M is the extractor artifact over a refusal, so in those the correction supplied a size instead of contradicting one. The text now separates the two cases; no count changed. The "does not prove" clause called a mid-conversation correction unmeasured while T5.9's correction itself arrives in the conversation, so it now names three unmeasured cases: a correction to anything but size, one before a recommendation, and one in a longer conversation. Counts re-derived here from `bench/runs/t59/t59-scores.json`: 27 rows, `correction_honoured` true on all 27, 9 of 9 per arm, `wrong_cause` 0 on every row, `correction_recorded` true on exactly the 2 named arm-3 rows, false with a reason on the other 7, absent on arms 1 and 2. The 45-turn `bench/runs/scores.json` is unchanged and carries `correction_honoured: null` on all 45 rows; the branch touches `bench/README.md` and this document only; `./bench/freeze_check.sh` (25 rows) and `./tools/check.sh` are green at the review tip.

---

### T10.3: Round 2 shoppers, population and freeze ★
```yaml
requires:   [T9.1, T10.1, T8.5]
fixture-ok: yes
size:       M · frontier
owns:       bench/round2/ (except arm2/ and runs/), seed/round2.go, seed/round2_test.go, tools/check.sh
status:     done
```
The four cases in product.md, at least two shoppers each, with hidden answers kept apart as in T1.3:

1. **New to the brand:** no history with the brand; the pool knows it runs off-size.
2. **Unlike the crowd:** the shopper's own evidence disagrees with the pool, and the right answer is theirs.
3. **Noisy refunds:** mixed causes where a refund status with no reason can't tell fit from the rest.
4. **Correction, then another agent:** a correction to agent A; agent B asks next, with PayPal access but not A's conversation.

Each case states, before any run, why it separates the arms, and is checked for fairness: a perfect reasoner given each arm's input gets the answer whenever that input holds it. Arms: 1 no memory; 2 PayPal history (T10.1); 3 Memento, shopper only (no pool); 4 Memento, full. The same neutral instrument, model and catalog in every arm. Arm 2's generated orders set `custom_field`, its refund notes stay fit-neutral, and line items or order ids are supplied to every arm or to none. If any shopper has a multi-item order, decide the fit-row key for it here. Then freeze: `bench/round2/FROZEN.md` with hashes of the shoppers, answers, population, prompts, extraction and grader, and a freeze check the gate runs.

**Done when:** the shoppers, answers and population are generated and tested for fairness. The freeze manifest and check are committed, and the gate runs the check. No arm 3 or 4 run exists before this commit.

The P2 review found `bench/runs/cmd/t{55,58,59}` mirror the direction rule but not the rival-birth threshold: every opposite-direction row lands unattached with no streak, while the driver claims it lands "below the rival-birth threshold". No frozen history reaches three consecutive opposites, so no published run is wrong, but a frozen-set extension would diverge from the layer. Mirror `RivalBirthThreshold` in the round-2 arms, or state the limitation in the run's notes.

Worker notes (in-review): `seed/round2.go` generates the eight shoppers (`Round2Seed` 41), their hidden answers and the round-2 population; `seed/round2_test.go` pins determinism, the seeded mark, two shoppers per case, the visible marks on the arm path, the fairness invariants and the pool statuses. The frozen fixtures are `bench/round2/shoppers/shoppers.json`, `bench/round2/hidden/answers.json`, `bench/round2/population/population.json` and 99 generated PayPal records under `bench/round2/paypal/<shopper>/`. `bench/round2/` adds the four arms and cases (`cases.go`), the facts each arm's input holds and the reference reasoner (`facts.go`, `reason.go`), the prompt template and the recall projection (`prompt.go`), the expected `recall_for_purchase` response per shopper (`recall.go`), the arm-2 record generator (`paypal.go`), the round-2 run schema and grader (`grade.go`), and `FROZEN.md` with `freeze_check.sh` (116 rows) called from `tools/check.sh`. Eight shoppers, two per case: `r2-new-plain`/`r2-new-otherbrand` (answer L, pool only), `r2-crowd-rival`/`r2-crowd-plain` (answer S, own evidence beats the pool; the first depends on the rival-birth threshold), `r2-noisy-up`/`r2-noisy-down` (L and S, reasons against mixed causes and an unexplained refund), `r2-corr-belief`/`r2-corr-standing` (L then M, and M then S, the correction standing on its own for the second). Repeats are two per shopper per arm (the budget arithmetic is in FROZEN.md section 2); shoppers stay at exactly two per case. `TestFairness` proves, per arm and shopper, that a perfect reasoner over that arm's input gets the answer exactly where the frozen hold table says the input holds it and names the missing fact where it does not; `TestTheCasesSeparateTheArms` proves each case's stated separation (the pool is the only step that answers case 1; naive pooling answers L in case 2 where own evidence answers S; case 3's pool is Claimed; only the Memento arms know the correction); `TestArmTwoInputCarriesNoCause` proves every record is the shopper's own, no free text names a cause, the correction appears nowhere, every capture has its order, and every order carries exactly one line item. `TestRunsBeginOnlyAfterTheFreeze` and the shell check both refuse an unopened `bench/round2/runs/` directory. No arm 3 or 4 run exists, and this task made no model call and no sandbox call. `./tools/check.sh` green; `bench/round2/freeze_check.sh` green at 116 rows, and a one-byte tamper on `hidden/answers.json` fails it as intended.

The P2 finding is answered in the arms rather than in a note: `bench/round2/reason.go` uses `domain.OppositeStreak` and `domain.RivalBirthThreshold` from the layer's own package, `seed/round2_test.go` walks `r2-crowd-rival` through them to prove exactly one rival is born, and the case's hidden answer is S, so a driver that mirrors only the direction rule answers L and gets that shopper wrong. The shopper is built with a changed-mind answer and a damaged return between the three opposites, both of which move no run, exactly as the service's answer path says; the rival then gets two more same-direction returns so it carries strictly more evidence than the belief it beats, because recall serves the belief with more evidence first and the case cannot rest on a tie.

Surprises for the next agent. (1) The layer's fit row is one document per purchase keyed by the capture, so a multi-item order would overwrite one line's row with the other's; round 2 therefore keeps exactly one line item per order and a test enforces it. That is a layer gap, and the issue text is in the task report for the user to file. (2) Every round-2 line uses the `regular` fit handle: the served layer resolves an item's line through `seed.PopulationLines()` and falls back to regular for a line the round-1 population never seeded, so a `slim` round-2 line would pool under a line the frozen pool does not name. (3) Recall serves only beliefs at Candidate or Venerable standing, so every shopper whose answer rests on own evidence carries at least two fit returns in that direction (three where the belief must outrank the pool), spread over at least 0.3 of the session extent; a case with one or two returns could refuse instead of answering. (4) The pool margins are wide on purpose: the case-2 shoppers' own too-large rows join the pool when their histories are ingested, and the line still has to read Earned small afterwards, so the population carries 40 too-small returns per line rather than the eight the Earned floor needs. (5) `r2-new-plain` and `r2-new-otherbrand` answer M in the shopper-only arm from the general statement and L only from the pool: the fairness table records that arm 3's answer there is a decoy, not an answer, because a statement is memory. (6) The expected recall fixture strict-decodes into `contract.RecallResponse`, so T10.4 can compare the live response against it field for field; ids are minted deterministically and a live layer will mint its own, so no run may compare ids. (7) Seeding the live index must pass the moment of seeding (`seed.GenerateRound2Population(time.Now().UTC())`), as T9.1's note already says for round 1, or the recent window drifts and an Earned line turns Out of true.

**Review (fresh eyes).** Re-verified from the committed files on a branch cut at the task tip, no live call, $0. The tip touches only `bench/round2/` (except `arm2/` and `runs/`), `seed/round2.go`, `seed/round2_test.go`, one line of `tools/check.sh` and this block; `bench/round2/arm2/` and `bench/round2/runs/` are untouched and no round-2 arm 3 or arm 4 run exists. The gate is green, and all 115 manifest rows verify, with the shoppers, answers and population hashes re-derived independently. Mutations, every one reverted: flipping the case-1 arm-4 first-turn hold, flipping the case-4 arm-3 after-correction hold, and declaring the case-1 arm-3 hold true each fail `TestFairness` on the expected side; dropping the rival birth (`streak.NoteOpposite`) makes `TestFairness` and `TestTheCasesSeparateTheArms` fail on `r2-crowd-rival` (own evidence answers L, the hidden answer is S), so the case really rests on `domain.RivalBirthThreshold`; one appended byte to `shoppers.json` and `Round2Seed` 41 to 42 each fail the freeze check and the generator test; `mkdir bench/round2/runs` fails the shell check. Arm 2's premise was re-read for `r2-noisy-up`: seven refund records at one SKU and size, three of them fit returns, no cause in any field. Every prompt is under the frozen 4000-word cap; the largest is arm 2 on `r2-crowd-rival` at 2228.

Findings. (1) High, owner: orchestrator (widen T10.4's owns) — `freeze_check.sh` fails whenever `bench/round2/runs` exists, and `TestNoArmThreeOrFourRunExists` fails on any file inside it, while T10.4 owns only `runs/` and its Done-when wants the check green after the last run. As frozen, T10.4 cannot land a single run; the guard needs a pre-freeze scope (or an `--allow-runs` mode) and its own owner before T10.4 claims. (2) Medium, owner T10.4 — the $5.00 cap does not hold at the P9 rates: 168 turns times the mean of P9's nine recorded agent turns ($0.0337) is $5.66, and the break-even is $0.0298 per turn, so only the cheapest recorded turns fit. Freeze §2's "well inside" uses round 1's neutral-instrument rate ($0.001-0.002 per turn, $0.34 for 168), not the agent rate this comparison's instrument implies. Two repeats (112 turns, about $3.77) fits with headroom; run one cell, record its rate, and choose repeats before the batch. (3) Medium, owner T10.4 — `FetchMemory`/`PromptFor` render the post-correction `ExpectedRecall` for both Memento arms, so the case-4 first-turn prompt already carries "Actually I have lost a size since then, use M now." and "I was measured last week, take S now."; a first turn driven through `PromptFor` is answered after the correction it must precede, and neither a test nor a line pins that the first turn fetches before it. Drive live per turn and render with `MemorySection`, or the freeze needs a pre-correction expectation. (4) Low, owner T10.4 (same owns amendment) — `HoldExpectation.Missing` is frozen but never read: only case 1 fills it and no test compares it with the reasoner's `Decision.Missing`; enforce it or drop it. (5) Low, owner orchestrator (accept or open a layer task) — `reason.go` restates the layer's size ladder (`internal/api/server/content.go` holds the same unexported list), as round 1's driver already does, so a ladder change would drift the reference reasoner silently.

Review fixes: `TestExpectedRecallIsWhatTheMementoArmsSee` now nulls arm 4's pool and requires the two Memento memories to be byte-identical, so FROZEN §9's "a test pins it" is true (a mutation deleting `pending_count` from `ProjectRecall` fails it); FROZEN §8's "a test asserts it" is replaced by the rewrite the second pass verifies. Test and document only: no frozen input, fixture or hash changed, and the gate is green at the review tip.

Review fixes: three blockers and the budget. (1) **Runs can now land.** `bench/round2/freeze_check.sh` gained `--allow-runs` (hashes still verified in full, the `runs/` refusal skipped), and `tools/check.sh` passes it when, and only when, `bench/round2/runs/FREEZE.json` exists. The Go guard is `TestRunsBeginOnlyAfterTheFreeze` with the marker written by `round2.BeginRuns` (`bench/round2/runs.go`): it names the freeze commit and pins `bench/round2/FROZEN.md` by hash, and the test refuses a run directory with no marker or a marker written against another freeze. The commit id is shape-checked rather than looked up in git, because CI checks out a shallow clone; the manifest hash is the binding that holds everywhere. With no marker and no flag the directory is still a freeze break, which is what keeps a run from landing before the freeze. The protocol is FROZEN.md section 8. (2) **Case 4's first turn no longer carries the correction.** `ExpectedRecall` takes a phase (`PhaseBeforeCorrection`, `PhaseAfterCorrection`), `TurnPhase` maps the first turn to the pre-correction phase and the correction, next-session and other-agent turns to the post one, and `PromptFor`/`FetchMemory` render the turn's phase. Before the correction the expected response carries no `overriding_correction` and nothing contradicted, which is what the layer serves before `record_correction`; the post-correction response stays for the later turns. `TestFirstTurnPrecedesTheCorrection` asserts the first-turn prompt has no correction text, the later turn does, and a shopper with no correction gets byte-identical memory in both phases. (3) **The frozen `Missing` text is enforced.** Each hold-table entry is now one of the `Missing*` constants in `reason.go`, and `TestFairness` requires the reasoner's `Decision.Missing` to equal the declared text character for character whenever the input holds no answer, requires an entry wherever the table says the input holds none, and forbids one where it holds the answer. (4) **Repeats are two, not three.** The task text's three does not fit T10.4's $5.00 cap: at the P9 mean of $0.0337 per turn, three repeats is 168 turns (96 first plus 72 correction) at about $5.66, while two is 112 turns (64 plus 48) at about $3.77, which leaves room for a discarded turn. FROZEN.md section 2 states the arithmetic. T10.4's own task text still says three repeats; the orchestrator should reconcile it, and the freeze's two wins until then. The manifest is re-frozen at 116 rows: `runs.go` is added as the protocol's own frozen file, and no fixture file is added because the pre-correction expectation is computed per phase rather than stored, and `./tools/check.sh` is green with the run directory absent.

Review, second pass. Re-verified the worker's `4dddf19` from the committed files, no live call, $0. The marker protocol bites: `bench/round2/runs/FREEZE.json` with a wrong `frozen_manifest_sha256` fails `TestRunsBeginOnlyAfterTheFreeze` ("was written against a different freeze"), and `freeze_check.sh` refuses the directory without `--allow-runs`; with a valid marker and a run file in place the whole gate is green, so a T10.4-shaped tree can land. The phase rule bites: forcing `TurnPhase` to the after phase, and separately letting `ExpectedRecall` carry the correction in the before phase, each fail `TestFirstTurnPrecedesTheCorrection` on the correction text and on `overriding_correction`. A drifted `Missing` text fails `TestFairness` character for character. The manifest is 116 rows, all verify, with `runs.go` pinned; 112 turns at the P9 mean is $3.77 and the 168-turn figure re-checks at $5.66. The first pass's findings (1) to (4) are closed; the pool-only assertion it asked for is carried on this branch. Remaining: (5) the size-ladder mirror, Low, owner orchestrator (accept or open a layer task); T10.4's task text still says three repeats, Low, owner orchestrator (reconcile it to the freeze's two, or a T10.4 worker will start the expensive batch); the gate wiring is a 12-line change to `tools/check.sh`, not the first brief's one line, recorded in FROZEN section 11 and owned by the orchestrator as an owns extension.

---


### T10.4: All four arms ★
```yaml
requires:   [T10.3, T10.6, T9.5, T9.6]
fixture-ok: no
size:       M · frontier
owns:       bench/round2/runs/
status:     not-started
```
Every shopper through every arm, two repeats (the freeze's two, not the three this text carried before it), plus the cross-session and cross-agent correction turns for case 4 (agent B is T9.5's agent for arms 3 and 4; for arms 1 and 2 it is the same instrument with no shared conversation). Arm 4 uses the live layer and the frozen population. Per-run token and spend caps as in P5. Budget: $5.00 cumulative. Run one cell first and check its cost.

**Done when:** every run is committed in the frozen run schema, graded by the frozen grader, with the freeze check green before the first run and after the last. Spend is recorded.

---

### T10.5: Round 2 results ★
```yaml
requires:   [T10.4]
fixture-ok: yes
size:       S · frontier
owns:       bench/round2/README.md
status:     not-started
```
The results as counts, each linked to its runs, with the limits first: we wrote the shoppers, the answers and the population, so this shows the mechanism on constructed cases, not real-world accuracy. Per case and per arm: right size first time, wrong-cause errors, corrections honoured (next turn, next session, other agent), which precedence step decided, citations that support, tokens. Where arms match, say so. List the headline candidates with exact counts; the user picks one.

**Done when:** every claim links to run files and the scores; the headline candidates are listed and none is chosen.

---

### T10.6: Keep the reference reasoner's ladder in step with the layer
```yaml
requires:   [T10.3]
fixture-ok: yes
size:       XS · mid
owns:       bench/round2/
status:     claimed:t107-kept
```
`bench/round2/reason.go` restates the layer's size ladder (unexported in `internal/api/server/content.go`), as round 1's driver does. A ladder change in the layer would drift the reference reasoner silently, and the freeze would keep the old order.

**Done when:** the reference reasoner's ladder is checked against the layer's (an exported constant, a behaviour test through the served path, or a recorded reason), and a mismatch fails a test; the gate passes.

---

### T10.7: The reference reasoner reads a kept purchase
```yaml
requires:   [T10.3]
fixture-ok: yes
size:       S · mid
owns:       bench/round2/
status:     claimed:t107-kept
```
The P10 review found arm 2's hold is asserted, not derived: `decidePayPal` refuses any input carrying the shopper's brand without reading the records, and the premise test covers returns only. But a kept purchase is a reason-free positive fit signal the layer counts (usual size, pooled kept), and arm 2 can read it. Measured on the frozen records, `r2-corr-standing` has one kept M capture (hidden answer M) and `r2-crowd-plain` one kept S (hidden answer S), so FROZEN's "no shopper's answer is derivable from PayPal's records alone" and case 4's "Arm 2 holds no answer" are false for at least one shopper. Teach the reference reasoner to read a kept purchase on the candidate line and set the hold table accordingly, or replace the blanket claim with the measured fact; either way fix FROZEN's stale §11 sentence (T10.4's two repeats are already reconciled) and re-freeze. Do this before any run: no `runs/` or marker exists.

**Done when:** the reasoner and hold table treat kept purchases honestly, the FROZEN claims match the measured records, §11's stale sentence is gone, the manifest is re-frozen and the gate passes.

---

## Exit Criteria

- [x] Arm 2 gets exactly what the Agent Toolkit returns, and nothing the layer made (T10.1).
- [x] Round 1 is relabelled honestly (T10.2).
- [ ] Four fair cases frozen before any arm 3 or 4 run (T10.3).
- [ ] Every run published and graded (T10.4).
- [ ] Results as counts, the headline picked by the user (T10.5).
