# memento-agent (T3.1; pivot behaviour T9.4; served layer T9.8)

The Memento Emptor reference shopping agent as a deployed Astropods service.

- Stack: TypeScript/Bun + Mastra. Chosen over LangChain because the adapter
  auto-wires OpenTelemetry through `serve()`, so "the agent recalled first"
  is evidenced by ordered tool spans on the platform collector, and the T0.8
  probe already proved this stack's trace shape live.
- Model: `claude-sonnet-4-6` via the gateway provider (no personal key; the
  platform injects `ASTRO_GATEWAY_URL`/`KEY`, deploy picks `MODEL_DEFAULT`).
  The same model runs in all three comparison arms.
- Tools: the layer's eleven frozen interface calls, each one an HTTP call to
  the served layer's edge (`agent/layer.ts`): `POST /tools/<tool>` with the
  deployment's agent key in `Authorization: Bearer` and the shopper id in
  the body. Tool names and input shapes are the layer's frozen contract;
  the descriptions keep the layer's own words plus this agent's
  recall-first guidance.
- Precedence (T9.4): the agent recommends from `guidance.basis` and states
  confidence from the label (Earned, Building, Claimed, Out of true). It
  never refuses when recall returns evidence; only `none` says plainly no
  memory applies. A pool answer cites counts with their seeded split; a
  statement answer says it is Claimed words, not evidence. A correction
  with no belief behind it is recorded with no belief id and stands on its
  own in the layer, deciding again on the next turn.
- Money: this task owns no purchase-execution tool on purpose. Orders and
  double-buy protection belong to T3.3 (`agent/orders/`). The agent asks for
  an explicit "yes" before any purchase and says plainly the purchase step
  is not wired yet instead of claiming it happened.

## The served layer

Every memory the agent reads or writes is the live layer: no stand-in state
exists in the deployed path, so a demo turn either reaches the layer or
reports that it could not. Two values configure it, both from the
deployment's environment and never from a file:

- `MEMENTO_AGENT_KEY` — the agent key the layer's edge authenticates. It
  lives in the platform secret store and arrives by reference
  (`--var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY`). Without it every tool call
  fails as `unconfigured` rather than answering from anywhere else.
- `MEMENTO_LAYER_URL` — the edge's base, defaulting to the hosted layer.

`agent/layer.ts` builds the call, `agent/tools.ts` binds the eleven tools to
it, and `agent/offline-check.ts` swaps the transport for the contract's own
frozen examples, which is how the recall-first contract and the request
shape are proved without a deployment.

## Run

```sh
cd agent/runtime
bun agent/offline-check.ts   # contract examples through the same tool code, no model, no network
ast spec validate            # spec check, local
ast push -V private -y       # build + register the blueprint
ast agent redeploy --id <deployment-id> --latest --var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY --wait
```

The key is set once in the account vault (`ast secrets create
MEMENTO_AGENT_KEY --description <text>`, value entered at the prompt or
passed from the keyring without echoing it); a redeploy then injects it by
reference. `ast secrets list` shows the name and never the value.

Shopping turns are driven through the platform messaging proxy with the
CLI's own login (the chat host sits behind SSO and cannot be driven
anonymously). Keep turns to single short messages and re-check
`ast billing get` after each. The Done-when trace must show the
`recall_for_purchase` tool span before the recommendation generation, and a
`record_correction` span where the shopper corrects a size.
