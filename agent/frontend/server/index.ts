/**
 * memento-shopper-frontend: the public shopper page and its chat relay.
 *
 * One container does three jobs (F2 composition):
 *  1. Serves the no-login shopper page + /health on the injected PORT
 *     (80 in production) with no auth guard — the frontend hostname.
 *  2. Relays the page's chat box to the in-process shopping agent
 *     (POST /api/chat), so anonymous turns drive the recall → recommend →
 *     correct → approve flow and land in the platform trace list with
 *     tool spans.
 *  3. Registers the messaging interface via serve(), so the same agent
 *     answers the chat interface and the same traces prove the turn.
 *
 * Anonymous spend guard: model turns are capped per container lifetime
 * (MAX_TURNS). Validation failures cost nothing and never increment the
 * counter. Both guards, the body validator and the turn budget, live in
 * relay.ts so they are tested without a gateway or a listening port
 * (relay.test.ts); this file owns only the HTTP plumbing above them. State
 * beyond the cap message is memory-only; the production filesystem is
 * read-only, so nothing here writes to disk.
 */

import { Agent } from "@mastra/core/agent";
import { Mastra } from "@mastra/core/mastra";
import { Observability } from "@mastra/observability";
import { OtelExporter } from "@mastra/otel-exporter";
import { serve } from "@astropods/adapter-mastra";
import { createOpenAI } from "@ai-sdk/openai";
import { DEMO_DEFAULTS, tools } from "./tools.js";
import { createTurnBudget, decideChatTurn, MAX_TURNS } from "./relay.js";

const AGENT_NAME = "memento-shopper";

const turnBudget = createTurnBudget(MAX_TURNS);

const gateway = createOpenAI({
  apiKey: process.env.ASTRO_GATEWAY_API_KEY,
  baseURL: process.env.ASTRO_GATEWAY_URL ? `${process.env.ASTRO_GATEWAY_URL}/v1` : undefined,
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
      serviceName: AGENT_NAME,
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
const MODELS = [process.env.MODEL_DEFAULT ?? "claude-sonnet-4-6"];

const INSTRUCTIONS = `You are the Memento Emptor shopper page assistant. You remember what happened to the shopper's money; you never move money. Your memory here is the recorded demo fixture (seeded marks shown), not live layer data.

Turn pattern, every time:
1. RECALL FIRST. Before any turn that may end in a recommendation, call recall_for_purchase. The demo shopper is "${DEMO_DEFAULTS.shopperId}": pass that shopper_id unless the user names another id, and never ask the user for a shopper id.
2. Recommend from the guidance. When overriding_correction is present, recommend the correction's size, and say the belief still stands beside it: cite the belief id (${DEMO_DEFAULTS.beliefId}), its 3 fit returns, and the seeded mark on each.
3. Pending refunds: when pending_count is above zero, the recall carries the first pending question — ask it in one short question.
4. Corrections: when the shopper corrects a size, call record_correction at once with the contradicted belief id, and the very next recommendation follows the correction.
5. Approval: before any purchase, ask the shopper to reply "yes". You own no purchase tool, so after a "yes" say plainly the purchase step is not wired on this page instead of claiming it happened. Keep replies short: one recommendation, its citations, one question at most.`;

const agent = new Agent({
  id: AGENT_NAME,
  name: "Memento Shopper",
  instructions: INSTRUCTIONS,
  model: ({ requestContext }) => gateway.chat(requestContext.get<"model", string | undefined>("model") ?? MODELS[0]),
  tools,
  defaultOptions: {
    tracingOptions: {
      tags: ["astro", `agent:${AGENT_NAME}`],
      metadata: {
        agent_id: AGENT_NAME,
      },
    },
  },
});

new Mastra({
  agents: {
    [AGENT_NAME]: agent,
  },
  observability,
});

const pageFile = Bun.file(new URL("../public/index.html", import.meta.url));

function json(status: number, value: unknown): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "content-type": "application/json; charset=utf-8" },
  });
}

const server = Bun.serve({
  port: Number(process.env.PORT ?? 80),
  hostname: "0.0.0.0",
  routes: {
    "/": async () => new Response(pageFile, { headers: { "content-type": "text/html; charset=utf-8" } }),
    "/health": () => json(200, { ok: true, agent: AGENT_NAME }),
    "/api/chat": {
      POST: async (req) => {
        let body: unknown;
        try {
          body = await req.json();
        } catch {
          return json(400, { error: "body must be JSON" });
        }
        const decision = decideChatTurn(body, turnBudget);
        if (decision.kind === "invalid") {
          return json(400, { error: decision.error });
        }
        if (decision.kind === "exhausted") {
          return json(429, { error: decision.error });
        }
        const turnId = `turn_${crypto.randomUUID().slice(0, 8)}`;
        try {
          const result = await agent.generate(decision.messages);
          return json(200, { reply: result.text, turn_id: turnId });
        } catch (err) {
          console.error(`chat turn ${turnId} failed:`, err instanceof Error ? err.message : err);
          return json(502, { error: "model turn failed; try one short message again", turn_id: turnId });
        }
      },
    },
  },
  fetch: () => new Response("not found", { status: 404 }),
});

// eslint-disable-next-line no-console
console.log(`${AGENT_NAME} frontend on :${server.port}`);

// Messaging registration (the chat interface + trace pipeline). Skipped
// only for local smoke tests (SKIP_SERVE=1): deployed, the page is already
// listening above, and a serve() failure exits so the platform restarts
// the container instead of running half-composed.
if (process.env.SKIP_SERVE !== "1") {
  serve(agent, { models: MODELS });
}
