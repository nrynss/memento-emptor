# Memento Emptor

**Memory for shopping agents built on PayPal, learned from what happened to the money.**

*Memento emptor*: "remember, buyer". *Caveat agens*: "let the agent beware".

The spec, for the [PayPal AI Hackathon](https://paypalaihackathon.devpost.com/). Where a dev-diary document and this file conflict, this file wins. Phases and tasks are derived from this file.

## The product

Shopping agents start every session with no memory, so they repeat mistakes the shopper has already paid for. Three returned size-M shirts from a brand that runs small mean nothing to the next session.

Preferences built only from what people *say* are weak. Payment outcomes alone explain nothing either: a refund says the money came back, not why. Memento Emptor combines the two without letting either one take over. A belief about a shopper has to be earned from evidence, and the shopper stays in charge of every decision.

**Memento Emptor is a memory layer, not an agent** (decided 9 Nov). Any shopping agent plugs into it. Evidence arrives from PayPal; the agent asks what applies before it recommends, and reports what the shopper says. One reference shopping agent exists to demonstrate it.

**It is for developers building shopping agents on PayPal's Agent Toolkit** (decided 9 Nov). PayPal already knows what happened after every purchase; Memento Emptor turns that into memory for any agent built on the toolkit. The developer adopts it. The shopper benefits, and is the face of the demo.

Proposed surface, not final:

| Interface | What it does |
|---|---|
| PayPal webhooks and transaction reads | Refunds, disputes and captures become evidence without the agent doing anything |
| `recall_for_purchase` | For this shopper and product: the beliefs that apply, each with its status and evidence, and any correction that overrides them |
| `record_correction` | The shopper's explicit correction, applied at once and declared against the belief it contradicts |
| `pending_questions` | Refunds still missing a reason, so the agent knows what to ask |
| `record_reason` | The shopper's answer, tied to that refund and classified |
| `record_feedback` | Fit feedback tied to a specific purchase |
| `explain_belief` | Why the layer believes something: every piece of evidence and its reason |

## The evidence model

**A payment outcome is not a cause.** Every piece of evidence keeps three things separate:

| Part | Source | Example |
|---|---|---|
| **Outcome** | PayPal: refund, dispute, capture | Refund of $34 on order 7XK… |
| **Item and variant** | The order's line items; Channel3 variant data | Brand X oxford shirt, size M |
| **Reason** | Disputes: PayPal's reason code. Refunds: the shopper's answer when the agent asks, classified | "too tight in the shoulders" → *fit: too small* |

Only evidence whose **reason** matches a belief counts toward it. A fit-related return counts toward "Brand X runs small". A dispute over an item that never arrived counts against the *seller*, never toward sizing.

**Not returning something is not satisfaction.**

| Signal | Strength | How it's written to Lambo |
|---|---|---|
| Return or dispute with a classified reason | Strong | `record_action`, linked to the transaction: a structural edge |
| Explicit fit feedback on a specific purchase ("the M fits well") | Strong | `record_action`, linked to the transaction: a structural edge |
| Kept past the return window, no feedback | **Weak, inferred** | Observation only, no structural edge. Means nothing before the window closes |
| A general statement ("I'm usually an M") | Assertion | `derive`: a `Derives` edge, which never promotes |

**Two kinds of belief** (decided 9 Nov): **fit**, about a brand's sizing for this shopper, and **seller reliability**. The evidence path is the same; the reason category decides which belief a piece of evidence counts toward. Cross-brand sizing, repeat buys as a preference signal, and quality or style preferences are out.

## Decision versus belief

Two separate clocks:

- **The decision follows the shopper, immediately.** An explicit, recent, first-person correction ("I've lost weight, use M") overrides the *current* recommendation. The layer returns the correction with precedence over the belief, so every agent that plugs in gets the same rule. Waiting for evidence would make the agent stubborn.
- **The belief follows the evidence.** "Size L for Brand X" stands, but recall marks it **contradicted**, so the agent and the ledger show both: *"you said M on 6 Oct; three returns said L."* The belief changes only when fit evidence for M accumulates.

Lambo keeps the history honest and the agent keeps the shopper in charge. Neither erases the other.

## The demo: one shopper, sizing first, then one dispute

1. **Three documented fit returns** teach *Brand X runs small, size up to L*. PayPal supplies each refund by webhook, the agent asks the shopper why it went back, and each classified answer is evidence for the belief, which reaches Earned.
2. **Channel3 finds a candidate**, a different SKU. It shares the cut of the returned items, and the size chart maps it to the shopper's measurements. The agent recommends the L and cites the three returns.
3. **The shopper corrects it**: "I've lost weight, use M". The very next recommendation is M. The L belief shows as contradicted, not deleted.
4. **Approval, then a PayPal sandbox purchase** of the M. After delivery the agent **asks** for fit feedback, and the "fits well" arrives tied to that purchase.
5. **A dispute that doesn't count toward sizing.** The shopper files a dispute for an item that never arrived. PayPal's reason code puts it against the seller; the sizing belief doesn't move. One dispute doesn't reach Earned, so the agent mentions it without treating it as settled. (To confirm: how the PayPal sandbox produces a dispute with a reason code.)
6. **The evidence ledger** shows what changed and why: the returns and their reasons, the correction, the purchase, the feedback, the dispute, and each belief's status. Beside it, traces show the agent recalled first, honoured the correction, and never skipped approval.

## The comparison

Decided 9 Nov. Three arms, with the same agent, model, shoppers and catalog:

| # | Condition |
|---|---|
| 1 | No memory: nothing about past purchases |
| 2 | Raw history: the shopper's PayPal transactions and past messages, handed to the agent directly |
| 3 | Memento Emptor: the layer's beliefs, evidence and corrections |

Arm 2 is the baseline that matters: "why not just give my agent the transaction history?"

Measured, and reported as counts, never percentages: right size first time against a hidden answer for each shopper; wrong-cause errors (a refund or dispute unrelated to fit changing the size recommendation); corrections honoured on the very next recommendation; citations that really support the recommendation; tokens per recommendation.

Freeze the shoppers, catalog, prompt and grader before any arm 3 run, and publish every run. We write both the seeded shoppers and the hidden answers, so this shows the mechanism on constructed cases, not real-world accuracy, and we say so. If arm 2 matches arm 3, we say so. The number the video leads with is chosen after the runs, not before.

## Shape

The stack, settled 9 Nov. Each piece has a job no other piece covers; the "without it" column is the test. The layer itself depends only on PayPal, Lambo, Elastic and Jev. Channel3 and Astropods belong to the reference agent, so a builder can swap in their own catalog and agent host.

| Piece | Job | Without it |
|---|---|---|
| **PayPal** | Outcomes (refunds, disputes with reason codes, captures, line items, webhooks) and the action (the approved purchase, with idempotent request IDs) | Nothing happens and nothing is evidenced |
| **Lambo** | Short, keyed, evidence-gated beliefs; contradiction flags; supersession; provenance; per-agent attribution | The agent can store facts, but can't tell an earned belief from a claim |
| **Channel3** | Candidate products and variants from real retailers | There's nothing to recommend |
| **Elastic** | Stores and searches raw evidence; Lambo evidence concepts carry the Elastic document ID | Prose ends up in Lambo's graph, and evidence can't carry across products |
| **Astropods** | Runs the reference agent as a service: approval and correction forms, feedback tied to traces, evals. Independent proof of how the agent behaved | Approval and feedback become ad-hoc UI, and "the agent checked memory first" is only our own claim about our own agent |
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

**The demo's bar is Venerable, shown in the ledger as "Earned"** (decided 9 Nov). Three returns can reach Venerable at most. Canonical would need six fit returns from one brand, which isn't believable, or counting the agent's own recommendations as evidence, which is circular.

| Ledger shows | Lambo state | Meaning |
|---|---|---|
| **Claimed** | Assertion, never promotes | The shopper said it in general; it isn't evidence |
| **Building** | Candidate | Evidence is accumulating |
| **Earned** | Venerable | Enough documented evidence, over enough time, to act on |
| **Out of true** | Contradicted, kept and shown | Something newer disagrees. A one-line tooltip explains the term where it first appears |

The seeded history has to get there honestly, and this is proved before anything is built on it: each return is its own interaction and cites the belief in `depends_on`; the returns span at least 0.3 of the session's time extent; three GC sweeps run; and the contradiction warning is seen on a Venerable belief.

## Double-buy protection

This belongs to the reference agent, which makes the purchase; the memory layer never moves money. Lambo's soft locks are coordination aids, not guarantees. Correctness lives in the payment path:

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

- Not a shopping agent. The reference agent is there to demonstrate the layer.
- Not a payment processor or a storefront. Purchases go through PayPal's sandbox.
- Not a recommender trained on what shoppers say. Statements never become beliefs on their own.
- Not a dispute-filing bot. PayPal's agent tools can't contest a dispute, and the product doesn't need them to.

## Cautions

- **Purchases are simulated.** In the sandbox they go through our own merchant account. Channel3 gives real product data and buy links, not the right to take payment for other retailers. Label real-retailer fulfilment as simulated.
- **The payment history is seeded.** Sandbox history has to be generated. Seeded rows are marked and easy to find.
- **"Kept" is inferred.** A kept item without explicit feedback is labelled as a weak signal, never as confirmed satisfaction.
- **No retailer logins.** Reasons come from PayPal dispute codes and the shopper. A refund the shopper never explains stays an outcome with no reason and counts toward nothing.
- **Keep prose out of Lambo.** Lambo holds short, keyed beliefs and evidence stubs.
- **Declare contradictions with `against`.** The shopper's correction writes it explicitly rather than relying on the possibly-superseded flag.

## To verify on the real stack

Assumptions this spec rests on, checked before anything is built on them.

- The seeded history reaches Venerable, and the contradiction warning shows on it (see the Lambo section).
- How the PayPal sandbox produces a refund webhook and a dispute with a reason code.
- Astropods' input forms, traces and evals behave as the concept describes, on the free tier.
- Elastic stays available through 21 Dec (a 14-day trial plus a 30-day extension only just reaches it).
- Whether Channel3's data says enough about a product's cut to match a new SKU to the returned ones.
