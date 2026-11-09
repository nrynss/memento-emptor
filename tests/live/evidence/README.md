# T2.3b: the evidence store, live (Elastic-native run)

Live sibling of T2.3. Nothing here runs in CI and nothing here gates a
build: there are no `*_test.go` files under `tests/live/`, so the gate's
`go test -race` has no tests to run. Two legs, both against the real
Serverless deployment (engine 9.7.0, same as T0.6):

- `live.py`: the T0.6 hybrid legs reproduced with Elastic-native
  embeddings (`.jina-embeddings-v3`, 1024 dims, `input_type` on, exactly
  what Lambo sends). Throwaway index, 8 T0.6 documents, BM25 + kNN +
  retriever-RRF, paraphrase cosine, then the index is deleted.
- `lambo.sh`: a 0.10.1 Lambo binary (build only, `/tmp`; nothing
  committed) deriving and recalling through the native embedder, with
  `connect_timeout_ms` set. Both sessions erased, every `t23b-live*`
  index deleted.

Credentials never enter files: `ELASTIC_URL` / `LAMBO_ES_API_KEY` come
from `source tools/env.sh`. The deployment URL is written into `/tmp`
scratch config only. Raw responses land in `out/` (git-ignored); the
scrubbed transcript is the task report (handoff text), never this file.

## How to run

```sh
source tools/env.sh
bash tests/live/evidence/run.sh
```

`run.sh` runs `live.py` on a fresh `t23b-live-<rand>` index, then
`lambo.sh`. Either leg exits non-zero on any failed assertion, and each
leg deletes its indices before exiting. Re-running creates new
throwaway indices; nothing shared is re-embedded.

## What passing means

- BM25 for a new-SKU-shaped text ranks all four same-line shirts above
  both distractors (the task's Done-when leg).
- Engine kNN reproduces the recorded native shape (`r2 > p1 > r1`,
  roomier cut trailing); retriever RRF keeps the reason phrases top-3
  with the distractors last.
- Paraphrase cosine stays in the paraphrase band (0.674 this run vs
  0.677 native / 0.68 settled).
- Lambo 0.10.1 derives and recalls through the native embedder, the
  session reports `calibration=calibrated`, and the run ends with no
  `t23b*` indices and an empty index listing.
