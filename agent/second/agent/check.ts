/**
 * Offline check for the second agent: its own tool code against the layer's
 * frozen contract examples, with the edge connection injected. No model, no
 * keys, no external network.
 *
 * The point is unchanged from T9.5 — its first recommendation follows a
 * correction it never saw spoken, because the layer carries it — and it now
 * runs over the same code the deployment dials with (agent/edge.ts). Two
 * checks guard the freshness that proof rests on: the deployment's
 * instructions carry no correction story and no belief id, and the tool
 * layer is the eleven frozen calls with nothing standing behind them.
 *
 * Run: bun agent/check.ts
 */
import { join } from "node:path";
import { bindLanding, connectEdge, EdgeRefusal, type Edge, type Wire } from "./edge.js";

const API_DIR = join(import.meta.dir, "../../../testdata/api");

let failures = 0;

function check(name: string, cond: boolean, detail?: string) {
  if (cond) {
    console.log(`ok: ${name}`);
  } else {
    failures += 1;
    console.log(`FAIL: ${name}${detail ? ` (${detail})` : ""}`);
  }
}

type JSON_ = Record<string, any>;

/** example reads one of the layer's frozen contract examples. */
async function example(name: string): Promise<JSON_> {
  return (await Bun.file(join(API_DIR, name)).json()) as JSON_;
}

/** One call the fake edge saw. */
interface Seen {
  tool: string;
  field: Wire;
}

/**
 * standInEdge answers with the contract's examples: the demo shopper's
 * correction story, the pool rung for a newcomer, and no memory for anyone
 * else. A standing correction it records is served back on that shopper's
 * next recall, which is what the layer does.
 */
async function standInEdge(): Promise<{ edge: Edge; seen: Seen[] }> {
  const correction = await example("recall_for_purchase.response.example-correction.json");
  const pool = await example("recall_for_purchase.response.example-pool.json");
  const nobody = await example("recall_for_purchase.response.example-none.json");
  const declared = await example("record_correction.response.example.json");
  const standing = await example("record_correction.response.example-standing.json");
  const fit = await example("product_fit.response.example.json");
  const fitClaimed = await example("product_fit.response.example-claimed.json");
  const questions = await example("pending_questions.response.example.json");
  const reason = await example("record_reason.response.example.json");
  const unclear = await example("record_reason.response.example-unclear.json");
  const feedback = await example("record_feedback.response.example.json");
  const explain = await example("explain_belief.response.example.json");
  const ledgerAll = await example("ledger_beliefs.response.example.json");
  const ledgerOne = await example("ledger_belief.response.example.json");
  const evidence = await example("ledger_evidence.response.example.json");
  const pooling = await example("set_pooling.response.example.json");

  const copy = (value: JSON_): JSON_ => JSON.parse(JSON.stringify(value));
  const held = new Map<string, JSON_>();
  const seen: Seen[] = [];
  const answer = (tool: string, field: Wire): JSON_ => {
    switch (tool) {
      case "recall_for_purchase": {
        const shopper = String(field.shopper_id);
        const note = held.get(shopper);
        if (note) {
          const out = copy(nobody);
          out.shopper_id = shopper;
          out.overriding_correction = note.correction;
          out.guidance = note.guidance;
          return out;
        }
        if (shopper === "shopper_01") return copy(correction);
        if (shopper === "shopper_02") return copy(pool);
        const out = copy(nobody);
        out.shopper_id = shopper;
        return out;
      }
      case "record_correction":
        if (typeof field.contradicts_belief_id === "string" && field.contradicts_belief_id !== "") {
          return copy(declared);
        } else {
          const note = copy(standing);
          held.set(String(field.shopper_id), note);
          return note;
        }
      case "product_fit":
        return copy(field.size === "M" ? fit : fitClaimed);
      case "pending_questions":
        return copy(questions);
      case "record_reason":
        return copy(/tight|small|short|sleeve|button|loose|large|big/i.test(String(field.answer_text)) ? reason : unclear);
      case "record_feedback":
        return copy(feedback);
      case "explain_belief":
        return copy(explain);
      case "ledger_beliefs":
        return copy(ledgerAll);
      case "ledger_belief":
        return copy(ledgerOne);
      case "ledger_evidence":
        return copy(evidence);
      case "set_pooling":
        return copy(pooling);
      default:
        throw new Error(`stand-in edge: unknown tool ${tool}`);
    }
  };
  return {
    seen,
    edge: {
      async call(tool, field) {
        seen.push({ tool, field });
        const out = answer(tool, field);
        out.trace_id = typeof field.trace_id === "string" ? field.trace_id : null;
        return out;
      },
    },
  };
}

const { edge, seen } = await standInEdge();
const landing = bindLanding(edge);

