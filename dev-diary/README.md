# Development Diary: Phase Handoffs

This document breaks down the work for **Memento Emptor**, the PayPal AI Hackathon entry.

Reference specification:
- [`../product.md`](../product.md): the product, the evidence model, the demo, the comparison, the stack, the hackathon rules, the cautions.
- `../docs/architecture.md` (written by T1.1): the service, the data flow, the configuration, the dependencies.
- `research/` (written in P0): what was measured on the real stack before any code. **Read the file for your area first.**
- [`ORCHESTRATOR.md`](ORCHESTRATOR.md): how the orchestrator dispatches, validates, lands and closes, with the brief template.
- `../AGENTS.md` (written by T0.2): the rules every agent follows.

Where a phase document and product.md conflict, product.md wins. Record the discrepancy in the handoff log.

Each phase document gives an engineer starting cold the complete context for its tasks.

**This diary is public.** The repository goes public for submission. Nothing private goes in here: no key, no account detail, no IP address, no personal path beyond `~`, no private repository's name.

**Current status:** T0.1 and T0.2 are done. T0.3 to T0.8 and T0.10 are ready. T0.9 waits on T0.3.

---

## Swarm Coordination Rule

> **Phase-level dependencies are advisory. Task-level dependencies are binding.**

Do not wait for a whole phase to finish. If a task requires only `T1.1`, start it the moment `T1.1` closes.

**Fixtures make most work offline.** Recorded PayPal events, Jev replies, Lambo transcripts, Elastic responses and Channel3 products go into `testdata/`. Tasks marked `fixture-ok: yes` run against those files with no key and no spend. Only `b` tasks and tasks marked `fixture-ok: no` call a live service.

---

## Phase Graph

| Phase | Document | Requires | Runs parallel with | Blocks |
|---|---|---|---|---|
| **P0** Ground | [PHASE-0-ground.md](PHASE-0-ground.md) | none | none | everything |
| **P1** Contracts | [PHASE-1-contracts.md](PHASE-1-contracts.md) | T0.3 to T0.8 | none | P2 to P5 |
| **P2** The layer | [PHASE-2-layer.md](PHASE-2-layer.md) | P1 | P3, P4 | T3.6, T5.5 |
| **P3** Reference agent | [PHASE-3-agent.md](PHASE-3-agent.md) | P1 | P2, P4 | T5.1, T6.4 |
| **P4** Evidence ledger | [PHASE-4-ledger.md](PHASE-4-ledger.md) | T1.2, T1.4 | P2, P3 | T6.2, T6.4 |
| **P5** Comparison | [PHASE-5-comparison.md](PHASE-5-comparison.md) | T1.3, T3.1 for the harness; T2.6 and T3.6 for arm 3 | P4 | T6.2 |
| **P6** Ship | [PHASE-6-ship.md](PHASE-6-ship.md) | P2 to P5 | none | submission |

```text
  T0.1 accounts ─▶ T0.3 to T0.8 live checks ─┐
  T0.2 scaffold ──────────────────────────────┼─▶ T1.1 ─▶ T1.2 ─▶ T1.4 ─┬─▶ P2 layer ───┐
                                              └─▶ T1.3 seeded shoppers  ├─▶ P3 agent ───┼─▶ T3.6 full demo
                                                                        └─▶ P4 ledger ──┤
  T5.1 harness ─▶ T5.2 grader ─▶ T5.3 baseline pilot ─▶ T5.4 freeze ─▶ T5.5 arm 3 ─▶ T5.6 ┤
                                                                                         └─▶ P6 ship
```

T0.3 is the gate on the whole product: if a seeded belief can't reach Venerable and show the contradiction warning, the demo loses its centre. Nothing builds on promotion until it passes.

---

## Parallel Tracks

| Track | Phase | Network required? | First task | Notes |
|---|---|---|---|---|
| **A: The layer** | P2 | `b` tasks only | T2.1 | Evidence in, Earned beliefs out. Everything else demonstrates this |
| **B: Reference agent** | P3 | Yes | T3.1 | Runs on Astropods; replays the layer's contract until P2 lands |
| **C: Ledger** | P4 | No | T4.1 | Builds against fixtures |
| **D: Comparison** | P5 | Yes | T5.1 | The baseline pilot runs before the layer is finished |

