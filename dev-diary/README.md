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

**Current status:** P0 to P5 are done as first built, and the pivot in product.md is decided. P7 and P8 are done; P9 and P10 carry the remaining work (surfaces, the demo's live gaps, and round 2's runs). P6 resumes after P9 and P10.

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
| **P7** Pivot contracts | [PHASE-7-pivot-contracts.md](PHASE-7-pivot-contracts.md) | T0.11, T0.12 | none | P8 to P10 |
| **P8** The layer, after the pivot | [PHASE-8-pivot-layer.md](PHASE-8-pivot-layer.md) | T7.1, T7.2 | T10.1, T10.2 | P9, T10.3 |
| **P9** Population and surfaces | [PHASE-9-pivot-surfaces.md](PHASE-9-pivot-surfaces.md) | T7.1, T7.2 for T9.1; T8.3 for T9.3; T8.5 for the rest | P8's live twins | T10.3, T10.4, P6 |
| **P10** Comparison, round 2 | [PHASE-10-comparison-round-2.md](PHASE-10-comparison-round-2.md) | T10.1 and T10.2 none; T9.1 and T8.5 for the freeze; T9.5 and T9.6 for the runs | P8, P9 | P6 |
| **P6** Ship | [PHASE-6-ship.md](PHASE-6-ship.md) | P2 to P5, then P9 and P10 after the pivot | none | submission |

```text
  T0.1 accounts ─▶ T0.3 to T0.8 live checks ─┐
  T0.2 scaffold ──────────────────────────────┼─▶ T1.1 ─▶ T1.2 ─▶ T1.4 ─┬─▶ P2 layer ───┐
                                              └─▶ T1.3 seeded shoppers  ├─▶ P3 agent ───┼─▶ T3.6 full demo
                                                                        └─▶ P4 ledger ──┤
  T5.1 harness ─▶ T5.2 grader ─▶ T5.3 baseline pilot ─▶ T5.4 freeze ─▶ T5.5 arm 3 ─▶ T5.6 ┤
                                                                                         └─▶ P6 ship
```

After the pivot:

