# P5: Comparison

Three arms, with the same agent, model, shoppers and catalog: no memory, raw PayPal history, and Memento Emptor. Arm 2 is the baseline that matters: "why not just give my agent the transaction history?" The arm called "raw history" in this phase is now reported as **Memento evidence without beliefs**; see `bench/README.md` for the relabel.

Read first: [`../product.md`](../product.md) (The comparison), `seed/`.

---

### T5.1: The harness
```yaml
requires:   [T1.3, T3.1]
fixture-ok: yes
size:       M · frontier
owns:       bench/harness/
status:     done
```
Runs a set of shoppers through one arm, with a per-run token and spend cap, recording every prompt, tool call, reply and token count. The arms differ only in what the agent is given. Arm 2 gets the shopper's PayPal transactions and past messages directly. No arm can read the hidden answers.

**Done when:** a dry run on fixtures produces a complete record for each arm; a test proves the arms' prompts differ only in the memory they carry.

Worker report: `bench/harness/` runs all six fixture shoppers through each arm offline on scripted replies. Caps are 4000 tokens and $0.05 per run at $5/1M blended (token cap binds first; spend is the backstop); a run over cap stops before any model call. Prompts are base-plus-memory by construction; the answer key is never loaded (static source check plus prompt-vs-Why check). Gate passes; handoff with cap rationale is in the task report.

Freeze-time follow-ups for T5.4 (from review, recorded here so the freeze author finds them with the task): F1 — arm-3 tally counts fits-well as counts-toward-empty (real layer counts verdict→kind per the T1.2 handoff; grader already scores feedback as fit support); F2 — arm-2 inputs omit amounts (rows carry `AmountMinor`) and all transaction IDs (grader resolves cites by ID first, so live arm-2 replies can only cite note substrings — freeze must define the arm-2 input contract); F3 — classified changed-mind tallied as "pending" though only below-floor unclear is truly pending; F4 — arm-3 stub is not the `recall_for_purchase` wire shape (no belief ids/values/guidance/pending count — T5.5 must run real recall). Also: `TestSeedHistoriesCarryNoCorrections` pins the no-corrections premise (revisit on the first live-correction run); word-count tokens are ~5x under live BPE — re-measure with the live tokenizer on the pilot (T5.3), don't trust the proxy.

---

### T5.2: The grader
```yaml
requires:   [T1.3, T5.1]
fixture-ok: yes
size:       M · frontier
owns:       bench/grader/
status:     done
```
Scores a run on the five measures in product.md, as counts: right size first time against the hidden answer; wrong-cause errors; corrections honoured on the very next recommendation; citations that really support the recommendation; tokens per recommendation.

A citation is checked mechanically where it can be (the cited evidence exists and has the right reason category). Any model judgement used is logged as a model's opinion, never as a measurement.

Worker report: `bench/grader/` scores hand-built runs on the five measures as counts, offline (no key, no network, no spend): right-size by normalized equality; wrong-cause only when a non-fit cite backs a size off the kept baseline (holding size is an exclusion); correction by next-turn size equality; cites mechanical (resolves plus fit kind, feedback counts by verdict per T1.2); tokens via harness.CountTokens with basis `harness-words-v1` recorded on every score. Good arm-3-style run plus a bad run per measure, all green.

Freeze-time follow-up for T5.4 (from review, recorded here so the freeze author finds it with the task): live replies are free text with no pinned reply→Size/Cites/WantSize extraction rule yet — without it T5.5 grades the extractor, not the arm. Freeze must define the extraction rule (or human-extraction procedure) alongside the hashes.

**Done when:** hand-built good and bad runs score as expected for every measure.

---

### T5.3: The baseline pilot ★
```yaml
requires:   [T5.1, T5.2]
fixture-ok: no
size:       M · frontier
owns:       bench/pilot/
status:     done
```
Arms 1 and 2 only, before any arm 3 run, to find out what a fair baseline does. If arm 2 scores perfectly, harden the shoppers with cases a real history would contain, or record that the lead is elsewhere. Don't weaken the baseline, and don't tune the cases against the layer.

**User decision:** the spend, before the first run.

**Done when:** the pilot's results are recorded, with any change to the shoppers and the reason for it.

