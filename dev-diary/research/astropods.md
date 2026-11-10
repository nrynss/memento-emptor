# Astropods: what the free tier does

Service: Astropods platform, CLI `ast/0.32.0 (BETA)`, account on the free tier
(one-time signup credits, no card on file). The `ast` CLI lives in `~/.ast/bin`,
which is not on a default tool-shell `PATH`; every probe session exported it
first. The CLI holds its own login tokens; no key was read from the keyring and
none appears in any recording.

Billing at the time of the probe: `$10.00` signup credit, `$0.00` used, spend
warning and limit at their defaults. Every live step below re-checked billing;
nothing spent anything, and the probe stopped well before the credits ran low.

What was run, in order (all commands from a scratch directory outside the repo;
only the recordings in `testdata/astropods/` are committed):

- `ast billing get` / `ast billing status` — tier and credit position.
- `ast docs help`, `ast docs agent` — platform and agent guides, read in full.
- `ast create` (mastra, langchain, and gateway-model variants) — scaffold shapes.
- `ast push` (private blueprint `probe-gw`) and `ast deploy --adapter web --wait`
  — one probe agent, build `<build-id>`, environment `main`, web chat at a
  dedicated `agents.astropods.ai` host, with an agent workload and a collector
  sidecar. Deploy output in `02-push-deploy.txt`.
- `ast agent get/access/list`, `ast agent pause/resume`, `ast agent usage`,
  `ast agent trace`, `ast eval status` — deployment lifecycle and empty states.
- `ast secrets create/list/get/delete` — vault round-trip, then deleted.
- `ast eval validate` (many iterations, all local) and `ast eval push` /
  `ast eval get` — evaluator schema discovery and server activation.

Scrub rule for every recording: account and user ids, chat host, deployment and
build ids, namespace, timestamps, and the SSO URLs the chat endpoint redirects
to are replaced with `<...>` placeholders. Spend amounts are kept, because the
task is to record usage.

## Language and deploy shape

Confirmed by scaffolding and by pushing and deploying the probe.

- Two template stacks: **TypeScript on Bun with Mastra** (`@mastra/core`,
  `@mastra/memory`, `@mastra/observability`, `@astropods/adapter-mastra`), and
  **Python 3.12 with LangChain/LangGraph** (`astropods-adapter-langchain`).
  `ast create --template mastra|langchain --model gateway|anthropic|openai`.
- Deploy shape is a container: `Dockerfile` plus `astropods.yml`
  (`spec: blueprint/v1`, `agent.build.context/dockerfile`, `models:`,
  `knowledge:`, `integrations:`, `interfaces:`, `ingestion:`, `inputs:`).
  `ast push` builds the image, registers a versioned private blueprint;
  `ast deploy` runs it in an environment with the chosen adapters
  (`--adapter web|slack`). The full spec schema is published at
  `https://astropods.com/schema/package.json`; `ast spec validate` checks a
  file against it. One free-tier note: the running probe reports a "Slowed by
  CPU limit" warning on the agent workload, so the free compute allocation is
  small and a chatty demo should stay concise.
- Model access without a personal key: the gateway variant declares
  `models: {default: {provider: gateway, models: [...]}}` and the platform
  injects `ASTRO_GATEWAY_URL` + `ASTRO_GATEWAY_API_KEY` at deploy time, with the
  deploy-time model pick injected as `MODEL_DEFAULT`. The scaffold default is a
  current Claude Sonnet via the gateway. The probe deployed this way and the
  account holds no model key.
- Local dev is `ast project start` (chat UI on localhost, hot reload), but it  runs no telemetry collector, so local runs prove interaction only, never
  traces. The probe was not run locally: gateway credentials exist only on a
  deployment, and borrowing another task's key for local model access would
  spend the wrong budget.

Meaning for the build: the reference agent is a container the project owns,
written in TypeScript/Mastra or Python/LangGraph, deployed with the web adapter
for the demo chat. No model key is needed if it uses the gateway.

## Pause for typed input and resume

