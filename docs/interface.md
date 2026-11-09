# Interface

Memento Emptor is a memory layer for shopping agents, not an agent. Your
agent plugs into it: evidence arrives from the payment side without your
agent doing anything, your agent asks what applies before it recommends,
and it reports what the shopper says. This document is written for a
developer building a shopping agent on the payment toolkit. It says what
to call, when, and what comes back.

Every example in this document validates against the frozen schemas in
`testdata/api/`, and the gate checks that. The schemas are final: the
tool names, the request shapes and the response shapes do not change
without a new contract task.

## Calling convention

The surface is nine MCP tools: six layer calls and three ledger reads.
Discover them with `tools/list`; call them with `tools/call`. Every call
is scoped to one shopper by `shopper_id`. Sessions isolate shoppers, so
one shopper's beliefs never leak into another's recall.

Reads (`recall_for_purchase`, `explain_belief`, the three `ledger_*`
reads) take no trace id. Writes (`record_correction`, `record_reason`,
`record_feedback`) take an optional `trace_id`: pass the host trace for
the turn, so per-trace review can show which turn recorded what. The
response echoes it back. The reference host records feedback per trace
and attaches traces of interest as dataset items; the trace id is the
join between a write and the turn that made it.

The layer never moves money. Orders, approval and double-buy protection
live in your agent, on the toolkit's orders tools with idempotent
request ids. The layer only remembers what happened to the money.

## The turn pattern

1. **Recall first.** Before any turn that may end in a recommendation,
   call `recall_for_purchase`. Your trace should show recall before the
   recommendation: that ordering is the independent proof your agent
   checked memory first.
2. **Recommend from the guidance.** The response carries `guidance`:
   what to recommend now, and whether it comes from a correction, a
   belief, or nothing yet. When `overriding_correction` is present,
   recommend from the correction. It is also embedded in each
   contradicted belief, so it cannot be read past.
3. **Ask about the gaps.** `pending_count` above zero means refunds are
   waiting on the shopper. Call `pending_questions` and ask about the
   first one in chat. An unanswered refund counts toward nothing.
4. **Record what the shopper says.** Corrections go to
   `record_correction` at once; answers go to `record_reason`; how the
   purchased size worked out goes to `record_feedback` after delivery.

## recall_for_purchase

Call it before recommending. Brand scopes the recall; line, SKU and the
leaned-toward size narrow it when known. It returns the beliefs that
apply, each with its ledger status and its evidence, plus any correction
that overrides them.

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

Response. The belief says size L, Earned over three documented returns,
but the shopper's correction says M — so the status reads Out of true
while the standing stays Venerable, the warning rides inline, the
correction ships again at the top level, and the guidance points at the
correction:

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
      "seeded": true,
      "updated_at": "2026-11-05T14:02:11Z"
    }
  ],
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

## record_correction

Call it the moment the shopper corrects you, in the shopper's verbatim
first-person words. `contradicts_belief_id` is required: the correction
is declared against the belief it contradicts, never floating. It
applies at once with precedence; the contradicted belief keeps its
status and standing and renders its warning beside it.

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
| Claimed | None | The shopper said it in general, or one data point exists. Never promotes on its own. |
| Building | Candidate | Evidence is accumulating. |
| Earned | Venerable | Enough documented evidence, over enough time, to act on. |
| Out of true | Venerable | A correction applies. Same standing, with the warning. |

What evidence counts toward, by reason:

| Reason | Counts toward |
|---|---|
| fit_too_small, fit_too_large | fit |
| damaged_or_defective, not_arrived | seller reliability |
| changed_mind, unclear, none | nothing |

How a signal is written:

| Signal | Write | Promotes |
|---|---|---|
| Classified return or dispute, fit feedback, correction | record_action | yes |
| General statement | derive | never |
| Kept past the window, no feedback | observe | never |

Seeded means marked. Every seeded row carries the mark, and the ledger
shows it beside the row and in the seeded notice. The examples above
show one consistent state: the three October returns are seeded, the
dispute, the pending refund, the correction and the feedback are live.
Answering the pending question would add a fourth fit row to the
evidence ledger; the snapshots here predate that answer.

## What the layer will not do

- Turn a refund or dispute into a belief without a reason that matches
  it. A non-arrival dispute counts against the seller, never toward
  sizing; a changed mind counts toward nothing.
- Promote a general statement. "I'm usually an M" is Claimed until fit
  evidence says otherwise.
- Delete a belief. The shopper's correction wins at once, with
  precedence, and the belief stays beside it.
- Move money. Purchases, approval and idempotency live in your agent.
