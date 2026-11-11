# T8.2b: the fit index writer, live (driver)

Live sibling of T8.2. It drives the real `fitindex` writer against the
Elastic deployment T0.12 recorded, in the test-prefixed index
`t82b-fit-rows`, and deletes the index afterwards.

Nothing here runs in CI and nothing here gates a build: there are no
`*_test.go` files under `tests/live/`, so the gate's `go test -race` has no
tests to run here and `go vet` only checks the driver builds. The driver is
a main program, run once by hand.

## What it proves

One recorded sandbox purchase runs through capture, refund and a classified
reason into one document, and the document keeps its place while its state
moves:

| Step | Written by | `purchase_state` | External version | `settled` | Docs |
|---|---|---|---|---|---|
| capture delivery | ingest → `writer.Captured` | `captured` | rank 1 | false | 1 |
| refund delivery | ingest → `writer.Refunded` | `refunded` | rank 2 | false | 1 |
| refund delivery replayed | ingest (duplicate) | `refunded` | rank 2 | false | 1 |
| shopper's answer | `writer.Classified` | `reason_classified` | rank 3 | true | 1 |
| refund delivery replayed again | ingest (duplicate) | `reason_classified` | rank 3 | true | 1 |
| refund fit write retried | `writer.Record` | `reason_classified` | rank 3 | true | 1 |
| capture fit write retried | `writer.Record` | `reason_classified` | rank 3 | true | 1 |
| a restarted writer replays the capture | `writer.Record` | `reason_classified` | rank 3 | true | 1 |
| an older version written directly | the index | `reason_classified` | 409 refused | true | 1 |

The version is T7.1's state rank with the event time inside it, and the
index stores it as the document's external version (`external_gte`). The
last row shows the refusal a fresh process gets: the index answers `409`
with `version_conflict_engine_exception` and the newer state stands. The
writer treats that refusal as done, because a delivery's retry must stay
harmless.

Every count at the end is one: one document for the purchase (by
`txn_hash`), one for the shopper's line (by `shopper_hash`), one in the
index.

## What is live, what is recorded

- **Live:** the Elastic deployment. The index is created with
  `fitindex.NewHTTPIndex` and `Create` from the recorded mapping; every
  write is a whole-document `PUT` at the row's external version; the reads,
  counts, the `409` and the delete are the deployment's own answers.
- **Recorded:** the PayPal events. The purchase is the T0.4 cycle the
  recordings hold end to end — order `ORDER-ID-US-1`, captured as
  `CAPTURE-ID-US-1` (`testdata/paypal/capture-completed.json`), refunded as
  `REFUND-ID-US-1` (`testdata/paypal/refund-record.json`, whose `up` link is
  that capture), with the line item from
  `testdata/paypal/order-get-approved-line-items.json`. They reach the
  writer through `ingest.Service` with `WithFitRecorder`, the production
  seam, so each fit row is written from the same event that records the
  evidence.
- **Recorded:** the reason. The shopper's answer is the T0.7 answer "too
  tight in the shoulders"; `fakes.NewRecordingReasonClassifier` replays it
  from `testdata/jev/` as `fit_too_small`, so no classifier call is made.
  `writer.Classified` is where T8.5's `record_reason` will call it.
- **Not exercised here:** webhook signature verification. That is T2.1's
  and T8.7's live leg; this driver's payment leg replays the recorded
  deliveries, and the live leg is the fit index.

The two delivery ids (`WH-T82B-CAPTURE-1`, `WH-T82B-REFUND-1`) key ingest's
idempotency and name the deliveries; they are not purchase state. Every
other id and amount is the recording's placeholder value, and so are the
capture and refund times. The answer's time is the driver's own: it sits
twenty minutes after the refund so the run is deterministic, and it is not
a recorded moment.

## How to run

```sh
source tools/env.sh
bash tests/live/fitindex/run.sh
```

The driver reads `ELASTIC_URL`, `LAMBO_ES_API_KEY` (the elastic keyring
entry) and `MEMENTO_IDENTITY_SALT`. `run.sh` runs the driver, saves the
driver's output as `out/transcript.txt` (git-ignored), and then checks the
`t82b-` listing is empty and that no key or deployment location entered any
file. The prefix and leak checks print to the terminal rather than into that
file, so a transcript that carries them came from the terminal, not from
`out/`.

On any 401, throttle or unexpected status: stop and report, do not retry in
a loop.

## Guards

- **Prefix first.** The driver lists `_cat/indices/t82b-*` before creating
  anything and refuses to run when the prefix is not empty, so it never
  deletes an index it did not create.
- **Cleanup always.** The delete runs in a deferred function, so a failed
  check still removes `t82b-fit-rows`. It deletes only that one name, after
  checking the prefix, and then prints the empty prefix listing.
- **No secret reaches a file.** The deployment is printed as
  `<deployment>`; a transport error's URL is redacted before it is printed;
  `run.sh` greps the whole directory for the URL, the key and the salt.

## Spend

No model spend. Elastic usage is one kilobyte-scale index, three state
writes, three replayed writes, a handful of reads, counts and a delete; no
billable resource was created beyond the existing project. The index is
deleted and the empty listing shown.
