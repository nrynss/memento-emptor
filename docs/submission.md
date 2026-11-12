# Submission: Memento Emptor

Draft text for the PayPal AI Hackathon Devpost form. The [README](../README.md) is the builder's guide. This file is the form's copy.

## Tagline

PayPal tells your agent what happened to the money. Memento Emptor tells it why, and what every other shopper's returns already taught.

*Memento emptor*: "remember, buyer". *Caveat agens*: "let the agent beware".

## About the project

Memento Emptor is a memory layer for shopping agents. It is not an agent. Any agent built on PayPal's Agent Toolkit plugs into it.

Shopping agents start each session with no memory. They repeat mistakes the shopper already paid for. A refund alone explains nothing. It says the money came back, not why.

Memento Emptor keeps the outcome, the item and the reason apart. It learns why each return happened. It learns about this shopper, and about this product across shoppers. A belief at either level must be earned from evidence. The shopper stays in charge of every decision.

**Right on day one for a new shopper.** A shopper new to a brand gets a guess from most agents. Memento Emptor answers from the pool. Other buyers' classified returns already say the shirt runs small. The agent sizes up and cites the counts.

In our comparison, only the full layer got the new-to-brand case right. It scored 3 of 4 by the committed scores and 4 of 4 by hand. Memento shopper-only scored 0 of 4. PayPal history scored 0 of 4. No memory refused all four.

Read the limits beside that number. We built the case so the pool is the only input holding the answer. Two shoppers share one product line. The pool is a seeded population, not live traffic. The result shows the mechanism, not real-world accuracy.

A correction also travels. It reaches the next session and another agent at 4 of 4 in both Memento arms. That portability is the supporting point.

Both comparison rounds report counts, never percentages, and state their limits first. The records are [bench/README.md](../bench/README.md) and [bench/round2/README.md](../bench/round2/README.md).

## How it works

Evidence arrives from PayPal with no agent involved. The layer verifies each webhook against PayPal before it stores anything. It then turns the refund, dispute or capture into evidence. A dispute's reason code comes from PayPal. A refund's reason comes from the shopper, classified by Jev.

The agent recalls before it recommends. `recall_for_purchase` returns the shopper's evidence and beliefs, the pooled product fit, any correction, and the precedence step that decided. It never refuses. Thin evidence comes back labelled thin.

The shopper's correction wins at once, and the belief stays. Recall returns the correction with precedence and declares it against the belief it contradicts. It never deletes the belief.

The shopper sees and contests everything in the evidence ledger. Pooling is opt-out. The pool carries reasons, not people. No shopper identity or free text leaves the shopper's own memory.

The layer never moves money. Orders, approval and double-buy protection stay in the agent, on the toolkit's orders tools with idempotent request ids.

## Built with

Every tool used in the build is below. The sponsor prizes named in [product.md](../product.md) are PayPal + AI, AG Grid, Channel3 and Render. The form's sponsor selection is confirmed at submission.

**PayPal.** The evidence source and the action. The Agent Toolkit's orders tools make the approved sandbox purchase with idempotent request ids. Webhooks deliver captures, refunds and disputes. The layer verifies every delivery before it stores anything. PayPal is sandbox only.

**Channel3.** Real product data for the reference agent. It finds candidate products and variants from real retailers, with buy links. The catalog names the line and SKU. The size comes from recall, not a size chart.

**AG Grid.** The evidence ledger and the merchant app. The ledger shows beliefs, the evidence behind each, contradictions and corrections, with editable rows. The merchant app shows returns per product line by reason and size. Both use ag-grid-community 36.3.0.

**Render.** The hosted demo. One web service serves the edge, the layer, the ledger app and the Lambo hub. The merchant app runs as its own service. Render also hosts the PayPal webhook endpoint.

**Elastic.** The fit index, `fit-rows`. It holds one structured row per purchase, aggregated into the pooled product-fit belief. The merchant app reads it through `product_fit`, never directly. The native embedder, `.jina-embeddings-v3`, also runs on Elastic. The hub runs store-only, not Lambo's recall tier.

**Astropods.** Runs the reference agent and the second agent as services. Approval and correction forms are agent-built chat turns. Feedback lands per trace, and traces of interest become dataset items. The traces are independent proof the agent recalled first and honoured the correction.

**Jev, via OpenRouter.** The reason classifier, pinned to `typesafe/jev-1.14`. It turns return text into a reason category with calibrated confidence. Low confidence means ask the shopper.

**Lambo.** The evidence graph. It holds short, keyed beliefs with contradiction flags, supersession and provenance. Each shopper is one session, keyed by a salted hash of their PayPal payer id. It is an existing dependency, pinned at v0.10.1. See below for what changed.

**Keel.** The Go framework the layer is built on, v0.9.0.

**Chaaya, Svelte 5, SvelteKit.** The ledger and merchant apps. Chaaya supplies the observables. Svelte 5 runes drive the UI.

**Astro, Starlight.** The builder docs site under `docs-site/`.

## What changed in Lambo during the hackathon

Lambo is our own existing open-source project, used here as a dependency. No Lambo code is written in this repository. During the hackathon we exercised it on the real stack and reported measured bugs as issues.

Three were fixed upstream in PR #239, released as v0.10.1:

- **#236**: the embedder's connect timeout was hardcoded at 5 s. It failed under a slow DNS stall. v0.10.1 adds `connect_timeout_ms`.
- **#237**: no calibration row for the Elastic-native embedder. v0.10.1 adds a measured jina-v3 row.
- **#238**: the recall tier dropped the timeout source from its error. v0.10.1 keeps it.

The deploy pins v0.10.1, verified by sha256. It adopts the native Elastic embedder with the new timeout knobs.

One more was found by this project and fixed upstream after the pin. **#241**: the ledger reader's session allowlist was fixed at startup, so a shopper created later could not be read. PR #242 fixed it in v0.11.0, with reads by session prefix. The deploy still pins v0.10.1. It works within the old limit and restarts the reader when the list of demo shoppers changes.

## What is simulated and seeded

- **Purchases are simulated.** They run in PayPal's sandbox through our own merchant account. Channel3 gives real product data and buy links, not the right to take payment for another retailer. Real-retailer fulfilment is simulated.
- **The payment history is seeded.** Sandbox history has to be generated. Every seeded row carries a mark, and the ledger shows it.
- **The pooled population is seeded too.** The shoppers behind a pooled belief are generated and marked. The pool shows how many of its settled purchases are seeded.
- **"Kept" is inferred.** A kept item without explicit feedback is a weak signal, never confirmed satisfaction.
- **No retailer logins.** Reasons come from PayPal dispute codes and the shopper. A refund the shopper never explains counts toward nothing.

## Try it

The hosted demo needs no setup:

- Layer and ledger: `https://memento-emptor.onrender.com`
- Merchant app: `https://memento-merchant.onrender.com`
- Judges' start-here page: `https://memento-emptor.onrender.com/judges`

A judge can also build from a clean clone. The README has the steps: `npm ci` in `web`, `merchant` and `docs-site`, then `./tools/check.sh`. A local layer run needs no key. The fixture embedder computes vectors in process.

## Video

Pending T6.4. The video is public, under 3 minutes, and shows the project working end to end. Its link goes here when it exists.

## Where it could go next

A network across sellers is not built. Many sellers feeding one shared layer would let memory and the pool follow the shopper everywhere. It needs consent and trust at the seller level. It is a different product. We describe it as the direction, not as something that ships here.

## Licence

Apache License 2.0. See [LICENSE](../LICENSE).