// The heart of T9.5: the first thing this agent does is ask the layer, and
// the answer carries a correction the deployment itself never held.
const first = await landing.recall({
  shopper_id: "shopper_01",
  brand_id: "brand-x-001",
  category: "shirts",
  fit_handle: "regular",
  size_considered: "M",
  trace_id: "trace_2nd_01",
});
check("the first call is recall_for_purchase", seen.length === 1 && seen[0].tool === "recall_for_purchase");
check("recall sends the shopper and the brand", seen[0].field.shopper_id === "shopper_01" && seen[0].field.brand_id === "brand-x-001");
const firstBeliefs = first.beliefs as JSON_[];
check("the demo recall serves the sizing belief", firstBeliefs.length === 1 && firstBeliefs[0].belief_id === "blf_8f2c41");
check(
  "the belief keeps standing while contradicted",
  firstBeliefs[0].status === "Out of true" && firstBeliefs[0].standing === "Venerable" && firstBeliefs[0].contradicted === true,
);
check("the belief rests on three fit returns", (firstBeliefs[0].evidence as unknown[]).length === 3);
check(
  "guidance obeys the correction",
  (first.guidance as JSON_).text === "Take size M for Brand X" && (first.guidance as JSON_).basis === "correction",
);
check(
  "the note is the shopper's own words",
  (first.overriding_correction as JSON_).correction_id === "corr_3b7e52",
);
check(
  "own record and pool both bow to the note",
  firstBeliefs[0].precedence.overridden_by === "correction" && (first.pool as JSON_).precedence.overridden_by === "correction",
);
check("the pool stays consulted and Earned", first.pool_lookup === "consulted" && (first.pool as JSON_).status === "Earned");
check(
  "the seeded split travels with the counts",
  (first.pool as JSON_).counts.settled === (first.pool as JSON_).seeded.settled + (first.pool as JSON_).live.settled,
);
check("one refund still wants its reason", first.pending_count === 1);

// A correction filed against the served belief applies at once.
const filed = await landing.correct({
  shopper_id: "shopper_01",
  text: "I've lost weight, use M",
  contradicts_belief_id: "blf_8f2c41",
  trace_id: "trace_2nd_02",
});
check("a filed correction takes precedence", (filed.correction as JSON_).takes_precedence === true);
check(
  "the belief reads Out of true beside it",
  filed.status === "Out of true" && (filed.guidance as JSON_).basis === "correction",
);

// A note filed where recall served nothing stands alone and keeps deciding.
const lone = await landing.correct({ shopper_id: "shopper_77", text: "Remember L for me" });
check("a lone note stands Claimed", lone.status === "Claimed" && lone.standing === "None" && lone.contradicted === false);
const revisit = await landing.recall({ shopper_id: "shopper_77", brand_id: "brand-x-001" });
check(
  "the lone note holds on a revisit",
  (revisit.overriding_correction as JSON_).correction_id === (lone.correction as JSON_).correction_id &&
    (revisit.guidance as JSON_).basis === "correction",
);

// A newcomer on the Earned-small line is sized up from the pool.
const pooled = await landing.recall({ shopper_id: "shopper_02", brand_id: "brand-x-001" });
check(
  "the pool settles a newcomer",
  (pooled.guidance as JSON_).basis === "pool" && (pooled.pool as JSON_).status === "Earned" && (pooled.pool as JSON_).direction === "small",
);
check("the pool answer keeps its counts", String((pooled.guidance as JSON_).text).includes("29 of the 47"));

// Strangers hear honesty, never a guess.
const stranger = await landing.recall({ shopper_id: "shopper_99", brand_id: "brand-x-001" });
check(
  "strangers get no beliefs and plain words",
  (stranger.beliefs as unknown[]).length === 0 && (stranger.guidance as JSON_).basis === "none",
);