Worker notes (in-review): `bench/pilot/` carries the extraction rule (`pilot.go`, the T5.4 freeze input), the live driver (`cmd/drive`) and grader (`cmd/grade`), ten graded runs + scores, and the pilot record (`README.md` with the spend table). Arm 1: 0/5. Arm 2: 5/5 right-size, 0 wrong-cause — no shopper changes (thin n=1 signal; the baseline reasoned correctly; the lead is recorded as tokens + cites instead), with concrete hardening candidates for the freeze. Headline finding: the reference agent cannot drive the arms — its shopper_01 default contaminated 2/2 turns through it (kept as discarded evidence) — so the pilot measured through a neutral same-model instrument (`bench/pilot/agent/`, deployment left Running). No harness, grader, seed, or fixture file touched.

Review (P5 E2E, round 1). The ten graded rows re-derive from the committed run files: recommended sizes, right-size, supported/cited, words and cost all match the table, and the ten trace costs sum to $0.016383, the pilot-attributable total to $0.085383. Two record defects found. (1) Low, fixed: the results paragraph said the size-changed pair left "the other six prose cites" and the no-hardening note said "7 of 9 arm-2 cites need human prose-mapping and resolve to non-fit rows"; the run files carry nine human-mapped cites, two of them supported, so seven resolve to non-fit rows and all nine needed the mapping step. Both sentences now state that. (2) Low, fixed by documenting it: every run file's `deployment` field reads `memento-agent`, even on the ten graded runs, because the frozen builder (`record.go`, `NewRunFile`) writes that constant. The provenance is the trace spans — `invoke_agent Pilot Baseline` on the graded ten, `invoke_agent Memento Agent` on the discarded pair — and `bench/pilot/README.md` now states what the field is and is not. No count changes.

---

### T5.4: Freeze ★
```yaml
requires:   [T5.3]
fixture-ok: yes
size:       XS · mid
owns:       bench/FROZEN.md
status:     done
```
Record the commit and a hash of every shopper, catalog entry, prompt and grader file.

Freeze record: `bench/FROZEN.md` (21 sha256 rows: 6 histories, hidden answers by hash only, seed generator, harness prompt template, grader, pilot extraction rule + run schema + driver/grade commands, neutral instrument). Catalog: no Channel3 entry is an arm input (synthetic candidate). Instrument: neutral `pilot-baseline` for all three arms; reference agent rejected. Repeats fixed: 3 per shopper per arm. Deviation recorded in-file: verifier is a manifest + procedure, no gate-run script fits in owns (needs an orchestrator follow-up).

**Done when:** a script verifies the hashes and the gate runs it; no arm 3 run exists before this commit.

---

### T5.4b: Freeze-check script
```yaml
requires:   [T5.4]
fixture-ok: yes
size:       XS · mid
owns:       bench/freeze_check.sh, tools/check.sh
status:     done
```
Generate `bench/freeze_check.sh` from the FROZEN.md manifest (single testable place for the manifest-extraction) and invoke it from `tools/check.sh`, so the gate verifies frozen hashes on every run. One-line check.sh change only: call the script, no other gate behavior changes.

**Done when:** the gate fails on a tampered frozen file and passes on the frozen tree.

