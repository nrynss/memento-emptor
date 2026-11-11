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
status:     done
```
Move the deploy from one writer for one session (`LAMBO_SESSION`) to the shape T0.11 recommends: one `lambo serve --transport http`, a credential with `session_prefix = "shopper-"` and `create = true`, `max_attached` 16, and sessions named by T7.2. `serve-web` starts with the demo shoppers as its allowlist, until [Lambo issue 241](https://github.com/nrynss/lambo/issues/241) lands. The layer addresses each shopper at `/mcp/s/<session>`.

**Done when:** `start.sh` brings up the hub locally and a test drives two shoppers through it with no cross-read. `deploy/README.md` says how a shopper session is created and how the ledger reader's allowlist is set. The gate passes.

**Notes (in-review).** `deploy/start.sh` now writes the T0.11 `[serve]` table into the same boot config: one pinned session (the seeded demo shopper), `max_attached = 16`, `attach_concurrency = 1`, `idle_detach_secs = 900`, and one `[[serve.credential]]` (`memento`, `token_env = "LAMBO_MCP_TOKEN"`, `session_prefix = "shopper-"`, `create = true`). The hub starts as `serve --transport http --bind 127.0.0.1 --port $LAMBO_MCP_PORT --agent memento-layer --max-sessions 128 --rate-limit-rps 200`, after `provision` and after the seeder returns, so the short-lived stdio writer still owns its session alone. The reader starts beside the hub with `LAMBO_WEB_SESSIONS` (space separated, default the demo shopper) passed as repeatable `--session`, whether or not the seed held: `serve-web` serves an absent allowlisted session as an empty window rather than refusing to start. `deploy/README.md` gains "The shopper hub", "The ledger reader's allowlist" and "A local hub run".

The demo session is T7.2's name for payer id `shopper_01` under `SeedSaltHex`: `shopper-` plus 64 hex, the exact vector `internal/identity`'s session-name test pins. `LAMBO_SESSION` is gone; `LAMBO_DEMO_SESSION` overrides and is refused unless it starts with the credential's prefix and passes the addressed-name rule. `LAMBO_MCP_TOKEN` is taken from the environment or generated per boot, exported to the children and never written or printed; `LAMBO_MCP_URL` is exported for the layer's graph client (T8.5). `LAMBO_EMBEDDER` (native, bge, fixture, none) and `LAMBO_SEED=0` are the new local knobs.

The local drive (fixture embedder, seed skipped, two shopper sessions created on demand) wrote one unique concept per session: each recalled its own marker and not the other's, and each exact-content `inspect` of the other's belief was "no concept matching". The seeded demo session, served at its T7.2 name through the same hub, held none of their markers. No token and a wrong token were both 401; an unprefixed name and a bare 64-hex hash were both 404. The reader answered 200 for the demo shopper and the uniform 404 for a shopper created after it started.

Five things the next agent should not have to rediscover:

- The fixture embedder cannot take the frozen arc to Venerable: it answers in process, so the arc's seven concepts land in the same second, their sweeps coalesce, and `gc_survived` never reaches 3 inside the seeder's six-minute wait; the seed exits 1 having written its concepts. That is why `LAMBO_SEED=0` exists and why the README's local section says the arc needs the deployed embedder's pace. The deployed native path is unchanged.
- `start.sh` cannot call `internal/identity`, so the demo session name is a pinned constant equal to the identity test vector. If the construction changes, the test vector and this constant move together. T8.1b should assert the deployed demo session's name against the package.
- The hub's `max_attached` counts the pinned demo session, so 15 shoppers attach at once. The 503-plus-`Retry-After` refusal when all 16 are attached is documented in the README but not exercised locally; T8.1b owns recording it.
- `--max-sessions 128` and `--rate-limit-rps 200` come from the T0.11 deploy shape and are still unmeasured on the 0.5 CPU starter; T8.1b owns measuring them. The fixture seed's six-minute timeout is a seeder property; T9.6 owns `deploy/seed/`.
- Lambo's memory is a write-behind log with no on-disk WAL, so a second `SIGTERM` during close loses the unflushed tail. `start.sh` now stops the children once and waits for them before it exits, so a deploy restart drains what is in flight; do not add a second kill path.

### T8.1b: The Lambo hub, live
```yaml
requires:   [T8.1]
fixture-ok: no
size:       XS · mid
owns:       tests/live/hub/
status:     done
```
The T8.1 shape against the live Elastic recall tier: three shoppers created on demand, each recalls only its own concepts, and the ledger reader serves the allowlisted ones. Budget: under $0.20 of embedding spend. Run one shopper first and check its cost. Test sessions are erased afterwards.

Also exercise the 503 + `Retry-After` refusal with all `max_attached` places full, and measure `--max-sessions 128` and `--rate-limit-rps 200` on the 0.5 CPU starter.

**Done when:** the transcript is in the handoff log, with spend and the erase shown.

**Notes (in-review).** `tests/live/hub/` holds the driver (`main.go`), `run.sh`, its README and the `out/` ignore. It boots the 0.10.1 release binary (the deploy's pinned sha) locally in the exact `start.sh` shape — one pinned demo session, `max_attached = 16`, the `shopper-` on-demand credential, `--max-sessions 128 --rate-limit-rps 200` — with the fixture embedder and the live recall tier in test-prefixed indices `t81b-*`, deleted afterwards.

The run passed end to end (transcript in `out/`, kept out of git):

- The prefix listing was empty before the run, and the pinned demo session equals what `internal/identity` builds for `shopper_01` under the seed salt (the 72-byte vector the identity test pins).
- One shopper first: its own marker recalled, spend stated as $0.00 of embedding (in-process vectors, no embedding endpoint called), under the $0.20 budget, then the other two shoppers.
- Each of the three recalls its own marker; all six cross-recalls carry no other session's marker, and every cross-session exact-content inspect is "no concept matching".
- The demo session (pinned, attached, nothing derived in it) recalls none of the test markers and inspects none of the test beliefs; its stats read `concepts=0`. The driver never addresses a deployment path outside `t81b-*`.
- The reader started with the demo plus the three test sessions: own beliefs 200 found=true, a cross-window read 200 found=false, the unscoped alias (the demo, first in the allowlist) 200 found=false for a test belief, `/api/sessions` exactly the four names, and a shopper created after the reader started the uniform 404.
- A second hub with `max_attached = 2` (pinned demo plus one hot shopper, kept hot by a stats loop) refused the newcomer with HTTP 503 in 1.379 ms and `Retry-After="5"`; the hot session still answered.
- 130 concurrent `initialize` with 5 sessions already live: 123x200, 7x503, zero other errors — 123+5 is exactly the 128 bound engaging.
- 600 stats calls over 8 workers sharing 4 live sessions: 600 ok, zero 429s, 98 req/s served on this 12-CPU machine — the 200rps ceiling untouched. The 0.5-CPU starter measurement from the brief stays open: the 128/200 flags and the RSS numbers above are dev-machine numbers, not starter numbers.
- Hub RSS 22.2 MiB at start, 37.2 MiB with 5 attached sessions (the release binary; T0.11's 0.4 MiB marginal was the same order).
- Cleanup deleted the 2 `t81b-*` indices (the data index and the meta index), the listing after is `[]`, and the temp run dir (store, configs, child logs) was removed. No key, URL or token entered the repo: the leak check over `tests/live/hub/` (excluding `out/`) is clean. The deployment URL lives only in the temp hub config under the run dir, which is removed at the end.

Five things the next agent should not have to rediscover:

- The native embedder cannot connect from here (the DNS stall T0.6b records outlasts its 5 s connect timeout), so the live leg is the recall tier with the fixture embedder, as in T0.11's check 4. Embedding spend is $0.00 by construction; the staged one-shopper-first gate still runs and states it.
- `serve-web`'s unscoped alias reads the first session on its allowlist, not the hub's pinned default: a reader started with the test sessions first serves a test belief at `/api/inspect`. The driver lists the demo first, mirroring `start.sh`'s allowlist, so the alias is the demo-is-default check. The first draft asserted found=false with the test sessions first and failed exactly this way.
- The rate burst must reuse live MCP sessions: opening one session per worker spends the 128 session ceiling (the first draft's 600-call probe died with 450 session-503s, zero 429s) instead of measuring the rate budget. The driver shares four sessions across eight workers under a client lock.
- The 503 needs every attached session non-idle: the probe keeps the one place hot with a stats loop while the newcomer attaches, otherwise the hub evicts instead of refusing. `max_attached = 2` keeps the probe to one hot session.
- `lambo_stats` with a receipt answers the outcome on the same call (wait_ms lets the server wait); a derive of two concepts returns one receipt per concept and the driver waits each. The hub and reader logs are scanned for `127.0.0.1:<port>`; `serve-web` logs its URL in the same shape, so one scanner starts both.

---

### T8.2: The fit index writer ★
```yaml
requires:   [T7.1, T7.2]
fixture-ok: yes
size:       M · mid
owns:       internal/fitindex/, internal/ingest/
status:     done
```
Ingest writes a fit row beside every Lambo write, from the same event: capture creates the row, a refund marks it returned, a classified reason gives it a category and direction, fit feedback and kept-past-the-window settle it, and a contested reason updates it. Each write replaces the whole document at the state-rank version with `external_gte` (T0.12: the partial `_update` API refuses external versioning). The index is created from T7.1's mapping. Only the layer's interface to the index is named in domain code (developer rule 6).

**Done when:** a fixture test drives one purchase through every state, in order and shuffled, and ends at the same row. A replayed webhook is harmless, and an older state never overwrites a newer one. A refund with no reason is stored but not settled. A shopper's correction writes nothing to the index. An opted-out shopper's events write no row, and opting out deletes their existing rows across every line their session knows, while leaving other shoppers' rows alone. The gate passes.

**Notes (in-review).** `internal/fitindex/` is the only writer of pooled rows. `Index` is the layer's whole interface to the index (`Create`, `Put`, `DeleteShopper`); `Writer` is one shopper's writer, built with the identity keyer, that shopper's id and a `Lines` resolver, and it holds each purchase's facts for the life of the process so it can replace the whole document every time. The transitions are `Captured`, `Refunded`, `Classified`, `Feedback`, `Kept` and `Contested`; `Record(domain.Evidence)` is what ingest calls, and it maps a payment capture, refund or kept outcome onto them. A refund's evidence never carries a reason through ingest, so `Record` stores it returned and unsettled: a caller holding an already-classified refund, as T9.1's seed rows do, must call `Refunded` and then `Classified`, or the row never settles. `Forget(lines)` recomputes the per-line hash for the shopper's known lines and deletes those rows; `WithPooling` is the consent check that stops new rows. `Mapping()` carries the recorded mapping's fields and types, whitespace aside, pinned against `testdata/elastic-fit/mapping.json`; `HTTPIndex` creates the index, writes `<index>/_doc/<txn_hash>?version=<row.Version()>&version_type=external_gte&refresh=true`, maps a 409 to `ErrOlderEvent` and deletes by query with the three line fields and the shopper hash. Ingest gained one seam: `WithFitRecorder` takes a `FitRecorder` and every capture and refund delivery records its fit row before storing the evidence, so a failed fit write fails the delivery and the retry writes both. A dispute writes no fit row.

Five things the next agent should not have to rediscover:

- `domain.Item` has no fit handle, so the writer asks the catalog through `Lines` rather than guessing one. `agent/catalog` is where a production resolver belongs; the tests use a function. This is a new seam, not a contract change: `domain.ProductLine` and the fit row are unchanged.
- The reason category is written as `domain.Category` names it (`damaged_or_defective`, `not_arrived`). The recorded population's `damaged` and `not_as_described` were the harness's synthetic labels, and T0.12 says the classifier's categories slot in. Only the direction is mapped (`small`/`large` to `too_small`/`too_large`).
- The document has no `event_seq`: T7.1's state-rank version supersedes T0.12's sequence, and the recorded mapping keeps the unused field. `refresh=true` is on each write so a read after a purchase sees its row; the index is one document per purchase, not a bulk path.
- The transaction id is the capture id, with the order id as the fallback when the link carries none; the same value on capture and refund. The two ids hash to different rows, so a purchase whose events disagree gets two. A multi-item order shares one capture id, so it would share one row, and an item in a second line would overwrite the first. The demo and the seed buy one item per purchase, and T9.1's population and T10.3's generated orders must keep it that way. If multi-item orders arrive, the row key needs a decision, not a workaround here.
- A reason of `unclear` or `none` is stored and leaves the row unsettled: it is not a cause, so the refund stays a pending question. A contest replaces the category and direction in place and keeps the first `settled_at` while the row stays settled; a correction that leaves no cause clears it. The event time still breaks ties inside the last rank.

**Review fixes (in-review).** Three defects the review found are fixed inside this package, each with a test that fails on the old behaviour. `HTTPIndex.DeleteShopper` reads `_delete_by_query`'s `failures` and returns `ErrIncompleteDelete` with the count that did go, so a partial opt-out cannot report success (`TestDeleteShopperReportsPartialFailures`). `Writer.record` takes the writer's lock before the consent check and holds it through the index write, and `Forget` holds the same lock across its deletes, so a write that passed the check cannot land behind the opt-out (`TestForgetWaitsForAWriteThatPassedTheConsentCheck` blocks a write inside the index call, runs Forget, releases the write and asserts the row is deleted and nothing remains). A contest that leaves the row unsettled clears `SettledAt` and `SettledKind` in the merged row, not only in the document (`TestContestWithNoCauseUnsettles` pins the zero time).

The writer is not wired into the served layer yet: `internal/api/server` builds its ingest service per shopper and owns `record_reason`, `record_feedback`, `record_correction` and `set_pooling`, and this task does not own that package. T8.5 passes the shopper's identity and a line resolver when it builds the writer, resolves the same line for the explicit `Classified`/`Feedback`/`Contested` calls (only `Record` asks the resolver), and calls `Forget` with the lines the shopper's session knows after recording the opt-out. T9.1 builds the seeded population through the same writer with `Seeded` set and the published seed salt; T8.2b uses `NewHTTPIndex` and `Create` against the live deployment.

### T8.2b: The fit index writer, live
```yaml
requires:   [T8.2]
fixture-ok: no
size:       XS · mid
owns:       tests/live/fitindex/
status:     done
```
One sandbox purchase through capture, refund and reason, written to a test-prefixed index on the live deployment, with a replayed webhook. No model spend. Test indices deleted afterwards, with an empty listing shown.

**Done when:** the transcript is in the handoff log.

**Notes (in-review).** `tests/live/fitindex/` holds the driver (`main.go`), `run.sh`, its README and the `out/` ignore. It creates `t82b-fit-rows` through `fitindex.NewHTTPIndex` and `Create` against the deployment T0.12 recorded, drives one recorded sandbox purchase through `ingest.Service` with `WithFitRecorder`, and deletes the index in a deferred cleanup before printing the empty `t82b-` listing. The purchase is the T0.4 cycle that is recorded end to end: the order with its line item, its capture and its refund (the refund's `up` link is that capture). The capture creates the row, the refund stores it returned and unsettled, and the shopper's answer settles it; the answer is the T0.7 one, replayed through `fakes.NewRecordingReasonClassifier`, so the reason is recorded rather than classified live. Only the index is live; webhook verification is T2.1's and T8.7's leg and this driver replays the recorded deliveries.

The run showed one document per purchase through every state: `captured` at rank 1, `refunded` at rank 2 (stored, unsettled, both timestamps), `reason_classified` at rank 3 (settled by `return_reason`, `fit_too_small`, `too_small`), one document for the purchase, for the shopper's line and in the index. A replayed delivery was a duplicate and wrote nothing, twice: before and after the reason. Retrying the fit write with the refund event and with the capture event rewrote the same version and source, and a fresh writer replaying the capture got the index's `409 version_conflict_engine_exception` and treated it as done, so a restarted process cannot undo a newer state. The index was deleted (`{"acknowledged":true}`) and `_cat/indices/t82b-*` was `[]`. No model spend; one kilobyte-scale index, six writes, reads, counts and the delete.

Four things the next agent should not have to rediscover:

- The recorded T0.4 cycle with line items is the first one: its order read carries the item and its refund's `up` link names its capture. The second cycle's recorded order read came back `PAYER_ACTION_REQUIRED`, so it carries no line items and does not show the purchase end to end; its capture and refund are recorded and pair the same way.
- `fitindex.HTTPIndex` exposes `Create`, `Put` and `DeleteShopper` and nothing else, so the driver carries its own small read/delete probe (document read, `_count`, `_cat/indices`, `DELETE` index). T8.3b's pool reader will need the same shape for its empty-listing check.
- The writer swallows `ErrOlderEvent` by design, so a fresh-process replay leaves no visible status. To show the refusal in a transcript, write the captured rank directly with the probe: the deployment's `409` names the current version, which is the refusal the writer received.
- The driver's versions are deterministic because the capture and refund times are the recording's and the answer's time is fixed twenty minutes after the refund, so a rerun reproduces the same three version numbers and the same document.

The dev-diary report's transcript is the handoff log entry; it is not committed here.

**Review (fresh eyes).** The review re-derived the three versions from the tree — rank × 1e12 plus the event's Unix seconds, the capture's and the refund's read from `capture-completed.json` and `refund-record.json` and the answer's placed twenty minutes past the refund — giving `1001794233521`, `2001794233529` and `3001794234729`, and read the run against them and against the row content, so the evidence is the tree's and not the report's word. Two inaccuracies were fixed in the notes, the first also in the driver's README: the answer's event time is the driver's own, not the recording's, and the second cycle's capture and refund are recorded and pair the same way, so what that cycle lacks is the order read with line items, not the pair. No change to the driver: its prefix check, its cleanup, its refusal to run over a non-empty prefix and its redaction hold as written, and nothing under `tests/live/` runs in CI or gates a build.

---

### T8.3: The pool reader ★
```yaml
requires:   [T7.1, T8.2]
fixture-ok: yes
size:       M · mid
owns:       internal/pool/
status:     done
```
`product_fit` over settled rows only, with T0.12's queries: the denominator, fit returns by direction, feedback by verdict, distinct shoppers by direction, and the seeded split. The status comes from T7.1's rule function, never a second copy of it. The connection stays warm, with a 20 s timeout, so a cold TLS handshake (about 6.9 s in T0.12) never lands on a recommendation. When the index can't be reached, the pool says so and recall carries on without it: it never pretends to an empty pool.

**Done when:** fixture tests reproduce T0.12's 600-purchase numbers exactly. The unreachable index is a distinct, tested outcome. The gate passes.

**Notes (in-review).** `internal/pool/` is the reader behind `product_fit`. `Reader.Fit(ctx, domain.ProductLine, size)` runs one line-and-size aggregation over settled rows and returns a `Fit`: `domain.DecidePool`'s verdict over the all-time and recent counts, the all-time counts, the seeded and live split, and the recent window. `Counts` carries the settled denominator, kept, returns by direction, feedback by verdict, reasons by the index's category label, and shoppers (fit and per direction). `Index` is the layer's whole interface to the pool's index — `Warm` and `Aggregate(Query)` — and `HTTPIndex` is the adapter: T0.12's recorded query, with its all-settled shopper cardinality replaced by the fit-classified one and a recent-window aggregation beside the split, a keep-alive transport, `DefaultTimeout` at 20 s, and `Warm` for boot. The package holds no threshold and no rule of its own. The fixture test pins the recorded numbers and the boundary they make: 45 small outcomes of 75 fit outcomes is exactly the 60% floor, 31 distinct shoppers point small, and 45 against 18 is more than twice, so the label is Earned/small. A second test warms the reader and reads twice over one TCP connection. Idle connections live for 90 s, so a read after a longer quiet spell pays one handshake; the 20 s timeout is what keeps that read succeeding.

Four things the next agent should not have to rediscover:

- **The Building floor needs a count the recording does not carry.** The recorded `settled.shoppers` counts every settled shopper (88 on the probe line), but `domain.PoolCounts.Shoppers` means fit-classified shoppers (63 there), because a kept purchase or a damage return is not sizing evidence. The query adds a `fit_shoppers` filter and cardinality: a settled return is fit-classified when its `reason_category` is one of the two fit categories, and every feedback row is. The fixture test derives 63 all, 55 seeded and 8 live from the recorded settled-shopper, non-fit-return and kept counts, then injects them into the answer it serves; the recorded population gives each shopper one kind of settled outcome, so that subtraction is exact.
- **The recent window is its own top-level aggregation.** A `filters` aggregation's sub-aggregations apply to all its buckets, and the ledger's recent count is seeded and live together, so `recent` sits beside `split` and repeats the line filter with `settled: true` and `settled_at >= now - domain.PoolRecentWindow`. It ranges on `settled_at`, the event that settled the row, not on `captured_at`.
- **The failure outcomes are distinct.** `ErrUnavailable` is a refused connection or the per-request timeout; `ErrIndexStatus` is a non-200 answer; `ErrIncomplete` is a `timed_out` answer or a failed shard; `ErrMalformedResponse` is an answer that does not decode, or that is missing a bucket the query asked for. A valid empty pool is none of them: it is Claimed with zero counts and no error. T8.5 maps `ErrUnavailable` to `contract.PoolUnavailable` and must not report an empty pool instead; it decides how to treat the other three, which are equally not counts.
- **Reasons come back as the index spells them.** `Counts.Reasons` is a `map[domain.Category]int` that keeps a category this build does not name, because the recorded population says `damaged` and `not_as_described` where the domain says `damaged_or_defective` and `not_arrived`. T8.5 maps them to `contract.ReasonCountsView`; that is a mapping seam, not a contract change. Reads are against the fixtures and a fake index only; no live deployment was called.

### T8.3b: The pool reader, live
```yaml
requires:   [T8.3]
fixture-ok: no
size:       XS · mid
owns:       tests/live/pool/
status:     done
```
A small seeded population in a test-prefixed index, read back through `product_fit`, with warm and cold latency recorded. No model spend. Test indices deleted afterwards.

**Done when:** the transcript and the latencies are in the handoff log.

**Notes (in-review).** `tests/live/pool/` holds the driver (`main.go`, `run.sh`, its README and the `out/` ignore). It creates `t83b-fit-rows` through `fitindex.NewHTTPIndex` and `Create`, writes eight seeded rows on the demo line in size M through one fit writer per shopper (five `Refunded`+`Classified` small, one large, one `Feedback` fits-well, one `Kept`), and reads them back through `pool.NewReader(...).Fit`, the `product_fit` path. Cleanup deletes the index in a deferred function before printing the empty `t83b-` listing.

The run read the pool as Building toward small: eight settled rows, kept 1, returns small 5 large 1, fits-well feedback 1, reasons `fit_too_small` 5 and `fit_too_large` 1, shoppers fit 7 small 5 large 1, seeded 8, live 0, recent 8. The cold read on a fresh transport took 6.863 s and the warm connection's `Warm` took 6.724 s; three warm reads took 239–240 ms. An empty size read as a valid empty pool (Claimed, zero counts, no error). No model spend; one kilobyte-scale index, 22 row writes, reads, counts and the delete.

Three things the next agent should not have to rediscover:

- The kept row settles the denominator but joins no fit-shopper count: `Shoppers.Fit` is 7 of 8, because the reader's `fit_shoppers` filter counts settled returns with a fit category plus settled feedback only. The driver's first run asserted 8 and the live deployment answered 7; the fix is in the driver's `wantCounts`, not the reader.
- The warm reader's `Warm` is itself a cold handshake on a fresh transport (6.724 s here), so the transcript's warm reads are only fast because `Warm` paid first. A layer that skips `Warm` pays the same ~6.9 s on its first recommendation read; the 20 s timeout is what keeps that read succeeding.
- `pool.HTTPIndex` exposes `Warm` and `Aggregate` only, so the driver carries its own small read/delete probe (prefix listing, `_count`, `DELETE`) for the empty-listing check, the same shape T8.2b's driver uses.

The dev-diary report's transcript is the handoff log entry; it is not committed here.

---

### T8.3c: Union per-direction shoppers
```yaml
requires:   [T8.3]
fixture-ok: yes
size:       S · mid
owns:       internal/pool/
status:     done
```
T8.3's per-direction shopper counts cover fit *returns* only, so feedback-only direction evidence never meets the rule's eight-shopper floor: 10 shoppers whose only small evidence is `feedback: too_small` read Building where the documented rule says Earned. Either add a second per-direction filter+cardinality that unions fit returns with fit feedback verdicts (and decide what the recorded fixture then pins), or narrow `domain.PoolCounts`/`ShopperCountsView` to returns-only — the latter is a contract change and the orchestrator's call.

**Done when:** the per-direction shopper counts match the documented rule, whichever resolution is chosen; the fixture pins the new numbers; the gate passes.

**Notes (in-review).** Chose resolution (a): the union filter, no contract change. `domain.PoolCounts` already documents each direction as returns *and* feedback (`TooSmall counts settled outcomes pointing small: a return whose reason is that direction, or fit feedback with that verdict`), and the T7.1 table test pins outcome-level union the same way, so narrowing to returns-only would have contradicted the documented rule and needed the orchestrator's call under AGENTS.md rule 1. The reader was the only place that disagreed: its per-direction shopper counts came from the `fit_returns` direction buckets, which see returns only.

`settledAggs` gains a `direction_shoppers` filters aggregation with one bucket per direction. Each bucket's filter is `fit return in that direction OR fit feedback carrying that verdict` (`minimum_should_match: 1`), with a single `shopper_hash` cardinality above it, so a shopper with both kinds of evidence counts once. `countsIn` now reads `Shoppers.TooSmall/TooLarge` from those union buckets and refuses an answer missing them; the `fit_returns` direction buckets keep their doc counts but lost their nested shopper cardinality, which was the source of the bug and is no longer read. `Reader.Fit`, `Result`, `Counts` and every contract shape are unchanged.

The fixture pins the new numbers. The T0.12 recording predates the union bucket, so the test injects it the way T8.3 already injected `fit_shoppers`. The union is exact, not estimated: the harness (`testdata/elastic-fit/harness.py`) gives every feedback row its own shopper id in a namespace disjoint from the return shoppers (`l1m-fb-...` vs `l1m-small-...`/`l1m-large-...`), so the union is the recorded return-distinct count plus the feedback verdict count. All-time small goes 31 → 36 and large 12 → 15; seeded 27 → 31 and 11 → 14; live small 4 → 5, live large stays 1. The verdict message now reads those numbers (45 small of 75 fit outcomes is the 60% floor, 36 shoppers, 45 against 18 more than twice). The query-shape test pins both union clauses and asserts the old nested cardinality is gone; the error table replaces the direction-missing-shoppers case with a union-bucket-missing-shoppers case; `pool_test` gains the exact bug scenario (10 feedback-only small shoppers earn) as a regression case.

Three things the next agent should not have to rediscover:

- Feedback rows also carry `fit_direction` (the harness sets it from the verdict), but the union's feedback leg filters on `settled_kind = feedback` plus `feedback_verdict`, not on `fit_direction`, so a future row with a verdict and no direction still counts.
- `fit_shoppers` (the Building floor) already unions both settled kinds the same way; the two filters are deliberately parallel.
- Nothing outside `internal/pool/` changed: `domain`, `contract`, `testdata/api` and `docs` are untouched, so T7.4's tightening is unaffected.

---

### T8.4: Corrections that stand on their own ★
```yaml
requires:   [T7.1]
fixture-ok: yes
size:       S · mid
owns:       internal/memory/
status:     done
```
A correction with no belief is written as a standing record with no `against`, and never promotes. When a matching belief is later written for the same subject, the standing correction is declared `against` it, once. A correction against an existing belief keeps today's path. The belief is never deleted (developer rule 4).

**Done when:** fixture tests cover a correction before any belief, the belief arriving later, a second matching belief not re-declaring, and today's path unchanged. The gate passes.

**Notes (in-review).** `internal/memory/correction.go` adds the standing path beside the unchanged `RecordCorrection`: `Correction` (words, `domain.Subject`, `domain.ProductLine`, size, event time, `Against`), `Belief` (content, kind, subject, line), `StandingCorrections` with `Record`, `Declare`, `Len` and `Standing`, and `EnsureBeliefDeclaring`, which derives a belief and then declares every matching standing correction against it. `Record` derives the words as a note (`ConceptNote`), so the record has no structural edge, no score and no canonization contribution; `Declare` writes one structural action per correction with the belief in `against` and nothing in `depends_on`, so the belief keeps its standing, dated by the correction's own event time. `Declare` skips every correction that already carries an `Against`, so a second matching belief declares nothing; a later correction is its own journal entry and never revives an earlier declaration; the same words about the same subject, line and size stay one entry, standing or declared. `internal/memory/subject.go` holds the match in one place, `SameSubject`: both sides must name the same subject kind and id, a fit belief must be about a brand and a seller belief about a seller, and the product line compares field by field only where both sides set it, so a side naming no line spans every line of its subject. The size is deliberately not a match key: a correction and a belief naming different sizes are exactly the pair a correction contradicts, so requiring equal sizes would never declare the disagreement. `internal/memory/correction_test.go` covers the correction before any belief, the belief arriving later and declared once, a second matching belief not re-declaring, a later correction not reviving the first, today's path unchanged (three returns, `Out of true` over `Venerable`, three dependents kept), a standing correction declared against an already-earned belief, the match table, and the refusal and repeat cases.

Three things the next agent should not have to rediscover:

- The journal is the caller's per-shopper state, not graph state. `memory.Graph` reads one concept by its exact content (`Inspect`) but cannot enumerate a session's concepts, and a correction's subject, line and size live only in the journal, never in the graph, so a process restart loses the ability to declare. T8.5 owns the per-shopper state that will hold it; rebuilding the journal from a shopper session needs an enumeration and structured fields the seam does not offer. The issue text is in the task report.
- The domain's write table still maps `SignalCorrection` to `record_action`, and that is right for the declared path; `Declare` checks it before every declaration. The standing record does not go through the table at all, because it cites no transaction and no belief and must earn nothing.
- `tests/` and `internal/api/server/` do not call the new path yet. `RecordCorrection` still refuses an unknown belief id, so the served layer keeps today's behaviour until T8.5 wires the standing branch and `EnsureBeliefDeclaring`.

---

### T8.6: Authenticated callers ★
```yaml
requires:   [T7.1, T7.2]
fixture-ok: yes
size:       M · frontier
owns:       internal/auth/, deploy/edge/
status:     done
```
Today the MCP endpoint has no authentication: anyone who reaches the URL can read any shopper's memory by `shopper_id`. product.md ("How it is consumed") requires every caller to be authenticated before anything with real shoppers goes live.

- **Agents:** a key per agent, issued by the seller running the layer, sent as a bearer token. A key is bound to that seller's shoppers; a call for any other `shopper_id` gets the same refusal as an unknown key, so the endpoint can't be used to find out who exists. Keys are stored hashed, can be revoked, and come from the keyring or Render secrets, never a file.
- **Shoppers:** the ledger reads and `set_pooling` act only for the signed-in shopper. For the hackathon, sign-in can be a per-shopper link token issued by the seller (the demo shopper's is a sandbox credential, safe to publish on the judges' page). Record the choice and its limits.
- **Merchant app:** its own key, good only for `product_fit`.
- **Webhooks:** unchanged. Verified PayPal deliveries only, as T2.1 built.

The middleware sits in `internal/auth/` and in front of every route at the edge. The tools' contract doesn't change: auth travels in headers, not in tool arguments.

**Done when:** table tests cover each caller kind with a good key, a wrong key, a revoked key, and a good key asking for another seller's shopper, with the refusals identical where they must be. A test proves no route but `/healthz` and the webhook answers without a key. The gate passes.

**Notes (in-review).** `internal/auth/` owns the whole check. `Keyring` is built once at boot from three environment variables — `MEMENTO_AGENT_KEYS`, `MEMENTO_SHOPPER_TOKENS`, `MEMENTO_MERCHANT_KEYS` — each holding whitespace-separated entries of `id|sha256|seller|shoppers|flags`, where flags is `-` or `revoked`. The entry holds the **digest** of a token, never the token: `IssueKey` mints a token (kind prefix plus 32 random bytes as unpadded base64url), returns it once with the entry line, and the deployment keeps only the entry. `HashToken` renders a digest for an entry, `NewToken` mints without a binding, and every token starts `memento_agent_`, `memento_shop_` or `memento_merchant_`. Revocation is the `revoked` flag (or dropping the entry) plus a restart, because the ring is read at boot. A missing variable is not an error — that kind then refuses everyone — and a malformed entry stops the edge from starting, because a typo must not widen or narrow access. `Authenticate` hashes the presented token and compares every entry in constant time without stopping the scan.

`Guard` is the middleware. `Surface()` lists the edge's routes and `newMux` in `deploy/edge/` mounts the guard on every one, so the route table cannot drift from what is gated: `GET /healthz` and `POST /webhooks/paypal` (and the layer's own `/webhooks/payment`) are public, and `/mcp`, `/tools/{tool}` and `GET /calls` need a key. The guard authenticates before it reads a body, then reads the tool (`/tools/{tool}` from the path, `/mcp` from `params.name`) and the shopper (`shopper_id`, at the top level of a tool body or of `params.arguments`), puts the body back byte for byte, and applies the table in `surface.go`. Every shopper-scoped tool admits an agent bound to that shopper and a shopper token naming themselves; the ledger reads (`ledger_beliefs`, `ledger_belief`, `ledger_evidence`) are shopper-only; `product_fit` names no shopper and admits every kind, so the merchant key reaches it while the scoped tools refuse the merchant. `GET /calls` is agent-only: the log carries every shopper's entries and the served layer does not scope it per shopper, so a shopper token must not read it — T9.2 needs a scoped call log from the layer (a T8.5 change) if the ledger's trace panel is to read it. Any failed authentication — no key, unknown key, revoked key, bad scheme, another seller's shopper, a shopper nobody serves, a missing `shopper_id` — answers with one fixed 401 `{"error":{"code":"unauthenticated","message":"unauthorized"}}` and `WWW-Authenticate: Bearer`; a valid key on a tool its kind may not use answers 403 `forbidden`. The guard strips client-sent `X-Memento-*` headers and the `Authorization` header, then sets `X-Memento-Caller-Kind`, `X-Memento-Caller-Id`, `X-Memento-Seller` and, when scoped, `X-Memento-Shopper` for the layer behind it. `X-Memento-Shopper` is set only where the guard checked the caller against that shopper: `product_fit` and an unknown tool send none, whatever the body names, so the layer may read that header as a shopper the caller is allowed to act for. `doc.go` records the shopper sign-in choice and its limits (bearer credential, no expiry, not the payment identity) and the key grammar.

Seven things the next agent should not have to rediscover:

- **The ledger app must send the token.** The SPA is static and public; T9.2 has to read the demo shopper's link token (`<ledger URL>/?t=<token>`) and put it in `Authorization: Bearer` on every `/tools/...` read, plus `record_correction` for the contest path. The demo token is a sandbox credential safe to publish; an agent key and the merchant key are Render secrets.
- **Deployment wiring is outside this task's `owns`.** `deploy/render.yaml` needs the three variables as `sync: false` env vars, `deploy/provision.sh` pushes them beside the five it already sends, `.env.example` should name them by name, and `tools/env.sh` should read them from the keyring; `deploy/start.sh` needs no change because the edge inherits the environment. Nothing invokes `IssueKey` yet — minting a key is a Go call today, so a task that wires the deployment owes a way to mint one without writing code. With no keys configured the edge still starts and refuses every API call, loudly in its log, while `/healthz` stays green: a keyless deploy reads as healthy.
- **The layer behind the edge does not check keys.** T8.5 consumes `X-Memento-*` (or `auth.PrincipalFrom` if it ever mounts in-process) and must not re-read `Authorization`, which the guard removes. Anything reaching the layer's port directly bypasses auth, so the port stays on loopback.
- **`product_fit` carries no shopper, so the merchant key is bound to no shopper.** Its seller scoping is a single-layer assumption: one seller runs one layer. Enumerating a seller's lines is still T9.3's open question.
- **The refusals are tested as bytes, not just statuses.** `TestRefusalsAreIdentical` compares body, status and the whole header set across every refusal, so a future message tweak that distinguishes an unknown key from another seller's shopper fails the gate. `TestForbiddenDoesNotVaryWithTheShopper` does the same for the 403 a merchant key gets, across shopper names, so the kind check cannot become an existence oracle.
- **`TestEveryToolIsClassified` fails when the contract grows.** A new tool must be given a rule in `surface.go`; otherwise the guard would pass it to any authenticated caller.
- **The webhook stays public on purpose, and the layer's verifier is the only gate there.** The guard refuses nobody on that path because T2.1's verification belongs to the payment source. Until that verification is real, a delivery naming any `shopper_id` is accepted if its event id verifies: a known event id can write evidence into any shopper's session, though reading that session still needs a key. The edge cannot turn this into a key requirement without breaking deliveries, so a real signature check is the fix, not a wider guard.

### T8.7: Verified webhook deliveries
```yaml
requires:   [T2.1, T8.2]
fixture-ok: no
size:       M · frontier
owns:       internal/payments/, internal/ingest/
status:     done
```
The webhook path is public by design, and its only gate is T2.1's verifier, which today replays a fixture: a known event id can write evidence into any shopper's session. Build the real adapter against PayPal's `POST /v1/notifications/verify-webhook-signature` with the transmission id, timestamp, signature and raw body, using the sandbox app's credentials. Reject anything that does not verify, keep the delivery idempotency T2.1 built, and record a good, a tampered, a replayed and an unknown delivery. Budget: no model spend; sandbox calls are free.

**Done when:** the four deliveries are recorded with their outcomes, a recorded PayPal delivery verifies live, a tampered body gets the same refusal as any unverified delivery, and the gate passes.

**Notes (in-review).** `internal/payments/` is the real gate. `Delivery{Header, Body}` carries the request headers and the exact body bytes; `Client.Verify` refuses locally when a transmission header or the body is missing, then posts to `/v1/notifications/verify-webhook-signature` and returns nil only for `SUCCESS`. Everything else refuses: `ErrNotVerified` for `FAILURE` or any other verdict, `ErrUnavailable` for a missing credential, a failed token grant, an unreachable endpoint or a non-200 answer, `ErrNoWebhook` when no subscription can be named. The verify request embeds the delivered bytes verbatim — the payload is assembled around them and never re-encoded — because the signature covers exactly those bytes. The access token is cached to `expires_in` less 60 seconds and renewed once when the source rejects a cached token with 401; the candidate subscription ids are listed from `GET /v1/notifications/webhooks` and cached. A configured `PUBLIC_URL` narrows the candidates to its own `/webhooks/paypal` subscription, so a deployment refuses deliveries signed for a stale subscription; without one, every subscription the app holds is tried, and the source answers `SUCCESS` only for the one that signed the delivery. Credentials come from `PAYPAL_CLIENT_ID` and `PAYPAL_CLIENT_SECRET`; no error carries either, and a test asserts it.

`internal/ingest/delivery.go` is the webhook path in the layer. `HandleDelivery(ctx, Delivery)` parses the event id, verifies the delivery, then calls T2.1's `Handle`, so rows and idempotency are unchanged. Verification before the duplicate check matters: a forged body naming a stored event id is refused, not answered as a duplicate. `NewService` installs `payments.New()` as the default gate, so production runs the real adapter with no extra wiring; `WithVerifier` replaces it and every test injects a fake, so the suite stays offline. Every verification failure is one refusal, `ErrBadSignature`, whose text names neither the delivery nor the reason; the cause stays matchable underneath with `errors.Is` (`payments.ErrNotVerified`, `ErrUnavailable`, `ErrNoWebhook`).

Recorded run. `internal/payments/cmd/livecheck` (`source tools/env.sh`, then `bash internal/payments/cmd/livecheck/run.sh`) reads one refund delivery PayPal already posted to the sandbox test endpoint, writes it byte-exact to `out/delivery.json`, re-verifies that recorded copy live, then drives four deliveries through `ingest.Service` with no verifier injected:

| Delivery | Live verdict | Ingest outcome |
|---|---|---|
| good | SUCCESS | one row, pending question |
| tampered (refunded amount changed after signing) | FAILURE | refused, store unchanged |
| replayed (headers and body unchanged) | SUCCESS | duplicate, the same one row |
| unknown (transmission id and signature from no delivery) | FAILURE | refused, store unchanged |

The tampered and unknown refusals are byte-identical: `ingest: delivery failed verification: payments: the delivery did not verify: verification status "FAILURE"`. `internal/ingest/testdata/recorded-deliveries.json` records the same four cases with their live verdicts, and `TestRecordedDeliveries` replays them through a fake verifier, so the gate checks the four outcomes offline. Full values stay in the git-ignored `out/`; stdout is redacted, and the run refuses to finish if a credential survives.

Four things the next agent should not have to rediscover:

- **The deployed webhook route still bypasses the gate.** `internal/api/server`'s `handleWebhook` decodes `{shopper_id, event_id}` and calls `IngestWebhook`; a real PayPal body never reaches `HandleDelivery`. That route must read the raw body bounded, as the other routes are, and pass `ingest.Delivery{Header: r.Header, Body: raw}` to the shopper's service (the delivery's `custom_id` is the shopper join). It is a change in `internal/api/server/`, outside this task's `owns`, so **this is unmet here**: until T8.5 (which owns `internal/api/server/`) lands it, the deployed path keeps T2.1's fixture gate while every wired caller of `HandleDelivery` is verified for real.
- **A redacted recording cannot verify live.** `testdata/paypal/webhook-delivery-*.json` keeps real transmission headers but redacts body ids, and the signature covers the exact bytes, so re-verifying one of those files answers FAILURE. The live run keeps its byte-exact delivery in `out/` and the committed fixture records shapes only.
- **Subscription discovery needs no new configuration, by design.** No environment variable names the subscription (adding one touches `tools/env.sh`, `.env.example` and the deployment, all outside `owns`). The sandbox app still holds its test-endpoint subscription beside any the Render deploy registers; with `PUBLIC_URL` unset on the service both are candidates, which cannot widen what verifies but costs one verify call per candidate. Setting `PUBLIC_URL` on the service is the tighter configuration.
- **The read leg of the live run is the recorded demo refund keyed to the live event id.** Live transaction reads are T2.1b's subject; what this run measures is the gate and idempotency, not the rows' provenance. One surprise from the sandbox store is worth keeping: an old `INVOICING.INVOICE.SENT` delivery that is still stored answers FAILURE because its subscription was replaced, which is direct evidence that a signature is bound to the subscription id, not just the body.

---

### T8.5: Recall never refuses ★
```yaml
requires:   [T8.1, T8.3, T8.3c, T8.4, T8.6]
fixture-ok: yes
size:       M · frontier
owns:       internal/api/server/, internal/precedence/, internal/standing/, cmd/memento/main.go
status:     done
```
Wire the pivot into the served layer. `recall_for_purchase` reads the shopper's session and the pool, and resolves the five-step precedence order as a pure function in `internal/precedence/`: correction, own evidence, pool (adjusted from the usual size), statement, nothing. Every item carries its label. When steps disagree, both come back with the lower one marked overridden. `record_correction` takes the optional belief. `product_fit` and `set_pooling` are served. Thin evidence comes back labelled thin, never as a refusal. Hold one `StandingCorrections` per shopper, serialised (two agents may speak for one shopper), and make the journal survive a restart: a layer-owned store or a Lambo enumeration/structured-attributes mechanism. A restart must not lose the ability to declare a standing correction against a later belief. Decide whether an identical repeated correction refreshes its event time, since precedence keys on recency. The standing branch of `record_correction` accepts an empty belief id, and every belief write calls `EnsureBeliefDeclaring`. Scope `/calls` to the caller's shopper (or document it agent-only). Also:
- resolve a purchase's line from one source for every writer call, so a caller cannot pool a row under the wrong product;
- pass a consistent capture id through the writer, and decide the order-id fallback's two-row hazard;
- hydrate `captured_at` after a restart or document the loss;
- treat `fitindex.ErrIncompleteDelete` as not done and retry `Forget` with the session's full line list;
- add a `memento key issue` command using `auth.IssueKey` that prints the token once;
- keep the MCP decoder strict (single object, unknown fields rejected); if batches are ever accepted, the guard must scope them or refuse.

**Done when:**
- A table test covers every step deciding, every pair of disagreeing steps, and the unreachable pool.
- Today's story test still passes, with the correction first.
- New story tests pass for: a new shopper decided by the pool; a shopper unlike the crowd decided by their own evidence; a standalone correction honoured, then still in force in a later session.
- The gate passes.

**Notes (in-review).** `internal/precedence/` is the pure order: `Decide` over presence (correction, own evidence, pool, statement), the winner plus which present steps ride overridden, with the table test covering every step deciding, every disagreeing pair, the three-way case and the unreachable pool. `internal/standing/` is the layer-owned journal store: one file per shopper with the pooling choice, the correction views and the undeclared journal entries, written atomically after every mutation and restored when the partition is created.

The served layer serves the pivot when the pivot half is wired (pool index, fit index, identity salt, line resolver, standing directory); with them absent it keeps the pre-pool shape, so the frozen base fixtures still describe it. Recall serves fit beliefs at Candidate and above plus general statements, resolves the usual size from kept and fits-well tallies with the statement as fallback, consults the pool at the usual size or the size under consideration, and marks every item with its step. Any index error answers unavailable and never an empty pool. `record_correction` with no belief id records a standing: named under the most recent brand context and journalled, declared against an existing match at once and against a later match when the belief is written; unscoped while the layer knows no brand, applied spanning every brand and never declared. Every belief write goes through `EnsureBeliefDeclaring`. `product_fit` maps the reader's counts with both index spellings of the non-fit reasons folded; `set_pooling` records the choice in the shopper's own memory, calls `Forget` with the session's full line list, and retries an incomplete delete before it reports not done. The deployed webhook route reads the raw body bounded, resolves the shopper from the custom id and passes `ingest.Delivery` to the shopper's service. `/calls` stays agent-only and honours the guard's `X-Memento-Shopper` header; the MCP decoder takes one object with unknown fields rejected. `memento key issue` mints one caller key with `auth.IssueKey`, printing the token once. The nine-tools comments were already eleven on main (sweep d0f71f3), so nothing here renamed them.

Decisions the next agent should not have to re-make:

- **Serving line.** A lone return (Claimed, no promotion) stays on the ledger until a second moves the belief to Building. This keeps today's story test green and matches all four pivot fixtures; read literally, product.md's "at any status" would also serve the lone return. If the product wants that, the story's one-return assertion and the seeded recall comparison move with it.
- **Standing durability.** Beliefs, evidence and views are still in-memory, so a restart forgets them; the standing file keeps the pooling choice, the views and the undeclared journal, which is re-recorded on restore and re-declared as beliefs are rewritten. Declared entries are not re-declared on restore, which avoids duplicate against edges for forgotten beliefs.
- **Repeat correction.** An identical repeat returns the stored correction and never refreshes its event time, in the journal (T8.4's dedup) and on the served path. A later correction with different words is a new entry and wins recency. A repeat under a changed brand context is its own entry, because the subject is part of the identity.
- **Contested rows.** The served `record_correction` never calls the writer's `Contested`: a size correction names no reason category, and misclassifying it into the pool would corrupt counts. Reason contests still arrive through classified answers.
- **Transaction key.** Every writer call resolves the transaction id from the row's own link (capture id, else order id), so both legs of one purchase key the same row whenever the records agree. When they disagree the writer's documented two-row hazard applies.
- **Restart and captured_at.** The writer rebuilds purchase facts from the events it sees after a restart, so `captured_at` is unknown until a capture arrives. Versions still order safely: post-restart events carry newer event times, and a redelivered old event meets the index's newer version and is swallowed as `ErrOlderEvent`.
- **Statement source.** General statements exist only through the seed today: no tool writes one live. The serving rule and the usual-size fallback already honour them when present.

Six things the next agent should not have to rediscover:

- The seeded recall comparison strips the pivot fields and the usual-size statement (pinned separately in `assertRecallPivot`): the base fixture predates the pivot by T7.1's design, and requiring the fields is T7.4's call.
- The guidance texts match the pivot fixture formats: pool "Size up to L ... 29 of the 47 ...", own-evidence with "ahead of the pool" only when the pool disagrees, none verbatim from the example-none fixture. A deciding statement's text is new ("the shopper's usual size").
- `IngestWebhook` (event id, fixture gate) stays for the programmatic path; the route is the verified `IngestDelivery` path. Delivery verification failures map to 400.
- The fake pool counts mirror the fixture numbers, including the `damaged` and `not_as_described` index spellings, which the view folds into the domain's reason fields.
- `state()` now takes the caller's context and fails the call when the standing restore fails, rather than starting empty and silently losing the declare ability.
- No test in this task touches the network: the pool, fit and verifier seams are fakes, and the standing store is memory or a temporary directory.

### T8.5b: The layer end to end, live ★
```yaml
requires:   [T8.5, T8.1b, T8.2b, T8.3b]
fixture-ok: no
size:       S · frontier
owns:       tests/live/precedence/
status:     done
```
The live layer through the hub and a test-prefixed fit index: one shopper per precedence step, plus a correction made in one MCP session and read in a fresh one. Budget: under $0.50. Run one case first and check its cost. Test sessions and indices removed afterwards.

**Done when:** every step decides at least once live, the cross-session correction holds, and the transcript and spend are in the handoff log.

**Notes (in-review).** `tests/live/precedence/` holds the driver (`main.go`), `run.sh`, its README and the `out/` ignore. It boots the 0.10.1 release binary locally in the T8.1 shape (fixture embedder, live recall tier, test-prefixed recall indices `t85b-*`), wires one served layer per shopper over its own hub session plus the live pool reader and fit writer on `t85b-fit-rows`, and decides one purchase per precedence step. The classifier is a recording replay and the payments are the P0 recordings, so the model spend is $0.00 against the $0.50 budget; one case runs first and states it. Test sessions and indices are removed afterwards, with the empty listing shown.

The run passed end to end (transcript in `out/`, kept out of git):

- Correction: a capture names the brand, then a standing correction with no belief is recorded Claimed and uncontradicted, and the next recall decides it (`Take size M for Brand X`). The same correction still decides over a fresh hub MCP session to the same shopper session, and over a rebuilt layer on the same standing directory (the undeclared standing is re-recorded).
- Pool: a new shopper on the seeded Earned-small line (8 small, 1 large, 1 fits-well on brand-x-001/shirts/regular size M) is decided by the pool, with the exact counts, the seeded 10 / live 0 split and the exact guidance text.
- Own evidence: the same line plus three too-large returns earns a live Earned/Venerable belief that beats the now-disagreeing Earned-small pool, which rides along overridden with the `ahead of the pool` text and the seeded 10 / live 3 split.
- Statement: a seeded general statement alone decides (`Take size M for Brand S: the shopper's usual size.`).
- Nothing: a fresh shopper on a lineless recall gets GuidanceNone, never a refusal.
- Cleanup deleted the 3 `t85b-*` indices and the listing after is `[]`; the run dir (store, configs, standing dir, child logs) was removed. No key, URL, salt or token entered the repo: the leak check over `tests/live/precedence/` (excluding `out/`) is clean.

