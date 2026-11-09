# T2.4b: the demo shopper's history against a real Lambo (live driver)

Live sibling of T2.4. It seeds the demo shopper's history
(`testdata/seed/histories/demo-shopper.json`) as real graph writes with
T0.3's settings (local session, backdated event times spanning the
extent, GC cadence from the proven toml, receipt waits), then proves the
Done-when clauses: the belief reaches Venerable, and the contradiction
shows on recall. Nothing here runs in CI and nothing here gates a build:
there are no `*_test.go` files under `tests/live/`, so the gate's
`go test -race` has no tests to run.

## What it writes

One write per history row, through the frozen write rule
(`internal/memory`, never modified here). Contents mirror the proven
T0.3 shape, with the fit-return wording taken from the demo fixture's
own notes:

| Step | Write | Time |
|---|---|---|
| sizing belief | derive `logic` | day 0 |
| general statement | derive `observation` | day 0 |
| two kept rows | derive `observation` each | days 1, 2 |
| three fit returns | `record_action` each, `depends_on` the belief | days 3, 6, 9 |
| seller belief | derive `logic` | day 11 |
| non-arrival dispute | `record_action`, `depends_on` only the seller belief | day 11 |
| shopper correction | `record_action`, `against` the sizing belief | write time |

Days count from an anchor 11 days back, printed as `ANCHOR=` by the
driver, so the three returns span 6 of 11 days (coverage 0.55, above the
0.3 bar) with 3 distinct dependent interactions. The correction is
declared at write time and cites `against`, never `depends_on`: it adds
no evidence to the belief it contradicts.

## How to run

```sh
source tools/env.sh
EMBEDDER=native LAMBO_BIN=$HOME/work/lambo/target/release/lambo \
  bash tests/live/memory/run.sh
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
or OpenRouter (fallback). No indices are created; nothing needs
deleting. The loopback `serve-web` reader is started only for the
`/api/inspect` ledger shape, then stopped.

## Checks

The driver asserts each Done-when clause and exits nonzero on failure:

1. After the third return: inspect reads `[Logic, venerable]`, blast
   radius 3, the three returns as `Dependency` dependents; recall lists
   the belief; stats reads `promotion_policy=Swarm` with mutation
   cadence sweeps.
2. After the dispute: sizing still Venerable with the same 3
   dependents; the seller belief inspects below Venerable with the
   dispute beside it.
3. After the correction: sizing stays Venerable with a `Contradicts`
   edge from the correction, blast radius still 3; recall renders the
   `Contradicted ... by` warning naming the correction; the general
   statement never promotes.
4. The web reader's `/api/inspect` for the belief reads
   `status: Venerable`, blast radius 3, gate bars met, admission next
   Canonical and not eligible, and the correction in `warnings`.

## Results

2026-11-09 session `demo-shopper`, agent `memento-t24b`, Lambo 0.10.1
binary built with `--features store-sqlite,embed-elastic,recall-elastic`
(build only; nothing committed). Committed transcript (scrub-checked:
no key, token, bearer, secret material, no deployment hostname) is
`transcript.jsonl` (32 MCP exchanges); ledger shapes are
`api-inspect-belief.json`, `api-inspect-assertion.json`,
`api-inspect-seller.json`.

- Embedder path that ran: **native**. `[embedder] kind = "elastic"` on
  `.jina-embeddings-v3` (1024 dims, `send_input_type` default on,
  `canary = "verify"`, `connect_timeout_ms = 30000`,
  `timeout_ms = 60000`). The scratch-session probe reported up on the
  first try, so the BGE-M3 fallback never ran: 0.10.1's configurable
  connect timeout survives this network's ~6.2 s DNS stall where 0.10.0's
  hardcoded 5 s could not connect at all. No `[recall]` tier was
  configured, so the run touched Elastic only through `_inference`
  embeds: no indices created, none to delete.
- Venerable proof: after the third fit return, inspect reads
  `Brand X oxford runs small for this shopper: take size L
  [Logic, venerable]`, blast radius 3, the three returns as `Dependency`
  dependents; `/api/inspect` reads `status: Venerable`,
  `distinct_interactions` 3.0/3.0, `coverage` 0.5454/0.3,
  `gc_survived` 17.0/3.0, admission next Canonical and not eligible,
  policy Swarm; GC sweeps ran from the file cadence
  (`trigger=mutations`, advancing `last_gc_epoch`).
- Warning rendering: after the `against` correction, inspect keeps
  `[Logic, venerable]` with blast radius 3 and a `Contradicts` edge from
  the correction; recall renders
  `Contradicted at 2026-11-09T20:07:12Z by memento-t24b:
  "Shopper correction: lost weight, take size M for Brand X"` beside the
  belief, and each return carries
  `Rests on a contradicted concept`; the web belief's `warnings` array
  carries the same correction.
- Dispute-adds-nothing check: after the non-arrival dispute citing only
  the seller belief, sizing still inspects Venerable with the same 3
  dependents; the seller belief inspects below Venerable with the
  dispute beside it; the general statement never promotes.
- Spend: $0. No OpenRouter leg ran; native `_inference` calls carry no
  in-band usage; no billable resources created.

Two things the next live run should know. Promotion is eventual: the
first attempt asserted Venerable on the inspect immediately after the
last receipt and caught the span mid-flush (2 of 3), so the driver now
polls inspect to convergence before asserting. And the runner's `AGENT`
variable collided with an exported `AGENT=1` in the operator shell, so
the script names its own `T24B_AGENT`; the committed transcript records
`memento-t24b` throughout.