```text
  T0.11 + T0.12 ─▶ T7.1 contracts ─▶ T7.2 keys ─┬─▶ T8.1 hub ────────────────┐
                                               ├─▶ T8.2 fit writer ─▶ T8.3 pool ─┼─▶ T8.5 recall ─▶ T8.5b live
                                               ├─▶ T8.4 corrections ─────────────┤
                                               ├─▶ T8.6 auth ────────────────────┘
                                               └─▶ T9.1 population
  T8.3 ─▶ T9.3 merchant app ─▶ T9.3d deployed
  T8.5 ─▶ T9.2 ledger, T9.4 agent ─▶ T9.5 second agent
  T8.5b + T9.1 + T7.3 ─▶ T9.6 live on Render ─▶ T9.7 demo
  T10.1 arm 2 input, T10.2 relabel ─▶ T10.3 freeze ─▶ T10.4 runs ─▶ T10.5 results ─▶ P6
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
| **E: Pivot layer** | P7, P8 | `b` tasks only | T7.1 | Recall never refuses; the pool is written at ingest and read back |
| **F: Pivot surfaces** | P9 | Yes, for agents and deploys | T9.1 | The population, ledger, merchant app, two agents, the demo |
| **G: Round 2** | P10 | Yes, for T10.1 and the runs | T10.1, T10.2 | Both start now; the freeze waits on T9.1 and T8.5 |

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
- **status:** `not-started`, `claimed:<agent>`, `in-review`, `done`, `blocked`, `held` (the user ordered the task not to run), or `superseded` (the user replaced the task's plan; the task says by what).

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
| P0 | 12 / 12 | Done |
| P1 | 8 / 8 | Done |
| P2 | 17 / 18 | In progress (T2.3c) |
| P3 | 6 / 6 | Done |
| P4 | 10 / 11 | In progress (T4.11) |
| P5 | 10 / 11 | In progress (T5.8b) |
| P6 | 1 / 8 | T6.2, T6.8 open; T6.3–T6.7 the user's |
| P7 | 7 / 7 | Done |
| P8 | 27 / 35 | In progress (T8.5k, T8.5n, T8.5o, T8.5q, T8.5r, T8.5s, T8.5u, T8.5v, T8.5x, T8.7b) |
| P9 | 30 / 32 | In progress (T9.15, T9.24) |
| P10 | 5 / 8 | In progress (T10.4, T10.5, T10.6) |

---

## Handoff Log

Newest first. What exists, what surprised, and what the next agent should not have to work out again.

### T8.5p: The live-origin ledger rows stay unmarked (done)

**What exists.** The decision, with a row-by-row audit, is in the phase note; `testdata/api/` is unchanged because every frozen ledger row already carries the mark its origin gives it. `seeded: false` on `blf_2b9d60`/`ev_51c2d8` and `blf_1a5e09`/`ev_9d4c22` records that both events were lived in the sandbox; the mark means generated history, not "arrived through the boot seed". `LoadSeed` copies each row's recorded mark. T8.5l's finding names this task.

**What surprised.** The review found the notes overclaimed twice: `explain_belief` has no seeded notice (only `ledger_belief` does), and flipping the four marks leaves `TestContractExamplesAgainstService` green because it compares the seed to the same fixture — what actually pins the value is `TestLedgerReadsNarrow`, the doc pin, `internal/fakes`' replay test and the web mirror. The mark is copied in four places: api fixtures, `testdata/replay/p4-ledger.json`, `internal/fakes/final.go`, and `web/src/lib/api/fixture.ts`.

**The next agent should not have to work out.** A mark change moves four places at once and is a change to the mark's meaning, needing a contract task. Do not "fix" a lived row to `seeded: true`.

### T9.23 / T9.20: Step 2 re-recorded, both clauses (done)

**What came before.** The first re-record (trace `ae5d5e49`, $0.1112, three Channel3 credits) proved only the sequencing: recall answered 2.6 s before the first search, no overlap. The reply then named `PSW8GeK`, a kept purchase its own evidence carried, after dropping the exclusion when the filtered search returned nothing — so the turn overran both budgets and reopened T9.22. T9.25 is the lever that closed it: the exclusion's source is in the tool description and the parameter description, and the instruction says an empty filtered answer is reported, never retried without the exclusions.
**What exists.** After T9.25's redeploy (build `bee5a9e7`), one fresh conversation with the product request and nothing else made trace `600bf64f`: `recall_for_purchase` 06.851–07.311, one `search_catalog` 09.718–10.883, answer generation 10.893. Recall answered 2.4 s before the search started, there was one search and one Channel3 credit, and the reply names `VyAkjCA` ("Button Down Oxford Shirt", Brand X/Sunspel, Regular) — in the demo cut, and no returned row carries it. Size M comes from recall under `corr_000004`. Evidence [`demo/evidence/step2-catalog/04-catalog-candidate-t925.json`](../demo/evidence/step2-catalog/04-catalog-candidate-t925.json). Turn cost $0.0660 (under the $0.10 budget) and one credit (within budget); both turns together moved usage $6.17 → $6.31 and credit $3.83 → $3.69. This closes T9.23's naming clause and T9.20's flaw.

**What surprised.** The hosted chat does not record agent replies. Two turns in that conversation each left the shopper's message and an **empty** agent message in `GET /conversations/<id>/history` (`content: ""`, `is_complete: true`), while each turn's trace carries the full reply. The demo's evidence reads traces, so the run is unaffected, but a person opening the deployment's chat URL sees no answer.

**The next agent should not have to work out.** Do not point a judge at the chat URL until the reply persistence is explained. A trace records span names and times, not tool arguments, so "the search excluded the returned SKUs" rests on the reply's words and the candidate not being a returned row — re-record rather than argue. Issue text for Astropods: "Hosted web chat at `deployments/<id>/messaging`: `POST /conversations/<id>/messages` runs the turn (trace carries the reply) but `GET /conversations/<id>/history` returns the user message and an empty agent message with `is_complete: true`; two turns, both empty, no error in the agent logs."

### T4.10: The ledger loads only what it shows (done)
### T8.5t: The demo driver expects thin evidence (done)

**What exists.** `demo/driver/main.go` asserts the shape T8.5i serves: one return comes back Claimed/None with its one too-small row, `own_evidence` deciding, guidance "Take size L for Brand X"; the final recall serves the sizing belief first with the Claimed fits-well belief beside it. Review rebuilt the driver's wiring on the fakes and ran both assertion blocks verbatim, mutation-checked the old emptiness check and the pre-T8.5i filter, asserted the first answer's classification where it is asked, and split the final recall's count and order diagnostics. No live run (the live leg is T8.5u's driver, not this one).

**What surprised.** Nothing executes this driver in CI; it compiles and vets only. The one-return line prints no belief id because that id is the sizing id, redacted only later.

**The next agent should not have to work out.** A gate run once flaked at web vitest; three full reruns and 25 focused reruns passed, mechanism unproven.

### T9.25: The exclusion lever (done)

**What exists.** `search_catalog`'s description and its `exclude_skus` parameter name recall's evidence rows as the exclusion's source; step 2 allows one search per turn unless the first returns no products (a second keeps the same exclusions, never a third), reports an empty-with-exclusions answer rather than retrying without them, and forbids a memory SKU as the candidate. Five offline checks pin the text and the model-visible parameter description (189 checks); review tightened two of them (the second-search clause must keep the exclusions; the instruction extraction reads only the template body) and folded T9.23's superseded wording in place.

**What surprised.** The model-visible parameter description is where the lever bites; a check reading only source text could be satisfied by a comment. The shelf may genuinely have no new candidate once the returns are excluded, so the re-record can honestly answer "no new candidate".

**The next agent should not have to work out.** A redeploy plus one fresh step-2 turn (T9.23) is the remaining proof; if it reports an empty shelf, that is a shelf-population decision, not an instruction bug.

### T4.10: The ledger loads only what it shows (done)

**What exists.** `LedgerState.refresh` loads beliefs and pending questions only; the evidence getter and field are gone, so a failed `ledger_evidence` read cannot label the ledger failed over data it never renders. Evidence reads stay on `LedgerClient` and the why view fetches its own for one belief. Tests pin the two-call load in live and fixture modes, a refusing evidence transport that changes nothing, and the live page rendering while the edge refuses that tool; re-adding the read fails six tests. Review mutation-checked both the read and the why view's own evidence call, and annotated T4.8's now-stale finding in place.

**What surprised.** The retry test hard-coded three refused requests (a load used to be three reads), so it moved with the read count. T4.1's frozen wire fixtures are untouched; `LedgerState` is a web-internal observable, not a frozen shape.

**The next agent should not have to work out.** The read count is stated in the retry test's comment; a future read-count change moves it again.

### T10.7: Arm 2 reads kept purchases (done)

**What exists.** The round-2 reference reasoner reads unreturned purchases on the candidate line (pairing order to capture by the report time both records share), marks returns by refund reference or dispute seller transaction, applies `domain.UsualSizeFor`, and answers arm 2 on the own-evidence rung. Two new `Missing*` texts and per-turn `MissingAfter`. Measured: `r2-corr-standing` holds M on the first turn (the P10 review's flaw), `r2-crowd-plain` ties M–S and holds nothing (the review's "kept S" was wrong; the S purchase is the feedback capture), the other six hold none. FROZEN's claims now state the measurement, §11's stale repeats sentence is gone, the manifest is re-frozen at 116 rows. Review re-derived every shopper from the records, mutation-checked the kept read, the tie branch and `MissingAfter`, and corrected a §4 sentence about the two new-to-brand shoppers.

**What surprised.** The kept rows are declared inside the session, so no return window has closed; no arm can see a window, so it is an accepted limit now stated in the freeze. The review's original "kept S" was a measurement error. T10.5's limits must say the kept rows were declared.

**The next agent should not have to work out.** T10.6 (ladder) must land before T10.4, or its edit fails the freeze under an open run marker. The order–capture pairing key is the report time; a record without `create_time` would unpair every purchase.

### T8.5w: Two corrected beliefs for one brand (done)

**What exists.** `assembleRecall` no longer requires every contradicted belief's inline correction to be the single top-level one: each contradicted belief must carry its own inline correction citing it, and the top-level override must be one of them. The High (rival birth + one correction per belief made every recall for that brand fail with `ErrOverrideMissing`) is reproduced in `two_corrections_test.go` and pinned at the assembler boundary in `invariant_test.go`. No JSON shape, schema or fixture changed — `BeliefView.ContradictedBy` was already per belief — only doc comments now say the top-level correction is the newer when two apply. Rule 4 holds: both beliefs stay contradicted on the ledger; a second recall is stable. Review added the citation check, pinned per-belief precedence, and documented the equal-time tie-break (later recorded wins).

**What surprised.** The P8 review expected a contract change; the assembler's equality check was the whole defect. Two corrections with different sizes now return both honestly rather than refusing, and nothing reconciles the shopper's own contradiction.

**The next agent should not have to work out.** A response list is never needed; the inline field exists. Equal timestamps resolve by insertion order.

### T4.9: The fixture contest state uses its shopper (done)

**What exists.** `createFixtureContestState(shopperId)` passes its shopper to the fixture transport, so a state's requests and the fixture that answers them agree; the belief page's fixture branch now builds through the helper (the live branch keeps its direct construction). `state.test.ts` proves `shopper_02` gets its frozen standing answer and `shopper_04` records fresh wording, and that the demo path and T4.6's single-shopper guard are unchanged. Review mutation-checked the argument (three non-demo tests fail without it) and fixed a glued doc comment from T4.4.

**What surprised.** A non-demo shopper's *belief* correction cannot succeed through the fixture: `blf_8f2c41` is the demo shopper's and the fixture refuses it by ownership, so the test proves the shopper check passed by the refusal reason. A real non-demo belief needs its own belief id.

**The next agent should not have to work out.** T4.6's handoff line saying the helper still used the demo shopper is now history; the helper honours its argument.

### T9.22: Catalog search after recall (done)

**What exists.** The reference agent's step-2 instruction now waits for recall's answer, builds `exclude_skus` from the returned beliefs' `item.sku` values, passes none when no rows came back, and names only a SKU the search returned. Five offline checks pin the clauses (184 checks, was 179); `tsc` and the gate are green. Review mutation-checked all five and corrected a false fact in the note (the search overlapped recall and ended 1.1 s after it answered, rather than ending before).

**What surprised.** The sequencing is model behaviour: the text is proven, not the model. T9.23 owns the redeploy and one live re-record. If the turn still parallelises, the tool description (which does not name recall as the source of `exclude_skus`) is the next lever and T9.22 reopens.

**The next agent should not have to work out.** The exclusion removes `tbnrl46` specifically, leaving `VyAkjCA`/`PSW8GeK` in the recording.

### T2.5b: Keel pin and seller subject (done)

**What exists.** The redundant `internal/version/pin.go` blank import is gone; `go mod why` resolves Keel through `internal/api/server` and `internal/auth`, and `go mod tidy -diff` keeps the requirement while pruning 27 `go.sum` module-only lines. The seller-subject decision is recorded in `docs/architecture.md`: dispute records do carry the payment account (`seller.merchant_id`/`payee`/`payer`), but that is one account for the whole layer, so the belief keys to the brand (`seller-for-{brand}`); the marketplace condition that would change it is stated. The `content.go` comment says the same. Review mutation-checked that restoring `pin.go` changed nothing and that altering the derivation fails the recorded graph content.

**What surprised.** The task text's premise ("dispute records carry no seller id") was false; they carry an account, not a per-brand seller. Every committed merchant id is the same redaction placeholder, so the "one account" sameness rests on the live reads.

**The next agent should not have to work out.** `go.sum`'s 27 unused module-only lines are harmless; nothing owns `go.mod`/`go.sum` tidy.

### T1.1d: One unknown-code rule (done, P1 closed)

**What exists.** `domain.StoredCategoryForDispute` is the only place an unknown dispute code's disposition is written (unknown → `none` with no error; anything else propagates), and both served ingest and the live driver call it; neither keeps a private copy. `CategoryForDispute` still refuses unknown codes, so the domain's strictness is unchanged and every pre-existing test stands. The task first landed `blocked` because its `owns` omitted `internal/ingest/` while its Done-when needed it; the orchestrator widened the line rather than cut the rule in half. Review ran three mutations (error on unknown; lenient domain; wrong category) and all bit.

**The next agent should not have to work out.** `internal/ingest`'s dispute tests still state expected categories independently of the frozen table on purpose: a new code fails there until its row is added.

### T9.21: The agent packages are type-checked (done)

**What exists.** Inside `check_agent`'s bun/modules guard, the gate now runs each package's own `tsc --noEmit -p tsconfig.json` after its offline check, so a type error fails the run; a missing `tsc` fails loudly rather than skipping. Both packages are green today; `typescript` was already a pinned devDependency, so no dependency or network was added. Review proved the failure paths (a string assigned to a number fails the gate; a renamed `tsc` fails it) and corrected two note claims: the skip is the guard's own (the frontend's fallback differs), and a missing binary reports "No such file or directory", not "command not found".

**What surprised.** The gate's `go vet` failed spuriously twice because the shared Go build cache was being clobbered by concurrent worktrees; a private `GOCACHE` was green, and `go clean -cache` fixed the shared one.

**The next agent should not have to work out.** The skip line names the offline check and the reason, not the typecheck; both steps are covered by the same guard.

### T4.8: The ledger reads live (done)

**What exists.** `web/src/lib/api/http.ts` posts one tool call to same-origin `/tools/<tool>` with the shopper link token as `Authorization: Bearer`; `live.ts` reads `?t=` (and `?s=` for the shopper the token binds, default the demo shopper). `/ledger` and `/belief` with a token render the layer's beliefs, evidence, pending questions, recall and live contest/standing/pooling writes, label their source, and show a failure alert with no data when a live read fails (never a replay passed off as live). Without `?t=` the demo story replays, labelled and with no fetch. Review fixed the belief page claiming a live source when nothing live answered, pinned the unreachable failure as token-free, and corrected the test count.

**What surprised.** A token does not name its shopper: the guard compares the body's `shopper_id` to the binding and nothing in the edge reports it, so `?s=` exists (default the demo shopper). The ledger page also fetches evidence it never renders, and a failed evidence read fails the page → **T4.10**.

**The next agent should not have to work out.** The token is meant to be visible in the URL only; no failure message may contain it. `demo/README.md`'s stale "fixture replay" lines are T9.20's.

### T9.11: The Astropods org id is scrubbed (done)

**What exists.** All three drivers (`demo/drive.py`, `demo/run.py`, `agent/second/drive.py`) read the org from `ASTRO_ORG` or the CLI's own config (`~/.ast/credentials.json`, the current profile's single org), stop with a clear message when it is missing, unreadable or ambiguous, and never pick a default; the keyring lookup uses the resolved org and fails clearly on a miss. `demo/run.py --summary-only` still needs no Astro login. The gate is green.

**What surprised.** The review's only fix was to the note; the code needed none. A dry run with no org and with two orgs stops as designed.

**The next agent should not have to work out.** The org id must not reappear in a driver; the CLI config is the source.

### T9.19: The gate runs the agents' offline checks (done)

**What exists.** `tools/check.sh` runs both agents' offline checks behind the same bun/modules guard the frontend typecheck uses, prints which check it skipped and why, and fails on a check failure; `ci.yml` installs both packages' frozen dependencies after setup-bun and caches on all three lockfiles. Review proved all four branch combinations (full, no bun, one package missing, both missing), mutation-checked a broken tool id (gate exit 1), and confirmed the pin comment and the `server/status.ts` fallback. The gate's counts are 130 and 125 on this tree (179 for the reference agent after T9.17).

**What surprised.** Neither agent package is type-checked anywhere: Bun transpiles without checking, and both `tsc` runs are green today → **T9.21**.

**The next agent should not have to work out.** The skip lines mean a bun-less workstation still does not exercise the tool surface; CI does, because it installs the frozen packages.

### T9.17: The reference agent searches the catalog (in review until T9.20)

**What exists.** `agent/runtime/agent/catalog.ts` adds `search_catalog`: a port of the Go matcher's rule (brand + category + equal fit set, never line name alone, excluding returned SKUs) over Channel3's `/search`, returning the recorded records unchanged. It is agent-local, not an edge tool; the eight layer tools still equal the access table, and the check asserts the split. The offline check replays `testdata/channel3` and pins field parity, the negative controls, the refusals and the HTTP shape (179 checks); `tsc` clean; gate green. Review found and fixed an untested refusal and corrected the recorded size list (7.5, 9, 9.5, 10, 10.5 — gapped).

**What surprised.** The deployed agent is Bun and the matcher is Go, so the rule is a port held to the same recordings. The layer's line identity differs from Channel3's (`brand-x-001`/`shirts`/`regular` vs `SMwk`/`dress-shirts`/`top-fit: Regular`), and the mapping lives in the agent instructions.

**The next agent should not have to work out.** T9.20 must create the `CHANNEL3_API_KEY` vault secret, redeploy with both `--var`s, run the live step-2 turn and close T9.17. `product.md`'s size-chart sentence was corrected: no size chart exists, the size comes from recall.

### T8.5i: Recall serves thin evidence (done)

**What exists.** `recallBeliefs` no longer gates on standing: every fit belief for the requested brand is served, so a lone return arrives as `Claimed`/`None` with its one evidence row, its `own_evidence` precedence step, and the pool beside it marked overridden. The correction rule is untouched. The seed already carried the case (`blf_2b9d60`, a one-row fits-well belief), and `assertRecallPivot` now pins it; `baseRecallBeliefs` is the single filter both fixture comparisons use. The now-reachable one-return guidance sentence was ungrammatical and is fixed; two- and three-return texts are byte-identical.

**What surprised.** The thin belief was in the frozen fixtures all along, withheld from every seeded recall. The guidance still has a wording defect (a fits-well belief is called "runs small", and "ahead of the pool" fires when the pool agrees) → **T8.5v**. The demo driver asserts the old withholding → **T8.5t**; the live layer tripwire is flipped but unrun → **T8.5u**.

**The next agent should not have to work out.** T8.5's own "Candidate and above" rule and "lone return stays on the ledger" advice are marked superseded in the phase notes.

### T9.14: The relay's status mapping is tested (done)

**What exists.** `agent/frontend/server/status.ts` owns the `/api/chat` mapping (400 invalid, including malformed JSON; 429 exhausted; 502 relay failure with `turn_id`), and `index.ts` answers refusal/failure paths only through it, with a source guard against a literal status handed to `json()`. The cases ride the gate's existing node step because extracting the import-free module removed the `Bun.serve` import; the same file is 22/22 under bun. Review verified every status off the wire with a closed-port gateway (400 both flavours, 429, 502's exact body) and fixed two header/note claims (the 200 reply and the 404 splat are not chat outcomes).

**What surprised.** The unparseable-body 400 was a fourth inline status outside the kinds; it moved too. `status.ts` must stay import-free or the node step loses it.

**The next agent should not have to work out.** The source guard reads `json(` call sites only; a `new Response(..., { status: 5xx })` in the chat handler would slip it.

### T9.18: The deploy README's pins cover bun (done)

**What exists.** `deploy/README.md`'s toolchain table lists Go, Node and Bun with their sources and says what the check reads (18 declarations); the Bun row is a required source in `tools/pincheck`, so a drifted or missing row fails the gate naming the file. The table test covers both new cases. Review corrected two prose claims (the quoted output line and "the four agent images"), and confirmed that a patch-stating row (`1.3.0`) is compared exactly against CI while a major-only row (`1.3`) is not.

**What surprised.** The table row is bun's third source; `tools/check.sh`'s pin comment still named two, which T9.19 carries.

**The next agent should not have to work out.** Adding a toolchain means adding it to `readmeKinds()` and the table together, or the row is unread.

### T9.9: Agent tool surface and retry discipline (done)

**What exists.** Both agents offer exactly the eight calls the edge admits to an agent key; the three shopper-only ledger reads are unbound rather than refused, and the access table was deliberately not widened (the ledger is the shopper's own view). `tools.ts` is a factory over the transport, so the offline checks exercise ids, input schemas and `execute` bindings against the frozen contract and the Go access table itself. The retry rule is a numbered step: reads may retry once, writes must be read back before a resend, `set_pooling` is the only write safe to repeat. Checks: 130 (reference) and 125 (second). Review strengthened three pins (read-back pairing, the retryable list, the write list) and corrected the note's count.

**What surprised.** The gate does not run either check, so the coverage was invisible to CI; **T9.19** now owns it and must also install the two agent packages, or the runner falls through the guard. The deployed builds do not match this commit until both agents are redeployed.

**The next agent should not have to work out.** The redeploy commands are in the T9.9 note; `--var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY` needs the input declared in `astropods.yml`, or the platform silently drops it.

### T4.6: One shopper at a time on the belief page (done)

**What exists.** The pooling fixture decodes each frozen answer and refuses when its shopper is not the transport's, so a case shopper with no frozen pooling answer fails closed instead of replaying the demo shopper's three removed rows. The correction fixture takes the shopper it answers, rejects another, refuses the demo belief from another shopper, and answers the frozen standing request's `shopper_02` byte for byte. The belief page builds its `ContestState` from the case shopper's transport and keys both the standing editor and the toggle by shopper, so a case change remounts them; the duplicate empty-state lede is gone so the sentence prints once. The frozen standing test now sends its own shopper, and a review test proves the editor remounts too (removing its key left the original suite green).

**What surprised.** The pooling fixtures carry the demo shopper's answers only, so the other four cases now say they have no answer rather than restamping three removed rows under another shopper — honest, and visible. `createFixtureContestState` still builds its transport for the demo shopper; **T4.9** fixes the helper.

**The next agent should not have to work out.** `{#key caseShopper}` (not the case) is right: two cases can share one shopper, and keying by shopper keeps that shopper's result.

### T8.5m: A correction id is never reused (done)

**What exists.** `takeCorrection` is the only writer of a correction's place: it inserts an id once, keeps the first view, and reserves its number against the mint sequence, so a restored `corr_000001` makes the next mint `corr_000002`. `explain_belief` and `ledger_belief` share `correctionsFor`, which serves one row per id even for a journal already written with duplicates. Tests cover the restart, a re-seed, a repeated order, and the T9.7 run's own seven-row/five-id journal (now five rows, next mint `corr_000005`). Rule 4 unchanged: the correction still wins, the belief stays, the timeline event stays single.

**What surprised.** The duplicate rows came from `LoadSeed` replaying beside the restored journal because beliefs are not restored, so `ErrAlreadySeeded` never fires after a restart. Review also found `LoadSeed` re-issues the correction's graph write at every boot even when the restore holds it — no served-state effect offline; **T8.5s** proves it on the live hub.

**The next agent should not have to work out.** Only correction ids are reserved: `ev`, `q`, `blf`, `rsn`, `fb` still start at 1 each process because nothing restores them; `reserveID` is generic if that changes.

### T1.1c: The live ingest driver on unknown codes (done)

**What exists.** The live ingest driver now maps an unknown dispute code the way served ingest stores it (`none` plus the code as text) instead of aborting, and its row assertions derive the category and counts from the case's own code instead of assuming non-arrival. It re-ran live: the recorded refund and non-arrival dispute stored one row each with verified signatures, both redeliveries were duplicate-safe, no new sandbox record exists, spend $0. The not-as-described spelling remains unobserved: the sandbox refuses the create (reason-only → `MISSING_ISSUE_TYPE`; with items → `TRANSACTION_NOT_ELIGIBLE`; `seller_transaction_id` → 500), the response is committed as a fixture, and a reproducible probe plus the README record all three shapes. Review fixed a probe that printed a created case's body and two inaccurate claims, and opened **T1.1d** because the disposition now exists in two places.

**What surprised.** T0.4's minimal create shape now fails earlier at schema; with items it reaches the same eligibility refusal. Naming the capture as seller transaction returns a 500, a sandbox defect.

**The next agent should not have to work out.** To observe the spelling, file the case as the shopper in the resolution center (the `tests/live/seed/dispute.py` click path) and re-run the driver: it prints the live code and the mapped category, no code change needed.

### T4.7: Plain language in the why cards (done)

**What exists.** The cards lead with the shopper's words, the category reaches the screen only through `reasonWord`, and the trace note says it is a fixture replay with a live transport still to come. Review fixed two dishonest mappings: `none` was being called "fits well" (the contract's `none` is "no classified reason", and an `UNAUTHORIZED_TRANSACTION` dispute would have printed as a fit verdict) — it now names no category and lets the recorded text stand — and a dispute's text was quoted as the shopper's words when it is PayPal's code; it now leads with "not arrived" and records the code second. A source guard forbids raw category interpolation and enforces the quote-first order.

**What surprised.** The web type's gloss ("none for a fits-well verdict") is wrong and caused the first defect; T4.8 now owns `web/src/lib/api/` and fixes it. The trace transport is a separate piece of work T4.8's original `owns` could not reach, so its line now covers `web/src/lib/why/` and `web/src/routes/belief/`.

**The next agent should not have to work out.** `none` is not a fit verdict; a dispute's reason text is a code, not the shopper's words.

### P7 E2E review: FINDINGS, one Medium opened

**What exists.** The review verified the contract end to end: eleven tools each with schema and both examples, all 33 examples round-trip deep-equal, every object schema closed, the pivot fields required, the docs byte-pinned, the pool and usual-size tables covering their boundaries, and T7.5/T7.6/T7.7's shapes intact. It landed three fixes on `0b071c5`: a test now pins the four required lists T7.4 tightened, `docs/architecture.md` states the identity HMAC construction instead of a sketch, and `pool_lookup`'s definition names all three `unavailable` reasons. P7's first exit box now ticks.

**What surprised.** A pool with no leading direction decides recall: `presence.Pool` is set from the view alone, so a Building pool with four fits-well shoppers and no returns answers "Size down to S … 0 of 4 came back too large, against 0 too small", and a level 2/2 pool takes the same path. product.md says the pool decides only when it says the product runs off-size. Owner: **T8.5r**, with both counts as regressions.

**Findings with owners.** (Medium) directionless pool decides → **T8.5r**. The diary's status paragraph and the T7.6 two-file note were stale → fixed here.

### T8.5j: The harness's seeded statement is marked (done)

**What exists.** `tests/live/precedence/main.go` marks `seed_stmt_001` and the statement step asserts exactly that belief with the mark, so a lost mark fails the driver. The pool seed already marked its ten rows. Proven offline through the real `LoadSeed` and `beliefView` (a flipped mark serves `seeded=false`), and the frozen seed's own statement is already `seeded: true`, so the path is pinned offline too. The README's proof row now includes the assertion. Review's only finding: the live leg needs an owner → **T8.5q** (run the driver once with the mark active).

**What surprised.** The statement row was the only unmarked row the harness built; `LoadSeed` copies the mark rather than forcing it, which is right for a loader.

**The next agent should not have to work out.** T8.5q is the live proof; the harness itself is honest offline.

### T8.5l: Feedback is idempotent per purchase (done)

**What exists.** A delivery for a purchase that already carries a recorded feedback takes the repeat path: no new `fb_` id, no evidence append, no graph write, no status move, and the answer is the feedback on record (id, verdict, belief, counts). Seeded feedback rows are marked by `LoadSeed`, and `SeedFromExamples` fills the seeded feedback's id from the frozen request/response pair (`fb_51c2d8`), so the documented history is not double-counted: `blf_2b9d60` stays Claimed with one `ev_51c2d8`. The repeat still settles the purchase-keyed fit row, so the demo's pool counts (22 → 23) are unchanged; the review added a test pinning that. Review also fixed a schema break (a hand-built seed answering `feedback_id: ""`) and made the repeat check precede the verdict computation.

**What surprised.** A repeat with different words is answered with the recorded verdict and silently drops the new words; an `unclear` first delivery marks no row, so every resend mints a fresh id. Both are now owned: **T8.5o**.

**The next agent should not have to work out.** Do not add a duplicate field to `RecordFeedbackResponse` without the orchestrator: it is a frozen shared type and its example already answers `accepted: true` for a seeded repeat. `tests/live/layer`'s feedback step is ungated (T8.5i owns it). T8.5p owns the `seeded: false` live-origin ledger rows.

### P5 E2E review: APPROVE, one Medium re-run opened

**What exists.** The P5 review re-derived every published count from `bench/runs/scores.json`, regraded all three rounds from copies (45 + 12 + 27 rows identical), and re-derived **111 prompt strings** against the frozen harness with **0 mismatches** (one instrument, arm names consistent). The freeze bites on a tampered file; the grader's rules are pinned by two mutation-checked tests. It landed four fixes on `b3a7374`: the hardened set is now published with its counts (the one set where arm 2 stops being perfect and arm 3 stops being zero was missing and wrongly called "not built"), the overruled contamination hit is recorded, the pilot's `deployment` label and cite counts are corrected, and t59's prompt-tail comment is accurate. All five P5 exit boxes tick.

**What surprised.** T5.8's note gives the wrong cause for two multi-brand refusals: the files show an empty belief list, and a Candidate belief should have been served; the runs carry no settle note, so the cause is unrecoverable. T5.5 overwrote one $0.0018 shakedown turn against the freeze's own "never deleted" rule — disclosed and accepted as a recorded deviation. T5.7 edited the freeze check and the grader test outside its owns — verified and recorded as an approved owns extension.

**Findings with owners.** (Medium) the two multi-brand cells need a re-run with a settle note → **T5.8b**. (Low) T5.5's overwritten shakedown turn → accepted deviation, recorded. (Low) T5.7's owns extension → approved in its notes.

### T9.7: The pivoted demo (done)

**What exists.** `demo/` now runs product.md's steps 0–7 against the hosted services: `reset.sh` restores the demo line (pool 22/22/0; opt-out removes only that shopper's own live rows, corrections are append-only), `run.py` drives the layer reads/writes, both agent deployments and both apps, and `RUN-6.md` plus `evidence/run-6/` are the recorded run with per-step labels (live, live read of seeded/existing data, or fixture replay). Spend $0.69 of $1.00; the five traced turns cost $0.2377 and every trace behind the task totals $0.634.

**What surprised.** The demo is only partly live, and the record says so: a new shopper is 401 (the agent key binds one shopper), a new purchase or dispute cannot ingest (the served layer builds ingest with the recording payment source and the PayPal subscription is not the layer), the ledger page is a fixture replay, and the reference agent has no product-search tool. Two real layer bugs surfaced: repeat feedback multiplies a purchase in the belief and can promote it, and a restart lets a new correction reuse a restored id (and `explain_belief` lists duplicates).

**The next agent should not have to work out.** Every gap names its owner: T9.15 (second shopper key), T8.7b (live payment events), T4.8 (live ledger), T9.17 (catalog search), T8.5l (feedback idempotency), T8.5m (correction ids and duplicate listing), T9.9 (agent surface/retry), T9.11 (org id), T4.6 (belief page). Read replies from `ast agent trace --json` (`output.text`), and poll briefly after a trace appears.