Two behaviours are pinned live with tripwires, both owned by T8.5 (`internal/api/server/` is outside this task's `owns`):

- Consulted-but-empty pool: the frozen `example-none` shape (pool absent, `pool_lookup: consulted`) is what the lookup returns for a wired pool on an empty line, but the served assembler rejects exactly that pair, so a full-line recall on the empty line fails with `pool absent with lookup "consulted"` instead of serving the empty pool. The driver asserts the failure; a success means the guard was fixed.
- Dangling declared views: after a restart the restored view cites a stale belief id and matches no precedence class — mechanically a total loss of force, not the review's "degrades to unscoped" (no class accepts a non-empty stale id). The Earned-small pool decides instead. Rebuilding the belief through a re-ingest declares nothing (the journal dropped the declared entry at persist time); the rebuilt partition replays the same mint order, so the new belief coincidentally reuses the old id and the stale view matches by id, not by content re-linking. Any divergence in mint order would leave the view dangling or attached to an unrelated belief. The driver asserts the post-restart loss of force and the coincidental post-rebuild match.

Five things the next agent should not have to rediscover:

- The served layer builds no fit writer without `IdentitySalt`: the first draft omitted it and every live row silently never wrote (the writer-nil paths skip by design), which the pool's live split caught.
- A recall that should name the brand needs brand context from an ingest or a belief: a lineless recall before any history falls back to the bare brand id in guidance, so the correction shopper ingests its capture before recording.
- The recorded purchases are reused across test shoppers, so their fit rows share document ids in the one test index and later writes overwrite earlier ones. The steps are ordered so no asserted count is affected: the pool step asserts the seeded-only split first, the own-evidence step asserts its seeded/live split next, and the declared shopper (which flips shared rows) asserts no pool counts.
- The driver's graph client reads receipts from `structuredContent` with a text fallback, maps a `no concept matching` inspect to None, and polls recall (not inspect) for promotion: the own-evidence belief reached Earned/Venerable on the live daemon within the first poll window.
- `evidenceCount` spells out only one and two; three renders `3 returns`, which is what the exact own-evidence guidance text asserts.

### T8.5c: Re-link declared views after restart
```yaml
requires:   [T8.5, T8.5b]
fixture-ok: yes
size:       S · mid
owns:       internal/api/server/
status:     done
```
T8.5b pinned two defects live, both needing `internal/api/server/`: a restored declared view matches no precedence class after restart (beliefs get fresh ids, the stale `ContradictsBeliefID` matches nothing), so the correction loses all force and the pool decides; and a wired pool consulted on an empty line returns Claimed with a `(nil, consulted)` lookup the assembler rejects. Re-link declared views by content match on restore, and decide the consulted-empty guard (serve it or refuse it loudly). Tripwires for both live in `tests/live/precedence/`.

**Done when:** a restarted layer re-links a declared correction to the rewritten belief by content; the consulted-empty case is decided and tested; the gate passes.

**Notes (in-review).** Both fixes are in `internal/api/server/`, fixture only, no contract or store change.

- Consulted-empty: serve it. The frozen `example-none` shape is already pool-absent with lookup `consulted`, and the contract suite pins `consulted with an empty pool` — the assembler's rejection contradicted the contract. `assembleRecall` now accepts a nil pool with a `consulted` lookup (unwired/skipped still ride none, unreachable still rides unavailable; a present pool still requires consulted). `TestRecallConsultedEmptyPoolServes` pins it: a wired pool answering Claimed on a full line serves no pool object, lookup consulted, guidance none.
- Re-link: `persistStanding` now journals every entry with its against cleared, plus a synthesized cover entry (words, live belief's subject, size, time) for each declared view with no same-words/same-subject journal entry — direct-against-belief corrections never journalled, so without the cover they could never re-link. `restoreStanding` re-records them as standings, so the rewritten belief re-declares them by subject match. `backfillDeclaration` re-links a restored view by content match on the correction's words when its citation is stale: unknown id, or an id the restart reused for an unrelated belief, detected by asking whether any same-words journal entry names the cited belief's subject. A citation explained that way is left alone, so the same words declared against one subject never cross-link to another subject's belief through the journal check. (Correction on review: across subjects on a divergent rebuild, both views can still collapse onto the first-rebuilt belief — the journal check is per subject, not per view. That cross-subject collapse plus the silent-view and two-belief cases are T8.5d's work, not this task's.)
- `TestDeclaredCorrectionRelinksAfterRestart` (direct path) and `TestStandingDeclaredRelinksAfterRestart` (standing-then-declared path) prove it with a divergent mint order: the restarted layer ingests a dispute first, so the rewritten fit belief cannot reuse its old id, and the correction still decides citing the new id. Both fail on the old code with the defect's exact shape (correction silent, own evidence decides). The gate passes.
- The live tripwires in `tests/live/precedence/` still pin the defects and are outside this task's `owns`, so they are untouched: the next live run will surface both as stale pins (the consulted-empty probe succeeds where it pinned a failure; the dangling view still overrides after restart where it pinned loss of force). Flipping those assertions to pin the fix needs a driver update with live Elastic and the hub binary, which this task cannot run offline.

Review FINDINGS (all owned, none blocking the landed mechanism): (1, High) same words declared against two subjects + divergent rebuild collapses both views onto the first-rebuilt belief while the second recall hard-fails `ErrOverrideMissing`. Needs persisted view↔entry pairing. (2, Medium) a declared view with no rewritten belief goes fully silent in the restart-before-redelivery window. (3, Low) same-subject two-belief collapse. All three plus the server-comparison update for T7.4's tightened fixtures ride in T8.5d; the live tripwire flip rides in T8.5e.

---

### T8.5d: Harden re-linking and serve the tightened shape
```yaml
requires:   [T8.5c, T7.4]
fixture-ok: yes
size:       M · mid
owns:       internal/api/server/, internal/standing/
status:     done
```
T8.5c's re-link works for the direct and standing paths, but the review proved three edge defects live in `internal/api/server/`: same-words/two-subjects cross-link on divergent rebuild (recall hard-fails `ErrOverrideMissing` — the refusal this layer exists to eliminate); silent declared views with no rewritten belief; same-subject two-belief collapse. Persist the view↔entry pairing (or equivalent) so a rebuild re-links exactly. In the same task, update the served-side comparison against T7.4's tightened fixtures (the two failing subtests the T7.4 review pinned).

**Done when:** the three edge cases are tested (including the throwaway reproduction: same words, fit belief + seller belief, divergent rebuild — both views stay attached to their own beliefs and no recall hard-fails); the server comparison passes against the tightened fixtures; the gate passes.

**Notes (in-review).** All in `owns`, fixture only, no contract or schema change.

- Pairing: `internal/standing` gains `Snapshot.Links` with one `ViewLink` per declared view (correction id plus the journal entry's words, subject, line and size; file layout stays version 1, additive). The server keeps the pairing per shopper, persists it with the views, and restores it with them.
- Re-link: `backfillDeclaration` links a rewritten belief only to the views paired to the entry that just declared it, so the same words about two subjects never cross-link however the rebuild reorders the mints. Views from before the pairing fall back to the old words match. `persistStanding` pairs each declared view with its live journal entry (or a cover over the cited belief's subject for a direct correction) and refuses to re-pair a view to a belief whose subject the pairing does not name — a stale id reused for an unrelated belief no longer corrupts the journal or the pairing.
- Dangling window: a restored declared view whose belief is not rewritten yet is honoured by its paired subject like a named standing, so the correction keeps its force across the restart instead of going silent; the guidance recommends from the correction alone with no stale belief id.
- Determinism: a standing recorded while two beliefs share one subject declares against exactly one — most evidence, then earliest, then smallest id — instead of citing by map order while the entry declares elsewhere.
- Tightened comparison: the served layer now reports `pool_lookup: unavailable` (with a null pool) whenever no pool index is wired or no full line is read, which is what the unwired base fixture pins; `server_test.go` and `http_test.go` no longer strip `pool`, `pool_lookup` or `precedence` and compare byte-exact (the usual-size statement filter stays, pinned by `assertRecallPivot`).
- Three tests in `pivot_test.go`: same words over a fit and a seller belief with a divergent id-swapping rebuild (both views stay attached, recall and the seller explain succeed); a declared view deciding with nothing rewritten (correction basis, no belief); a standing over three same-subject beliefs citing the strongest. Each fails with its fix neutered (cross-link, silent none-guidance, weakest citation).
- Finding (not a cut, outside `owns`): 6 web vitest failures (`api.test.ts` 3, `pool.test.ts` 2, `precedence.test.ts` 1) expect the pre-tightened shapes (null precedence, no pool precedence) and fail identically on the base commit without this change — T7.4's web-side fallout, owned by whoever owns `web/`. Every Go package passes, including with `-race`; `gofmt` and `go vet` are clean.

---

### T8.5e: Flip the re-link tripwires live
```yaml
requires:   [T8.5d]
fixture-ok: no
size:       XS · mid
owns:       tests/live/precedence/
status:     done
```
The T8.5b driver pins the defect shapes (loss of force after restart; `pool absent with lookup` rejection). After T8.5d lands, flip both tripwires to pin the fix, live against the hub. Budget: under $0.20 of embedding spend.

**Done when:** both tripwires assert the fixed behaviour live; test indices and sessions erased; spend recorded.

**Notes (in-review).** The driver's two defect pins are flipped to pin the fixes T8.5d landed, and both pass live against the same 0.10.1 hub and Elastic deployment (fixture embedder, recording classifier). The proof-table rows, the tripwire prose and the driver's own header and budget lines are updated with them.

- Consulted-empty: a full-line recall on a line with no settled rows serves the frozen `example-none` shape — pool null, `pool_lookup: consulted`, `GuidanceNone`, no beliefs and no correction — matching `TestRecallConsultedEmptyPoolServes`. The old `pool absent with lookup` refusal pin is gone.
- Declared view: after the restart the correction still decides (same correction id, the stale belief id kept in the payload, `guidance.belief_id` null, the guidance text unchanged, the Earned-small pool riding along overridden by `correction`); after a deliberately divergent rebuild it re-links to the rewritten belief. The rebuild ingests a dispute first, so the seller belief takes `blf_000001` and the fit belief cannot reuse the pre-restart id: live it was `blf_000002`. The driver asserts the new id differs, the rewritten belief carries the warning from the same correction, the override cites the new id, and recall answers instead of returning `ErrOverrideMissing`.
- The seven pre-existing steps still pass unchanged (standing correction, the fresh-session copy, the restart, pool, own evidence, statement, nothing), with the exact guidance texts and seeded/live splits.

Spend: $0.00 of the $0.20 embedding budget (the fixture embedder computes vectors in process; no model call). The one-case-first gate printed its $0.00 line before the remaining steps. Elastic usage was test-prefixed index churn: 3 `t85b-*` indices deleted, the after listing `[]`, and the run dir removed. The hub was the existing 0.10.1 release binary earlier runs kept (it reports `lambo 0.10.1`); the release checksum could not be re-fetched because the machine was offline, so the deploy's pinned sha was not re-verified this run.

**Review (fresh eyes).** Re-ran `run.sh` on this branch against the same cached hub and deployment: it passes, prints its `$0.00` line, deletes 3 `t85b-*` indices and shows the after listing `[]`, with the run dir removed. The cached hub binary's sha256 `b04eeab4…` is byte-identical to the v0.10.1 release asset's published `.sha256`, which is the file `deploy/Dockerfile` fetches and verifies, so the offline caveat above no longer stands and the driver's README now records the sha. Three mutation checks confirm the flipped pins bite: disabling the re-link fails the rebuild recall with `ErrOverrideMissing` on the rewritten belief; restoring the pre-T8.5d guidance rule (a stale belief id in the guidance while its belief is unheld) fails the restart-window assertion; restoring the old assembler guard fails the consulted-empty probe with `pool absent with lookup "consulted"`. One limit: ignoring the persisted pairing in the re-link still passes live, because this single-subject rebuild resolves by the journal entry's words and subject, so the pairing's cross-subject job stays `pivot_test.go`'s proof — the driver README claimed otherwise and is corrected here. The seven pre-existing steps keep their old assertions (this commit changes none of them) and pass. Gate green. One defect in the driver's own guards is fixed here: a refused run (non-empty prefix) still deleted every index under the prefix in its deferred cleanup — the very index the guard refused over — so cleanup now skips the names the pre-run listing found, and `run.sh` runs its listing and leak checks even when the driver fails instead of exiting before them.

**Verdict: APPROVE**, with four findings fixed on the review branch, all owned here: the clean-prefix guard defect (Medium), the overclaimed words-only assertion (Low), the provenance wording (Low) and the skipped run.sh checks (Low). Landed with the task.

---

### T8.5f: Hub client honours the attach bound, with offline shape tests
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       S · mid
owns:       cmd/memento/
status:     done
```
P0's review found two gaps in `cmd/memento/`. First, the hub client does not honour T0.11's attach-bound refusal: `postLocked` turns any non-200 into an error, and `call`'s single retry re-initialises, which cannot relieve a hub whose `max_attached` places are all full — so a burst of more than fifteen concurrent shoppers surfaces errors where the documented behaviour is 503 plus `Retry-After` and a retry after that delay. Second, the MCP reply shapes the client depends on (receipts, admission checks, warnings, blast radius) have no offline test: no test reads `testdata/lambo/`, so shape drift is caught only live.

**Done when:** a refused attach (503 with `Retry-After`) is retried after the stated delay, bounded, and surfaces a typed error when the bound persists, with a table test; a replay test reads `testdata/lambo/01-seed.jsonl` (or the recorded turns) and pins the reply shapes the client parses; the client's existing behaviour is unchanged on the happy path; the gate passes.

**Notes (in-review).** All in `cmd/memento/`, fixture only, no live call and no spend.

- Attach bound: the handshake's `initialize` and every `tools/call` map HTTP 503 to `*attachBoundError`, which wraps `errAttachBound` (so `errors.Is` and `errors.As` both work) and carries the call that met it, the status, an excerpt and the delay. `Retry-After` is read as whole seconds; an absent header, a date, a fraction or a negative count falls back to one second. `call` retries the open and the call after that delay, at most `attachAttempts` (3) attempts, and the last refusal returns the typed error. The wait is bounded by the caller's context. Non-bound failures keep their single re-open retry, and a healthy hub still costs exactly one open and one call.
- Table test (`hub_attach_test.go`): a fake hub answers a canned handshake list and one status for tool calls, and counts what reached it. Cases: healthy (1 open, 1 call, no wait); refused once with `Retry-After: 0` (2 opens, 1 call, 1 zero wait); refused once with `Retry-After: 2` (waits 2s); refused with no header (waits the default); full forever (typed error at 3 opens, 0 calls, so no call slips through without an attach); a refused call retried after its stated delay (3 opens, 3 calls, waits 1s twice); a 500 (not the bound, 1 open); a 500 on the call (the one re-open retry kept, 2 opens and 2 calls). The delay is recorded through `hubGraph.wait`, a field set in the constructor, so the table does not spend the stated seconds; `waitContext` is tested on its own as spending a 20ms delay, spending nothing for zero, and returning the context's error when it ends first. The error type is tested through a wrapping `fmt.Errorf`.
- Mutation checks: deleting the retry fails the five cases that need it plus the context case; deleting either 503 branch fails every bound case; renaming the recorded inspect keys (`blast_radius`, `checks`, `warnings`) fails the inspect cases; reading the write receipt under another key fails all four writer cases; changing the no-match substring fails the error-envelope case. `admission.status` and each of its two switch cases are pinned by the precedence case instead: on the recordings the structured status and the text mark carry the same value, so either source alone covers the outcome and renaming the key goes unnoticed.
- Replay test (`hub_test.go`): a fake hub serves the recorded `jsonrpc` replies for one tool at a time, so the real client parses the real bytes with no hub process and no network. `01-seed.jsonl`, `02-correction-dispute.jsonl` and `03-second-session.jsonl` are replayed through `Inspect`, `callTool`+`waitApplied` and `Recall`. Pinned: inspect state from the admission status (and its text fallback), the two admission checks the client reads (`distinct_interactions`, `coverage`, with the later inspect's recalculated 0.5999…), blast radius, the warning text `Contradicted by …`, four write receipts with their `applied` stats polls, the shape difference between a write receipt (string) and the stats receipt (object, so `callTool` refuses it), the ranked recall labels and an empty recall. `TestInspectStructuredStatusWins` adds one reply per state the switch reads, with the structured status and the text mark deliberately disagreeing, so the structured read is pinned on its own rather than through the fallback.
- Defect found and fixed here while pinning the mapping: `Inspect`'s no-match path returned a zero `GraphState` (`""`), not `domain.GraphNone` as its own comment and `fakes.RecordingEvidenceGraph` say, so a belief the hub had not applied yet reached the layer as an invalid state. Both branches now return `GraphNone`; owned and fixed in this task.
- One limit, recorded so the next agent does not overclaim: the recorded text no-match branch cannot distinguish the `no concept matching` substring from `hubStateOf`'s fallback, because both yield `None` with no evidence. A separate case pins the detection on the error envelope, where there is no fallback. The recorded `02` stats reply for the correction carries `applied` with a stale-dependents listing after it; `waitApplied` matches on the substring and stops there.
- The gate passes after `npm ci` in `web/` and `merchant/`.

**Review (fresh eyes).** Re-derived from the committed tree, no live call, `$0`. The bound, the attempt count, the parsing, the replay pins and the no-match fix hold; two test defects were fixed on the review branch, both Low and owned here. The fraction case was vacuous: `Retry-After: 1.5` truncated to one second equals the fallback, so a client that accepted fractions passed it; the case now reads `2.5`, and truncating it fails. The notes claimed `admission.status` was pinned by the recordings, but renaming the key — or either switch case — survived every replay, because a recorded reply renders both sources from one value; `TestInspectStructuredStatusWins` now pins both states and the precedence between them. The rest of the mutation list reproduces: deleting the retry fails the five bound cases and the context case, deleting either 503 branch fails its bound cases, and renaming `blast_radius`, `checks` or `warnings`, reading the receipt under another key, or changing the no-match substring each fail. The `GraphNone` defect is real and covered on both branches: `domain.StatusForGraph` refuses `""` with `ErrUnknownGraphState`, so a belief the hub had not applied yet failed `ReadBelief`; reverting the error-envelope return fails `TestInspectNoConceptErrorMapsToNone`, reverting the text return fails the recorded `03` no-match case, and no other line of `Inspect` changed. Limits: the recorded text no-match branch still cannot be separated from the fallback (above), and a `Retry-After` past about 9.2e9 seconds overflows `time.Duration`, reads as no wait and makes the retries immediate — only a broken hub can send that, and `attachAttempts` still bounds it. Gate green.

**Verdict: APPROVE**, with two findings fixed on the review branch, both Low and owned here (the vacuous fraction case, the structured status the notes overclaimed).

---

### T8.1c: Record the hub's recall tier decision
```yaml
requires:   [T8.1]
fixture-ok: no
size:       XS · mid
owns:       deploy/start.sh, deploy/README.md, dev-diary/research/elastic.md
status:     done
```
`deploy/start.sh` never writes a `[recall]` section, so the deployed hub runs store-only, while T0.11's check 4 and `research/elastic.md` describe the deployment as Lambo's Elastic recall tier. Decide one of two: configure `[recall]` with the Elastic backend (and record what it costs per write and what changes in recall), or record store-only as the deliberate deployment decision and correct the two documents that claim otherwise. If configuring needs embedding spend or a new secret beyond the standing budget, stop and ask the user first.

**Done when:** either the hub's config carries the recall tier with its cost and behaviour recorded, or the store-only decision is recorded in `deploy/README.md` and the stale claims are corrected; the deploy still boots; the gate passes.

**Notes (in-review).** Chose (b), store-only as the decision. Configuring the tier is cheaper than the brief feared — no new secret (`ELASTIC_URL` and `LAMBO_ES_API_KEY` are already in the environment) and no rebuild (the pinned 0.10.1 release builds with the `ship` feature set, which carries `recall-elastic`), so it is a `[recall]` section and nothing else. But it is not demonstrably free, and it is not better for the demo. The tier mirrors each flush's vectors into Elastic under `{prefix}-meta` and `{prefix}-v-<contract hash>`, so it is one bulk write per flush and one kNN query per vector probe on a Vector Database project that bills on ingest volume and storage (T0.12 check 8); nothing measures or bounds that here, `guard.sh` audits the Render plan rather than Elastic ingestion, and this task forbids live calls. It also changes recall for the worse at this size: with no `[recall]` the SQLite store's read is an exact cosine scan and the session holder ranks the vectors its graph already holds (`exact_vector_scan`), while the tier puts a lagging index on the vector leg, adds a stale-session and repair state, and — because `vectordb_document` keeps vectors out of `_source` — leaves the engine's HNSW score where Lambo would re-score exactly. The demo's reads (a belief by id, exact-content `inspect`, a correction's force, dependents) are not vector reads and are answered the same either way; durable memory is on the disk in `lambo.db` regardless.

`deploy/README.md` gains "The hub's recall tier: store-only, by decision": what the hub recalls without it, what it cannot do (no engine-side kNN; per-probe cost grows with the session's concept count), the exact `[recall]` section enabling it would need (`timeout_ms = 20000` for the cold-TLS finding), what would change in recall, what it would cost, and why the layer's own `fit-rows` pool is separate: the layer creates and reads it over HTTP through `internal/fitindex` and `internal/pool` with its own mapping and a fixed name, no alias or template links it to Lambo's indices (T0.12's coexistence check), and the hub never reads it. "First boot" step 3 now says the config carries no tier. `dev-diary/research/elastic.md` is corrected where it read as the deployment's shape: the header pointer states the hub runs store-only, the tier section is retitled "measured against this deployment, not deployed" and scopes its checks to the capability measurement, and the hosting recommendation no longer calls the project Lambo's recall tier. `deploy/start.sh` gains a comment at `write_config` (and one in the header) saying the absence is deliberate, so the next reader does not "fix" it; the generated TOML is unchanged, which is the point.

Boot proof, the documented local shape with the fixture embedder, a scratch salt and no Elastic key (`APP_DIR`/`LAMBO_DATA_DIR` under `/tmp`, `bash -n` clean; shellcheck is not on this machine): pivoted layer on, hub with its one pinned session at its salt-derived name, reader bound, layer listening with `pool=false, fit=false`, edge `/healthz` 200, hub without a token 401, reader 200, no `[recall]` in the generated `lambo.toml`, and a clean trap shutdown with no leftover children. Spend $0.00, no live call.

Two things the next agent should not have to rediscover:

- The README's local recipe was wrong and is fixed here: without `MEMENTO_IDENTITY_SALT` the layer exits at boot (`LAMBO_MCP_URL and LAMBO_MCP_TOKEN need MEMENTO_IDENTITY_SALT`), and the salt must be exactly 64 hex characters (`identity salt is malformed: need 64 hexadecimal characters`). The recipe now passes `$(openssl rand -hex 32)`, which also switches on the pivoted boot order.
- The stale line this task expected in `AGENTS.md` ("Elastic deployment, also Lambo's recall tier") was already fixed on `main` before this branch landed, so the keyring comment and the diary agree. What is left is outside `owns`: `product.md` (the Elastic row and the Lambo capability table), `docs/architecture.md` ("What lives where", "Dependencies"), `dev-diary/research/README.md`'s index entry for `elastic.md`, and `PHASE-0-ground.md`'s T0.11 check 4 ("as the deploy runs it") still describe the tier as the deployed job. The review records them as one finding for the orchestrator's doc sweep.

**Review (fresh eyes).** APPROVE. Re-verified from the tree, not from these notes. Scope: the commit touches only `deploy/start.sh`, `deploy/README.md`, `dev-diary/research/elastic.md` and this task's lines. `bash -n` is clean; `write_config` copied out and run for `native`, `bge` and `fixture` writes `[store]`, `[embedder]`, `[daemon]`, `[serve]` and `[[serve.credential]]`, and no `[recall]`, in all three. The documented local recipe was booted end to end with the 0.10.1 release binary and the three Go binaries this tree builds: healthz 200, hub 401 without the token, reader 200 for the salt-derived demo name, a second session created on demand at `/mcp/s/<random>`, `pool=false`/`fit=false` in the layer's log, no `[recall]` in the generated `lambo.toml`, and a clean trap shutdown with no leftover children. The salt reasoning is `cmd/memento`'s: 64 hex makes `memento session --shopper shopper_01` print a 72-byte name, a short salt exits with `identity salt is malformed: need 64 hexadecimal characters`, and without the salt `run` refuses `LAMBO_MCP_URL and LAMBO_MCP_TOKEN need MEMENTO_IDENTITY_SALT`, which is the exit the recipe now avoids. Against the Lambo v0.10.1 tag, the printed `[recall]` block is a real section (`kind`, `url`, `api_key = { env = ... }`, `index_prefix`, `timeout_ms`), the release's `ship` feature still carries `recall-elastic`, indices are named `{prefix}-meta` and `{prefix}-v-{<8-byte contract hash>}`, the SQLite adapter returns `exact_vector_scan = true`, and the section's lag, stale-session, repair and `vectordb_document` claims match the tag's own documented behaviour.

The decision is sound, and its strongest reason does not depend on cost: the served read path never asks the hub for a vector search. `Service.Recall` ranks from the layer's own beliefs, the pool and the correction, and `hubGraph.Recall` exists only so the client satisfies the seam, so the tier would put a lagging approximate mirror on a leg the demo does not query per request, while the hub's one real vector use — a hybrid derive on the holder — is already answered exactly in process. Even free, the tier would not be better at this size.

Two inaccuracies fixed here. The note claimed `AGENTS.md` still carried the stale line; the parent commit had already fixed it. The README's "What it cannot do" ended by saying a hub "cannot share a vector index between processes" and justified it with "one process holds every session", which the paragraph above it contradicts: `serve-web` is a second process and runs the same exact scan against SQLite. The remaining stale claims are out of `owns` and are named in the two-things list. Gate green (`npm ci` in `web/` and `merchant/` first; both were unpopulated in this worktree). Spend $0.00: no live call, no key.

---

### T8.1d: Retire the recall-tier claim in the product docs
```yaml
requires:   [T8.1c]
fixture-ok: yes
size:       XS · mid
owns:       product.md, docs/architecture.md, dev-diary/research/README.md, dev-diary/PHASE-0-ground.md
status:     done
```
T8.1c recorded the hub as store-only and corrected `research/elastic.md`, but present-tense claims that Elastic is the graph's or Lambo's recall tier remain in `product.md` (the consumption section and the stack table), `docs/architecture.md`, `dev-diary/research/README.md` and T0.11's check 4 ("as the deploy runs it"). `product.md` is the first file a reader opens, so it must not assert the retired shape.

**Done when:** each listed passage either states the store-only decision with its reason or is marked historical; no product doc asserts the tier as the deployed shape; the gate passes.

**Notes (in-review).** Retired the claim in all four listed files. No code or fixture changed, and the `fit-rows` claims are untouched: the pool stays the layer's own index with its own mapping and fixed name.

- `product.md`: the Shape section's Elastic row now leads with the fit index and says it is **not Lambo's recall tier** (decided in T8.1c), with the reason: the served reads never ask the graph for a vector search, the graph's one real vector use (a hybrid derive) is answered exactly in process, and enabling the tier is config-only but would add metered Elastic ingest and a lagging approximate mirror. The "What Lambo gives us" row keeps the true capability fact (exists, optional, `--features recall-elastic`, #18) and adds that it is not the deploy's shape.
- `docs/architecture.md`: the "What lives where" Evidence store row and the "Dependencies" Evidence store row drop "the graph's recall tier" and name the store-only decision; the paragraph after the tables states the decision and its reason, and scopes the 20 s cold-handshake finding to what enabling the tier would need.
- `dev-diary/research/README.md`: the `elastic.md` index entry now scopes the tier to what was measured against this deployment and not deployed (T8.1c); `elastic-fit.md`'s "coexistence with Lambo's recall tier" becomes "with a Lambo recall index", which is what T0.12 check 7 actually ran.
- `dev-diary/PHASE-0-ground.md`: T0.11's check 4 is marked superseded in sequence by T8.1c (the check measured the tier with the flag on, not the deploy's shape). T0.6's "Lambo also uses this deployment as its recall tier" paragraph carries the same note: it was not on the task's list but asserted the same retired shape in the same file.

**Second finding, raised in this task and fixed here.** `product.md` and `docs/architecture.md` also said Elastic stored and searched the raw evidence with its vectors. The served layer does not: `internal/evidence.Store` holds the documents in process (`docs []Document`), `cmd/memento/main.go` builds one per shopper with `evidence.NewStore`, no code path writes an evidence document to Elastic, and the live Elastic path is the `fit-rows` pool. T2.3c opened with exactly this gap and owns `internal/evidence/`, `cmd/memento/`, `product.md` and `docs/architecture.md`. Both files now state the served shape instead of the intent: raw evidence in the layer's process store, one per shopper; beliefs in the hub; the pool in Elastic. The design intent is kept as a direction T2.3c owns, and the ledger's consequence is stated where the ledger is described: after a layer restart its evidence panel holds only what the boot replay restores, while recall still answers from the pool and the corrections the standing file restored. One nuance the docs say plainly rather than repeat the short form: `Service.Recall` enumerates the layer's own belief records, so the belief-backed steps return as events replay even though the hub keeps the beliefs.

Two things the next agent should not have to rediscover:

- `product.md` has no recall-tier claim in "How it is consumed". The two claims the brief calls the consumption section and the stack table are the "What Lambo gives us" capability row and the Shape section's Elastic row; both are the ones above.
- `docs/architecture.md`'s Configuration section says the embedder direction is native embeddings served by the hosted deployment. That is about the embedder, not the recall tier or the evidence store, so it stands.

Gate green (`npm ci` in `web/` and `merchant/` first; both were unpopulated in this worktree). Spend $0.00: no live call, no key.

**Review (fresh eyes).** APPROVE, with four doc corrections committed on the review branch, all inside `owns` and all re-derived from the tree. Scope holds: against the merge base the two commits touch only `product.md`, `docs/architecture.md`, `dev-diary/research/README.md`, `dev-diary/PHASE-0-ground.md` and this task's lines (`main` has since moved three commits ahead of the base, which is what makes a `main..HEAD` diff look wider). `grep -ni "recall tier" product.md docs/` now returns only negative statements, and the reason given matches T8.1c's record item for item: the served reads ask the graph for no vector search (`hubGraph.Recall` has no caller; `Service.Recall` ranks `st.beliefs`, the pool and the correction), the one real vector use is a hybrid derive answered in process, and the tier is config-only but metered and lagging. `T2.3c` exists on `main` with `owns: internal/evidence/, cmd/memento/, product.md, docs/architecture.md`, so every passage naming it as the owner of the persistence decision points at a task that can do the work.

The four corrections, each proved from code or the boot script:

- T0.11 check 4 (PHASE-0) said "as the deploy ran it at the time", which keeps the retired premise: the deploy never ran the tier. `git log -S'[recall]' -- deploy/start.sh` finds only T8.1c's comment commit, `deploy/README.md` says "No `[recall]` section is written", and the marker's own italic note already said so. It now reads "in a test run, which the deploy never does".
- Both files listed the layer's store as holding "messages, classified reasons and catalog text". The served path indexes exactly one document kind: `indexEvidence` builds `evidence.ForReason(row, fitHandle)` (`internal/api/server/server.go`), and beyond it `ForMessage`, `ForProduct` and `MatchCandidate` are reached only from `internal/evidence`'s own tests, while the served path's `IndexDoc` call passes text alone. Both passages now say one text per classified outcome, the reason in words joined to its item.
- Both files said the ledger's panel is "empty until new events arrive". `deploy/start.sh`'s `start_memento` boots the layer with `-seed`, and `LoadSeed` re-indexes every seeded evidence row, so the demo story's rows are back as soon as boot's replay finishes; only a live event's row is lost. Both passages now say that, which also makes the loss consistent with the belief sentence beside it.
- The Dependencies table carried a row titled "Evidence store" whose own text read "Not the raw evidence store either", and the sentence under it said the layer "depends only on the first four". The row is now "Fit index (Elastic)", matching "What lives where", and the sentence names the four.

The builder-flagged recall-after-restart sentence stands, and I checked it rather than trusting the note: `Service.Recall` (`internal/api/server/tools.go`) enumerates `st.beliefs`, `state()` starts that map empty, `restoreStanding` restores only the pooling choice, the correction views and the journal, and nothing lists beliefs from the hub — so after a restart the pool and a restored correction decide while belief-backed steps can only return as the writes replay, which is what T8.5b (pool decides) and T8.5e (correction decides, pool overridden) saw live. The sentence says exactly that in both files.

Findings, none blocking, and not fixed here because it sits outside `owns`:

- (Low) `internal/evidence/store.go`'s type comment calls the store "shopper messages, classified reasons and product texts with their vectors", and `CheckContract` says "Vectors from one embedding space do not answer in another", while `Store` holds `docs []Document` and a contract kind and no vector of any sort; the comment names two kinds this layer never stores. **Owner: T2.3c** (`owns: internal/evidence/`, and it is already the task that decides the store's persistence).

Limits worth recording:

- "recall tier" still appears in `dev-diary/research/elastic-fit.md` (its §7 heading and two body lines) and in the live harnesses' READMEs (`testdata/lambo-multi/`, `tests/live/hub/`, `tests/live/precedence/`). Those are past-tense measurement records and harnesses that switch the tier on deliberately, none is a product doc, and none is in this task's `owns` or the brief's grep scope; the `elastic-fit.md` index entry now scopes its §7 to what T0.12 ran.
- `indexEvidence` passes only `doc.Text` through the seam (`Store.IndexDoc`), so a stored document is a bare message-kind text with no `Line` and no `Seeded`, and `Documents()`/`LedgerSearch` have no production caller; the ledger's seeded mark comes from `st.evidence`, so rule 7 holds where it is observed today. The next agent to persist this store should know the fields are not populated on the served path.
- The branch is three commits behind `main` (the claim, T2.3c's opening, T9.8's opening), so landing needs a rebase; the only collision is this task's `status` line against the claim commit.

Gate green in the review worktree (`npm ci` in `web/` and `merchant/` first; both were unpopulated there). Spend $0.00: no live call, no key.

---

### T8.5h: Record a general statement, or state the limitation
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       S · mid
owns:       internal/api/contract/, internal/api/server/, testdata/api/, docs/interface.md
status:     claimed:t85h-statement
```
The precedence order's fourth rung is a general statement, and the served layer serves statements when they exist, but no tool records one: statements reach the store only through the seed. A builder reading `docs/interface.md` would look for a way to record the shopper's usual size and find none, so rung 4 cannot be exercised live. Decide: add a `record_statement` tool (contract change with its fixtures and docs), or state the limitation plainly in `docs/interface.md` and name the seed as the only source.

**Done when:** either the tool exists, is served, and its fixture set validates, or the documentation states the limitation and the seed as the only source; the gate passes.

---

### T8.5i: Recall never withholds thin evidence
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       S · mid
owns:       internal/api/server/, tests/live/layer/
status:     not-started
```
The P2 review found `internal/api/server/recall.go` serves Venerable/Candidate fit beliefs and evidenceless statements but drops a fit belief with one return (`GraphNone`, one evidence), with a comment saying "recall never withholds, but one return is a question, not an answer" — which is withholding. `product.md` says it never refuses, thin evidence comes back labelled thin, status is a label never a gate, and own evidence counts at any status. The T8.5 handoff recorded this and assigned it to T7.4, which could not reach this package and never mentioned it, so the finding is now owned here. Round 2's arm 4 relies on thin evidence being served.

**Done when:** a one-return fit belief is served, labelled with its status and precedence, with the pool beside it exactly as thin evidence is served elsewhere; the live layer tripwire that pinned the withholding is flipped; the gate passes.

---

## Exit Criteria

- [x] One Lambo process serves every shopper, live (T8.1, T8.1b).
- [x] Every purchase writes and updates its fit row at ingest, live (T8.2, T8.2b).
- [x] The pool reads back settled-only counts and its status, live (T8.3, T8.3b).
- [x] A correction stands on its own (T8.4).
- [x] Every caller is authenticated, and no route returns a shopper's memory without a key (T8.6).
- [x] Webhook deliveries are verified against PayPal's signature, not a fixture (T8.7).
- [x] Recall never refuses, and says which step decided, live (T8.5, T8.5b).
