/**
 * The agent's tools: the layer's nine frozen interface calls plus one
 * PayPal-flavoured order read. All fake-backed (see memory.ts): tool names
 * and shapes match the frozen contract, so the live swap changes no tool.
 */

import { createTool } from "@mastra/core/tools";
import { z } from "zod";
import {
  explainBelief,
  ledgerBelief,
  ledgerBeliefs,
  ledgerEvidence,
  lookupReturn,
  pendingQuestions,
  recallForPurchase,
  recordCorrection,
  recordFeedback,
  recordReason,
} from "./memory.js";

const optionalTrace = z.string().optional().describe("Host trace id for per-trace review");

export const recallTool = createTool({
  id: "recall_for_purchase",
  description:
    "Recall what applies before recommending: the beliefs for this shopper and product, each with status and evidence, plus any shopper correction that overrides them. Call first on every turn that may end in a recommendation; when a correction is present, recommend from it.",
  inputSchema: z.object({
    shopper_id: z.string(),
    brand_id: z.string().optional(),
    line_name: z.string().optional(),
    sku: z.string().optional(),
    size_considered: z.string().optional(),
    trace_id: optionalTrace,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async ({ context }) => recallForPurchase(context.shopper_id, context.trace_id),
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
  execute: async ({ context }) =>
    recordCorrection(context.shopper_id, context.text, context.contradicts_belief_id, context.trace_id),
});

export const pendingQuestionsTool = createTool({
  id: "pending_questions",
  description:
    "List the refunds still missing a reason, oldest first. Ask the shopper about the first one in chat. An unanswered refund counts toward nothing.",
  inputSchema: z.object({
    shopper_id: z.string(),
    limit: z.number().optional(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async ({ context }) => pendingQuestions(context.shopper_id),
});

export const recordReasonTool = createTool({
  id: "record_reason",
  description:
    "Record the shopper's answer to a pending question. When the answer is vague the question stays open and the response asks again.",
  inputSchema: z.object({
    shopper_id: z.string(),
    question_id: z.string(),
    answer_text: z.string().describe("The shopper's verbatim answer"),
    trace_id: optionalTrace,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async ({ context }) => {
    void context.shopper_id;
    return recordReason(context.question_id, context.answer_text, context.trace_id);
  },
});

export const recordFeedbackTool = createTool({
  id: "record_feedback",
  description:
    "Record explicit fit feedback tied to a specific purchase, joined on the transaction's custom id. Counts toward fit as evidence for the worn size.",
  inputSchema: z.object({
    shopper_id: z.string(),
    custom_id: z.string(),
    sku: z.string(),
    size: z.string(),
    feedback_text: z.string().describe("The shopper's verbatim words about the fit"),
    trace_id: optionalTrace,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async ({ context }) => {
    void context.shopper_id;
    return recordFeedback(context.custom_id, context.sku, context.size, context.feedback_text, context.trace_id);
  },
});

export const explainBeliefTool = createTool({
  id: "explain_belief",
  description:
    "Explain why the layer believes something: every piece of evidence with its reason, every correction, and the history in order. Cite this, not the raw transaction history, when justifying a recommendation.",
  inputSchema: z.object({
    shopper_id: z.string(),
    belief_id: z.string(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async ({ context }) => {
    void context.shopper_id;
    return explainBelief(context.belief_id) ?? { error: `unknown belief ${context.belief_id}` };
  },
});

export const ledgerBeliefsTool = createTool({
  id: "ledger_beliefs",
  description: "List the ledger's belief rows: every belief with its ledger status, underlying standing, contradiction warning, seeded mark and evidence count.",
  inputSchema: z.object({
    shopper_id: z.string(),
    include_seeded: z.boolean().optional(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async ({ context }) => ledgerBeliefs(context.shopper_id),
});

export const ledgerBeliefTool = createTool({
  id: "ledger_belief",
  description: "Read one ledger belief row in full, with its corrections and its seeded notice.",
  inputSchema: z.object({
    shopper_id: z.string(),
    belief_id: z.string(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async ({ context }) => {
    void context.shopper_id;
    return ledgerBelief(context.belief_id) ?? { error: `unknown belief ${context.belief_id}` };
  },
});

export const ledgerEvidenceTool = createTool({
  id: "ledger_evidence",
  description: "List the ledger's evidence rows: outcomes with their reasons, what each counts toward, seeded marks and belief links.",
  inputSchema: z.object({
    shopper_id: z.string(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async ({ context }) => ledgerEvidence(context.shopper_id),
});

export const lookupReturnTool = createTool({
  id: "lookup_return",
  description:
    "Look up a past order's return: item plus reason. Read-only; backed by recorded sandbox deliveries until T3.3 wires live PayPal reads.",
  inputSchema: z.object({
    order_id: z.string(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async ({ context }) => lookupReturn(context.order_id),
});

export const tools = {
  recall_for_purchase: recallTool,
  record_correction: recordCorrectionTool,
  pending_questions: pendingQuestionsTool,
  record_reason: recordReasonTool,
  record_feedback: recordFeedbackTool,
  explain_belief: explainBeliefTool,
  ledger_beliefs: ledgerBeliefsTool,
  ledger_belief: ledgerBeliefTool,
  ledger_evidence: ledgerEvidenceTool,
  lookup_return: lookupReturnTool,
};
