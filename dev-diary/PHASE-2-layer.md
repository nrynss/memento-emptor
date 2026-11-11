# P2: The layer

The product. Evidence arrives from PayPal, reasons are classified, beliefs are earned in Lambo, and any agent can ask what applies. It depends only on PayPal, Lambo, Elastic and Jev.

The live (`b`) tasks below record their transcripts in the worker reports rather than pasting them into the handoff log; the reviewed T2.2b/T2.3b/T2.4b numbers are re-derived from committed recordings or committed transcripts, and every driver is committed with its own assertions. Their Done-when lines were met that way.

Read first: [`../product.md`](../product.md), `../docs/architecture.md`, `../docs/interface.md`.

---

### T2.1: PayPal ingest ★
```yaml
requires:   [T1.4]
fixture-ok: yes
size:       M · mid
owns:       internal/ingest/
status:     done
```

The webhook endpoint, with signature verification. Refunds, disputes and captures become durable evidence rows, idempotent on PayPal's event ID. Line items come from the transaction. A dispute's reason code goes through T1.1's table. A refund with no reason becomes a pending question.

**Done when:** tests cover a redelivered event, a bad signature, an out-of-order refund and capture, and a dispute for each mapped code.

note: worker branch task/t21-ingest; `internal/ingest/` (Service over the fakes.PaymentSource interface, rows keyed by event id, items from the order GET, disputes classified via CategoryForDispute, refunds filed pending with nil reason). All four Done-when cases have tests; `./tools/check.sh` passes in the worktree.

---

### T2.1b: PayPal ingest, live
```yaml
requires:   [T2.1]
fixture-ok: no
size:       XS · mid
owns:       tests/live/ingest/
status:     done
```
A real sandbox refund and dispute reach the running service and produce the right rows.

**Done when:** the transcript is in the handoff log.

note: worker branch task/t21b-live; tests/live/ingest/ (live PaymentSource over REST plus the ingest drive; no *_test.go, no CI impact). Pure read-back of existing T1.3b records, nothing created: newest ledger REFUNDED and DISPUTE.CREATED, stored deliveries re-fetched, both signatures verified live, one refund row with nil reason plus pending question, one dispute row at not_arrived counting toward the seller, both redeliveries duplicate. Transcript is in the worker report; full transcript with real ids in out/ (git-ignored). `./tools/check.sh` passes in the worktree.

---

### T2.2: Reason classification ★
```yaml
requires:   [T1.4]
fixture-ok: yes
size:       S · frontier
owns:       internal/reasons/
status:     done
```
The Jev client behind the interface: the shopper's answer becomes a reason category with a confidence. Below T0.7's floor, the refund stays a pending question and the agent asks again. A refund the shopper never explains stays an outcome with no reason and counts toward nothing. Elastic's nearest labelled reasons are passed in if T0.7 showed that it helps.

**Done when:** tests replay T0.7's labelled set and match its results; the never-explained case is covered.

Notes (worker t22-reasons). `internal/reasons/` holds the client behind the
`fakes.ReasonClassifier` seam plus a pure `Settle` mapping classification to
attached reason or pending question. Replay only: fixture transport serves
`testdata/jev/` with keys asserted absent. Floor re-measured with no drift:
8 clear at 1.00 accepted, 3 vague ask again, margin 0.04; refusal maps to
unclear at 0. Neighbour context omitted by test: all 11 recorded requests
carry the answer alone and still separate, so nothing retrieved travels
with the question.

---

### T2.2b: Reason classification, live
```yaml
requires:   [T2.2]
fixture-ok: no
size:       XS · mid
owns:       tests/live/reasons/
status:     done
```
**Done when:** the labelled set gives T0.7's results on the live service, within the brief's budget; the transcript is in the handoff log.

Notes (worker t22b-live). `tests/live/reasons/` holds a main-program
driver (no `*_test.go`, so CI builds it but never runs it), plus
`run.sh`, `README.md`, and a git-ignored `out/` for raw replies. One
sequential run of all 11 phrases through the frozen client with the key
attached by the driver's transport wrapper: 11/11 categories match the
recorded table, 8 clear attach and 3 vague ask again, floor margin
unchanged at 0.04 (clear 1.00 against vague 0.96). Confidence wobbled
on two vague items (`not great` 0.94→0.93, `meh` 0.68→0.51), both
still `unclear` and far under the floor; verdicts are what gate
beliefs, and the driver fails on any category or verdict mismatch.
Model resolved to `typesafe/jev-1.14-20261014`, identical to T0.7, so
the floor stands. Total $0.000235284 for 5,602 input tokens, exactly
T0.7's total; no guard tripped. Full transcript is in the worker's
report.

---

### T2.3: The evidence store
```yaml
requires:   [T1.4]
fixture-ok: yes
size:       M · mid
owns:       internal/evidence/
status:     done
```
Raw documents in Elastic: shopper messages, classified reasons and product text. Each Lambo evidence concept carries its Elastic document ID, so prose stays out of the graph. Hybrid search answers whether a new product matches the items behind a belief, using what T0.5 found the data can support. Ledger search over the evidence.

**Done when:** tests replay T0.6's queries; a test proves a Lambo write carries a document ID and no prose body.

