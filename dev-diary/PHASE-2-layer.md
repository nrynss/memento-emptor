# P2: The layer

The product. Evidence arrives from PayPal, reasons are classified, beliefs are earned in Lambo, and any agent can ask what applies. It depends only on PayPal, Lambo, Elastic and Jev.

Read first: [`../product.md`](../product.md), `../docs/architecture.md`, `../docs/interface.md`.

---

### T2.1: PayPal ingest ★
```yaml
requires:   [T1.4]
fixture-ok: yes
size:       M · mid
owns:       internal/ingest/
status:     done
```

The webhook endpoint, with signature verification. Refunds, disputes and captures become durable evidence rows, idempotent on PayPal's event ID. Line items come from the transaction. A dispute's reason code goes through T1.1's table. A refund with no reason becomes a pending question.

**Done when:** tests cover a redelivered event, a bad signature, an out-of-order refund and capture, and a dispute for each mapped code.

note: worker branch task/t21-ingest; `internal/ingest/` (Service over the fakes.PaymentSource interface, rows keyed by event id, items from the order GET, disputes classified via CategoryForDispute, refunds filed pending with nil reason). All four Done-when cases have tests; `./tools/check.sh` passes in the worktree.

---

### T2.1b: PayPal ingest, live
```yaml
requires:   [T2.1]
fixture-ok: no
size:       XS · mid
owns:       tests/live/ingest/
status:     done
```
A real sandbox refund and dispute reach the running service and produce the right rows.

**Done when:** the transcript is in the handoff log.

note: worker branch task/t21b-live; tests/live/ingest/ (live PaymentSource over REST plus the ingest drive; no *_test.go, no CI impact). Pure read-back of existing T1.3b records, nothing created: newest ledger REFUNDED and DISPUTE.CREATED, stored deliveries re-fetched, both signatures verified live, one refund row with nil reason plus pending question, one dispute row at not_arrived counting toward the seller, both redeliveries duplicate. Transcript is in the worker report; full transcript with real ids in out/ (git-ignored). `./tools/check.sh` passes in the worktree.

---

### T2.2: Reason classification ★
```yaml
requires:   [T1.4]
fixture-ok: yes
size:       S · frontier
owns:       internal/reasons/
status:     done
```
The Jev client behind the interface: the shopper's answer becomes a reason category with a confidence. Below T0.7's floor, the refund stays a pending question and the agent asks again. A refund the shopper never explains stays an outcome with no reason and counts toward nothing. Elastic's nearest labelled reasons are passed in if T0.7 showed that it helps.

**Done when:** tests replay T0.7's labelled set and match its results; the never-explained case is covered.

Notes (worker t22-reasons). `internal/reasons/` holds the client behind the
`fakes.ReasonClassifier` seam plus a pure `Settle` mapping classification to
attached reason or pending question. Replay only: fixture transport serves
`testdata/jev/` with keys asserted absent. Floor re-measured with no drift:
8 clear at 1.00 accepted, 3 vague ask again, margin 0.04; refusal maps to
unclear at 0. Neighbour context omitted by test: all 11 recorded requests
carry the answer alone and still separate, so nothing retrieved travels
with the question.

---

### T2.2b: Reason classification, live
```yaml
requires:   [T2.2]
fixture-ok: no
size:       XS · mid
owns:       tests/live/reasons/
status:     done
```
**Done when:** the labelled set gives T0.7's results on the live service, within the brief's budget; the transcript is in the handoff log.

Notes (worker t22b-live). `tests/live/reasons/` holds a main-program
driver (no `*_test.go`, so CI builds it but never runs it), plus
`run.sh`, `README.md`, and a git-ignored `out/` for raw replies. One
sequential run of all 11 phrases through the frozen client with the key
attached by the driver's transport wrapper: 11/11 categories match the
recorded table, 8 clear attach and 3 vague ask again, floor margin
unchanged at 0.04 (clear 1.00 against vague 0.96). Confidence wobbled
on two vague items (`not great` 0.94→0.93, `meh` 0.68→0.51), both
still `unclear` and far under the floor; verdicts are what gate
beliefs, and the driver fails on any category or verdict mismatch.
Model resolved to `typesafe/jev-1.14-20261014`, identical to T0.7, so
the floor stands. Total $0.000235284 for 5,602 input tokens, exactly
T0.7's total; no guard tripped. Full transcript is in the worker's
report.

