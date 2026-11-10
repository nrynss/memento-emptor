# P9: Population and surfaces

The seeded population behind the pool, the ledger and the merchant app that show it, the two agents that use it, the hosted service, and the demo.

Read first: [`../product.md`](../product.md) ("The demo", "Cautions"), [`PHASE-7-pivot-contracts.md`](PHASE-7-pivot-contracts.md), [`PHASE-8-pivot-layer.md`](PHASE-8-pivot-layer.md), and the P3, P4 and T6.1 tasks whose code you change.

---

### T9.1: The seeded population
```yaml
requires:   [T7.1, T7.2]
fixture-ok: yes
size:       M · mid
owns:       seed/population.go, seed/population_test.go, testdata/seed/population/
status:     done
```
A deterministic generator for the shoppers behind the pool, written through the T8.2 row shape with the seeded salt and the seeded mark on every row. It must include the demo brand's oxford with a pooled belief that reaches Earned honestly under the T7.1 rule (product.md's demo step 0), lines that stay Building, and one line that is Out of true. Existing shoppers and their transaction ids stay byte-identical (the T5.7 rule).

**Done when:** a test pins the population's counts, and the T7.1 status rule run over it gives the intended status for every line and size. A second run is byte-identical. Every row carries the seeded mark. The gate passes.

**Notes (in-review).** `seed/population.go` is the generator and `seed/population_test.go` pins it; the committed fixture is `testdata/seed/population/population.json`. `GeneratePopulation(asOf)` returns the anchor and the rows, `WritePopulation` writes the fixture, and `DemoOxfordLine()` and `PopulationLines()` name the lines. Every row is minted as its own shopper, hashed through `identity.SeedSaltHex` with `LineShopperHash`, and keyed to `identity.TxnHash`; the raw shopper and transaction ids never reach a row, and every row carries `Seeded: true`. The population is a hand-written truth table rather than a random sample, so a second run is byte-identical by construction.

The six line-and-size pairs, by settled fit counts (small / large / fits-well) and the status `domain.DecidePool` gives them:

| Line (brand / category / fit handle) | Size | Counts | Status |
|---|---|---|---|
| brand-x-001 / shirts / regular (the demo oxford) | M | 12 / 3 / 4 | Earned, small |
| brand-x-001 / shirts / regular | L | 0 / 4 / 0 | Building, large |
| brand-y-002 / shirts / slim | M | 6 / 1 / 1 | Building, small |
| brand-z-003 / shirts / regular | M | 8 / 0 / 8 | Building, small |
| brand-v-004 / shirts / relaxed | M | 1 / 1 / 0 | Claimed |
| brand-w-005 / shirts / regular | M | 10 / 4 / 0 | Out of true, small |

69 rows, 66 settled; the demo line also carries two kept purchases, a damaged return, two captures still inside their window and an unexplained refund, which settle nothing or point nowhere.

Four things the next agent should not have to rediscover:

- The demo line uses the contract's line naming (`brand-x-001` / `shirts` / `regular`, size `M`), not the pre-pivot seed's item ids (`BRX`, `dress-shirts`). `product_fit`, the ledger and the merchant app query the former; get the names from `DemoOxfordLine()` and `PopulationLines()`, and the fixture is the truth for counts.
- `DecidePool` judges the recent window only once a direction is Earned, and that window must itself earn. Between three and seven recent fit shoppers no direction can reach the eight-shopper bar, so an Earned line with three to seven recent shoppers reads Out of true. That is why the demo line carries eight too-small shoppers inside the window and the reversed line carries six. Seeding live must pass the moment of seeding as `asOf`: seeding the fixture's fixed anchor later drifts rows out of the window and changes statuses, so T9.6 passes the seeding moment.
- The generator writes `domain.FitRow` directly because T8.2's writer is not on `main` yet; that writer is the consumer. `event_seq` is not a field (T7.1 dropped it): `FitRow.Version()` is the state rank with event time inside it.
- Each row is one shopper's one purchase, so outcome counts and distinct-shopper counts coincide. A repeat shopper would need the pool reader's dedupe rule, which T8.3 owns; this population deliberately does not pin it. The test's `populationCounts` helper is a test-only read of the rows, not a second pool reader.

The existing seeded shoppers and their transaction ids are untouched and their golden fixture test still passes; the population's transaction ids use their own prefix and share nothing with them.

---