note: worker branch task/t23-evidence. `internal/evidence/` implements the
`fakes.EvidenceStore` seam: `Document` values (message, reason, product)
keyed by the brand/category/fit-handle line, `IndexDocument` assigning
`doc-0001…` ids, `GraphStub` composing `evidence[<id>] <key>` and nothing
else, `ForReason`/`ForProduct` building the text, and `FuseRRF` for the
combined legs. Tests replay T0.6's recorded legs and prove the stub carries
the id with no prose. Built shape, recorded in T8.1d: the store holds the
texts in process (it is not an Elastic writer), beliefs live in the hub, and
Elastic holds the pooled `fit-rows` index. T2.3c owns whether the raw
evidence persists across a restart; it also owns this package's vectors
comment.

---

### T2.3b: The evidence store, live
```yaml
requires:   [T2.3]
fixture-ok: no
size:       XS · mid
owns:       tests/live/evidence/
status:     done
```

**Done when:** a new SKU matches the demo's returned items on the real deployment; the transcript is in the handoff log.

note: worker branch task/t23b-live. Elastic-native run (user direction):
direct-Elastic legs in `tests/live/evidence/live.py` plus a Lambo leg in
`lambo.sh`, entry `run.sh`, README how-to; no `*_test.go` so CI never runs
it. Direct legs: BM25 for a new-SKU text ranks all four same-line shirts
above both distractors; engine kNN reproduces the recorded native shape
(r2 0.74 > p1 0.65 > r1 0.64, roomier cut last); retriever RRF keeps the
reason phrases top-3 with distractors last; paraphrase cosine 0.674 vs
0.677 native / 0.68 settled. Lambo leg: 0.10.1 binary built with
store-sqlite,embed-fixture,embed-bge,embed-elastic,recall-elastic (build
only, nothing committed); `connect_timeout_ms` knob present and read
(zero refused naming the key); DNS stall from T0.6b gone (0.01s), so the
default-5s failure no longer reproduces here; two derives + backfill +
recall through `.jina-embeddings-v3` with session `calibration=calibrated`
and the 0.86 merge row flagging a 0.88 pair. Every throwaway index deleted,
empty listing verified; spend ~15 short native inference calls, no
in-band cost fields, console billing still a user step.

### T2.3c: Evidence survives a restart, or the loss is recorded
```yaml
requires:   [T2.3, T8.5]
fixture-ok: yes
size:       M · mid
owns:       internal/evidence/, cmd/memento/, product.md, docs/architecture.md
status:     not-started
```
The served layer builds `internal/evidence.NewStore`, which holds documents in process (`docs []Document`), so a restart loses every shopper's raw evidence while the hub keeps beliefs and the pool keeps rows. `product.md` and `docs/architecture.md` describe Elastic as storing and searching raw evidence, but no code path writes evidence documents to Elastic: the live Elastic path is the `fit-rows` pool. Decide and record: persist raw evidence (an Elastic evidence index with its mapping and contract, or a disk-backed store) so a restart keeps the ledger and recall citations, or record the loss in the docs and make the ledger show it plainly instead of an empty panel.

**Done when:** either evidence survives a layer restart with a test that proves it (ledger and recall citations still answer after the restart), or the loss is documented in `product.md` and `docs/architecture.md` with the decision recorded and the ledger's post-restart state stated; no product doc claims Elastic stores raw evidence unless the served layer does; the gate passes.

---

### T2.4: The Lambo writer ★
```yaml
requires:   [T1.4]
fixture-ok: yes
size:       M · frontier
owns:       internal/memory/
status:     done
```
Applies T1.1's write rule. Each reasoned outcome is its own interaction and cites its belief in `depends_on`. Corrections are declared with `against`. General statements go through `derive`. Evidence counts only toward the belief its reason matches. Reads a belief's Lambo state and maps it to the ledger's four statuses.

**Done when:** replaying the demo story gives Earned after the third return, Out of true after the correction with the belief still present, and no change to sizing after the dispute. A test proves a reasonless refund writes nothing to a belief.

Notes: built on branch task/t24-memory in internal/memory/ (doc, graph, writer, read, story + writer tests). Writer takes the cited belief's kind and refuses mismatches with ErrKindMismatch; verdict→kind lives in the writer per T1.2's handoff. Read path returns status Out of true with standing kept. Gate passes. Handoff text is in the worker's report.

---

### T2.4b: The Lambo writer, live ★
```yaml
requires:   [T2.4, T1.3]
fixture-ok: no
size:       S · frontier
owns:       tests/live/memory/
status:     done
```
The demo shopper's history against a real Lambo with T0.3's settings.

**Done when:** the belief really reaches Venerable, and the contradiction shows on recall; the transcript is in the handoff log.

Notes (worker t24b-live). `tests/live/memory/` holds the live driver
(`drive.py` MCP-stdio seeder with receipt waits and convergence polling,
`run.sh` which generates the config into gitignored `out/` so the
deployment URL never enters the repo, `README.md`), the scrubbed
32-exchange `transcript.jsonl`, and the web `api-inspect-*.json` shapes.
Two full native runs, both exit 0. Seeded on the 0.10.1 binary
(`store-sqlite,embed-elastic,recall-elastic`; build only) with
`[embedder] kind = "elastic"` on `.jina-embeddings-v3` plus 0.10.1's
`connect_timeout_ms`/`timeout_ms`: the probe reported up first try, so
the BGE-M3 fallback never ran and no `[recall]` tier was configured
(Elastic touch was `_inference` embeds only; no indices, nothing to
delete). Inspect reads `[Logic, venerable]`, blast radius 3, three
`Dependency` dependents; web gate bars met (distinct 3/3, coverage
0.5454/0.3); recall renders the `Contradicted ... by` warning naming the
correction with the belief kept beside it; the dispute leaves sizing
untouched and the seller belief below Venerable. Spend $0. Surprises for
the next live run: promotion is eventual (the driver polls inspect to
convergence; the first attempt asserted on the immediate inspect and
caught the span mid-flush), and the runner's `AGENT` var collided with
an exported `AGENT=1` in the operator shell (the script uses
`T24B_AGENT`). Full gate result in the report.

