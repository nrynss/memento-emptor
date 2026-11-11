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
status:     done
```
The service, the ledger and Lambo, on the hosting T0.9 chose. PayPal webhooks point at the public endpoint. Lambo's data persists across a restart. A budget guard protects the credits while the demo is public.

**Done when:** the full demo runs against the public URL from a clean browser; a restart loses nothing; the guard is shown to stop at its limit.

**Notes (worker, T6.1).** `deploy/` holds the whole shape, verified end to end in Docker against the final image: edge (same-origin SPA + API proxy + `/webhooks/paypal` → `/webhooks/payment` shim), memory layer seeded from `testdata/replay`, Lambo 0.10.1 sidecar seeded once per contract (Venerable sizing belief, seller-only dispute, declared correction; marker-idempotent on the disk), `render.yaml` + `provision.sh` replay the settled plan/disk/region, `guard.sh` audits run-rate and inventory (dry-run trip: $32.25 fixture vs $8 ceiling, exit 2, nothing called), `paypal-webhook.sh` subscribes the three proved event types idempotently. One recorded deviation: runtime docker, not go (frozen service cannot serve the SPA; $0 cost delta). Restart proof: inspect byte-identical (Venerable, blast 3, Contradicts) across a container restart with the seed skipped. FINDING for the orchestrator: the live Render create is blocked on repo access — the API refuses the private repo as unfetchable with no GitHub connection (nothing billable created; guard still reads $0.00/mo). Unblock: connect GitHub on the dashboard or run `provision.sh` after T6.5 makes the repo public, then the webhook script; first green deploy auto-follows the land.

**Notes (worker, T6.1, cont.).** Rebased onto current `main` (clean). `provision.sh` as committed sent a spec-invalid payload (`dockerfilePath`/`dockerContext` at the wrong level, `envVars` nested in `serviceDetails`, boolean `autoDeploy`): the API answered `invalid JSON` before any repo check. Fixed to the public-spec shape — same settled values, `envSpecificDetails` nesting, top-level `envVars`, `autoDeployTrigger: commit` — and the corrected attempt reaches the true blocker: `passed in repository URL is invalid or unfetchable` (repo still private, no GitHub connection; 0 services, $0 spent). Verified locally from the final image: `docker build` green with `lambo 0.10.1` in-image; boot serves `/healthz` 200, SPA 200, and the `/webhooks/paypal` shim proxies to the layer (structured verdict, not 404); a container replacement with the same data volume keeps a sentinel file and serves healthy after restart. Guard: snapshot dry-run trips (exit 2, $32.25 vs $8 ceiling, nothing called); live audit reads $0.00 run-rate, exit 0. `check.sh` passes. No public URL, no instance/disk ids: nothing billable exists. Unblock is unchanged — land on `main`, then connect GitHub or go public (T6.5), then rerun `provision.sh` verbatim.

**Notes (worker, T6.1, cont. 2).** Rebased onto current `main` clean (no conflicts; `main` never touched `deploy/`, and the replayed tree is byte-identical there); `./tools/check.sh` green. Retried the settled create after the user connected GitHub: the API no longer reports the repo as unfetchable — it now stops earlier at `402 {"message":"Payment information is required to complete this request. To add a card, visit https://dashboard.render.com/billing"}` (provision.sh exit 22; a second probe with a dummy env var answered identically, so no secret was stored anywhere). Nothing billable exists: services list reads 0, guard live audit reads $0.00/mo, exit 0; snapshot dry-run still trips (exit 2, nothing called). The repo-readability question is therefore still unverified — billing gates ahead of any clone attempt. Credit confirmation stays a dashboard step: the public API exposes no billing/credit endpoint (probed paths 404). Unblock is the user's call, either way: add a card on the dashboard billing page (the pre-flight research/hosting.md already flagged), then rerun `provision.sh` verbatim; or go public first (T6.5). Do not point live PayPal webhooks anywhere until the first green deploy serves its public URL.

**Notes (worker, T6.1, cont. 3).** Rebased onto current `main` clean (`main` never touched `deploy/`); `check.sh` green. Both external blockers are gone: the card is on file (no more 402) and the GitHub connection reads the repo (branch validation now reaches the real branch list — the unpushed task branch is correctly reported absent, and it stays unpushed). Provisioned the shell verbatim (`provision.sh`, branch `main`): service `memento-emptor`, plan starter, region virginia, docker runtime, 1 GB disk at `/var/lambo-data` — settled shape holds, run-rate $7.25/mo (credit figure stays a dashboard step; the API exposes no billing endpoint). First deploy `build_failed` exactly as documented pre-land: `main` carries no Dockerfile until this lands, and the green deploy auto-follows via `autoDeployTrigger: commit`. Fixed two live-shape parses the 402 era hid: the API wraps items (`{"service": ...}`, `{"disk": ...}`) — `provision.sh` and `guard.sh` now unwrap. Guard live audit reads the real inventory ($7.25/mo, exit 0); snapshot dry-run still trips (exit 2, nothing called). One diagnostic probe service was created and deleted in the same session (dummy env values only); inventory is exactly the one settled service. Still unmet, all waiting on the land plus first green deploy: no public URL, no restart proof, and no PayPal webhooks pointed (do not point any until the public URL serves).

---

### T6.2: The judges' page ★
```yaml
requires:   [T4.3, T6.1, T9.3d, T10.5]
fixture-ok: yes
size:       S · mid
owns:       web/src/routes/judges/
status:     in-review
```
The first screen is a "For judges: start here" table: the demo, the evidence ledger, the merchant app, the traces, both comparison rounds, and the seeded rows clearly marked. Test credentials for the sandbox, if a judge needs them, are safe to publish and are sandbox-only.

**Done when:** every link works from a clean browser; every claim on the page links to its evidence.

**Notes (worker, T6.2, resumed after T10.5).** Rebased onto current `main` clean; the tip touches only `web/src/routes/judges/` and this block. The static guide at `/judges` keeps its start-here table, walkthrough and simulated/seeded section, and the round-2 placeholder is now the measured results from `bench/round2/README.md`: committed first-turn right size per case and arm (arm 3 10/16, arm 4 12/16), the hand reading beside it labelled as a reading and not the measurement (12/16 and 16/16), the extraction artifact stated before the counts, the correction counts for the next turn, the next session and another agent, the $0.789309 spend against the $5.00 cap, the recall difference in one sentence (28 of 56 Memento turns), and the four headline candidates presented as candidates with no choice made. Every count links to `bench/round2/runs/r2-scores.json`, `r2-summary.json`, `r2-recall-compare.json`, `r2-corrections.json`, the named run files or the round-2 record. The table row carries the summary counts and links to the section and `r2-scores.json`. The test allows digits in the round-2 section and row now, keeps the no-verdict/no-promise guard and the no-percentage check, pins the four candidates and that each has a link, and keeps every other claim/evidence check. `./tools/check.sh` green (web 27 files / 240 tests; build writes `judges.html`, and the edge serves `/judges` through the SPA fallback the same way `/ledger` is served). No live read, no credential value.

**Remaining work.** Link re-verification only: re-check every link from a clean browser once T6.1's first green deploy serves the public URL and T6.5 makes the repository public. The hosted-demo link is the page's own `/`, so it follows whatever origin the app is served from; the GitHub links point at `main`.

**Handoff notes.** The traces row links the Astropods console (`https://astropods.com`) and the committed trace records, because no deployment page URL or account host is committed; replace it with the direct `MEMENTO-REF` and `MEMENTO-SECOND` pages if T6.3 or the operator publishes them. The page is not linked from `/`, because the root page and the layout are outside this task's `owns`; T6.3 should point the submission's demo URL at `/judges`, and the orchestrator should mint a task owning `web/src/routes/+page.svelte` or the layout if an on-site link is wanted.

