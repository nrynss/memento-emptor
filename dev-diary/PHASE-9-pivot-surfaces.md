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
status:     not-started
```
The T9.8 review found three low defects in the live agent surface. Both agents expose `ledger_beliefs`, `ledger_belief` and `ledger_evidence`, which the edge admits for `CallerShopper` only, so an agent key gets a 403 on each. `tools.ts` has no offline coverage: the checks inject the transport below it, so tool ids, schemas and execute bindings are exercised only by a live build. And the reference prompt tells the agent to retry any failed tool call once, so a `record_correction` that landed and then timed out could be written twice; retries must be for reads only, or a write must recall before retrying.

**Done when:** each agent's exposed tools match the edge's access table (or the table is widened deliberately, recorded), a check exercises the tool bindings themselves against the frozen contract, and the retry instruction cannot duplicate a write; the gate passes and the deployed builds match the committed code.

---

### T9.7: The demo, after the pivot ★
```yaml
requires:   [T9.2, T9.3d, T9.5, T9.6, T9.8]
fixture-ok: no
size:       M · frontier
owns:       demo/
status:     claimed:t97-demo
```
product.md's demo from step 0 to step 7, live, on the hosted service: the new shopper sized from the pool on day one; the three returns; the correction; the purchase and feedback; the dispute that leaves sizing alone; the ledger; the second agent honouring the correction; the merchant app's "why". The words about pooling follow the consent decision (T7.3). Budget: under $1.00. Rehearse step 0 first and check its cost.

**Done when:** a recorded run covers every step, with each step's trace or screen linked in the handoff log. `demo/reset.sh` restores the starting state. Spend is recorded.

---

### T9.11: Scrub the Astropods org id from the drivers
```yaml
requires:   [T9.7, T9.9]
fixture-ok: yes
size:       XS · mid
owns:       demo/drive.py, agent/second/drive.py
status:     not-started
```
The P3 review found the Astropods org id committed in `demo/drive.py` and `agent/second/drive.py` as a `secret-tool` attribute. It is an identifier, not a credential, but the diary's own rule excludes account detail and the scrub rule lists user ids as placeholders. Replace it with a placeholder and read the org from the environment or the CLI's own config.

**Done when:** neither driver carries the org id; both read it from the environment or the CLI; a run still works; the gate passes.

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
status:     not-started
```
`agent/frontend/server/index.ts` maps kinds to statuses (400 for an invalid body, 429 for an exhausted budget, 502 for a relay failure), and no test pins that mapping: a future edit could answer 500 for an exhausted budget with the gate green. The file starts `Bun.serve` and cannot be imported under `node --test`, so the test needs the bun runner T9.13 adds to CI.

**Done when:** a test runs the mapping under bun and fails if any kind answers a different status; the gate runs it where bun exists; the gate passes.

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
status:     not-started
```
T9.13 made CI's `bun-version: '1.3.14'` the repository's first toolchain pin with no second source: `tools/pincheck` compares Go and Node only, and the `oven/bun:1` image tags float past it. Extend the pin check to read the Dockerfiles' `oven/bun:` build tags and CI's `bun-version:` and compare them by major, as Node is compared. Also correct `tools/check.sh`'s comment and fallback text, which still say CI installs neither bun nor the package's modules.

**Done when:** the pin check covers bun and fails on a major disagreement or a missing pin; the gate's prose matches what CI does; the table test proves the new failure mode; the gate passes.

---

## Exit Criteria

- [x] A seeded population where the demo oxford is Earned honestly, and other lines are Building or Out of true (T9.1).
- [x] The ledger shows the deciding step and the pool, with no other shopper's rows (T9.2).
- [x] The merchant app is built and deployed, reading only `product_fit` (T9.3, T9.3d).
- [x] The reference agent never refuses on evidence and records standalone corrections (T9.4).
- [x] A second agent honours a correction it never saw (T9.5).
- [ ] The pivoted layer is live, and the demo runs end to end (T9.6, T9.7).
