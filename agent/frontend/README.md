# memento-shopper-frontend (T3.8)

The Memento Emptor public shopper page, deployed on `probe-gw` (the
deployment whose frontend hostname is Public). `memento-agent` is
untouched: it has no frontend host, so the page ships as a separate
deployment with its own spec here.

- Stack: TypeScript/Bun, Mastra agent + plain `Bun.serve` static page in
  one container. The adapter's `serve()` registers messaging; the page,
  `/health` and `POST /api/chat` share the injected `PORT` (80 in
  production). No auth guard is mounted on purpose — the frontend host is
  the no-login surface; the chat host stays signed in.
- Image stays **root** with `EXPOSE 80` (binding :80 needs root). The
  production filesystem is read-only: all state is memory-only.
- Tools: `recall_for_purchase` + `record_correction` only, served from the
  recorded demo fixture (`server/demo-state.ts`) — the same ids and words
  the reference agent replays. No purchase tool: approval is an explicit
  "yes" in chat, then an honest not-wired statement. Live corrections
  persist in container memory and reset on redeploy (the demo reset).
- Anonymous spend guard: at most 30 model turns per container lifetime
  (`/api/chat` answers 429 past the cap; validation failures never count).

## Run

```sh
cd agent/frontend
bun install
bun run typecheck        # tsc --noEmit, local
node --test server/relay.test.ts   # the chat relay's guards; node only, no install
SKIP_SERVE=1 bun server/index.ts   # local smoke: page + validation only (no gateway creds locally)
ast spec validate -f astropods.yml
ast push probe-gw -y
ast agent redeploy --name probe-gw --latest --adapter web --wait
```

## Guards and their proof

`server/relay.ts` holds the two rules an anonymous page needs: the request
shape `validateMessages` accepts (1..20 messages, `user`|`assistant`, 1..1000
characters of text, ending on the shopper's turn) and the 30-turn
`createTurnBudget` cap that answers 429 past it. `server/index.ts` only
plumbs them into `POST /api/chat`, so `decideChatTurn` is where "a malformed
body costs no turn" and "the cap refuses without consuming" are decided.
`server/relay.test.ts` tests both; it imports only `node:test` and
`node:assert`, so `tools/check.sh` runs it with node, without bun or an
install. The gate also type-checks `server/relay.ts` with the TypeScript
already installed for `web/` where bun or this package's modules are absent.

`testdata/t38-public-page.txt` is T3.8's live result (anonymous 200, the
container's 404, the four turn costs, the proof turn id) with placeholders
for hosts; it says on its face which lines are quoted and which are the
page's own.

Chat turns go through the page (`POST /api/chat`, same origin) or the
platform messaging proxy with the CLI login; both land in the deployment
trace list with the tool spans in order. Keep turns to single short
messages and re-check `ast billing get` after each server-side step.
