# P0: Ground

What every later phase codes against: the measured stack, and recordings of each outside service in `testdata/`, so most tasks run offline. Product.md's "To verify on the real stack" list is closed here.

Read first: [`../product.md`](../product.md).

---

### T0.1: Accounts and keys
```yaml
requires:   []
fixture-ok: no
size:       S · user
owns:       none (the keyring)
status:     done
```
The user creates or confirms:
- a PayPal Developer sandbox app, with one sandbox business account (the merchant) and one sandbox personal account (the shopper);
- a Channel3 key with the hackathon credits;
- an Elastic deployment (see T0.6 before starting the trial clock);
- an Astropods account on the free tier;
- a Render account with the credit applied;
- an OpenRouter key, which is how Jev is called.

Values go in the system keyring, as `service <name> account memento-emptor`, with a `kind` attribute where a service has more than one value. They aren't written to any file in the project.

**Done when:** each key is in the environment or the keyring, and T0.3 to T0.9 can read them. No value is written to any file in the repository.

---

### T0.2: Scaffold
```yaml
requires:   []
fixture-ok: yes
size:       S · mid
owns:       LICENSE, .gitignore, .env.example, AGENTS.md, go.mod, go.sum, internal/version/, web/, tools/check.sh, tools/env.sh, .github/, dev-diary/research/README.md
status:     done
```
Apache-2.0 licence. A Go module on the current Keel release, and a Svelte 5 app on the current Chaaya release under `web/`. `tools/check.sh`, the gate: formatting, vet, tests, and the web build. CI runs it. `AGENTS.md`: the rules every agent follows, drawn from README's Developer Guidelines, plus how to run the gate and where keys come from, with each variable's `secret-tool lookup` command.

`tools/env.sh`, sourced before any live run: it exports each key from the keyring under the variable name the code reads (`PAYPAL_CLIENT_ID`, and so on). A variable already set in the environment is left alone. A missing key is reported by variable name. The script holds no values, never prints one, and works from any worktree. The code reads only environment variables, and `.env` is gitignored as a precaution, though the project doesn't use one.

If Keel's layout changes the `owns` paths in the phase documents, update them in this commit.

**Done when:** the gate passes on a clean clone; CI is green; `.env.example` names every key T0.1 creates, with no values; sourcing `tools/env.sh` sets each stored key, names each missing one, and prints no value.

**Notes.** Built against Keel `v0.9.0` and Chaaya `0.6.0`, with Go `1.27.1` and Node `26`. The licence is Apache-2.0, by the user's decision. The task text above was corrected from MIT. `owns` widened from `web/package.json` to `web/`, because the scaffold writes the whole Svelte app, not one file. The gate runs gofmt, `go vet`, `go test -race`, a lockfile check, svelte-check, eslint, vitest and the web build. `tools/env.sh` was sourced under bash and zsh: a preset variable is left alone, an absent one is filled from the keyring, and a missing key is named without a value. CI has not been observed on a push yet. The workflow runs the same gate. An independent review found the web test step vacuous and the Go filter too broad. The web app now carries a real test, the gate checks the lockfile against `package.json`, and the Go filter drops only `web/node_modules`. Landed on `main` at `a34def3` after the review returned APPROVE. CI ran green on `main` at `c00e481` (run 42613167776), so every Done-when clause is met.

---

### T0.3: Lambo promotion on a seeded session ★
```yaml
requires:   []
fixture-ok: no
size:       M · frontier
owns:       dev-diary/research/lambo.md, testdata/lambo/
status:     done
```
**The gate on the whole product.** On Lambo v0.9.1 (or the current release), build a small seeded session by hand and show every condition product.md lists:

