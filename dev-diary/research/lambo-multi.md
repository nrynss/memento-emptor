# Lambo serving many shoppers (T0.11)

How the release the deploy pins serves every shopper, measured. Lambo
**v0.10.1**, the tag `deploy/Dockerfile` fetches. The machine's checkout was
two commits past the tag, so the binary here was built from the tag's own
source tree (`git archive v0.10.1`), with the release feature set:

```sh
git archive v0.10.1 | tar -x -C /tmp/lambo-v0.10.1
cd /tmp/lambo-v0.10.1
LAMBO_GIT_SHA=5dd3cee cargo build --locked --release --features ship
./target/release/lambo --version        # lambo 0.10.1
sha256sum target/release/lambo          # da299ff5…452e4260d
```

`ship` is what `.github/workflows/release.yml` builds every published asset
with (`FEATURES: ${{ matrix.features || 'ship' }}`), so `store-sqlite` and
`recall-elastic` are in the deployed binary even though neither is in
Cargo's `default`. Local checks used the SQLite store and the `fixture`
embedder; check 4 used the live Elastic deployment with the fixture embedder
(no embedding API call, so no embedding spend) and test indices only.

The short answer: **v0.10.1 already multiplexes.** One `lambo serve
--transport http` process hosts one writer per session and many sessions at
once, attaches a session the first time a request addresses it, and bounds
how many stay attached. The 64-hex payer hash fits the addressed-name rule
with a prefix. A session name is addressable at `/mcp/s/<session>`; the MCP
tools themselves carry no session argument. The one thing still fixed at
start is the ledger reader, `serve-web` (check 7): it needs the name on its
start-time allowlist.

Config the checks generated for the 20-session runs; the committed
`testdata/lambo-multi/lambo-multi.toml` is the same table at the deploy's
`max_attached = 16` (check 1b lowers it to 4, check 7 to 8):

```toml
[store]
kind = "sqlite"
path = "<run-dir>/lambo.db"

[embedder]
kind = "fixture"
dim = 1024
calibration = "off"

[daemon]
gc_interval = 1
gc_idle_floor = 1
gc_max_interval_secs = 3600
canonization_eval_interval_secs = 5

[serve]
sessions = ["shopper-pinned"]
default_session = "shopper-pinned"
max_attached = 32
attach_concurrency = 1
idle_detach_secs = 900

[[serve.credential]]
name = "memento"
token_env = "LAMBO_MULTI_TOKEN"
session_prefix = "shopper-"
create = true
```

Commands (keys only for check 4, from `tools/env.sh`, never written):

```sh
source tools/env.sh
export LAMBO_BIN=/tmp/lambo-v0.10.1/target/release/lambo
export LAMBO_MULTI_TOKEN="$(python3 -c 'import secrets;print(secrets.token_urlsafe(32))')"
cd testdata/lambo-multi
python3 check1.py        # then check1b_eviction.py, check2.py, check3.py,
                         # check5.py, check6.py, check7.py
python3 check4.py        # live Elastic
```

Every transcript and report is in `testdata/lambo-multi/transcripts/` and
`reports/`. The token was in the environment only; no file holds it.

## Source facts this rests on (read at the tag)

