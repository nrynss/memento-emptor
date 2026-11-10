# memento-agent (T3.1; pivot behaviour T9.4)

The Memento Emptor reference shopping agent as a deployed Astropods service.

- Stack: TypeScript/Bun + Mastra. Chosen over LangChain because the adapter
  auto-wires OpenTelemetry through `serve()`, so "the agent recalled first"
  is evidenced by ordered tool spans on the platform collector, and the T0.8
  probe already proved this stack's trace shape live.
- Model: `claude-sonnet-4-6` via the gateway provider (no personal key; the
  platform injects `ASTRO_GATEWAY_URL`/`KEY`, deploy picks `MODEL_DEFAULT`).
  The same model runs in all three comparison arms.
- Tools: the layer's eleven frozen interface calls plus one PayPal-flavoured
  order read. All fake-backed (see `agent/memory.ts`): tool names and shapes
  match the frozen contract, so the live swap changes no tool the agent
  reads.
- Precedence (T9.4): the agent recommends from `guidance.basis` and states
  confidence from the label (Earned, Building, Claimed, Out of true). It
  never refuses when recall returns evidence; only `none` says plainly no
  memory applies. A pool answer cites counts with their seeded split; a
  statement answer says it is Claimed words, not evidence. A correction
  with no belief behind it is recorded with no belief id and stands on its
  own (container memory only), deciding again on the next turn.
- Money: this task owns no purchase-execution tool on purpose. Orders and
  double-buy protection belong to T3.3 (`agent/orders/`). The agent asks for
  an explicit "yes" before any purchase and says plainly the purchase step
  is not wired yet instead of claiming it happened.

## Standing configuration

The deployed agent initially talks to the fake-backed tools because the
layer is not served yet (T2.5 is later). This is the recorded standing
configuration, not a gap. The fake serves the frozen demo-shopper state
(`shopper_01`): the Out-of-true L belief with three seeded fit returns, the
M correction (same words as the live layer-side correction in T3.6, served
here as fixture state, not a live recall), one pending refund, the fits-well
feedback row, and the seller-side non-arrival dispute.

## Run

```sh
cd agent/runtime
bun agent/offline-check.ts   # fake-first proof, no model, no network
ast spec validate            # spec check, local
ast push -V private -y       # build + register the blueprint
ast deploy --adapter web --wait
```

Shopping turns are driven through the platform messaging proxy with the
CLI's own login (the chat host sits behind SSO and cannot be driven
anonymously). Keep turns to single short messages and re-check
`ast billing get` after each. The Done-when trace must show the
`recall_for_purchase` tool span before the recommendation generation.
