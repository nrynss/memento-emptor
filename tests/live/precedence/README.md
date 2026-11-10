# T8.5b: the layer end to end, live (driver)

Live sibling of T8.5. It boots one hub in the deployed shape against the
Elastic deployment T0.12 recorded, with the fixture embedder (in-process
vectors, so the embedding spend is $0.00) and the live recall tier, in
the test-prefixed indices `t85b-*`. One served layer per shopper rides
its own hub session plus the live pool reader and fit writer on
`t85b-fit-rows`, and decides one purchase per precedence step.

Nothing here runs in CI and nothing here gates a build: there are no
`*_test.go` files under `tests/live/`, so the gate's `go test -race` has
no tests to run here and `go vet` only checks the driver builds. The
driver is a main program, run once by hand.

## What it proves

| Step | Result |
|---|---|
| one case first, spend checked | standing correction decides; $0.00, under the $0.50 budget |
| correction over a fresh hub MCP session | same correction still decides |
| correction after a layer restart | undeclared standing re-recorded, still decides |
| own evidence (three too-large returns) | belief beats the disagreeing Earned-small pool, which rides along overridden with the `ahead of the pool` text |
| pool (new shopper, seeded Earned-small line) | `GuidancePool`, exact counts and seeded split, exact guidance text |
| statement (seeded general statement only) | `GuidanceStatement`, the usual-size text |
| nothing (fresh shopper, lineless recall) | `GuidanceNone`, never a refusal |
| probe: consulted-but-empty pool | pinned: the served assembler rejects the frozen `example-none` shape; owner T8.5 |
| dangling view (declared, restart, rebuild) | pinned: post-restart the view matches nothing and the pool decides; post-rebuild the id is coincidentally reused so the stale view matches by id, not by re-linking; owner T8.5 |
| cleanup | every `t85b-*` index deleted, the listing empty, the run dir removed |

## What is live, what is recorded

- **Live:** the Elastic recall tier (every hub session's documents are
  mirrored into test-prefixed indices, each query filtered by
  `session_id`) and the fit/pool index `t85b-fit-rows` (seeded pool rows
  through the real writer, settled rows through the served layer's own
  writers, reads through the pool reader). The hub is the 0.10.1 release
  binary (the deploy's pinned sha), run locally in the same shape
  `deploy/start.sh` boots. The graph writes (derive, record_action,
  inspect) are the hub's own answers per shopper session; promotion to
  Candidate/Building is the daemon's, polled, not assumed.
- **Not a model:** every shopper answer replays a labelled Jev
  recording (`testdata/jev`), so no classification call is made; the
  fixture embedder computes vectors in process, so no embedding endpoint
  is called. Stated spend is $0.00 by construction; the staged
  one-case-first gate still runs and states it.
- **Recorded payments:** the webhook deliveries replay the P0
  recordings (`fakes.DemoStoryInputs`), whose shapes the T2.1b live leg
  proves identical to the sandbox reads. Webhook verification is T8.7's
  leg; this driver calls `IngestWebhook` directly like the pivot tests.
- **Local:** the hub, the store file, the standing directory and the
  child logs, all under a temp run dir that is removed at the end.

## How to run

```sh
source tools/env.sh
LAMBO_BIN=/path/to/lambo-0.10.1 bash tests/live/precedence/run.sh
```

The driver reads `ELASTIC_URL`, `LAMBO_ES_API_KEY` and
`MEMENTO_IDENTITY_SALT` (the elastic and identity keyring entries).
`run.sh` runs the driver, saves the transcript under `out/`
(git-ignored), and then checks the `t85b-` listing is empty and that no
key or deployment location entered any file. The hub token is minted per
run and reaches the children through the environment only.

On any 401, throttle or unexpected status: stop and report, do not retry
in a loop. The own-evidence step polls recall up to 15 minutes for the
daemon's promotion; a timeout there is a finding, not a cut.

## Guards

- **Prefix first.** The driver lists `_cat/indices/t85b-*` before
  creating anything and refuses to run when the prefix is not empty, so
  it never deletes an index it did not create.
- **Cleanup always.** The delete runs in a deferred function, so a failed
  check still removes the test indices. It deletes only names under the
  prefix, then prints the empty listing and removes the run dir.
- **No secret reaches a file.** The deployment is printed as
  `<deployment>`; keys, the salt and the per-run token are scrubbed
  before printing; `run.sh` greps the whole directory for the URL, the
  key and the salt.

## The pinned findings

Two behaviours are pinned live, each with a tripwire that fails loudly
if the behaviour changes, both owned by T8.5
(`internal/api/server/` is outside this task's `owns`).

- **Consulted-but-empty pool.** The contract's `example-none` shape is
  pool-absent with `pool_lookup: "consulted"`, which is what the lookup
  returns for a wired pool on an empty line. The served assembler
  rejects exactly that pair (`pool absent with lookup "consulted"`), so
  a full-line recall on an empty line fails instead of serving the
  empty pool. The driver attempts it and asserts the failure; a success
  means the guard was fixed and the driver needs updating.
- **Dangling declared views.** A correction declared against a belief
  cites the belief by the layer's in-memory id. After a restart the
  layer mints fresh ids while the restored view keeps the stale one, so
  the view matches no precedence class and the correction drops out of
  recall entirely (the Earned-small pool decides instead). Rebuilding
  the belief through a re-ingest declares nothing — the journal dropped
  the declared entry at persist time — and the rebuilt partition replays
  the same mint order, so the new belief coincidentally reuses the old
  id and the stale view matches by id, not by content re-linking. Any
  divergence in mint order would leave the view dangling or attached to
  an unrelated belief. The driver asserts the post-restart loss of
  force and the coincidental post-rebuild match.

## The shared-transaction caveat

The recorded purchases are reused across test shoppers, so their fit
rows share document ids in the one test index and later writes
overwrite earlier ones (one purchase, one row: in production a purchase
belongs to one shopper). The steps are ordered so no asserted count is
affected: the pool step asserts the seeded-only split before any live
shopper writes, the own-evidence step asserts its seeded/live split
before the declared shopper reuses the same purchases, and the declared
shopper's step asserts no pool counts.

## Spend

No model spend. Elastic usage is test-prefixed index churn (creates,
mirrored writes, reads, counts, deletes); no billable resource was
created beyond the existing project. The indices are deleted and the
empty listing shown.