Worker notes (in-review): `bench/freeze_check.sh` extracts the ```manifest block from `bench/FROZEN.md` §1 via one commented `extract_manifest` function, pins the 21-row count (manifest edits fail), then `sha256sum -c` strict (tamper/missing/unreadable fail, nothing skipped). `tools/check.sh` gains exactly one line: `bench/freeze_check.sh || fail "freeze" ...` before gofmt. Tamper, missing-file, and row-count failures all proven (reversible mutations, restored); full gate green on the frozen tree.

---

### T5.5: All arms on the frozen set ★
```yaml
requires:   [T5.4, T2.6, T3.6]
fixture-ok: no
size:       M · frontier
owns:       bench/runs/
status:     done
```
All three arms on the frozen set, with the repeat count fixed in `FROZEN.md`.

**Done when:** every run is committed, complete, and verified against the frozen hashes.

Worker notes (in-review): 45/45 turns committed under `bench/runs/` (15 per arm, 3 repeats per shopper per arm) plus `scores.json` from the frozen grade command; freeze check green after all runs landed. Driver `bench/runs/cmd/t55/` (run-record scaffolding only: transport, cumulative cap, arm-3 memory fetch; prompts/extraction/schema/grading all frozen imports). Arm 3 served REAL recall per turn (live Lambo 0.10.1, native embedder, throwaway sessions erased after; settle polls committed in run notes). Counts: arm 1 right-size 0/15, arm 2 15/15, arm 3 graded 3/15 — all three are unclear-turn refusals mentioning M, an extractor artifact (no override-to-empty in the frozen rule); true arm-3 recommendations 0/15. Wrong-cause 0 everywhere. Spend $0.082 of $5.00 (incl. one overwritten $0.0018 shakedown turn, disclosed). Full table, spend ledger and deviations in the task report.

---

### T5.6: Results
```yaml
requires:   [T5.5]
fixture-ok: yes
size:       S · frontier
owns:       bench/README.md
status:     done
```
The results as counts, never percentages, each linked to its runs. The limits are stated up front: we wrote both the shoppers and the hidden answers, so this shows the mechanism on constructed cases and not real-world accuracy. If arm 2 matches arm 3 on a measure, the document says so.

**User decision:** which number the video leads with, chosen after seeing the results.

**Done when:** every claim links to a run; the limits are stated; the user has chosen the lead number.

Worker notes (in-review): `bench/README.md` written offline ($0 spend): limits up front; arm 1 right-size 0/15, arm 2 15/15 with 0 wrong-cause, arm 3 true recommendations 0/15 (12 outright + 3 informed refusals; graded 3/15 is the frozen-rule extractor artifact on `t55-a3-unclear-r{1,2,3}.json`) with 0 wrong-cause; correction measure nil across arms; cites 40/6 arm 2, 0/0 arms 1 and 3; tokens 1309/3606/2242 words; arm 2 beats arm 3 on right-size stated plainly. Three lead-number candidates with exact counts, none chosen (`LEAD NUMBER: [user picks]`). Every claim links to run files + `bench/runs/scores.json`.

---

### T5.7: Hardened comparison shoppers
```yaml
requires:   [T5.4, T1.3]
fixture-ok: yes
size:       M · frontier
owns:       seed/, testdata/seed/, bench/FROZEN.md
status:     done
```
Arm 2 scored a perfect 15/15 on the current set by correct causal algebra over clean histories — so per product.md, harden with cases a real history would contain: LONG NOISY histories (many mixed-signal rows where raw context drowns and beliefs must carry) and MULTI-BRAND histories (same shopper, several brands, cross-brand cut matching). Design 4 new shoppers (2 per class) with hidden answers, generate through the T1.3 generator path (new hidden answers, same seeded-marking + determinism rules; NO edits to existing shoppers or answers), and AMEND the freeze: append the new rows to FROZEN.md with fresh hashes (freeze amendment, same commit as the fixtures).

**Done when:** 4 new deterministic shoppers with hidden answers validate like T1.3's set (deterministic, marked, answers separated and unguessable from arm inputs); the freeze manifest covers them; all gates pass.

Worker notes (in-review): `seed/seed.go` gains four builders under the same GoldenSeed 7, appended after the existing six so every existing transaction id is byte-identical (proven: `git status` shows no modification to existing history files). New shoppers: `cmp-noisy-small` (13 rows over 30 days, three too-small in M buried in changed-mind/vague/fraud/damage/lost-parcel/kept/fits-well noise, answer L), `cmp-noisy-large` (13 rows over 28 days, three too-large in M amid seller-fault/kept-colour/vague noise plus another brand's fits-well, answer S), `cmp-brands-steady` (candidate Brand Q kept M + fits-well M with nothing against it while Brand P runs small twice, answer M), `cmp-brands-split` (candidate Brand R runs large twice while Brand T runs small once beside a kept M, answer S). `TestHardenedShoppersAreFair` pins event-time order + session extent, 10-row/3-fit/0.3-coverage on the noisy pair, per-brand single-direction fit evidence on the multi-brand pair, and never-fit on every non-fit classified row. A throwaway perfect-reasoner check through the frozen grader (deleted before commit) scored all four right with 0 wrong-cause and decisive answers. FROZEN.md manifest amended 21 to 25 rows (4 new histories, re-pinned answers + seed.go) with a plain amendment note; `bench/freeze_check.sh` count 21 to 25 (owns-boundary, see report); `bench/grader/grader_test.go` shape pin 5 to 9 (owns-boundary, see report). `./tools/check.sh` green, freeze-check green, $0 spend.

Review (P5 E2E, round 1). The freeze amendment checks out: the four new history hashes are appended, `answers.json` and `seed.go` are re-pinned, and the five original history rows keep the hashes they had before this commit (the `bench/FROZEN.md` diff at `3aeb829`). The two edits outside the task's `owns` — `bench/freeze_check.sh`'s row count 21 to 25 and `bench/grader/grader_test.go`'s hidden-answer shape pin 5 to 9 — are both correct against the amended manifest (25 rows in the block, nine answers, the count pin fails a 26-row edit). Finding: Low, owner: orchestrator. Both were disclosed in these notes rather than claimed silently, but rule 2 wants an owns extension or a follow-up task on the record; propose recording the extension beside T5.7, or minting a task that owns `bench/freeze_check.sh` and `bench/grader/grader_test.go` so a later manifest change has an owner.

---

### T5.8: Re-run on the hardened set
```yaml
requires:   [T5.7]
fixture-ok: no
size:       M · frontier
owns:       bench/runs/
status:     done
```
Re-run all three arms on the hardened shoppers only (repeat count per freeze), with the real `recall_for_purchase` + pending shape for arm 3, neutral instrument, cumulative $5 cap, publish-everything rule.

**Done when:** every hardened-shopper run is committed, complete, and verified against the amended freeze.

Worker notes (in-review): 12/12 turns committed under `bench/runs/t58/` (4 hardened shoppers x 3 arms x 1) plus `scores.json` from the frozen grade command; freeze check green before the first turn and after the last. Driver `bench/runs/cmd/t58/` (run-record scaffolding only, T5.5 shape; cumulative cap counts prior T5.5 committed spend). Counts: arm 1 right-size 0/4 (clean refusals), arm 2 4/4 with 0 wrong-cause, arm 3 2/4 (both noisy-shopper Venerable beliefs recommend; both multi-brand Candidate beliefs refuse) with 0 wrong-cause. Spend $0.038 this task, ~$0.20 cumulative incl. pilot + T5.5, of $5.00. Full table, spend ledger and deviations in the task report. The two arm-3 run files that carry a belief, `t58-a3-noisy-large-r1.json` and `t58-a3-noisy-small-r1.json`, record its subject as `Kind`/`ID`/`Name`, because the driver marshals the served contract value directly and `domain.Subject` had no JSON tags when the runs were made; T7.6 added the tags, and those files stay as recorded rather than being rewritten to the tagged shape (T7.7: `bench/README.md`, "The round-1 recordings predate the subject tags").

Review (P5 E2E, round 1). The 12 scores re-derive from the committed run files: the frozen grade command reproduces `bench/runs/t58/scores.json` byte for byte, and the counts above hold (arm 1 0/4, arm 2 4/4, arm 3 2/4, 0 wrong-cause, $0.038118 trace cost). One claim does not: in both multi-brand cells the served response is `"beliefs": []` with guidance "No recommendation yet: too little evidence" (`t58-a3-brands-split-r1.json`, `t58-a3-brands-steady-r1.json`), so no Candidate belief was served or refused — `recall_for_purchase` serves Candidate standing (`internal/api/server/recall.go`), so a Candidate belief would have shown in the response. The seeded history does hold two Brand R fit returns, so a belief should have existed; why recall served none is not in the record, because the t58 files carry the response but no settle note. `bench/README.md` gains a "Hardened set" section with the counts and this limit, and the "not built" hardening sentence there is gone. Finding: Medium, owner: a new `T5.8b` (owning `bench/runs/`) to re-run the two multi-brand arm-3 cells with the settle note and pre/post recall recorded, so the belief's standing at recall time is in the record; until then the record says the refusal happened and does not claim a cause.

---

### T5.9: Correction-round comparison
```yaml
requires:   [T5.8]
fixture-ok: no
size:       M · frontier
owns:       bench/runs/
status:     done
```
Every shopper, every arm, one correction turn: after the arm's first recommendation (or refusal), the shopper says "Actually I take L now" (uniform text naming an explicit size that differs from what the arm just gave; where the arm already recommended L, the text names M instead so the correction always contradicts). Then re-ask once. Grade with the frozen grader: honoured-on-next-turn, belief/state kept (arm 3: prior belief retained beside the correction, per the decision-vs-belief rule), history intact (arm 2), no fabrication. This is the signature move raw history has no mechanism for — run it honestly and report whatever happens, including arm 2 honouring it too.

**Done when:** 27 correction turns committed (9 shoppers × 3 arms), graded, and verified against the freeze; any new input files hashed into the manifest by amendment in the same commit.

Worker notes (in-review): 27/27 turns committed under `bench/runs/t59/` (one per cell, first turns reused byte-for-byte from the committed T5.5 r1 / T5.8 r1 files, never re-run) plus `t59-scores.json` from the new grade mode over the frozen grader; no frozen file touched, freeze check green, gate green. Driver `bench/runs/cmd/t59/` (drive + grade modes, T5.8 shape; uniform rule: effective-L → "Actually I take M now", everything else → "Actually I take L now", documented per run). Counts: corrections honoured 9/9 in every arm (27/27), wrong-cause 0 everywhere — corrections do not discriminate the arms; arm 2 honours corrections just as well as arm 3, stated plainly. The arms differ in mechanism, not outcome: arm 3 recorded a real `record_correction` in only 2/9 cells (the noisy pair, Venerable beliefs kept with standing + evidence intact, contradicted with the override beside them, correction-basis guidance — asserted in code per turn); in 7/9 cells recall served no belief so no `record_correction` call exists on the agent-visible surface and none was made (recorded per run; the correction travelled in conversation text only, like arm 1). One frozen-check false positive overruled with a recorded machine-verifiable exception (a3-noisy-large quotes its own live `corr_000001` from its prompt memory block). Spend: see handoff report (4 orphaned traces from platform stalls disclosed, all honoured, none graded). No new input files, so the freeze manifest gains no rows. The two arm-3 run files that carry a belief, `t59-a3-noisy-large.json` and `t59-a3-noisy-small.json`, record its subject as `Kind`/`ID`/`Name` for the same reason as T5.8's pair: the driver marshals the served contract value directly, and `domain.Subject` had no JSON tags when the runs were made. T7.7 leaves round 1 as frozen history and records the decision with its reason in `bench/README.md`; a re-run of this driver now emits `kind`/`id`/`name` and differs from those files in exactly that key casing.

Review (P5 E2E, round 1). Re-derived from the committed files: 27 rows, `correction_honoured` true on all 27 (9 of 9 per arm), `wrong_cause` 0, `correction_recorded` true on exactly the two noisy arm-3 cells and false with `no_belief_reason` on the other seven, and every t59 first turn byte-identical to its `source_run_id` in prompt, reply and trace cost. The overruled contamination hit checks out mechanically: the only hit, `corr_`, appears three times in that turn's own second prompt, so it is a self-quote and not another shopper's memory. `bench/README.md`'s Corrections section now states it, so the one graded turn carrying `contaminated: true` is no longer left to the task report alone. The t59 driver's `secondPromptArm3` comment claimed "the same transcript tail as the other arms"; it now states the one wording that differs ("the layer memory above"), so behaviour and comment agree.

---

## Exit Criteria

- [x] The harness and grader work on fixtures (T5.1, T5.2).
- [x] The baseline was piloted before any arm 3 run (T5.3).
- [x] The set was frozen (T5.4).
- [x] All arms ran and are published (T5.5).
- [x] Results reported as counts, with limits (T5.6).

Ticked by the P5 E2E review. The harness and grader tests are green in the
gate (`bench/harness`, `bench/grader`), and both grader rules that carry a
measure were mutation-checked (the fit-feedback branch and the wrong-cause
exclusion each fail a named test when removed). The pilot's twelve run files
name arms 1 and 2 only, and the freeze commit `b3bb59c` is an ancestor of the
first arm-3 commit `8a34cd2`. `bench/freeze_check.sh` verifies the 25 rows and
fails on a one-byte tamper (checked, restored). All three scores files
reproduce byte for byte from the committed run files with the frozen grader
(45 + 12 + 27 rows), and every count in `bench/README.md` re-derives from them.
