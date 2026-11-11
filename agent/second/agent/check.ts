/**
 * Offline check for the second agent: its own tool code against the layer's
 * frozen contract examples, with the edge connection injected. No model, no
 * keys, no external network.
 *
 * The point is unchanged from T9.5 — its first recommendation follows a
 * correction it never saw spoken, because the layer carries it — and it now
 * runs over the same code the deployment dials with (agent/edge.ts). Three
 * things guard the freshness that proof rests on: the deployment's
 * instructions carry no correction story and no belief id; the tools offered
 * are exactly the eight calls the edge's access table admits to an agent key
 * (the shopper-only ledger reads are absent), each binding checked against
 * the contract's own request schema and example; and the retry rule lets a
 * read be tried again while a write must be looked up before it is resent.
 *
 * Run: bun agent/check.ts
 */
import { join } from "node:path";
import { bindLanding, connectEdge, EdgeRefusal, type Edge, type Wire } from "./edge.js";
import { createTools, tools } from "./tools.js";

const API_DIR = join(import.meta.dir, "../../../testdata/api");
const REPO_DIR = join(import.meta.dir, "../../..");

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
 * next recall, which is what the layer does. A tool outside the agent's
 * eight is an error: this deployment offers no such call.
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

// Answers, feedback, questions, explanation and the pooled read behave.
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
const fit = await landing.fit({ brand_id: "brand-x-001", category: "shirts", fit_handle: "regular", size: "M" });
check("the pooled read is Earned on the demo line", (fit.pool as JSON_).status === "Earned");
const bare = await landing.fit({ brand_id: "brand-x-001", category: "shirts", fit_handle: "regular", size: "XL" });
check("an empty line reads Claimed", (bare.pool as JSON_).status === "Claimed");
const out = await landing.pooling({ shopper_id: "shopper_02", pooling: false });
check("opting out reports the rows removed", out.pooling === false && out.rows_removed === 3);

// --- The tool surface: what the edge admits to an agent key ---
//
// The table is read from the layer's own source, so it cannot drift away from
// the tools this deployment offers without this check going red.

/** contractNames maps every frozen tool constant to its frozen name. */
async function contractNames(): Promise<Map<string, string>> {
  const source = await Bun.file(join(REPO_DIR, "internal/api/contract/tools.go")).text();
  const names = new Map<string, string>();
  for (const match of source.matchAll(/^\t(Tool\w+)\s+ToolName\s*=\s*"([a-z_]+)"/gm)) {
    names.set(match[1], match[2]);
  }
  return names;
}

/** agentAdmitted names the tools the edge's access table admits to an agent key. */
async function agentAdmitted(): Promise<string[]> {
  const names = await contractNames();
  const source = await Bun.file(join(REPO_DIR, "internal/auth/surface.go")).text();
  const table = source.slice(source.indexOf("var toolAccess = map[contract.ToolName]ToolAccess{"));
  const admitted: string[] = [];
  let entries = 0;
  for (const match of table.matchAll(/^\tcontract\.(Tool\w+):\s*\{Callers:\s*\[\]CallerKind\{([^}]*)\}/gm)) {
    entries += 1;
    if (!/\bCallerAgent\b/.test(match[2])) {
      continue;
    }
    const name = names.get(match[1]);
    if (name === undefined) {
      throw new Error(`the access table names ${match[1]}, which the contract does not define`);
    }
    admitted.push(name);
  }
  if (entries !== names.size) {
    throw new Error(`the access table parsed ${entries} entries against ${names.size} frozen names`);
  }
  return admitted.sort();
}

const sameList = (a: string[], b: string[]) => a.length === b.length && a.every((value, i) => value === b[i]);

let admitted: string[] = [];
try {
  admitted = await agentAdmitted();
} catch (cause) {
  check("the edge's access table parses", false, cause instanceof Error ? cause.message : String(cause));
}
check("the access table admits the eight agent calls", sameList(admitted.slice(), [
  "explain_belief",
  "pending_questions",
  "product_fit",
  "recall_for_purchase",
  "record_correction",
  "record_feedback",
  "record_reason",
  "set_pooling",
]));
check(
  "the deployment offers exactly the tools an agent key reaches",
  sameList(Object.keys(tools).sort(), admitted),
  `offers ${JSON.stringify(Object.keys(tools).sort())}`,
);
check(
  "the landing holds one method per admitted tool",
  Object.keys(landing).length === admitted.length,
  `landing has ${Object.keys(landing).length}`,
);
for (const name of admitted) {
  check(`every admitted call reached the edge: ${name}`, seen.some((entry) => entry.tool === name));
}
for (const shopperOnly of ["ledger_beliefs", "ledger_belief", "ledger_evidence"]) {
  check(
    `${shopperOnly} is not offered: the edge admits shoppers only`,
    !(shopperOnly in tools) && admitted.every((name) => name !== shopperOnly),
  );
}

