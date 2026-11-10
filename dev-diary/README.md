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
| P1 | 5 / 5 | Done |
| P2 | 10 / 10 | Done |
| P3 | 5 / 6 | In progress |
| P4 | 4 / 4 | Done |
| P5 | 2 / 6 | In progress |
| P6 | 0 / 7 | Not started |

---

## Handoff Log

Newest first. What exists, what surprised, and what the next agent must not have to work out again.

### R2 review: P2 layer (FINDINGS — 1 non-blocking, landed)

Re-verified all six exit criteria against code + committed live artifacts; all fail-closed properties hold against constructed counterexamples (override deleted, changed-mind, uncovered fake, approve-first both paths). One new offline test landed (`TestChangedMindClosesQuestionWithoutBelief` — accepted-but-counts-nowhere had writer-level but no service-level coverage). One real honesty gap found: the frozen offline story's third return is mixed-direction (`fit_too_large`) while the live T2.6 run was fixed to all-small — the offline suite and the live run prove different things. Remedy + product question with the user.

### R4 review: P4 ledger (APPROVE with fix + 4 findings, landed)

All four exit criteria re-verified with the reviewer's own runs (96/96 web tests). Real fix landed: the seller why-view showed fits-well feedback as "Counts toward nothing" — now names the kind it counts toward when `counts_toward` is non-null, with regression test. Findings: (1) rendered-comment defect wider than recorded — 4 files incl. both live pages (needs sweep task, frozen T4.2 file untouched); (2) contest UI built but wired to no route — judges can't reach it (needs wiring task); (3) ledger→belief deep links still missing (accurately described, needs ledger-route edit); (4) raw reason codes on why cards (`fit_too_small` verbatim — wording follow-up).

### R1 review: P1 contracts (APPROVE, landed)

Re-verified all five exit criteria against code, not handoffs. Freeze holds: no post-P1 modification of any frozen path, no redefined enums, `testdata/api/` ↔ contract equality proven by gate test. Seam honesty confirmed (`var _` assertions, identical method sets; `memory.Graph` minus `Recall` is documented and load-free). One doc fix landed (embedder note no longer claims 0.10.1-closed issues still block the native switch). No structural findings; freeze stands.

### T4.3: The why view (done) ★

**What exists.** `web/src/lib/why/` + belief route: per-belief evidence with outcome/item/reason, correction beside the belief in plain words ("you said M; three returns said L"), non-counted items each with their exclusion reason, seeded marks with tooltip, pending question shown open, trace anchors marked as fixtures, back-to-ledger link on every state. Step walk 2→3→4→5 verified against fixtures.