---

### T2.5: The interface calls ★
```yaml
requires:   [T2.1, T2.2, T2.3, T2.4]
fixture-ok: yes
size:       M · frontier
owns:       internal/api/server/, cmd/memento/
status:     done
```

Notes (worker t25-server). `internal/api/server/` holds the service: nine frozen tools over MCP (JSON-RPC on POST /mcp: initialize, tools/list from the frozen schemas, tools/call) plus one plain POST /tools/{name} per tool for the ledger, GET /calls[?trace_id=] for the call log, POST /webhooks/payment for ingest, GET /healthz. Plain-route errors use the shared error envelope; MCP tool errors ride inside the result with isError. `cmd/memento/` wires the real pieces behind the fakes' seams (ingest, classifier, evidence store; recording graph) and serves offline with `-seed` loading the frozen demo history. Every call is logged with shopper and trace; recall is assembled through one constructor that fails closed (ErrOverrideMissing) instead of serving a contradicted belief without its override. Responses normalized the tagless domain.Subject keys at the encode boundary, by a decode-rename-reencode walk, so the wire matched the frozen schemas byte-exact; that was true of the code at this task's end and is not the code now — T7.6 put `json:"kind"`, `json:"id"` and `json:"name"` on the domain type, deleted the walk, and the service encodes with a plain marshal.

Proven: all 10 testdata/api examples pass against the service on fakes (TestContractExamplesAgainstService: reads + idempotent correction byte-exact; the two minting writes modulo the minted id, which is asserted present; accepted record_reason replays the recorded clear answer since the fixture's answer text has no P0 recording and the fake never invents one); the story drive (TestStoryEndToEnd) earns Venerable on the third return, contradicts without deleting, leaves sizing struct-equal after the dispute, grows the rival on feedback, and shows recall logged before the correction under one trace; spy-over-fake proves every evidence action is a stub carrying a doc id with no shopper prose. `./tools/check.sh` passes in the worktree; live smoke: the seeded binary serves recall matching the fixture over MCP.

Handoff. (1) The graph seam here is fakes.EvidenceGraph with an unexported 15-line adapter to memory.Graph; the live swap (T2.4b/T2.6) passes the real client behind the same seam and drops the fake, no handler change. (2) Recall serves earned (Venerable) fit beliefs for the brand only; Building beliefs stay on the ledger until earned — confirm this bar with T2.6 before the agent depends on it. (3) internal/version/pin.go's tools-tagged blank import is now redundant (server imports the shared wire package for real); removing it touches version/ so it is a follow-up, not this task. (4) Seller subjects derive as seller-for-{brand} since dispute records carry no seller identity; the reference agent can supply a real seller id when one exists. (5) go.sum gained the shared package's hashes; the gate's two-way lockfile check passes with them.
The service, on Keel, serving T1.2's contract over MCP and the ledger's read endpoints. `recall_for_purchase` returns the correction with precedence over the belief. Every call is logged, so the ledger can show that a recommendation followed a recall.

**Done when:** every example in `testdata/api/` passes against the service on fakes; a test proves that a recall can't return a contradicted belief without its override.

---

### T2.6: The layer end to end ★
```yaml
requires:   [T2.5, T2.1b, T2.2b, T2.3b, T2.4b]
fixture-ok: no
size:       S · frontier
owns:       tests/live/layer/
status:     done
```

Notes (worker t26-live). `tests/live/layer/` holds the live driver
(main-program `main.go`, no `*_test.go`, so CI builds it but never
runs it), `run.sh` orchestrating all three legs, `README.md`, and a
git-ignored `out/`. Leg 1 re-runs the T2.1b ingest driver (read-only,
nothing created): refund `t13b-demo-005` verifies SUCCESS and files
one nil-reason row plus a pending question; dispute `t13b-demo-006`
verifies SUCCESS and writes one `not_arrived` row toward the seller;
both redeliveries duplicate. Leg 2 re-runs the T2.3b evidence driver:
BM25 + native-kNN + RRF legs pass on the real deployment, paraphrase
0.674 in band, throwaway indices deleted. Leg 3 walks the demo story
through the T2.5 service with recording payments (DemoStoryInputs,
whose seam shapes leg 1 shows the live reads also produce), a live Jev
classifier, and a live Lambo 0.10.1 graph client (`lamboGraph` over MCP
stdio, native `.jina-embeddings-v3`, session `t26-layer` erased afterward):
three refunds in, three live classifications all `fit_too_small` at
1.00 — "too tight in the shoulders" ($0.000021336), "the sleeves are
way too short and I can't button it" ($0.000021630), "sleeves too
short in M" ($0.000021420, the frozen story's exact third answer per
`internal/fakes/replay.go` ReturnThree) — Earned/Venerable with `Take
size L` guidance, vague "not great" ask-again (`unclear` 0.94,
$0.000021210), dispute leaves sizing recall byte-identical while
the seller reads Claimed, correction overrides to M at once with
standing Venerable, fits-well feedback accepted, trace-R orders
recall before correction. Spend: Jev $0.000085596 for 4 calls
(0.000021336 + 0.000021630 + 0.000021420 + 0.000021210, pinned build
`typesafe/jev-1.14-20261014`, per-call model+cost guard on every
call); PayPal/Elastic/Lambo $0 billable.
Review follow-ups, all closed: F1 — the third answer was
`fit_too_large` ("it swims on me, far too big") while the frozen
story uses three `fit_too_small` answers; fixed to the exact fixture
wording above and re-run live, Venerable proof intact. F2 — the
spend/model guard skipped the 2nd Jev call (one overwritten `reply`
var); the guard now lives inside `answerReason`, so every live call
is guarded before proceeding. F3 — per-call costs and confidences
are quoted above so the total recomputes.
Surprises: an erased Lambo session id is tombstoned and can never be
served again, so the driver never pre-erases (re-runs need fresh OUT
or SESSION); `grep --exclude-dir` must precede the path or it is
taken as a file.
With no agent: real sandbox events in, a scripted client calling the interface, real Lambo, Elastic and Jev. Refund, pending question, reason, Earned, correction, override, dispute against the seller.