// Answers, feedback, questions, explanation and the ledger reads behave.
const vague = await landing.reason({ shopper_id: "shopper_01", question_id: "q_9f04ab", answer_text: "it was fine I guess" });
check("a vague answer asks again", vague.ask_again === true && vague.accepted === false);
const plain = await landing.reason({
  shopper_id: "shopper_01",
  question_id: "q_9f04ab",
  answer_text: "Far too tight across the shoulders",
});
check("a plain fit answer counts toward fit", plain.accepted === true && plain.category === "fit_too_small");
const fb = await landing.feedback({
  shopper_id: "shopper_01",
  custom_id: "memento-shopper01-06",
  sku: "PSW8GeK",
  size: "M",
  feedback_text: "The M fits well across the shoulders.",
});
check("a fits-well remark is accepted", fb.verdict === "fits_well" && fb.accepted === true);
const asks = await landing.questions({ shopper_id: "shopper_01" });
check("one question is open for the demo shopper", (asks.questions as unknown[]).length === 1);
const why = await landing.explain({ shopper_id: "shopper_01", belief_id: "blf_8f2c41" });
check("the explanation carries history and the note", (why.timeline as unknown[]).length > 0 && (why.corrections as unknown[]).length === 1);
const rows = await landing.ledger({ shopper_id: "shopper_01" });
check("the ledger serves rows", (rows.beliefs as unknown[]).length > 0);
const one = await landing.ledgerRow({ shopper_id: "shopper_01", belief_id: "blf_8f2c41" });
check("one ledger row reads in full", (one.belief as JSON_).belief_id === "blf_8f2c41");
const proof = await landing.ledgerProof({ shopper_id: "shopper_01" });
check("the ledger proof serves rows", (proof.evidence as unknown[]).length > 0);
const fit = await landing.fit({ brand_id: "brand-x-001", category: "shirts", fit_handle: "regular", size: "M" });
check("the pooled read is Earned on the demo line", (fit.pool as JSON_).status === "Earned");
const bare = await landing.fit({ brand_id: "brand-x-001", category: "shirts", fit_handle: "regular", size: "XL" });
check("an empty line reads Claimed", (bare.pool as JSON_).status === "Claimed");
const out = await landing.pooling({ shopper_id: "shopper_02", pooling: false });
check("opting out reports the rows removed", out.pooling === false && out.rows_removed === 3);

// The tool layer is the eleven frozen calls, all through the edge.
check("the landing holds the eleven frozen tools", Object.keys(landing).length === 11);
for (const tool of [
  "recall_for_purchase",
  "record_correction",
  "pending_questions",
  "record_reason",
  "record_feedback",
  "explain_belief",
  "product_fit",
  "set_pooling",
  "ledger_beliefs",
  "ledger_belief",
  "ledger_evidence",
]) {
  check(`every frozen tool reached the edge: ${tool}`, seen.some((s) => s.tool === tool));
}

// The prompt carries no correction story and no belief id: whatever this
// agent answers about the shopper's size came from the layer.
const prompt = await Bun.file(join(import.meta.dir, "index.ts")).text();
check("the instructions carry no correction story", !/lost weight|corr_[0-9a-z]/i.test(prompt));
check("the instructions carry no belief id", !/blf_[0-9a-z]/i.test(prompt.split("const INSTRUCTIONS")[1] ?? ""));

// The HTTP path: the request this deployment sends, against a local stub.
interface Caught {
  method: string;
  path: string;
  authorization: string;
  body: JSON_;
}
const caught: Caught[] = [];
const stub = Bun.serve({
  port: 0,
  hostname: "127.0.0.1",
  async fetch(request) {
    const url = new URL(request.url);
    const body = (await request.json().catch(() => ({}))) as JSON_;
    caught.push({
      method: request.method,
      path: url.pathname,
      authorization: request.headers.get("authorization") ?? "",
      body,
    });
    if (url.pathname === "/tools/ledger_belief") {
      return new Response(JSON.stringify({ error: { code: "unauthenticated", message: "unauthorized" } }), {
        status: 401,
        headers: { "content-type": "application/json" },
      });
    }
    return new Response(JSON.stringify(await example("recall_for_purchase.response.example-correction.json")), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  },
});
const dialled = bindLanding(connectEdge({ host: stub.url.toString(), token: "offline-check-key" }));
const answer = await dialled.recall({
  shopper_id: "shopper_01",
  brand_id: "brand-x-001",
  category: "shirts",
  fit_handle: "regular",
  size_considered: "M",
});
check("the HTTP path answers with the layer's response", (answer.guidance as JSON_).basis === "correction");
const sent = caught[caught.length - 1];
check("the call posts to the tool's edge path", sent.method === "POST" && sent.path === "/tools/recall_for_purchase");
check("the key travels as a bearer token", sent.authorization === "Bearer offline-check-key");
check("the body carries the shopper and the brand", sent.body.shopper_id === "shopper_01" && sent.body.brand_id === "brand-x-001");
let refused: unknown;
try {
  await dialled.ledgerRow({ shopper_id: "shopper_01", belief_id: "blf_8f2c41" });
} catch (cause) {
  refused = cause;
}
check(
  "a refusal surfaces the edge's own code",
  refused instanceof EdgeRefusal && refused.status === 401 && refused.code === "unauthenticated",
);
let keyless: unknown;
try {
  await bindLanding(connectEdge({ host: stub.url.toString() })).fit({
    brand_id: "brand-x-001",
    category: "shirts",
    fit_handle: "regular",
    size: "M",
  });
} catch (cause) {
  keyless = cause;
}
check("a deployment without a key fails loudly", keyless instanceof EdgeRefusal && keyless.code === "unconfigured");
stub.stop(true);

if (failures > 0) {
  console.log(`${failures} check(s) failed`);
  process.exit(1);
}
console.log("offline check passed: second agent answers from the served layer's edge");
