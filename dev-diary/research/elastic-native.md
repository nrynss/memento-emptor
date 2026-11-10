# Elastic native embedder probe (T0.6b)

Question: should the demo switch from the settled BGE-M3-via-OpenRouter
contract (`research/elastic.md`) to embeddings created natively through the
Elasticsearch Inference API (Lambo 0.10.0, `kind = "elastic"`)? The settled
contract stands regardless; this file is the stay-or-switch evidence.

Deployment under test: the same Serverless deployment as T0.6. Every request
below sent the key as `Authorization: ApiKey` from the environment, same
placeholders as `testdata/elastic/`. Recordings (vectors omitted, rankings
kept, same rule as T0.6): `testdata/elastic-native/`.

## Native endpoint used

`GET /_inference/_all` lists six `text_embedding` endpoints
(`inference-endpoints.json`). Only `.jina-embeddings-v3` fits both the demo
and Lambo's rule (task type `text_embedding`, float elements, cosine):
native 1024 dims, exactly the settled width. The alternatives are wider
(gemini 3072, OpenAI 1536/3072), smaller (jina-v5-nano 768), or refused by
Lambo (sparse `.elser-2-elastic`, task-type `embedding` clip/multimodal).
A wider endpoint would also force a mapping change; jina-v3 needs none.

`input_type` is accepted (INGEST for documents, SEARCH for queries both
200). This matters more than it looks: without the field the paraphrase
pair scores 0.47; with INGEST/INGEST it scores 0.68. Lambo always sends
the field, so the Lambo path uses the good configuration; a hand client
that omits it gets the bad one.

## Paraphrase cosine: 0.68 either way

| Pair | Settled (BGE-M3 via OpenRouter) | Native (jina-v3 via `_inference`) |
|---|---|---|
| `too tight in the shoulders` / `snug across the chest` | 0.68 (T0.6), 0.6794 re-checked live | 0.6769 |

Head-to-head the number is a wash. The spaces differ underneath: the
settled unrelated floor is 0.45-0.60, the native one is 0.16-0.44, and a
native SEARCH query tops out at 0.48 exact cosine where the settled query
reaches 0.93. Absolute thresholds do not transfer across embedders.

## kNN ranking effect: same shape

Engine kNN over the 8 T0.6 documents with native INGEST vectors and a
native SEARCH query (`native-pair-probe.json`):

native: r2 0.74 > p1 0.65 > r1 0.64 > p2 0.62 > p4 0.62 > p3 0.62 > p5 0.56 > p6 0.55

settled (T0.6): r2 0.93 > r1 0.84 > p3 0.80 > p4 0.77 > p2 0.76 > p1 0.76 > p5 0.758 > p6 0.73

Both put the two reason phrases and all four same-line shirts above both
distractors, and both leave the roomier cut (p6) trailing. The internal
order among the shirts shuffles (p1 second natively, fifth settled), which
is expected across embedding spaces. No ranking gain from switching.

## Cost: OpenRouter is metered, Elastic is not (in-band)

- OpenRouter BGE-M3: the pair re-check cost $1.6e-07 for 16 tokens
  (~$1e-8/token), metered per call in the response `usage`.
- Native: `_inference` responses carry no usage or cost fields at all.
  Nothing in-band says who bills or how much; the spend is console-side.
  Total native volume for this probe was about 30 short inference calls
  plus kilobyte-scale index churn, every index deleted. The user confirms
  the project billing in the Elastic console before any switch (same
  standing check as T0.6).

## Latency: warm is equal, cold is not

Warm (established connection): curl does inference in 1.0-1.3s, OpenRouter
in 1.8s for the pair. No meaningful difference.

Cold is where they diverge, and the divergence is this network, not the
endpoint. Fresh DNS for the deployment hostname stalls ~6.2s (its AAAA
never answers; `dig` times out, `getaddrinfo` waits then falls back to A),
while `openrouter.ai` resolves in 0.08s. Consequences measured:

