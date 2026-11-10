# Deploy (T6.1)

One Render web service serves the public demo: the ledger app, the
memory layer replaying the frozen story (`testdata/replay/`), and
Lambo beside them with its store on a persistent disk.

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
   `OPENROUTER_API_KEY` is present.
5. `serve-web` exposes the live graph read-only on loopback.

A restart reseeds nothing: the marker and the SQLite file persist on
the disk. Prove it with `inspect` on the sizing belief before and
after a restart (Venerable plus the contradiction warning).

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
| `start.sh` | Boot supervisor (config, provision, seed-once, serve) |
| `edge/` | Same-origin static files, API proxy, webhook path shim |
| `seed/` | First-boot Lambo seeder (proven arc, marker-idempotent) |
| `guard.sh` | Spend ceiling: audit, alert, suspend |
| `guard-snapshot-over-limit.json` | Dry-run trip fixture (placeholder ids) |
| `paypal-webhook.sh` | Webhook subscription, idempotent, sandbox only |