// Each binding, against the contract's own request schema and example.
type ZodField = {
  isOptional(): boolean;
  _def?: { typeName?: string; innerType?: ZodField };
};
type ToolLike = {
  id?: string;
  description?: string;
  inputSchema?: { shape?: Record<string, ZodField> };
  execute?: unknown;
};

/** unwrap peels the optional and default wrappers off a zod field. */
function unwrap(field: ZodField): ZodField {
  let current = field;
  for (let depth = 0; depth < 4; depth += 1) {
    const inner = current._def?.innerType;
    if (inner === undefined) {
      return current;
    }
    current = inner;
  }
  return current;
}

/** zodKind names the type a zod field carries, in the contract's words. */
function zodKind(field: ZodField): string {
  const typeName = unwrap(field)._def?.typeName ?? "";
  const kinds: Record<string, string> = {
    ZodString: "string",
    ZodNumber: "number",
    ZodBoolean: "boolean",
  };
  return kinds[typeName] ?? typeName.replace(/^Zod/, "").toLowerCase();
}

/** schemaKind names a JSON Schema property type in the same words. */
const schemaKind = (type: string | undefined): string => (type === "integer" ? "number" : (type ?? ""));

/** canonical renders the fields a body actually carries, so undefined adds nothing. */
function canonical(value: Record<string, unknown>): string {
  const keys = Object.keys(value)
    .filter((key) => value[key] !== undefined)
    .sort();
  return JSON.stringify(keys.map((key) => [key, value[key]]));
}

const sent: Seen[] = [];
const bound = createTools(
  bindLanding({
    async call(tool, field) {
      sent.push({ tool, field });
      return {};
    },
  }),
);

for (const name of Object.keys(tools).sort()) {
  const offered = (tools as Record<string, ToolLike>)[name];
  const executable = (bound as Record<string, ToolLike>)[name];
  if (offered === undefined || executable === undefined) {
    check(`the ${name} binding exists`, false, "the deployment offers no such tool");
    continue;
  }
  check(`the ${name} binding carries its frozen id`, offered.id === name, `id ${String(offered.id)}`);
  check(`the ${name} binding has a description`, typeof offered.description === "string" && offered.description.length > 0);
  check(`the ${name} binding has an execute`, typeof offered.execute === "function");

  const schema = await example(`${name}.request.schema.json`);
  const properties = (schema.properties ?? {}) as Record<string, { type?: string }>;
  const shape = offered.inputSchema?.shape ?? {};
  const names = Object.keys(shape);
  const required = names.filter((field) => !shape[field].isOptional()).sort();
  check(
    `the ${name} input schema requires the contract's fields`,
    sameList(required, [...((schema.required ?? []) as string[])].sort()),
    `requires ${JSON.stringify(required)}, contract requires ${JSON.stringify(schema.required)}`,
  );
  const unknown = names.filter((field) => properties[field] === undefined);
  check(`the ${name} input schema names only contract fields`, unknown.length === 0, `unknown ${JSON.stringify(unknown)}`);
  const mistyped = names.filter(
    (field) => properties[field] !== undefined && zodKind(shape[field]) !== schemaKind(properties[field].type),
  );
  check(`the ${name} input schema keeps the contract's types`, mistyped.length === 0, `mistyped ${JSON.stringify(mistyped)}`);

  const request = await example(`${name}.request.example.json`);
  await (executable.execute as (input: unknown, options?: unknown) => Promise<unknown>)(request, {});
  const last = sent[sent.length - 1];
  check(
    `the ${name} execute posts the contract's example request`,
    last?.tool === name && canonical(last.field) === canonical(request),
    `sent ${last ? JSON.stringify(last.field) : "nothing"}`,
  );
  check(`the ${name} execute sends one request per call`, sent.filter((call) => call.tool === name).length === 1);
}

