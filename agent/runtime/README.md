# memento-agent (T3.1; pivot behaviour T9.4; served layer T9.8; catalog search T9.17)

The Memento Emptor reference shopping agent as a deployed Astropods service.

- Stack: TypeScript/Bun + Mastra. Chosen over LangChain because the adapter
  auto-wires OpenTelemetry through `serve()`, so "the agent recalled first"
  is evidenced by ordered tool spans on the platform collector, and the T0.8
  probe already proved this stack's trace shape live.
- Model: `claude-sonnet-4-6` via the gateway provider (no personal key; the
  platform injects `ASTRO_GATEWAY_URL`/`KEY`, deploy picks `MODEL_DEFAULT`).
  The same model runs in all three comparison arms.
- Tools: the eight layer calls the edge's access table admits to an agent
  key, each one an HTTP call to the served layer's edge (`agent/layer.ts`):
  `POST /tools/<tool>` with the deployment's agent key in
  `Authorization: Bearer` and the shopper id in the body. The three ledger
  reads (`ledger_beliefs`, `ledger_belief`, `ledger_evidence`) are
  shopper-only in that table, so this agent holds no tool for them rather
  than offering one the edge would refuse. Tool names and input shapes are
  the layer's frozen contract; the descriptions keep the layer's own words
  plus this agent's recall-first guidance. The ninth tool, `search_catalog`,
  is agent-local, not an edge tool: it searches Channel3 directly through
  `agent/catalog.ts` and returns Channel3's product records unchanged.
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
- Retries: a read that fails may be called once more; a write
  (`record_correction`, `record_reason`, `record_feedback`) is read back
  first — recall, pending questions, explanation — and resent only when the
  read shows it did not land. `set_pooling` states a choice, so repeating it
  changes nothing extra. The offline check pins that wording, because nothing
  in the tool code retries a call on its own.

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

`agent/layer.ts` builds the call, `agent/tools.ts` binds the eight layer
tools to it, and `agent/offline-check.ts` swaps the transport for the
contract's own frozen examples, which is how the recall-first contract and
the request shape are proved without a deployment. It also reads the edge's
access table (`internal/auth/surface.go`) and the contract's request schemas
(`testdata/api`) and compares them with the tools this deployment offers, so
a tool id or an input field that drifts from the layer fails the check here
rather than as a refusal on a live turn.

## The catalog

`search_catalog` is the one tool the layer does not serve. The layer holds no
product shelf, so the agent searches Channel3 itself (`agent/catalog.ts`) for
a candidate that shares the cut of what the shopper returned: same brand,
same category, equal category-specific fit label. The rule is the same one
`agent/catalog` (Go) applies for the demo driver, and the offline check
replays the recordings in `testdata/channel3` that package's tests use.

The tool's answer is Channel3's own product records, unchanged: no size note,
no fulfilment label and no match evidence is added, so the model can only see
fields Channel3 returned. On apparel `variants` is null and sizes live at the
retailer's offer URL; where Channel3 models variants, its listed sizes are
quoted as they are. Real-retailer fulfilment stays simulated in the demo.
Channel3 is not a layer caller and the edge's access table neither admits nor
refuses this tool, so the edge set stays exactly the eight memory calls.

Two values configure the catalog, both from the deployment's environment:

- `CHANNEL3_API_KEY` — the catalog key, from the platform secret store by
  reference (`--var CHANNEL3_API_KEY=@CHANNEL3_API_KEY`). Without it a search
  fails as `catalog-unconfigured` rather than answering from a recording.
- `CHANNEL3_CATALOG_URL` — the API base, defaulting to Channel3's own.

A search costs one Channel3 credit; product detail calls are free and this
agent makes none.

## Run

```sh
cd agent/runtime
bun agent/offline-check.ts   # contract examples and catalog recordings through the same tool code, no model, no network
ast spec validate            # spec check, local
ast push -V private -y       # build + register the blueprint
ast agent redeploy --id <deployment-id> --latest \
  --var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY \
  --var CHANNEL3_API_KEY=@CHANNEL3_API_KEY --wait
```

The keys are set once in the account vault (`ast secrets create
MEMENTO_AGENT_KEY --description <text>` and `ast secrets create
CHANNEL3_API_KEY --description <text>`, value entered at the prompt or
passed from the keyring without echoing it); a redeploy then injects each by
reference. `ast secrets list` shows the names and never the values.

Shopping turns are driven through the platform messaging proxy with the
CLI's own login (the chat host sits behind SSO and cannot be driven
anonymously). Keep turns to single short messages and re-check
`ast billing get` after each. The Done-when trace must show the
`recall_for_purchase` tool span before the recommendation generation, and a
`record_correction` span where the shopper corrects a size.