| Fact | Where |
|---|---|
| One process may own many sessions; each session has exactly one owning process | `src/mcp/serve.rs` module doc, spec §2.2 as amended by #32 |
| A stdio serve owns exactly one session; HTTP takes repeatable `--session` plus `[serve] sessions` | `src/mcp/serve/pinned.rs::plan_sessions` |
| An HTTP serve routes `/mcp` (default session) and `/mcp/s/{session}` | `src/mcp/serve/transport.rs` |
| A credential with `session_prefix` (or an unpinned exact name) makes the serve attach on demand; the implicit loopback credential does not reach past pinned sessions | `src/mcp/serve/authority.rs::reaches_past_pinned`, `on_demand_credentials` |
| Addressed ids: 1–128 bytes of `[A-Za-z0-9._:-]`, not starting with `.`, never percent-decoded | `src/surface/session.rs::parse_addressed` |
| Attach bounds: 60 s whole attach, 5 s existence probe, 15 s client wait, 30 s negative cache, 2 s minimum idle for eviction | `src/mcp/serve/registry/on_demand.rs` |
| Defaults: `max_attached` 16, `attach_concurrency` 2 (SQLite clamps to 1), `idle_detach_secs` 900 | `src/config/serve.rs` |
| HTTP guards: 32 live MCP sessions, 50 req/s per credential, burst 2× | `src/mcp/serve/http_guards.rs` |
| SQLite: WAL, `busy_timeout` 8 s, one pooled connection per process | `src/store/sqlite/mod.rs` |
| Recall tier: one data index per embedding contract `{prefix}-v-{contract_hash}`, one `{prefix}-meta` marker index; every query filters `session_id`; marker `_id` is SHA-256 of the session id | `src/store/tiered/elastic.rs` |
| `serve-web` serves an explicit allowlist (repeatable `--session` plus `[web] sessions`) and refuses any other id with the uniform 404 | `src/cli/serve_web/state.rs` (the allowlist), `scope.rs` (the refusal), `src/surface/session.rs::is_pinned` |

## Check 1: one process or many

One `lambo serve --transport http --port 0` with the credential above. A
single PID served 20 sessions, each attached at `/mcp/s/<session>` on its
first request; every session's stats named itself with three concepts, and a
recall in one session did not reach another's. Transcript:
`transcripts/check1.jsonl`, report `reports/check1.json`.

| Measurement | Value |
|---|---|
| Startup to listening | 0.027 s |
| First `initialize` after listening | 0.002 s |
| Baseline RSS (one pinned session) | 21.1 MiB |
| RSS with 20 attached, before writes | 26.8 MiB |
| RSS after 3 concepts in each | 28.3 MiB |
| Open file handles: baseline → 20 attached | 14 → 34 |
| Marginal per attached session (3 concepts) | ≈ 0.4 MiB |
| Per concept, measured over 100 added concepts | 29.7 KiB |
| One 500-concept session, implied | ≈ 14.5 MiB |

The alternative — **one writer process per session** — measured over 10
spawns of `lambo serve --session <one>` against the same store:

| Per-process cost | Value |
|---|---|
| Startup to answered `initialize` | 0.006 s median |
| Resident memory | 19.7 MiB median, 19.9 MiB max |
| File handles | 13 |

So the hub is one 21 MiB process instead of 20 MiB per shopper. Two hundred
shoppers would be 4 GB as processes and about 100 MiB as attached sessions
(with small sessions).

**Bounds, measured (`transcripts/check1b.jsonl`).** `max_attached` counts the
pinned sessions too: with `max_attached = 4` and one pinned session, three
on-demand sessions filled it. The harness let them pass the 2 s minimum idle
before attaching a fourth, so `shopper-d` attached by evicting the least
recently used idle session (`reason="evicted"`); a later request re-attached
`shopper-a` and read its concept back. With `idle_detach_secs = 2`, the
sweeper detached every on-demand session with `reason="idle"`; data stayed in
SQLite. Pinned sessions are never evicted or idle-detached.

**How many fit the Render starter (0.5 CPU, 512 MB, one 1 GB disk).** Leaving
192 MiB for the Go edge, the layer and the OS, 320 MiB fits

- 16 attached demo-sized sessions (16 × 0.4 MiB + 21 MiB ≈ 28 MiB), or
- 16 sessions of 500 concepts each (16 × 14.9 + 21 ≈ 259 MiB), or
- 20 sessions of 500 concepts each (20 × 14.9 + 21 ≈ 319 MiB), the
  report's ceiling for that budget.

