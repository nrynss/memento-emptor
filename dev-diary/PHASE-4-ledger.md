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
status:     done
notes:      The ledger page appends `explainColumnDefs()` and `contestEditColumnDefs()` after the read-only columns; the Contest button opens `ContestEditor` for that row with form id `contest-<belief id>`, submits through the existing `ContestState`, and reads the state's rows back so the row moves to Out of true with the correction beside it and the evidence count unchanged. Each row links to `/belief?belief=<id>`; `/belief` reads the link through `linkedBeliefId`/`pickBeliefId` and keeps its `/ledger` back link. Deep-link helpers in `web/src/lib/ledger/deeplink.ts`; the route flow is covered in `contest-route.test.ts` (step 4 fixture, real grid click, editor submit, kept belief, both links). The correction fixture answers only `blf_8f2c41`, so Contest on another row fails honestly instead of widening it. The why page's own link resolution is covered in `belief-deeplink.test.ts`, which supplies the browser guard and the request URL because vitest reports `$app/environment` as the server. T9.2's task line corrected. Branch task/t45.
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
status:     done
notes:      One shopper at a time on the pivot surfaces. The pooling fixture decodes the frozen answer and refuses when the answer's shopper is not the one the transport was built for, so a shopper_02 transport fails closed instead of replaying shopper_01's three rows removed. The correction fixture takes the shopper it answers (default the demo shopper), rejects a request naming another, and refuses the demo belief blf_8f2c41 from another shopper, while the standing answer — which names no belief — replays for the shopper the transport was built for, so a transport built for shopper_02 answers the frozen standing request byte for byte and no fixture-set change was needed. The belief page builds its ContestState from createFixtureCorrectionTransport(caseShopper) and keys the standing editor and PoolingToggle by the case shopper, so a case change remounts both with no earlier shopper's answer on screen. The standing section's duplicate lede is gone, so the empty-state sentence prints once, from the editor's own preview. New web/src/routes/belief/belief-shoppers.test.ts covers cross-shopper rejection on both fixtures, the frozen standing request and answer agreeing, the toggle and standing-editor remounts, the fail-closed pooling answer for a case the fixture does not carry, and the single empty state; mutation checks on the pooling answer guard, the toggle key, the standing-editor key, the removed lede and the correction shopper guard each fail it. The pooling examples exist for the demo shopper only, so the toggle for the other four cases reports that there is no answer rather than restamping three removed rows under another shopper. web/src/lib/contest/standing.test.ts's exactly-as-frozen case sends the frozen standing request unchanged against a transport built for that request's own shopper (shopper_02), so the test, the request example and the fixture agree. Branch task/t46. Gate green, web 180 tests in 18 files, merchant untouched, no live call, no spend.
```
The P4 review found the pivot's pooling and standing fixtures are built with the recall case's shopper but answer only the demo shopper: a `shopper_02` pooling call returns `shopper_01`'s answer (`{answered: 'shopper_01', removed: 3}`), a `shopper_02` standing correction throws, and `PoolingToggle` keeps its opening choice without a key, so switching cases leaves the previous shopper's result on screen. Make each fixture answer the shopper it was built for (reject a mismatch), reset the toggle when the case changes, and make the frozen standing request and fixture agree.

Also fix the repeated standing-correction empty-state sentence the T9.7 run saw on the belief page (it prints twice).

**Done when:** every pooling and standing fixture answers only its own shopper and rejects another; switching the case remounts the toggle; the frozen example and the fixture agree; a test covers a cross-shopper rejection; the empty-state sentence renders once; the gate passes.

---

### T4.7: Plain language in the why cards (Low)
```yaml
requires:   [T4.3]
fixture-ok: yes
size:       XS · mid
owns:       web/src/lib/why/
status:     done
notes:      The why cards in web/src/lib/why/ now lead with the shopper's words and name the reason category in plain language (`reasonWord`: too small, too large, damaged, not arrived, changed mind, unclear, from the merchant grid's reason columns and the domain's category comments); the raw category code no longer renders. A dispute's reason text is the payment source's code rather than the shopper's words, so `reasonReading` leads it with the plain word and never quotes the code. The layer's `none` is the zero value, no classified reason rather than a fit verdict, so it names no word: a card can never call an unauthorised-charge dispute "fits well". The reason column comes first in both the counted and the not-counted card. The trace note now says this is a fixture replay whose links are on-page placeholders, with the live transport still to come (T4.8); the traces.ts header no longer promises a live mode. Tests: every classified category maps to a lowercase word with no underscore, `none` maps to null, the demo's counted rows read "too small", the dispute reads "not arrived" unquoted, `reasonReading` is tested to quote the shopper's words and never a dispute code, and a source guard fails if the template prints a raw category or prints the category before the quote. Branch task/t47; review fixes on task/t47-review.
```
The P4 review found the why cards print the raw reason category (`fit_too_small`) above the shopper's words, and the trace note promises a "live mode" the fixture-only ledger does not have.

**Done when:** the cards show the shopper's words first with the category named in plain language (or omitted), the note describes what exists, and the gate passes.

---

### T4.8: The ledger reads live
```yaml
requires:   [T4.1, T9.6]
fixture-ok: yes
size:       M · mid
owns:       web/src/lib/api/, web/src/routes/ledger/, web/src/lib/why/, web/src/routes/belief/
status:     done
notes:      Live reads behind `LedgerClient`. `web/src/lib/api/http.ts` is the HTTP transport: one POST to `/tools/<tool>` with the link token as `Authorization: Bearer` and the request as JSON; a non-2xx answer fails with the layer's own code and message (the edge's 401 is `unauthenticated: unauthorized`), an unreachable edge fails as `unreachable`, and a bodiless or non-JSON answer as `bad-response`, so a failed read never returns data. `web/src/lib/api/live.ts` reads the link: `?t=` is the token and `?s=` the shopper it acts for, defaulting to the demo shopper, because every scoped request must name a `shopper_id` and the layer does not report which shopper a token binds. No `?t=` means the labelled fixture replay, which is what keeps the gate offline. The ledger page with a token loads beliefs, evidence and pending questions live and labels the source (`data-ledger-source="live"`, "Live: the layer's own rows for shopper X"); a failed live read shows `role="alert"` "The live ledger read failed: … Nothing on this page is live data" with no rows and no live or replay label. Without a token it shows "Fixture replay: the demo story at a chosen step, not a live ledger", keeps the step picker, and makes no live call. The per-row Why link and the why page's back link carry the shopper link, so the two pages stay live together; the token rides in those links because it is already in the page's own URL and the links are same-origin. The why page reads beliefs, evidence, pending questions, `recall_for_purchase`, `record_correction` and `set_pooling` through the same transport when linked, hides the fixture step and case pickers, and labels a failed detail read and a failed recommendation read separately; its live source label appears only once one of those two reads has answered, so a page that answered nothing wears no source label and its alerts say what failed. A live contest or standing correction is a live write through the edge. Both pages show an empty state when the layer holds no rows rather than an endless "Loading". The `ReasonCategory` gloss now reads "None is the zero value: no classified reason" — it never meant a fits-well verdict. The why view's trace note now says the links are placeholders on the page because the layer serves no per-belief trace list a shopper link can read (its call log is agent-only), and `traces.ts` no longer promises a live trace transport. Tests: `http.test.ts` (path, bearer, body, the layer's error envelope, unreachable, bad body, no token in a failure message), `live.test.ts` (link parsing incl. no token, live state rows, a refused read), `ledger-live.test.ts` (live rows labelled; failure labelled with no data; replay labelled with no live call), `live-belief.test.ts` (live belief, evidence and recommendation; a failed recommendation; a failed belief read; nothing live answered). Web 208 tests in 22 files, svelte-check, eslint and build green. No live call, no spend. Branch task/t48; review fixes on task/t48-review.
```
The T9.7 run found the deployed ledger page is a fixture replay: its live numbers exist only as API reads, and the screen is labelled as a replay. Add an HTTP transport behind `LedgerClient` (same-origin or the edge, with the shopper link token), keep the fixture transport for tests, and fall back loudly rather than silently showing a replay as live.

**Done when:** the deployed ledger renders the live layer's beliefs, evidence and recall for the signed-in shopper; the fixture transport remains for tests; a failed live read is labelled, never shown as live; the gate passes.

**Notes (in-review).** The transport and the link live in `web/src/lib/api/http.ts` and `web/src/lib/api/live.ts`; the fixture transport is untouched and still serves every offline test. The reads are same-origin `/tools/<tool>`, which the deployed edge already proxies to the layer, so no server, deploy or key change was needed: the static app reads the token from its own URL and puts it in the header. The three things the next agent should not have to work out:

- **The token does not name its shopper.** The guard checks the body's `shopper_id` against the token's binding, so the page must send the id the token belongs to, and nothing in the edge reports it. The link therefore names it too (`&s=<shopper>`); without `s` the page sends the demo shopper, which is the binding of the published demo token. A token for another shopper needs `s`, or the read answers 401 and is labelled as a failure.
- **The trace links stay placeholders in live mode too.** `/calls` is admitted to agent keys only and is keyed by trace rather than belief, so no shopper-token read can turn a belief into a trace link. The note beside the list now says that instead of promising a live trace transport.
- **The deployed app needs a redeploy to pick this up** (the Render service builds from `main` on land); nothing else changes. The live page is `<edge>/ledger?t=<token>` and the why page `<edge>/belief?t=<token>`. `demo/README.md` is stale once this lands: its surface table's "the page as deployed (a fixture replay; the live reads are the API)" cell, its step-6 row "The deployed page is a **fixture replay** … and has no live transport yet", and its item 3 "The ledger page is a fixture replay … Owner: T4.8" all describe the state this task removes, the last one naming this task as its own owner. That file sits outside this task's `owns`; owner **T9.20**, which owns `demo/` (or the orchestrator, if it prefers a docs task).

**Findings with owners.**

- **Low — the ledger page reads evidence it never renders.** `LedgerState.refresh` fetches `ledger_evidence` on every load and the ledger page shows none of it: the per-row count comes from the belief row, and a shopper reads the evidence itself in the why view. Two consequences. The load is a third round trip, and because the three reads share one `Promise.all` a failed evidence read labels the whole page failed even though the page displays nothing from it. It fails closed and loud, so nothing is shown as live that is not; removing the read is a change to T4.1's frozen state shape, so it is the orchestrator's call. The task's Done-when says the deployed ledger renders the live layer's beliefs, evidence and recall: the page itself renders beliefs and pending questions, and evidence and the recommendation render on the why view each row links to with the shopper link. If the orchestrator reads "the deployed ledger" as the ledger page alone, that clause stays unmet. Owner: a new task over `web/src/lib/api/state.ts` and `web/src/routes/ledger/` (proposed T4.10), or the orchestrator if the state is meant to stay the shared loader for both pages.


---

### T4.9: The fixture contest state builds its transport for its shopper
```yaml
requires:   [T4.6]
fixture-ok: yes
size:       XS · mid
owns:       web/src/lib/contest/state.ts
status:     not-started
```
`createFixtureContestState` calls `createFixtureCorrectionTransport()` with no argument, so a caller passing a non-demo shopper gets a state whose requests name that shopper against a demo-shopper transport: every correction fails with "fixture correction knows only one shopper". It is latent (the belief page constructs its state directly; the ledger passes the demo shopper) and fails closed, but the helper is wrong. Pass the shopper through, and replace the page's duplicate wiring with the helper if that simplifies it.

**Done when:** `createFixtureContestState` builds its transport for the shopper it was given; the belief page uses the helper; a test proves a non-demo shopper's correction reaches a transport built for it; the gate passes.

---

### T4.10: The ledger page stops loading evidence it never shows
```yaml
requires:   [T4.8]
fixture-ok: yes
size:       XS · mid
owns:       web/src/lib/api/, web/src/routes/ledger/
status:     not-started
```
`LedgerState.refresh` fetches `ledger_evidence` on every load, but the ledger table renders none of it (the per-row count comes from the belief row; the evidence lives in the why view). Because the three reads share one `Promise.all`, a failed evidence read labels the whole page failed although nothing from it is displayed. Decide: drop the read, or load it lazily for the row that needs it, or record why the shared loader is deliberate.

**Done when:** the ledger page no longer fails over data it does not show (or the reason is recorded); the why view still renders evidence; the gate passes.

---

## Exit Criteria

- [x] The ledger reads through a typed client (T4.1).
- [x] The four statuses render, seeded rows marked (T4.2).
- [x] Every belief can be explained, including what wasn't counted (T4.3).
- [x] A shopper can contest a belief (T4.4, T4.5).

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
