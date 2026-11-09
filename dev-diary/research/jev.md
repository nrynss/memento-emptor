# Jev reason classifier

Service: OpenRouter Decisions API (`POST https://openrouter.ai/api/alpha/decisions`),
model `typesafe/jev-1.14` (pinned; no `latest` alias). The build string the
endpoint resolved on this run was `typesafe/jev-1.14-20261014`. Ran with
`OPENROUTER_API_KEY` read from the environment via `source tools/env.sh`.
Nothing below identifies the account.

## Questions asked

One `choice` question per shopper answer. The categories are exclusive: fit
direction (which belief a fit return counts toward), seller-reliability causes,
a no-cause return, and an explicit `unclear` outlet so vague answers are not
forced into a wrong confident pick.

- instructions: `Why is the shopper returning the item? Pick the single
  category that best matches the shopper's answer.`
- state: `{"answer": "<the shopper's words>"}`
- criteria:
  - `fit_too_small`: too small or too tight (tight shoulders, short sleeves,
    cannot button or zip, runs small).
  - `fit_too_large`: too big or too loose (swims on the shopper, runs large).
  - `damaged_or_defective`: arrived broken, damaged, cracked, torn, or does
    not work.
  - `not_arrived`: never arrived, lost in shipping, delivery failed.
  - `changed_mind`: changed their mind, no longer wants it, no product fault.
  - `unclear`: no usable reason; vague, ambiguous, or off-topic, so the agent
    must ask again.

## Labelled set and results

Eleven phrases, one Decisions call each, run sequentially with a short pause
between calls. No retries were needed: eleven `200`s, no `429`s. The full
request and reply for each phrase is recorded in `testdata/jev/`.

| # | Shopper answer | Expected | Choice | Confidence | Verdict |
|---|---|---|---|---|---|
| 1 | too tight in the shoulders | fit_too_small | fit_too_small | 1.00 | correct |
| 2 | the sleeves are way too short and I can't button it | fit_too_small | fit_too_small | 1.00 | correct |
| 3 | it swims on me, far too big | fit_too_large | fit_too_large | 1.00 | correct |
| 4 | arrived broken | damaged_or_defective | damaged_or_defective | 1.00 | correct |
| 5 | the screen was cracked when I opened the box | damaged_or_defective | damaged_or_defective | 1.00 | correct |
| 6 | it never arrived, tracking has been stuck for weeks | not_arrived | not_arrived | 1.00 | correct |
| 7 | changed my mind | changed_mind | changed_mind | 1.00 | correct |
| 8 | just don't want it anymore | changed_mind | changed_mind | 1.00 | correct |
| 9 | not great | unclear | unclear (0.95) | 0.94 | correct, asks again |
| 10 | it wasn't what I wanted | ambiguous | changed_mind (0.97) | 0.96 | resolved as changed_mind, asks again under the floor |
| 11 | meh, sending it back | unclear | unclear (0.73) | 0.68 | correct, asks again |

Every probability not shown above was 0.00 except item 11, which put 0.27 on
`changed_mind`. No refusal answers were observed (the schema allows a
`refusal` answer type; T2.2 must treat one as ask-again).

## Confidence floor

Decision rule for `record_reason`:

```text
ask_again = (choice == "unclear") or (confidence < 0.98)
```

On this set the floor separates clean from unclear: all eight clear answers
came back at 1.00, and all three vague or ambiguous answers came back below
1.00 (0.94, 0.96, 0.68). The `unclear` category carries the truly vague
answers (items 9 and 11); the floor catches the near-miss phrasing (item 10)
that lands on a real category just under full confidence.

Caveats, stated plainly: eleven phrases is a calibration sample, not proof.
Jev's probabilities here are sharp (nearly all mass on one category), so the
gap between item 10 (0.96) and the clear answers (1.00) is thin; a different
wording could land inside it on either side. T2.2 re-measures the floor on
real shopper answers before it gates any belief, and keeps the `unclear`
category as the load-bearing part for vague answers.

## Cost

Per-call `usage.cost` from each reply (USD), input tokens in parentheses:

| # | Cost | # | Cost |
|---|---|---|---|
| 1 | 0.000021336 (508) | 7 | 0.000021252 (506) |
| 2 | 0.000021630 (515) | 8 | 0.000021378 (509) |
| 3 | 0.000021462 (511) | 9 | 0.000021210 (505) |
| 4 | 0.000021252 (506) | 10 | 0.000021378 (509) |
| 5 | 0.000021504 (512) | 11 | 0.000021336 (508) |
| 6 | 0.000021546 (513) | | |

Total for the eleven calls: **$0.000235284** for 5,602 input tokens
(about $0.0000214 and 509 input tokens per call; output tokens are returned
but unbilled). At this shape, 1,000 classifications cost about **$0.021**.
The key's limit is $50 with about $49.9998 remaining after this run, so the
task spent a negligible fraction of the budget and no guard was tripped.

## What this means for the build

- The reason categories above are sufficient for the demo's evidence path:
  fit direction feeds the sizing belief, `damaged_or_defective` and
  `not_arrived` count against the seller and never toward sizing, and a
  `changed_mind` refund stays an outcome with no reason that counts toward
  nothing. That is rule 2 of the developer guidelines, enforced by category.
- A refund the shopper never explains, or explains below the floor, stays in
  `pending_questions` and the agent asks again. No code path may turn it into
  a belief without a classified reason.
- Pin `typesafe/jev-1.14` in T2.2. A version change re-opens this file's
  floor measurement.

## Recordings

`testdata/jev/` holds one file per phrase (`01-…` through `11-…`), each with
the exact request body and the reply as returned. Scrubbed of keys: the
`Authorization` header was never stored, and the only identifier retained is
the per-request generation id, which identifies the reply, not the account.
The key-balance check's response (which carries account and workspace ids) is
summarised above as figures only and is not committed anywhere.
