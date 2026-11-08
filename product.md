# Memento Emptor

**Remember, buyer. Memory for shopping agents, learned from what you did with your money and corrected by what you say.**

The spec, for the [PayPal AI Hackathon](https://paypalaihackathon.devpost.com/). Where a dev-diary document and this file conflict, this file wins. Anything still undecided is under [Open questions](#open-questions); phases and tasks come after those are settled.

## The product

Shopping agents start every session with no memory, so they repeat mistakes the shopper has already paid for. Three returned size-M shirts from a brand that runs small mean nothing to the next session.

Preferences built only from what people *say* are weak. Payment outcomes alone explain nothing either: a refund says the money came back, not why. Memento Emptor combines the two without letting either one take over. A belief about a shopper has to be earned from evidence, and the shopper stays in charge of every decision.

## The evidence model

**A payment outcome is not a cause.** Every piece of evidence keeps three things separate:

| Part | Source | Example |
|---|---|---|
| **Outcome** | PayPal: refund, dispute, capture | Refund of $34 on order 7XK… |
| **Item and variant** | The order's line items; Channel3 variant data | Brand X oxford shirt, size M |
| **Reason** | Disputes: PayPal's reason code. Refunds: the return record or the shopper's message, classified | "too tight in the shoulders" → *fit: too small* |

Only evidence whose **reason** matches a belief counts toward it. A fit-related return counts toward "Brand X runs small". A dispute over an item that never arrived counts against the *seller*, never toward sizing.

**Not returning something is not satisfaction.**

| Signal | Strength | How it's written to Lambo |
|---|---|---|
| Return or dispute with a classified reason | Strong | `record_action`, linked to the transaction: a structural edge |
| Explicit fit feedback on a specific purchase ("the M fits well") | Strong | `record_action`, linked to the transaction: a structural edge |
| Kept past the return window, no feedback | **Weak, inferred** | Observation only, no structural edge. Means nothing before the window closes |
| A general statement ("I'm usually an M") | Assertion | `derive`: a `Derives` edge, which never promotes |

## Decision versus belief

Two separate clocks:

- **The decision follows the shopper, immediately.** An explicit, recent, first-person correction ("I've lost weight, use M") overrides the *current* recommendation. That rule lives in the agent, not in Lambo. Waiting for evidence would make the agent stubborn.
- **The belief follows the evidence.** "Size L for Brand X" stands, but recall marks it **contradicted**, so the agent and the ledger show both: *"you said M on 6 Oct; three returns said L."* The belief changes only when fit evidence for M accumulates.

Lambo keeps the history honest and the agent keeps the shopper in charge. Neither erases the other.

## The demo: one shopper, one brand, sizing

1. **Three documented fit returns** teach *Brand X runs small, size up to L*. PayPal supplies each refund, the reason comes from the retailer's returns record, and each one is evidence for the belief.
2. **Channel3 finds a candidate**, a different SKU. It shares the cut of the returned items, and the size chart maps it to the shopper's measurements. The agent recommends the L and cites the three returns.
3. **The shopper corrects it**: "I've lost weight, use M". The very next recommendation is M. The L belief shows as contradicted, not deleted.
4. **Approval, then a PayPal sandbox purchase** of the M. After delivery the agent **asks** for fit feedback, and the "fits well" arrives tied to that purchase.
5. **The evidence ledger** shows what changed and why: the returns and their reasons, the correction, the purchase, the feedback, and the belief's status. Beside it, traces show the agent recalled first, honoured the correction, and never skipped approval.

## Shape

The stack as proposed in the concept. Each piece has a job no other piece covers; the "without it" column is the test. Which of these stay is an open question.

| Piece | Job | Without it |
|---|---|---|
| **PayPal** | Outcomes (refunds, disputes with reason codes, captures, line items, webhooks) and the action (the approved purchase, with idempotent request IDs) | Nothing happens and nothing is evidenced |
| **Lambo** | Short, keyed, evidence-gated beliefs; contradiction flags; supersession; provenance; per-agent attribution | The agent can store facts, but can't tell an earned belief from a claim |
| **Channel3** | Candidate products and variants from real retailers | There's nothing to recommend |
| **Kernel** | Return reasons, size charts and listing snapshots from retailer pages that have no API | Returns have no reasons, sizes don't transfer between brands, and nothing records what was promised |
| **Elastic** | Stores and searches raw evidence; Lambo evidence concepts carry the Elastic document ID | Prose ends up in Lambo's graph, and evidence can't carry across products |
| **Astropods** | Runs the agent as a service: approval and correction forms, feedback tied to traces, evals | Approval and feedback become ad-hoc UI, and "the agent checked memory first" is only a claim |
| **Evidence ledger (AG Grid)** | Beliefs, the evidence behind each, contradictions and corrections, with editable rows | The shopper can't see or contest what the agent believes |
| **Reason classifier (Jev)** | Return text to reason category, with calibrated confidence. Low confidence means ask the shopper | Every refund is a guess about its cause |
| **Render** | The hosted demo URL, the PayPal webhook endpoint, the ledger app | No public demo |

### What PayPal's tools cover

The Agent Toolkit / MCP server covers orders, refunds, invoices, disputes (list, get, accept the claim), subscriptions, shipment tracking, transaction listing, merchant insights and the product catalog. It has no payouts, no contesting a dispute or submitting evidence, and no vaulting; those need the regular REST API. Checkout needs the buyer's approval, so each purchase needs one approval or a vaulted payment method set up once.

### What Lambo gives us (v0.9.1, read from the code on 9 Nov)

| Needed | Status |
|---|---|
| Promotion only from structural evidence, never from `Derives` | Exists |
| Gates measured in event time, so seeded history counts as spread out | Exists |
| Contradiction declared with `against`, shown beside the stale fact on recall | Exists (#19, #179) |
| Undeclared replacements flagged as possibly superseded | Exists (#198) |
| Newer structural evidence demotes a canonical belief; assertions never do | Exists (#20) |
| Elasticsearch recall tier | Exists, optional (`--features recall-elastic`, #18) |

The promotion bar under the default Swarm policy:

| State | Needs |
|---|---|
| **Candidate** | Survived 3 GC sweeps, and dependents from 2 other interactions |
| **Venerable** | Dependents from at least 3 distinct interactions other than its own, covering at least 0.3 of the session's time extent. Edges younger than 60 seconds don't count |
| **Canonical** | Still meets Venerable, more than 5 dependents, outside the 300-second re-promotion cooldown |

Sweeps can be driven by `gc_interval` in `lambo.toml`. Under the `Solo` policy a concept reaches Candidate by score band with no sweeps, but later hops need the same evidence.

**Three returns alone can reach Venerable at most.** See Open questions.

## Double-buy protection

Lambo's soft locks are coordination aids, not guarantees. Correctness lives in the payment path:

- a `PayPal-Request-Id` on every order creation and capture, so retries are idempotent
- **durable order state** in the app (proposed → approved → captured, or failed and refunded), with a uniqueness rule on open orders per (shopper, need)
- webhook handling that is idempotent on PayPal's event ID

Soft locks help a second agent *notice* that a purchase is in flight. They are never what prevents it.

## Hackathon rules that shape the build

- **Closes 12 Nov, 12:00 PST (13 Nov, 01:30 IST).** Winners 21 Dec.
- The project must **meaningfully use both PayPal and AI**, with PayPal in the free sandbox and central to the project.
- A new project, or an existing one with meaningful progress during the hackathon. Lambo is a dependency; the submission says what changed.
- **A working build**: a hosted demo URL or setup instructions a judge can follow. A mockup does not qualify.
- **A public repo** with a README and an open-source licence visible in the About section.
- **A public YouTube video under 3 minutes**, with no third-party trademarks or unlicensed music, showing the project working end to end.
- A text description, a list of tools and how each was used, and **sponsor tools selected on the form** for sponsor prizes.
- Keep the demo, test credentials, trials and credits active until 21 Dec. After the deadline, don't edit the repo, video or linked materials until winners are announced.
- **Judging, equally weighted:** technological implementation, design (a complete product, not a proof of concept), potential impact (a specific problem for a specific audience, shown solved in the demo), innovation, presentation.
- **Prize fit:** Best Use of Agentic Commerce, Best Use of PayPal + AI, Most Creative, AG Grid, Channel3, Render.

## For judges (to build)

A live page whose first screen is a "For judges: start here" table: the demo, the evidence ledger, the traces, and the seeded rows clearly marked.

## Not this

- Not a payment processor or a storefront. Purchases go through PayPal's sandbox.
- Not a recommender trained on what shoppers say. Statements never become beliefs on their own.
- Not a dispute-filing bot. PayPal's agent tools can't contest a dispute, and the product doesn't need them to.

## Cautions

- **Purchases are simulated.** In the sandbox they go through our own merchant account. Channel3 gives real product data and buy links, not the right to take payment for other retailers. Label real-retailer fulfilment as simulated.
- **The payment history is seeded.** Sandbox history has to be generated. Seeded rows are marked and easy to find.
- **"Kept" is inferred.** A kept item without explicit feedback is labelled as a weak signal, never as confirmed satisfaction.
- **Never handle real retailer credentials.** Use a retailer site we control or clearly labelled test accounts; the shopper performs any login.
- **Keep prose out of Lambo.** Lambo holds short, keyed beliefs and evidence stubs.
- **Declare contradictions with `against`.** The shopper's correction writes it explicitly rather than relying on the possibly-superseded flag.

## Open questions

To talk through before phases and tasks.

1. **Memory layer or shopping agent?** The concept is a memory layer, but the demo is an agent. Is the product something any shopping agent plugs into (as Kalari plugs into a coding agent), with one reference agent for the demo, or is it the shopping agent itself?
2. **Who is it for?** The shopper, a shopping-agent builder, or a merchant? The judging asks for a specific problem for a specific audience.
3. **What belief does the demo earn?** Three returns reach Venerable at most; Canonical needs more than five dependents. Either the demo's bar is Venerable, or the belief needs more evidence (purchases, feedback, size-chart matches, recommendations that relied on it).
4. **Which pieces stay?** Kernel, Elastic and Astropods have no prize listed, and Elastic has a judge on the panel. Each has to earn its place on the product's merits.
5. **Where do return reasons really come from?** A retailer site we build and Kernel reads, the shopper's own message, or both.
6. **Does it go beyond sizing?** The same evidence model covers sellers, brands and repeat buys. How much of that is in the product, as distinct from the demo?
7. **How do we show it works?** Kalari measured with-and-without runs. The equivalent here might be the same shopper and the same catalog with memory on and off.
8. **Ledger vocabulary and tagline.** "In true / Out of true" as belief statuses, and *Caveat agens* as a tagline, are proposed and not decided.
