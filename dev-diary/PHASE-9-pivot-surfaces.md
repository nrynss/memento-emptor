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
status:     claimed:t93g-node-pin
```
The demo image builds `web/` on Node 22 while T0.2's contract and CI pin Node 26. The deployed build works, but the pin has drifted, so a future frontend build could pass locally and CI and fail in the image (or the reverse). Align the image's build stage with the pinned Node version, prove the image builds and serves the app, and record the pin so it is checked with the rest of the toolchain.

**Done when:** the Dockerfile's frontend build uses the pinned Node major; the image builds and serves `/` and `/healthz`; the pin is named where the other toolchain pins are recorded; the gate passes.

---

### T9.3h: Align the merchant image's Node pin
```yaml
requires:   [T9.3d]
fixture-ok: no
size:       XS · mid
owns:       deploy/merchant/Dockerfile, deploy/README.md
status:     not-started
```
T9.3g aligned the demo image's frontend build to the pinned Node 26, but `deploy/merchant/Dockerfile` still builds `merchant/` on `node:22-bookworm`. It is a separate image, so the pin drifted independently; align it the same way, prove the merchant image builds and serves `/` and `/healthz`, and record the pin beside the others.

**Done when:** the merchant image's build stage uses Node 26; the image builds and serves `/` and `/healthz`; the pin table covers it; the gate passes.

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
- [x] The merchant app is built and deployed, reading only `product_fit` (T9.3, T9.3d).
- [x] The reference agent never refuses on evidence and records standalone corrections (T9.4).
- [x] A second agent honours a correction it never saw (T9.5).
- [ ] The pivoted layer is live, and the demo runs end to end (T9.6, T9.7).
