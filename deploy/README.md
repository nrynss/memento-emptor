# Deploy (T6.1)

One Render web service serves the public demo: the ledger app, the
memory layer replaying the frozen story (`testdata/replay/`), and one
Lambo hub that holds every shopper's session beside them, with its
store on a persistent disk.

## Settled shape

From `dev-diary/research/hosting.md`: one web service, plan
`starter` (0.5c-512mb, $7.00/mo) plus a 1 GB disk at
`/var/lambo-data` ($0.25/mo), Hobby workspace ($0): **$7.25/mo**
against the $50 credit. One recorded deviation: the image builds from
`deploy/Dockerfile` (runtime docker, not go), because the frozen
service cannot serve the SPA or the public webhook path and the image
must also carry the Lambo binary plus the web build. Plan, disk,
region and cost are unchanged: $0 delta.

## Toolchain pins

The two deploy images choose three toolchain versions between them, and
the reference agents' images the fourth. Go and Node must match the
versions the contract (T0.2) records, bun must match the one CI
installs, and no build may pass in one place and fail in another:

| Tool | Pin | Where the images pin it | Where the pin is checked |
|---|---|---|---|
| Go | 1.27 | `Dockerfile` and `merchant/Dockerfile`: `golang:1.27-bookworm` | `go.mod` (`go 1.27.1`) and `ci.yml`'s `setup-go` 1.27.1 |
| Node | 26 | `Dockerfile`'s `web-build` and `merchant/Dockerfile`'s `web-build`: `node:26-bookworm` | `ci.yml`'s `setup-node` 26 |
| Bun | 1 | `agent/frontend/Dockerfile`'s one stage, and the builder plus runtime stages of `agent/runtime/`, `agent/second/` and `bench/pilot/agent/`: `oven/bun:1` and `oven/bun:1-slim` | `ci.yml`'s `setup-bun` 1.3.14 |
| Lambo | 0.10.1 | `Dockerfile`: the release download, sha256-pinned | the image build's own `sha256sum -c`; no CI step pins it |

Both asymmetries are intended: the Go tag floats the latest 1.27 patch
where CI and `go.mod` fix 1.27.1, and the bun tags name a major where CI
installs 1.3.14, so those two are compared by major, as Node is. The
table is no longer the record alone: the gate runs `tools/pincheck`,
which reads both deploy images' `FROM` lines, the `oven/bun:` `FROM`
line of every Dockerfile in the tree, CI's `go-version`, `node-version`
and `bun-version`, `go.mod` and all three rows of this table, and fails
naming the file and version that drift. It also fails when a source
stops stating its pin, so it cannot pass by reading less: over this tree
it reads 18 declarations and prints `go 1.27, node 26 and bun 1 agree`.

The Node major is the whole frontend pin: neither `web/package.json` nor
`merchant/package.json` declares an `engines` field, so nothing but the
base image and CI's `setup-node` decides which Node builds the app. Both
`web-build` stages, the demo image's and the merchant image's, run the
same `npm ci` and `npm run build` as CI, on the same major. The
gate itself runs `npm ci --dry-run`, which checks the lockfile resolves
without installing.

## Provision (operator, after this lands on main)

Render builds the branch it is pointed at, so provisioning before the
land only reserves the shell. Two prerequisites, both outside this
task: the deploy files are on `main`, and the workspace can fetch the
repo — either the repo is public (T6.5), or GitHub is connected on
the dashboard (Settings). Without one of those the API refuses the
repo as unfetchable and nothing billable is created (verified: the
create call fails before any resource exists).

Keys travel in environment variables at provision time, never in the repo.

API (values from the keyring via `tools/env.sh`, never printed):

```sh
source tools/env.sh
owner=$(curl -s -H "Authorization: Bearer $RENDER_API_KEY" \
  https://api.render.com/v1/owners | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["owner"]["id"])')
# Fill env values from the environment into the payload and POST it;
# see ORCHESTRATOR handoff for the exact call. Shape:
# type web_service, runtime docker, plan starter, region virginia,
# disk lambo-data 1 GB at /var/lambo-data, healthCheckPath /healthz,
# envSpecificDetails.dockerfilePath ./deploy/Dockerfile, dockerContext .,
# branch main, autoDeployTrigger commit, env values as top-level envVars.
```

