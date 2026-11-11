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
Ingest writes a fit row beside every Lambo write, from the same event: capture creates the row, a refund marks it returned, a classified reason gives it a category and direction, fit feedback and kept-past-the-window settle it, and a contested reason updates it. Each write replaces the whole document at the state-rank version with `external_gte` (T0.12: the partial `_update` API refuses external versioning). The index is created from T7.1's mapping. Only the layer's interface to the index is named in domain code (developer rule 6). (T8.5x correction: the served layer writes capture, refund, classified reason and feedback rows; a kept row comes from the seed, and a contested reason has no served tool. See T8.5x.)

**Done when:** a fixture test drives one purchase through every state, in order and shuffled, and ends at the same row. A replayed webhook is harmless, and an older state never overwrites a newer one. A refund with no reason is stored but not settled. A shopper's correction writes nothing to the index. An opted-out shopper's events write no row, and opting out deletes their existing rows across every line their session knows, while leaving other shoppers' rows alone. The gate passes.

**Notes (in-review).** `internal/fitindex/` is the only writer of pooled rows. `Index` is the layer's whole interface to the index (`Create`, `Put`, `DeleteShopper`); `Writer` is one shopper's writer, built with the identity keyer, that shopper's id and a `Lines` resolver, and it holds each purchase's facts for the life of the process so it can replace the whole document every time. The transitions are `Captured`, `Refunded`, `Classified`, `Feedback`, `Kept` and `Contested`; `Record(domain.Evidence)` is what ingest calls, and it maps a payment capture, refund or kept outcome onto them. A refund's evidence never carries a reason through ingest, so `Record` stores it returned and unsettled: a caller holding an already-classified refund, as T9.1's seed rows do, must call `Refunded` and then `Classified`, or the row never settles. `Forget(lines)` recomputes the per-line hash for the shopper's known lines and deletes those rows; `WithPooling` is the consent check that stops new rows. `Mapping()` carries the recorded mapping's fields and types, whitespace aside, pinned against `testdata/elastic-fit/mapping.json`; `HTTPIndex` creates the index, writes `<index>/_doc/<txn_hash>?version=<row.Version()>&version_type=external_gte&refresh=true`, maps a 409 to `ErrOlderEvent` and deletes by query with the three line fields and the shopper hash. Ingest gained one seam: `WithFitRecorder` takes a `FitRecorder` and every capture and refund delivery records its fit row before storing the evidence, so a failed fit write fails the delivery and the retry writes both. A dispute writes no fit row.

Five things the next agent should not have to rediscover:

- `domain.Item` has no fit handle, so the writer asks the catalog through `Lines` rather than guessing one. `agent/catalog` is where a production resolver belongs; the tests use a function. This is a new seam, not a contract change: `domain.ProductLine` and the fit row are unchanged.
- The reason category is written as `domain.Category` names it (`damaged_or_defective`, `not_arrived`). The recorded population's `damaged` and `not_as_described` were the harness's synthetic labels, and T0.12 says the classifier's categories slot in. Only the direction is mapped (`small`/`large` to `too_small`/`too_large`).
- The document has no `event_seq`: T7.1's state-rank version supersedes T0.12's sequence, and the recorded mapping keeps the unused field. `refresh=true` is on each write so a read after a purchase sees its row; the index is one document per purchase, not a bulk path.
- The transaction id is the capture id, with the order id as the fallback when the link carries none; the same value on capture and refund. The two ids hash to different rows, so a purchase whose events disagree gets two. A multi-item order shares one capture id, so it would share one row, and an item in a second line would overwrite the first. The demo and the seed buy one item per purchase, and T9.1's population and T10.3's generated orders must keep it that way. If multi-item orders arrive, the row key needs a decision, not a workaround here.
- A reason of `unclear` or `none` is stored and leaves the row unsettled: it is not a cause, so the refund stays a pending question. A contest replaces the category and direction in place and keeps the first `settled_at` while the row stays settled; a correction that leaves no cause clears it. The event time still breaks ties inside the last rank.

**Review fixes (in-review).** Three defects the review found are fixed inside this package, each with a test that fails on the old behaviour. `HTTPIndex.DeleteShopper` reads `_delete_by_query`'s `failures` and returns `ErrIncompleteDelete` with the count that did go, so a partial opt-out cannot report success (`TestDeleteShopperReportsPartialFailures`). `Writer.record` takes the writer's lock before the consent check and holds it through the index write, and `Forget` holds the same lock across its deletes, so a write that passed the check cannot land behind the opt-out (`TestForgetWaitsForAWriteThatPassedTheConsentCheck` blocks a write inside the index call, runs Forget, releases the write and asserts the row is deleted and nothing remains). A contest that leaves the row unsettled clears `SettledAt` and `SettledKind` in the merged row, not only in the document (`TestContestWithNoCauseUnsettles` pins the zero time).

The writer was not yet wired into the served layer when this task closed: `internal/api/server` builds its ingest service per shopper and owns `record_reason`, `record_feedback`, `record_correction` and `set_pooling`, and this task does not own that package. T8.5 (landed) passes the shopper's identity and a line resolver when it builds the writer, resolves the same line for the explicit `Classified`/`Feedback` calls (only `Record` asks the resolver; T8.5x correction: the served layer has no `Contested` call), and calls `Forget` with the lines the shopper's session knows after recording the opt-out. T9.1 builds the seeded population through the same writer with `Seeded` set and the published seed salt; T8.2b uses `NewHTTPIndex` and `Create` against the live deployment.

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

The dev-diary report's transcript is the handoff log entry; it is not committed here. That entry's block was not pasted from a run: it was written by hand around the driver's steps, so its rows do not match what the driver prints. T8.2c re-ran the driver against the live deployment and records which lines drift; the block is superseded once the handoff log carries T8.2c's transcript.

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
- **Deployment wiring is outside this task's `owns`.** `deploy/render.yaml` needs the three variables as `sync: false` env vars, `deploy/provision.sh` pushes them beside the five it already sends, `.env.example` should name them by name, and `tools/env.sh` should read them from the keyring; `deploy/start.sh` needs no change because the edge inherits the environment. `memento key issue` (T8.5) now mints a key without writing code, and T9.6 minted the deployment's three from it. With no keys configured the edge still starts and refuses every API call, loudly in its log, while `/healthz` stays green: a keyless deploy reads as healthy.
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

- **The deployed webhook route still bypassed the gate when this task closed.** `internal/api/server`'s `handleWebhook` decoded `{shopper_id, event_id}` and called `IngestWebhook`; a real PayPal body never reached `HandleDelivery`. This was a change in `internal/api/server/`, outside this task's `owns`, so it was **unmet here**. T8.5 later landed it: the route reads the raw body bounded, resolves the shopper from the resource's `custom_id`, and passes `ingest.Delivery{Header, Body}` to the shopper's service, so the deployed path now verifies every delivery (review checked `handleWebhook` → `IngestDelivery` → verifier).
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

The served layer serves the pivot when the pivot half is wired (pool index, fit index, identity salt, line resolver, standing directory); with them absent it keeps the pre-pool shape, so the frozen base fixtures still describe it. Recall serves fit beliefs at Candidate and above plus general statements (superseded by T8.5i: every fit belief for the brand is served, at any standing, so a lone return arrives labelled Claimed), resolves the usual size from kept and fits-well tallies with the statement as fallback, consults the pool at the usual size or the size under consideration, and marks every item with its step. Any index error answers unavailable and never an empty pool. `record_correction` with no belief id records a standing: named under the most recent brand context and journalled, declared against an existing match at once and against a later match when the belief is written; unscoped while the layer knows no brand, applied spanning every brand and never declared. Every belief write goes through `EnsureBeliefDeclaring`. `product_fit` maps the reader's counts with both index spellings of the non-fit reasons folded; `set_pooling` records the choice in the shopper's own memory, calls `Forget` with the session's full line list, and retries an incomplete delete before it reports not done. The deployed webhook route reads the raw body bounded, resolves the shopper from the custom id and passes `ingest.Delivery` to the shopper's service. `/calls` stays agent-only; the single service's handler narrows it by the guard's `X-Memento-Shopper` header (`TestCallsScopeHonoursGuardHeader`), and the served director (`cmd/memento`) answers it by aggregating every shopper service's log with no header filter, which is the agent-only shape T8.6 recorded (review correction: the header is honoured by the service, not by the deploying director, and no caller kind but an agent reaches either); the MCP decoder takes one object with unknown fields rejected. `memento key issue` mints one caller key with `auth.IssueKey`, printing the token once. The nine-tools comments were already eleven on main (sweep d0f71f3), so nothing here renamed them.

Decisions the next agent should not have to re-make:

- **Serving line.** Superseded by T8.5i: a lone return (Claimed, no promotion) is now served, labelled Claimed with its own-evidence step, and this task's rule that held it back until a second return is gone. The decision as written here was: a lone return stays on the ledger until a second moves the belief to Building, which kept the story test green and matched all four pivot fixtures; read literally, product.md's "at any status" also serves the lone return.
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
- The live tripwires in `tests/live/precedence/` pinned the defects while this task closed, and they were outside this task's `owns`, so they were left untouched: the next live run would have surfaced both as stale pins (the consulted-empty probe succeeds where it pinned a failure; the dangling view still overrides after restart where it pinned loss of force). T8.5e flipped both; read its notes and the driver, not this sentence.

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
status:     done
```
The precedence order's fourth rung is a general statement, and the served layer serves statements when they exist, but no tool records one: statements reach the store only through the seed. A builder reading `docs/interface.md` would look for a way to record the shopper's usual size and find none, so rung 4 cannot be exercised live. Decide: add a `record_statement` tool (contract change with its fixtures and docs), or state the limitation plainly in `docs/interface.md` and name the seed as the only source.

**Done when:** either the tool exists, is served, and its fixture set validates, or the documentation states the limitation and the seed as the only source; the gate passes.

**Notes (in-review).** Took the documentation option. A `record_statement` tool is not small here: `TestToolsFrozen` pins the surface at eleven names; `internal/auth`'s `toolAccess` table is outside this task's `owns` and `TestEveryToolIsClassified` fails until a new name is classified; and both agent frontends bind the eleven tools and assert the count, so the tool would drag `internal/auth`, `agent/runtime/` and `agent/second/` into the commit. That is a frozen-contract change, which rule 1 makes the orchestrator's call, and what it would buy is the single Claimed label the seed already supplies, since the layer never promotes a statement. The merchant app and the web ledger bind no tool, so they would not move. Docs changed:

- `docs/interface.md` gains "Where the general statement comes from", between the precedence order and the new-shopper example: no tool records a statement, the boot replay's derived notes are their only source, the reason is that a statement is a Claimed label and never evidence (so no code path may turn it into a belief), what adding a tool would actually take, and how recall serves one when it is there (step 4, Claimed, standing None, overridden by own evidence and the pool, and the usual-size fallback).
- The rung-4 table row, the turn-pattern step for recording what the shopper says, the "How a signal is written" row and the new-shopper example's lead sentence all say the same thing, so a builder meets the limit where they look for the tool.
- One fixture corrected for honesty: `recall_for_purchase.response.example-pool.json`'s statement belief carried `seeded: false`, which the layer's own seed does not produce — a statement belief reaches memory only through `LoadSeed`, which copies the mark each row records, and the seed the layer boots from (`SeedFromExamples`, over the frozen ledger fixtures) carries its statement row `blf_77c310` marked, because a statement is generated history and never a lived event (the copied mark is what lets the frozen set's two live-origin rows stay unmarked; T8.5p records that reading) — so it now carries that mark, and the document's block is byte-identical to it. `ledger_beliefs.response.example.json` and `internal/fakes/final.go` already marked the same statement `seeded: true`, so the recall example was the outlier against its siblings, not a deliberate shape. Rule 7 holds at the one row the doc used to contradict itself on.

A follow-on `record_statement` tool would be a new contract task for the orchestrator to open; the tool name, the two schemas, the route, the table test and `internal/auth`'s access table all belong in that one commit. Gate green, `npm ci` in `web/` and `merchant/` first; `go test ./internal/api/...` green; spend $0.00, no live call.

One limit, outside `owns`: `tests/live/precedence/main.go`'s `statementSeed` builds its statement belief with `Seeded: false`, and `LoadSeed` copies a row's mark rather than forcing it, so that harness serves an unmarked statement where rule 7 says a seeded row is marked. Nothing the layer itself boots with is built that way, and no open task owns `tests/live/precedence/` (T8.5b and T8.5e are both done), so this is written for the orchestrator to open a task over it or fold it into the next live run. **Owner: a new task over `tests/live/precedence/`.**

**Review (fresh eyes).** APPROVE, with five wording corrections committed on the review branch, all inside `owns`. Re-verified from the tree, no live call, $0. Scope holds: against `main` the commit touches only `docs/interface.md`, `testdata/api/recall_for_purchase.response.example-pool.json` and this task's lines.

The fixture correction is what the served path produces. `SeedFromExamples` reads `ledger_beliefs.response.example.json` verbatim into the seed, and that file's statement row (`blf_77c310`) is `seeded: true`; `LoadSeed` copies the mark into the belief record (`seeded: view.Seeded`), and recall serves that record's mark (`Seeded: rec.seeded`, `internal/api/server/tools.go`). `internal/fakes/final.go` marks the same statement the same way. The document's block is byte-identical to the fixture, pinned by `TestInterfaceDocExamples`, and the schema, Go-type, marshal-back and pool-arithmetic tests all pass on the corrected file.

The limitation is stated with its reason where a builder meets it: the rung-4 row, the turn-pattern step, "How a signal is written" and the new-shopper lead all name the seed, and the new section says why a statement is never evidence. Its account of what recall does with one matches the code: `recallBeliefs` serves an evidenceless `GraphNone` fit belief, `beliefStep` calls it the statement step, `domain.StatusForGraph` labels `GraphNone` Claimed, and `usualSize` falls back to the statement's size. `TestStatementsAndKeptNeverPromote` pins the never-promotes half in `internal/memory`.

Option (a) is sound, not merely cautious. `TestToolsFrozen` (`contract_test.go`) pins eleven names inside `owns`, but `internal/auth`'s `TestEveryToolIsClassified` fails until `toolAccess` classifies a new name, and both agent frontends bind the eleven tools and assert the count, so the tool is a contract change outside `owns` — which rule 1 makes the orchestrator's call — for a label the seed already supplies. The merchant app and the web ledger bind no tool and would not move.

Five corrections, each small and inside `owns`. Three in the new section: "the statement in the example below is one of them" claimed a served provenance the deploy does not give that shopper, since the layer replays one seed for the shopper it is started for, so it now states that limit and says the example has the shape; "seeds it with the shopper's history" implied a call a builder can make, and there is none; "a route that derives the note" implied a new route, where `/tools/{tool}` already covers a new tool and the missing piece is the handler. Two in these notes: "which no path can produce" was false as written — `LoadSeed` copies the mark rather than forcing it — so the reason now names the seed that marks its rows; and the tool list was said to sit in "the agent frontends, the ledger and the merchant app", where only the frontends carry it.

Gate green after `npm ci` in `web/` and `merchant/` (neither was populated in the review worktree): `gofmt`, `go vet`, `go test -race` over every Go package, both lockfile checks, both Svelte checks, both lints, both test runs and both builds, ending `OK all checks passed`. Spend $0.00.

---

### T8.5i: Recall never withholds thin evidence
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       S · mid
owns:       internal/api/server/, tests/live/layer/
status:     done
```
The P2 review found `internal/api/server/recall.go` serves Venerable/Candidate fit beliefs and evidenceless statements but drops a fit belief with one return (`GraphNone`, one evidence), with a comment saying "recall never withholds, but one return is a question, not an answer" — which is withholding. `product.md` says it never refuses, thin evidence comes back labelled thin, status is a label never a gate, and own evidence counts at any status. The T8.5 handoff recorded this and assigned it to T7.4, which could not reach this package and never mentioned it, so the finding is now owned here. Round 2's arm 4 relies on thin evidence being served.

