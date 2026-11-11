# Lambo promotion on a seeded session (T0.3)

Lambo v0.10.0, local-only: SQLite store plus the `fixture` embedder, so no
key, no network, no spend. Binary built from the v0.10.0 tag with
`--features store-sqlite` (the default build omits the SQLite store and
`serve` refuses with `store kind sqlite is not compiled into this binary`).
Nothing here touched the Elastic deployment.

Every threshold below was re-verified against the 0.10.0 sources before the
run. The 0.9.1 promotion table in product.md still holds exactly: Candidate
needs `gc_survived >= 3` and dependents from 2 other interactions
(`src/canon/stage1.rs`); Venerable needs dependents from at least 3 distinct
other interactions covering at least 0.3 of the session's time extent, with
edges younger than 60 s not counting (`src/canon/stage2.rs`,
`canonization_edge_min_age` default 60 s); Canonical needs blast radius
above 5 outside the 300 s re-promotion cooldown (`src/canon/stage3.rs`,
`canonization_repromotion_cooldown` default 300 s, `EvalParams` from
`Config`). No silent change: the one real 0.9.1-to-0.10.0 difference this
task touches is that `/api/inspect` now carries the contradiction warning
in its `warnings` array (verified below), and `match_strategy` plus the
coverage-window cap are library settings, not file knobs
(`src/config.rs` refuses them under `[daemon]`).

## Setup

Config (`testdata/lambo/lambo.toml`). Cadence only; the bars live in code
and are not settable:

```toml
[store]
kind = "sqlite"
path = "<local run dir>/lambo.db"

[embedder]
kind = "fixture"
dim = 1024
calibration = "off"

[daemon]
gc_interval = 1
gc_idle_floor = 1
gc_max_interval_secs = 3600
canonization_eval_interval_secs = 5
```

