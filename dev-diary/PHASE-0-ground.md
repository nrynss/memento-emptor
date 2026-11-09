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
status:     not-started
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
status:     not-started
```
Lambo needs a persistent disk, and the service needs a public HTTPS endpoint for PayPal webhooks. Work out what Render costs for both until winners are announced, against the $50 credit. **User decision:** everything on Render, or Lambo on the user's own server with Render serving the public demo and webhooks. Recommend one.

**Done when:** `research/hosting.md` has the costs and the user's decision.

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

---

## Exit Criteria

- [x] Accounts and keys in place (T0.1).
- [x] Scaffold and gate passing (T0.2).
- [ ] A seeded belief reaches Venerable, shows the contradiction warning, and ignores a non-fit dispute (T0.3).
- [ ] PayPal refund, dispute and webhook behaviour recorded (T0.4).
- [ ] Channel3, Elastic, Jev and Astropods each measured and recorded (T0.5 to T0.8).
- [ ] Hosting decided (T0.9).
- [ ] AG Grid confirmed inside Chaaya (T0.10).
