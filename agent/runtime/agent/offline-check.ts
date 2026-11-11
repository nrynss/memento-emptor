/**
 * Offline check: the agent's own tool code against the layer's frozen
 * contract examples. No model, no keys, no external network.
 *
 * Three layers are proved here. The tool bindings themselves (agent/tools.ts):
 * the ids the model is offered, their input schemas against the contract's
 * own request schemas, and each `execute` reaching the frozen tool name with
 * exactly the example request. The set of tools the deployment holds against
 * the edge's access table (internal/auth/surface.go): an agent key reaches
 * eight calls, and the three shopper-only ledger reads are not among them. And
 * the memory contract (correction wins, belief stays, one rung per precedence
 * step, the standalone correction) plus the HTTP call shape, over the
 * transport the deployment dials with (agent/layer.ts), with the contract's
 * own example responses in place of the network. A local stub server proves
 * the request the deployed code would send, that one call sends one request,
 * and that a refusal is reported with the layer's own code.
 *
 * The catalog search is proved the same way: its transport replays
 * `testdata/channel3`, the recordings `agent/catalog`'s own tests use, and
 * every returned product is compared field for field with the recording so
 * the tool can carry nothing Channel3 did not return. The search is
 * agent-local, so it is explicitly outside the edge's access table.
 *
 * Run: bun agent/offline-check.ts
 */
import { join } from "node:path";
import {
  buildCatalog,
  httpCatalogTransport,
  CatalogError,
  CodeBadLimit,
  CodeCatalogBadResponse,
  CodeCatalogUnconfigured,
  CodeEmptyQuery,
  CodeInvalidWant,
  type CatalogTransport,
  type ProductRecord,
  type SearchInput,
} from "./catalog.js";
import { buildLayer, httpTransport, LayerError, type Body, type LayerTransport } from "./layer.js";
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

/** One recorded call, as the fake edge saw it. */
interface Recorded {
  tool: string;
  body: Body;
}

/**
 * fakeEdge answers like the served layer does on the contract's examples.
 * shopper_01 is the demo correction story, shopper_02 the pool rung,
 * shopper_04 own evidence, shopper_06 the statement rung, anyone else none.
 * A standing correction recorded through the edge is served back on that
 * shopper's next recall, as the layer does. Any tool outside the agent's
 * eight is an error: this deployment holds no binding for one.
 */
async function fakeEdge(): Promise<{ transport: LayerTransport; calls: Recorded[] }> {
  const correction = await example("recall_for_purchase.response.example-correction.json");
  const pool = await example("recall_for_purchase.response.example-pool.json");
  const own = await example("recall_for_purchase.response.example-own.json");
  const none = await example("recall_for_purchase.response.example-none.json");
  const declared = await example("record_correction.response.example.json");
  const standing = await example("record_correction.response.example-standing.json");
  const fit = await example("product_fit.response.example.json");
  const fitBuilding = await example("product_fit.response.example-building.json");
  const fitClaimed = await example("product_fit.response.example-claimed.json");
  const questions = await example("pending_questions.response.example.json");
  const reason = await example("record_reason.response.example.json");
  const unclear = await example("record_reason.response.example-unclear.json");
  const feedback = await example("record_feedback.response.example.json");
  const explain = await example("explain_belief.response.example.json");
  const pooling = await example("set_pooling.response.example.json");

  // The statement rung decides on the shopper's Claimed words alone: the
  // pool rung's example with the pool gone, the belief no longer overridden
  // and the guidance on the statement.
  const statement: JSON_ = JSON.parse(JSON.stringify(pool));
  statement.beliefs[0].precedence = { step: "statement", overridden: false, overridden_by: null };
  statement.pool = null;
  statement.pool_lookup = "unavailable";
  statement.guidance = {
    text: "Take size M for Brand X: the shopper's usual size.",
    basis: "statement",
    correction_id: null,
    belief_id: statement.beliefs[0].belief_id,
  };

  const calls: Recorded[] = [];
  const standings = new Map<string, JSON_>();
  const route = (tool: string, body: Body): JSON_ => {
    switch (tool) {
      case "recall_for_purchase": {
        const shopper = String(body.shopper_id);
        const held = standings.get(shopper);
        if (held) {
          const out = JSON.parse(JSON.stringify(none));
          out.shopper_id = shopper;
          out.overriding_correction = held.correction;
          out.guidance = held.guidance;
          return out;
        }
        if (shopper === "shopper_01") return JSON.parse(JSON.stringify(correction));
        if (shopper === "shopper_02") return JSON.parse(JSON.stringify(pool));
        if (shopper === "shopper_04") return JSON.parse(JSON.stringify(own));
        if (shopper === "shopper_06") return JSON.parse(JSON.stringify(statement));
        const out = JSON.parse(JSON.stringify(none));
        out.shopper_id = shopper;
        return out;
      }
      case "record_correction": {
        if (typeof body.contradicts_belief_id === "string" && body.contradicts_belief_id !== "") {
          return JSON.parse(JSON.stringify(declared));
        }
        const out = JSON.parse(JSON.stringify(standing));
        standings.set(String(body.shopper_id), out);
        return out;
      }
      case "product_fit": {
        if (body.size === "M") return JSON.parse(JSON.stringify(fit));
        if (body.size === "L") return JSON.parse(JSON.stringify(fitBuilding));
        return JSON.parse(JSON.stringify(fitClaimed));
      }
      case "pending_questions":
        return JSON.parse(JSON.stringify(questions));
      case "record_reason":
        return JSON.parse(
          JSON.stringify(/tight|small|short|sleeve|button|loose|large|big/i.test(String(body.answer_text)) ? reason : unclear),
        );
      case "record_feedback":
        return JSON.parse(JSON.stringify(feedback));
      case "explain_belief":
        return JSON.parse(JSON.stringify(explain));
      case "set_pooling":
        return JSON.parse(JSON.stringify(pooling));
      default:
        throw new Error(`fake edge: unknown tool ${tool}`);
    }
  };
  return {
    calls,
    transport: {
      async call(tool, body) {
        calls.push({ tool, body });
        const out = route(tool, body);
        out.trace_id = typeof body.trace_id === "string" ? body.trace_id : null;
        return out;
      },
    },
  };
}

