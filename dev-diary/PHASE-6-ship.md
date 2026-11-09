# P6: Ship

Everything the submission needs. Product.md's "Hackathon rules that shape the build" is the checklist.

Read first: [`../product.md`](../product.md).

---

### T6.1: Deploy ★
```yaml
requires:   [T2.6, T3.6, T0.9]
fixture-ok: no
size:       M · mid
owns:       deploy/
status:     not-started
```
The service, the ledger and Lambo, on the hosting T0.9 chose. PayPal webhooks point at the public endpoint. Lambo's data persists across a restart. A budget guard protects the credits while the demo is public.

**Done when:** the full demo runs against the public URL from a clean browser; a restart loses nothing; the guard is shown to stop at its limit.

---

### T6.2: The judges' page ★
```yaml
requires:   [T4.3, T5.6, T6.1]
fixture-ok: yes
size:       S · mid
owns:       web/src/routes/judges/
status:     not-started
```
The first screen is a "For judges: start here" table: the demo, the evidence ledger, the traces, the comparison results, and the seeded rows clearly marked. Test credentials for the sandbox, if a judge needs them, are safe to publish and are sandbox-only.

**Done when:** every link works from a clean browser; every claim on the page links to its evidence.

---

### T6.3: README and submission text ★
```yaml
requires:   [T5.6, T6.1]
fixture-ok: yes
size:       S · frontier
owns:       README.md, docs/submission.md
status:     not-started
```
The README is written for a developer on PayPal's Agent Toolkit, opening with the description line in product.md. It covers how to plug in, setup a judge can follow, the architecture, and what is simulated and seeded. `docs/submission.md` drafts the form's text: the description; each tool and how it was used; what changed in Lambo during the hackathon, since it is an existing dependency.

**Done when:** a clean clone follows the setup steps to a running system; every sponsor tool's use is described from what was built.

---

### T6.4: The video ★
```yaml
requires:   [T3.6, T4.3, T5.6]
fixture-ok: no
size:       M · user
owns:       docs/video.md
status:     not-started
```
Under 3 minutes, on YouTube, public, showing the project working end to end, with no third-party trademarks or unlicensed music. Opens on the shopper's problem, shows what a builder plugs in, leads with the number chosen in T5.6, and signs off with *Caveat agens*, with its translation. An agent drafts the script in `docs/video.md`; the user records and publishes.

**Done when:** the video is public and under 3 minutes, and its link is in `docs/submission.md`.

---

### T6.5: Go public ★
```yaml
requires:   [T6.2, T6.3]
fixture-ok: no
size:       S · user
owns:       none (repository settings)
status:     not-started
```
Before the switch, the orchestrator checks the whole history for keys, account details and anything the diary rule forbids. The user makes the repository public, with the MIT licence visible in the About section.

**Done when:** the repository is public, and the scan's result is in the handoff log.

---

### T6.6: Submit ★
```yaml
requires:   [T6.4, T6.5]
fixture-ok: no
size:       S · user
owns:       none (Devpost)
status:     not-started
```
The user submits on Devpost, selecting every sponsor tool used so that it counts for sponsor prizes: PayPal, Channel3, AG Grid, Render, Elastic and Astropods. Hosted demo URL, public repository, video.

**Done when:** Devpost's confirmation email has arrived, and the ledger of hackathon entries is updated.

---

### T6.7: Keep it running
```yaml
requires:   [T6.6]
fixture-ok: no
size:       XS · user
owns:       none
status:     not-started
```
After the deadline, submissions are locked. Don't edit the repository, video or linked materials until winners are announced. Keep the demo, test credentials, trials and credits active until then.

**Done when:** winners are announced, with the demo still up.

---

## Exit Criteria

- [ ] Public demo running, and survives a restart (T6.1).
- [ ] Judges' page and README complete (T6.2, T6.3).
- [ ] Video public (T6.4).
- [ ] Repository public with a visible licence (T6.5).
- [ ] Submitted (T6.6).
- [ ] Kept running until winners are announced (T6.7).