The default `max_attached = 16` is the recommendation and is safe at every
session size measured. The 320 MiB budget counts only the Go edge, the layer
and the OS; the deploy also runs the `serve-web` reader, a second Lambo
process this task did not measure, so 20 sessions of 500 concepts is an
upper bound, not a safe default. Across all shoppers over time the
accumulating bound is the disk, not concurrent memory: the store grew to
about 8 KiB per concept in check 1 (1.3 MB for 160 concepts), so the 1 GB
disk holds roughly 125k concepts across all shoppers.

## Check 2: sessions on demand

A session that had never been written to was created by the first
`initialize` at `/mcp/s/shopper-<64 hex>` in the running process: same PID,
an attach line in the log, `concepts=0` before the first write, the write and
its recall in the same session. `create = true` on the credential is what
allows it; with `create = false` an absent session is the uniform 404 and an
existing one still attaches. Transcript `transcripts/check2.jsonl`.

The addressed-name rule, exercised against the running hub:

| Name | Bytes | Result |
|---|---|---|
| `shopper-` + 64-hex hash | 72 | 200 |
| bare 64-hex hash | 64 | 404 — outside the credential's prefix |
| `shopper-` + 120 × `a` | 128 | 200 |
| `shopper-` + 121 × `a` | 129 | 404 |
| `shopper-Shopper:01_A.B-2` | 24 | 200 — every allowed punctuation |
| `shopper-a b` | 11 | 404 (space) |
| `shopper-a+b` | 11 | 404 (plus) |
| `shopper-%2Dencoded` | 18 | 404 (percent is never decoded) |
| `shopper-ä` | 10 | 404 (non-ASCII) |
| `other-a` | 7 | 404 (outside the prefix) |
| `.shopper-leading-dot` | 20 | 404 |
| `shopper/slash` | 13 | 404 (unrouted path) |

A 72-byte `shopper-<hash>` is the naming recommendation: it is inside the
128-byte limit and inside the prefix that makes on-demand creation possible.
A bare hash is not addressable unless the credential's prefix happens to
match it.

## Check 3: many writers, one SQLite store

Two deploy-relevant shapes, every write receipt-waited (60 s cap), then the
database counted directly with SQLite read-only. Transcript
`transcripts/check3.jsonl`.

| Case | Writers | Writes | Wall | Failures | Lock errors | DB rows per session |
|---|---|---|---|---|---|---|
| 8 stdio processes, one session each | 8 | 160 | 0.04 s | 0 | 0 | 20 each |
| One hub, 8 sessions at once | 8 | 160 | 0.10 s | 0 | 0 | 20 each |

No `SQLITE_BUSY`, no `database is locked`, no lost write, no receipt timeout.
Mechanism: WAL plus an 8 s busy timeout and one pooled connection per
process; the write lock is held only for a flush. The fixture embedder makes
these numbers a floor — a real embedder adds its own latency, not lock
contention.

**One writer per session is enforced differently than expected.** A second
`lambo serve --session <held>` does not refuse to start: it becomes a proxy
to the holder over the holder's Unix socket, takes no lease, and its write
lands in the holder (`stats` owner stays `memento-hub`). So a layer that
spawns a writer per request is not a correctness problem, but it is a
process per call; the HTTP hub is the shape to use.

## Check 4: the recall tier with many sessions

Live Elastic, `[recall]` on, `index_prefix = "t011-multi-<rand>"`, one hub,
three sessions, three concepts each, the fixture embedder. Transcript
`transcripts/check4.jsonl`, report `reports/check4.json`.

- Two indices appeared: one data index
  `t011-multi-bb7582-v-271bc9a15ddef9b4` (one per embedding contract, named
  by the contract hash) and `t011-multi-bb7582-meta`.
- All three sessions' documents sat in the **one** data index, three per
  session, each with `session_id` a keyword field. The marker index held one
  document per attached session (including the pinned one), each `_id` a
  64-hex SHA-256 of the session id.
