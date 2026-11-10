# Architecture

Memento Emptor is a memory layer for shopping agents, not an agent. Any
agent built on the payment toolkit plugs into it: evidence arrives from
the payment side without the agent doing anything, the agent asks what
applies before it recommends, and reports what the shopper says. One
reference shopping agent demonstrates the layer; it lives outside the
layer, under `agent/`.

Built from the P0 research, not from the brief. Each claim below names
the recording that proves it.

## Data flow: webhook to belief

1. **Webhook delivery.** The payment source calls the hosted endpoint.
   Every delivery is verified through `verify-webhook-signature` and
   ingested idempotently on the payment source's event id; manual resend
   replays a missed event, and no polling path is used. The catalog
   product event in the ledger proves events arrive that are not
   purchases, so ingest filters by event type.
   (`dev-diary/research/paypal.md` §4, `testdata/paypal/webhook-*.json`)
2. **Outcome without reason.** A refund record carries no reason field,
   confirmed twice over on live refunds and on the webhook resource. A
   refund therefore enters the layer as an outcome with no reason and
   counts toward nothing until the shopper explains it. A dispute
   carries its reason top-level and per item, so it enters classified.
   (`research/paypal.md` §§2–3, `refund-record.json`, `dispute-get.json`)
3. **Join to the item.** Line items live on the order read, not on the
   capture; the custom id propagates order → capture → refund and
   dispute, so it is the join key. The reporting API is not authorized
   for this app and is not a fallback.
   (`research/paypal.md` §5, `order-get-approved-line-items.json`)
4. **Pending questions.** A refund the shopper never explains, or
   explains below the confidence floor, stays in `pending_questions`
   and the agent asks again. No code path turns it into a belief.
   (`research/jev.md`, `internal/domain` write rule)
5. **Classification.** Return text goes to the reason classifier as one
   exclusive-choice question over six categories: two fit directions,
   two seller-reliability causes, changed-mind, and an explicit unclear
   outlet. The floor rule asks again when the choice is unclear or
   confidence is under 0.98; on the labelled set every clear phrase
   returned 1.00 and every vague one fell below the floor.
   (`research/jev.md`, `testdata/jev/`)
6. **Structural write.** The write rule maps each signal to its graph
   operation: a classified return or dispute and explicit fit feedback
   are structural writes linked to the transaction; a general statement
   is an assertion that never promotes; a kept item is an observation
   with no edge. The shopper's correction is a structural write
   declared against the belief it contradicts, applied at once with
   precedence while the belief stays.
   (`internal/domain`, `research/lambo.md` checks 3–4)
7. **Promotion.** The graph admits candidates on evidence alone and
   promotes on the stage's verdict. Three fit returns citing the belief,
   spread across the session's time extent with seeded event times, take
   it to Venerable; the demo's bar is Venerable, shown in the ledger as
   Earned. One dispute leaves the sizing belief untouched and does not
   itself promote. (`research/lambo.md` checks 1–4)
8. **Recall.** Before recommending, the agent recalls for the shopper
   and product: the beliefs that apply, each with status and evidence,
   and any correction that overrides them, marked so the agent cannot
   read the belief without seeing the override. The contradiction
   warning renders on recall and on inspect while the belief stays
   Venerable. (`research/lambo.md` checks 3 and 5)

## What lives where

| Store | Holds | Never holds |
|---|---|---|
| **Evidence graph** | Short, keyed beliefs and evidence stubs; contradiction flags; supersession; provenance; per-agent attribution. Sessions isolate shoppers on a shared store file. | Prose. The graph holds stubs; the documents live in the evidence store. |
| **Evidence store** | Raw evidence documents with their vectors; hybrid text-plus-vector search over past purchases; the graph's recall tier. | Beliefs. Search finds evidence; promotion decides belief. |
| **App's own store** | Durable order state (proposed → approved → captured, or failed and refunded) with one open order per shopper and need; webhook event ids for idempotency; seeded marks; ledger reads. | Money movement. Orders and captures are issued from the reference agent, never from the layer. |

The evidence store keeps the existing hosted deployment; no index is
self-hosted. Hybrid search needs the `retriever: { rrf: ... }` shape on
this deployment; the graph re-scores neighbours itself, so that syntax
note covers hand-written queries only. The graph's recall tier needs a
20 s timeout against this deployment's cold handshake; the failure mode
without it is a dropped error source, recorded for the graph's
maintainers. (`research/elastic.md`, `research/lambo.md` check 6)

## Configuration

Keys come from the system keyring or the environment and are never
written to a file. The loader exports the seven code-read variables and
names a missing key without printing a value. There is no `.env`.

The embedder sits behind the `EmbedderContract` interface in
`internal/domain`: a kind name plus validation, no model name, no
dimensions, no endpoint, no key. The direction is native embeddings
served by the evidence store itself. Three filed graph issues
(connect timeout, missing calibration row, dropped timeout source)
closed in graph 0.10.1; adopting the native contract still needs
calibration confirmation on the demo shopper plus console billing, then
re-embedding. Until then every session
matches the contract it was built on. (Handoff log: issues 236–238;
`research/elastic.md`; `research/elastic-native.md`.)

Graph cadence (sweep interval, eval interval) lives in the graph's own
config file; the promotion bars live in the graph's code and are not
settable. The classifier model is pinned; a version change re-opens the
floor measurement. (`research/lambo.md`, `research/jev.md`)

## Dependencies

| Piece | Job | Without it |
|---|---|---|
| **Payment evidence source** | Outcomes with reason codes, line items, webhooks; the approved purchase | Nothing happens and nothing is evidenced |
| **Evidence graph** | Short, keyed, evidence-gated beliefs; contradiction flags; provenance | The agent can store facts but cannot tell an earned belief from a claim |
| **Evidence store** | Raw evidence and vector search; the graph's recall tier | Prose ends up in the graph and evidence cannot carry across products |
| **Reason classifier** | Return text to category with calibrated confidence | Every refund is a guess about its cause |
| **Catalog and agent host** | Candidate products; the reference agent as a service with approval, correction forms, feedback tied to traces, and evals | There is nothing to recommend and no independent proof the agent checked memory first |

The layer itself depends only on the first four. Catalog data and the
agent host belong to the reference agent, so a builder swaps in their
own catalog and host. Correctness of purchases — idempotent request
ids, durable order state, webhook idempotency — lives in the payment
path of the reference agent; coordination hints from the graph are
never what prevents a double buy. (`product.md` § Double-buy
protection; `research/astropods.md`; `research/channel3.md`)
