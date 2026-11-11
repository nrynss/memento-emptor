# Hosting on Render (T0.9)

> Current deploy (pointer, not a re-verdict): the service runs as a **Docker**
> image, not the `runtime: go` shape in the provisioning call below; the
> deviation and its reason are recorded at the top of `deploy/Dockerfile`. The
> plan, region, 1 GB disk and $7.25/month are unchanged. A second paid web
> service for the merchant view (T9.3d, `deploy/merchant/render.yaml`) brings
> the combined run-rate to $14.25/month, still against the same credit.

Decision (user, confirmed before this task): everything on Render. Lambo runs
there too, beside the service, on a persistent disk. This file records what
that costs against the credit, the endpoint shape PayPal delivers to, and
the exact calls the P6 agent replays to provision it.

Render pricing and docs below were read from the public pricing page, the
docs (`free`, `disks`, `custom-domains`, `faq`), and the public API
OpenAPI schema. No billable resource was created: the only live calls were
two read-only API listings against the existing workspace (key read from
the environment via `source tools/env.sh`, never shown).

## Why the free tier cannot hold the demo

Free web services spin down after 15 minutes without inbound traffic and
take about a minute to spin back up; while spun down, local filesystem
changes are lost; and Free services cannot attach a persistent disk at
all (`docs: free`: "Free web services ... cannot [use] persistent disks",
"Paid compute plans do _not_ spin down"). A sleeping endpoint plus an
ephemeral store fails both halves of this task: PayPal deliveries would
hit a cold service, and the Lambo SQLite file would not survive a restart.
The demo needs the cheapest paid compute plan.

## What everything-on-Render costs

Cheapest shape that meets both needs: one paid web service plus one
persistent disk. Workspace plan stays Hobby, which has no monthly fee;
compute and storage bill per workspace monthly, prorated to the second.

| Item | Choice | Rate (public pricing page) |
|---|---|---|
| Web service compute | `0.5c-512mb` (0.5 CPU, 512 MB RAM) | $7.00 / month |
| Persistent disk | 1 GB, smallest the API accepts (`serviceDisk.sizeGB`: minimum 1, defaults to 1) | $0.25 / GB / month = $0.25 / month |
| Workspace plan | Hobby | $0.00 / month |
| TLS, `onrender.com` subdomain | Included with every web service | $0.00 |
| Custom domain | Not needed for the demo or webhooks (2 included on Hobby regardless) | $0.00 |
| Bandwidth | 5 GB included on Hobby, then $0.15 / GB; inbound is free. Webhook deliveries are kilobytes each | $0.00 at demo scale |
| Build pipeline | 500 minutes included per month; the demo needs a handful of deploys | $0.00 at demo scale |

Monthly total: **$7.00 + $0.25 = $7.25 / month.**

Against the $50 credit already applied to the account (T0.1):
**$50 / $7.25 = about 6.9 months of runway**, several times over the
retention rule in product.md. The credit covers the demo, the webhook
endpoint, and the ledger app for the whole retention period with roughly
$35 to spare, before bandwidth or build overages that will not materialize
at demo traffic.

Commands run, in order (values redacted to counts; no ids recorded):

```
source tools/env.sh
curl https://api.render.com/v1/owners                        # 1 owner: key works, workspace exists
curl 'https://api.render.com/v1/services?limit=5'            # 0 services: clean slate, nothing provisioned
```

Spend: $0.00. Read-only listings create nothing billable.

## Why this instance and disk size fit

- Lambo needs the 0.10.0 binary built with `--features store-sqlite`
  (the default build omits the SQLite store and `serve` refuses it;
  T0.3). The store is one local file named by `[store] path` in
  `lambo.toml`. The whole seeded session that reaches Venerable replays
  from 164 KB of transcripts (`testdata/lambo/`), so the live SQLite
  file is hundreds of kilobytes at most: the 1 GB minimum disk holds it
  about a thousand times over, with daily snapshots on top.
- The embedder adds no local weight: the contract is embeddings
  over HTTPS (BGE-M3 via OpenRouter at the time of writing; the project
  direction is now Elastic-native once Lambo #236 lands — either way no
  local model weight), and recall is the Elastic
  Serverless deployment (T0.6), so nothing large or stateful sits in
  instance RAM beside the Go service and the Lambo process.
- 512 MB RAM / 0.5 CPU is the smallest paid plan and comfortably holds a
  Go API plus a small Rust sidecar at demo concurrency. If load ever
  disagrees, the next step up (`1c-2g`, $25 / month + $0.25 disk =
  $25.25 / month) still fits the credit with room to spare.

## Webhook endpoint shape, and why PayPal delivery is plausible

Every web service gets a public HTTPS URL of the form
`https://<service>.onrender.com`, with HTTP/2 and fully managed TLS
(pricing page; `docs: custom-domains` confirms the subdomain survives
even if a custom domain is added later). A custom domain is optional and
unneeded: PayPal delivers to any public HTTPS URL registered on the
webhook, and the registered path is free-form, e.g.
`POST https://<service>.onrender.com/webhooks/paypal`.