**Done when:** a one-return fit belief is served, labelled with its status and precedence, with the pool beside it exactly as thin evidence is served elsewhere; the live layer tripwire that pinned the withholding is flipped; the gate passes.

**Notes (in-review).** The gate in `recallBeliefs` is gone. It selects every fit belief the shopper holds for the requested brand at any standing; only the kind and the brand filter anything out. A lone return therefore comes back as the graph reads it — Claimed/None, its one evidence row attached, its step named `own_evidence` — and `usualSize`, the pool query and the precedence order see it exactly as they see a Venerable belief, so the pool rides beside it and is marked overridden by own evidence. The correction rule is untouched: `overridingCorrection` and `assembleRecall` are unchanged, and the assembler still refuses a contradicted belief without its override.

The seeded recall now serves three beliefs instead of two: the Earned sizing belief, the one-evidence fits-well feedback (`blf_2b9d60`, Claimed/None), and the general statement. `assertRecallPivot` pins the thin one — Claimed/None, one row, `own_evidence` overridden by the correction — and the byte-exact comparison against the frozen base fixture filters both thin beliefs through one helper, `baseRecallBeliefs`, which `http_test.go` also uses, so the fixture's two filters cannot drift apart. The story test's one-return step now asserts the served thin belief (one belief, Claimed/None, one too-small row, own-evidence step deciding, guidance "Take size L for Brand X"), and its final recall expects the fits-well belief beside the Earned sizing one.

One wording fix the new serving made reachable: `guidanceForOwnEvidence` produced "your own one return say it runs small" because only counts of two or more could reach it before. `ownEvidenceClause` now agrees the verb, so a lone return reads "your own one return says it runs large, ahead of the pool." The two- and three-return texts are byte-identical to before, which the frozen pivot fixture, `TestRecallUnlikeCrowdDecidedByOwnEvidence` and the live precedence driver already pin.

The new service test is `TestRecallServesOneReturnBesideThePool`: a wired Earned-small pool, one too-large return, and the recall must carry the thin belief with its own-evidence step, the pool overridden beside it, and that guidance sentence. Against the old `recallBeliefs` it fails at "one return serves 0 beliefs", and the story test fails the same way; both pass on this commit. The live tripwire in `tests/live/layer/main.go` is flipped to match: the one-return step asserts the served Claimed belief with its one row, its precedence and its guidance, and the final recall expects the Claimed fits-well belief beside the sizing one. No live call was made (spend $0.00); that driver is outside CI and needs the hub, the deployment URL and a key.

Gate green after `npm ci` in `web/` and `merchant/`: `gofmt`, `go vet`, `go test -race` over every Go package, both lockfile checks, both Svelte checks, both lints, both test runs and both builds, ending `OK all checks passed`. Spend $0.00: no live call, no key.

Handoff finding with no home in this task's `owns`: `demo/driver/main.go` carries the same two pins — one-return "no belief, no guidance" and "final recall serves only the Earned sizing belief" — so that driver would fail at those steps now. `demo/` is outside T8.5i's paths; it needs its own task (a small one over `demo/driver/`) to make the same two assertions. Until then the demo driver and the live layer driver disagree about the served shape.

---

### T8.5j: Mark the live precedence harness's seeded statement
```yaml
requires:   [T8.5e]
fixture-ok: no
size:       XS · mid
owns:       tests/live/precedence/
status:     done
```
The T8.5h review found `tests/live/precedence/main.go` builds its statement belief with `Seeded: false` and `LoadSeed` copies the mark through, so rule 7 ("seeded means marked") does not hold at that row. No product path is built that way, but the harness's own fixture should be honest. Mark every seeded row the harness builds and assert the mark where it checks the served statement.

**Done when:** the harness's seeded statement carries the mark, its assertions check it, and a live run still passes; the gate passes.

**Notes (in-review).** `statementSeed` now builds its row with `Seeded: true`, and the statement step asserts the mark where it already checked the guidance: the recall must serve exactly one belief, `seed_stmt_001`, with `Seeded` true, so an unmarked row fails the driver instead of passing as lived history. Its transcript line prints the belief id and the mark. The pool seed already carried `Seeded: true` on all ten rows; the pool step's seeded/live split (10 seeded, 0 live) is that mark's assertion, and both the `seedPool` doc comment and the split check now say so, so no row the harness builds is left unmarked.

Proof without a live call: a temporary in-package check, deleted before the commit, built the harness's own `statementSeed` through the real `LoadSeed` over the graph fake and read `Recall` back — guidance basis statement, text "Take size M for Brand S: the shopper's usual size.", belief `seed_stmt_001` with `seeded=true`. The same check flipped the copy's mark to false and got `seeded=false` back, so the new assertion bites. The mark path it exercises is the shipped one: `LoadSeed` copies `view.Seeded` into the belief record and `beliefView` serves `rec.seeded`.

The live leg is left to the next run of the T8.5b driver: it needs the hub binary, the deployment URL and its key, and any live run is spend-facing, so this task made no live call (spend $0.00). The assertion is what that run will exercise — a live pass also needs the seeded statement to reach the served recall, which no offline check can prove.

Gate green, `npm ci` in `web/` and `merchant/` first. Against `main` the commit touches only `tests/live/precedence/main.go` and this task's lines.

---

### T8.5k: One spelling for not-as-described in the pool view
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       XS · mid
owns:       internal/api/server/, internal/fitindex/, testdata/api/, docs/interface.md, dev-diary/PHASE-8-pivot-layer.md
status:     done
```
`internal/api/server/recall.go` folds a literal `"not_as_described"` reason category into `not_arrived` counts, a seam for the recorded Elastic population's labels. Since T1.1b, a not-as-described dispute counts under `damaged_or_defective`, so the same cause would read as non-arrival in one path and damage in the other if a classifier label ever reaches evidence. Disputes write no fit rows, so the pool never sees them today; decide and record whether the fold stays (with its reason) or maps to damage.

**Done when:** the fold's mapping matches the domain's damage category or is recorded as a deliberate label-only seam with its reason; a test pins it; the gate passes.

**Notes (in-review).** The fold stays, and a not-as-described cause now counts as damage. `reasonCountsView` sends the recorded population's `not_as_described` label to `contract.ReasonCountsView.DamagedOrDefective`, the field `domain.CategoryDamagedOrDefective` feeds, and only `domain.CategoryNotArrived` reaches `NotArrived`. The `damaged` label keeps counting as damage. T1.1b filed `MERCHANDISE_OR_SERVICE_NOT_AS_DESCRIBED` under `damaged_or_defective` because the goods arrived and were wrong, so the one cause no longer reads as non-arrival in the pool view while the dispute path calls it damage.

The trace that decides the shape: no served path can put either legacy label into a fit row today, so the alias is a read-side spelling, not something the layer produces.

- `internal/reasons/client.go` accepts a classifier choice only from the six domain categories and refuses anything else with `ErrUnknownCategory`; `record_reason` passes that category to the fit writer.
- `internal/domain/reason.go` maps the not-as-described dispute code to `CategoryDamagedOrDefective`, and `StoredCategoryForDispute` stores an unknown code with `CategoryNone`.
- `internal/fitindex/index.go` is the only writer of pool rows: `Record` returns before the index for a dispute, and `Classified` and `Contested` reject every category outside `domain.Categories`, so neither `damaged` nor `not_as_described` can be stored.
- `internal/api/server/pool.go` writes fit rows from a capture, a refund, a classified answer and fit feedback only, and `internal/ingest`'s `FitRecorder` names capture, refund and kept only.
- `internal/api/server/seed.go`'s `LoadSeed` replays beliefs, evidence, feedback and corrections into the shopper's graph and never calls the fit writer; the boot replay only calls `LoadSeed`, so seeded history writes no pool row.
- The served population carries only domain words: `testdata/seed/population/population.json` has 37 `fit_too_small`, 13 `fit_too_large`, one `damaged_or_defective` and 18 `none`. The legacy labels live only in the recorded T0.12 population (`testdata/elastic-fit/`) and the fixtures and tests that mirror it (`internal/pool/httpindex_test.go`, `internal/api/server/pivot_test.go`).

The alias stays rather than being deleted because `internal/pool` keeps each index label as it arrives in `Counts.Reasons`: an index that already holds the recorded population would report its two `not_as_described` returns nowhere, leaving the reason breakdown short of the settled denominator, if the view dropped the label, and mapping it to damage states the cause T1.1b decided. Dropping it would also leave the view's "unknown categories are dropped" rule swallowing a label the project itself recorded.

Tests. `TestReasonCountsViewGivesEachCauseOneField` feeds every domain category, both legacy labels and an unknown one through `reasonCountsView` and pins each field: damage 23 (5 domain, 7 `damaged`, 11 `not_as_described`), non-arrival 13 (the domain constant alone), and the unclear, none and unknown labels dropped. `TestServedFitWriterRefusesTheLegacyPoolLabels` drives the real `fitindex.Writer` the service builds: `Classified` and `Contested` refuse both labels with `ErrUnknownReason` and write no row, a not-as-described dispute carrying the damage category writes no row, and `domain.Categories` names neither label, so a task that widens the writer's accepted set, or makes `Record` write a dispute row, fails there; a producer that bypasses the writer is outside that test. `TestProductFitServed` now expects damage 3 / non-arrival 0 from the fake pool's `damaged` 2 + `not_as_described` 1.

Mutations, each reverted: putting `not_as_described` back under `NotArrived` fails the mapping test (`NotArrived: 24`, want 13) and `TestProductFitServed` (`NotArrived: 1`); widening `fitindex.validCategory` to accept the two labels fails the reachability test at the first refusal and at "a refused label wrote 1 fit rows".

The delivered finding is closed by the orchestrator's rule-1 call: the fixture and its doc block moved in this commit, with the served view. `testdata/api/product_fit.response.example.json` now reads `damaged_or_defective: 3, not_arrived: 0` in the all block and 2 / 0 in the seeded block, because the old counts were `damaged` 2 + `not_as_described` 1 and `damaged` 1 + `not_as_described` 1; the live and recent blocks carried no `not_as_described` and stay 1 / 0. The same pool block is repeated in `testdata/api/recall_for_purchase.response.example-correction.json`, `...-own.json` and `...-pool.json`, and moved with it. `docs/interface.md` carries four validating copies — the `product_fit` example and the three recall examples — which moved in step, so the byte pin holds. No other frozen example or golden carries the old pair: a search of `testdata/` and `docs/` for `damaged_or_defective` 2 / `not_arrived` 1 and 1 / 1 finds nothing after the move, and `testdata/replay/`, the web fixtures and the merchant fixtures carry no pool reason counts. `TestPoolFixturesMatchTheRule` still finds each block's status and arithmetic, and `TestInterfaceDocExamples` pins every doc block byte-identical to its fixture.

Gate green after `npm ci` in `web/` and `merchant/` (neither was populated in this worktree), and green again on a full re-run after the fixture move: `gofmt`, `go vet`, `go test -race` over every Go package, both lockfile checks, both Svelte checks, both lints, both test runs and both builds, ending `OK all checks passed`. Spend $0.00: no live call, no key.

---

### T8.5l: Feedback is idempotent per purchase
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       S · mid
owns:       internal/api/server/, internal/memory/
status:     done
```
The T9.7 live run found `record_feedback` is not idempotent per purchase in the shopper's belief: repeated feedback appended the same capture's evidence (`ev_51c2d8`) three times to `blf_2b9d60` and lifted it Claimed → Building, while the pool row stayed one. A retried or duplicated feedback delivery must not multiply that purchase's evidence or move the belief's status.