Partly confirmed; the live two-turn exchange is still to run (see the gap).

- Proven live: `ast agent pause` moves the deployment to `Stopped`
  ("Deployment is paused"); `ast agent resume` restarts it (`pending` while
  workloads become ready, then `Running`). Transcript: `09-pause-resume.txt`.
  This pauses the whole deployment, not one conversation.
- Documented in the spec schema (`connections[]`): each entry carries
  `required`, and "whether the chat is blocked until the user allows this
  connection. Defaults to true". That is a platform-level pause-until-the-user-
  acts gate, declared in `astropods.yml`. Not exercised live (it needs a real
  OAuth connection to gate on).
- The conversational form — the agent asks, the shopper types, the agent
  resumes — is ordinary messaging turn-taking over the web adapter's REST API
  (`POST /api/conversations`, `POST .../messages`, `GET .../history`,
  `GET .../stream` over SSE; endpoints from the scaffold's Postman collection).
  The probe agent was written for exactly this: its instructions say to ask the
  user to type "yes" before acting, and to act only after the confirmation.
- Not yet run: the deployed web chat sits behind user-scoped SSO (anonymous
  requests get a login redirect), so a scripted turn-taking exchange needs an
  interactive session. The agent is left `Running` for that step.

Meaning for the build: approval and correction forms are agent-built chat turns
("reply yes to approve", "which size instead?"), resumed on the shopper's next
message; the platform contributes deployment pause/resume and the
block-until-allowed connection gate. No separate "forms" primitive was found.

## Feedback arriving with trace context

Shape confirmed in the CLI; live write still to run (needs a trace).

- `ast eval review --trace-id <id> --set key=value` "saves your values for a
  trace's evaluators as its human review", replacing any earlier review;
  `--update-dataset` also updates the trace's dataset item. Evaluators left out
  are not part of the review. `ast eval get` shows each blueprint's keys and
  accepted values, so a feedback form can offer exactly the accepted values.
- `ast dataset add <dataset> --trace-id <id>` attaches a trace (with its
  current evaluator values) to a dataset as an item; the trace must belong to
  the dataset's deployment and have an input.

Meaning for the build: shopper feedback (approval, correction, fit note) is
recorded per trace as a human review, and traces of interest become dataset
items. The write itself is one CLI call per trace once the demo turns exist.

## OpenTelemetry traces showing tool calls in order

Platform side confirmed; a live trace with tool spans still to run.

- The probe deployment runs two workloads: `agent` and `collector`. The
  platform injects `OTEL_EXPORTER_OTLP_ENDPOINT` (deployed environments only)
  plus `ASTRO_AGENT_NAME`, `ASTRO_AGENT_BUILD`, `ASTRO_AGENT_ID`, and
  `ASTRO_EXTERNAL_AGENT_URL` (only when the agent exposes an endpoint).
- The Mastra adapter auto-wires OpenTelemetry when the endpoint is set
  (`serve(agent)` registers the agent and configures observability; the
  template tags spans `astro` / `agent:<name>` with stable metadata). Other
  stacks use the documented adapters (AI SDK, Claude Agent SDK) or manual
  OpenTelemetry with the GenAI semantic conventions.
- `ast agent trace` lists traces (`--limit`, window, evaluated filter) and
  `--trace-id` shows one trace's detail ("Overview"). Today it reports "No
  traces found", and `ast agent usage` shows zero traces, tokens, model spend,
  and compute (`0.000 CU-hours`). Empty states: `10-trace-usage-empty.txt`.
- The probe agent carries one demo tool (`lookup_return`: order id in, item
  plus reason out), so the first real turn will show whether tool spans land in
  order. Not yet run, for the same SSO reason as above.

Meaning for the build: "the agent recalled first, honoured the correction, and
never skipped approval" is evidenced by per-deployment OTel traces the
platform collects; the reference agent only has to use an instrumented adapter
and call tools for the checkable steps.

## Evaluators over those traces

Schema and activation confirmed live; a scored run still needs a trace.

- A blueprint ships a default `preset/default-evaluation` set (seen via
  `ast eval get` before pushing anything custom): `exposed_pii`,
  `leaked_credentials`, `disclosed_system_instructions`,
  `unnecessary_tool_call` (all boolean), `claim_grounding`
  (`grounded | unsupported | contradicted | unverifiable | no_claims`),
  `user_sentiment` (`positive | neutral | negative | unclear`).
- A custom set is `EVALUATION.yaml` beside `astropods.yml`, discovered field by
  field through local `ast eval validate` (free, no server contact):
  `schema: evaluation/v1`, 1–10 evaluators, each with snake_case `key`, 1–50
  char `label`, `type: llm`, a `prompt` (or `prompt_file`), and
  `output: {type: boolean}` or `{type: enum, options: [...]}`.
  Discovery transcript: `06-eval-validate.txt`. Every evaluator is an LLM
  judge with a custom prompt; there is no code-defined metric type.
- The probe's one-evaluator set (`followed_approval`: did the assistant ask for
  confirmation before acting) validated and was activated on the server with
  `ast eval push`; `ast eval get` confirms it. Transcript:
  `07-eval-preset-and-push.txt`; the file: `evaluation-set.yaml`.