Plausibility rests on two already-proved halves. On Render's side: the
endpoint is public, cert-managed, and (on paid compute) never spins
down. On PayPal's side: T0.4 proved webhook CRUD, signature verification
returning SUCCESS, automatic delivery of a live dispute event, and
resend. The remaining delivery requirements belong to builders, not to
hosting: answer 2xx fast and verify the signature (T0.4's pattern),
key idempotency on PayPal's event id (T2.1's rule), and keep secrets in
environment variables or secret files, never in the repo.

Two disk caveats the P6 agent must plan around (both in `docs: disks`):

1. A disk-backed service runs exactly one instance and loses
   zero-downtime deploys: each redeploy stops the instance before the
   new one starts, so the endpoint is unreachable for a few seconds.
   PayPal retries unacknowledged deliveries (T0.4 proved resend), and
   event-id idempotency makes the retry safe, but deploys during the
   judged demo window should be avoided.
2. The disk is visible only to the running instance: not during the
   build or pre-deploy command, and not from one-off jobs. The Lambo
   binary and its `lambo.toml` must therefore ship in the service image
   (or build), with only the data directory on the disk mount
   (e.g. mount at `/var/lambo-data`, `[store] path` beneath it).

## If the credit runs out

Billing is monthly per workspace: plan fee ($0 on Hobby) plus compute
and storage prorated to the second, plus any bandwidth or pipeline
overage past the included amounts (`docs: faq`). Promo credit offsets
the invoice; before provisioning, the P6 agent confirms the remaining
credit on the dashboard billing page. If credit exhausts with no valid
payment behind it, Render suspends the services: the API models
`billing` as a suspension cause, and unpaid overage already suspends
services for the remainder of the month. Suspension means the demo and
the webhook endpoint go dark while the disk and its daily snapshots
(snapshots kept 7 days) retain the Lambo store, so recovery is
re-activating billing, not re-seeding. At $7.25 / month against $50,
this path requires several retention periods' worth of neglect; the
real pre-flight check is that a payment method is on file if Render
requires one for paid compute.

Reference only (not the decision): splitting Lambo onto its own private
service would add a second paid plan ($7.00 more, $14.25 / month total)
and buy nothing the demo needs, since the private service would still
be reached through the public one. The single web service above is the
cheapest shape that fits.

## For the P6 agent: exact provisioning calls (creates nothing yet)

Every id below is a placeholder; the workspace already exists and holds
no services. Prefer the dashboard for the one-off setup, the API for
repeatability. Either way: plan `0.5c-512mb`, region matching the
Elastic deployment's region where possible, disk `sizeGB: 1`.

API (key from the environment, never written down):

```
POST https://api.render.com/v1/services
Authorization: Bearer $RENDER_API_KEY
Content-Type: application/json

{
  "type": "web_service",
  "name": "<service-name>",
  "ownerId": "<workspace-id from the dashboard Settings page>",
  "repo": "<repo-url>",
  "branch": "<branch>",
  "serviceDetails": {
    "runtime": "go",
    "plan": "0.5c-512mb",
    "region": "<region>",
    "disk": {
      "name": "<disk-name>",
      "mountPath": "/var/lambo-data",
      "sizeGB": 1
    },
    "healthCheckPath": "/healthz",
    "envVars": [
      {"key": "PAYPAL_CLIENT_ID", "value": "<from keyring at deploy time>"},
      {"key": "PAYPAL_CLIENT_SECRET", "value": "<from keyring at deploy time>"},
      {"key": "LAMBO_ES_API_KEY", "value": "<from keyring at deploy time>"},
      {"key": "ELASTIC_URL", "value": "<from keyring at deploy time>"},
      {"key": "OPENROUTER_API_KEY", "value": "<from keyring at deploy time>"}
    ]
  }
}
```

Equivalent as `render.yaml` (infrastructure-as-code alternative):

```yaml
services:
  - type: web
    name: <service-name>
    runtime: go
    plan: starter            # dashboard name for 0.5c-512mb
    region: <region>
    buildCommand: <build>
    startCommand: <start>
    healthCheckPath: /healthz
    disk:
      name: <disk-name>
      mountPath: /var/lambo-data
      sizeGB: 1
    envVars:
      - key: PAYPAL_CLIENT_ID
        sync: false
      - key: PAYPAL_CLIENT_SECRET
        sync: false
      - key: LAMBO_ES_API_KEY
        sync: false
      - key: ELASTIC_URL
        sync: false
      - key: OPENROUTER_API_KEY
        sync: false
```

Then, on the PayPal side (pattern proved in T0.4, owned by the ingest
task): register `https://<service>.onrender.com/webhooks/paypal` for the
refund, capture, and dispute event types, and re-run the
verify-signature check from `research/paypal.md`.

Pre-flight checklist for P6: remaining credit on the dashboard billing
page; payment method on file if Render asks for one on paid compute;
region choice; the 0.10.0 Lambo binary with `store-sqlite` in the
image; no deploy during the judged window.
