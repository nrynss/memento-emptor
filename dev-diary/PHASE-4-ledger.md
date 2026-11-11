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
status:     done
notes:      Why view in web/src/lib/why/ + web/src/routes/belief/: step and belief pickers, counted evidence with outcome/item/reason side by side, frozen correction beside the contradicted belief, not-counted rows each with its reason, pending questions, fixture trace placeholders, client-side search. Plain ledger language on screen (no graph standing). Gate green. Branch task/t43-why.
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
status:     done
notes:      Editable rows in web/src/lib/contest/ (types, decode, client, fixture, state, columns, ContestEditor). Every edit goes through record_correction only; fixture edit moves the L belief to Out of true with standing kept and the row visible. Branch task/t44-contest.
```
Editable rows: a shopper's edit goes through `record_correction`, so it follows the same rule as anything else the shopper says: it wins the decision at once and doesn't rewrite the belief.

**Done when:** an edit in fixture mode moves the belief to Out of true and leaves it visible; a test proves the row edit calls the correction path and nothing else.

---

### T4.5: The contest route (High)
```yaml
requires:   [T4.4, T9.2]
fixture-ok: yes
size:       S · mid
owns:       web/src/routes/ledger/, web/src/lib/ledger/, web/src/lib/contest/
status:     claimed:t45-contest-route
```
The P4 review found the contest path has no route: `/ledger` is read-only, `/belief` renders only the standing editor, and no route passes a belief id to the declared contest editor. `contestEditColumnDefs` is never used and the built ledger has no "Contest" string. A shopper cannot contest a belief in the app, so T4.4's exit criterion is unmet. Add the contest column to the ledger, open the editor per row with a unique form id, let the existing state apply the layer's answer, and link `/ledger` to `/belief` (which should honour a belief query parameter). Correct T9.2's task line that says the ledger shows precedence/pool/opt-out when only `/belief` does.

**Done when:** a shopper can open the contest editor from the ledger for a belief row, submit a correction, and see the layer's answer with the belief and its evidence kept; a component test covers the flow; the ledger and belief pages link both ways; the gate passes.

---

### T4.6: One shopper at a time on the belief page (Medium)
```yaml
requires:   [T9.2]
fixture-ok: yes
size:       S · mid
owns:       web/src/routes/belief/, web/src/lib/ledger/pooling.ts, web/src/lib/contest/fixture.ts
status:     not-started
```
The P4 review found the pivot's pooling and standing fixtures are built with the recall case's shopper but answer only the demo shopper: a `shopper_02` pooling call returns `shopper_01`'s answer (`{answered: 'shopper_01', removed: 3}`), a `shopper_02` standing correction throws, and `PoolingToggle` keeps its opening choice without a key, so switching cases leaves the previous shopper's result on screen. Make each fixture answer the shopper it was built for (reject a mismatch), reset the toggle when the case changes, and make the frozen standing request and fixture agree.

**Done when:** every pooling and standing fixture answers only its own shopper and rejects another; switching the case remounts the toggle; the frozen example and the fixture agree; a test covers a cross-shopper rejection; the gate passes.

---

### T4.7: Plain language in the why cards (Low)
```yaml
requires:   [T4.3]
fixture-ok: yes
size:       XS · mid
owns:       web/src/lib/why/
status:     not-started
```
The P4 review found the why cards print the raw reason category (`fit_too_small`) above the shopper's words, and the trace note promises a "live mode" the fixture-only ledger does not have.

**Done when:** the cards show the shopper's words first with the category named in plain language (or omitted), the note describes what exists, and the gate passes.

---

### T4.8: The ledger reads live
```yaml
requires:   [T4.1, T9.6]
fixture-ok: yes
size:       M · mid
owns:       web/src/lib/api/, web/src/routes/ledger/
status:     not-started
```
The T9.7 run found the deployed ledger page is a fixture replay: its live numbers exist only as API reads, and the screen is labelled as a replay. Add an HTTP transport behind `LedgerClient` (same-origin or the edge, with the shopper link token), keep the fixture transport for tests, and fall back loudly rather than silently showing a replay as live.

**Done when:** the deployed ledger renders the live layer's beliefs, evidence and recall for the signed-in shopper; the fixture transport remains for tests; a failed live read is labelled, never shown as live; the gate passes.

---

## Exit Criteria

- [x] The ledger reads through a typed client (T4.1).
- [x] The four statuses render, seeded rows marked (T4.2).
- [x] Every belief can be explained, including what wasn't counted (T4.3).
- [ ] A shopper can contest a belief (T4.4).

---

## Review (fresh eyes, after the pivot)

Re-verified every Done-when clause and the four exit criteria against the tree, with T9.2's precedence and pool surfaces in view. Mutation checks: dropping `contradicts_belief_id` from the correction request fails five contest tests; adding a shopper-id field to a grid row failed no test before this review and fails the new no-leak test after it; a reintroduced page-header comment fails the new header guard.

**Fixed on the review branch.** (1) The four P4 Svelte files opened with a `//` header that Svelte compiles as visible page text; each header now sits inside `<script>`, and `web/src/lib/ledger/svelte-headers.test.ts` guards every web component and route. The merchant app's two files had the same defect, fixed with the same guard in `merchant/`. (2) `grid.test.ts` now renders every step in both themes; it previously alternated one theme per step, so T4.2's "both themes" claim was tested only at step 9. (3) `rows.test.ts` and `grid.test.ts` pin the grid's key set, column set and rendered text against shopper ids and transaction links; a row could carry a shopper id with no test failing before this. (4) `api.test.ts` no longer claims a static assignment the test does not make; the decoders and the step-9 equality are the actual gate.

