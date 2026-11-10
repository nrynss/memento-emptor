# T2.7b: the direction rule against a real Lambo (live driver)

Live sibling of T2.7. It re-proves the corrected writer
(`internal/memory`: a reason's direction must match the belief's direction,
else `ErrDirectionMismatch` with no graph touch, no `against`) as real
graph writes with T0.3's settings (local session, backdated event times
spanning the extent, GC cadence from the proven toml, receipt waits). The
opposite-direction return is recorded as its own interaction with neither
`depends_on` nor `against` -- what the corrected writer does (refuse,
touch nothing) and what the service does with a match-less answer (store
it unattached, reason kept, never silent). Nothing here runs in CI and
nothing here gates a build: there are no `*_test.go` files under
`tests/live/`, so the gate's `go test -race` has no tests to run.

## What it writes

| Step | Write | Time |
|---|---|---|
| runs-small belief | derive `logic` | day 0 |
| three too-small returns | `record_action` each, `depends_on` the belief | days 3, 6, 9 |
| one too-large return | `record_action` alone, no `depends_on`, no `against` | day 10 |

Days count from an anchor 11 days back, printed as `ANCHOR=` by the
driver, so the three supporting returns span 6 of 11 days (coverage 0.55,
above the 0.3 bar) with 3 distinct dependent interactions. The three
same-direction wordings are T2.4b's frozen returns (`too tight in the
shoulders`, `too tight across the chest`, `sleeves too short in M`); the
opposite return (`far too big, swims on me`) points the other way.

## How to run

```sh
source tools/env.sh
EMBEDDER=native LAMBO_BIN=$HOME/work/lambo/target/release/lambo \
  bash tests/live/direction/run.sh
```

`EMBEDDER` picks the `[embedder]` section the run seeds through; the
config is generated into `out/` (gitignored) so the deployment URL never
enters the repo:

- `native`: `kind = "elastic"` on `.jina-embeddings-v3` (T0.6b's shape,
  plus 0.10.1's `connect_timeout_ms`/`timeout_ms` against the DNS
  stall). Needs `ELASTIC_URL` + `LAMBO_ES_API_KEY`. A scratch-session
  probe runs first; when it reports the embedder down, the script stops
  and the operator reruns with `EMBEDDER=bge_m3`.
- `bge_m3`: the settled T0.6 contract via OpenRouter
  (`baai/bge-m3`, native 1024 dims). Needs `OPENROUTER_API_KEY`.
- `fixture`: the T0.3 local-only contract. Needs no key.

The binary is built (not committed) from the local Lambo checkout at
the 0.10.1 release with the SQLite store plus the elastic
embedder/recall features:

```sh
cargo build --release --features store-sqlite,embed-elastic,recall-elastic
```

No `[recall]` tier is configured: the store's own ranking serves
recall, so the run touches Elastic only through `_inference` (native)
or OpenRouter (fallback). No indices are created; the cleanup step
verifies the `t27b*` listing is empty. The throwaway session is erased
after the checks pass (`erase-session` reports the removed counts), and
an `inspect` afterward finds no concept in the session, proving the
session data is gone.

## Checks

The driver asserts each Done-when clause and exits nonzero on failure:

1. After the third same-direction return: inspect reads
   `[Logic, venerable]`, blast radius 3, the three returns as
   `Dependency` dependents; recall lists the belief; stats reads
   `promotion_policy=Swarm` with mutation cadence sweeps.
2. After the lone opposite-direction return: sizing still inspects
   `[Logic, venerable]` with blast radius 3, the opposite return is
   absent from the belief's neighbourhood (no support), no
   `Contradicts` edge and no contradiction warning (no contradict);
   the opposite return inspects standalone with no edge to the belief.
3. Recall reflects the corrected behavior: the sizing query still
   serves the belief with no `Contradicted` warning, and the opposite
   return is visible on its own query (unattached, never silent).

## Results

Session `t27b-direction`, agent `memento-t27b`, Lambo 0.10.1 binary
built with `--features store-sqlite,embed-elastic,recall-elastic`
(build only; nothing committed). Committed files carry no transcript:
the raw `transcript.jsonl` stays under `out/` (gitignored), scrub-checked
like T2.4b (no key, token, bearer, secret material, no deployment
hostname). Key excerpts are in the worker's handoff report.

- Embedder path that ran: **native**. `[embedder] kind = "elastic"`
  on `.jina-embeddings-v3` (1024 dims, `canary = "verify"`,
  `connect_timeout_ms = 30000`, `timeout_ms = 60000`). The
  scratch-session probe reported up on the first try, so the BGE-M3
  fallback never ran.
- Venerable proof: after the third too-small return, inspect reads
  `Brand X oxford runs small for this shopper: take size L
  [Logic, venerable]`, blast radius 3, the three returns as
  `Dependency` dependents.
- No-attach proof: after the standalone too-large return, the belief
  still inspects `[Logic, venerable]` with blast radius 3, the
  opposite return absent from its neighbourhood, no `Contradicts`
  edge, no warning; the opposite return inspects with no edge to the
  belief.
- Recall proof: the sizing query serves the belief with no
  `Contradicted` warning; the opposite return answers its own query.
- Cleanup: session erased (`erase-session` report under
  `out/run/erase.txt`), post-erase inspect finds no concept in the
  session, `t27b*` index listing empty.
- Spend: $0. No OpenRouter leg ran; native `_inference` calls carry no
  in-band usage; no billable resources created.