**Surprises.** The eslint rule wants `rel="external"` on a non-`resolve()` href even when it is an absolute URL built by a helper (`repositoryFile`), so the GitHub links carry it. The static adapter writes `judges.html`; the edge's direct-file check serves the SPA fallback for `/judges`, and the client router renders it, the same path `/ledger` takes. `LICENSE` is Apache 2.0 while T6.5's text says MIT; the page says "open-source licence" and links the file, and the licence choice is the orchestrator's/user's call at T6.5.

---

### T6.3: README and submission text ★
```yaml
requires:   [T6.1, T9.6, T9.3d, T10.5]
fixture-ok: yes
size:       S · frontier
owns:       README.md, docs/submission.md
status:     user: T6.3 is the user's
```
The README is written for a developer on PayPal's Agent Toolkit, opening with the description line in product.md. It follows product.md's "How it is consumed": a seller or platform runs the layer beside its PayPal app. It covers how to plug in (keys included), setup a judge can follow, the architecture, and what is simulated and seeded. `docs/submission.md` drafts the form's text: the description; each tool and how it was used; what changed in Lambo during the hackathon, since it is an existing dependency. The README closes with where the product could go next: a network across sellers, as product.md describes it, clearly marked as not built.

**Done when:** a clean clone follows the setup steps to a running system; every sponsor tool's use is described from what was built.

---

### T6.4: The video ★
```yaml
requires:   [T9.7, T10.5]
fixture-ok: no
size:       M · user
owns:       docs/video.md
status:     user: T6.4 is the user's
```
Under 3 minutes, on YouTube, public, showing the project working end to end, with no third-party trademarks or unlicensed music. Opens on the shopper's problem, shows what a builder plugs in, leads with the number the user picks from T10.5, and signs off with *Caveat agens*, with its translation. An agent drafts the script in `docs/video.md`; the user records and publishes.

**Done when:** the video is public and under 3 minutes, and its link is in `docs/submission.md`.

---

### T6.5: Go public ★
```yaml
requires:   [T6.2, T6.3]
fixture-ok: no
size:       S · user
owns:       none (repository settings)
status:     user: T6.5 is the user's
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
status:     user: T6.6 is the user's
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
status:     user: T6.7 is the user's
```
After the deadline, submissions are locked. Don't edit the repository, video or linked materials until winners are announced. Keep the demo, test credentials, trials and credits active until then.

**Done when:** winners are announced, with the demo still up.

---

### T6.8: Builder docs site
```yaml
requires:   [T6.3]
fixture-ok: yes
size:       S · mid
owns:       docs-site/
status:     claimed:t68-docs
```
An Astro Starlight site publishing the builder documentation: what the layer is, the interface reference from `docs/interface.md`, the architecture from `docs/architecture.md`, the submission summary, and links to the demo, ledger, and repo. Separate from the ledger web app; static build.

**Done when:** the site builds statically; every page a judge or builder needs is reachable from its front page.

## Exit Criteria

- [ ] Public demo running, and survives a restart (T6.1).
- [ ] Judges' page and README complete (T6.2, T6.3).
- [ ] Video public (T6.4).
- [ ] Repository public with a visible licence (T6.5).
- [ ] Submitted (T6.6).
- [ ] Kept running until winners are announced (T6.7).
- [ ] Builder docs site live (T6.8).
