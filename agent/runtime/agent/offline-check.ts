/**
 * Offline check: the agent answers a shopping request against the fakes
 * before anything is deployed. No model, no network, no keys: it exercises
 * the fake-backed tools the deployed agent will call and asserts the
 * recall-first contract (correction wins, belief stays, vague answers ask
 * again) plus one block per precedence step (correction, own evidence,
 * pool, statement, none) and the standalone correction that stands on its
 * own when recall served no belief. Run: bun agent/offline-check.ts
 */
import {
  explainBelief,
  ledgerBeliefs,
  lookupReturn,
  pendingQuestions,
  productFit,
  recallForPurchase,
  recordCorrection,
  recordFeedback,
  recordReason,
  setPooling,
} from "./memory.js";

let failures = 0;

function check(name: string, cond: boolean, detail?: string) {
  if (cond) {
    console.log(`ok: ${name}`);
  } else {
    failures += 1;
    console.log(`FAIL: ${name}${detail ? ` (${detail})` : ""}`);
  }
}

// Turn 1: a shopping request answered against the fakes. Recall first.
const recall = recallForPurchase("shopper_01", "trace_3f8a01") as {
  beliefs: Array<{
    belief_id: string;
    status: string;
    standing: string;
    contradicted: boolean;
    evidence: unknown[];
  }>;
  overriding_correction: { text: string } | null;
  guidance: { text: string; basis: string };
  pending_count: number;
};
check("recall returns the sizing belief", recall.beliefs.length === 1);
check("recall belief is Out of true, standing Venerable", recall.beliefs[0].status === "Out of true" && recall.beliefs[0].standing === "Venerable");
check("recall belief stays contradicted, not deleted", recall.beliefs[0].contradicted === true);
check("recall cites 3 fit returns", recall.beliefs[0].evidence.length === 3);
check("correction wins the recommendation", recall.guidance.text === "Take size M for Brand X" && recall.guidance.basis === "correction");
check("overriding correction present", recall.overriding_correction?.text === "I've lost weight, use M");
check("pending refund surfaces", recall.pending_count === 1);

// Unknown shoppers are isolated: no memory leaks across sessions.
const stranger = recallForPurchase("shopper_99") as { beliefs: unknown[]; pending_count: number };
check("unknown shopper gets no beliefs", stranger.beliefs.length === 0 && stranger.pending_count === 0);

// Correction applies at once with precedence.
const corr = recordCorrection("shopper_01", "I've lost weight, use M", "blf_8f2c41", "trace_3f8a02") as {
  correction: { takes_precedence: boolean };
  status: string;
  guidance: { basis: string };
};
check("correction takes precedence", corr.correction.takes_precedence === true);
check("belief stays Out of true beside the correction", corr.status === "Out of true" && corr.guidance.basis === "correction");

// Pending question, then the vague answer asks again (frozen ask-again shape).
const pending = pendingQuestions("shopper_01") as { questions: Array<{ question_id: string }> };
check("one pending question", pending.questions.length === 1 && pending.questions[0].question_id === "q_9f04ab");
const vague = recordReason("q_9f04ab", "not great", "trace_3f8a03") as { ask_again: boolean; accepted: boolean };
check("vague answer asks again", vague.ask_again === true && vague.accepted === false);
const clear = recordReason("q_9f04ab", "Too tight in the shoulders, couldn't button it.", "trace_3f8a03") as {
  accepted: boolean;
  category: string;
};
check("clear fit answer counts toward fit", clear.accepted === true && clear.category === "fit_too_small");

// Feedback ties to the purchase.
const fb = recordFeedback("memento-shopper01-06", "PSW8GeK", "M", "The M fits well across the shoulders.", "trace_3f8a04") as {
  verdict: string;
  accepted: boolean;
};
check("fits-well feedback accepted", fb.verdict === "fits_well" && fb.accepted === true);

// Explanation carries the whole history for citations.
const expl = explainBelief("blf_8f2c41") as { timeline: unknown[]; corrections: unknown[] } | null;
check("explanation has 6 timeline steps and the correction", expl !== null && expl.timeline.length === 6 && expl.corrections.length === 1);
check("unknown belief explains nothing", explainBelief("blf_nope") === null);

// Ledger reads serve the four rows.
const rows = ledgerBeliefs("shopper_01") as { beliefs: Array<{ evidence_count: number }> };
check("ledger lists 4 rows, strongest first", rows.beliefs.length === 4 && rows.beliefs[0].evidence_count === 3);

// PayPal-flavoured read is recall-shaped, never a purchase.
const ret = lookupReturn("ORD-7XK-01") as { outcome: string; reason: string };
check("lookup_return reads the recorded return", ret.outcome === "refunded" && ret.reason === "too tight in the shoulders");

