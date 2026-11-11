---
title: What the layer is
description: "A builder's introduction to Memento Emptor: the problem, the evidence model, the two levels of memory, precedence, and the limits."
---

Memento Emptor is a memory layer for shopping agents. It is not an agent, a
storefront, or a payment processor. Any agent built on PayPal's Agent Toolkit
can call it over MCP, and one reference agent exists to demonstrate it.

The layer's job is narrow and stated in its name: *memento emptor*, "remember,
buyer". It remembers why a payment outcome happened, keeps that reason tied to
the transaction, and decides what applies the next time the shopper is about to
buy.

## The problem

Shopping agents start every session with no memory, so they repeat mistakes the
shopper has already paid for. Three returned size-M shirts from a brand that
runs small mean nothing to the next session. A shopper buying a brand for the
first time gets a guess, even when hundreds of earlier buyers already returned
that shirt for being tight.

Preferences built only from what people *say* are weak. Payment outcomes alone
explain nothing either: a refund says the money came back, not why. A non-arrival
dispute and a sizing return can look the same in a transaction list, and only one
of them says anything about fit.

## What it does differently

The layer joins the two sources without letting either take over:

- **Outcomes** come from PayPal: refunds, disputes with reason codes, captures,
  line items, and webhooks.
- **Reasons** come from PayPal's dispute codes, or from the shopper when the
  agent asks why a refund happened. Return text is classified into a fixed set
  of categories, with low confidence meaning ask again.
- **Evidence counts toward a belief only through a reason that matches it.** A
  fit-related return can support "this brand runs small". A dispute over an item
  that never arrived counts against the seller, never toward sizing. A refund
  the shopper never explains counts toward nothing.

That rule is the centre of the product. No code path turns a payment outcome
into a belief without a classified reason that matches it.

## Two levels of memory

| Level | About | Earned from |
|---|---|---|
| **Shopper fit** | How a brand's sizing fits *this shopper* | This shopper's fit-classified returns and purchase-tied fit feedback |
| **Product fit** | How a product or brand runs *for everyone* | The same classified outcomes, pooled across shoppers, one interaction per shopper's purchase |
| **Seller reliability** | Whether a seller delivers what was ordered | Non-arrival and not-as-described disputes, by PayPal reason code |

Shopper beliefs live in an evidence graph, one session per shopper. Product-fit
beliefs do not: a pooled session would put every shopper's evidence into one
recall surface and break isolation. Instead, each classified outcome and each
purchase-tied fit verdict also writes one structured row to a fit index, with no
prose and no identity, and the pooled belief is an aggregation over settled rows. The pool counts distinct shoppers
within a product line; it cannot link one shopper's rows across lines.

The two levels are why a new shopper can still get a useful answer. On day one,
the shopper's own memory is empty, but the pool can say the line runs small
across the shoppers who bought it. The agent sizes up from the shopper's usual
size and cites the pool by its counts.

## The evidence model

Every piece of evidence keeps three things separate:

| Part | Source | Example |
|---|---|---|
| **Outcome** | PayPal: refund, dispute, capture | A refund of $34 on one order |
| **Item and variant** | The order's line items, plus catalog variant data | Brand X oxford shirt, size M |
| **Reason** | A dispute's PayPal reason code, or the shopper's answer, classified | "Too tight in the shoulders" becomes *fit: too small* |

Not returning something is not satisfaction. The signals are deliberately
unequal:

| Signal | Strength | How it is written |
|---|---|---|
| A return or dispute with a classified reason | Strong | A structural write linked to the transaction |
| Explicit fit feedback on a specific purchase | Strong | A structural write linked to the transaction |
| Kept past the return window, no feedback | Weak, inferred | An observation, never a structural edge. Only the seeded population writes kept rows today |
| A general statement ("I'm usually an M") | Assertion | A derived note, which never promotes |