- Lambo's elastic embedder never connects from here: its connect timeout
  is a hardcoded 5s (`DEFAULT_CONNECT_TIMEOUT`, not settable from
  `lambo.toml`), and name resolution alone takes 6.2s. Two `derive`
  attempts failed after ~17s with `embedder unavailable ... error sending
  request`, nothing written. An isolated reqwest 0.12.28 + rustls client
  (Lambo's exact versions) fails the same way in 5.0s; curl and OpenSSL
  succeed in under 1s.
- The recall tier survived the same stall in T0.6 only because
  `timeout_ms = 20000` covers it. On a network with healthy DNS the native
  embedder should connect like any reqwest client, with one ~1s cold call;
  Lambo's own docs warn that replay waits on a startup liveness embed.

So the settled path works from everywhere DNS is merely functional; the
native path additionally needs resolution under 5s (or a Lambo change to
make the timeout configurable) and otherwise fails every CLI write.

## What a switch needs

Feature flag: build with `embed-elastic` (in 0.10.0 released binaries; a
local build adds it beside `store-sqlite,embed-fixture,embed-bge,
recall-elastic` as T0.6 did for the tier). Proven: a 0.10.0 binary builds
with exactly that set.

`lambo.toml` (placeholders, no values):

```toml
[embedder]
kind = "elastic"
url = "https://<deployment>"
model = ".jina-embeddings-v3"
dim = 1024
api_key_env = "LAMBO_ES_API_KEY"
# send_input_type = true   # default; this deployment accepts it.
# canary = "verify"        # recommended: Elastic can change the model
                           # behind a stable id without changing the id.
```

`url` plus `model` are required and have no defaults; `model` is the
inference endpoint id. `dim` must equal the endpoint width (1024 here, so
no mapping change). The key goes over `ApiKey` on https only. Config
resolution accepts this shape (the failed derives got past resolve to the
network and formed the full inference URL).

Re-embed path: stop the serve holding the session, then with the elastic
`[embedder]` configured run
`lambo re-embed --session <s> --agent <a>`. Every concept is embedded
through the new space BEFORE the graph is touched, and the vector rewrite
plus contract swap flush as one transaction; `--missing-only` backfills
NULL vectors without touching the contract. Not exercised live here: it
embeds through the new embedder, so it hits the same DNS/connect wall on
this network. Any `send_input_type` flip is a different space
(`;input_type=off` in the contract) and needs its own re-embed.

Two standing caveats from the 0.10.0 notes apply to the demo: there is no
calibration row for `kind = "elastic"`, so the measured constants the
demo leans on (merge threshold, the 0.50 layer floor, the
suspected-supersession trigger precision) would need re-measuring on
jina-v3; and one service would then own both of recall's vector
dependencies (embedder plus tier on Elastic).

## Recommendation: stay

> Pointer: this stay verdict was the evidence on Lambo 0.10.0. The user has
> since directed the demo to Elastic-native embeddings now that Lambo 0.10.1
> ships configurable elastic timeouts and a jina-v3 calibration row; the
> P2 live twins (T2.3b/T2.4b) proved the native path end to end. The stay
> reasoning below is kept as measured; the switch itself belongs to a later
> task, which re-verifies calibration on the demo shopper and confirms
> console billing.

Stay on BGE-M3 via OpenRouter. The native path matches the settled number
(0.677 vs 0.68) and the ranking shape with no quality gain, while:

1. it invalidates the calibrated constants with no replacement row;
2. its cost is unmetered in-band (console check still open);
3. it couples both vector dependencies to one service, against the
   issue-233 caveat the project already accepted;
4. it cannot write from this network at all (hardcoded 5s connect timeout
   vs a 6.2s DNS stall), where the settled path resolves in 0.08s;
5. switching forces a `re-embed` of every session for zero measured gain.

Revisit if OpenRouter cost or latency becomes a problem, if the console
shows native inference is trivially billed, and if Lambo makes the
embedder connect timeout configurable (or this network's DNS is fixed) —
then re-measure the calibration constants on jina-v3 before touching the
demo shopper.

## Spend

- OpenRouter: $1.6e-07 (16-token pair re-check). Native-side T0.6-shape
  comparison leg stays far under the ~$0.00001 bound.
- Elastic: about 30 short `_inference` calls plus kilobyte-scale throwaway
  indices, every one deleted (empty listing verified). No billable
  resources, no settings changed. Console-side billing unchecked: user step.
