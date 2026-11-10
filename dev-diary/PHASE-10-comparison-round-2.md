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
status:     claimed:t101-arm2
```
Record exactly what the Agent Toolkit returns for a shopper's history: transaction listing, line items, refund and dispute status, dispute reason codes. Start from T0.4's recordings (`research/paypal.md`) and fill any gap with sandbox reads. Transaction Search is enabled on the sandbox app and the token carries `https://uri.paypal.com/services/reporting/search/read`; the reporting read must use a date range of 31 days or less (`400` otherwise). Stop if the scope disappears before the run. Write the extraction that turns that, and only that, into arm 2's prompt input: no refund reason, no classification, no signal strength, nothing the layer produced. No model spend.

**Done when:** the recorded responses and the extraction, with a test, are in `bench/round2/arm2/`. A test proves the extraction's output has no field the Agent Toolkit didn't return.

---

### T10.2: Round 1, relabelled
```yaml
requires:   []
fixture-ok: yes
size:       XS · light
owns:       bench/README.md
status:     in-review
```
In `bench/README.md`, the round 1 arm called "raw history" is renamed **Memento evidence without beliefs**, with one paragraph saying why: its rows carried the layer's classified reasons, signal strength and write kind. No count changes, and no run file is touched. A pointer to round 2 is added.

**Done when:** the relabel and the pointer are in, and the freeze check still passes.

Worker notes (in-review): `bench/README.md` gains a "Round 1, relabelled" section between Limits and Setup that names the arm **Memento evidence without beliefs**, says why (its prompt carried the layer's evidence rows, so each row already held the classified reason, the signal strength and the write kind; only the beliefs over them were absent), and points at round 2 (`product.md` "The comparison" and this phase document). Every other mention uses the new name: title, Setup arms bullet, headline table row, headline prose, the "what this does and does not prove" paragraph, and lead-number candidates 1 and 3. No count changed, and no run file, scores file or freeze input was touched; `bench/freeze_check.sh` green (25 rows) and `./tools/check.sh` green.

Surprises for the next agent: the committed round 1 run files keep `arm_name: "raw-history"` and the prompt line "Memory: raw history", because they are frozen T5.5 output and the manifest pins `bench/harness/harness.go`; the report label cannot be pushed into them, so a reader comparing this document to a run file will meet both names and should change neither. `bench/pilot/README.md` (the T5.3 record) and `dev-diary/PHASE-5-comparison.md` (its header and T5.9's task text) still say raw history; both are outside this task's owns. Separately, this round 1 record still reports the correction measure as nil and does not point at T5.9's 27 correction turns (9 of 9 honoured in every arm, `bench/runs/t59/t59-scores.json`); fixing that adds counts, which this task forbids, so it needs a follow-up task owning `bench/README.md` to add the correction-round pointer.

Review fixes: the Setup bullet's manifest description was stale since T5.6 — it said 21 sha256 rows and 5 arm-facing histories while `bench/freeze_check.sh` pins 25 and the T5.7 amendment added 4 histories — so it now reads 25 and 9. The relabel paragraph and the Setup arms bullet say "evidence rows" rather than "per-purchase rows" (some rows are shopper statements), and the relabel paragraph states that the frozen run files keep the old `arm_name` and prompt line, so the report is not mistaken for a rewrite of them. The round 2 pointer says "tasks and freeze" rather than "frozen inputs", because no round 2 freeze exists yet (T10.3 owns it). The gate and the freeze check were re-run green at the review tip.

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
