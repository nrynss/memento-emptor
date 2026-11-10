# memento-second (T9.5)

The second Memento Emptor shopping agent: its own Astropods deployment,
its own name, its own instructions, its own traces — recalling through
the same layer as the reference agent.

- Stack: TypeScript/Bun + Mastra, gateway model `claude-sonnet-4-6` like
  the reference agent, so a turn costs the same on either side.
- Tools: the layer's eleven frozen interface calls. The descriptions are
  the layer's own frozen words; everything else in the instructions is
  written fresh, with no correction story inside. That is the point: when
  this agent honours the shopper_01 correction on its first
  recommendation, the layer carried it, not the prompt.
- State: stand-in answers from `agent/store.ts` until the served layer
  (T9.6) wires both deployments to one backend. The values are the
  layer's demo-shopper state (same belief and correction ids, same
  pooled tallies), so both agents recall the same correction.
- Money: no ordering tool on purpose. The agent asks for an explicit
  "yes" before anything purchase-shaped and says outright that buying is
  not connected here.

## Run

```sh
cd agent/second
bun agent/check.ts        # stand-in proof, no model, no network
ast spec validate         # spec check, local
ast push -V private -y    # build + register the blueprint
ast deploy --adapter web --wait
```

Drive turns through the platform messaging proxy with the CLI's own
login (the chat host sits behind SSO): `DEPLOYMENT=<id> OUT=<dir>
python3 agent/second/drive.py create|send|history`. Keep turns single
short messages and re-check `ast billing get` after each. The Done-when
trace must show `recall_for_purchase` before the recommendation, with
the reply following the recalled correction.
