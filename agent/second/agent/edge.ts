/**
 * The layer's edge, as the second agent's only source of memory.
 *
 * This deployment keeps nothing itself: every read and every write is one
 * `POST <edge>/tools/<tool>` with an agent key in `Authorization: Bearer`
 * and the shopper id in the body. The connection is injected so the offline
 * check can answer from the contract's frozen examples, but the deployed
 * path always dials the live edge: a deployment with no key refuses to call
 * rather than answer from anywhere local.
 */

/** HOSTED_EDGE is the served layer when the deployment names no other edge. */
export const HOSTED_EDGE = "https://memento-emptor.onrender.com";

/** RING_TIMEOUT_MS is how long one call may take before the turn hears of it. */
const RING_TIMEOUT_MS = 20_000;

/** The local fault codes: a call that could not be made, or could not be read. */
export const NoKey = "unconfigured";
export const NoAnswer = "layer-unreachable";
export const LateAnswer = "layer-timeout";
export const Unreadable = "layer-bad-response";

/** EdgeRefusal is one call the edge would not serve, or could not be reached for. */
export class EdgeRefusal extends Error {
  /** status is the HTTP status, or 0 when nothing answered. */
  readonly status: number;
  /** code is the edge's error code, or one of this module's own. */
  readonly code: string;
  /** tool is the frozen tool the call named. */
  readonly tool: string;

  constructor(tool: string, status: number, code: string, message: string) {
    super(`${tool}: ${code}${status ? ` (HTTP ${status})` : ""}: ${message}`);
    this.name = "EdgeRefusal";
    this.tool = tool;
    this.status = status;
    this.code = code;
  }
}

/** Wire is one call's request body: exactly the fields the contract names. */
export type Wire = Record<string, unknown>;

/** Edge is one frozen tool call: the request in, the response out. */
export interface Edge {
  call(tool: string, field: Wire): Promise<Record<string, unknown>>;
}

/** Where is the edge, and who is calling it. */
export interface Where {
  /** host is the edge's base; the hosted layer when empty. */
  host?: string;
  /** token is the deployment's agent key. */
  token?: string;
}

/** codeIn reads the edge's `{error:{code}}` answer, or returns empty. */
function codeIn(answer: unknown): string {
  if (typeof answer !== "object" || answer === null) {
    return "";
  }
  const fault = (answer as { error?: unknown }).error;
  if (typeof fault !== "object" || fault === null) {
    return "";
  }
  const code = (fault as { code?: unknown }).code;
  return typeof code === "string" ? code : "";
}

/**
 * connectEdge dials the served layer. It keeps no state between calls: each
 * one carries its own key and body.
 */
export function connectEdge(where: Where): Edge {
  const host = (where.host?.trim() || HOSTED_EDGE).replace(/\/+$/, "");
  const token = where.token?.trim() ?? "";
  return {
    async call(tool: string, field: Wire): Promise<Record<string, unknown>> {
      if (token === "") {
        throw new EdgeRefusal(tool, 0, NoKey, "MEMENTO_AGENT_KEY is empty: this deployment holds no key");
      }
      let answer: Response;
      try {
        answer = await fetch(`${host}/tools/${tool}`, {
          method: "POST",
          headers: {
            authorization: `Bearer ${token}`,
            "content-type": "application/json",
            accept: "application/json",
          },
          body: JSON.stringify(field),
          signal: AbortSignal.timeout(RING_TIMEOUT_MS),
        });
      } catch (cause) {
        if (cause instanceof Error && cause.name === "TimeoutError") {
          throw new EdgeRefusal(tool, 0, LateAnswer, `nothing within ${RING_TIMEOUT_MS} ms`);
        }
        throw new EdgeRefusal(tool, 0, NoAnswer, cause instanceof Error ? cause.message : String(cause));
      }
      const text = await answer.text();
      let parsed: unknown;
      try {
        parsed = text === "" ? {} : JSON.parse(text);
      } catch {
        throw new EdgeRefusal(tool, answer.status, Unreadable, text.slice(0, 200));
      }
      if (!answer.ok) {
        const code = codeIn(parsed) || `http-${answer.status}`;
        throw new EdgeRefusal(tool, answer.status, code, JSON.stringify(parsed).slice(0, 300));
      }
      if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
        throw new EdgeRefusal(tool, answer.status, Unreadable, "the answer is not an object");
      }
      return parsed as Record<string, unknown>;
    },
  };
}

