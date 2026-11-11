# Interface

Memento Emptor is a memory layer for shopping agents, not an agent. Your
agent plugs into it: evidence arrives from the payment side without your
agent doing anything, your agent asks what applies before it recommends,
and it reports what the shopper says. This document is written for a
developer building a shopping agent on the payment toolkit, and for the
seller running the merchant app. It says what to call, when, and what
comes back.

Every example in this document validates against the frozen schemas in
`testdata/api/`, and the gate checks that. Each fenced block marked
`validate=` is byte-identical to its fixture file, not only equal after
parsing: a reformatting of either side fails the build. The schemas are
final: the tool names, the request shapes and the response shapes do not
change without a new contract task.

## Calling convention

The surface is eleven MCP tools: eight layer calls and three ledger
reads. Discover them with `tools/list`; call them with `tools/call`.
Shopper calls are scoped to one shopper by `shopper_id`. Sessions isolate
shoppers, so one shopper's beliefs never leak into another's recall.

Reads (`explain_belief`, `product_fit`, the three
`ledger_*` reads) take no trace id. `recall_for_purchase` is a read and
accepts one: pass the turn's trace with it, so the trace shows recall ran
before the recommendation. Writes (`record_correction`, `record_reason`,
`record_feedback`, `set_pooling`) take an optional `trace_id`: pass the
host trace for the turn, so per-trace review can show which turn recorded
what. The response echoes it back. The reference host records feedback
per trace and attaches traces of interest as dataset items; the trace id
is the join between a write and the turn that made it.

`product_fit` is the merchant's read. It is the only pooled read the
merchant app is given, and it carries no shopper identity, no purchase row
and no free text. The merchant key opens this tool and nothing else.

The layer never moves money. Orders, approval and double-buy protection
live in your agent, on the toolkit's orders tools with idempotent
request ids. The layer only remembers what happened to the money.

## The turn pattern

1. **Recall first.** Before any turn that may end in a recommendation,
   call `recall_for_purchase`. Your trace should show recall before the
   recommendation: that ordering is the independent proof your agent
   checked memory first.
2. **Recommend from the guidance.** The response carries `guidance`: the
   size or belief to recommend now, and `basis`, the precedence step that
   decided it. Each item in the response carries its own `status` label
   and its `precedence` step; when steps disagree, both are returned and
   the lower one is marked `overridden`. When `overriding_correction` is
   present, recommend from the correction. It is also embedded in each
   contradicted belief, so it cannot be read past.
3. **Ask about the gaps.** `pending_count` above zero means refunds are
   waiting on the shopper. Call `pending_questions` and ask about the
   first one in chat. An unanswered refund counts toward nothing.
