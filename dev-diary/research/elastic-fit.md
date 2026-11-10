# The Elastic fit index (T0.12)

The gate on pooled product fit and the merchant view: one document per purchase,
keyed by a hash of the transaction ID, updated in place at ingest; settled rows
only; a salted per-line shopper hash; seeded and live rows in one index; and the
line-and-size aggregation fast enough to run on every `recall_for_purchase`.

Deployment: the same Elastic Cloud deployment as T0.6, engine 9.7.0, but it is
an **Elasticsearch Vector Database project**, not a general Elasticsearch
Serverless project (check 8). Every request sent the key as `Authorization:
ApiKey` read from the environment; no value and no deployment URL appears here
or in `testdata/elastic-fit/`. Recordings replace the URL and cluster
identifiers with `<deployment>`, the Lambo binary with `<lambo>`, and the
temporary config directory with `<tmp>`.

Reproduce with:

```sh
source tools/env.sh
LAMBO_BIN=<lambo 0.10.1> python3 testdata/elastic-fit/harness.py
```

The harness is new code in `testdata/elastic-fit/`; it generates the population
from a hand-written truth table, not from another project. Recordings:
`mapping.json`, `states.json`, `population.json`, `aggregations.json`,
`cardinality.json`, `latency.json`, `coexistence.json`, `limits.json`,
`cleanup.json`, `run.json`.

All four indices this task created carried the `t012-` prefix; none survived
(check the last section). No index outside that prefix was created, modified or
deleted. The run began from an empty `t012-` listing (`run.json`); the cancelled
earlier attempt's index was gone.

## 1. Mapping

Index creation, exactly as later tasks create it:

```json
PUT /t012-fit-rows
{
  "mappings": {
    "dynamic": "strict",
    "properties": {
      "txn_hash":         {"type": "keyword"},
      "shopper_hash":     {"type": "keyword"},
      "brand":            {"type": "keyword"},
      "category":         {"type": "keyword"},
      "fit_handle":       {"type": "keyword"},
      "size":             {"type": "keyword"},
      "purchase_state":   {"type": "keyword"},
      "settled":          {"type": "boolean"},
      "settled_kind":     {"type": "keyword"},
      "reason_category":  {"type": "keyword"},
      "fit_direction":    {"type": "keyword"},
      "feedback_verdict": {"type": "keyword"},
      "reason_contested": {"type": "boolean"},
      "seeded":           {"type": "boolean"},
      "captured_at":      {"type": "date"},
      "refunded_at":      {"type": "date"},
      "settled_at":       {"type": "date"},
      "event_seq":        {"type": "integer"}
    }
  }
}
```

The deployment answered `200` and applied its own settings; it needs no
`settings` block from the client:

```json
GET /t012-fit-rows/_settings
{"t012-fit-rows":{"settings":{"index":{
  "mode":"vectordb_document",
  "mapping":{"exclude_source_vectors":"true"}}}}}
```

Fields later tasks build on:

