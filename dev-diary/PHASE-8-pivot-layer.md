# P8: The layer, after the pivot

Recall that never refuses, corrections that stand on their own, the fit index written at ingest, the pool read back, and one Lambo process serving every shopper. Everything in P9 and P10 demonstrates this.

Read first: [`../product.md`](../product.md), [`PHASE-7-pivot-contracts.md`](PHASE-7-pivot-contracts.md), `research/lambo-multi.md`, `research/elastic-fit.md`, and the P2 tasks whose packages you change.

---

### T8.1: One Lambo process for every shopper
```yaml
requires:   [T0.11, T7.2]
fixture-ok: yes
size:       S · mid
owns:       deploy/start.sh, deploy/README.md
status:     claimed:t81-hub
```
Move the deploy from one writer for one session (`LAMBO_SESSION`) to the shape T0.11 recommends: one `lambo serve --transport http`, a credential with `session_prefix = "shopper-"` and `create = true`, `max_attached` 16, and sessions named by T7.2. `serve-web` starts with the demo shoppers as its allowlist, until [Lambo issue 241](https://github.com/nrynss/lambo/issues/241) lands. The layer addresses each shopper at `/mcp/s/<session>`.

**Done when:** `start.sh` brings up the hub locally and a test drives two shoppers through it with no cross-read. `deploy/README.md` says how a shopper session is created and how the ledger reader's allowlist is set. The gate passes.

### T8.1b: The Lambo hub, live
```yaml
requires:   [T8.1]
fixture-ok: no
size:       XS · mid
owns:       tests/live/hub/
status:     not-started
```
The T8.1 shape against the live Elastic recall tier: three shoppers created on demand, each recalls only its own concepts, and the ledger reader serves the allowlisted ones. Budget: under $0.20 of embedding spend. Run one shopper first and check its cost. Test sessions are erased afterwards.

Also exercise the 503 + `Retry-After` refusal with all `max_attached` places full, and measure `--max-sessions 128` and `--rate-limit-rps 200` on the 0.5 CPU starter.

**Done when:** the transcript is in the handoff log, with spend and the erase shown.

---

### T8.2: The fit index writer ★
```yaml
requires:   [T7.1, T7.2]
fixture-ok: yes
size:       M · mid
owns:       internal/fitindex/, internal/ingest/
status:     done
```
Ingest writes a fit row beside every Lambo write, from the same event: capture creates the row, a refund marks it returned, a classified reason gives it a category and direction, fit feedback and kept-past-the-window settle it, and a contested reason updates it. Each write replaces the whole document at the state-rank version with `external_gte` (T0.12: the partial `_update` API refuses external versioning). The index is created from T7.1's mapping. Only the layer's interface to the index is named in domain code (developer rule 6).

**Done when:** a fixture test drives one purchase through every state, in order and shuffled, and ends at the same row. A replayed webhook is harmless, and an older state never overwrites a newer one. A refund with no reason is stored but not settled. A shopper's correction writes nothing to the index. An opted-out shopper's events write no row, and opting out deletes their existing rows across every line their session knows, while leaving other shoppers' rows alone. The gate passes.

**Notes (in-review).** `internal/fitindex/` is the only writer of pooled rows. `Index` is the layer's whole interface to the index (`Create`, `Put`, `DeleteShopper`); `Writer` is one shopper's writer, built with the identity keyer, that shopper's id and a `Lines` resolver, and it holds each purchase's facts for the life of the process so it can replace the whole document every time. The transitions are `Captured`, `Refunded`, `Classified`, `Feedback`, `Kept` and `Contested`; `Record(domain.Evidence)` is what ingest calls, and it maps a payment capture, refund or kept outcome onto them. A refund's evidence never carries a reason through ingest, so `Record` stores it returned and unsettled: a caller holding an already-classified refund, as T9.1's seed rows do, must call `Refunded` and then `Classified`, or the row never settles. `Forget(lines)` recomputes the per-line hash for the shopper's known lines and deletes those rows; `WithPooling` is the consent check that stops new rows. `Mapping()` carries the recorded mapping's fields and types, whitespace aside, pinned against `testdata/elastic-fit/mapping.json`; `HTTPIndex` creates the index, writes `<index>/_doc/<txn_hash>?version=<row.Version()>&version_type=external_gte&refresh=true`, maps a 409 to `ErrOlderEvent` and deletes by query with the three line fields and the shopper hash. Ingest gained one seam: `WithFitRecorder` takes a `FitRecorder` and every capture and refund delivery records its fit row before storing the evidence, so a failed fit write fails the delivery and the retry writes both. A dispute writes no fit row.

Five things the next agent should not have to rediscover:

- `domain.Item` has no fit handle, so the writer asks the catalog through `Lines` rather than guessing one. `agent/catalog` is where a production resolver belongs; the tests use a function. This is a new seam, not a contract change: `domain.ProductLine` and the fit row are unchanged.
- The reason category is written as `domain.Category` names it (`damaged_or_defective`, `not_arrived`). The recorded population's `damaged` and `not_as_described` were the harness's synthetic labels, and T0.12 says the classifier's categories slot in. Only the direction is mapped (`small`/`large` to `too_small`/`too_large`).
- The document has no `event_seq`: T7.1's state-rank version supersedes T0.12's sequence, and the recorded mapping keeps the unused field. `refresh=true` is on each write so a read after a purchase sees its row; the index is one document per purchase, not a bulk path.
- The transaction id is the capture id, with the order id as the fallback when the link carries none; the same value on capture and refund. The two ids hash to different rows, so a purchase whose events disagree gets two. A multi-item order shares one capture id, so it would share one row, and an item in a second line would overwrite the first. The demo and the seed buy one item per purchase, and T9.1's population and T10.3's generated orders must keep it that way. If multi-item orders arrive, the row key needs a decision, not a workaround here.
- A reason of `unclear` or `none` is stored and leaves the row unsettled: it is not a cause, so the refund stays a pending question. A contest replaces the category and direction in place and keeps the first `settled_at` while the row stays settled; a correction that leaves no cause clears it. The event time still breaks ties inside the last rank.

**Review fixes (in-review).** Three defects the review found are fixed inside this package, each with a test that fails on the old behaviour. `HTTPIndex.DeleteShopper` reads `_delete_by_query`'s `failures` and returns `ErrIncompleteDelete` with the count that did go, so a partial opt-out cannot report success (`TestDeleteShopperReportsPartialFailures`). `Writer.record` takes the writer's lock before the consent check and holds it through the index write, and `Forget` holds the same lock across its deletes, so a write that passed the check cannot land behind the opt-out (`TestForgetWaitsForAWriteThatPassedTheConsentCheck` blocks a write inside the index call, runs Forget, releases the write and asserts the row is deleted and nothing remains). A contest that leaves the row unsettled clears `SettledAt` and `SettledKind` in the merged row, not only in the document (`TestContestWithNoCauseUnsettles` pins the zero time).

The writer is not wired into the served layer yet: `internal/api/server` builds its ingest service per shopper and owns `record_reason`, `record_feedback`, `record_correction` and `set_pooling`, and this task does not own that package. T8.5 passes the shopper's identity and a line resolver when it builds the writer, resolves the same line for the explicit `Classified`/`Feedback`/`Contested` calls (only `Record` asks the resolver), and calls `Forget` with the lines the shopper's session knows after recording the opt-out. T9.1 builds the seeded population through the same writer with `Seeded` set and the published seed salt; T8.2b uses `NewHTTPIndex` and `Create` against the live deployment.

### T8.2b: The fit index writer, live
```yaml
requires:   [T8.2]
fixture-ok: no
size:       XS · mid
owns:       tests/live/fitindex/
status:     claimed:t82b-live-fit
```
One sandbox purchase through capture, refund and reason, written to a test-prefixed index on the live deployment, with a replayed webhook. No model spend. Test indices deleted afterwards, with an empty listing shown.

**Done when:** the transcript is in the handoff log.

---

### T8.3: The pool reader ★
```yaml
requires:   [T7.1, T8.2]
fixture-ok: yes
size:       M · mid
owns:       internal/pool/
status:     claimed:t83-pool-reader
```
`product_fit` over settled rows only, with T0.12's queries: the denominator, fit returns by direction, feedback by verdict, distinct shoppers by direction, and the seeded split. The status comes from T7.1's rule function, never a second copy of it. The connection stays warm, with a 20 s timeout, so a cold TLS handshake (about 6.9 s in T0.12) never lands on a recommendation. When the index can't be reached, the pool says so and recall carries on without it: it never pretends to an empty pool.

**Done when:** fixture tests reproduce T0.12's 600-purchase numbers exactly. The unreachable index is a distinct, tested outcome. The gate passes.

### T8.3b: The pool reader, live
```yaml
requires:   [T8.3]
fixture-ok: no
size:       XS · mid
owns:       tests/live/pool/
status:     not-started
```
A small seeded population in a test-prefixed index, read back through `product_fit`, with warm and cold latency recorded. No model spend. Test indices deleted afterwards.

**Done when:** the transcript and the latencies are in the handoff log.

---

### T8.4: Corrections that stand on their own ★
```yaml
requires:   [T7.1]
fixture-ok: yes
size:       S · mid
owns:       internal/memory/
status:     done
```
A correction with no belief is written as a standing record with no `against`, and never promotes. When a matching belief is later written for the same subject, the standing correction is declared `against` it, once. A correction against an existing belief keeps today's path. The belief is never deleted (developer rule 4).

**Done when:** fixture tests cover a correction before any belief, the belief arriving later, a second matching belief not re-declaring, and today's path unchanged. The gate passes.

**Notes (in-review).** `internal/memory/correction.go` adds the standing path beside the unchanged `RecordCorrection`: `Correction` (words, `domain.Subject`, `domain.ProductLine`, size, event time, `Against`), `Belief` (content, kind, subject, line), `StandingCorrections` with `Record`, `Declare`, `Len` and `Standing`, and `EnsureBeliefDeclaring`, which derives a belief and then declares every matching standing correction against it. `Record` derives the words as a note (`ConceptNote`), so the record has no structural edge, no score and no canonization contribution; `Declare` writes one structural action per correction with the belief in `against` and nothing in `depends_on`, so the belief keeps its standing, dated by the correction's own event time. `Declare` skips every correction that already carries an `Against`, so a second matching belief declares nothing; a later correction is its own journal entry and never revives an earlier declaration; the same words about the same subject, line and size stay one entry, standing or declared. `internal/memory/subject.go` holds the match in one place, `SameSubject`: both sides must name the same subject kind and id, a fit belief must be about a brand and a seller belief about a seller, and the product line compares field by field only where both sides set it, so a side naming no line spans every line of its subject. The size is deliberately not a match key: a correction and a belief naming different sizes are exactly the pair a correction contradicts, so requiring equal sizes would never declare the disagreement. `internal/memory/correction_test.go` covers the correction before any belief, the belief arriving later and declared once, a second matching belief not re-declaring, a later correction not reviving the first, today's path unchanged (three returns, `Out of true` over `Venerable`, three dependents kept), a standing correction declared against an already-earned belief, the match table, and the refusal and repeat cases.

Three things the next agent should not have to rediscover:

- The journal is the caller's per-shopper state, not graph state. `memory.Graph` reads one concept by its exact content (`Inspect`) but cannot enumerate a session's concepts, and a correction's subject, line and size live only in the journal, never in the graph, so a process restart loses the ability to declare. T8.5 owns the per-shopper state that will hold it; rebuilding the journal from a shopper session needs an enumeration and structured fields the seam does not offer. The issue text is in the task report.
- The domain's write table still maps `SignalCorrection` to `record_action`, and that is right for the declared path; `Declare` checks it before every declaration. The standing record does not go through the table at all, because it cites no transaction and no belief and must earn nothing.
- `tests/` and `internal/api/server/` do not call the new path yet. `RecordCorrection` still refuses an unknown belief id, so the served layer keeps today's behaviour until T8.5 wires the standing branch and `EnsureBeliefDeclaring`.

---

### T8.6: Authenticated callers ★
```yaml
requires:   [T7.1, T7.2]
fixture-ok: yes
size:       M · frontier
owns:       internal/auth/, deploy/edge/
status:     done
```
Today the MCP endpoint has no authentication: anyone who reaches the URL can read any shopper's memory by `shopper_id`. product.md ("How it is consumed") requires every caller to be authenticated before anything with real shoppers goes live.

- **Agents:** a key per agent, issued by the seller running the layer, sent as a bearer token. A key is bound to that seller's shoppers; a call for any other `shopper_id` gets the same refusal as an unknown key, so the endpoint can't be used to find out who exists. Keys are stored hashed, can be revoked, and come from the keyring or Render secrets, never a file.
- **Shoppers:** the ledger reads and `set_pooling` act only for the signed-in shopper. For the hackathon, sign-in can be a per-shopper link token issued by the seller (the demo shopper's is a sandbox credential, safe to publish on the judges' page). Record the choice and its limits.
- **Merchant app:** its own key, good only for `product_fit`.
- **Webhooks:** unchanged. Verified PayPal deliveries only, as T2.1 built.

The middleware sits in `internal/auth/` and in front of every route at the edge. The tools' contract doesn't change: auth travels in headers, not in tool arguments.

**Done when:** table tests cover each caller kind with a good key, a wrong key, a revoked key, and a good key asking for another seller's shopper, with the refusals identical where they must be. A test proves no route but `/healthz` and the webhook answers without a key. The gate passes.

**Notes (in-review).** `internal/auth/` owns the whole check. `Keyring` is built once at boot from three environment variables — `MEMENTO_AGENT_KEYS`, `MEMENTO_SHOPPER_TOKENS`, `MEMENTO_MERCHANT_KEYS` — each holding whitespace-separated entries of `id|sha256|seller|shoppers|flags`, where flags is `-` or `revoked`. The entry holds the **digest** of a token, never the token: `IssueKey` mints a token (kind prefix plus 32 random bytes as unpadded base64url), returns it once with the entry line, and the deployment keeps only the entry. `HashToken` renders a digest for an entry, `NewToken` mints without a binding, and every token starts `memento_agent_`, `memento_shop_` or `memento_merchant_`. Revocation is the `revoked` flag (or dropping the entry) plus a restart, because the ring is read at boot. A missing variable is not an error — that kind then refuses everyone — and a malformed entry stops the edge from starting, because a typo must not widen or narrow access. `Authenticate` hashes the presented token and compares every entry in constant time without stopping the scan.

`Guard` is the middleware. `Surface()` lists the edge's routes and `newMux` in `deploy/edge/` mounts the guard on every one, so the route table cannot drift from what is gated: `GET /healthz` and `POST /webhooks/paypal` (and the layer's own `/webhooks/payment`) are public, and `/mcp`, `/tools/{tool}` and `GET /calls` need a key. The guard authenticates before it reads a body, then reads the tool (`/tools/{tool}` from the path, `/mcp` from `params.name`) and the shopper (`shopper_id`, at the top level of a tool body or of `params.arguments`), puts the body back byte for byte, and applies the table in `surface.go`. Every shopper-scoped tool admits an agent bound to that shopper and a shopper token naming themselves; the ledger reads (`ledger_beliefs`, `ledger_belief`, `ledger_evidence`) are shopper-only; `product_fit` names no shopper and admits every kind, so the merchant key reaches it while the scoped tools refuse the merchant. `GET /calls` is agent-only: the log carries every shopper's entries and the served layer does not scope it per shopper, so a shopper token must not read it — T9.2 needs a scoped call log from the layer (a T8.5 change) if the ledger's trace panel is to read it. Any failed authentication — no key, unknown key, revoked key, bad scheme, another seller's shopper, a shopper nobody serves, a missing `shopper_id` — answers with one fixed 401 `{"error":{"code":"unauthenticated","message":"unauthorized"}}` and `WWW-Authenticate: Bearer`; a valid key on a tool its kind may not use answers 403 `forbidden`. The guard strips client-sent `X-Memento-*` headers and the `Authorization` header, then sets `X-Memento-Caller-Kind`, `X-Memento-Caller-Id`, `X-Memento-Seller` and, when scoped, `X-Memento-Shopper` for the layer behind it. `X-Memento-Shopper` is set only where the guard checked the caller against that shopper: `product_fit` and an unknown tool send none, whatever the body names, so the layer may read that header as a shopper the caller is allowed to act for. `doc.go` records the shopper sign-in choice and its limits (bearer credential, no expiry, not the payment identity) and the key grammar.

