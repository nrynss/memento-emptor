/**
 * The agent's tools: the layer's eleven frozen interface calls.
 *
 * Every tool is a call to the served layer's edge through an injected
 * transport (see layer.ts). There is no stand-in behind any of them, so a
 * turn that reads memory reads the live layer or reports that it could not.
 * Tool names and input shapes are the layer's frozen contract; the
 * descriptions keep the layer's own words where they carry the call and add
 * the recall-first guidance this agent needs. No purchase-execution tool
 * exists here on purpose: orders and double-buy protection belong to T3.3
 * (agent/orders/), so this agent can recommend and ask approval but cannot
 * move money.
 */

import { createTool } from "@mastra/core/tools";
import { z } from "zod";
import { layerFromEnv } from "./layer.js";

const layer = layerFromEnv();

const optionalTrace = z.string().optional().describe("Host trace id for per-trace review");

export const recallTool = createTool({
  id: "recall_for_purchase",
  description:
    "Recall what applies before recommending: the shopper's beliefs, the pooled product fit, and any correction, each with its status label and precedence step. Call first on every turn that may end in a recommendation; recommend from guidance.basis and state confidence from its label. It never refuses: thin evidence comes back labelled thin, and basis none says plainly no memory applies.",
  inputSchema: z.object({
    shopper_id: z.string(),
    brand_id: z.string().describe("The brand asked about; Brand X is brand-x-001"),
    line_name: z.string().optional(),
    category: z.string().optional(),
    fit_handle: z.string().optional(),
    sku: z.string().optional(),
    size_considered: z.string().optional(),
    trace_id: optionalTrace,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => layer.recallForPurchase(input),
});

export const recordCorrectionTool = createTool({
  id: "record_correction",
  description:
    "Record the shopper's explicit correction at once. The correction takes precedence immediately; the belief stays and is shown beside it. Pass the contradicted belief id when recall served a belief; omit it when recall served no belief, and the correction stands on its own instead of travelling in chat text only.",
  inputSchema: z.object({
    shopper_id: z.string(),
    text: z.string().describe("The shopper's verbatim first-person words"),
    contradicts_belief_id: z.string().optional(),
    trace_id: optionalTrace,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => layer.recordCorrection(input),
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
  execute: async (input) => layer.pendingQuestions(input),
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
  execute: async (input) => layer.recordReason(input),
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
  execute: async (input) => layer.recordFeedback(input),
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
  execute: async (input) => layer.explainBelief(input),
});

export const ledgerBeliefsTool = createTool({
  id: "ledger_beliefs",
  description:
    "List the ledger's belief rows: every belief with its ledger status, underlying standing, contradiction warning, seeded mark and evidence count.",
  inputSchema: z.object({
    shopper_id: z.string(),
    include_seeded: z.boolean().optional(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => layer.ledgerBeliefs(input),
});

export const ledgerBeliefTool = createTool({
  id: "ledger_belief",
  description: "Read one ledger belief row in full, with its corrections and its seeded notice.",
  inputSchema: z.object({
    shopper_id: z.string(),
    belief_id: z.string(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => layer.ledgerBelief(input),
});

export const ledgerEvidenceTool = createTool({
  id: "ledger_evidence",
  description:
    "List the ledger's evidence rows: outcomes with their reasons, what each counts toward, seeded marks and belief links.",
  inputSchema: z.object({
    shopper_id: z.string(),
    belief_id: z.string().optional(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => layer.ledgerEvidence(input),
});

export const productFitTool = createTool({
  id: "product_fit",
  description:
    "Read the pooled product fit for one line and size: its status, the direction it rates, and the settled counts behind it with their seeded split. No shopper identity, no row, no free text. Cite these counts when the recommendation rests on the pool.",
  inputSchema: z.object({
    brand_id: z.string(),
    category: z.string(),
    fit_handle: z.string(),
    size: z.string(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => layer.productFit(input),
});

export const setPoolingTool = createTool({
  id: "set_pooling",
  description:
    "Record the shopper's pooling choice: pooling is opt-out, so their classified outcomes count toward the pool unless they say no. Opting out removes the rows already written and the response says how many.",
  inputSchema: z.object({
    shopper_id: z.string(),
    pooling: z.boolean(),
    trace_id: optionalTrace,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => layer.setPooling(input),
});

export const tools = {
  recall_for_purchase: recallTool,
  record_correction: recordCorrectionTool,
  pending_questions: pendingQuestionsTool,
  record_reason: recordReasonTool,
  record_feedback: recordFeedbackTool,
  explain_belief: explainBeliefTool,
  product_fit: productFitTool,
  set_pooling: setPoolingTool,
  ledger_beliefs: ledgerBeliefsTool,
  ledger_belief: ledgerBeliefTool,
  ledger_evidence: ledgerEvidenceTool,
};