| Field | Type | Meaning and values |
|---|---|---|
| `txn_hash` | keyword | `sha256("txn\|" + transaction id)`; the document `_id` is the same value, so one purchase is one row |
| `shopper_hash` | keyword | salted hash per (shopper, product line): `sha256(<line salt> + "#" + shopper id)`, where the harness's line salt is `brand\|category\|fit_handle` and production adds a deployment secret. The same shopper in two lines gets unrelated hashes |
| `brand`, `category`, `fit_handle` | keyword | the product line. All three are present on every row; a query for a line filters all three |
| `size` | keyword | the purchased size, on the product's own label |
| `purchase_state` | keyword | `captured`, `refunded`, `reason_classified`, `feedback`, `kept`, `contested` |
| `settled` | boolean | product.md's settled rule, evaluated by the writer: returned with a classified reason, fit feedback, or kept past the window |
| `settled_kind` | keyword | `return_reason`, `feedback`, `kept`; absent while unsettled |
| `reason_category` | keyword | `fit_too_small`, `fit_too_large`, `changed_mind`, `damaged`, `not_as_described` (the classifier's categories slot in here) |
| `fit_direction` | keyword | `too_small` / `too_large`; present only for a fit-classified return or a fit feedback verdict. It says the item was too small/large on the buyer |
| `feedback_verdict` | keyword | `fits_well`, `too_small`, `too_large` |
| `reason_contested` | boolean | true once the shopper contests this purchase's reason; the corrected category and direction replace the old values in place |
| `seeded` | boolean | T1.3's seeded mark. Every seeded row carries `true`; the ledger splits on it |
| `captured_at`, `refunded_at`, `settled_at` | date | event times, ISO 8601. `settled_at` is when the row became settled |
| `event_seq` | integer | the writer's per-purchase event sequence; also the external version of the write (check 2) |

No prose field exists. `dynamic: strict` refuses one at write time:

```json
PUT /t012-fit-rows/_doc/prose-probe
{"note": "a prose field the row does not allow"}
-> 400 strict_dynamic_mapping_exception
```

`settled`/`settled_kind` plus the three timestamps are additions the aggregation
needs; product.md names the settled rule and the event times, and the merchant
view needs the reason category. The row still has no free text, no order
detail, no price and no shopper identity. There is no vector field and no
embedding: the fit index stores structured rows only.

## 2. Rows move through states

One index (`t012-fit-rows`) took three purchases through every state. Each
write is a whole-document `PUT` to `_id = txn_hash` with
`version_type=external_gte` and the event sequence as the version.

| Step | Purchase | `purchase_state` | HTTP | `_version` | Docs in index | Docs for this purchase |
|---|---|---|---|---|---|---|
| captured | A | captured | 201 created | 1 | 1 | 1 |
| refunded, no reason | A | refunded | 200 updated | 2 | 1 | 1 |
| reason classified | A | reason_classified | 200 updated | 3 | 1 | 1 |
| captured | B | captured | 201 created | 1 | 2 | 1 |
| fit feedback | B | feedback | 200 updated | 2 | 2 | 1 |
| captured | C | captured | 201 created | 1 | 3 | 1 |
| kept past window | C | kept | 200 updated | 2 | 3 | 1 |
| reason contested | A | contested | 200 updated | 4 | 3 | 1 |
| replay current | A | contested | 200 updated | 4 | 3 | 1 |
| replay older webhook | A | reason_classified (v3) | 409 conflict | — | 3 | 1 |
| strict `external`, equal version | C | kept | 409 conflict | — | 3 | 1 |

The contested row after the last write; its category and direction were
corrected in place, it stayed settled, and it is still one document:

```json
GET /t012-fit-rows/_doc/aaaa…aaaa
{"_version": 4, "found": true, "_source": {
  "purchase_state": "contested", "settled": true,
  "settled_kind": "return_reason",
  "reason_category": "fit_too_large", "fit_direction": "too_large",
  "reason_contested": true,
  "captured_at": "…", "refunded_at": "…", "settled_at": "…"}}
```

**Versioning choice: `external_gte`, version = the event sequence.** A webhook
retry carries the same sequence, so it is accepted (`200`, same version, one
row) and rewrites identical content; a late, older webhook is refused (`409
version_conflict_engine_exception`) and cannot undo a newer state. Strict
`external` refuses the equal-version retry (`409`), which would turn every
delivery retry into an error the writer must classify. `external_gte` needs no
extra idempotency state beyond the sequence it already stores.

The partial `_update` API cannot carry external versioning on this deployment
(check 8), so the writer rewrites the whole row. `event_seq` remains in the row
as the audit value and the version. The recording advanced `_version` on each
write, while the stored rows kept `event_seq` at 1 except the final contested
row, which carried 4; the `_version` column above is the sequence actually
applied. Later tasks should set `event_seq` to the event's own sequence and use
it as the version.

## 3. Settled-only aggregations

Population: **600 purchases** across 3 lines and sizes S, M and L, 588 distinct
shopper hashes. The probe line is `sunspel` / `shirt` / `regular`, size `M`:
120 purchases, built to the truth table below, including 20 unsettled ones.

The one query later tasks reuse (also the merchant view: the
`reason_category` terms is its "why it comes back"):

```json
POST /t012-fit-pop/_search
{
  "size": 0, "track_total_hits": true,
  "query": {"bool": {"filter": [
    {"term": {"brand": "sunspel"}},
    {"term": {"category": "shirt"}},
    {"term": {"fit_handle": "regular"}},
    {"term": {"size": "M"}}
  ]}},
  "aggs": {"split": {
    "filters": {"filters": {
      "all":    {"match_all": {}},
      "seeded": {"term": {"seeded": true}},
      "live":   {"term": {"seeded": false}}
    }},
    "aggs": {"settled": {
      "filter": {"term": {"settled": true}},
      "aggs": {
        "shoppers": {"cardinality": {"field": "shopper_hash"}},
        "settled_kind": {"terms": {"field": "settled_kind"}},
        "reason_category": {"terms": {"field": "reason_category"}},
        "fit_returns": {
          "filter": {"term": {"settled_kind": "return_reason"}},
          "aggs": {"direction": {
            "terms": {"field": "fit_direction"},
            "aggs": {"shoppers": {"cardinality": {"field": "shopper_hash"}}}
          }}
        },
        "feedback": {
          "filter": {"term": {"settled_kind": "feedback"}},
          "aggs": {"verdict": {"terms": {"field": "feedback_verdict"}}}
        }
      }
    }}
  }}
}
```

Truth and response for the probe line, all rows:

| Measure | Expected | Got |
|---|---|---|
| purchases in the line and size | 120 | 120 |
| **settled purchases** (denominator) | 100 | 100 |
| `settled_kind`: return_reason / feedback / kept | 70 / 20 / 10 | 70 / 20 / 10 |
| fit returns by direction: too_small / too_large | 40 / 15 | 40 / 15 |
| feedback by verdict: fits_well / too_small / too_large | 12 / 5 / 3 | 12 / 5 / 3 |
| distinct shoppers per direction: too_small / too_large | 31 / 12 | 31 / 12 |
| reason category: fit_too_small / fit_too_large / changed_mind / damaged / not_as_described | 40 / 15 / 8 / 5 / 2 | 40 / 15 / 8 / 5 / 2 |
| distinct shoppers among settled rows | 88 | 88 |

The server's `took` was 3 ms for this run (1 ms median across the latency
samples in check 6). The totals and every per-bucket count the harness asserts
matched the hand-written truth, so no difference needed explaining. The 88
distinct settled shoppers are the exact count of the generated rows, checked in
check 4.