### T4.5: The contest route (done)

**What exists.** `/ledger` appends the why and contest columns; clicking Contest opens `ContestEditor` for that belief with `formId=contest-<belief id>`, submits through `record_correction` only, and copies the layer's answer back into the grid, so the row becomes Out of true with the correction beside it while its value, evidence count and seeded mark stay. Every row links to `/belief?belief=<id>`, and `/belief` resolves the link, keeping its back link. Review mutation-checked the click, the citation and the answer-apply path; it also found the deep-link wiring had no test (vitest resolves `$app/environment` to the server build, so the whole path could be deleted green) and added one that mounts the real page with a browser URL. P4's fourth exit box now ticks.

**What surprised.** The correction fixture answers only `blf_8f2c41`; contesting another row fails honestly rather than widening a fixture. Under vitest, any `page.url` read must sit behind `browser`.

**The next agent should not have to work out.** The Why href is a literal `/belief` (base path unset); if the app gains a base path, the renderer needs `resolve` too.

### T10.3: Round-2 freeze (done)

**What exists.** Eight frozen round-2 shoppers (two per case), hidden answers apart, a 121-row population, 99 arm-2 PayPal records, the four arms with separation statements, the reference reasoner and hold table, phase-aware expected recall (the first turn precedes the correction), and a 116-row freeze manifest the gate verifies. Review found and fixed the blockers: runs can now land (`freeze_check.sh --allow-runs` plus a `runs/FREEZE.json` marker bound to the manifest hash, so a run directory without a marker is still a freeze break), case 4's first-turn prompts no longer carry the correction, the frozen `Missing` text is enforced, and repeats drop from three to two — the P9 mean puts three at 168 turns ≈ $5.66 against the $5.00 cap, two at 112 ≈ $3.77.

**What surprised.** The marker's commit id is shape-checked rather than looked up, because CI is a shallow clone; the manifest hash is the binding. Case 4's first turn would have been answered after the correction it must precede if the prompts rendered the post-correction recall.

**The next agent should not have to work out.** T10.4's text now says two repeats. T10.6 owns the size-ladder mirror between the reference reasoner and the layer. Running a cell first and recording its cost is still the guard.

### T1.1b: Not-as-described disputes (done)

**What exists.** `MERCHANDISE_OR_SERVICE_NOT_AS_DESCRIBED` files under `damaged_or_defective` (the goods arrived and were wrong; a new category would have moved the frozen reason-counts shape, mirrored in both frontends). Served ingest keeps `CategoryForDispute` strict but catches its sentinel: an unknown code stores the dispute with category `none` and the raw code as text, writes no belief, and a redelivery answers Duplicate. A code-less dispute stores with a nil reason, which the frozen schema requires. Both table tests fail if the frozen table grows past them. Review: F1, an unknown-code dispute reaches `ledger_evidence` with an empty `write_op` (schema-forbidden) → **T8.5n**; F2, the fitindex comment's not-arrival-only rationale → **T8.5k** (widened); F3, a code-less dispute is audit-only → **T8.5n**; the live driver still aborts on unknown codes → **T1.1c**.

### T9.13: Bun in CI (done)

**What exists.** CI now runs `setup-bun` pinned to 1.3.14, caches bun's package downloads keyed on `agent/frontend/bun.lock`, and runs `bun install --frozen-lockfile` before the gate, so the shopper-page typecheck is the full package there; a workstation without bun still takes the gate's fallback. Proved locally both ways; the Actions run itself is unprovable here and lands with the push. Review found the bun pin has no second source (pincheck ignores it; the `oven/bun:1` images float) → **T9.16**, and the gate's comment still says CI installs neither bun nor the modules → **T9.16**.

### T7.7: The round-1 wire shape (done, P7 closed)

**What exists.** Round 1 stays frozen history: the drivers are untouched, and the reason is recorded in `bench/README.md` (published output every count is read from; changing commands revises future runs, never history; no gate or freeze row covers a run file), at `renderMemoryBlock` in all three drivers, and in the T5.8/T5.9 notes. `PHASE-2-layer.md`'s stale encode-boundary sentence now states the walk as what T2.5 built and names T7.6 as its removal.

**What surprised.** Four committed round-1 files carry a subject, not two: the t58 noisy pair records it inside prompt/arm_memory_response, and the t59 pair also writes it structurally in the arm-3 pre/post recall blocks. The uppercase grade block has a different cause and does not move on a re-run.

**The next agent should not have to work out.** A re-run's only shape difference is the subject key casing; the grade block stays uppercase because the grader's own types are untagged.

### T9.12: The public shopper page is guarded (done)

**What exists.** The relay's body validator, 30-turn budget and validate-then-charge decision live in `agent/frontend/server/relay.ts` with 15 `node:test` cases; the T3.8 result is committed as `agent/frontend/testdata/t38-public-page.txt`, a reassembly with per-line provenance (the raw output was never captured); the gate runs the relay tests and type-checks the package where bun and its modules exist, else `server/relay.ts`. Review re-ran the three mutations and widened one: charging a turn before validating failed only one case, so a malformed body now has its own spent-budget case.

**What surprised.** `node`'s ESM resolver needs the `.ts` extension where bun and `tsc` take `.js`, so the test imports `./relay.ts` and the tsconfig allows it. CI carries no bun, so the full-package typecheck is workstation-only until T9.13.

**The next agent should not have to work out.** Keep `relay.ts` import-free: one import takes both the node test run and the fallback typecheck away. T9.14 owns the status-mapping test that needs bun.

### P4 E2E review: FINDINGS, fixes landed

**What exists.** The P4 review verified the ledger's data layer, table, why view and contest machinery, mutation-checked the contest's `against` citation and the no-leak rule, and landed fixes on `33dd9e8`: the four P4 Svelte files' `//` headers were visible page text (the build contained them), so they moved inside `<script>` and new guards cover every web and merchant component; every grid step now renders in both themes; the column set, row key set, rendered text and transaction links are pinned against shopper ids; and the data-layer test header no longer claims an assignment it does not make. Web is 163 tests, merchant 47. All gate checks pass.

**What surprised.** The High finding: the contest path has no route. `/ledger` is read-only, `/belief` renders only the standing editor, `contestEditColumnDefs` is never used and the built ledger contains no "Contest" string — so a shopper cannot contest a belief in the app, and T4.4's exit criterion is unmet. The pivot's pooling and standing fixtures also answer only the demo shopper while the page builds them per case, so `shopper_02` sees `shopper_01`'s pooling result.

