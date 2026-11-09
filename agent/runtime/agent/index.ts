/**
 * memento-agent: the Memento Emptor reference shopping agent.
 *
 * Mastra stack (chosen over LangChain): the adapter auto-wires OpenTelemetry
 * through serve(), so "the agent recalled first" is evidenced by ordered
 * tool spans on the platform collector; and the T0.8 probe already proved
 * this exact stack's trace shape live.
 *
 * Model: claude-sonnet-4-6 via the gateway provider (no personal key; the
 * platform injects ASTRO_GATEWAY_URL/KEY, deploy picks MODEL_DEFAULT). The
 * same model runs in all three comparison arms.
 *
 * Standing configuration: the memory tools are fake-backed (see memory.ts)
 * because the layer is not served yet. No purchase-execution tool exists
 * here on purpose: orders and double-buy protection belong to T3.3
 * (agent/orders/), so this agent can recommend and ask approval but cannot
 * move money.
 */

import { Agent } from "@mastra/core/agent";
import { Mastra } from "@mastra/core/mastra";
import { Memory } from "@mastra/memory";
import { LibSQLStore } from "@mastra/libsql";
import { Observability } from "@mastra/observability";
import { OtelExporter } from "@mastra/otel-exporter";
import { serve } from "@astropods/adapter-mastra";
import { createOpenAI } from "@ai-sdk/openai";
import { tools } from "./tools.js";

const gateway = createOpenAI({
  apiKey: process.env.ASTRO_GATEWAY_API_KEY,
  baseURL: `${process.env.ASTRO_GATEWAY_URL}/v1`,
});

const memory = new Memory({
  storage: new LibSQLStore({
    id: "memory",
    url: ":memory:",
  }),
});

function resolveOtlpTracesEndpoint(): string {
  const raw = process.env.OTEL_EXPORTER_OTLP_ENDPOINT || "http://localhost:4318";
  try {
    const url = new URL(raw);
    if (!url.pathname || url.pathname === "/") {
      url.pathname = "/v1/traces";
    }
    return url.toString();
  } catch {
    return `${raw.replace(/\/+$/, "")}/v1/traces`;
  }
}

const observability = new Observability({
  configs: {
    otel: {
      serviceName: "memento-agent",
      exporters: [
        new OtelExporter({
          provider: {
            custom: {
              endpoint: resolveOtlpTracesEndpoint(),
              protocol: "http/protobuf",
            },
          },
        }),
      ],
    },
  },
});

// MODEL_DEFAULT is the deploy-time pick from the gateway options.
// T3.1 decision, used in all three comparison arms: claude-sonnet-4-6.
const MODELS = [process.env.MODEL_DEFAULT ?? "claude-sonnet-4-6"];

const INSTRUCTIONS = `You are the Memento Emptor reference shopping agent. You remember what happened to the shopper's money; you never move money.

Turn pattern, every time:
1. RECALL FIRST. Before any turn that may end in a recommendation, call recall_for_purchase. Never recommend from history or guesswork. The demo shopper is shopper_01: pass shopper_id "shopper_01" unless the user names another id, and never ask the user for a shopper id. If a tool call errors, retry it once with shopper_01 before saying anything about the error.
2. Recommend from the guidance. When overriding_correction is present, recommend the correction's size, and say the belief still stands beside it: cite the belief id, its 3 fit returns, and the seeded mark on each.
3. Pending refunds: when pending_count is above zero, call pending_questions and ask the shopper about the first one in one short question.
4. Corrections: when the shopper corrects a size, call record_correction at once with the contradicted belief id, and the very next recommendation follows the correction.
5. Reasons and feedback: send answers to record_reason and fit notes to record_feedback with the purchase custom id.
6. When challenged ("why this size?"), call explain_belief and cite it, never the raw transaction history.

Approval: before any purchase, ask the shopper to reply "yes". You own no purchase tool, so after a "yes" say plainly the purchase step is not wired yet instead of claiming it happened. Keep replies short: one recommendation, its citations, one question at most.`;

const agent = new Agent({
  id: "memento-agent",
  name: "Memento Agent",
  instructions: INSTRUCTIONS,
  // .chat() uses Chat Completions: the gateway's Responses API rejects Claude tool results.
  model: ({ requestContext }) => gateway.chat(requestContext.get<"model", string | undefined>("model") ?? MODELS[0]),
  memory,
  tools,
  defaultOptions: {
    tracingOptions: {
      tags: ["astro", "agent:memento-agent"],
      metadata: {
        agent_id: "memento-agent",
      },
    },
  },
});

new Mastra({
  agents: {
    "memento-agent": agent,
  },
  observability,
});

serve(agent, { models: MODELS });