**Unsettled purchases count toward nothing.** A sampled `captured` row and a
sampled `refunded` row (a refund with no reason yet) are both present with
`settled: false`; adding `{"term":{"settled":true}}` to the very same filter
returns `0`. The line and size contains 120 documents but only 100 settled
purchases, so the denominator is the settled filter's `doc_count`, not the
query's total.

The denominator is settled **purchases**, including a return with a non-fit
classified reason. Only `fit_returns_by_direction` uses `fit_direction`, so a
damaged or changed-mind return stays in the denominator and out of the fit
counts — which is what makes "14 of 60 came back too small" honest.

## 4. Distinct counts

`cardinality` is approximate (HyperLogLog++). Its `precision_threshold`
defaults to 3000 and the parameter's ceiling is 40000. Each row below runs the
check-3 line-and-size filter, adding `settled: true` for the probe rows, or
`match_all` for the whole index, with one `cardinality` aggregation on
`shopper_hash`; `precision_threshold` is set only where the table names it. One
probe sample, with the threshold set:

```json
POST /t012-fit-pop/_search
{"size": 0,
 "query": {"bool": {"filter": [
   {"term": {"brand": "sunspel"}}, {"term": {"category": "shirt"}},
   {"term": {"fit_handle": "regular"}}, {"term": {"size": "M"}},
   {"term": {"settled": true}}]}},
 "aggs": {"distinct": {"cardinality": {"field": "shopper_hash",
                                      "precision_threshold": 40000}}}}
```

