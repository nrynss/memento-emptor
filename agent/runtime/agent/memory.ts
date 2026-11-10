/**
 * Fake-backed memory for the reference agent.
 *
 * Standing configuration, not a gap: the layer is not served to this agent
 * yet (T9.6 wires it), so the deployed agent talks to these fake-backed
 * tools. Every response follows the frozen contract in testdata/api/: the
 * demo correction story (shopper_01) plus one shopper per remaining
 * precedence step (shopper_02 pool, shopper_04 own evidence, shopper_06
 * statement, anyone else none), and the standing correction that recall
 * with no belief produces. Tool names and shapes are exactly what the live
 * layer serves, so swapping the backend later changes no tool the agent
 * reads. No secrets, no network, no keys.
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

// The pooled product fit both recall scenarios share. Counts mirror the
// frozen contract example: an Earned-small pool over 60 settled purchases
// in size M, with the seeded/live split the ledger shows.
const POOL_COUNTS = {
  settled: 60,
  kept: 6,
  returns: { too_small: 24, too_large: 4 },
  feedback: { fits_well: 12, too_small: 5, too_large: 2 },
  reasons: {
    fit_too_small: 24,
    fit_too_large: 4,
    damaged_or_defective: 2,
    not_arrived: 1,
    changed_mind: 4,
  },
  shoppers: { fit: 39, too_small: 25, too_large: 6 },
};

const POOL_SEEDED = {
  settled: 48,
  kept: 5,
  returns: { too_small: 20, too_large: 3 },
  feedback: { fits_well: 9, too_small: 4, too_large: 2 },
  reasons: {
    fit_too_small: 20,
    fit_too_large: 3,
    damaged_or_defective: 1,
    not_arrived: 1,
    changed_mind: 3,
  },
  shoppers: { fit: 30, too_small: 19, too_large: 5 },
};

const POOL_LIVE = {
  settled: 12,
  kept: 1,
  returns: { too_small: 4, too_large: 1 },
  feedback: { fits_well: 3, too_small: 1, too_large: 0 },
  reasons: {
    fit_too_small: 4,
    fit_too_large: 1,
    damaged_or_defective: 1,
    not_arrived: 0,
    changed_mind: 1,
  },
  shoppers: { fit: 9, too_small: 6, too_large: 1 },
};

const POOL_RECENT = {
  settled: 44,
  kept: 5,
  returns: { too_small: 20, too_large: 3 },
  feedback: { fits_well: 8, too_small: 4, too_large: 1 },
  reasons: {
    fit_too_small: 20,
    fit_too_large: 3,
    damaged_or_defective: 1,
    not_arrived: 0,
    changed_mind: 2,
  },
  shoppers: { fit: 22, too_small: 18, too_large: 3 },
};

function poolView(overridden: boolean, overriddenBy: string | null) {
  return {
    line: { brand_id: DEMO_BRAND, category: "shirts", fit_handle: "regular" },
    size: "M",
    status: "Earned",
    direction: "small",
    counts: POOL_COUNTS,
    seeded: POOL_SEEDED,
    live: POOL_LIVE,
    recent: POOL_RECENT,
    precedence: { step: "pool", overridden, overridden_by: overriddenBy },
  };
}

// A new shopper's only words: their usual size, Claimed, never evidence.
// On the pool line the pool overrides it; on a lineless recall it decides.
function statementBelief(overridden: boolean, overriddenBy: string | null) {
  return {
    belief_id: "blf_77c310",
    kind: "fit",
    subject: { kind: "brand", id: DEMO_BRAND, name: "Brand X" },
    value: "shopper says they are usually size M",
    status: "Claimed",
    standing: "None",
    contradicted: false,
    contradicted_by: null,
    evidence: [],
    precedence: { step: "statement", overridden, overridden_by: overriddenBy },
    seeded: false,
    updated_at: "2026-11-08T09:20:00Z",
  };
}

export function recallForPurchase(shopperId: string, traceId?: string) {
  if (shopperId === DEMO_SHOPPER) {
    return {
      shopper_id: shopperId,
      beliefs: [
        {
          ...SIZING_BELIEF,
          precedence: { step: "own_evidence", overridden: true, overridden_by: "correction" },
        },
      ],
      pool: poolView(true, "correction"),
      pool_lookup: "consulted",
      overriding_correction: CORRECTION,
      guidance: GUIDANCE,
      pending_count: 1,
      ...(traceId ? { trace_id: traceId } : {}),
    };
  }
  // A new shopper on the seeded Earned-small line: the pool decides from
  // the usual size, and the shopper's own Claimed statement rides along
  // overridden, never hidden.
  if (shopperId === "shopper_02") {
    return {
      shopper_id: shopperId,
      beliefs: [statementBelief(true, "pool")],
      pool: poolView(false, null),
      pool_lookup: "consulted",
      overriding_correction: null,
      guidance: {
        text: "Size up to L for Brand X: 29 of the 47 fit-classified settled purchases at size M came back too small, against 6 too large.",
        basis: "pool",
        correction_id: null,
        belief_id: null,
      },
      pending_count: 0,
      ...(traceId ? { trace_id: traceId } : {}),
    };
  }
  // A shopper unlike the crowd: their own Building evidence beats the
  // disagreeing Earned-small pool, which rides along overridden.
  if (shopperId === "shopper_04") {
    return {
      shopper_id: shopperId,
      beliefs: [
        {
          belief_id: "blf_9b31c7",
          kind: "fit",
          subject: { kind: "brand", id: DEMO_BRAND, name: "Brand X" },
          value: "runs large, take size S",
          status: "Building",
          standing: "Candidate",
          contradicted: false,
          contradicted_by: null,
          evidence: [
            {
              evidence_id: "ev_2f7a01",
              outcome: { kind: "refund", status: "COMPLETED", amount_minor: 19500, currency: "USD" },
              item: {
                brand_id: DEMO_BRAND,
                brand_name: "Brand X",
                line_name: "oxford shirt",
                sku: "tbnrl46",
                size: "M",
                category: "shirts",
              },
              reason: { category: "fit_too_large", text: "too loose across the back", confidence: 1.0 },
              counts_toward: "fit",
              write_op: "record_action",
              link: {
                order_id: "ORD-9QM-01",
                capture_id: "CAP-9QM-01",
                refund_id: "RFD-9QM-01",
                custom_id: "memento-shopper04-01",
                event_id: "WH-3EV-01",
              },
              seeded: true,
              occurred_at: "2026-10-10T10:00:00Z",
            },
            {
              evidence_id: "ev_2f7a02",
              outcome: { kind: "refund", status: "COMPLETED", amount_minor: 19500, currency: "USD" },
              item: {
                brand_id: DEMO_BRAND,
                brand_name: "Brand X",
                line_name: "oxford shirt",
                sku: "tbnrl46",
                size: "M",
                category: "shirts",
              },
              reason: { category: "fit_too_large", text: "sleeves too long in M", confidence: 1.0 },
              counts_toward: "fit",
              write_op: "record_action",
              link: {
                order_id: "ORD-9QM-02",
                capture_id: "CAP-9QM-02",
                refund_id: "RFD-9QM-02",
                custom_id: "memento-shopper04-02",
                event_id: "WH-3EV-02",
              },
              seeded: true,
              occurred_at: "2026-10-18T10:00:00Z",
            },
          ],
          precedence: { step: "own_evidence", overridden: false, overridden_by: null },
          seeded: true,
          updated_at: "2026-10-18T10:00:00Z",
        },
      ],
      pool: poolView(true, "own_evidence"),
      pool_lookup: "consulted",
      overriding_correction: null,
      guidance: {
        text: "Take size S for Brand X: your own two returns say it runs large, ahead of the pool.",
        basis: "own_evidence",
        correction_id: null,
        belief_id: "blf_9b31c7",
      },
      pending_count: 0,
      ...(traceId ? { trace_id: traceId } : {}),
    };
  }
  // A shopper with only their usual size and no pool: the Claimed
  // statement decides, labelled as words, not evidence.
  if (shopperId === "shopper_06") {
    return {
      shopper_id: shopperId,
      beliefs: [
        {
          belief_id: "blf_9d44e2",
          kind: "fit",
          subject: { kind: "brand", id: "brand-s-009", name: "Brand S" },
          value: "shopper says they are usually size M",
          status: "Claimed",
          standing: "None",
          contradicted: false,
          contradicted_by: null,
          evidence: [],
          precedence: { step: "statement", overridden: false, overridden_by: null },
          seeded: false,
          updated_at: "2026-11-07T10:00:00Z",
        },
      ],
      pool_lookup: "consulted",
      overriding_correction: null,
      guidance: {
        text: "Take size M for Brand S: the shopper's usual size.",
        basis: "statement",
        correction_id: null,
        belief_id: "blf_9d44e2",
      },
      pending_count: 0,
      ...(traceId ? { trace_id: traceId } : {}),
    };
  }
  // A correction recorded while no belief existed stays in force: it
  // decides again on a later turn, in this container's memory only.
  const standing = standingCorrections.get(shopperId);
  if (standing) {
    return {
      shopper_id: shopperId,
      beliefs: [],
      pool_lookup: "consulted",
      overriding_correction: standing,
      guidance: {
        text: `Take size ${standing.size} for Brand X`,
        basis: "correction",
        correction_id: standing.correction_id,
        belief_id: null,
      },
      pending_count: 0,
      ...(traceId ? { trace_id: traceId } : {}),
    };
  }
  // Nobody the layer knows: it never refuses, it says plainly that no
  // memory applies yet.
  return {
    shopper_id: shopperId,
    beliefs: [],
    pool_lookup: "consulted",
    overriding_correction: null,
    guidance: {
      text: "No memory applies yet: no correction, no fit evidence, no pool and no general statement. Ask the shopper which size they usually take.",
      basis: "none",
    },
    pending_count: 0,
    ...(traceId ? { trace_id: traceId } : {}),
  };
}

// Live corrections persist in container memory only (the demo reset is a
// redeploy). A correction recorded while recall served no belief stands on
// its own and decides again on a later turn, until the container restarts.
const standingCorrections = new Map<string, { correction_id: string; text: string; size: string; recorded_at: string; takes_precedence: true }>();

function lastSizeToken(text: string): string {
  const m = text.match(/\b(XXL|XL|XS|S|M|L|\d{2})\b(?=[^.]*$)/i);
  return m ? m[1].toUpperCase() : "";
}

export function recordCorrection(shopperId: string, text: string, contradictsBeliefId?: string, traceId?: string) {
  if (!contradictsBeliefId) {
    const size = lastSizeToken(text);
    const correction = {
      correction_id: `corr_stand_${String(standingCorrections.size + 1).padStart(2, "0")}`,
      text,
      recorded_at: new Date().toISOString(),
      takes_precedence: true as const,
    };
    standingCorrections.set(shopperId, { ...correction, size });
    return {
      correction,
      status: "Claimed",
      standing: "None",
      contradicted: false,
      guidance: {
        text: size ? `Take size ${size}` : "Follow the shopper's correction",
        basis: "correction",
        correction_id: correction.correction_id,
        belief_id: null,
      },
      ...(traceId ? { trace_id: traceId } : {}),
    };
  }
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

/**
 * Pooled product fit for one line and size, fake-backed like the rest:
 * settled counts with their seeded split, never a shopper row. Unknown
 * lines are Claimed with zero counts: the pool says nothing, not nothing
 * settled.
 */
