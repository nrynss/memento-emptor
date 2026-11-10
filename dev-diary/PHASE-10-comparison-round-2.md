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
status:     not-started
```
Record exactly what the Agent Toolkit returns for a shopper's history: transaction listing, line items, refund and dispute status, dispute reason codes. Start from T0.4's recordings (`research/paypal.md`) and fill any gap with sandbox reads. The reporting read needs the app's Transaction Search permission: confirm the OAuth token carries `https://uri.paypal.com/services/reporting/search/read` before recording, and stop if it does not. Write the extraction that turns that, and only that, into arm 2's prompt input: no refund reason, no classification, no signal strength, nothing the layer produced. No model spend.

**Done when:** the recorded responses and the extraction, with a test, are in `bench/round2/arm2/`. A test proves the extraction's output has no field the Agent Toolkit didn't return.

---

### T10.2: Round 1, relabelled
```yaml
requires:   []
fixture-ok: yes
size:       XS · light
owns:       bench/README.md
status:     not-started
```
In `bench/README.md`, the round 1 arm called "raw history" is renamed **Memento evidence without beliefs**, with one paragraph saying why: its rows carried the layer's classified reasons, signal strength and write kind. No count changes, and no run file is touched. A pointer to round 2 is added.

**Done when:** the relabel and the pointer are in, and the freeze check still passes.

---

### T10.3: Round 2 shoppers, population and freeze ★
```yaml
requires:   [T9.1, T10.1, T8.5]
fixture-ok: yes
size:       M · frontier
owns:       bench/round2/ (except arm2/ and runs/), seed/round2.go, seed/round2_test.go
status:     not-started
```
The four cases in product.md, at least two shoppers each, with hidden answers kept apart as in T1.3:

1. **New to the brand:** no history with the brand; the pool knows it runs off-size.
2. **Unlike the crowd:** the shopper's own evidence disagrees with the pool, and the right answer is theirs.
3. **Noisy refunds:** mixed causes where a refund status with no reason can't tell fit from the rest.
4. **Correction, then another agent:** a correction to agent A; agent B asks next, with PayPal access but not A's conversation.

Each case states, before any run, why it separates the arms, and is checked for fairness: a perfect reasoner given each arm's input gets the answer whenever that input holds it. Arms: 1 no memory; 2 PayPal history (T10.1); 3 Memento, shopper only (no pool); 4 Memento, full. The same neutral instrument, model and catalog in every arm. Then freeze: `bench/round2/FROZEN.md` with hashes of the shoppers, answers, population, prompts, extraction and grader, and a freeze check the gate runs.

**Done when:** the shoppers, answers and population are generated and tested for fairness. The freeze manifest and check are committed, and the gate runs the check. No arm 3 or 4 run exists before this commit.

---

### T10.4: All four arms ★
```yaml
requires:   [T10.3, T9.5, T9.6]
fixture-ok: no
size:       M · frontier
owns:       bench/round2/runs/
status:     not-started
```
Every shopper through every arm, three repeats, plus the cross-session and cross-agent correction turns for case 4 (agent B is T9.5's agent for arms 3 and 4; for arms 1 and 2 it is the same instrument with no shared conversation). Arm 4 uses the live layer and the frozen population. Per-run token and spend caps as in P5. Budget: $5.00 cumulative. Run one cell first and check its cost.

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

## Exit Criteria

- [ ] Arm 2 gets exactly what the Agent Toolkit returns, and nothing the layer made (T10.1).
- [ ] Round 1 is relabelled honestly (T10.2).
- [ ] Four fair cases frozen before any arm 3 or 4 run (T10.3).
- [ ] Every run published and graded (T10.4).
- [ ] Results as counts, the headline picked by the user (T10.5).
