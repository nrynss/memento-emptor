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
   with the 0.10.1 timeout knobs). No embedder key means no live
   graph: the replay still serves, loudly.
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
and otherwise generates one for that boot; both the layer and the hub
start in the same boot, so they agree either way. The token is
exported, never written to a file or printed. The deploy also exports
`LAMBO_MCP_URL`, the hub's loopback URL, for the layer's graph client;
wiring that client is T8.5.

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
  LAMBO_EMBEDDER=fixture LAMBO_SEED=0 LAMBO_MCP_TOKEN=local-run-token \
  PORT=18081 LAMBO_MCP_PORT=17724 LAMBO_WEB_PORT=17723 \
  bash deploy/start.sh
```

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