export function productFit(brandId: string, category: string, fitHandle: string, size: string) {
  if (brandId === DEMO_BRAND && category === "shirts" && fitHandle === "regular" && size === "M") {
    return {
      pool: {
        line: { brand_id: brandId, category, fit_handle: fitHandle },
        size,
        status: "Earned",
        direction: "small",
        counts: POOL_COUNTS,
        seeded: POOL_SEEDED,
        live: POOL_LIVE,
        recent: POOL_RECENT,
      },
    };
  }
  const zero = {
    settled: 0,
    kept: 0,
    returns: { too_small: 0, too_large: 0 },
    feedback: { fits_well: 0, too_small: 0, too_large: 0 },
    reasons: {
      fit_too_small: 0,
      fit_too_large: 0,
      damaged_or_defective: 0,
      not_arrived: 0,
      changed_mind: 0,
    },
    shoppers: { fit: 0, too_small: 0, too_large: 0 },
  };
  return {
    pool: {
      line: { brand_id: brandId, category, fit_handle: fitHandle },
      size,
      status: "Claimed",
      direction: null,
      counts: zero,
      seeded: zero,
      live: zero,
      recent: zero,
    },
  };
}

/**
 * The shopper's pooling choice, fake-backed: opting out reports the rows
 * removed, opting back in removes nothing.
 */
export function setPooling(shopperId: string, pooling: boolean, traceId?: string) {
  return {
    shopper_id: shopperId,
    pooling,
    rows_removed: pooling ? 0 : 1,
    ...(traceId ? { trace_id: traceId } : {}),
  };
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
