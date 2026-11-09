# Elastic evidence store (T0.6)

Deployment under test: Elastic Serverless, engine 9.7.0 (`You Know, for Search`).
Every request below sent the key as `Authorization: ApiKey` with the value read
from the environment (`LAMBO_ES_API_KEY`, via `source tools/env.sh`). No value
appears here or in `testdata/elastic/`; the host is written as `<deployment>`,
the cluster and licence ids as placeholders (see `deployment.json`).

## Hybrid search: yes on both legs

Eight documents in a throwaway index (created, queried, deleted): four product
texts in one brand line (three returned for fit, one kept), two distractors,
two bare reason phrases (`hybrid-documents.json`).

- BM25 for a new-SKU-shaped text finds the returned items: the new candidate
  and the three returned shirts rank above both distractors.
- kNN over BGE-M3 vectors (see below) puts `too tight in the shoulders`
  second (0.84) for a fresh `snug across the chest, pulls at the buttons`
  query, ahead of every product text; the returned shirts follow ahead of
  the distractors. Exact cosine of the paraphrase pair is 0.68, clear of the
  unrelated floor (0.45-0.60) and inside BGE-M3's paraphrase band.
- Combined hybrid needs the `retriever: { rrf: ... }` shape on this
  deployment; top-level `rank: { rrf: {} }` is refused with a 400
  `parsing_exception`. The combined ranking puts both reason phrases and all
  four same-line shirts above the distractors.

Lambo does not use server-side RRF (it re-scores neighbours itself), so the
syntax note only matters for hand-written queries. Full rankings:
`testdata/elastic/hybrid-search-results.json`.

## Embedding choice: BGE-M3 through OpenRouter

`baai/bge-m3` is served on OpenRouter's OpenAI-compatible embeddings endpoint
(`POST {openrouter}/v1/embeddings`), probed live: native 1024 dims, response
model `BAAI/bge-m3`, cost $5e-08 for 5 tokens (`openrouter-bge-m3-probe.json`).
It matches Lambo's `bge_m3` adapter exactly (Bearer auth, `{base}/v1/embeddings`
with base `https://openrouter.ai/api`), so the settled configuration is:

```toml
[embedder]
kind = "bge_m3"
dim = 1024
url = "https://openrouter.ai/api"
model = "baai/bge-m3"
api_key_env = "OPENROUTER_API_KEY"
```

Proved end to end: two derives and a paraphrase recall against the deployment
through this embedder (session `t06bge`, transcript in
`lambo-recall-tier.json`). The session reports `calibration=calibrated`, and
naming BGE-M3 with the model keeps the measured suspected-supersession trigger
valid. No `dimensions` request is sent: BGE-M3 serves 1024 natively.

Why not the alternatives:

- Elastic-native embeddings: blocked. [Lambo issue 233](https://github.com/nrynss/lambo/issues/233)
  is open, unassigned, with no PR. Beyond availability, the issue itself warns
  that one service would then own both of recall's dependencies.
- Qwen3-Embedding via OpenRouter: served, but it is not BGE-M3's scale, so
  Lambo's measured constants (merge threshold, 0.50 layer floor, trigger
  precision) would need the Qwen calibration path instead of applying as is.
- In-process candle BGE-M3 or a local llama-server: no second service to pay,
  but weights plus RAM or a sidecar beside the hosted service; heavier ops for
  the demo than one more use of the key already stored.
- Cloudflare Workers AI BGE-M3: needs a new account and key the project does
  not hold.

Re-embedding later (for example if issue 233 ships) is a deliberate, atomic
operation (`lambo re-embed`), so this choice does not lock the demo shopper
in: it only fixes the contract every later session is refused against unless
it matches.

## Lambo recall tier: confirmed against this deployment

The installed binary (0.8.0) does not carry the tier: it refuses `[recall]`
and names `--features recall-elastic`. A 0.9.1 binary built from the local
checkout with `store-sqlite,embed-fixture,embed-bge,recall-elastic` (build
only, nothing committed) passes every check in `lambo-recall-tier.json`:

- index creation: meta index plus one data index per embedding contract with
  Lambo's pinned mapping (`dense_vector`, 1024, cosine, plain float HNSW).
- external-version writes: stale rewrite refused with 409
  `version_conflict_engine_exception`; newer rewrite accepted.
- kNN search against Lambo's mapping.
- `recall-index backfill` repair report.
- erase-session removes the session from the index (0 docs remain) via the
  `_delete_by_query` path.
- fallback: with the tier pointed at a dead endpoint, derive still records
  and recall still answers from the store.

Two findings, both recorded for the build:

1. Default timeout too tight for cold TLS. The first request through
   reqwest/rustls takes about 6.8 s against this deployment (curl: 0.78 s),
   so the default 5 s per-request timeout fails provisioning and the first
   flush copy. `timeout_ms = 20000` in `[recall]` fixes everything, but the
   failure message hides the cause (`error sending request for url (...)`
   with the `TimedOut` source dropped). Proposed Lambo issue text is in the
   handoff log of this task's report: raise or document the default for
   Serverless, and print the source.
2. Serverless runs indices in `vectordb_document` mode with
   `exclude_source_vectors=true`, so Lambo's `_source`-based exact re-scoring
   has nothing to re-score and keeps the engine's HNSW score. Lambo already
   handles this branch; it is the documented accepted difference, widened
   slightly. No action.

## Hosting decision: keep this deployment; start nothing

The brief's framing (start the trial clock now, or self-host) does not apply:
this is not a trial deployment. `GET /_license` reports an active enterprise
licence issued to Elastic Serverless with a system expiry, and
`/_license/trial_status` answers 410 as unavailable in serverless mode
(`deployment.json`). There is no trial to start and no extension to request.

Recommendation: keep the existing Serverless project as the evidence store
and Lambo recall tier, and do not self-host Elasticsearch beside the service.
Self-hosting trades a working, measured index for an always-on stateful
process the demo then owns (disk, memory, upgrades) with no evidence it buys
anything: every tier mechanic above passes on Serverless. The user should
confirm billing for the project in the Elastic console; nothing in this task
predicts that bill beyond "tiny" (below).

## Spend

- OpenRouter embeddings: 214 tokens across the hybrid probes (about $0.000002
  at the metered $5e-08/5 tokens), plus the Lambo end-to-end session (canary
  pin, two derives, one recall; bounded under 1k tokens). Total bounded under
  $0.00001 against the stored key.
- Elastic: a handful of kilobyte-scale test indices on the existing
  deployment, every one deleted (verified empty listing). No billable
  resources created, no trial started, no cluster settings changed.