**Done when:** feedback for a purchase already carrying that capture's evidence is a no-op that reports the duplicate; a table test proves three sends leave one evidence row and the same status; the gate passes.

**Notes (in-review).** Feedback is now recorded once per purchase, joined on the purchase's own evidence row rather than on the shopper's words. `evidenceRecord` carries the feedback it was recorded as (`recordedFeedback`: its id, its verdict and the belief it counts toward), so the second and third delivery for the same purchase are recognised before anything is minted or written.

What a repeat does: nothing to the belief. No `fb_` id is minted, no evidence row is appended, nothing reaches the graph, and the belief's status cannot move — the `Claimed → Building` lift came from the extra graph actions, so no write is the whole of that fix. The answer is the one already on record: the same `feedback_id`, verdict, `counts_toward` and `belief_id`. That is the contract-consistent option the brief allowed ("a benign no-op with the same answer as the first"), and the frozen fixtures already read it that way: `record_feedback.response.example.json` answers `accepted: true` for the seeded purchase, which is a repeat of the history's copy, so a duplicate marker would have had to disagree with the contract. No field was added to a shared type: the response shape is frozen and outside `owns` (rule 1).

The seeded arc's copy is recognised, not double-counted. `LoadSeed` marks each seeded feedback row with the verdict and the id the history records, and `SeedFromExamples` takes that id from the fixture pair that names it: `record_feedback.request.example.json` names the purchase (`custom_id`, SKU, size) and the response names `fb_51c2d8`, the id `explain_belief.response.example.json`'s timeline repeats for the same feedback. One purchase's feedback therefore has one identity across the fixture set, and the served layer answers `fb_51c2d8` for it rather than an id it invented. `Seed` gained `FeedbackIDs` (evidence id → feedback id); a seed built by a harness that names no id carries none, and a repeat then mints the id it holds the row under, once, and answers that same id thereafter — the frozen response schema forbids an empty `feedback_id`.

The repeat still settles the purchase-keyed fit row from the verdict on record, which is idempotent per purchase by design. That keeps the pool's behaviour exactly as today (the T9.7 gap itself recorded "the pool row stayed one"): a first delivery that repeats the seeded copy still counts its purchase once, so the demo's 22 → 23 settled and 0 → 1 live stand.

Tests, in `internal/api/server/feedback_test.go`: a table of three sends (the first, an exact retry, and the same feedback in other words) against a fresh purchase proves after *every* send that the belief lists one evidence row with the purchase's id, that the status has not moved, and that the three answers carry one feedback id; it also pins one graph action and one minted id. A second test drives the same three sends against the seeded service and proves the belief keeps its one seeded `ev_51c2d8`, stays `Claimed`, keeps its own reason words, and that no graph action and no mint were added. Both fail on the parent commit at exactly those assertions (two evidence rows after the retry; two `ev_51c2d8` rows on the seeded belief after the first send).

`internal/memory` is untouched, deliberately: the writer sees only the graph content and the stub, never the purchase, so the purchase's identity cannot be checked there. The rule lives where the purchase is joined.

Three things outside `owns` for the orchestrator: `docs/interface.md`'s `record_feedback` section does not yet say a repeat answers with what is on record and changes nothing (a documentation task, or the landing commit — the review branch adds the sentence); T9.7's gap 5 in `PHASE-9-pivot-surfaces.md` and `demo/RUN-6.md` now describe a closed defect, and a re-run of step 4 would show the belief staying `Claimed` with its one evidence row while the pool still moves 22 → 23; and `tests/live/layer/main.go`'s feedback step is a live harness no gate runs, so check it against the new answer before the next live pass (the review did: it is one first delivery on a purchase the seed does not hold, so it reads the same answer it always did).

One behaviour a reviewer should know: the duplicate check runs before the new words are read for what they count toward, because the purchase decides, not the wording. A repeat delivery whose new words would read as a seller verdict (damaged, not arrived) therefore answers with the recorded fit feedback instead of the `ErrKindMismatch` a second write used to raise. A first delivery of either kind is unchanged.

Gate green after `npm ci` in `web/` and `merchant/` (neither was populated in this worktree): `gofmt`, `go vet`, `go test -race` over every Go package, both lockfile checks, both Svelte checks, both lints, both test runs and both builds, ending `OK all checks passed`. Spend $0.00: no live call, no key.

**Review (fresh eyes).** APPROVE, with the fixes below committed on the review branch, all inside `owns` plus the one `docs/interface.md` sentence. Re-verified from the tree, no live call, $0.

Scope holds: against `main` the commit touches only `internal/api/server/` and this task's lines. The first delivery is unchanged — the only edit on its path is the marking written *after* the graph action, the evidence append and the journal persist — and a repeat is answered before anything is minted or written. The seeded copy is the fixture's own: `LoadSeed` marks the row, `SeedFromExamples` fills `FeedbackIDs` from the `record_feedback` request/response pair, and the served answer for purchase `memento-shopper01-06` is `fb_51c2d8` on `blf_2b9d60`, which stays `Claimed` with its one `ev_51c2d8` row after three sends.

Mutation-checked, each reverted: removing the repeat path fails both builder tests with exactly the parent symptoms (`retry send left 2 evidence rows, want 1`; `the seeded belief lists 2 evidence rows [ev_51c2d8 ev_51c2d8]`); removing `LoadSeed`'s marking fails the seeded test the same way; removing the fixture join fails on `feedback id "", want … fb_51c2d8`.

Two tests added, because the pool claim was asserted in prose and pinned nowhere. `TestRepeatFeedbackSettlesTheSeededPurchasesRowOnce` wires the fit writer into the seeded service and proves the history's copy — a repeat by construction — puts the purchase's one row into the pool settled by fits-well feedback, so the demo's 22 → 23 stands; removing the fit-row call from the repeat path fails it (`first send left 0 fit rows`). `TestRepeatFeedbackAnswersAnIdTheHistoryDidNotName` pins the fix below.

The empty id was a schema break. The note said a seed naming no id answers with `""`, but `record_feedback.response.schema.json` requires `feedback_id` with `minLength: 1`, so a hand-built seed's repeat answered outside the frozen shape. `repeatFeedback` now mints the id it holds the row under, once, and answers that id on later repeats. The `verdict`/`counts` computation also moved below the repeat check, which is what the note already said the code did but did not; a classification whose result was discarded is gone. A duplicated assertion block in the seeded test is removed.

Findings, none blocking, none fixed here:

- (Medium) A repeat whose new words read as a *different* verdict is answered with the recorded one, `accepted: true`, and the new words are dropped: "arrived damaged" for a purchase whose feedback says it fits well now answers `fits_well` instead of the `memory.ErrKindMismatch` a first seller delivery still raises. Rule 2 holds — nothing new counts toward a belief and no outcome becomes one — but the caller is told the layer accepted words it did not record, and `record_feedback` cannot record `damaged_or_defective` or `not_arrived` at all. The same shape covers the verdicts that record nothing: an `unclear` or `changed_mind` first delivery marks no row, so every resend mints a fresh `fb_` id. One decision fixes both — what a second delivery for one purchase means — and it is a contract question, not a small edit. **Owner: T8.5o**, a new task over `record_feedback`'s second delivery, which took the orchestrator's decision and closed it: the purchase's recorded verdict decides a repeat, a different verdict is refused with `ErrFeedbackConflict` and writes nothing, seller verdicts record against the seller belief, and an `unclear` or `changed_mind` first delivery keeps its one id across resends.
- (Low) `tests/live/layer/main.go`'s feedback step is ungated, as the note says: `go vet` and `go test` compile it, nothing runs its `main`. It is one first delivery for a purchase the seed does not hold, so this commit leaves its answer alone. T8.5i owns `tests/live/layer/`; no fix is needed and none is proposed.
- (Low) T8.5h's review note says the boot seed "marks every row it carries"; the frozen ledger fixtures carry `blf_2b9d60`/`ev_51c2d8` and `blf_1a5e09`/`ev_9d4c22` as `seeded: false`, and `LoadSeed` copies a row's mark rather than forcing it. Those two rows were recorded live in the run they came from, so marking them true would misreport the demo, but the note's general claim is wider than the tree and rule 7's wording is absolute. **Owner: T8.5p** (decide whether the boot seed's live-origin rows carry the mark, and correct the note either way), which decided the mark records origin: both rows stay unmarked as lived sandbox events, and the note now says the seed carries each row's recorded mark.

Gate green in the review worktree after `npm ci` in `web/` and `merchant/`: `gofmt`, `go vet`, `go test -race` over every Go package, both lockfile checks, both Svelte checks, both lints, both test runs and both builds, ending `OK all checks passed`. Spend $0.00: no live call, no key.

---

### T8.5m: A correction id is never reused
```yaml
requires:   [T8.5d]
fixture-ok: yes
size:       S · mid
owns:       internal/api/server/, internal/standing/
status:     done
```
The T9.7 live run found `restoreStanding` restores the journal's correction ids but not the mint sequence, so after a restart the next `record_correction` minted `corr_000001` over a restored one. A correction id must identify one correction for the life of the store.

**Done when:** the mint sequence resumes past every restored id (or ids are minted from a source that cannot collide); a restart test proves the next correction gets a fresh id and the restored one still resolves; `explain_belief` lists each correction once after a restore (the T9.7 review saw seven rows for five ids, because `restoreStanding` and `LoadSeed` both append the seeded entry and a reused id is appended again); the gate passes.

**Notes (in-review).** One insertion point now owns a correction id. `shopperState.takeCorrection` records a correction the first time its id is seen and does nothing on a second sighting, so the restored view and the seed that replays it stay one correction in one place in the arrival order; `reserveID` reads the number out of a minted shape (`corr_000001`) and holds the prefix's sequence past it, so the next mint starts where the store left off. An opaque fixture id (`corr_3b7e52`) names no number and reserves nothing, which is why the seeded demo still mints from one. `restoreStanding`, `LoadSeed` and both record paths insert through it, and `mint`'s comment now says what holds the sequence.

The read has its own guard: a journal written before this fix can already name one id twice, the restore collapses it at insertion, and `correctionsFor` walks the order newest first, keeps one row per id and serves the same list to `explain_belief` and `ledger_belief`, so no order that names an id twice can double a row. RULE 4 is unchanged: a restored correction still declares against its rewritten belief by its journal entry, still wins recall, and the belief still stays on the ledger.

`TestRestoredCorrectionIDIsNeverMintedAgain` records a declared correction, restarts over the same standing directory, proves the restored id is the one that decides, then records a second correction: it gets `corr_000002`, the restored view still reads its own words against its own belief, and the order holds two ids. `TestRestartAndReseedListEachCorrectionOnce` seeds, records a live correction, restarts, replays the seed and asserts `explain_belief` and `ledger_belief` list the seeded and the live correction once each, with the count equal to the store's distinct corrections. `TestExplainBeliefListsARepeatedOrderOnce` pushes a duplicate id straight into the order and asserts the listing still shows one row. All three fail on the code before this task; the correction and precedence tests pass unchanged.

Review added `TestRestoredJournalOfSevenRowsServesFive`, which feeds the T9.7 run's own journal — five ids in seven rows, the rows `demo/evidence/run-6/02-belief-explained.json` served — through the restore and the seed replay: both listings serve five rows in the run's newest-first order, the timeline still holds the seed's one correction event, the next correction is `corr_000005`, the journal goes back to disk with six distinct rows after that write, and a second sighting of an id keeps the words already on file. It pins the insertion guard, which the three tests above left unpinned: with `takeCorrection`'s first-sighting return removed all three still pass, because the read collapses the rows.

Two things the next agent should not have to rediscover: the duplicate row came from the seed replaying beside the restored journal, since `LoadSeed` after a restart proceeds — beliefs are not restored, so `ErrAlreadySeeded` never fires — and appends the seeded corrections again, while the id reuse came from the restore dropping the sequence; and the arrival order is still the newest-first sort key for both listings, so a duplicate is a wrong row rather than a wrong sequence.

---

### T8.7b: Live payment events reach the layer
```yaml
requires:   [T8.7, T8.5]
fixture-ok: no
size:       M · frontier
owns:       cmd/memento/, internal/payments/, internal/ingest/, deploy/paypal-webhook.sh
status:     not-started
```
The T9.7 run found the served layer cannot ingest a live PayPal outcome: the verifier is real, but `cmd/memento` builds the ingest service with the recording demo source, and the PayPal app's only webhook subscription does not point at the layer. So a new purchase or dispute cannot be recorded live; the demo re-read the existing capture and wrote feedback through the layer instead. Build the real REST source (order, capture, refund and dispute reads) behind the existing seam, wire it in `cmd/memento`, point the sandbox app's subscription at the public webhook route, and prove one live delivery end to end. Budget: no model spend; sandbox calls only.

**Done when:** a real sandbox capture and a real refund delivery reach the layer, write their evidence and fit rows, and are idempotent on redelivery; an unknown event is refused; the gate passes.

---

### T8.5n: Unattached ingest rows carry a write_op
```yaml
requires:   [T1.1b]
fixture-ok: yes
size:       XS · mid
owns:       internal/api/server/
status:     done
```
The T1.1b review found an unknown-code dispute is stored, is kept in the ledger (its reason is non-nil), and is emitted with the zero `write_op`, which the frozen schema forbids (`record_action`, `derive` or `observe`) and the web decoder rejects, so `ledger_evidence` fails to decode for that shopper. `markUnattached` already sets `observe` for a changed-mind refund; the ingest store path does not. Also decide whether a code-less dispute (stored, no reason, skipped by the ledger) should appear unattached or remain audit-only, and make the comment match.

**Done when:** every unattached ingest row carries `observe`; the frozen schema and the web decoder accept it; the code-less dispute's visibility is decided and tested; the gate passes.

**Notes (T8.5n, in-review).** The ingest store path now calls `markUnattached` for every row that counts toward nothing, so an unknown-code dispute is stored with `write_op: observe` instead of the zero value. The row keeps the source's code as its reason text with category `none`, counts toward nothing, links no belief, and still appears in `ledger_evidence` unattached.

A code-less dispute stays audit-only: the outcome is stored with no reason, no belief link and `observe`, so no stored ingest row carries a zero write op, but `ledger_evidence` still leaves it out. It has nothing to show beside the outcome and nothing to ask: a refund with no reason waits in pending questions, while a dispute with no code has no shopper answer to collect, so it stays an audit row. `markUnattached` and `LedgerEvidence` now say exactly that, and the ledger's filter is unchanged: a row with no reason and no belief link stays out.

A second decode failure sat beside the write op. `LedgerEvidence` copied an unattached row's belief links with `append([]string(nil), ...)`, which collapses an empty list to `nil` and marshals `belief_ids: null`; the frozen row requires an array and the web decoder's `Array.isArray` rejects null, so a changed-mind refund or a direction-mismatch answer could already fail to decode. The copy now goes through a non-nil slice and an unattached row emits `[]`.

Tests. `TestUnknownDisputeCodeRidesAsObservation` ingests a dispute whose code is outside the frozen table and pins the emitted row: reason text is the code, category `none`, `counts_toward` null, `write_op` `observe`, no belief link; it then marshals the response and checks the emitted `write_op` against the frozen schema's enum (`testdata/api/common.schema.json`, the same three values as `web/src/lib/api/decode.ts`'s `writeOps`) and `belief_ids` against `[]`. `TestCodeLessDisputeStaysAuditOnly` ingests a dispute with no code and pins the decided shape: stored, no reason, `observe`, no belief link, no belief born, and no ledger row. Both fail on the pre-fix tree: `write_op = ""`, plus `belief_ids = null` in the first.