const { transport, calls } = await fakeEdge();
const layer = buildLayer(transport);

// --- The catalog: the agent's own shelf over Channel3's recordings ---
//
// `agent/catalog` (Go) proves this same rule against these same recordings.
// The tool returns Channel3's product records unchanged, so the checks below
// also compare each returned record with its recording, field for field.

const CHANNEL3_DIR = join(REPO_DIR, "testdata/channel3");

/** channel3Recording reads one recorded Channel3 search answer. */
async function channel3Recording(name: string): Promise<JSON_> {
  return (await Bun.file(join(CHANNEL3_DIR, name)).json()) as JSON_;
}

/** recordedCatalog answers the two recorded searches and nothing else. */
async function recordedCatalog(): Promise<{
  transport: CatalogTransport;
  calls: { query: string; limit: number }[];
}> {
  const oxford = await channel3Recording("search-oxford-shirts.json");
  const puma = await channel3Recording("search-puma-running-shoes.json");
  const catalogCalls: { query: string; limit: number }[] = [];
  return {
    calls: catalogCalls,
    transport: {
      async search(query, limit) {
        catalogCalls.push({ query, limit });
        if (query === "mens oxford button-down shirt") {
          return oxford.products as ProductRecord[];
        }
        if (query === "puma running shoes") {
          return puma.products as ProductRecord[];
        }
        return [];
      },
    },
  };
}

const recorded = await recordedCatalog();
const catalog = buildCatalog(recorded.transport);

// Turn 1: a shopping request answered from the served layer's shapes.
// Recall first, and the correction decides while the belief stays.
const recall = await layer.recallForPurchase({
  shopper_id: "shopper_01",
  brand_id: "brand-x-001",
  category: "shirts",
  fit_handle: "regular",
  size_considered: "M",
  trace_id: "trace_3f8a01",
});
check("the first call is recall_for_purchase", calls.length === 1 && calls[0].tool === "recall_for_purchase");
check("recall sends the shopper and the brand", calls[0].body.shopper_id === "shopper_01" && calls[0].body.brand_id === "brand-x-001");
const beliefs = recall.beliefs as JSON_[];
check("recall returns the sizing belief", beliefs.length === 1 && beliefs[0].belief_id === "blf_8f2c41");
check(
  "recall belief is Out of true, standing Venerable",
  beliefs[0].status === "Out of true" && beliefs[0].standing === "Venerable",
);
check("recall belief stays contradicted, not deleted", beliefs[0].contradicted === true);
check("recall cites 3 fit returns", (beliefs[0].evidence as unknown[]).length === 3);
const guidance = recall.guidance as JSON_;
check(
  "correction wins the recommendation",
  guidance.text === "Take size M for Brand X" && guidance.basis === "correction",
);
check("overriding correction present", (recall.overriding_correction as JSON_).correction_id === "corr_3b7e52");
check("pending refund surfaces", recall.pending_count === 1);
check("the trace id travels", recall.trace_id === "trace_3f8a01");