**Done when:** each step's result matches product.md's demo; the transcript is in the handoff log.

---

### T2.7: Direction-aware reason matching ★
```yaml
requires:   [T2.5]
fixture-ok: yes
size:       M · frontier
owns:       internal/domain/, internal/memory/, internal/api/server/
status:     done
```
The writer and the server attach any fit-kind reason to the most-supported fit belief with no direction check (R2 finding F1). Change the rule: a reason's direction must match the belief's direction to count toward it. A `fit_too_large` return does not attach to a runs-small belief; it attaches to an existing same-kind opposite-direction belief if one is present, otherwise it counts toward nothing (visible as unattached with its reason, never silent). No automatic `against` edges — contradictions stay shopper-driven so Venerable semantics are untouched. Update the domain mapping, the memory writer check, and the server's `beliefForReason`; align the offline story's third answer with the frozen all-small fixtures; add mixed-direction tests proving a "too big" return neither supports nor contradicts the runs-small belief. Contract change (user decision): frozen fixtures move in the same commit if the contract shape changes.

**Done when:** mixed-direction tests prove no-attach/no-contradict on mismatch and correct attach on match; the offline story matches the frozen fixtures; all P2 suites green.

Notes (worker t27-direction). Direction is now part of the match in three places: `internal/domain/direction.go` maps each fit category to its way (`fit_too_small`→small, `fit_too_large`→large, everything else nowhere) and reads a belief's way from its composed value; `internal/memory/` refuses a cross-direction offer with new sentinel `ErrDirectionMismatch` (same style as `ErrKindMismatch`, no graph touch, no `against`); `internal/api/server/` routes by direction in `beliefForReason` and stores a match-less classified answer unattached (reason kept, `counts_toward` null, `observe`, empty belief link) with the question closed. The offline story's third answer is all-small again (recorded `too tight in the shoulders`; the mixed-direction third was the R2-reported bug). No wire/shape change: `testdata/` and frozen fixtures untouched. Mixed-direction tests at both levels: writer proves no-support (dependents/standing unchanged) and no-contradict (no `against`, no warning) on mismatch plus attach on match, both ways; service proves the same through `RecordReason`/`LedgerEvidence` (unattached row visible with its reason) plus correct attach when a runs-large belief is present. `./tools/check.sh` passes in the worktree.

---

### T2.7b: Direction rule, live
```yaml
requires:   [T2.7]
fixture-ok: no
size:       XS · frontier
owns:       tests/live/direction/
status:     done
```
Re-prove the corrected writer against live Lambo (native embedder, 0.10.1+): same-direction returns earn, an opposite-direction return attaches nowhere, recall reflects it. Throwaway session, deleted after; spend bounded like T2.4b.

**Done when:** live inspect + recall show the corrected behavior; transcript in the handoff log.