No fixture moves: every frozen evidence row is attached to a belief, so none describes an unattached row. Gate green (`OK all checks passed`) after `npm ci` in `web/` and `merchant/`. Spend $0.00: no live call, no key.

**Review (fresh eyes).** APPROVE, with the fixes below committed on the review branch, all inside `owns`. Re-verified from the tree, no live call, $0.

Scope holds: against `main` the change touches only `internal/api/server/` and this task's lines, and no fixture moved. The unknown-code dispute now lands `observe` with its code as reason text and no link, the attached reasoned rows keep `record_action`, and the code-less dispute is stored with no reason, `observe` and no link, and stays out of the ledger. The ledger's visibility rule is the reason-nil and no-link test, not `write_op`, and only a refund files a pending question, so the audit-only decision hides nothing the shopper could answer.

Two store paths did not name a write op when they stored a row. `storeIngestResult` created the record and set the op only after `attachRow`, and `RecordReason` minted the row for a question whose event stored no evidence and set the op only further down. A failed attach — a refused graph write, a failed fit write, a canceled request — left a served row with the zero op, exactly the schema break this task closes. Both rows are now created as observations; an attach still upgrades them to `record_action`. `TestStoredIngestRowCarriesItsOwnWriteOp` and `TestStoredAnswerRowCarriesItsOwnWriteOp` drive a refused attach on each path and read the stored row; either default removed fails its test with `stored write_op = "", want observe`.

The task's `continue` was unpinned. Removing it runs `markUnattached` over an attached dispute row and turns it into `observe` with no link, with the whole server package still green. `TestUnattachedRowsDecode` now ingests the not-arrived dispute and pins `record_action` beside its one link; removing the `continue` fails it on both assertions.

The `belief_ids` fix was only pinned by the new dispute shape. `TestUnattachedRowsDecode` also drives the two rows that were already served before this task — a changed-mind refund and a direction-mismatch answer — and checks each marshals `belief_ids: []` and a write op from the frozen enum, in the same response as an attached answer that keeps `record_action` and its link. Reverting the non-nil copy fails on `changed_mind` and `fit_too_large` with `belief_ids marshal null`.

Mutation-checked, each reverted: the pre-fix tree fails all five tests with the symptoms above; the non-nil copy, either creation default and the `continue` each fail their own test. Gate green in the review worktree after `npm ci` in `web/` and `merchant/`, ending `OK all checks passed`. Spend $0.00: no live call, no key.

---

### T8.5o: A second feedback with different words
```yaml
requires:   [T8.5l]
fixture-ok: yes
size:       S · mid
owns:       internal/api/server/, internal/api/contract/, testdata/api/, docs/interface.md, docs/architecture.md, agent/runtime/agent/, agent/second/agent/, dev-diary/PHASE-8-pivot-layer.md
status:     done
```
A repeat feedback whose new words read as a different verdict (another fit verdict, `changed_mind`, or a seller verdict) is answered with the recorded one and `accepted: true`, silently dropping the new words; `record_feedback` cannot record `damaged_or_defective`/`not_arrived` at all. The same shape: an `unclear`/`changed_mind` first delivery marks no row, so every resend mints a fresh `fb_` id. Decide and implement: refuse the conflicting repeat honestly, record it as a second distinct feedback, or widen the tool — the response shape is frozen, so this is the orchestrator's call.

**Done when:** a repeat with different words gets a decided, honest answer (not a silent drop); an `unclear` first delivery is idempotent on resend; tests pin both; the gate passes.

**Notes (in-review).** The decision: a purchase holds one feedback. A second delivery is judged by the verdict its new words read as, and the purchase's recorded verdict decides what the layer does with it. That keeps the response shape frozen — `accepted` is the one field that can say whether anything counted — while making every answer true of the store.

1. **Seller verdicts are seller evidence, not fit feedback.** `damaged_or_defective` and `not_arrived` resolve to the seller belief through `ensureSellerBelief`, the helper `beliefForReason`'s seller branch calls (the brief's `sellerBeliefFor`; there is no helper under that name), and the purchase's row is written through the reason path with the category the verdict names. They answer `accepted: true`, `counts_toward: seller_reliability` and the seller `belief_id`. `beliefForFeedback` runs only for a fit verdict, and `feedbackFitRow` is reached only when the verdict counts toward fit, so a seller verdict creates or joins no fit belief and settles no fit row. This is the same honesty rule `record_reason` already held: the verdict is the reason, and the belief it counts toward must match it.

2. **A repeat of the recorded verdict changes nothing.** It answers the first `feedback_id`, the recorded `verdict`, `counts_toward`, `belief_id`, and writes no graph action and no evidence row; the purchase still settles its one fit row when the recorded verdict is a fit verdict. `accepted` now echoes whether that verdict counts toward anything rather than being true by construction.

3. **A repeat whose words read as a different verdict is refused with `ErrFeedbackConflict`.** It is wrapped with the purchase and both verdicts, so the message says the purchase already holds a recorded verdict and which one; `errors.Is` matches the sentinel. Nothing is minted, nothing is written, no belief is created and the fit row does not move. The sentinel maps to an invalid request (400) beside `ErrFeedbackMismatch`, so the caller reads a refusal, not a server fault. Rule 2 holds: the new words never become evidence.

4. **A first delivery that counts toward nothing is recorded on the purchase.** `unclear` and `changed_mind` mint one `fb_` id and mark the purchase's capture row with the verdict — the same `recordedFeedback` marker T8.5l put there — so every resend answers that id with `accepted: false` and `counts_toward: null`. A resend whose words read as a different verdict is a conflict (shape 3), including after a no-count first delivery.

The tests are in `internal/api/server/feedback_test.go`: `TestSellerFeedbackWritesSellerEvidence` pins the seller response, the seller belief, the purchase row's seller category and link, one graph action citing the seller belief, no fit belief, and both the fit rows and the fit index unchanged before and after the first delivery and its repeat; `TestSameVerdictRepeatIsIdempotent` tables a fit and a seller verdict and pins one id, one belief, one mint and no action on the repeat; `TestConflictingFeedbackIsRefused` covers a fresh purchase, the seeded purchase and a no-count first delivery, asserting `errors.Is`, the "already holds a recorded" text, and the store itself — actions, derives, beliefs, evidence links and reasons, feedback marker, mint count and fit rows all unchanged; `TestNoCountFeedbackIsIdempotentOnResend` tables unclear and changed-mind and pins the one id and `accepted: false` across three sends, with no action, no belief and the fit row untouched.

On the parent commit these fail at exactly the defects: a seller delivery errors with `memory: evidence counts toward a different belief kind: feedback counts toward "seller_reliability", offered to "fit"` after already deriving a fit belief; a conflicting repeat answers `accepted: true` with the recorded verdict and no error; and a no-count delivery answers an id it never records, so its first resend mints a second.

Neither response shape changed and no example changed: the frozen `record_feedback` pair is a fits-well repeat, still `accepted: true` on `fb_51c2d8`, so `TestInterfaceDocExamples` stays byte-exact. The review follow-up updates the request schema's description, with no property, required entry or type changed. The `recordedFeedback` marker and the new sentinel are internal to `internal/api/server/`.

**Follow-up (orchestrator).** The `owns` line now also covers `internal/api/contract/types.go` and `docs/interface.md`, and both state the three shapes: the response comment names the fit, seller and no-count verdicts and the repeat and conflict rules, and the interface document's `record_feedback` section says the same without moving any `validate=` block. T8.5l's finding now names T8.5o as its owner and says this task closes it. The second task commit is comment and prose only: no type, field, tag or schema shape changed, and `TestInterfaceDocExamples` stays byte-exact. The review follow-up updates the tool descriptions and the request schema's description, with no property, required entry, type or example changed. The ungated `tests/live/layer/main.go` feedback step is one first fits-well delivery on a purchase the seed does not hold, so it reads the same answer it always did.

Gate green after `npm ci` in `web/` and `merchant/` (neither was populated in this worktree), ending `OK all checks passed`. Spend $0.00: no live call, no key.

**Review (fresh eyes).** APPROVE, with the fixes below committed on the review branch, all inside the `owns` line the orchestrator widened for the follow-up. Re-verified from the tree, not from these notes; no live call, $0. Scope holds: against the task tip (`2f865a7`) the task touches only `internal/api/server/`, `internal/api/contract/` (comments and the one `record_feedback` description), `testdata/api/record_feedback.request.schema.json` (description only), `docs/interface.md` and `docs/architecture.md` (prose), `agent/runtime/agent/tools.ts` and `agent/second/agent/tools.ts` (the `record_feedback` description and `feedback_text` field description), and this file's T8.5o/T8.5l lines. No response shape, schema property, required entry, type or example changed.

The four shapes hold. A seller verdict takes `ensureSellerBelief` and the evidence-row write `writeEvidenceRowAt` (the same path `writeEvidenceRow` delegates to), the same belief helper `record_reason`'s seller branch uses, with the domain category `damaged_or_defective` or `not_arrived`; the response's `counts_toward` and `belief_id` are the seller belief the stored row names, and the row's `beliefIDs` and the belief's `evIDs` agree. It creates or joins no fit belief and puts no fit row: `TestSellerFeedbackWritesSellerEvidence` reads `st.beliefs`, the purchase row and the fit index's rows and `puts`, not only the response. A same-verdict repeat answers the first id and writes no action. A different verdict returns `ErrFeedbackConflict` before `st.mint` and before any write; the conflict cases read the store — actions, derives where one existed, beliefs, links, reasons, marker, mint sequence and fit rows — after the refusal. A no-count first delivery mints one id and marks the capture row, so a resend answers that id with `accepted: false` and `counts_toward: null`, and a different-verdict resend after it is the same conflict. The sentinel maps to 400, now pinned by `TestFeedbackConflictIsRefusedAsAnInvalidRequest`.

Mutation-checked, each reverted: moving the conflict check below `st.mint("fb")` fails all three conflict cases on the mint count; removing the no-count marker fails both no-count cases at `purchase holds <nil>`; setting the repeat's `accepted` back to true fails at `accepted true ... want accepted false`; routing a seller verdict through `beliefForFeedback` fails on the fit belief; dropping `ErrFeedbackConflict` from `isInvalid` fails the new mapping test; using the capture time for the seller write fails the new last-change assertion. Against the parent (`2f865a7`, with only the new sentinel added so the file compiles) the seller cases fail with the `ErrKindMismatch` the note names, the conflict cases answer `accepted: true`, and the no-count cases answer an id the purchase never records.

