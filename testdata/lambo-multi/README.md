# Lambo multi-shopper rig (T0.11)

Harness and recordings for [`dev-diary/research/lambo-multi.md`](../../dev-diary/research/lambo-multi.md):
one `lambo serve` process serving many shopper sessions on demand, SQLite in
one file, the fixture embedder, and the live Elastic recall tier for check 4.

| Path | What it is |
|---|---|
| `mcplib.py` | MCP clients: streamable HTTP per addressed session, and stdio |
| `harness.py` | configs, process supervision, RSS/fd helpers, Elasticsearch calls, transcripts |
| `check1.py` | one process or many; resident memory, handles, per-concept cost |
| `check1b_eviction.py` | `max_attached` eviction, idle detach, re-attach |
| `check2.py` | sessions on demand; the addressed-name rule |
| `check3.py` | many writers, one SQLite store; one writer per session |
| `check4.py` | the Elastic recall tier with many sessions (live; test indices only) |
| `check5.py` | isolation across 20 sessions with overlapping brands |
| `check6.py` | promotion per session, seeded interleaved |
| `check7.py` | the ledger's per-request session choice and its fixed allowlist |
| `lambo-multi.toml` | the config the checks generate, for reading |
| `transcripts/` | every tool call and result, scrubbed |
| `reports/` | the summary each check wrote |

`run-all.sh` replays everything. The binary must be Lambo v0.10.1 built with
the release feature set; the build command is in the script's header. Check 4
needs `ELASTIC_URL` and `LAMBO_ES_API_KEY` (source `tools/env.sh`) and deletes
every index it creates under `t011-multi-`.

No key, deployment URL or bearer token is in any file here. The checks hold
the serving token in the environment only, and the transcripts were scrubbed
for the key, the deployment host, the run directory and the user's home.