Seven things the next agent should not have to rediscover:

- **The ledger app must send the token.** The SPA is static and public; T9.2 has to read the demo shopper's link token (`<ledger URL>/?t=<token>`) and put it in `Authorization: Bearer` on every `/tools/...` read, plus `record_correction` for the contest path. The demo token is a sandbox credential safe to publish; an agent key and the merchant key are Render secrets.
- **Deployment wiring is outside this task's `owns`.** `deploy/render.yaml` needs the three variables as `sync: false` env vars, `deploy/provision.sh` pushes them beside the five it already sends, `.env.example` should name them by name, and `tools/env.sh` should read them from the keyring; `deploy/start.sh` needs no change because the edge inherits the environment. Nothing invokes `IssueKey` yet — minting a key is a Go call today, so a task that wires the deployment owes a way to mint one without writing code. With no keys configured the edge still starts and refuses every API call, loudly in its log, while `/healthz` stays green: a keyless deploy reads as healthy.
- **The layer behind the edge does not check keys.** T8.5 consumes `X-Memento-*` (or `auth.PrincipalFrom` if it ever mounts in-process) and must not re-read `Authorization`, which the guard removes. Anything reaching the layer's port directly bypasses auth, so the port stays on loopback.
- **`product_fit` carries no shopper, so the merchant key is bound to no shopper.** Its seller scoping is a single-layer assumption: one seller runs one layer. Enumerating a seller's lines is still T9.3's open question.
- **The refusals are tested as bytes, not just statuses.** `TestRefusalsAreIdentical` compares body, status and the whole header set across every refusal, so a future message tweak that distinguishes an unknown key from another seller's shopper fails the gate. `TestForbiddenDoesNotVaryWithTheShopper` does the same for the 403 a merchant key gets, across shopper names, so the kind check cannot become an existence oracle.
- **`TestEveryToolIsClassified` fails when the contract grows.** A new tool must be given a rule in `surface.go`; otherwise the guard would pass it to any authenticated caller.
- **The webhook stays public on purpose, and the layer's verifier is the only gate there.** The guard refuses nobody on that path because T2.1's verification belongs to the payment source. Until that verification is real, a delivery naming any `shopper_id` is accepted if its event id verifies: a known event id can write evidence into any shopper's session, though reading that session still needs a key. The edge cannot turn this into a key requirement without breaking deliveries, so a real signature check is the fix, not a wider guard.

