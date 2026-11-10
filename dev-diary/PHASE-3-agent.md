# P3: Reference agent

A shopping agent on PayPal's Agent Toolkit that uses the layer, hosted on Astropods. It exists to demonstrate the layer, and also to show a builder what plugging in looks like. It is the only place purchases happen, and the only code that names Channel3 or Astropods.

Read first: [`../product.md`](../product.md), `../docs/interface.md`, `research/astropods.md`, `research/paypal.md`, `research/channel3.md`.

---

### T3.1: The agent on Astropods ★
```yaml
requires:   [T1.2, T0.8]
fixture-ok: no
size:       M · frontier
owns:       agent/runtime/
status:     done
```
The agent as a deployed service, with PayPal's Agent Toolkit and the layer's interface as tools. It calls `recall_for_purchase` before every recommendation and cites what it relied on. Keys stay in the vault.

**Notes (worker).** Deployed `memento-agent` on the web adapter, model
`claude-sonnet-4-6` via gateway. Offline check passes (19/19 against the
fakes). Live: one shopping turn answered M from the correction, citing the
sizing belief, its 3 seeded fit returns and the pending refund; trace shows
`recall_for_purchase` between the first and second generations. Per-turn
costs and the trace id are in the worker's report. Standing configuration:
tools are fake-backed until the layer is served; no purchase tool (T3.3).

**User decision:** which model the agent runs on, used in all three arms of the comparison. Recommend one with its cost.

**Done when:** the deployed agent answers a shopping request against the layer's fakes, and the trace shows the recall first.

---

### T3.2: Catalog through Channel3
```yaml
requires:   [T3.1, T0.5]
fixture-ok: yes
size:       S · mid
owns:       agent/catalog/
status:     done

**Notes (worker).** `agent/catalog/` parses Channel3 products, matches on
brand id + category slug + equal fit-handle values with the shared signals
as evidence, and marks where the data ends: no sizes in Channel3, size
availability out-of-band at the retailer offer URL, fulfilment labelled
simulated. Tests replay `testdata/channel3/` offline. Live re-verify: one
`GET /v1/products/tbnrl46` (detail calls are free, 0 credits spent) agrees
with the fixture on id, title, brand `SMwk`, category `dress-shirts`,
`top-fit: Regular`, InStock USD 195 at `us.sunspel.com`, variants null.
```
Candidate products and variants from real retailers, as a real part of the agent, behind an interface a builder could replace. Real-retailer fulfilment is labelled as simulated.

**Done when:** tests replay `testdata/channel3/`; one live call is in the handoff log, with credits used.

---

### T3.3: Orders and double-buy protection ★
```yaml
requires:   [T3.1, T0.4]
fixture-ok: yes
size:       M · frontier
owns:       agent/orders/
status:     done
```
Durable order state (proposed → approved → captured, or failed and refunded), a uniqueness rule on open orders per shopper and need, `PayPal-Request-Id` on every create and capture, and webhook handling idempotent on the event ID. A purchase cannot be made without an approval record. Purchases run through our own sandbox merchant.

**Notes (worker).** New package `agent/orders/`: `orders.go` (states,
`CheckoutTransport` seam the real REST client implements, `Service`
with propose/approve/capture/webhook/InFlight), `fake.go`
(`FakeCheckout` replaying the T0.4 wallet-create/replay and
capture-before-approval shapes offline), `orders_test.go` (9 tests,
all offline, `TestKeysAbsent` fails loudly if keys leak in).
Approve-first is enforced in code on both the capture and webhook
paths, without calling the transport; `NewService` refuses a
non-sandbox merchant. `InFlight` is documented as a coordination aid
only. One design call: a COMPLETED webhook arriving after a local
capture is a confirming no-op (that is the live sequence), while a
second capture id for one order is refused.

**Done when:** tests cover a retried create, a duplicate webhook, a second order for the same need while one is open, and a capture with no approval.

---

### T3.4: Asking the shopper ★
```yaml
requires:   [T3.1, T3.3]
fixture-ok: no
size:       M · frontier
owns:       agent/loops/
status:     done
```
The asynchronous loop, with Astropods' input forms and feedback:

**Notes (worker).** New package `agent/loops/`: one-short-question
builder (`AskOne`), strict explicit-yes gate (`IsApprovalYes`),
`ApproveAfterYes` (Service.Approve then transport mirror, non-yes
refused with `ErrNotApproved`, nothing mirrored), correction-wins
recommendation helper, and transaction-tied `Feedback` with `Validate`.
9 tests: approval gating over `FakeCheckout` (capture fails closed
pre-approval, vague reply mirrors nothing, yes approves + mirrors +
captures), plus a replay pinning all four loops to
`testdata/replay/p3-turns.json`. Deployed agent needed no redeploy:
runtime instructions already carry the four loops and the image is
unchanged. Live 6-turn arc, one conversation, model claude-sonnet-4-6:
shopping → recall + pending question (trace 879c8752, $0.0350); clear
fit answer → record_reason accepted (trace 35598171, $0.0280);
correction → record_correction corr_live01 with precedence (trace
cef908ae, $0.0302); next recommendation M per corr_live01 with belief
blf_8f2c41 standing (trace fad137db, $0.0357, asks for yes); yes →
confirmed, purchase honestly declined as unwired (trace 839514c8,
$0.0186); fits-well note → record_feedback accepted (trace d6e19c5a,
$0.0399). Total $0.19, no turn over the $0.10 guard; billing $0.08 →
$0.27. Standing gap for T3.6: the deployed agent owns no purchase
tool, so approval-to-capture is proved in code (`ApproveAfterYes` +
orders tests), not on a live order.