// Correction step: the own evidence and the pool both ride along overridden.
const demoPool = recall.pool as JSON_;
check(
  "correction step marks own evidence overridden",
  beliefs[0].precedence.step === "own_evidence" &&
    beliefs[0].precedence.overridden === true &&
    beliefs[0].precedence.overridden_by === "correction",
);
check(
  "correction step marks the pool overridden",
  demoPool.precedence.step === "pool" &&
    demoPool.precedence.overridden === true &&
    demoPool.precedence.overridden_by === "correction",
);
check(
  "correction step consulted the Earned pool",
  recall.pool_lookup === "consulted" && demoPool.status === "Earned" && demoPool.direction === "small",
);
check(
  "the pool's seeded split is quoted, not summed",
  demoPool.counts.settled === demoPool.seeded.settled + demoPool.live.settled,
);

// Pool step: a newcomer is sized from the pool over their usual size.
const pooled = await layer.recallForPurchase({ shopper_id: "shopper_02", brand_id: "brand-x-001" });
const pooledPool = pooled.pool as JSON_;
const pooledBeliefs = pooled.beliefs as JSON_[];
check(
  "pool step decides from the pool",
  (pooled.guidance as JSON_).basis === "pool" && pooledPool.status === "Earned" && pooledPool.direction === "small",
);
check("pool guidance cites its counts", String((pooled.guidance as JSON_).text).includes("29 of the 47"));
check(
  "pool step marks the statement overridden",
  pooledBeliefs[0].precedence.step === "statement" &&
    pooledBeliefs[0].precedence.overridden === true &&
    pooledBeliefs[0].precedence.overridden_by === "pool",
);

// Own-evidence step: the shopper unlike the crowd, pool overridden.
const own = await layer.recallForPurchase({ shopper_id: "shopper_04", brand_id: "brand-x-001" });
check("own-evidence step beats the pool", (own.guidance as JSON_).basis === "own_evidence");
check(
  "own-evidence step marks the pool overridden",
  (own.pool as JSON_).precedence.overridden === true && (own.pool as JSON_).precedence.overridden_by === "own_evidence",
);

// Statement step: the usual size decides, labelled Claimed, never evidence.
const stmt = await layer.recallForPurchase({ shopper_id: "shopper_06", brand_id: "brand-x-001" });
const stmtBeliefs = stmt.beliefs as JSON_[];
check("statement step decides from the usual size", (stmt.guidance as JSON_).basis === "statement");
check(
  "statement step stays Claimed and un-overridden",
  stmtBeliefs[0].status === "Claimed" && stmtBeliefs[0].precedence.overridden === false,
);

// None step: the layer never refuses, it says plainly no memory applies.
const none = await layer.recallForPurchase({ shopper_id: "shopper_99", brand_id: "brand-x-001" });
check(
  "none step says so plainly, never refuses",
  (none.beliefs as unknown[]).length === 0 &&
    (none.guidance as JSON_).basis === "none" &&
    String((none.guidance as JSON_).text).includes("No memory applies yet"),
);

// A correction against a served belief returns the declared shape.
const corr = await layer.recordCorrection({
  shopper_id: "shopper_01",
  text: "I've lost weight, use M",
  contradicts_belief_id: "blf_8f2c41",
  trace_id: "trace_3f8a02",
});
check("declared correction takes precedence", (corr.correction as JSON_).takes_precedence === true);
check(
  "the belief stays Out of true beside the correction",
  corr.status === "Out of true" && (corr.guidance as JSON_).basis === "correction",
);

// Standalone correction: recall served nothing, so the correction is
// recorded with no belief id, stands Claimed, and decides again.
const stand = await layer.recordCorrection({ shopper_id: "shopper_77", text: "Actually use size M for me" });
check(
  "standalone correction stands Claimed",
  stand.status === "Claimed" && stand.standing === "None" && stand.contradicted === false,
);
const again = await layer.recallForPurchase({ shopper_id: "shopper_77", brand_id: "brand-x-001" });
check(
  "standalone correction stays in force",
  (again.overriding_correction as JSON_).correction_id === (stand.correction as JSON_).correction_id &&
    (again.guidance as JSON_).basis === "correction",
);

