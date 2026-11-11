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
The shopper's ledger links each belief to its why view, and the why view at `/belief` shows which precedence step decided each recommendation, an overridden step beside the one that won, a standing correction with no belief, and the pooled belief that applies to them as counts with its seeded count. It never shows another shopper's rows. The shopper can opt out of pooling here, and back in, through `set_pooling`, with the number of rows removed shown. Fix the pre-pivot correction types in `web/src/lib/contest/`: a standing correction is schema-valid now, and the decoder must accept it. Svelte 5 runes only.

**Done when:** component tests cover each step, an overridden pair, a standing correction, and the opt-out toggle. The pooled row shows counts only. The gate passes.

**Notes (in-review).** New in `web/src/lib/ledger/`: `precedence.ts` (five steps, decoders, plain-words lines), `recall.ts` (fixture recall for one case per shopper plus a statement-decides synthesis with the pool unreachable, and the demo-oxford `product_fit` fixture), `pool.ts` (counts-only pool decoders and lines), `pooling.ts` (typed `set_pooling` client, fixture transport, state), and four small views (`DecisionLine`, `PoolCounts`, `StandingCorrection`, `PoolingToggle`). `web/src/routes/belief/+page.svelte` gains a recommendation case picker (one shopper per case), the decision line with overridden steps, the pooled counts, a standing-correction form with its list, and the pooling toggle. `web/src/lib/contest/` now accepts the standing shape everywhere: optional `contradicts_belief_id` / `belief_id`, `contradicted` as a boolean, five-step guidance basis, a standing fixture path, `submitStandingCorrection`, and a standing mode for `ContestEditor` with an `onRecorded` callback.

**Correction (T4.5).** The task line above said the shopper's ledger shows precedence, the pool and the opt-out. Those surfaces live on `/belief`, which `/ledger` now links to per row; the line is corrected to say so.

Four things the next agent should not have to rediscover:

- The pool row is counts by construction: `PoolView` has no field for shopper identity, rows or free text, and a test pins its key set, so no view can leak another shopper's rows. Cross-shopper isolation is proved by rejection: every fixture transport fails a mismatched shopper instead of inventing an answer.
- Under vitest, `svelte/reactivity` resolves to the server build, so Chaaya `notify()` never invalidates template effects in tests (a minimal `Notifier` plus template stays stale; the browser build is unaffected). `PoolingToggle` and the standing list therefore mirror the layer's answer into local `$state` after each call, following the existing `ContestEditor` pattern, instead of reading the notified state in the template.
- `decodePoolView` omits `precedence` when the fixture has none, matching the contract's `omitempty`, so the decoded pool stays byte-equal to the frozen example.
- `contest.test.ts` constrains its own directory (one `.call` in `client.ts` only, no `ledger_beliefs` reads, no `Out of true` literal outside `fixture.ts`/`decode.ts`): keep new contest code inside those lines or the gate's own tests fail first.

---

### T9.2b: Ledger reads the tightened contract
```yaml
requires:   [T7.4]
fixture-ok: yes
size:       S · mid
owns:       web/src/lib/ledger/, web/src/lib/api/, web/src/routes/belief/, web/src/lib/contest/
status:     done
```
T7.4 requires the pivot pool and precedence fields, so the ledger decoders and fixture tests that expected them absent now fail (6 tests: api snapshots, pool precedence, pre-pool belief). Update the decoders to the required shape and the tests to the tightened fixtures. No behaviour change beyond the contract.

**Done when:** the 6 failing tests pass against the tightened fixtures; the gate passes.

**Notes (worker).** The decoders now require the tightened shape: `BeliefView` carries `precedence` (new `BeliefPrecedenceView` in `web/src/lib/api/types.ts`, decoded strictly in `decode.ts`, null on ledger and explain reads per the contract; the five fixture beliefs gain `precedence: null`), `decodePoolView` requires `precedence` and always returns it (null on `product_fit`), and `decodeRecallResponse` requires `pool` and `pool_lookup` plus `precedence` on every belief (`pool: null` with `unavailable` is an answer, never a refusal). `RecallBelief` is now an alias of `BeliefView`: the old optional override conflicted with the required field, and the empty-interface form tripped the lint. Tests updated to the tightened fixtures: the frozen pool decodes with `precedence: null` (the key-set pin gains `precedence`), and the unwired recall case decodes its belief precedence (`own_evidence` overridden by `correction`) with a null pool and `unavailable` lookup. The belief page and contest code needed no changes; `rows.test.ts` gained the one new literal field. All 6 tests pass and web is green (158 tests, svelte-check, eslint, build).

Two things the next agent should not have to rediscover:

- `merchant/src/lib/pool/pool.test.ts` (`serves the Earned demo oxford equal to the frozen snapshot`) fails the same way: merchant's `decodePool` drops `precedence`, so the decoded fixture no longer equals the raw example. `merchant/` is outside this task's `owns`; it needs a T9.3 follow-up to carry the required pool `precedence` the same way. Until then the gate stops at merchant vitest.
- The T9.2 note saying `decodePoolView` omits `precedence` when the fixture has none is stale after this task (the field is now required and always returned); left untouched because T9.2 is done.
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
status:     done
```
Its own Render service, following T6.1's provision and guard pattern. It reaches the layer's `product_fit` and nothing else. The user has pre-approved new services for the pivot: record the service's monthly cost in the brief and the handoff log, and keep the guard, but do not stop for a second approval.

**Done when:** the public URL serves the app; the guard reports it; the cost is recorded in the handoff log.

**Notes (worker).** `deploy/merchant/` holds the whole shape: `main.go` (+`main_test.go`) is a small edge serving the merchant build and proxying only `POST /tools/product_fit` to `MEMENTO_LAYER_URL` with `MEMENTO_MERCHANT_KEY` injected server-side (any other `/tools/` path 404s before any backend is touched; the browser never sees the key); `Dockerfile` builds the merchant app plus the edge into one stateless image; `render.yaml` is the blueprint; `provision.sh` replays the API create; `guard.sh` audits the new two-service settled shape. Monthly cost: **$7.00** for this service (starter, no disk), **$14.25** combined with the main demo service ($7.25) against the $50 credit — recorded here and in the handoff log. Live: service `memento-merchant` (`srv-dbposse7bikc73au5i9g`), `https://memento-merchant.onrender.com`, starter/virginia/docker, no disk; `guard.sh --check` reads 2 services, 1 GB disk, $14.25/mo against the $15.00/mo ceiling, exit 0. The merchant key was minted with `memento key issue --kind merchant --id merchant-app`, installed as a Render secret; the entry for the layer's `MEMENTO_MERCHANT_KEYS` is in the handoff log. The image is proved locally: `/healthz` 200, `/` serves the app, the proxy injects the service key (a wrong caller key still gets the layer's answer), other tools 404 untouched, and keyless boot serves the app with the read at 503. The gate passes. First deploy is `build_failed` on purpose — Render builds branch `main`, which has no `Dockerfile` until this lands; the green deploy auto-follows the land (the T6.1 pattern).

Three things the next agent should not have to rediscover:

- The merchant build imports the frozen `product_fit` example from the repo-root `testdata/api`, so the image's web stage copies `testdata/api` beside `merchant/`; copying `merchant/` alone fails the build.
- The old single-service guard (`deploy/guard.sh`, T9.6) trips (exit 2, nothing suspended) once this service exists, until its ceiling is raised to the combined $14.25 shape; `deploy/merchant/guard.sh` is the green guard for the pair.
- Live reads need two follow-ups outside this `owns`: the layer must carry the handoff entry in `MEMENTO_MERCHANT_KEYS` (T9.6), and the page must be wired to same-origin `/tools/product_fit` (a `merchant/` change, T9.3 follow-up). Until both, the proxy answers 502/503 and the app reads its bundled snapshot.


---

### T9.4: The reference agent, after the pivot ★
```yaml
requires:   [T8.5]
fixture-ok: no
size:       M · frontier
owns:       agent/runtime/
status:     done
```
The reference agent reads the precedence step and states how sure it is from the label. It never refuses when recall returns evidence, and it says when an answer rests on the pool or on a Claimed statement. It records a standalone correction when recall served no belief (the 7-in-9 gap from T5.9). The existing guarantees stay: recall first, approval before any purchase, double-buy protection. Re-run the T3.5 evals and add one per precedence step. Budget: under $1.00. Run one eval first and check its cost.

**Done when:** the evals pass, with traces showing recall first on every turn. The standalone correction is recorded live at least once. Spend is recorded.

**Notes (worker).** All code in `agent/runtime/`; the fake memory now serves the pivot shapes. `agent/memory.ts` keeps the frozen `shopper_01` correction story (plus pool and precedence views) and adds one shopper per remaining step: `shopper_02` pool (Earned small, statement overridden), `shopper_04` own evidence beating the pool, `shopper_06` statement only, anyone else none (example-none text, never a refusal). `recordCorrection` takes an optional belief id: with none it stands Claimed/None and persists in container memory, deciding again on a later turn. New `product_fit` and `set_pooling` tools complete the eleven frozen calls. `agent/index.ts` instructions: recommend from `guidance.basis`, state confidence from the label, cite belief/correction/evidence ids with quoted pool counts, never let a correction travel in chat text only, recall on every turn including follow-ups. `agent/offline-check.ts` pins all five steps plus the standalone round-trip (39 checks). `agent/runtime/EVALUATION.yaml` is the 8-judge set: the three T3.5 prompts kept, plus one judge per precedence step that fires only on its own basis (so the set also reports which rung decided).

