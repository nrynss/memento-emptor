# P1: Contracts

The shared types, the interface, the seeded shoppers and the offline fakes. P1 is serial and exact: everything after it codes against this. Contracts freeze when it ends.

Read first: [`../product.md`](../product.md), and every file in `research/`.

---

### T1.1: The domain model and architecture ★
```yaml
requires:   [T0.2, T0.3, T0.4, T0.6, T0.7]
fixture-ok: yes
size:       M · frontier
owns:       internal/domain/, docs/architecture.md
status:     done
```
The types the whole product shares:

- **Evidence:** outcome, item and variant, reason, each kept separate; source; transaction link; seeded mark.
- **Reason categories**, and which belief kind each counts toward. Fit reasons count toward **fit**. Non-arrival and similar count toward **seller reliability**. A reason matching neither counts toward nothing.
- **PayPal dispute code mapping:** a fixed table from each reason code to a category.
- **Belief:** kind, subject (brand, or seller), value, status (Claimed, Building, Earned, Out of true) and its mapping to Lambo's states.
- **The write rule:** a table from each signal in product.md to `record_action` or `derive`, as code that can be tested.

`docs/architecture.md`: the service, the data flow from webhook to belief, what lives in Lambo versus Elastic versus the app's own store, configuration, and the dependencies.

**Done when:** a table test covers every signal, every reason category and every dispute code from `research/paypal.md`; the document describes the flow as built from the research, not from the brief.

Notes (T1.1, in-review): `internal/domain/` holds evidence, the reason
categories with their belief-kind mapping, the three-code dispute table,
belief statuses with the graph-state round trip, the five-signal write
rule, and the swappable embedder contract. Table tests cover all 7
categories, all 5 signals and all 3 observed codes; unknowns map to an
error that counts toward nothing. `docs/architecture.md` describes the
webhook-to-belief flow as built. Full gate result in the report.

---

### T1.2: The interface ★
```yaml
requires:   [T1.1, T0.8]
fixture-ok: yes
size:       M · frontier
owns:       internal/api/contract/, docs/interface.md
status:     done
```

Notes (T1.2, in-review): `internal/api/contract/` holds the frozen wire
types, the nine MCP tool descriptors, and the gate test: a focused schema
checker proving every example in `testdata/api/` validates, every example
matches its Go type with unknown fields rejected, and every validating
block in `docs/interface.md` equals its fixture. Nineteen schemas and
twenty examples cover the six layer calls and three ledger reads on one
consistent shopper state. Recall carries both the ledger status and the
underlying standing with the warning inline, plus the correction twice.
Full gate result in the report.
Final names and schemas for the proposed surface in product.md: `recall_for_purchase`, `record_correction`, `pending_questions`, `record_reason`, `record_feedback`, `explain_belief`. Served over MCP, so that any agent on PayPal's toolkit can call them, and shaped so that T0.8's findings on Astropods work.

The contract `recall_for_purchase` must honour: beliefs that apply, each with its status and evidence, and any correction that overrides them, marked so that an agent cannot read the belief without seeing the override. The ledger's read endpoints are defined here too.

`docs/interface.md` is written for the audience, a developer building on PayPal's Agent Toolkit: what to call, when, and what comes back.

**Done when:** every call has a schema, an example request and an example response in `testdata/api/`; the document's examples validate against the schemas in the gate.

---

### T1.3: Seeded shoppers ★
```yaml
requires:   [T1.1, T0.3]
fixture-ok: yes
size:       M · frontier
owns:       seed/, testdata/seed/
status:     done
```
A generator for seeded shopper histories, every row marked. Event times spread across the period so promotion's gates are met honestly, as T0.3 established.

Notes (T1.3, in-review): `seed/` holds the deterministic generator
(`Generate` from a fixed seed; `GoldenSeed` reproduces the committed
fixtures byte for byte) plus tests for determinism, the mark on every
row, the demo shopper's promotion shape (three fit returns on distinct
days, coverage above the 0.3 bar, dispute counting toward the seller
only), wrong-cause rows never counting toward fit, hidden answers kept
off the arm path, and golden fixtures. `testdata/seed/histories/`
holds the demo shopper and five comparison shoppers (broken-item
refund, non-arrival dispute, changed mind, genuine size change, vague
answer); `testdata/seed/hidden/answers.json` holds the right size per
comparison shopper. Event time is relative day offsets, and links mint
only the ids each outcome kind actually has. Full gate result in the
report.

- **The demo shopper:** the three fit returns, realistic background purchases, the dispute later.
- **The comparison shoppers:** a set with varied histories, including cases built to tempt a wrong cause (a refund for a broken item, a non-arrival dispute, a changed mind, a shopper whose size genuinely changed).
- **Hidden answers:** the right size for each comparison shopper, stored apart from anything an arm can read.

**Done when:** the generator is deterministic from a seed; every row carries the mark; hidden answers are in their own path, and a test proves no arm-facing output contains them.

---

### T1.3b: The demo shopper in the sandbox
```yaml
requires:   [T1.3, T0.4]
fixture-ok: no
size:       S · mid
owns:       tests/live/seed/
status:     in-review
```
Create the demo shopper's history as real sandbox orders, captures and refunds. Purchases and refunds exist in PayPal, so the evidence comes from PayPal. Where the sandbox can't backdate, record how seeded event times are applied on the layer's side, and mark them.

**Done when:** the demo shopper's transactions can be read back from the sandbox; the transcript is in the handoff log.

Notes (T1.3b, in-review): `tests/live/seed/` holds the live driver (`run.sh` create/wait/capture/refund/read-back, `approve.py` scripted shopper approval, `dispute.py` resolution-center filing, `README.md` with the cycle table and anchor rule). No `*_test.go` under `tests/live/`, so the gate has nothing to run there. All six cycles read back from the sandbox: two kept captures, three refunded fit returns with the reason in words on each refund record, one open non-arrival dispute with the reason code top-level and per item. Seeded day offsets (1, 2, 3, 6, 9, 11 of 11) apply on the layer's side via `Row.SeededEvidence(anchor)`; real ids live only in the task report, placeholders in committed files. Full gate result in the report.

---

### T1.4: Fixtures and fakes ★
```yaml
requires:   [T1.1, T1.2]
fixture-ok: yes
size:       M · mid
owns:       internal/fakes/, testdata/replay/
status:     done
```
Fake clients for PayPal, Jev, Elastic and Lambo that replay P0's recordings, behind the same interfaces the real clients will implement. A replay of the demo shopper's whole story, from first webhook to final ledger state, as fixtures that P2, P3 and P4 can each use alone.

**Done when:** the gate replays the story with no key and no network; a test fails if a recording contains a key pattern.

Notes (T1.4, in-review): `internal/fakes/` holds the seam (four interfaces the real P2 clients implement) plus recording-backed fakes and the story runner; `testdata/replay/` holds the manifest, the nine-step log, and the per-phase fixtures (`p2-inputs`, `p3-turns`, `p4-ledger`), with `p4-ledger` copied verbatim from the frozen contract snapshots. The replay leaves the pending refund pending: it answers `q_9f04ab` with the recorded vague phrase ("not great", unclear 0.94, ask again), since a clear fit answer would add a fourth fit row the frozen snapshots predate. Full gate result in the report.

---

## Exit Criteria

- [ ] Domain model, write rule and architecture in place (T1.1).
- [ ] Interface frozen, documented for builders (T1.2).
- [ ] Demo and comparison shoppers generated, with hidden answers kept apart (T1.3).
- [ ] The demo shopper exists in the sandbox (T1.3b).
- [ ] The whole story replays offline (T1.4).