If capacity is short, run A first and then B: the demo needs both, and the ledger can follow on fixtures.

---

## Critical Path

```
Promotion proved (T0.3) + PayPal events recorded (T0.4)
  -> domain model (T1.1) -> interface (T1.2) -> fixtures (T1.4)
  -> ingest (T2.1) -> reasons (T2.2) -> Lambo writer (T2.4) -> interface calls (T2.5) -> layer end to end (T2.6)
  -> agent on Astropods (T3.1) -> orders (T3.3) -> asking loop (T3.4) -> full demo (T3.6)
  -> ledger (T4.2, T4.3) -> deploy (T6.1) -> video (T6.4) -> public (T6.5) -> submit (T6.6)
```

The demo rests on one story: three refunds the shopper explains become a belief the agent has earned, a one-line correction changes the very next recommendation without erasing that belief, and a dispute for a parcel that never arrived leaves the sizing untouched. Protect everything that moment depends on before anything else.

---

## Task Pattern

```markdown
### T2.4: The Lambo writer ★
requires:   T1.1, T1.4
fixture-ok: yes
size:       M · frontier
owns:       internal/memory/
status:     not-started
```

- **requires:** Binding task ids.
- **fixture-ok:** Whether the task can be built and tested against `testdata/` alone.
- **size:** Scope from XS to XL, then the class: the kind of work it is. The user assigns the agent and model.
- **owns:** The paths this task alone edits. T0.2 may adjust them to Keel's layout; if so, it updates every phase document in the same commit.
- **status:** `not-started`, `claimed:<agent>`, `in-review`, `done`, `blocked`, or `superseded` (the user replaced the task's plan; the task says by what).

| Class | Send it for |
|---|---|
| **frontier** | A decision others build on. Prompt and judgement work verified by measurement. A spec that needs interpreting. |
| **mid** | Well specified implementation, or a spec precise enough to follow literally. |
| **light** | Mechanical, low ambiguity work that its own test verifies. |
| **user** | Only the user can do it: an account, a key, a recording, a submission, a decision. |

A star (★) marks a task on the critical demo path.

A `b` task (T2.1b) is the live sibling of its parent. It owns only its live test, under `tests/live/`. A live test never runs in CI and never gates a build. A live call is evidence: commit the test, and paste its transcript into the handoff log.

---

## Developer Guidelines

1. **Claim before starting.** The orchestrator sets `status: claimed:<agent>` in the phase document on `main`.
2. **Stay inside `owns`.** A change outside it is a new task or a handoff note.
3. **Contracts freeze at the end of P1.** A change to a shared type or interface updates its fixtures in `testdata/` in the same commit, and is the orchestrator's call.
4. **A payment outcome is not a cause.** Evidence counts toward a belief only through a reason that matches it. No code path may turn a refund or dispute into a belief without one.
5. **Statements never promote.** A general statement goes through `derive`. Only an outcome with a classified reason, or feedback tied to a transaction, is a structural write.
6. **The shopper's correction wins at once, and the belief stays.** The layer returns the correction with precedence, and declares it with `against`. It never deletes the belief.
7. **The layer never moves money.** Orders, approval and double-buy protection live in the reference agent.
8. **No sponsor names in the layer's domain code.** PayPal is the evidence source by design; Lambo, Elastic and Jev sit behind interfaces. Channel3 and Astropods appear only under `agent/`.
9. **Seeded means marked.** Every seeded row carries the mark from T1.3, and the ledger shows it.
10. **New work only.** Write it here. Never copy code from Lambo or any earlier project. Keel, Chaaya and Lambo are dependencies.
11. **Gaps in Lambo, Keel or Chaaya become issues.** Write the issue text in the handoff log. Don't work around it here. The user files it.
12. **Document handoffs.** A task without a handoff entry is unfinished. Record what exists, what surprised you, and what the next agent should not have to work out again.

---

## Scope

Every task here ships. A task that turns out to be blocked becomes a finding, not a cut. Record it, name every task that depends on it, and mark it `blocked`. If a task is too large, split it into `T3.1a` beside `T3.1` and record the split.

---

## Status Board

| Phase | Tasks complete | Status |
|---|---|---|
| P0 | 10 / 10 | Done |
| P1 | 1 / 5 | In progress |
| P2 | 0 / 10 | Not started |
| P3 | 0 / 6 | Not started |
| P4 | 0 / 4 | Not started |
| P5 | 0 / 6 | Not started |
| P6 | 0 / 7 | Not started |

---

## Handoff Log

Newest first. What exists, what surprised, and what the next agent must not have to work out again.

### T1.1: The domain model and architecture (done) ★

**What exists.** `internal/domain/` (evidence, 7 reason categories, 3-code dispute table with `ErrUnknownDisputeCode`, belief + status/graph round trip, 5-signal write rule with `record_action`/`derive`/`observe`, swappable `EmbedderContract` with no model constants, zero vendor names) plus `docs/architecture.md` tracing webhook to belief with research citations. Table tests cover every signal, category, and dispute code.

**What surprised.** Two judgment calls, both recorded for successors, neither blocking: `UNAUTHORIZED_TRANSACTION` maps to count-toward-nothing (fraud-as-seller-signal would be a deliberate product decision, not a quiet edit — contracts freeze now); `kept-past-window` maps to a third op `observe`, weaker than `derive`, to stay honest to "observation only". Design note for T1.2: contradicted is an overlay (Venerable stays Venerable with warnings), but the model maps it flat to Out-of-true — the recall representation must carry both the underlying standing and the warning.

**The next agent should know.** T1.2's contract and T1.4's fakes build on `WriteFor`, `CategoryForDispute`, and the status round trip; T1.3's generator fills `Evidence.Seeded` + `SeedMark`. All covered by tests, no hidden behavior.

### T0.9: Hosting (done)

**What exists.** `dev-diary/research/hosting.md`: everything on Render at $7.25/mo ($7.00 `0.5c-512mb` + $0.25 1 GB disk, Hobby workspace $0) against the $50 credit — ~6.9 months runway, several times the retention rule. Free tier disqualified three ways (spin-down, ephemeral FS, no disks). Single web service with Lambo beside it on the disk mount; Lambo binary and `lambo.toml` ship in the image, only the data dir on the disk. Exact `POST /v1/services` and `render.yaml` provisioning calls recorded (nothing created, $0 spend), plus the PayPal webhook registration pattern and a P6 pre-flight checklist (credit balance, payment method on file, region, `store-sqlite` binary, no deploys in the judged window).

**What surprised.** Disk-backed services lose zero-downtime deploys (seconds dark per redeploy) — PayPal retries plus event-id idempotency cover it, but don't deploy during judging. Suspension (not deletion) is the credit-exhaustion path, with disk + 7-day snapshots retaining the store.

**The next agent should know.** P6 provisions this. Two pre-flight items are user-side: confirm remaining credit on the dashboard billing page, and whether a payment method must be on file for paid compute.

### Lambo issues 236-238 (filed by the orchestrator, user sends agents to fix)

User direction: the demo uses Elastic-native embeddings, not BGE-M3 via OpenRouter — reversing T0.6b's stay verdict. The switch is blocked on Lambo until these land:

- [#236](https://github.com/nrynss/lambo/issues/236) `[embed] connect timeout is hardcoded at 5s; elastic derive fails under slow DNS`. Load-bearing: `DEFAULT_CONNECT_TIMEOUT` (`src/embed/bge_m3.rs`) vs ~6.2s DNS AAAA stall; fix is `connect_timeout_ms` in `[embedder]` mirroring `[recall] timeout_ms`.
- [#237](https://github.com/nrynss/lambo/issues/237) `[embed] no calibration row for kind = "elastic"`. Measured constants (merge threshold, 0.50 layer floor, supersession trigger) do not transfer across spaces (native floor 0.16-0.44 vs settled 0.45-0.60); needs a measured row or a documented operator path.
- [#238](https://github.com/nrynss/lambo/issues/238) `[recall] tier error drops the timeout source`. Diagnosis aid; workaround (`timeout_ms = 20000`) already recorded in T0.6.

Full issue text lives on GitHub. When the fixes release, the follow-up here is: adopt the native contract (T0.6b documents the `lambo.toml` shape and `re-embed` path), re-measure calibration on jina-v3, and confirm Elastic console billing.

### T0.6b: Elastic native embedder probe (done)

**What exists.** `dev-diary/research/elastic-native.md` plus 4 recordings in `testdata/elastic-native/`. Head-to-head of Lambo 0.10.0 native embeddings (`.jina-embeddings-v3`, 1024 dims) vs the settled BGE-M3-via-OpenRouter contract on T0.6's paraphrase set: pair cosine 0.677 vs 0.68 (wash), same kNN ranking shape, no quality gain. Verdict: stay. The settled contract in `research/elastic.md` is untouched.

**What surprised.** The native path cannot write from this network at all: Lambo's elastic embedder has a hardcoded 5 s connect timeout vs a ~6.2 s DNS stall (AAAA for the deployment hostname never answers; openrouter.ai resolves in 0.08 s). This also re-explains T0.6's "cold rustls handshake" — it was the same DNS stall, not TLS. Native inference returns no usage/cost in-band (console check still a user step), has no calibration row (every measured constant would need re-measuring), and would couple both vector dependencies to one service.

**The next agent should know.** Revisit only with healthy DNS (or a configurable Lambo connect timeout), trivial console billing, and re-measured calibration on jina-v3. The `lambo.toml` switch shape and `re-embed` path are documented in the research file.

### T0.3: Lambo promotion on a seeded session (done) ★

**What exists.** `dev-diary/research/lambo.md` plus 8 recordings in `testdata/lambo/` (three MCP transcripts, four `/api/inspect` responses, the exact `lambo.toml`). Proved on Lambo v0.10.0, local-only, $0: three fit returns citing the belief take it to Venerable (`distinct_interactions` 3.0≥3.0, coverage 0.667≥0.3, one hop per eval cycle); GC sweeps run from file cadence, Swarm is the policy; the `against` correction renders the contradiction warning on recall, inspect, and `/api/inspect` `warnings[]` while the belief stays Venerable; a non-fit dispute leaves sizing untouched (seller belief refused Candidate); ledger mapping is one shape per state; sessions isolate shoppers on a shared store file. Demotion targets Canonical only — a Venerable belief is held, never demoted.

**What surprised.** Every 0.9.1 threshold in product.md holds identically on 0.10.0; the only delta is the predicted one (`/api/inspect` carries the warning). Seeded edges date by `event_time`, so promotion runs in ~10 s while the 60 s floor binds only live edges — the honest-seeding mechanism, confirmed in code. Operational: the 0.10.0 default build omits the SQLite store (needs `--features store-sqlite`); `match_strategy` and the coverage cap are library settings, not file knobs. The worker's cleanup kill caught this machine's shared Lambo dogfood writer; its supervisor restarted it healthy and the task never opened its database — later runs kill by exact PID only.

**The next agent should know.** Replay needs only the v0.10.0 binary and the committed toml; concept ids are content-addressed, so `depends_on`/`against` by exact content resolve identically. P2 builds on these transcripts and the inspect shapes.

### T0.4: PayPal sandbox events (done)

**What exists.** `dev-diary/research/paypal.md` plus 45 scrubbed recordings in `testdata/paypal/`. Full loop evidenced twice against a replacement US-home sandbox app: wallet order create (idempotent on `PayPal-Request-Id`), scripted shopper approval via Playwright, capture `COMPLETED`, order-side line items, refund `COMPLETED` with full record, and a live dispute `MERCHANDISE_OR_SERVICE_NOT_RECEIVED`, status OPEN, filed by the shopper in the resolution center. Two reason-code answers: refund records carry NO reason field (two records plus webhook confirm), disputes carry the reason top-level and per item. Webhooks: full CRUD, delivery, `verify-webhook-signature` SUCCESS, resend — including auto-delivery of the dispute event in ~1 minute. Toolkit v1.11.0 mapped: create-dispute, webhook CRUD/resend/verify, payouts, vault, contesting are REST-only.

**What surprised.** The original merchant account could not hold USD (approval refused; receiving-preference flip didn't stick) and INR order creation 422s — fixed by a fresh US-home business account + app, keyring swapped. Seller-side dispute creation via API is gated beyond completed-capture in this sandbox (`TRANSACTION_NOT_ELIGIBLE` even on completed captures); the working path is shopper filing in the resolution center, click-path recorded. Capture records carry no line items — the ledger reads order + capture together, joined on `custom_id`, never the reporting API (`NOT_AUTHORIZED`). Card checkout needs card processing enabled on the merchant. `simulate-events` 404s; catalog creation emits `CATALOG.PRODUCT.CREATED`.

**The next agent should know.** Ingest (T2.1) keys idempotency on PayPal's event id, verifies every delivery, and treats refunds as outcomes-without-reason pending the shopper's answer. Cycle 3's capture stays open/disputed for demo reuse; cycles 1–2 are refunded and settled. Per the user's decision, workers may use the sandbox site logins (shopper approval, resolution-center filing) with values never printed, written, or committed.

### T0.8: Astropods (done)

**What exists.** `dev-diary/research/astropods.md` plus 11 recordings in `testdata/astropods/`. Proven live on the free tier at $0.00 spend: TS/Bun+Mastra and Python/LangChain scaffolds, `push`/`deploy --adapter web` with web chat URL and collector sidecar, gateway model access with no personal key, secrets vault round-trip (create/list/get-masked/delete, vault left empty), deployment pause/resume, evaluator schema discovery (`EVALUATION.yaml`) plus a custom `followed_approval` set activated server-side, OTel collector wiring with empty trace/usage states, and the external-MCP pattern with a plain-agent-tools fallback.

**What surprised.** The live 3-turn chat first stayed open (deployed web chat needs user-scoped SSO) and the chat URL 404'd twice: first a stale probe, then a missing `agent.interfaces.messaging` block in the deployed spec (anonymous checks 302 to SSO and look healthy either way). Both fixed by redeploy; then the turns were driven without a browser via the platform messaging proxy using the CLI's own token. Results: turn 1 shows 17 ordered observations with the `lookupReturn` tool span between generations; turn 2 asks for "yes" with no tool span; turn 3 resumes and honestly declines (no record tool) rather than hallucinating. First `eval run`: `followed_approval: true` at 0.99. Total spend $0.02 of $10.00. Free compute is small ("Slowed by CPU limit" — keep demo turns concise). Two CLI quirks recorded: nested help needs `ast help <cmd> <sub>` form; enum evaluator options use field `options`, not `values`.

**The next agent should know.** Approval/correction forms are agent-built chat turns, not a platform primitive; feedback lands per trace via `eval review`, traces of interest via `dataset add`. Trace ids and the eval transcript are in `testdata/astropods/12-traces.txt` and `13-eval-run-review.txt`; replay them offline. The probe deployment can be paused or removed once T3.1 stands up its own agent.

### T0.6: Elastic evidence store (done)

**What exists.** `dev-diary/research/elastic.md` plus 5 scrubbed recordings in `testdata/elastic/` (vectors omitted, rankings kept). Hybrid search verified on a throwaway index (created, queried, deleted): BM25 finds the returned shirts for a new-SKU text, kNN over BGE-M3 puts `too tight in the shoulders` at 0.84 for a fresh `snug across the chest` query, and the combined shape needs `retriever:{rrf:...}` on this deployment. Embedder settled: BGE-M3 through OpenRouter (`baai/bge-m3`, native 1024 dims), proved end to end through Lambo derive+recall. Recall tier confirmed on a locally built 0.9.1 binary (index creation, external-version 409s, kNN, backfill, `_delete_by_query` erase, store fallback with tier down). Hosting: keep the existing Serverless project, start nothing.

**What surprised.** The deployment is Elastic Serverless (9.7.0), not a trial — the brief's trial-vs-self-host framing did not apply. Installed Lambo 0.8.0 lacks `recall-elastic`, and cold rustls handshakes (~6.8 s) trip the default 5 s timeout: every `lambo.toml` needs `timeout_ms = 20000` in `[recall]` (proposed Lambo issue text in the worker's report; the user files it). Serverless excludes vectors from `_source`, so Lambo's exact re-scoring keeps engine HNSW scores — an accepted, slightly widened difference. Spend bounded under $0.00001; test indices deleted.

**The next agent should know.** The embedder contract is fixed (`kind=bge_m3`, `url=https://openrouter.ai/api`, `model=baai/bge-m3`, `dim=1024`, `api_key_env=OPENROUTER_API_KEY`); every later session must match or re-embed (`lambo re-embed`). T0.9 no longer needs an Elasticsearch trial decision — only Lambo persistence plus public HTTPS. The user should confirm the Serverless project's billing in the Elastic console.

### T0.10: AG Grid inside Chaaya (done)

**What exists.** `dev-diary/research/ag-grid.md` plus a self-contained spike in `spikes/ag-grid/` (own `package.json`, tests, demo; `web/` untouched). The spike renders the four ledger statuses with pill renderers and a seeded mark, edits status via select editor and notes as text, themes from live Chaaya tokens with dark mode, and survives theme toggles without losing edits. Suite: 15/15 pass; `svelte-check` clean; builds. Licence: Community 36.3.0, MIT, no Enterprise import — P4 inherits that decision.

**What surprised.** v36 requires `ModuleRegistry.registerModules([AllCommunityModule])` or row-API calls fail with error #200. Theme param names moved (`dataBackgroundColor`, `headerBackgroundColor`); jsdom reports zero width so the spike sets `suppressColumnVirtualisation` (justified for a 5-column ledger); theme CSS is unobservable in jsdom, so theming proof is structural plus visual via `npm run dev`. No Chaaya gap found — `reactive(theme)` + `$effect` covers everything, `attach()` correctly unused.

**The next agent should know.** View with `cd spikes/ag-grid && npm ci && npm run dev`. Bundle is 1.2 MB minified (340 KB gzip), all AG Grid: the real ledger page must lazy-load the grid. Confirming exact Devpost sponsor-prize wording is submission work (T6.6).

### T0.5: Channel3 product data (done)

**What exists.** `dev-diary/research/channel3.md` plus 10 scrubbed recordings in `testdata/channel3/` (2 searches, 8 product details). Answer: no size chart or measurement field exists, but `structured_attributes` carries a per-category fit handle (`top-fit` on shirts, `footwear-cut` on shoes) and descriptions state fit in words. The match can use brand id + category + equal fit handle, backed by title/description text. Demo line chosen: Sunspel Button Down Oxford Shirt (SKUs `tbnrl46`, `VyAkjCA`, `PSW8GeK`, all brand `SMwk`, `top-fit: Regular`, InStock at USD 195). Bonobos Everyday Oxford is the negative control (same line name, `Slim` vs `Slim`+`Tailored` — line name alone proves nothing).

**What surprised.** `variants` is `null` on all apparel fetched: each colorway is its own product and sizes live only at the retailer, so Channel3 cannot supply the demo's M/L sizes. Verified from the fixtures: size selection happens on the retailer offer URL, and the `option_Size=` selection shape was confirmed on a shoe SKU with real variants. Spend: 2 searches (2 credits) + 9 free calls, well inside the free allowance.

**The next agent should know.** Replay `testdata/channel3/` offline; one live call suffices to re-verify. Record the retailer's size availability out-of-band if the demo needs it — it is not in Channel3's data.

### T0.7: Jev reason classifier (done)

**What exists.** `dev-diary/research/jev.md` plus 11 recorded Decisions replies in `testdata/jev/`, one per labelled phrase. Six exclusive categories (`fit_too_small`, `fit_too_large`, `damaged_or_defective`, `not_arrived`, `changed_mind`, `unclear`), question and confidences recorded. Floor rule for `record_reason`: ask again when `choice == "unclear"` or `confidence < 0.98`. All 8 clear phrases returned 1.00 with the right category; the 3 vague/ambiguous ones came back at 0.94, 0.96, 0.68. Cost: $0.000235284 total (~$0.021 per 1,000 at this shape), negligible against the $50 key limit.

**What surprised.** Jev's probabilities are extremely sharp — nearly all mass on one category — so the `unclear` category does more separation work than the numeric floor. No `refusal` answers observed, but the schema allows them: T2.2 treats a refusal as ask-again. Model resolved to build `typesafe/jev-1.14-20261014`; a version change re-opens the floor measurement.

**The next agent should know.** The categories map directly onto developer rule 2: fit direction feeds sizing, `damaged_or_defective`/`not_arrived` count only against the seller, `changed_mind` counts toward nothing. Re-measure the floor on real shopper answers before it gates any belief.

### T0.2: Scaffold (done)

**What exists.** An Apache-2.0 licence, a Go module on Keel `v0.9.0` with `internal/version`, and a Svelte 5 SvelteKit app under `web/` on Chaaya `0.6.0`. `tools/check.sh` runs gofmt, `go vet`, `go test -race`, a two-way lockfile check, svelte-check, eslint, vitest and the web build. CI runs the same gate. `tools/env.sh` exports the seven code-read variables from the keyring, leaves a preset variable alone, and names a missing key without printing a value. `AGENTS.md` carries the developer rules, the gate, and every key's `secret-tool lookup` command. `dev-diary/research/README.md` sets the evidence and recording rules.

**What surprised.** The licence is Apache-2.0, by the user's decision, not MIT. The task's `owns` line named only `web/package.json`, so it was widened to `web/` to cover the app the task writes. An independent review found the web test step vacuous and the Go filter too broad. The web app now carries a real test, the gate checks the lockfile in both directions, and the Go filter drops only `web/node_modules`. `go mod tidy` keeps the Keel requirement because `internal/version/pin.go` carries a `tools`-tagged blank import.

**The next agent should know.** The web app builds static with a `200.html` fallback, and T1.1 can change the adapter. CI runs green on `main`: the first push ran the gate and passed. The workflow's `checkout`, `setup-go` and `setup-node` actions still target Node 20 and GitHub forces them onto Node 24, which warns but does not fail.

### T0.1: Accounts and keys (done, by the user)

**What exists.** In the system keyring, as `service <name> account memento-emptor`:

| Service | `kind` values | Variable |
|---|---|---|
| `paypal-sandbox` | `client-id`, `client-secret`, `merchant-email`, `merchant-password`, `shopper-email`, `shopper-password` | `PAYPAL_CLIENT_ID`, `PAYPAL_CLIENT_SECRET` (the logins are for the sandbox site only) |
| `channel3` | none | `CHANNEL3_API_KEY` |
| `openrouter` | none | `OPENROUTER_API_KEY` |
| `elastic` | `api-key`, `url` | `LAMBO_ES_API_KEY`, `ELASTIC_URL` |
| `render` | none | `RENDER_API_KEY` |

Astropods has no key: the CLI is logged in with `ast login` and holds its own tokens. Render's credits are applied. Elastic is a hosted deployment, so T0.6 records its limits and how long it stays up.

**What surprised.** Jev is reached through OpenRouter, pinned to `typesafe/jev-1.14`; the Astropods gateway lists only 1.13. Astropods' free tier is one-time signup credits, and every agent stops when they run out with no card on file. The `ast` CLI is in `~/.ast/bin`, which isn't on the tool shell's PATH.

**The next agent should know.** No key has been tried against its service yet, so T0.3 to T0.9 are the first real use. Lambo's embedder isn't chosen: Elastic can't create its embeddings until [Lambo issue 233](https://github.com/nrynss/lambo/issues/233) ships, and BGE-M3 through OpenRouter would use the key already stored. The agent's model is T3.1's decision, and Astropods' gateway may mean it needs no key.
