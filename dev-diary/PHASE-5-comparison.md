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

---

### T5.2: The grader
```yaml
requires:   [T1.3, T5.1]
fixture-ok: yes
size:       M · frontier
owns:       bench/grader/
status:     not-started
```
Scores a run on the five measures in product.md, as counts: right size first time against the hidden answer; wrong-cause errors; corrections honoured on the very next recommendation; citations that really support the recommendation; tokens per recommendation.

A citation is checked mechanically where it can be (the cited evidence exists and has the right reason category). Any model judgement used is logged as a model's opinion, never as a measurement.

**Done when:** hand-built good and bad runs score as expected for every measure.

---

### T5.3: The baseline pilot ★
```yaml
requires:   [T5.1, T5.2]
fixture-ok: no
size:       M · frontier
owns:       bench/pilot/
status:     not-started
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
