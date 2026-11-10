/**
 * pilot-baseline: the T5.3 pilot instrument, not a product agent.
 *
 * The deployed reference agent (memento-agent) cannot implement the
 * comparison's arms 1 and 2: its shopper_01 default makes it recall the
 * demo shopper's memory even when the turn names a comparison shopper
 * (proven twice, kept as pilot-a1-broken-discarded-1/2). So the pilot
 * drives this neutral agent instead: the same gateway model
 * (claude-sonnet-4-6, the T3.1 pick), no tools, no memory beyond the user
 * message. Arms differ only in the message they carry, exactly the
 * property the harness proves offline for its prompts.
 *
 * This agent is a measuring instrument. It is deleted after the pilot;
 * T5.5 decides its own driver from the freeze.
 */

import { Agent } from "@mastra/core/agent";
import { Mastra } from "@mastra/core/mastra";
import { Memory } from "@mastra/memory";
import { LibSQLStore } from "@mastra/libsql";
import { Observability } from "@mastra/observability";
import { OtelExporter } from "@mastra/otel-exporter";
import { serve } from "@astropods/adapter-mastra";
import { createOpenAI } from "@ai-sdk/openai";

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
      serviceName: "pilot-baseline",
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

// MODEL_DEFAULT is the deploy-time pick from the gateway options: the T3.1
// model, so per-turn cost is comparable with the reference agent's turns.
const MODELS = [process.env.MODEL_DEFAULT ?? "claude-sonnet-4-6"];

// Neutral on purpose: no tools, no memory, no shopper default. The user
// message carries the whole arm; the reply recommends from it alone.
const INSTRUCTIONS = `You are a shopping assistant taking part in a controlled comparison. You have no tools, no memory of past purchases, and no knowledge of any shopper beyond what the current user message contains.

Answer ONLY from the user message. Recommend one size and give one sentence of justification citing only what the message gave you. Do not invent purchases, reasons, or corrections. Do not ask for a shopper id. Do not ask about approval or purchase: the task ends at the recommendation.

Keep the reply short: one recommendation, its citations, nothing else.`;

const agent = new Agent({
  id: "pilot-baseline",
  name: "Pilot Baseline",
  instructions: INSTRUCTIONS,
  model: ({ requestContext }) => gateway.chat(requestContext.get<"model", string | undefined>("model") ?? MODELS[0]),
  memory,
  defaultOptions: {
    tracingOptions: {
      tags: ["astro", "agent:pilot-baseline"],
      metadata: {
        agent_id: "pilot-baseline",
      },
    },
  },
});

new Mastra({
  agents: {
    "pilot-baseline": agent,
  },
  observability,
});

serve(agent, { models: MODELS });