// Pending question, then the vague answer asks again.
const pending = await layer.pendingQuestions({ shopper_id: "shopper_01" });
check("one pending question", (pending.questions as unknown[]).length === 1);
const vague = await layer.recordReason({ shopper_id: "shopper_01", question_id: "q_9f04ab", answer_text: "not great" });
check("vague answer asks again", vague.ask_again === true && vague.accepted === false);
const clear = await layer.recordReason({
  shopper_id: "shopper_01",
  question_id: "q_9f04ab",
  answer_text: "Too tight in the shoulders, couldn't button it.",
});
check("clear fit answer counts toward fit", clear.accepted === true && clear.category === "fit_too_small");

// Feedback ties to the purchase.
const fb = await layer.recordFeedback({
  shopper_id: "shopper_01",
  custom_id: "memento-shopper01-06",
  sku: "PSW8GeK",
  size: "M",
  feedback_text: "The M fits well across the shoulders.",
});
check("fits-well feedback accepted", fb.verdict === "fits_well" && fb.accepted === true);

// Explanation serves the layer's rows.
const expl = await layer.explainBelief({ shopper_id: "shopper_01", belief_id: "blf_8f2c41" });
check(
  "explanation carries the timeline and the correction",
  (expl.timeline as unknown[]).length > 0 && (expl.corrections as unknown[]).length === 1,
);

// Pool and pooling tools serve counts, never shopper rows.
const fit = await layer.productFit({ brand_id: "brand-x-001", category: "shirts", fit_handle: "regular", size: "M" });
check("product_fit serves the Earned pool", (fit.pool as JSON_).status === "Earned");
check("product_fit carries no shopper", calls[calls.length - 1].body.shopper_id === undefined);
const optout = await layer.setPooling({ shopper_id: "shopper_02", pooling: false });
check("opting out reports the rows removed", optout.pooling === false && optout.rows_removed === 3);

// --- The tool surface: what the edge admits to an agent key ---
//
// The table is read from the layer's own source, so it cannot drift away from
// the tools this deployment offers without this check going red. An agent key
// reaches the eight calls that act for a shopper plus the pooled read; the
// three ledger reads are shopper-only and are not bound here.

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

/**
 * AGENT_LOCAL_TOOLS are the tools this agent owns rather than the layer. The
 * edge's access table admits only layer calls, so a catalog search is neither
 * admitted nor refused there: it must sit outside the edge set and the edge
 * set must stay exactly as the table reads.
 */
const AGENT_LOCAL_TOOLS = ["search_catalog"];

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
const offeredEdge = Object.keys(tools)
  .filter((name) => !AGENT_LOCAL_TOOLS.includes(name))
  .sort();
check(
  "the deployment offers exactly the tools an agent key reaches, plus its own catalog search",
  sameList(offeredEdge, admitted),
  `offers ${JSON.stringify(offeredEdge)}`,
);
check(
  "the catalog search is agent-local, outside the edge's set",
  sameList([...AGENT_LOCAL_TOOLS].sort(), ["search_catalog"]) &&
    AGENT_LOCAL_TOOLS.every((name) => !admitted.includes(name)),
  `offers ${JSON.stringify(Object.keys(tools).sort())}`,
);
check(
  "the layer binding holds one method per admitted tool",
  Object.keys(layer).length === admitted.length,
  `layer has ${Object.keys(layer).length}`,
);
for (const name of admitted) {
  check(`the memory contract exercises ${name}`, calls.some((call) => call.tool === name));
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
  outputSchema?: { shape?: Record<string, ZodField> };
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

const sent: Recorded[] = [];
const bound = createTools(
  buildLayer({
    async call(tool, body) {
      sent.push({ tool, body });
      return {};
    },
  }),
  catalog,
);

for (const name of admitted) {
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
    last?.tool === name && canonical(last.body) === canonical(request),
    `sent ${last ? JSON.stringify(last.body) : "nothing"}`,
  );
  check(`the ${name} execute sends one request per call`, sent.filter((call) => call.tool === name).length === 1);
}