// --- The prompt and the retry rule ---
//
// The prompt carries no correction story and no belief id: whatever this
// agent answers about the shopper's size came from the layer. And the retry
// rule lets a read be tried again while a write is looked up before it is
// resent, so a write that landed cannot be written twice by a retry.
const prompt = await Bun.file(join(import.meta.dir, "index.ts")).text();
check("the instructions carry no correction story", !/lost weight|corr_[0-9a-z]/i.test(prompt));
check("the instructions carry no belief id", !/blf_[0-9a-z]/i.test(prompt.split("const INSTRUCTIONS")[1] ?? ""));
const instructions = prompt.split("const INSTRUCTIONS")[1] ?? "";
check("the retry rule is in the instructions", /look before sending it again/.test(instructions));
check("a lookup may be tried again", instructions.includes("earns one more try with shopper_01"));
const lookupList = /A lookup — ([^—]+) — earns one more try/.exec(instructions)?.[1] ?? "";
const lookups = lookupList
  .split(/\s*,\s*|\s+or\s+/)
  .map((name) => name.trim())
  .filter((name) => name !== "")
  .sort();
check(
  "only the four reads may be tried again",
  sameList(lookups, ["explain_belief", "pending_questions", "product_fit", "recall_for_purchase"]),
  `lookup list ${JSON.stringify(lookups)}`,
);
check("a write is never resent on a guess", instructions.includes("never send it again on a guess"));
const writeList = /A note that writes — ([^—]+) —/.exec(instructions)?.[1] ?? "";
const writers = writeList
  .split(/\s*,\s*|\s+or\s+/)
  .map((name) => name.trim())
  .filter((name) => name !== "")
  .sort();
check(
  "the rule names the three row-adding writes",
  sameList(writers, ["record_correction", "record_feedback", "record_reason"]),
  `write list ${JSON.stringify(writers)}`,
);
for (const reader of ["recall_for_purchase shows a correction", "pending_questions shows whether the question has been answered", "explain_belief shows a remark in the record"]) {
  check(`the rule names its confirmation: ${reader.split(" ")[0]}`, instructions.includes(reader));
}
check("the rule asks before filing a second time", instructions.includes("ask them before filing it a second time"));
check("set_pooling is named as safe to repeat", instructions.includes("set_pooling only states a preference"));
check("no blanket retry survives", !/give it one more attempt|retry (it|the call|that call|any call)\b/i.test(instructions));

// --- The HTTP path: the request this deployment sends, against a local stub ---
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
    // The ledger reads are the paths the edge refuses an agent key on.
    if (url.pathname === "/tools/ledger_belief") {
      return new Response(JSON.stringify({ error: { code: "unauthenticated", message: "unauthorized" } }), {
        status: 401,
        headers: { "content-type": "application/json" },
      });
    }
    if (url.pathname === "/tools/ledger_evidence") {
      return new Response("<html>bad gateway</html>", { status: 502 });
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
const posted = caught[caught.length - 1];
check("the call posts to the tool's edge path", posted.method === "POST" && posted.path === "/tools/recall_for_purchase");
check("the key travels as a bearer token", posted.authorization === "Bearer offline-check-key");
check("the body carries the shopper and the brand", posted.body.shopper_id === "shopper_01" && posted.body.brand_id === "brand-x-001");

// One call, one request: nothing here retries on its own, so a write can be
// duplicated only by the model, never by this code.
const before = caught.length;
await dialled.questions({ shopper_id: "shopper_01" });
check("one tool call sends exactly one request", caught.length === before + 1);

// A refused call carries the edge's own code and status.
let refused: unknown;
try {
  await connectEdge({ host: stub.url.toString(), token: "offline-check-key" }).call("ledger_belief", {
    shopper_id: "shopper_01",
    belief_id: "blf_8f2c41",
  });
} catch (cause) {
  refused = cause;
}
check(
  "a refusal surfaces the edge's own code",
  refused instanceof EdgeRefusal && refused.status === 401 && refused.code === "unauthenticated",
);
let garbled: unknown;
try {
  await connectEdge({ host: stub.url.toString(), token: "offline-check-key" }).call("ledger_evidence", {
    shopper_id: "shopper_01",
  });
} catch (cause) {
  garbled = cause;
}
check("a non-JSON answer is a reported failure", garbled instanceof EdgeRefusal && garbled.status === 502);
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
console.log("offline check passed: second agent answers from the served layer's edge, only the calls it may make");