**Findings with owners.** (High) no contest route → **T4.5** (also owns the ledger→belief link and T9.2's stale task line). (Medium) belief page mixes shoppers on pooling and standing correction → **T4.6**. (Low) raw reason categories and a "live mode" promise in the why cards → **T4.7**. Exit criteria 1–3 tick; 4 stays unticked until T4.5 lands. R4's findings are adopted by T4.5–T4.7.

### T2.2d: Live reasons driver on thirteen phrases (done)

**What exists.** `tests/live/reasons/` pins thirteen recordings: `expectedPhrases = 13`, `expectedTotalCost = 0.000278040` (the sum of the thirteen fixtures' own usage costs; 6,620 input tokens), and a `-check` mode that refuses a wrong set size, a pin that disagrees with the summed costs, or a pin over the derived bound (10 × the per-call reference × the call count). `run.sh` runs it before requiring the key, so a stale set cannot spend. Review re-derived every number from the fixtures and mutation-checked five refusals.

**What surprised.** The review found the budget prose claimed a cap the post-hoc guard cannot deliver (the over-guard reply is billed before the guard reads it). Fixed to call the figure a ceiling the recorded replay fits under.

**The next agent should not have to work out.** `-check` is the zero-cost way to prove set, pin and guard agree; extend it rather than hand-counting fixtures.

### T7.6: Subject JSON tags (done)

**What exists.** `domain.Subject` carries `json:"kind"`, `json:"id"`, `json:"name"`; both rename workarounds are deleted (the server's encode walk and the fakes' normalization), and `TestExamplesMarshalBackToSchema` now validates marshaled output with no forgiveness. `TestSubjectSerializesAsTheContract` pins the three keys. No fixture changed: every `testdata/api` example marshals back deep-equal to its file, verified with a throwaway check over all 33. Mutation: stripping the tags fails the domain pin, seven contract examples, the served-examples test and the replay comparison.

**What surprised.** A third, undeclared reliance existed: `bench/runs/cmd/t55|t58|t59` marshal contract values directly, so four committed round-1 files (the t58 noisy pair and the t59 noisy pair) carry `Kind`/`ID`/`Name`. They are not in the freeze manifest and nothing reads them, so nothing broke; T7.7 owns the decision.

**The next agent should not have to work out.** Decoding was never broken (encoding/json matches field names case-insensitively), so old documents still load. T7.7 must state the decision where the commands are documented and may amend `PHASE-2-layer.md`'s now-false encode-boundary sentence (its `owns` covers both).

### P3 E2E review: FINDINGS, fixes landed

**What exists.** The P3 review verified all eight tasks from code, tests and records, mutation-checked every money-safety guard (approve-first on capture and webhook, double-buy, request-id idempotency, webhook idempotency, strict yes, catalog fit matching), and landed ten fixes on `6c5d31d`: correct test counts and stale claims in the phase notes, malformed YAML fences, the demo README's three-turn record, a previously unread Channel3 fixture now replayed, a dead test wrapper removed, and the paging limitation stated. All six P3 exit boxes now tick.

**What surprised.** T3.4's live-loop evidence was fake-backed: at its close the deployed agent served a fake memory, so `corr_live01` and the "accepted" writes came from the fake, not the layer. The task was marked done on a clause it could not meet. T9.7's pivoted live run is now the substantive recording. Also: `source tools/env.sh` makes the gate red, because the offline tests' key guard fires on the exported keys (now stated in AGENTS.md).

**Findings with owners.** (Medium) T3.4's fake-backed live evidence → **T9.7**. (Low) `recall_first` has no committed fail evidence and no prompt pin → **T10.3**. (Low) T3.8 has no transcript and the frontend has no gate typecheck → **T9.12**. (Low) the Astropods org id is committed in both drivers → **T9.11**. (Low) the README's T3.7 heading and T3.4 test count were wrong → fixed. (Low) R3's F4 has no owner → closed against the P9 decision, with the org-id scrub owned by T9.11. (Low) T6.4's narration must not present the agent as buying → recorded in T6.4's task text.

### T2.2c: The story's third classifier answer (done)

**What exists.** The labelled Jev set is thirteen recordings. `12-…sleeves-too-short-in-m.json` is the frozen third answer (`fit_too_small`, 1.00), and `13-…tight-across-chest.json` was needed because the replay's second return answer was unrecorded too (the old tests pinned it as an intentional missing-recording probe). `fakes.Run` now classifies every documented return through the seam and fails unless it is a clear fit reason, so no category is written into the replay; `TestReplayOffline` counts each answer exactly once, and the story test answers its third return with the new recording. Spend: two calls, $0.000042756. Review findings: the live reasons driver still pins eleven and would refuse the set (owner **T2.2d**, opened), and the notes' "$0.10 budget" claim was corrected — no brief stated one.

**What surprised.** The thirteen-file set breaks `tests/live/reasons` before any call, because its guard reads and rejects any set not exactly eleven.

**The next agent should not have to work out.** `internal/reasons/doc.go`'s "eleven-phrase labelled set" is the T0.7 floor measurement and stays; the current set is thirteen.

### T9.3i: Toolchain pins checked mechanically (done)

**What exists.** `tools/pincheck` reads every place Go and Node pins live — both images' build stages, CI's `setup-go`/`setup-node`, `go.mod`, and the README table — and fails naming the drifting file, its line and both versions; a source that lost its pin fails instead of shortening the list. `tools/check.sh` runs it before gofmt, so CI gets it for free. `TestCheckTable` (14 cases) proves each drift and missing-pin path, and the current tree prints `go 1.27 and node 26 agree across 9 declarations`. The README no longer claims the table is the only guard, and its one intended asymmetry (the Go tag floats the patch) is stated.

**What surprised.** The guard's source list is literal: a third image or a second workflow pinning another Node would pass. Tightening that needs a decision about single-toolchain images, so it is recorded as a limit.

**The next agent should not have to work out.** When an image or workflow joins the repo, add it to `tools/pincheck`'s source list or the guard silently ignores it.

### T8.5h: The statement rung (done)

**What exists.** `docs/interface.md` now says rung 4 is fed only by the seed, gives the reason (a statement is a Claimed label, never evidence, so nothing may promote it), and says what a `record_statement` tool would really cost: a frozen tool name, its two schemas, a handler on the existing `/tools/{tool}` route, and the auth `toolAccess` table plus both agent frontends — which is why the option was not taken. The limit appears at the rung-4 row, the turn pattern, the write-signal table and the new-shopper example. One fixture corrected: the recall example's statement belief now carries `seeded: true`, matching the ledger example and the seed path; the doc block stays byte-identical.

**What surprised.** The seed marks are copied, not forced, so a hand-built seed can yield an unmarked seeded row; the live precedence harness does exactly that. Review also found the doc implied a builder could seed a statement on demand, when seeding is a boot-time fixture change for one shopper.

**The next agent should not have to work out.** T8.5j owns the harness's unmarked row. Adding the tool remains a contract change spanning `internal/api/contract`, `internal/api/server`, `testdata/api`, `internal/auth` and both agent frontends.

### P2 E2E review: FINDINGS, fixes landed

**What exists.** The P2 review verified all twelve landed tasks from code, recordings and tests, re-derived the T2.2b/T2.3b/T2.4b numbers from committed evidence, and landed seven fixes on `4aa80d3`: the reasons driver's per-call spend guard now runs on every call and its model line is no longer tautological; the deployed-route replay assertion (same delivery twice → one question); the dispute test fails when the frozen code table grows; the memory README and the layer driver/README/phase notes no longer overstate what "leg 1 proves"; and T2.3's note records the built store shape. All six P2 exit boxes now tick, with the third reworded (raw evidence lives outside the graph, in the layer's own store — not Elastic). Mutation checks covered the write mapping (rule 2), both idempotency guards, the direction table, the rival threshold and the classifier's ask-again path.

**What surprised.** The Medium finding: recall withholds a one-return Claimed fit belief, though product.md says thin evidence comes back labelled thin and status is never a gate. The T8.5 handoff had assigned it to T7.4, which could not reach the package and never recorded it, so it had become ownerless; it is now **T8.5i**, and round 2's arm 4 depends on the fix.

**Findings with owners.** (Medium) recall withholds thin evidence → **T8.5i**. (Low) the frozen story's third answer has no classifier recording → **T2.2c**. (Low) two T2.5 items have no owner (redundant Keel import; seller-subject derivation) → **T2.5b**. (Low) the comparison arms mirror the direction rule but not the rival-birth threshold → recorded in T10.3's brief. (Low) the b-tasks' transcripts live in worker reports, not the handoff log → the phase intro now says so and the reviewed numbers are re-derived from committed evidence.

### T9.8: Agents read the served layer (done)

**What exists.** Both deployed agents now call the live layer through its edge (`POST https://memento-emptor.onrender.com/tools/{tool}` with an agent key), with the fake memory/store deleted. The key is declared as an `agent.inputs` secret in both blueprints and resolved at deploy from the account vault. Live proof: reference trace `ed617f32e79fb8d20fde1e1174bc1ef0` — `recall_for_purchase` then `record_correction`, minting `corr_000001`; second-agent trace `6d764a788aba072bcaabb043e2551448` — asked only for a size, recalls first, recommends M from `corr_000001` over `blf_8f2c41` with the 22-settled seeded pool overridden. The second agent's prompt cannot produce that text; it can only come from the shared layer. Spend $0.14 of $2.00. Review APPROVE with one prose fix (the reference agent's tool descriptions are the layer's words plus its own guidance, not verbatim).

**What surprised.** A `--var` against an undeclared `agent.inputs` entry is accepted and silently drops the secret; only a declared input is validated. The first attempt failed honestly because of it (the agent said its key was not set). Two livability notes: messaging history can return an empty assistant message (read replies from the trace), and `ast secrets create` accepts values only via argv.

**Key-handling incident, accepted by the user.** A worker's `secret-tool search --all` printed all three token values into its local session transcript (never into a repo file, commit or application log). The user chose to accept the local exposure rather than rotate; the incident is recorded here so the next agent knows the transcript is sensitive if it is ever shared. SC4 name: T9.9 covers the three low findings (dead ledger tools, no tool-binding coverage, retry-on-write).

### P2 E2E review: FINDINGS (fixes landed, owners opened)

### P1 E2E review: FINDINGS, fixes landed

**What exists.** The P1 review verified all five tasks against the tree and landed eight fixes on `cc5625b`: four builder-doc contradictions in `docs/interface.md` (a table row saying a correction promotes — it never does; a claim that reads take no trace id; a missing `set_pooling` section; a wrong standing-correction description) and one in `docs/architecture.md`; three gate gaps (no serialize-direction contract check, no per-tool example check, the seed golden test bypassed `Write`'s file layout); a scoped `KeyVariables` comment; and T1.2's notes moved after its Done-when with a supersession line. The reviewer also re-derived the pivot pool fixtures from the population (Claimed 2 settled 1/1, Building 8 with recent 5, Earned 63.2% share — all exact) and mutation-checked the write table, pool thresholds, doc pin, fixture drift, key pattern and seeding. All five P1 exit boxes now tick.

**What surprised.** Two spec gaps: `product.md` names not-as-described disputes while the frozen table refuses that code and ingest fails the whole delivery on it (owner **T1.1b**); and `domain.Subject` has no JSON tags, so marshaling emits `Kind`/`ID`/`Name` that every schema rejects, with the wire shape kept correct by two rename workarounds (owner **T7.6**). A third: no tool records a general statement, so precedence rung 4 is unreachable for builders (owner **T8.5h**). None blocks the demo.

**The next agent should not have to work out.** The new serialize-direction test (`TestExamplesMarshalBackToSchema`) is the guard that would have caught the Subject tags; keep it. The replay's final ledger is a literal mirroring the frozen snapshots, not a story-derived state — its coherence is by construction, so it cannot catch a story/ledger inconsistency beyond the pinned waypoints.

### T8.1d: Retire the recall-tier claim (done, P8 closed)

**What exists.** `product.md`, `docs/architecture.md`, `dev-diary/research/README.md` and `dev-diary/PHASE-0-ground.md` now state the hub's store-only decision with its reason, or mark the passage historical in sequence; no product doc asserts the tier as the deployed shape. The same task fixed a second false claim: Elastic does **not** store the layer's raw evidence. The served layer keeps each shopper's evidence texts in its own process store (no vectors), beliefs live in the hub, and Elastic holds only the pooled `fit-rows` index. The consequence is documented: after a restart the pool and a restored correction still decide, belief-backed steps return as writes replay, and the seeded story's evidence rows return at boot through `LoadSeed` (only a live event's row is lost until T2.3c decides persistence).

**What surprised.** The review found four more wrong sentences: T0.11 said "as the deploy ran it at the time" though the deploy never ran the tier; both product docs described the store as holding messages and catalog text when the served path indexes one text per classified outcome; the post-restart panel was overstated as fully empty when the seed re-indexes at boot; and an architecture dependency row's title contradicted its own text.

**The next agent should not have to work out.** P8 is 18/18. T2.3c owns evidence durability and the `internal/evidence` comment that still mentions vectors; its `owns` includes that package and both product docs.

### T9.3h: Merchant image Node pin (done)

**What exists.** `deploy/merchant/Dockerfile`'s `web-build` stage is `node:26-bookworm`, matching the demo image and CI; `deploy/README.md`'s toolchain table covers both images and the T9.3g "known drift" paragraph is gone. Proved local: `--no-cache --target web-build` answers `v26.13.0` inside the stage; the full image serves `/healthz` and `/` (Memento Merchant), 404s other `/tools/` paths, and answers `product_fit` 502 `layer-unreachable` with a key against a dead loopback layer (503 `key_not_configured` without a key). Gate green.

**What surprised.** The review caught three false claims in the notes and one in the README: "503" where the edge answers 502 with a key set (503 is only the missing-key case), a Go patch pin where the tag floats the latest 1.27 patch, and "as the gate" for `npm ci` — the gate runs `npm ci --dry-run`, only CI installs. All corrected.

**The next agent should not have to work out.** The pin now lives in two `FROM` lines; T9.3i is the task that makes a disagreement fail mechanically.

### P0 remediation: T8.5f, T8.1c, T9.3g (done)

**What exists.** Three findings from the P0 review are fixed and landed. **T8.5f**: the hub client treats HTTP 503 as the attach bound (`*attachBoundError` wrapping the `errAttachBound` sentinel, `Retry-After` parsed as whole seconds with a 1s fallback for absent/date/fraction/negative, retry after that delay at most 3 attempts, ctx-bounded, typed error when it persists; 500 keeps the old single re-open retry). Offline tests replay `testdata/lambo/{01,02,03}` through the real client over `httptest` and pin receipts (string) vs stats receipt (object), applied, admission status/checks, blast radius, warning text, ranked recall labels, empty recall and no-match → None. The task also fixed a real defect: `Inspect`'s no-match branches returned the zero `GraphState` (`""`), which `StatusForGraph` rejects, so an unpromoted belief reached the layer as an invalid state; both now return `domain.GraphNone`. **T8.1c**: store-only is the recorded, deliberate hub recall-tier decision, with the reasoning that the served read path never asks the hub for vector search (the one real vector use, a hybrid derive, is answered exactly in process), so an approximate lagging mirror would be no better even free; the config-only path, its Elastic ingest cost and the separate `fit-rows` pool are documented, `research/elastic.md` corrected, and the local-run recipe's required 64-hex salt fixed. **T9.3g**: the demo image's `web-build` stage is `node:26-bookworm` matching CI, proved by `node --version` inside the stage (`v26.13.0`) and a full image serving `/healthz` and `/`; `deploy/README.md` gained a toolchain-pins table.

**What surprised.** The review found two vacuous assertions worth remembering: `Retry-After: 1.5` truncated to 1s equal to the fallback, so a fraction-accepting implementation passed (now pinned with `2.5`); and the inspect state test could not tell the structured status from its text fallback (now pinned with a reply whose two sources disagree). Both tasks' happy paths were pinned unchanged (one open, one call).

**Findings with owners.** (F1, Low) present-tense "Elastic is the graph's/Lambo's recall tier" claims remain in `product.md`, `docs/architecture.md`, `research/README.md` and T0.11 → **T8.1d**. (F1, Low) no CI step compares image toolchain tags with `ci.yml`, so the pin table is documentation only → **T9.3i**. (F2, Low) T9.3h's `requires` now includes T9.3g. All fixes are in the tasks; every finding has an owner.

### P0 E2E review: FINDINGS, remediation opened

**What exists.** The P0 review verified all thirteen tasks against committed evidence and landed the small fixes on `0500d9f`: PayPal recordings scrubbed of shopper country, street address, given/surname and three real `debug_id`s (two resend files also normalised to JSON); research-doc corrections (Puma size list, Elastic document inventory, Astropods spend reconciliation, hosting's docker runtime, the `states.json` aliasing quirk plus a harness deep-copy, the 37-scope grant's location, the `inspect` warnings key, the per-run store path); `research/README.md` gained its three missing rows; `.gitignore` ignores the AG Grid spike build; `product.md`'s Elastic-trial bullet is corrected to the Vector Database licence. The reviewer re-derived the Lambo GC epochs, the fit-index truth table, the latency medians and the Jev cost from the raw recordings; mutation-checked three replay tests (all bite); and confirmed the gate/CI installs now agree. Gate green.

**What surprised.** The PayPal recordings carried real shopper detail beyond the placeholders the doc listed (country, street, given/surname, debug ids) — all redacted now. The `states.json` step bodies are the purchase's final row because the harness recorded the dict it later mutated; the per-step truth is `reads[].get._source`. And the Elastic "trial" was never a trial.

**Findings with owners.** (S1, Medium) the hub client ignores the attach-bound 503/`Retry-After` → **T8.5f**, which also folds S2 (offline MCP shape tests over `testdata/lambo/`). (S3, Low) `start.sh` writes no `[recall]`, so the hub is store-only while two docs claim the Elastic tier → **T8.1c** (configure or record the decision). (S4, Low) the image builds `web/` on Node 22 against the Node 26 pin → **T9.3g** (landed; its own carry-over for the merchant image is **T9.3h**). (S5, Medium) the sandbox shopper email is in git history across 93 commits (tree clean, repo private). Closed by user decision: the sandbox shopper address is not a secret, so it stays and no history rewrite is done.

### T8.5e: Flip the re-link tripwires live (done, P8 closed)

**What exists.** The T8.5b driver's two defect pins are flipped to pin T8.5d's fixes and pass live: a consulted-but-empty line serves the frozen `example-none` shape (pool null, lookup consulted, guidance none), and a declared correction keeps force across a restart and re-links after a deliberately divergent rebuild (`blf_000001` → `blf_000002`) with no `ErrOverrideMissing`. Seven pre-existing precedence steps keep their old assertions and pass. Review APPROVE with four fixes, including a real one: a refused run (non-empty `t85b-` prefix) still deleted every index under the prefix in its deferred cleanup — the guard's promise did not hold; cleanup now skips the pre-run listing and a refusal fails loudly naming the survivor. Spend $0.00 of the $0.20 budget; indices erased, listing `[]`.

**What surprised.** The cached 0.10.1 binary is byte-identical to the release asset's published checksum (the same file the Dockerfile verifies), so the offline caveat is resolved. Ignoring the persisted pairing still passes this single-subject rebuild because the journal entry resolves it; the pairing's cross-subject job remains `internal/api/server/pivot_test.go`'s proof, and the driver README no longer claims otherwise.

**The next agent should not have to work out.** P8 is 15/15. The next program is the E2E review/remediation loop, one phase at a time, starting at P0.

### CI: merchant dependencies (fixed)

**What happened.** The first pushes after merchant joined the gate (`ff9c922`, `7d4719d`) went red in CI with `FAIL [merchant deps] merchant/node_modules is missing`. The gate itself was fine — CI's workflow only ran `npm ci` in `web/`, and T9.3 added merchant to `tools/check.sh` without touching `.github/workflows/ci.yml`. Fixed as `88292f2`: the workflow installs merchant dependencies and caches both lockfiles; the run after it is green.

**The next agent should not have to work out.** Whenever `tools/check.sh` gains a frontend or a build step, the CI workflow gains its install in the same change. The local gate passing does not prove CI: run a push or diff the workflow's installs against the gate's `check_frontend` calls. This class of failure reads as a code failure but is a missing install.

### T9.6: The pivoted layer live on Render (done)

**What exists.** `main` is pushed (`ff9c922`) and both Render services redeployed `live` from it. The public layer now serves the pivot end to end: `recall_for_purchase` for `shopper_01` reads `pool_lookup: consulted`, pool Earned small (22 settled, 22 seeded, 0 live) overridden by correction `corr_3b7e52`, guidance basis `correction`; `product_fit` for the demo oxford reads Earned small; the merchant proxy answers the same shape with no caller credential. `deploy/guard.sh --check` green at 2 services / 1 GB disk / $14.25/mo against $16.00. Spend $0.00 of model.

**What surprised.** The merchant proxy 401'd after the first pivoted deploy: T9.3d's service key was minted as `merchant-app` while the layer's `MEMENTO_MERCHANT_KEYS` held `merchant-app-01` from a different mint, so the two never met. Fixed as an operator step — the service key's digest appended to the layer secret, layer redeployed, proxy read green. A keyless 401 on the proxy is that gap, not a code defect.

**The next agent should not have to work out.** Minting a merchant key and registering its digest on the layer's secret are one step, and the proxy read is the check. The old Lambo arc seed stays until the demo says otherwise; its session is orphaned by the live-salt session, not colliding. T9.7 is the last P9 task.

### Stack landing: T7.4, T7.5, T8.5d, T9.2b, T9.3f (done)

**What exists.** The pivot core is now required end to end: contract schemas and Go types require pool, pool_lookup and per-item precedence (with the null conventions pinned); the frozen replay carries the tightened shape; the served layer re-links declared views by persisted view↔entry pairing and compares without stripping; both frontends decode the required fields strictly. Landed as one fast-forward (7 commits: 5 task + 1 review fix + 1 diary reconcile) so `main` never pointed at an intermediate red state. Gate green on the landing branch and re-run on `main`. Reviews were APPROVE across all three (T8.5d, T9.2b, T9.3f); spend $0 throughout.

**What surprised.** Rebasing the stack onto `main` duplicated the T7.5 and T9.3f diary sections (parallel claim/task edits of the same file region) — reconciled on the landing branch: T7.4's worker notes moved back under T7.4, one T7.5 kept, one T9.3f kept with the three transport notes moved to T9.3e. Code merged cleanly; only diary conflicted.

**The next agent should not have to work out.** T8.5e (flip the re-link tripwires live, requires T8.5d) is the next layer task; the live tripwires in `tests/live/precedence/` still pin the old defect shapes. T9.6's public-URL proof and the merchant green deploy still wait on the user's push approval. P7 is 5/5; P8 is 14/15.

### T9.2: Ledger with precedence and the pool (done)

**What exists.** The ledger decodes all five precedence steps from the frozen fixtures (correction with two overridden steps, own with pool overridden, pool with statement overridden, none said plainly, plus a synthesized statement case), renders the decision line with the loser struck through, shows a standing correction ("no belief yet" + date), and renders the pooled belief as counts only (key set pinned; no shopper id, custom id, event id or free text in the render path). The opt-out toggle lands "Pooling is off: 3 existing fit rows removed" from the frozen answer; opt-in removes 0. The pre-pivot correction types now accept the schema-valid standing correction. Svelte 5 runes only; every fixture transport rejects a mismatched shopper. Review APPROVE with no fixes (comment-only notes: the vitest `svelte/reactivity` server/client split means notified Chaaya state doesn't invalidate templates in tests, so the toggle mirrors answers into local `$state` — the browser build is unaffected).

**What surprised.** `PoolView` carries no identity by construction, and the seeded split renders ("48 of the 60 settled purchases are seeded history"). The ledger's pool decoder and the merchant app's pool client both decode pool counts — deliberate duplication, since the merchant app must not import shopper-side code.

**The next agent should not have to work out.** Guidance bases are exactly the contract's five steps. `decodePoolPrecedence` omits the key when the fixture has none, staying byte-equal per `omitempty`. T9.6 must send the demo shopper's link token as a bearer header on ledger reads and contest calls.

### T9.3e: Merchant page reads live (done, live vintage waits on deploy)

**What exists.** `merchant/src/lib/pool/source.ts` loads live-first with whole-grid snapshot fallback (structurally never half-live) and names the vintage in the UI; `createSameOriginTransport` POSTs `/tools/product_fit` with no credential. Fixture transport untouched for tests. Proved against a local edge + served layer: all six lines read the T9.1 verdicts with seeded-only counts (demo oxford settled 22 / seeded 22 / live 0 — distinct from the bundled 60/48 snapshot). Review APPROVE with no fixes.

**What surprised.** Nothing structural. The built bundle carries `/tools/product_fit` and no `authorization` string.

**The next agent should not have to work out.** The live vintage needs T9.6's merchant-key entry plus a merchant-service redeploy (no edge/layer change). After any transport change, grep the page chunk for the endpoint and the absence of `authorization`.

### T9.3d: Merchant app deployed (done, verify after push)

**What exists.** `deploy/merchant/` provisions `memento-merchant` (starter, virginia, docker, no disk) following the T6.1 pattern: `provision.sh`, `render.yaml`, edge `main.go` (injects the merchant key server-side, drops caller auth), `guard.sh` with settled/over-limit snapshots, README. Service is live in shape; first deploy is `build_failed` because Render builds `main`, which gains the Dockerfile only on land — the green deploy auto-follows the push via `autoDeployTrigger: commit` (T6.1 precedent). Guard live: 2 services, 1 GB disk, run-rate $14.25/mo vs $15.00 ceiling, exit 0. Cost: **$7.00/mo** new ($14.25/mo combined vs $50 credit ≈ 3.5 months runway). Review APPROVE with two fixes (static-dir env name, transient API response unset).

**What surprised.** Render's env-vars API echoes secret values on write/read — the worker rotated once after an in-transit echo (dead token, no file copy). Treat API/dashboard output as sensitive. The merchant build imports repo-root `testdata/api`, so the image's web stage copies it.

**The next agent should not have to work out.** T9.6 appends the merchant key entry (digest in the T9.3d notes) to the layer's `MEMENTO_MERCHANT_KEYS` and raises `deploy/guard.sh`'s ceiling to the combined shape. **T9.3e** (new) wires the page from the fixture transport to same-origin `/tools/product_fit`. Post-push verify: `curl https://memento-merchant.onrender.com/healthz` → 200, then `/` → 200 HTML, then the layer entry makes live reads return the pool. Until then the proxy answers 502/503 and the app reads its bundled snapshot.

### T9.6b: Image wiring (done, public proof waits on push)

**What exists.** `cmd/memento/main.go` is a per-shopper director (one `Service` per hub session, default shopper-less service); `deploy/start.sh` boots hub → reader → layer → seed-fit-only, pivoting on `MEMENTO_IDENTITY_SALT`; `deploy/Dockerfile` ships the population fixture. Single `fit-rows` seeding via the disk marker, honoring the staged population. Review APPROVE with no fixes.

**What surprised.** One service cannot serve two hub sessions (no shopper on the graph seam), so the director mirrors the T8.5b driver. The web stage now copies `testdata/api` (the ledger fixture needs it). The marker shares the staged seeder's path, so the live `fit-rows` is honoured, never reseeded.

**The next agent should not have to work out.** `MEMENTO_MERCHANT_KEYS` still needs the merchant entry appended on the live service (digest in the T9.3d notes). Post-push check: both reads through the edge, then `bash deploy/guard.sh --check`. The arc retires with the replay (orphaned, not colliding). T9.6 closes on the public-URL proof.

### T9.6: Staged deploy work (in-review, T9.6b owns the live proof)

**What exists.** `deploy/seed/` seeds the T9.1 population plus the demo graph, each under its own disk marker (`asOf` at the seeding moment; `Refunded` then `Classified`; skips on `MEMENTO_FIT_SEED=0` or fixture embedder; refuses the marker unless the pool reader returns T9.1's table). Auth plumbing: salt + three caller-key vars as `sync: false` in `render.yaml`, keyring rows in `tools/env.sh` (service `memento-keys`), names in `.env.example`; keys minted with `memento key issue`. Guard ceiling raised $8→$16 for the two-service shape ($14.25/mo run-rate, green live). Review APPROVE; the one finding (missing T9.6b task) was stale — T9.6b exists.

**What surprised.** Population seed at scale: 187 forced-refresh writes in ~96 s (~516 ms/write). Starter-shape flags measured in a 0.5 CPU/512 MB container: 128 cap engages (116+14 refused), 200rps limiter binds before CPU. Nine vars are set on the live service and stage for the next deploy. The old Lambo arc seed stays until the pivoted image lands (name collides with the hub's future shopper session).

**The next agent should not have to work out.** T9.6b wires the image and proves the public URL. Keep the arc seed until then.

### T9.5: Second agent (done)

**What exists.** `agent/second/` is a second Astropods deployment (`memento-second`, own name/prompt/traces): fresh prompt with zero shared 5-grams vs the reference instructions, tool descriptions byte-matching the frozen contract, own stand-in store with the same ids. Live proof: reference trace (record_correction → M) and second trace (recall → first recommendation M from `corr_3b7e52`, citing rung, Out-of-true, counts). Spend $0.13 of $0.50. Review APPROVE with no fixes.

**What surprised.** Both deployments still answer from frozen stand-in state — the proof is structural (the prompt cannot produce the correction on its own), not a shared backend. T9.6 must wire both deployments to one served layer and re-drive the pair to upgrade the proof. The proxy is lenient about conversation ids (a mis-addressed send still returned 200). The second agent offers to "lock in" M — approval-shaped, nothing behind it; the demo script must not accept.

**The next agent should not have to work out.** Drive with `DEPLOYMENT=<id> OUT=<dir> python3 agent/second/drive.py` — never bake a deployment id into a script. Push needed an `AGENT.md` beside `astropods.yml`. T9.6's re-drive is load-bearing for the demo claim.

### T9.4: Reference agent, after the pivot (done)

**What exists.** The reference agent reads the precedence step and states its certainty from the label; never refuses when recall returns evidence; says when an answer rests on the pool or a Claimed statement; records a standalone correction when recall served no belief (trace `8238de5b`, `corr_stand_01` with no belief id, re-served next session — closing the T5.9 7-in-9 gap). Recall-first traces on all six live turns; approval-before-purchase true everywhere. Eight-judge eval set (3 kept T3.5 judges + 5 per-basis judges). Spend $0.92 of $1.00. Review APPROVE with one notes fix (F1/F2 judge-strictness findings now owned by T10.3).

**What surprised.** The agent needed two instruction tightenings: recording is its own confirmation (no asking first), and never rely on an earlier turn's recall. The pool reply once paraphrased a count; a quote-don't-paraphrase line fixed it. The `yes`-confirmation turn carries no tool spans (nothing to recommend; purchase honestly not wired).

**The next agent should not have to work out.** Server eval set has diverged from `agent/evals/EVALUATION.yaml` — re-pushing from `agent/evals/` would revert the five new judges; reconcile before reusing. T10.3 extends the evidence clauses for evidence-free recalls and maps applicable judges per trace.

### T9.3: Merchant app (done)

**What exists.** `merchant/` is a separate read-only Svelte 5 + Chaaya + AG Grid app: one pool table (line and size, settled purchases, returns by reason and direction, feedback by verdict, status, seeded count) with a why-panel (counts behind the status, the judged rule). It reads only through the layer's `product_fit` with the merchant key; a test proves no Elastic configuration exists, and the client rejects any non-pool read before the transport. `tools/check.sh` runs a shared frontend step for web then merchant. Review APPROVE with one fix (client-side request validation now covers the HTTP transport too).

**What surprised.** The `Out of true` fixture needs a recent window that is itself judged (≥3 fit shoppers) yet fails the Earned bar — the 3-to-7 recent-shopper band can never earn, so a quiet Earned line reads Out of true. The page loads through the fixture transport; the HTTP transport wiring is T9.3d's config task.

**The next agent should not have to work out.** `toProductFitRequest` strips the display-only `label`; seller lines come from `testdata/api` or the app's own config. Authz still belongs to the layer; the client validation is syntactic. T9.3d wires the HTTP transport (base URL + merchant key, key never in a file).

### T8.1: One Lambo process for every shopper (done)

**What exists.** `deploy/start.sh` writes the T0.11 `[serve]` table into the boot config (one pinned demo session, `max_attached = 16`, credential `memento` with `session_prefix = "shopper-"` and `create = true`) and starts the hub (`--max-sessions 128 --rate-limit-rps 200`) after provision and seed, then the `serve-web` reader from `LAMBO_WEB_SESSIONS`. The demo session constant equals T7.2's name for the demo payer under the seed salt. Review APPROVE: re-ran the two-shopper drive locally (own marker only, no cross-read, 401/404s correct), fixed a double-shutdown that lost Lambo's write-behind tail (idempotent `stop_all` plus a final `wait`), and corrected the product.md line that still claimed one writer per session.

**What surprised.** Lambo's memory is a write-behind log with no on-disk WAL, so signalling the children twice on TERM lost the unflushed tail every stop until the fix. The fixture embedder cannot converge the frozen arc (sweeps coalesce, seed times out), so `LAMBO_SEED=0` is the local loop. `serve-web` serves an absent allowlisted session as an empty window.

**The next agent should not have to work out.** Keep seed-then-hub order; the seeder's stdio writer must own its session alone. Do not pin shoppers (15 attach beside the demo pin). The demo constant and `internal/identity`'s test vector move together. Owners: T8.1b for the unexercised 503 path and the unmeasured rate flags; T9.6 for the seeder fixture-embedder convergence.

### T8.6: Authenticated callers (done)

**What exists.** `internal/auth/` stores hashed, revocable keys for three caller kinds, bound to one seller and its shoppers; `auth.IssueKey` mints them once, `Authenticate` scans every entry in constant time, and the guard refuses an unbound `shopper_id` with the same 401 as an unknown key. `deploy/edge` mounts the guard on every route of `auth.Surface()`; only `/healthz` and the webhook are public, refusals are byte-identical, and client `X-Memento-*`/`Authorization` headers are stripped. Review found and fixed one real defect: the guard asserted `X-Memento-Shopper` from the body even for out-of-scope or unknown tools, which a merchant key could exploit before T8.5 trusted the header.

**What surprised.** The deployment can start with no key variables and stay `/healthz`-green while refusing every API call — fail-closed, but invisible in Render. Nothing invokes `IssueKey` yet. The webhook's only gate is still T2.1's replay fixture; a known event id can write evidence into any shopper's session (pollution, not disclosure). `/calls` is agent-only because the layer returns every entry unscoped. The guard forwards a body it cannot parse; the layer's strict single-object decoder rejects it today.

**The next agent should not have to work out.** T9.6 wires the three `MEMENTO_*` variables into `deploy/render.yaml`, `.env.example` and `tools/env.sh`, mints keys with T8.5's command, and sets them as Render secrets. T8.5 adds the key command, scopes `/calls`, keeps the MCP decoder strict, and retries a partial opt-out. T8.7 replaces the fixture webhook verifier with a real signature check. T9.2 sends the demo shopper's link token as a bearer header.

### T8.7: Verified webhook deliveries (done)

**What exists.** `internal/payments/` verifies deliveries against `POST /v1/notifications/verify-webhook-signature` with the transmission fields, subscription id and exact body bytes; only SUCCESS passes, everything else refuses fail-closed with no credential in the error. `internal/ingest/delivery.go` adds `HandleDelivery`, which verifies before T2.1's idempotency check; tests inject a fake verifier. Recorded live: good→one row + pending, tampered→identical refusal, replayed→duplicate, unknown→identical refusal; a byte-exact delivery re-verifies live. Committed fixture `internal/ingest/testdata/recorded-deliveries.json` replays all four offline. Review APPROVE with one fix (ordering wording; the route gap now names T8.5).

**What surprised.** A redacted recording cannot verify live — the signature covers the exact bytes, so the byte-exact delivery lives in git-ignored `out/` and only shapes/verdicts are committed. The signature is bound to the subscription id, so multi-subscription discovery tries every candidate unless `PUBLIC_URL` narrows it. The `Client` caches the subscription list for process lifetime (T8.5 owns the refresh decision).

**The next agent should not have to work out.** T8.5 wires the deployed route: read the raw body bounded like the other routes, pass `ingest.Delivery{Header, Body}` to the shopper's service, and resolve the shopper via `custom_id`. Until then the deployed path keeps T2.1's fixture gate while every wired caller of `HandleDelivery` is verified for real. Set `PUBLIC_URL` on the service once the Render subscription exists beside the test-endpoint one.

### T8.3b: Pool reader, live (done)

**What exists.** `tests/live/pool/` (driver, `run.sh`, README, git-ignored `out/`) writes 8 seeded rows to `t83b-fit-rows` through the fit writer (5× returned/classified `fit_too_small`, 1× `fit_too_large`, 1× feedback `fits_well`, 1× kept) and reads them back through `pool.Reader.Fit`. Review APPROVE with one fix (cleanup-failure stderr now redacted like every other error path).

**Live transcript** (verbatim from `bash tests/live/pool/run.sh`; index deleted, prefix empty before and after):

```
$ source tools/env.sh
$ bash tests/live/pool/run.sh

T8.3b live pool reader -- index t83b-fit-rows, deployment <deployment>
-- prefix listing before: GET /_cat/indices/t83b-* -> []
-- create: PUT /t83b-fit-rows with the recorded mapping (no settings) -> ok
-- wrote 8 seeded rows: 5 returned+classified fit_too_small, 1 fit_too_large, 1 feedback fits_well, 1 kept
-- _count: 8 docs, 8 settled

== cold read (fresh transport)
Fit: Building/small, settled=8 seeded=8 live=0 recent=8
cold: 6.863 s

== Warm + 3 warm reads
Warm: 6.724 s (Warm itself pays the handshake on a fresh transport)
warm reads: 239 ms / 240 ms / 239 ms, all agreeing with the cold read

== full counts
kept 1, returns small 5 large 1, feedback fits-well 1
reasons {fit_too_small:5, fit_too_large:1}
shoppers {Fit:7, TooSmall:5, TooLarge:1}
seeded 8 / live 0 / recent 8

== empty size XXL
Claimed, zero counts, no error

-- cleanup: DELETE /t83b-fit-rows -> 200 {"acknowledged":true}
-- prefix listing after: GET /_cat/indices/t83b-* -> []

model spend: $0 (rows written directly, no classifier call)
elastic: one kilobyte-scale index, 22 row writes, reads/counts/delete; nothing billable beyond the existing project
PASS T8.3b live pool: seeded pool read warm and cold; index deleted
```

**Latencies.** Cold 6.863 s (fresh transport), Warm 6.724 s (also a handshake), warm reads 239–240 ms. Call `Warm` at boot — the first touch always pays the ~6.9 s handshake, inside the 20 s timeout. The kept row settles the denominator but joins no fit-shopper count (Fit 7 of 8).

### T8.3c: Union per-direction shoppers (done)

**What exists.** `internal/pool/` gains a `direction_shoppers` filters aggregation: one bucket per direction, each a union of fit returns that way and fit feedback with that verdict (one shopper counts once). Resolution (a), no contract change — `domain.PoolCounts` already documents each direction as returns and feedback, and narrowing would have contradicted the rule. The old nested cardinality under `fit_returns/direction` is gone from the query and unread. The fixture pins 36/15 union shoppers (was 31/12 returns-only), with a regression case (10 feedback-only small shoppers → Earned). Review APPROVE with one comment fix.

**What surprised.** Feedback rows carry `fit_direction` set from the verdict, but the union's feedback leg filters on `settled_kind + feedback_verdict` instead, so a future verdict row without a direction still counts. The index's `Shoppers.Fit` (all-settled) and the documented Building floor are different counts; the floor uses the fit-classified cardinality.

**The next agent should not have to work out.** Nothing about the served layer changes shape. T8.5 reads the union buckets as served.

### T8.3: Pool reader (done)

**What exists.** `internal/pool/` reads `product_fit` over settled rows only: the line-and-size aggregation (denominator, returns by direction, feedback by verdict, distinct shoppers by direction, seeded split, recent window over `settled_at`), with the status from `domain.DecidePool` and no second copy of the rule. Keep-alive connection with a 20 s timeout; an unreachable index is `ErrUnavailable` and recall carries on without it. Fixture tests reproduce T0.12's 600-purchase numbers (600 / 120 / 100 settled / 40-15 / 12-5-3 / 31-12, seeded-live split) and land Earned/small exactly at the 60% floor. Review APPROVE with one fix (settled-kind filters now sourced from the domain constants).

**What surprised.** The index's per-direction shopper counts cover fit *returns* only, so feedback-only direction evidence never meets the eight-shopper floor: 10 shoppers with only `too_small` feedback read Building where the rule says Earned. That is T8.3c (new): union the counts or narrow the rule — the latter is a contract change and the orchestrator's call. The T0.12 recording predates the `fit_shoppers` filter, so the fixture test derives 63/55/8 from the recorded counts. The index predates `settled_at`-window reads; the recent window is a separate top-level aggregation.

**The next agent should not have to work out.** T8.5 builds the reader in the layer, calls `Warm` at boot (a nil return is not "the pool is configured" — a wrong key shows up on the first `Fit`), maps `ErrUnavailable` to `contract.PoolUnavailable`, and maps `Counts.Reasons` to the contract views. T8.3b does the live read. T8.3c resolves the union question before the pool is served.

### T8.5c: Re-link declared views (done, edge cases owned by T8.5d)

**What exists.** `internal/api/server/` re-links declared corrections by content match on restore and serves the consulted-empty pool shape. Review FINDINGS: one High (same-words/two-subjects cross-link on divergent rebuild — recall hard-fails), one Medium (silent views with no rewritten belief), one Low (same-subject two-belief collapse) — all owned by T8.5d, plus the server-comparison update for the tightened fixtures. The live tripwire flip is T8.5e's.

**The next agent should not have to work out.** T8.5d persists the view↔entry pairing and updates the comparison; T8.5e flips the live tripwires.

### T8.5b: Layer end to end, live (done)

**What exists.** `tests/live/precedence/` (driver, `run.sh`, README, git-ignored `out/`) boots the release binary in the T8.1 shape with the live recall tier, wires one served layer per shopper plus the live pool reader and fit writer, and decides one purchase per precedence step. Review APPROVE with one wording fix (driver header aligned with the pinned total-loss-of-force behaviour).

**Live transcript** (verbatim from `bash tests/live/precedence/run.sh`; test indices/sessions erased, prefix empty before and after):

```
T8.5b live layer -- every precedence step decides, live
deployment: <deployment> (elastic, T0.12's project)
budget: under $0.50 of model spend; test sessions erased afterwards

== correction decides (Take size M for Brand X)
-- standing correction with no belief, recorded Claimed and uncontradicted
-- next recall decides it, over pool+beliefs
-- same correction still decides over a fresh hub MCP session to the same shopper session
-- same correction still decides over a rebuilt layer on the same standing dir

== pool decides (Earned small, exact counts, seeded 10 / live 0)
-- new shopper on the seeded Earned-small line, decided by the pool
-- guidance text carries the exact counts and split

== own evidence decides (live Earned/Venerable belief beats the disagreeing pool)
-- three too-large returns earn the belief live; the pool rides along overridden
-- guidance carries "ahead of the pool" and the seeded 10 / live 3 split

== statement decides (Take size M for Brand S: the shopper's usual size.)
-- seeded general statement alone decides

== nothing decides (GuidanceNone, never a refusal)
-- fresh shopper on a lineless recall gets GuidanceNone

-- cleanup: deleted all 3 t85b-* indices, listing after []
-- run dir removed (store, configs, standing dir, child logs)

model spend: $0.00 (recording classifier, fixture embedder; one case ran first and stated $0.00)
```

Two behaviours pinned live with tripwires, both owned by **T8.5c**: (1) consulted-but-empty pool — the lookup returns Claimed with `(nil, consulted)` and the assembler rejects it; (2) dangling declared views — post-restart the restored view matches no class (total loss of force, pool decides), and post-rebuild replays the mint order so the stale view matches by id coincidence, not content re-linking.

### T8.5: Recall never refuses (done)

**What exists.** `internal/precedence/` resolves the five steps as a pure function (singles, all six disagreeing pairs, three-way, unreachable pool, `View` marking). `internal/api/server/` serves the pivot: recall with pool/pool_lookup/per-item precedence, `record_correction` with the optional belief, `product_fit`, `set_pooling` (opt-out, `Forget` with retry), the raw-body webhook joined via `custom_id`, `/calls` honoring `X-Memento-Shopper`, a strict MCP decoder, and a `memento key issue` command. `internal/standing/` persists one journal per shopper (only undeclared entries; atomic save/restore; service lock serialises). Repeat corrections return the stored entry without refreshing time. One `fitindex.Writer` per shopper resolves one line for every call. Review APPROVE with no code changes.

**What surprised.** The serving line excludes a lone return (Claimed, no promotion), so recall answers `GuidanceNone` — a soft refusal where product.md promises thin evidence; changing it moves the story, so it is structural (T7.4, after a product call). Declared-correction views dangle after restart (beliefs get fresh ids; the correction degrades to unscoped) — T8.5b should pin or fix re-linking by content match. No `Kept` writer call exists on any served path: kept-settling needs a return-window sweeper no task owns yet (orchestrator to task it). The seed's unserved general statement now renders beside the contradicted belief; T7.4's tightening must match the guidance texts pinned in the tests.

**The next agent should not have to work out.** T7.4 can now require the pool and precedence fields, add Claimed/Building examples, resolve the `GuidanceBelief` alias, add JSON tags to `domain.Subject`, and settle the rank-order question — nothing here contradicts it. T8.5b proves the live layer and pins the dangling-view behaviour.

### T8.1b: Lambo hub, live (done)

**What exists.** `tests/live/hub/` (driver, `run.sh`, README, git-ignored `out/`) drives the T8.1 shape against the live Elastic recall tier: three shoppers created on demand, each recalling only its own marker; the seeded demo session holds none; the reader serves the allowlisted ones (200s) and 404s a late shopper; the 503 + `Retry-After: 5` fires with every `max_attached` place full; 130 concurrent `initialize` hold exactly 128 with 7 refused; 600 stats calls over shared sessions serve 98 req/s with 0 throttled. Hub RSS 22.2 MiB → 37.2 MiB with 5 sessions. Review APPROVE with doc fixes (reader serves demo + three, not three; ordered pairs, not every pair; temp config carries the URL, removed at end).

**Live transcript** (verbatim from `bash tests/live/hub/run.sh`; test indices deleted, prefix empty before and after):

```
T8.1b live hub: one process for every shopper, against the live recall tier
hub binary: lambo 0.10.1
deployment: <deployment> (elastic, T0.12's project)
embedder: fixture (in-process vectors; the tier mirrors them live)
machine: linux/amd64, 12 CPUs
flags under measure: --max-sessions 128 --rate-limit-rps 200
budget: under $0.20 of embedding spend; test sessions erased afterwards

== 0. the test namespace is clean, and the demo session is the package's
-- prefix listing before: GET /_cat/indices/t81b-* -> []
-- demo session: package builds shopper-00895719…; start.sh pins the same 72 bytes

== hub up: addr=127.0.0.1:35155 (one pinned session, max_attached=16, credential prefix shopper-, create=true)
-- hub RSS at start: 22.2 MiB

== 1. one shopper first, then its cost
-- shopper-t81b-alpha recalls "Vexilhold": found its own marker
-- spend after one shopper: $0.00 of embedding (fixture computes in process; no embedding endpoint called)
-- under the $0.20 budget; continuing with the other two shoppers

== 2. two more shoppers, each recalling only its own concepts
-- shopper-t81b-alpha: own marker "Vexilhold" found; no other session's marker in its recalls
-- shopper-t81b-bravo: own marker "Brasswren" found; no other session's marker in its recalls
-- shopper-t81b-charlie: own marker "Codlefirth" found; no other session's marker in its recalls
-- every cross-session exact-content inspect: no concept matching

== 3. the seeded demo session holds none of their markers
-- demo session: no test marker recalled, no test belief inspected; stats: (concepts=0)

== 4. the ledger reader serves the allowlisted ones
-- reader with the demo shopper plus [alpha bravo charlie]
-- GET /s/alpha/api/inspect for its own belief: 200 found=true
-- GET /s/bravo/api/inspect for its own belief: 200 found=true
-- GET /s/charlie/api/inspect for its own belief: 200 found=false
-- GET /s/bravo/api/inspect for alpha's belief: 200 found=false
-- GET /api/inspect for a test belief: 200 found=false (default session is the demo shopper)
-- GET /api/sessions -> [demo alpha bravo charlie]
-- late shopper attached on the hub after the reader started
-- GET /s/late/api/inspect: 404 (uniform; the allowlist is start-time state)

== 5. the 503 + Retry-After refusal with every place full
-- second hub (max_attached=2: the pinned demo plus one shopper)
-- newcomer attach with every place full: HTTP 503, Retry-After="5"
-- the hot session still answers; a caller must retry after 5 seconds

== 6. --max-sessions 128 and --rate-limit-rps 200
-- 130 concurrent initialize with 5 already live: 123x200, 7x503, 0 other
-- the 503s past the ceiling are the bound working: 7 refused of 130 (ceiling 128)
-- 600 stats calls over 8 workers: 600 ok, 0 throttled (429), 0 other = 98 req/s served
-- no call throttled: the hub served 98 req/s without touching the 200rps ceiling
-- hub RSS with 5 attached sessions: 37.2 MiB (started at 22.2 MiB)

-- hub stopped (one SIGTERM, then waited, so the write-behind log drains)

model spend: $0.00 (no model call: the fixture embedder computes vectors in process)
elastic: test-prefixed indices only, deleted below; no billable resource beyond the existing project

-- cleanup: deleted 2 index(es) under t81b-
-- prefix listing after: GET /_cat/indices/t81b-* -> []
-- run dir removed (store, configs, child logs); nothing keyed survives locally

PASS T8.1b live hub: three shoppers isolated, the reader allowlisted, the 503 recorded, the ceilings measured, the namespace erased
```

**Spend.** $0.00 embedding; test indices created, read, deleted. The 128/200 and RSS numbers are 12-CPU machine numbers; T9.6 must measure them on the 0.5 CPU starter.

### T8.2c: Re-run the fit-index transcript (in review)

**What exists.** Re-ran `tests/live/fitindex/run.sh` unchanged against the live deployment: one recorded purchase through capture, refund and a classified reason into `t82b-fit-rows`, one document at each state, both replays and both retried fit writes leaving the row, the direct older write refused with `409 version_conflict_engine_exception: [<txn hash>]: version conflict, current version [3001794234729] is higher than the one provided [1001794233521]`, one document for the purchase, for the shopper's line and in the index, and the index deleted with the `t82b-` listing `[]` before and after. The three versions re-derive from the recordings: `1001794233521`, `2001794233529`, `3001794234729`. Review re-derived them and read every line of the transcript against the driver's print statements before this replaced the hand-written block above.

**What surprised.** T8.2b's block was hand-written around the driver's steps, not pasted and not stale output: `main.go` has one commit, so no earlier version of the driver could have printed it. The cheapest tell is step 1 — a capture files no question, so a transcript saying it did was not printed by this driver; the second is the `409`, whose reason carries the transaction hash in brackets before the versions.

**The next agent should not have to work out.** A pasted transcript is not evidence; re-run instead, it is one index and a handful of writes. `run.sh` writes only the driver's output to `out/transcript.txt` and prints its prefix and leak checks to the terminal, so a block carrying those check lines came from the terminal. Spend: $0 model, one kilobyte-scale index, deleted.

### T8.2b: Fit index writer, live (done)

**What exists.** `tests/live/fitindex/` (driver `main.go`, `run.sh`, README, git-ignored `out/`) drives `fitindex.NewHTTPIndex` + `Create` against the live deployment: one recorded sandbox purchase (T0.4 cycle 1) through capture → refund → classified reason, plus a replayed delivery and an older-version retry, then deletes the `t82b-fit-rows` index. Only the index was live; the payment events and classification are the recordings. Review APPROVE with two doc fixes (the answer's event time is the driver's own `refundAt + 20m`, not a recording; the second cycle does have a paired capture+refund, it lacks the order read with line items).

**Live transcript** (re-run by T8.2c from the unchanged driver; index deleted, prefix empty before and after). The block that stood here before T8.2c was written by hand, not pasted: steps 3, 5, 6 and 7 used a print form `docSource.String` never emits, step 1 reported a filed question, and steps 8 and 9 dropped words the driver prints. The three versions it showed re-derive from the recordings, so only the provenance was wrong. The run's closing `done:` line names the run directory relatively, because `run.sh` prints its absolute path and this diary carries no personal path.

Run: `source tools/env.sh`, then `bash tests/live/fitindex/run.sh`. Output:

```
T8.2b live fit index writer
deployment: <deployment> (elastic, T0.12's project)
index: t82b-fit-rows

-- prefix listing before: GET /_cat/indices/t82b-* -> []
-- create: PUT /t82b-fit-rows with the recorded mapping (no settings) -> ok
-- purchase: capture CAPTURE-ID-US-1, order ORDER-ID-US-1, refund REFUND-ID-US-1, brand-x-001/shirts/regular size M
-- row id: sha256("txn|"+CAPTURE-ID-US-1) = 5184598358cef128…
-- shopper hash for the line (live salt): 16e82eb3dda8fbba…

== 1. capture delivery WH-T82B-CAPTURE-1 -> ingest -> writer.Captured
   ingest: 1 evidence row(s), duplicate=false, pending question=false
   doc: version=1001794233521 state=captured settled=false kind=- reason=- direction=- verdict=- contested=false seeded=false captured_at=2026-11-09T14:12:01Z refunded_at=- settled_at=- line=brand-x-001/shirts/regular size=M

== 2. refund delivery WH-T82B-REFUND-1 -> ingest -> writer.Refunded
   ingest: 1 evidence row(s), duplicate=false, pending question=true
   doc: version=2001794233529 state=refunded settled=false kind=- reason=- direction=- verdict=- contested=false seeded=false captured_at=2026-11-09T14:12:01Z refunded_at=2026-11-09T14:12:09Z settled_at=- line=brand-x-001/shirts/regular size=M

== 3. replay the refund delivery (before the reason): ingest returns the stored rows
   ingest: 1 evidence row(s), duplicate=true (nothing written)
   doc: version=2001794233529 state=refunded settled=false kind=- reason=- direction=- verdict=- contested=false seeded=false captured_at=2026-11-09T14:12:01Z refunded_at=2026-11-09T14:12:09Z settled_at=- line=brand-x-001/shirts/regular size=M

== 4. shopper's answer -> recorded classifier -> writer.Classified
   answer "too tight in the shoulders" -> category fit_too_small (confidence 1.00, testdata/jev/01-fit-too-small-tight-shoulders.json)
   doc: version=3001794234729 state=reason_classified settled=true kind=return_reason reason=fit_too_small direction=too_small verdict=- contested=false seeded=false captured_at=2026-11-09T14:12:01Z refunded_at=2026-11-09T14:12:09Z settled_at=2026-11-09T14:32:09Z line=brand-x-001/shirts/regular size=M

== 5. replay the refund delivery (now behind the reason): the newer state stands
   ingest: duplicate=true (nothing written)
   fit write retried with the refund event: doc: version=3001794234729 state=reason_classified settled=true kind=return_reason reason=fit_too_small direction=too_small verdict=- contested=false seeded=false captured_at=2026-11-09T14:12:01Z refunded_at=2026-11-09T14:12:09Z settled_at=2026-11-09T14:32:09Z line=brand-x-001/shirts/regular size=M

== 6. retry the fit write with the capture event (an older state, same writer)
   doc: version=3001794234729 state=reason_classified settled=true kind=return_reason reason=fit_too_small direction=too_small verdict=- contested=false seeded=false captured_at=2026-11-09T14:12:01Z refunded_at=2026-11-09T14:12:09Z settled_at=2026-11-09T14:32:09Z line=brand-x-001/shirts/regular size=M

== 7. a fresh writer (a restarted process) hands the index the captured event
   writer returned nil (an older version is done, not an error); doc: version=3001794234729 state=reason_classified settled=true kind=return_reason reason=fit_too_small direction=too_small verdict=- contested=false seeded=false captured_at=2026-11-09T14:12:01Z refunded_at=2026-11-09T14:12:09Z settled_at=2026-11-09T14:32:09Z line=brand-x-001/shirts/regular size=M

== 8. the index refuses a write at the captured rank directly
   PUT at the captured rank -> 409 version_conflict_engine_exception: [5184598358cef1284be45ff567ea286d7f96156bfdbfe32cd4a24d446ec163c2]: version conflict, current version [3001794234729] is higher than the one provided [1001794233521]

== 9. one document per purchase
   docs for the purchase: 1, docs for the shopper line: 1, docs in t82b-fit-rows: 1

== states and versions
   captured           rank 1, version 1001794233521 (2026-11-09T14:12:01Z)
   refunded           rank 2, version 2001794233529 (2026-11-09T14:12:09Z)
   reason_classified  rank 3, version 3001794234729 (2026-11-09T14:32:09Z)

model spend: $0 (no model call: the answer and its category are recorded)
elastic: one kilobyte-scale index, 3 state writes plus 3 replayed writes, reads and counts; deleted below

-- cleanup: DELETE /t82b-fit-rows -> 200 {"acknowledged":true}
-- prefix listing after: GET /_cat/indices/t82b-* -> []

PASS T8.2b live fit index: one purchase, one row, every state, replayed writes harmless, index deleted
--- cleanup check: the t82b- prefix is empty ---
[]
--- leak check: no key or deployment location entered tests/live/fitindex ---
done: T8.2b live checks pass; index deleted; transcript in tests/live/fitindex/out (git-ignored)
```

**Spend.** $0 model; one kilobyte-scale index, 3 state writes + 3 replayed writes + reads/counts + one delete on the existing project.

### T8.2: Fit index writer (done)

**What exists.** `internal/fitindex/` is the layer's whole index interface (`Create`/`Put`/`DeleteShopper`) and a per-shopper `Writer` for captured → refunded → reason → settled/contested, `Record(domain.Evidence)`, `Forget(lines)` and an opt-out check; `Mapping()` is T0.12's mapping field for field. `internal/ingest/WithFitRecorder` writes the row beside capture and refund deliveries; disputes and shopper words write nothing. Tests cover every state ordered and shuffled, webhook replay, older-state refusal, a reasonless refund, corrections, opt-out across lines, and the mapping. Review APPROVE; the three low code findings (partial-delete success, consent/write race, unsettled time) were fixed on the task branch before landing.

**What surprised.** The row key is the capture id with the order id as fallback, so a multi-item order shares one row; every demo and seed purchase is single-item. `domain.Item` has no fit handle, hence the `Lines` resolver. The mapping still declares `event_seq` while the writer never sets it: T7.1's rank-plus-time version supersedes it. `Record` drops a refund's attached reason, so a seed refund must be written `Refunded` then `Classified`. `refresh=true` on every write is a forced refresh per document.

**The next agent should not have to work out.** T8.5 builds one writer per shopper in the server state and calls `Classified`/`Feedback`/`Contested`/`Forget`, resolving the same line the writer would; T8.2b proves the live adapter; T9.6 seeds through `Refunded` then `Classified` and measures the refresh cost. Owners: T8.5 for line duality, capture-id consistency, `captured_at` hydration and partial-delete retry; T9.6 for the refund reason and refresh cost; T7.4 for the rank-order edge.

### Finding owners (user, stands)

Every finding, including a low one, names the future task that owns it or is fixed before the task closes. A finding with neither blocks the close. The rule is developer rule 11 in AGENTS.md and is in the orchestrator's validation list; review briefs ask for severity **and owner task**. The current owners are recorded in the task texts (T7.4, T8.5, T9.2, T9.3, T9.6, T10.3) and in the handoff entries.

### Low-finding sweep (docs)

The stale "nine frozen tools" comments in `internal/api/server/mcp.go`, `internal/api/server/server.go` and `cmd/memento/main.go`, and the same count in `agent/runtime/README.md`, are corrected to eleven. The historical round 1 name is annotated in `bench/pilot/README.md` and `PHASE-5-comparison.md`, pointing at `bench/README.md`. Remaining low findings are not doc-only and stay with the task that owns the file: the durable standing-correction journal and its serialisation/repeat-time rules (T8.5), the pre-pivot correction types in `web/src/lib/contest/` (T9.2), the Claimed/Building `product_fit` fixtures and the population-aligned example counts (T7.4), and the 31-day/arm-2 fairness notes (T10.3).

### T9.1: Seeded population (done)

**What exists.** `seed/population.go` generates 69 `domain.FitRow` rows across six line-and-size pairs, using `identity.SeedSaltHex` per-line hashes, `identity.TxnHash` row ids and the seeded mark on every row; no raw shopper or transaction id is in the fixture. `GeneratePopulation(asOf)` takes the anchor and `WritePopulation` writes `testdata/seed/population/population.json`. The demo oxford M is **Earned small** (12 distinct too-small shoppers, 63% share, and the recent window earns on its own); oxford L and brand-y are **Building**; brand-z is Building because the 60% share fails; brand-v is **Claimed** below the floor; brand-w is **Out of true** (the recent window flipped to large). Review APPROVE with one fix pinning a settled row's `EventAt` to its `SettledAt`.

**What surprised.** The recent-window rule is strict: an Earned line must have a recent window that earns on its own, so the demo line carries exactly eight recent too-small shoppers; a handful of recent rows would flip it Out of true. The seeding moment must be passed as `asOf` or statuses drift. The population's counts differ from T7.1's illustrative `product_fit` example (22 settled / 19 shoppers here, 60 / 39 there) while the status and direction agree; T7.4 should align or label the example.

**The next agent should not have to work out.** Use `DemoOxfordLine()` / `PopulationLines()` for Go callers; the Svelte ledger and merchant app cannot import Go, so T9.3 takes its line list from `testdata/api` or its own config. The row is `domain.FitRow`; T8.2's writer is the consumer. Live seeding (T9.6) passes the seeding moment as `asOf`.

### T8.4: Corrections that stand on their own (done)

**What exists.** `internal/memory/correction.go` adds `Correction`, `Belief`, `StandingCorrections` (`Record`, `Declare`, `Len`, `Standing`) and `EnsureBeliefDeclaring` beside the unchanged `RecordCorrection`. A standing correction derives as a note (never promotes); a later matching belief gets one `against` declaration dated by the correction; a second matching belief or a later correction never repeats it. `SameSubject` matches subject kind and id, fit↔brand / seller↔seller, and line fields only where both sides set them; size is carried, not gated. Review found no defect in the task; four small fixes committed.

**What surprised.** The journal is per-shopper caller memory, not graph state: Lambo can inspect a concept by exact content but cannot enumerate a session's concepts or store structured attributes, so a restart would lose the ability to declare. T8.5 must persist the journal (it now owns `internal/standing/`) and serialise writes when two agents speak for one shopper. An identical repeated correction keeps its first event time; T8.5 must decide whether a repeat refreshes it.

**The next agent should not have to work out.** T8.5 wires the standing branch (an empty belief id is not an error), calls `EnsureBeliefDeclaring` on every belief write, holds one journal per shopper, and persists it across restarts. Issue text for the user: **Lambo: no way to enumerate a session's concepts or store structured attributes beside one.** `lambo_inspect` needs the exact content and recall is query-based, so the layer cannot rebuild a set of standing corrections after a restart. Requested: a session-scoped list of concepts with type and event time (optionally filtered by type or key), and/or structured attributes beside a concept.

### T10.1: What arm 2 actually gets (done)

**What exists.** `bench/round2/arm2/` holds the MCP-run recordings (transactions succeeded/pending, three orders, two refunds, one dispute, a 28-tool list, untouched envelopes), `extract.go`, `doc.go`, `extract_test.go` and a README. `arm2.Load(dir)` plus `arm2.Extract(recordings, Selector{CustomID|PayerEmail})` return arm 2's prompt block: returned fields only, trimmed, ordered. The test proves no invented field (mutations fail), and the demo extraction pins 6 transactions (5 successful + 1 pending hold), 3 orders with line items, 2 refunds and 1 dispute with its reason code. Review found and closed a provenance gap: a fresh `tools/list` (28 tools, `get_order` included) and one `get_order` envelope, matching `get-order-1.json` field for field.

**What surprised.** `list_transactions` returns `transaction_info` only — no line items, no payer block; line items need `get_order`, and only for order ids the caller already holds. The default `transaction_status=S` hides the pending dispute hold; a second call with `P` is needed. The listing is merchant-wide with no shopper filter: the only per-shopper key is `custom_field` (the seller's `custom_id`), so T10.3 must set it on generated orders. Refund free text (`note_to_payer`, `transaction_subject`) reaches arm 2 through the plain listing; the demo says "fit return", so generated notes must stay fit-neutral or case 3 is unfair. `list_disputes` now returns this merchant's disputes, unlike T0.4.

**The next agent should not have to work out.** Feed `arm2.Extract` verbatim as arm 2's prompt memory. Use a 31-day reporting window and `transaction_status` S and P. Do not add a reason or signal to the extraction. T10.3 owns the fairness checks: set `custom_field`, keep notes fit-neutral, and decide whether line items and order ids are supplied to every arm. The `get_order` envelope proves the tool call; the other two order bodies are the same tool's output kept as recorded.

### T7.2: Shopper and purchase keys (done)

**What exists.** `internal/identity/` builds all three stored keys from an explicit salt: `shopper-<64 hex>` session names, per-line shopper hashes, and the unsalted `sha256("txn|" + id)` row id. `NewKeyer` validates 32 bytes after hex-decoding (`ErrMissingSalt` / `ErrMalformedSalt`, no fallback, and the error never echoes the value); the `Keyer` is immutable. HMAC messages are version-tagged and length-prefixed: `memento/session/v1|…` and `memento/line-shopper/v1|…`. `SeedSaltHex` is the published seed salt (sha256 of a stated phrase) for seeded rows and seeded shoppers; live keys use `MEMENTO_IDENTITY_SALT`, now exported by `tools/env.sh` from the keyring and named by `.env.example`. Review APPROVE with two doc fixes.

**What surprised.** The session name is fixed at 72 bytes, so no payer can sit at the graph's 128-byte limit; `ValidSessionName` pins the 128/129 boundary and a 128-byte payer is exercised. `NewKeyer` accepts uppercase hex because it decodes to the same bytes. The per-line construction replaces T0.12's `sha256(line salt + "#" + id)` sketch with one salt over a purpose-tagged, length-prefixed message; T0.12's synthetic per-line hashes do not replay, but its transaction hash does.

**The next agent should not have to work out.** Use `SeedSaltHex` only for rows and sessions carrying the seeded mark; live callers use `MEMENTO_IDENTITY_SALT`. `TxnHash` takes no salt, so rotation moves session names and shopper hashes but not row ids. Nothing calls the package yet: T8.1's deploy naming, T8.2's writer and T9.1's population are first. AGENTS.md now lists the keyring entry.

### Identity salt created (user, stands)

The live identity salt is in the keyring at `service identity account memento-emptor kind salt`, read as `MEMENTO_IDENTITY_SALT`: 32 random bytes as 64 lowercase hex characters, generated once and never printed. It keys the salted payer-ID session name and the salted per-line shopper hash; the transaction row id stays the unsalted `sha256("txn|" + id)`. Seeded rows use a separate published seed salt. Rotating the live salt changes every session name and per-line hash, orphaning existing sessions and breaking opt-out's recompute. The same value must reach the live Render service as a secret.

### T7.1: Pivot contracts (done)

**What exists.** `internal/domain/` holds the fit row (`ProductLine`, state rank, settled kind, verdict, `FitRow.Version()` = rank × 1e12 + event Unix seconds), the pool rule (`PoolCounts`, `PoolVerdict`, `DecidePool`, constants 3 / 8 / 60% / 2× / 90 days) and `UsualSizeFor`, each table-tested. `internal/api/contract/` has the five-step precedence (`own_evidence` replaces `belief` on the wire; `GuidanceBelief` aliases it in Go), an optional correction belief, `product_fit` and `set_pooling` (eleven tools). `testdata/api/` has 12 new and 5 changed fixtures; `docs/interface.md` carries 31 blocks; `docs/architecture.md` gains the fit-index write path, the per-shopper hub and the merchant app. Review APPROVE with one medium and four low findings.

**What surprised.** The served layer's fixture comparison forced every pivot field to stay optional and the base recall example to stay pre-pool, because `testdata/replay/` is frozen; a conforming implementation could currently never return a pool. That tightening is T7.4, after T8.5. Below three shoppers the pool reads Claimed; the 90-day window is judged only at or above the floor, so a quiet line keeps its label; the 60% share only bites because fits-well outcomes dilute it. `tools/list` advertises the two new tools before dispatch serves them (no current caller discovers tools), and the wire basis renamed `belief` to `own_evidence`.

**The next agent should not have to work out.** T7.4 owns the contract tightening and the Claimed/Building fixtures; never edit `testdata/replay/` to force the contract. T8.5 owns the stale "nine frozen tools" comments in the server and `cmd/memento/main.go`; T9.2 owns the stale pre-pivot correction types in `web/src/lib/contest/`; T9.4 owns the agent README's tool count.

### T10.2b: Round 1 corrections reported (done)

**What exists.** `bench/README.md` gains a `## Corrections` section: 27 turns (9 shoppers × 3 arms), 27 of 27 honoured (9 of 9 per arm), 0 wrong-cause, and arm 3 routing 2 of 9 through the real `record_correction` path with nothing to declare against in the other 7. Evidence is `bench/runs/t59/t59-scores.json`; the section says plainly that corrections did not discriminate the arms. The frozen 45-turn column stays `nil`, scoped to that set and pointing at the new section. Review APPROVE with two wording fixes (the correction contradicts the first size only where one was given; the unmeasured cases are named correctly).

**What surprised.** `correction_recorded` exists only on arm-3 rows, so 2 of 9 is not 2 of 27; `correction_honoured` is a boolean in the T5.9 scores but `null` in the T5.5 scores — the two columns must never be merged. The highlighted evidence cell `t59-a3-noisy-large` carries the self-quote contamination overrule (the frozen check fired on the run's own `corr_` id); the section makes no contamination claim, but a reader opening the scores may pause there.

**The next agent should not have to work out.** Keep the 45-turn set and T5.9's 27 turns visibly separate. Round 2 still needs a cross-session and cross-agent correction turn; T5.9's are same-session only.

### T10.2: Round 1 relabelled (done)

**What exists.** `bench/README.md` now names round 1's second arm **Memento evidence without beliefs**, states why (its prompt carried the layer's evidence rows: classified reason, signal strength, write kind, with no beliefs over them), and points at round 2. No count, run file or frozen input changed; the frozen run files keep `arm_name: "raw-history"` and the old prompt line deliberately. Review APPROVE with fixes: the stale "21 sha256 rows / 5 histories" line now reads 25 / 9, and "per-purchase rows" became "evidence rows".

**What surprised.** The old name survives in `bench/pilot/README.md` and `dev-diary/PHASE-5-comparison.md`; those are historical records, not rewritten. The round 1 record still reports corrections as nil and never points at T5.9's 27 published turns — a medium completeness gap, queued as T10.2b rather than fixed here because T10.2 forbids count changes.

**The next agent should not have to work out.** Do not push the new label into frozen run files or `bench/harness/`: the manifest pins them. T10.2b owns the correction pointer; it must not touch any other count.

### PayPal Transaction Search enabled (user, stands)

The sandbox app's grant now carries **37 scopes**, including `https://uri.paypal.com/services/reporting/search/read`. `GET /v1/reporting/transactions` answers `200`; the date range must be **31 days or less** (a longer range is `400 INVALID_REQUEST`). `transaction_details[]` carries `transaction_info`, `payer_info`, `cart_info`, `shipping_info` and the other reporting blocks. The ledger still reads orders/captures for evidence; the reporting API is arm 2's transaction-listing source (T10.1). The earlier `NOT_AUTHORIZED` recording stays as the pre-enablement state.

### New services pre-approved (user, stands)

The user has approved creating the pivot's new services: the merchant app's own Render service (T9.3d) and the second Astropods deployment (T9.5). Record each one's monthly cost or tier in the handoff log and keep the task's guard; do not stop for a second approval. A spend beyond a task's brief is still the user's call.

### T0.12: Elastic fit index (done)

**What exists.** `dev-diary/research/elastic-fit.md` records all eight checks with requests, responses and results, the exact mapping (18 structured keyword/boolean/integer/date fields, `dynamic: strict`, no text or vector), the reusable line-and-size aggregation, and the write rule: whole-document `PUT` to `_id = sha256("txn|" + transaction id)` with `version = event_seq` and `external_gte` (a `409` is an older event that must not win; the same version is a harmless retry). `testdata/elastic-fit/` holds the generator/harness and ten redacted recordings; `t012-*` indices were deleted (live check: empty listing). Review passed with non-blocking findings; fixes committed.

**What surprised.** The deployment is an Elasticsearch **Vector Database** project, not general Serverless: only `vectordb_document` index mode, `_update` refuses external versioning (whole-row rewrite is the path), and billing is storage/Search Power/ingest rather than VCU. Cold TLS repeats at ~6.9 s against ~236 ms warm, so the client needs the 20 s timeout T0.6 settled. `cardinality` is exact at the 588-shopper pool under the default 3000 threshold; 3000 and 40000 are its practical exactness bounds.

**The next agent should not have to work out.** Do not invent a different hash: `sha256("txn|" + id)` is what the recording pins. Set `event_seq` to the purchase's event sequence (the review found the committed `states.json` request bodies are the final row, not per-step snapshots, and rows kept `event_seq: 1` while `_version` advanced; the mechanism rests on the GETs and counts, and the next live state run must copy the body per step). Settled means a classified return reason, fit feedback, or kept past the window; a no-reason refund and an in-window purchase count toward nothing; `fit_direction` exists only on fit returns and fit feedback; the denominator is the settled filter's `doc_count`. Give the client `timeout_ms: 20000` and a kept-alive pool.

### T0.11: Lambo serving many shoppers (done)

**What exists.** `dev-diary/research/lambo-multi.md` and `testdata/lambo-multi/` (harness, eight transcripts, eight reports). Lambo v0.10.1 multiplexes: one `lambo serve --transport http` hosts one writer per session and many sessions, attaching a session on its first request under a credential `session_prefix` (`shopper-`) with `create = true`. Measured: 21.1 MiB hub, about 0.4 MiB per attached small session, about 29.7 KiB per concept; eight concurrent writers lost nothing; the Elastic recall tier shares one contract index filtered by `session_id`; 20 sessions over 380 pairs leaked nothing; interleaved promotion stayed isolated. Deploy shape: one hub in `start.sh`, `max_attached: 16`, sessions `shopper-<64-hex>`, the layer on `/mcp/s/<session>`, the ledger on `/s/<session>/api/inspect`. Spend $0.00.

**What surprised.** The deploy's "one writer per session" is only half true: one *process* can own many sessions. A second writer on a held session silently becomes a proxy into the holder, not a refusal. `max_attached` includes the one pinned session HTTP requires, so 15 shoppers attach beside it. `serve-web`'s allowlist is fixed at start: a shopper created later is a 404 until restart; filed as [Lambo issue 241](https://github.com/nrynss/lambo/issues/241). Review findings, non-blocking: `serve-web`'s own RSS was not measured (16 is safe; 20 is an upper bound only, so measure the reader before raising `max_attached`), `--max-sessions` and `--rate-limit-rps` are unmeasured on the 0.5 CPU starter, check 4's cleanup is not in a `finally`, the 503 refusal path is not recorded, and `serve-web`'s default `max_loaded_sessions` is 4 (tune it in the ledger work).

**The next agent should not have to work out.** Keep the seed-then-hub order in `start.sh`: the seeder's short-lived stdio writer must own its session alone while it runs. Do not pin every shopper; pinned sessions never idle-detach and each spends a `max_attached` place. The Elastic tier keeps one data index per embedding contract, filtered by `session_id`.

### Pivot decided after P5 (user)

The comparison's "raw history" arm was handed rows the layer had already classified (reason, signal strength, write kind), so its 15/15 and 4/4 measure Memento's evidence without beliefs, not raw PayPal history. The arm that gated on Earned refused and lost. product.md now proposes: recall never refuses (status is a label), corrections stand alone, a real PayPal-only arm 2, pooled product fit across shoppers, and a merchant view (decided: ships). Per shopper stays in Lambo, one session each; the pool goes to an Elastic fit index, written at ingest, one row per purchase, settled rows only (kept past the window counts), a per-product-line shopper hash, seeded and live in one index. T0.11 and T0.12 tested both halves and both passed (entries above), so the pivot's implementation phases can now be written. Open: consent for pooling, and which headline the new comparison supports.

### T5.9: Correction round (done)

**What exists.** 27 committed correction turns (9 shoppers × 3 arms) graded against the frozen grader: 27/27 honoured, 0 wrong-cause everywhere. Arm 3 routed only 2/9 corrections through the real `record_correction` path (both noisy-shopper Venerable beliefs, post-recall asserted); elsewhere every arm just followed the in-conversation correction. Review APPROVE, no fixes (verified the uniform rule, the contamination exception, and the honest headline from committed files).

**What surprised.** Corrections do not discriminate the arms — obedience is universal, mechanism differs. The layer's signature move engaged in only 2 of 27 turns. T5.6's lead-number pick stays parked: nothing here unlocks an arm-3 win either.

### Custom domain decision (user, stands)

`memento.nryn.dev` was added to Render and then removed again per the user's call: keep the Render URL for the demo — it reads better for the hackathon. Cleaned up (204 on delete, domain list empty, render URL 200). No DNS was ever pointed at it, so nothing to unwind on the Cloudflare side.

### T3.8: Public shopper page (done)

**What exists.** `agent/frontend/` separate deployment (own spec, ROOT image serving :80): no-login 200 on the public frontend host, chat host untouched, one messaging turn + four page turns with recall-first spans, $0.13 of $0.50. Review APPROVED with no fixes (verified composition, cap behavior, purchases-unwired, hygiene).

**What surprised.** F1–F3 all held as built: ROOT + `EXPOSE 80` with an explicit do-not-add-USER comment; page + `serve()` compose in one container; separate deployment leaves `agent/runtime/` untouched. Counter is pre-generation (a 502 burns budget too) — conservative, kept deliberately.

**The next agent should know.** Judges start from the fixture proof (`corr_3b7e52`, `blf_8f2c41`); live corrections live only in container memory and reset on redeploy (the honest demo reset).

### R0 review: P0 ground (APPROVE with fix, landed)

Re-verified all ten exit criteria against research + fixtures. Real defect caught and fixed: unredacted shopper emails + Mumbai street address in three capture fixtures (+ one webhook.site UUID), scrubbed per the file's own precedent with new SHOPPER-* placeholders; phase exit boxes checked (were unchecked despite done tasks); T0.6→T0.6b→native-direction coherence pointers added. No structural findings.

### R3 skipped: P3 review deferred (agent phase incomplete)

T3.6 full demo still running at review time. P3 gets its phase review after T3.6 lands (covering T3.1–T3.8 as one arc).

### R3 review: P3 agent end-to-end (APPROVE, landed)

Walked the whole arc as one story: recall-first → candidate+marks → ask/approve/correction/feedback → idempotent approve-first capture → evals → 6-step demo from reset → no-login frontend. Money safety proved in code + tests: no capture path without explicit-yes mirrored into the transport; deployed agent owns no purchase tool. Honesty fix landed (runtime README listed a fixture correction as "live"). Findings for follow-up: (F1) probe Dockerfile stayed root for :80 while the agent image runs as `bun` — T3.8 must confirm port binding; (F2) shopper page + `serve(agent)` composition unproven; (F3) `frontend:true` in the frozen runtime spec needs an owns decision; (F4) deployment id committed in demo/ files. Closed against the P9 decision (T9.5/T9.8): the reference deployment id is the operator's documented stand-in in `demo/drive.py`, not a secret — the org id's scrub is **T9.11**.

### R5 review: P5 comparison (APPROVE, landed)

Re-verified both exit criteria with independent runs (6 shoppers × 3 arms, byte-equality, cap math, three-way hidden-answer check incl. a failed attempt to construct a leaking shopper). Fix landed: F1–F4 + extraction rule moved from README handoffs onto the T5.1/T5.2 phase-doc lines where T5.4's freeze author reads them. Findings (advisories for freeze/pilot): arm-2 inputs omit amounts/IDs (define the contract at freeze; ID-cites in arm-2 runs = hallucination signal); pilot needs a two-turn driver + extraction rule in bench/pilot/; T2.6 direction note lives only in README (P2 follow-up, not P5's to fix).

### R4 review: P4 ledger (APPROVE with fix + 4 findings, landed)

All four exit criteria re-verified with the reviewer's own runs (96/96 web tests). Real fix landed: the seller why-view showed fits-well feedback as "Counts toward nothing" — now names the kind it counts toward when `counts_toward` is non-null, with regression test. Findings: (1) rendered-comment defect wider than recorded — 4 files incl. both live pages (needs sweep task, frozen T4.2 file untouched); (2) contest UI built but wired to no route — judges can't reach it (needs wiring task); (3) ledger→belief deep links still missing (accurately described, needs ledger-route edit); (4) raw reason codes on why cards (`fit_too_small` verbatim — wording follow-up).

### R2 review: P2 layer (FINDINGS — 1 non-blocking, landed)

Re-verified all six exit criteria against code + committed live artifacts; all fail-closed properties hold against constructed counterexamples (override deleted, changed-mind, uncovered fake, approve-first both paths). One new offline test landed (`TestChangedMindClosesQuestionWithoutBelief` — accepted-but-counts-nowhere had writer-level but no service-level coverage). One real honesty gap found: the frozen offline story's third return is mixed-direction (`fit_too_large`) while the live T2.6 run was fixed to all-small — the offline suite and the live run prove different things. Remedy + product question with the user.

### R1 review: P1 contracts (APPROVE, landed)

Re-verified all five exit criteria against code, not handoffs. Freeze holds: no post-P1 modification of any frozen path, no redefined enums, `testdata/api/` ↔ contract equality proven by gate test. Seam honesty confirmed (`var _` assertions, identical method sets; `memory.Graph` minus `Recall` is documented and load-free). One doc fix landed (embedder note no longer claims 0.10.1-closed issues still block the native switch). No structural findings; freeze stands.

### T4.3: The why view (done) ★

**What exists.** `web/src/lib/why/` + belief route: per-belief evidence with outcome/item/reason, correction beside the belief in plain words ("you said M; three returns said L"), non-counted items each with their exclusion reason, seeded marks with tooltip, pending question shown open, trace anchors marked as fixtures, back-to-ledger link on every state. Step walk 2→3→4→5 verified against fixtures.

**What surprised.** Review approved with one CSS-class rename (graph vocabulary out of user-facing names). Mount-testing gap confirmed real (Vitest resolves svelte to server build; needs a test-alias change outside any task's owns — recorded, not worked around). Confirmed follow-ups: ledger→belief deep links need a ledger-route edit (new task); ledger-wide evidence search is server-side work. Simplicity bar held: no standing/Venerable/Canonical in user strings, `untrack()`-guarded refresh (reactive subscribes on any read — loops otherwise).

### T4.4: Contesting a belief (done)

**What exists.** `web/src/lib/contest/` (CorrectionButton column, ContestEditor form, correction-path-only transport, state helpers, source-scan + mount tests — 19/19 with reviewer's additions). Edit moves Earned→Out of true, standing kept, row visible; three layers of proof that only `record_correction` fires.

**What surprised.** Review caught a real rendered bug: file-header `//` comments in Svelte compile to visible text nodes (fixed; same pattern pre-exists frozen in T4.2's `LedgerGrid.svelte` — follow-up task to sweep all `.svelte` files, not a silent fix). T4.4's own mount tests needed a client-entry import path (Vitest resolves `svelte` to server build here).

### T5.8: Re-run on the hardened set (done)

**What exists.** 12 committed runs (4 shoppers × 3 arms) with frozen grading: arm 1 0/4 refusals, arm 2 4/4, arm 3 2/4 (both noisy-shopper Venerable beliefs recommend; multi-brand Candidate beliefs honestly refuse), 0 wrong-cause everywhere. Verdict: still no run where arm 3 wins and raw history fails — the noisy pair narrows it (arm 3 acts) but does not flip it. $0.038 this task (~$0.20 cumulative of $5.00).

**What surprised.** Review confirmed the honest shape (graph-level non-interference live; refusal/routing proved offline) and fenced transcript-only claims. The comparison remains decisive *against* the layer on right-size — recorded plainly, no spin.

### T5.7: Hardened comparison shoppers (done)

**What exists.** 4 new deterministic shoppers via the generator (`cmp-noisy-small → L`, `cmp-noisy-large → S`, `cmp-brands-steady → M`, `cmp-brands-split → S`), hidden answers separated, freeze manifest amended to 25 rows. Review walked each as a perfect reasoner: all fair, arm 2 would also get all four right (honest — hardening is about realistic noise, not anti-arm-2 tuning).

**What surprised.** Nothing structural. Observation carried forward: `brands-split` moves on 2 fit rows while noisy pair pins ≥3 — T5.8 reports per-case evidence counts outright.

### T5.6: Results (done)

**What exists.** `bench/README.md`: results as counts with run links, limits up front, three lead-number candidates unchosen. Arm 1 0/15, arm 2 15/15, arm 3 0/15 real recommendations — stated plainly with both numbers wherever the graded artifact appears. Corrections nil (documented, not manufactured).

**What surprised.** Review's only changes were wording precision (no percentages anywhere, all-15-turns cite scope). Counts recomputed independently; hidden-answer absence verified programmatically.

### T5.5: All arms on the frozen set (done) ★

**What exists.** 45 committed runs (5 shoppers × 3 arms × 3 repeats) with full records, graded by the frozen grader: arm 1 0/15 (one M-default guess), arm 2 15/15 0 wrong-cause, arm 3 0/15 real recommendations (12 outright + 3 informed refusals; graded 3/15 on the mention-artifact — T5.6 presents both numbers). Real `recall_for_purchase` + pending per run; cumulative $5 cap enforced in code ($0.08 total spend). Freeze verified green before and after.

**What surprised.** Review confirmed the headline with an independent recompute; the only fix was doc dedup. Arm 2 wins honestly (causal algebra from raw rows); arm 3's layer compacts to nothing on thin histories (max 1–2 same-direction rows, never Earned) and refuses instead of guessing — the honesty T5.6 must report: arm 2 beats arm 3 here *because the layer refuses without Earned belief*. Neutral instrument for all arms (reference agent contaminates).

### T6.1: Deploy (done)

**What exists.** `deploy/` (Dockerfile, render.yaml, provision/guard/start/paypal-webhook scripts, seed-once): service `memento-emptor` + 1 GB disk provisioned, $7.25/mo shape verified live. Review approved with no fixes; settled-shape match, API unwrap fixes, Done-when honestly gated on landing (Dockerfile rides the merge).

**What surprised.** Review caught nothing (clean approve). Pre-land blockers were both external (unfetchable repo → GitHub connect; 402 → user card), now resolved.

### T5.4b: Freeze-check script (done)

**What exists.** `bench/freeze_check.sh` (executable, single commented `extract_manifest`, strict `sha256sum -c`, 21-row pin) invoked from `tools/check.sh` right after the tool-PATH loop, failing the whole gate before gofmt. Tamper/missing-file/row-count deviations proven by reversible in-test mutations.

**What surprised.** The `whole-manifest-deleted` edge fails closed but without the friendly message (grep exits 1 under `set -e` before the echo) — message polish only, recorded, deliberately not churned. `shellcheck` isn't installed here (`bash -n` used instead).

### T5.4: Freeze (done)

**What exists.** `bench/FROZEN.md`: 21-row sha256 manifest (histories, answers-hash-only, seed, harness, grader, extraction, drivers, neutral instrument), pinned model/caps/basis/repeats, publish-everything rule, every follow-up resolved as adopt/defer/owner. Review re-verified hashes, prompt parity, hidden-answer separation byte-level; one doc fix (row count, F2/F3 labels).

**What surprised.** Review proved the neutral-instrument rejection with the discarded runs (recall with wrong shopper is unguessable under no-memory prompts). $5 cap is declared-not-enforced (T5.5 codes it); probe-turn spend should be committed as marked runs.

**The next agent should know.** Freeze inputs live where T5.5 reads them. Verifier script is a minted follow-up (T5.4b in the phase doc), not a gap.

### T5.3: The baseline pilot (done)

**What exists.** `bench/pilot/` ran arms 1+2 live across all 5 comparison shoppers (neutral same-model instrument — the reference agent contaminated 2 turns with demo memory, kept as discarded evidence): arm 1 0/5, arm 2 5/5 right-size 0 wrong-cause. Extraction rule committed + tested (deterministic first, one documented human override, verbatim-ID cites only). No shopper changes (n=1 per cell; hardening would overfit — candidates recorded for freeze). Spend $0.085 of $5.00.

**What surprised.** The reference agent cannot drive the arms (shopper_01 default recalls demo memory) — graded runs use the tool-less neutral instrument; T5.5 must verify recall target per turn. Arm 2's strength is honest causal algebra (damage≠fit etc.), but 7/9 cites need human prose-mapping. Review verified token math, prompt parity, and hidden-answer separation byte-level; one doc fix (neutral instrument named).

**The next agent should know.** Freeze inputs recorded where T5.4 reads them: extraction rule, arm-2 input contract gap (amounts/IDs omitted), $5 cap declared-not-enforced (enforce cumulatively in T5.5), probe-turn spend should be committed as marked run files.

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

### T2.7b: Direction rule, live (done)

**What exists.** `tests/live/direction/` driver (no CI tests, `out/` ignored): Venerable on 3 same-direction returns live (blast 3, Swarm), opposing standalone concept stays unattached with no `Contradicts`/warning; recall serves the belief clean while ranking the opposite first; session erased, `t27b*` indices empty. $0 spend.

**What surprised.** Review scoped the claim honestly: the live leg proves graph-level non-interference (no auto-attach/contradict by the daemon), NOT the service path — refusal (`ErrDirectionMismatch`) and unattached-row routing are proved offline in T2.7's tests. Wording fixed accordingly ("byte-identical" → asserted properties; cleanup counts marked transcript-only).

### T2.8b: Rival birth, live (done)

**What exists.** `tests/live/rival/` driver (no CI tests, `out/` ignored): Venerable on 3 same-direction returns live, opposing standalone stays unattached with no `Contradicts`/warning, recall clean while ranking the opposite; rival born at 3rd consecutive opposite (Claimed/None, 1 row), absent on interrupted streak; sessions erased, `t28b*` indices empty. $0 spend.

**What surprised.** Review scoped the claim honestly (graph-level non-interference proved live; refusal + routing proved offline) and fixed two README overclaims (unattached = ledger rows with no graph write, not "citing nothing"; gap-inspect accepts refusal or non-admitting answer).

### T2.8: Rival-belief birth rule (done) ★

**What exists.** `OppositeStreak` streak state (in-memory service state, never the graph) with `RivalBirthThreshold = 3` in one place: 3 consecutive opposite-direction returns birth an unpromoted same-kind rival via the existing mint path; below 3 they land unattached (T2.7); any same-direction return or shopper correction resets; no auto-against, existing belief untouched, no second rival (4th joins the newborn — pinned by review-added test).

**What surprised.** Review proved the backstop question: `ErrDirectionMismatch` still fires as defense-in-depth behind paths that now always offer matching direction — correct to keep, guards `attachRow`/seed against future callers.

### T2.7: Direction-aware reason matching (done) ★

**What exists.** Direction is part of the match: `Category.Direction()` vs stored belief direction; mismatch attaches to an existing opposite-direction belief if present, else counts toward nothing visibly (`accepted:false/ask_again:false`, reason kept, unattached in ledger evidence); never auto-`against`. Writer backstops with `ErrDirectionMismatch`. Mixed-direction tests both levels; offline story all-small matching frozen fixtures; contract shape unchanged (F2 prose fixed: accepted:false no longer implies ask_again:true).

**What surprised.** Review confirmed the pinned rule's unreachable branch: opposite-direction beliefs can't arise via the service (creation needs zero existing fit beliefs) — genuine direction reversal (shopper's size changed, three "far too big" returns) stays unattached forever. User decision (F1): N consecutive opposite-direction classified returns birth a same-kind rival belief (N=3 mirrors the Earned bar) — new tasks T2.8 (offline) + T2.8b (live) in the phase doc.

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

### T3.8: Public shopper page (done)

**What exists.** `agent/frontend/` separate deployment (own spec, ROOT image serving :80): no-login 200 on the public frontend host, chat host untouched, one messaging turn + three page turns with recall-first spans, $0.13 of $0.50. F1–F3 answered in code: root stays, page+serve() compose, separate deployment (runtime spec untouched).

**What surprised.** The 30-turn cap increments before generation (a 502 burns budget too — conservative, left as-is); corrections live in container memory only (redeploy resets — the honest demo reset). Purchases provably unwired.

### T3.7: Public reachability probe (done — reachable: anonymous 200 on the frontend host)

**What exists.** `testdata/astropods/15-public-probe.txt` + research section: minimal page + chat endpoint on `probe-gw` port 80 with `frontend:true, messaging:true`. Anonymous GETs to BOTH the chat host and the dedicated frontend host 302 to the platform SSO login — before the flip, and twice after Public was switched ON (3-minute propagation waits, access-list unchanged, CLI shows no Public flag). Model turn via messaging proxy works ($0.0057, 17 obs, tool span) — the platform path is fine; only the front door ignores Public. Total $0.06 of $0.25.

**What surprised.** The deployment exposes two hostnames (chat + frontend); both 302 anonymously. `ast agent access set` offers `anyone` (= any signed-in user) but no public option. Public is documented only in an adapter-auth warning and has never been observed working — platform gap, not config. The earlier flip landed on `memento-agent` by mistake (user-confirmed; its posture is the user's call).

**The next agent should know.** No-login serving on Astropods is PROVEN on the public frontend host (dedicated `agents.public.astropods.ai` hostname; chat host correctly stays signed-in): anonymous 200 with our page body, model turn $0.0057 with tool spans. Follow-up T3.8 (public shopper page) added to the phase doc, parked until the full demo exists (requires T3.6). memento-agent's own Public posture is the user's call.

### T3.6: The full demo (done) ★

**What exists.** `demo/` scripted driver + reset script + two committed run records: all six steps live (T1.3b read-backs, live Jev, live Lambo Venerable/erased, Channel3 details, 3 deployed-agent turns/run, one real sandbox purchase per run through the orders machine, dispute seller-only, byte-identical recall) — run 2 from reset repeats run 1 (trace ids and reply wording the only differences; turn-1 pending-ask shape differs run-to-run and is tabled, not hidden).

**What surprised.** Review held the run records to honesty: recording twins named, fake-backed agent ids marked fixture state everywhere, pre-correction L served by live recall (not agent-consumed), capture REFUNDED-by-design (not all-COMPLETED), placeholder claims qualified. Standing finding for the video script (T6.4): the agent never consumes the live layer in either run — it demonstrates correction-honoured-M against a fixture twin with the same words. The video must NEVER narrate "the live layer told the agent."

**The next agent should know.** T3.8 (public shopper page, parked) serves the real demo once built; memento-agent is back to invite-only (user confirmed).

### T3.5: Evals (done)

**What exists.** `agent/evals/` 3-judge set (`recall_first`, `correction_honoured`, `approval_before_purchase`) pushed over the T3.1/T3.4 traces, every judge pass-and-fail recorded. Fail cases: errored recall, constructed ignores-correction counterfactual, constructed purchase-without-yes counterfactual (both pairs committed with identical prompts pinned by test, off-platform verdicts True/False as expected). $0.07 server spend, well under guard.

**What surprised.** Review caught a vacuous fail case (feedback turn proving nothing about ignoring corrections) — fixed by constructing the discriminating counterfactual, verified off-platform. Deeper findings, recorded: the judges read reply text, not tool spans (span order stays human-checked); pushing the custom set REPLACED the preset safety judges — re-add (safety three at minimum) as T3.6/T5 follow-up, 3+6=9 fits the cap of 10. `correction_honoured` scores false on every non-recommendation turn by design — scope eval sweeps to recommendation turns or expect red on feedback/approval turns.

### T3.4: Asking the shopper (done) ★

**What exists.** `agent/loops/` policy: ask-one-first-pending, strict explicit-yes gate, approve-then-mirror (local record and transport agree before capture), correction-wins recommendation, transaction-tied feedback; 7 tests incl. fail-closed capture and replay pin to `p3-turns.json`. Live: 6 deployed turns, one conversation — pending poll → reason (fit-too-small accepted) → correction (`corr_live01`, precedence) → honoured next turn → yes-approval (purchase honestly declined, no purchase tool) → fits-well feedback. Traces `879c8752/35598171/cef908ae/fad137db/839514c8/d6e19c5a`; spend $0.19 total (billing $0.08 → $0.27), max turn under the $0.10 guard.

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

**What surprised.** The licence is Apache-2.0, by the user's decision, not MIT. The task's `owns` line named only `web/package.json`, so it was widened to `web/` to cover the app the task writes. An independent review found the web test step vacuous and the Go filter too broad. The web app now carries a real test, the gate checks the lockfile in both directions, and the Go filter drops only `web/node_modules`. `go mod tidy` keeps the Keel requirement because `internal/api/server` and `internal/auth` import `keel/wire` for real (T2.5b later deleted the redundant `internal/version/pin.go` blank import).

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
