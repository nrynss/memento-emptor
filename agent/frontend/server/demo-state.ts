/**
 * Demo-fixture memory for the shopper page agent.
 *
 * Standing configuration, not a gap: the layer is not served yet, so this
 * agent answers from the RECORDED demo-shopper fixture — the same ids and
 * words the reference agent replays (belief blf_8f2c41 with its three
 * seeded fit returns, the recorded correction corr_3b7e52, the pending
 * refund question). Field shapes match the frozen layer contract; only the
 * backend is a fixture. No secrets, no network, no keys.
 *
 * State is memory-only: a live correction recorded through the page is
 * kept in this container and resets on redeploy. That is a consequence of
 * the production read-only filesystem, and it doubles as the demo reset.
 */

export const DEMO_SHOPPER = "shopper_01";
export const DEMO_BRAND_ID = "brand-x-001";
export const SIZING_BELIEF_ID = "blf_8f2c41";

interface Correction {
  correction_id: string;
  text: string;
  contradicts_belief_id: string;
  recorded_at: string;
  takes_precedence: boolean;
}

function fitReturn(evidenceId: string, n: string, text: string, occurredAt: string) {
  return {
    evidence_id: evidenceId,
    outcome: { kind: "refund", status: "COMPLETED", amount_minor: 19500, currency: "USD" },
    item: {
      brand_id: DEMO_BRAND_ID,
      brand_name: "Brand X",
      line_name: "oxford shirt",
      sku: "tbnrl46",
      size: "M",
      category: "shirts",
    },
    reason: { category: "fit_too_small", text, confidence: 1.0 },
    counts_toward: "fit",
    write_op: "record_action",
    link: {
      order_id: `ORD-7XK-0${n}`,
      capture_id: `CAP-7XK-0${n}`,
      refund_id: `RFD-7XK-0${n}`,
      custom_id: `memento-shopper01-0${n}`,
      event_id: `WH-2EV-0${n}`,
    },
    seeded: true,
    occurred_at: occurredAt,
  };
}

// The recorded correction from the demo: kept as the fixture starting
// point so every cited id matches the recorded run. A live correction
// through the page supersedes it in this container's memory.
const RECORDED_CORRECTION: Correction = {
  correction_id: "corr_3b7e52",
  text: "I've lost weight, use M",
  contradicts_belief_id: SIZING_BELIEF_ID,
  recorded_at: "2026-11-05T14:02:11Z",
  takes_precedence: true,
};

const EVIDENCE = [
  fitReturn("ev_6d1a09", "1", "too tight in the shoulders", "2026-10-14T10:00:00Z"),
  fitReturn("ev_6d1a10", "2", "too tight across the chest", "2026-10-20T10:00:00Z"),
  fitReturn("ev_6d1a11", "3", "sleeves too short in M", "2026-10-26T10:00:00Z"),
];

const PENDING_QUESTION = {
  question_id: "q_9f04ab",
  outcome: { kind: "refund", status: "COMPLETED", amount_minor: 19500, currency: "USD" },
  item: {
    brand_id: DEMO_BRAND_ID,
    brand_name: "Brand X",
    line_name: "oxford shirt",
    sku: "tbnrl46",
    size: "M",
    category: "shirts",
  },
  link: {
    order_id: "ORD-7XK-05",
    capture_id: "CAP-7XK-05",
    refund_id: "RFD-7XK-05",
    custom_id: "memento-shopper01-05",
    event_id: "WH-2EV-05",
  },
  seeded: false,
  asked_count: 1,
  last_asked_at: "2026-11-08T18:40:00Z",
  awaiting_since: "2026-11-07T09:15:00Z",
};

// Live corrections recorded through the page chat. Memory-only by design.
let liveCorrection: Correction | null = null;
let liveCorrectionCount = 0;

function activeCorrection(): Correction {
  return liveCorrection ?? RECORDED_CORRECTION;
}

function sizingBelief(correction: Correction) {
  return {
    belief_id: SIZING_BELIEF_ID,
    kind: "fit",
    subject: { kind: "brand", id: DEMO_BRAND_ID, name: "Brand X" },
    value: "runs small, take size L",
    status: "Out of true",
    standing: "Venerable",
    contradicted: true,
    contradicted_by: correction,
    evidence: EVIDENCE,
    seeded: true,
    updated_at: correction.recorded_at,
  };
}

/** Recall what applies before recommending, per the frozen contract shape. */
export function recallForPurchase(shopperId: string, traceId?: string) {
  if (shopperId !== DEMO_SHOPPER) {
    return {
      shopper_id: shopperId,
      beliefs: [],
      overriding_correction: null,
      guidance: { text: "No memory for this shopper yet; ask what size they take.", basis: "none" },
      pending_count: 0,
      ...(traceId ? { trace_id: traceId } : {}),
    };
  }
  const correction = activeCorrection();
  return {
    shopper_id: shopperId,
    beliefs: [sizingBelief(correction)],
    overriding_correction: correction,
    guidance: {
      text: correction.text,
      basis: "correction",
      correction_id: correction.correction_id,
      belief_id: SIZING_BELIEF_ID,
    },
    pending_count: 1,
    pending_first_question: PENDING_QUESTION,
    ...(traceId ? { trace_id: traceId } : {}),
  };
}

/** Record the shopper's live correction at once; it takes precedence. */
export function recordCorrection(
  shopperId: string,
  text: string,
  contradictsBeliefId: string,
  traceId?: string,
) {
  void shopperId;
  liveCorrectionCount += 1;
  liveCorrection = {
    correction_id: `corr_page${String(liveCorrectionCount).padStart(2, "0")}`,
    text,
    contradicts_belief_id: contradictsBeliefId,
    recorded_at: new Date().toISOString(),
    takes_precedence: true,
  };
  const correction = liveCorrection;
  return {
    correction,
    belief_id: contradictsBeliefId,
    status: "Out of true",
    standing: "Venerable",
    contradicted: true,
    guidance: {
      text,
      basis: "correction",
      correction_id: correction.correction_id,
      belief_id: contradictsBeliefId,
    },
    ...(traceId ? { trace_id: traceId } : {}),
  };
}
