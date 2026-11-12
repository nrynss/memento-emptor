---
title: Submission summary
description: "Memento Emptor in one page: what it is, what it shows, where to try it, and what it is built on."
---

Memento Emptor is middleware a merchant runs beside its PayPal app. PayPal's
refunds, disputes and captures flow in through verified webhooks. Any shopping
agent the merchant runs calls it over MCP before it recommends. It learns why
each purchase came back, so agents stop repeating what the shopper already paid
for. The layer never moves money.

## What it shows

- **A payment outcome is not a cause.** The outcome, the item and the reason are
  kept apart. Only a matching reason moves a belief.
- **Recall never refuses.** It answers in a fixed order: the shopper's own
  correction, their own fit evidence, fit pooled across shoppers, then what they
  said in general. Every answer cites its evidence.
- **The shopper's correction wins at once**, and a second agent that never saw
  the conversation honours it.
- **Right on day one for a new shopper.** In round 2's new-to-brand case, only
  the full layer got the size right: 3 of 4 by the committed scores and 4 of 4 by
  hand, against 0 of 4 for every other setup. We built that case and the pool is
  seeded, so it shows the mechanism, not real-world accuracy. See
  [the comparison](../comparison/).

## Try it

- The hosted demo and judges' start page:
  [memento-emptor.onrender.com/judges](https://memento-emptor.onrender.com/judges)
- The merchant app:
  [memento-merchant.onrender.com](https://memento-merchant.onrender.com)
- The code, with setup steps a judge can follow:
  [github.com/nrynss/memento-emptor](https://github.com/nrynss/memento-emptor)

## Built on

PayPal (sandbox, Agent Toolkit, webhooks) and AI (Jev for reasons, Claude
Sonnet for the agents), with the sponsor tools AG Grid, Channel3, Elastic,
Astropods and Render. See [Sponsors](../sponsors/) for how each was used, and
our feedback.

The full form text is in
[`docs/submission.md`](https://github.com/nrynss/memento-emptor/blob/main/docs/submission.md).