`gc_interval = 1` means every mutation triggers a GC sweep; it is the
documented way to watch canonization on a small session
(`lambo.example.toml`: "Lower it if you want to watch the canonization
state machine work on a small swarm session"). It logs one benign warning
at startup: `gc_idle_floor (1) >= gc_interval (1)` disables the time
trigger, which is exactly the intent here. `calibration = "off"` silences
the expected no-calibration-row warning for the unmeasured fixture
contract. Defaults kept: 60 s edge age floor, 300 s re-promotion cooldown,
14-day coverage cap, Swarm policy (no `promotion_policy` key),
`canonization_supersede` off (the default).

Commands, in order (key read from nowhere; there is none):

```
lambo --config lambo.toml provision
lambo --config lambo.toml serve --session t03-shopper-1        # stdio MCP
```

The session id names the shopper. One writer process owns it; every MCP
call below went through that writer. All writes were receipt-waited
(`lambo_stats` with `receipt` plus `wait_ms`, up to the 34000 ms clamp).

## Check 1: three fit returns take the belief to Venerable

Interaction 1, `event_time` ten days back, `lambo_derive` (type `logic`):

- `Brand X oxford runs small for this shopper: take size L`

Same interaction date, a second `lambo_derive` (type `observation`) for
the ledger's Claimed state:

- `Shopper says they are usually size M`

Interactions 2-4, one `lambo_record_action` each, `depends_on` citing the
belief by its exact content, `event_time` at day 3, day 6, day 9 of the
ten-day extent:

- `Fit return 1 ... reason too tight in the shoulders`
- `Fit return 2 ... reason too tight across the chest`
- `Fit return 3 ... reason sleeves too short in M`

Dependent span 6 days over a 9-day session extent: coverage 6/9 = 0.667,
distinct 3. `lambo_inspect` on the belief read, at first poll:

- `focus: ... [Logic, venerable]`, blast radius 3, the three returns as
  `Dependency` dependents
- `admission.checks`: `distinct_interactions` 3.0 >= 3.0 met (stage 2),
  `coverage` 0.6667 >= 0.3 met (stage 2), `blast_radius` 3.0 above-5.0
  unmet (stage 3), cooldown and budget met; `status: Venerable`,
  `next: Canonical`, `eligible: false`, policy `Swarm`

The audit table agrees: `None -> Candidate` then `Candidate -> Venerable`
(one hop per 5 s eval cycle, seconds after the last seeding write).
Promotion ran that fast because seeded edges are dated by their
`event_time`: the 60 s `min_age` floor binds live edges
(`event_time` absent, dated by write time), while seeded history counts as
spread out at once. That is the honest-seeding mechanism product.md
assumes, confirmed in `src/store/memory.rs` (`e.about_time()` eligibility)
against the contradiction detector's stricter write-time floor
(`src/daemon/contradiction.rs`, review M3).

Transcript: `testdata/lambo/01-seed.jsonl` (`initialize`, `tools/list`,
both derives, all three returns with receipt waits, the inspect,
`lambo_recall`, `lambo_stats`).

## Check 2: GC sweeps run from the file's cadence; Swarm is the policy

`lambo_stats` across the run reports `promotion_policy: Swarm` and
`gc.last_sweep.trigger: mutations` with `last_gc_epoch` advancing on every
write (epochs 5, 11, 19, 28, 38 across the five seeding receipts). The
belief's row reads `gc_survived = 12`, `canonization_status = Venerable`;
the Stage-1 gate (`gc_survived >= 3`, dependents from >= 2 other
interactions) is itself proved by the `None -> Candidate` transition.
Nothing was collected (`collected: 0` every sweep).

Swarm is the right policy: it is the default, admits Candidates on
evidence alone (survival plus two other dependent interactions), and every
later hop is its stage's evidence verdict. Solo admits Stage 1 by a
recurrence score band instead of evidence (read from
`src/canon/policy.rs`, not run): a belief that must be *earned* from
returns wants the evidence-led policy, and the demo's future multi-writer
traffic is what Swarm is for.

## Check 3: correction with `against` warns on the Venerable belief

`lambo_record_action` with `against` citing the belief (no `event_time`,
so declared at write time):

- `Shopper correction: lost weight, take size M for Brand X`

After the receipt, `lambo_inspect` still reads `[Logic, venerable]` with
the same three `Dependency` dependents, and hop 1 now also shows a
`Contradicts` edge from the correction. `lambo_recall` renders:

- `Brand X oxford runs small for this shopper: take size L [Logic]
  (score 1.82, id c1360ff5)`
- `⚠ Contradicted at <write time> by memento-t03: "Shopper correction:
  lost weight, take size M for Brand X" (id 73a96fc2)`

and each return carries `⚠ Rests on a contradicted concept`. The belief
stays: status Venerable, blast radius still 3.

`/api/inspect` (serve-web reader beside the writer, loopback) carries the
same warning on 0.10.0. Exact shape, saved as
`testdata/lambo/api-inspect-belief.json`:

```json
{
  "focus": "Brand X oxford runs small for this shopper: take size L",
  "found": true,
  "status": "Venerable",
  "blast_radius": 3,
  "dependents": [
    {"content": "Fit return 1: ...", "concept_type": "Resource", "edge": "Dependency"},
    {"content": "Fit return 2: ...", "concept_type": "Resource", "edge": "Dependency"},
    {"content": "Fit return 3: ...", "concept_type": "Resource", "edge": "Dependency"}
  ],
  "truncated": false,
  "promotion_policy": "Swarm",
  "gate_progress": {
    "policy": "Swarm",
    "gc_survived": {"current": 12.0, "bar": 3.0, "met": true, "strictly_above": false},
    "blast_radius": {"current": 3.0, "bar": 5.0, "met": false, "strictly_above": true},
    "distinct_interactions": {"current": 3.0, "bar": 3.0, "met": true, "strictly_above": false},
    "coverage": {"current": 0.5999, "bar": 0.3, "met": true, "strictly_above": false},
    "in_cooldown": false
  },
  "admission": {"policy": "Swarm", "status": "Venerable", "next": "Canonical",
    "eligible": false, "reasons": ["stage 3: 3 dependents, needs more than 5"]},
  "warnings": [
    "⚠ Contradicted at <write time> by memento-t03: \"Shopper correction: lost weight, take size M for Brand X\" (id 73a96fc2)"
  ]
}
```

Coverage reads 0.5999 here rather than 0.667 because the correction,
dispute and rival interactions below extended the session extent to ten
days while the dependent span stayed six. Still above 0.3, still Venerable.

## Check 4: a non-fit dispute adds nothing to the sizing belief

Derived `Seller Acme ships reliably for this shopper` (type `logic`),
then `lambo_record_action` with `depends_on` citing only the seller
belief:

- `Dispute opened: Brand X parcel never arrived; carrier lost it`

Afterwards the sizing belief still inspects `[Logic, venerable]` with
exactly the same three dependents and blast radius 3. The seller belief
inspects with blast radius 1 and `None -> Candidate refused: survived 2
GC sweeps, needs at least 3; dependents from 1 other interactions, needs
at least 2`. One dispute reaches neither Building nor Earned, and the
sizing belief does not move: the reason (never arrived) routes to the
seller, never to sizing. Transcript in `02-correction-dispute.jsonl`.

## Check 5: ledger-readable states

`/api/inspect` gives the ledger one shape per state (all saved):

| Ledger shows | Lambo reads | Evidence |
|---|---|---|
| Claimed | `status: None`, `blast_radius: 0`, no `warnings` key, admission `dependent_interactions 0.0 < 2.0` | `api-inspect-assertion.json` |
| Building | `status: Candidate` (not reached here; the admission block names the failing gate the same way) | inspect text `None -> Candidate refused` lines |
| Earned | `status: Venerable`, `gate_progress` with all Stage-2 bars met, `admission.next: Canonical, eligible: false` | `api-inspect-belief.json` |
| Out of true | same as Earned plus `warnings: ["⚠ Contradicted ..."]` | `api-inspect-belief.json` |

The seller belief (`status: None`, one dependent) is the dispute's
ledger row: present, counted, not settled.

## Check 6: one session per shopper in practice

A second writer, `serve --session t03-shopper-2` on the same store file:
`lambo_stats` reads `concept_count: 0`, `lambo_recall` for the sizing
query returns nothing, and `lambo_inspect` on the belief's exact content
answers `no concept matching ... in session 't03-shopper-2'`. Sessions
are the isolation boundary: recall, inspect and stats all scope to the
served session, and two shoppers' graphs share a store file without
seeing each other. Transcript: `03-second-session.jsonl`.

## Newer structural evidence against a Venerable belief

The demotion rule (`src/canon/supersession.rs`, issue #20) is written for
Canonical only: a Canonical concept goes `Canonical -> None` when a
contradicting concept earns newer Stage-2-clearing dependent evidence, and
is otherwise untouched. A Venerable belief is never demoted by newer
evidence. What newer evidence does to a Venerable is hold it: with
supersession on, Stage 3 refuses the `Venerable -> Canonical` hop while a
contradicting concept holds newer evidence (`held: contradicted by <id>
with newer evidence`). Supersession defaults off and was off here, so no
probe ran; the code path is read, not exercised.

Demonstrated instead: after the correction, a rival belief plus genuine
structural support were written (`Brand X oxford in size M fits this
shopper now` plus `Fit feedback: Brand X oxford shirt size M fits well`
citing it). The sizing belief stayed `[Logic, venerable]` with the same
three dependents; the rival sits at `status: None` with one dependent
(`api-inspect-rival.json`). Newer support for the other side neither
demotes the Venerable belief nor promotes the rival on one data point.

## What later tasks replay

`testdata/lambo/`: `lambo.toml` (exact config; binary needs
`--features store-sqlite`), `01-seed.jsonl`, `02-correction-dispute.jsonl`,
`03-second-session.jsonl` (every MCP request/response pair: `initialize`,
`tools/list`, `lambo_derive`, `lambo_record_action`, `lambo_recall`,
`lambo_inspect`, `lambo_stats` including receipt waits), and the four
`/api/inspect` responses. No key appears in any of them: the transcripts
were scrub-checked for key, token, bearer and secret material. Replay
needs only the v0.10.0 binary and the toml; the SQLite file itself is not
committed. Concept ids in one transcript (e.g. `c1360ff5`, `73a96fc2`)
are content-addressed the same way across replays, so `depends_on` and
`against` by exact content resolve identically.

## Spend and surroundings

Spend: none. No paid service called; the fixture embedder computes vectors
in-process, SQLite is a local file, serve-web stayed on loopback.

One operational note: a process-list kill during cleanup caught the
machine's shared Lambo dogfood writer (a different rig, session
`lambo-dev`) because its command line matched the pattern. Its supervisor
restarted it at once with the identical pinned invocation; an MCP
`initialize` against it answered healthy on the first try, and this task
never opened its database. Later runs here kill by exact PID only.