Fixes committed on the review branch:

- The seller write dated the belief's last change at the purchase's capture time, before the belief's own creation in the test clock, while the fit path uses the feedback time. `writeEvidenceRow` now delegates to `writeEvidenceRowAt`, which takes the event time; the seller branch passes the delivery's `now`, and the reason and ingest paths keep the row's own time. `TestSellerFeedbackWritesSellerEvidence` pins it; reverting the call fails it.
- Comments and prose that still said feedback is fit-only, or that a changed-mind answer asks again: `FeedbackVerdict`, `RecordFeedbackRequest`, `TimelineFeedback`, `TimelineEvidence` and the response comment in `internal/api/contract/types.go`; `evidenceRecord.Counts`, `recordedFeedback` and `ErrFeedbackMismatch` in `server.go`; the `Seed.FeedbackIDs` comment, which still said a seed naming no id answers with none — the T8.5l behaviour this task closes; and `docs/interface.md`'s opening, no-count sentence, write table and ledger note.

**Orchestrator call on the review finding.** Folded into this task rather than deferred. The `owns` line now covers `internal/api/contract/`, `testdata/api/`, `docs/architecture.md` and both agents' `agent/` directories. `ToolRecordFeedback`'s doc comment and `Tools()` description, the two copied descriptions and their `feedback_text` field descriptions, the request schema's description and `docs/architecture.md`'s structural-write sentence now state the fit, seller and no-count verdicts and the repeat/conflict rule. Every other tool description is byte-identical; the schema keeps every property, required entry and type, and `TestInterfaceDocExamples` stays byte-exact. `agent/loops/loops.go`'s `Feedback` comment still says fit feedback and remains outside `owns`; it is not a machine description an agent reads from `tools/list`.

The deployed agents still carry the old descriptions until they are redeployed; this branch made no redeploy. The exact commands, each from the repository root after the layer lands:

Reference agent (`memento-agent`):

```sh
cd agent/runtime
bun agent/offline-check.ts
ast spec validate
ast push -V private -y
ast agent redeploy --id <deployment-id> --latest \
  --var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY \
  --var CHANNEL3_API_KEY=@CHANNEL3_API_KEY --wait
```

Second agent (`memento-second`):

```sh
cd agent/second
bun agent/check.ts
ast spec validate
ast push -V private -y
ast agent redeploy --id <deployment-id> --latest --var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY --wait
```

The deployment ids stay out of the diary; `<deployment-id>` is the operator's own id. Both packages' offline checks and `./node_modules/.bin/tsc --noEmit -p tsconfig.json` pass on this branch.

Gate green in the review worktree after `npm ci` in `web/` and `merchant/` and `bun install --frozen-lockfile` in both agent packages; the gate ran both agents' offline checks and both typechecks, ending `OK all checks passed`. Spend $0.00: no live call, no key.

---

### T8.5p: Decide the seeded mark on the live-origin ledger rows
```yaml
requires:   [T8.5h]
fixture-ok: yes
size:       XS · mid
owns:       testdata/api/, dev-diary/PHASE-8-pivot-layer.md
status:     done
```
The T8.5h review note says the boot seed "marks every row it carries", but `blf_2b9d60`/`ev_51c2d8` and `blf_1a5e09`/`ev_9d4c22` are `seeded: false` in the frozen ledger fixtures, and `LoadSeed` copies the mark. Decide whether those live-origin rows should be marked (rule 7 says seeded means marked) or whether `seeded: false` is the deliberate record of their origin, correct the fixtures and the note accordingly, and keep the doc pin byte-exact.

**Done when:** the mark on every frozen ledger row is the recorded decision, the T8.5h note matches, and the gate passes.

**Notes (in-review).** The decision: `seeded: false` on both rows is the deliberate record of their origin, not a gap the boot seed left. No file in `testdata/api/` changes, because every frozen ledger row already carries the mark its origin gives it.

The rule's subject is generated history, not the loader's reach. `domain.SeedMark`'s comment states it — "Seeded history is generated, not lived" — and every carrier repeats it: the `Seeded` comments on `contract`'s evidence, belief and question views and on `domain.Evidence` and `domain.FitRow` all read "Seeded marks generated history"; `product.md` says the payment history has to be generated and seeded rows are marked and easy to find; and the frozen notice this fixture set carries reads "Seeded history: generated, not lived. The three returns are seeded; the correction is live." Rule 7 binds a seeded row to the mark. A row the shopper lived is not a seeded row, and marking it true is the misreport the rule exists to prevent.

Both rows are lived. `ev_51c2d8` is the capture `memento-shopper01-06` recorded in an earlier sandbox cycle (SKU `PSW8GeK`, size M), and `ev_9d4c22` is the open not-arrived dispute on `ORD-7XK-04`; the three October returns are the generated history. The fixture set therefore splits the way the history happened: the three returns and the derived statement are marked as generated, while the capture, the dispute and the pending refund `q_9f04ab` stay unmarked as lived.

The audit, row by row:

- `ledger_beliefs.response.example.json` holds four belief rows and five embedded evidence rows. `blf_8f2c41` is marked with three marked returns (`ev_6d1a09/10/11`), `blf_2b9d60` is unmarked with its one unmarked capture, `blf_1a5e09` is unmarked with its one unmarked dispute, `blf_77c310` is marked and evidenceless. Each belief's mark agrees with its own rows.
- `ledger_evidence.response.example.json` carries the same five evidence rows newest first under the same marks, each attached to the belief whose mark matches.
- `ledger_belief.response.example.json` carries `blf_8f2c41` marked, with its three marked returns, and the seeded notice; `explain_belief.response.example.json` carries the same belief and rows without the notice, which its response schema does not have.
- `pending_questions.response.example.json` carries `q_9f04ab` unmarked: the live refund still waiting for its reason.
- `recall_for_purchase.response.example.json` and `-correction.json` carry `blf_8f2c41` with its three marked returns; `-own.json` carries `shopper_04`'s two generated returns and their belief marked; `-pool.json` carries the derived statement marked; `-none.json` carries no row.
- The `product_fit`, `set_pooling` and `record_*` examples carry no row marks; the pool examples' `seeded` blocks are counts of the seeded population, not marks on a row.

The served path copies the marks rather than forcing them: `LoadSeed` builds each belief and evidence record with the view's own `Seeded`, so the seed replay serves `seeded: false` on both. A flip of the four marks in a scratch copy (nothing committed, nothing live) left `server_test.go`'s `compareExample` green, because `SeedFromExamples` reads the same fixture the comparison expects: that test pins the copy, not the value. It failed every check that does pin the value, none of them inside this task's `owns`. `TestLedgerReadsNarrow` asserts an unseeded-only read holds exactly two beliefs, and those two are these, so marking either true shrinks that read and fails the filter test. `TestInterfaceDocExamples` holds `docs/interface.md`'s blocks byte-identical to the fixtures. `internal/fakes`' `TestReplayMatchesContract` holds the replay's ledger, `testdata/replay/p4-ledger.json`, equal to the same api examples. `internal/fakes/final.go` carries the same two marks and `TestReplayMatchesFixtures` binds its replay output to that ledger. The web's own fixture mirror holds the last copy: `web/src/lib/api/api.test.ts` compares `fixtureFinalBeliefs` and `fixtureFinalEvidence` against these files. The mark therefore lives in the api fixtures, the replay snapshot, the fake and the web mirror, and a change lands in all four or fails the build.

The T8.5h note now matches. Its corrected clause says the seed copies each row's recorded mark and carries its one statement row marked, because a statement is generated history and never a lived event; the clause no longer claims the seed marks every row it carries. T8.5l's finding now names this task as its owner and the decision.

The doc pin: no fixture changed, so `TestInterfaceDocExamples` still compares the document's blocks byte-for-byte against the files and passes; the document already recorded the split in words ("the three October returns are seeded, the dispute, the pending refund, the correction and the feedback are live"). Gate green after `npm ci` in `web/` and `merchant/` (neither was populated in this worktree). Spend $0.00: no live call, no key.

**Review (fresh eyes).** APPROVE, with three corrections committed on the review branch, all inside `owns`. Re-verified from the tree, not from these notes; no live call, $0. Scope holds: against the task tip (`5868762`) the commit touches only this file and no fixture changed (`main` has since moved past the tip, which is what makes a `main..HEAD` diff look wider).

The decision holds against the mark's meaning. `domain.SeedMark`'s comment, the three `Seeded` comments in `internal/api/contract`, the two in `internal/domain`, `product.md`'s "Seeded rows are marked and easy to find" and the web ledger's own tooltip (`SEEDED_MARK_TOOLTIP` = "Seeded history: generated, not lived.") all say the same thing, and `LoadSeed` copies `view.Seeded` into both records (`seed.go`), so nothing about the loader forces a mark. The two rows' origin is recorded outside these fixtures: `demo/README.md` calls `ev_51c2d8` "recorded in an earlier sandbox cycle", `demo/RUN-6.md` calls `ev_9d4c22` "recorded in an earlier cycle", and `docs/interface.md` states the split in words. The tree had already committed to this reading where it bites: `TestLedgerReadsNarrow` asserts the unseeded-only read of these fixtures holds exactly these two beliefs.

The row audit reproduces, file by file: four beliefs and five embedded evidence rows in `ledger_beliefs` (3+1+1+0) whose marks agree with their own rows; the same five in `ledger_evidence`, newest first (11-08, 11-06, 10-26, 10-20, 10-14), each linked to the belief whose mark matches; `blf_8f2c41` marked in `ledger_belief`, `explain_belief` and both base recall examples with its three marked returns; `shopper_04`'s two returns and their belief marked in `-own`; `blf_4c2a71` marked in `-pool`; no row in `-none`; `q_9f04ab` unmarked in `pending_questions`; and no belief, evidence or question view at all in the `product_fit`, `set_pooling` and `record_*` examples, so their `seeded` blocks are `PoolCounts`, not marks.

Three corrections. (Medium) The notes said `explain_belief`'s fixture carries the seeded notice; it carries no such field — its schema requires `belief`, `corrections` and `timeline` only — and `ledger_belief` is the one example with `seeded_notice`. (Medium) The notes said `compareExample` pins the two marks; a flip of all four marks in a scratch copy (nothing committed) left it green, because the seed reads the fixture the comparison expects, so the paragraph now names what does pin the value — `TestLedgerReadsNarrow`, `TestInterfaceDocExamples`, `internal/fakes`' `TestReplayMatchesContract` with `TestReplayMatchesFixtures`, and the web suite's fixture comparisons — and records where else the mark is copied. (Low) T8.5h's corrected clause said "a statement reaches memory only through `LoadSeed`", which is wider than the tree: `SetPooling` calls `memory.RecordStatement` for the pooling-choice note (`pool.go`), a note that never becomes a served statement belief, so the clause now reads "a statement belief".

Limits. In the offline replay all six rows arrive through the same payment leg (`testdata/replay/p2-inputs.json`), so "lived" here means "arrived as an event rather than generated history" — the reading `docs/interface.md` records, and the fixture set carries no other field that separates the two origins.

Gate green in the review worktree after `npm ci` in `web/` and `merchant/` (neither was populated here), ending `OK all checks passed`; the pinning tests were also run alone and pass there, and the flipped copy in the scratch tree failed `TestLedgerReadsNarrow`, `TestInterfaceDocExamples`, `TestReplayMatchesContract` and the web suite's fixture comparisons. Spend $0.00: no live call, no key.

---

### T8.5q: Run the precedence driver once with the mark assertion
```yaml
requires:   [T8.5j]
fixture-ok: no
size:       XS · mid
owns:       tests/live/precedence/
status:     claimed:t85q-live
```
T8.5j's Done-when says a live run still passes after the harness marks its seeded statement. The offline check proves the mark path up to the served belief, but only a live run proves the seeded statement reaches `Recall` on the real hub. Run `tests/live/precedence/run.sh` once with the keys loaded, paste the transcript (redacted), and record the spend. Budget: under $0.20 of embedding spend (the fixture embedder spends $0).

**Done when:** the live run passes with the mark assertion active; the transcript is recorded; test indices are erased and spend recorded.

---

