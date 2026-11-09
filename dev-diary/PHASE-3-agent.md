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
status:     claimed:t32-catalog
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
status:     claimed:t33-orders
```
Durable order state (proposed → approved → captured, or failed and refunded), a uniqueness rule on open orders per shopper and need, `PayPal-Request-Id` on every create and capture, and webhook handling idempotent on the event ID. A purchase cannot be made without an approval record. Purchases run through our own sandbox merchant.

**Done when:** tests cover a retried create, a duplicate webhook, a second order for the same need while one is open, and a capture with no approval.

---

### T3.4: Asking the shopper ★
```yaml
requires:   [T3.1, T3.3]
fixture-ok: no
size:       M · frontier
owns:       agent/loops/
status:     not-started
```
The asynchronous loop, with Astropods' input forms and feedback:

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
status:     not-started
```
Astropods evaluators over real traces. Did the agent recall before recommending? Did a correction change the very next recommendation? Did any purchase skip approval?

**Done when:** each evaluator passes on a good run and fails on a deliberately broken one; both are recorded.

---

### T3.6: The full demo ★
```yaml
requires:   [T2.6, T3.2, T3.3, T3.4, T3.5, T1.3b]
fixture-ok: no
size:       M · frontier
owns:       demo/
status:     not-started
```
Product.md's six demo steps, end to end, live on the real stack, starting from the seeded demo shopper. A reset script returns the demo to its starting state.

**Done when:** a clean run from the reset script completes all six steps; the run is recorded in `demo/`; a second run from reset gives the same outcome.

---

## Exit Criteria

- [ ] The agent runs on Astropods, recalling first (T3.1).
- [ ] Real products from Channel3 (T3.2).
- [ ] Purchases are idempotent and approved (T3.3).
- [ ] Reasons, approvals, corrections and feedback all flow (T3.4).
- [ ] Evaluators check the rules (T3.5).
- [ ] The full demo runs, repeatably, from reset (T3.6).