- `ast eval run` queues up to 50 recent unevaluated traces (or one with
  `--trace-id`); runs finish in the background, `ast eval status` reports
  queued/in-progress/completed/failed/outdated (all zero today).

Meaning for the build: the comparison's checks (right size, wrong-cause,
correction honoured, citations) map to custom LLM-judge evaluators plus human
review, scored over the demo's traces. Judge cost per run is unmeasured — the
first `eval run` should be one trace, watching billing.

## Secrets vault

Fully confirmed live, then cleaned up.

- `ast secrets create T08_PROBE` (value via `--value`, `--plain` keeps it
  unencrypted, `--description` allowed; names are `[A-Za-z0-9_]`), `list`
  shows name, type, and timestamps but never values, `get` masks the value as
  `******`, `delete` removes it. Full round-trip: `05-secrets-vault.txt`.
  Account vault is empty again afterwards.
- Env-scoped values (`--env`/`--blueprint`) override account values with the
  same name; `deploy --var KEY=@SECRET_NAME` (or `@` for same-name) injects a
  vault secret by reference instead of `--from-local` plaintext.

Meaning for the build: PayPal and other credentials the reference agent needs
at runtime live in the vault and arrive by reference; nothing secret touches
the repo or the image.

## Calling an external MCP server

Documented pattern only; not run live.

- The documented path is the framework client's: `MCPClient` with
  `MCPOAuthClientProvider` (OAuth 2.1 dynamic registration + PKCE), tools from
  `mcp.listTools()` passed straight into the agent. The one production
  gotcha is the redirect URI: it must resolve back to the agent, so the agent
  declares `agent.interfaces.frontend: true` to get a public host, and reads
  the platform-injected `ASTRO_EXTERNAL_AGENT_URL` with a localhost fallback.
  Without an exposed endpoint the flow degrades to pasting the authorization
  code back to the agent. Token storage must survive the read-only production
  filesystem (Redis via a `knowledge` entry, not a file).
- Gotchas the docs name: resource-indicator (RFC 8707) mismatches on some
  authorization servers, and stale client registrations when the redirect URI
  changes between local and deployed runs.
- Not run: pointing the probe at a real MCP server needs a public MCP URL plus
  chat turns to trigger the tool, which is the same blocked live step as the
  traces. The probe's local `lookup_return` tool stands in for the pattern
  (Mastra tools "work out of the box" and surface in the playground).

Meaning for the build: if the reference agent needs PayPal or Channel3 tools
over MCP, the code path exists but carries OAuth/first-deploy friction;
wrapping their HTTPS APIs as plain agent tools is the fallback, and it uses
the same tool-trace evidence.

## Gap closed: the live conversation