### T8.5r: A pool with no direction never decides recall
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       S · mid
owns:       internal/api/server/
status:     done
```
The P7 review found `tools.go` sets `presence.Pool` from the pool view alone, so a Building pool with no leading direction wins precedence step 3 and `guidanceForPool` reads "no direction" as "runs large": four distinct fits-well shoppers with no returns and a shopper with no history produce "Size down to S for brand-x-001: 0 of the 4 fit-classified settled purchases at size M came back too large, against 0 too small." A level Building pool (2/2) takes the same path. product.md says the pool decides only when it says the product runs off-size.

**Done when:** a pool with no direction never decides a recommendation (the next step decides, or the pool rides as context without a size); both counts (0/4 and 2/2) are regression cases; the guidance text cannot read a missing direction as a direction; the gate passes.

**Notes (in-review).** `lookupPool` now yields a `*poolStep` only when the pool read names a direction: a Claimed pool, a Building pool with the directions level, and a Building pool whose only fit outcomes are fits-well are still consulted but yield no step. That step is the single source of both the pool's presence in precedence and its guidance: presence is `step != nil`, the served view rides in `step.view`, and `guidanceForPool` now takes the direction as an argument and answers with the nothing-applies guidance when it is none, so no path can read a missing direction as "runs large". `internal/api/server/pool_step_test.go` pins the four served shapes (four fits-well shoppers with no returns, a level 2/2 pool, a Building small pool that still sizes up to L, and a statement-only shopper whose statement decides), the own-evidence case whose guidance no longer says "ahead of the pool" when the pool names nothing, and the guidance builder's guard directly. No fixture changed: every frozen recall example either carries a direction (Earned small) or no pool at all, so `TestInterfaceDocExamples` stayed byte-exact. One thing the next agent should not have to rediscover: `product_fit` still serves a directionless Building pool with `direction: null`, and that is intended — the merchant read reports the counts; only recall's precedence step requires a direction, and `pool_step_test.go` pins the asymmetry. The `precedence` package's own `Presence` doc now names the direction rule (its comment corrected in review; no shared type moved), while the gate itself stays at the server's call site.

**Review (fresh eyes).** APPROVE, with three corrections committed on the review branch: one comment inside `owns`, the allowed `precedence` comment, and this note's `product_fit` sentence, which a new test now pins. Re-verified from the tree, not from these notes; no live call, $0. Scope holds against the commit's parent (`ab47a81`): the commit touches only `internal/api/server/` and this task's lines, and no fixture or `docs/interface.md` example moved. Every frozen recall example carries a direction (`small`) or no pool at all, so `TestInterfaceDocExamples` runs byte-exact and green.

The gate holds in both directions. Restoring the old presence rule (dropping the direction check in `lookupPool`) fails all four directionless-pool recall tests, the Building-small test still passing; restoring the old read of a missing direction (the `guidanceForPool` default sizing down) fails the builder test with the exact "Size down to S for brand-x-001: 2 of the 4 ... came back too large, against 2 too small." sentence the P7 review found. No reachable pool state with a real direction is dropped: `domain.DecidePool` answers Claimed with no direction below the Building floor, Building with `leadingDirection` (small, large, or none when the two are level), and Earned or Out of true always with the earned direction. Claimed-with-a-direction is therefore unreachable, and if a future rule produced it the guard would still keep it absent, which matches product.md's "below 3 shoppers the pool says nothing".

Corrections. (Medium) `precedence.Presence`'s `Pool` doc said a Building label alone made the pool present, which the gate now contradicts; it names the direction rule instead. (Low) `assembleRecall`'s comment said a consulted-and-absent pool meant a Claimed pool only; it now names the directionless Building case too. (Low) the note's "a Building pool with `direction: null`" read as if the frozen Building example (`product_fit.response.example-building.json`, which is Building small) were directionless; it now says a directionless Building pool, and a new test pins `product_fit`'s answer for it while recall serves null.

The contract's `PoolConsulted` comment stays as it was: "the pool has something to say" now means the pool names a direction, so the sentence is not made false, and tightening it is outside this task's `owns`. Limits: the gate's two bun packages were skipped here because bun and their `node_modules` are absent, the same workstation limit the script prints; the pool path is proved offline against the fake index, with no live read. Hygiene: plain commit message, no key, account detail, IP, personal path or date in the diary.

---

### T8.5s: A boot replay's repeated correction write is harmless
```yaml
requires:   [T8.5m]
fixture-ok: no
size:       XS · mid
owns:       tests/live/hub/, internal/api/server/, product.md, docs/architecture.md, dev-diary/PHASE-8-pivot-layer.md
status:     done
```
The T8.5m review found `LoadSeed` calls `memory.RecordCorrection` before `takeCorrection` decides the id is already restored, so every boot re-issues that correction's graph write even when the journal holds it. No served-state effect offline; prove it against the live hub and either make the boot skip the write for a restored id or record that the repeat is harmless and why.

**Done when:** a live boot replay shows whether the correction's graph write is duplicated, and the boot either skips it or the harmlessness is recorded; the gate passes.

**Notes (in-review).** The boot replay keeps writing the seed's correction, and the repeat is harmless: Lambo reinforces an identical action in place instead of duplicating it. The T8.5m review flagged `LoadSeed`'s `memory.RecordCorrection` for a correction the journal already restored as a re-issued graph write, and this task first skipped it. A probe of Lambo 0.10.1 — the release the deploy pins, with the fixture embedder and a SQLite store — recorded one action three times with identical content and event time, `depends_on` citing one belief. The settled receipt for a repeat read `recorded action: 0 concept(s) created (0 embedded), 0 edge(s)`; the belief read back `blast radius: 1`, `dependents (1)`; the action's dependency edge carried `reinforcements: 3` after the three writes, created at 1 and incremented once per repeat. The premise was wrong: a repeat resolves the one action node and the one dependency edge with its dependent, reinforced, and opens one interaction (with its temporal edge) per call. The skip and the test that pinned it are reverted, and a comment at the call records the mechanism.

The same holds for the seed's five evidence actions, which the replay re-issues every boot: four `WriteEvidence` writes for the three returns and the dispute and one `WriteFeedback` for the capture are identical each lap (each stub's doc id is the same, restored from the evidence journal or re-minted in the same order), so each reinforces its action, adds no dependent and leaves blast radius unchanged. A live hub restart would show the same shape — no dependent added and no blast-radius change — so this note records the mechanism and the probe (taken against the same 0.10.1 binary the deploy pins) as the answer to the duplication question. No live run was made: the Done-when's live clause is the orchestrator's call, and the gate passes.

The residual is honest: the repeat is not free. Every call opens an interaction (with its temporal edge) and increments the action's reinforcement counter, so interactions, their temporal edges and reinforcements accumulate on each boot even though no dependent or blast radius moves. Persisted belief records remain the prerequisite for ever skipping writes safely: without them the replay cannot know the graph already holds a belief's edges, and a skip would silently drop them if the store were rebuilt while the journal survived. That is T2.3c's recorded limit, that restoring belief records needs its own task; T8.5s owns this finding and records it here, and `product.md` and `docs/architecture.md` state the measured behaviour.

**Orchestrator's acceptance (live clause).** The Done-when asked for a live boot replay. The mechanism is accepted in its place: the probe ran the same `lambo_record_action` the boot calls, on the same 0.10.1 release the deploy pins, and a repeat created no concept or edge and left the blast radius at one; the action contents the boot writes are stable across boots. A live restart would count the same reinforcements, and reaching the loopback reader in the deployed shell was not worth the ops risk here, so the live replay was not run. The README's P8 line now states the measured mechanism instead of promising a live proof.

Gate green after `npm ci` in `web/` and `merchant/`, ending `OK all checks passed`; the two bun checks (`agent/runtime`, `agent/second`) were skipped for their absent modules, the same workstation limit the script prints. Spend $0.00: no live call, no key.

---

### T8.5t: The demo driver expects thin evidence to be served
```yaml
requires:   [T8.5i]
fixture-ok: yes
size:       XS · mid
owns:       demo/driver/
status:     done
```
`demo/driver/main.go` pins the old withholding at its one-return step ("one return earns nothing…") and its final recall ("feedback grew a rival belief"), so it would fail against the served layer after T8.5i. Update the driver's assertions to the served thin belief, and keep `demo/run.py`'s evidence expectations consistent.

**Done when:** the demo driver's one-return and final-recall steps assert the served shape; the gate passes.

**Notes (in-review).** The one-return step now asserts the served shape T8.5i pinned: exactly one belief, Claimed/None (a label, never a gate), its one evidence row carrying the too-small reason, the `own_evidence` precedence step deciding and not overridden, and the guidance the driver's no-pool wiring produces — basis `own_evidence`, text "Take size L for Brand X". Its transcript line states the label and the sentence instead of "no belief, no guidance". The final recall after the fits-well feedback asserts two beliefs: the earned sizing belief first by its id, and the feedback's belief beside it, Claimed with its one row, ranked after the three-return belief. The old "feedback grew a rival belief" failure is gone.

`demo/run.py` already reads the served shape: it selects beliefs by id (`blf_8f2c41`, `blf_2b9d60`, `blf_1a5e09`) and records every served belief's precedence, so a thin belief beside them is recorded rather than tripping a count assertion. It needed no change, which is as well because it sits outside this task's `owns`.

No live call was made (the driver needs the hub binary, the deployment URL and keys). Proof without one: a temporary in-package test in `demo/driver/`, deleted before the commit, built the driver's service wiring on the fakes (no pool, the recording classifier and graph) and ran the new assertions verbatim — one return reads Claimed/None with 1 row and guidance "Take size L for Brand X"; the final recall serves the sizing belief then the Claimed fits-well belief with 1 row, guidance "Take size M for Brand X". The one-return line prints status and guidance but no belief id: that id is the sizing id, redacted only after the third answer, so printing it earlier would put it in the transcript unscrubbed.

Gate green after `npm ci` in `web/` and `merchant/`, ending `OK all checks passed`. Spend $0.00: no live call, no key.

**Review (fresh eyes).** APPROVE, with the two diagnostic fixes below committed on the review branch, both inside `owns`. Re-verified from the tree, no live call, $0.

Scope holds: against the task's base the commit touches only `demo/driver/main.go` and this task's lines. The one-return step asserts the shape T8.5i pinned — one belief, Claimed/None, its one too-small row, `own_evidence` deciding and not overridden, and the no-pool guidance "Take size L for Brand X" — and its transcript line states the label and the sentence. The final recall asserts the sizing belief first by its id and the fits-well belief Claimed with its one row; `recallBeliefs` sorts by evidence count, so three rows ahead of one is the rank the note claims.

A temporary in-package check in `demo/driver/` (deleted before the review commit) built the driver's own wiring — fresh store, recording classifier, the recording payment source behind `purchaseSource`, no pool, and a graph stub that reads None below three dependents and Venerable at three — and ran both assertion blocks verbatim: one return reads Claimed/None with 1 row and guidance "Take size L for Brand X", and the final recall serves the sizing belief then "size M fits this shopper now", Claimed with 1 row. It also read `thin.BeliefID == sizingID`, which is the whole reason the one-return line prints no id: the id is redacted only at the third answer, so printing it earlier would put it in the console transcript raw, and the final recall checks it where it can. The limit stands: nothing runs `main()`, `go vet` only compiles it, and T8.5u runs the layer harness rather than this demo driver, so a live demo run is what closes it.

Mutation-checked, each reverted: restoring the old emptiness check on the one-return step fails with "one return earns nothing, recall serves 1 beliefs"; putting the pre-T8.5i guard back in `recallBeliefs` (`len(view.Evidence) < 2`) fails the new assertion with "one return serves 0 beliefs".

Two diagnostic fixes. The first answer's own classification is now asserted where it is asked (accepted, no ask-again, too-small), because the new step depends on it and a classifier drift would otherwise surface as "one return serves 0 beliefs" and read as a layer defect. The final recall's count and order checks are separate, so a right-count/wrong-order run names the belief it got instead of reporting a count it did not get wrong.

Gate green in the review worktree after `npm ci` in `web/` and `merchant/`, ending `OK all checks passed`. The builder's report of one gate run failing at web vitest did not reproduce: the review ran the full web suite three times and the five web test files that poll a mounted component for up to 4 s five times each, all green, and the diary records no known flake. Nothing changed on that account; it is recorded here so the next run is not surprised. Spend $0.00: no live call, no key.

---

### T8.5u: Run the layer driver with thin evidence served
```yaml
requires:   [T8.5i]
fixture-ok: no
size:       XS · mid
owns:       tests/live/layer/
status:     not-started
```
T8.5i flipped `tests/live/layer`'s one-return tripwire but did not run it; only a live run proves the served thin belief and its guidance end to end. Run it once with the keys loaded, record the redacted transcript and the spend (the fixture embedder spends $0), and erase the test indices.

**Done when:** the live layer run passes with the thin-evidence assertion active; the transcript is recorded; indices erased and spend recorded.

---

### T8.5v: The own-evidence sentence tells the truth about the evidence
```yaml
requires:   [T8.5i]
fixture-ok: yes
size:       XS · mid
owns:       internal/api/server/
status:     done
```
The T8.5i review found `guidanceForOwnEvidence` defaults the direction clause to "runs small" unless the belief's value reads large, and `poolDisagrees` means "a pool is present and own evidence overrode it", not "the pool recommends a different size". A lone fits-well return is served (T8.5i) with the sentence "Take size M for Brand X: your own one return says it runs small, ahead of the pool." — the size is right but the sentence misstates the evidence and self-contradicts, and the "ahead of the pool" clause fires when the pool merely agrees. Make the clause derive from the belief's own value (fits-well, too small, too large) and say whether the pool agrees, is overridden, or is absent.

**Done when:** a fits-well own-evidence recall names fits-well and does not say "runs small"; a pool that agrees is not called overridden; the existing own-evidence texts for too-small/too-large are unchanged where correct; tests pin all three; the gate passes.

**Notes (in-review).** The sentence now reads both halves from the data they are about. `fitWords` maps a runs-small value to "runs small", a runs-large value to "runs large", and the emerging fits-well value ("size M fits this shopper now") to "fits well"; a value naming neither leaves the clause at "your own N returns say so", so no sentence invents a direction. `ownEvidenceClause` takes those words with the count, so a lone fits-well row reads "Take size M for Brand X: your own one purchase says it fits well, ahead of the pool." The noun follows the evidence too: the fits-well belief is backed by captures whose feedback said the size fits, never by returns, so that clause names purchases while every too-small and too-large text keeps "return" byte-identical.

`poolDisagrees` is gone. `poolAgrees` compares the size the pool recommends — its consulted size stepped one rung the way its direction points, the arithmetic `guidanceForPool` already uses — with the size the own evidence names. An agreeing pool reads ", and the pool agrees."; a pool recommending another size keeps the existing ", ahead of the pool."; with no pool the sentence stays the bare "Take size L for Brand X". The last shape is deliberate: the Done-when keeps the too-small and too-large texts unchanged where correct, and a missing pool is not a disagreement to announce. The response already says whether the pool was consulted (`pool` null, `pool_lookup` unavailable), so the sentence claims nothing false by staying silent, and the demo driver and the live layer harness keep the texts they pin.

A directionless Building pool agrees with nothing: it names no size, so `poolAgrees` is false and the sentence reads "ahead of the pool". That is the same pool the precedence step already leaves behind, and the size arithmetic is untouched.

Sizes and precedence are untouched. Every row of the new table also checks that a present pool is still the weaker step, marked overridden by the own evidence that decided, with or without agreement: only the prose distinguishes the two.

`TestOwnEvidenceSentenceTellsTheTruth` (new file `internal/api/server/guidance_test.go`) is a table over the six shapes: a lone fits-well row beside a pool that runs small; a lone too-small row beside a pool that agrees; a too-large row beside a pool that agrees; two too-large returns beside a pool that agrees; two too-large returns beside a pool that disagrees; and two too-large returns with no pool. Each row runs the real service (ingest, then `record_reason` or `record_feedback`, then `recall_for_purchase`) and asserts the exact sentence, the `own_evidence` basis, the phrases that must not appear, and the unchanged pool precedence where a pool rides. The agreeing rows use a new `earnedLargePool` — `earnedSmallPool` with the directions swapped — so the pool recommends the size the shopper's own rows do. On the parent commit the fits-well row fails with "your own one return says it runs small" and the three agreeing rows fail with "ahead of the pool", which is the finding; the disagreeing and no-pool rows pass before and after, which is the "unchanged where correct" half.

Fixtures and docs did not move: the only own-evidence fixture, `recall_for_purchase.response.example-own.json`, is a pool that disagrees, so its sentence is byte-identical, and `TestInterfaceDocExamples` passes against `docs/interface.md` unchanged. `fixture-ok: yes` was available and not needed.

Gate green after `npm ci` in `web/` and `merchant/` (neither was populated in this worktree): `gofmt`, `go vet`, `go test -race` over every Go package, both lockfile checks, both Svelte checks, both lints, both test runs and both builds, ending `OK all checks passed`. Spend $0.00: no live call, no key.

**Review (fresh eyes).** APPROVE, with one prose correction committed on the review branch, inside `owns`. Re-verified from the committed tree, not from these notes; no live call, $0.

Scope holds. Against the task tip the commit touches four files — `content.go`, `guidance_test.go`, `tools.go` and this task's lines — and no fixture or doc block moved. The only own-evidence fixture, `recall_for_purchase.response.example-own.json`, is a pool that disagrees (consulted at M, runs small, so recommends L, against the belief's S), so its sentence is byte-identical, and `TestInterfaceDocExamples` passes against `docs/interface.md` unchanged.

The direction clause is read from the value. `fitWords` maps "runs small" and "runs large" through `domain.DirectionForValue` and the fits-well value the layer composes (`feedbackBeliefValue`, "size M fits this shopper now") to "fits well"; a value naming neither falls to "your own N returns say so", which the new unit test pins so no direction is invented. The service-level table drives the real wiring (`pivotHarness`, ingest, `record_reason`/`record_feedback`, `recall_for_purchase`), and its one- and two-row cases pin the count.

`poolAgrees` uses the same arithmetic as `guidanceForPool`: the direction read as `up := direction == FitDirectionSmall`, the consulted size stepped by `neighbourSize`, compared case-insensitively with the size `lastSize` reads from the belief. Both read `poolView.Size`, which `pool.Fit` echoes as the queried size. No second rule. The absent pool leaves the bare "Take size L for Brand X": the demo driver wires no pool, and the live layer harness wires none and asks without a full line, so both keep the texts they pin.

Mutation-checked, each reverted: restoring the parent `content.go`/`tools.go` fails exactly four rows — the fits-well row with "your own one return says it runs small" and the three agreeing rows with "ahead of the pool" — while the disagreeing and no-pool rows pass. Replacing the stepped comparison with the consulted size alone fails four rows, so the table pins the arithmetic and not just the phrase. The presence marking is untouched: every pool row asserts the pool still rides overridden by `own_evidence` with or without agreement, and the response's `pool`/`pool_lookup` still say whether a pool was consulted.

One correction. The new fits-well sentence called the evidence "one return"; that row is a captured purchase carrying fits-well feedback (`outcome.kind: capture`, reason "fits well"), so the sentence was the same kind of misstatement the task fixes. `ownEvidenceClause` now names purchases for the fits-well value and keeps "return" for the two directional values, which leaves every existing too-small/too-large text byte-identical. `TestOwnEvidenceSentenceTellsTheTruth`'s fits-well row also forbids "return", and `TestOwnEvidenceWordsFollowTheValue` pins the mapping and the directionless fall phrase.

Limit and dependency. A Building pool with no direction reads "ahead of the pool" (it is present and the weaker step). T8.5r owns that pool-step change; if it makes a directionless pool absent from the response the sentence becomes bare, and if it leaves it as context the precedence wording stands. No test row covers it here because that shape belongs to T8.5r, and no size arithmetic moves in this task. The step expression is written in both `poolAgrees` and `guidanceForPool`; a change to the pool's stepping must move both, which T8.5r should watch.

Gate green in the review worktree after `npm ci` in `web/` and `merchant/`, ending `OK all checks passed`; the new tests and the package were also run alone. Spend $0.00: no live call, no key.

---

### T8.5w: Two corrected beliefs for one brand (High)
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       M · mid
owns:       internal/api/server/, internal/api/contract/
status:     done
```
The P8 review proved a High: `recallBeliefs` serves every fit belief, but `assembleRecall` allows one override. Rival birth (three opposite answers) plus `record_correction` against each belief makes every recall for that brand hard-fail with `server: contradicted belief has no overriding correction`. The ledger's contest path can trigger it, and it does not heal. Decide the repair (one override per belief in the response, or refusing the second correction at write time) — a response change is contract-shaped and the orchestrator's call — and pin it with a test that fails on today's behaviour.

