# P5: Comparison

Three arms, with the same agent, model, shoppers and catalog: no memory, raw PayPal history, and Memento Emptor. Arm 2 is the baseline that matters: "why not just give my agent the transaction history?"

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

**User decision (PARKED by user): the lead-number pick is blocked until the comparison is decisive — i.e. until some honest run shows arm 3 winning where raw history fails (earned-level evidence and/or a correction round). No number is chosen until then. This gates T6.4 (video script), not T5.6: the results document is complete as written.

**Done when:** every claim links to a run; the limits are stated.

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

---

### T5.8: Re-run on the hardened set
```yaml
requires:   [T5.7]
fixture-ok: no
size:       M · frontier
owns:       bench/runs/
status:     claimed:t58-rerun
```
Re-run all three arms on the hardened shoppers only (repeat count per freeze), with the real `recall_forphurchase` + pending shape for arm 3, neutral instrument, cumulative $5 cap, publish-everything rule.

**Done when:** every hardened-shopper run is committed, complete, and verified against the amended freeze.

---

## Exit Criteria

- [ ] The harness and grader work on fixtures (T5.1, T5.2).
- [ ] The baseline was piloted before any arm 3 run (T5.3).
- [ ] The set was frozen (T5.4).
- [ ] All arms ran and are published (T5.5).
- [ ] Results reported as counts, with limits (T5.6).