// --- The retry rule: a read may be retried, a write may not be repeated blind ---
//
// Nothing in the code retries: one call is one request (checked above), so a
// duplicated write could only come from the model repeating it. The
// instructions therefore decide, and this pins them.

const instructions = (await Bun.file(join(import.meta.dir, "index.ts")).text()).split("const INSTRUCTIONS")[1] ?? "";
check("the retry rule is in the instructions", /Retries\./.test(instructions));
check(
  "a read may be tried again",
  /read that fails \(recall_for_purchase, pending_questions, explain_belief, product_fit, search_catalog\) may be called once more/.test(
    instructions,
  ),
);
check("a write may not be repeated blind", instructions.includes("A write may not be repeated blind"));
for (const [tool, reader, subject] of [
  ["record_correction", "recall_for_purchase", "a correction"],
  ["record_reason", "pending_questions", "a reason"],
  ["record_feedback", "explain_belief", "feedback"],
] as const) {
  check(
    `${tool}'s retry reads ${reader} first`,
    instructions.includes(`${reader} for ${subject}`) && instructions.includes(tool),
    `wants "${reader} for ${subject}"`,
  );
}
check("the rule asks before resending what it cannot see", instructions.includes("ask before sending it again"));
check("set_pooling is named as safe to repeat", /set_pooling states the shopper's choice/.test(instructions));
check("no blanket retry survives", !/retry (it|the call|that call|any call)\b/i.test(instructions));

// --- The catalog search: a real candidate line, size and SKU, Channel3's fields only ---
//
// The recordings are the ones agent/catalog's tests replay, so the rule is
// checked against the same evidence. Every product the search returns is
// compared with its recording field for field: the tool may drop a product
// the cut does not match, but it may not add a field Channel3 did not send.

/** idsOf reads product ids in listing order. */
const idsOf = (products: ProductRecord[]): string[] =>
  products.map((product) => (typeof product.id === "string" ? product.id : ""));

/** sizeLabels reads the Size option's listed labels, or empty when the record models no variants. */
function sizeLabels(product: ProductRecord): string[] {
  const variants = product.variants;
  if (typeof variants !== "object" || variants === null) {
    return [];
  }
  const options = (variants as { options?: unknown }).options;
  if (!Array.isArray(options)) {
    return [];
  }
  const labels: string[] = [];
  for (const option of options) {
    if (typeof option !== "object" || option === null) {
      continue;
    }
    if ((option as { name?: unknown }).name !== "Size") {
      continue;
    }
    const values = (option as { values?: unknown }).values;
    if (!Array.isArray(values)) {
      continue;
    }
    for (const value of values) {
      if (typeof value !== "object" || value === null) {
        continue;
      }
      const label = (value as { label?: unknown }).label;
      if (typeof label === "string") {
        labels.push(label);
      }
    }
  }
  return labels;
}

/** catalogRefusal runs a search expected to fail and returns its typed refusal. */
async function catalogRefusal(run: () => Promise<unknown>): Promise<CatalogError | undefined> {
  try {
    await run();
  } catch (cause) {
    return cause instanceof CatalogError ? cause : undefined;
  }
  return undefined;
}

const oxfordWant: SearchInput = {
  query: "mens oxford button-down shirt",
  brand_id: "SMwk",
  category: "dress-shirts",
  fit_attribute: "top-fit",
  fit_values: ["Regular"],
};

const line = await catalog.search(oxfordWant);
check(
  "the oxford search returns the three SKUs that share the cut",
  sameList(idsOf(line.products), ["tbnrl46", "VyAkjCA", "PSW8GeK"]),
  `ids ${JSON.stringify(idsOf(line.products))}`,
);
const newLine = await catalog.search({ ...oxfordWant, exclude_skus: ["tbnrl46"] });
check(
  "excluding a returned SKU leaves the other two",
  sameList(idsOf(newLine.products), ["VyAkjCA", "PSW8GeK"]),
  `ids ${JSON.stringify(idsOf(newLine.products))}`,
);
const candidate = newLine.products.find((product) => product.id === "PSW8GeK");
check("the demo search names a real candidate line", candidate?.title === "Button Down Oxford Shirt");
check("the demo search names the candidate's real SKU", candidate?.id === "PSW8GeK");
check(
  "the apparel candidate carries no size Channel3 did not return",
  candidate?.variants === null && sizeLabels(candidate ?? {}).length === 0,
);

const footwear = await catalog.search({
  query: "puma running shoes",
  brand_id: "T0QO",
  category: "athletic-shoes",
  fit_attribute: "footwear-cut",
  fit_values: ["Low cut"],
});
check(
  "the footwear search keeps the two Low cut SKUs",
  sameList(idsOf(footwear.products), ["4d8UHzT", "VWvTIPF"]),
  `ids ${JSON.stringify(idsOf(footwear.products))}`,
);
const darter = footwear.products.find((product) => product.id === "VWvTIPF");
check("the footwear search names a real candidate line", darter?.title === "Men's Puma Darter Pro 2 Running Shoes");
check("the footwear search names the candidate's real SKU", darter?.id === "VWvTIPF");
check(
  "the footwear search names the sizes Channel3 lists",
  sameList(sizeLabels(darter ?? {}), ["7.5", "9", "9.5", "10", "10.5"]),
  `sizes ${JSON.stringify(sizeLabels(darter ?? {}))}`,
);

// Field for field: no field appears that Channel3 did not return.
const recordings = new Map<string, string[]>();
for (const recording of [await channel3Recording("search-oxford-shirts.json"), await channel3Recording("search-puma-running-shoes.json")]) {
  for (const product of recording.products as JSON_[]) {
    recordings.set(String(product.id), Object.keys(product).sort());
  }
}
const returned = new Map<string, ProductRecord>();
for (const product of [...line.products, ...newLine.products, ...footwear.products]) {
  returned.set(String(product.id), product);
}
for (const [id, product] of returned) {
  const recordedFields = recordings.get(id);
  check(
    `candidate ${id} carries exactly Channel3's fields`,
    recordedFields !== undefined && sameList(Object.keys(product).sort(), recordedFields),
    `fields ${JSON.stringify(Object.keys(product).sort())}`,
  );
}
for (const invented of ["size_note", "fulfilment", "shared", "available_sizes", "match", "size_chart"]) {
  check(
    `no candidate carries an invented ${invented} field`,
    [...returned.values()].every((product) => !(invented in product)),
  );
}
check("the search answer carries only Channel3's products field", sameList(Object.keys(line).sort(), ["products"]));

// The negative control: one line name, divergent fit labels. The line name
// alone never matches, exactly as agent/catalog's own test proves.
const slim = await catalog.search({
  query: "mens oxford button-down shirt",
  brand_id: "ISZ8",
  category: "dress-shirts",
  fit_attribute: "top-fit",
  fit_values: ["Slim"],
});
check("the Bonobos line name alone does not match both SKUs", sameList(idsOf(slim.products), ["V9BYM3z"]));
const slimTailored = await catalog.search({
  query: "mens oxford button-down shirt",
  brand_id: "ISZ8",
  category: "dress-shirts",
  fit_attribute: "top-fit",
  fit_values: ["Slim", "Tailored"],
});
check("an equal fit set matches the other Bonobos SKU", sameList(idsOf(slimTailored.products), ["m4iRrEM"]));

// A search that cannot match says so, and never reaches the catalog.
const callsBeforeRefusal = recorded.calls.length;
check(
  "an incomplete cut is refused",
  (await catalogRefusal(() => catalog.search({ ...oxfordWant, brand_id: "" })))?.code === CodeInvalidWant,
);
check(
  "a search with no query is refused",
  (await catalogRefusal(() => catalog.search({ ...oxfordWant, query: "   " })))?.code === CodeEmptyQuery,
);
check(
  "a non-positive limit is refused",
  (await catalogRefusal(() => catalog.search({ ...oxfordWant, limit: 0 })))?.code === CodeBadLimit,
);
check("a refused search never reaches the catalog", recorded.calls.length === callsBeforeRefusal);

// The tool binding itself: agent-local, and the same factory the deployment uses.
const searchTool = (tools as Record<string, ToolLike>).search_catalog;
const searchBound = (bound as Record<string, ToolLike>).search_catalog;
check("search_catalog is offered with its own id", searchTool?.id === "search_catalog" && searchBound !== undefined);
check(
  "search_catalog has a description",
  typeof searchTool?.description === "string" && searchTool.description.length > 0,
);
const toolCallsBefore = recorded.calls.length;
const toolAnswer = (await (searchBound?.execute as (input: unknown, options?: unknown) => Promise<unknown>)(
  { ...oxfordWant, exclude_skus: ["tbnrl46"] },
  {},
)) as { products: ProductRecord[] };
check(
  "the tool asks the catalog for the line with the default limit",
  recorded.calls.length === toolCallsBefore + 1 &&
    recorded.calls[recorded.calls.length - 1].query === oxfordWant.query &&
    recorded.calls[recorded.calls.length - 1].limit === 10,
);
check("the tool returns the matched Channel3 records", sameList(idsOf(toolAnswer.products), ["VyAkjCA", "PSW8GeK"]));
check("the tool's answer carries only products", sameList(Object.keys(toolAnswer).sort(), ["products"]));
check(
  "the tool's input names the cut the rule needs",
  sameList(Object.keys(searchTool?.inputSchema?.shape ?? {}).sort(), [
    "brand_id",
    "category",
    "exclude_skus",
    "fit_attribute",
    "fit_values",
    "limit",
    "query",
  ]),
);
check(
  "the tool's output carries Channel3's products",
  sameList(Object.keys(searchTool?.outputSchema?.shape ?? {}), ["products"]),
);
check(
  "search_catalog is not a frozen edge tool",
  !(await Bun.file(join(API_DIR, "search_catalog.request.schema.json")).exists()),
);
check(
  "the instructions search the catalog before naming a product",
  /FIND THE CANDIDATE/.test(instructions) && instructions.includes("search_catalog"),
);
check(
  "the instructions name the catalog's own ids",
  instructions.includes("SMwk") && instructions.includes("dress-shirts") && instructions.includes("top-fit"),
);
check(
  "the instructions forbid inventing a size",
  /Never state a size the catalog does not carry/.test(instructions),
);

// --- The HTTP path: the request the deployment sends, against a local stub ---
interface Seen {
  method: string;
  path: string;
  authorization: string;
  apiKey: string;
  contentType: string;
  body: JSON_;
}
const seen: Seen[] = [];
const stub = Bun.serve({
  port: 0,
  hostname: "127.0.0.1",
  async fetch(request) {
    const url = new URL(request.url);
    const body = (await request.json().catch(() => ({}))) as JSON_;
    seen.push({
      method: request.method,
      path: url.pathname,
      authorization: request.headers.get("authorization") ?? "",
      apiKey: request.headers.get("x-api-key") ?? "",
      contentType: request.headers.get("content-type") ?? "",
      body,
    });
    // The catalog's own paths: one search answer, one refusal, one
    // unreadable answer and one that carries no products.
    if (url.pathname === "/v1/search") {
      const products = (await channel3Recording("search-oxford-shirts.json")).products as JSON_[];
      const stripe = products.find((product) => product.id === "PSW8GeK");
      return new Response(JSON.stringify({ products: [stripe] }), {
        status: 200,
        headers: { "content-type": "application/json" },
      });
    }
    if (url.pathname === "/refused/search") {
      return new Response(JSON.stringify({ error: { code: "invalid_api_key", message: "bad key" } }), {
        status: 401,
        headers: { "content-type": "application/json" },
      });
    }
    if (url.pathname === "/garbled/search") {
      return new Response("<html>catalog down</html>", { status: 502 });
    }
    if (url.pathname === "/empty/search") {
      return new Response(JSON.stringify({}), { status: 200, headers: { "content-type": "application/json" } });
    }
    // The three ledger reads are the paths the edge refuses an agent key on.
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

const httpLayer = buildLayer(httpTransport({ url: stub.url.toString(), key: "offline-check-key" }));
const live = await httpLayer.recallForPurchase({
  shopper_id: "shopper_01",
  brand_id: "brand-x-001",
  category: "shirts",
  fit_handle: "regular",
  size_considered: "M",
  trace_id: "trace_3f8a09",
});
check("the HTTP path answers with the layer's response", (live.guidance as JSON_).basis === "correction");
const sentOverHttp = seen[seen.length - 1];
check("the call posts to the tool's edge path", sentOverHttp.method === "POST" && sentOverHttp.path === "/tools/recall_for_purchase");
check("the agent key travels as a bearer token", sentOverHttp.authorization === "Bearer offline-check-key");
check(
  "the body carries the shopper and the brand",
  sentOverHttp.body.shopper_id === "shopper_01" && sentOverHttp.body.brand_id === "brand-x-001",
);
check("the call sends JSON", sentOverHttp.contentType.startsWith("application/json"));

// One call, one request: the transport never retries on its own, so a write
// can be duplicated only by the model, never by this code.
const before = seen.length;
await httpLayer.pendingQuestions({ shopper_id: "shopper_01" });
check("one tool call sends exactly one request", seen.length === before + 1);

// A refused call carries the layer's own code and status.
let refused: unknown;
try {
  await httpTransport({ url: stub.url.toString(), key: "offline-check-key" }).call("ledger_belief", {
    shopper_id: "shopper_01",
    belief_id: "blf_8f2c41",
  });
} catch (cause) {
  refused = cause;
}
check(
  "a refused call surfaces the layer's code",
  refused instanceof LayerError && refused.status === 401 && refused.code === "unauthenticated",
);
let garbled: unknown;
try {
  await httpTransport({ url: stub.url.toString(), key: "offline-check-key" }).call("ledger_evidence", {
    shopper_id: "shopper_01",
  });
} catch (cause) {
  garbled = cause;
}
check("a non-JSON answer is a reported failure", garbled instanceof LayerError && garbled.status === 502);

// No key: the deployment refuses to call rather than answer locally.
let unconfigured: unknown;
try {
  await buildLayer(httpTransport({ url: stub.url.toString() })).productFit({
    brand_id: "brand-x-001",
    category: "shirts",
    fit_handle: "regular",
    size: "M",
  });
} catch (cause) {
  unconfigured = cause;
}
check(
  "a deployment without an agent key fails loudly",
  unconfigured instanceof LayerError && unconfigured.code === "unconfigured",
);

// The catalog search's own HTTP path: its request shape and its refusals.
const httpCatalog = buildCatalog(httpCatalogTransport({ url: `${stub.url}/v1`, key: "offline-catalog-key" }));
const onlineCatalog = await httpCatalog.search(oxfordWant);
const catalogRequest = seen[seen.length - 1];
check("the catalog search posts to its own path", catalogRequest.method === "POST" && catalogRequest.path === "/v1/search");
check("the catalog key travels in x-api-key", catalogRequest.apiKey === "offline-catalog-key");
check(
  "the catalog body carries the query and limit",
  catalogRequest.body.query === oxfordWant.query && catalogRequest.body.limit === 10,
);
check(
  "a catalog answer is matched, not echoed",
  sameList(idsOf(onlineCatalog.products), ["PSW8GeK"]),
  `ids ${JSON.stringify(idsOf(onlineCatalog.products))}`,
);
const beforeCatalogCall = seen.length;
await httpCatalog.search({ ...oxfordWant, limit: 5 });
check("one catalog call sends exactly one request", seen.length === beforeCatalogCall + 1);

let catalogRefused: unknown;
try {
  await httpCatalogTransport({ url: `${stub.url}/refused`, key: "offline-check-key" }).search("mens oxford", 1);
} catch (cause) {
  catalogRefused = cause;
}
check(
  "a refused catalog call carries the service's code",
  catalogRefused instanceof CatalogError && catalogRefused.status === 401 && catalogRefused.code === "invalid_api_key",
);
let catalogGarbled: unknown;
try {
  await httpCatalogTransport({ url: `${stub.url}/garbled`, key: "offline-check-key" }).search("mens oxford", 1);
} catch (cause) {
  catalogGarbled = cause;
}
check(
  "a non-JSON catalog answer is a reported failure",
  catalogGarbled instanceof CatalogError &&
    catalogGarbled.status === 502 &&
    catalogGarbled.code === CodeCatalogBadResponse,
);
let catalogEmpty: unknown;
try {
  await httpCatalogTransport({ url: `${stub.url}/empty`, key: "offline-check-key" }).search("mens oxford", 1);
} catch (cause) {
  catalogEmpty = cause;
}
check(
  "an answer with no products array is a reported failure, never an empty shelf",
  catalogEmpty instanceof CatalogError && catalogEmpty.code === CodeCatalogBadResponse,
);
let catalogUnconfigured: unknown;
try {
  await httpCatalogTransport({ url: `${stub.url}/v1` }).search("mens oxford", 1);
} catch (cause) {
  catalogUnconfigured = cause;
}
check(
  "a deployment without a catalog key fails loudly",
  catalogUnconfigured instanceof CatalogError && catalogUnconfigured.code === CodeCatalogUnconfigured,
);

stub.stop(true);

if (failures > 0) {
  console.log(`${failures} check(s) failed`);
  process.exit(1);
}
console.log(
  "offline check passed: the layer's eight tools plus the agent-local catalog search, recall first, no blind write",
);