**What surprised.** Review approved with one CSS-class rename (graph vocabulary out of user-facing names). Mount-testing gap confirmed real (Vitest resolves svelte to server build; needs a test-alias change outside any task's owns — recorded, not worked around). Confirmed follow-ups: ledger→belief deep links need a ledger-route edit (new task); ledger-wide evidence search is server-side work. Simplicity bar held: no standing/Venerable/Canonical in user strings, `untrack()`-guarded refresh (reactive subscribes on any read — loops otherwise).

### T4.4: Contesting a belief (done)

**What exists.** `web/src/lib/contest/` (CorrectionButton column, ContestEditor form, correction-path-only transport, state helpers, source-scan + mount tests — 19/19 with reviewer's additions). Edit moves Earned→Out of true, standing kept, row visible; three layers of proof that only `record_correction` fires.

**What surprised.** Review caught a real rendered bug: file-header `//` comments in Svelte compile to visible text nodes (fixed; same pattern pre-exists frozen in T4.2's `LedgerGrid.svelte` — follow-up task to sweep all `.svelte` files, not a silent fix). T4.4's own mount tests needed a client-entry import path (Vitest resolves `svelte` to server build here).

### T5.2: The grader (done)

**What exists.** `bench/grader/` scores hand-built runs on all five measures as counts: equality, baseline-anchored wrong-cause, next-turn correction, exactly-one-row citations (fail-closed on ambiguity), harness-counter tokens with recorded basis. Mechanical-vs-opinion line pinned; `Score.Opinions` empty offline.

**What surprised.** Review added split-outcome discrimination tests (right-size-true + wrong-cause-true coexist; honouring-true + bogus-cite-false coexist) — measures stay separate counts. One forward finding for the freeze: live replies are free text with no pinned reply→Size/Cites/WantSize extraction rule yet — without it T5.5 grades the extractor, not the arm. Freeze must define the extraction rule (or human-extraction procedure) alongside the hashes.

### T5.1: The harness (done)

**What exists.** `bench/harness/` runs every seed shopper through 3 arms offline (scripted replies, no model/keys/network): complete records with prompt/reply/token counts/spend math, per-arm tool calls (none / `read_history` / `recall_for_purchase`), prompts differing ONLY in the memory block (byte-proven), caps enforced pre- and post-call (4000 tokens, $0.05 — token cap binds first), hidden answers unreachable (static + prompt-content + indirect-leak assessment).

**What surprised.** Review passed with 4 freeze-time follow-ups, recorded for T5.4: F1 — arm-3 tally counts fits-well as `counts toward ""` (real layer counts verdict→kind, T1.2 handoff); F2 — classified changed-mind tallied as "pending" though only below-floor unclear is truly pending; F3 — arm-2 inputs omit amounts + all transaction IDs (freeze must define the arm-2 input contract); F4 — arm-3 stub is not the wire shape (T5.5 must run real `recall_for_purchase`). Also: `TestSeedHistoriesCarryNoCorrections` pins the no-corrections premise (revisit on the first live-correction run); word-count tokens are ~5x under live BPE — re-measure with the live tokenizer on the pilot (T5.3), don't trust the proxy.

### T2.1: PayPal ingest (done)

**What exists.** `internal/ingest/` over the fakes' `PaymentSource` as-is: signature-checked webhook handling, event-id idempotency (redelivery returns `Duplicate`), order-GET line items joined on `custom_id`, disputes via the frozen table (unknown codes store nothing), reasonless refunds filed pending. Tests cover redelivery, bad signature, refund-before-capture (via stub — fixtures stage no same-transaction pair), and all three mapped codes.

**What surprised.** Dispute rows take the outcome from the verified delivery (the record carries none); items prefer the order read. Design (each delivery self-sufficient) is what makes order irrelevant — worth keeping in T2.1b.

### T2.2: Reason classification (done)

**What exists.** `internal/reasons/` Jev client behind the classifier seam: 11/11 labelled phrases agree with the fake, floor pinned at 0.98 (no drift), never-explained and below-floor both settle to nil-reason pending. Elastic neighbour context deliberately omitted — `TestNeighbourContextOmitted` re-opens the decision if a future recording needs it.

### T2.3: The evidence store (done)

**What exists.** `internal/evidence/` implementing `fakes.EvidenceStore` as-is: documents (message/reason/product + brand/category/fit-handle line), graph stubs (`evidence[<id>] <key>`, prose never enters), brand+category+fit matching, deterministic RRF fusion, contract-gated store with `ReEmbed`. Tests replay T0.6's legs; stub test proves doc-ID-only writes. Embedder stays behind `domain.EmbedderContract` — native switch changes only the kind.

**What surprised.** The store assigns `doc-0001…` ids while recordings use `p1…r2` — T2.5 must persist assigned ids into stubs, never assume recording ids.

### T2.1b: PayPal ingest, live (done)

**What exists.** `tests/live/ingest/` driver (reads only — zero new sandbox records): token, webhook config, 20-entry ledger, two deliveries re-fetched and re-verified (`SUCCESS`, SHA256withRSA), refund→capture→order and dispute→capture→order chains. Refund row nil-reason → pending; dispute `not_arrived` 1.0 → seller; redeliveries duplicate-safe. A T0.4-recorded body against the current webhook id correctly returns FAILURE — the crypto is really checked.

**What surprised.** Verify API field is `webhook_event`, not `request_body` (the latter 400s) — noted for whoever next touches `research/paypal.md`. Capture GET's `up` link yields the order id, so the existing T1.3b records sufficed with nothing created; driver catalogs the SKUs and fails on unknown ones.

### T2.6: The layer end to end, live (done) ★

**What exists.** `tests/live/layer/` scripted driver (no CI tests, `out/` ignored): full demo arc against real PayPal reads, live Jev (4 calls, $0.000085596, pinned build, per-call guard), live Lambo 0.10.1 native, throwaway indices deleted, session erased. Refund→pending→reason→Earned, correction wins with standing kept, feedback tied, dispute seller-only, recall-before-correction under trace-R.

**What surprised.** Review caught a real defect pre-land: the third live answer was opposite-direction (`fit_too_large`) while fixtures use three `fit_too_small` — fixed to the exact fixture wording, re-run live, Venerable intact; per-call costs now quoted recomputably. Tombstoned Lambo sessions never serve again (documented). Design note for P2 consumers: `beliefForReason` attaches any fit-kind reason to the most-supported fit belief with no direction check — fine under the frozen fixtures (all same-direction), but a mixed-direction world would need an explicit rule; recorded, not changed.

### T2.5: The interface calls (done) ★

**What exists.** `internal/api/server/` + `cmd/memento/`: the Keel service composing all four layer pieces behind the fakes' seams, 9 frozen tools over MCP plus plain per-tool routes, call log, webhook ingest, health check. Recall assembles through one constructor that fails closed (`ErrOverrideMissing` — nil/foreign/mismatched override can never serve). All 10 contract examples pass on fakes; story drive earns/contradicts/holds/grows; recall-before-correction logged under one trace.

**What surprised.** Frozen `domain.Subject` has no JSON tags — the service normalizes keys at its encode boundary rather than touching frozen types (accepted pattern; contracts stay frozen). Recall serves only Venerable fit beliefs — T2.6 confirms the bar before the agent depends on it. `pin.go` blank import now redundant (server imports the wire package for real) — left alone, harmless.

### T2.4b: The Lambo writer, live (done) ★

**What exists.** `tests/live/memory/` driver (no CI tests, `out/` ignored): demo shopper seeded against live Lambo 0.10.1 with NATIVE embeddings (`.jina-embeddings-v3`, `connect_timeout_ms = 30000` — the #236 knob working; 0.10.0's hardcoded 5s could not connect on this network). Belief Venerable on inspect (3.0/3.0, coverage 0.545, gc 17/3, Swarm), warning on recall, seller/assertion at None, dispute-adds-nothing. Sessions erased; no indices created (store ranking serves recall).

**What surprised.** Promotion is eventual — assert to convergence, not right after the receipt (span join needs interaction rows). Runner env collision (`AGENT=1` in the operator shell) — scripts now use their own var names.

**The next agent should know.** The native path is proven end to end on 0.10.1; the remaining switch work is the demo-shopper calibration confirmation + console billing.

### T2.3b: The evidence store, live (done)

**What exists.** `tests/live/evidence/` driver (no CI tests, `out/` ignored): T0.6's hybrid legs reproduced on the real deployment with native `.jina-embeddings-v3` — BM25 ranks all four same-line shirts above distractors, kNN reproduces the recorded native shape to 4 decimals, RRF keeps reason phrases top-3. Lambo 0.10.1 (build-only) derives and recalls through the native embedder: `connect_timeout_ms`/`timeout_ms` resolve, `= 0` refused with a source-carrying error (#238 fix visible), session reports `calibration=calibrated`, shipped jina-v3 row live, paraphrase 0.674 (wash vs 0.677/0.68 — no re-calibration). DNS stall from T0.6b is gone (0.01s), so the knob is proven-read but no longer load-bearing here. Indices verified deleted (independent empty-listing check); sessions erased; nothing shared re-embedded.

**What surprised.** Native `_inference` gives no usage/cost fields and `_cluster/stats` 410s on Serverless — nothing billable is API-visible; console billing confirmation stays the user's step. OpenRouter spend $0.

**The next agent should know.** The native contract shape and `re-embed` path are proven; the remaining switch work is calibration confirmation on the demo shopper + console billing, both P2-closing items.

### T2.2b: Reason classification, live (done)

**What exists.** `tests/live/reasons/` driver (no `*_test.go`, `out/` ignored): all 11 phrases through the live endpoint via the frozen client — 11/11 categories and verdicts match T0.7, model string identical (`typesafe/jev-1.14-20261014`), floor margin unchanged (1.00 vs 0.96), $0.000235284 total. Key env-only, header-only.

**What surprised.** Vague-item confidences wobbled (0.94→0.93, 0.68→0.51) — verdicts unchanged, floor intact. Fail criterion is category + verdict; drift prints loud DRIFT lines. Calibration-sample caveat stands.

### T2.4: The Lambo writer (done) ★

**What exists.** `internal/memory/` applying the write rule through its own `Graph` seam (production stays fakes-free; test adapter in `story_test.go`): story arc None→Candidate→Venerable/Earned, correction → Out-of-true with standing Venerable and 3 dependents kept, dispute leaves sizing struct-equal, reasonless/unmatched refunds write nothing. Cross-kind writes refused with `ErrKindMismatch` — rule 2 is load-bearing in the writer.

**What surprised.** `Insight` carries `DistinctInteractions`/`Coverage` (unasserted, available for `explain_belief`); T2.5 needs its own ~15-line adapter for the real client.

### T3.2: Catalog through Channel3 (done)

**What exists.** `agent/catalog/` matching on brand id + category + equal fit-handle sets with shared cut signals as evidence; Bonobos negative control (line name alone never matches); `ExcludeIDs` skips returned SKUs; `FulfilmentSimulated` on every candidate; apparel size gap pointed out-of-band at the retailer URL; shoe InStock sizes with narrowing echoed. Tests replay all fixtures; one live detail call re-verified the demo SKU (0 credits — detail calls are free).

**What surprised.** Search and detail share one wire shape, so one decoder covers both. No dependency gaps.

### T3.7: Public reachability probe (done — negative: platform gap)

**What exists.** `testdata/astropods/15-public-probe.txt` + research section: minimal page + chat endpoint on `probe-gw` port 80 with `frontend:true, messaging:true`. Anonymous GETs to BOTH the chat host and the dedicated frontend host 302 to the platform SSO login — before the flip, and twice after Public was switched ON (3-minute propagation waits, access-list unchanged, CLI shows no Public flag). Model turn via messaging proxy works ($0.0057, 17 obs, tool span) — the platform path is fine; only the front door ignores Public. Total $0.06 of $0.25.

**What surprised.** The deployment exposes two hostnames (chat + frontend); both 302 anonymously. `ast agent access set` offers `anyone` (= any signed-in user) but no public option. Public is documented only in an adapter-auth warning and has never been observed working — platform gap, not config. The earlier flip landed on `memento-agent` by mistake (user-confirmed; its posture is the user's call).

**The next agent should know.** No-login serving on Astropods is PROVEN on the public frontend host (dedicated `agents.public.astropods.ai` hostname; chat host correctly stays signed-in): anonymous 200 with our page body, model turn $0.0057 with tool spans. Follow-up T3.8 (public shopper page) added to the phase doc, parked until the full demo exists (requires T3.6). memento-agent's own Public posture is the user's call.

### T3.5: Evals (done)

**What exists.** `agent/evals/` 3-judge set (`recall_first`, `correction_honoured`, `approval_before_purchase`) pushed over the T3.1/T3.4 traces, every judge pass-and-fail recorded. Fail cases: errored recall, constructed ignores-correction counterfactual, constructed purchase-without-yes counterfactual (both pairs committed with identical prompts pinned by test, off-platform verdicts True/False as expected). $0.07 server spend, well under guard.

**What surprised.** Review caught a vacuous fail case (feedback turn proving nothing about ignoring corrections) — fixed by constructing the discriminating counterfactual, verified off-platform. Deeper findings, recorded: the judges read reply text, not tool spans (span order stays human-checked); pushing the custom set REPLACED the preset safety judges — re-add (safety three at minimum) as T3.6/T5 follow-up, 3+6=9 fits the cap of 10. `correction_honoured` scores false on every non-recommendation turn by design — scope eval sweeps to recommendation turns or expect red on feedback/approval turns.

### T3.4: Asking the shopper (done) ★

**What exists.** `agent/loops/` policy: ask-one-first-pending, strict explicit-yes gate, approve-then-mirror (local record and transport agree before capture), correction-wins recommendation, transaction-tied feedback; 9 tests incl. fail-closed capture and replay pin to `p3-turns.json`. Live: 6 deployed turns, one conversation — pending poll → reason (fit-too-small accepted) → correction (`corr_live01`, precedence) → honoured next turn → yes-approval (purchase honestly declined, no purchase tool) → fits-well feedback. Traces `879c8752/35598171/cef908ae/fad137db/839514c8/d6e19c5a`; spend $0.19 total (billing $0.08 → $0.27), max turn under the $0.10 guard.

**What surprised.** Turn 2's answer reinforced the old L-direction belief while M takes precedence — correct per decision-vs-belief, agent stated both. Approval-to-capture is proved in code, not on a live order: the deployed agent owns no purchase tool, so T3.6 needs the orders wiring to close that gap.

**The next agent should know.** T3.4's approve loop must mirror real approval into the transport or capture correctly fails. T3.5's evaluators score these traces.

### T3.3: Orders and double-buy protection (done) ★

**What exists.** `agent/orders/` durable state (proposed → approved → captured/failed/refunded), uniqueness on open (shopper, need) with `ErrNeedBusy`, `PayPal-Request-Id` on create and capture, webhook idempotency on event id, approve-first enforced on BOTH the capture path and the webhook path (transport never called without an approval record, order never moves). Sandbox-only construction. 9 offline tests: retried create, duplicate webhook, second-order-while-open, capture-without-approval, source mismatch, refund flow, fixture shapes, key-absence.

**What surprised.** Capture-API-then-webhook means a COMPLETED delivery for an already-captured order is a confirming no-op while a conflicting capture id is refused. Concurrent proposes can strand a never-approved order server-side — harmless (uniqueness still blocks money), but the production row store needs the same keys as unique constraints. T3.4's approve loop must mirror real approval into the transport or capture correctly fails.

### T3.1: The agent on Astropods (done) ★

**What exists.** `agent/runtime/` Mastra agent (`memento-agent`, web adapter, `messaging: true`), model `claude-sonnet-4-6` via gateway (deploy-time `MODEL_DEFAULT`, same string all three arms). Fake-backed memory tools matching the frozen contract; offline check 19/19. Deployed turn: `Brand X oxford, what size?` → M from the correction, citing the Out-of-true belief (standing Venerable), 3 fit returns, seeded marks, pending refund asked about, approval requested, no purchase claimed. Trace `b51715f4…`: generation → `recall_for_purchase` 187ms → generation → `pending_questions` → generation. Cost: $0.0367 good turn; billing $0.02 → $0.07 ($9.93 credit left). Re-verify spend independently with `ast billing get`.

**What surprised.** Mastra `createTool` takes `execute: (inputData, context)` — the wrong signature passed spec validation and only failed on live turns (model asked the user for a shopper id instead of retrying). Fixed + defaulted demo shopper + retry-once instruction. urllib's default UA gets Cloudflare 1010 on astropods.com (browser UA fixes the proxy drive). No purchase tool on purpose — orders live in T3.3.

**The next agent should know.** Standing config: fake-backed tools until T2.5 serves the layer. Redeploy after edits: `ast agent redeploy --name memento-agent --latest --wait`. `probe-gw` left Running.

### T4.2: The ledger's table (done) ★

**What exists.** `web/src/lib/ledger/` (row mapping, plain-JS grid wrapper, Chaaya theming, tooltip/pill renderers, keyboard navigation) + `web/src/routes/ledger/` step-picker page. Renders all 9 fixture steps in both themes (row counts pinned), keyboard traversal tested with real ArrowDown/Up, Out-of-true pill + tooltip + correction line, `Seeded` pills, weak-signal mark for kept rows. AG Grid lazy-loaded (landing chunk has zero grid references). `ag-grid-community@36.3.0` pinned exact; no Enterprise import.

**What surprised.** Scope note: `web/package.json` + lockfile changed (the grid dep — required, lockfile check green). Chaaya `reactive` subscribes on any property read, so refresh runs inside `untrack()` keyed on state identity. Landing lesson: the main checkout's `node_modules` predated the merge, so the main gate failed svelte-check until a fresh `npm ci` — environment staleness, not a code defect; re-run `npm ci` on main after any branch that adds a web dep.

**The next agent should know.** `toLedgerGridRow(s)` gives T4.3 row identity; `correction` already populated for beside-the-belief display. T4.4 adds editing separately — the grid is read-only here by design.

### T4.1: The ledger's data layer (done)

**What exists.** `web/src/lib/api/` typed client over injectable transport plus stepwise fixture mode (steps 1–9): decoders fail loudly on misshapen responses, step-9 snapshots `toEqual` frozen files, Earned at step 4, Out-of-true with kept standing from step 5, pending held open, seller dispute seller-only.

**The next agent should know.** `createFixtureState('shopper_01', step)` + `reactive(state)` + `refresh()`; JSON imports widen so decoders are the type check; grid lazy-loading still T4.2's per the spike note.

### T1.3b: The demo shopper in the sandbox (done)

**What exists.** `tests/live/seed/` live driver (run/approve/dispute scripts + README, no `*_test.go` so CI never executes it, `out/` gitignored): all six demo-shopper cycles created and read back via REST — 2 kept captures, 3 refunded captures with reason-in-words notes, 1 capture under an OPEN non-arrival dispute. Seeded-day mapping via `custom_id` join + anchor rule; day-0 statement has no PayPal counterpart by design.

**What surprised.** Order create with items needs `amount.breakdown.item_total`; approval needs two clicks (extras land benignly on `PAYMENT_ALREADY_DONE` — polling is truth); no throttling across 6 logins; resolution-center filing quirks recorded (curly apostrophes, native selects, required documents text, keyboard-Enter submit). Webhook deliveries not captured — REST read-back is the ledger's path anyway.

**The next agent should know.** Credentials travel only in env/browser sessions; `out/` holds real ids and stays ignored. Re-running creates a second shopper history — P5's arm runs use the recorded transcripts, not a live re-run.

### T1.4: Fixtures and fakes (done)

**What exists.** `internal/fakes/` (four interfaces — payment, classifier, store, graph — plus replay driver; unknown requests fail with `ErrNoRecording`, never invented) and `testdata/replay/` (manifest, 9-step story, per-phase inputs for P2/P3/P4). Gate proves offline: replay runs with keys unset and proxies broken, key-pattern scan covers replayed fixtures. `record_reason` stays ask-again (vague "not great" → unclear 0.94 < 0.98 floor) so the frozen T1.2 snapshots don't move.

**What surprised.** `domain.Subject` has no JSON tags (frozen) so replay output uses `Kind/ID/Name` vs snapshots' lowercase — decoding is case-insensitive, comparison test normalizes explicitly. Seeded returns enter pre-classified (as the seed model intends); the live Jev leg covers only the vague answer, but `ClassifierFake` serves all 11 labelled phrases for T2.2's table test.

**The next agent should know.** P2's real clients implement the fakes' interfaces as-is or record the difference. P1 contracts are now frozen except the live T1.3b twin: any shared-type change updates `testdata/` in the same commit and is the orchestrator's call.

### T1.2: The interface (done) ★

**What exists.** `internal/api/contract/` (wire types reusing domain enums, 9 frozen tools: 6 layer calls + 3 ledger reads), `docs/interface.md` (builder doc with 19 injected `validate=` examples), `testdata/api/` (schemas + examples per call, one consistent shopper story, plus an unclear ask-again variant). Gate test proves doc↔fixture equality, schema validity, full mutual coverage, and non-vacuity (break-and-restore). Recall carries both sides: status Out-of-true + standing Venerable + inline `contradicted_by` + top-level `overriding_correction`. Writes accept optional `trace_id` for P3's per-trace review; doc mandates recall-first ordering.

**What surprised.** The feedback row needed a judgment call: fits-well verdicts wire `{category: none, words, 1.0}` with `counts_toward: fit` — P2's writer must apply verdict→kind, not reason→kind, for feedback. `record_reason`'s accepted answer would add a fourth fit row the snapshots predate; T1.4's replay author decides whether to answer it. Scope note: `testdata/api/` was committed under this task's Done-when though the owns line names only the contract and doc — accepted, recorded here.

**The next agent should know.** Standing enum deliberately excludes `contradicted`. T1.4 builds fakes against these schemas; P2/P3 code to the wire types.

### T1.3: Seeded shoppers (done) ★

**What exists.** `seed/` generator (deterministic from `GoldenSeed`, fixtures reproducible via rewrite flag) plus `testdata/seed/histories/` (demo + 5 comparison shoppers) and `testdata/seed/hidden/answers.json`. Demo shopper mirrors the proven shape: statement, kept background, three fit returns days 3/6/9 of 11 (coverage ~0.55), later non-arrival dispute toward seller only. Comparison set tempts wrong-cause errors (broken, non-arrival, changed mind, genuine size change, vague). Tests prove determinism, universal seed marking, promotion-gate honesty, and hidden-answer separation (no `right_size` in any history).

**What surprised.** Kept rows mint order/capture/event ids but no refund/dispute ids (links fill only what each outcome kind has); dispute rows fold the purchase capture into the link since a bare capture carries no signal. Verified the marking claim independently (serialized `SeedMark`, materialized `Seeded`).

**The next agent should know.** `seed.Write` materializes arm inputs; `Row.SeededEvidence(anchor)` converts relative days to absolute times. Comparison answers: broken M, never-arrived L, changed-mind M, size-changed L, unclear M.

### T1.1: The domain model and architecture (done) ★

**What exists.** `internal/domain/` (evidence, 7 reason categories, 3-code dispute table with `ErrUnknownDisputeCode`, belief + status/graph round trip, 5-signal write rule with `record_action`/`derive`/`observe`, swappable `EmbedderContract` with no model constants, zero vendor names) plus `docs/architecture.md` tracing webhook to belief with research citations. Table tests cover every signal, category, and dispute code.

**What surprised.** Two judgment calls, both recorded for successors, neither blocking: `UNAUTHORIZED_TRANSACTION` maps to count-toward-nothing (fraud-as-seller-signal would be a deliberate product decision, not a quiet edit — contracts freeze now); `kept-past-window` maps to a third op `observe`, weaker than `derive`, to stay honest to "observation only". Design note for T1.2: contradicted is an overlay (Venerable stays Venerable with warnings), but the model maps it flat to Out-of-true — the recall representation must carry both the underlying standing and the warning.

**The next agent should know.** T1.2's contract and T1.4's fakes build on `WriteFor`, `CategoryForDispute`, and the status round trip; T1.3's generator fills `Evidence.Seeded` + `SeedMark`. All covered by tests, no hidden behavior.

### T0.9: Hosting (done)

**What exists.** `dev-diary/research/hosting.md`: everything on Render at $7.25/mo ($7.00 `0.5c-512mb` + $0.25 1 GB disk, Hobby workspace $0) against the $50 credit — ~6.9 months runway, several times the retention rule. Free tier disqualified three ways (spin-down, ephemeral FS, no disks). Single web service with Lambo beside it on the disk mount; Lambo binary and `lambo.toml` ship in the image, only the data dir on the disk. Exact `POST /v1/services` and `render.yaml` provisioning calls recorded (nothing created, $0 spend), plus the PayPal webhook registration pattern and a P6 pre-flight checklist (credit balance, payment method on file, region, `store-sqlite` binary, no deploys in the judged window).

**What surprised.** Disk-backed services lose zero-downtime deploys (seconds dark per redeploy) — PayPal retries plus event-id idempotency cover it, but don't deploy during judging. Suspension (not deletion) is the credit-exhaustion path, with disk + 7-day snapshots retaining the store.

**The next agent should know.** P6 provisions this, nothing else needed from the user: no payment method required (confirmed in the dashboard).

### Lambo 0.10.1 closes 236-238 (unblocked)

Lambo v0.10.1 ships the fix (`fix(embed,recall): elastic timeouts, transport diagnostics, jina-v3 calibration`, PR #239): configurable elastic timeouts, error sources, and a calibration row for jina-v3. The Elastic-native direction is unblocked code-wise. Remaining on our side, in order: adopt the native contract (shape + `re-embed` path already documented in T0.6b), verify the shipped calibration against our paraphrase set (T2.3b/T2.4b live twins are the natural home — brief them to build 0.10.1 with `embed-elastic` + `recall-elastic`), and confirm Elastic console billing (still a user step).

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

**What exists.** `dev-diary/research/astropods.md` plus 11 recordings in `testdata/astropods/`. Proven live on the free tier: TS/Bun+Mastra and Python/LangChain scaffolds, `push`/`deploy --adapter web` with web chat URL and collector sidecar, gateway model access with no personal key, secrets vault round-trip (create/list/get-masked/delete, vault left empty), deployment pause/resume, evaluator schema discovery (`EVALUATION.yaml`) plus a custom `followed_approval` set activated server-side, OTel collector wiring with empty trace/usage states, and the external-MCP pattern with a plain-agent-tools fallback. Probe spend was $0.00; later turns cost $0.02 (T0.8 close-out) plus $0.06 (T3.7) — see the running total in the T3.7 entry.

**What surprised.** The live 3-turn chat first stayed open (deployed web chat needs user-scoped SSO) and the chat URL 404'd twice: first a stale probe, then a missing `agent.interfaces.messaging` block in the deployed spec (anonymous checks 302 to SSO and look healthy either way). Both fixed by redeploy; then the turns were driven without a browser via the platform messaging proxy using the CLI's own token. Results: turn 1 shows 17 ordered observations with the `lookupReturn` tool span between generations; turn 2 asks for "yes" with no tool span; turn 3 resumes and honestly declines (no record tool) rather than hallucinating. First `eval run`: `followed_approval: true` at 0.99. Total spend $0.02 of $10.00. Free compute is small ("Slowed by CPU limit" — keep demo turns concise). Two CLI quirks recorded: nested help needs `ast help <cmd> <sub>` form; enum evaluator options use field `options`, not `values`.

**The next agent should know.** Approval/correction forms are agent-built chat turns, not a platform primitive; feedback lands per trace via `eval review`, traces of interest via `dataset add`. Trace ids and the eval transcript are in `testdata/astropods/12-traces.txt` and `13-eval-run-review.txt`; replay them offline. The probe deployment can be paused or removed once T3.1 stands up its own agent.

### T0.6: Elastic evidence store (done)

**What exists.** `dev-diary/research/elastic.md` plus 5 scrubbed recordings in `testdata/elastic/` (vectors omitted, rankings kept). Hybrid search verified on a throwaway index (created, queried, deleted): BM25 finds the returned shirts for a new-SKU text, kNN over BGE-M3 puts `too tight in the shoulders` at 0.84 for a fresh `snug across the chest` query, and the combined shape needs `retriever:{rrf:...}` on this deployment. Embedder settled: BGE-M3 through OpenRouter (`baai/bge-m3`, native 1024 dims), proved end to end through Lambo derive+recall. Recall tier confirmed on a locally built 0.9.1 binary (index creation, external-version 409s, kNN, backfill, `_delete_by_query` erase, store fallback with tier down). Hosting: keep the existing Serverless project, start nothing.

**What surprised.** The deployment is Elastic Serverless (9.7.0), not a trial — the brief's trial-vs-self-host framing did not apply. Installed Lambo 0.8.0 lacks `recall-elastic`, and cold rustls handshakes (~6.8 s) trip the default 5 s timeout: every `lambo.toml` needs `timeout_ms = 20000` in `[recall]` (proposed Lambo issue text in the worker's report; the user files it). Serverless excludes vectors from `_source`, so Lambo's exact re-scoring keeps engine HNSW scores — an accepted, slightly widened difference. Spend bounded under $0.00001; test indices deleted.

**The next agent should know.** The embedder contract is fixed (`kind=bge_m3`, `url=https://openrouter.ai/api`, `model=baai/bge-m3`, `dim=1024`, `api_key_env=OPENROUTER_API_KEY`); every later session must match or re-embed (`lambo re-embed`). T0.9 no longer needs an Elasticsearch trial decision — only Lambo persistence plus public HTTPS. Billing resolved by the user in the console: 0.7095 ECU month-to-date (vector DB 0.6921, add-ons 0.0174), FREE during trial.

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