A live purchase never settles as kept: the layer models no return window and
nothing decides that one has closed. It settles on a classified return reason or
on purchase-tied feedback. The seeded population is where kept rows come from,
and they carry the seeded mark.

## Status is a label, never a gate

Every belief and every recall item carries a status:

| Ledger shows | Graph state | Meaning |
|---|---|---|
| **Claimed** | Assertion or None | The shopper said it in general, or too little evidence exists to act on; it is not evidence |
| **Building** | Candidate | Evidence is accumulating |
| **Earned** | Venerable | Enough documented evidence, over enough time, to act on |
| **Out of true** | Contradicted, kept and shown | Something newer disagrees |

Recall **never refuses** because the evidence is thin. It labels the evidence and
says which rule decided. An empty answer says so plainly instead of pretending a
belief exists.

## What applies to a purchase

The layer resolves what applies in a fixed order and says which rung it used:

1. **The shopper's correction**, if one is in force.
2. **The shopper's own fit evidence** for this brand, at any status.
3. **The pooled product-fit belief**, adjusted from the shopper's usual size only
   when the pool says the product runs off-size.
4. **The shopper's general statement**, labelled Claimed.
5. **Nothing**, said plainly.

A correction is a first-person statement from the shopper ("I've lost weight, use
M"). It wins the recommendation at once, because waiting for evidence would make
the agent stubborn. The belief it contradicts is not deleted: recall returns both,
marks the belief contradicted, and embeds the correction inline so no agent can
read the belief without seeing the override. The belief changes only when new
evidence earns the change.

When two rungs disagree, both are returned and the lower one is marked
overridden. Nothing is hidden.

## The pool is counts, not people

Pooling is opt-out. A shopper's classified outcomes count toward the pool unless
they say no; saying no stops new fit rows and removes the rows already written.
The pool carries reason categories, sizes, direction and counts. No shopper
identity, free text or order detail leaves the shopper's own memory.

The pool says nothing below three distinct shoppers. Its thresholds are fixed
before any comparison run:

| Status | Rule for one product line and size over settled rows |
|---|---|
| **Building** | At least 3 distinct shoppers with a fit-classified outcome, and not yet Earned |
| **Earned** | At least 8 distinct shoppers in one direction; that direction is at least 60% of fit-classified settled outcomes; and it is at least twice the opposite direction |
| **Out of true** | Earned overall, but the last 90 days of event time, taken alone, fail the Earned rule for that direction or meet it for the opposite one |

A purchase still inside its return window, and a refund with no reason yet, count
toward nothing. `settled` is the denominator, so a new product does not look
perfect before its returns arrive.

## What it will not do

- Refuse a recall because the evidence is thin. It labels the evidence, names the
  deciding step, and says plainly when nothing applies.
- Show one shopper's rows to another shopper, or to the merchant app. The pool
  carries counts, never a row.
- Turn a refund or dispute into a belief without a reason that matches it.
- Promote a general statement or a standing correction.
- Delete a belief. The correction wins at once, and the belief stays beside it.
- Move money. Purchases, approval and idempotency live in your agent.

## Limits to know before adopting

- **Memory carries across agents within one layer.** A store's chat assistant and
  its checkout agent share what the layer knows. Memory does not follow the
  shopper to another seller.
- **The pool covers one layer's sales.** "Runs small" means across this seller's
  or platform's buyers, not all of PayPal. A marketplace has plenty of buyers; a
  small shop's lines may stay Building.
- **A network across sellers is not built.** Many sellers feeding one shared
  layer would let memory follow the shopper everywhere. It needs consent and
  trust at the seller level and is a different product.
- **A general statement has no tool.** It reaches a shopper's memory only through
  that shopper's seed. A statement recorded through a tool would be a contract
  change.
- **Keep prose out of the evidence graph.** Beliefs are short and keyed; the
  layer journals the evidence documents per shopper.

The [interface reference](/interface/) says what to call, when, and what comes
back. The [architecture page](/architecture/) says how the pieces fit.