---

### T2.3: The evidence store
```yaml
requires:   [T1.4]
fixture-ok: yes
size:       M · mid
owns:       internal/evidence/
status:     done
```
Raw documents in Elastic: shopper messages, classified reasons and product text. Each Lambo evidence concept carries its Elastic document ID, so prose stays out of the graph. Hybrid search answers whether a new product matches the items behind a belief, using what T0.5 found the data can support. Ledger search over the evidence.

**Done when:** tests replay T0.6's queries; a test proves a Lambo write carries a document ID and no prose body.

---

### T2.3b: The evidence store, live
```yaml
requires:   [T2.3]
fixture-ok: no
size:       XS · mid
owns:       tests/live/evidence/
status:     done
```

**Done when:** a new SKU matches the demo's returned items on the real deployment; the transcript is in the handoff log.

note: worker branch task/t23b-live. Elastic-native run (user direction):
direct-Elastic legs in `tests/live/evidence/live.py` plus a Lambo leg in
`lambo.sh`, entry `run.sh`, README how-to; no `*_test.go` so CI never runs
it. Direct legs: BM25 for a new-SKU text ranks all four same-line shirts
above both distractors; engine kNN reproduces the recorded native shape
(r2 0.74 > p1 0.65 > r1 0.64, roomier cut last); retriever RRF keeps the
reason phrases top-3 with distractors last; paraphrase cosine 0.674 vs
0.677 native / 0.68 settled. Lambo leg: 0.10.1 binary built with
store-sqlite,embed-fixture,embed-bge,embed-elastic,recall-elastic (build
only, nothing committed); `connect_timeout_ms` knob present and read
(zero refused naming the key); DNS stall from T0.6b gone (0.01s), so the
default-5s failure no longer reproduces here; two derives + backfill +
recall through `.jina-embeddings-v3` with session `calibration=calibrated`
and the 0.86 merge row flagging a 0.88 pair. Every throwaway index deleted,
empty listing verified; spend ~15 short native inference calls, no
in-band cost fields, console billing still a user step.

---

### T2.4: The Lambo writer ★
```yaml
requires:   [T1.4]
fixture-ok: yes
size:       M · frontier
owns:       internal/memory/
status:     done
```
Applies T1.1's write rule. Each reasoned outcome is its own interaction and cites its belief in `depends_on`. Corrections are declared with `against`. General statements go through `derive`. Evidence counts only toward the belief its reason matches. Reads a belief's Lambo state and maps it to the ledger's four statuses.

**Done when:** replaying the demo story gives Earned after the third return, Out of true after the correction with the belief still present, and no change to sizing after the dispute. A test proves a reasonless refund writes nothing to a belief.

Notes: built on branch task/t24-memory in internal/memory/ (doc, graph, writer, read, story + writer tests). Writer takes the cited belief's kind and refuses mismatches with ErrKindMismatch; verdict→kind lives in the writer per T1.2's handoff. Read path returns status Out of true with standing kept. Gate passes. Handoff text is in the worker's report.

---

### T2.4b: The Lambo writer, live ★
```yaml
requires:   [T2.4, T1.3]
fixture-ok: no
size:       S · frontier
owns:       tests/live/memory/
status:     done
```
The demo shopper's history against a real Lambo with T0.3's settings.

**Done when:** the belief really reaches Venerable, and the contradiction shows on recall; the transcript is in the handoff log.

Notes (worker t24b-live). `tests/live/memory/` holds the live driver
(`drive.py` MCP-stdio seeder with receipt waits and convergence polling,
`run.sh` which generates the config into gitignored `out/` so the
deployment URL never enters the repo, `README.md`), the scrubbed
32-exchange `transcript.jsonl`, and the web `api-inspect-*.json` shapes.
Two full native runs, both exit 0. Seeded on the 0.10.1 binary
(`store-sqlite,embed-elastic,recall-elastic`; build only) with
`[embedder] kind = "elastic"` on `.jina-embeddings-v3` plus 0.10.1's
`connect_timeout_ms`/`timeout_ms`: the probe reported up first try, so
the BGE-M3 fallback never ran and no `[recall]` tier was configured
(Elastic touch was `_inference` embeds only; no indices, nothing to
delete). Inspect reads `[Logic, venerable]`, blast radius 3, three
`Dependency` dependents; web gate bars met (distinct 3/3, coverage
0.5454/0.3); recall renders the `Contradicted ... by` warning naming the
correction with the belief kept beside it; the dispute leaves sizing
untouched and the seller belief below Venerable. Spend $0. Surprises for
the next live run: promotion is eventual (the driver polls inspect to
convergence; the first attempt asserted on the immediate inspect and
caught the span mid-flush), and the runner's `AGENT` var collided with
an exported `AGENT=1` in the operator shell (the script uses
`T24B_AGENT`). Full gate result in the report.