**Done when:** two corrected beliefs for one brand cannot make recall refuse; the chosen shape is tested and documented; the gate passes.

**Notes (in-review).** Chose the third shape: no response change and no write refusal. `BeliefView.ContradictedBy` already carried one correction per belief, so the response could represent two contradictions; only `assembleRecall` refused, because it required *every* contradicted belief's inline correction to equal the single top-level one. The assembler now requires each contradicted served belief to carry its own inline correction, and the top-level override to be one of those corrections; it no longer requires them all to be the same one. The top level keeps its meaning — the correction to honour first, which `overridingCorrection` already picked as the newer of the two, most specific first, with arrival order breaking a tie on record time (the reproduction pins one clock, so both corrections tie and the one recorded second is the top level) — and the older correction rides inline on the belief it contradicts. Both beliefs are served, both stay contradicted and on the ledger, and neither is dropped: rule 4 holds for each of them.

Why not the other two. Adding a list (or a per-belief correction field) to `RecallResponse` is a JSON shape change, the orchestrator's call under rule 1, and it buys nothing the inline field does not already carry: the fixture would move for a field nothing needs. Refusing the second correction at write time would leave the newborn rival uncorrectable, so a shopper sold two opposite fits could correct only one and the other would keep deciding; the ledger's contest path would answer a legitimate shopper action with an error, and "the correction wins at once" would hold for one belief only.

What stays enforced. A contradicted belief with no inline correction still fails closed, and a top-level override matching none of the served beliefs still fails closed, so a second correction is never a way past a warning. `beliefView` still fails when a contradicted belief's correction is gone. A shopper whose two corrections name different sizes gets both, the newer at the top level: the response is honest about the shopper's own contradiction instead of refusing to answer.

Two things the next agent should not have to rediscover. The JSON shape, the schemas and every fixture are unchanged, so nothing here needed the orchestrator's call: only doc comments in `internal/api/contract/types.go` moved, to say that the top-level correction is the newer when two apply and that each belief keeps its own inline. And the reproduction is a service-level test, not an assembler one: `internal/api/server/two_corrections_test.go` drives rival birth (one small answer, three large) then one correction per belief, and it failed on the unfixed tree with `server: contradicted belief has no overriding correction: belief "blf_000001"`; `internal/api/server/invariant_test.go` now pins the assembler's boundary — two inline corrections assemble, a foreign top-level override fails, an inline-less contradicted belief fails.

---

### T8.5x: Kept and contested transitions have no served producer
```yaml
requires:   [T8.5]
fixture-ok: yes
size:       S · mid
owns:       internal/api/server/, internal/ingest/, product.md, docs/interface.md, docs/architecture.md, dev-diary/PHASE-8-pivot-layer.md
status:     done
```
The P8 review found two pool-row transitions product.md names have no served producer: no served path emits `OutcomeKept` (`Writer.Kept` runs only from the seed and tests), so a live purchase never settles as kept past the window; and `Writer.Contested` has no caller outside tests while `record_reason` cannot re-answer a closed question, so a contested reason never changes a row. Implement the served paths or record the limitations in product.md with their reasons.

**Done when:** each transition has a served producer or a recorded limitation; a test pins whichever was chosen; the gate passes.

**Notes (in-review).** The orchestrator chose the recorded limitation, so no served path was invented. `product.md` "How a fit row is written" now states both: a live purchase never settles as kept past its return window because that needs the window's length and a clock- or delivery-driven decision that it has closed, and the layer models neither; a contested reason never reaches the fit writer's contested state because contesting it means re-answering the same refund's question and `record_reason` accepts only an open question, so a contest on a closed question id would take a change to a frozen tool's shape. The review block corrects the wider claim: a restart can reopen the question through a redelivery, and the classified path can then change the row; only the writer's contested state stays unreachable. The paragraph and the bullets above it say what exists instead: the seeded population declares kept rows through the fit writer, each carries the seeded mark and lands in the pool's settled and kept counts; a reason is recorded once per question, a refund's through one `record_reason` answer and a dispute's from PayPal's code at ingest; the shopper's `record_correction` overrides a belief for the recommendation and never enters the pool. The evidence-model table's kept row points at that paragraph. No live passage of a window is claimed anywhere.

The code evidence: `internal/api/server/pool.go`'s `recordFitRows` handles a capture and a refund and continues for every other kind; `ingest.Service.Handle` accepts capture, refund and dispute only and refuses `domain.OutcomeKept` with `ErrUnknownEvent` before `recordFit` runs, so not even a synthetic kept delivery reaches the fit writer; `Writer.Contested` has no caller outside tests (`internal/fitindex` tests and `pool_reasons_test.go`); `RecordReason` deletes a question from the pending set on a clear answer, so a second call naming it answers `ErrUnknownQuestion` with the fit row unchanged. One surprise: `ErrQuestionClosed` is declared and mapped to the wire but no code path returns it, and its comment claimed the check existed; the comment now says it is reserved for the re-answer path.

Two tests pin the limitations the builder chose, one per transition: `internal/ingest/kept_test.go`'s `TestKeptDeliveryIsRefusedAndWritesNoFitRow` gives `Handle` a verified kept delivery and proves the recorder receives nothing; `internal/api/server/pool_transitions_test.go`'s `TestRecordReasonCannotReanswerAClosedQuestion` answers a refund question once, proves the fit row settled at `reason_classified`, then proves a second answer naming it is refused as unknown and leaves the row and the pending set unchanged. The review adds a third, `TestRestartRedeliveryReopensTheRefundQuestion`, because a redelivery after a restart is treated as new: the layer does not persist the event ids behind its duplicate guard, so the same refund files a new question and a second answer changes the row through the classified path (see the review block). No wire shape, schema or fixture changed. Outside those tests, the code edits are comments in `internal/api/server/pool.go`, `internal/api/server/server.go` and `internal/ingest/ingest.go` that named a kept or Contested served call the code does not have; the docs edits are prose only and leave every `validate=` block byte-identical to its fixture.

Other claims found:

- Fixed in place in `docs/interface.md`: the "How a signal is written" row now reads "observe, from the seed only", and the prose under it says a kept row is the seed's and the fit writer's, not a served outcome, because the layer models no return window and nothing decides one has closed. The `product_fit` prose beside the example now says the example's counts are illustrative, that the served path writes no kept row, and that `live` reads `kept` as 0 in the layer's own index. The example block itself was not edited: it is byte-pinned to `testdata/api/`, so changing it would change a frozen fixture.
- Fixed in place in `docs/architecture.md`: "The rank's limit is documented, not moved" now says the kept write has no served producer and the contested write has no served caller, so neither kept nor contested can collide with a later classification; feedback is the one rank-4 write the served layer reaches, and the existing sentence covers a classification meeting it. "Only settled rows count" now says a live purchase settles only by a classified reason or feedback and the seeded population is where kept rows come from.
- Annotated in place (this file is in `owns`): T8.2's brief now carries the T8.5x correction that the served layer writes capture, refund, classified reason and feedback only; T8.2's notes name `Classified`/`Feedback` instead of a `Contested` call. Still with the orchestrator: T7.1's note in `dev-diary/PHASE-7-pivot-contracts.md` says "kept and refund are two arms of one outcome switch"; that file is outside this task's `owns`.