| Query | `precision_threshold` | Truth | Value |
|---|---|---|---|
| probe line and size, settled rows | default (3000) | 88 | **88** |
| probe line and size, settled rows | 40000 | 88 | **88** |
| whole index, all rows | default (3000) | 588 | **588** |
| whole index, all rows | 40000 | 588 | **588** |
| whole index, all rows | 100 | 588 | **582** |
| whole index, all rows | 100000 | 588 | **588** |

At this population's size the default is exact in practice, as Elastic's docs
describe: counts are accurate up to the configured threshold, "although not
guaranteed"; above it the count is an estimate. The default stops being exact
above **3000 distinct values**; raising `precision_threshold` to its 40000
maximum moves that boundary to **40000**. The threshold-100 run shows the
estimate in action: 582 for a true 588, about 1% low. The deployment accepted
100000 and returned the exact count, so 40000 remains the ceiling to rely on.

For the demo's population the default is the right setting. If the seeded pool
later grows past 3000 distinct shoppers per line, the query should raise the
threshold to 40000 — or replace the buyer count with a `filter`+`value_count`
over a per-shopper unique document, which is exact at any size.

## 5. Seeded split

The same query's `filters` aggregation, as the ledger shows it:

| Measure | Seeded | Live | All |
|---|---|---|---|
| purchases in the line and size | 100 | 20 | 120 |
| settled (denominator) | 85 | 15 | 100 |
| `settled_kind` return / feedback / kept | 60 / 17 / 8 | 10 / 3 / 2 | 70 / 20 / 10 |
| reason category fit_too_small / fit_too_large / changed_mind / damaged / not_as_described | 35 / 13 / 6 / 4 / 2 | 5 / 2 / 2 / 1 / 0 | 40 / 15 / 8 / 5 / 2 |
| fit returns too_small / too_large | 35 / 13 | 5 / 2 | 40 / 15 |
| feedback fits_well / too_small / too_large | 10 / 4 / 3 | 2 / 1 / 0 | 12 / 5 / 3 |
| distinct shoppers too_small / too_large | 27 / 11 | 4 / 1 | 31 / 12 |
| distinct shoppers among settled rows | 75 | 13 | 88 |

The harness asserts the per-bucket document and settled counts, the fit-direction
and verdict counts, and the per-direction shopper counts against the truth
table, and every one matched. The seeded and live `settled_kind`,
`reason_category` and settled-shopper splits are not in the truth manifest; the
response values (60/17/8 and 10/3/2; 35/13/6/4/2 and 5/2/2/1; 75 and 13) agree
with the rows the generator built, and the 88 total is the exact count checked
in check 4. The population's seeded and live shopper sets are disjoint, so
75 + 13 = 88; a real shopper with both seeded and live rows would be counted
once in the `all` bucket, which is the correct behaviour and the reason the
ledger shows the counts per bucket rather than adding them.

## 6. Speed inside recall

The line-and-size aggregation above, measured from this machine over the
deployment:

| | Samples | Median | p95 | Min | Max | Server `took` (median) |
|---|---|---|---|---|---|---|
| warm, one kept-alive connection | 25 | **236 ms** | 237 ms | 235 ms | 237 ms | 1 ms |
| cold, fresh process and new TLS connection | 8 | **6887 ms** | 6957 ms | 6849 ms | 6975 ms | 1 ms |

All 8 cold samples were over 5 s; no warm sample was. The warm wall time is the
network round trip plus 1 ms of query work; the aggregation itself is not the
cost.