note (worker t27b-live). `tests/live/direction/` holds the live driver
(`drive.py` MCP-stdio seeder with receipt waits and convergence polling,
`run.sh` which generates the config into gitignored `out/` so the
deployment URL never enters the repo, `README.md`), 21-exchange raw
transcript under `out/run/` (gitignored, scrub-checked: no key, token,
bearer, secret material, no deployment hostname). One native run, exit
0, all 18 checks PASS. Seeded on the 0.10.1 binary
(`store-sqlite,embed-elastic,recall-elastic`; build only) with
`[embedder] kind = "elastic"` on `.jina-embeddings-v3` plus 0.10.1's
`connect_timeout_ms`/`timeout_ms`: the scratch-session probe reported up
first try, so the BGE-M3 fallback never ran and no `[recall]` tier was
configured (Elastic touch was `_inference` embeds only; no indices,
`t27b*` listing verified empty). Session `t27b-direction`, agent
`memento-t27b`. Venerable proof: after three too-small returns the
belief inspects `[Logic, venerable]`, blast radius 3, the three returns
as `Dependency` dependents. No-attach proof: the standalone too-large
return (no `depends_on`, no `against`) leaves the belief unchanged on
every asserted property (still `[Logic, venerable]`, blast radius 3,
opposite absent, no `Contradicts`, no warning; the driver checks the
listed properties, not a before/after byte comparison) and inspects
itself as `[Resource]` with `None -> Candidate refused` and only its own
`Derives` edge. Shape note: the standalone return is a graph-level
non-interference probe over direct MCP calls, bypassing `WriteEvidence`
and `RecordReason`; the writer refusal (no graph touch) and the service
routing (match-less answer as an unattached ledger row, no graph write)
are proved offline by T2.7's writer and service tests. Recall
proof: the sizing query serves the belief with no `Contradicted`
warning; the opposite return is visible on its own query (rank/score per
the transcript excerpts; the driver asserts presence only) --
unattached, never silent. Cleanup: `erase-session` removed 1 session
(5 interactions, 5 concepts, 12 edges, 5 write intents, 2
canonization events); post-erase inspect refuses, confirming the session
is gone (counts and listings from gitignored `out/`; committed files
prove the script checks, not the outcome). Spend $0. Surprise: none --
no eventual-promotion retry beyond the inherited convergence poll
(3 polls), no auto-contradiction from the daemon on the directly
opposing wording. Full transcript excerpts in the worker's report.

---

### T2.8: Rival-belief birth rule ★
```yaml
requires:   [T2.7]
fixture-ok: yes
size:       S · frontier
owns:       internal/domain/, internal/memory/, internal/api/server/
status:     done
```
N consecutive opposite-direction classified returns (N=3, mirroring the Earned bar) birth a same-kind opposite-direction rival belief instead of landing unattached. The count resets on any same-direction return or on a correction. The rival starts unpromoted (None/Candidate per the normal gates — birth is not promotion) and the existing belief is untouched (no auto-against, no demotion). Update the domain rule, writer, and router; add consecutive-opposite tests (birth at exactly 3, reset on interruption, no birth at 2).

**Done when:** tests prove birth at 3, reset on interruption, and no birth at 2; the existing belief is unchanged; all P2 suites green.

note (worker t28-rival). Streak state lives in the service's per-shopper state (`streaks` map in `shopperState`, keyed by kind + subject id): runtime state, never beliefs, never the graph. `domain.OppositeStreak` counts consecutive opposite-direction classified returns against `RivalBirthThreshold = 3`; reaching it consumes the run. `beliefForReason` advances the run on a direction mismatch, breaks it on a same-direction attach, and mints the rival via the existing `newBelief` + `fitBeliefValue` (direction set, value derived from the reason) on the Nth consecutive opposite; a directionless fit answer lands unattached without moving the run. `RecordCorrection` clears all streaks for the shopper. The writer is unchanged: the birth path offers matching direction, so `ErrDirectionMismatch` stays the backstop and no against edge is ever declared by birth. Service tests drive seven answers on one shopper through six test-only extra refund events (the frozen story holds four): birth at exactly 3 (rival Claimed/None, 1 row, existing belief at 1 row untouched, 0 against), no birth at 2 (both unattached), reset on interruption (opp, opp, same, opp, opp gives none; next opp gives rival), correction resets (opp, opp, correction, opp, opp gives none; next opp gives rival; only the correction's against edge on the graph). `testdata/` and `internal/fakes/` byte-untouched; all P2 suites green.

---

### T2.8b: Rival birth, live
```yaml
requires:   [T2.8]
fixture-ok: no
size:       XS · frontier
owns:       tests/live/rival/
status:     done
```
Re-prove the rival-birth rule against live Lambo (native embedder, 0.10.1+): three consecutive opposite-direction returns birth the rival; an interrupted streak does not. Throwaway session, deleted after.

**Done when:** live inspect shows the rival born at 3 and absent when interrupted; transcript in the handoff log.

note (worker t28b-live). `tests/live/rival/` holds the live driver
(main-program `main.go`, no `*_test.go`, so CI builds it but never
runs it), `run.sh`, `README.md`, and a git-ignored `out/`. The
scripted client drives the frozen service over recording payments
(`DemoStoryInputs` plus five extra same-shape refund events),
the recorded classifier (`testdata/jev`, both answers at 1.00 — no
live classification service runs), and a live Lambo 0.10.1 graph
client over fresh sessions with native `.jina-embeddings-v3`; the
evidence store is the in-memory lexical store. One case per session
(belief contents carry no shopper id, so the cases would share
concepts): birth in `t28b-rival-birth`, interrupted streak in
`t28b-rival-gap`. Birth: two opposites land unattached
(ledger: 1 fit belief), the third births rival `blf_000002`
(`runs large, take size S`, Claimed/None, 1 row, not contradicted)
while the runs-small belief keeps 1 row, standing None, no
contradiction; live inspect holds the rival (`[Logic]`, blast radius
1, `None -> Candidate refused`, no `Contradicts`, no `Dependency`
to the old belief) and the old belief unchanged (blast radius 1, no
`Contradicts`, no `Dependency` to the rival). Interrupted
(opp, opp, same, opp): all three opposites unattached, 1 fit belief
with 2 rows, 3 unattached too-large rows; rival inspect refuses
(`no concept matching ... in session 't28b-rival-gap'`). Both
sessions erased (birth 4 interactions/4 concepts/10 edges, gap
3/3/7); post-erase inspect refuses; `t28b*` index listing empty.
Spend $0. Surprises: the daemon links the near-duplicate belief
wordings with a symmetric `Semantic` edge (its own similarity
linking, not a service write), so "untouched" is asserted as no
support/no contradiction with blast radius unchanged, not as no
cross-reference; and the two cases need separate sessions (shared
contents across shoppers). Full excerpts in the worker's report.

