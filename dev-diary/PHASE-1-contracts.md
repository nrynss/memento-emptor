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

Final names and schemas for the proposed surface in product.md: `recall_for_purchase`, `record_correction`, `pending_questions`, `record_reason`, `record_feedback`, `explain_belief`. Served over MCP, so that any agent on PayPal's toolkit can call them, and shaped so that T0.8's findings on Astropods work.

The contract `recall_for_purchase` must honour: beliefs that apply, each with its status and evidence, and any correction that overrides them, marked so that an agent cannot read the belief without seeing the override. The ledger's read endpoints are defined here too.

`docs/interface.md` is written for the audience, a developer building on PayPal's Agent Toolkit: what to call, when, and what comes back.

**Done when:** every call has a schema, an example request and an example response in `testdata/api/`; the document's examples validate against the schemas in the gate.

Notes (T1.2, in-review): `internal/api/contract/` holds the frozen wire
types, the nine MCP tool descriptors, and the gate test: a focused schema
checker proving every example in `testdata/api/` validates, every example
matches its Go type with unknown fields rejected, and every validating
block in `docs/interface.md` equals its fixture. Nineteen schemas and
twenty examples cover the six layer calls and three ledger reads on one
consistent shopper state. Recall carries both the ledger status and the
underlying standing with the warning inline, plus the correction twice.
Full gate result in the report.

Superseded by the pivot (T7.1, T7.4), which the phase-7 notes carry: the
frozen surface is now eleven tools (eight layer calls and three ledger
reads), 23 schema files and 33 examples. `product_fit` and `set_pooling`
are part of it, the pool and per-item `precedence` are required, and the
document's blocks are byte-pinned.

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
status:     done
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

### T1.1b: Not-as-described disputes, and unknown codes that must not fail ingest
```yaml
requires:   [T1.1, T2.1]
fixture-ok: yes
size:       S · mid
owns:       internal/domain/, internal/ingest/, product.md, testdata/api/
status:     done
```
The P1 review found `product.md` claims seller reliability is earned from "Non-arrival **and not-as-described** disputes, by PayPal reason code", but the frozen table has no not-as-described code: `CategoryForDispute` refuses `MERCHANDISE_OR_SERVICE_NOT_AS_DESCRIBED`, and `internal/ingest` fails the whole dispute delivery on that error, so a real not-as-described dispute is rejected rather than stored. T1.1's Done-when never observed the code, which is why this survived phase validation.

Decide: add the code (with its own category, since `not_arrived` would mislabel it in the merchant counts) or amend `product.md` to non-arrival only. Either way, an unknown dispute code must store the dispute with no classified reason and must not fail the delivery's idempotent retry; a reason is not a cause when the code is unknown.

**Done when:** a table test pins the chosen disposition of a not-as-described code; an unknown dispute code stores the dispute, writes no belief, and does not return an error that drops the delivery; the product doc and the table agree; the gate passes.

Notes (T1.1b, in-review): Option (a). `MERCHANDISE_OR_SERVICE_NOT_AS_DESCRIBED` now maps to the existing `damaged_or_defective` category, so the `product.md` sentence holds and the reason-counts shape stays as frozen. No new category: the counts view is one contract field per category, copied in the API contract and both frontends, and the code needs no field of its own, because the goods arrived and were wrong, which is the seller class damage already carries. `not_arrived` was refused: the parcel did arrive, so that label would state the wrong fact. `CategoryForDispute` still refuses a code outside the table, and `internal/ingest` no longer fails the delivery on that refusal: the dispute is stored with the unknown code as its text and `none` as its category, so it counts toward nothing and no belief is written, and the retry answers as a duplicate. A dispute that carries no code at all stores with no reason. The table tests pin the not-as-described disposition, including that it must never read as non-arrival. Full gate result in the report.

---

### T1.1c: The live ingest driver and the unrecorded dispute code
```yaml
requires:   [T1.1b]
fixture-ok: no
size:       XS · mid
owns:       tests/live/ingest/, testdata/paypal/
status:     done
```
T1.1b made served ingest store an unknown dispute code instead of failing, but `tests/live/ingest`'s source read still returns the `CategoryForDispute` error, so a live run on an unrecognised code aborts where the served path now stores it. No recorded sandbox dispute carries `MERCHANDISE_OR_SERVICE_NOT_AS_DESCRIBED` either, so the new mapping's spelling is documented, not observed. Align the driver with the served path and record one sandbox dispute with that code if the sandbox can produce one; if it cannot, record that limit.

**Done when:** the live driver stores an unknown code the way served ingest does; the not-as-described mapping is either observed in a recording or its absence is recorded; the gate passes.

Notes (T1.1c, in-review): `tests/live/ingest/main.go` maps the dispute read's
code the way served ingest does: `ErrUnknownDisputeCode` yields `CategoryNone`
with the code kept as the reason's text, so the read no longer aborts on a code
outside the frozen table. The row assertions now derive the expected category
and counts from the live case's own code instead of assuming non-arrival; a
case carrying no code asserts the row carries no reason. The live run re-read
the T1.3b refund and non-arrival dispute, verified both signatures live, and
printed the code-to-category line. The not-as-described spelling stays
unobserved: `POST /v1/customer/disputes` with that reason refuses a full item
shape on `TRANSACTION_NOT_ELIGIBLE` (recorded in
`testdata/paypal/dispute-create-not-as-described-not-eligible.json`), the
reason-only shape fails schema before eligibility is reached
(`MISSING_ISSUE_TYPE`), and naming the capture as `seller_transaction_id`
returns 500 `INTERNAL_SERVER_ERROR`. `probe_not_as_described.py` reproduces
the probe. The shopper-side resolution-center filing is the path that produced
the demo's non-arrival case and is a new sandbox record, so this task did not
file one. Spend: sandbox calls only, $0; no new sandbox records.

---

### T1.1d: One unknown-code rule for ingest and the live driver
```yaml
requires:   [T1.1b]
fixture-ok: yes
size:       XS · mid
owns:       internal/domain/, tests/live/ingest/
status:     claimed:t11d-unknown-rule
```
The T1.1c review found the unknown-dispute-code disposition now exists in two places: `internal/ingest.handleDispute` and the live driver's `disputeCategory`. A rule change would drift (loudly, via an assertion mismatch, but drift). Export the disposition from `internal/domain` (or a shared helper) and have both call it.

**Done when:** one function decides an unknown code's category and error, and ingest and the driver both use it; the existing tests pass unchanged; the gate passes.

---

## Exit Criteria

- [x] Domain model, write rule and architecture in place (T1.1).
- [x] Interface frozen, documented for builders (T1.2).
- [x] Demo and comparison shoppers generated, with hidden answers kept apart (T1.3).
- [x] The demo shopper exists in the sandbox (T1.3b).
- [x] The whole story replays offline (T1.4).
