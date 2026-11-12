---
title: Sponsors
description: "The sponsor tools Memento Emptor is built on, what each one does in the build, and our feedback from using them."
---

Four sponsor tools carry the build. Each section says what the tool does in
Memento Emptor, then what we ran into while building on it. The feedback comes
from the research records the build was planned on, linked under each one.

## PayPal

**What it does here: the evidence, and the action.**

- Webhooks for captures, refunds and disputes reach the layer with no agent
  involved. Every delivery is verified with PayPal before anything is stored.
- Dispute reason codes become classified reasons. A refund's reason comes from
  the shopper, classified by Jev.
- Evidence is read under the merchant's own token, which is why a merchant runs
  the layer beside its PayPal app.
- The Agent Toolkit's orders tools make the approved sandbox purchase, with
  explicit shopper approval and idempotent request ids. The layer itself never
  moves money.

**Feedback.**

- The webhook event simulator (`POST /v1/notifications/simulate-events`)
  answered 404 with an empty body on both sandbox hosts, so we drove real events
  instead.
- No API available to our app showed a sandbox merchant's home currency. A
  "seller doesn't accept payments in your currency" refusal cost us a
  replacement sandbox account.
- Manual resend delivered within about a minute. We did not observe an
  automatic retry in the sandbox.
- A refund carries no reason. That gap is why this layer exists, and a reason
  field on refunds would help every agent built on the toolkit.

Research: [`dev-diary/research/paypal.md`](https://github.com/nrynss/memento-emptor/blob/main/dev-diary/research/paypal.md).

## AG Grid

**What it does here: both data screens.**

- The shopper's evidence ledger: beliefs, the evidence behind each, status
  labels, the seeded mark, and contest.
- The merchant app: returns per product line and size, by reason and direction.
- AG Grid Community 36.3.0, themed for light and dark with Chaaya tokens.

**Feedback.**

- The bundle is about 1.2 MB minified (340 KB gzip), so the ledger loads the
  grid lazily rather than on the landing page.
- Version 36 themes apply through `adoptedStyleSheets`, which jsdom does not
  implement, so theme tests stay structural.
- Tests must wait for `firstDataRendered`, and a container measured at zero
  width renders only the first column until column virtualisation is turned off.

Research: [`dev-diary/research/ag-grid.md`](https://github.com/nrynss/memento-emptor/blob/main/dev-diary/research/ag-grid.md).

## Channel3

**What it does here: real products for the agent.**

- The reference agent searches for candidate products and variants from real
  retailers, with buy links.
- Per-SKU fit labels (`top-fit`) tell the layer when a new shirt shares a cut
  with the ones the shopper returned. The size comes from recall, not a size
  chart.

**Feedback.**

- The fit vocabulary is per category: `top-fit` for shirts, `footwear-cut` and
  `shoe-collar-height` for shoes.
- Two SKUs in one product line can carry different fit labels, so the line name
  alone does not prove a shared cut. Comparison has to be per SKU. A normalised
  cut field across categories would help.

Research: [`dev-diary/research/channel3.md`](https://github.com/nrynss/memento-emptor/blob/main/dev-diary/research/channel3.md).

## Render

**What it does here: hosting.**

- One Docker web service runs the edge, the layer, the ledger app and the Lambo
  memory hub on a persistent disk.
- The merchant app runs as a second service.
- Render hosts the public PayPal webhook endpoint, and a push to `main` deploys
  automatically. Keys live in environment variables, never in the repository.

**Feedback.**

- An environment variable change does not redeploy on its own. A new key sat
  behind the old instance until we triggered a manual deploy.
- A disk-backed service runs one instance and loses zero-downtime deploys, so
  each deploy briefly drops webhook deliveries. PayPal's resend and event-id
  idempotency cover the gap.

Research: [`dev-diary/research/hosting.md`](https://github.com/nrynss/memento-emptor/blob/main/dev-diary/research/hosting.md).

## Also built on

- **Lambo**: graph memory, one session per shopper.
- **Elastic**: the pooled fit index.
- **Jev via OpenRouter**: the reason classifier.
- **Astropods**: both agents and their traces.