1. Three fit returns, each its own interaction, each citing the belief in `depends_on`, with event times spanning at least 0.3 of the session's time extent, take the belief to **Venerable**.
2. The three GC sweeps run from `lambo.toml` cadence settings. Record the settings, and whether `Swarm` or `Solo` is the right policy.
3. A correction declared with `against` shows the **contradiction warning on the Venerable belief**, and the belief stays.
4. A dispute with a non-fit reason adds nothing to the sizing belief.
5. The state of a belief can be read back in a form the ledger can show (Claimed, Building, Earned, Out of true).
6. How one session per shopper works in practice.

Also read how newer structural evidence affects a Venerable belief (the demotion rule was written for Canonical), and record it. Record the MCP transcripts later tasks replay: `initialize`, `tools/list`, `lambo_derive`, `lambo_record_action`, `lambo_recall`, `lambo_inspect`, `lambo_stats` with a receipt.

If any of 1 to 4 fails, this task is `blocked`, and the orchestrator asks the user with options. Nothing builds on promotion until it passes.

**Done when:** `research/lambo.md` holds each check's command and result; the transcripts are in `testdata/lambo/` with no key.

**Notes.** PASS, on Lambo v0.10.0 (local-only: SQLite plus fixture
embedder, no Elastic touch). Three fit returns with backdated event times
spanning 0.3 of the session extent took the belief to Venerable
(`distinct` 3, coverage 0.667); GC ran from `lambo.toml`
(`gc_interval = 1`, mutation trigger) under the default Swarm policy.
Correction with `against` shows the contradiction warning on the
Venerable belief (`warnings` on recall, inspect and `/api/inspect`) and
the belief stays. The non-fit dispute cites only a seller belief and the
sizing belief does not move. Ledger states read back as `None` (Claimed),
`Venerable` plus `gate_progress` (Earned), and `warnings` (Out of true);
one session id per shopper, verified by an empty second session. Newer
rival support leaves the Venerable belief standing (demotion is written
for Canonical only). Full evidence in `research/lambo.md`; transcripts in
`testdata/lambo/`.

---

### T0.4: PayPal sandbox events ★
```yaml
requires:   [T0.1]
fixture-ok: no
size:       M · frontier
owns:       dev-diary/research/paypal.md, testdata/paypal/
status:     done
```

**Notes.** Three full order → scripted shopper approval → capture cycles
against a replacement US-home sandbox merchant (the original merchant refused
USD; the INR detour is superseded); cycles 1–2 refunded, cycle 3 left open.
Refund-has-no-reason is CONFIRMED on two full records, the refund webhook
resource and the delivered event; `custom_id` propagates order → capture →
refund as the join key. Capture records carry no line items; the order GET is
authoritative. A live dispute with reason
`MERCHANDISE_OR_SERVICE_NOT_RECEIVED` was filed via the resolution center
(API-side creation stays `TRANSACTION_NOT_ELIGIBLE` after five attempts);
`CUSTOMER.DISPUTE.CREATED` auto-delivered and verifies with `SUCCESS`, and the
repeatable click path is recorded. Webhook delivery, signature verification
and resend verified on invoice, refund and dispute events; the ledger shows
no backfill. Card capture blocked by `PAYEE_NOT_ENABLED_FOR_CARD_PROCESSING`;
the event simulator 404s; transaction listing is `NOT_AUTHORIZED`. Detail in
`research/paypal.md`; 45 recordings in `testdata/paypal/`.
Against the sandbox, establish and record:

- an order created, approved by the sandbox shopper and captured, with `PayPal-Request-Id` idempotency seen working;
- a refund, and what its webhook and record really carry. **Confirm the brief's claim that a refund has no reason field**;
- a **dispute with a reason code**, and how the sandbox produces one. This is product.md's open item. If the sandbox can't, record what can stand in, and label it for the demo;
- webhook delivery to a local endpoint, signature verification, and redelivery behaviour;
- line items available from a capture or transaction read;
- which of these the Agent Toolkit or MCP server covers, and which need the REST API.

**Done when:** `research/paypal.md` answers each point with evidence; recordings are in `testdata/paypal/`, scrubbed of keys and account details.

