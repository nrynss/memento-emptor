/**
 * The agent's tools: the eight layer calls an agent key reaches.
 *
 * Every tool is a call to the served layer's edge through an injected
 * transport (see layer.ts). There is no stand-in behind any of them, so a
 * turn that reads memory reads the live layer or reports that it could not.
 * Tool names and input shapes are the layer's frozen contract; the
 * descriptions keep the layer's own words where they carry the call and add
 * the recall-first guidance this agent needs. The three ledger reads
 * (ledger_beliefs, ledger_belief, ledger_evidence) are shopper-only in the
 * edge's access table, so this agent holds no binding for them: a tool the
 * agent key cannot call is a tool the model must not be offered. No
 * purchase-execution tool exists here on purpose: orders and double-buy
 * protection belong to T3.3 (agent/orders/), so this agent can recommend and
 * ask approval but cannot move money.
 *
 * createTools takes the layer, which is how the offline check binds these
 * same ids, schemas and execute functions to a fake transport; the deployed
 * `tools` is that factory over the layer built from the environment.
 */

import { createTool } from "@mastra/core/tools";
import { z } from "zod";
import { layerFromEnv, type Layer } from "./layer.js";

const optionalTrace = z.string().optional().describe("Host trace id for per-trace review");

/** createTools binds the agent's tools to one layer. */
export function createTools(layer: Layer) {
  const recallTool = createTool({
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

  const recordCorrectionTool = createTool({
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

  const pendingQuestionsTool = createTool({
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

  const recordReasonTool = createTool({
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

  const recordFeedbackTool = createTool({
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

  const explainBeliefTool = createTool({
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

  const productFitTool = createTool({
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

  const setPoolingTool = createTool({
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

  return {
    recall_for_purchase: recallTool,
    record_correction: recordCorrectionTool,
    pending_questions: pendingQuestionsTool,
    record_reason: recordReasonTool,
    record_feedback: recordFeedbackTool,
    explain_belief: explainBeliefTool,
    product_fit: productFitTool,
    set_pooling: setPoolingTool,
  };
}

/** tools is the deployed set: the agent's tools over the served layer's edge. */
export const tools = createTools(layerFromEnv());
