# P7: Pivot contracts

The shapes the pivot builds on: recall with the precedence order and a status label on every item, corrections that stand on their own, pooled product fit, the fit row, and the keys that name a shopper. P7 is short and exact: P8 to P10 code against it. A change to a shared type after P7 updates its fixtures in the same commit, and is the orchestrator's call.

Read first: [`../product.md`](../product.md) in full (the pivot is decided there: "Precedence", "Lambo holds the shopper; Elastic holds the pool", the pool status table), then `research/lambo-multi.md` and `research/elastic-fit.md`.

---

### T7.1: Pivot contracts ★
```yaml
requires:   [T0.11, T0.12]
fixture-ok: yes
size:       M · frontier
owns:       internal/api/contract/, internal/domain/, docs/interface.md, docs/architecture.md, testdata/api/
status:     done
```
Change the frozen contract to the pivot in product.md, and update every fixture in `testdata/api/` in the same commit.

- **`recall_for_purchase`** never refuses. Each item it returns carries its status label (Claimed, Building, Earned, Out of true). The response names the **precedence step** that decided it: correction, own evidence, pool, statement, or nothing. When steps disagree it returns both and marks the lower one overridden, never hidden. `Guidance.Basis` grows to match the five steps. The correction-first rule stays as it is: an overriding correction ships at the top level and inside each contradicted belief.
- **`record_correction`**: `ContradictsBeliefID` becomes optional. With no belief, the correction is a standing record. Describe in `docs/interface.md` how it is later declared `against` a matching belief.
- **`product_fit`**, new: for a product line and size, the pooled status, counts by direction and by feedback verdict, the settled denominator, distinct shoppers by direction, and the seeded split. No shopper identity, no row, no free text.
- **`set_pooling`**, new: the shopper's opt-out or opt back in (pooling is opt-out by default, product.md). The response says how many existing rows were removed.
- **The fit row** as a domain type: product line (brand, category, fit handle), size, purchase state, reason category, fit direction, per-line shopper hash, transaction hash, seeded mark, event times. The state rank used as the version, with event time breaking ties (product.md). Use T0.12's mapping as recorded.
- **The pool status rule** as a pure, table-tested function over counts: Building at 3 distinct shoppers, Earned at 8 or more in one direction with a 60% share and at least twice the opposite, Out of true when the last 90 days alone fail it. The thresholds are constants with one definition, named in `docs/interface.md`.
- **Usual size**: the rule from product.md (most often kept or fits-well, in any brand, else Claimed), as a pure function with its own table test.

`docs/architecture.md` gains the second write path (ingest to the fit index), the per-shopper Lambo hub, and the merchant app reading only `product_fit`.

**Done when:** every new and changed example in `testdata/api/` validates against its schema and its Go type, with unknown fields rejected. The status rule's table test covers each boundary (2 vs 3 shoppers, 7 vs 8, 59% vs 60%, exactly twice, a 90-day flip). The precedence enum, the fit row and the usual-size rule each have a table test. The gate passes.

**Notes (in-review).** `internal/domain/` gains three pure pieces, each with its own table test: the fit row (`ProductLine`, `FitPurchaseState` with `Rank`, `FitSettledKind`, `FitVerdict`, `FitRow.Version()`, which is the state rank with event time inside it), the pool rule (`PoolCounts`, `DecidePool`, and the five thresholds `PoolBuildingShoppers`, `PoolEarnedShoppers`, `PoolEarnedSharePercent`, `PoolEarnedRatio`, `PoolRecentWindow`) and the usual-size rule (`UsualSizeFor`). `internal/api/contract/` turns `GuidanceBasis` into the five precedence steps, adds `PrecedenceView` to every belief and to the pool, adds `PoolView` with the counts shapes, `product_fit` and `set_pooling` (eleven tools), and makes the correction's belief optional. `testdata/api/` has twelve new examples, four new schemas and five changed schemas; the document carries all thirty-one validating blocks.

Four things the next agent should not have to rediscover:

