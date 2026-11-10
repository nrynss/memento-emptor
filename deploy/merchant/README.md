# Merchant app, deployed (T9.3d)

Its own Render service for the sandbox merchant's read-only app, following
T6.1's provision and guard pattern. It serves the `merchant/` build and
reaches the layer's `product_fit` and nothing else: any other `/tools/`
path is refused in the edge, before any backend is touched.

## Settled shape

One stateless web service, plan `starter` (0.5c-512mb, $7.00/mo), region
virginia, no disk, health check `/healthz`. The merchant key and the layer
URL travel as environment values set at provision time (Render secrets),
never in a file; the browser never sees the key, because the edge injects
it server-side on the single proxied route.

Cost: **$7.00/mo**. Together with the main demo service ($7.25/mo):
**$14.25/mo** against the $50 credit, about 3.5 months of runway. Record
any change in the handoff log; the user has pre-approved this service.

## Provision (operator, after this lands on main)

Render builds the branch it is pointed at, so provisioning before the land
only reserves the shell: the first deploy fails until `main` carries
`deploy/merchant/Dockerfile`, then the green deploy auto-follows.

Mint the merchant key first, with T8.5's command. The token goes onto
this service; the entry goes onto the layer (appended to the layer's
`MEMENTO_MERCHANT_KEYS`, T9.6's side):

```sh
token=$(go run ./cmd/memento key issue --kind merchant --id merchant-app --seller seller-demo 2>/dev/null)
```

Then create the shell (values from the environment, never printed):

```sh
source tools/env.sh
MEMENTO_MERCHANT_KEY="$token" bash deploy/merchant/provision.sh
unset token
```

Confirm on the dashboard before creating: the $50 credit and the current
inventory (exactly the `memento` service). Re-check after: the list holds
exactly `memento` and `memento-merchant`, both starter.

Until the layer registers the entry, `POST /tools/product_fit` on this
service answers 502/503 and the app keeps reading its bundled snapshot;
no secret is ever written to a file. To check for a leak, load the value
and grep for it (`grep -rlF -- "$VALUE"` prints only file names, and the
value is never echoed).

## Wiring the live read

The page loads through the fixture transport today (T9.3): wiring the
HTTP transport to same-origin `/tools/product_fit` is a `merchant/`
change and belongs to a follow-up task, not this directory. The proxy is
ready for it: no caller credential is needed browser-side, because the
edge holds the merchant key server-side.

## Spend guard

`deploy/merchant/guard.sh --check` is the live audit over both demo
services (exit 0 within the $15.00/mo ceiling, 1 on alert, 2 when it
stopped something with `--stop`). Demonstrate the trip without real
exhaustion:

```sh
bash deploy/merchant/guard.sh --snapshot deploy/merchant/guard-snapshot-settled.json
bash deploy/merchant/guard.sh --snapshot deploy/merchant/guard-snapshot-over-limit.json --stop
```

The settled snapshot reads $14.25/mo (exit 0); the over-limit one trips
(exit 2, nothing called).

Note: `deploy/guard.sh` (T9.6) still ceilings the single-service shape;
it trips (exit 2, nothing suspended) once this service exists, until its
ceiling is raised to the combined $14.25/mo shape.

## Files

| File | Job |
|---|---|
| `main.go`, `main_test.go` | The merchant edge: static app + product_fit-only proxy |
| `Dockerfile` | The merchant image (edge + merchant build) |
| `render.yaml` | The settled shape as a blueprint |
| `provision.sh` | API create, with the settled-shape assert |
| `guard.sh` | Spend ceiling: audit, alert, suspend |
| `guard-snapshot-settled.json` | Green-shape fixture (placeholders, dry-run only) |
| `guard-snapshot-over-limit.json` | Dry-run trip fixture (placeholders, dry-run only) |
