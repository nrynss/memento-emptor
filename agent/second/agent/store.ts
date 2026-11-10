/**
 * Stand-in layer state for the second agent.
 *
 * Standing configuration, not a gap: the served layer (T9.6) is not wired
 * to either deployment yet, so this deployment answers from the same frozen
 * demo-shopper state the reference agent answers from — the shopper_01
 * correction story, the Earned-small pooled line, and the ledger rows.
 * The values (belief and correction ids, the three returns, the pooled
 * tallies with their seeded split) are the layer's state, so a correction
 * the reference agent records for this shopper is the correction this
 * agent recalls. No secrets, no network, no keys.
 */

export const SHOPPER = "shopper_01";
export const BRAND = "brand-x-001";
export const FIT_BELIEF = "blf_8f2c41";

const fitKind = "fit";

function pastReturn(tag: string, seq: string, words: string, when: string) {
  return {
    evidence_id: tag,
    outcome: { kind: "refund", status: "COMPLETED", amount_minor: 19500, currency: "USD" },
    item: {
      brand_id: BRAND,
      brand_name: "Brand X",
      line_name: "oxford shirt",
      sku: "tbnrl46",
      size: "M",
      category: "shirts",
    },
    reason: { category: "fit_too_small", text: words, confidence: 1.0 },
    counts_toward: fitKind,
    write_op: "record_action",
    link: {
      order_id: `ORD-7XK-0${seq}`,
      capture_id: `CAP-7XK-0${seq}`,
      refund_id: `RFD-7XK-0${seq}`,
      custom_id: `memento-shopper01-0${seq}`,
      event_id: `WH-2EV-0${seq}`,
    },
    seeded: true,
    occurred_at: when,
  };
}

const FIRST = pastReturn("ev_6d1a09", "1", "too tight in the shoulders", "2026-10-14T10:00:00Z");
const SECOND = pastReturn("ev_6d1a10", "2", "too tight across the chest", "2026-10-20T10:00:00Z");
const THIRD = pastReturn("ev_6d1a11", "3", "sleeves too short in M", "2026-10-26T10:00:00Z");

export const SHOPPER_NOTE = {
  correction_id: "corr_3b7e52",
  text: "I've lost weight, use M",
  contradicts_belief_id: FIT_BELIEF,
  recorded_at: "2026-11-05T14:02:11Z",
  takes_precedence: true,
};

const FIT_RECORD = {
  belief_id: FIT_BELIEF,
  kind: "fit",
  subject: { kind: "brand", id: BRAND, name: "Brand X" },
  value: "runs small, take size L",
  status: "Out of true",
  standing: "Venerable",
  contradicted: true,
  contradicted_by: SHOPPER_NOTE,
  evidence: [FIRST, SECOND, THIRD],
  seeded: true,
  updated_at: "2026-11-05T14:02:11Z",
};

