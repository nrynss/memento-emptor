# T2.6: the layer end to end, live (driver)

Live sibling of the T2.5 story drive. It walks product.md's demo with
no agent: a scripted client calling the frozen interface, real sandbox
events read back, live Jev classifications, a real Lambo 0.10.1 with
native embeddings, and the real Elastic deployment. Nothing here runs
in CI and nothing here gates a build: there is no `*_test.go` under
`tests/live/`, so the gate's `go test -race` has no tests to run.

## What it proves

Refund in, pending question out, live-classified reason in, Earned
after the third return, a correction that overrides at once while the
belief stands, a dispute that counts only against the seller, fit
feedback tied to the purchase, and the ledger showing every step.

## How to run

```sh
source tools/env.sh
LAMBO_BIN=$HOME/work/lambo/target/release/lambo \
  bash tests/live/layer/run.sh
```

`run.sh` runs three legs in order and stops on the first failure:

1. `tests/live/ingest/run.sh`: one live sandbox refund and one live
   dispute read back from the existing T1.3b history (nothing created)
   through the same ingest code the interface serves.
2. `tests/live/evidence/run.sh`: the hybrid legs on the real Elastic
   deployment (throwaway indices, deleted afterward).
3. The layer driver (`main.go` in this directory): the interface story
   on the T2.5 service with recording payments (DemoStoryInputs, whose
   seam shapes leg 1 shows the live sandbox reads also produce), a live
   Jev classifier, and
   a live Lambo graph client over a fresh `t26-layer` session with
   native `.jina-embeddings-v3` embeddings. The session is erased at
   the end, so its id is spent: a re-run needs a fresh `OUT` directory
   (the store file lives there) or a new `SESSION` value, because an
   erased session id is tombstoned and can never be served again.

## What is reused vs newly created

- Reused, nothing created: the T1.3b sandbox records (leg 1 reads
  them back); the T0.7 phrase table (leg 3's driver checks live Jev
  against it); the T2.3b/T2.4b proof that native inference answers on
  this network.
- Newly created, all trivial: 4 live Jev classifications (about
  $0.0001 total, guarded at 10x the reference per-call cost); native
  `_inference` embeds for one Lambo session (no billable resources,
  session erased); throwaway Elastic indices (deleted by leg 2).

## Placeholders

Real PayPal ids stay in `out/` (git-ignored, mode 0600); stdout
carries fixture ids only, and the driver refuses to finish if a real
id survives redaction. Keys travel only in headers and the generated
Lambo config under `out/`; `run.sh` scans this directory for every
secret value after the run.