- The served layer's tests compare its JSON to the base fixtures, and the server builds `RecordCorrectionResponse` with plain string fields. So every pivot field is optional (`omitempty`), the base `recall_for_purchase.response.example.json` stays the pre-pool shape, and the pivot examples are extra files. T8.5 switches the served layer to emit them; making them required in the schema is a contract change after P7, and the orchestrator's call.
- The 60% share counts every fit-classified settled outcome, fits-well included. On directional outcomes alone the "twice the opposite" rule already implies a two-thirds share, so the 60% floor would never bind; it bites when fits-well verdicts dilute a direction. The fixture pool is 61.7% and Earned for exactly that reason.
- Below the Building floor the pool reports Claimed, the ledger's only word for a belief with no evidence behind it. The recent window is judged only at or above the floor: read literally, a quiet line would flip to Out of true with nothing newer to disagree. `DecidePool`'s table test pins both.
- T0.12's event-sequence version is superseded by the state rank plus event time, so the row carries the times and computes the version; `event_seq` is not a field. The domain's direction words are `small`/`large` while the index stores `too_small`/`too_large`; the writer maps them, and `docs/architecture.md` says so.

Open for T9.3: `product_fit` answers for one line and size. The merchant app needs the seller's line list from somewhere (its own config or a later read), since the contract does not enumerate lines. Full gate result in the report.

---

### T7.2: Shopper and purchase keys
```yaml
requires:   [T7.1]
fixture-ok: yes
size:       S · mid
owns:       internal/identity/, .env.example, tools/env.sh
status:     claimed:t72-identity-keys
```
One place that turns PayPal identifiers into the keys the layer stores:

- **Session name:** `shopper-<hex>` from a salted hash of the payer ID, matching Lambo's addressed-name rule (1 to 128 bytes of `[A-Za-z0-9._:-]`, `research/lambo-multi.md`).
- **Per-line shopper hash:** salted over (payer ID, product line), so it counts distinct shoppers within a line and can't link one shopper across lines.
- **Transaction hash:** the construction T0.12 pinned (`sha256("txn|" + transaction id)`), so the row id is stable.

The salt is a key: it comes from the keyring through `tools/env.sh` and is never written to a file. The entry is `service identity account memento-emptor kind salt`, read as `MEMENTO_IDENTITY_SALT`; it holds 32 random bytes as 64 lowercase hex characters. It is named in `.env.example` by name only and in the handoff log. Seeded rows use a fixed, published seed salt marked as seeded, so the population replays byte for byte; live rows use the keyring salt. Generate it once: rotating it changes every session name and per-line hash, orphaning existing sessions and breaking opt-out's recompute.

**Done when:** table tests pin every construction, including a payer whose session name is exactly at the length limit. A test shows the per-line hash differs across lines for one payer. No salt value appears in any committed file. The gate passes.

---

### T7.3: Consent for pooling
```yaml
requires:   []
fixture-ok: yes
size:       XS · user
owns:       product.md
status:     done
```
The one pivot decision still open in product.md. What is pooled is settled: reason categories, sizes and counts, with a per-line hash and no identity. The user decides whether the shopper opts in, opts out, or accepts pooling as a term of using the layer. The orchestrator records the decision in product.md. The demo's words (T9.6) and the README (T6.3) follow it.

**Done when:** product.md's "Open decisions" no longer lists consent, and the decision is in the evidence model section.

**Notes.** Decided by the user: opt-out by default. Opting out is recorded in the shopper's own memory, stops new fit rows, and removes existing rows by recomputing each per-line hash from the lines the session knows. `set_pooling` is added to T7.1's contract; T8.2 honours it; T8.5 serves it; T9.2 puts it in the ledger. Also decided: sellers are the second audience, beside the buyer side, which stays first.

### T7.4: Tighten the pivot contract
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       S · mid
owns:       internal/api/contract/, testdata/api/, docs/interface.md, docs/architecture.md
status:     not-started
```
T7.1 could not make the pivot's core required: `internal/api/server` compares its served JSON to the base fixtures and `testdata/replay/` is frozen, so `pool`, `pool_lookup` and per-item `precedence` are optional and a conforming implementation could never return a pool. Once T8.5 serves the pivot, make them required, add a Claimed and a Building `product_fit` example (all four current pool fixtures are Earned), and update the schemas, Go types, tests and docs in the same commit. Do this before the round 2 freeze (T10.3).

**Done when:** the schemas and Go types require the pool and precedence fields; the Claimed and Building examples validate and are pinned; every fixture and doc block is updated; the gate passes; no other contract behaviour changes.

---

## Exit Criteria

- [ ] The pivot's contract, fit row, status rule and usual-size rule are in place with fixtures (T7.1).
- [ ] Every stored key is built in one place, with the salt from the keyring (T7.2).
- [x] Consent for pooling is decided: opt-out (T7.3).
- [ ] The contract requires the pool and per-item precedence once the served layer supports them (T7.4).
