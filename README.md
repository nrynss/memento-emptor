# Memento Emptor

**PayPal tells your agent what happened to the money. Memento Emptor tells it why, and what every other shopper's returns already taught.**

*Memento emptor*: "remember, buyer". *Caveat agens*: "let the agent beware".

**Watch** [the demo video](https://www.youtube.com/watch?v=N31hivEIdrc) (2:55) · **Read** [the docs](https://nrynss.github.io/memento-emptor/) · **Try** [the hosted demo](https://memento-emptor.onrender.com/judges)

## Right on day one for a new shopper

A shopper new to a brand gets a guess from most agents. Memento Emptor answers from the pool. Other buyers' classified returns already say the shirt runs small. The agent sizes up and cites the counts.

In our comparison, only the full layer got the new-to-brand case right. It scored 3 of 4 by the committed scores and 4 of 4 by hand. Memento shopper-only scored 0 of 4. PayPal history scored 0 of 4. No memory refused all four.

Read the limits beside that number. We built the case so the pool is the only input holding the answer. Two shoppers share one product line. The pool is a seeded population, not live traffic. The result shows the mechanism, not real-world accuracy. The counts and runs are in [bench/round2/README.md](bench/round2/README.md).

A correction also travels. It reaches the next session and another agent at 4 of 4 and 4 of 4 in both Memento arms. That portability is the supporting point.

## What it is

Memento Emptor is middleware: a memory layer a merchant runs beside its PayPal app. PayPal's refunds, disputes and captures go in. Any shopping agent the merchant runs calls it over MCP before it recommends. It is not an agent. Any agent built on PayPal's Agent Toolkit plugs into it.

Shopping agents start each session with no memory. They repeat mistakes the shopper already paid for. Three returned size-M shirts from a brand that runs small mean nothing to the next session.

A refund alone explains nothing. It says the money came back, not why. Memento Emptor keeps the outcome, the item and the reason apart. It learns why each return happened. It learns at two levels: about this shopper, and about this product across shoppers. A belief at either level must be earned from evidence. The shopper stays in charge of every decision.

### Who runs it

A seller or a platform runs one layer beside its PayPal app. The evidence belongs to the seller. Refunds, disputes and captures are read under the merchant's PayPal token. Webhooks go to the merchant's PayPal app.

| Who | How they use it |
|---|---|
| The seller's shopping agents | Call the layer with their own key. Recall before recommending. Report reasons, feedback and corrections. Orders stay on the Agent Toolkit. The layer never moves money. |
| PayPal | Delivers the seller's refund, dispute and capture webhooks. The layer turns them into evidence with no agent involved. |
| The seller's shoppers | Sign in to the ledger. See, contest and correct what the agents believe. Opt out of pooling. Each sees only their own memory. |
| The seller | Reads the merchant app: why its products come back, by reason and size. |

Memory carries across agents within one layer. It does not follow the shopper to another seller. The pool covers one layer's sales. "Runs small" means across this seller's buyers, not all of PayPal.

## The evidence model

A payment outcome is not a cause. Every piece of evidence keeps three things separate: the outcome, the item and variant, and the reason. Only evidence whose reason matches a belief counts toward it. A fit return counts toward "Brand X runs small". A parcel that never arrived counts against the seller, never toward sizing.

Three kinds of belief exist. **Shopper fit** is how a brand fits this shopper. **Product fit** is how a product runs for everyone, pooled across shoppers. **Seller reliability** is whether a seller delivers what was ordered.

Status is a label, never a gate. Claimed, Building, Earned and Out of true travel with every item recall returns. The agent says how sure it is. The layer never withholds evidence because it has not reached Earned.

The shopper's correction wins at once, and the belief stays. Recall returns the correction with precedence and declares it against the belief it contradicts. It never deletes the belief. Recall resolves what applies in a fixed order: the correction, the shopper's own fit evidence, the pooled product fit, the shopper's general statement, then nothing said plainly. It never refuses. Thin evidence comes back labelled thin.

Pooling is opt-out. A shopper's classified outcomes count toward the pool unless they say no. Saying no stops new fit rows and removes the ones already written. The pool carries reasons, not people. No shopper identity, free text or order detail leaves the shopper's own memory.

The full contract is in [docs/interface.md](docs/interface.md). The design is in [docs/architecture.md](docs/architecture.md).

## Plug your agent in

The layer serves eleven MCP tools: eight layer calls and three ledger reads. Discover them with `tools/list`. Call them with `tools/call`. The edge also accepts `POST <base>/tools/<tool>`.

An agent calls with a key issued to it. Send the key in `Authorization: Bearer` and the shopper id in the body. An agent reads only the shoppers its seller serves. A shopper reads only their own ledger. The webhook path accepts only verified PayPal deliveries.

The eight agent tools:

| Tool | What it does |
|---|---|
| `recall_for_purchase` | The shopper's evidence and beliefs, the pooled product fit, any correction, and the precedence step that decided. It never refuses. |
| `record_correction` | The shopper's explicit correction, applied at once and declared against the belief it contradicts. It stands on its own when no belief exists yet. |
| `record_reason` | The shopper's answer about a refund, tied to that refund and classified. |
| `record_feedback` | Fit feedback tied to a specific purchase. |
| `pending_questions` | Refunds still missing a reason, so the agent knows what to ask. |
| `set_pooling` | The shopper's opt-out, or opt back in. |
| `explain_belief` | Why the layer believes something: every piece of evidence and its reason. |
| `product_fit` | The pooled fit belief for a product or brand, its status and counts by reason. No shopper identities. This is the merchant's read. |

The turn pattern is short. Recall first, before any turn that may end in a recommendation. Recommend from the guidance and its precedence step. Ask about the gaps `pending_questions` reports. Record what the shopper says. Honour consent. Your trace should show recall before the recommendation. That ordering is the independent proof your agent checked memory first.

Orders, approval and double-buy protection stay in your agent, on the toolkit's orders tools with idempotent request ids. The layer only remembers what happened to the money.

## Run it

### The hosted demo

The demo runs against hosted services. No setup is needed to read it.

- The layer and the ledger app: `https://memento-emptor.onrender.com`
- The merchant app: `https://memento-merchant.onrender.com`
- The judges' start-here page: `https://memento-emptor.onrender.com/judges`

The ledger without a shopper token replays the demo story from fixtures and labels itself a replay. With a shopper link token it reads the layer live. The recorded run is [demo/RUN-6.md](demo/RUN-6.md), with per-step evidence in [demo/evidence/run-6/](demo/evidence/run-6/).

### Build and test from a clean clone

You need Go 1.27, Node 26 and npm. Bun is optional. The gate checks the agents' offline contracts when bun and their modules are present, and says so when it skips them.

```sh
npm ci --prefix web
npm ci --prefix merchant
npm ci --prefix docs-site
./tools/check.sh
```

The gate runs the freeze checks, the toolchain pin check, gofmt, vet, the Go tests with the race detector, then the web, merchant and docs-site builds and tests. CI runs the same script on every push. The pins are Go 1.27, Node 26, Bun 1 and Lambo 0.10.1.

### Run the layer locally

A local run needs no key. The fixture embedder computes vectors in process. It needs the app tree the image builds: the three Go binaries, the pinned Lambo release binary, the web build and the test data. Run the clean-clone steps above first, so `web/build` exists. Then assemble the tree:

```sh
APP=/tmp/memento-local/app
mkdir -p "$APP/bin" "$APP/testdata/seed"
CGO_ENABLED=0 go build -trimpath -o "$APP/bin/memento" ./cmd/memento
CGO_ENABLED=0 go build -trimpath -o "$APP/bin/edge" ./deploy/edge
CGO_ENABLED=0 go build -trimpath -o "$APP/bin/seed" ./deploy/seed
REL=https://github.com/nrynss/lambo/releases/download/v0.10.1/lambo-0.10.1-linux-x86_64
curl -fsSL -o "$APP/bin/lambo" "$REL"
echo "$(curl -fsSL "$REL.sha256" | cut -d' ' -f1)  $APP/bin/lambo" | sha256sum -c -
chmod +x "$APP/bin/lambo"
cp -r web/build "$APP/web"
cp -r testdata/api testdata/jev "$APP/testdata/"
cp -r testdata/seed/population "$APP/testdata/seed/"
```

Then boot:

```sh
APP_DIR=/tmp/memento-local/app LAMBO_DATA_DIR=/tmp/memento-local/data \
  MEMENTO_IDENTITY_SALT=$(openssl rand -hex 32) \
  LAMBO_EMBEDDER=fixture LAMBO_SEED=0 LAMBO_MCP_TOKEN=local-run-token \
  PORT=18081 LAMBO_MCP_PORT=17724 LAMBO_WEB_PORT=17723 \
  bash deploy/start.sh
```

The salt names the shopper sessions. Any 64-hex value works for a local run. The edge answers `/healthz` on port 18081 and serves the app at `/`. The MCP endpoint answers 401 until a caller sends a key, as on the hosted demo. The full runbook, including the Docker image and the seeded arc, is in [deploy/README.md](deploy/README.md).

### Keys for a live run

Keys come from the system keyring or the environment. They are never written to a file in this repository. There is no `.env`. [.env.example](.env.example) names every variable the code reads and holds no value.

```sh
source tools/env.sh
```

The loader exports each key from the keyring and names a missing one without printing a value. PayPal is sandbox only. No live PayPal credential enters this repository.

## Architecture

One Render web service serves the demo: the edge, the memory layer, the ledger app and one Lambo hub. The hub holds every shopper's session on a persistent disk. The merchant app runs as its own service.

| Store | Holds | Never holds |
|---|---|---|
| Evidence graph (Lambo) | Short, keyed beliefs and evidence stubs. Contradiction flags, supersession, provenance. One session per shopper. | Prose. Pooled beliefs. |
| Evidence store (the layer) | Each shopper's raw evidence texts and ledger rows, journalled on disk. A restart keeps the rows and their citations. | Beliefs. Pooled counts. |
| Fit index (Elastic) | One structured row per purchase, aggregated into the pooled product-fit belief. The merchant app reads it through `product_fit`. | Beliefs, shopper identity, prose. |
| App's own store | Durable order state and webhook event ids for idempotency. | Money movement. |

Lambo holds the shopper. Elastic holds the pool. Each shopper is one Lambo session, keyed by a salted hash of their PayPal payer id. Sessions isolate shoppers, so one shopper's beliefs never reach another's recall. The pool lives in the fit index instead, because a pooled session would break that isolation. The hub runs store-only. It does not use Lambo's Elastic recall tier. The layer reads the fit index directly.

The reference agent and the second agent live under `agent/`. They run on Astropods. The merchant app lives under `merchant/`. The ledger app lives under `web/`. The builder docs site lives under `docs-site/`.

## Sponsors we built on

Six sponsor tools carry the build. Each row says what it does here and what we ran into. The full write-up is in [docs-site/src/content/docs/sponsors.md](docs-site/src/content/docs/sponsors.md), and the evidence is in each tool's research file.

| Sponsor | What it does here | Our feedback |
|---|---|---|
| **PayPal** | The evidence and the action. Verified webhooks bring in captures, refunds and disputes with no agent involved. Dispute reason codes become classified reasons. The Agent Toolkit's orders tools make the approved sandbox purchase with idempotent request ids. | The webhook event simulator answered 404 on both sandbox hosts, so we drove real events. No API showed a sandbox merchant's home currency. Manual resend worked; we never observed an automatic retry. A refund carries no reason, which is the gap this layer fills. A reason field on refunds would help every agent. [research](dev-diary/research/paypal.md) |
| **AG Grid** | Both data screens: the shopper's evidence ledger and the merchant app's returns by line, size and reason. Community 36.3.0, light and dark. | About 1.2 MB minified (340 KB gzip), so the ledger loads it lazily. v36 themes use `adoptedStyleSheets`, which jsdom lacks, so theme tests stay structural. Tests must wait for `firstDataRendered`. [research](dev-diary/research/ag-grid.md) |
| **Channel3** | Real products and buy links for the reference agent. Per-SKU fit labels tell the layer when a new shirt shares a cut with returned ones. | Fit vocabulary is per category (`top-fit`, `footwear-cut`), and SKUs in one line can carry different fit labels, so comparison is per SKU. A normalised cut field would help. [research](dev-diary/research/channel3.md) |
| **Elastic** | The pooled fit index, `fit-rows`: one structured row per settled purchase, with a salted per-line shopper hash and no identity or free text. Aggregations by line, size, reason and direction, with exact distinct-shopper counts, decide the pool's status. The merchant app reads it through `product_fit`. Elastic's jina-embeddings-v3 inference endpoint also embeds Lambo's memory. | Serverless refuses external versioning on the partial `_update`, so each row is a whole-document write with `version_type=external_gte`. A cold first connection from our services took about 6.9 s against about 236 ms warm, so recall keeps a warm connection and a 20 s timeout. The project is a Vector Database project with an active licence, not a trial, which the trial-status endpoint (410 in serverless) did not make obvious. [research](dev-diary/research/elastic-fit.md) |
| **Astropods** | Runs both shopping agents, the reference agent and a second agent with its own prompt, as deployments. Model calls go through the platform gateway with no personal model key. Every turn's trace shows its tool spans, the independent proof an agent recalled before recommending and honoured the correction. Evaluators score traces, and traces of interest become dataset items. | `ast agent restart --component` rejected both the component and workload names, so we redeployed instead. A deployed spec must declare `agent.interfaces.messaging`, or the chat host 404s behind a sign-in that looks healthy to anonymous checks. A no-login page needs the console's Public switch; the CLI's access options keep sign-in. Scripted messaging calls need a browser user agent to pass the bot check. [research](dev-diary/research/astropods.md) |
| **Render** | The hosted demo: one Docker service for the edge, layer, ledger and Lambo hub on a disk, a second for the merchant app, and the public PayPal webhook endpoint. Pushes to `main` deploy automatically. | An env var change does not redeploy on its own. A disk-backed service has no zero-downtime deploy, so each deploy briefly drops webhooks; PayPal's resend and event-id idempotency cover it. [research](dev-diary/research/hosting.md) |

Also built on: Lambo (graph memory per shopper) and Jev via OpenRouter (the reason classifier).

## The comparison

We ran two rounds. Both report counts, never percentages. Both state their limits first. We wrote the shoppers and the hidden answers, so each shows the mechanism on constructed cases, not real-world accuracy.

**Round 1** asked whether the evidence pipeline or the belief gate carries the answer. The arm given the layer's classified evidence scored 15 of 15. The arm gated on Earned belief refused and scored 0 of 15 true recommendations. Wrong-cause errors were 0 in 45 runs. The lesson drove the pivot: keep the pipeline, turn the gate into a label, and add the pool. The record is [bench/README.md](bench/README.md).

**Round 2** ran four arms with the same agent, model and catalog: no memory, PayPal history alone, Memento shopper-only, and Memento full. Eight constructed shoppers, four cases, two repeats, 112 turns. First-turn right size, committed: arm 1 0/16, arm 2 4/16, arm 3 10/16, arm 4 12/16. A hand reading of the same replies gives 0/16, 4/16, 12/16 and 16/16. It is kept beside the scores and never substituted for them. Wrong-cause errors were 0 in all 112 turns. Spend was $0.789309 of the $5.00 cap. The record is [bench/round2/README.md](bench/round2/README.md).

## What is simulated and seeded

- **Purchases are simulated.** They run in PayPal's sandbox through our own merchant account. Channel3 gives real product data and buy links, not the right to take payment for another retailer. Real-retailer fulfilment is simulated.
- **The payment history is seeded.** Sandbox history has to be generated. Every seeded row carries a mark, and the ledger shows it.
- **The pooled population is seeded too.** The shoppers behind a pooled belief are generated and marked. The pool shows how many of its settled purchases are seeded.
- **"Kept" is inferred.** A kept item without explicit feedback is a weak signal, never confirmed satisfaction.
- **No retailer logins.** Reasons come from PayPal dispute codes and the shopper. A refund the shopper never explains counts toward nothing.

The demo record names what is live and what is not, step by step. See [demo/README.md](demo/README.md).

## Where this could go next

A network across sellers is not built. Many sellers feeding one shared layer would let memory and the pool follow the shopper everywhere. It needs consent and trust at the seller level. It is a different product. We describe it as the direction, not as something that ships here.

## Licence

Apache License 2.0. See [LICENSE](LICENSE).