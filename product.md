# Memento Emptor

**PayPal tells your agent what happened to the money. Memento Emptor tells it why, and what every other shopper's returns already taught.**

*Memento emptor*: "remember, buyer". *Caveat agens*: "let the agent beware".

The spec, for the [PayPal AI Hackathon](https://paypalaihackathon.devpost.com/). Where a dev-diary document and this file conflict, this file wins. Phases and tasks are derived from this file.

## The product

Shopping agents start every session with no memory, so they repeat mistakes the shopper has already paid for. Three returned size-M shirts from a brand that runs small mean nothing to the next session. And a shopper buying a brand for the first time gets a guess, even when hundreds of earlier buyers already returned that shirt for being tight.

Preferences built only from what people *say* are weak. Payment outcomes alone explain nothing either: a refund says the money came back, not why. Memento Emptor combines the two without letting either one take over. It finds out why each refund happened, keeps that reason tied to the transaction, and learns at two levels: about **this shopper**, and about **this product across shoppers**. A belief at either level has to be earned from evidence, and the shopper stays in charge of every decision.

**Pivot (decided 10 Nov).** The comparison (T5.5 to T5.9) showed which half of the original design works. The arm called "raw history" was handed rows the layer had already classified, with reasons, signal strength and write kind, and it scored 15 of 15 and 4 of 4 with no wrong-cause errors. The arm that gated on Earned belief refused instead and scored 0 of 15. So the evidence pipeline works and the per-shopper belief gate, on its own, costs right answers. The product keeps the pipeline, turns the gate into a label, and adds the one thing a single shopper's history cannot have: what other shoppers' classified returns say about the same product.

**Memento Emptor is a memory layer, not an agent** (decided 9 Nov). Any shopping agent plugs into it. Evidence arrives from PayPal; the agent asks what applies before it recommends, and reports what the shopper says. One reference shopping agent exists to demonstrate it.

**It is for developers building shopping agents on PayPal's Agent Toolkit** (decided 9 Nov). PayPal already knows what happened after every purchase; Memento Emptor turns that into memory for any agent built on the toolkit. The developer adopts it. The shopper benefits, and is the face of the demo.

**Sellers are the second audience** (decided 10 Nov). The buyer side stays first: developers of shopping agents, with the shopper fronting the demo. Sellers on PayPal adopt the other half: the same pooled reasons tell them why a SKU comes back, which a refund total never does. They get their own read-only **merchant app**, a separate service that reads the pool only through the layer's `product_fit`, never Elastic directly, so no shopper row can reach it.

**Memory belongs to the shopper, not the agent** (decided 10 Nov). It is keyed to the shopper's PayPal identity, so a correction made to one agent reaches every other agent that plugs into **the same layer**. A different seller's layer never sees it.

## How it is consumed

Decided 10 Nov. The evidence belongs to the seller: refunds, disputes and captures are read under the merchant's PayPal token, webhooks go to the merchant's PayPal app, and the Agent Toolkit runs on a merchant's REST credentials (`research/paypal.md`). So **a seller or a platform runs one Memento layer beside its PayPal app**: a store, or a marketplace with a shopping assistant.

| Who | How they use it |
|---|---|
| **The seller's shopping agents** | Call the layer over MCP with their own key: recall before recommending, then report reasons, feedback and corrections. Orders stay on the Agent Toolkit; the layer never moves money |
| **PayPal** | Delivers the seller's refund, dispute and capture webhooks to the layer, which turns them into evidence with no agent involved |
| **The seller's shoppers** | Sign in to the ledger to see, contest and correct what the agents believe, and to opt out of pooling. Each sees only their own memory |
| **The seller** | Reads the merchant app: why its products come back, by reason and size |

Two limits follow, and the product says them plainly:

- **Memory carries across agents within one layer**, such as a store's chat assistant and its checkout agent. It does not follow the shopper to another seller.
- **The pool covers one layer's sales.** "Runs small" means across this seller's or platform's buyers, not all of PayPal. A marketplace has plenty of buyers; a small shop's lines may stay Building.

**Every caller is authenticated** (decided 10 Nov). An agent calls with a key issued to it, and reads only the shoppers its seller serves. A shopper reads only their own ledger. The webhook path accepts only verified PayPal deliveries, as today. Nothing reachable from the public URL returns a shopper's memory without one of these.

**Not built: a network across sellers.** Many sellers feeding one shared layer would let memory and the pool follow the shopper everywhere. It needs consent and trust at the seller level, and it is a different product. The README describes it as where this could go next.

Surface (the pivot's shapes are settled in T7.1):

| Interface | What it does |
|---|---|
| PayPal webhooks and transaction reads | Refunds, disputes and captures become evidence without the agent doing anything |
| `recall_for_purchase` | For this shopper and product: the shopper's own evidence and beliefs, the pooled product belief, any correction, and which one applies under the precedence rule. Every item carries its status. **It never refuses**: thin evidence comes back labelled thin |
| `record_correction` | The shopper's explicit correction, applied at once and declared against the belief it contradicts. **Stands on its own when no belief exists yet** (decided 10 Nov): it is kept as a standing record with no `against`, never promotes, and is applied by the precedence order. When a matching belief appears later, the correction is declared against it then |
| `set_pooling` | The shopper's opt-out (or opt back in). Opting out stops new fit rows and removes the ones already written |
| `product_fit` | For a product or brand: the pooled fit belief, its status, and counts by reason category. No shopper identities |
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

Only evidence whose **reason** matches a belief counts toward it. A fit-related return counts toward "Brand X runs small". A dispute over an item that never arrived, or arrived not as described, counts against the *seller*, never toward sizing.

**Not returning something is not satisfaction.**

| Signal | Strength | How it's written to Lambo |
|---|---|---|
| Return or dispute with a classified reason | Strong | `record_action`, linked to the transaction: a structural edge |
| Explicit fit feedback on a specific purchase ("the M fits well") | Strong | `record_action`, linked to the transaction: a structural edge |
| Kept past the return window, no feedback | **Weak, inferred** | Observation only, no structural edge. Means nothing before the window closes |
| A general statement ("I'm usually an M") | Assertion | `derive`: a `Derives` edge, which never promotes |

**Three kinds of belief** (two decided 9 Nov, the third 10 Nov):

| Belief | About | Earned from |
|---|---|---|
| **Shopper fit** | How a brand's sizing fits *this* shopper | This shopper's fit-classified returns and fit feedback |
| **Product fit** | How a product or brand runs *for everyone* ("runs small in the shoulders") | Fit-classified returns and feedback pooled across shoppers, one interaction per shopper's purchase |
| **Seller reliability** | Whether a seller delivers what was ordered | Non-arrival and not-as-described disputes, by PayPal reason code. A not-as-described code counts in the damage category: the parcel arrived and the goods were wrong, so it is a seller fault of the same class, not a missing delivery |

The evidence path is the same for all three; the reason category decides which belief a piece of evidence counts toward. Cross-brand size mapping, repeat buys as a preference signal, and quality or style preferences are still out.

**Pooling carries reasons, not people** (decided 10 Nov). A product-fit belief holds reason categories, sizes and counts. No shopper identity, free text, or order detail leaves the shopper's own memory. `explain_belief` on a pooled belief shows counts, never another shopper's rows.

**Pooling is opt-out** (decided 10 Nov). A shopper's classified outcomes count toward the pool unless they say no. Saying no is recorded in their own memory, stops new fit rows for them, and removes the rows already written: their session knows which product lines they bought, so the layer recomputes each per-line hash with the salt and deletes those rows. No link across lines is ever stored to make this possible. The ledger offers the choice, and an agent can record it when the shopper asks.

**Status is a label, never a gate** (decided 10 Nov). Claimed, Building, Earned and Out of true travel with every item recall returns. The agent says how sure it is; the layer never withholds evidence because it hasn't reached Earned yet.

## Precedence: what applies to this purchase

Two separate clocks, as before:

- **The decision follows the shopper, immediately.** An explicit, recent, first-person correction ("I've lost weight, use M") overrides the *current* recommendation. The layer returns the correction with precedence over the belief, so every agent that plugs in gets the same rule. Waiting for evidence would make the agent stubborn.
- **The belief follows the evidence.** "Size L for Brand X" stands, but recall marks it **contradicted**, so the agent and the ledger show both: *"you said M on 6 Oct; three returns said L."* The belief changes only when fit evidence for M accumulates.

Recall resolves what applies in a fixed order (decided 10 Nov), and says which rung it used:

1. **The shopper's correction**, if one is in force.
2. **The shopper's own fit evidence** for this brand, at any status. Fit is personal: a shopper with broad shoulders who needs L where everyone else needs M keeps their L.
3. **The pooled product-fit belief**, adjusted from the shopper's usual size only when the pool says the product runs off-size. The usual size (decided 10 Nov) is the size the shopper most often kept or said fits well, in any brand. Failing that, it is their own general statement, labelled Claimed. It is never mapped across brands.
4. **The shopper's general statement**, labelled Claimed.
5. **Nothing**, said plainly.

When rungs disagree, recall returns both and marks the lower one as overridden, never hidden: *"most buyers size up in this shirt; your own two returns say your usual M is right."*

Lambo keeps the history honest and the agent keeps the shopper in charge. Neither erases the other.

## The demo: a new shopper, then one shopper's history, then one dispute

Decided 10 Nov: step 0 and step 7 are new, and the rest stands.

0. **Day one, no history.** A shopper new to Brand X asks for an oxford. Their own memory is empty, and so is their PayPal history with the brand. The pooled product-fit belief is Earned from many shoppers' classified returns ("runs small in the shoulders"), so the agent sizes up from the shopper's usual size and cites the pool by its counts. This is the move no single shopper's history can make.
1. **Three documented fit returns** teach *Brand X runs small, size up to L*. PayPal supplies each refund by webhook, the agent asks the shopper why it went back, and each classified answer is evidence for the belief, which reaches Earned.
2. **Channel3 finds a candidate**, a different SKU. It shares the cut of the returned items; the catalog names the line and SKU, and the size comes from recall (no size chart exists; see research/channel3.md). The agent recommends the L and cites the three returns.
3. **The shopper corrects it**: "I've lost weight, use M". The very next recommendation is M. The L belief shows as contradicted, not deleted.
4. **Approval, then a PayPal sandbox purchase** of the M. After delivery the agent **asks** for fit feedback, and the "fits well" arrives tied to that purchase.
5. **A dispute that doesn't count toward sizing.** The shopper files a dispute for an item that never arrived. PayPal's reason code puts it against the seller; the sizing belief doesn't move. One dispute doesn't reach Earned, so the agent mentions it without treating it as settled. (To confirm: how the PayPal sandbox produces a dispute with a reason code.)
6. **The evidence ledger** shows what changed and why: the returns and their reasons, the correction, the purchase, the feedback, the dispute, and each belief's status. Beside it, traces show the agent recalled first, honoured the correction, and never skipped approval.
7. **A second agent, same shopper.** A different agent, its own Astropods deployment with its own name, prompt and traces (decided 10 Nov), recalls through the layer and gets the correction from step 3 without being told. Then the merchant app shows Brand X the same oxford's returns by reason category and size: why it comes back, not just how often.

## The comparison

Re-scoped 10 Nov (decided). The first round (T5.1 to T5.9) stays published as it ran, with one honest relabel: its "raw history" arm was given rows the layer had already classified, so it is reported as **Memento evidence without beliefs**, not as raw PayPal history. That round showed a frontier model reasons well over clean, reasoned evidence, and that gating on Earned costs right answers.

The new round has the same agent, model and catalog in every arm:

| # | Condition |
|---|---|
| 1 | No memory: nothing about past purchases |
| 2 | **PayPal history**: exactly what the Agent Toolkit returns for this shopper (transactions, line items, refund and dispute status, dispute reason codes) and nothing the layer produced. No refund reasons, no classification, no signal strength |
| 3 | **Memento, shopper only**: the shopper's own reasoned evidence, labels and corrections, with no pool. The first round's winning arm, under its honest name |
| 4 | **Memento, full**: shopper evidence, the pool, corrections, and the precedence rule |

Arm 2 is the baseline that matters: "why not just give my agent the transaction history?" This time it gets the transaction history and only that. Arm 3 is the fair upper bound for anything one shopper's own data can do.

The shopper set has four cases, each chosen because it separates the arms for a stated reason, and fair to every arm:

| Case | What it tests |
|---|---|
| **New to the brand** | No personal history with the brand; the pool knows it runs off-size |
| **Unlike the crowd** | The shopper's own evidence disagrees with the pool, and the right answer is theirs. Naive pooling fails here |
| **Noisy refunds** | Mixed causes (damage, non-arrival, changed mind, fit) where a refund status without a reason can't tell fit from the rest |
| **Correction, then another agent** | A correction made to agent A; agent B asks next, with PayPal access but not A's conversation |

Measured, and reported as counts, never percentages: right size first time against a hidden answer for each shopper; wrong-cause errors (a refund or dispute unrelated to fit changing the size recommendation); corrections honoured on the very next recommendation, **and in the next session and by another agent**; which precedence rung decided the answer; citations that really support the recommendation; tokens per recommendation.

Freeze the shoppers, the pooled population, catalog, prompt and grader before any arm 3 or 4 run, and publish every run. We write both the seeded shoppers and the hidden answers, so this shows the mechanism on constructed cases, not real-world accuracy, and we say so. If arm 2 matches arm 3, we say so. The number the video leads with is chosen after the runs, not before.

## Shape

The stack, settled 9 Nov. Each piece has a job no other piece covers; the "without it" column is the test. The layer itself depends only on PayPal, Lambo, Elastic and Jev. Channel3 and Astropods belong to the reference agent, so a builder can swap in their own catalog and agent host.

| Piece | Job | Without it |
|---|---|---|
| **PayPal** | Outcomes (refunds, disputes with reason codes, captures, line items, webhooks) and the action (the approved purchase, with idempotent request IDs) | Nothing happens and nothing is evidenced |
| **Lambo** | Short, keyed, evidence-gated beliefs; contradiction flags; supersession; provenance; per-agent attribution | The agent can store facts, but can't tell an earned belief from a claim |
| **Channel3** | Candidate products and variants from real retailers | There's nothing to recommend |
| **Elastic** | Holds the fit index that pooled product-fit beliefs and the merchant app aggregate over (decided 10 Nov). **Not Lambo's recall tier** (decided in T8.1c): the hub runs store-only, because the served reads never ask the graph for a vector search and its one real vector use, a hybrid derive, is answered exactly in process; enabling the tier is config-only, but it would add metered Elastic ingest and a lagging approximate mirror. **Not the raw evidence store**: the layer journals that itself (see "Where the served layer keeps evidence") | Prose ends up in Lambo's graph, evidence can't carry across products, and nothing learns across shoppers |
| **Astropods** | Runs the reference agent as a service: approval and correction forms, feedback tied to traces, evals. Independent proof of how the agent behaved | Approval and feedback become ad-hoc UI, and "the agent checked memory first" is only our own claim about our own agent |
| **Evidence ledger (AG Grid)** | Beliefs, the evidence behind each, contradictions and corrections, with editable rows. Each item shows which precedence step decided it, and pooled rows show their seeded count. Its evidence panel is read from the layer's own record, which is journalled per shopper, so a restart keeps the rows and their citations | The shopper can't see or contest what the agent believes |
| **Merchant app (AG Grid)** | A separate read-only app: returns per product line by reason and size, pool status and seeded count, read through `product_fit` with no shopper rows (decided 10 Nov) | The merchant can't see why a SKU comes back |
| **Reason classifier (Jev)** | Return text to reason category, with calibrated confidence. Low confidence means ask the shopper | Every refund is a guess about its cause |
| **Render** | The hosted demo URL, the PayPal webhook endpoint, the ledger app, and the merchant app as its own service (decided 10 Nov) | No public demo |

### Where the served layer keeps evidence

The served layer keeps each shopper's raw evidence — one text per classified outcome, carrying the reason in words — in its own process store, one per shopper, journalled on disk. Beliefs live in the hub. Elastic holds the pooled fit index, `fit-rows`, and nothing else of the layer's, so the graph's evidence stubs carry the document id the layer's own store minted. (Recorded in T8.1d; the journal is the T2.3c decision.)

The journal is one file per shopper under the standing directory, beside the corrections journal. It holds the evidence partition: the documents behind the doc ids, the ledger rows with their ids, reasons, links, write operations, feedback markers and ordering, the open questions, and each belief's evidence citations and history in order. It is written after every live write that changes it and read back before the boot seed replays, so a row a live event wrote comes back under its own id, with its seeded mark and its place in the citations, and the seed names a row it also restores once instead of adding a second. A boot that restores it writes nothing, so a second restart leaves the file as it was. A service with no standing directory keeps the earlier shape: the store is still the layer's own, but a restart then keeps only what the boot replay restores.

That store keeps the evidence texts, not vectors, and the served layer runs no vector search over past purchases. The hybrid text-plus-vector search measured on Elastic is a design direction, not the shape the deploy runs.

The consequence is at the ledger. Its evidence panel is read from the layer's own record, so after a restart it holds both the live rows the journal restored and the demo story's rows the boot replay restores, each with the seeded mark it was written with, and each belief's evidence count is the citations the journal restored. No new field was needed: the frozen row already carries the seeded mark, and the belief read already carries its evidence. Recall and explain_belief read the same restored citations, so a restart keeps them instead of answering from the pool and the corrections alone.

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
| Elasticsearch recall tier | Exists, optional (`--features recall-elastic`, #18). Not the deploy's shape (T8.1c): the hub runs store-only, on its store's exact scan |

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

**Lambo holds the shopper; Elastic holds the pool** (decided 10 Nov). Each shopper is one Lambo session, keyed by a salted hash of their PayPal payer ID. Sessions are Lambo's isolation boundary (proved in T0.3, check 6), so one shopper's beliefs, corrections and contradictions never reach another's recall. Product-fit beliefs are not held in Lambo. A pooled session would put every shopper's evidence into one recall surface, which breaks that isolation, and Lambo's bar of three interactions is a recurrence test for one agent's memory, not a claim about a population. Instead, every classified outcome is also written as a structured row (no prose, no identity) to an Elastic fit index, and the pooled belief is an aggregation over it. Settled purchases for the same product and size are the denominator, so the pool says "14 of 60 bought in M came back too small", not just "14 returns".

How a fit row is written (decided 10 Nov):

- **One document per purchase**, keyed by a hash of the transaction ID and updated in place as the purchase moves on: captured, refunded, reason classified, fit feedback, kept past the window, contested. Each write replaces the whole document at a version taken from the purchase state's rank (captured, then refunded, then reason classified, then settled or contested), with event time breaking ties inside a rank (decided 10 Nov). A late webhook can never overwrite a later state, and retries are harmless.
- **Written at ingest**, by the same code that writes the shopper's Lambo session. Nothing reads Lambo sessions to build the pool.
- **Only settled rows count.** A purchase settles when it is returned with a classified reason, gets fit feedback, or is **kept past the return window**. A purchase still inside its window counts toward nothing, so a new product doesn't look perfect before its returns arrive. A refund with no reason yet is not settled.
- **The shopper key is a salted hash per (shopper, product line)**: it counts distinct shoppers within a line, and it can't link one shopper's rows across lines. The merchant app never needs links across lines.
- **Seeded and live rows share one index.** Every seeded row carries the seeded mark, and every aggregation can split on it.
- **A shopper's correction never enters the pool.** It is about the shopper, not the product. A contested reason does change the row, since it corrects a fact about that purchase.

Pooled status uses the same ledger words under an explicit rule, stated in the code and the ledger (decided 10 Nov), for one product line and size over settled rows:

| Status | Rule |
|---|---|
| **Building** | At least 3 distinct shoppers with a fit-classified outcome, and not yet Earned |
| **Earned** | At least 8 distinct shoppers in one direction; that direction is at least 60% of fit-classified settled outcomes; and it is at least twice the opposite direction |
| **Out of true** | Earned overall, but the last 90 days of event time, taken alone, fail the Earned rule for that direction or meet it for the opposite one |

Below 3 shoppers the pool says nothing. The thresholds are fixed before the comparison runs, and they are not tuned to it.

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
- Not a review site. Pooled beliefs come from classified payment outcomes and purchase-tied feedback, never from ratings or free-text reviews, and they never show one shopper's data to another.
- Not a size-chart service. The layer doesn't map sizes across brands; it reports how a product runs relative to its own label.

## Cautions

- **Purchases are simulated.** In the sandbox they go through our own merchant account. Channel3 gives real product data and buy links, not the right to take payment for other retailers. Label real-retailer fulfilment as simulated.
- **The payment history is seeded.** Sandbox history has to be generated. Seeded rows are marked and easy to find.
- **"Kept" is inferred.** A kept item without explicit feedback is labelled as a weak signal, never as confirmed satisfaction.
- **No retailer logins.** Reasons come from PayPal dispute codes and the shopper. A refund the shopper never explains stays an outcome with no reason and counts toward nothing.
- **Keep prose out of Lambo.** Lambo holds short, keyed beliefs and evidence stubs.
- **Declare contradictions with `against`.** The shopper's correction writes it explicitly rather than relying on the possibly-superseded flag.
- **The population is seeded too.** The shoppers behind a pooled belief are generated, marked like every seeded row, and the ledger shows the count of seeded contributors. The pooled comparison shows the mechanism on a constructed population, not real-world return rates.
- **One shopper can't move the pool alone.** Each shopper's purchase counts once toward a product-fit belief, however many times they return it.

## To verify on the real stack

Assumptions this spec rests on, checked before anything is built on them.

- The seeded history reaches Venerable, and the contradiction warning shows on it (see the Lambo section).
- How the PayPal sandbox produces a refund webhook and a dispute with a reason code.
- Astropods' input forms, traces and evals behave as the concept describes, on the free tier.
- ~~Elastic stays available through 21 Dec (a 14-day trial plus a 30-day extension only just reaches it).~~ Proved differently: the deployment is not a trial — it is a Vector Database project with an active licence, so there is no clock to extend (T0.6, T0.12). Staying available is a billing question for the console, not a trial window.
- Whether Channel3's data says enough about a product's cut to match a new SKU to the returned ones.
- ~~Whether one Lambo `serve` can write to many shopper sessions, or needs one writer per session. The deploy today runs one writer for one session (`LAMBO_SESSION`, default `demo-shopper`). If Lambo can't multiplex, that's a Lambo issue for the user to file, not a workaround here. (T0.11)~~ Proved: one `lambo serve --transport http` hub hosts one writer per session and many sessions, attaching on demand (T0.11); the deploy runs that hub (T8.1).
- The fit index on the Elastic deployment: per-purchase rows updated in place, settled-only aggregations by product line, size, reason category and direction, distinct-shopper counts, and its speed inside recall. (T0.12)
- What the Agent Toolkit's transaction listing actually returns for a refund, so arm 2 gets exactly that and nothing more.

## Open decisions (pivot, 10 Nov)

- **The headline.** "Right on day one for a new shopper" (the pool), or "the same memory across every agent" (portability). The comparison decides which one holds up; the video leads with that one.