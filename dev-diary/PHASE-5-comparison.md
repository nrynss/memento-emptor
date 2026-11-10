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
status:     claimed:t53-pilot
```
Arms 1 and 2 only, before any arm 3 run, to find out what a fair baseline does. If arm 2 scores perfectly, harden the shoppers with cases a real history would contain, or record that the lead is elsewhere. Don't weaken the baseline, and don't tune the cases against the layer.

**User decision:** the spend, before the first run.

**Done when:** the pilot's results are recorded, with any change to the shoppers and the reason for it.

---

### T5.4: Freeze ★
```yaml
requires:   [T5.3]
fixture-ok: yes
size:       XS · mid
owns:       bench/FROZEN.md
status:     not-started
```
Record the commit and a hash of every shopper, catalog entry, prompt and grader file.

**Done when:** a script verifies the hashes and the gate runs it; no arm 3 run exists before this commit.

---

### T5.5: All arms on the frozen set ★
```yaml
requires:   [T5.4, T2.6, T3.6]
fixture-ok: no
size:       M · frontier
owns:       bench/runs/
status:     not-started
```
All three arms on the frozen set, with the repeat count fixed in `FROZEN.md`.

**Done when:** every run is committed, complete, and verified against the frozen hashes.

---

### T5.6: Results
```yaml
requires:   [T5.5]
fixture-ok: yes
size:       S · frontier
owns:       bench/README.md
status:     not-started
```
The results as counts, never percentages, each linked to its runs. The limits are stated up front: we wrote both the shoppers and the hidden answers, so this shows the mechanism on constructed cases and not real-world accuracy. If arm 2 matches arm 3 on a measure, the document says so.

**User decision:** which number the video leads with, chosen after seeing the results.

**Done when:** every claim links to a run; the limits are stated; the user has chosen the lead number.

---

## Exit Criteria

- [ ] The harness and grader work on fixtures (T5.1, T5.2).
- [ ] The baseline was piloted before any arm 3 run (T5.3).
- [ ] The set was frozen (T5.4).
- [ ] All arms ran and are published (T5.5).
- [ ] Results reported as counts, with limits (T5.6).