Or point the dashboard at `deploy/render.yaml` (blueprint path, same
shape). Either way, confirm on the dashboard before creating: the $50
credit and $0 pending baseline. Re-check after: the service list must
show exactly this service.

## First boot

`deploy/start.sh` is the container command:

1. The edge answers `/healthz` at once, so Render sees a healthy
   service while Lambo works.
2. The memory layer boots with `-seed`: the demo story is servable
   before any live event arrives.
3. `lambo.toml` is written to the disk from `ELASTIC_URL` +
   `LAMBO_ES_API_KEY` (native `.jina-embeddings-v3`, the T0.6b shape
   with the 0.10.1 timeout knobs). No `[recall]` section is written:
   the hub is store-only by decision (see below). No embedder key means
   no live graph: the replay still serves, loudly.
4. `deploy/seed` probes the embedder in a scratch session, then plants
   the proven arc once per contract (marker file on the disk):
   Venerable sizing belief, seller-only dispute, declared correction.
   Native-down falls back to the spare BGE contract once, when
   `OPENROUTER_API_KEY` is present. The seed runs **before** the hub:
   its short-lived stdio writer owns its session alone while it runs.
5. The hub starts on loopback and attaches the seeded session as its
   one pinned session. Every other shopper's session is created on
   demand (below).
6. `serve-web` exposes the demo shoppers' live graph read-only on
   loopback, under the allowlist it started with.

A restart reseeds nothing: the marker and the SQLite file persist on
the disk. Prove it with `inspect` on the sizing belief before and
after a restart (Venerable plus the contradiction warning).

## The shopper hub

One `lambo serve --transport http` process, one writer per session,
many sessions. The shape is the measured one in
`dev-diary/research/lambo-multi.md`:

```toml
[serve]
sessions = ["shopper-<seeded demo shopper>"]  # pinned; /mcp serves it
default_session = "shopper-<seeded demo shopper>"
max_attached = 16
attach_concurrency = 1
idle_detach_secs = 900

[[serve.credential]]
name = "memento"
token_env = "LAMBO_MCP_TOKEN"
session_prefix = "shopper-"
create = true
```

and the process:

```sh
lambo --config lambo.toml serve \
  --transport http --bind 127.0.0.1 --port "$LAMBO_MCP_PORT" \
  --agent memento-layer --max-sessions 128 --rate-limit-rps 200
```

**How a shopper session is created.** Nothing is provisioned per
shopper. The session name is T7.2's construction
(`internal/identity`): `shopper-` plus the lowercase hex HMAC-SHA256
of the shopper's PayPal payer id under the identity salt
(`MEMENTO_IDENTITY_SALT`, a platform secret; seeded shoppers use the
published seed salt instead). That is 72 bytes, inside the graph's
1–128 byte addressed-name rule and inside the credential's prefix. The
layer addresses `POST /mcp/s/<session>` with the bearer token and
speaks MCP there; the tools take no session argument. The first
request creates the session (`create = true`), later requests
re-attach it from SQLite. The hub's ceiling is 128 live MCP client
sessions, so a caller opens one per shopper request rather than holding
them open.

Bounds, from the measurements: `max_attached` counts pinned sessions,
so the pinned demo shopper leaves 15 shoppers attached at once. While
all 16 are attached and none is idle, the hub answers HTTP 503 with
`Retry-After`; a caller must retry that (the layer's graph client is
T8.5). An on-demand session unused for 900 s detaches; its data stays
in SQLite and the next request re-attaches it. Never pin every
shopper: a pinned session never idle-detaches and holds its place
permanently.

The hub accepts one credential, so every request needs the bearer
token. `start.sh` uses `LAMBO_MCP_TOKEN` when the environment sets it
and otherwise generates one for that boot, then exports it for the hub
and for the layer's graph client. It is never written to a file or
printed. The deploy also exports `LAMBO_MCP_URL`, the hub's loopback
URL; wiring the layer's graph client is T8.5.

## The hub's recall tier: store-only, by decision

`start.sh` writes `[store]`, `[embedder]`, `[daemon]` and `[serve]` into
`lambo.toml`, and no `[recall]` section. The hub therefore runs Lambo's
plain SQLite store with recall served from the session's own rows and
the holder's graph. That is a decision, not an omission: T0.11's check 4
and `research/elastic.md` measured the Elastic tier against this
deployment, and neither describes the deployed shape.