// The pooled line both deployments read: Earned toward small over sixty
// settled size-M purchases, split into its seeded and lived parts.
const POOL_ALL = {
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

const POOL_SEEDED_PART = {
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

const POOL_LIVED_PART = {
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

const POOL_WINDOW = {
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

function pooledLine(beaten: boolean, beatenBy: string | null) {
  return {
    line: { brand_id: BRAND, category: "shirts", fit_handle: "regular" },
    size: "M",
    status: "Earned",
    direction: "small",
    counts: POOL_ALL,
    seeded: POOL_SEEDED_PART,
    live: POOL_LIVED_PART,
    recent: POOL_WINDOW,
    precedence: { step: "pool", overridden: beaten, overridden_by: beatenBy },
  };
}

function claimedSize(beaten: boolean, beatenBy: string | null) {
  return {
    belief_id: "blf_77c310",
    kind: "fit",
    subject: { kind: "brand", id: BRAND, name: "Brand X" },
    value: "shopper says they are usually size M",
    status: "Claimed",
    standing: "None",
    contradicted: false,
    contradicted_by: null,
    evidence: [],
    precedence: { step: "statement", overridden: beaten, overridden_by: beatenBy },
    seeded: false,
    updated_at: "2026-11-08T09:20:00Z",
  };
}

export function recall(shopperId: string, traceId?: string) {
  if (shopperId === SHOPPER) {
    return {
      shopper_id: shopperId,
      beliefs: [
        {
          ...FIT_RECORD,
          precedence: { step: "own_evidence", overridden: true, overridden_by: "correction" },
        },
      ],
      pool: pooledLine(true, "correction"),
      pool_lookup: "consulted",
      overriding_correction: SHOPPER_NOTE,
      guidance: {
        text: "Take size M for Brand X",
        basis: "correction",
        correction_id: SHOPPER_NOTE.correction_id,
        belief_id: FIT_BELIEF,
      },
      pending_count: 1,
      ...(traceId ? { trace_id: traceId } : {}),
    };
  }
  // A shopper new to the brand: the Earned-small pool settles it from the
  // usual size, and their own Claimed usual-size note rides along beaten.
  if (shopperId === "shopper_02") {
    return {
      shopper_id: shopperId,
      beliefs: [claimedSize(true, "pool")],
      pool: pooledLine(false, null),
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
  // A note filed while recall served no belief keeps deciding later turns
  // in this container only; the demo reset is a redeploy.
  const held = heldNotes.get(shopperId);
  if (held) {
    return {
      shopper_id: shopperId,
      beliefs: [],
      pool_lookup: "consulted",
      overriding_correction: held,
      guidance: {
        text: `Take size ${held.size} for Brand X`,
        basis: "correction",
        correction_id: held.correction_id,
        belief_id: null,
      },
      pending_count: 0,
      ...(traceId ? { trace_id: traceId } : {}),
    };
  }
  // Strangers get honesty, never a refusal-shaped silence and never a guess.
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

// Notes filed live stay in container memory only (see recall above).
const heldNotes = new Map<string, { correction_id: string; text: string; size: string; recorded_at: string; takes_precedence: true }>();

function sizeFromWords(text: string): string {
  const m = text.match(/\b(XXL|XL|XS|S|M|L|\d{2})\b(?=[^.]*$)/i);
  return m ? m[1].toUpperCase() : "";
}

export function noteCorrection(shopperId: string, text: string, againstId?: string, traceId?: string) {
  if (!againstId) {
    const size = sizeFromWords(text);
    const note = {
      correction_id: `corr_stand_${String(heldNotes.size + 1).padStart(2, "0")}`,
      text,
      recorded_at: new Date().toISOString(),
      takes_precedence: true as const,
    };
    heldNotes.set(shopperId, { ...note, size });
    return {
      correction: note,
      status: "Claimed",
      standing: "None",
      contradicted: false,
      guidance: {
        text: size ? `Take size ${size}` : "Follow the shopper's correction",
        basis: "correction",
        correction_id: note.correction_id,
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
      contradicts_belief_id: againstId,
      recorded_at: new Date().toISOString(),
      takes_precedence: true,
    },
    belief_id: againstId,
    status: "Out of true",
    standing: "Venerable",
    contradicted: true,
    guidance: {
      text,
      basis: "correction",
      correction_id: "corr_live01",
      belief_id: againstId,
    },
    ...(traceId ? { trace_id: traceId } : {}),
  };
}

const VAGUE_SHAPE = {
  reason_id: "rsn_77aa11",
  question_id: "q_9f04ab",
  category: "unclear",
  confidence: 0.94,
  counts_toward: null,
  accepted: false,
  ask_again: true,
  belief_id: null,
};

export function answerReason(questionId: string, answerText: string, traceId?: string) {
  const clear = /tight|small|short|sleeve|button|loose|large|big/i.test(answerText);
  if (!clear) {
    return { ...VAGUE_SHAPE, question_id: questionId, ...(traceId ? { trace_id: traceId } : {}) };
  }
  return {
    reason_id: "rsn_live01",
    question_id: questionId,
    category: "fit_too_small",
    confidence: 1.0,
    counts_toward: "fit",
    accepted: true,
    ask_again: false,
    belief_id: FIT_BELIEF,
    ...(traceId ? { trace_id: traceId } : {}),
  };
}

export function noteFeedback(customId: string, sku: string, size: string, words: string, traceId?: string) {
  void customId;
  void sku;
  void size;
  const fits = /fits?\s+well/i.test(words);
  return {
    feedback_id: "fb_live01",
    verdict: fits ? "fits_well" : "unclear",
    counts_toward: fits ? "fit" : null,
    belief_id: fits ? "blf_2b9d60" : null,
    accepted: fits,
    ...(traceId ? { trace_id: traceId } : {}),
  };
}

const OPEN_ASK = {
  question_id: "q_9f04ab",
  outcome: { kind: "refund", status: "COMPLETED", amount_minor: 19500, currency: "USD" },
  item: {
    brand_id: BRAND,
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

export function openQuestions(shopperId: string) {
  if (shopperId !== SHOPPER) {
    return { shopper_id: shopperId, questions: [] };
  }
  return { shopper_id: shopperId, questions: [OPEN_ASK] };
}

export function pooledFit(brandId: string, category: string, handle: string, size: string) {
  if (brandId === BRAND && category === "shirts" && handle === "regular" && size === "M") {
    return {
      pool: {
        line: { brand_id: brandId, category, fit_handle: handle },
        size,
        status: "Earned",
        direction: "small",
        counts: POOL_ALL,
        seeded: POOL_SEEDED_PART,
        live: POOL_LIVED_PART,
        recent: POOL_WINDOW,
      },
    };
  }
  const bare = {
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
      line: { brand_id: brandId, category, fit_handle: handle },
      size,
      status: "Claimed",
      direction: null,
      counts: bare,
      seeded: bare,
      live: bare,
      recent: bare,
    },
  };
}

export function poolingChoice(shopperId: string, pooling: boolean, traceId?: string) {
  return {
    shopper_id: shopperId,
    pooling,
    rows_removed: pooling ? 0 : 1,
    ...(traceId ? { trace_id: traceId } : {}),
  };
}

const GOOD_FEEDBACK_ROW = {
  evidence_id: "ev_51c2d8",
  outcome: { kind: "capture", status: "COMPLETED", amount_minor: 19500, currency: "USD" },
  item: {
    brand_id: BRAND,
    brand_name: "Brand X",
    line_name: "oxford shirt",
    sku: "PSW8GeK",
    size: "M",
    category: "shirts",
  },
  reason: { category: "none", text: "the M fits well across the shoulders", confidence: 1.0 },
  counts_toward: fitKind,
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

const LOST_PARCEL = {
  evidence_id: "ev_9d4c22",
  outcome: { kind: "dispute", status: "OPEN", amount_minor: 8900, currency: "USD" },
  item: {
    brand_id: BRAND,
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

const FIT_ROWS_NOW = {
  belief_id: "blf_2b9d60",
  kind: "fit",
  subject: { kind: "brand", id: BRAND, name: "Brand X" },
  value: "size M fits this shopper now",
  status: "Claimed",
  standing: "None",
  contradicted: false,
  contradicted_by: null,
  evidence: [GOOD_FEEDBACK_ROW],
  seeded: false,
  updated_at: "2026-11-08T20:10:00Z",
};

const SELLER_ROWS = {
  belief_id: "blf_1a5e09",
  kind: "seller_reliability",
  subject: { kind: "seller", id: "seller-acme-01", name: "Acme" },
  value: "ships reliably for this shopper",
  status: "Claimed",
  standing: "None",
  contradicted: false,
  contradicted_by: null,
  evidence: [LOST_PARCEL],
  seeded: false,
  updated_at: "2026-11-06T11:30:00Z",
};

const SHOPPER_WORDS = {
  belief_id: "blf_77c310",
  kind: "fit",
  subject: { kind: "brand", id: BRAND, name: "Brand X" },
  value: "shopper says they are usually size M",
  status: "Claimed",
  standing: "None",
  contradicted: false,
  contradicted_by: null,
  evidence: [],
  seeded: true,
  updated_at: "2026-10-14T10:00:00Z",
};

export function describeBelief(beliefId: string) {
  if (beliefId !== FIT_BELIEF) {
    return null;
  }
  return {
    belief: FIT_RECORD,
    corrections: [SHOPPER_NOTE],
    timeline: [
      { at: "2026-10-14T10:00:00Z", kind: "evidence", summary: "Fit return 1: oxford shirt size M refunded; too tight in the shoulders.", ref_id: "ev_6d1a09" },
      { at: "2026-10-20T10:00:00Z", kind: "evidence", summary: "Fit return 2: oxford shirt size M refunded; too tight across the chest.", ref_id: "ev_6d1a10" },
      { at: "2026-10-26T10:00:00Z", kind: "evidence", summary: "Fit return 3: oxford shirt size M refunded; sleeves too short in M.", ref_id: "ev_6d1a11" },
      { at: "2026-10-26T10:00:05Z", kind: "status", summary: "Belief reached Earned: three documented fit returns over twelve days.", ref_id: FIT_BELIEF },
      { at: "2026-11-05T14:02:11Z", kind: "correction", summary: "Shopper correction: lost weight, take size M for Brand X.", ref_id: "corr_3b7e52" },
      { at: "2026-11-08T20:10:00Z", kind: "feedback", summary: "Fit feedback: oxford shirt size M fits well.", ref_id: "fb_51c2d8" },
    ],
  };
}

export function ledgerRows(shopperId: string) {
  void shopperId;
  return {
    shopper_id: shopperId,
    beliefs: [
      { belief: FIT_RECORD, evidence_count: 3, last_change_at: "2026-11-05T14:02:11Z" },
      { belief: FIT_ROWS_NOW, evidence_count: 1, last_change_at: "2026-11-08T20:10:00Z" },
      { belief: SELLER_ROWS, evidence_count: 1, last_change_at: "2026-11-06T11:30:00Z" },
      { belief: SHOPPER_WORDS, evidence_count: 0, last_change_at: "2026-10-14T10:00:00Z" },
    ],
  };
}

export function ledgerRow(beliefId: string) {
  const rows = ledgerRows(SHOPPER).beliefs;
  const row = rows.find((r) => r.belief.belief_id === beliefId);
  if (!row) {
    return null;
  }
  return {
    belief: row.belief,
    corrections: row.belief.belief_id === FIT_BELIEF ? [SHOPPER_NOTE] : [],
    seeded_notice:
      row.belief.belief_id === FIT_BELIEF
        ? "Seeded history: generated, not lived. The three returns are seeded; the correction is live."
        : undefined,
  };
}

export function ledgerProof(shopperId: string) {
  void shopperId;
  return {
    shopper_id: shopperId,
    evidence: [
      { evidence: GOOD_FEEDBACK_ROW, belief_ids: ["blf_2b9d60"] },
      { evidence: LOST_PARCEL, belief_ids: ["blf_1a5e09"] },
      { evidence: THIRD, belief_ids: [FIT_BELIEF] },
      { evidence: SECOND, belief_ids: [FIT_BELIEF] },
      { evidence: FIRST, belief_ids: [FIT_BELIEF] },
    ],
  };
}