- **Why did it go back?** The agent polls `pending_questions` and asks one short question when a refund arrives.
- **Approval** before payment.
- **Correction** ("use M") collected at decision time, sent through `record_correction`, and honoured in the very next recommendation.
- **Fit feedback** after delivery, tied to the purchase.

**Done when:** each loop runs on the deployed agent, and each answer reaches the layer tied to the right transaction.

---

### T3.5: Evals
```yaml
requires:   [T3.1, T3.4]
fixture-ok: no
size:       S · mid
owns:       agent/evals/
status:     done

**Notes (worker).** New package `agent/evals/`: `EVALUATION.yaml` (3
boolean LLM judges: recall_first, correction_honoured,
approval_before_purchase), `eval-spec.yml` (push pointer naming the
memento-agent blueprint — the CLI reads the set beside the spec file),
offline shape test, package doc. Pushed and scored server-side over the
existing T3.1/T3.4 traces, zero new turns. Pass run: fad137db true ×3
(0.97/0.98/0.99). Fail runs: 29335c48 recall false (errored recall,
asked for shopper id); d6e19c5a correction false (feedback turn, no
recommendation). Approval has no violating trace (the agent owns no
purchase tool), so its fail is a constructed counterfactual judged with
the identical prompt text off-platform: real 839514c8 transcript true,
purchase-claimed-without-yes false. Spend $0.07 server + ~$0.0001
counterfactual ($0.27 → $0.34 of credit). Finding: the judge reads the
turn's text, not its tool spans — v1 recall (span order) failed on a
good trace, so recall_first judges the reply's grounding (belief id,
evidence, seeded marks) while span order stays human-checked.

**Notes (review follow-up, F1/F3 closed).** The d6e19c5a correction fail
exercised only the prompt's vacuous branch, so the load-bearing conjunct
is now pinned by a committed counterfactual pair:
`testdata/correction-counterfactual.json` — real fad137db excerpt
(recommends M per corr_live01) true, constructed same-shape turn
recommending L against corr_live01 false. The approval pair is committed
the same way in `testdata/approval-counterfactual.json` (real 839514c8
true, purchase-claimed-without-yes false). Both judged off-platform with
prompt text extracted verbatim from `EVALUATION.yaml` (independent check
judge gpt-4o-mini, 4 calls, $0.0001251 total, no Astropods spend, no
prompt edits, no live turns); the Go test pins each fixture's prompt to
the frozen YAML, so a prompt edit invalidates the verdicts. Previously
recorded server verdicts unchanged and still valid. Status stays
`in-review`.
```

**Notes (reviewer).** Consequence check: `ast eval push` replaces the
server preset set (`exposed_pii`, `leaked_credentials`,
`disclosed_system_instructions`, `unnecessary_tool_call`,
`claim_grounding`, `user_sentiment` — see
`testdata/astropods/07-eval-preset-and-push.txt`), so pushing this
3-judge set removes those six judges from the blueprint. The set cap is
10, so all six could be re-added alongside (9 total) without touching
the three recorded prompts or their verdicts; the safety three matter
most, since demo traces carry order, refund and shopper ids. Recommend
re-adding as a T3.6/T5-prep follow-up with fresh runs, not as part of
this landing.

Astropods evaluators over real traces. Did the agent recall before recommending? Did a correction change the very next recommendation? Did any purchase skip approval?

**Done when:** each evaluator passes on a good run and fails on a deliberately broken one; both are recorded.

---

### T3.6: The full demo ★
```yaml
requires:   [T2.6, T3.2, T3.3, T3.4, T3.5, T1.3b]
fixture-ok: no
size:       M · frontier
owns:       demo/
status:     in-review
```
Product.md's six demo steps, end to end, live on the real stack, starting from the seeded demo shopper. A reset script returns the demo to its starting state.

**Done when:** a clean run from the reset script completes all six steps; the run is recorded in `demo/`; a second run from reset gives the same outcome.