// Correction step: the demo shopper's own belief and the pool both ride
// along overridden by the correction that decides.
const demo = recallForPurchase("shopper_01", "trace_3f8a01") as {
  beliefs: Array<{ precedence: { step: string; overridden: boolean; overridden_by: string | null } }>;
  pool: { status: string; precedence: { step: string; overridden: boolean; overridden_by: string | null } };
  pool_lookup: string;
};
check("correction step marks own evidence overridden", demo.beliefs[0].precedence.step === "own_evidence" && demo.beliefs[0].precedence.overridden === true && demo.beliefs[0].precedence.overridden_by === "correction");
check("correction step marks the pool overridden", demo.pool.precedence.step === "pool" && demo.pool.precedence.overridden === true && demo.pool.precedence.overridden_by === "correction");
check("correction step consulted the pool", demo.pool_lookup === "consulted" && demo.pool.status === "Earned");

// Pool step: a new shopper on the Earned-small line is sized up from the
// usual size, and the Claimed statement rides along overridden.
const pooled = recallForPurchase("shopper_02") as {
  beliefs: Array<{ precedence: { step: string; overridden: boolean; overridden_by: string | null }; status: string }>;
  pool: { status: string; direction: string; counts: { settled: number }; seeded: { settled: number }; precedence: { step: string; overridden: boolean } };
  guidance: { text: string; basis: string };
};
check("pool step decides from the pool", pooled.guidance.basis === "pool" && pooled.pool.status === "Earned" && pooled.pool.direction === "small");
check("pool guidance cites counts", pooled.guidance.text.includes("29 of the 47") && pooled.guidance.text.includes("too small"));
check("pool step shows the seeded split", pooled.pool.seeded.settled === 48 && pooled.pool.counts.settled === 60);
check("pool step marks the statement overridden", pooled.beliefs[0].status === "Claimed" && pooled.beliefs[0].precedence.step === "statement" && pooled.beliefs[0].precedence.overridden === true);
check("pool step pool rides along", pooled.pool.precedence.step === "pool" && pooled.pool.precedence.overridden === false);

// Own-evidence step: the shopper unlike the crowd keeps their size, and
// the disagreeing pool rides along overridden.
const own = recallForPurchase("shopper_04") as {
  guidance: { text: string; basis: string };
  pool: { precedence: { step: string; overridden: boolean; overridden_by: string | null } };
};
check("own-evidence step beats the pool", own.guidance.basis === "own_evidence" && own.guidance.text.includes("ahead of the pool"));
check("own-evidence step marks the pool overridden", own.pool.precedence.step === "pool" && own.pool.precedence.overridden === true && own.pool.precedence.overridden_by === "own_evidence");

// Statement step: the usual size decides, labelled Claimed, never as
// evidence.
const stmt = recallForPurchase("shopper_06") as {
  guidance: { text: string; basis: string };
  beliefs: Array<{ status: string; precedence: { step: string; overridden: boolean } }>;
};
check("statement step decides from the usual size", stmt.guidance.basis === "statement" && stmt.guidance.text.includes("usual size"));
check("statement step stays Claimed", stmt.beliefs[0].status === "Claimed" && stmt.beliefs[0].precedence.overridden === false);

// None step: the layer never refuses, it says plainly no memory applies.
const none = recallForPurchase("shopper_99") as {
  beliefs: unknown[];
  guidance: { text: string; basis: string };
  pool_lookup: string;
};
check("none step says so plainly, never refuses", none.beliefs.length === 0 && none.guidance.basis === "none" && none.guidance.text.includes("No memory applies yet"));

// Standalone correction: recall served no belief, so the correction is
// recorded with no belief id, stands Claimed, and stays in force.
const stand = recordCorrection("shopper_99", "Actually use size M for me") as {
  correction: { correction_id: string };
  status: string;
  standing: string;
  contradicted: boolean;
  guidance: { text: string; basis: string; belief_id: string | null };
};
check("standalone correction stands Claimed", stand.status === "Claimed" && stand.standing === "None" && stand.contradicted === false);
check("standalone correction decides the next recommendation", stand.guidance.basis === "correction" && stand.guidance.text === "Take size M" && stand.guidance.belief_id === null);
const again = recallForPurchase("shopper_99") as {
  overriding_correction: { correction_id: string } | null;
  guidance: { text: string; basis: string };
};
check("standalone correction stays in force", again.overriding_correction?.correction_id === stand.correction.correction_id && again.guidance.basis === "correction" && again.guidance.text === "Take size M for Brand X");

// Pool and pooling tools serve counts, never shopper rows.
const fit = productFit("brand-x-001", "shirts", "regular", "M") as {
  pool: { status: string; counts: { shoppers: { fit: number } } };
};
check("product_fit serves the Earned pool", fit.pool.status === "Earned" && fit.pool.counts.shoppers.fit === 39);
const empty = productFit("brand-x-001", "shirts", "regular", "XL") as {
  pool: { status: string; counts: { settled: number } };
};
check("product_fit on an empty line says Claimed", empty.pool.status === "Claimed" && empty.pool.counts.settled === 0);
const optout = setPooling("shopper_02", false) as { pooling: boolean; rows_removed: number };
check("opting out removes rows", optout.pooling === false && optout.rows_removed === 1);
const optin = setPooling("shopper_02", true) as { pooling: boolean; rows_removed: number };
check("opting back in removes nothing", optin.pooling === true && optin.rows_removed === 0);

if (failures > 0) {
  console.log(`${failures} check(s) failed`);
  process.exit(1);
}
console.log("offline check passed: shopping request answered against the fakes, recall first");