4. **Record what the shopper says.** Corrections go to
   `record_correction` at once; answers go to `record_reason`; how the
   purchased size worked out goes to `record_feedback` after delivery.
   A general statement — "I'm usually an M" — has no tool: statements
   arrive only from the seed (see "Where the general statement comes
   from").
5. **Honour consent.** Pooling is opt-out. When the shopper asks to be
   left out of it, call `set_pooling` with `pooling: false`; their rows
   leave the pool and no new ones are written.

## recall_for_purchase

Call it before recommending. Brand scopes the recall; line, category, fit
handle, SKU and the leaned-toward size narrow it when known. The brand,
the category and the fit handle together name the product line the pool
is keyed by, so pass them when the candidate is known. It returns the
shopper's own beliefs, the pooled product fit when the pool has something
to say, and any correction that overrides them. **It never refuses**:
thin evidence comes back labelled thin, and an empty answer says so
plainly.

Request:

```json validate=recall_for_purchase.request.example
{
  "shopper_id": "shopper_01",
  "brand_id": "brand-x-001",
  "line_name": "oxford shirt",
  "sku": "PSW8GeK",
  "size_considered": "L",
  "trace_id": "trace_3f8a01"
}
```

`category` and `fit_handle` are optional and omitted above; pass them when
the candidate is known, because the pool is keyed by the brand, category
and fit handle together. `line_name` and `sku` narrow the shopper's own
beliefs; they do not name a pool line on their own.

Response. The belief says size L, Earned over three documented returns,
but the shopper's correction says M — so the status reads Out of true
while the standing stays Venerable, the warning rides inline, the
correction ships again at the top level, and the guidance points at the
correction. The pool for the same line and size says the item runs small,
which agrees with the belief; the correction overrides both:

```json validate=recall_for_purchase.response.example-correction
{
  "shopper_id": "shopper_01",
  "beliefs": [
    {
      "belief_id": "blf_8f2c41",
      "kind": "fit",
      "subject": {"kind": "brand", "id": "brand-x-001", "name": "Brand X"},
      "value": "runs small, take size L",
      "status": "Out of true",
      "standing": "Venerable",
      "contradicted": true,
      "contradicted_by": {
        "correction_id": "corr_3b7e52",
        "text": "I've lost weight, use M",
        "contradicts_belief_id": "blf_8f2c41",
        "recorded_at": "2026-11-05T14:02:11Z",
        "takes_precedence": true
      },
      "evidence": [
        {
          "evidence_id": "ev_6d1a09",
          "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
          "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
          "reason": {"category": "fit_too_small", "text": "too tight in the shoulders", "confidence": 1.0},
          "counts_toward": "fit",
          "write_op": "record_action",
          "link": {"order_id": "ORD-7XK-01", "capture_id": "CAP-7XK-01", "refund_id": "RFD-7XK-01", "custom_id": "memento-shopper01-01", "event_id": "WH-2EV-01"},
          "seeded": true,
          "occurred_at": "2026-10-14T10:00:00Z"
        },
        {
          "evidence_id": "ev_6d1a10",
          "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
          "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
          "reason": {"category": "fit_too_small", "text": "too tight across the chest", "confidence": 1.0},
          "counts_toward": "fit",
          "write_op": "record_action",
          "link": {"order_id": "ORD-7XK-02", "capture_id": "CAP-7XK-02", "refund_id": "RFD-7XK-02", "custom_id": "memento-shopper01-02", "event_id": "WH-2EV-02"},
          "seeded": true,
          "occurred_at": "2026-10-20T10:00:00Z"
        },
        {
          "evidence_id": "ev_6d1a11",
          "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
          "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
          "reason": {"category": "fit_too_small", "text": "sleeves too short in M", "confidence": 1.0},
          "counts_toward": "fit",
          "write_op": "record_action",
          "link": {"order_id": "ORD-7XK-03", "capture_id": "CAP-7XK-03", "refund_id": "RFD-7XK-03", "custom_id": "memento-shopper01-03", "event_id": "WH-2EV-03"},
          "seeded": true,
          "occurred_at": "2026-10-26T10:00:00Z"
        }
      ],
      "precedence": {"step": "own_evidence", "overridden": true, "overridden_by": "correction"},
      "seeded": true,
      "updated_at": "2026-11-05T14:02:11Z"
    }
  ],
  "pool": {
    "line": {"brand_id": "brand-x-001", "category": "shirts", "fit_handle": "regular"},
    "size": "M",
    "status": "Earned",
    "direction": "small",
    "counts": {
      "settled": 60,
      "kept": 6,
      "returns": {"too_small": 24, "too_large": 4},
      "feedback": {"fits_well": 12, "too_small": 5, "too_large": 2},
      "reasons": {"fit_too_small": 24, "fit_too_large": 4, "damaged_or_defective": 2, "not_arrived": 1, "changed_mind": 4},
      "shoppers": {"fit": 39, "too_small": 25, "too_large": 6}
    },
    "seeded": {
      "settled": 48,
      "kept": 5,
      "returns": {"too_small": 20, "too_large": 3},
      "feedback": {"fits_well": 9, "too_small": 4, "too_large": 2},
      "reasons": {"fit_too_small": 20, "fit_too_large": 3, "damaged_or_defective": 1, "not_arrived": 1, "changed_mind": 3},
      "shoppers": {"fit": 30, "too_small": 19, "too_large": 5}
    },
    "live": {
      "settled": 12,
      "kept": 1,
      "returns": {"too_small": 4, "too_large": 1},
      "feedback": {"fits_well": 3, "too_small": 1, "too_large": 0},
      "reasons": {"fit_too_small": 4, "fit_too_large": 1, "damaged_or_defective": 1, "not_arrived": 0, "changed_mind": 1},
      "shoppers": {"fit": 9, "too_small": 6, "too_large": 1}
    },
    "recent": {
      "settled": 44,
      "kept": 5,
      "returns": {"too_small": 20, "too_large": 3},
      "feedback": {"fits_well": 8, "too_small": 4, "too_large": 1},
      "reasons": {"fit_too_small": 20, "fit_too_large": 3, "damaged_or_defective": 1, "not_arrived": 0, "changed_mind": 2},
      "shoppers": {"fit": 22, "too_small": 18, "too_large": 3}
    },
    "precedence": {"step": "pool", "overridden": true, "overridden_by": "correction"}
  },
  "pool_lookup": "consulted",
  "overriding_correction": {
    "correction_id": "corr_3b7e52",
    "text": "I've lost weight, use M",
    "contradicts_belief_id": "blf_8f2c41",
    "recorded_at": "2026-11-05T14:02:11Z",
    "takes_precedence": true
  },
  "guidance": {
    "text": "Take size M for Brand X",
    "basis": "correction",
    "correction_id": "corr_3b7e52",
    "belief_id": "blf_8f2c41"
  },
  "pending_count": 1,
  "trace_id": "trace_3f8a01"
}
```

`precedence` is the pivot's addition: `step` names the precedence step an
item came from, `overridden` says a stronger step decided instead, and
`overridden_by` names it. Nothing is dropped when steps disagree; the
lower one stays in the response so the agent and the ledger can show both
sides.

### The precedence order

Recall resolves what applies in a fixed order and says which step it
used. The steps are ordered, strongest first:

| Step | `basis` | When it decides |
|---|---|---|
| 1. The correction | `correction` | An explicit, recent, first-person correction is in force. It wins at once; waiting for evidence would make the agent stubborn |
| 2. The shopper's own evidence | `own_evidence` | The shopper's own fit evidence for this brand, at any status. Fit is personal: a shopper who needs L where everyone else needs M keeps their L |
| 3. The pool | `pool` | The pooled product-fit belief, adjusted from the shopper's usual size only when the pool says the product runs off-size |
| 4. The statement | `statement` | The shopper's general statement, which only the seed supplies. It is Claimed and never promotes |
| 5. Nothing | `none` | None of the above applies, and the response says so |

The own-evidence step's `basis` is `own_evidence`. The pre-pivot schema
spelled that step `belief`, so a caller matching the literal string must
switch to the new value.

The shopper's usual size is the size the shopper most often kept or said
fits well, in any brand; failing that, it is the size read from their
general statement. It is never mapped across brands. A tie for most often
is not "most often", so it falls through to the statement.

When two steps disagree, both come back: the winner's item is served with
`overridden: false`, and the loser's item stays with `overridden: true`
and `overridden_by` naming the winner. Status is a label, never a gate: a
Building belief can beat the pool, and a pool at Building can still be
the best thing the layer knows.

`pool_lookup` says whether the pool was consulted: `consulted` when the
index answered, `unavailable` when it could not be reached. Recall
carries on without a pool and never pretends an empty one.

### Where the general statement comes from

Rung 4 needs a general statement, and no tool on this surface records
one. Statements reach a shopper's memory only through the seed: replaying
it derives each seeded belief that carries no evidence as a note, and
those notes are the statements recall serves. The statement in the
example below is one of them, and it carries the seeded mark like every
other seeded row.

The reason is the rule rung 4 rests on. A statement is a Claimed label,
never evidence: it is the shopper's word about themselves, not something
a payment outcome or a purchase-tied answer established, so no code path
may turn one into a belief. Recording it through a tool would mean a new
frozen tool name with its request and response schemas, a route that
derives the note and never writes a structural edge, and every fixture
and tool list in the build updated in the same commit. That is a
contract change, and this surface does not have it. A builder who needs
the shopper's usual size recorded seeds it with the shopper's history.

When a statement is there, recall serves it at step 4 as Claimed with
standing None and no evidence: own evidence and the pool both override
it, and it decides only when nothing stronger applies. It is also where
the shopper's usual size comes from, when no kept or fits-well purchase
names one or those purchases tie.

### A new shopper, decided by the pool

No history with the brand, no correction, no fit evidence — only a
general statement from the seed, Claimed. The request names the brand,
category and fit handle, so the pool answers for the line:

```json validate=recall_for_purchase.request.example-pool
{
  "shopper_id": "shopper_02",
  "brand_id": "brand-x-001",
  "line_name": "oxford shirt",
  "category": "shirts",
  "fit_handle": "regular",
  "sku": "PSW8GeK",
  "size_considered": "L",
  "trace_id": "trace_9c1b22"
}
```

The pool knows the line runs small across its shoppers, so the agent
sizes up from the shopper's usual size and cites the pool by its counts.
This is the move no single shopper's history can make:

```json validate=recall_for_purchase.response.example-pool
{
  "shopper_id": "shopper_02",
  "beliefs": [
    {
      "belief_id": "blf_4c2a71",
      "kind": "fit",
      "subject": {"kind": "brand", "id": "brand-x-001", "name": "Brand X"},
      "value": "shopper says they are usually size M",
      "status": "Claimed",
      "standing": "None",
      "contradicted": false,
      "contradicted_by": null,
      "evidence": [],
      "precedence": {"step": "statement", "overridden": true, "overridden_by": "pool"},
      "seeded": true,
      "updated_at": "2026-11-08T09:20:00Z"
    }
  ],
  "pool": {
    "line": {"brand_id": "brand-x-001", "category": "shirts", "fit_handle": "regular"},
    "size": "M",
    "status": "Earned",
    "direction": "small",
    "counts": {
      "settled": 60,
      "kept": 6,
      "returns": {"too_small": 24, "too_large": 4},
      "feedback": {"fits_well": 12, "too_small": 5, "too_large": 2},
      "reasons": {"fit_too_small": 24, "fit_too_large": 4, "damaged_or_defective": 2, "not_arrived": 1, "changed_mind": 4},
      "shoppers": {"fit": 39, "too_small": 25, "too_large": 6}
    },
    "seeded": {
      "settled": 48,
      "kept": 5,
      "returns": {"too_small": 20, "too_large": 3},
      "feedback": {"fits_well": 9, "too_small": 4, "too_large": 2},
      "reasons": {"fit_too_small": 20, "fit_too_large": 3, "damaged_or_defective": 1, "not_arrived": 1, "changed_mind": 3},
      "shoppers": {"fit": 30, "too_small": 19, "too_large": 5}
    },
    "live": {
      "settled": 12,
      "kept": 1,
      "returns": {"too_small": 4, "too_large": 1},
      "feedback": {"fits_well": 3, "too_small": 1, "too_large": 0},
      "reasons": {"fit_too_small": 4, "fit_too_large": 1, "damaged_or_defective": 1, "not_arrived": 0, "changed_mind": 1},
      "shoppers": {"fit": 9, "too_small": 6, "too_large": 1}
    },
    "recent": {
      "settled": 44,
      "kept": 5,
      "returns": {"too_small": 20, "too_large": 3},
      "feedback": {"fits_well": 8, "too_small": 4, "too_large": 1},
      "reasons": {"fit_too_small": 20, "fit_too_large": 3, "damaged_or_defective": 1, "not_arrived": 0, "changed_mind": 2},
      "shoppers": {"fit": 22, "too_small": 18, "too_large": 3}
    },
    "precedence": {"step": "pool", "overridden": false, "overridden_by": null}
  },
  "pool_lookup": "consulted",
  "overriding_correction": null,
  "guidance": {
    "text": "Size up to L for Brand X: 29 of the 47 fit-classified settled purchases at size M came back too small, against 6 too large.",
    "basis": "pool",
    "correction_id": null,
    "belief_id": null
  },
  "pending_count": 0,
  "trace_id": "trace_9c1b22"
}
```

### Unlike the crowd, decided by the shopper's own evidence

The pool says the line runs small. This shopper's own two returns say it
runs large on them, at only Building. Their evidence wins, because fit is
personal, and the pool is returned beside it marked overridden:

```json validate=recall_for_purchase.response.example-own
{
  "shopper_id": "shopper_04",
  "beliefs": [
    {
      "belief_id": "blf_9b31c7",
      "kind": "fit",
      "subject": {"kind": "brand", "id": "brand-x-001", "name": "Brand X"},
      "value": "runs large, take size S",
      "status": "Building",
      "standing": "Candidate",
      "contradicted": false,
      "contradicted_by": null,
      "evidence": [
        {
          "evidence_id": "ev_2f7a01",
          "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
          "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
          "reason": {"category": "fit_too_large", "text": "too loose across the back", "confidence": 1.0},
          "counts_toward": "fit",
          "write_op": "record_action",
          "link": {"order_id": "ORD-9QM-01", "capture_id": "CAP-9QM-01", "refund_id": "RFD-9QM-01", "custom_id": "memento-shopper04-01", "event_id": "WH-3EV-01"},
          "seeded": true,
          "occurred_at": "2026-10-10T10:00:00Z"
        },
        {
          "evidence_id": "ev_2f7a02",
          "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
          "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
          "reason": {"category": "fit_too_large", "text": "sleeves too long in M", "confidence": 1.0},
          "counts_toward": "fit",
          "write_op": "record_action",
          "link": {"order_id": "ORD-9QM-02", "capture_id": "CAP-9QM-02", "refund_id": "RFD-9QM-02", "custom_id": "memento-shopper04-02", "event_id": "WH-3EV-02"},
          "seeded": true,
          "occurred_at": "2026-10-18T10:00:00Z"
        }
      ],
      "precedence": {"step": "own_evidence", "overridden": false, "overridden_by": null},
      "seeded": true,
      "updated_at": "2026-10-18T10:00:00Z"
    }
  ],
  "pool": {
    "line": {"brand_id": "brand-x-001", "category": "shirts", "fit_handle": "regular"},
    "size": "M",
    "status": "Earned",
    "direction": "small",
    "counts": {
      "settled": 60,
      "kept": 6,
      "returns": {"too_small": 24, "too_large": 4},
      "feedback": {"fits_well": 12, "too_small": 5, "too_large": 2},
      "reasons": {"fit_too_small": 24, "fit_too_large": 4, "damaged_or_defective": 2, "not_arrived": 1, "changed_mind": 4},
      "shoppers": {"fit": 39, "too_small": 25, "too_large": 6}
    },
    "seeded": {
      "settled": 48,
      "kept": 5,
      "returns": {"too_small": 20, "too_large": 3},
      "feedback": {"fits_well": 9, "too_small": 4, "too_large": 2},
      "reasons": {"fit_too_small": 20, "fit_too_large": 3, "damaged_or_defective": 1, "not_arrived": 1, "changed_mind": 3},
      "shoppers": {"fit": 30, "too_small": 19, "too_large": 5}
    },
    "live": {
      "settled": 12,
      "kept": 1,
      "returns": {"too_small": 4, "too_large": 1},
      "feedback": {"fits_well": 3, "too_small": 1, "too_large": 0},
      "reasons": {"fit_too_small": 4, "fit_too_large": 1, "damaged_or_defective": 1, "not_arrived": 0, "changed_mind": 1},
      "shoppers": {"fit": 9, "too_small": 6, "too_large": 1}
    },
    "recent": {
      "settled": 44,
      "kept": 5,
      "returns": {"too_small": 20, "too_large": 3},
      "feedback": {"fits_well": 8, "too_small": 4, "too_large": 1},
      "reasons": {"fit_too_small": 20, "fit_too_large": 3, "damaged_or_defective": 1, "not_arrived": 0, "changed_mind": 2},
      "shoppers": {"fit": 22, "too_small": 18, "too_large": 3}
    },
    "precedence": {"step": "pool", "overridden": true, "overridden_by": "own_evidence"}
  },
  "pool_lookup": "consulted",
  "overriding_correction": null,
  "guidance": {
    "text": "Take size S for Brand X: your own two returns say it runs large, ahead of the pool.",
    "basis": "own_evidence",
    "correction_id": null,
    "belief_id": "blf_9b31c7"
  },
  "pending_count": 0,
  "trace_id": "trace_4a7e10"
}
```

### Nothing applies

The pool has too few shoppers to say anything, and the shopper has no
correction, no fit evidence and no statement. Recall answers, labels the
gap and points the agent at the next question; it does not fail:

```json validate=recall_for_purchase.response.example-none
{
  "shopper_id": "shopper_05",
  "beliefs": [],
  "pool": null,
  "pool_lookup": "consulted",
  "overriding_correction": null,
  "guidance": {
    "text": "No memory applies yet: no correction, no fit evidence, no pool and no general statement. Ask the shopper which size they usually take.",
    "basis": "none",
    "correction_id": null,
    "belief_id": null
  },
  "pending_count": 0,
  "trace_id": null
}
```

### The unwired shape

`testdata/api/recall_for_purchase.response.example.json` is the same
correction case, as a layer without a wired pool index returns it. The
tightened contract requires the pivot's core on every response: `pool`
and `pool_lookup` are always present, and every belief carries
`precedence`. Here the pool is null and the lookup is `unavailable`,
because no index was consulted; the belief still names its step,
`own_evidence` overridden by the correction. A null pool is an answer,
not a refusal: the correction decides, and recall carries on without the
pool rather than pretend an empty one.

```json validate=recall_for_purchase.response.example
{
  "shopper_id": "shopper_01",
  "beliefs": [
    {
      "belief_id": "blf_8f2c41",
      "kind": "fit",
      "subject": {"kind": "brand", "id": "brand-x-001", "name": "Brand X"},
      "value": "runs small, take size L",
      "status": "Out of true",
      "standing": "Venerable",
      "contradicted": true,
      "contradicted_by": {
        "correction_id": "corr_3b7e52",
        "text": "I've lost weight, use M",
        "contradicts_belief_id": "blf_8f2c41",
        "recorded_at": "2026-11-05T14:02:11Z",
        "takes_precedence": true
      },
      "evidence": [
        {
          "evidence_id": "ev_6d1a09",
          "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
          "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
          "reason": {"category": "fit_too_small", "text": "too tight in the shoulders", "confidence": 1.0},
          "counts_toward": "fit",
          "write_op": "record_action",
          "link": {"order_id": "ORD-7XK-01", "capture_id": "CAP-7XK-01", "refund_id": "RFD-7XK-01", "custom_id": "memento-shopper01-01", "event_id": "WH-2EV-01"},
          "seeded": true,
          "occurred_at": "2026-10-14T10:00:00Z"
        },
        {
          "evidence_id": "ev_6d1a10",
          "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
          "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
          "reason": {"category": "fit_too_small", "text": "too tight across the chest", "confidence": 1.0},
          "counts_toward": "fit",
          "write_op": "record_action",
          "link": {"order_id": "ORD-7XK-02", "capture_id": "CAP-7XK-02", "refund_id": "RFD-7XK-02", "custom_id": "memento-shopper01-02", "event_id": "WH-2EV-02"},
          "seeded": true,
          "occurred_at": "2026-10-20T10:00:00Z"
        },
        {
          "evidence_id": "ev_6d1a11",
          "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
          "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
          "reason": {"category": "fit_too_small", "text": "sleeves too short in M", "confidence": 1.0},
          "counts_toward": "fit",
          "write_op": "record_action",
          "link": {"order_id": "ORD-7XK-03", "capture_id": "CAP-7XK-03", "refund_id": "RFD-7XK-03", "custom_id": "memento-shopper01-03", "event_id": "WH-2EV-03"},
          "seeded": true,
          "occurred_at": "2026-10-26T10:00:00Z"
        }
      ],
      "precedence": {"step": "own_evidence", "overridden": true, "overridden_by": "correction"},
      "seeded": true,
      "updated_at": "2026-11-05T14:02:11Z"
    }
  ],
  "pool": null,
  "pool_lookup": "unavailable",
  "overriding_correction": {
    "correction_id": "corr_3b7e52",
    "text": "I've lost weight, use M",
    "contradicts_belief_id": "blf_8f2c41",
    "recorded_at": "2026-11-05T14:02:11Z",
    "takes_precedence": true
  },
  "guidance": {
    "text": "Take size M for Brand X",
    "basis": "correction",
    "correction_id": "corr_3b7e52",
    "belief_id": "blf_8f2c41"
  },
  "pending_count": 1,
  "trace_id": "trace_3f8a01"
}
```

## The contradiction rule

A belief and its warning travel together, in every tool that returns a
belief:

- `status` is the ledger status for display. It reads Out of true while
  a correction applies.
- `standing` is the underlying graph state with the overlay removed:
  None, Candidate or Venerable. It never reads contradicted; a
  contradicted belief keeps the standing its evidence earned.
- `contradicted` plus `contradicted_by` is the warning itself: the
  applying correction, inline, so no agent reads the belief without
  seeing the override.

A correction wins the recommendation at once, and the belief stays. The
belief changes only when new evidence earns the change: one "fits well"
does not promote the rival belief, and the Venerable belief is never
demoted by it.

`precedence` is always present and null on ledger and explain reads:
they show the belief, not the step that served it. Recall sets it on
every belief, and the pool's own block sets it when recall returns one;
`product_fit` carries null for the same reason the ledger does.

## record_correction

Call it the moment the shopper corrects you, in the shopper's verbatim
first-person words. `contradicts_belief_id` is optional. With a belief,
the correction is declared against it and applies at once with
precedence; the contradicted belief keeps its status and standing and
renders its warning beside it.

Request:

```json validate=record_correction.request.example
{
  "shopper_id": "shopper_01",
  "text": "I've lost weight, use M",
  "contradicts_belief_id": "blf_8f2c41",
  "trace_id": "trace_3f8a02"
}
```

Response. The very next recommendation is M, and the L belief stands
beside the warning, not deleted:

```json validate=record_correction.response.example
{
  "correction": {
    "correction_id": "corr_3b7e52",
    "text": "I've lost weight, use M",
    "contradicts_belief_id": "blf_8f2c41",
    "recorded_at": "2026-11-05T14:02:11Z",
    "takes_precedence": true
  },
  "belief_id": "blf_8f2c41",
  "status": "Out of true",
  "standing": "Venerable",
  "contradicted": true,
  "guidance": {
    "text": "Take size M for Brand X",
    "basis": "correction",
    "correction_id": "corr_3b7e52",
    "belief_id": "blf_8f2c41"
  },
  "trace_id": "trace_3f8a02"
}
```

### A correction with no belief yet

When the shopper corrects something the layer has no belief about, send
the correction without `contradicts_belief_id`. It is kept as a standing
record: no against declaration, `contradicted: false`, labelled Claimed,
and it still decides the recommendation through the correction step.

Request:

```json validate=record_correction.request.example-standing
{
  "shopper_id": "shopper_02",
  "text": "I've lost weight, use M",
  "trace_id": "trace_9c1b23"
}
```

Response:

```json validate=record_correction.response.example-standing
{
  "correction": {
    "correction_id": "corr_5d1a09",
    "text": "I've lost weight, use M",
    "recorded_at": "2026-11-09T10:12:00Z",
    "takes_precedence": true
  },
  "status": "Claimed",
  "standing": "None",
  "contradicted": false,
  "guidance": {
    "text": "Take size M",
    "basis": "correction",
    "correction_id": "corr_5d1a09",
    "belief_id": null
  },
  "trace_id": "trace_9c1b23"
}
```

A standing correction is never promoted and never discarded. When a
matching belief is written later, for the same subject, the layer
declares the standing correction against it once: the correction then
carries `contradicts_belief_id` and the belief carries `contradicted_by`,
exactly like a correction recorded against an existing belief. A second
matching belief does not re-declare it. The belief is never deleted.

If a matching belief already exists when you send the correction without
`contradicts_belief_id`, the layer declares the correction against that
belief at once: read the response's `belief_id` and `contradicted` rather
than assuming the standing shape. When two beliefs share the subject, the
strongest one (most evidence, then earliest, then smallest id) is the one
declared against, chosen deterministically rather than by map order.

## product_fit

The merchant's read, and the only one it is given. For one product line
and one size it returns the pooled product-fit belief: the status, the
direction it rates, and the settled counts behind it. It carries no
shopper identity, no purchase row and no free text, so no shopper's data
can reach the merchant app.

Request. The brand, category and fit handle name the line together; the
size names the variant:

```json validate=product_fit.request.example
{
  "brand_id": "brand-x-001",
  "category": "shirts",
  "fit_handle": "regular",
  "size": "M"
}
```

Response:

```json validate=product_fit.response.example
{
  "pool": {
    "line": {"brand_id": "brand-x-001", "category": "shirts", "fit_handle": "regular"},
    "size": "M",
    "status": "Earned",
    "direction": "small",
    "counts": {
      "settled": 60,
      "kept": 6,
      "returns": {"too_small": 24, "too_large": 4},
      "feedback": {"fits_well": 12, "too_small": 5, "too_large": 2},
      "reasons": {"fit_too_small": 24, "fit_too_large": 4, "damaged_or_defective": 2, "not_arrived": 1, "changed_mind": 4},
      "shoppers": {"fit": 39, "too_small": 25, "too_large": 6}
    },
    "seeded": {
      "settled": 48,
      "kept": 5,
      "returns": {"too_small": 20, "too_large": 3},
      "feedback": {"fits_well": 9, "too_small": 4, "too_large": 2},
      "reasons": {"fit_too_small": 20, "fit_too_large": 3, "damaged_or_defective": 1, "not_arrived": 1, "changed_mind": 3},
      "shoppers": {"fit": 30, "too_small": 19, "too_large": 5}
    },
    "live": {
      "settled": 12,
      "kept": 1,
      "returns": {"too_small": 4, "too_large": 1},
      "feedback": {"fits_well": 3, "too_small": 1, "too_large": 0},
      "reasons": {"fit_too_small": 4, "fit_too_large": 1, "damaged_or_defective": 1, "not_arrived": 0, "changed_mind": 1},
      "shoppers": {"fit": 9, "too_small": 6, "too_large": 1}
    },
    "recent": {
      "settled": 44,
      "kept": 5,
      "returns": {"too_small": 20, "too_large": 3},
      "feedback": {"fits_well": 8, "too_small": 4, "too_large": 1},
      "reasons": {"fit_too_small": 20, "fit_too_large": 3, "damaged_or_defective": 1, "not_arrived": 0, "changed_mind": 2},
      "shoppers": {"fit": 22, "too_small": 18, "too_large": 3}
    },
    "precedence": null
  }
}
```

The counts above are illustrative: the status and direction match the
seeded population's oxford line at size M, but the counts do not. They
show the shape and the arithmetic — seeded plus live equals the total on
every field, and recent never exceeds the total — not the population's
own rows. The Claimed and Building examples below do carry the seeded
population's rows for their lines, with no live rows yet.

The counts are settled purchases only. A purchase still inside its
return window, and a refund with no reason yet, count toward nothing.
`settled` is the denominator, so a new product does not look perfect
before its returns arrive. `returns` counts settled returns by direction,
`feedback` counts settled fit feedback by verdict, `reasons` counts
settled returns by reason category — the merchant's "why it comes back" —
and `kept` counts purchases kept past the window with no feedback.
`shoppers` counts distinct shoppers, never identities; `shoppers.fit` is
the pool's Building floor and is not the sum of the two directions, since
a shopper who returned one size each way counts once. `seeded` and `live`
split every count, and `recent` is the last 90 days of event time alone.

`direction` is `small`, `large`, or null when no direction leads. Status
below three distinct shoppers is Claimed: the pool has nothing to say,
and recall's pool step does not apply.

### The pool's status rule

The rule is fixed, and it is the same one the code applies. Every
threshold has one definition in `internal/domain`, named here:

| Status | Rule |
|---|---|
| **Claimed** | Below 3 distinct shoppers with a fit-classified outcome. The pool says nothing |
| **Building** | At least 3 distinct shoppers, and not yet Earned |
| **Earned** | At least 8 distinct shoppers in one direction; that direction is at least 60% of fit-classified settled outcomes; and it is at least twice the opposite direction |
| **Out of true** | Earned overall, but the last 90 days of event time, taken alone, earn the opposite direction or fail the rule for this one |

| Constant | Value | Where it lives |
|---|---|---|
| Building floor | 3 distinct shoppers | `domain.PoolBuildingShoppers` |
| Earned shoppers | 8 distinct shoppers in one direction | `domain.PoolEarnedShoppers` |
| Earned share | 60% of fit-classified settled outcomes | `domain.PoolEarnedSharePercent` |
| Earned ratio | 2× the opposite direction | `domain.PoolEarnedRatio` |
| Recent window | 90 days of event time | `domain.PoolRecentWindow` |

The directional counts the rule reads are returns and feedback together:
a direction is how the size ran, whichever way the shopper told us. A
fits-well verdict points nowhere, but it is fit evidence, so it dilutes a
direction's share. The recent window is judged on its own only once it
reaches the Building floor; below that the pool has nothing to say about
the window, and a line that has gone quiet keeps its label.

Every pooled row is marked when it is seeded, and the split is shown, so
a seeded population is never passed off as lived sales.

### Below the floor: Claimed

Two settled purchases on the relaxed line at size M, one each way. Two
distinct shoppers are below the Building floor of three, so the pool
says nothing: status Claimed, direction null. The counts are the seeded
population's own rows for this line, all seeded, with no live rows yet;
both events fall inside the recent window, so recent equals the total:

```json validate=product_fit.response.example-claimed
{
  "pool": {
    "line": {"brand_id": "brand-v-004", "category": "shirts", "fit_handle": "relaxed"},
    "size": "M",
    "status": "Claimed",
    "direction": null,
    "counts": {
      "settled": 2,
      "kept": 0,
      "returns": {"too_small": 1, "too_large": 1},
      "feedback": {"fits_well": 0, "too_small": 0, "too_large": 0},
      "reasons": {"fit_too_small": 1, "fit_too_large": 1, "damaged_or_defective": 0, "not_arrived": 0, "changed_mind": 0},
      "shoppers": {"fit": 2, "too_small": 1, "too_large": 1}
    },
    "seeded": {
      "settled": 2,
      "kept": 0,
      "returns": {"too_small": 1, "too_large": 1},
      "feedback": {"fits_well": 0, "too_small": 0, "too_large": 0},
      "reasons": {"fit_too_small": 1, "fit_too_large": 1, "damaged_or_defective": 0, "not_arrived": 0, "changed_mind": 0},
      "shoppers": {"fit": 2, "too_small": 1, "too_large": 1}
    },
    "live": {
      "settled": 0,
      "kept": 0,
      "returns": {"too_small": 0, "too_large": 0},
      "feedback": {"fits_well": 0, "too_small": 0, "too_large": 0},
      "reasons": {"fit_too_small": 0, "fit_too_large": 0, "damaged_or_defective": 0, "not_arrived": 0, "changed_mind": 0},
      "shoppers": {"fit": 0, "too_small": 0, "too_large": 0}
    },
    "recent": {
      "settled": 2,
      "kept": 0,
      "returns": {"too_small": 1, "too_large": 1},
      "feedback": {"fits_well": 0, "too_small": 0, "too_large": 0},
      "reasons": {"fit_too_small": 1, "fit_too_large": 1, "damaged_or_defective": 0, "not_arrived": 0, "changed_mind": 0},
      "shoppers": {"fit": 2, "too_small": 1, "too_large": 1}
    },
    "precedence": null
  }
}
```

### Almost there: Building

Eight settled purchases on the slim line at size M: six too-small
returns, one too-large return, one fits-well. Eight distinct shoppers
reach the floor, and the small direction leads, but its six shoppers are
short of the eight the Earned rule needs, so the pool is Building in the
small direction. Same provenance as above: the seeded population's own
rows, no live rows yet, and a recent window holding only the five
too-small returns of the last 90 days:

```json validate=product_fit.response.example-building
{
  "pool": {
    "line": {"brand_id": "brand-y-002", "category": "shirts", "fit_handle": "slim"},
    "size": "M",
    "status": "Building",
    "direction": "small",
    "counts": {
      "settled": 8,
      "kept": 0,
      "returns": {"too_small": 6, "too_large": 1},
      "feedback": {"fits_well": 1, "too_small": 0, "too_large": 0},
      "reasons": {"fit_too_small": 6, "fit_too_large": 1, "damaged_or_defective": 0, "not_arrived": 0, "changed_mind": 0},
      "shoppers": {"fit": 8, "too_small": 6, "too_large": 1}
    },
    "seeded": {
      "settled": 8,
      "kept": 0,
      "returns": {"too_small": 6, "too_large": 1},
      "feedback": {"fits_well": 1, "too_small": 0, "too_large": 0},
      "reasons": {"fit_too_small": 6, "fit_too_large": 1, "damaged_or_defective": 0, "not_arrived": 0, "changed_mind": 0},
      "shoppers": {"fit": 8, "too_small": 6, "too_large": 1}
    },
    "live": {
      "settled": 0,
      "kept": 0,
      "returns": {"too_small": 0, "too_large": 0},
      "feedback": {"fits_well": 0, "too_small": 0, "too_large": 0},
      "reasons": {"fit_too_small": 0, "fit_too_large": 0, "damaged_or_defective": 0, "not_arrived": 0, "changed_mind": 0},
      "shoppers": {"fit": 0, "too_small": 0, "too_large": 0}
    },
    "recent": {
      "settled": 5,
      "kept": 0,
      "returns": {"too_small": 5, "too_large": 0},
      "feedback": {"fits_well": 0, "too_small": 0, "too_large": 0},
      "reasons": {"fit_too_small": 5, "fit_too_large": 0, "damaged_or_defective": 0, "not_arrived": 0, "changed_mind": 0},
      "shoppers": {"fit": 5, "too_small": 5, "too_large": 0}
    },
    "precedence": null
  }
}
```

## set_pooling

Call it when the shopper asks to be left out of the pool, or to opt back
in. Pooling is opt-out: a shopper's classified outcomes count toward the
pool unless they say no. Opting out stops new fit rows for that shopper
and removes the rows already written; the response says how many. Opting
back in removes none. The choice is recorded in the shopper's own memory,
so it reaches every agent that plugs into the same layer.

Request. Opting out:

```json validate=set_pooling.request.example
{
  "shopper_id": "shopper_01",
  "pooling": false,
  "trace_id": "trace_3f8a05"
}
```

Response:

```json validate=set_pooling.response.example
{
  "shopper_id": "shopper_01",
  "pooling": false,
  "rows_removed": 3,
  "trace_id": "trace_3f8a05"
}
```

Opting back in:

```json validate=set_pooling.response.example-optin
{
  "shopper_id": "shopper_01",
  "pooling": true,
  "rows_removed": 0,
  "trace_id": null
}
```

A correction is about the shopper, not the product, and never enters the
pool.

## pending_questions

Call it when `pending_count` is above zero, or whenever the chat has a
natural opening. A refund record carries no reason field, so a refund
the shopper never explains stays an outcome with no reason and counts
toward nothing. The response lists the open questions, oldest first.

Request:

```json validate=pending_questions.request.example
{
  "shopper_id": "shopper_01",
  "limit": 10
}
```

Response. One refund still missing its reason, already asked once:

```json validate=pending_questions.response.example
{
  "shopper_id": "shopper_01",
  "questions": [
    {
      "question_id": "q_9f04ab",
      "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
      "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
      "link": {"order_id": "ORD-7XK-05", "capture_id": "CAP-7XK-05", "refund_id": "RFD-7XK-05", "custom_id": "memento-shopper01-05", "event_id": "WH-2EV-05"},
      "seeded": false,
      "asked_count": 1,
      "last_asked_at": "2026-11-08T18:40:00Z",
      "awaiting_since": "2026-11-07T09:15:00Z"
    }
  ]
}
```

## record_reason

Send the shopper's verbatim answer to one pending question. The layer
classifies it on its side. A clear fit answer becomes evidence; a vague
answer, or a classification under the confidence floor, stays pending
and the response says to ask again.

Request:

```json validate=record_reason.request.example
{
  "shopper_id": "shopper_01",
  "question_id": "q_9f04ab",
  "answer_text": "Too tight in the shoulders, couldn't button it.",
  "trace_id": "trace_3f8a03"
}
```

Response when the answer is clear. It counted toward fit and now
supports the L belief:

```json validate=record_reason.response.example
{
  "reason_id": "rsn_77aa10",
  "question_id": "q_9f04ab",
  "category": "fit_too_small",
  "confidence": 1.0,
  "counts_toward": "fit",
  "accepted": true,
  "ask_again": false,
  "belief_id": "blf_8f2c41",
  "trace_id": "trace_3f8a03"
}
```

Response when the answer is vague. It counted toward nothing,
`ask_again` is true, and the question stays open:

```json validate=record_reason.response.example-unclear
{
  "reason_id": "rsn_77aa11",
  "question_id": "q_9f04ab",
  "category": "unclear",
  "confidence": 0.94,
  "counts_toward": null,
  "accepted": false,
  "ask_again": true,
  "belief_id": null,
  "trace_id": "trace_3f8a03"
}
```

## record_feedback

After delivery, ask how the purchased size worked out and send the
verbatim answer tied to the purchase. The join key is the `custom_id`
that propagates across the order, capture, refund and dispute records.
A fits-well verdict counts toward fit as supporting evidence for the
worn size. A verdict of unclear counts toward nothing.

Request:

```json validate=record_feedback.request.example
{
  "shopper_id": "shopper_01",
  "custom_id": "memento-shopper01-06",
  "sku": "PSW8GeK",
  "size": "M",
  "feedback_text": "The M fits well across the shoulders.",
  "trace_id": "trace_3f8a04"
}
```

Response. The M fits, and the answer now supports the emerging M
belief:

```json validate=record_feedback.response.example
{
  "feedback_id": "fb_51c2d8",
  "verdict": "fits_well",
  "counts_toward": "fit",
  "belief_id": "blf_2b9d60",
  "accepted": true,
  "trace_id": "trace_3f8a04"
}
```

## explain_belief

Call it when challenged — by the shopper ("why do you think that?") or
by your own judge. It answers with every piece of evidence and its
reason, every correction, and the history in order. Cite this, not the
raw transaction history, when justifying a recommendation.

Request:

```json validate=explain_belief.request.example
{
  "shopper_id": "shopper_01",
  "belief_id": "blf_8f2c41"
}
```

Response:

```json validate=explain_belief.response.example
{
  "belief": {
    "belief_id": "blf_8f2c41",
    "kind": "fit",
    "subject": {"kind": "brand", "id": "brand-x-001", "name": "Brand X"},
    "value": "runs small, take size L",
    "status": "Out of true",
    "standing": "Venerable",
    "contradicted": true,
    "contradicted_by": {
      "correction_id": "corr_3b7e52",
      "text": "I've lost weight, use M",
      "contradicts_belief_id": "blf_8f2c41",
      "recorded_at": "2026-11-05T14:02:11Z",
      "takes_precedence": true
    },
    "evidence": [
      {
        "evidence_id": "ev_6d1a09",
        "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
        "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
        "reason": {"category": "fit_too_small", "text": "too tight in the shoulders", "confidence": 1.0},
        "counts_toward": "fit",
        "write_op": "record_action",
        "link": {"order_id": "ORD-7XK-01", "capture_id": "CAP-7XK-01", "refund_id": "RFD-7XK-01", "custom_id": "memento-shopper01-01", "event_id": "WH-2EV-01"},
        "seeded": true,
        "occurred_at": "2026-10-14T10:00:00Z"
      },
      {
        "evidence_id": "ev_6d1a10",
        "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
        "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
        "reason": {"category": "fit_too_small", "text": "too tight across the chest", "confidence": 1.0},
        "counts_toward": "fit",
        "write_op": "record_action",
        "link": {"order_id": "ORD-7XK-02", "capture_id": "CAP-7XK-02", "refund_id": "RFD-7XK-02", "custom_id": "memento-shopper01-02", "event_id": "WH-2EV-02"},
        "seeded": true,
        "occurred_at": "2026-10-20T10:00:00Z"
      },
      {
        "evidence_id": "ev_6d1a11",
        "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
        "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
        "reason": {"category": "fit_too_small", "text": "sleeves too short in M", "confidence": 1.0},
        "counts_toward": "fit",
        "write_op": "record_action",
        "link": {"order_id": "ORD-7XK-03", "capture_id": "CAP-7XK-03", "refund_id": "RFD-7XK-03", "custom_id": "memento-shopper01-03", "event_id": "WH-2EV-03"},
        "seeded": true,
        "occurred_at": "2026-10-26T10:00:00Z"
      }
    ],
    "precedence": null,
    "seeded": true,
    "updated_at": "2026-11-05T14:02:11Z"
  },
  "corrections": [
    {
      "correction_id": "corr_3b7e52",
      "text": "I've lost weight, use M",
      "contradicts_belief_id": "blf_8f2c41",
      "recorded_at": "2026-11-05T14:02:11Z",
      "takes_precedence": true
    }
  ],
  "timeline": [
    {"at": "2026-10-14T10:00:00Z", "kind": "evidence", "summary": "Fit return 1: oxford shirt size M refunded; too tight in the shoulders.", "ref_id": "ev_6d1a09"},
    {"at": "2026-10-20T10:00:00Z", "kind": "evidence", "summary": "Fit return 2: oxford shirt size M refunded; too tight across the chest.", "ref_id": "ev_6d1a10"},
    {"at": "2026-10-26T10:00:00Z", "kind": "evidence", "summary": "Fit return 3: oxford shirt size M refunded; sleeves too short in M.", "ref_id": "ev_6d1a11"},
    {"at": "2026-10-26T10:00:05Z", "kind": "status", "summary": "Belief reached Earned: three documented fit returns over twelve days.", "ref_id": "blf_8f2c41"},
    {"at": "2026-11-05T14:02:11Z", "kind": "correction", "summary": "Shopper correction: lost weight, take size M for Brand X.", "ref_id": "corr_3b7e52"},
    {"at": "2026-11-08T20:10:00Z", "kind": "feedback", "summary": "Fit feedback: oxford shirt size M fits well.", "ref_id": "fb_51c2d8"}
  ]
}
```

## The ledger reads

The evidence ledger shows what changed and why: beliefs, the evidence
behind each, contradictions and corrections, with seeded rows marked.
Three reads serve it over the same MCP surface:

- `ledger_beliefs` lists the rows, strongest first, each with ledger
  status, underlying standing, contradiction warning, seeded mark and
  evidence count. The status filter is omitted for all rows; the
  seeded flag exists for the comparison harness and stays true in the
  demo.
- `ledger_belief` reads one row in full, with its corrections and a
  seeded notice in words when the belief is seeded.
- `ledger_evidence` lists the evidence rows, newest first, with what
  each counts toward, its seeded mark, and the beliefs it supports.

Ledger rows request:

```json validate=ledger_beliefs.request.example
{
  "shopper_id": "shopper_01",
  "include_seeded": true
}
```

Ledger rows response. Four rows: the contradicted L belief, the
emerging M belief with its one supporting purchase, the seller belief
with its one non-arrival dispute, and the shopper's general statement,
which is Claimed and never promotes:

```json validate=ledger_beliefs.response.example
{
  "shopper_id": "shopper_01",
  "beliefs": [
    {
      "belief": {
        "belief_id": "blf_8f2c41",
        "kind": "fit",
        "subject": {"kind": "brand", "id": "brand-x-001", "name": "Brand X"},
        "value": "runs small, take size L",
        "status": "Out of true",
        "standing": "Venerable",
        "contradicted": true,
        "contradicted_by": {
          "correction_id": "corr_3b7e52",
          "text": "I've lost weight, use M",
          "contradicts_belief_id": "blf_8f2c41",
          "recorded_at": "2026-11-05T14:02:11Z",
          "takes_precedence": true
        },
        "evidence": [
          {
            "evidence_id": "ev_6d1a09",
            "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
            "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
            "reason": {"category": "fit_too_small", "text": "too tight in the shoulders", "confidence": 1.0},
            "counts_toward": "fit",
            "write_op": "record_action",
            "link": {"order_id": "ORD-7XK-01", "capture_id": "CAP-7XK-01", "refund_id": "RFD-7XK-01", "custom_id": "memento-shopper01-01", "event_id": "WH-2EV-01"},
            "seeded": true,
            "occurred_at": "2026-10-14T10:00:00Z"
          },
          {
            "evidence_id": "ev_6d1a10",
            "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
            "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
            "reason": {"category": "fit_too_small", "text": "too tight across the chest", "confidence": 1.0},
            "counts_toward": "fit",
            "write_op": "record_action",
            "link": {"order_id": "ORD-7XK-02", "capture_id": "CAP-7XK-02", "refund_id": "RFD-7XK-02", "custom_id": "memento-shopper01-02", "event_id": "WH-2EV-02"},
            "seeded": true,
            "occurred_at": "2026-10-20T10:00:00Z"
          },
          {
            "evidence_id": "ev_6d1a11",
            "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
            "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
            "reason": {"category": "fit_too_small", "text": "sleeves too short in M", "confidence": 1.0},
            "counts_toward": "fit",
            "write_op": "record_action",
            "link": {"order_id": "ORD-7XK-03", "capture_id": "CAP-7XK-03", "refund_id": "RFD-7XK-03", "custom_id": "memento-shopper01-03", "event_id": "WH-2EV-03"},
            "seeded": true,
            "occurred_at": "2026-10-26T10:00:00Z"
          }
        ],
        "precedence": null,
        "seeded": true,
        "updated_at": "2026-11-05T14:02:11Z"
      },
      "evidence_count": 3,
      "last_change_at": "2026-11-05T14:02:11Z"
    },
    {
      "belief": {
        "belief_id": "blf_2b9d60",
        "kind": "fit",
        "subject": {"kind": "brand", "id": "brand-x-001", "name": "Brand X"},
        "value": "size M fits this shopper now",
        "status": "Claimed",
        "standing": "None",
        "contradicted": false,
        "contradicted_by": null,
        "evidence": [
          {
            "evidence_id": "ev_51c2d8",
            "outcome": {"kind": "capture", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
            "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "PSW8GeK", "size": "M", "category": "shirts"},
            "reason": {"category": "none", "text": "the M fits well across the shoulders", "confidence": 1.0},
            "counts_toward": "fit",
            "write_op": "record_action",
            "link": {"order_id": "ORD-7XK-06", "capture_id": "CAP-7XK-06", "custom_id": "memento-shopper01-06", "event_id": "WH-2EV-06"},
            "seeded": false,
            "occurred_at": "2026-11-08T20:10:00Z"
          }
        ],
        "precedence": null,
        "seeded": false,
        "updated_at": "2026-11-08T20:10:00Z"
      },
      "evidence_count": 1,
      "last_change_at": "2026-11-08T20:10:00Z"
    },
    {
      "belief": {
        "belief_id": "blf_1a5e09",
        "kind": "seller_reliability",
        "subject": {"kind": "seller", "id": "seller-acme-01", "name": "Acme"},
        "value": "ships reliably for this shopper",
        "status": "Claimed",
        "standing": "None",
        "contradicted": false,
        "contradicted_by": null,
        "evidence": [
          {
            "evidence_id": "ev_9d4c22",
            "outcome": {"kind": "dispute", "status": "OPEN", "amount_minor": 8900, "currency": "USD"},
            "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "chino", "sku": "chino-khaki-32", "size": "32", "category": "pants"},
            "reason": {"category": "not_arrived", "text": "MERCHANDISE_OR_SERVICE_NOT_RECEIVED", "confidence": 1.0},
            "counts_toward": "seller_reliability",
            "write_op": "record_action",
            "link": {"order_id": "ORD-7XK-04", "capture_id": "CAP-7XK-04", "dispute_id": "PP-D-51713", "custom_id": "memento-shopper01-04", "event_id": "WH-2EV-04"},
            "seeded": false,
            "occurred_at": "2026-11-06T11:30:00Z"
          }
        ],
        "precedence": null,
        "seeded": false,
        "updated_at": "2026-11-06T11:30:00Z"
      },
      "evidence_count": 1,
      "last_change_at": "2026-11-06T11:30:00Z"
    },
    {
      "belief": {
        "belief_id": "blf_77c310",
        "kind": "fit",
        "subject": {"kind": "brand", "id": "brand-x-001", "name": "Brand X"},
        "value": "shopper says they are usually size M",
        "status": "Claimed",
        "standing": "None",
        "contradicted": false,
        "contradicted_by": null,
        "evidence": [],
        "precedence": null,
        "seeded": true,
        "updated_at": "2026-10-14T10:00:00Z"
      },
      "evidence_count": 0,
      "last_change_at": "2026-10-14T10:00:00Z"
    }
  ]
}
```

One row in full:

```json validate=ledger_belief.request.example
{
  "shopper_id": "shopper_01",
  "belief_id": "blf_8f2c41"
}
```

```json validate=ledger_belief.response.example
{
  "belief": {
    "belief_id": "blf_8f2c41",
    "kind": "fit",
    "subject": {"kind": "brand", "id": "brand-x-001", "name": "Brand X"},
    "value": "runs small, take size L",
    "status": "Out of true",
    "standing": "Venerable",
    "contradicted": true,
    "contradicted_by": {
      "correction_id": "corr_3b7e52",
      "text": "I've lost weight, use M",
      "contradicts_belief_id": "blf_8f2c41",
      "recorded_at": "2026-11-05T14:02:11Z",
      "takes_precedence": true
    },
    "evidence": [
      {
        "evidence_id": "ev_6d1a09",
        "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
        "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
        "reason": {"category": "fit_too_small", "text": "too tight in the shoulders", "confidence": 1.0},
        "counts_toward": "fit",
        "write_op": "record_action",
        "link": {"order_id": "ORD-7XK-01", "capture_id": "CAP-7XK-01", "refund_id": "RFD-7XK-01", "custom_id": "memento-shopper01-01", "event_id": "WH-2EV-01"},
        "seeded": true,
        "occurred_at": "2026-10-14T10:00:00Z"
      },
      {
        "evidence_id": "ev_6d1a10",
        "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
        "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
        "reason": {"category": "fit_too_small", "text": "too tight across the chest", "confidence": 1.0},
        "counts_toward": "fit",
        "write_op": "record_action",
        "link": {"order_id": "ORD-7XK-02", "capture_id": "CAP-7XK-02", "refund_id": "RFD-7XK-02", "custom_id": "memento-shopper01-02", "event_id": "WH-2EV-02"},
        "seeded": true,
        "occurred_at": "2026-10-20T10:00:00Z"
      },
      {
        "evidence_id": "ev_6d1a11",
        "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
        "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
        "reason": {"category": "fit_too_small", "text": "sleeves too short in M", "confidence": 1.0},
        "counts_toward": "fit",
        "write_op": "record_action",
        "link": {"order_id": "ORD-7XK-03", "capture_id": "CAP-7XK-03", "refund_id": "RFD-7XK-03", "custom_id": "memento-shopper01-03", "event_id": "WH-2EV-03"},
        "seeded": true,
        "occurred_at": "2026-10-26T10:00:00Z"
      }
    ],
    "precedence": null,
    "seeded": true,
    "updated_at": "2026-11-05T14:02:11Z"
  },
  "corrections": [
    {
      "correction_id": "corr_3b7e52",
      "text": "I've lost weight, use M",
      "contradicts_belief_id": "blf_8f2c41",
      "recorded_at": "2026-11-05T14:02:11Z",
      "takes_precedence": true
    }
  ],
  "seeded_notice": "Seeded history: generated, not lived. The three returns are seeded; the correction is live."
}
```

Evidence rows request:

```json validate=ledger_evidence.request.example
{
  "shopper_id": "shopper_01"
}
```

Evidence rows response. Newest first. The feedback row carries no
failure reason — its reason category is none with the shopper's words —
and `counts_toward` carries the verdict's kind, so one shape serves
returns, disputes and feedback:

```json validate=ledger_evidence.response.example
{
  "shopper_id": "shopper_01",
  "evidence": [
    {
      "evidence": {
        "evidence_id": "ev_51c2d8",
        "outcome": {"kind": "capture", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
        "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "PSW8GeK", "size": "M", "category": "shirts"},
        "reason": {"category": "none", "text": "the M fits well across the shoulders", "confidence": 1.0},
        "counts_toward": "fit",
        "write_op": "record_action",
        "link": {"order_id": "ORD-7XK-06", "capture_id": "CAP-7XK-06", "custom_id": "memento-shopper01-06", "event_id": "WH-2EV-06"},
        "seeded": false,
        "occurred_at": "2026-11-08T20:10:00Z"
      },
      "belief_ids": ["blf_2b9d60"]
    },
    {
      "evidence": {
        "evidence_id": "ev_9d4c22",
        "outcome": {"kind": "dispute", "status": "OPEN", "amount_minor": 8900, "currency": "USD"},
        "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "chino", "sku": "chino-khaki-32", "size": "32", "category": "pants"},
        "reason": {"category": "not_arrived", "text": "MERCHANDISE_OR_SERVICE_NOT_RECEIVED", "confidence": 1.0},
        "counts_toward": "seller_reliability",
        "write_op": "record_action",
        "link": {"order_id": "ORD-7XK-04", "capture_id": "CAP-7XK-04", "dispute_id": "PP-D-51713", "custom_id": "memento-shopper01-04", "event_id": "WH-2EV-04"},
        "seeded": false,
        "occurred_at": "2026-11-06T11:30:00Z"
      },
      "belief_ids": ["blf_1a5e09"]
    },
    {
      "evidence": {
        "evidence_id": "ev_6d1a11",
        "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
        "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
        "reason": {"category": "fit_too_small", "text": "sleeves too short in M", "confidence": 1.0},
        "counts_toward": "fit",
        "write_op": "record_action",
        "link": {"order_id": "ORD-7XK-03", "capture_id": "CAP-7XK-03", "refund_id": "RFD-7XK-03", "custom_id": "memento-shopper01-03", "event_id": "WH-2EV-03"},
        "seeded": true,
        "occurred_at": "2026-10-26T10:00:00Z"
      },
      "belief_ids": ["blf_8f2c41"]
    },
    {
      "evidence": {
        "evidence_id": "ev_6d1a10",
        "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
        "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
        "reason": {"category": "fit_too_small", "text": "too tight across the chest", "confidence": 1.0},
        "counts_toward": "fit",
        "write_op": "record_action",
        "link": {"order_id": "ORD-7XK-02", "capture_id": "CAP-7XK-02", "refund_id": "RFD-7XK-02", "custom_id": "memento-shopper01-02", "event_id": "WH-2EV-02"},
        "seeded": true,
        "occurred_at": "2026-10-20T10:00:00Z"
      },
      "belief_ids": ["blf_8f2c41"]
    },
    {
      "evidence": {
        "evidence_id": "ev_6d1a09",
        "outcome": {"kind": "refund", "status": "COMPLETED", "amount_minor": 19500, "currency": "USD"},
        "item": {"brand_id": "brand-x-001", "brand_name": "Brand X", "line_name": "oxford shirt", "sku": "tbnrl46", "size": "M", "category": "shirts"},
        "reason": {"category": "fit_too_small", "text": "too tight in the shoulders", "confidence": 1.0},
        "counts_toward": "fit",
        "write_op": "record_action",
        "link": {"order_id": "ORD-7XK-01", "capture_id": "CAP-7XK-01", "refund_id": "RFD-7XK-01", "custom_id": "memento-shopper01-01", "event_id": "WH-2EV-01"},
        "seeded": true,
        "occurred_at": "2026-10-14T10:00:00Z"
      },
      "belief_ids": ["blf_8f2c41"]
    }
  ]
}
```

## Reference tables

Ledger status, and what earns it:

| Ledger shows | Standing | Meaning |
|---|---|---|
| Claimed | None | The shopper said it in general, one data point exists, or a pool is below its floor. Never promotes on its own. |
| Building | Candidate | Evidence is accumulating. |
| Earned | Venerable | Enough documented evidence, over enough time, to act on. |
| Out of true | Venerable | A correction applies, or the pool's recent window disagrees. Same standing, with the warning. |

What evidence counts toward, by reason:

| Reason | Counts toward |
|---|---|
| fit_too_small, fit_too_large | fit |
| damaged_or_defective, not_arrived | seller reliability |
| changed_mind, unclear, none | nothing |

How a signal is written:

| Signal | Write | Promotes |
|---|---|---|
| Classified return or dispute, fit feedback | record_action | yes |
| The shopper's correction | record_action, citing the belief in `against` only | never: it overrides, it adds no evidence |
| General statement | derive, from the seed only | never |
| Kept past the window, no feedback | observe | never |

The statement row is the seed's: no tool records one, so the `derive`
there is written by the boot replay, and a statement the seed does not
carry does not exist. See "Where the general statement comes from".

Seeded means marked. Every seeded row carries the mark, and the ledger
shows it beside the row and in the seeded notice. The `shopper_01`
examples show one consistent state: the three October returns are seeded,
the dispute, the pending refund, the correction and the feedback are
live. The Earned pooled example is illustrative: its status and
direction match the seeded population's oxford line at size M, but its
counts do not. The Claimed and Building pooled examples carry the seeded
population's own rows for their lines, with no live rows yet, and every
example's `seeded` split is shown rather than passed off as lived sales.
Answering the pending question would add a fourth fit row to the
evidence ledger; the snapshots here predate that answer.

## What the layer will not do

- Refuse a recall because the evidence is thin. It labels the evidence
  and says which step decided, and says plainly when nothing applies.
- Show one shopper's rows to another shopper, or to the merchant app.
  The pool carries counts, never a row.
- Turn a refund or dispute into a belief without a reason that matches
  it. A non-arrival dispute counts against the seller, never toward
  sizing; a changed mind counts toward nothing.
- Promote a general statement or a standing correction. "I'm usually an
  M" is Claimed until fit evidence says otherwise.
- Delete a belief. The shopper's correction wins at once, with
  precedence, and the belief stays beside it.
- Move money. Purchases, approval and idempotency live in your agent.