---

### T2.2c: Record the frozen story's third classifier answer
```yaml
requires:   [T2.2]
fixture-ok: yes
size:       XS · mid
owns:       testdata/jev/, internal/fakes/
status:     done
```
The frozen story's third answer ("sleeves too short in M") has no Jev recording in `testdata/jev/`, so `TestStoryEndToEnd` reuses the "too tight in the shoulders" reply and `fakes.Run` hard-codes its category; the frozen phrase is validated only by T2.6's live leg. Record one classification for that phrase, add it to the fixture set, and let the story replay its own answer.

**Done when:** the third answer has its own recording with the same shape as the other eleven; the replay uses it and the pinned phrase count moves with it; the gate passes.

**Notes (in-review).** The labelled set is now thirteen recordings. `12-fit-too-small-sleeves-too-short-in-m.json` is the frozen third answer, classified at `fit_too_small`, confidence 1.00, on the pinned `typesafe/jev-1.14` resolving to `typesafe/jev-1.14-20261014` as the T0.7 build did. The second file, `13-fit-too-small-tight-across-chest.json`, closes a gap this task found rather than a gap the brief named: `fakes.Run`'s second documented return answer is "too tight across the chest", which was unrecorded too, so the replay could not drop its hard-coded category with one new file. Both files hold the exact request body the client sends and the reply as returned, in the same shape as recordings 1 to 11, with the usage and cost fields intact; the eleven existing files are byte-identical.

`fakes.Run` no longer carries a category per return. It classifies each documented answer through the seam, fails unless the answer is a clear fit reason (`AskAgain` false and `CountsToward` fit), and writes the structural evidence it did before, so the log, waypoints and ledger fixtures are unchanged. `TestReplayOffline` runs the replay through a counting classifier and asserts each of the story's four answers is asked exactly once, which fails if a category is written into the replay again.

The pinned counts moved with the set: `internal/reasons/resolve_test.go`'s labelled-set count and `TestNeighbourContextOmitted`'s recorded-body count read thirteen, and `internal/fakes`' playback table and package doc say thirteen. Both recorded phrases are clear 1.00 fit answers, so the floor measurement, its 0.04 margin and the exclusion of neighbour context are untouched.

Spend: two calls, $0.000042756 in total ($0.000021420 for 510 input tokens and $0.000021336 for 508), which recomputes from the two recordings' own usage. Each sits an order of magnitude under the committed per-call guards, which stop a run at ten times T0.7's $0.0000214 reference call ($0.000214). When this task closed no committed driver classified this pair — `tests/live/layer` classifies the third answer but never the second, and `tests/live/reasons` refused a thirteen-file set (see the finding) — so the tree showed the cost but no guard ran on these two calls. T2.2d moved `tests/live/reasons` to the thirteen-recording set, so a live rerun of that driver now covers both.

Findings, with severity and owner:

