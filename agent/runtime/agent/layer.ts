/**
 * The served layer, as the reference agent's only memory.
 *
 * Every belief, correction, pool count and ledger row the agent reads or
 * writes comes from the Memento layer over its edge:
 * `POST <base>/tools/<tool>` with the deployment's agent key in
 * `Authorization: Bearer` and the shopper id in the body. Nothing here
 * stands in for the layer: a missing key or a refused call surfaces as an
 * error the turn reports, so a demo turn can never be answered from a
 * copy of the shapes.
 *
 * The transport is injected, which is how the offline check runs the same
 * tool code against the contract's own frozen examples without a network.
 */

/** DEFAULT_LAYER_URL is the hosted layer's edge when the deployment names none. */
export const DEFAULT_LAYER_URL = "https://memento-emptor.onrender.com";

/** CALL_TIMEOUT_MS bounds one tool call so a stalled layer cannot stall a turn. */
const CALL_TIMEOUT_MS = 20_000;

/** The error code on a call the deployment cannot make: no agent key. */
export const CodeUnconfigured = "unconfigured";

/** The error code on a call that never reached the layer. */
export const CodeUnreachable = "layer-unreachable";

/** The error code on a call that reached the layer and never answered in time. */
export const CodeTimeout = "layer-timeout";

/** The error code on a call the layer answered with something that is not JSON. */
export const CodeBadResponse = "layer-bad-response";

/**
 * LayerError is one refused, unreachable or unreadable tool call. It carries
 * the HTTP status and the layer's own error code where there is one, so a
 * caller can tell a bad key (401) from a missing belief (404) without
 * reading the message.
 */
export class LayerError extends Error {
  /** status is the HTTP status, or 0 when no response arrived. */
  readonly status: number;
  /** code is the layer's error code, or this module's own for a local fault. */
  readonly code: string;
  /** tool is the frozen tool name the call named. */
  readonly tool: string;

  constructor(tool: string, status: number, code: string, message: string) {
    super(`${tool}: ${code}${status ? ` (HTTP ${status})` : ""}: ${message}`);
    this.name = "LayerError";
    this.tool = tool;
    this.status = status;
    this.code = code;
  }
}

/** Body is one tool call's request body: the contract's request fields. */
export type Body = Record<string, unknown>;

/**
 * LayerTransport performs one frozen tool call and returns the tool's
 * response. The deployed path is httpTransport; the offline check injects
 * the contract's frozen examples instead.
 */
export interface LayerTransport {
  call(tool: string, body: Body): Promise<Record<string, unknown>>;
}

/** HTTPConfig names where the layer is and which deployment key calls it. */
export interface HTTPConfig {
  /** url is the edge's base, without a trailing slash; the default when empty. */
  url?: string;
  /** key is the deployment's agent key, sent as `Bearer`. */
  key?: string;
}

/** errorCode reads the layer's `{error:{code}}` answer, or empty. */
function errorCode(parsed: unknown): string {
  if (typeof parsed !== "object" || parsed === null) {
    return "";
  }
  const error = (parsed as { error?: unknown }).error;
  if (typeof error !== "object" || error === null) {
    return "";
  }
  const code = (error as { code?: unknown }).code;
  return typeof code === "string" ? code : "";
}

/**
 * httpTransport calls the served layer through its edge. It refuses to send
 * without an agent key rather than fall back to anything local.
 */
export function httpTransport(config: HTTPConfig): LayerTransport {
  const base = (config.url?.trim() || DEFAULT_LAYER_URL).replace(/\/+$/, "");
  const key = config.key?.trim() ?? "";
  return {
    async call(tool: string, body: Body): Promise<Record<string, unknown>> {
      if (key === "") {
        throw new LayerError(
          tool,
          0,
          CodeUnconfigured,
          "MEMENTO_AGENT_KEY is empty: the deployment carries no agent key",
        );
      }
      let response: Response;
      try {
        response = await fetch(`${base}/tools/${tool}`, {
          method: "POST",
          headers: {
            authorization: `Bearer ${key}`,
            "content-type": "application/json",
            accept: "application/json",
          },
          body: JSON.stringify(body),
          signal: AbortSignal.timeout(CALL_TIMEOUT_MS),
        });
      } catch (cause) {
        if (cause instanceof Error && cause.name === "TimeoutError") {
          throw new LayerError(tool, 0, CodeTimeout, `no answer within ${CALL_TIMEOUT_MS} ms`);
        }
        const message = cause instanceof Error ? cause.message : String(cause);
        throw new LayerError(tool, 0, CodeUnreachable, message);
      }
      const text = await response.text();
      let parsed: unknown;
      try {
        parsed = text === "" ? {} : JSON.parse(text);
      } catch {
        throw new LayerError(tool, response.status, CodeBadResponse, text.slice(0, 200));
      }
      if (!response.ok) {
        const code = errorCode(parsed) || `http-${response.status}`;
        const message =
          typeof parsed === "object" && parsed !== null
            ? JSON.stringify((parsed as { error?: unknown }).error ?? parsed).slice(0, 300)
            : text.slice(0, 200);
        throw new LayerError(tool, response.status, code, message);
      }
      if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
        throw new LayerError(tool, response.status, CodeBadResponse, "response is not an object");
      }
      return parsed as Record<string, unknown>;
    },
  };
}

/** RecallInput is `recall_for_purchase`'s request; the layer requires a brand. */
export interface RecallInput {
  shopper_id: string;
  brand_id: string;
  line_name?: string;
  category?: string;
  fit_handle?: string;
  sku?: string;
  size_considered?: string;
  trace_id?: string;
}

