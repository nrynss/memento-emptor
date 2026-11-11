/**
 * What POST /api/chat answers, per outcome: the status and the body (T9.14).
 *
 * Kept free of imports and of server construction, like relay.ts, so the
 * mapping is tested without a gateway, a model call or a listening port
 * (relay.test.ts). server/index.ts shapes every /api/chat answer here, so a
 * refusal's status is pinned in this file and nowhere else: a malformed body
 * is 400, a spent budget is 429, and a failed model turn is 502.
 */

/** The refusal a body that is not JSON gets; the relay never sees it. */
export const MALFORMED_JSON_ERROR = "body must be JSON";

/** The answer text a model turn that threw gets. */
export const RELAY_FAILURE_ERROR = "model turn failed; try one short message again";

/** Status for a body the relay refuses: not JSON, or not the accepted shape. */
export const STATUS_INVALID = 400;

/** Status for a request over the anonymous turn budget. */
export const STATUS_EXHAUSTED = 429;

/** Status for a model turn the gateway failed after the relay accepted it. */
export const STATUS_RELAY_FAILURE = 502;

/** The JSON body of one /api/chat answer. */
export interface ChatReplyBody {
  /** What the page shows the shopper. */
  error: string;
  /** The logged turn id, present only on a failed model turn. */
  turn_id?: string;
}

/** One /api/chat answer: the status, and the body index.ts serialises. */
export interface ChatReply {
  status: number;
  body: ChatReplyBody;
}

/** The answer for a request body that is not JSON: 400, no turn charged. */
export function malformedJsonReply(): ChatReply {
  return { status: STATUS_INVALID, body: { error: MALFORMED_JSON_ERROR } };
}

/**
 * The answer for a turn the relay did not accept: 400 when the body's shape
 * is refused, 429 when the budget is spent. The text is the decision's own.
 */
export function refusalReply(refusal: { kind: "invalid" | "exhausted"; error: string }): ChatReply {
  return {
    status: refusal.kind === "invalid" ? STATUS_INVALID : STATUS_EXHAUSTED,
    body: { error: refusal.error },
  };
}

/** The answer for a model turn that threw: 502, carrying the logged turn id. */
export function relayFailureReply(turnId: string): ChatReply {
  return { status: STATUS_RELAY_FAILURE, body: { error: RELAY_FAILURE_ERROR, turn_id: turnId } };
}