The three turns were driven without a browser, through the platform's
messaging proxy (`/api/v1/deployments/<deployment-id>/messaging/...`, the
same proxy the console chat UI uses), authenticated with the CLI's own
login held in process memory and never written anywhere. Transcripts:
`testdata/astropods/11-chat-turns.txt` (turns), `12-traces.txt` (ordered
spans), `13-eval-run-review.txt` (judge + human review),
`14-vault-clean.txt` (vault still empty).

- Turn 1 ("Look up the return for order 7XK.") produced trace
  `828c0f1b...`: 17 observations in order, with the `[tool] lookupReturn`
  span between the two model generations. Cost $0.0055.
- Turn 2 ("Record size L for Brand X for me.") produced trace `aa7abb1a...`:
  10 observations, no tool span — the agent asked the user to type "yes"
  instead of acting. Cost $0.0028.
- Turn 3 ("yes") produced trace `90c85c11...`: 10 observations, no tool span.
  The probe owns no record tool, so the agent resumed and honestly declined
  rather than hallucinating the write; the pause-resume mechanics held.
  Cost $0.0032.
- `eval run` on the turn-2 trace: the `followed_approval` judge scored
  `true` at confidence 0.99. `eval review --set followed_approval=true`
  saved the human review on the same trace.
- Total spend for the whole close-out: usage $0.02 of the signup credit.
  The vault holds no probe secrets.

## Chat URL revival (close-out probe)

The chat host once answered 404 in an interactive session while the deployment
itself stayed `Running` with URL status `ready`. Direct checks showed the host
alive throughout: every path (`/`, `/health`, `/api/agent/config`) answers 302
to the platform SSO login, which is the healthy anonymous response — a 404 is
the failure mode. Inbound traffic counting showed the single failed request,
confirming the failure sat behind the login, not in DNS or routing.
`ast agent restart --component agent` could not run: it demands `--component`
and then rejects both `agent` and the full workload name, which looks like a
CLI-side naming bug. The cheap fix was `ast agent redeploy --name probe-gw
--wait`: same blueprint build, same environment, same deployment id, same chat
URL, back to `Running` with the agent and collector workloads. No spend moved
at any step. If a 404 recurs in the browser, re-check `ast agent get` for
`Running`/`ready`, confirm the exact host character by character (an unknown
host is the simplest way to get a bare 404), and redeploy before rebuilding:
nothing about the failure pointed at the image.

## Chat URL root cause (close-out, second pass)

The 404 survived the redeploy because the deployed spec never declared the
chat interface. The probe's `astropods.yml` had messaging only under
`dev.interfaces` (local `ast project start`, chat UI verified at the local
sidecar root). The deployed contract is `agent.interfaces.messaging: true`
("Agent supports the messaging protocol"); without it the deployment ran only
the agent and collector workloads, so the chat host had no web sidecar behind
it — anonymous paths still 302 to SSO (healthy-looking), while any
authenticated request to `/` lands on nothing: the user's 404. The Postman
collection documents only API routes (`/api/conversations`, `/health`,
`/api/agent/config`); there is no separate UI path to try. The fix was adding
the four-line `agent.interfaces: {messaging: true}` block, `ast spec
validate`, `ast push` (new build), and `ast agent redeploy --latest`: same
deployment id, same chat URL, `Running`, still `$0.00` spent. The sidecar is
platform-managed and does not appear in the components list, so only the
user's browser can confirm the UI now serves post-login. If the chat host
still 404s, the fallback is the console Agent Details page
(`astropods.com/<account>/<blueprint>`, 200 anonymously, chat tab when signed
in), not another path on the chat host.

## Public reachability probe (frontend + messaging on probe-gw)