---

### T2.5: The interface calls ★
```yaml
requires:   [T2.1, T2.2, T2.3, T2.4]
fixture-ok: yes
size:       M · frontier
owns:       internal/api/server/, cmd/memento/
status:     done
```

Notes (worker t25-server). `internal/api/server/` holds the service: nine frozen tools over MCP (JSON-RPC on POST /mcp: initialize, tools/list from the frozen schemas, tools/call) plus one plain POST /tools/{name} per tool for the ledger, GET /calls[?trace_id=] for the call log, POST /webhooks/payment for ingest, GET /healthz. Plain-route errors use the shared error envelope; MCP tool errors ride inside the result with isError. `cmd/memento/` wires the real pieces behind the fakes' seams (ingest, classifier, evidence store; recording graph) and serves offline with `-seed` loading the frozen demo history. Every call is logged with shopper and trace; recall is assembled through one constructor that fails closed (ErrOverrideMissing) instead of serving a contradicted belief without its override. Responses normalize the tagless domain.Subject keys at the encode boundary so the wire matches the frozen schemas byte-exact.

Proven: all 10 testdata/api examples pass against the service on fakes (TestContractExamplesAgainstService: reads + idempotent correction byte-exact; the two minting writes modulo the minted id, which is asserted present; accepted record_reason replays the recorded clear answer since the fixture's answer text has no P0 recording and the fake never invents one); the story drive (TestStoryEndToEnd) earns Venerable on the third return, contradicts without deleting, leaves sizing struct-equal after the dispute, grows the rival on feedback, and shows recall logged before the correction under one trace; spy-over-fake proves every evidence action is a stub carrying a doc id with no shopper prose. `./tools/check.sh` passes in the worktree; live smoke: the seeded binary serves recall matching the fixture over MCP.

Handoff. (1) The graph seam here is fakes.EvidenceGraph with an unexported 15-line adapter to memory.Graph; the live swap (T2.4b/T2.6) passes the real client behind the same seam and drops the fake, no handler change. (2) Recall serves earned (Venerable) fit beliefs for the brand only; Building beliefs stay on the ledger until earned — confirm this bar with T2.6 before the agent depends on it. (3) internal/version/pin.go's tools-tagged blank import is now redundant (server imports the shared wire package for real); removing it touches version/ so it is a follow-up, not this task. (4) Seller subjects derive as seller-for-{brand} since dispute records carry no seller identity; the reference agent can supply a real seller id when one exists. (5) go.sum gained the shared package's hashes; the gate's two-way lockfile check passes with them.
The service, on Keel, serving T1.2's contract over MCP and the ledger's read endpoints. `recall_for_purchase` returns the correction with precedence over the belief. Every call is logged, so the ledger can show that a recommendation followed a recall.

**Done when:** every example in `testdata/api/` passes against the service on fakes; a test proves that a recall can't return a contradicted belief without its override.

---

### T2.6: The layer end to end ★
```yaml
requires:   [T2.5, T2.1b, T2.2b, T2.3b, T2.4b]
fixture-ok: no
size:       S · frontier
owns:       tests/live/layer/
status:     claimed:t26-live
```
With no agent: real sandbox events in, a scripted client calling the interface, real Lambo, Elastic and Jev. Refund, pending question, reason, Earned, correction, override, dispute against the seller.

**Done when:** each step's result matches product.md's demo; the transcript is in the handoff log.

---

## Exit Criteria

- [ ] PayPal events become evidence, idempotently (T2.1, T2.1b).
- [ ] Reasons are classified, and unclear ones ask again (T2.2, T2.2b).
- [ ] Raw evidence lives in Elastic, not in Lambo (T2.3, T2.3b).
- [ ] Beliefs are earned, contradicted and kept as product.md says (T2.4, T2.4b).
- [ ] The interface serves any MCP client (T2.5).
- [ ] The whole story works live with no agent (T2.6).
