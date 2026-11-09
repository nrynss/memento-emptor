# T2.2b: reason classification, live

Live sibling of T2.2. It classifies the eleven labelled shopper answers
through the real classifier service via the frozen client in
`internal/reasons/` (pinned model `typesafe/jev-1.14`) and compares
category and confidence against the recorded T0.7 table in
`testdata/jev/` and `dev-diary/research/jev.md`.

Nothing here runs in CI and nothing here gates a build: there are no
`*_test.go` files under `tests/live/reasons/`, so the gate's
`go test -race` has no tests to run here and `go vet` only checks the
driver builds. The driver is a main program, run once by hand.

## How to run

```sh
source tools/env.sh
bash tests/live/reasons/run.sh
```

The driver reads the key from `OPENROUTER_API_KEY` and attaches it as
the `Authorization` header; the frozen client itself sets no auth
header, so the wrapper in `main.go` is the only place the key travels.
The key is never printed, written, or committed.

One call per phrase, sequential, with a short pause between calls. The
driver fails loudly on the first transport or shape error and on any
category mismatch or ask-again verdict flip against the recorded table.
Confidence drift on a vague phrase is printed as a DRIFT observation
but does not fail the run: vague answers sample nondeterministically,
and the belief gate rests on the category and the verdict, not the
exact vague-item probability.

## Budget and guards

Eleven calls cost about **$0.00024** total at T0.7's measured
$0.0000214 per call. After the first call the driver stops if the
per-call cost exceeds 10x that reference, or if the endpoint resolves
a different model build than `typesafe/jev-1.14-20261014` (a version
change re-opens the floor). Either stop is reported, not retried.

## Outputs

- Stdout: the per-phrase transcript (answer, recorded vs live
  category and confidence, ask-again verdict, per-call cost) plus the
  total spend and the resolved model string. Kept in the handoff log.
- `out/`: one raw reply per phrase (`01-response.json` …),
  git-ignored. Replies carry per-request generation ids, never the key.
