/**
 * Fake-backed memory for the reference agent.
 *
 * Standing configuration, not a gap: the layer is not served yet (T2.5 is
 * later), so the deployed agent initially talks to these fake-backed tools.
 * Every response below matches the frozen contract in testdata/api/ and the
 * replay in testdata/replay/p3-turns.json. Tool names and shapes are exactly
 * what the live layer will serve, so swapping the backend later changes no
 * tool the agent reads. No secrets, no network, no keys.
 */

export const DEMO_SHOPPER = "shopper_01";
export const DEMO_BRAND = "brand-x-001";
export const SIZING_BELIEF_ID = "blf_8f2c41";

const fitToward = "fit";

function fitReturn(
  evidenceId: string,
  n: string,
  text: string,
  occurredAt: string,
) {
  return {
    evidence_id: evidenceId,
    outcome: { kind: "refund", status: "COMPLETED", amount_minor: 19500, currency: "USD" },
    item: {
      brand_id: DEMO_BRAND,
      brand_name: "Brand X",
      line_name: "oxford shirt",
      sku: "tbnrl46",
      size: "M",
      category: "shirts",
    },
    reason: { category: "fit_too_small", text, confidence: 1.0 },
    counts_toward: fitToward,
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

export const CORRECTION = {
  correction_id: "corr_3b7e52",
  text: "I've lost weight, use M",
  contradicts_belief_id: SIZING_BELIEF_ID,
  recorded_at: "2026-11-05T14:02:11Z",
  takes_precedence: true,
};

const R1 = fitReturn("ev_6d1a09", "1", "too tight in the shoulders", "2026-10-14T10:00:00Z");
const R2 = fitReturn("ev_6d1a10", "2", "too tight across the chest", "2026-10-20T10:00:00Z");
const R3 = fitReturn("ev_6d1a11", "3", "sleeves too short in M", "2026-10-26T10:00:00Z");

export const SIZING_BELIEF = {
  belief_id: SIZING_BELIEF_ID,
  kind: "fit",
  subject: { kind: "brand", id: DEMO_BRAND, name: "Brand X" },
  value: "runs small, take size L",
  status: "Out of true",
  standing: "Venerable",
  contradicted: true,
  contradicted_by: CORRECTION,
  evidence: [R1, R2, R3],
  seeded: true,
  updated_at: "2026-11-05T14:02:11Z",
};

const FEEDBACK_EVIDENCE = {
  evidence_id: "ev_51c2d8",
  outcome: { kind: "capture", status: "COMPLETED", amount_minor: 19500, currency: "USD" },
  item: {
    brand_id: DEMO_BRAND,
    brand_name: "Brand X",
    line_name: "oxford shirt",
    sku: "PSW8GeK",
    size: "M",
    category: "shirts",
  },
  reason: { category: "none", text: "the M fits well across the shoulders", confidence: 1.0 },
  counts_toward: fitToward,
  write_op: "record_action",
  link: {
    order_id: "ORD-7XK-06",
    capture_id: "CAP-7XK-06",
    custom_id: "memento-shopper01-06",
    event_id: "WH-2EV-06",
  },
  seeded: false,
  occurred_at: "2026-11-08T20:10:00Z",
};

const RIVAL_BELIEF = {
  belief_id: "blf_2b9d60",
  kind: "fit",
  subject: { kind: "brand", id: DEMO_BRAND, name: "Brand X" },
  value: "size M fits this shopper now",
  status: "Claimed",
  standing: "None",
  contradicted: false,
  contradicted_by: null,
  evidence: [FEEDBACK_EVIDENCE],
  seeded: false,
  updated_at: "2026-11-08T20:10:00Z",
};

const DISPUTE_EVIDENCE = {
  evidence_id: "ev_9d4c22",
  outcome: { kind: "dispute", status: "OPEN", amount_minor: 8900, currency: "USD" },
  item: {
    brand_id: DEMO_BRAND,
    brand_name: "Brand X",
    line_name: "chino",
    sku: "chino-khaki-32",
    size: "32",
    category: "pants",
  },
  reason: {
    category: "not_arrived",
    text: "MERCHANDISE_OR_SERVICE_NOT_RECEIVED",
    confidence: 1.0,
  },
  counts_toward: "seller_reliability",
  write_op: "record_action",
  link: {
    order_id: "ORD-7XK-04",
    capture_id: "CAP-7XK-04",
    dispute_id: "PP-D-51713",
    custom_id: "memento-shopper01-04",
    event_id: "WH-2EV-04",
  },
  seeded: false,
  occurred_at: "2026-11-06T11:30:00Z",
};

const SELLER_BELIEF = {
  belief_id: "blf_1a5e09",
  kind: "seller_reliability",
  subject: { kind: "seller", id: "seller-acme-01", name: "Acme" },
  value: "ships reliably for this shopper",
  status: "Claimed",
  standing: "None",
  contradicted: false,
  contradicted_by: null,
  evidence: [DISPUTE_EVIDENCE],
  seeded: false,
  updated_at: "2026-11-06T11:30:00Z",
};

const STATEMENT_BELIEF = {
  belief_id: "blf_77c310",
  kind: "fit",
  subject: { kind: "brand", id: DEMO_BRAND, name: "Brand X" },
  value: "shopper says they are usually size M",
  status: "Claimed",
  standing: "None",
  contradicted: false,
  contradicted_by: null,
  evidence: [],
  seeded: true,
  updated_at: "2026-10-14T10:00:00Z",
};

const GUIDANCE = {
  text: "Take size M for Brand X",
  basis: "correction",
  correction_id: CORRECTION.correction_id,
  belief_id: SIZING_BELIEF_ID,
};

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
  return {
    shopper_id: shopperId,
    beliefs: [SIZING_BELIEF],
    overriding_correction: CORRECTION,
    guidance: GUIDANCE,
    pending_count: 1,
    ...(traceId ? { trace_id: traceId } : {}),
  };
}

export function recordCorrection(shopperId: string, text: string, contradictsBeliefId: string, traceId?: string) {
  void shopperId;
  return {
    correction: {
      correction_id: "corr_live01",
      text,
      contradicts_belief_id: contradictsBeliefId,
      recorded_at: new Date().toISOString(),
      takes_precedence: true,
    },
    belief_id: contradictsBeliefId,
    status: "Out of true",
    standing: "Venerable",
    contradicted: true,
    guidance: {
      text,
      basis: "correction",
      correction_id: "corr_live01",
      belief_id: contradictsBeliefId,
    },
    ...(traceId ? { trace_id: traceId } : {}),
  };
}

const ASK_AGAIN = {
  reason_id: "rsn_77aa11",
  question_id: "q_9f04ab",
  category: "unclear",
  confidence: 0.94,
  counts_toward: null,
  accepted: false,
  ask_again: true,
  belief_id: null,
};

export function recordReason(questionId: string, answerText: string, traceId?: string) {
  const clear = /tight|small|short|sleeve|button|loose|large|big/i.test(answerText);
  if (!clear) {
    return { ...ASK_AGAIN, question_id: questionId, ...(traceId ? { trace_id: traceId } : {}) };
  }
  return {
    reason_id: "rsn_live01",
    question_id: questionId,
    category: "fit_too_small",
    confidence: 1.0,
    counts_toward: "fit",
    accepted: true,
    ask_again: false,
    belief_id: SIZING_BELIEF_ID,
    ...(traceId ? { trace_id: traceId } : {}),
  };
}

export function recordFeedback(
  customId: string,
  sku: string,
  size: string,
  feedbackText: string,
  traceId?: string,
) {
  void customId;
  void sku;
  void size;
  const fitsWell = /fits?\s+well/i.test(feedbackText);
  return {
    feedback_id: "fb_live01",
    verdict: fitsWell ? "fits_well" : "unclear",
    counts_toward: fitsWell ? "fit" : null,
    belief_id: fitsWell ? "blf_2b9d60" : null,
    accepted: fitsWell,
    ...(traceId ? { trace_id: traceId } : {}),
  };
}

const PENDING_QUESTION = {
  question_id: "q_9f04ab",
  outcome: { kind: "refund", status: "COMPLETED", amount_minor: 19500, currency: "USD" },
  item: {
    brand_id: DEMO_BRAND,
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

export function pendingQuestions(shopperId: string) {
  if (shopperId !== DEMO_SHOPPER) {
    return { shopper_id: shopperId, questions: [] };
  }
  return { shopper_id: shopperId, questions: [PENDING_QUESTION] };
}

export function explainBelief(beliefId: string) {
  if (beliefId !== SIZING_BELIEF_ID) {
    return null;
  }
  return {
    belief: SIZING_BELIEF,
    corrections: [CORRECTION],
    timeline: [
      { at: "2026-10-14T10:00:00Z", kind: "evidence", summary: "Fit return 1: oxford shirt size M refunded; too tight in the shoulders.", ref_id: "ev_6d1a09" },
      { at: "2026-10-20T10:00:00Z", kind: "evidence", summary: "Fit return 2: oxford shirt size M refunded; too tight across the chest.", ref_id: "ev_6d1a10" },
      { at: "2026-10-26T10:00:00Z", kind: "evidence", summary: "Fit return 3: oxford shirt size M refunded; sleeves too short in M.", ref_id: "ev_6d1a11" },
      { at: "2026-10-26T10:00:05Z", kind: "status", summary: "Belief reached Earned: three documented fit returns over twelve days.", ref_id: SIZING_BELIEF_ID },
      { at: "2026-11-05T14:02:11Z", kind: "correction", summary: "Shopper correction: lost weight, take size M for Brand X.", ref_id: "corr_3b7e52" },
      { at: "2026-11-08T20:10:00Z", kind: "feedback", summary: "Fit feedback: oxford shirt size M fits well.", ref_id: "fb_51c2d8" },
    ],
  };
}

export function ledgerBeliefs(shopperId: string) {
  void shopperId;
  return {
    shopper_id: shopperId,
    beliefs: [
      { belief: SIZING_BELIEF, evidence_count: 3, last_change_at: "2026-11-05T14:02:11Z" },
      { belief: RIVAL_BELIEF, evidence_count: 1, last_change_at: "2026-11-08T20:10:00Z" },
      { belief: SELLER_BELIEF, evidence_count: 1, last_change_at: "2026-11-06T11:30:00Z" },
      { belief: STATEMENT_BELIEF, evidence_count: 0, last_change_at: "2026-10-14T10:00:00Z" },
    ],
  };
}

export function ledgerBelief(beliefId: string) {
  const rows = ledgerBeliefs(DEMO_SHOPPER).beliefs;
  const row = rows.find((r) => r.belief.belief_id === beliefId);
  if (!row) {
    return null;
  }
  return {
    belief: row.belief,
    corrections: row.belief.belief_id === SIZING_BELIEF_ID ? [CORRECTION] : [],
    seeded_notice:
      row.belief.belief_id === SIZING_BELIEF_ID
        ? "Seeded history: generated, not lived. The three returns are seeded; the correction is live."
        : undefined,
  };
}

export function ledgerEvidence(shopperId: string) {
  void shopperId;
  return {
    shopper_id: shopperId,
    evidence: [
      { evidence: FEEDBACK_EVIDENCE, belief_ids: ["blf_2b9d60"] },
      { evidence: DISPUTE_EVIDENCE, belief_ids: ["blf_1a5e09"] },
      { evidence: R3, belief_ids: [SIZING_BELIEF_ID] },
      { evidence: R2, belief_ids: [SIZING_BELIEF_ID] },
      { evidence: R1, belief_ids: [SIZING_BELIEF_ID] },
    ],
  };
}

/**
 * PayPal-flavoured order read, fake-backed like the T0.8 probe's lookupReturn.
 * Live PayPal orders and money movement belong to T3.3 (agent/orders/); this
 * task owns no purchase-execution tool, so nothing here can move money.
 */
export function lookupReturn(orderId: string) {
  if (orderId === "ORD-7XK-01") {
    return {
      order_id: orderId,
      item: "Brand X oxford shirt, size M",
      outcome: "refunded",
      reason: "too tight in the shoulders",
    };
  }
  return { order_id: orderId, item: null, outcome: "unknown", reason: null };
}
