# T8.3b: the pool reader, live (driver)

Live sibling of T8.3. It writes a small seeded population to the
test-prefixed index `t83b-fit-rows` on the Elastic deployment T0.12
recorded, reads it back through the pool reader's `product_fit` path, and
records warm and cold latency.

Nothing here runs in CI and nothing here gates a build: there are no
`*_test.go` files under `tests/live/`, so the gate's `go test -race` has no
tests to run here and `go vet` only checks the driver builds. The driver is
a main program, run once by hand.

## What it proves

Eight seeded purchases on the demo line (`brand-x-001` / `shirts` /
`regular`, size `M`) settle into one pool:

| Purchase | Written by | `settled_kind` |
|---|---|---|
| five returns | `writer.Refunded` then `writer.Classified` | `return_reason`, `fit_too_small` |
| one return | `writer.Refunded` then `writer.Classified` | `return_reason`, `fit_too_large` |
| one feedback | `writer.Feedback` | `feedback`, `fits_well` |
| one kept | `writer.Kept` | `kept` |

The pool reads back as Building toward small: seven fit-classified
shoppers meet the three-shopper floor, but five shoppers pointing small are
below the eight-shopper Earned floor. (The kept purchase settles the
denominator without joining the fit-shopper count.) The seeded split
carries the whole pool and the live split is empty; every row settled
inside the recent window, so the recent split equals the whole pool. A second size with no rows reads as a
valid empty pool (Claimed, zero counts, no error).

## What is live, what is recorded

- **Live:** the Elastic deployment. The index is created with
  `fitindex.NewHTTPIndex` and `Create` from the recorded mapping; every
  write is a whole-document `PUT` at the row's external version; the reads
  are `pool.HTTPIndex.Aggregate` through `pool.Reader.Fit`, the
  `product_fit` path, with a 20 s timeout and a keep-alive transport.
- **Recorded:** nothing. The population is synthetic and written directly
  through the fit writer with `Seeded: true`; no payment, classifier or
  model call is involved. Event times are the run's own: captures start an
  hour before the run and each purchase settles minutes later, so every
  `settled_at` falls inside the 90-day recent window.

## How to run

```sh
source tools/env.sh
bash tests/live/pool/run.sh
```

The driver reads `ELASTIC_URL`, `LAMBO_ES_API_KEY` (the elastic keyring
entry) and `MEMENTO_IDENTITY_SALT`. `run.sh` runs the driver, saves the
transcript under `out/` (git-ignored), and then checks the `t83b-` listing is
empty and that no key or deployment location entered any file.

On any 401, throttle or unexpected status: stop and report, do not retry in
a loop.

## Guards

- **Prefix first.** The driver lists `_cat/indices/t83b-*` before creating
  anything and refuses to run when the prefix is not empty, so it never
  deletes an index it did not create.
- **Cleanup always.** The delete runs in a deferred function, so a failed
  check still removes `t83b-fit-rows`. It deletes only that one name, after
  checking the prefix, and then prints the empty prefix listing.
- **No secret reaches a file.** The deployment is printed as
  `<deployment>`; a transport error's URL is redacted before it is printed;
  `run.sh` greps the whole directory for the URL, the key and the salt.

## Spend

No model spend. Elastic usage is one kilobyte-scale index, about two dozen
row writes, a handful of reads, counts and a delete; no billable resource
was created beyond the existing project. The index is deleted and the empty
listing shown.