### T8.7: Verified webhook deliveries
```yaml
requires:   [T2.1, T8.2]
fixture-ok: no
size:       M · frontier
owns:       internal/payments/, internal/ingest/
status:     claimed:t87-webhook-verify
```
The webhook path is public by design, and its only gate is T2.1's verifier, which today replays a fixture: a known event id can write evidence into any shopper's session. Build the real adapter against PayPal's `POST /v1/notifications/verify-webhook-signature` with the transmission id, timestamp, signature and raw body, using the sandbox app's credentials. Reject anything that does not verify, keep the delivery idempotency T2.1 built, and record a good, a tampered, a replayed and an unknown delivery. Budget: no model spend; sandbox calls are free.

**Done when:** the four deliveries are recorded with their outcomes, a recorded PayPal delivery verifies live, a tampered body gets the same refusal as any unverified delivery, and the gate passes.

---

### T8.5: Recall never refuses ★
```yaml
requires:   [T8.1, T8.3, T8.4, T8.6]
fixture-ok: yes
size:       M · frontier
owns:       internal/api/server/, internal/precedence/, internal/standing/, cmd/memento/main.go
status:     not-started
```
Wire the pivot into the served layer. `recall_for_purchase` reads the shopper's session and the pool, and resolves the five-step precedence order as a pure function in `internal/precedence/`: correction, own evidence, pool (adjusted from the usual size), statement, nothing. Every item carries its label. When steps disagree, both come back with the lower one marked overridden. `record_correction` takes the optional belief. `product_fit` and `set_pooling` are served. Thin evidence comes back labelled thin, never as a refusal. Hold one `StandingCorrections` per shopper, serialised (two agents may speak for one shopper), and make the journal survive a restart: a layer-owned store or a Lambo enumeration/structured-attributes mechanism. A restart must not lose the ability to declare a standing correction against a later belief. Decide whether an identical repeated correction refreshes its event time, since precedence keys on recency. The standing branch of `record_correction` accepts an empty belief id, and every belief write calls `EnsureBeliefDeclaring`. Scope `/calls` to the caller's shopper (or document it agent-only). Also:
- resolve a purchase's line from one source for every writer call, so a caller cannot pool a row under the wrong product;
- pass a consistent capture id through the writer, and decide the order-id fallback's two-row hazard;
- hydrate `captured_at` after a restart or document the loss;
- treat `fitindex.ErrIncompleteDelete` as not done and retry `Forget` with the session's full line list;
- add a `memento key issue` command using `auth.IssueKey` that prints the token once;
- keep the MCP decoder strict (single object, unknown fields rejected); if batches are ever accepted, the guard must scope them or refuse.

