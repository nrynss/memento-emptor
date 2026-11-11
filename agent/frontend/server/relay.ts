/**
 * The chat relay's two guards: the request shape a turn must carry, and the
 * anonymous turn budget that bounds model spend per container lifetime.
 *
 * Kept free of imports and of server construction so both guards can be
 * tested without a gateway credential, a model call or a listening port
 * (see relay.test.ts). server/index.ts owns the HTTP plumbing and calls
 * these; nothing here parses a request body or formats a Response.
 */

/** Most messages one /api/chat request may carry, user and assistant together. */
export const MAX_REQUEST_MESSAGES = 20;

/** Longest single message, in characters, the relay forwards to the model. */
export const MAX_MESSAGE_CHARS = 1000;

/** Demo model turns per container lifetime; bounds anonymous spend. */
export const MAX_TURNS = 30;

/** The cap's refusal text: the page owner resets the budget on redeploy. */
export const TURNS_EXHAUSTED_ERROR =
  "demo turn budget exhausted on this deployment; the page owner resets it on redeploy";

/**
 * Message shapes accepted by agent.generate(): plain-text user turns and
 * text-part assistant turns (AI SDK v5 model messages).
 */
export type RelayMessage =
  | { role: "user"; content: string }
  | { role: "assistant"; content: [{ type: "text"; text: string }] };

/** A validated request body, or the reason it was refused. */
export type ValidationResult =
  | { ok: true; messages: RelayMessage[] }
  | { ok: false; error: string };

/** The relay's decision for one POST /api/chat body. */
export type ChatDecision =
  | { kind: "invalid"; error: string }
  | { kind: "exhausted"; error: string }
  | { kind: "turn"; messages: RelayMessage[] };

/** A turn allowance: one model turn is taken per accepted request. */
export interface TurnBudget {
  /**
   * Consumes one turn and reports whether a model call may run. Returns
   * false, without consuming, once the allowance is spent.
   */
  take(): boolean;
  /** Turns consumed so far. */
  used(): number;
  /** Turns still available. */
  remaining(): number;
}

/** Creates a budget allowing `max` model turns (MAX_TURNS by default). */
export function createTurnBudget(max: number = MAX_TURNS): TurnBudget {
  let used = 0;
  return {
    take(): boolean {
      if (used >= max) return false;
      used += 1;
      return true;
    },
    used(): number {
      return used;
    },
    remaining(): number {
      return max - used;
    },
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * Validates a POST /api/chat body: 1..MAX_REQUEST_MESSAGES messages, each
 * user or assistant with 1..MAX_MESSAGE_CHARS of text, ending with a user
 * turn. Accepted text is trimmed, and assistant turns are wrapped as text
 * parts for agent.generate().
 */
export function validateMessages(body: unknown): ValidationResult {
  if (!isRecord(body) || !("messages" in body)) {
    return { ok: false, error: "body must be {messages: [{role, content}]}" };
  }
  const { messages } = body;
  if (!Array.isArray(messages) || messages.length === 0 || messages.length > MAX_REQUEST_MESSAGES) {
    return {
      ok: false,
      error: `messages must be a non-empty array of at most ${MAX_REQUEST_MESSAGES} turns`,
    };
  }
  const clean: RelayMessage[] = [];
  for (const m of messages) {
    if (!isRecord(m)) {
      return { ok: false, error: "each message must be {role, content}" };
    }
    const { role, content } = m;
    if ((role !== "user" && role !== "assistant") || typeof content !== "string") {
      return { ok: false, error: "role must be user|assistant and content a string" };
    }
    const text = content.trim();
    if (text.length === 0 || text.length > MAX_MESSAGE_CHARS) {
      return { ok: false, error: `content must be 1..${MAX_MESSAGE_CHARS} characters` };
    }
    clean.push(role === "user" ? { role, content: text } : { role, content: [{ type: "text", text }] });
  }
  const last = clean[clean.length - 1];
  if (last.role !== "user") {
    return { ok: false, error: "the last message must be from the user" };
  }
  return { ok: true, messages: clean };
}

/**
 * Decides one chat turn in order: a malformed body is refused before the
 * budget is touched, a spent budget is refused before any model call, and
 * only an accepted turn consumes one.
 */
export function decideChatTurn(body: unknown, budget: TurnBudget): ChatDecision {
  const parsed = validateMessages(body);
  if (!parsed.ok) {
    return { kind: "invalid", error: parsed.error };
  }
  if (!budget.take()) {
    return { kind: "exhausted", error: TURNS_EXHAUSTED_ERROR };
  }
  return { kind: "turn", messages: parsed.messages };
}