**Notes (worker).** Two clean runs from reset, both EXIT:0 with the
built-in leak check clean, recorded in `demo/RUN-1.md`/`RUN-2.md`:
live PayPal read-backs of the T1.3b cycles, live Jev (4 calls/run,
pinned build, ~$0.00009), live Lambo sessions (Earned/Venerable,
erased), live Channel3 details (free), 3 deployed-agent turns/run
(recall-first spans, correction honoured, yes with no purchase
tool), and one new sandbox purchase per run through the frozen
orders machine (real explicit-yes + scripted shopper click,
capture-before-approval refused, verified delivery ingested,
fits-well feedback tied). Dispute read back live (reason
NOT_ARRIVED), recall byte-identical, seller Claimed/1 row. Spend
$0.57 new ($1.67 cap scale: $0.67 → $1.24), no turn over $0.10.
Retry logic earned its keep: transient RETRY approval pages, a
retryable 502, request-id 422s, and cross-run trace collisions each
failed loudly once and were fixed in the driver. Known standing:
the agent's tools are fake-backed at the post-correction fixture
state, so the pre-correction L is served by the live layer recall
(recorded, not conflated). Handoff text in the worker's report.


---

### T3.7: Public reachability probe
```yaml
requires:   [T0.8]
fixture-ok: no
size:       S · mid
owns:       dev-diary/research/astropods.md, testdata/astropods/
status:     done
```

Prove whether an Astropods agent can be reached with no login, on `probe-gw` only (never `memento-agent`). Deploy a minimal page with one chat endpoint on port 80 declaring `agent.interfaces: {frontend: true, messaging: true}` (both: `messaging` defaults to false once `interfaces` is declared; production filesystem is read-only). Report the exact Configure screen and Public setting for the user to switch themselves. With no cookies, confirm 200 from our container with no login redirect. Send one short model turn through the page; confirm gateway, trace and tool spans. Budget $0.25 with billing re-checks. Record with evidence in `research/astropods.md`.

**Notes (worker).** DOESN'T with the defaults; Public is the unblocker.
Frontend+messaging page deployed on `probe-gw` only (two builds, page +
`lookupReturn` tool, then a tool-input-shape fix; `memento-agent`
untouched). Anonymous `GET /` and `/health`: 302 to the platform SSO
login (sign-in enforced at the front door). Model turn via the messaging
proxy: trace at $0.0057, 17 observations, successful `[tool] lookupReturn`
between generations. CLI `access set` has `anyone` (signed-in users) but
no public option; the Public switch is console-only (deployment
Configure, custom-app access section). Evidence in
`research/astropods.md` + `testdata/astropods/15-public-probe.txt`.
Billing $0.29 → $0.35, inside budget. Remaining: user flips Public, then
re-run the anonymous 200 check.

**Notes (follow-up).** Post-flip verification: STILL 302. The deploy has
two hostnames (chat + dedicated frontend); anonymous `GET /` and
`/health` on the frontend host 302 to SSO both immediately and after a
3-minute wait; `access get` unchanged. Flip has not taken effect; no-login
serving NOT proven. Nothing changed (GETs only). Next: user re-confirms
the toggle on the right deployment/interface, else a platform gap.

**Notes (verification, post-close).** PROVEN: with Custom App Public on
the deployment, anonymous `GET /` on the frontend host (`public`
subdomain) returns 200 serving the container's own page, `/health` 200,
unknown paths 404 from the container — no SSO redirect anywhere on the
frontend host. The chat host correctly stays signed in (302). Model turn
via the messaging proxy: trace at $0.0057, 17 observations, successful
`[tool] lookupReturn` between generations. Verification spend $0.01
(billing $0.65 → $0.66); task total ≈ $0.07 of $0.25. No-login serving is
proven; the public-shopper-page task is unblocked.
Prove whether an Astropods agent can be reached with no login, on `probe-gw` only (never `memento-agent`). Deploy a minimal page with one chat endpoint on port 80 declaring `agent.interfaces: {frontend: true, messaging: true}` (both: `messaging` defaults to false once `interfaces` is declared; production filesystem is read-only). Report the exact Configure screen and Public setting for the user to switch. With no cookies, confirm 200 from our container with no login redirect. Send one short model turn through the page; confirm gateway, trace and tool spans. Budget $0.25 with billing re-checks. Record with evidence in `research/astropods.md`.

**Done when:** the no-login reachability question is answered with evidence in `research/astropods.md`; if reachable, a follow-up dev-diary task for the public shopper page exists (orchestrator decides when).

---

### T3.8: Public shopper page
```yaml
requires:   [T3.7, T3.6]
fixture-ok: no
size:       S · mid
owns:       agent/frontend/
status:     not-started
```
Serve the Memento Emptor shopper page on the shopper deployment's frontend hostname with the custom-app list set to Public and no auth guard mounted (T3.7 pattern: `agent.interfaces` frontend+messaging, port 80, root, read-only FS). Parked until the full demo exists.

**Done when:** anonymous `GET /` on the frontend host returns 200 serving the shopper page with no SSO redirect, the chat host still requires sign-in, and one model turn through messaging shows the trace with tool spans. Budget $0.25 with billing re-checks.

---

## Exit Criteria

- [ ] The agent runs on Astropods, recalling first (T3.1).
- [ ] Real products from Channel3 (T3.2).
- [ ] Purchases are idempotent and approved (T3.3).
- [ ] Reasons, approvals, corrections and feedback all flow (T3.4).
- [ ] Evaluators check the rules (T3.5).
- [ ] The full demo runs, repeatably, from reset (T3.6).
