# memento-second (T9.5; served layer T9.8)

The second Memento Emptor shopping agent: its own Astropods deployment,
its own name, its own instructions, its own traces — recalling through
the same layer as the reference agent.

- Stack: TypeScript/Bun + Mastra, gateway model `claude-sonnet-4-6` like
  the reference agent, so a turn costs the same on either side.
- Tools: the eight layer calls the edge's access table admits to an agent
  key, each one an HTTP call to the served layer's edge (`agent/edge.ts`):
  `POST /tools/<tool>` with this deployment's agent key in
  `Authorization: Bearer` and the shopper id in the body. The three ledger
  reads are shopper-only there, so no tool exists here for them. The
  descriptions are the layer's own frozen words; everything else in the
  instructions is written fresh, with no correction story inside. That is the
  point: when this agent honours a correction on its first recommendation,
  the layer carried it, not the prompt.
- State: none of its own. Both deployments read and write the one served
  layer, so a correction recorded through the reference agent is what this
  agent recalls next.
- Money: no ordering tool on purpose. The agent asks for an explicit
  "yes" before anything purchase-shaped and says outright that buying is
  not connected here.
- Retries: a lookup gets one more attempt; a writing note is looked up
  before it is resent, and asked about when the lookup cannot show it.
  Nothing in `agent/edge.ts` retries, so a duplicate write could only come
  from the prompt, and the offline check pins the rule there.

## Configuration

Two values come from the deployment's environment, never from a file:

- `MEMENTO_AGENT_KEY` — the agent key the edge authenticates, injected from
  the platform secret store (`--var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY`).
  With none, every tool call fails as `unconfigured` rather than answering
  from anywhere local.
- `MEMENTO_LAYER_URL` — the edge's base, defaulting to the hosted layer.

## Run

```sh
cd agent/second
bun agent/check.ts        # contract examples through the same tool code, no model, no network
ast spec validate         # spec check, local
ast push -V private -y    # build + register the blueprint
ast agent redeploy --id <deployment-id> --latest --var MEMENTO_AGENT_KEY=@MEMENTO_AGENT_KEY --wait
```

Drive turns through the platform messaging proxy with the CLI's own
login (the chat host sits behind SSO): `DEPLOYMENT=<id> OUT=<dir>
python3 agent/second/drive.py create|send|history`. Keep turns single
short messages and re-check `ast billing get` after each. The Done-when
trace must show `recall_for_purchase` before the recommendation, with
the reply following the recalled correction.