**Findings with owners.**

- **High — the contest path has no route.** `/ledger` is read-only and `/belief` renders only the standing editor, so a shopper cannot contest a belief in the app even though the state, client and editor tests pass. This is exit criterion 4 unmet, and R4's finding 2, still without an owner. **T4.5** (proposed) owns `web/src/routes/ledger/`, `web/src/lib/ledger/`, `web/src/lib/contest/`: add the contest column beside `ledgerColumnDefs()`, open the editor for the clicked row with a per-belief form id, and let the existing state apply the layer's answer.
- **Medium — the belief page mixes shoppers on two pivot surfaces.** The pooling state and the standing-correction state are built for the recall case's shopper (`shopper_02`, `shopper_04`, `shopper_05`), but both fixtures answer only the demo shopper: a `shopper_02` pooling call returns `shopper_01`'s answer (3 rows removed), and a `shopper_02` standing correction fails with "fixture knows only the demo shopper". `PoolingToggle` also keeps its opening choice in local state and is not keyed, so switching cases leaves the previous shopper's result on screen. T9.2's rejection rule does not cover pooling: the transport is built for whichever shopper a case names and still returns the demo shopper's frozen answer. **T4.6** (proposed) owns `web/src/routes/belief/`, `web/src/lib/ledger/pooling.ts`, `web/src/lib/contest/fixture.ts`.
- **Low — the why cards print raw reason categories** (`fit_too_small`) above the shopper's words, and the trace note promises a "live mode" the fixture-only ledger does not have. This is R4's finding 4, still without an owner. **T4.7** (proposed) owns `web/src/lib/why/`.
- **Low — T9.2's task line says the shopper's ledger shows precedence, the pool and the opt-out; the table at `/ledger` shows none of them.** All four surfaces live on `/belief`, which T9.2's note states but the task text does not. **T4.5** should either move the surfaces or correct the task line.
- **Low — the ledger table has no path to the why view.** `/belief` links back to `/ledger`, but `/ledger` has no per-row or header link, and `/belief` ignores a belief query parameter, so a judge reaches the explanation only by typing the route. This is R4's finding 3, still without an owner. **T4.5** owns the link and the parameter.
- **Low — the frozen standing request example is `shopper_02`'s, while the standing fixture accepts only `shopper_01`.** The test named "exactly as frozen" replays the response for a different shopper's request. **T4.6** owns the fixture side; the contract example is the orchestrator's to change.

R4's findings are adopted by T4.5 (contest UI, deep links), T4.7 (reason wording) and T4.6 (the remaining fixture and page surfaces); the orchestrator should annotate that handoff entry with them.

**Exit criteria.** 1, 2 and 3 are proved by the tree and should be ticked. 4 is not, and should be ticked when T4.5 lands.