Live on `memento-agent` (build `92126f63`), six fresh-conversation turns, each with `recall_for_purchase` first in its spans: correction (`4c55d2dc`, $0.033), pool (`76814311`, $0.028, cites 60 settled 48 seeded/12 live), own-evidence (`0367307b`, $0.030), statement (`94f168f5`, $0.026, Claimed words-not-evidence), none (`165ec9cd`, $0.024, asks usual size), standalone single-turn (`8238de5b`, $0.038, recall + `record_correction` with no belief id, `corr_stand_01`). A later-session turn re-served the stored standalone correction. The `yes` confirmation turn carries no tool spans (nothing to recommend, purchase honestly not wired). Eval runs over all six traces: every step judge passes on its own step; approval true on all six; recall_first and correction_honoured true on all four traces where recall returned evidence or a standalone correction (F, A, B, C). Two prompt-strictness findings, both judge-side (agent replies correct): recall_first stays false on the statement-only and none traces because its evidence-citation clauses assume evidence exists; correction_honoured is false by its own design on turns with no correction. Owner: T10.3 (extend the evidence clauses for evidence-free recalls in the round-2 grader freeze, same class as the standalone fix already made). Spend $0.92 of $1.00 (turns $0.46 incl. two superseded-build turns, eval runs $0.46). Two surprises: the pool reply once paraphrased `Shoppers.Fit` (39) as fits-well verdicts (fixed by a quote-don't-paraphrase line, no re-drive needed); the first conditional judge wording confused the LLM judge into reasoned-true/output-false, fixed by unconditional per-basis prompts.

---

### T9.5: The second agent
```yaml
requires:   [T9.4]
fixture-ok: no
size:       S · mid
owns:       agent/second/
status:     done
```
A second Astropods deployment with its own name and prompt and its own traces, recalling through the same layer. It is written fresh: no shared prompt text with the reference agent beyond the layer's tool descriptions, so that its picking up a correction proves the layer carried it, not the prompt. Budget: under $0.50.

**Done when:** a correction made through the reference agent is honoured by the second agent's first recommendation, shown in both agents' traces. Spend is recorded.

**Notes (in-review).** `agent/second/` is the second deployment (`memento-second`, Second Fitting Helper, build `d2ae3104`, gateway `claude-sonnet-4-6` like the reference so turns cost the same). Four files are fresh: `agent/index.ts` (instructions rewritten — zero shared 5-grams with the reference prompt, verified by script), `agent/store.ts` (stand-in layer state with the same demo-shopper values: belief `blf_8f2c41`, correction `corr_3b7e52`, three seeded returns, the Earned-small pool with its seeded split), `agent/tools.ts` (the eleven frozen tools; descriptions are the layer's own frozen words from `internal/api/contract`, the one shared text allowed), `agent/check.ts` (29 offline checks, all green). No ordering tool, no `lookup_return` (reference-only read).

Live, one short turn each, single-message conversations: the reference agent took "I've lost weight — for Brand X oxfords, use M for me" (trace `17688f8f`): `recall_for_purchase` then `record_correction` between the generations, reply recommends M citing the fresh correction over `blf_8f2c41` with both sides and the pool counts shown. The second agent, asked only "I need a Brand X oxford shirt. What size should I take?" with no mention of any correction (trace `3620b1d9`): `recall_for_purchase` first, then its first recommendation is M from the recalled `corr_3b7e52` over `blf_8f2c41`, naming the rung, the Out-of-true label, the three seeded returns and the pool tallies with the 48-seeded/12-live split. The correction travelled through the layer state, not the prompt — the second prompt contains no correction story. Spend $0.13 of $0.50 (push+deploy $0.01, reference turn $0.05, second turn $0.04, one mis-addressed send $0.03).

Four things the next agent should not have to rediscover:

- Both deployments still answer from frozen stand-in state (same as T9.4's standing configuration); T9.6 must point both at the served layer before the demo claims a shared backend. The proof here is structural: the second prompt cannot produce the correction on its own.
- Drive turns with `agent/second/drive.py` (`DEPLOYMENT=<id> OUT=<dir> python3 drive.py create|send|history`); it needs the CLI's keyring token in process memory like `demo/drive.py`, and a browser UA. Never bake a deployment id into the script — the reference id lives in `demo/drive.py`, the second id stays in the operator's environment.
- `ast spec validate` passes locally; `ast push -V private -y` then `ast deploy --adapter web --wait`. A fresh blueprint needs an `AGENT.md` beside `astropods.yml` or push warns. The deploy took about a minute to reach Running.
- The second agent's closing line offered to "lock in" the size on a yes — approval-shaped, no purchase claimed, but the demo script should not take it up on the offer; there is nothing behind it yet.

---

### T9.6: The pivoted layer live on Render
```yaml
requires:   [T8.5b, T9.1, T7.3]
fixture-ok: no
size:       S · mid
owns:       deploy/render.yaml, deploy/seed/, deploy/guard.sh, deploy/provision.sh, .env.example, tools/env.sh
status:     done
```
The hosted service runs the T8.1 hub, the fit index (its live name, not a test prefix) and the seeded population, seeded once as T6.1 does. Measure `--max-sessions 128` and `--rate-limit-rps 200` on the 0.5 CPU starter; the T8.1b numbers are from a 12-CPU machine and do not stand for the starter. The Elastic key and the shopper salt reach the service as Render secrets, never in a file. Add the three auth key variables (`MEMENTO_AGENT_KEYS`, `MEMENTO_SHOPPER_TOKENS`, `MEMENTO_MERCHANT_KEYS`) to `deploy/render.yaml` as `sync: false`, name them in `.env.example`, and read them in `tools/env.sh` from the keyring; mint the keys with T8.5's command, never invent them. If seeding through the writer, call `Refunded` then `Classified` so a reason-bearing seed refund settles, and measure the forced-refresh cost at population scale. The fixture embedder cannot converge the frozen arc; keep `LAMBO_SEED=0` for the local loop. Raise `deploy/guard.sh`'s ceiling to the combined two-service shape ($14.25/mo run-rate); until then it trips exit 2 while `deploy/merchant/guard.sh` stays green for the pair. Append the merchant key entry to the layer's `MEMENTO_MERCHANT_KEYS` (digest from the T9.3d notes — secret stays in Render, never in a file). Any change to the monthly cost is recorded in the handoff log; the user has pre-approved the pivot's new services.

**Done when:** the public URL answers `recall_for_purchase` for the demo shopper with the pool step working, and `product_fit` for the demo oxford reads Earned. The guard reports green. Spend is recorded.

Staged (landed): the seed driver, auth key plumbing, starter-shape guard. The live-image wiring and the public-URL proof are T9.6b.

**Notes (worker).** `deploy/seed/` now seeds the T9.1 population as well as the demo graph, each under its own disk marker. The population seed runs first: it generates the population with `asOf` at the seeding moment, writes all 69 rows through one fit writer per shopper (`Refunded` then `Classified` for reason-bearing refunds, `Captured`+`Feedback`/`Kept` for the rest), and refuses the marker unless the pool reader returns T9.1's table (demo oxford Earned small; the other lines Building, Claimed, Out of true). The live index name is `fit-rows`; test runs take `-fit-index` with their own prefix (deleted afterwards) and `-fit-only`. The fit part is skipped when `MEMENTO_FIT_SEED=0` or `LAMBO_EMBEDDER=fixture`, so the documented local loop never touches the live index. `render.yaml` declares the salt plus the three auth vars as `sync: false`; `provision.sh` pushes all nine vars; `.env.example` names the three; `tools/env.sh` reads them from the keyring service `memento-keys`. The keys are minted with `memento key issue` (agent `agent-reference-01`, shopper `shop-demo-01`, merchant `merchant-app-01`, all seller `seller-acme-01`): entries live in the keyring for the deploy, tokens in the keyring service `memento-tokens` for their callers. All nine vars are set on the live service; values stage for the next deploy and the service stays green.

Measured. Population seed at scale: 187 forced-refresh writes in about 96 s (about 516 ms per write), verified back as 66 settled of 69 rows with the exact six-line verdicts. Hub flags on the starter shape (the pinned 0.10.1 binary in a container capped at 0.5 CPU and 512 MB, fixture embedder): 130 concurrent initializes with 9 live attach 116 and refuse 14 with 503, zero other errors; 600 stats calls over 8 workers sharing 4 sessions answer 156 ok with 444 throttled at 429 and zero other errors. The 200rps limiter, not the CPU, is the bound on the starter too. Hub RSS about 13 MB against the 512 MB instance.

Cost. The live inventory is now two starter services (the layer plus T9.3d's merchant app, provisioned while this task ran) and one 1 GB disk: $14.25/mo, recorded here. `guard.sh` audits that shape with a $16.00 ceiling and reports green live; the snapshot dry-run still trips. This task's own delta is $0: env vars and markers change nothing billable.

Two things the next agent should not have to rediscover:

- The public-URL Done-when is blocked outside this task's `owns`: the deployed image still runs the pre-pivot layer (`product_fit` answers unknown tool; recall has no pool or precedence fields), because wiring the pivoted layer into the image needs `cmd/memento` (pool reader, fit writer, identity salt, line resolver, standing dir, hub graph client), `deploy/start.sh` (boot order and env) and `deploy/Dockerfile` (population fixture) — none of which this task owns. The follow-up wiring task owns that; this task's seeder, keys, vars and guard are staged for it. The pool half is proved: the same live Elastic deployment returned the exact verdicts for the seeded population under the `t96-` prefix before the index was deleted.
- Keep the old Lambo arc seed until the pivoted image lands: removing it now would darken the current demo with no replacement, and its session is the same name the hub will serve the new layer on, so the wiring task should decide whether the arc retires with the replay.

**Public-URL proof (after the push that landed the stack, `ff9c922`, both services redeployed `live`).** Through the edge with the demo agent key: `POST /mcp tools/call recall_for_purchase` for `shopper_01` on the demo line reads `pool_lookup: consulted`, pool **Earned small** (22 settled, 22 seeded, 0 live, precedence `pool` overridden by `correction`), overriding correction `corr_3b7e52` ("I've lost weight, use M"), guidance basis `correction`, and the two beliefs it contradicts — the frozen story end to end. `POST /tools/product_fit` for brand-x-001/shirts/regular/M reads **Earned small**, 22 settled / 22 seeded / 0 live, precedence null. Merchant proxy (no caller credential) answers the same Earned shape, so the deployed page's live vintage is real. `bash deploy/guard.sh --check`: 2 services, 1 GB disk, $14.25/mo against the $16.00 ceiling, green. Spend $0.00 of model (no model call).

**Merchant key gap found and fixed live.** The merchant service held a key whose digest was absent from the layer's `MEMENTO_MERCHANT_KEYS` (T9.3d minted `merchant-app`; T9.6's entry was `merchant-app-01` from a different mint), so the proxy answered 401 `unauthenticated` after the first pivoted deploy. Fixed as an operator step: the merchant service's key digest was appended to the layer's `MEMENTO_MERCHANT_KEYS` (`merchant-app-live|…|seller-acme-01|-|-`), the layer redeployed, and the proxy read above is the proof. Values never entered the repo; the Render env-vars echo was held in a private temp file and deleted. Next agent: when minting a merchant key, register its digest on the layer's secret in the same step, and verify the proxy read — a keyless 401 is this exact gap, not a code defect.

---

### T9.6b: Wire the pivoted layer into the deployed image
```yaml
requires:   [T8.5, T9.6]
fixture-ok: no
size:       M · mid
owns:       cmd/memento/main.go, deploy/start.sh, deploy/Dockerfile
status:     done
```
Build the pool reader + fit writer (live salt, catalog line resolver, standing dir on disk) + hub-backed graph client into the served layer, boot it in `start.sh` before/after the seed, and ship the population fixture in the image. First boot seeds `fit-rows` once via the disk marker (seed through `Refunded` then `Classified`; skip when `LAMBO_EMBEDDER=fixture` or `MEMENTO_FIT_SEED=0`). Register the merchant key entry in the layer's `MEMENTO_MERCHANT_KEYS`. Decide the old-Lambo-arc seed's retirement (its session name collides with the hub's future shopper session).

**Done when:** the public URL answers `recall_for_purchase` for the demo shopper with the pool step working, and `product_fit` for the demo oxford reads Earned. The guard reports green. Spend is recorded.

**Notes (worker).** `cmd/memento/main.go` serves the pivoted layer when the hub is wired (`LAMBO_MCP_URL` plus `LAMBO_MCP_TOKEN`; fail closed without the salt). One `Service` per shopper over its own hub session (named by the live keyer), its own evidence store and the shared pool reader, fit writer, static line resolver and standing dir; the default service answers only what names no shopper (`tools/list`, `product_fit`, health, the aggregated call log) over a graph nothing can reach. The hub client speaks MCP at `/mcp/s/<session>` with the bearer token, opens its client session lazily, re-opens it once after a failure, waits every write receipt to applied, and maps `no concept matching` to None. `staticLines` resolves through the seeded population's line list with brand and category from the item itself and the regular fallback for unseeded lines; unknown brand or category refuses. New `session` subcommand prints one shopper's hub session under the live salt for the boot script; the offline path is byte-identical without the hub variables. `deploy/start.sh` pivots on `MEMENTO_IDENTITY_SALT`: the demo session and reader allowlist move to the live name, the standing dir is created, the hub (plus a TCP wait) comes before the layer, the seed runs fit-only under the shared disk marker, and the graph arc is never planted again — it retires with the replay, unread on the same disk. `LAMBO_SEED=0` skips the fit seed; the binary still skips it under fixture or `MEMENTO_FIT_SEED=0`. `deploy/Dockerfile` builds with `seed/` and ships `testdata/seed/population` in the image; the web stage also carries `testdata/api`, which the ledger's fixture imports need (main's image never built after T9.2 without it — pre-existing, fixed here).

**Arc retirement.** The old arc lives at the seed-salt session; the pivoted layer serves the live-salt session, so the arc is orphaned, not colliding, the moment the image lands. Nothing plants it on fresh disks; the existing disk keeps it unread beside the live data. The ledger reader follows the live session; the stale session drops from its allowlist. No contract change: the frozen shape is untouched, and T7.4 stays with the orchestrator.

**Local proof (this task's Done-when).** Final image booted locally with live keys native embedder and `MEMENTO_FIT_INDEX=t96b-proof`: pool and fit wired, memory seed holds, fit seed 69 rows / 187 writes with all six T9.1 verdicts verified back before the marker. `recall_for_purchase` for `shopper_01` on the full demo line: basis correction, pool Earned small consulted (settled 22, seeded 22, live 0), beliefs Out of true / Venerable / contradicted plus the Claimed statement — the frozen story exactly. `product_fit` for the demo oxford: Earned small with the seeded counts. The unknown shopper still gets GuidanceNone, never a refusal, and an unwired `product_fit` refuses rather than report an empty pool (host-level hub run). Restart keeps a live standing correction through the disk journal and the same hub session. Proof index deleted afterwards (`t96b-proof*` listing `[]`); production `fit-rows` untouched. Spend $0.00 of model (no model call; Elastic test-prefix churn only). The gate passes.

**Post-push check (orchestrator, after main lands and Render redeploys).** `curl` the public layer (through the edge with the agent key as bearer): `POST /mcp tools/call recall_for_purchase` for `shopper_01` on the demo line reads basis correction with pool Earned small consulted; `POST /tools/product_fit` for brand-x-001/shirts/regular/M reads Earned small. Then `bash deploy/guard.sh --check` (green at the $16.00 ceiling) and the merchant-key append below.

**Merchant key (operator step, not a repo change).** The entry (digest, in the keyring service `memento-keys` kind `merchant`, already verified present) must be appended to the live service's `MEMENTO_MERCHANT_KEYS` Render secret beside the current entries, then verified with the merchant proxy read. Values never enter the repo; nothing here changed the key variables.

Three things the next agent should not have to rediscover:

- The graph seam carries no shopper, so one service cannot serve two hub sessions: the director is the whole isolation story. The edge's `X-Memento-Shopper` wins routing; a body without one reaches the default service and its frozen error.
- The marker `seed_fit_only` writes derives to the same fit-marker path the legacy seed used, so the staged live population is honoured on the first pivoted boot instead of reseeded.
- The hub needs its port to accept before the layer's seed replays: `wait_hub` is the only new boot dependency, and a hub that never answers fails the boot instead of serving half a layer.

---

### T9.3e: Merchant page reads live pool data
```yaml
requires:   [T9.3d, T9.6]
fixture-ok: yes
size:       S · mid
owns:       merchant/
status:     done
```
Wire the merchant page from the fixture transport to same-origin `/tools/product_fit`; no caller credential is needed (the edge holds the merchant key server-side). Until then the proxy answers 502/503 and the app reads its bundled snapshot. T9.6 registers the key entry on the layer first.

**Done when:** the deployed page renders live pool counts through the proxy; the fixture transport remains for tests; the gate passes.

**Notes (worker).** `merchant/src/lib/pool/http.ts` gains `createSameOriginTransport`, which posts to `/tools/product_fit` on its own origin with no `authorization` header — the edge injects the service key and drops any client-sent one. `merchant/src/lib/pool/source.ts` is the page's read order as `loadSellerPools`: the whole six-line grid loads live first, and wholly from the bundled snapshot when the live read fails, never a half-live mix; a snapshot failure fails loudly. `+page.svelte` reads live-first with the snapshot fallback and names the vintage under the heading ("Live pool counts, read through the merchant proxy." vs "Bundled snapshot: the live read is not answering yet."). The fixture transport is untouched and still serves every test. New gates: `same-origin.test.ts` (same-origin post shape, no credential, no other tool, no shopper field, loud refusal) and `source.test.ts` (live-first, whole-grid snapshot fallback, loud snapshot failure). The gate passes: merchant 8 files, 45 tests, both frontends green.

**Live proof (local edge + served layer, not the public layer).** Seeded the T9.1 population into scratch index `t93e-proof` (`deploy/seed -fit-only`, all six verdicts verified back), served it with the unmodified pivoted layer on its loopback port (pool and fit wired; hub address dummy — `product_fit` serves from the default service and never dials it), and pointed the unmodified merchant edge at it with a locally minted merchant key. Through the proxy with no caller credential, all six lines read Earned/Building/Building/Building/Claimed/Out-of-true with seeded-only counts (demo oxford Earned small, settled 22, seeded 22, live 0 — the live shape, distinct from the bundled 60/48 snapshot); any other `/tools/` path 404s at the edge untouched. With the layer stopped the proxy answers 502 `layer-unreachable` and the page falls back to the snapshot (unit-proved). Index deleted afterwards (`t93e-proof*` listing `[]`); production `fit-rows` untouched. Spend $0.00 (no model call; Elastic test-prefix churn only).

**Live vintage verified (after the T9.6 push).** The deployed merchant service now reads the layer: `POST https://memento-merchant.onrender.com/tools/product_fit` with no caller credential returns Earned small, 22 settled / 22 seeded / 0 live for the demo oxford — the live shape, not the bundled 60/48 snapshot. The merchant key gap behind the earlier 401 is recorded under T9.6.

Three things the next agent should not have to rediscover:

- The same-origin transport sends no key by construction, so the no-elastic proof test's env-var pin (`MEMENTO_LAYER_URL`, `MEMENTO_MERCHANT_KEY`) is untouched: those names now belong to the direct-layer transport and the deploy, not the page.
- The deployed page picks up this change with the next merchant-service redeploy (static build); no edge or layer change is needed, but the live vintage also needs T9.6's merchant-key entry on the layer's `MEMENTO_MERCHANT_KEYS`.
- The built bundle carries `/tools/product_fit` and no `authorization` string — grep the page chunk after any transport change.

---

### T9.3f: Merchant reads the tightened contract
```yaml
requires:   [T7.4]
fixture-ok: yes
size:       S · mid
owns:       merchant/
status:     done
```
T7.4 requires the pivot pool `precedence`, but merchant's `decodePool` dropped it, so the decoded Earned demo oxford no longer equalled the frozen snapshot. Carry the required pool `precedence` through `decode.ts` + `types.ts` the way T9.2b did for web. One-test fix; no contract change.

**Done when:** the failing snapshot test passes; the gate passes.

**Notes (worker).** `PoolView` gains `precedence: PoolPrecedenceView | null` (required, null on `product_fit`), with the new `PoolPrecedenceView` mirroring web's `web/src/lib/ledger/pool.ts` step union; `decodePool` requires the key and `decodePoolPrecedence` returns null for the null wire value while rejecting a misshapen one. `makeRow` in `fixture.ts` takes `Omit<PoolView, 'precedence'>` and injects `precedence: null`, so the five hand-built rows round-trip through the decoder unchanged. `index.ts` re-exports the new type. The snapshot test passes unmodified, and merchant is green (8 files, 45 tests, svelte-check, eslint).

Two things the next agent should not have to rediscover:

- The merchant `precedence` decoder deliberately matches web's semantics (missing key fails, null decodes to null) rather than the old merchant behaviour of silently dropping the field; a future non-null pool precedence decodes without further changes.
- `makeRow` round-trips every hand-built row through `decodeProductFitResponse`, so any future required pool field needs no new fixture edits beyond the `Omit` line — but the decoded row must still satisfy `decidePool`'s re-decision check in `pool.test.ts`.

---

### T9.3g: Align the image's Node pin
```yaml
requires:   [T9.3d]
fixture-ok: no
size:       XS · mid
owns:       deploy/Dockerfile, deploy/README.md
status:     done
```
The demo image builds `web/` on Node 22 while T0.2's contract and CI pin Node 26. The deployed build works, but the pin has drifted, so a future frontend build could pass locally and CI and fail in the image (or the reverse). Align the image's build stage with the pinned Node version, prove the image builds and serves the app, and record the pin so it is checked with the rest of the toolchain.

**Done when:** the Dockerfile's frontend build uses the pinned Node major; the image builds and serves `/` and `/healthz`; the pin is named where the other toolchain pins are recorded; the gate passes.

**Notes (worker).** `deploy/Dockerfile`'s `web-build` stage moves from `node:22-bookworm` to `node:26-bookworm`, the major T0.2's stack records (Go `1.27.1`, Node `26`), CI's `setup-node` pins, and the new table in `deploy/README.md` names. Nothing else in the file needed the pin: `deploy/Dockerfile` builds `web/` only, and the merchant app is a separate image (`deploy/merchant/Dockerfile`). `deploy/README.md` gains a **Toolchain pins** section — Go 1.27, Node 26, Lambo 0.10.1, each with the image line and where that pin is checked — plus the fact that neither `web/package.json` nor `merchant/package.json` declares `engines`, so the image base and CI's `setup-node` are the whole frontend pin.

Proof, local docker 29.9.3: `docker build -f deploy/Dockerfile -t memento-t93g:local .` succeeds; a second build at `--target web-build` answers `v26.13.0` to `node --version` inside the stage and leaves `index.html`, `200.html` and `_app` in `/src/web/build`, so the app is built on the pinned major, not on a cached older base. The runtime image serves `/healthz` with `{"ok":true}` (200) and `/` with the ledger app (200, `Memento Emptor` title, `_app` chunk references), and `/belief` and `/ledger` answer 200 through the `200.html` fallback. Booted with the fixture embedder, `LAMBO_SEED=0` and a throwaway identity salt minted per run, so no live index and no key was touched.

Three things the next agent should not have to rediscover:

- `MEMENTO_IDENTITY_SALT` is not optional in any run of the image or of `start.sh`: the supervisor always exports `LAMBO_MCP_URL` and `LAMBO_MCP_TOKEN`, and the layer refuses those without the salt that names its sessions, so a boot without one stops with `LAMBO_MCP_URL and LAMBO_MCP_TOKEN need MEMENTO_IDENTITY_SALT`. The documented local loop was stale on this point; fixed here by adding a per-run salt to its command, since that section is how the image is proved.
- The merchant image was still on `node:22-bookworm` (`deploy/merchant/Dockerfile`), a second image this task's `owns` excluded. Fixed by T9.3h: both images' `web-build` stages now use Node 26, and `deploy/README.md`'s toolchain table covers both.
- A `web-build` cache hit after a base-image change is the failure mode to watch when re-proving this task: the pin lives in a `FROM` line, and only `--target web-build` plus `node --version` inside the stage shows the major the app was actually built on.

**Review (fresh eyes).** Re-verified from the committed files on a branch cut at the task tip, no live deploy, no key. `docker build -f deploy/Dockerfile --target web-build` on docker 29.9.3 answers `v26.13.0` inside the stage and leaves `index.html`, `200.html`, `belief.html`, `ledger.html` and `_app` in `/src/web/build`; the full image serves `/healthz` with `{"ok":true}` (200), `/` with the `Memento Emptor` title and `_app` references (200), and `/belief` and `/ledger` (200, through the `200.html` fallback, which the edge reaches because `directFile` stats the bare path). Booting the same image without `MEMENTO_IDENTITY_SALT` stops with `LAMBO_MCP_URL and LAMBO_MCP_TOKEN need MEMENTO_IDENTITY_SALT`, so the README's *required, not optional* is confirmed, not inferred; `openssl rand -hex 32` is the 64 hex characters `identity.NewKeyer` accepts. The base image is the only Node pin: neither app declares `engines`, `packageManager` or an `.nvmrc`-style file. Fixes landed in this review: the pin table's third column now names what checks each pin (the Lambo pin is the image build's own `sha256sum -c`, no CI step), the Go tag's patch float is stated, the drift paragraph names T9.3h as its owner, and the local-run salt note states the no-index-wired precondition it rests on.

---

### T9.3h: Align the merchant image's Node pin
```yaml
requires:   [T9.3d, T9.3g]
fixture-ok: no
size:       XS · mid
owns:       deploy/merchant/Dockerfile, deploy/README.md
status:     done
```
T9.3g aligned the demo image's frontend build to the pinned Node 26, but `deploy/merchant/Dockerfile` still builds `merchant/` on `node:22-bookworm`. It is a separate image, so the pin drifted independently; align it the same way, prove the merchant image builds and serves `/` and `/healthz`, and record the pin beside the others.

**Done when:** the merchant image's build stage uses Node 26; the image builds and serves `/` and `/healthz`; the pin table covers it; the gate passes.

**Notes (worker).** `deploy/merchant/Dockerfile`'s `web-build` stage moves from `node:22-bookworm` to `node:26-bookworm`, with a comment naming the pin the way `deploy/Dockerfile`'s web stage does. Nothing else in the file needed it: the `go-build` stage is already `golang:1.27-bookworm` (the tag the demo image uses, whose `1.27` patch floats where `go.mod` and CI fix `1.27.1`), and the runtime stage is `debian:trixie-slim`, which the pin does not cover. `deploy/README.md`'s **Toolchain pins** section now speaks for both images: the table's "Where the images pin it" column names `Dockerfile` and `merchant/Dockerfile` for Go and Node, and the Node paragraph says both `web-build` stages run the same `npm ci` and `npm run build` as CI on the same major. T9.3g's known-drift paragraph is gone, since the drift it named is fixed.

Proof, local docker 29.9.3: `docker build --no-cache -f deploy/merchant/Dockerfile --target web-build` answers `v26.13.0` to `node --version` inside the stage and leaves `index.html`, `200.html` and `_app` in `/src/merchant/build`, so the merchant app is built on the pinned major rather than a cached older base. The full image starts with `MEMENTO_LAYER_URL` pointed at a dead loopback port and `MEMENTO_MERCHANT_KEY` holding a scratch value, serves `/healthz` with `{"ok":true}` (200), `/` with the `Memento Merchant` title and its `_app` chunk (200s), answers any other `/tools/` path with the edge's 404 `unknown_tool`, and answers `POST /tools/product_fit` with 502 `layer-unreachable` after dialling that dead loopback address, so nothing leaves the machine. With no merchant key the same request is 503 `key_not_configured` and the layer is not dialled at all. The container and both image tags were removed afterwards; the gate passes (Go vet and tests, both lockfiles, both svelte-checks, both lints, both vitest suites, both builds).

Three things the next agent should not have to rediscover:

- The merchant image is a second `web-build` stage, not a second copy of the pin: `deploy/Dockerfile` and `deploy/merchant/Dockerfile` each carry `node:26-bookworm` in their own `FROM`, so a future Node move is two lines, and T9.3i is the task that makes a disagreement fail mechanically.
- The `--no-cache` is the point: the pin lives in a `FROM` line, and a cache hit after a base change reuses the stage built on the old major. `--target web-build` plus `node --version` inside it is the check.
- The merchant edge needs no layer to answer `/` and `/healthz`: it dials the layer only on the proxy path, and only when a merchant key is set. The image is therefore proved with a dead loopback layer URL and a scratch key; a proxy read against a real layer is proved in T9.3e, which owns its live vintage.

**Review (fresh eyes).** Re-verified from the committed files on a branch cut at the task tip, no live deploy, no key, $0 (local docker only). `git show --stat 444b8bc` is the three files the task names: `deploy/merchant/Dockerfile`, `deploy/README.md`, and this block. `deploy/merchant/Dockerfile`'s `web-build` is `node:26-bookworm`, matching `deploy/Dockerfile`'s `web-build` and `ci.yml`'s `setup-node` `26`; T0.2's notes record Go `1.27.1` and Node `26`, so the table's "checked by" column is sound. A `--no-cache --target web-build` build on docker 29.9.3 answers `v26.13.0` and `npm 11.20.0` inside the stage and leaves `index.html`, `200.html`, `version.json`, `_app/` and `immutable/` in `/src/merchant/build`; the full image serves `/healthz` with `{"ok":true}` (200), `/` with the `Memento Merchant` title and its `_app` chunk (200), the chunk itself (200), `/tools/other` 404 `unknown_tool`, and `POST /tools/product_fit` 502 `layer-unreachable` with a scratch key against a dead loopback URL (503 `key_not_configured` without one). The gate passes at the tip. Fixes landed in this review: `deploy/README.md` no longer calls `npm ci` a gate command — the gate runs `npm ci --dry-run` and the install is CI's, so the sentence now says "as CI"; and the worker's proof above is corrected to 502 `layer-unreachable`, to the scratch merchant key it actually needs, to the dial it does make (loopback only), and to the Go tag's 1.27 patch float. Finding (Low), owner T9.3i: T9.3g's carry-over bullet still reads in the present tense ("The merchant image is still on `node:22-bookworm` ... the drift is recorded in `deploy/README.md`'s toolchain section until it lands"), both claims now false; its fix is a one-sentence past-tense rewrite in the T9.3g block, which is outside this task's `owns` and outside the phase lines this review may touch, so T9.3i's brief should name that block.

---

### T9.3i: Check the toolchain pins mechanically
```yaml
requires:   [T9.3g, T9.3h]
fixture-ok: yes
size:       XS · mid
owns:       tools/check.sh, .github/workflows/ci.yml, tools/pincheck/
status:     done
```
The image toolchain pins live in Dockerfiles and in a README table; no check compares them with `ci.yml`'s `setup-node` and `setup-go`, so the Node 22 drift could recur silently. Add a gate step that reads every Dockerfile's build stages and the CI versions and fails when a frontend build stage's Node major or a Go build stage's Go version disagrees with its pin.

**Done when:** a table test proves the check fails on a mismatched Node or Go stage and passes on the current tree; the gate runs it; the gate passes.

**Notes (worker).** `tools/pincheck` reads the six places the pins live — `deploy/Dockerfile` and `deploy/merchant/Dockerfile`'s `golang:` and `node:` `FROM` lines, `ci.yml`'s `go-version:` and `node-version:`, `go.mod`'s `go` directive, and `deploy/README.md`'s table rows — and fails on the first disagreement, naming the drifting file, the line, and both versions. It is standard library only, no npm package, no key, no network. `tools/check.sh` runs it as `go run ./tools/pincheck` before gofmt, with a comment saying why; `ci.yml` is unchanged because CI runs `check.sh` and the step rides inside it.

Comparison follows the granularity each file carries. Node is compared by major and Go by major.minor, because the image tag and the table name no patch; the two files that do name one, `go.mod` and CI, are additionally held to each other exactly, so a patch drift inside the same minor fails too. The expected value is the one most files state, so a single drifted file is named and the agreeing ones are not blamed; a tie names every pin for that toolchain. Every source must state its pin — a Dockerfile missing its Go or Node stage, a CI key that vanished, a dropped table row — so the check cannot pass by reading less. `-root <dir>` points it at another tree.

Proof: `go test -race ./tools/pincheck` runs `TestCheckTable`'s fourteen cases on a fixture tree — Node 22 in either image, Node 22 in both images at once, Node 22 in CI, Node 24 in the table, Go 1.26 in the merchant image, Go 1.26 in the table, a 1.27.0/1.27.1 patch drift between `go.mod` and CI, a `go.mod` that lost its `go` directive, a Dockerfile missing its web stage, a lost `setup-node`, a lost table row, and an unpinned `golang:latest` — each asserted to fail naming that file and that version, plus the agreeing baseline; `TestCurrentTreeAgrees` reads this repository itself and passes. On a copy of the real files, `go run ./tools/pincheck -root <copy>` names `deploy/Dockerfile`'s node stage line 32 at 22 against `deploy/merchant/Dockerfile`'s 26, `deploy/merchant/Dockerfile`'s go stage line 7 at 1.26 against `deploy/Dockerfile`'s 1.27, `go.mod`'s 1.27.0 against CI's 1.27.1, and CI's 22 against the images' 26, exiting 1 each time; on the untouched tree it prints `go 1.27 and node 26 agree across 9 declarations`. The gate passes (`./tools/check.sh`, green end to end, the pin step included).

Two findings, both Low, both outside this task's `owns`:

- `deploy/README.md`'s toolchain section still says "no CI step reads these tags, so the table is the record and the named checks are the guard". The gate now reads them, so that limit is closed and the sentence is false. Fixed by the T9.3i review, whose brief granted that one sentence: it now says the gate runs `tools/pincheck` over every file that states a pin, fails naming the drifted file and version, and fails when a source stops stating its pin. The asymmetry next to it (the Go tag floats the patch where `go.mod` and CI fix 1.27.1) is still true and still intended.
- T9.3h's review filed a Low finding so T9.3i's brief would name the stale present-tense bullet in T9.3g's notes. The brief does not, and the bullet in the tree already reads in the past tense ("was still on `node:22-bookworm` ... Fixed by T9.3h"), so there is nothing left to fix; the finding is closed, and the orchestrator should not re-file it.

Three things the next agent should not have to rediscover:

- The comparison keys are deliberate, not incidental: Node's key is its major and Go's is its major.minor. Pinning a Node patch or a Go minor at a finer grain means changing `minParts`, and the exact `go.mod`-vs-CI rule already covers the patch where both files state one.
- Each image the check reads is required to carry both a `golang:` and a `node:` stage. A future image that is only one of the two fails the check by design rather than dropping out of it silently; the requirement loops in `dockerPins` are where that changes. The two images are named literally, so an image added elsewhere is not read at all — the review records that edge.
- The failure path is provable without editing `deploy/`: copy the five files to a scratch tree and run `go run ./tools/pincheck -root <scratch>`. The `FROM` line and the README row are the only places a pin is written, so a copy is a faithful test of the real read.

**Review (fresh eyes).** Re-verified from the committed files on a branch cut at the task tip, no live call, no key, $0. `git show --stat fbd2088` is the four files the brief allows: `tools/check.sh`, `tools/pincheck/main.go`, `tools/pincheck/main_test.go`, and this block. `.github/workflows/ci.yml` is untouched, and the reason it can be holds: line 35 runs `./tools/check.sh`, so the pin step rides inside the one script CI runs and no second install path is created. `go run ./tools/pincheck` on the tree prints `go 1.27 and node 26 agree across 9 declarations` and exits 0; that is two stages in each image, CI's two keys, `go.mod`, and the table's two rows, so no source is missing from the count.

Mutation-checked on a copy of the five real files under a scratch root (`-root`, `deploy/` never edited): Node 26→22 in the demo image, in the merchant image, in `ci.yml` and in the table; Node 22 in both images at once; Go 1.26 in the merchant image; Go 1.27.1→1.28.1 in `ci.yml`; `go.mod` 1.27.1→1.27.0; `golang:latest`; `go.mod`'s directive deleted; `go-version:` deleted; the merchant Dockerfile deleted. Each exits 1 and names the drifting file, its line and both versions — the tie names all four pins, each deletion names the lost source — and the demo image at 22 against the merchant's 26 is reported with the demo image as the one that moved, not the first file read. An untouched copy exits 0, so the failure is the mutation and not the copy.

Coverage added in this review: two cases in `TestCheckTable`, `go.mod` losing its `go` directive and the two-two tie. The fourteen cases now reach every reader's missing-pin branch and the no-majority path; before them the only missing-pin cases were the node ones, read through the same loops as the go ones but not asserted.

Fixes landed in this review: `deploy/README.md`'s toolchain section no longer says "no CI step reads these tags, so the table is the record and the named checks are the guard". It now says the gate runs `tools/pincheck` over every file that states a pin, fails naming the drifted file and version, and fails when a source stops stating its pin; the Go tag's patch float stays as the one intended asymmetry. Nothing in `tools/pincheck` itself needed a change: the granularity (Node by major, Go by major.minor, with `go.mod` and CI held to the patch exactly) is what the files state, and a `golang:1.27.1-bookworm` tag on the copy still agrees, so the finer-grain case does not produce a false failure.

Two limits, both recorded rather than changed here:

- The check names its three pin files literally, so a third image added under `deploy/`, or a second workflow, would not be read. It is the worker's documented choice — the "both stages required" rule and the loops that would change are in the notes above — and no P9 task adds either file, so this is a known edge of the guard, not a defect in it: the orchestrator's call is whether to walk the tree, which needs a decision about an image that carries one toolchain only.
- T9.3i's `owns` line reads `tools/check.sh, .github/workflows/ci.yml`, and the landed commit also creates `tools/pincheck/`. The review brief allowed the path and the task block does not, so the record and the commit disagree by one directory. The fix is one line in this block, which only the orchestrator writes.

---

### T9.8: Agents read the served layer
```yaml
requires:   [T9.4, T9.5, T9.6]
fixture-ok: no
size:       M · frontier
owns:       agent/runtime/, agent/second/
status:     done
```
Both deployed agents are fake-backed: `agent/runtime/agent/memory.ts` and `agent/second/agent/store.ts` serve a standing copy of the pivot shapes, so a demo turn never reaches the live layer. T9.7's central claim — a correction made through the reference agent is honoured by the second agent from the shared layer — cannot be shown while the tools are stand-ins. Replace the fake tools with HTTP calls to the layer's edge (`/tools/{tool}`) carrying an agent key in `Authorization: Bearer` and the shopper id in the body, wire the key through the deployment's secret store (never a file, never printed), redeploy both agents, and prove one live turn per agent: recall first, correction recorded, the other agent honouring it. Budget: under $2.00 of agent spend; run one turn first and check its cost. If a live turn cannot be made to work, record precisely what fails and hand the decision back to the orchestrator rather than faking it.

**Done when:** both agents' tools call the served layer through its edge with an agent key supplied by the deployment's secret store; a live reference-agent turn records a correction and a live second-agent turn honours it from the same layer, with both traces recorded; no token appears in any file or log; the gate passes.

**Notes (in-review).** Both deployments now read and write the served layer; no stand-in state is left in either path. `agent/runtime/agent/memory.ts` and `agent/second/agent/store.ts` are deleted. Each deployment has one transport of its own — `agent/runtime/agent/layer.ts` and `agent/second/agent/edge.ts` — which posts `POST <edge>/tools/<tool>` with `Authorization: Bearer <agent key>` and the shopper id in the body, times out at 20 s, and turns a refused or unreadable answer into a typed error carrying the layer's own code and status. `layerFromEnv` / `landingFromEnv` read `MEMENTO_AGENT_KEY` and `MEMENTO_LAYER_URL` from the deployment's environment, and the edge defaults to the hosted layer. Both `tools.ts` files are now thin bindings over the transport; the reference agent's fake `lookup_return` read is gone, because the layer serves no order read (orders stay T3.3). The tool input schemas now require a brand on recall and say Brand X is `brand-x-001`; the instructions name the brand and line so a live recall can answer.

The key path, with no value in the repo: keyring → account vault secret `MEMENTO_AGENT_KEY` (`ast secrets create`; `ast secrets get` shows `Value: ******`) → declared per blueprint as a `secret: true` input in `agent.inputs` → injected by reference at deploy (`ast agent redeploy --latest --var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY`) → the deployment's own env, which the platform reports by name and secrecy only. Declaring the input is the step that matters: `--var` on a blueprint that declares no input is accepted and silently injects nothing, which the first live turn proved the honest way — the agent answered that its key was not set (trace `fbb0a92d34e04834423a77104c729c91`, $0.039, no write).

Live turns, one message each. Reference agent (`memento-agent`, build `027f15d4`, trace `ed617f32e79fb8d20fde1e1174bc1ef0`, $0.0505): "I've lost weight — for Brand X oxfords, use M for me" produced `recall_for_purchase` (239 ms) then `record_correction` (319 ms) in that order; the reply cites the recalled `corr_3b7e52`, the Out-of-true belief `blf_8f2c41` with its three returns, and the pooled 22 settled / 22 seeded / 0 live, and reports the correction it wrote, `corr_000001`, minted by the layer. Second agent (`memento-second`, build `5cad0ad3`, trace `6d764a788aba072bcaabb043e2551448`, $0.0346): "I need a Brand X oxford shirt. What size should I take?" produced `recall_for_purchase` first and then recommended M from the same `corr_000001` over `blf_8f2c41`, keeping the 22-settled seeded pool visible as overridden. Both traces were produced with no mention of the correction in the second agent's prompt. The layer's own call log shows the three calls to shopper_01 in order — the reference agent's recall then its record_correction, then the second agent's recall — so the writes and reads landed on the served layer, not on anything local.

The offline checks still run offline and no longer need a fake tool layer: they inject a fake transport that answers from the frozen examples in `testdata/api` (so the checks read the contract's own shapes), plus a loopback stub that pins the request the deployment sends — path, bearer token, shopper in the body, JSON content type — and the fault paths (401 carries the layer's code, a non-JSON answer, a deployment with no key). `bun agent/offline-check.ts` runs 58 checks, `bun agent/check.ts` 49; both green.

Spend $0.14 of the $2.00 budget: $0.039 the wiring-failure turn, $0.0505 the reference turn, $0.0346 the second, the rest platform compute for two builds and four redeploys. No token appears in the tree or in any commit (`git grep` and the full history both come back empty for the value).

Three things the next agent should not have to rediscover:

- A deployment variable only reaches the container when the blueprint declares it under `agent.inputs`; `--var KEY=@SECRET` against an undeclared input reports success and injects nothing, and the symptom is the agent's own "key not set" answer, not a platform error.
- The platform's messaging history returns the assistant message with empty content on these builds, while the reply text is in the trace's `mastra.agent_run.output`. The T9.7 demo should read the reply from the trace (where T9.4 and T9.5 read it), not rely on `history`.
- The live pool for the demo line is seeded-only (22 settled, 22 seeded, 0 live) after the T9.6 population seed, and the correction the reference turn wrote is `corr_000001`. A demo reset that replays the arc will mint a new id; nothing in the agents pins either value.

**Review (T9.8).** Re-verified from the committed files and the platform, not from this note. Both offline checks are green as stated (58 and 49). `ast agent trace` returns both traces with the recorded costs, the build ids these deployments report, the recall span before the correction span in the reference turn, and — in the second agent's reply — `corr_000001`'s own words, which its instructions do not carry. `ast deploy --dry-run --var NOT_DECLARED=hello` beside the declared secret validates clean, so the undeclared-input trap is real: the platform drops it silently while a declared input is validated. No keyring value appears in the tree or anywhere in the history. Three findings, all low and none blocking.

- Both agents expose `ledger_beliefs`, `ledger_belief` and `ledger_evidence`, but the edge admits shopper callers only for those three (`internal/auth/surface.go`), so the deployed agent key is refused on each. No instruction asks for them and no T9.8 turn called one; the refusal is reported honestly rather than answered from a copy, so the demo is unaffected. Owner: T9.9 (proposed, `agent/runtime/`, `agent/second/`): align each agent's exposed tools with the edge's access table, or the orchestrator widens the rule.
- The offline checks inject the transport below `tools.ts`, so the tool ids, schemas and `execute` bindings are not exercised without a deployment. Recall and `record_correction` are the only two proven live. Owner: T9.9 (proposed).
- The reference agent's step 1 says "If a tool call errors, retry it once". Against a live layer a `record_correction` that landed and then timed out would be written twice. Owner: T9.9 (proposed): retry reads only, or recall before retrying a write. It is a prompt edit plus a redeploy, so it waits for the orchestrator.

Two review fixes, both prose, no behaviour change: `agent/runtime/agent/tools.ts` and its README said the tool descriptions were the layer's own frozen words. That holds for the second agent and for six of the reference agent's eleven; the descriptions are the layer's words plus this agent's recall-first guidance, and both now say names and input shapes are the frozen contract.


---

### T9.9: Agent tool surface and retry discipline
```yaml
requires:   [T9.8]
fixture-ok: yes
size:       S · mid
owns:       agent/runtime/, agent/second/
status:     done
```
The T9.8 review found three low defects in the live agent surface. Both agents expose `ledger_beliefs`, `ledger_belief` and `ledger_evidence`, which the edge admits for `CallerShopper` only, so an agent key gets a 403 on each. `tools.ts` has no offline coverage: the checks inject the transport below it, so tool ids, schemas and execute bindings are exercised only by a live build. And the reference prompt tells the agent to retry any failed tool call once, so a `record_correction` that landed and then timed out could be written twice; retries must be for reads only, or a write must recall before retrying.

**Done when:** each agent's exposed tools match the edge's access table (or the table is widened deliberately, recorded), a check exercises the tool bindings themselves against the frozen contract, and the retry instruction cannot duplicate a write; the gate passes and the deployed builds match the committed code.

**Deployed (orchestrator).** Both agents were redeployed from the committed code after the task: `memento-agent` build `40454bd3` and `memento-second` build `363ada53`, both Running. One live verification turn on the reference build (trace `4a60646cc27b9f8b201a104e01017e3d`, $0.0312) recalled first and recommended M from `corr_000004`, so the eight-tool surface and the new retry wording do not break the served path.

**Notes (in-review).** Both agents now offer exactly the eight calls the edge's access table admits to an agent key. `ledger_beliefs`, `ledger_belief` and `ledger_evidence` are gone from `agent/runtime/agent/tools.ts`, `agent/runtime/agent/layer.ts`, `agent/second/agent/tools.ts` and `agent/second/agent/edge.ts`, so an agent key is never offered a call the edge would refuse. The table was not widened, and widening it is not advised: those three reads return one shopper's ledger rows, the ledger is the shopper's own view (T9.2 serves it to a shopper link), and an agent key is bound to no shopper, so a shopper's whole ledger is the wrong thing for it to read. Nothing in either agent's instructions asks for one; `explain_belief` and `recall_for_purchase` already carry the evidence an agent needs to justify a recommendation. If a later task wants an agent to read ledger rows, it should widen the table as its own change, with its own fixture.

Both `tools.ts` files are now a factory — `createTools(layer)` and `createTools(landing)` — with the deployed `tools` export as that factory over the transport built from the environment. The deployed path is otherwise untouched: same ids, same descriptions, same schemas, same HTTP request. The factory is what makes the bindings testable offline.

The checks now exercise the bindings themselves rather than only the transport below them (130 checks for the reference agent, 125 for the second). Each reads the edge's access table from `internal/auth/surface.go` and the frozen names from `internal/api/contract/tools.go` and compares the offered tool set with the set admitted to `CallerAgent`; for every tool it reads `<tool>.request.schema.json` from `testdata/api`, compares required fields, field names and field types against the zod schema, then calls that tool's `execute` through the factory and compares the body the transport received with `<tool>.request.example.json`. A changed id, a renamed or retyped field, or an extra or missing tool on either side fails the check: `id: "record_feedback"` renamed to `record_feedbackx` and `pooling` retyped to a string were both tried, both failed, and both were reverted.

The retry rule is its own step in each agent's instructions, and step 1 now points at it instead of carrying "retry it once". A failing read may be called once more. `record_correction`, `record_reason` and `record_feedback` each append a row, so a write that errored or timed out is read back first — `recall_for_purchase` for a correction, `pending_questions` for a reason, `explain_belief` for feedback — reported if it landed, and resent only when the read shows it did not; when the read cannot show it, the agent asks the shopper. `set_pooling` states a choice instead of appending a row, so it is named safe to repeat. The checks pin that wording. Neither transport retries on its own — a stub-server check counts exactly one request per call — so a duplicated write could only come from the model, and the instructions are where it is prevented.

The deployed builds do not match the committed code until the orchestrator redeploys both. This task made no live call and spent nothing. The build and redeploy commands are the standing ones, run per agent from its own directory after the offline check is green:

```sh
cd agent/runtime && bun agent/offline-check.ts && ast push -V private -y && ast agent redeploy --id <deployment-id> --latest --var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY --wait
cd agent/second && bun agent/check.ts && ast push -V private -y && ast agent redeploy --id <deployment-id> --latest --var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY --wait
```

Three things the next agent should not have to rediscover:

- The check parses the Go access table and the contract's name constants. A reformatted table (a field beside `Callers`, an entry split across lines) makes the parse throw and the run fail with the counts it parsed, rather than passing with an empty set. Widen the expression then; keep the count assertion, because it is what turns a silent no-op into a failure.
- The tool set bound by `Layer`/`Landing` and the set offered to the model are two lists that must agree, and the factory keeps them one: `createTools` calls one layer method per binding. A frozen tool an agent may call needs both, and the check stays red until both exist.
- The retry wording is load-bearing and pinned sentence by sentence. Rewording that step without updating the check reds the agent's own offline check; rewording it without any check would let a `record_correction` that landed be written twice.


---

### T9.7: The demo, after the pivot ★
```yaml
requires:   [T9.2, T9.3d, T9.5, T9.6, T9.8]
fixture-ok: no
size:       M · frontier
owns:       demo/
status:     done
```
product.md's demo from step 0 to step 7, live, on the hosted service: the new shopper sized from the pool on day one; the three returns; the correction; the purchase and feedback; the dispute that leaves sizing alone; the ledger; the second agent honouring the correction; the merchant app's "why". The words about pooling follow the consent decision (T7.3). Budget: under $1.00. Rehearse step 0 first and check its cost.

**Done when:** a recorded run covers every step, with each step's trace or screen linked in the handoff log. `demo/reset.sh` restores the starting state. Spend is recorded.

**Notes (in-review).** `demo/` is refreshed for the pivot: `reset.sh` restores the demo line to 22 settled / 22 seeded / 0 live (it opts the shopper out of pooling to remove an earlier run's live fit rows and always opts back in, then asserts the seeded belief, the capture and the dispute, and the spend baseline); `run.sh` + `run.py` walk steps 0-7 against the hosted edge, both agent deployments and both apps; `README.md` labels every step live, existing data or fixture replay and maps it to its artifact; `RUN-6.md` is the recorded run with its evidence under `demo/evidence/run-6/`. Raw answers and full trace ids stay in `out/` (git-ignored, mode 0600); the committed evidence is redacted (short trace ids, no key, no account id).

Live, in one pass (step 7 resumed once after a transient platform 401): step 0 pool **Earned small**, 22 settled (22 seeded, 0 live), 12 too-small / 3 too-large / 4 fits-well, 2 kept, 19 fit shoppers; step 1 the three seeded returns (`ev_6d1a09/10/11`, all `fit_too_small`), belief `blf_8f2c41` "runs small, take size L", Venerable (status Out of true, a correction already in force at the start); step 2 the same live reads give the belief's value and the pool, but no live Channel3 candidate call exists (the reference agent has no product-search tool), so the candidate SKU `PSW8GeK` appears only in step 4's captured purchase; step 3 traces `ded28ebc`/`c94448ee`/`82d6512f`, the agent recalls, records `corr_000004` and then recommends M under it while the belief stays Out of true; step 4 the existing capture `ev_51c2d8` (`memento-shopper01-06`, SKU `PSW8GeK`, M), recorded in an earlier sandbox cycle, and trace `11d83a4f` records `fits_well`, settling the fit row (22 → 23 settled, 0 → 1 live); step 5 the seller belief `blf_1a5e09` Claimed with one not-arrived dispute row and the sizing belief untouched; step 6 live ledger reads plus screens of the deployed ledger page, which is a fixture replay (`demo/evidence/run-6/06-ledger-belief.png`, `06-ledger-table.png`); step 7 trace `9d4893e7` (second agent, fresh conversation) recalls first and recommends M citing `corr_000004` and the 23/22/1 pool, and the merchant proxy reads Earned small 23 settled (22 seeded, 1 live) in `07-merchant.png`.

Spend **$0.69** of the $1.00 budget (billing usage $4.95 → $5.64; credit $5.05 → $4.36). The recorded run's five turns are $0.2377 (`ded28ebc` $0.0337, `c94448ee` $0.0759, `82d6512f` $0.0658, `11d83a4f` $0.0239, `9d4893e7` $0.0385); the rehearsal step-0 turn, the eight turns across three failed attempts before the recorded pass and the two superseded step-7 turns come to $0.396, so the sixteen traces behind the task total $0.634 and the rest of the billing delta is the platform's compute.

Eight things the next agent should not have to rediscover:

- **Step 0's empty shopper is blocked, not faked.** The only agent key binds `shopper_01`, so `shopper_02` answers 401 before the pool is reached. The pool read is live; a day-one turn needs the operator to widen `MEMENTO_AGENT_KEYS`'s binding (or mint a second agent key). Every run records this as a gap.
- **No live purchase or dispute can be ingested.** The PayPal app's only webhook subscription points at an external test receiver, and the served layer's `cmd/memento` passes `fakes.NewRecordingPaymentSource` as its event source, so a verified delivery would have no event to read (the unsigned-delivery probe and the subscription facts are in `demo/evidence/run-6/04-purchase-blockers.json`). Step 4 uses the capture already in memory and writes the fit row through `record_feedback`; a new purchase needs a layer/deploy task (T8.7b).
- **The deployed ledger page is a fixture replay** (60 settled / 48 seeded in the screen) with no live transport. Step 6's live numbers are the API reads; a `web/` task (T4.8) wires the transport.
- **`record_feedback` is not idempotent per purchase in the shopper's belief**: each call appends the same evidence again, and the seeded arc's one copy plus this run's two feedback calls left `ev_51c2d8` three times on `blf_2b9d60`, lifting it Claimed → Building. The pool row is idempotent. Layer task (T8.7b).
- **A restart can reuse a correction id**: `restoreStanding` restores the journal's correction ids but not the mint sequence (`st.mint` in `internal/api/server/server.go` starts every prefix at 1), so the next `record_correction` minted `corr_000001` over the restored one during an early attempt. The run's own `corr_000004` is unaffected. Layer task (T8.7b).
- **`explain_belief` lists a correction twice**: the live answer for `blf_8f2c41` carries seven corrections for five ids (`corr_000001` and `corr_3b7e52` twice each, `demo/evidence/run-6/02-belief-explained.json`), because `restoreStanding` and `LoadSeed` both add the seeded correction to the correction order and a reused id is appended a second time; the read de-duplicates nothing. Layer task (T8.7b).
- **Step 2's Channel3 candidate is not live**: the reference agent has no product-search tool, so no Channel3 call exists in the run and the candidate SKU appears only in step 4's captured purchase. T9.9 owns the agent's tool surface, or a new task the orchestrator opens adds the search.
- **Recall-first is not every turn**: the reference agent skipped `recall_for_purchase` on the recorded feedback turn and on one correction attempt (it recalled on the ask and next-recommendation turns). T9.9 owns the prompt/retry discipline. Also, the seller belief never reaches recall and agent keys are refused on ledger reads, so no agent turn can mention the dispute (T9.9's surface alignment or a recall extension), and both apps render their own source comments as visible text while the belief page prints its standing-correction empty-state sentence twice (T4.8).

The pre-pivot T3.6 record (`RUN-1.md`, `RUN-2.md`, `drive.py`, `driver/`) is kept as history and is not used by the pivot run.

**Review (fresh eyes, T9.7).** Re-verified from the committed files, the platform's traces and free layer reads; no paid turn was re-run. `git show --stat 399cae9` is `demo/` plus this block. The five traced turns' costs match the evidence to six decimals on `ast agent trace`, and so do their tool spans (`ded28ebc`, `c94448ee`, `82d6512f` and `9d4893e7` recall first, `c94448ee` then records, `11d83a4f` records feedback without recalling — the recorded gap). The traces after the claim commit total $0.634 (thirteen reference and three second-agent turns), so the $0.69 billing delta carries about $0.06 of platform compute; the $4.95/$5.05 starting figures are the operator's reading and are not in a committed file. A live free read confirms the served layer still answers 23 settled / 22 seeded / 1 live through both the layer and the merchant proxy, `shopper_02` 401 on the agent key, a 403 for the agent key on `ledger_beliefs`, three copies of `ev_51c2d8` on `blf_2b9d60`, and `explain_belief` returning seven corrections for five ids. `reset.sh` removes only the demo shopper's own fit rows (`fitindex.Writer.Forget` deletes by that shopper's per-line hash, on the lines their session knows), so the seeded population, other lines and the append-only corrections are out of reach; it now always ends pooled, and refuses unless the pool, the belief, the capture and the dispute are back. Fixes landed in this review: a per-step label column in both records (step 1's refunds named seeded, step 4's purchase and step 5's dispute named existing data, step 2's missing Channel3 call labelled), the reset prose brought onto what the script does, the spend split replaced by the trace-measured figures, gap 5's "three feedback calls" corrected to one seeded copy plus two calls, gap 8's page claim replaced by the duplicate sentence actually visible in the screen, and two new gaps (the duplicate correction listing, the missing Channel3 call) with owners. Findings: (Low, fix before close, T9.7) `demo/evidence/run-6/summary.json`'s `belief_before_correction` names `blf_8f2c41` with `blf_2b9d60`'s "Building" status and a post-correction guidance, because it took `beliefs[0]` of a post-correction read; `run.py` now records the pre-turn belief and leaves the field null when a run is re-summarised, and the operator can rebuild the file with `python3 demo/run.py --summary-only` where `demo/out` lives. (Medium, rule 11, orchestrator) the owners this task proposes now exist: T8.5l (feedback idempotency), T8.5m (correction id reuse and the duplicate listing), T8.7b (live payment events), T9.15 (second shopper key), T9.17 (catalog search), T4.8 (live ledger) and T4.6 (belief page). Every gap names one. The gate is untouched by a `demo/`-only change and `./tools/check.sh` passes on the review branch (`npm ci` in `web/` and `merchant/` first).

---

### T9.11: Scrub the Astropods org id from the drivers
```yaml
requires:   [T9.7, T9.9]
fixture-ok: yes
size:       XS · mid
owns:       demo/drive.py, demo/run.py, agent/second/drive.py
status:     done
```
The P3 review found the Astropods org id committed in `demo/drive.py` and `agent/second/drive.py` as a `secret-tool` attribute. It is an identifier, not a credential, but the diary's own rule excludes account detail and the scrub rule lists user ids as placeholders. Replace it with a placeholder and read the org from the environment or the CLI's own config.

**Done when:** neither driver carries the org id; both read it from the environment or the CLI; a run still works; the gate passes.

**Notes (in-review).** The org id is gone from all three drivers (`demo/drive.py`, `demo/run.py`, `agent/second/drive.py`), which carried it in the `secret-tool` username. Each gained one `astro_org()` helper: `ASTRO_ORG` when set, otherwise the current profile's single org from the ast CLI's own config (`~/.ast/credentials.json`, `profiles[current_profile].org_tokens`), and a stop naming both sources when there is no file, no org, unreadable JSON, or more than one logged-in org. There is no default and no silent pick; the keyring lookup is now `default_<org>_access_token` with the org from that helper, and a keyring miss is a clear message rather than a `CalledProcessError`. `demo/drive.py` and `agent/second/drive.py` resolve it at import, before billing or any request; `demo/run.py` resolves it inside `astro_token()`, so `--summary-only` never reaches the keyring lookup; that path still shells `ast billing get`, as it always has (see the review finding below). Both drivers' `main()` gained the `if __name__ == "__main__":` guard `demo/run.py` already had, which is what lets the resolution be exercised without a call.

Proof, no live call and $0: with the real CLI config each driver resolves the org and reads a non-empty token from the keyring (no network); the same three fail against an empty `HOME` with `drive: no org: set ASTRO_ORG, or log in with the ast CLI (~/.ast/credentials.json: No such file or directory)` and exit 1 before billing or any request; a two-org config fails naming `ASTRO_ORG`, a malformed config fails the same way, `ASTRO_ORG` alone resolves with no config present, and an `ASTRO_ORG` with no keyring entry fails with the keyring message. `grep` for the id over the three files finds nothing.

Three things the next agent should not have to rediscover:

- The CLI's config key form is `org_…`, and the keyring username is `default_<that key>_access_token`. `ASTRO_ORG` takes that same form; a bare id builds a username no keyring entry matches and surfaces as the keyring message, not as a bad org.
- `bench/pilot/cmd/drive/main.go` and the three `bench/runs` drivers (T55, T58, T59) read the same config file, but take an arbitrary key from the map. These helpers sort the keys and refuse more than one org, so a second logged-in org is a loud stop here rather than a coin toss.
- `astro_org` is repeated in the three scripts on purpose: they are standalone scripts with no shared module, and each `owns` line is three files. A fourth caller should copy the helper; a shared module is a new task, not a quiet import.

The operator-facing documentation is each script's docstring plus the comment over the constant, because `demo/README.md` is outside this task's `owns`; naming `ASTRO_ORG` in that file belongs to the proposed `demo/` task in the review findings below, not to this one.

Finding (Low), owner: a proposed task for the orchestrator to number (`owns: tools/check.sh`). The gate has no step over the Python the demo runs, so a syntax error in these three drivers reaches the run rather than the gate; `python3 -m py_compile` over `demo/` and `agent/*/drive.py` would close it, on the `node`/`python3` the gate already requires.

**Review (fresh eyes, T9.11).** Re-verified from the committed files, no live call, $0. `git show --stat fda5ae4` is the three drivers plus this block. No tracked file carries the org id or a `default_org_…` username any more; each driver builds `default_<org>_access_token` from `ASTRO_ORG` or the current profile's single key in `~/.ast/credentials.json`, and against the real config `demo/drive.py` and `agent/second/drive.py` both resolve that org and read a non-empty token from the keyring with no network (`demo/run.py` builds the same username inside `astro_token()`, on the same code). Against scratch `HOME`s with `ASTRO_ORG` unset, both drivers stop with exit 1 before billing or any request: no config → `set ASTRO_ORG, or log in with the ast CLI (~/.ast/credentials.json: No such file or directory)`; two orgs → `holds 2; set ASTRO_ORG to name one`; malformed JSON → `is not JSON`; an empty profile, `{}`, or a `current_profile` that names no profile → `holds none`; an `ASTRO_ORG` with no keyring entry → the keyring message. `demo/run.py` resolves lazily: importing it with no config succeeds, and only the helper stops. `python3 -m py_compile` over the three is clean and `./tools/check.sh` passes on this branch. Limit: no live messaging turn was made, so "a run still works" rests on the resolved username matching the live keyring entry, not on a send.

Findings, each with an owner:

- (Low) The id is gone from HEAD, not from the history: `git log --all -S` on the old `secret-tool` username finds it in four earlier commits, and this diary is public. Removing it there is a rewrite, which is the user's call, not a worker's; owner: the user, decision recorded by the orchestrator.
- (Low) `--summary-only` is not call-free: `main()` runs `billing()` on every path, after the summary branch, so that path still makes the live `ast billing get` request that needs the CLI's own login, while `demo/README.md` says "no live call at all". This task's note is corrected above; the org resolution is not on that path, so the scrub itself stands. Owner: a proposed `demo/` task for the orchestrator to number.
- (Low) Deployment ids stay baked in `demo/drive.py`, `demo/run.py`'s reference default and `demo/driver/main.go` while the org id moves out; a deployment is an account-scoped identifier like the org, so the same proposed `demo/` task should decide whether it moves out too.
- (Low) The same config is read by four `bench/` drivers that take an arbitrary key from `org_tokens` (`bench/pilot/cmd/drive/main.go`, `bench/runs/cmd/t55`, `t58`, `t59`), so a second logged-in org makes their choice a coin toss rather than the stop these three drivers now give. Owner: a proposed `bench/` task for the orchestrator to number.

---

### T9.12: Guard the public shopper page
```yaml
requires:   [T3.8]
fixture-ok: yes
size:       S · mid
owns:       agent/frontend/, tools/check.sh
status:     done
```
The P3 review found T3.8's live result has no committed transcript, nothing tests `validateMessages` or the 30-turn cap, and no gate step type-checks `agent/frontend/` (the gate checks `web/` and `merchant/` only), so a future edit there is unguarded. Add the transcript (redacted), tests for message validation and the cap, and a frontend typecheck step to the gate.

**Done when:** the transcript is committed with no key or account detail; both behaviours have tests that fail when broken; the gate type-checks `agent/frontend/`; the gate passes.

**Notes (worker).** The relay's two guards moved out of `server/index.ts` into `server/relay.ts`, which imports nothing: `validateMessages` (1..20 messages, `user`|`assistant`, 1..1000 characters of trimmed text, ending on the shopper's turn), `createTurnBudget`/`MAX_TURNS` (30), and `decideChatTurn`, which fixes the order — a malformed body is refused before the budget is touched, a spent budget is refused before any model call, and only an accepted turn consumes one. `server/index.ts` keeps the HTTP plumbing and its 400/429/502 bodies unchanged. `server/relay.test.ts` is 15 `node:test` cases over those three, so it needs no bun, no install, no gateway and no model call: the shape refusals (each with its message), trimming, the 20-message boundary, assistant turns wrapped as text parts, exactly 30 turns allowed and the 31st refused with `TURNS_EXHAUSTED_ERROR`, refusals never incrementing, a malformed body still refused as malformed once the budget is spent (the order itself), and budgets independent per container.

The transcript is `agent/frontend/testdata/t38-public-page.txt`. The run's raw output is **not recoverable** — its drive script stayed outside the repo, as T3.7's did, so there was no capture to redact. The file says so on its face and reassembles what the tree still carries instead: the anonymous 200 on the frontend host, `/health` 200 and the unknown-path 404 from the container, the chat host's 302 to the platform SSO, the four turn costs ($0.0174 proxy, $0.0169 ask, $0.0277 correction, $0.0181 yes), the proof turn `a89f4f49` as the page prints it, billing $1.39 → $1.52 (+$0.13), and the standing configuration. Every line carries its provenance (a T3.8 note, the page's own text, this package's spec, or `server/relay.ts` as it reads). Placeholders for every host; no key, no account id, no IP.

`tools/check.sh` gains two steps after the two Svelte frontends. First, `node --test agent/frontend/server/relay.test.ts`, always, on the node the gate already requires. Second, a typecheck: `bun run typecheck` (tsc --noEmit over `server/`) where `bun` and `agent/frontend/node_modules` both exist, and otherwise `web/node_modules/.bin/tsc --noEmit --strict` over `server/relay.ts` with the limit printed on the `--` line. The fallback is the closest equivalent that adds no dependency: the package tracks `bun.lock` only, so `npm ci` cannot provide its types, and the TypeScript it borrows is the one `web/` already installs (the gate fails earlier if that install is missing). **Limit, stated:** CI takes the fallback branch, because the `ubuntu-latest` image ships no `bun` and `ci.yml` installs neither bun nor this package's modules; the full-package typecheck therefore runs only on a workstation that has run `bun install` in `agent/frontend/`.

Proof. Mutation-checked: removing the last-message-must-be-the-shopper check fails 3 tests; letting the budget take one turn too many (`used > max`) fails 4; taking a turn before validating fails 2. Each reverted, `diff` clean. The fallback typecheck is not vacuous: a `let used: string` in `relay.ts` exits 2 with four TS errors. `./tools/check.sh` is green twice — with bun and the package's modules present (so `bun run typecheck` runs), and with bun off `PATH` and `node_modules` moved aside (so the fallback branch runs and prints its limit). No live call, no spend.

Three things the next agent should not have to rediscover:

- `node`'s ESM resolver does not map `./relay.js` to `relay.ts`, so the test imports `./relay.ts` and `tsconfig.json` gains `allowImportingTsExtensions` (legal because the typecheck script is `--noEmit`). The server files keep the `./x.js` style that bun and `tsc` resolve.
- Keep `server/relay.ts` import-free. The moment it imports anything, `node --test` needs that dependency installed and the fallback typecheck loses its subject; the guards would then only run where bun and this package's modules exist, which CI is not.
- Finding (Low), owner: T9.13 *Typecheck the shopper page in CI* (`requires: [T9.12]`, `owns: .github/workflows/ci.yml`). CI should `setup-bun` and run `bun install --frozen-lockfile` in `agent/frontend/` so the full-package typecheck, not just `relay.ts`, is guarded on every push; `ci.yml` is outside this task's `owns`, so the limit stands until that task lands. The task's other files need no change.
- Finding (Low), owner: a runtime test of the relay's status mapping, proposed for the orchestrator to number and land after T9.13, because the test needs bun on the runner. `decideChatTurn`'s kinds are tested, but the three lines in `server/index.ts` that turn a kind into 400/429 (and a failed model turn into 502) are not, so a future edit could answer 500 for an exhausted budget and the gate would stay green. It is pre-existing and unchanged here, and `server/index.ts` cannot be imported under `node --test` (it starts `Bun.serve` on the injected `PORT` and registers `serve()`), which is why this task did not close it.

**Review (fresh eyes).** Re-verified from the committed files on a branch cut at the task tip, no live call, no key. Scope is `agent/frontend/`, `tools/check.sh` and this task's phase lines, nothing else. The refactor is behaviour-preserving: `decideChatTurn` returns the same six refusal strings as the old inline validator and `TURNS_EXHAUSTED_ERROR` is byte-identical to the old 429 body, so the 400, 429 and 502 bodies are unchanged, and the old `turnsServed >= MAX_TURNS` check is the new `used >= max`. Mutation-checked again from the committed suite: the last-message check deleted fails 3 cases, `used > max` fails 4, and charging the turn before validating fails 2 — the last of those is the one case added in review, because the order was only pinned before the budget was spent and a charge-first relay therefore failed just one test. The fallback typecheck is not vacuous (`let used: string` exits 2 with four TS errors) and the gate takes the `bun run typecheck` branch where bun and this package's modules exist; both branches ran green. The transcript's figures match T3.8's notes line for line (four turn costs, proof turn `a89f4f49`, billing $1.39 → $1.52), it carries no shell-prompt line, no host, IP or account detail, and it states on its face that it is a reassembly rather than a capture: the Done-when asks that a transcript be committed with no key or account detail, and that is what it is, so the task is met rather than blocked. What the file cannot do is stand as the byte-exact output of the run; that output is gone, and nothing in this task's `owns` could bring it back.

---

### T9.13: Typecheck the shopper page in CI
```yaml
requires:   [T9.12]
fixture-ok: yes
size:       XS · mid
owns:       .github/workflows/ci.yml
status:     done
```
T9.12 wired the frontend guard into the gate, but the full-package typecheck needs `bun` and the package's modules, which the CI runner does not have: CI takes the reduced fallback that checks only `server/relay.ts`. Add `setup-bun` and a frozen `bun install` for `agent/frontend/` to the workflow so CI runs the same full typecheck as a workstation.

**Done when:** CI installs bun and the package's frozen dependencies and the gate's frontend step runs the full typecheck there; a run shows it green; no other gate step changes.

**Notes (worker).** `.github/workflows/ci.yml` gains three steps before the gate and nothing else moves: `oven-sh/setup-bun@v2` with `bun-version: '1.3.14'` (the version this task proved the gate on, quoted like the existing `go-version` and `node-version` pins and tag-pinned like the other actions), `actions/cache@v4` over `~/.bun/install/cache` keyed `${{ runner.os }}-bun-${{ hashFiles('agent/frontend/bun.lock') }}`, and `bun install --frozen-lockfile` with `working-directory: agent/frontend`. setup-bun caches its downloaded executable by default (`no-cache` defaults to false); the explicit cache covers the package downloads the install fetches, and its key moves with the lockfile. `tools/check.sh` is unchanged: the frontend step still runs `bun run typecheck` where bun and `agent/frontend/node_modules` exist and otherwise prints its limit and checks `server/relay.ts` alone, so a machine without bun behaves exactly as before. The workflow parses as YAML with all nine steps reading back in order, and `go run ./tools/pincheck` still agrees across 9 declarations, because that check reads only `go-version:` and `node-version:`.

Proof on the workstation: `bun --version` is 1.3.14; `bun install --frozen-lockfile` in `agent/frontend/` installs 282 packages and leaves `bun.lock` untouched; `bun run typecheck` (the package's `tsc --noEmit` over `server/`) exits 0; `./tools/check.sh` is green and its frontend step prints `-- agent/frontend typecheck (bun run typecheck)` then `$ tsc --noEmit`, the full-package branch. `npm ci` in `web/` and `merchant/` supplied the gate's other dependencies.

**Limit, stated.** GitHub Actions cannot run in this worktree, so no CI run backs this change; what is proved is the exact command sequence the job now runs, locally. The "a run shows it green" clause stays unproven until the workflow runs on a push. Finding (Low), owner: a future task the orchestrator numbers, extending `tools/pincheck` to bun, because bun's pin is hand-kept and nothing compares it to a second source; `package.json` sets no `packageManager` or `engines.bun`, so leaving `bun-version` off would float to `latest` rather than pin anything. T9.14 can now run its mapping test under the bun CI installs.

**Review (fresh eyes).** Re-verified from the committed files on a branch cut at the task tip, no key, $0. `3d92d26..6009942` is `.github/workflows/ci.yml` and this block, nothing else, and `tools/check.sh` is untouched — which is what "no other gate step changes" means here.

The workflow parses as YAML and reads back as nine steps. `oven-sh/setup-bun@v2` (step 4) carries `bun-version: '1.3.14'`, quoted like `go-version` and `node-version` and tag-pinned like every other action; v2's own `action.yml` declares that input, and its `no-cache` defaults false, so the executable cache the note leans on is real. `actions/cache@v4` (step 5) covers `~/.bun/install/cache`, which is the path `bun pm cache` prints, keyed on `hashFiles('agent/frontend/bun.lock')`, the lockfile the package tracks. `bun install --frozen-lockfile` runs in `agent/frontend` at step 8, ahead of the gate at step 9. Go's and Node's pins are unchanged from `main`, and `go run ./tools/pincheck` still prints `go 1.27 and node 26 agree across 9 declarations`.

The install guards something: in a scratch copy with `zod` bumped to `^4.0.0`, `bun install --frozen-lockfile` exits 1 with `lockfile had changes, but lockfile is frozen` and creates no `node_modules`, so a `package.json` edit that skips the lockfile fails CI at the install instead of passing a stale tree.

The job's command sequence, run here: `bun --version` is 1.3.14, the pin; `bun install --frozen-lockfile` in `agent/frontend/` installs 282 packages and leaves `bun.lock` unchanged (same sha256 before and after); `bun run typecheck` exits 0; `./tools/check.sh` is green and prints `-- agent/frontend typecheck (bun run typecheck)` then `$ tsc --noEmit`. With bun off `PATH` and `agent/frontend/node_modules` moved aside — the runner as `main` leaves it — the same gate is green and prints `-- agent/frontend typecheck (bun or agent/frontend/node_modules absent: server/relay.ts only)`, so the fallback still holds and this commit is the one that moves CI onto the full branch. Bun 1.3.14 is fetchable: npm's `bun` carries that version and GitHub's `bun-v1.3.14` release answers, so setup-bun has something to install.

Limits. No GitHub Actions run exists, so "a run shows it green" stays open; nothing inside a worktree can produce it, so it goes to the orchestrator with the push the way T9.3d's and T9.6b's post-push proofs did. The workstation's bun equals the pin, so what is proved is the command sequence, not the runner image.

Two findings, both Low, both owned by **T9.15 (proposed, owns `tools/pincheck/`, `tools/check.sh`)**; the note's "a future task the orchestrator numbers" is a placeholder, and rule 11 wants a task.

- (Low) `tools/pincheck` reads `go-version:` and `node-version:` only, so this commit adds CI's first toolchain pin with no second source. `ciPins` matches those two keys by name and ignores the rest; `agent/frontend/package.json` states no `packageManager` or `engines.bun`, so an omitted `bun-version` resolves `latest`; and the four Dockerfiles that name `oven/bun:` (seven stages, in `agent/frontend`, `agent/runtime`, `agent/second` and `bench/pilot`) carry a rolling major tag, so the image that serves the page builds with the newest 1.x while CI type-checks on 1.3.14. The extension is well defined and cheap: read `oven/bun:` `FROM` tags and CI's `bun-version:`, compare by major the way Node is compared, and fail when a source stops stating one. `:1` against 1.3.14 agrees at major 1, so such a check lands green today.
- (Low) `tools/check.sh`'s frontend comment still says the package "tracks bun.lock, not package-lock.json, so `npm ci` cannot stand in for it, and CI installs neither", and its fallback line still announces "bun or agent/frontend/node_modules absent". CI now supplies both. The behaviour is right — the fallback is still what a workstation without bun takes — so only that prose is false, and it sits outside this task's `owns`.

Approve: the workflow does what the task asks, scope is clean, and both Done-when clauses a worktree can settle are settled.

---

### T9.14: Test the relay's status mapping
```yaml
requires:   [T9.13]
fixture-ok: yes
size:       XS · mid
owns:       agent/frontend/
status:     done
```
`agent/frontend/server/index.ts` maps kinds to statuses (400 for an invalid body, 429 for an exhausted budget, 502 for a relay failure), and no test pins that mapping: a future edit could answer 500 for an exhausted budget with the gate green. The file starts `Bun.serve` and cannot be imported under `node --test`, so the test needs the bun runner T9.13 adds to CI.

**Done when:** a test runs the mapping under bun and fails if any kind answers a different status; the gate runs it where bun exists; the gate passes.

**Notes (worker).** `agent/frontend/server/status.ts` holds the mapping and imports nothing, the way `relay.ts` does: `STATUS_INVALID` (400), `STATUS_EXHAUSTED` (429) and `STATUS_RELAY_FAILURE` (502), with `malformedJsonReply()`, `refusalReply(kind)` and `relayFailureReply(turnId)` returning the status and body `index.ts` serialises. `index.ts` builds every refusal and failure answer through them — the unparseable body is the fourth such answer, and it moved out of the handler for the same reason — so `json(200, …)` is the only literal status it hands `json()`; the unknown-path fallback's `new Response("not found", { status: 404 })` is not a `/api/chat` answer and stays as it was.

`server/relay.test.ts` gains a `the /api/chat status mapping` block of seven cases (22 in the file now): 400 for a body that is not JSON; 400 for a body the relay refuses, echoing the validator's own text; 429 for a spent budget with `TURNS_EXHAUSTED_ERROR`; 502 with `{error, turn_id}` and no other key; the three statuses pinned as 400/429/502 and distinct; the mapping driven by `decideChatTurn`'s own decisions, so the join between the two modules is exercised rather than the constants alone; and a source guard over `index.ts`. That guard reads the file as text (the module starts `Bun.serve` on the injected `PORT`, so importing it is not an option), fails if any `json(` call in it carries a literal status other than 200, and requires each of the three constructors to be called.

Why the cases sit in `relay.test.ts` rather than a new `status.test.ts`: the gate's frontend test step names that file and only that file (`tools/check.sh` is T9.19's), and extracting the mapping made it import-free, so it no longer needs bun to be testable. Keeping the cases in the file the gate already runs means the mapping is guarded on every gate run, CI included, with no gate edit; the same file is green under bun as well.

Proof. Mutation-checked, each reverted and `git diff` read back: `STATUS_EXHAUSTED = 500` fails three cases (the 429 body text, the pinned statuses, the relay-decision case), and the gate's own command `node --test agent/frontend/server/relay.test.ts` goes red the same way; inlining `json(502, failure.body)` in `index.ts` fails the source guard with `index.ts answers a refusal with a literal status; take it from status.ts`. `bun test server/relay.test.ts` is green (22 pass, bun 1.3.14), `bun run typecheck` is green over `server/` after `bun install --frozen-lockfile` (282 packages, `bun.lock` untouched), and `node --test agent/frontend/server/relay.test.ts` is green with bun off `PATH`, so the mapping runs where bun is absent too. `./tools/check.sh` is green and its frontend step takes the `bun run typecheck` branch. `npm ci` in `web/` and `merchant/` supplied the rest. No live call, no key, no spend.

Limit, stated: the task asked that the test run under bun because the mapping lived behind `Bun.serve`; extraction removed that reason, so the gate runs the cases on the node it already requires, everywhere, and bun runs the same file when it is there. That is stronger than "where bun exists" on the gate side and weaker nowhere: no runner the gate has is left out.

Finding (Low), owner **T9.19** (`owns: tools/check.sh`): the gate's fallback typecheck — the branch a workstation without bun or `agent/frontend/node_modules` takes — still names `server/relay.ts` alone, so `server/status.ts` is untypechecked on that branch even though the tests above cover it. The line becomes `… --moduleResolution Bundler server/relay.ts server/status.ts`. CI takes the full-package branch and is unaffected. If the orchestrator also wants bun to be the gate's runner for this file rather than node, the addition inside that same bun branch is `(cd agent/frontend && bun test server/relay.test.ts)`.

Three things the next agent should not have to rediscover:

- `status.ts` must stay import-free. The gate's node step resolves `./status.ts` only because the import carries the extension; `./status.js` is bun's and `tsc`'s spelling, not node's, so importing through it would break the gate's test step the way `relay.ts`'s own note warns.
- The source guard treats 200 as the only literal status `json()` may be called with in `index.ts`. A future endpoint that answers another success status through `json()` needs the guard widened on purpose; that is the intended friction, not an oversight.
- `refusalReply` takes its text from the decision, so the cap's wording stays single-sourced in `relay.ts`'s `TURNS_EXHAUSTED_ERROR`, and the 400/429 choice is the only thing this module decides.

**Review (fresh eyes).** Re-verified from the committed files on a branch cut at the task tip, no live call, no key, $0. Scope is `agent/frontend/` and these task lines: `8599e6e..c1620e6` is `server/index.ts`, `server/status.ts`, `server/relay.test.ts` and the T9.14 block, and `tools/check.sh` is untouched. `status.ts` imports nothing; `index.ts` hands `json()` a variable on every refusal and failure path, and a literal only on `/health` and the accepted turn. Each kind's status and body read back from the tests: 400 for a body that is not JSON and for a body the relay refuses (no `turn_id` on either), 429 for a spent budget in `TURNS_EXHAUSTED_ERROR`'s words, 502 carrying exactly `{error, turn_id}`, and the three statuses distinct. The source guard bites both ways: `json(502, failure.body)` in `index.ts` fails it with *index.ts answers a refusal with a literal status; take it from status.ts*, and routing the malformed body through `refusalReply` instead of `malformedJsonReply` fails the constructor clause with *malformedJsonReply must shape index.ts's answer*.

Mutations from the committed suite, each reverted with `git status` clean: `STATUS_EXHAUSTED = 500` fails the three cases the note names, under node and bun alike, and swapping `refusalReply`'s two statuses fails three more. `node --test agent/frontend/server/relay.test.ts` is 22 pass, and 22 pass with bun off `PATH`; `bun test server/relay.test.ts` is 22 pass on bun 1.3.14. `bun install --frozen-lockfile` installs 282 packages and leaves `bun.lock` byte-identical; `bun run typecheck` is green, and it does compile `status.ts` — a `: string` annotation added to `STATUS_RELAY_FAILURE` exits 2 with two TS errors at `server/status.ts`. Run locally as the container runs it (`PORT=8099`, `SKIP_SERVE=1`, the gateway's base URL pointed at a closed local port, so no key, no model call and no outbound traffic): `/health` 200, a body that is not JSON 400 `{"error":"body must be JSON"}`, `{"messages":[]}` 400 with the validator's text, a well-formed turn 502 `{error, turn_id}` thirty times over, the thirty-first 429 with `TURNS_EXHAUSTED_ERROR`, and an unknown path 404 — the four chat answers read off the wire, and `./status.js` resolving at runtime. The gate ran green twice (`GATE EXIT=0` both times, before and after this review's wording fixes) and its frontend step printed `-- agent/frontend typecheck (bun run typecheck)`; its node step runs the same `node --test` command, so the mapping is guarded everywhere the gate runs, bun or not, which is what "the gate runs it where bun exists" reduces to once the `Bun.serve` import is gone. Only the accepted turn's 200 is outside `status.ts`, and the comments now say so: this review fixed `index.ts`'s "every `/api/chat` answer's status and body come from status.ts", false for that turn, and the note's "`json(200, …)` is the only status literal left in that file", false for the fallback's `{ status: 404 }`. No test or behaviour changed with either fix.

Limit: the guard reads `json(` call sites, so an answer written as `new Response(..., { status: 500 })` inside the chat handler would pass it; the direct cases still pin the helpers, so only a bypass of `json()` slips through, and the unknown-path 404 is not a chat outcome. The T9.19 finding above is owned: T9.19's `owns` is `tools/check.sh`, it is `not-started`, and its suggested line compiles here (`tsc --noEmit … server/relay.ts server/status.ts` exits 0); T9.19's own block does not yet repeat it, so the dispatch should carry the finding rather than rely on this block being read.

---

### T9.15: A second shopper for day one
```yaml
requires:   [T9.8]
fixture-ok: no
size:       XS · mid
owns:       none (Render secrets and the keyring)
status:     not-started
```
The T9.7 run found the agent key binds only `shopper_01`, so a new shopper's day-one recall answered 401 before the pool: the demo's step 0 has no live path. Mint an agent key bound to a second demo shopper (or widen the existing entry's shopper list), set it on the layer's secret, redeploy, and prove the new shopper's first recall reads the pool.

**Done when:** a second demo shopper's recall through the edge returns the pool (never 401); the key is in the keyring and the Render secret, never a file; the proof is recorded.

---

### T9.16: Check the bun pin, and fix the gate's CI comment
```yaml
requires:   [T9.13]
fixture-ok: yes
size:       XS · mid
owns:       tools/pincheck/, tools/check.sh
status:     done
```
T9.13 made CI's `bun-version: '1.3.14'` the repository's first toolchain pin with no second source: `tools/pincheck` compares Go and Node only, and the `oven/bun:1` image tags float past it. Extend the pin check to read the Dockerfiles' `oven/bun:` build tags and CI's `bun-version:` and compare them by major, as Node is compared. Also correct `tools/check.sh`'s comment and fallback text, which still say CI installs neither bun nor the package's modules.

**Done when:** the pin check covers bun and fails on a major disagreement or a missing pin; the gate's prose matches what CI does; the table test proves the new failure mode; the gate passes.

**Notes (worker).** `tools/pincheck` compares a third toolchain. `kindBun` reads the `oven/bun:` `FROM` line of every Dockerfile in the tree and CI's `bun-version:`, and compares them by major, as Node is. The Dockerfiles are discovered by a walk (`.git` and `node_modules` skipped; a file named `Dockerfile` or `Dockerfile.*`), not listed, so a file that gains a bun stage is read without an edit here. Both stages of a two-stage file are pins, so one that drifts its builder or its slim runtime is named at its line. The check fails when no Dockerfile states a bun base image: CI would then be the only source, and the comparison this task exists for would be CI against itself. Go and Node are untouched: the two deploy images are still held to a Go and a Node stage, `ciPins` now requires all three keys, and the majority, tie and exact-Go-patch rules are the same code path. The tree reads 17 declarations, `go 1.27, node 26 and bun 1 agree`.

`tools/check.sh` says what CI does now: the frontend comment names `setup-bun` at the version `tools/pincheck` compares against the images plus `bun install --frozen-lockfile` in `agent/frontend/`, so CI checks the whole package; the fallback is what a workstation without bun or the modules takes, and its `--` line says that rather than implying CI takes it. The gate's pin comment names bun's two sources beside Go's and Node's.

Proof. `go test ./tools/pincheck/` is green over 23 table cases, four of them bun: a drift in the shopper image, a drift in the runtime image's builder stage, a drift in CI, and a Dockerfile added with a bun stage (the discovery clause); plus CI losing `bun-version:`, a tag of `oven/bun:latest`, a bun image named bare or by digest only (the review's two cases), and a tree whose Dockerfiles state no bun image. Mutation-checked against the real tree, each reverted with `git checkout` and `git status` clean afterwards: `agent/runtime/Dockerfile`'s builder moved to `oven/bun:2` fails naming `agent/runtime/Dockerfile's bun build stage (line 2) says 2, but agent/frontend/Dockerfile's bun build stage (line 9) says 1`; `ci.yml`'s `bun-version: '2.0.0'` fails naming the file and both versions; deleting that line fails with `no bun-version: line`; a probe `Dockerfile` added under a new directory and removed after fails naming it. `./tools/check.sh` is green both ways: first with `agent/frontend/node_modules` absent, where the frontend step prints its new fallback line, then after `bun install --frozen-lockfile` (bun 1.3.14, 282 packages, `bun.lock` untouched), where it runs `bun run typecheck`. `npm ci` in `web/` and `merchant/` supplied the other dependencies.

Limit, stated: the comparison is by major, as the task specifies, so the images' `oven/bun:1` and CI's 1.3.14 agree and a Dockerfile that moved to another 1.x patch would stay green. That asymmetry is the same one the table records for Go, and tightening it would need the images to name a patch. No GitHub Actions run backs this; the workflow's steps are unchanged and pushing is the orchestrator's.

Finding (Low), owner: a future task the orchestrator numbers, `owns: deploy/README.md`. That section's prose still lists what the gate reads as "both images' `FROM` lines, CI's `go-version` and `node-version`, `go.mod` and these rows", and its table has no Bun row, so a reader is not told bun is checked or where its pins live. `deploy/README.md` is outside this task's `owns`.

**Review (fresh eyes).** Re-verified from the committed files on a branch cut at the task tip, no live call and no key, $0. `3e94b6c` is this block, `tools/check.sh` and `tools/pincheck/`'s two files, and nothing else. The tree's seven `oven/bun:` stages sit in four Dockerfiles (`agent/frontend` line 9; `agent/runtime`, `agent/second` and `bench/pilot/agent` at lines 2 and 21), and `go run ./tools/pincheck` prints `go 1.27, node 26 and bun 1 agree across 17 declarations`: those seven, both deploy images' Go and Node stages, CI's three keys, `go.mod` and the table's two rows.

Mutation-checked against the real tree, each reverted to a clean `git status`: `agent/runtime/Dockerfile`'s builder at `oven/bun:2` fails with `agent/runtime/Dockerfile's bun build stage (line 2) says 2, but agent/frontend/Dockerfile's bun build stage (line 9) says 1`; `ci.yml`'s `bun-version: '2.0.0'` fails with `.github/workflows/ci.yml's setup-bun (line 33) says 2.0.0, but agent/frontend/Dockerfile's bun build stage (line 9) says 1`; a `tmp/probe/Dockerfile` added with `oven/bun:2` in both of its stages is discovered with no edit to the check and named at lines 1 and 4. A scratch root holding only the non-Dockerfile sources fails with `no Dockerfile states a base image for oven/bun, so CI's bun pin has no second source`, and that root plus one `FROM oven/bun:1` file passes over 11 declarations. `./tools/check.sh` is green both ways: with `agent/frontend/node_modules` absent its step prints `-- agent/frontend typecheck (no bun here, or agent/frontend/node_modules absent: server/relay.ts only)` and the run ends `OK all checks passed`; after `bun install --frozen-lockfile` (bun 1.3.14, 282 packages, `bun.lock`'s sha256 unchanged) the same step prints `-- agent/frontend typecheck (bun run typecheck)`. `npm ci` in `web/` and `merchant/` supplied the rest. The gate's prose matches `ci.yml` step for step: `setup-bun` at 1.3.14 (line 33), both `npm ci` installs, `bun install --frozen-lockfile` in `agent/frontend` (line 49), then the gate; the fallback branch is the one a workstation without bun takes, which is what its `--` line now says.

Fixes landed in this review: a bun base image named with no tag was skipped in silence, so `agent/frontend/Dockerfile`'s `FROM oven/bun:1` cut to `FROM oven/bun` read `16 declarations` and passed — the one way a file could drop its bun pin without the check saying so, against this file's own "a source that has lost one is a failure" and the task's missing-pin clause. `bunDockerPins` now fails on a bare `oven/bun` or an `oven/bun@sha256:` digest with `<file> line <n>: <ref> states no tag, so it names no version to compare`, and two table cases prove each, which is where 23 comes from (the worker's 21 plus these two). Proved on the real tree: the cut line fails at line 9, the restored file is green over 17 declarations.

Findings, with owners.

- (Low) `readmePins` reads the Go and Node rows only, so the Bun row T9.18 adds to `deploy/README.md` would be a pin nothing compares: a row moved to 2 would leave the gate green. Closing it either makes that row a required source, which is the rule the other sources follow, or records why it is not. Owner **T9.18**, whose `owns: deploy/README.md` then needs `tools/pincheck/` added by the orchestrator; if the row is to stay unread, a numbered follow-on task owns the reason.
- (Low) the finding above names "a future task the orchestrator numbers, `owns: deploy/README.md`"; that task is **T9.18** on `main` (`8e3986a`: `requires: [T9.16]`, `owns: deploy/README.md`, `not-started`), and its brief matches what the finding describes. T9.18 is not in this branch's copy of this file, because the branch was cut at `227c67f`, so the placeholder stays here and the orchestrator's landing resolves it rather than this review writing a task this tree does not carry.

Limits. Discovery is a case-sensitive match on a file named `Dockerfile` or `Dockerfile.*`, skipping `.git` and `node_modules`, so a lowercase `dockerfile` would go unread; no such file exists. The comparison is by major, as the task specifies, so a 1.x patch drift between an image tag and CI stays green, the worker's stated limit re-checked. A `FROM golang` or `FROM node` that lost its tag still fails as "no base image for go/node" rather than at its line: a misleading message on a case that does fail, left alone here. Go's and Node's fourteen cases read the same path and the same messages as before.

Approve: scope is clean, both clauses a worktree can settle are settled, and the gate passes.

---

### T9.17: The reference agent searches the catalog
```yaml
requires:   [T9.8]
fixture-ok: yes
size:       S · mid
owns:       agent/runtime/
status:     in-review
```
The T9.7 run found product.md's demo step 2 has no live path: the reference agent exposes no product-search tool, so the Channel3 candidate never appears in a turn and the SKU exists only in step 4's capture. Add a catalog search tool (backed by `agent/catalog`, which already replays the Channel3 recordings) to the reference agent, bind it to the live layer's edge as the other tools are, and prove one live turn that names a real candidate line and SKU before recall.

**Done when:** a live reference-agent turn searches the catalog and names the matched candidate (line, size, SKU) before its recommendation; the tool's output carries no field Channel3 did not return; the offline checks cover it; the gate passes.

**Notes (in-review).** The reference agent now offers a ninth tool, `search_catalog`, and the demo's step 2 has a path in the code. Two facts shaped the build. A Bun deployment cannot import `agent/catalog` (Go), so the rule is ported to `agent/runtime/agent/catalog.ts`: same brand, same category, equal category-specific fit-label set, never the line name alone, with `exclude_skus` skipping the products behind the belief. The port is checked against the recordings `agent/catalog`'s tests use and against the same negative controls, so the two implementations are held to one set of fixtures. And the tool is agent-local, not an edge tool: the layer holds no product shelf and Channel3 appears only behind this agent, so `internal/auth/surface.go` neither admits nor refuses `search_catalog`. The check now separates the two sets: the eight layer tools this deployment offers still equal the table's `CallerAgent` set exactly, the catalog tool is named as the one local exception, and the layer binding still holds one method per admitted tool.

The answer is Channel3's own product records, unchanged: the search filters the recording's products by the cut and returns `{products}`, with no size note, no fulfilment label, no shared-signal list and no match evidence added. That is the Done-when's field clause held literally, and the check enforces it twice: every returned record's field set equals the recorded record's field set, and a list of plausible invented fields (`size_note`, `fulfilment`, `shared`, `available_sizes`, `match`, `size_chart`) must be absent. The two marks `agent/catalog` carries for behaviour — sizes live at the retailer, real-retailer fulfilment is simulated — did not vanish; they are in the tool description and in the instructions, which the model reads, rather than in the output, where they would be fields Channel3 never returned.

The live path is `httpCatalogTransport`: `POST <base>/search` with the key in `x-api-key`, a 20 s timeout, and typed refusals (`catalog-unconfigured`, `catalog-unreachable`, `catalog-timeout`, `catalog-bad-response`, plus the local `catalog-empty-query`, `catalog-bad-limit` and `catalog-invalid-want`). `astropods.yml` declares `CHANNEL3_API_KEY` as a secret input and `CHANNEL3_CATALOG_URL` as a non-secret default, so the T9.8 trap applies: the key reaches the container only because the blueprint declares it. The redeploy is the standing command plus the new var, after the vault secret exists:

```sh
cd agent/runtime && bun agent/offline-check.ts && ast spec validate && ast push -V private -y && ast agent redeploy --id <deployment-id> --latest --var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY --var CHANNEL3_API_KEY=@CHANNEL3_API_KEY --wait
```

Offline, `bun agent/offline-check.ts` runs 179 checks (T9.9 left 130) and `./node_modules/.bin/tsc --noEmit` is clean. The catalog side replays `testdata/channel3`: the oxford search returns `tbnrl46`, `VyAkjCA` and `PSW8GeK`; excluding `tbnrl46` leaves the other two; the candidate names the line "Button Down Oxford Shirt" and the SKU `PSW8GeK`; the Bonobos line name alone matches only the equal `top-fit` set, as `agent/catalog` proves; an incomplete cut, an empty query and a non-positive limit are refused before the transport is called. The size clause is proved where Channel3 carries a size: the footwear search against `footwear-cut: Low cut` names `VWvTIPF`, its line and the five size labels its search answer lists — 7.5, 9, 9.5, 10 and 10.5, each with `available` null, so the answer lists them without marking any in stock. The demo oxford line has `variants: null`, and the check proves the tool says exactly that rather than inventing a size — which is the honest reading of "line, size and SKU" for that line: the size it recommends comes from recall, and the catalog names the line and SKU. The HTTP half is pinned against a local stub: the search path, the `x-api-key` header, the query and limit body, one request per call, a 401 carrying the service's code, a non-JSON answer, an answer with no products array, a deployment with no key, and one that cannot be reached. The 20 s timeout path is the one refusal left untested here: it needs a stall the stub does not simulate.

No live call, no spend, $0. The live turn the phase block's Done-when asks for was not made: this task's brief scoped the proof to offline and forbade live calls. The code path and the key wiring are in place; what remains is a deployment and a turn. Three things the next agent should not have to rediscover:

- `agent/catalog` (Go) and `agent/catalog.ts` (the agent) are two implementations of one rule, kept in step only by the shared recordings and the same negative controls. Keep their fixtures the same; a new recording should reach both.
- The layer's line identity and Channel3's are different: recall's `brand-x-001` / `shirts` / `regular` is Channel3's `SMwk` / `dress-shirts` / `top-fit: Regular`. The mapping is demo identity, carried in the instructions, not derived from the layer, so a second line (or T9.15's second shopper asking about one) needs its mapping added there.
- A search costs one Channel3 credit and product detail calls are free; this tool makes one search per product turn and no detail calls. A demo run should budget one credit per step-2 turn.

Findings, each with an owner. The live step-2 turn is unproven, owner: T9.20, opened on main after this branch was cut (`requires: [T9.17]`, `owns: demo/`, `fixture-ok: no`), which adds the catalog call to `demo/run.py`, takes the turn and records the trace — `demo/` is outside this task's `owns`. The vault secret and the redeploy have no task of their own: they are T9.20's first move, an operator command rather than a code change, and `ast secrets list` shows the name, never the value. The per-turn Channel3 credit is a new line in the demo's spend, owner: T9.20, whose Done-when records it.

**Review (fresh eyes).** Re-verified from the committed files on a branch cut at the task tip, no live call, no key, $0. `git show --name-status` is seven files under `agent/runtime/` plus this block; `internal/auth/surface.go` and `testdata/api` are untouched, so the edge's set did not move. The match rule was read against `agent/catalog`'s and replayed from the recordings: the three Sunspel SKUs share `top-fit: Regular`, excluding `tbnrl46` leaves the other two, Bonobos `Slim` returns `V9BYM3z` alone and `Slim`+`Tailored` returns `m4iRrEM` alone, and the two Puma `footwear-cut: Low cut` SKUs come back with the rest of `T0QO`'s line filtered out — brand and category match exactly, the fit values are compared as sets, and the title is never read. The tool adds no field: it returns `structuredClone` of Channel3's record, and the check compares each returned record's key set with the recording's and names six plausible invented fields it forbids. The offered edge tools still equal the eight `CallerAgent` names exactly, `search_catalog` sits outside that set, and the layer binding holds eight methods.

Mutations from the committed code, each reverted with `git status` clean: dropping the set-size equality in `equalFitSets` fails two checks, dropping the fit comparison fails three, ignoring `exclude_skus` fails two, adding a `size_note` to each record fails six, emptying `AGENT_LOCAL_TOOLS` fails two, and mapping a failed fetch to the wrong code fails the new unreachable check. The checks are 179 and `tsc --noEmit` is clean; `./tools/check.sh` is green after `npm ci` in `web/` and `merchant/`, its frontend step taking the fallback branch and printing its limit. Limits, stated: only top-level fields are compared with the recording, so a field invented inside a nested object would pass, though `structuredClone` adds none; the 20 s timeout path has no case; and `bun agent/offline-check.ts` in a tree with no `agent/runtime/node_modules` lets bun's auto-install resolve a zod whose `_def` carries no `typeName`, which fails the eight schema-type checks until `bun install --frozen-lockfile` restores zod 3.25.76 — T9.19's gate step must install the frozen packages for that reason. Fixes landed in this review: the size claim now quotes the five labels the recording actually lists and says none is marked available, the HTTP-half sentence names the unreachable case the check now carries, the check count follows, and the pending live turn, redeploy and credit now name T9.20 instead of a task nobody had opened. Findings: (Medium, T9.20) the Done-when's live turn is unproven, which is a scheduling gap rather than a code one — this task's brief forbade live calls; (Low, T9.20) the redeploy with `CHANNEL3_API_KEY` is T9.20's prose precondition and not a clause of any Done-when, so the dispatch should carry it rather than rely on this block; (Low, T9.20 or the orchestrator) `product.md`'s step 2 still says the size chart maps the candidate to the shopper's measurements, and `dev-diary/research/channel3.md` already records that no size chart exists and that step must come from the retailer's page, so the step-2 turn can name the line and SKU from the catalog and the size only from recall — `product.md` is outside every task's `owns`, so rewording it is the orchestrator's call.

---

### T9.18: The deploy README's pin section covers bun
```yaml
requires:   [T9.16]
fixture-ok: yes
size:       XS · mid
owns:       deploy/README.md, tools/pincheck/
status:     done
```
`tools/pincheck` now compares Go, Node and bun, but `deploy/README.md`'s toolchain section still lists only Go and Node and still says what the gate reads. Add bun's pin and its sources, and correct the paragraph about what the check covers.

The T9.16 review found the row that task adds would be a pin nothing compares: `readmePins` filters to Go and Node, so a Bun row moved to 2 leaves the gate green. Make the Bun row a required source, or record why it stays unread.

**Done when:** the pin section lists all three toolchains with their sources and the check's coverage; the Bun row is read by the check (or its exclusion is recorded with its reason); the gate passes.

**Notes (worker).** `deploy/README.md`'s toolchain table gains a Bun row: pin 1, pinned by `agent/frontend/Dockerfile`'s one stage and the builder plus runtime stages of `agent/runtime/`, `agent/second/` and `bench/pilot/agent/` (`oven/bun:1`, `oven/bun:1-slim`), checked by `ci.yml`'s `setup-bun` 1.3.14. The section's opening names the four toolchains the table records and says which source holds each: the contract (T0.2) for Go and Node, CI for bun. The paragraph after the table names what the check reads — both deploy images' `FROM` lines, the `oven/bun:` `FROM` line of every Dockerfile in the tree, CI's `go-version`, `node-version` and `bun-version`, `go.mod` and all three rows — and its coverage, 18 declarations, up one from 17. Both asymmetries are stated in the same sentence, Go's patch float and bun's major-only tags, and the "cannot pass by reading less" clause stays.

`tools/pincheck` reads the row. `readmePins` filtered to Go and Node; it now filters and requires through `readmeKinds()` (`go`, `node`, `bun`), so the row is a source on the same terms as the rest and one the table has lost is a failure rather than one fewer pin. The T9.16 finding is closed by reading it, not by recording an exclusion. The row is compared by major, as the images and CI are, so the images' `oven/bun:1` and CI's 1.3.14 agree and a 1.x patch drift in the row alone would stay green; that is the same limit the Dockerfile tags carry, and the section states it rather than implying a patch is compared.

Proof. Two table cases are added: `bun drifts in the readme table` (wants `bun: deploy/README.md` and `says 2`) and `the table loses its bun row` (wants `deploy/README.md: no bun row`); `go test ./tools/pincheck/` is green over 25 cases. Mutation-checked against the real tree, each reverted with `git checkout` and `git status` clean afterwards: the row moved to 2 fails with `bun: deploy/README.md's toolchain table bun row (line 30) says 2, but agent/frontend/Dockerfile's bun build stage (line 9) says 1`; the row deleted fails with `deploy/README.md: no bun row in the toolchain table, so the table states no bun pin`. `go run ./tools/pincheck` prints `go 1.27, node 26 and bun 1 agree across 18 declarations`. `./tools/check.sh` is green after `npm ci` in `web/` and `merchant/`; the frontend step takes its no-`node_modules` fallback, which T9.19's brief covers.

Finding (Low), owner **T9.19**, whose `owns` is `tools/check.sh`: that file's pin comment still says "Bun's live in every Dockerfile's `oven/bun:` build stage and CI's setup-bun", which is now one source short — the table row is bun's third. The fix is that sentence plus the count it states for Go and Node, and it belongs to the task that is already editing that file's prose.

**Review (fresh eyes).** Re-verified from the committed files on a branch cut at the task tip, no live call and no key, $0. Against the task's base the branch is `deploy/README.md`, `tools/pincheck`'s two files and this task's own lines, and nothing else. The table states Go, Node, Bun and Lambo, and the Bun row names the tree's bun stages exactly: `agent/frontend/Dockerfile` line 9, and lines 2 and 21 of `agent/runtime/`, `agent/second/` and `bench/pilot/agent/` (`oven/bun:1`, `oven/bun:1-slim`); CI's `setup-bun` is line 33 at `1.3.14`. `go run ./tools/pincheck` prints `go 1.27, node 26 and bun 1 agree across 18 declarations`, and the count is the code's own: the two deploy images' Go and Node stages (4), the seven `oven/bun:` stages (7), CI's three keys (3), `go.mod` (1) and the table's three rows (3).

Mutation-checked on the real tree, each reverted with `git checkout` and a clean `git status` after: the Bun row at `| Bun | 2 |` fails `bun: deploy/README.md's toolchain table bun row (line 30) says 2, but agent/frontend/Dockerfile's bun build stage (line 9) says 1`; the row deleted fails `deploy/README.md: no bun row in the toolchain table, so the table states no bun pin`; the row at `1.3` stays green, the major comparison the section states; the row at `1.3.0` fails against CI's `1.3.14`, because `disagreements` also holds pins that state a patch to exact agreement. The last of those corrects the notes: "a 1.x patch drift in the row alone would stay green" holds for `1.3` and not for `1.3.0`.

`go test ./tools/pincheck/` is green over the 25 table cases, and both new ones bite: cutting `readmeKinds` back to Go and Node fails exactly `bun drifts in the readme table` and `the table loses its bun row`.

Fixes landed in this review, both in `deploy/README.md`. The paragraph quoted the check's output as `go 1.27, node 26 and bun 1 agree`, a prefix of the line it prints, beside a count stated twice; it now quotes the printed line with the count inside it. The opening called the bun-pinning images "the reference agents' images", which is short one: `bench/pilot/agent/` pins bun and is neither agent's deployed image, so it now says "the four agent images".

Findings, with owners: none new. The task's Low finding is right that `tools/check.sh`'s pin comment names bun's two sources where the check now reads three, and T9.19 owns that file. Its brief does not mention the comment, only the agent steps, so the orchestrator should carry that sentence into the brief rather than leave it for that worker to find; the `owns` line already covers it. One clause of the finding is not right: the Go and Node count of six places is still six (both images' stages, `setup-go`, `setup-node`, `go.mod` and the table), so only the bun sentence needs the third source.

Limits. No GitHub Actions run backs this, and the task changes no workflow. The 18 is a snapshot of the tree: a Dockerfile added later keeps the check green and makes the line stale, which is why it now reads as what the command prints. Bun is compared by major, as the section states, and only a row that states a patch is held exactly. `./tools/check.sh` is green with `npm ci` in `web/` and `merchant/`; `agent/frontend/node_modules` is absent here, so the frontend step takes its no-modules fallback, which T9.19's brief covers.

Approve: scope is clean, the Bun row is a required source that fails on drift and on loss, both cases are proved by a table test and by mutation on the tree, the section states what the check reads and its coverage, and the gate passes.

---

### T9.19: The gate runs the agents' offline checks
```yaml
requires:   [T9.9, T9.13]
fixture-ok: yes
size:       XS · mid
owns:       tools/check.sh, .github/workflows/ci.yml
status:     done
```
T9.9 gave both agents' tool bindings offline coverage, but `tools/check.sh` does not invoke it: `bun agent/offline-check.ts` and `bun agent/check.ts` run only when someone runs them by hand, so a tool-surface drift passes the gate. Add both checks to the gate behind the same bun guard the frontend step uses, and install the two agent packages' frozen dependencies in CI, or the runner lacks `agent/runtime/node_modules` and `agent/second/node_modules` and both checks fall through.

**Done when:** the gate runs both agent checks where bun is present and states the fallback where it is not; a broken tool id fails the gate; the gate passes.

**Notes (in-review).** Both agents' offline checks now run in the gate, behind the frontend step's bun guard, and CI installs both packages' frozen dependencies. `tools/check.sh` gains `check_agent`, called once for `agent/runtime agent/runtime/agent/offline-check.ts` and once for `agent/second agent/second/agent/check.ts`. With `bun` on PATH and that package's `node_modules` present it prints `-- <name> (bun <script>)` and runs the check through the captured-output helper the frontend steps already use, so a failing check prints its own `FAIL:` lines and fails the gate. Without bun it prints `skipped: bun is not on PATH; it cannot be run on node`; without the package's modules, `skipped: <dir>/node_modules is absent; run bun install --frozen-lockfile in <dir>`. Both checks are Bun programs (`Bun.file`, `import.meta.dir`) that resolve their dependencies from the package's own tree, so node cannot stand in for bun the way it does for `server/relay.ts`.

`.github/workflows/ci.yml` gains two steps after the shopper-page install, `bun install --frozen-lockfile` with `working-directory: agent/runtime` and `agent/second`, so the runner does not fall through the guard. The `actions/cache@v4` key now moves with all three `bun.lock` files, and the setup-bun comment names all three checks. The workflow parses as YAML and reads back as eleven steps.

Proof on the workstation, no key, $0:

- `./tools/check.sh` is green and prints `-- agent/runtime offline check (bun agent/runtime/agent/offline-check.ts)` then `-- agent/second offline check (bun agent/second/agent/check.ts)`; run by hand from the repository root, the two scripts pass 130 and 125 checks apiece.
- With bun off `PATH` and both agent packages installed, the same gate is green and prints both `skipped: bun is not on PATH; it cannot be run on node` lines. With bun present and `agent/runtime/node_modules` moved aside, it is green and prints that check's `node_modules is absent` text while the second check still runs. The modules-absent text names the install command, so the line says how to get the check back.
- A broken id fails the gate: `id: "record_feedback"` changed to `record_feedbackx` in `agent/second/agent/tools.ts` makes the gate exit 1 with `FAIL: the record_feedback binding carries its frozen id (id record_feedbackx)` and `FAIL [agent/second offline check]`, after the second agent's other 124 checks pass. Reverted, `git status` clean.
- On the tree the two amendments below assume -- T9.14's `server/status.ts` and T9.18's Bun row applied and reverted -- the gate is green with bun hidden: the fallback runs `tsc ... server/relay.ts server/status.ts`, both agent checks print their skipped lines, and `tools/pincheck` prints `go 1.27, node 26 and bun 1 agree across 18 declarations`.

Two amendments rode in the same commit. The pin comment now names each toolchain's sources -- including the `deploy/README.md` table row as Bun's third source beside every Dockerfile's `oven/bun:` stage and CI's setup-bun -- and states 18 declarations in all. The fallback typecheck now covers `server/status.ts` beside `server/relay.ts`, since T9.14 makes the status mapping import-free like the relay and the fallback was otherwise leaving it unchecked where bun or the modules are absent.

**Limit, stated.** This branch is cut at 329918d, which carries neither T9.14's `server/status.ts` nor T9.18's README Bun row (`task/t914` at c1620e6, `task/t918` at 2047be0). On that base `tools/pincheck` reads 17 declarations, not 18, and the no-bun fallback exits 2 with `TS6053: File 'server/status.ts' not found`. With bun and `agent/frontend/node_modules` installed -- the state CI runs in -- the gate is green on the base and prints `-- agent/frontend typecheck (bun run typecheck)` plus both agent checks. Both halves of this task are proved on the tree those two tasks produce, so the orchestrator must land them before or with this branch. No GitHub Actions run exists in a worktree, so the runner is unproven, as it was for T9.13.

**Base correction (review).** That paragraph's base is stale. The commit rests on `4563e1d`, not `329918d`: T9.18 closed at `84fae2f` and T9.14 at `4563e1d`, both before it. On the landed tree `go run ./tools/pincheck` prints 18 declarations, `agent/frontend/server/status.ts` is present, and the no-bun fallback compiles it, so the 17-count and the `TS6053` line describe `329918d` alone and nothing is left to land before this branch.

**Surprise.** Both checks run unchanged from the repository root (`bun agent/<pkg>/agent/<script>.ts`), because they resolve the contract examples and the Go access table from `import.meta.dir`, not the working directory. That is why the gate need not `cd` into the package, and why the guard tests for the package's `node_modules` rather than the root's.

**Review (fresh eyes).** Re-verified from the committed files on a branch cut at the task tip, no key and no live call, $0. Scope: `c5741cf` is `tools/check.sh` and `.github/workflows/ci.yml`, `c7aca57` is this block, and no other file moves.

- With bun 1.3.14 and all three bun packages installed, `./tools/check.sh` exits 0 and prints `-- agent/runtime offline check (bun agent/runtime/agent/offline-check.ts)` then `-- agent/second offline check (bun agent/second/agent/check.ts)`, after the frontend's full-package typecheck. By hand from the repository root the two scripts exit 0 with 130 and 125 `ok:` lines and no `FAIL`.
- With bun off `PATH` and every package's modules in place, the gate exits 0, takes the frontend fallback (`tsc ... server/relay.ts server/status.ts`, exit 0) and prints both `skipped: bun is not on PATH; it cannot be run on node` lines. With bun present and only `agent/runtime/node_modules` moved aside it exits 0, prints `skipped: agent/runtime/node_modules is absent; run bun install --frozen-lockfile in agent/runtime`, and still runs the second check; with both moved aside it prints both lines, each naming its own directory. Modules restored, `git status` clean.
- Mutation: `id: "record_feedback"` changed to `record_feedbackx` in `agent/second/agent/tools.ts` fails the gate, exit 1, with `FAIL: the record_feedback binding carries its frozen id (id record_feedbackx)` after 124 `ok:` lines and `FAIL [agent/second offline check] agent/second offline check reported problems` last, so the captured log reaches the terminal. Reverted, `git status` clean.
- "Cannot be run on node" holds as written: with the modules installed, `node agent/second/agent/check.ts` exits 1 on `ERR_MODULE_NOT_FOUND` for `./edge.js`, because the package's specifiers name `.js` over `.ts`.
- Both carried findings are in place. The pin comment names bun's three sources -- the `oven/bun:` stage of every Dockerfile, CI's `setup-bun` and the table's Bun row -- which is what `tools/pincheck` reads, and it prints `go 1.27, node 26 and bun 1 agree across 18 declarations`. The fallback typecheck names `server/status.ts` beside `server/relay.ts` and compiles both.
- CI: the workflow parses as YAML and reads back as eleven steps, with `bun install --frozen-lockfile` in `agent/runtime` and `agent/second` after `setup-bun`, and the `actions/cache@v4` key moving with all three `bun.lock` files. Go's and Node's pins and `bun-version: '1.3.14'` are unchanged from `main`, and `pincheck` still agrees.

Landing note. `main` has moved past this branch's cut: T9.17 rewrote `agent/runtime/agent/offline-check.ts` and added `agent/runtime/agent/catalog.ts`. The gate step names no count, so it runs whatever that package's script becomes: with `main`'s `agent/runtime` copied in and its modules in place, `bun agent/runtime/agent/offline-check.ts` exits 0 with 179 `ok:` lines, and the working tree was restored clean. The note's "130" describes the tree it was written on, not the gate's contract. `main` touches neither `tools/check.sh`, `.github/workflows/ci.yml`, `agent/runtime/package.json`, `agent/runtime/bun.lock` nor `agent/second/`, so this task's paths merge without conflict.

Finding (Low), owner **T9.19a (proposed, `owns: tools/check.sh`; the orchestrator numbers it)**. Neither agent package is type-checked anywhere. Each carries a `tsconfig.json` and `typescript` as a devDependency but no `typecheck` script, and the gate runs their code through Bun, which transpiles without checking types. `./node_modules/.bin/tsc --noEmit -p tsconfig.json` exits 0 in both packages today, so the step is green to add inside `check_agent`'s modules guard; until it exists, a type error Bun passes over -- an argument typed `string | undefined` where the tool schema requires a string, say -- reaches the gate green. It is coverage this task did not promise rather than a fault in it, which is why it is recorded here and not written.

Limits. No GitHub Actions run exists in a worktree, so the runner stays unproven the way T9.13's did; what is proved is the command sequence locally, plus `bun install --frozen-lockfile` succeeding in both new working directories. The skip lines are the gate's honesty about a workstation without bun: there the tool surface is still unexercised, which is the trade the brief makes. `agent/second/node_modules` was moved aside only together with `agent/runtime`'s; the guard is one function called twice, and each line named its own directory.

Approve: scope is clean, both sides of the guard and the failure path are proved on the tree, the carried findings are in place, and the two tasks the note asked to land first are already on `main`.

---

### T9.20: The demo's step 2 runs live
```yaml
requires:   [T9.17]
fixture-ok: no
size:       XS · mid
owns:       demo/
status:     in-review
```
T9.17 gave the reference agent a catalog search, but no live turn has used it and `demo/run.py` does not call it, so product.md's step 2 still has no live path. After the agent is redeployed with `CHANNEL3_API_KEY`, add the catalog call to the demo driver, take one live turn that names the matched candidate (line, size, SKU) before its recommendation, and record the step-2 evidence and the one Channel3 credit it costs.

**Done when:** a live reference-agent turn searches the catalog and names the matched candidate before its recommendation; the demo driver records it; the credit and any spend are recorded; the gate passes.

**Notes (in-review).** The live turn is trace `c2a3bf4d` (the full id stays in `demo/out/`). It was one fresh conversation sent through `demo/drive.py`, with the prompt "I need a Brand X oxford shirt in the regular fit. Which one would you recommend?" and no correction. The deployment offers nine tools, `search_catalog` among them. The trace's order is `recall_for_purchase` (22.637–22.995), `search_catalog` (22.640–24.106), then the answer generation at 24.131, so the search finished before the model wrote the recommendation. The reply announces that it will "check your purchase history and search the catalog at the same time", then opens its recommendation with "Here's my recommendation:", names the line "Men's Button Down Oxford Shirt" and SKU `tbnrl46`, then gives size M under correction `corr_000004`; the search span ended before the answer generation, so the catalog result preceded the recommendation. The size comes from recall, not the catalog: the oxford line's `variants` is null, so the catalog names the line and SKU and recall names the size, as T9.17 recorded.

`demo/run.py` now carries the call. `TOOL_NAMES` includes `search_catalog`; `step2(ref_conv)` runs after step 1, sends the product request, requires recall first, and records the search span, that its end precedes the answer generation, and the named line, SKU and size from the reply checked against `testdata/channel3/search-oxford-shirts.json`. A full run includes the step-2 turn in the trace-cost total and the summary's `step2_catalog` block. `python3 demo/run.py --step2-trace <trace-id>` writes the same evidence from a turn sent through `demo/drive.py`, with no live call of its own; that is how this turn's evidence was written to `demo/evidence/step2-catalog/02-catalog-candidate.json`. The evidence redactor now also strips URL query strings, because a catalog offer link can carry an affiliate id, which is account detail.

Credit and spend. One catalog search, **one Channel3 credit**; the turn made no product-detail call. Trace `c2a3bf4d` cost **$0.0553**; usage spend moved $5.86 → $5.92 and credit $4.14 → $4.08. That is under the $0.25 budget, and it is the only live call this task made. The conversation create is not a turn, and the trace reads are free.

Limit, recorded rather than smoothed over: `tbnrl46` is the SKU the three seeded returns carry, so it matches the cut but is the returned item, not a new candidate. The trace carries tool span names and times, not the tool's arguments or answer, so the demo cannot tell whether the SKU came from the search answer or from recall. The catalog tool's `exclude_skus` description offers to skip the products the shopper already returned, but the agent's step-2 instruction never asks for it, and the spans show the search launched 3 ms after recall and outliving it, so the search was issued before recall answered. Finding (owner T9.17): tighten the agent's step-2 instruction so recall answers first and the search is given the returned SKUs as `exclude_skus`; that file is outside this task's `owns`. The T9.17 review's note that `product.md`'s step 2 still says the size chart maps the candidate to the shopper's measurements also stands; `product.md` is outside every task's `owns`.

The gate passes after `npm ci` in `web/` and `merchant/`.

**Review (fresh eyes, T9.20).** Re-verified from the committed files; no live call, $0. `git show --stat ab64aa9` is `demo/` plus this block. The evidence's spans match the note to the millisecond, its named line and SKU are the recording's (`tbnrl46`, "Men's Button Down Oxford Shirt", `variants` null), `turn.cost` is 0.055296, and the redactor strips a query string: the recording's offer link carries `?o=…&aid=…`, and the committed reply holds only the bare path, so no affiliate id is in the evidence. The three README lines about the ledger were checked against T4.8's landed code on `main` — `live.ts`'s `?t=`/`?s=` with the demo shopper as the absent `?s=` default, the replay label without a token, and the failed read's alert with no grid and no pending line — and T4.8 landed on `main` after this branch's cut, so this tree does not carry that code and the lines describe `main`, not the branch. `./tools/check.sh` passes.

Verdict: the Done-when's mechanism is met and its candidate clause is not. The turn searched, the search ended before the answer generation, the driver records the ordering, the credit and the spend, and the gate is green. But `product.md`'s step 2 asks for "a different SKU" that "shares the cut of the returned items", and `tbnrl46` is the SKU the three seeded returns carry, so the turn proves the catalog call and not step 2's candidate. The builder recorded that limit rather than glossing it. T9.20 stays in review: T9.17 fixes the agent's instruction, then T9.20 records one turn on the redeployed agent.

The cause is narrower than "the demo cannot tell". The reply's first sentence is "I'll check your purchase history and search the catalog at the same time!", and the spans agree: `search_catalog` starts 3 ms after `recall_for_purchase` and ends 1.1 s after it, so the search was issued before recall answered. The conversation was fresh, and neither the agent's instructions nor the tool's description names a SKU (the three appear only in the recordings, the checks that replay them and this task's evidence), so no SKU was in context when the search was called and it cannot have been handed the returned ones to exclude. Finding (Medium, owner T9.17): recall's answer carries each returned SKU (`beliefs[].evidence[].item.sku`, `internal/domain/evidence.go`), so the fix is instruction-level: recall first, then `search_catalog` with `exclude_skus` from those rows, and name only a SKU the search returned. `demo/run.py` still cannot read tool arguments, so the demo keeps recording `returned_sku` instead of claiming a new candidate.

Finding (Low, owner T9.20, when the turn is re-recorded): `catalog_products()` accepts any SKU in `testdata/channel3/search-oxford-shirts.json`, and that answer holds ten products of which three share the cut (brand `SMwk`, `dress-shirts`, `top-fit` `Regular`), so a reply naming `3SAdGdJ` or `LCfcrFc` would be recorded as the matched candidate although the search could not have returned it. The check should read the cut the demo already names for the ask, so it stops accepting a product the cut excludes, and the evidence should say the match is the demo's reading of the recording rather than the tool's answer.

Fix landed in this review: `normalise_gaps` used to drop step 2's gap as soon as any catalog evidence existed, so a full run whose turn named a returned SKU would have summarised a clean step 2; it now carries that limit with owner T9.17. Also corrected here: the note's and `demo/README.md`'s "the reply opens with" (the reply announces the two calls first), and the claim that the agent's instruction asks for `exclude_skus` (the tool's description does; the instruction does not).

Limits, stated: the trace was not re-read, so the span times rest on the committed evidence and the nine offered tools on the builder's read of the deployment; the size is the reply's and is checked against nothing; the `$0.25` budget lives in the brief and the one-credit cost in T9.17's note, neither in a committed file; and a full run now puts step 2's turn into step 3's conversation, so its step-3 turns will differ in shape from run-6's, which had none.

---

### T9.21: Type-check the agent packages
```yaml
requires:   [T9.19]
fixture-ok: yes
size:       XS · mid
owns:       tools/check.sh
status:     done
```
Neither agent package is type-checked anywhere: both carry a `tsconfig.json` and `typescript` as a devDependency but no `typecheck` script, and the gate runs them through Bun, which transpiles without checking. `./node_modules/.bin/tsc --noEmit -p tsconfig.json` exits 0 in both today, so the step is green to add inside `check_agent`'s modules guard.

**Done when:** the gate type-checks both agent packages where their modules exist, and a type error fails it; the gate passes.

**Notes (worker).** `check_agent` in `tools/check.sh` now type-checks each package after its offline check: it `cd`s into the package in a subshell and runs `./node_modules/.bin/tsc --noEmit -p tsconfig.json` through the `run_frontend` capture the other steps use, so tsc's own diagnostics reach the terminal and a failure ends the gate at `FAIL [<dir> typecheck] <dir> typecheck reported problems`. The `cd` is what makes it the package's own TypeScript and the package's own tsconfig: `-p tsconfig.json` is relative to the working directory, and `./node_modules/.bin/tsc` is the binary `bun install --frozen-lockfile` puts there. The offline checks still run from the repository root, unchanged, because they resolve the contract examples from `import.meta.dir`.

Both type-checks sit inside the guard that already existed, so the skip is the guard's own, unchanged: without bun, `skipped: bun is not on PATH; it cannot be run on node` for each package, and with bun present but a package's modules absent, `skipped: <dir>/node_modules is absent; run bun install --frozen-lockfile in <dir>` for that package while the other runs both its steps. The section comment gains T9.21 in its heading and one paragraph saying why the step exists — Bun transpiles without checking types, so a type error its runtime passes over would otherwise reach the gate green.

Proof. `./tools/check.sh` exits 0 and prints `-- agent/runtime typecheck (tsc --noEmit)` after that package's offline check, then `-- agent/second typecheck (tsc --noEmit)`; both commands run by hand in their packages exit 0 with no diagnostics. The two mutations, each reverted with `git checkout` and a clean `git status` after: appending `const typecheckMutation: number = "not a number";` to `agent/runtime/agent/catalog.ts` fails the gate, exit 1, with `agent/catalog.ts(353,7): error TS2322: Type 'string' is not assignable to type 'number'.` then `FAIL [agent/runtime typecheck] agent/runtime typecheck reported problems`, and the run stops there, before the second package's steps; the same line in `agent/second/agent/tools.ts` fails at `agent/tools.ts(157,7)` with `FAIL [agent/second typecheck] agent/second typecheck reported problems`. With bun off `PATH` the gate is green and prints both `skipped: bun is not on PATH` lines; with `agent/runtime/node_modules` moved aside, green, with that package's `node_modules is absent` line and the second package's check and type-check both running. Modules restored, `git status` clean apart from this branch's own files.

Limit, stated. A `node_modules` installed with production dependencies only carries no `./node_modules/.bin/tsc`, and that run fails the gate with `./node_modules/.bin/tsc: No such file or directory` rather than skipping. That is deliberate: a package whose TypeScript is absent cannot be type-checked, and its offline check would still run, so skipping silently would hide exactly the coverage this task adds. CI's `bun install --frozen-lockfile` installs devDependencies, so the runner is unaffected, though no Actions run exists in a worktree and the runner is unproven here as it was for T9.19. `skipLibCheck: true` in both tsconfigs means a type error inside a dependency's declarations is not reported; that is the packages' own setting, and the gate reads it rather than overriding it.

Findings: none new. T9.19's Low finding — that neither agent package is type-checked anywhere — is closed by this task.

**Notes (review).** Re-verified from the committed tree with both packages' modules installed from their frozen lockfiles: the scope is `tools/check.sh` and this section; both packages type-check with their own 5.9.3 `tsc --noEmit -p tsconfig.json` after their offline check, each project listing every source file under `agent/`; and `./tools/check.sh` exits 0, printing each `-- <dir> typecheck (tsc --noEmit)` line after its package's check. A fresh mutation — `const reviewMutation: number = "not a number";` appended to `agent/runtime/agent/index.ts` — fails the gate at `agent/index.ts(121,7): error TS2322` and `FAIL [agent/runtime typecheck] agent/runtime typecheck reported problems`, stopping before `agent/second`'s steps; the same line in `agent/second/agent/index.ts` fails that package's `tsc` run the same way. With bun off `PATH` the gate is green and prints both `skipped: bun is not on PATH` lines; with `agent/runtime/node_modules` moved aside it is green, names that package's absent modules, and still runs `agent/second`'s check and type-check.

Two claims in the notes above were corrected here. The skip is the guard's own, not the frontend's fallback: the frontend falls back to a partial `tsc` run over `server/relay.ts` and `server/status.ts`, while these two packages type-check nothing when bun or their modules are absent. And a `node_modules` with no `./node_modules/.bin/tsc` fails the gate with `./node_modules/.bin/tsc: No such file or directory`, not `command not found` — observed by moving that binary aside in `agent/second` and running the gate.

---

### T9.22: Search the catalog after recall, excluding returned SKUs
```yaml
requires:   [T9.17]
fixture-ok: yes
size:       XS · mid
owns:       agent/runtime/
status:     claimed:t922-sequencing
```
T9.20's live turn proved the mechanism but named `tbnrl46`, a SKU the three seeded returns carry: the agent issued `search_catalog` 3 ms after recall started and before recall answered, so it had no returned SKUs to exclude, and its step-2 instruction never asks for sequencing or exclusion (the tool description offers `exclude_skus`; the instruction does not). product.md's step 2 wants a *different* SKU sharing the cut. Change the instruction: recall answers first, then `search_catalog` with `exclude_skus` from the returned rows' `item.sku`, and name only a SKU the search returned.

**Done when:** the instruction sequences recall before the catalog search and passes the returned SKUs as `exclude_skus`; the offline check covers the sequencing rule; the deployment is redeployed from the committed code; the gate passes.

---

## Exit Criteria

- [x] A seeded population where the demo oxford is Earned honestly, and other lines are Building or Out of true (T9.1).
- [x] The ledger shows the deciding step and the pool, with no other shopper's rows (T9.2).
- [x] The merchant app is built and deployed, reading only `product_fit` (T9.3, T9.3d).
- [x] The reference agent never refuses on evidence and records standalone corrections (T9.4).
- [x] A second agent honours a correction it never saw (T9.5).
- [ ] The pivoted layer is live, and the demo runs end to end (T9.6, T9.7).
