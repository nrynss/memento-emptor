# Research: what was measured on the real stack

Each file here records one outside service measured before any product code depended on it. The
question is never "does the documentation say this works". It is "what did the service do when we
ran it, and what did it say back".

Read the file for your area before you build on it. Where a finding contradicts `product.md`, the
handoff log records the discrepancy and the orchestrator asks the user.

## What a file holds

Every area file opens with:

- the service, its version or tier, and the account it ran under, with no account detail that
  identifies a person.
- the commands or requests that were run, in order, with the key read from the environment and
  never shown.
- the result each one gave, as a short transcript or the fields that matter.
- what the result means for the build, in plain words.
- any credit, spend or trial window the run used.

A claim without a command or a transcript is not evidence. A transcript that shows a key is
scrubbed before it is committed.

## Recordings

`testdata/<area>/` holds the responses the tasks replay. A recording is the smallest complete
answer: one event, one reply, one page of results. Name it for what it contains, not for when it
was taken.

A recording carries no key, no token, no account email, no account id and no IP address. Replace
each with a fixed placeholder that keeps the shape. Record the replacement in the area file, so a
reader knows what was removed.

Tasks marked `fixture-ok: yes` build and test against these files with no key and no spend. A
task marked `fixture-ok: no` calls the live service.

## Files

| File | Answers |
|---|---|
| `lambo.md` | Whether a seeded belief reaches Venerable, the GC cadence, the contradiction warning on a Venerable belief, a non-fit dispute that changes nothing, the ledger states, and one session per shopper. The gate on the whole product. |
| `paypal.md` | Order, approval and capture with idempotency. What a refund and its webhook carry, including whether a refund has a reason. A dispute with a reason code. Webhook signature and redelivery. Line items. Which calls the Agent Toolkit covers. |
| `channel3.md` | Whether the data says enough about a product's cut to match a new SKU to returned ones. What a match can use instead. The brand and line chosen for the demo. Credits used. |
| `elastic.md` | Hybrid search over product and return-reason documents. The embedding model. Lambo's recall tier: index creation, external-version writes, kNN search, delete by query, and fallback when the index is down. The hosting decision. |
| `jev.md` | Reason categories and probabilities for shopper answers. The confidence floor and the labelled set that shows it separates clear answers from unclear ones. Cost per call. |
| `astropods.md` | Typed input and resume, feedback with trace context, OpenTelemetry traces, evaluators, the secrets vault, and calling an external MCP server on the free tier. The agent's language and deploy shape. |
| `hosting.md` | What Render costs for the service and a persistent disk against the credit. The user's decision on where Lambo runs. |
| `ag-grid.md` | AG Grid mounted through its plain JavaScript API in a Svelte 5 component on Chaaya. Cell renderers, tokens, editing, dark mode, and the licence tier. |

A file is written by its task. An empty file here means the task has not run, not that the answer
is unknown.