---

### T0.5: Channel3 product data
```yaml
requires:   [T0.1]
fixture-ok: no
size:       S · mid
owns:       dev-diary/research/channel3.md, testdata/channel3/
status:     done
```
Search and fetch real products with variants through the API or MCP server. The question that matters: **does the data say enough about a product's cut** to match a new SKU to returned ones? If not, record what the match can use instead (title, description text, brand line). Choose a brand and product line with enough variants for the demo.

**Done when:** `research/channel3.md` answers the question with examples; recordings are in `testdata/channel3/`; credits used are recorded.

**Notes.** Answered over the REST API (no MCP server needed). There is no size chart or measurement field, but `structured_attributes` carries a per-category fit handle (`top-fit: Regular/Slim` on shirts, `footwear-cut` on shoes) and descriptions state the fit in words, so the match can use brand id + category + equal fit handle backed by title/description text. `variants` is null on all apparel fetched (each colorway is its own product; sizes live at the retailer), while one shoe SKU shows the documented Size-selector shape with `exists`/`available` and `option_Size=` selection. Demo line: Sunspel Button Down Oxford Shirt (3 SKUs sharing `top-fit: Regular`); Bonobos Everyday Oxford is the negative control (same line name, divergent `top-fit`). Spend: 2 searches (2 credits), 9 free calls.

---

### T0.6: Elastic evidence store
```yaml
requires:   [T0.1]
fixture-ok: no
size:       S · mid
owns:       dev-diary/research/elastic.md, testdata/elastic/
status:     done
```

Notes (worker, T0.6). Live against the existing deployment only: no trial
started, no billable resources created, all test indices deleted afterwards.
Hybrid search passes on both legs (BM25 finds the returned shirts; `snug
across the chest` lands next to `too tight in the shoulders` at cosine 0.68).
Settled embedder: BGE-M3 through OpenRouter (`baai/bge-m3`, native 1024 dims),
proved end to end through Lambo; Elastic-native creation stays blocked on
Lambo issue 233 (open, unassigned). Recall tier mechanics all pass on this
Serverless deployment with `timeout_ms = 20000` (default 5 s trips on the
~6.8 s cold TLS handshake; proposed Lambo issue text in the task report).
Hosting recommendation: keep the existing Serverless project, start nothing,
self-host nothing; the trial-clock framing does not apply (no trial on this
deployment) and the user confirms billing in the console. Evidence in
`dev-diary/research/elastic.md`, recordings in `testdata/elastic/`.
Hybrid search (BM25 plus vector) over a handful of product and return-reason documents: does a new SKU's text find the returned items, and does "snug across the chest" land near "too tight in the shoulders"? Choose the embedding model.

