# T2.8b: the rival-birth rule against a real Lambo (live driver)

Live sibling of T2.8. It re-proves the pinned rule
(`internal/domain/rival.go`, `internal/api/server/`: N=3 consecutive
opposite-direction classified returns birth a same-kind
opposite-direction rival belief, unpromoted, with the existing belief
untouched; the count resets on any same-direction return) as real
graph writes with T0.3's settings (fresh sessions, the frozen service,
receipt waits). The classifier is the recorded fake
(`testdata/jev`: both answers replay at 1.00), so no live
classification service runs here; the payments replay the P0
recordings' shapes; only Lambo and the native Elastic embedder are
live. Nothing here runs in CI and nothing here gates a build: there
are no `*_test.go` files under `tests/live/`, so the gate's
`go test -race` has no tests to run.

## What it writes

One case per session: belief contents carry no shopper id, so the two
cases would share concepts in one session and the interrupted case
could never prove absence. `run.sh` drives one case per session and
erases both after.

| Case | Session | Writes |
|---|---|---|
| birth | `t28b-rival-birth` | runs-small belief (derive `logic`, day 0); two too-large returns, each its own interaction citing nothing (unattached); third too-large return births the rival (derive + `depends_on` the rival) |
| gap | `t28b-rival-gap` | runs-small belief; too-large, too-large (unattached); too-small (attaches, resets the run); too-large (unattached, run at 1) |

The birth case uses the frozen story's events (`WH-2EV-01/02/03/05`);
the gap case uses five extra refund events in the same shape
(`WH-2EV-07..11`, built by `extraRefund`).

## How to run

```sh
source tools/env.sh
LAMBO_BIN=$HOME/work/lambo/target/release/lambo \
  bash tests/live/rival/run.sh
```

`CASE` picks one case (`birth` or `gap`); `SESSION` overrides the
session id; `OUT` overrides the scratch directory; `LAMBO_CFG` points
at a pre-generated config. The config is generated into `out/`
(gitignored) so the deployment URL never enters the repo.

The binary is built (not committed) from the local Lambo checkout at
the 0.10.1 release with the SQLite store plus the elastic
embedder/recall features:

```sh
cargo build --release --features store-sqlite,embed-elastic,recall-elastic
```

No `[recall]` tier is configured: the store's own ranking serves
recall, so the run touches Elastic only through `_inference` (native).
No indices are created; the cleanup step verifies the `t28b*`
listing is empty. Both throwaway sessions are erased after the checks
pass (`erase-session` reports the removed counts), and an `inspect`
afterward finds no concept in the session, proving the session data
is gone.

## Checks

The driver asserts each Done-when clause and exits nonzero on failure:

1. Birth: the first two opposites land unattached (not accepted, no
   belief, `fit_too_large`, counts toward nothing) with 1 fit belief
   on the ledger; the third is accepted onto a new rival id. Ledger:
   2 fit beliefs; rival `runs large, take size S`, Claimed/None, 1
   row, not contradicted; existing belief 1 row, not contradicted,
   standing None.
2. Birth, live: inspect of the rival content holds the rival with
   blast radius 1, `None -> Candidate refused` (birth is not
   promotion), no `Contradicts`, and no `Dependency` edge to the old
   belief. Inspect of the old belief still holds it with blast radius
   1, no `Contradicts`, and no `Dependency` edge to the rival.
3. Gap: opposite, opposite, same (attaches to the runs-small belief),
   opposite — all three opposites unattached, 1 fit belief with 2
   rows, 3 unattached too-large rows, nothing contradicted.
4. Gap, live: inspect of the rival content refuses — no such concept
   in the session — while the runs-small belief is suggested.

## Results

Sessions `t28b-rival-birth` / `t28b-rival-gap`, agent `memento-t28b`,
Lambo 0.10.1 binary built with
`--features store-sqlite,embed-elastic,recall-elastic` (build only;
nothing committed). Committed files carry no transcript: the raw
inspect excerpts stay under `out/` (gitignored), scrub-checked like
T2.4b (no key, token, bearer, secret material, no deployment
hostname). Key excerpts are in the worker's handoff report.
Everything below this point rests on the gitignored run outputs and
that report, not on committed files: a reader without them can verify
what the driver checks (from `main.go`), but not the live outcome.

- Embedder path that ran: **native**. `[embedder] kind = "elastic"`
  on `.jina-embeddings-v3` (1024 dims, `canary = "verify"`,
  `connect_timeout_ms = 30000`, `timeout_ms = 60000`). No probe
  fallback exists on this task; an unreachable embedder fails the
  first receipt wait instead of seeding.
- Birth proof: ledger holds 2 fit beliefs; the rival inspects
  `[Logic]`, blast radius 1, its birth return as the only
  `Dependency` dependent, `None -> Candidate refused`; the old
  belief inspects with blast radius 1, its own return as the only
  dependent, no `Contradicts` edge, no warning.
- Interrupted proof: ledger holds 1 fit belief with 2 rows and 3
  unattached too-large rows; rival inspect answers `no concept
  matching ... in session 't28b-rival-gap'`, suggesting only the
  runs-small belief.
- Cleanup: both sessions erased (`erase-session` reports under
  `out/erase-<case>.txt`: birth 4 interactions / 4 concepts / 10
  edges, gap 3 / 3 / 7); post-erase inspect refuses; `t28b*` index
  listing empty.
- Spend: $0. The classifier is the recorded fake and the payments
  are recording fakes; native `_inference` calls carry no in-band
  usage and no billable resources were created.

## Surprises

- The daemon links the two near-duplicate belief wordings with a
  symmetric `Semantic` edge in both neighbourhoods (its own
  similarity linking, not a service write). The pinned rule's
  "untouched" means no support and no contradiction, so the driver
  asserts exactly that — blast radius unchanged at 1 each way, no
  `Dependency` edge between the beliefs, no `Contradicts` anywhere —
  rather than the absence of any cross-reference. (The T2.7b probe
  never surfaced this: its lone opposite return was a longer,
  less-similar sentence and drew no such edge.)
- Belief contents carry no shopper id, so the graph is one shared
  world across shoppers: the two cases must run in separate sessions
  or the gap case inspects the birth case's rival. `CASE` selects the
  case; `run.sh` runs both and erases both.