/** CorrectionInput is `record_correction`'s request. */
export interface CorrectionInput {
  shopper_id: string;
  text: string;
  contradicts_belief_id?: string;
  trace_id?: string;
}

/** QuestionsInput is `pending_questions`'s request. */
export interface QuestionsInput {
  shopper_id: string;
  limit?: number;
}

/** ReasonInput is `record_reason`'s request. */
export interface ReasonInput {
  shopper_id: string;
  question_id: string;
  answer_text: string;
  trace_id?: string;
}

/** FeedbackInput is `record_feedback`'s request. */
export interface FeedbackInput {
  shopper_id: string;
  custom_id: string;
  sku: string;
  size: string;
  feedback_text: string;
  trace_id?: string;
}

/** ExplainInput is `explain_belief`'s request. */
export interface ExplainInput {
  shopper_id: string;
  belief_id: string;
}

/** FitInput is `product_fit`'s request: a line and size, never a shopper. */
export interface FitInput {
  brand_id: string;
  category: string;
  fit_handle: string;
  size: string;
}

/** PoolingInput is `set_pooling`'s request. */
export interface PoolingInput {
  shopper_id: string;
  pooling: boolean;
  trace_id?: string;
}

/** LedgerInput selects one shopper's ledger rows. */
export interface LedgerInput {
  shopper_id: string;
  include_seeded?: boolean;
}

/** LedgerRowInput is a single ledger row, or one row's evidence. */
export interface LedgerRowInput {
  shopper_id: string;
  /** belief_id narrows the read to one belief; ledger_belief requires it. */
  belief_id?: string;
}

/** The layer the agent's tools call: one method per frozen tool. */
export interface Layer {
  recallForPurchase(input: RecallInput): Promise<Record<string, unknown>>;
  recordCorrection(input: CorrectionInput): Promise<Record<string, unknown>>;
  pendingQuestions(input: QuestionsInput): Promise<Record<string, unknown>>;
  recordReason(input: ReasonInput): Promise<Record<string, unknown>>;
  recordFeedback(input: FeedbackInput): Promise<Record<string, unknown>>;
  explainBelief(input: ExplainInput): Promise<Record<string, unknown>>;
  productFit(input: FitInput): Promise<Record<string, unknown>>;
  setPooling(input: PoolingInput): Promise<Record<string, unknown>>;
  ledgerBeliefs(input: LedgerInput): Promise<Record<string, unknown>>;
  ledgerBelief(input: LedgerRowInput): Promise<Record<string, unknown>>;
  ledgerEvidence(input: LedgerRowInput): Promise<Record<string, unknown>>;
}

/**
 * buildLayer binds a transport to the eleven frozen tools. Each method sends
 * exactly the contract's request fields: the layer rejects anything else.
 */
export function buildLayer(transport: LayerTransport): Layer {
  return {
    recallForPurchase: (input) =>
      transport.call("recall_for_purchase", {
        shopper_id: input.shopper_id,
        brand_id: input.brand_id,
        line_name: input.line_name,
        category: input.category,
        fit_handle: input.fit_handle,
        sku: input.sku,
        size_considered: input.size_considered,
        trace_id: input.trace_id,
      }),
    recordCorrection: (input) =>
      transport.call("record_correction", {
        shopper_id: input.shopper_id,
        text: input.text,
        contradicts_belief_id: input.contradicts_belief_id,
        trace_id: input.trace_id,
      }),
    pendingQuestions: (input) =>
      transport.call("pending_questions", {
        shopper_id: input.shopper_id,
        limit: input.limit,
      }),
    recordReason: (input) =>
      transport.call("record_reason", {
        shopper_id: input.shopper_id,
        question_id: input.question_id,
        answer_text: input.answer_text,
        trace_id: input.trace_id,
      }),
    recordFeedback: (input) =>
      transport.call("record_feedback", {
        shopper_id: input.shopper_id,
        custom_id: input.custom_id,
        sku: input.sku,
        size: input.size,
        feedback_text: input.feedback_text,
        trace_id: input.trace_id,
      }),
    explainBelief: (input) =>
      transport.call("explain_belief", {
        shopper_id: input.shopper_id,
        belief_id: input.belief_id,
      }),
    productFit: (input) =>
      transport.call("product_fit", {
        brand_id: input.brand_id,
        category: input.category,
        fit_handle: input.fit_handle,
        size: input.size,
      }),
    setPooling: (input) =>
      transport.call("set_pooling", {
        shopper_id: input.shopper_id,
        pooling: input.pooling,
        trace_id: input.trace_id,
      }),
    ledgerBeliefs: (input) =>
      transport.call("ledger_beliefs", {
        shopper_id: input.shopper_id,
        include_seeded: input.include_seeded ?? true,
      }),
    ledgerBelief: (input) =>
      transport.call("ledger_belief", {
        shopper_id: input.shopper_id,
        belief_id: input.belief_id,
      }),
    ledgerEvidence: (input) =>
      transport.call("ledger_evidence", {
        shopper_id: input.shopper_id,
        belief_id: input.belief_id,
      }),
  };
}

/**
 * layerFromEnv builds the deployed path's layer from the deployment's own
 * environment: the agent key arrives from the platform's secret store and
 * is never read from a file.
 */
export function layerFromEnv(env: Record<string, string | undefined> = process.env): Layer {
  return buildLayer(
    httpTransport({ url: env.MEMENTO_LAYER_URL, key: env.MEMENTO_AGENT_KEY }),
  );
}