- Recall stayed scoped: each session found its own token, and none of the six
  cross-session queries returned another session's token.
- Inspect stayed scoped: each cross-session exact-content inspect was `no
  concept matching`, on the tier-backed store.
- Every index created under the test prefix was deleted; the listing after
  deletion is `[]` (`indices_after_delete` in the report), and no other
  `t011-*` index existed at the end.

The kNN body is filtered in the engine
(`"filter": { "term": { "session_id": ... } }`) and deletes/repairs use the
same filter, so sharing one index is not sharing recall. The prefix is
deployment-wide: two environments on one deployment must not share it.

**Spend: $0.00 of the $0.50 budget.** The fixture embedder computes vectors
in-process, so no embedding endpoint was called; the only live calls were
index create, search and delete on the trial deployment.

## Check 5: isolation at scale

Twenty sessions, five concepts each, all about the same two brands, each
sentence carrying a word unique to its session. One hub, all 20 attached.
Transcript `transcripts/check5.jsonl`.

- Positive control: every session recalled its own marker and inspected its
  own belief (20/20).
- Every ordered pair (380) queried recall with the other session's marker
  and inspected the other session's exact belief: **780 calls, 0 leaks**.
- A shared-brand recall (`Brand X oxford chinos sizing`) returned no other
  session's marker in any session.
- `lambo_stats` reported `concepts=5` for each of the 20, and the private
  database held exactly 20 sessions × 5 concepts.

## Check 6: promotion per session

Three sessions, one belief each, seeded **interleaved** (one hop of work in
each session, round after round), with event times three days apart across
ten days. One hub
served all three. Transcript `transcripts/check6.jsonl`.

| Session | Status | distinct_interactions | coverage | blast radius |
|---|---|---|---|---|
| shopper-one | Venerable | 3 | 0.6667 | 3 |
| shopper-two | Venerable | 3 | 0.6667 | 3 |
| shopper-three | Venerable | 3 | 0.6667 | 3 |

Every session reached the same stage-2 verdict with the same numbers, so no
other session's writes, sweeps or time extent moved it. Twenty noise
concepts then written into the first session (dated inside its own window)
left all three beliefs Venerable at the same coverage: `unmoved` is true for
all three.

One operational detail worth knowing: with `gc_interval = 1`, four
back-to-back writes into a session can coalesce into fewer sweeps than the
belief needs for `gc_survived >= 3` (the inspect text showed 1, then 2).
Spacing a few extra mutations into the session's own event-time window
cleared stage 1; the belief then reached Venerable on the next evaluation.
The seeding harness writes fillers for this. Nothing was lost and no other
session was touched.

## Check 7: the ledger per shopper

`lambo serve-web --session a --session b` reads the session from the request
path, so one reader process serves several shoppers at once:

| Request | Result |
|---|---|
| `GET /s/shopper-alpha/api/inspect?focus=<alpha belief>` | 200, `found: true` |
| `GET /s/shopper-bravo/api/inspect?focus=<bravo belief>` | 200, `found: true` |
| `GET /s/shopper-bravo/api/inspect?focus=<alpha belief>` | 200, `found: false` |
| `GET /api/inspect?focus=<alpha belief>` (alias, default session) | 200, `found: true` |
| `GET /api/sessions` (`[web] list_sessions = true`) | `[alpha, bravo]` |
| `GET /s/shopper-late/api/inspect` (`shopper-late` exists in the store) | **404** |
| Restart with `--session shopper-late` | 200, `found: true` |

So the session is chosen per request **from a start-time allowlist**, not
per request over the store. Transcript `transcripts/check7.jsonl`.

## The deploy shape

One Render service, one instance (disk-backed services run exactly one), one
disk, one SQLite file:

