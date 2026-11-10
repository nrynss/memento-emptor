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
status:     not-started
```
A deterministic generator for the shoppers behind the pool, written through the T8.2 row shape with the seeded salt and the seeded mark on every row. It must include the demo brand's oxford with a pooled belief that reaches Earned honestly under the T7.1 rule (product.md's demo step 0), lines that stay Building, and one line that is Out of true. Existing shoppers and their transaction ids stay byte-identical (the T5.7 rule).

**Done when:** a test pins the population's counts, and the T7.1 status rule run over it gives the intended status for every line and size. A second run is byte-identical. Every row carries the seeded mark. The gate passes.

---

### T9.2: The ledger, with precedence and the pool
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       S · mid
owns:       web/src/lib/ledger/, web/src/routes/belief/
status:     not-started
```
The shopper's ledger shows which precedence step decided each recommendation, an overridden step beside the one that won, a standing correction with no belief, and the pooled belief that applies to them as counts with its seeded count. It never shows another shopper's rows. Svelte 5 runes only.

**Done when:** component tests cover each step, an overridden pair, and a standing correction. The pooled row shows counts only. The gate passes.

---

### T9.3: The merchant app ★
```yaml
requires:   [T8.3]
fixture-ok: yes
size:       M · mid
owns:       merchant/, tools/check.sh
status:     not-started
```
A separate, read-only app for the sandbox merchant's product lines. It reads the pool only through the layer's `product_fit`, never Elastic, and holds no Elastic key. One AG Grid table: product line and size, settled purchases, returns by reason category and direction, feedback by verdict, pool status, seeded count. A row opens the "why" for that line and size: the counts behind its status, and the rule it was judged by. The same stack as `web/` (Svelte 5, Chaaya, AG Grid community), with its own package, tests and build, and a step in `tools/check.sh`.

**Done when:** it builds and tests against fixtures from `testdata/api/`. Each status renders with its rule. No component can show a shopper row, and a test proves the app has no Elastic configuration. The gate passes, including the new step.

### T9.3d: The merchant app, deployed
```yaml
requires:   [T9.3, T6.1]
fixture-ok: no
size:       S · mid
owns:       deploy/merchant/
status:     not-started
```
Its own Render service, following T6.1's provision and guard pattern. It reaches the layer's `product_fit` and nothing else. Before anything is created, record the monthly cost of the new service in the brief and ask the user: creating a paid service is the user's call.

**Done when:** the public URL serves the app; the guard reports it; the cost is recorded; the user approved the creation.

---

### T9.4: The reference agent, after the pivot ★
```yaml
requires:   [T8.5]
fixture-ok: no
size:       M · frontier
owns:       agent/runtime/
status:     not-started
```
The reference agent reads the precedence step and states how sure it is from the label. It never refuses when recall returns evidence, and it says when an answer rests on the pool or on a Claimed statement. It records a standalone correction when recall served no belief (the 7-in-9 gap from T5.9). The existing guarantees stay: recall first, approval before any purchase, double-buy protection. Re-run the T3.5 evals and add one per precedence step. Budget: under $1.00. Run one eval first and check its cost.

**Done when:** the evals pass, with traces showing recall first on every turn. The standalone correction is recorded live at least once. Spend is recorded.

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
owns:       deploy/render.yaml, deploy/seed/, deploy/guard.sh, deploy/provision.sh
status:     not-started
```
The hosted service runs the T8.1 hub, the fit index (its live name, not a test prefix) and the seeded population, seeded once as T6.1 does. The Elastic key and the shopper salt reach the service as Render secrets, never in a file. Any change to the monthly cost is recorded and approved by the user first.

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

- [ ] A seeded population where the demo oxford is Earned honestly, and other lines are Building or Out of true (T9.1).
- [ ] The ledger shows the deciding step and the pool, with no other shopper's rows (T9.2).
- [ ] The merchant app is built and deployed, reading only `product_fit` (T9.3, T9.3d).
- [ ] The reference agent never refuses on evidence and records standalone corrections (T9.4).
- [ ] A second agent honours a correction it never saw (T9.5).
- [ ] The pivoted layer is live, and the demo runs end to end (T9.6, T9.7).