Lambo also uses this deployment as its recall tier: a `[recall]` section with `kind = "elastic"`, on a binary built with `--features recall-elastic`, and the key read from `LAMBO_ES_API_KEY`. The tier only indexes and searches vectors, so Lambo also needs an `[embedder]`. Creating embeddings through Elastic is [Lambo issue 233](https://github.com/nrynss/lambo/issues/233); this project moves to it once a release includes it. Changing the embedder later means re-embedding, so settle it before seeding the demo shopper. Confirm the tier's index creation, external-version writes, kNN search and `_delete_by_query` all work on the chosen deployment, and that recall falls back to the store when the index is down.

**User decision before the trial starts:** the demo must stay up until winners are announced (product.md). A 14-day trial plus the 30-day extension only just reaches that if started now. Options: start the trial now, or self-host Elasticsearch beside the service. Recommend one with the evidence.

**Done when:** `research/elastic.md` records the search results, the embedding choice and the hosting decision; recordings are in `testdata/elastic/`.

**Notes.** Landed on 0.9.1-era findings: Serverless deployment (no trial), BGE-M3 via OpenRouter settled, recall tier confirmed with `timeout_ms = 20000`. Lambo 0.10.0 has since closed issue 233 (native Elastic embedder); the stay-or-switch probe is T0.6b, which does not change this task's contract.

---

### T0.6b: Elastic native embedder probe
```yaml
requires:   [T0.6]
fixture-ok: no
size:       XS · mid
owns:       dev-diary/research/elastic-native.md, testdata/elastic-native/
status:     done
```
Lambo 0.10.0 closed issue 233: embeddings can now be created natively through the Elasticsearch Inference API. Probe whether the demo should switch from T0.6's settled BGE-M3-via-OpenRouter contract: replay T0.6's paraphrase set (`testdata/elastic/hybrid-documents.json`) through the native path and compare paraphrase cosine, kNN ranking effect, cost, latency, and the `lambo.toml` / feature-flag changes a switch needs, including the re-embed path. Recommend stay-or-switch with evidence. This task changes nothing: the settled contract in `research/elastic.md` stands unless a later task adopts the recommendation.

**Done when:** `research/elastic-native.md` records the comparison and the recommendation; recordings are in `testdata/elastic-native/`; any test indices deleted; spend recorded.

**Notes.** Verdict: stay (see `research/elastic-native.md`, recordings in
`testdata/elastic-native/`). Native `.jina-embeddings-v3` (1024 dims, same
width) matches the settled number head-to-head (pair cosine 0.677 vs 0.68)
and the kNN ranking shape (reason phrases plus all four same-line shirts
above both distractors either way), with no quality gain; absolute
thresholds do not transfer (native floor 0.16-0.44 vs settled 0.45-0.60)
and there is no calibration row for `kind = "elastic"`. Native inference
returns no usage/cost in-band (console check stays a user step). Warm
latency is equal (~1s); cold from this network the native path cannot
connect at all (hardcoded 5s connect timeout vs a ~6.2s DNS stall on the
deployment hostname — AAAA never answers — while openrouter.ai resolves
in 0.08s). `lambo.toml` shape and `embed-elastic` build proven; `re-embed`
documented but not exercisable live on this network. Every test index
deleted (empty listing verified). Spend: $1.6e-07 OpenRouter; ~30 short
native calls, unmetered in-band.

**Later direction (pointer, not a re-verdict).** The stay above was the
task's evidence on Lambo 0.10.0. The user has since directed the demo to
Elastic-native embeddings now that Lambo 0.10.1 ships the missing pieces
(configurable elastic timeouts and a jina-v3 calibration row); see the
"unblocked" entry in `dev-diary/README.md` and the P2 live twins that
proved the native path. The settled BGE-M3 contract in `research/elastic.md`
stands as the measured fallback until a later task adopts the native
contract and re-verifies calibration on the demo shopper.

---

### T0.7: Jev reason classifier
```yaml
requires:   [T0.1]
fixture-ok: no
size:       S · frontier
owns:       dev-diary/research/jev.md, testdata/jev/
status:     done
```
Jev is called through OpenRouter's decisions API (`https://openrouter.ai/api/alpha/decisions`, model `typesafe/jev-1.14`) with `OPENROUTER_API_KEY`. Don't use a `latest` alias: the classifier's confidence floor is measured on this version. Classify shopper answers ("too tight in the shoulders", "arrived broken", "changed my mind", an unclear one) into the reason categories, with probabilities. Set a confidence floor below which the agent asks again, and show on a small labelled set that the floor separates clear answers from unclear ones. Record the cost per call against the balance.

**Done when:** `research/jev.md` holds the questions, the labelled set, the results and the floor; recorded replies are in `testdata/jev/`.

---

### T0.8: Astropods
```yaml
requires:   [T0.1]
fixture-ok: no
size:       M · frontier
owns:       dev-diary/research/astropods.md, testdata/astropods/
status:     done
```

**Notes.** Research in `research/astropods.md`, recordings in
`testdata/astropods/`. Vault, pause/resume, eval activation, and the deploy
shape are proven live with `$0.00` spent. One step left for the orchestrator:
a 3-turn chat with the running probe to evidence traces, tool order, and
feedback; the file names both ways to close it.
On the free tier, confirm the features the concept relies on, which so far come only from the brief: pause for typed input and resume; feedback that arrives with trace context; OpenTelemetry traces showing tool calls in order; evaluators over those traces; a secrets vault; calling an external MCP server. Record which language the agent is written in and how it deploys.

If a feature is missing, record what the reference agent would do instead, and the orchestrator asks the user.

**Done when:** `research/astropods.md` has each feature's result with evidence; recordings are in `testdata/astropods/`.

---

### T0.9: Hosting
```yaml
requires:   [T0.1, T0.3]
fixture-ok: no
size:       S · mid
owns:       dev-diary/research/hosting.md
status:     done
```
Lambo needs a persistent disk, and the service needs a public HTTPS endpoint for PayPal webhooks. Work out what Render costs for both until winners are announced, against the $50 credit. **User decision:** everything on Render, or Lambo on the user's own server with Render serving the public demo and webhooks. Recommend one.

**Done when:** `research/hosting.md` has the costs and the user's decision.

**Notes.** PASS on the confirmed everything-on-Render decision: one paid
web service (`0.5c-512mb`, $7.00 / month) plus the minimum 1 GB
persistent disk ($0.25 / month) is $7.25 / month on a $0 Hobby workspace,
so the $50 credit buys about 6.9 months of runway, several times over the
retention rule in product.md. Free tier excluded with evidence (15-minute
spin-down, ephemeral filesystem, no disks). Endpoint is
`https://<service>.onrender.com` with managed TLS, custom domain
optional; PayPal delivery is plausible on the T0.4 webhook proofs plus an
always-on instance. Two read-only API listings confirmed the key works
(1 owner) and the slate is clean (0 services); nothing billable created.
Exact create-service / `render.yaml` calls and the P6 pre-flight list are
in `research/hosting.md`.

---

### T0.10: AG Grid inside Chaaya
```yaml
requires:   [T0.2]
fixture-ok: yes
size:       S · mid
owns:       dev-diary/research/ag-grid.md, spikes/ag-grid/
status:     done
```
Mount AG Grid through its plain JavaScript API inside a Svelte 5 component on Chaaya: custom cell renderers, theming from Chaaya's tokens, row editing, dark mode. Choose the licence tier and confirm it meets the sponsor prize's terms.

**Done when:** the spike renders the four belief statuses with an editable row; `research/ag-grid.md` records what worked and any Chaaya gap, with issue text if needed.

**Notes.** Community tier (MIT), no Enterprise import; meets the sponsor prize on the face of it, with Devpost wording left to submission. `spikes/ag-grid/` is self-contained (own package, tests, demo) and `web/` is untouched. Plain-JS `createGrid` in `$effect` after `ModuleRegistry.registerModules([AllCommunityModule])`; theme from live Chaaya tokens with `colorSchemeDark` for dark mode, applied via `setGridOption` so edits survive toggling; status pill and seeded-mark renderers; status select editor plus editable note with `onEdit` reporting. jsdom needs ResizeObserver/matchMedia/rAF stubs, `suppressColumnVirtualisation`, and a `firstDataRendered` wait; theme CSS is not observable there. No Chaaya gap found.

### T0.11: Lambo serving many shoppers ★
```yaml
requires:   [T0.3]
fixture-ok: no
size:       M · frontier
owns:       dev-diary/research/lambo-multi.md, testdata/lambo-multi/
status:     in-review
```
**The gate on per-shopper memory after the pivot.** product.md now keeps every shopper in their own Lambo session, keyed by a salted hash of their PayPal payer ID. T0.3 proved two sessions don't see each other, on one writer each. The deploy runs one writer for one session (`deploy/start.sh`, `LAMBO_SESSION`). Find out, on the Lambo release the deploy pins (`deploy/Dockerfile`), how the layer serves many shoppers:

1. **One process or many.** Can one `lambo serve` (MCP) write to and read from more than one session, for example by a session argument per call, or is it one writer per session? Read the source and confirm by running it. If it's one writer per session, measure what each costs (resident memory, startup time, file handles) and how many fit on the deploy's Render instance (`deploy/render.yaml`).
2. **Sessions on demand.** Can a session be created the first time a shopper appears, without a restart? Which characters and lengths a session name allows, and whether a 64-character hex hash fits.
3. **Many writers, one store.** Several sessions writing at once to the same store file, with the store kind the deploy uses: any lock errors, lost writes or receipt timeouts.
4. **Recall tier with many sessions.** With `recall-elastic` on, as the deploy runs it: how sessions share the Elastic recall index, and whether recall stays scoped to its session there too.
5. **Isolation at scale.** Repeat T0.3's check 6 across at least 20 seeded sessions with overlapping brands: no recall, inspect or stats in one session returns another's concept.
6. **Promotion per session.** One belief taken to Venerable in each of three sessions, seeded in interleaved order, so one session's writes and GC sweeps don't change another's promotion or time extent.
7. **The ledger per shopper.** Can `serve-web` or `/api/inspect` read a session chosen per request, or is it fixed at start?

Local first: the SQLite store, and the fixture embedder wherever the check allows it. Use the live Elastic deployment only for check 4, in a test index prefix, and delete every test index afterwards. Budget: under $0.50 of embedding spend. Run one case first and check its cost.

Any gap becomes Lambo issue text in the handoff log, for the user to file. Don't work around it here. If 1 and 2 both fail (one writer per session and no sessions on demand), this task is `blocked`, and the orchestrator asks the user with options.

**Done when:** `research/lambo-multi.md` holds each check's command and result, and a recommendation for how the deploy should serve many shoppers. Transcripts are in `testdata/lambo-multi/` with no key. Test indices are deleted (empty listing shown) and spend is recorded.

**Notes.** Lambo v0.10.1 already multiplexes: one `lambo serve --transport http` hosts one writer per session and many sessions, and a credential with `session_prefix` plus `create = true` attaches a new shopper's session on its first `/mcp/s/<session>` request, bounded by `max_attached` (default 16, pinned sessions included), `idle_detach_secs` (900) and `max_sessions` (32 MCP clients); idle detach and LRU eviction both leave the data in SQLite and re-attach on the next request. Measured: 21.1 MiB baseline plus about 0.4 MiB per attached small session and 29.7 KiB per concept, against 19.7 MiB per separate stdio writer; 512 MB fits the recommended 16 attached sessions even at 500 concepts each. The addressed-name rule is 1 to 128 bytes of `[A-Za-z0-9._:-]`, so a 64-hex payer hash is named `shopper-<hash>` (72 bytes) — a bare hash is outside the credential's prefix and not addressable. Eight concurrent writers on one SQLite file lost nothing and hit no lock errors; a second writer on a held session proxies into the holder rather than refusing. The Elastic recall tier keeps all sessions in one contract index with a `session_id` filter: three sessions shared one data index, recall and inspect stayed scoped, and every test index under `t011-multi-` was deleted (`[]` listing). Twenty sessions with overlapping brands leaked nothing across 380 pairs (780 calls); three interleaved sessions each reached Venerable at coverage 0.6667 and were unmoved by noise in another. `serve-web` takes the session per request from a start-time allowlist, so a shopper created after the reader started is a 404 until a restart — the one gap, written up as issue text in the research file. Spend: $0.00 (fixture embedder; no embedding endpoint called). Recommendation: one HTTP hub in `deploy/start.sh` (prefix credential, `max_attached` 16), sessions named `shopper-<hash>`, the ledger reading `/s/<session>/api/inspect` from a `serve-web` started with the demo shoppers.

### T0.12: The Elastic fit index ★
```yaml
requires:   [T0.6]
fixture-ok: no
size:       M · frontier
owns:       dev-diary/research/elastic-fit.md, testdata/elastic-fit/
status:     claimed:t012-elastic-fit
```
**The gate on pooled product fit and the merchant view.** product.md (the evidence model, "Lambo holds the shopper; Elastic holds the pool") settles the row: one document per purchase, keyed by a hash of the transaction ID, updated in place, written at ingest. Only settled rows count: returned with a classified reason, fit feedback, or kept past the return window. The shopper key is a salted hash per (shopper, product line). Seeded and live rows share one index, with the seeded mark on every row. Prove that design on the existing Serverless deployment (`research/elastic.md`):

1. **Mapping.** A test index with a field for each part of the row: product line (brand, category, fit handle), size, purchase state, reason category, fit direction, per-line shopper hash, transaction hash, seeded mark, event times (captured, refunded, settled). Record the mapping exactly as later tasks will create it. No prose fields.
2. **Rows move through states.** Write one purchase through every state in product.md's order (captured, refunded with no reason, reason classified; and separately feedback, and kept past the window; and a contested reason). Show each update lands in place, the document count stays one per purchase, and replaying a webhook-shaped write is harmless (`external` versioning or a sequence-number guard; record which, and why).
3. **Settled-only aggregations.** Over a seeded population of at least 500 purchases across at least 3 lines and several sizes, built to known answers: for one line and size, settled purchases (denominator), fit returns by direction, feedback by verdict, and distinct shoppers per direction. Every number matches the known answer, or the difference is explained. Show an unsettled purchase and a refund with no reason count toward nothing.
4. **Distinct counts.** `cardinality` is approximate. Record its `precision_threshold`, show it's exact at this population's size, and say at what size it stops being exact.
5. **Seeded split.** The same aggregations split by the seeded mark, as the ledger will show them.
6. **Speed inside recall.** Warm and cold latency of the line-and-size aggregation from this machine, since it runs on every `recall_for_purchase`. Record whether it needs the `timeout_ms` treatment T0.6 found for the cold TLS handshake.
7. **Beside Lambo's recall tier.** The fit index sits next to Lambo's recall index on the same deployment without touching it: different name, no shared alias or template, and Lambo recall still answers afterwards.
8. **Serverless limits.** Anything this deployment refuses or bills differently for these operations (index mode, update-by-id, aggregations), read from the responses and the docs.

No embeddings are needed; the fit index has no vector field. Every index this task creates uses a test prefix and is deleted afterwards. Budget: no model spend. Elastic usage is whatever these calls cost on the Serverless project; record the console reading if the user provides it.

If 2 or 3 fails, this task is `blocked`, and the orchestrator asks the user with options.

**Done when:** `research/elastic-fit.md` holds each check's request, response and result, the exact mapping, and the aggregation queries later tasks will reuse. Recordings are in `testdata/elastic-fit/` with no key and no deployment URL. Test indices are deleted (empty listing shown).

---

## Exit Criteria

- [x] Accounts and keys in place (T0.1).
- [x] Scaffold and gate passing (T0.2).
- [x] A seeded belief reaches Venerable, shows the contradiction warning, and ignores a non-fit dispute (T0.3).
- [x] PayPal refund, dispute and webhook behaviour recorded (T0.4).
- [x] Channel3, Elastic, Jev and Astropods each measured and recorded (T0.5 to T0.8).
- [x] Hosting decided (T0.9).
- [x] AG Grid confirmed inside Chaaya (T0.10).
- [ ] Lambo serves many shoppers, each isolated, with a recommended deploy shape (T0.11).
- [ ] The Elastic fit index updates per purchase in place and aggregates settled rows exactly (T0.12).