/** Landing is the layer bound to the eleven frozen tools. */
export interface Landing {
  recall(call: {
    shopper_id: string;
    brand_id: string;
    line_name?: string;
    category?: string;
    fit_handle?: string;
    sku?: string;
    size_considered?: string;
    trace_id?: string;
  }): Promise<Record<string, unknown>>;
  correct(call: {
    shopper_id: string;
    text: string;
    contradicts_belief_id?: string;
    trace_id?: string;
  }): Promise<Record<string, unknown>>;
  questions(call: { shopper_id: string; limit?: number }): Promise<Record<string, unknown>>;
  reason(call: {
    shopper_id: string;
    question_id: string;
    answer_text: string;
    trace_id?: string;
  }): Promise<Record<string, unknown>>;
  feedback(call: {
    shopper_id: string;
    custom_id: string;
    sku: string;
    size: string;
    feedback_text: string;
    trace_id?: string;
  }): Promise<Record<string, unknown>>;
  explain(call: { shopper_id: string; belief_id: string }): Promise<Record<string, unknown>>;
  fit(call: {
    brand_id: string;
    category: string;
    fit_handle: string;
    size: string;
  }): Promise<Record<string, unknown>>;
  pooling(call: {
    shopper_id: string;
    pooling: boolean;
    trace_id?: string;
  }): Promise<Record<string, unknown>>;
  ledger(call: { shopper_id: string; include_seeded?: boolean }): Promise<Record<string, unknown>>;
  ledgerRow(call: { shopper_id: string; belief_id?: string }): Promise<Record<string, unknown>>;
  ledgerProof(call: { shopper_id: string; belief_id?: string }): Promise<Record<string, unknown>>;
}

/**
 * bindLanding puts the eleven frozen tools on one edge. Each method sends
 * the contract's request fields and nothing else.
 */
export function bindLanding(edge: Edge): Landing {
  return {
    recall: (call) =>
      edge.call("recall_for_purchase", {
        shopper_id: call.shopper_id,
        brand_id: call.brand_id,
        line_name: call.line_name,
        category: call.category,
        fit_handle: call.fit_handle,
        sku: call.sku,
        size_considered: call.size_considered,
        trace_id: call.trace_id,
      }),
    correct: (call) =>
      edge.call("record_correction", {
        shopper_id: call.shopper_id,
        text: call.text,
        contradicts_belief_id: call.contradicts_belief_id,
        trace_id: call.trace_id,
      }),
    questions: (call) =>
      edge.call("pending_questions", { shopper_id: call.shopper_id, limit: call.limit }),
    reason: (call) =>
      edge.call("record_reason", {
        shopper_id: call.shopper_id,
        question_id: call.question_id,
        answer_text: call.answer_text,
        trace_id: call.trace_id,
      }),
    feedback: (call) =>
      edge.call("record_feedback", {
        shopper_id: call.shopper_id,
        custom_id: call.custom_id,
        sku: call.sku,
        size: call.size,
        feedback_text: call.feedback_text,
        trace_id: call.trace_id,
      }),
    explain: (call) =>
      edge.call("explain_belief", { shopper_id: call.shopper_id, belief_id: call.belief_id }),
    fit: (call) =>
      edge.call("product_fit", {
        brand_id: call.brand_id,
        category: call.category,
        fit_handle: call.fit_handle,
        size: call.size,
      }),
    pooling: (call) =>
      edge.call("set_pooling", {
        shopper_id: call.shopper_id,
        pooling: call.pooling,
        trace_id: call.trace_id,
      }),
    ledger: (call) =>
      edge.call("ledger_beliefs", {
        shopper_id: call.shopper_id,
        include_seeded: call.include_seeded ?? true,
      }),
    ledgerRow: (call) =>
      edge.call("ledger_belief", { shopper_id: call.shopper_id, belief_id: call.belief_id }),
    ledgerProof: (call) =>
      edge.call("ledger_evidence", { shopper_id: call.shopper_id, belief_id: call.belief_id }),
  };
}

/**
 * landingFromEnv dials the edge from this deployment's own environment: the
 * key arrives from the platform's secret store, never from a file.
 */
export function landingFromEnv(env: Record<string, string | undefined> = process.env): Landing {
  return bindLanding(connectEdge({ host: env.MEMENTO_LAYER_URL, token: env.MEMENTO_AGENT_KEY }));
}