1. **One long-lived writer hub**, started by `deploy/start.sh`:
   ```sh
   lambo --config "$CONFIG" provision          # idempotent, first boot
   lambo --config "$CONFIG" serve \
     --transport http --bind 127.0.0.1 --port "$LAMBO_MCP_PORT" \
     --agent memento-layer --max-sessions 128 --rate-limit-rps 200
   ```
   with `[serve] max_attached = 16`, `attach_concurrency = 1`,
   `idle_detach_secs = 900`, and one credential whose
   `session_prefix = "shopper-"` and `create = true`, its token from the
   environment (`token_env`). `max_sessions` and `rate_limit_rps` are flags;
   their defaults are 32 MCP sessions and 50 req/s with a 2× burst, and this
   harness raised both (128 and 1000) so its own concurrent bursts were not
   throttled. Raise them for ingest bursts.
2. **Session names**: `shopper-` + the 64-hex salted hash of the payer id
   (72 bytes). One shopper, one session, one writer at a time; a returning
   shopper re-attaches from SQLite on the next request.
3. **The layer addresses** `POST /mcp/s/<session>` with the bearer token and
   speaks MCP there; the tools take no session argument. Keep one MCP client
   session per shopper request (or a small pool) so the 128-session ceiling is
   not exhausted.
4. **The ledger**: `serve-web` still starts with the sessions it serves. For
   the demo, start it with the demo shoppers and use `/s/<session>/api/...`
   URLs; a shopper created after boot is not readable until the process is
   restarted with that name (issue below). Do not pin every shopper in
   `serve`: pinned sessions are never idle-detached and occupy `max_attached`
   permanently.
5. **Keep the seeder** as it is for the frozen demo story; the hub is what
   later live ingest talks to. Both share the store file, but the seeder's
   short-lived stdio writer must own its session alone while it runs, so keep
   `start.sh`'s order (seed, then start the hub) or give the seeder a session
   the hub does not pin. The pinned session spends one of the 16
   `max_attached` places, so 15 shoppers attach at once beside it. While all
   16 are attached and none is idle, a further request is HTTP 503 with
   `Retry-After` (the source's attach bound); the layer must retry it.

Do **not** run one writer process per shopper on this instance: 19.7 MiB
each means about 16 fit 512 MB before the Go services, against 16 attached
sessions costing 28 MiB in the hub.

## Gaps to file (Lambo issues, for the user)

**1. `serve-web` cannot read a session created after it started.**

> **Title:** serve-web: an allowlist fixed at startup cannot read a session created later
>
> **Body:** In v0.10.1 `lambo serve-web` serves the ordered union of
> `--session` and `[web] sessions`; a request for any other session is the
> uniform 404 (`src/cli/serve_web/scope.rs`, calling
> `src/surface/session.rs::is_pinned`). A per-shopper
> deployment names sessions after a hash and creates them while the reader
> runs, so the portal cannot show a shopper who first appears after boot
> without restarting it with that name. A `[[web.credential]]` already
> accepts a `session_prefix`, and `"*"` still means the allowlist, not the
> store: a prefix grant still has to pass `is_pinned`, which is the
> allowlist. Requested: let a configured
> `[[web.credential]] session_prefix` (or a new `[web] session_prefix`)
> authorize reads of any **existing** session under it by relaxing that
> allowlist gate, using the existing loaded-view cap and TTL plus an idle
> rule, and keeping the same
> uniform 404 for absent ids. Until then a deployment must restart the reader
> to add a shopper, which drops the ledger for everyone else for the restart
> window. Affects the T4 ledger and the T6.1 deploy.

No other gap: the checks that decide the deploy shape all passed, so the
task is not blocked.

## What later tasks replay

- `testdata/lambo-multi/lambo-multi.toml` — the exact serve table.
- `testdata/lambo-multi/transcripts/` — every tool call, addressed-name
  probe, HTTP read and process measurement.
- `testdata/lambo-multi/reports/` — the summary numbers above.
- The binary is not committed (millions of bytes, built from the tag). The
  build command is in `run-all.sh`'s header.
