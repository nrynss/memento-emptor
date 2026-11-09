/**
 * Offline check: the agent answers a shopping request against the fakes
 * before anything is deployed. No model, no network, no keys: it exercises
 * the fake-backed tools the deployed agent will call and asserts the
 * recall-first contract (correction wins, belief stays, vague answers ask
 * again). Run: bun agent/offline-check.ts
 */
import {
  explainBelief,
  ledgerBeliefs,
  lookupReturn,
  pendingQuestions,
  recallForPurchase,
  recordCorrection,
  recordFeedback,
  recordReason,
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

if (failures > 0) {
  console.log(`${failures} check(s) failed`);
  process.exit(1);
}
console.log("offline check passed: shopping request answered against the fakes, recall first");
