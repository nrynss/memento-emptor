/**
 * Offline check for the second agent: the stand-in layer answers the
 * demo-shopper story the way the live layer will, so the deployed agent's
 * first recommendation already honours the shopper_01 correction. No
 * model, no network, no keys. Run: bun agent/check.ts
 */
import {
  answerReason,
  describeBelief,
  ledgerProof,
  ledgerRow,
  ledgerRows,
  noteCorrection,
  noteFeedback,
  openQuestions,
  pooledFit,
  poolingChoice,
  recall,
} from "./store.js";

let failures = 0;

function check(name: string, cond: boolean, detail?: string) {
  if (cond) {
    console.log(`ok: ${name}`);
  } else {
    failures += 1;
    console.log(`FAIL: ${name}${detail ? ` (${detail})` : ""}`);
  }
}

// The heart of T9.5: recall for the demo shopper serves the correction
// with precedence, and the guidance obeys it while the L belief stands.
const first = recall("shopper_01", "trace_2nd_01") as {
  beliefs: Array<{ belief_id: string; status: string; standing: string; contradicted: boolean; evidence: unknown[]; precedence: { step: string; overridden: boolean; overridden_by: string | null } }>;
  pool: { status: string; precedence: { step: string; overridden: boolean; overridden_by: string | null } };
  pool_lookup: string;
  overriding_correction: { correction_id: string; text: string } | null;
  guidance: { text: string; basis: string };
  pending_count: number;
};
check("demo recall serves the sizing belief", first.beliefs.length === 1 && first.beliefs[0].belief_id === "blf_8f2c41");
check("belief keeps standing while contradicted", first.beliefs[0].status === "Out of true" && first.beliefs[0].standing === "Venerable" && first.beliefs[0].contradicted === true);
check("belief rests on three fit returns", first.beliefs[0].evidence.length === 3);
check("guidance obeys the correction", first.guidance.text === "Take size M for Brand X" && first.guidance.basis === "correction");
check("overriding note is the shopper's own words", first.overriding_correction?.correction_id === "corr_3b7e52" && first.overriding_correction?.text === "I've lost weight, use M");
check("own record and pool both bow to the note", first.beliefs[0].precedence.overridden_by === "correction" && first.pool.precedence.overridden_by === "correction");
check("pool stays consulted and Earned", first.pool_lookup === "consulted" && first.pool.status === "Earned");
check("one refund still wants its reason", first.pending_count === 1);

// A live correction against the served belief applies at once.
const filed = noteCorrection("shopper_01", "I've lost weight, use M", "blf_8f2c41", "trace_2nd_02") as {
  correction: { takes_precedence: boolean };
  status: string;
  guidance: { basis: string };
};
check("filed correction takes precedence", filed.correction.takes_precedence === true);
check("belief reads Out of true beside it", filed.status === "Out of true" && filed.guidance.basis === "correction");

// A note filed where recall served nothing stands alone and keeps deciding.
const lone = noteCorrection("shopper_77", "Remember L for me") as {
  correction: { correction_id: string };
  status: string;
  standing: string;
  contradicted: boolean;
  guidance: { text: string; basis: string; belief_id: string | null };
};
check("lone note stands Claimed", lone.status === "Claimed" && lone.standing === "None" && lone.contradicted === false);
check("lone note settles the next suggestion", lone.guidance.basis === "correction" && lone.guidance.belief_id === null);
const revisit = recall("shopper_77") as {
  overriding_correction: { correction_id: string } | null;
  guidance: { text: string; basis: string };
};
check("lone note holds on revisit", revisit.overriding_correction?.correction_id === lone.correction.correction_id && revisit.guidance.basis === "correction");

// The new shopper on the Earned-small line is sized up from the pool.
const pooled = recall("shopper_02") as {
  guidance: { text: string; basis: string };
  pool: { status: string; direction: string; counts: { settled: number }; seeded: { settled: number } };
};
check("pool settles the newcomer", pooled.guidance.basis === "pool" && pooled.pool.status === "Earned" && pooled.pool.direction === "small");
check("pool answer keeps the seeded split", pooled.pool.seeded.settled === 48 && pooled.pool.counts.settled === 60);

// Strangers hear honesty, never a guess.
const stranger = recall("shopper_99") as { beliefs: unknown[]; guidance: { basis: string; text: string } };
check("strangers get no beliefs and plain words", stranger.beliefs.length === 0 && stranger.guidance.basis === "none");

// Vague answers keep the question open; clear fit answers count.
const vague = answerReason("q_9f04ab", "it was fine I guess") as { ask_again: boolean; accepted: boolean };
check("vague answer asks again", vague.ask_again === true && vague.accepted === false);
const plain = answerReason("q_9f04ab", "Far too tight across the shoulders") as { accepted: boolean; category: string };
check("plain fit answer counts toward fit", plain.accepted === true && plain.category === "fit_too_small");

// Feedback, questions, explanation and ledger reads behave.
const fb = noteFeedback("memento-shopper01-06", "PSW8GeK", "M", "The M fits well across the shoulders.") as { verdict: string; accepted: boolean };
check("fits-well remark accepted", fb.verdict === "fits_well" && fb.accepted === true);
const asks = openQuestions("shopper_01") as { questions: Array<{ question_id: string }> };
check("one open question for the demo shopper", asks.questions.length === 1 && asks.questions[0].question_id === "q_9f04ab");
const why = describeBelief("blf_8f2c41") as { timeline: unknown[]; corrections: unknown[] } | null;
check("explanation carries history and the note", why !== null && why.timeline.length === 6 && why.corrections.length === 1);
check("unknown belief explains nothing", describeBelief("blf_nope") === null);
const rows = ledgerRows("shopper_01") as { beliefs: Array<{ evidence_count: number }> };
check("ledger serves four rows, strongest first", rows.beliefs.length === 4 && rows.beliefs[0].evidence_count === 3);
const one = ledgerRow("blf_8f2c41") as { corrections: unknown[] } | null;
check("ledger row carries the note", one !== null && one.corrections.length === 1);
const proof = ledgerProof("shopper_01") as { evidence: unknown[] };
check("ledger proof serves five rows", proof.evidence.length === 5);
const fit = pooledFit("brand-x-001", "shirts", "regular", "M") as { pool: { status: string } };
check("pooled read is Earned on the demo line", fit.pool.status === "Earned");
const bare = pooledFit("brand-x-001", "shirts", "regular", "XL") as { pool: { status: string } };
check("empty line reads Claimed", bare.pool.status === "Claimed");
const out = poolingChoice("shopper_02", false) as { pooling: boolean; rows_removed: number };
check("opting out removes a row", out.pooling === false && out.rows_removed === 1);
const back = poolingChoice("shopper_02", true) as { rows_removed: number };
check("opting back in removes nothing", back.rows_removed === 0);

if (failures > 0) {
  console.log(`${failures} check(s) failed`);
  process.exit(1);
}
console.log("offline check passed: second agent answers from the same layer state");