The gate is green in the worktree.

**Review (fresh eyes).** The kept limit holds: `Writer.Kept` is called by the seed (`deploy/seed/main.go`) and by the `tests/live/pool` harness, and both write seeded rows; `ingest.Service.Handle` refuses `domain.OutcomeKept` with `ErrUnknownEvent` before `recordFit`, and `internal/api/server/pool.go` writes fit rows from capture and refund only, so no served delivery reaches `Kept`. The contested limit holds for the writer's state: `Writer.Contested` has no caller outside tests, and `record_reason` deletes a clear answer's question, so a second call naming it is `ErrUnknownQuestion`. The builder's reason for that limit was too wide, though: the layer does not persist the event ids its redelivery guard uses, so after a restart the same refund delivery is processed as new and re-files the question under a new id; a second answer with a different cause then settles the row again through `Classified`. `TestRestartRedeliveryReopensTheRefundQuestion` pins that (a limitation, not wanted behaviour), and `product.md` now says the fit writer's contested state stays unreachable while the classified path can be re-answered after a restart. A future fix persists the delivered event ids, or refuses a redelivery whose event rows the partition already holds; neither is in this task.

Two more claims were corrected. `docs/architecture.md` said the rank-4 collision is not reachable from a served event, but feedback is a served rank-4 write; it now says the kept and contested writes cannot collide, and feedback keeps the existing sentence. T8.2's brief and notes named a `Contested` served call; both now carry the correction (this file is in `owns`). T7.1's note stays with the orchestrator, whose file it is.

The builder's tests pass. Mutation check: removing the `delete(st.pending, question.id)` in `RecordReason` fails `TestRecordReasonCannotReanswerAClosedQuestion` with `second answer = <nil>`; the delete is restored. The gate passes after `npm ci` in `web/` and `merchant/`; spend $0.00, no live call.

---

### T8.2c: Re-run the fit-index transcript
```yaml
requires:   [T8.2b]
fixture-ok: no
size:       XS · mid
owns:       tests/live/fitindex/
status:     done
```
The P8 review found T8.2b's step-3 transcript line (`version=3001794233529 state=refunded settled=false`) cannot come from the committed driver: it prints a form `docSource.String` never emits, and a refunded document at the reason-classified rank, which no refund write can carry. The three versions the notes rest on do re-derive; only provenance is wrong. Re-run the driver so the transcript is real, or record the correction in the handoff if a run is not worth the churn.

**Done when:** a real transcript (or an honest correction of the note) is recorded; test indices are erased; spend recorded.

**Notes (in-review).** Re-ran the committed driver unchanged — `source tools/env.sh`, then `bash tests/live/fitindex/run.sh` — against the live deployment. The full verbatim transcript is the one this task's report to the orchestrator carries, and the runner's git-ignored `tests/live/fitindex/out/transcript.txt`; the handoff-log entry is the orchestrator's edit and had not landed when this branch was committed. Everything T8.2b's notes claim holds on the real output: `captured` at `1001794233521`; `refunded` at `2001794233529` with both times and no settlement; the replay before the reason printing the unchanged row at `2001794233529` — the disputed line reads in full `version=2001794233529 state=refunded settled=false kind=- reason=- direction=- verdict=- contested=false seeded=false captured_at=2026-11-09T14:12:01Z refunded_at=2026-11-09T14:12:09Z settled_at=- line=brand-x-001/shirts/regular size=M`; `reason_classified` at `3001794234729` settled by `return_reason`/`fit_too_small`/`too_small`; the retried refund and capture fit writes and the restarted writer all leaving that row; the direct older write answered `409 version_conflict_engine_exception: [<txn hash>]: version conflict, current version [3001794234729] is higher than the one provided [1001794233521]`; and one document for the purchase, one for the shopper line, one in the index. The `t82b-` listing was `[]` before the run and `[]` after it; `DELETE /t82b-fit-rows` answered `200 {"acknowledged":true}`; the driver's cleanup check and `run.sh`'s empty-prefix check passed (the saved `indices-after.json` is `[]`), and `run.sh`'s leak check left no artifact, so only its terminal output shows that pass. Spend: $0 model (the answer and its category are recorded, no classifier call), one kilobyte-scale index on the existing project.

The review's finding undercounted the drift: the old block was not this driver's output at any commit. `tests/live/fitindex/main.go` has one commit and has never been edited since, so the block could not have been stale output either — it was written by hand around the driver's steps. Four of its `doc` lines use the short `version=… state=… settled=…` form that `docSource.String` never prints (steps 3, 5, 6 and 7); step 1 said `pending question filed` where the driver prints `pending question=false` (a capture files no question); step 8 dropped the conflict's `[<txn hash>]: version conflict,` prefix and its `the one provided`; step 9 said `docs for the line` where the driver prints `docs for the shopper line`; the `== states and versions` block was missing; and the two spend lines were reworded. Only step 3's version is wrong as a value: `3001794233529` is the reason-classified rank carrying the refund's time, which no refund write can produce. Every version T8.2b's notes rest on re-derives, so the substance stood and only the transcript needed replacing. It is not replaced yet: the handoff log's T8.2b block is still the hand-written text, and the review paragraph below says what the orchestrator's edit has to do.

Three things the next agent should not have to rediscover:

- A pasted transcript is not evidence. Re-run instead: the driver is unchanged, `run.sh` tees to `out/`, and the whole run is one index and a handful of writes.
- The step-1 delivery answer prints `pending question=false`, not a filed question: the refund is what files it. A transcript line claiming otherwise is a hand-written one, and this is the cheapest way to spot the rest.
- The `409`'s reason carries the transaction hash in brackets before the versions, so a transcript that shows only the versions did not come from the index's answer.

The transcript does not live in this phase note: it belongs in the handoff log, where this phase's b-tasks record theirs, and this note carries no copy of it. Until that entry lands, the T8.2b block above it in the handoff log is superseded, not evidence.

**Review (fresh eyes).** Re-derived the three versions from the tree — rank × 1e12 plus the event's Unix second, with the capture's and the refund's times read from `capture-completed.json` and `refund-record.json` and the answer's placed twenty minutes past the refund — giving `1001794233521`, `2001794233529` and `3001794234729`. The replayed delivery is the refunded row, so step 3 can print only `2001794233529`, and the value the builder quotes is that one. The transcript was checked line for line against the driver's print statements: `snapshot.String` prefixes `version=%d ` and `docSource.String` prints its fields in the long form and order the transcript uses; step 1's `pending question=false` follows from the capture returning no `Pending`, while step 2's refund is what files the question; step 8 prints `conflictReason`'s `type: reason`, and both parts are Elastic's own `[…]: version conflict, current version […] is higher than the one provided […]`; step 9 prints `docs for the shopper line`; the `== states and versions` block is printed from `Rank()` and `rowVersion`; and every count, the empty prefix listing and the delete's `{"acknowledged":true}` are forms the probe and cleanup print. The drift inventory holds exactly: the old block's steps 3, 5, 6 and 7 use a short form no `docSource.String` emits, step 1 files a question, step 8 drops the transaction hash and `the one provided`, step 9 says `docs for the line`, the states block is absent, both spend lines are reworded, and only step 3's version is wrong as a value. `main.go` has one commit, so no stale output explains the block.

Two things this note got wrong about its own evidence are corrected above: the transcript was not already a handoff-log entry, and the old block was not replaced. Nothing in the tree carries the transcript — the complete copy is the runner's git-ignored `out/`, read for this review from the builder's worktree — and the handoff log still labels the hand-written block "verbatim from `bash tests/live/fitindex/run.sh`". The task's Done-when therefore stays open until the orchestrator pastes the transcript into a T8.2c handoff entry and marks the T8.2b block superseded; `dev-diary/README.md` is the orchestrator's file, so this branch does not edit it. One clarification did land in the driver's README: `run.sh` writes only the driver's output to `out/transcript.txt` and prints its prefix and leak checks to the terminal, so a block carrying those check lines came from a terminal paste — the old block carried them, which is part of why it read as real. No key, deployment location, IP or personal path is in the committed text; the transcript names the deployment as `<deployment>` and this note names the transaction hash only as `<txn hash>`; spend $0 model, one kilobyte-scale index deleted.

---

## Exit Criteria

- [x] One Lambo process serves every shopper, live (T8.1, T8.1b).
- [x] Every purchase writes and updates its fit row at ingest, live (T8.2, T8.2b).
- [x] The pool reads back settled-only counts and its status, live (T8.3, T8.3b).
- [x] A correction stands on its own (T8.4).
- [x] Every caller is authenticated, and no route returns a shopper's memory without a key (T8.6).
- [x] Webhook deliveries are verified against PayPal's signature, not a fixture (T8.7).
- [x] Recall never refuses, and says which step decided, live (T8.5, T8.5b).

## E2E review (fresh eyes): findings

The phase review re-verified every exit criterion against the tree, mutation-checked seven
guarantees, and re-derived the live records where a committed recording exists. It APPROVES the
landed phase with the four findings below; the orchestrator numbers their tasks (the ids here are
proposals, not claims).

**F1 (High). Two corrected beliefs for one brand make recall hard-fail.** `recallBeliefs` serves
every fit belief the shopper holds for the brand (T8.5i), and `assembleRecall` can carry one
correction only: each contradicted belief's `ContradictedBy` must equal the single
`OverridingCorrection`, or the call fails `ErrOverrideMissing`. A shopper who holds two fit beliefs
for one brand — reachable through `record_reason`, whose opposite-direction streak births a rival at
three (`TestRivalBirthAtThree`) — and has corrected each of them gets `server: contradicted belief
has no overriding correction` on every recall for that brand. Reproduced on this branch through the service's
own calls, which are the ones the routes dispatch to: one small answer, three large answers (rival born), `record_correction` against
each belief, then `recall_for_purchase`; the ledger reads still list both beliefs, so the failure is
recall's alone. The trigger is a shopper action the ledger offers (the contest path sends
`contradicts_belief_id`), and it does not heal. The repair is structural: either the response carries
one override per belief, which is a contract change and the orchestrator's call, or recall serves
one correction and refuses the second belief at write time. **Proposed owner: a new task over
`internal/api/server/` (and `internal/api/contract/` if the response shape changes).**

**F2 (Medium). Two pool-row transitions the product names have no served producer.** (a) `product.md`'s
third settlement rule ("kept past the return window") has no producer: no served path emits
`domain.OutcomeKept`, so `Writer.Kept` is reached only by `deploy/seed` and the tests. Every live
purchase therefore stays outside the pool's denominator until a refund with a reason or feedback
settles it, which is the opposite of what the product says about a new product looking perfect too
early. T8.5's handoff raised this and left it ownerless ("orchestrator to task it"); nothing in P9 or
P10 has taken it since. (b) `product.md`'s "a contested reason does change the row" has no producer
either: `Writer.Contested` has no caller outside the tests, and the only contest the ledger offers is
`record_correction`, which by T8.5's decision writes no fit row. Reason contests therefore reach the
row only as a second `Classified`, which `record_reason` cannot issue because the purchase's question
is already answered (`ErrQuestionClosed`). The decisions are a return-window clock and who owns the
sweep, and whether a reason contest is a served write. **Proposed owner: a new task over
`internal/api/server/` and `internal/ingest/`, or a recorded limitation in `product.md` if the demo
does not need either.**

**F3 (Low). One live transcript line cannot come from the committed driver.** The handoff log's
T8.2b transcript labels itself verbatim from `bash tests/live/fitindex/run.sh`, but its step 3
(replayed refund before the reason) prints `version=3001794233529 state=refunded settled=false`: a
short print form the committed `docSource.String` does not produce, and a version at the
reason-classified rank, which no refund write can carry (refund is rank 2). The committed driver
asserts the row is unchanged there (`wantSameRow`), so it could not have printed or passed that line.
The three versions the task's notes rest on do re-derive from the recordings
(`1001794233521`, `2001794233529`, `3001794234729`), so the substance holds and only the transcript's
provenance does not. **Proposed owner: the orchestrator's handoff-log correction, or a re-run when
keys are next loaded.**

**F4 (Low). The `/calls` header claim was wider than the served path.** T8.5's note said the call log
"honours the guard's `X-Memento-Shopper` header"; the service's handler does, but the deploying
director in `cmd/memento` aggregates every shopper service's log and never filters on it, so the
header is honoured by a path the deploy does not take. The route is agent-only either way, so no
shopper reaches it. The note is corrected in place on this review branch.

**Doc corrections committed on the review branch** (no code, no fixture): the "serving line"
decision that T8.5i superseded, T8.7's "the deployed webhook route still bypasses the gate", T8.6's
"nothing invokes `IssueKey` yet", T8.2's "the writer is not wired into the served layer yet",
T8.5c's "the live tripwires still pin the defects", and T8.5's `/calls` sentence above.

**Mutation checks the review ran** (each reverted): the precedence ranks, the Earned shopper floor,
the writer's consent check, a declaration's `against` edge, the `/calls` route rule, the
thin-evidence serving gate, and the evidence reason gate (`Evidence.CountsToward`). Every one
fails a committed test when broken.
