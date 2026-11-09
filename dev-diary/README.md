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
| P0 | 2 / 10 | In progress |
| P1 | 0 / 5 | Not started |
| P2 | 0 / 10 | Not started |
| P3 | 0 / 6 | Not started |
| P4 | 0 / 4 | Not started |
| P5 | 0 / 6 | Not started |
| P6 | 0 / 7 | Not started |

---

## Handoff Log

Newest first. What exists, what surprised, and what the next agent must not have to work out again.

### T0.2: Scaffold (done)

**What exists.** An Apache-2.0 licence, a Go module on Keel `v0.9.0` with `internal/version`, and a Svelte 5 SvelteKit app under `web/` on Chaaya `0.6.0`. `tools/check.sh` runs gofmt, `go vet`, `go test -race`, a two-way lockfile check, svelte-check, eslint, vitest and the web build. CI runs the same gate. `tools/env.sh` exports the seven code-read variables from the keyring, leaves a preset variable alone, and names a missing key without printing a value. `AGENTS.md` carries the developer rules, the gate, and every key's `secret-tool lookup` command. `dev-diary/research/README.md` sets the evidence and recording rules.

**What surprised.** The licence is Apache-2.0, by the user's decision, not MIT. The task's `owns` line named only `web/package.json`, so it was widened to `web/` to cover the app the task writes. An independent review found the web test step vacuous and the Go filter too broad. The web app now carries a real test, the gate checks the lockfile in both directions, and the Go filter drops only `web/node_modules`. `go mod tidy` keeps the Keel requirement because `internal/version/pin.go` carries a `tools`-tagged blank import.

**The next agent should know.** The web app builds static with a `200.html` fallback, and T1.1 can change the adapter. CI has not been observed on a push yet, because the branch has not been pushed. The workflow runs the same gate.

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
