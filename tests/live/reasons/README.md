# T2.2b: reason classification, live

Live sibling of T2.2. It classifies the thirteen labelled shopper
answers through the real classifier service via the frozen client in
`internal/reasons/` (pinned model `typesafe/jev-1.14`) and compares
category and confidence against the recorded table in `testdata/jev/`
and `dev-diary/research/jev.md`: the eleven T0.7 phrases plus the two
T2.2c recorded for the frozen story's returns.

Nothing here runs in CI and nothing here gates a build: there are no
`*_test.go` files under `tests/live/reasons/`, so the gate's
`go test -race` has no tests to run here and `go vet` only checks the
driver builds. The driver is a main program, run once by hand.

## How to run

```sh
source tools/env.sh
bash tests/live/reasons/run.sh
```

The script first runs the driver's `-check` mode, which loads the set,
refuses anything that is not the thirteen pinned recordings and proves
the pinned cost total is their own recorded sum. It reads no key and
sends nothing, so a stale set or a drifted pin stops before the first
paid call. To run that half on its own:

```sh
go run ./tests/live/reasons -check
```

The live driver reads the key from `OPENROUTER_API_KEY` and attaches it
as the `Authorization` header; the frozen client itself sets no auth
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

The thirteen recordings' own usage costs sum to **$0.000278040** for
6,620 input tokens, about T0.7's measured $0.0000214 per call, and the
driver pins that total. `-check` sums the recorded costs, compares them
with the pin, and refuses a set that is not thirteen recordings or a pin
that disagrees with them.

On every call the driver stops if the per-call cost exceeds 10x that
reference. The over-guard reply is already billed when the guard reads
it, so the guard ends the run before the next call rather than capping
what one reply can cost. The guard times the call count, **$0.002782**,
is the ceiling a replay of the recorded set fits under, since every
recording sits far below the per-call guard; `-check` refuses the set as
well if the pin ever sits over that ceiling, which would mean the pin or
the guard is wrong. On the first call the driver also stops if the
endpoint resolves a different model build than
`typesafe/jev-1.14-20261014` (a version change re-opens the floor). A
later reply that resolves another build is counted as a mismatch and
fails the run at the end. Either stop is reported, not retried.

## Outputs

- Stdout: the set check line (recordings, recorded total, recorded
  tokens, run guard), then the per-phrase transcript (answer, recorded
  vs live category and confidence, ask-again verdict, per-call cost)
  plus the live total, the recorded set total and the resolved model
  string. Kept in the handoff log.
- `out/`: one raw reply per phrase (`01-response.json` …),
  git-ignored. Replies carry per-request generation ids, never the key.
