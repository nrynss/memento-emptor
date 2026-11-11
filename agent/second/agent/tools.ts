/**
 * The second agent's tools: the layer's eleven frozen interface calls.
 *
 * Descriptions are the layer's own frozen words (internal/api/contract:
 * Tools), which is the one shared text the task allows — the agent's
 * instructions are written fresh and carry no correction story. Each tool
 * calls the served layer's edge through an injected connection (see
 * edge.ts); nothing stands in for the layer, so a turn either reads live
 * memory or reports that it could not. No purchase-execution tool exists
 * here on purpose: the agent can advise and ask for approval but cannot
 * move money.
 */

import { createTool } from "@mastra/core/tools";
import { z } from "zod";
import { landingFromEnv } from "./edge.js";

const landing = landingFromEnv();

const traceField = z.string().optional().describe("Host trace id for per-trace review");

export const recallTool = createTool({
  id: "recall_for_purchase",
  description:
    "Recall what applies before recommending. It never refuses: the shopper's beliefs and the pooled product fit come back each with a status label, thin evidence included. Guidance names the precedence step that decided (correction, own_evidence, pool, statement, none); when steps disagree both are returned with the lower marked overridden. When a correction is present, recommend from it.",
  inputSchema: z.object({
    shopper_id: z.string(),
    brand_id: z.string().describe("The brand asked about; Brand X is brand-x-001"),
    line_name: z.string().optional(),
    category: z.string().optional(),
    fit_handle: z.string().optional(),
    sku: z.string().optional(),
    size_considered: z.string().optional(),
    trace_id: traceField,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => landing.recall(input),
});

export const correctionTool = createTool({
  id: "record_correction",
  description:
    "Record the shopper's explicit correction at once. With a belief it takes precedence over it immediately and the belief stays beside the warning; with no belief it is kept as a standing record that contradicts nothing yet and is declared against a matching belief when one appears.",
  inputSchema: z.object({
    shopper_id: z.string(),
    text: z.string().describe("The shopper's verbatim first-person words"),
    contradicts_belief_id: z.string().optional(),
    trace_id: traceField,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => landing.correct(input),
});

export const questionsTool = createTool({
  id: "pending_questions",
  description:
    "List the refunds still missing a reason, oldest first. Ask the shopper about the first one; send the answer to record_reason. An unanswered refund counts toward nothing.",
  inputSchema: z.object({
    shopper_id: z.string(),
    limit: z.number().optional(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => landing.questions(input),
});

export const reasonTool = createTool({
  id: "record_reason",
  description:
    "Record the shopper's answer to a pending question. The layer classifies it; when the answer is vague or confidence is low the question stays open and the response asks again.",
  inputSchema: z.object({
    shopper_id: z.string(),
    question_id: z.string(),
    answer_text: z.string().describe("The shopper's verbatim answer"),
    trace_id: traceField,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => landing.reason(input),
});

export const feedbackTool = createTool({
  id: "record_feedback",
  description:
    "Record explicit fit feedback tied to a specific purchase, joined on the transaction's custom id. Counts toward fit as evidence for the worn size.",
  inputSchema: z.object({
    shopper_id: z.string(),
    custom_id: z.string(),
    sku: z.string(),
    size: z.string(),
    feedback_text: z.string().describe("The shopper's verbatim words about the fit"),
    trace_id: traceField,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => landing.feedback(input),
});

export const explainTool = createTool({
  id: "explain_belief",
  description:
    "Explain why the layer believes something: every piece of evidence with its reason, every correction, and the history in order. Cite this, not the raw transaction history, when justifying a recommendation.",
  inputSchema: z.object({
    shopper_id: z.string(),
    belief_id: z.string(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => landing.explain(input),
});

export const poolTool = createTool({
  id: "product_fit",
  description:
    "Read the pooled product fit for one product line and size: its status, the direction it rates, settled counts by direction, feedback by verdict and reason by category, distinct shoppers by direction, and the seeded split. Carries no shopper identity, no row and no free text.",
  inputSchema: z.object({
    brand_id: z.string(),
    category: z.string(),
    fit_handle: z.string(),
    size: z.string(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => landing.fit(input),
});

export const poolingTool = createTool({
  id: "set_pooling",
  description:
    "Record the shopper's pooling choice. Pooling is opt-out: their classified outcomes count toward the pool unless they say no. Opting out stops new fit rows and removes the rows already written, and the response says how many.",
  inputSchema: z.object({
    shopper_id: z.string(),
    pooling: z.boolean(),
    trace_id: traceField,
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => landing.pooling(input),
});

export const ledgerListTool = createTool({
  id: "ledger_beliefs",
  description:
    "List the ledger's belief rows: every belief with its ledger status, underlying standing, contradiction warning, seeded mark and evidence count.",
  inputSchema: z.object({
    shopper_id: z.string(),
    include_seeded: z.boolean().optional(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => landing.ledger(input),
});

export const ledgerOneTool = createTool({
  id: "ledger_belief",
  description: "Read one ledger belief row in full, with its corrections and its seeded notice.",
  inputSchema: z.object({
    shopper_id: z.string(),
    belief_id: z.string(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => landing.ledgerRow(input),
});

export const ledgerProofTool = createTool({
  id: "ledger_evidence",
  description:
    "List the ledger's evidence rows: outcomes with their reasons, what each counts toward, seeded marks and belief links.",
  inputSchema: z.object({
    shopper_id: z.string(),
    belief_id: z.string().optional(),
  }),
  outputSchema: z.object({}).passthrough(),
  execute: async (input) => landing.ledgerProof(input),
});

export const tools = {
  recall_for_purchase: recallTool,
  record_correction: correctionTool,
  pending_questions: questionsTool,
  record_reason: reasonTool,
  record_feedback: feedbackTool,
  explain_belief: explainTool,
  product_fit: poolTool,
  set_pooling: poolingTool,
  ledger_beliefs: ledgerListTool,
  ledger_belief: ledgerOneTool,
  ledger_evidence: ledgerProofTool,
};
