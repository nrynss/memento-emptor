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
status:     not-started
```
Change the frozen contract to the pivot in product.md, and update every fixture in `testdata/api/` in the same commit.

- **`recall_for_purchase`** never refuses. Each item it returns carries its status label (Claimed, Building, Earned, Out of true). The response names the **precedence step** that decided it: correction, own evidence, pool, statement, or nothing. When steps disagree it returns both and marks the lower one overridden, never hidden. `Guidance.Basis` grows to match the five steps. The correction-first rule stays as it is: an overriding correction ships at the top level and inside each contradicted belief.
- **`record_correction`**: `ContradictsBeliefID` becomes optional. With no belief, the correction is a standing record. Describe in `docs/interface.md` how it is later declared `against` a matching belief.
- **`product_fit`**, new: for a product line and size, the pooled status, counts by direction and by feedback verdict, the settled denominator, distinct shoppers by direction, and the seeded split. No shopper identity, no row, no free text.
- **The fit row** as a domain type: product line (brand, category, fit handle), size, purchase state, reason category, fit direction, per-line shopper hash, transaction hash, seeded mark, event times. The state rank used as the version, with event time breaking ties (product.md). Use T0.12's mapping as recorded.
- **The pool status rule** as a pure, table-tested function over counts: Building at 3 distinct shoppers, Earned at 8 or more in one direction with a 60% share and at least twice the opposite, Out of true when the last 90 days alone fail it. The thresholds are constants with one definition, named in `docs/interface.md`.
- **Usual size**: the rule from product.md (most often kept or fits-well, in any brand, else Claimed), as a pure function with its own table test.

`docs/architecture.md` gains the second write path (ingest to the fit index), the per-shopper Lambo hub, and the merchant app reading only `product_fit`.

**Done when:** every new and changed example in `testdata/api/` validates against its schema and its Go type, with unknown fields rejected. The status rule's table test covers each boundary (2 vs 3 shoppers, 7 vs 8, 59% vs 60%, exactly twice, a 90-day flip). The precedence enum, the fit row and the usual-size rule each have a table test. The gate passes.

---

### T7.2: Shopper and purchase keys
```yaml
requires:   [T7.1]
fixture-ok: yes
size:       S · mid
owns:       internal/identity/, .env.example, tools/env.sh
status:     not-started
```
One place that turns PayPal identifiers into the keys the layer stores:

- **Session name:** `shopper-<hex>` from a salted hash of the payer ID, matching Lambo's addressed-name rule (1 to 128 bytes of `[A-Za-z0-9._:-]`, `research/lambo-multi.md`).
- **Per-line shopper hash:** salted over (payer ID, product line), so it counts distinct shoppers within a line and can't link one shopper across lines.
- **Transaction hash:** the construction T0.12 pinned (`sha256("txn|" + transaction id)`), so the row id is stable.

The salt is a key: it comes from the keyring through `tools/env.sh` and is never written to a file. The keyring entry (service and kind) is named here, in `.env.example` by name only, and in the handoff log. The user creates it. Seeded rows use a fixed, published seed salt marked as seeded, so the population replays byte for byte; live rows use the keyring salt.

**Done when:** table tests pin every construction, including a payer whose session name is exactly at the length limit. A test shows the per-line hash differs across lines for one payer. No salt value appears in any committed file. The gate passes.

---

### T7.3: Consent for pooling
```yaml
requires:   []
fixture-ok: yes
size:       XS · user
owns:       product.md
status:     not-started
```
The one pivot decision still open in product.md. What is pooled is settled: reason categories, sizes and counts, with a per-line hash and no identity. The user decides whether the shopper opts in, opts out, or accepts pooling as a term of using the layer. The orchestrator records the decision in product.md. The demo's words (T9.6) and the README (T6.3) follow it.

**Done when:** product.md's "Open decisions" no longer lists consent, and the decision is in the evidence model section.

---

## Exit Criteria

- [ ] The pivot's contract, fit row, status rule and usual-size rule are in place with fixtures (T7.1).
- [ ] Every stored key is built in one place, with the salt from the keyring (T7.2).
- [ ] Consent for pooling is decided (T7.3).
