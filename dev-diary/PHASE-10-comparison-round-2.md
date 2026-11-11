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
owns:       bench/round2/ (except arm2/ and runs/), seed/round2.go, seed/round2_test.go
status:     claimed:t103-freeze
```
The four cases in product.md, at least two shoppers each, with hidden answers kept apart as in T1.3:

1. **New to the brand:** no history with the brand; the pool knows it runs off-size.
2. **Unlike the crowd:** the shopper's own evidence disagrees with the pool, and the right answer is theirs.
3. **Noisy refunds:** mixed causes where a refund status with no reason can't tell fit from the rest.
4. **Correction, then another agent:** a correction to agent A; agent B asks next, with PayPal access but not A's conversation.

Each case states, before any run, why it separates the arms, and is checked for fairness: a perfect reasoner given each arm's input gets the answer whenever that input holds it. Arms: 1 no memory; 2 PayPal history (T10.1); 3 Memento, shopper only (no pool); 4 Memento, full. The same neutral instrument, model and catalog in every arm. Arm 2's generated orders set `custom_field`, its refund notes stay fit-neutral, and line items or order ids are supplied to every arm or to none. If any shopper has a multi-item order, decide the fit-row key for it here. Then freeze: `bench/round2/FROZEN.md` with hashes of the shoppers, answers, population, prompts, extraction and grader, and a freeze check the gate runs.

**Done when:** the shoppers, answers and population are generated and tested for fairness. The freeze manifest and check are committed, and the gate runs the check. No arm 3 or 4 run exists before this commit.

The P2 review found `bench/runs/cmd/t{55,58,59}` mirror the direction rule but not the rival-birth threshold: every opposite-direction row lands unattached with no streak, while the driver claims it lands "below the rival-birth threshold". No frozen history reaches three consecutive opposites, so no published run is wrong, but a frozen-set extension would diverge from the layer. Mirror `RivalBirthThreshold` in the round-2 arms, or state the limitation in the run's notes.

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

- [x] Arm 2 gets exactly what the Agent Toolkit returns, and nothing the layer made (T10.1).
- [x] Round 1 is relabelled honestly (T10.2).
- [ ] Four fair cases frozen before any arm 3 or 4 run (T10.3).
- [ ] Every run published and graded (T10.4).
- [ ] Results as counts, the headline picked by the user (T10.5).