**What the hub recalls without it.** Every leg: keyword candidates, the
recent leg, the graph legs, and the vector leg as an exact cosine scan
over the session's own stored vectors. The SQLite store declares that
read exact (`exact_vector_scan`), so the session holder ranks the
vectors its in-memory graph already holds with the same scorer, and a
holder's recall and hybrid derive never leave the process. `serve-web`
and the `lambo recall` CLI hold no graph and run the same scan against
SQLite. What the demo reads — a belief by id, exact-content `inspect`, a
correction's force, dependents and blast radius — is served the same way
with or without the tier, because those are not vector reads. Durable
memory sits on the disk in `lambo.db` either way.

**What it cannot do.** There is no engine-side kNN index: each vector
probe scores the session's own stored vectors, so the per-query cost
grows with the session's concept count on the hub's 0.5 CPU and memory.
A demo shopper's session holds a few hundred vectors at most, because
the layer writes beliefs and corrections and not catalogue data. Nor
does a second process need a shared index: `serve-web` runs the same
exact scan against SQLite, for the one session a request names.

**What enabling it would take, and what it would change.** The pinned
0.10.1 binary ships `recall-elastic` (the release's `ship` feature set),
so this is a config-only change, with no rebuild and no new secret:

```toml
[recall]
kind = "elastic"
url = "<ELASTIC_URL>"
api_key = { env = "LAMBO_ES_API_KEY" }
index_prefix = "lambo"
timeout_ms = 20000     # cold TLS handshake is ~6.8 s; the 5 s default fails
```

It would create `lambo-meta` and `lambo-v-<contract hash>` beside the fit
index and move the vector leg of recall to an Elastic kNN query:
candidates come from the engine, filtered to the session by
`session_id` there (T0.11 check 4). The index lags the flush because no
refresh is forced, a failed mirror marks the session stale and recall
falls back to the primary until the holder repairs it, and
`vectordb_document` mode keeps vectors out of `_source`, so Lambo's
exact re-scoring has nothing to re-score and the engine's HNSW score
stands (T0.6's second finding). None of that answers anything the
store-only hub does not already answer; the tier's value is scale, and
the sessions do not have the scale.

**What it would cost.** No extra embedding call: the tier mirrors the
vectors the embedder already produced, it does not embed again. What it
adds is one bulk write to the deployment per flush and one kNN query per
vector probe, on a Vector Database project that bills on ingest volume
and storage (T0.12 check 8). That is neither measured nor bounded here,
so the standing budget cannot be said to cover it, and `guard.sh` audits
the Render plan rather than Elastic ingestion. A later task that wants
the tier should measure one write and one recall against it and state
the numbers, instead of assuming it is free because the key is already
in the environment.

**The layer's own pool is separate and unaffected.** `fit-rows` is the
layer's index, created and read by `internal/fitindex` and
`internal/pool` over plain HTTP with its own mapping, a fixed name, and
no alias, template or data stream linking it to Lambo's indices (T0.12's
coexistence check). The layer reads it directly and never through the
hub, so the hub's store-only setting cannot see it or change it, and a
hub tier would write under its own `index_prefix` and still not touch
it.

## The ledger reader's allowlist

`serve-web` reads the session from the request path, so one reader
serves every demo shopper at `/s/<session>/api/...`. Its list is fixed
at start (Lambo issue 241), so a shopper created after boot is the
uniform 404 there until the reader is restarted with that name.
`LAMBO_WEB_SESSIONS` is that list, space separated, and `start.sh`
passes each name as `--session`; it defaults to the seeded demo
shopper, the T7.2 name for payer id `shopper_01` (the vector
`internal/identity`'s test pins). Set it to the full list, demo
shopper included. The reader starts beside the hub whether or not the
seed held: an empty window is better than a missing ledger. A restart
of the reader drops the ledger for every shopper for the restart
window, so add a demo shopper only when the list really changes.

## A local hub run

No key is needed: the fixture embedder computes vectors in process.
Build the image's three Go binaries and point `start.sh` at a local
`/app` tree (a copy of the Lambo release binary, `bin/`, `web/` and
`testdata/`):

```sh
APP_DIR=/tmp/memento-local/app LAMBO_DATA_DIR=/tmp/memento-local/data \
  MEMENTO_IDENTITY_SALT=$(openssl rand -hex 32) \
  LAMBO_EMBEDDER=fixture LAMBO_SEED=0 LAMBO_MCP_TOKEN=local-run-token \
  PORT=18081 LAMBO_MCP_PORT=17724 LAMBO_WEB_PORT=17723 \
  bash deploy/start.sh
```

The salt is the one thing that has no default: it is a scratch 64-hex
value here, any value will do for a local run, and it switches on the
pivoted path, so the hub boots first and the layer replays into it
exactly as the deployed service orders them. Without it the layer exits
at boot (`LAMBO_MCP_URL and LAMBO_MCP_TOKEN need MEMENTO_IDENTITY_SALT`),
because the hub session names are keyed hashes under that salt. With it
and no `ELASTIC_URL`, the layer logs that the pool reads nothing and no
fit row is written: `product_fit` refuses rather than report an empty
pool.

`MEMENTO_IDENTITY_SALT` is required, not optional: `start.sh` always
exports the hub's URL and token, and the layer refuses those without the
salt that names its sessions. With the seed off, the fixture embedder and
no index wired, the value names sessions and nothing else, so a throwaway
value minted per run is enough here. The same boot runs in the image
itself: `docker run -e PORT=18099 -e LAMBO_EMBEDDER=fixture -e
LAMBO_SEED=0 -e LAMBO_MCP_TOKEN=local-run-token -e
MEMENTO_IDENTITY_SALT="$(openssl rand -hex 32)" -p 18099:18099 <image>`
answers `/healthz` with `{"ok":true}` and serves `/` from `/app/web`.
The hub comes up on `127.0.0.1:17724` in seconds, the ledger reader on
`127.0.0.1:17723`, and two sessions at `/mcp/s/shopper-<hash>` write
and recall without reading each other. `LAMBO_EMBEDDER` also accepts
`native` and `bge`; left unset it picks from the environment as the
deployed service does.

`LAMBO_SEED=0` leaves the frozen demo arc out. The arc's promotion wait
expects the deployed embedder's slower pace: the fixture embedder lands
its writes faster than the graph's sweep cadence, the belief does not
reach its sweep count inside the wait (the effect T0.11's check 6
records), and the seed would spend minutes to time out. Without the
flag the seeder runs first, as on the deployed service, and the hub
starts when it returns; the marker is per embedder kind, so a fixture
run never stands in for the deployed seed.

## PayPal webhooks

Deliveries go to `https://<service>.onrender.com/webhooks/paypal`;
the edge rewrites the path to the layer's frozen `/webhooks/payment`
route. After the first green deploy:

```sh
source tools/env.sh
PUBLIC_URL=https://<service>.onrender.com bash deploy/paypal-webhook.sh
```

It subscribes the three proved event types (capture completed and
refunded, dispute created), idempotently. Raw-delivery verification
and the shopper join belong to the ingest task: the shim answers with
the layer's verdict and documents the gap rather than inventing one.

## Spend guard

Fixed-rate billing means the ceiling is a run-rate and inventory
audit. `deploy/guard.sh --check` is the live audit (exit 0 within the
settled shape, 1 on alert, 2 when it stopped something with `--stop`
by suspending the drifted service). Unknown plans alert and never
auto-stop. Demonstrate the trip without real exhaustion:

```sh
bash deploy/guard.sh --snapshot deploy/guard-snapshot-over-limit.json --stop
```

It prints the suspend it would issue and exits 2; nothing is called.

## Files

| File | Job |
|---|---|
| `Dockerfile` | The demo image (service + edge + seeder + SPA + Lambo 0.10.1, sha-pinned) |
| `render.yaml` | The settled shape as a blueprint |
| `start.sh` | Boot supervisor (config, provision, seed-once, hub, ledger reader) |
| `edge/` | Same-origin static files, API proxy, webhook path shim |
| `seed/` | First-boot Lambo seeder (proven arc, marker-idempotent) |
| `guard.sh` | Spend ceiling: audit, alert, suspend |
| `guard-snapshot-over-limit.json` | Dry-run trip fixture (placeholder ids) |
| `paypal-webhook.sh` | Webhook subscription, idempotent, sandbox only |