**Done when:**
- A table test covers every step deciding, every pair of disagreeing steps, and the unreachable pool.
- Today's story test still passes, with the correction first.
- New story tests pass for: a new shopper decided by the pool; a shopper unlike the crowd decided by their own evidence; a standalone correction honoured, then still in force in a later session.
- The gate passes.

### T8.5b: The layer end to end, live ★
```yaml
requires:   [T8.5, T8.1b, T8.2b, T8.3b]
fixture-ok: no
size:       S · frontier
owns:       tests/live/precedence/
status:     not-started
```
The live layer through the hub and a test-prefixed fit index: one shopper per precedence step, plus a correction made in one MCP session and read in a fresh one. Budget: under $0.50. Run one case first and check its cost. Test sessions and indices removed afterwards.

**Done when:** every step decides at least once live, the cross-session correction holds, and the transcript and spend are in the handoff log.

---

## Exit Criteria

- [ ] One Lambo process serves every shopper, live (T8.1, T8.1b).
- [ ] Every purchase writes and updates its fit row at ingest, live (T8.2, T8.2b).
- [ ] The pool reads back settled-only counts and its status, live (T8.3, T8.3b).
- [x] A correction stands on its own (T8.4).
- [x] Every caller is authenticated, and no route returns a shopper's memory without a key (T8.6).
- [ ] Webhook deliveries are verified against PayPal's signature, not a fixture (T8.7).
- [ ] Recall never refuses, and says which step decided, live (T8.5, T8.5b).