**It needs the `timeout_ms` treatment T0.6 found.** The first request on a cold
connection spends about 6.8–7.0 s (the same ~6.8 s T0.6 measured through
reqwest/rustls, against 0.78 s for curl there), so a client with the 5 s default
would fail the first `recall_for_purchase` aggregation on a cold start. The
fit-index client should set a per-request timeout above the measured cold
maximum — 20000 ms is the value T0.6 settled on for Lambo's recall tier and is
comfortably above this measurement — and should keep a connection pool so
normal calls stay in the 236 ms band. This is a transport timeout, not a query
budget.

## 7. Beside Lambo's recall tier

The fit index and Lambo's recall index share the deployment and nothing else.
Lambo 0.10.1 was pointed at this deployment with `index_prefix = "t012-lambo"`
(so the task's own `t012-` rule holds and T0.11's `t011-` indices are never
addressed), and it provisioned, derived and recalled while the fit index was
live:

```text
$ <lambo> --version
lambo 0.10.1
$ <lambo> provision --config <tmp>/lambo.toml
sqlite schema provisioned (init_schema, idempotent); re-keyed 0 concept(s) for unstemmed identifiers (#148)
$ <lambo> derive --session t012recall --agent t012 \
    --content "Brand X oxford shirts run small through the shoulders" --kind logic --config <tmp>/lambo.toml
derived 1 concept(s): 1 created, 0 matched existing
$ <lambo> recall --session t012recall --query "do these oxford shirts run small?" --config <tmp>/lambo.toml
Brand X oxford shirts run small through the shoulders [Logic] (score 0.43, id e3a1872a)
```

The recall section that names the shared deployment:

```toml
[recall]
kind = "elastic"
url = "<deployment>"
api_key = { env = "LAMBO_ES_API_KEY" }
index_prefix = "t012-lambo"
timeout_ms = 20000
```

Coexistence evidence:

| Check | Result |
|---|---|
| names | fit: `t012-fit-rows`, `t012-fit-pop`; Lambo: `t012-lambo-meta`, `t012-lambo-v-8bb5eef88bb8fc9a` — no shared name |
| aliases | none before or after (`[]`) |
| index templates | 56 before and 56 after, identical list; none matches `t012-*` |
| component templates | 61 before and 61 after, identical list |
| fit index count | the fit population index held 600 documents before and after |
| Lambo recall | answered with the derived concept |

Lambo's data index is contract-keyed (`-v-<embedding contract hash>`) and its
meta index is `{prefix}-meta`; its prefix is an operator setting, so it can be
told to stay out of another task's namespace. The fit index has a fixed name
and no alias, template or data stream linking it to Lambo's indices.

## 8. Serverless limits

This deployment is an **Elasticsearch Vector Database project**. That changes
what it accepts and how it bills; the refusals below are this deployment's
responses, and the billing description is Elastic's docs (no console reading was
provided):

- **Only one index mode.** `index.mode: standard` is refused:
  `400 illegal_argument_exception: index mode [standard] is not allowed for
  vectordb projects; only [vectordb_document] is supported`. New indices get
  `vectordb_document` automatically, and `index.mapping.exclude_source_vectors`
  is `true` and cannot be turned off:
  `400 illegal_argument_exception: [index.mapping.exclude_source_vectors]
  cannot be set to [false] when [index.mode=vectordb_document]`.
- **Structured fields still work.** Keyword, boolean, integer and date fields,
  `_source` retrieval, filters, `_count`, `_bulk`, and `terms` / `filter` /
  `filters` / `cardinality` / `value_count` aggregations all answered normally
  inside `vectordb_document` mode. The vector defaults (bfloat16 vectors,
  32-element dynamic float arrays, vector preload and merge settings) do not
  touch a row with no vector.
