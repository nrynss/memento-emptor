/**
 * The shopper page agent's tools: recall before recommending, and the
 * shopper's correction recorded at once. Both are served from the
 * recorded demo fixture (see demo-state.ts); tool names and shapes match
 * the frozen layer contract, so the live swap later changes no tool the
 * agent reads. This task owns no purchase-execution tool on purpose:
 * approval is an explicit "yes" in chat, and the agent says plainly the
 * purchase step is not wired on this page instead of claiming it happened.
 */

import { createTool } from "@mastra/core/tools";
import { z } from "zod";
import { DEMO_SHOPPER, SIZING_BELIEF_ID, recallForPurchase, recordCorrection } from "./demo-state.js";

const optionalTrace = z.string().optional().describe("Host trace id for per-trace review");

export const recallTool = createTool({
  id: "recall_for_purchase",
  description:
    "Recall what applies before recommending: the sizing belief with its evidence and seeded marks, plus the shopper correction that overrides it. Call first on every turn that may end in a recommendation; when a correction is present, recommend from it.",
  inputSchema: z.object({
    shopper_id: z.string(),
    brand_id: z.string().optional(),
    line_name: z.string().optional(),
    trace_id: optionalTrace,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => recallForPurchase(input.shopper_id, input.trace_id),
});

export const recordCorrectionTool = createTool({
  id: "record_correction",
  description:
    "Record the shopper's explicit correction at once. The correction takes precedence over the contradicted belief immediately; the belief stays and is shown beside it. The contradicted belief id is required.",
  inputSchema: z.object({
    shopper_id: z.string(),
    text: z.string().describe("The shopper's verbatim first-person words"),
    contradicts_belief_id: z.string(),
    trace_id: optionalTrace,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) =>
    recordCorrection(input.shopper_id, input.text, input.contradicts_belief_id, input.trace_id),
});

export const tools = {
  recall_for_purchase: recallTool,
  record_correction: recordCorrectionTool,
};

export const DEMO_DEFAULTS = { shopperId: DEMO_SHOPPER, beliefId: SIZING_BELIEF_ID };
