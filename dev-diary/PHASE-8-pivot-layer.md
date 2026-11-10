# P8: The layer, after the pivot

Recall that never refuses, corrections that stand on their own, the fit index written at ingest, the pool read back, and one Lambo process serving every shopper. Everything in P9 and P10 demonstrates this.

Read first: [`../product.md`](../product.md), [`PHASE-7-pivot-contracts.md`](PHASE-7-pivot-contracts.md), `research/lambo-multi.md`, `research/elastic-fit.md`, and the P2 tasks whose packages you change.

---

### T8.1: One Lambo process for every shopper
```yaml
requires:   [T0.11, T7.2]
fixture-ok: yes
size:       S · mid
owns:       deploy/start.sh, deploy/README.md
status:     not-started
```
Move the deploy from one writer for one session (`LAMBO_SESSION`) to the shape T0.11 recommends: one `lambo serve --transport http`, a credential with `session_prefix = "shopper-"` and `create = true`, `max_attached` 16, and sessions named by T7.2. `serve-web` starts with the demo shoppers as its allowlist, until [Lambo issue 241](https://github.com/nrynss/lambo/issues/241) lands. The layer addresses each shopper at `/mcp/s/<session>`.

**Done when:** `start.sh` brings up the hub locally and a test drives two shoppers through it with no cross-read. `deploy/README.md` says how a shopper session is created and how the ledger reader's allowlist is set. The gate passes.

### T8.1b: The Lambo hub, live
```yaml
requires:   [T8.1]
fixture-ok: no
size:       XS · mid
owns:       tests/live/hub/
status:     not-started
```
The T8.1 shape against the live Elastic recall tier: three shoppers created on demand, each recalls only its own concepts, and the ledger reader serves the allowlisted ones. Budget: under $0.20 of embedding spend. Run one shopper first and check its cost. Test sessions are erased afterwards.

**Done when:** the transcript is in the handoff log, with spend and the erase shown.

---

### T8.2: The fit index writer ★
```yaml
requires:   [T7.1, T7.2]
fixture-ok: yes
size:       M · mid
owns:       internal/fitindex/, internal/ingest/
status:     not-started
```
Ingest writes a fit row beside every Lambo write, from the same event: capture creates the row, a refund marks it returned, a classified reason gives it a category and direction, fit feedback and kept-past-the-window settle it, and a contested reason updates it. Each write replaces the whole document at the state-rank version with `external_gte` (T0.12: the partial `_update` API refuses external versioning). The index is created from T7.1's mapping. Only the layer's interface to the index is named in domain code (developer rule 6).

**Done when:** a fixture test drives one purchase through every state, in order and shuffled, and ends at the same row. A replayed webhook is harmless, and an older state never overwrites a newer one. A refund with no reason is stored but not settled. A shopper's correction writes nothing to the index. The gate passes.

### T8.2b: The fit index writer, live
```yaml
requires:   [T8.2]
fixture-ok: no
size:       XS · mid
owns:       tests/live/fitindex/
status:     not-started
```
One sandbox purchase through capture, refund and reason, written to a test-prefixed index on the live deployment, with a replayed webhook. No model spend. Test indices deleted afterwards, with an empty listing shown.

**Done when:** the transcript is in the handoff log.

---

### T8.3: The pool reader ★
```yaml
requires:   [T7.1, T8.2]
fixture-ok: yes
size:       M · mid
owns:       internal/pool/
status:     not-started
```
`product_fit` over settled rows only, with T0.12's queries: the denominator, fit returns by direction, feedback by verdict, distinct shoppers by direction, and the seeded split. The status comes from T7.1's rule function, never a second copy of it. The connection stays warm, with a 20 s timeout, so a cold TLS handshake (about 6.9 s in T0.12) never lands on a recommendation. When the index can't be reached, the pool says so and recall carries on without it: it never pretends to an empty pool.

**Done when:** fixture tests reproduce T0.12's 600-purchase numbers exactly. The unreachable index is a distinct, tested outcome. The gate passes.

### T8.3b: The pool reader, live
```yaml
requires:   [T8.3]
fixture-ok: no
size:       XS · mid
owns:       tests/live/pool/
status:     not-started
```
A small seeded population in a test-prefixed index, read back through `product_fit`, with warm and cold latency recorded. No model spend. Test indices deleted afterwards.

**Done when:** the transcript and the latencies are in the handoff log.

---

### T8.4: Corrections that stand on their own ★
```yaml
requires:   [T7.1]
fixture-ok: yes
size:       S · mid
owns:       internal/memory/
status:     not-started
```
A correction with no belief is written as a standing record with no `against`, and never promotes. When a matching belief is later written for the same subject, the standing correction is declared `against` it, once. A correction against an existing belief keeps today's path. The belief is never deleted (developer rule 4).

**Done when:** fixture tests cover a correction before any belief, the belief arriving later, a second matching belief not re-declaring, and today's path unchanged. The gate passes.

---

### T8.5: Recall never refuses ★
```yaml
requires:   [T8.1, T8.3, T8.4]
fixture-ok: yes
size:       M · frontier
owns:       internal/api/server/, internal/precedence/
status:     not-started
```
Wire the pivot into the served layer. `recall_for_purchase` reads the shopper's session and the pool, and resolves the five-step precedence order as a pure function in `internal/precedence/`: correction, own evidence, pool (adjusted from the usual size), statement, nothing. Every item carries its label. When steps disagree, both come back with the lower one marked overridden. `record_correction` takes the optional belief. `product_fit` is served. Thin evidence comes back labelled thin, never as a refusal.

**Done when:**
- A table test covers every step deciding, every pair of disagreeing steps, and the unreachable pool.
- Today's story test still passes, with the correction first.
- New story tests pass for: a new shopper decided by the pool; a shopper unlike the crowd decided by their own evidence; a standalone correction honoured, then still in force in a later session.
- The gate passes.

### T8.5b: The layer end to end, live ★
```yaml
requires:   [T8.5, T8.1b, T8.2b, T8.3b]
fixture-ok: no
size:       S · frontier
owns:       tests/live/precedence/
status:     not-started
```
The live layer through the hub and a test-prefixed fit index: one shopper per precedence step, plus a correction made in one MCP session and read in a fresh one. Budget: under $0.50. Run one case first and check its cost. Test sessions and indices removed afterwards.

**Done when:** every step decides at least once live, the cross-session correction holds, and the transcript and spend are in the handoff log.

---

## Exit Criteria

- [ ] One Lambo process serves every shopper, live (T8.1, T8.1b).
- [ ] Every purchase writes and updates its fit row at ingest, live (T8.2, T8.2b).
- [ ] The pool reads back settled-only counts and its status, live (T8.3, T8.3b).
- [ ] A correction stands on its own (T8.4).
- [ ] Recall never refuses, and says which step decided, live (T8.5, T8.5b).
