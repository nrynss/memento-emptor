# T8.1b: the Lambo hub, live (driver)

Live sibling of T8.1. It drives one hub in the deployed shape against the
Elastic deployment T0.12 recorded, with the fixture embedder (in-process
vectors, so the embedding spend is $0.00) and the live recall tier, in the
test-prefixed indices `t81b-*`, and deletes them afterwards.

Nothing here runs in CI and nothing here gates a build: there are no
`*_test.go` files under `tests/live/`, so the gate's `go test -race` has no
tests to run here and `go vet` only checks the driver builds. The driver is
a main program, run once by hand.

## What it proves

Three shoppers created on demand through one hub, each writing two
concepts carrying its own marker word:

| Step | Result |
|---|---|
| one shopper first, spend checked | own marker recalled; $0.00, under the $0.20 budget |
| all three shoppers | each recalls its own marker |
| cross-recall (6 pairs) | no other session's marker in any recall |
| cross-inspect (exact belief content) | no concept matching, every pair |
| seeded demo session (pinned, attached) | none of the test markers, none of the test beliefs |
| ledger reader started with the three sessions | each own belief 200 found=true |
| cross-window read | 200 found=false |
| unscoped alias (the default session is the demo) | 200 found=false for a test belief |
| `GET /api/sessions` | exactly the three allowlisted sessions |
| a shopper created after the reader started | the uniform 404 (allowlist is start-time state, Lambo issue 241) |
| second hub with `max_attached = 2`, every place full | HTTP 503 with `Retry-After`; the hot session still answers |
| 130 concurrent `initialize` | more than 32 held (the old default no longer binds; the flag is 128) |
| concurrent stats burst | service rate and any 429s under `--rate-limit-rps 200` recorded |
| cleanup | every `t81b-*` index deleted, the listing empty, the run dir removed |

## What is live, what is recorded

- **Live:** the Elastic recall tier. Every session's documents are mirrored
  into test-prefixed indices, each query filtered by `session_id`; the
  index listing and the delete are the deployment's own answers. The hub
  and the reader are the 0.10.1 release binary (the deploy's pinned sha),
  run locally in the same shape `deploy/start.sh` boots.
- **Not a model:** the fixture embedder computes vectors in process, so no
  embedding endpoint is called and no embedding spend exists to check
  beyond the $0.00 the transcript states. The native embedder cannot
  connect from here (the DNS stall T0.6b records outlasts its 5 s connect
  timeout), so the live leg is the recall tier, as in T0.11's check 4.
- **Local:** the hub, the reader, the store file and the child logs, all
  under a temp run dir that is removed at the end.

## How to run

```sh
source tools/env.sh
LAMBO_BIN=/path/to/lambo-0.10.1 bash tests/live/hub/run.sh
```

The driver reads `ELASTIC_URL` and `LAMBO_ES_API_KEY` (the elastic keyring
entries). `run.sh` runs the driver, saves the transcript under `out/`
(git-ignored), and then checks the `t81b-` listing is empty and that no key
or deployment location entered any file. The hub token is minted per run
and reaches the children through the environment only.

On any 401, throttle or unexpected status: stop and report, do not retry in
a loop.

## Guards

- **Prefix first.** The driver lists `_cat/indices/t81b-*` before creating
  anything and refuses to run when the prefix is not empty, so it never
  deletes an index it did not create.
- **Cleanup always.** The delete runs in a deferred function, so a failed
  check still removes the test indices. It deletes only names under the
  prefix, then prints the empty listing and removes the run dir.
- **The seeded demo session is left alone.** Nothing is derived in it on
  the hub, and the driver never addresses a deployment path outside
  `t81b-*`; the only assertion about it is that the pinned name equals
  what `internal/identity` builds for `shopper_01` under the seed salt,
  and that it holds none of the test markers.
- **No secret reaches a file.** The deployment is printed as
  `<deployment>`; keys and the per-run token are scrubbed before printing;
  `run.sh` greps the whole directory for the URL and the key.

## Spend

No model spend. Elastic usage is test-prefixed index churn (creates,
mirrored writes, reads, counts, deletes); no billable resource was created
beyond the existing project. The indices are deleted and the empty listing
shown.