- **Low, owner T2.2b (reopen).** `tests/live/reasons` reads every `.json` under `testdata/jev` and then refuses any set that is not eleven phrases (`main.go`'s `len(want) != 11`), and its `run.sh`, `README.md` and total-cost line say eleven too, so no live rerun of T2.2b can start while the set holds thirteen. That driver is T2.2b's own path: reopening it moves the count, or derives the set from the directory instead of pinning it. Closed by T2.2d, which pins thirteen and the thirteen recordings' summed cost, and adds a dry `-check` that proves both before a call.
- **Low, owner T2.2c review (fixed in this review).** `internal/api/server/story_test.go`'s `TestStoryEndToEnd` answered its third return with "too tight in the shoulders" and carried a comment saying T2.2 owned the fixture; it now answers "sleeves too short in M" from its own recording, the comment is gone, and the no-prose assertion names that answer too. `internal/reasons/doc.go`'s "eleven-phrase labelled set" is the T0.7 floor measurement — the eight clear and three vague phrases `TestFloorSeparatesLabelledSet` re-measures — not the fixture count, so it stays as written.

**Review (fresh eyes).** Re-derived from the tree, not the report: the two recordings carry the story's second and third answers with the same request shape as recordings 1 to 11 (identical but for `state.answer`), the eleven older files are byte-identical, the spend recomputes, and no recording carries a key. `fakes.Run` classifies each documented answer through the seam and refuses anything that is not a clear fit reason. Mutation checks: with `12-fit-too-small-sleeves-too-short-in-m.json` moved aside, `TestReplayOffline` fails on the missing recording and `TestStoryEndToEnd` fails in the service; with the classify call replaced by a hard-coded `fit_too_small`, `TestReplayOffline` fails each once-only answer count. `./tools/check.sh` passes.

---

### T2.5b: Two ownerless T2.5 items
```yaml
requires:   [T2.5]
fixture-ok: yes
size:       XS · mid
owns:       internal/version/, internal/domain/, internal/api/server/, docs/architecture.md
status:     done
```
`internal/version/pin.go`'s blank Keel import is redundant (the server and auth import `keel/wire` for real) and its comment ("until the first package imports Keel for real") is now false. Seller subjects are `seller-for-{brand}` because dispute records carry no seller id; decide the seller-id source or record the derivation as the decision.

**Done when:** the redundant import and comment are gone or justified; the seller-subject derivation is either replaced by a real seller id or recorded as the deliberate shape with its reason; the gate passes.

Notes (worker t25b-housekeeping). `internal/version/pin.go` is deleted: the tools-tagged blank import was the file's whole content and the requirement is carried by real imports, so `go mod why -m github.com/nrynss/keel` still resolves through `internal/api/server`, and the default build, a `-tags tools` build, vet and the tests pass without it. `go mod tidy -diff` would prune go.sum's unused module-only hashes but keeps the Keel requirement; go.mod and go.sum are outside this task's owning paths and are unchanged. The seller subject keeps its brand derivation, now recorded as the decision: `docs/architecture.md` "The seller subject: keyed by the brand" states what a dispute actually carries, why the layer does not key on it, and the condition that would change it. `domain.SubjectSeller`'s doc names the derivation and points there. No code path, wire shape or fixture changed.

Proven: `./tools/check.sh` passes in the worktree (gofmt, vet, `go test -race` over every package, both node lockfile checks, svelte-check, eslint, vitest and build for web/ and merchant/). No live calls; spend $0.

Handoff. (1) The premise that dispute records carry no seller id does not hold: `testdata/paypal/dispute-get.json` carries `seller.merchant_id`, `seller.email` and `seller.name` on each disputed transaction, and the capture and order reads carry the same account as `payee.merchant_id`. What the records lack is a per-brand seller, which is the reason the derivation stands. (2) Because of that, `internal/api/server/content.go`'s comment above `sellerSubject` ("Dispute records carry no seller identity") was false: that file was outside this task's `owns`, so `owns` was widened to `internal/api/server/` and d425dfc rewrites the comment to say the record names the payment account. The `owns` line above does not yet name the widened path; the orchestrator records it when landing, because a task definition is the orchestrator's to write. (3) `dev-diary/README.md`'s T1 handoff gives `pin.go`'s blank import as why `go mod tidy` keeps the Keel requirement; the real imports are the reason now. Owner: the orchestrator's landing of this task, since `README.md` is the orchestrator's alone to edit; the sentence becomes that tidy keeps the Keel requirement because the server and auth import `keel/wire` for real. (4) A `go mod tidy` run here would remove go.sum entries nothing needs; the gate never asks for tidy, so they stay until a task owning go.mod and go.sum decides. Closed with no action: the review re-ran `go mod tidy -diff` and it removes 27 module-only `go.sum` lines and keeps both Keel hashes, and no open task owns go.mod or go.sum, so the extra hashes stay and nothing depends on them.

**Review (fresh eyes).** APPROVE, with two corrections committed on the review branch: `docs/architecture.md` now says the committed PayPal copies redact the merchant id to one placeholder, and the handoff items above carry their owners. Re-verified from the committed files on a branch cut at the task tip, no live call, $0. Scope: the two commits touch `internal/version/pin.go` (deleted), `internal/domain/belief.go` (a doc comment), `internal/api/server/content.go` (a comment) and `docs/architecture.md` (a section), plus these lines, and nothing else. `pin.go` is gone cleanly: `go build ./...`, `go build -tags tools ./...` and `go vet ./...` pass, `go mod why -m github.com/nrynss/keel` resolves through `internal/api/server` to `keel/wire`, and `go mod tidy -diff` keeps both Keel hashes and prunes 27 `go.sum` lines, every one a `/go.mod` hash and none a code hash. Mutation: restoring the `tools`-tagged blank import changes neither build and leaves `go mod tidy -diff` byte-identical, so nothing needed it; changing `sellerSubject`'s id from `seller-for-…` to `seller-…` fails `TestStoryEndToEnd` and three `pivot_test.go` cases on the recorded graph content, and both changes were reverted. The decision records honestly: `testdata/paypal/dispute-get.json` carries `seller.merchant_id`, `seller.email` and `seller.name`, the captures and the order read carry the same account as `payee.merchant_id`, and the refund as `payer.merchant_id`, so what the records lack is a per-brand seller, not a seller identity; the section says so, names the marketplace that would change the derivation, and no contract type, wire shape or fixture moved. `content.go`'s comment now says the record names the payment account, one for the whole layer, which matches the fixtures and points at the section. Limits: every merchant id in the committed fixtures is the one redaction placeholder, so the sameness the section cites is checkable as structure rather than as two distinct values, and one disputed transaction is all the committed evidence behind "every disputed transaction". The gate passes at the tip. Findings with owners: the `owns` line above not naming `internal/api/server/` → the orchestrator, when landing; `dev-diary/README.md`'s T1 handoff still giving `pin.go` as the reason tidy keeps Keel → the orchestrator, when landing. Nothing else open.

---

### T2.2d: Move the live reasons driver to the thirteen-phrase set
```yaml
requires:   [T2.2c]
fixture-ok: no
size:       XS · mid
owns:       tests/live/reasons/
status:     done
```
T2.2c took the labelled Jev set to thirteen recordings, but `tests/live/reasons/` still pins eleven: `main.go`'s `len(want) != 11` guard, `run.sh`, the README and the total-cost line. A live rerun would refuse the set before making a call. Move the driver, its script and its README to the thirteen-phrase set; no live call is needed to make the change.

**Done when:** the driver accepts the thirteen-recording set and would run against it; the pinned total matches the recorded costs; the gate passes.

**Notes (in-review).** The driver, its script and its README now read thirteen. `main.go` pins `expectedPhrases = 13` and `expectedTotalCost = 0.000278040`, the exact sum of the thirteen recordings' `response.usage.cost` for 6,620 recorded input tokens: the eleven T0.7 phrases still total $0.000235284, and the two T2.2c recordings add $0.000042756. Each recording is loaded with its cost and input token count, and a fixture without a positive cost is refused rather than silently shortening the total.

A new `-check` mode runs the structural half with no key read and no call sent: it loads the set, refuses anything that is not thirteen recordings, sums the recorded costs against the pin, and refuses a pin that sits over the run's derived bound — the per-call guard times the call count, ten times T0.7's $0.0000214 reference for thirteen calls, $0.002782. `run.sh` runs that check before it asks for the key, so a stale set or a moved pin stops before the first paid phrase. The live transcript prints the recorded set total beside the live one and takes every count from `len(want)`; the hard-coded "of 11", "all 11" and "about $0.00024" wording is gone, and the README states the thirteen-recordings total, the derived run bound and the dry check.

Done-when evidence, dry only, $0 spend. `go run ./tests/live/reasons -check` prints `set: 13 recordings, recorded total 0.000278040 USD for 6620 input tokens, under the 0.002782000 USD run guard`, and `bash tests/live/reasons/run.sh` prints that same line before stopping at the empty key, so the set is accepted and the live path reaches its only remaining precondition. Mutation checks, on a copy of the set or a reverted edit of `main.go`: the set minus `13-fit-too-small-tight-across-chest.json` fails `labelled set holds 12 phrases, want 13`; a pin of 0.000278000 fails `recorded set totals ..., the pin and the recordings disagree`; a zeroed cost in one fixture fails `has no recorded usage cost`; a reference of 0.0000010 fails `pinned total ... is over the 0.000130000 run guard`, which is the derived check biting. `./tools/check.sh` passes in the worktree, with `npm ci` run first in `web/` and `merchant/`.

**Review (fresh eyes), APPROVE.** Re-derived from the tree, not the report: the
thirteen fixtures' `response.usage.cost` sum to $0.000278040 over 6,620 input
tokens; the eleven T0.7 files reproduce `dev-diary/research/jev.md`'s per-phrase
costs and its $0.000235284 for 5,602 tokens exactly, the two T2.2c files add
$0.000042756 and 1,018, and 10 × 0.0000214 × 13 = $0.002782, so the pin, both
counts and the bound all hold. The commit touches only `tests/live/reasons/` and
the T2.2c and T2.2d lines; `testdata/` and `internal/` are identical to `main`.
The two T2.2c sentences the worker corrected read as honest correction: the
past-tense framing ("when this task closed", "refused a thirteen-file set")
names what was true then, and the finding keeps the defect's original text while
recording its closure, so nothing is rewritten out of the record. `-check` takes
no key (the check path reads no environment variable and builds no client) and
sends nothing: it passes with a bogus key exported and with none, and writes no
`out/`. Mutation checks of my own, on copies of the set and a reverted edit of
`main.go`: a set minus one file fails `labelled set holds 12 phrases, want 13`; a
set with a fourteenth file fails `holds 14 phrases, want 13`; one changed cost
fails `recorded set totals ..., the pin and the recordings disagree`; a zeroed
cost fails `has no recorded usage cost`; a reference of 0.0000010 fails the
derived check with `pinned total 0.000278040 USD is over the 0.000130000 run
guard`. With `run.sh`'s check line pointed at the twelve-file copy and no key
set, the script stops on the phrase count and never reaches the key line, so a
stale set cannot spend. `./tools/check.sh` passes. Wording says thirteen
throughout except the two composition sentences ("the eleven T0.7 phrases plus
the two T2.2c recorded"), which the per-file costs above confirm. No date, path,
account detail or key in the change; the commit message is plain.

One finding, low, owner T2.2d (fixed in this review). The README and
`checkSet`'s comment said a run of thirteen "cannot cost more than" the derived
$0.002782, which the guard cannot deliver: an over-guard reply is billed before
the guard reads it, so the guard ends the run before the next call, it does not
cap what the last one costs. Both now call the figure the ceiling a replay of the
recorded set fits under, and say why the pin must sit below it. No behaviour
changed and the printed check line is unchanged, so the evidence above still
matches the tree. Observation, no finding: the 6,620-token figure is printed and
re-derives, but nothing pins it, and no behaviour or guard rests on it; and
`run()` checks the key before the set, so a bare `go run` of the driver refuses
on the key first, while `run.sh` runs the check before either, so the spend order
holds.

---

## Exit Criteria

- [x] PayPal events become evidence, idempotently (T2.1, T2.1b).
- [x] Reasons are classified, and unclear ones ask again (T2.2, T2.2b).
- [x] Raw evidence lives outside the graph, in the layer's own store (T2.3, T2.3b).
- [x] Beliefs are earned, contradicted and kept as product.md says (T2.4, T2.4b).
- [x] The interface serves any MCP client (T2.5).
- [x] The whole story works live with no agent (T2.6).