### T9.2: The ledger, with precedence and the pool
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       S · mid
owns:       web/src/lib/ledger/, web/src/routes/belief/, web/src/lib/contest/
status:     done
```
The shopper's ledger shows which precedence step decided each recommendation, an overridden step beside the one that won, a standing correction with no belief, and the pooled belief that applies to them as counts with its seeded count. It never shows another shopper's rows. The shopper can opt out of pooling here, and back in, through `set_pooling`, with the number of rows removed shown. Fix the pre-pivot correction types in `web/src/lib/contest/`: a standing correction is schema-valid now, and the decoder must accept it. Svelte 5 runes only.

**Done when:** component tests cover each step, an overridden pair, a standing correction, and the opt-out toggle. The pooled row shows counts only. The gate passes.

**Notes (in-review).** New in `web/src/lib/ledger/`: `precedence.ts` (five steps, decoders, plain-words lines), `recall.ts` (fixture recall for one case per shopper plus a statement-decides synthesis with the pool unreachable, and the demo-oxford `product_fit` fixture), `pool.ts` (counts-only pool decoders and lines), `pooling.ts` (typed `set_pooling` client, fixture transport, state), and four small views (`DecisionLine`, `PoolCounts`, `StandingCorrection`, `PoolingToggle`). `web/src/routes/belief/+page.svelte` gains a recommendation case picker (one shopper per case), the decision line with overridden steps, the pooled counts, a standing-correction form with its list, and the pooling toggle. `web/src/lib/contest/` now accepts the standing shape everywhere: optional `contradicts_belief_id` / `belief_id`, `contradicted` as a boolean, five-step guidance basis, a standing fixture path, `submitStandingCorrection`, and a standing mode for `ContestEditor` with an `onRecorded` callback.

Four things the next agent should not have to rediscover:

- The pool row is counts by construction: `PoolView` has no field for shopper identity, rows or free text, and a test pins its key set, so no view can leak another shopper's rows. Cross-shopper isolation is proved by rejection: every fixture transport fails a mismatched shopper instead of inventing an answer.
- Under vitest, `svelte/reactivity` resolves to the server build, so Chaaya `notify()` never invalidates template effects in tests (a minimal `Notifier` plus template stays stale; the browser build is unaffected). `PoolingToggle` and the standing list therefore mirror the layer's answer into local `$state` after each call, following the existing `ContestEditor` pattern, instead of reading the notified state in the template.
- `decodePoolView` omits `precedence` when the fixture has none, matching the contract's `omitempty`, so the decoded pool stays byte-equal to the frozen example.
- `contest.test.ts` constrains its own directory (one `.call` in `client.ts` only, no `ledger_beliefs` reads, no `Out of true` literal outside `fixture.ts`/`decode.ts`): keep new contest code inside those lines or the gate's own tests fail first.

---

### T9.3: The merchant app ★
```yaml
requires:   [T8.3, T8.6]
fixture-ok: yes
size:       M · mid
owns:       merchant/, tools/check.sh
status:     done
```
A separate, read-only app for the sandbox merchant's product lines. It reads the pool only through the layer's `product_fit`, with the merchant key from T8.6, never Elastic, and holds no Elastic key. One AG Grid table: product line and size, settled purchases, returns by reason category and direction, feedback by verdict, pool status, seeded count. A row opens the "why" for that line and size: the counts behind its status, and the rule it was judged by. The same stack as `web/` (Svelte 5, Chaaya, AG Grid community), with its own package, tests and build, and a step in `tools/check.sh`. The Go `DemoOxfordLine()` / `PopulationLines()` helpers are not importable from Svelte; take the seller's line list from `testdata/api` or its own config.

**Done when:** it builds and tests against fixtures from `testdata/api/`. Each status renders with its rule. No component can show a shopper row, and a test proves the app has no Elastic configuration. The gate passes, including the new step.

**Notes (in-review).** `merchant/` mirrors `web/`'s stack (Svelte 5, Chaaya, AG Grid community, SvelteKit static, vitest) with its own package, lockfile and build. `src/lib/pool/` is the `product_fit` read: types, decoders, a one-method `MerchantClient`, a fixture transport over the six seller lines, an HTTP transport that posts only to `/tools/product_fit` with the merchant key, the line list (the Go helpers are not importable from Svelte, so it lives here as data), and the status rule. The Earned demo oxford fixture is the frozen `product_fit.response.example.json` itself; the other five lines are built so `decidePool` re-decides the status each serves, which the gate checks. `rule.ts` mirrors `internal/domain/pool.go`'s thresholds for explanation only; the layer decides. `src/lib/merchant/` is the read-only AG Grid table (line and size, settled, returns by direction, reasons by category, fits-well feedback, status, direction, seeded count) plus the why panel (`explainWhy`: the counts behind the status and the rule). Proof tests: `pool.test.ts` pins the single-method client and refuses any request naming more than the line and size; `grid.test.ts` and `rows.test.ts` assert no column, row or rendered cell can carry a shopper field; `no-elastic.test.ts` scans shipped source, the dependency lists and the only two environment names the app reads. `tools/check.sh` now runs each frontend through a shared `check_frontend` step (web, then merchant) and excludes both `node_modules` trees from the Go walk. The gate passes: Go vet and tests, both lockfiles, both svelte-checks, both lints, both vitest suites (merchant: 6 files, 37 tests), both builds.

Four things the next agent should not have to rediscover:

- `MerchantClient.productFit` validates with `checkProductFitRequest`, which requires exactly the four line-and-size fields, so both transports reject anything more before it reaches the layer: pass seller lines through `toProductFitRequest`, since `MerchantLine` carries a display `label` the contract rejects.
- `svelte-check` resolves `node:` imports only with `@types/node` in devDependencies; `merchant` carries it while `web/` does not need it.
- The `Out of true` fixture needs a recent window that is itself judged (at least 3 fit shoppers) yet fails the Earned bar; between three and seven recent fit shoppers no direction can earn, so a quiet Earned line reads Out of true.
- The page loads through the fixture transport; wiring the HTTP transport is a config task (base URL plus merchant key), and the key never enters a file.

### T9.3d: The merchant app, deployed
```yaml
requires:   [T9.3, T6.1]
fixture-ok: no
size:       S · mid
owns:       deploy/merchant/
status:     not-started
```
Its own Render service, following T6.1's provision and guard pattern. It reaches the layer's `product_fit` and nothing else. The user has pre-approved new services for the pivot: record the service's monthly cost in the brief and the handoff log, and keep the guard, but do not stop for a second approval.

**Done when:** the public URL serves the app; the guard reports it; the cost is recorded in the handoff log.

---

### T9.4: The reference agent, after the pivot ★
```yaml
requires:   [T8.5]
fixture-ok: no
size:       M · frontier
owns:       agent/runtime/
status:     in-review
```
The reference agent reads the precedence step and states how sure it is from the label. It never refuses when recall returns evidence, and it says when an answer rests on the pool or on a Claimed statement. It records a standalone correction when recall served no belief (the 7-in-9 gap from T5.9). The existing guarantees stay: recall first, approval before any purchase, double-buy protection. Re-run the T3.5 evals and add one per precedence step. Budget: under $1.00. Run one eval first and check its cost.

**Done when:** the evals pass, with traces showing recall first on every turn. The standalone correction is recorded live at least once. Spend is recorded.

**Notes (worker).** All code in `agent/runtime/`; the fake memory now serves the pivot shapes. `agent/memory.ts` keeps the frozen `shopper_01` correction story (plus pool and precedence views) and adds one shopper per remaining step: `shopper_02` pool (Earned small, statement overridden), `shopper_04` own evidence beating the pool, `shopper_06` statement only, anyone else none (example-none text, never a refusal). `recordCorrection` takes an optional belief id: with none it stands Claimed/None and persists in container memory, deciding again on a later turn. New `product_fit` and `set_pooling` tools complete the eleven frozen calls. `agent/index.ts` instructions: recommend from `guidance.basis`, state confidence from the label, cite belief/correction/evidence ids with quoted pool counts, never let a correction travel in chat text only, recall on every turn including follow-ups. `agent/offline-check.ts` pins all five steps plus the standalone round-trip (39 checks). `agent/runtime/EVALUATION.yaml` is the 8-judge set: the three T3.5 prompts kept, plus one judge per precedence step that fires only on its own basis (so the set also reports which rung decided).

Live on `memento-agent` (build `92126f63`), six fresh-conversation turns, each with `recall_for_purchase` first in its spans: correction (`4c55d2dc`, $0.033), pool (`76814311`, $0.028, cites 60 settled 48 seeded/12 live), own-evidence (`0367307b`, $0.030), statement (`94f168f5`, $0.026, Claimed words-not-evidence), none (`165ec9cd`, $0.024, asks usual size), standalone single-turn (`8238de5b`, $0.038, recall + `record_correction` with no belief id, `corr_stand_01`). A later-session turn re-served the stored standalone correction. The `yes` confirmation turn carries no tool spans (nothing to recommend, purchase honestly not wired). Eval runs over all six traces: every step judge passes on its own step; approval true on all six; recall_first and correction_honoured true on all four traces where recall returned evidence or a standalone correction (F, A, B, C). Two prompt-strictness findings, both judge-side (agent replies correct): recall_first stays false on the statement-only and none traces because its evidence-citation clauses assume evidence exists; correction_honoured is false by its own design on turns with no correction. Owner: the T10 comparison prep (extend the evidence clauses for evidence-free recalls, same class as the standalone fix already made). Spend $0.92 of $1.00 (turns $0.46 incl. two superseded-build turns, eval runs $0.46). Two surprises: the pool reply once paraphrased `Shoppers.Fit` (39) as fits-well verdicts (fixed by a quote-don't-paraphrase line, no re-drive needed); the first conditional judge wording confused the LLM judge into reasoned-true/output-false, fixed by unconditional per-basis prompts.

---

### T9.5: The second agent
```yaml
requires:   [T9.4]
fixture-ok: no
size:       S · mid
owns:       agent/second/
status:     not-started
```
A second Astropods deployment with its own name and prompt and its own traces, recalling through the same layer. It is written fresh: no shared prompt text with the reference agent beyond the layer's tool descriptions, so that its picking up a correction proves the layer carried it, not the prompt. Budget: under $0.50.

**Done when:** a correction made through the reference agent is honoured by the second agent's first recommendation, shown in both agents' traces. Spend is recorded.

---

### T9.6: The pivoted layer live on Render
```yaml
requires:   [T8.5b, T9.1, T7.3]
fixture-ok: no
size:       S · mid
owns:       deploy/render.yaml, deploy/seed/, deploy/guard.sh, deploy/provision.sh, .env.example, tools/env.sh
status:     not-started
```
The hosted service runs the T8.1 hub, the fit index (its live name, not a test prefix) and the seeded population, seeded once as T6.1 does. Measure `--max-sessions 128` and `--rate-limit-rps 200` on the 0.5 CPU starter; the T8.1b numbers are from a 12-CPU machine and do not stand for the starter. The Elastic key and the shopper salt reach the service as Render secrets, never in a file. Add the three auth key variables (`MEMENTO_AGENT_KEYS`, `MEMENTO_SHOPPER_TOKENS`, `MEMENTO_MERCHANT_KEYS`) to `deploy/render.yaml` as `sync: false`, name them in `.env.example`, and read them in `tools/env.sh` from the keyring; mint the keys with T8.5's command, never invent them. If seeding through the writer, call `Refunded` then `Classified` so a reason-bearing seed refund settles, and measure the forced-refresh cost at population scale. The fixture embedder cannot converge the frozen arc; keep `LAMBO_SEED=0` for the local loop. Any change to the monthly cost is recorded in the handoff log; the user has pre-approved the pivot's new services.

**Done when:** the public URL answers `recall_for_purchase` for the demo shopper with the pool step working, and `product_fit` for the demo oxford reads Earned. The guard reports green. Spend is recorded.

---

### T9.7: The demo, after the pivot ★
```yaml
requires:   [T9.2, T9.3d, T9.5, T9.6]
fixture-ok: no
size:       M · frontier
owns:       demo/
status:     not-started
```
product.md's demo from step 0 to step 7, live, on the hosted service: the new shopper sized from the pool on day one; the three returns; the correction; the purchase and feedback; the dispute that leaves sizing alone; the ledger; the second agent honouring the correction; the merchant app's "why". The words about pooling follow the consent decision (T7.3). Budget: under $1.00. Rehearse step 0 first and check its cost.

**Done when:** a recorded run covers every step, with each step's trace or screen linked in the handoff log. `demo/reset.sh` restores the starting state. Spend is recorded.

---

## Exit Criteria

- [x] A seeded population where the demo oxford is Earned honestly, and other lines are Building or Out of true (T9.1).
- [x] The ledger shows the deciding step and the pool, with no other shopper's rows (T9.2).
- [ ] The merchant app is built and deployed, reading only `product_fit` (T9.3, T9.3d).
- [ ] The reference agent never refuses on evidence and records standalone corrections (T9.4).
- [ ] A second agent honours a correction it never saw (T9.5).
- [ ] The pivoted layer is live, and the demo runs end to end (T9.6, T9.7).
