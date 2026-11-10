# Architecture

Memento Emptor is a memory layer for shopping agents, not an agent. Any
agent built on the payment toolkit plugs into it: evidence arrives from
the payment side without the agent doing anything, the agent asks what
applies before it recommends, and reports what the shopper says. One
reference shopping agent demonstrates the layer; it lives outside the
layer, under `agent/`. A separate read-only merchant app serves the
seller from the same layer.

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
   with no edge. The shopper's correction is a structural write applied
   at once with precedence while the belief stays; with no belief to
   declare against, it is kept as a standing record until a matching
   belief appears.
   (`internal/domain`, `research/lambo.md` checks 3–4)
7. **Promotion.** The graph admits candidates on evidence alone and
   promotes on the stage's verdict. Three fit returns citing the belief,
   spread across the session's time extent with seeded event times, take
   it to Venerable; the demo's bar is Venerable, shown in the ledger as
   Earned. One dispute leaves the sizing belief untouched and does not
   itself promote. (`research/lambo.md` checks 1–4)
8. **The second write.** The same event that writes the shopper's graph
   also writes that purchase's row to the fit index. The row is the only
   input to the pooled belief; nothing reads a shopper's graph to build
   the pool. (`research/elastic-fit.md` checks 1–2, "The second write
   path" below)
9. **Recall.** Before recommending, the agent recalls for the shopper
   and product and gets, each with a status label: the shopper's own
   beliefs, the pooled product fit, and any correction, plus the
   precedence step that decided. A correction wins at once and the belief
   keeps its standing with the warning inline; when two steps disagree
   both are returned and the lower one is marked overridden. Recall never
   refuses; thin evidence comes back labelled thin.
   (`research/lambo.md` checks 3 and 5, `research/elastic-fit.md` check 3)

## The second write path: ingest to the fit index

The pooled product-fit belief is an aggregation over structured rows in
the evidence store, not a belief in the graph. One purchase is one row.

- **One document per purchase.** The document id is
  `sha256("txn|" + transaction id)`, so the same purchase updates in
  place. Each write replaces the whole document, because the deployment
  refuses external versioning on the partial update API.
  (`research/elastic-fit.md` check 2)
- **The version is the purchase state's rank, with event time breaking
  ties.** The rank order is captured, then refunded, then reason
  classified, then settled or contested; a later state always wins, so a
  late webhook cannot overwrite it. The write uses `external_gte`, so a
  delivery retry carrying the same event rewrites the same row and is
  harmless, while an older event is refused. This supersedes the
  research's event-sequence version: the sequence was the stand-in
  before the state rank was decided. (`research/elastic-fit.md` check 2,
  `product.md` "How a fit row is written")
- **The rank's limit is documented, not moved.** Kept, feedback and
  contested share rank 4 above reason_classified at rank 3, so a reason
  classified after a kept, feedback or contested write cannot apply: the
  writer's merge keeps the higher rank whatever the event times say, and
  within rank 4 the later event wins. The rank stays as it is — it is
  frozen with the contract, and renumbering it would renumber every row
  version. In practice each purchase settles once: the kept path fires
  only for purchases never refunded, and classification answers the
  pending question the refund opened. When a classification and a
  feedback do meet on one row, they point the same way — the shopper's
  answer and their feedback agree on how the size ran — so the
  directional counts the status rule reads are unchanged; only the
  settled-kind attribution differs.
- **The row carries no prose and no identity.** Product line (brand,
  category, fit handle), size, purchase state, settled kind, reason
  category, fit direction, feedback verdict, the contested mark, a
  salted per-line shopper hash, the transaction hash, the seeded mark and
  the event times. The domain's direction words are `small` and `large`;
  the index stores the research's keywords `too_small` and `too_large`,
  and the writer maps between them. The index is created with
  `dynamic: strict`, so a prose field is refused at write time.
  (`research/elastic-fit.md` check 1)
- **Only settled rows count.** A purchase settles when it is returned
  with a classified reason, gets fit feedback, or is kept past the return
  window. A purchase still inside its window, and a refund with no reason
  yet, are stored but count toward nothing.
  (`research/elastic-fit.md` check 3)
- **The shopper key is salted per line.** `sha256(<line salt> + "#" +
  shopper id)` counts distinct shoppers within one line and cannot link
  one shopper's rows across lines. The salt is a key; it is never
  written to a file. (`research/elastic-fit.md` checks 1 and 4)
- **Seeded and live rows share one index.** Every seeded row carries the
  mark, and every aggregation splits on it, so the seeded population is
  shown as seeded. (`research/elastic-fit.md` check 5)
- **The pool is read with one line-and-size query over settled rows.**
  Filter the three line fields plus size; the denominator is the settled
  filter's count, not the query's total. Distinct shoppers come from a
  cardinality aggregation over the fit-classified rows for the Building
  floor, and one per direction for the directional counts. The
  aggregation measured about 236 ms warm; the first request on a cold
  connection took about 6.9 s, so the client keeps connections alive and
  uses a 20 s timeout, well above the measured cold maximum.
  (`research/elastic-fit.md` check 6)
- **The status rule is a pure function of the counts.** The reader calls
  it; the ledger and the merchant app read the same numbers.
  (`internal/domain`, `docs/interface.md` "The pool's status rule")

## The per-shopper hub

The graph holds one session per shopper, keyed by a salted hash of the
payer id: `shopper-` plus 64 hex characters, 72 bytes, inside the graph's
1–128 byte addressed-name rule. Sessions are the isolation boundary: one
shopper's beliefs, corrections and contradictions never reach another's
recall. A pooled session would put every shopper's evidence into one
recall surface, which is why the pool lives in the evidence store
instead. (`research/lambo-multi.md` checks 1, 2, 5; `product.md` "Lambo
holds the shopper; Elastic holds the pool")

One long-lived `lambo serve --transport http` process is the hub. A
credential with `session_prefix = "shopper-"` and `create = true` makes
it attach a shopper's session on first request at `/mcp/s/<session>`; the
tools themselves carry no session argument. Pinned sessions come from
`[serve] sessions`, on-demand ones attach up to `max_attached` (16 is the
measured recommendation) and are idle-detached; the ledger reader
`serve-web` still starts with an allowlist, so it is restarted with a
new shopper's name until the graph's issue 241 lands.
(`research/lambo-multi.md` checks 2, 7, and "The deploy shape")

Measured cost: a hub with 20 attached sessions held about 28 MiB, against
about 20 MiB per separate writer process, so the hub is the deploy shape
for a small instance. Writes are WAL with an 8 s busy timeout and one
connection per process; eight sessions writing at once produced no lock
errors and no lost write. (`research/lambo-multi.md` checks 1 and 3)

## The merchant app

The seller gets a separate, read-only service. It reaches the layer's
`product_fit` and nothing else: its key is good only for that tool, it
holds no evidence-store key, and no shopper row can reach it. It shows
one row per product line and size with the pooled status, the settled
denominator, returns by reason and direction, feedback by verdict, the
distinct-shopper counts and the seeded count. A row opens the counts
behind its status and the rule it was judged by. (`product.md` "Sellers
are the second audience", `docs/interface.md` "product_fit")

## What lives where

| Store | Holds | Never holds |
|---|---|---|
| **Evidence graph** | Short, keyed beliefs and evidence stubs; contradiction flags; supersession; provenance; per-agent attribution. One session per shopper isolates them on a shared store file. | Prose. The graph holds stubs; the documents live in the evidence store. Pooled beliefs: a pooled session would break shopper isolation. |
| **Evidence store** | Raw evidence documents with their vectors; hybrid text-plus-vector search over past purchases; the graph's recall tier. Also the fit index: one structured row per purchase, aggregated into the pooled product-fit belief and read by the merchant app. | Beliefs. Search finds evidence; promotion decides belief. The fit index holds no shopper identity, no row detail and no prose. |
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
written to a file. There is no `.env`. The loader exports the code-read
variables and names a missing key without printing a value.

The pool's salt is one of those keys: the per-line shopper hash and the
session name are salted, and the salt never appears in a committed file.
Seeded rows use a fixed, published seed salt marked as seeded, so the
seeded population replays byte for byte; live rows use the keyring salt.

The embedder sits behind the `EmbedderContract` interface in
`internal/domain`: a kind name plus validation, no model name, no
dimensions, no endpoint, no key. The direction is native embeddings
served by the evidence store itself. Three filed graph issues
(connect timeout, missing calibration row, dropped timeout source)
closed in graph 0.10.1; adopting the native contract still needs
calibration confirmation on the demo shopper plus console billing, then
re-embedding. Until then every session matches the contract it was built
on. (Handoff log: issues 236–238; `research/elastic.md`;
`research/elastic-native.md`.)

Graph cadence (sweep interval, eval interval) lives in the graph's own
config file; the promotion bars live in the graph's code and are not
settable. The classifier model is pinned; a version change re-opens the
floor measurement. (`research/lambo.md`, `research/jev.md`)

## Dependencies

| Piece | Job | Without it |
|---|---|---|
| **Payment evidence source** | Outcomes with reason codes, line items, webhooks; the approved purchase | Nothing happens and nothing is evidenced |
| **Evidence graph** | Short, keyed, evidence-gated beliefs; contradiction flags; provenance | The agent can store facts but cannot tell an earned belief from a claim |
| **Evidence store** | Raw evidence and vector search; the graph's recall tier; the fit index behind the pooled belief and the merchant app | Prose ends up in the graph, and nothing learns across shoppers |
| **Reason classifier** | Return text to category with calibrated confidence | Every refund is a guess about its cause |
| **Catalog and agent host** | Candidate products; the reference agent as a service with approval, correction forms, feedback tied to traces, and evals | There is nothing to recommend and no independent proof the agent checked memory first |

The layer itself depends only on the first four. Catalog data and the
agent host belong to the reference agent, so a builder swaps in their
own catalog and host. Correctness of purchases — idempotent request
ids, durable order state, webhook idempotency — lives in the payment
path of the reference agent; coordination hints from the graph are
never what prevents a double buy. (`product.md` § Double-buy
protection; `research/astropods.md`; `research/channel3.md`)
