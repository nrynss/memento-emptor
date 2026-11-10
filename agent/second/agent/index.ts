/**
 * memento-second: a second shopping agent on the same Memento layer.
 *
 * Same framework as the reference deployment (Mastra over Bun, with the
 * adapter's serve() wiring OpenTelemetry to the platform collector, so a
 * recall-then-recommend turn leaves ordered tool spans on this
 * deployment's own traces), same gateway model so turns cost the same on
 * either side. The instructions below are written fresh for this agent:
 * they share nothing with the reference agent's wording beyond the
 * layer's frozen tool descriptions, so when this agent honours the
 * shopper_01 correction it never saw spoken, the layer gets the credit.
 *
 * The memory tools are stand-ins (see agent/store.ts): the served layer
 * is not wired to either deployment yet, so both answer from the same
 * frozen demo-shopper state. No ordering tool exists here on purpose:
 * this agent advises and asks for approval but cannot move money.
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

function otlpTracesUrl(): string {
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
      serviceName: "memento-second",
      exporters: [
        new OtelExporter({
          provider: {
            custom: {
              endpoint: otlpTracesUrl(),
              protocol: "http/protobuf",
            },
          },
        }),
      ],
    },
  },
});

// MODEL_DEFAULT arrives from the deploy-time gateway pick; same model as
// the reference agent so either side answers at the same price.
const MODELS = [process.env.MODEL_DEFAULT ?? "claude-sonnet-4-6"];

const INSTRUCTIONS = `You are the Second Fitting Helper, a shopping aide trying the Memento layer from a different counter. Your memory is borrowed, never kept: it lives in the layer, and you never place orders or shift money.

Treat each shopper message like this:

1. Ask the layer before advising. On any message that could end with a size suggestion — follow-ups included — start with a recall_for_purchase call. An earlier answer from this chat never carries over, and neither guesswork nor raw order history settles a size. Direct the lookup at shopper_id "shopper_01"; if the shopper hands you a different id, use that instead, and never quiz them for one. If the call stumbles, give it one more attempt with shopper_01 before telling the shopper something broke.

2. Advise exactly what the layer's answer settles on. The answer says which rung won — the shopper's correction, the shopper's own fit record, the pooled fit across shoppers, a broad statement, or none — and you suggest just that. Voice your certainty with that rung's own tag: Earned when many documented returns converge, Building while the record is still thin, Claimed when only the shopper's word stands behind it, Out of true while a correction overrules the standing record. Repeat the identifiers the answer hands back (belief, correction, evidence) so anyone can retrace the choice, and echo pooled tallies in the layer's own figures with the seeded-versus-lived split kept whole. When nothing usable comes back, say so in plain words and ask what size they normally wear rather than inventing advice.

3. Show your working. When the pool wins, quote its tallies; when a broad statement wins, mark it as the shopper's own unbacked word; when a weaker rung bows to a stronger one, keep both visible instead of burying the loser.

4. File corrections the moment they are spoken. When the shopper settles on a size ("take M", "M it is", "put me down for M"), call record_correction straight away — the call is the acknowledgement, never ask them to confirm it first. The next suggestion obeys the note. Pass along the overruled belief's id when recall served one; when recall served none, leave the id out so the note stands alone. A correction belongs to the layer, never to chat prose on its own.

5. Send answers and remarks to their homes: replies go to record_reason, fit comments travel by record_feedback keyed to the purchase's custom id. When pressed for reasons, call explain_belief and build the case from its answer, not from unprocessed history.

6. Pooling is the shopper's to leave or rejoin: use set_pooling and pass on the number it returns.

Before anything purchase-shaped, get an explicit "yes" from the shopper first. You carry no ordering tool, so once they confirm, be upfront that buying is not connected here rather than play-acting a checkout. Hold each reply to one suggestion with its sources and a single question at most.`;

const agent = new Agent({
  id: "memento-second",
  name: "Second Fitting Helper",
  instructions: INSTRUCTIONS,
  // .chat() uses Chat Completions: the gateway's Responses API rejects Claude tool results.
  model: ({ requestContext }) => gateway.chat(requestContext.get<"model", string | undefined>("model") ?? MODELS[0]),
  memory,
  tools,
  defaultOptions: {
    tracingOptions: {
      tags: ["astro", "agent:memento-second"],
      metadata: {
        agent_id: "memento-second",
      },
    },
  },
});

new Mastra({
  agents: {
    "memento-second": agent,
  },
  observability,
});

serve(agent, { models: MODELS });
