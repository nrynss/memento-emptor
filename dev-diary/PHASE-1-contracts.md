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
status:     claimed:t11-domain
```
The types the whole product shares:

- **Evidence:** outcome, item and variant, reason, each kept separate; source; transaction link; seeded mark.
- **Reason categories**, and which belief kind each counts toward. Fit reasons count toward **fit**. Non-arrival and similar count toward **seller reliability**. A reason matching neither counts toward nothing.
- **PayPal dispute code mapping:** a fixed table from each reason code to a category.
- **Belief:** kind, subject (brand, or seller), value, status (Claimed, Building, Earned, Out of true) and its mapping to Lambo's states.
- **The write rule:** a table from each signal in product.md to `record_action` or `derive`, as code that can be tested.

`docs/architecture.md`: the service, the data flow from webhook to belief, what lives in Lambo versus Elastic versus the app's own store, configuration, and the dependencies.

**Done when:** a table test covers every signal, every reason category and every dispute code from `research/paypal.md`; the document describes the flow as built from the research, not from the brief.

---

### T1.2: The interface ★
```yaml
requires:   [T1.1, T0.8]
fixture-ok: yes
size:       M · frontier
owns:       internal/api/contract/, docs/interface.md
status:     not-started
```
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
status:     not-started
```
A generator for seeded shopper histories, every row marked. Event times spread across the period so promotion's gates are met honestly, as T0.3 established.

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
status:     not-started
```
Create the demo shopper's history as real sandbox orders, captures and refunds. Purchases and refunds exist in PayPal, so the evidence comes from PayPal. Where the sandbox can't backdate, record how seeded event times are applied on the layer's side, and mark them.

**Done when:** the demo shopper's transactions can be read back from the sandbox; the transcript is in the handoff log.

---

### T1.4: Fixtures and fakes ★
```yaml
requires:   [T1.1, T1.2]
fixture-ok: yes
size:       M · mid
owns:       internal/fakes/, testdata/replay/
status:     not-started
```
Fake clients for PayPal, Jev, Elastic and Lambo that replay P0's recordings, behind the same interfaces the real clients will implement. A replay of the demo shopper's whole story, from first webhook to final ledger state, as fixtures that P2, P3 and P4 can each use alone.

**Done when:** the gate replays the story with no key and no network; a test fails if a recording contains a key pattern.

---

## Exit Criteria

- [ ] Domain model, write rule and architecture in place (T1.1).
- [ ] Interface frozen, documented for builders (T1.2).
- [ ] Demo and comparison shoppers generated, with hidden answers kept apart (T1.3).
- [ ] The demo shopper exists in the sandbox (T1.3b).
- [ ] The whole story replays offline (T1.4).
