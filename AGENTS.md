# Memento Emptor: agent protocol

Binding for every human and coding agent working in this repository. It says how work runs and
what every change must satisfy. What the product is lives in [`product.md`](product.md). How the
orchestrator picks, dispatches, validates and lands lives in
[`dev-diary/ORCHESTRATOR.md`](dev-diary/ORCHESTRATOR.md).

Read this file in full before you change anything. **It is the only file that says how work runs.**
No other document repeats these rules. If one ever does, this file wins and the copy gets deleted.

## Read order

1. This file, in full.
2. [`product.md`](product.md).
3. [`dev-diary/README.md`](dev-diary/README.md), the task pattern and the status board.
4. The phase document for the task, and every task in its `requires` line.
5. The research file for the area you touch, in `dev-diary/research/`.
6. The fixtures and earlier code the task names.

Then assert three things. You own every path you will edit. The shapes you need already exist.
You can validate the change on its own. If any part reads false, stop and report before touching
code.

## How work runs

One task, one agent, one branch, one worktree. The orchestrator alone writes task status lines,
lands commits, and edits `dev-diary/README.md`. A worker builds the task, commits on its branch,
and sets the task to `in-review`.

1. **Claim before starting.** The orchestrator writes `status: claimed:<agent>` in the phase
   document on `main` and commits it. A task with any other status is not yours.
2. **Stay inside `owns`.** The task's `owns` line is the complete set of paths you may edit. A
   change outside it is a new task or a handoff note, never a quiet edit.
3. **Work on your own branch in your own worktree.** Never edit the main checkout in place. Never
   push, merge, or land.
4. **Prove it.** `./tools/check.sh` passes in your worktree. The task's own checks pass. For a
   live task, the transcript is in your report.
5. **Report.** Branch, `git log --oneline main..HEAD`, each **Done when** clause with its evidence,
   anything unmet, and anything surprising.

A task that turns out to be blocked becomes a finding, not a cut. Record it, name every task that
depends on it, and mark it `blocked`.

## The gate

```sh
./tools/check.sh
```

It runs formatting, vet, the Go tests, the Svelte check, lint, the web tests and the web build. CI
runs the same script on every push to `main` and every pull request. A commit that fails its own
gate does not exist.

Run it from the repository root. It reads the Go packages and then builds `web/`. Install the web
dependencies once with `npm ci` inside `web/`.

## Keys

Keys come from the system keyring or the environment. They are never written to a file in this
repository, never committed, and never printed. The project uses no `.env`. `.env.example` names
the variables and holds no value.

Before a live run, source the loader. It exports every key the code reads, leaves a variable you
already set alone, and reports a missing key by name. It holds no value and prints none.

```sh
source tools/env.sh
```

The keyring entries are `service <name> account memento-emptor`, with a `kind` attribute where one
service holds more than one value. Each variable's lookup:

```sh
# PayPal Developer sandbox app.
secret-tool lookup service paypal-sandbox account memento-emptor kind client-id
secret-tool lookup service paypal-sandbox account memento-emptor kind client-secret

# Channel3 product data.
secret-tool lookup service channel3 account memento-emptor

# OpenRouter, which is how Jev is called.
secret-tool lookup service openrouter account memento-emptor

# Elastic deployment, also Lambo's recall tier.
secret-tool lookup service elastic account memento-emptor kind api-key
secret-tool lookup service elastic account memento-emptor kind url

# Render, for the hosted demo and the PayPal webhook endpoint.
secret-tool lookup service render account memento-emptor
```

The PayPal sandbox site logins are in the keyring too, for a person signing in to the sandbox. No
code reads them.

```sh
secret-tool lookup service paypal-sandbox account memento-emptor kind merchant-email
secret-tool lookup service paypal-sandbox account memento-emptor kind merchant-password
secret-tool lookup service paypal-sandbox account memento-emptor kind shopper-email
secret-tool lookup service paypal-sandbox account memento-emptor kind shopper-password
```

Astropods has no key. The `ast` CLI holds its own tokens.

**PayPal is sandbox only.** No live PayPal credential enters this repository or any process it
starts.

To check for a leak, load the value and grep for it. `grep -rlF -- "$VALUE"` prints only file
names, and the value is never echoed.

## Spend

Channel3, Render, Jev and the Elastic trial are finite credits. Every live task carries a budget
and a guard in its brief. Run one case first, check its cost, and stop and report if it goes over.

Anything outward is the user's call: filing an issue, pushing, making the repository public,
publishing the video, submitting, or spending past the brief's budget. Ask first, every time.

## Developer rules

1. **Contracts freeze at the end of P1.** A change to a shared type or interface updates its
   fixtures in `testdata/` in the same commit, and is the orchestrator's call.
2. **A payment outcome is not a cause.** Evidence counts toward a belief only through a reason
   that matches it. No code path may turn a refund or dispute into a belief without one.
3. **Statements never promote.** A general statement goes through `derive`. Only an outcome with a
   classified reason, or feedback tied to a transaction, is a structural write.
4. **The shopper's correction wins at once, and the belief stays.** The layer returns the
   correction with precedence, and declares it with `against`. It never deletes the belief.
5. **The layer never moves money.** Orders, approval and double-buy protection live in the
   reference agent.
6. **No sponsor names in the layer's domain code.** PayPal is the evidence source by design.
   Lambo, Elastic and Jev sit behind interfaces. Channel3 and Astropods appear only under
   `agent/`.
7. **Seeded means marked.** Every seeded row carries the mark from T1.3, and the ledger shows it.
8. **New work only.** Write it here. Never copy code from Lambo or any earlier project. Keel,
   Chaaya and Lambo are dependencies.
9. **Gaps in Lambo, Keel or Chaaya become issues.** Write the issue text in the handoff log. Do
   not work around it here. The user files it.
10. **Document handoffs.** A task without a handoff entry is unfinished. Record what exists, what
    surprised you, and what the next agent should not have to work out again.

## Code and writing

- Go code follows Keel's style: `ctx` first on I/O, one sentinel per condition, wrapped errors,
  `errors.Is` and `errors.As`, no panics, no stored contexts, and a doc comment on every exported
  identifier.
- Svelte 5 only. Runes, not stores, not `export let`, not `$:`. Wrap a Chaaya observable with
  `reactive`. Use `attach(target)` to run the target's own `attach()` in the component's effect,
  and it calls the teardown when the component goes away.
- Comments describe behaviour. A comment naming a retry, fallback or default the code lacks is a
  defect.
- No dates, windows or target days in `dev-diary/`. Order work by `requires` only. The hackathon
  deadline lives in `product.md`.
- Plain commit messages. No `Co-Authored-By`, no Claude or Anthropic attribution.
- This diary is public. Nothing private goes in it: no key, no account detail, no IP address, no
  personal path beyond `~`, no private repository's name.