- **Update by id.** A whole-document `PUT /<index>/_doc/<id>?version=N&version_type=external_gte`
  is the write path and works. The partial `_update` API refuses external
  versioning: `400 action_request_validation_exception: internal versioning
  can not be used for optimistic concurrency control. Please use if_seq_no and
  if_primary_term instead`. Plain `_update` (no version) works, and
  `_update_by_query` answers 200 with one version conflict and nothing updated;
  neither is the ingest path.
- **Bulk.** One 600-document `_bulk` with external versions returned `errors:
  false`, 600 items, all `201`.
- **Billing differs from Elasticsearch Serverless.** Vector Database projects
  bill on storage (GB, 1 TB maximum per project), reserved search capacity
  (Search Power, with a 16 GB active-storage minimum), ingest volume (GB) and
  an hourly infrastructure fee — not the VCU model of Elasticsearch Serverless.
  Running more queries does not raise the search charge. No console reading was
  provided for this run, so none is recorded; the calls created four
  kilobyte-scale indices, all deleted.
- **Workload mismatch to note.** The project type is tuned for data that changes
  infrequently ("aggressive segment merging improves query speed"), while this
  index updates one row per purchase event. That is a performance
  characteristic, not a refusal; 600 rows and in-place updates caused no
  errors, and warm query cost stayed at 1 ms of server time. If update volume
  grows, measure again rather than assuming the vector-database defaults are
  optimal for a frequently updated structured index.

## What later tasks reuse

- Create the index with the mapping in check 1 and nothing else. The deployment
  supplies the mode and settings.
- Write one row per purchase to `_id = sha256("txn|" + transaction id)`, whole-document,
  `version = event_seq`, `version_type=external_gte`. Treat a `409` as an older
  event that must not win; a same-version write is a harmless retry.
- Set `settled` and `settled_kind` in the writer: settled on a classified
  return reason, fit feedback, or kept past the window. A refund with no reason
  and a purchase still inside the window stay `false`.
- Set `fit_direction` only on fit-classified returns and fit feedback; the
  denominator never uses it.
- The line-and-size query in check 3 is `product_fit`'s source and, with the
  `reason_category` terms, the merchant view. Filter the three line fields plus
  size; the denominator is the `settled` filter's `doc_count`.
- Count distinct shoppers with `cardinality` at the default 3000 up to that
  population; raise it to 40000 past 3000 per line, or move to an exact
  per-shopper row.
- Give the client a timeout above the cold-connection measurement (20000 ms,
  as T0.6 settled for Lambo) and keep connections alive.
- The shopper hash must be salted per line. The harness's synthetic salt is the
  line identity; production adds a deployment secret. The writer must never
  store the raw payer ID, transaction ID or reason text in this index.

The pooled-status thresholds (Building / Earned / Out of true) are not decided
by this task; they are computed over the counts above and fixed before the
comparison runs.

## Spend and cleanup

No model spend: the fit index has no embeddings. Elastic usage was four
kilobyte-scale indices, 600 rows, about 45 `_search` calls plus the `_count` and
listing reads, one 600-row bulk, and one probe document inserted and deleted. No
billable resource was created beyond the existing project, and no console reading
was provided.

Every index this task created was deleted; the empty listing:

```text
GET /_cat/indices/t012-*
{"response": []}
```

Removed: `t012-fit-rows`, `t012-fit-pop`, `t012-lambo-meta`,
`t012-lambo-v-8bb5eef88bb8fc9a`. Recorded in `cleanup.json`.

## Verdict

Checks 1 to 8 pass. The mapping carries every part of the row with no prose
field; the row moves through every state in place with one document per
purchase and a harmless webhook replay; settled-only aggregation matches the
hand-written truth on every number the harness asserts, including distinct
shoppers per direction, and the seeded split is recorded and agrees with the
generated rows; `cardinality` is exact at this population and its approximate
boundary is recorded; the line-and-size query is ~236 ms warm but needs a
timeout above the ~6.9 s cold connection; and the index sits beside Lambo's
recall tier without aliases, templates or a shared name, while Lambo recall
still answers. Nothing is blocked.