Question: can a judge reach an Astropods agent with no login? Answer: not
with the defaults — anonymous requests get a 302 to the platform SSO login,
and only the console **Public** switch on the deployment's Configure screen
removes sign-in. The switch is untested here (it needs the user's click);
everything below is the evidence for both halves.

- Built a minimal probe on `probe-gw` only (`memento-agent` untouched, same
  build and `Running` throughout). One container serves a static page plus
  `/health` on the injected `PORT` (80 in production) with **no auth guard**,
  and registers the messaging interface, with
  `agent.interfaces: {frontend: true, messaging: true}` nested under
  `agent:` (`messaging` defaults to false once `interfaces` is declared).
  The Dockerfile stays root with `EXPOSE 80` (binding :80 needs root); the
  production filesystem is read-only, so state is memory-only; the gateway
  model is unchanged. Transcript: `testdata/astropods/15-public-probe.txt`.
  Two builds: the page plus a `lookupReturn` demo tool, then a fix to the
  tool's `execute` input shape (the input arrives unwrapped, not as
  `{context}`); redeployed with `--latest --adapter web`.
- Anonymous (fresh session, no cookies, no auth): `GET /` and `GET /health`
  on the deployment host both answer **302 to the platform SSO login**.
  That is the documented failure mode: a frontend agent keeps OIDC sign-in
  at the front door by default, before any traffic reaches the container.
- Why the switch matters: the web list (built-in chat) and the custom list
  (own app) live on the agent's **Configure** screen. CLI
  `ast agent access set` offers `anyone` — any *signed-in* Astropods user —
  but no public option (`access get` on the probe shows web restricted to
  one user, custom defaulting to account members). **Public removes
  sign-in** (documented only in the Node/Python adapter auth warning), and
  with it on no request carries an identity — so a public surface must
  either add an `anyone` entry or, as here, mount no guard at all (a guard
  would answer 401 to anonymous callers).
- Model turn through the messaging proxy (CLI login token in process
  memory, never printed; a browser UA is required because the default
  scripting UA trips the edge bot check): create 201, send 200, trace
  `<trace-id>` at $0.0057 with 17 observations in order, including a
  successful `[tool] lookupReturn` span between the two generations and
  the correct return details in the reply. Gateway, traces and tool spans
  all still work on the frontend+messaging build.
- The exact step for the user: open the console at
  `astropods.com`, the agents list, the `probe-gw` deployment,
  **Configure**, the custom-app (own-app/frontend) access section, and
  switch **Public** on. That is distinct from granting `anyone`, which
  keeps sign-in. After the flip, the check to re-run is the anonymous
  `GET /`: expect 200 serving the page with no SSO redirect.
- Post-flip check (user switched Public on in the console): the deploy
  exposes **two** hostnames — the messaging (chat) host and a dedicated
  frontend host (the earlier probes hit the chat host, not the page).
  Anonymous `GET /` and `/health` on the **frontend** host still answer
  302 to the platform SSO login, immediately and after a 3-minute wait;
  `access get` is unchanged and shows no Public flag. So the flip has not
  taken effect on anonymous traffic, and no-login serving is NOT proven.
  Nothing was changed for this check (GETs only). Next: the user
  re-confirms the toggle persisted on the right deployment/interface, else
  this becomes a platform gap for the handoff log.
- Billing for the probe: usage $0.29 before the deploy, $0.35 after
  (two model turns $0.0056 + $0.0057, the rest platform compute), inside
  the $0.25 task budget. Account usage later rose to $0.60 from concurrent
  tasks' spend; the post-flip verification itself (anonymous GETs plus
  billing reads) cost nothing. Scrub rule for every recording: hosts, deployment
  and build ids, conversation/message/session/user ids, and the SSO URLs
  are placeholders; spend amounts are kept.

Meaning for the build: judges can reach a no-login page only after Public
is switched on the shopper deployment; the shopper page itself must stay
guard-free. The follow-up shopper-page task is for the orchestrator to
schedule once Public is verified on the probe.

## Credits used

Usage spend `$0.35` of the `$10.00` signup credit at the T3.7 close-out
($0.27 after the T3.4 demo arc, then the probe's two builds, two redeploys
and two model turns at $0.0056 + $0.0057). Billed spend `$0.00`, credit
left `$9.65`. The T3.7 probe-period delta was $0.06, inside its $0.25
budget. Every live step re-checked billing; nothing spent past its guard.
