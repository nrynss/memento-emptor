# P4: Evidence ledger

Where a person can see what the layer believes, and why, and contest it. Built on Chaaya and AG Grid against fixtures, so it needn't wait for P2.

Read first: [`../product.md`](../product.md), `../docs/interface.md`, `research/ag-grid.md`.

---

### T4.1: The ledger's data layer
```yaml
requires:   [T1.2, T1.4, T0.10]
fixture-ok: yes
size:       S · mid
owns:       web/src/lib/api/
status:     done
notes:      Typed client in web/src/lib/api/ (types, decoders, fixture, Chaaya state). Fixture replays the nine-step story; step 9 equals the frozen ledger snapshots. Gate test pins types to testdata/api. Branch task/t41-apiclient.
```
A typed client for the ledger's read endpoints, with a fixture mode that serves the demo story at any step.

**Done when:** the client's types are checked against `testdata/api/` in the gate.

---

### T4.2: The ledger table ★
```yaml
requires:   [T4.1]
fixture-ok: yes
size:       M · frontier
owns:       web/src/lib/ledger/, web/src/routes/ledger/
status:     done
notes:      Read-only AG Grid table in web/src/lib/ledger/ + web/src/routes/ledger/: six columns plus seeded mark, pill per status, Out-of-true tooltip + correction line, kept/observe mapped to a weak-signal label, Chaaya light/dark themes, autoHeight rows, lazy-loaded grid chunk. Step picker replays fixture steps 1-9. Tests: story at every step both themes, arrow-key walk of every row, phone-width render. Branch task/t42-table.
```
Beliefs in AG Grid: kind, subject, value, status, evidence count, last change. The four statuses, **Claimed**, **Building**, **Earned** and **Out of true**, each with its own treatment, and a one-line tooltip explaining "Out of true" where it first appears. Seeded rows are visibly marked. Kept-without-feedback is labelled as a weak inferred signal. Light and dark, and usable at phone width.

**Done when:** the fixture story renders correctly at each step, in both themes, and a keyboard user can reach every row.

---

### T4.3: The why view ★
```yaml
requires:   [T4.2]
fixture-ok: yes
size:       M · mid
owns:       web/src/lib/why/, web/src/routes/belief/
status:     claimed:t43-why
```
For one belief: each piece of evidence with its outcome, item and reason side by side; the correction beside the belief it contradicts ("you said M; three returns said L"); and evidence that was deliberately not counted, such as the dispute that went against the seller. Links to the Astropods trace for each recommendation. Search across the evidence.

**Done when:** the demo's L belief and the seller belief both show complete evidence from fixtures; a non-counted item shows why it wasn't counted.

---

### T4.4: Contesting a belief
```yaml
requires:   [T4.2, T1.2]
fixture-ok: yes
size:       S · mid
owns:       web/src/lib/contest/
status:     claimed:t44-contest
```
Editable rows: a shopper's edit goes through `record_correction`, so it follows the same rule as anything else the shopper says: it wins the decision at once and doesn't rewrite the belief.

**Done when:** an edit in fixture mode moves the belief to Out of true and leaves it visible; a test proves the row edit calls the correction path and nothing else.

---

## Exit Criteria

- [ ] The ledger reads through a typed client (T4.1).
- [ ] The four statuses render, seeded rows marked (T4.2).
- [ ] Every belief can be explained, including what wasn't counted (T4.3).
- [ ] A shopper can contest a belief (T4.4).
