# T5.3 baseline pilot: arms 1 and 2, live

Status: pilot complete, 10 clean runs graded, 2 discarded contaminated turns kept as evidence.

The arm called "raw history" in this pilot is now reported as **Memento evidence without beliefs**; see `bench/README.md` for the relabel.

## What was measured

Every comparison shopper (`cmp-*`, never the demo shopper) through arms 1
(no memory) and 2 (raw history), one live model turn each: a fresh
conversation, one user message, the reply scored. The message of each run is
the harness prompt for that shopper and arm, verbatim. Arms differ only in
the memory block they carry.

Model: gateway Sonnet `claude-sonnet-4-6`, the T3.1 pick, in all runs.
Turns are single-recommendation by construction (one message, one reply).

## The instrument (why not the reference agent)

The pilot first drove two turns through the deployed reference agent
(memento-agent). Both are discarded: the replies cite the *demo* shopper's
memory (`corr_3b7e52`, `blf_8f2c41`, `ev_6d1a09-11`) although the prompts
name `cmp-broken` with no memory. The agent's `shopper_01` default hijacked
the recall; the model cannot guess content-addressed ids, so the recall
provably ran with the wrong shopper. Files:
`runs/pilot-a1-broken-discarded-1.json` and `-2.json` (contaminated, never
graded; the grade command skips them).

The ten graded runs drive `pilot-baseline` instead: the same gateway model,
no tools, no memory beyond the user message. Its source is `agent/` in this
directory (committed so T5.5 can redeploy it); the deployment
(`pilot-baseline`, id scrubbed per the research rule) is left Running for
verification and possible reuse. To remove it:

    ast agent delete --name pilot-baseline --confirm pilot-baseline

The `deployment` field in every run file reads `memento-agent`: the frozen
record builder (`record.go`, `NewRunFile`) writes that constant, so the
field is a fixed label and not a record of what answered. The provenance is
the trace spans in each file — `invoke_agent Pilot Baseline` on the ten
graded runs, `invoke_agent Memento Agent` on the discarded pair — so the
field must not be read as saying the reference agent drove the graded turns.
The builder is frozen (`bench/FROZEN.md` pins `record.go`) and the ten run
files are published output, so the field stays as written; this note is the
record of what it means.

No shopper, harness, grader, or seed file was touched. Frozen fixtures are
read-only inputs to this task.

## The extraction rule (freeze input for T5.4)

Live replies are free text; the grader scores Size and Cites. The pilot's
reduction rule (`pilot.go`, tested offline in `pilot_test.go`):

- **Size, deterministic, first match wins:** (1) explicit `size X`
  mention whose token is in the size vocabulary; (2) first standalone
  vocabulary token; (3) empty when the reply names no size (the agent
  asked a question instead of recommending). A human override wins only
  when the deterministic pick is judged not to be the reply's
  recommendation, with the reason recorded in the run file.
- **Cites, deterministic:** every history transaction id appearing
  verbatim in the reply. SKU or size mentions are never cites.
- **Cites, human:** prose references mapped to a note-substring ref that
  resolves to exactly one history row, each recorded with the reply quote
  and the reason. Prose too vague to map to one row is not a cite.
- **Contamination:** any reply naming demo-shopper facts (`7XK`,
  `Brand X`, `blf_`, `corr_`, `ev_`, `PSW8GeK`, `tbnrl46`,
  `memento-shopper01`, `shopper_01`) is discarded, never graded.
- Extraction never consults the hidden answers; the grade command checks
  no run prompt carries a hidden rationale string.

The rule fired cleanly on 9 of 10 graded runs. The tenth
(`pilot-a2-size-changed`) took the human override: the deterministic
`size M` names the *returned* item while the recommendation is the bold L
("sizing up to L is warranted"). Three arm-2 kept-row prose cites were
dropped as unmappable (line names differ, or the phrase spans two rows);
each drop is recorded in its run file.

## Results (counts)

Grading: `go run ./cmd/grade` (run files + hidden answers -> `scores.json`).
CorrectionHonoured is nil everywhere by design: single-turn runs, no
corrections (seed histories carry none).

| run | arm | recommended | right | right-size | wrong-cause | supported/cited | words | cost USD |
|---|---|---|---|---|---|---|---|---|
| pilot-a1-broken | 1 | - (refused) | M | 0 | 0 | 0/0 | 87 | 0.001137 |
| pilot-a1-never-arrived | 1 | M (default guess) | L | 0 | 0 | 0/0 | 99 | 0.001326 |
| pilot-a1-changed-mind | 1 | - (refused) | M | 0 | 0 | 0/0 | 83 | 0.001050 |
| pilot-a1-size-changed | 1 | - (refused) | L | 0 | 0 | 0/0 | 83 | 0.001050 |
| pilot-a1-unclear | 1 | - (refused) | M | 0 | 0 | 0/0 | 86 | 0.001092 |
| pilot-a2-broken | 2 | M | M | 1 | 0 | 0/2 | 233 | 0.002121 |
| pilot-a2-never-arrived | 2 | L | L | 1 | 0 | 0/1 | 216 | 0.001929 |
| pilot-a2-changed-mind | 2 | M | M | 1 | 0 | 0/2 | 227 | 0.002079 |
| pilot-a2-size-changed | 2 | L | L | 1 | 0 | 2/2 | 288 | 0.002361 |
| pilot-a2-unclear | 2 | M | M | 1 | 0 | 0/2 | 237 | 0.002238 |

Arm 1: 0/5 right-size. Four refusals (no sizing information given) and one
wrong default guess. No cites offered, no wrong-cause errors (nothing moves
off baseline without a recommendation).

Arm 2: 5/5 right-size, 0 wrong-cause errors. Every reply shows the causal
discrimination the shoppers were built to test: damage is not fit, a parcel
that never arrived is not fit, a changed mind counts toward nothing, two
fresh too-small returns outweigh a stale fits-well note, a vague answer
counts toward nothing. Only the size-changed cites resolve to fit rows
(2/2); the other seven prose cites resolve to statement, kept, seller, or
changed-mind rows (0 supported) — correct non-fit handling, but every one
of them needed the human mapping step. Nothing here looks like a leak: the
prompts carry no answer material (verified for both arms; the harness test
proves it for the construction), and all ten replies pass the
contamination check.

## Shoppers: no changes, and why

Arm 2 scored perfectly on right-size (5/5, one run per cell). The brief
allows hardening the shoppers or recording that the lead is elsewhere. No
shopper is added or changed in this task, for three reasons:

1. One run per cell is a thin signal for a ceiling; rebuilding the
   comparison set on it would overfit the set to defeat the baseline.
2. The baseline earned its 5/5 by correct causal reasoning, not by a leak
   or a guess. Hardening now would mean inventing cases where correct
   reasoning from raw history fails but the layer succeeds — that is a
   freeze-level set decision, and set edits live outside this task's owns.
3. The lead is visible elsewhere already: tokens (arm 2 runs 216-288
   words/turn against arm 1's 83-99; a belief-compact arm 3 should undercut
   raw history substantially) and citations (all 9 arm-2 cites need human
   prose-mapping, and 7 of the 9 resolve to non-fit rows; arm 3 cites belief
   and evidence ids mechanically).

Hardening candidates for the freeze, if it wants headroom on right-size
or wrong-cause: histories long enough that the signal drowns in noise
(raw context carries everything; the layer compacts to earned beliefs),
and multi-brand histories where evidence for one brand tempts the
recommendation for another. Both are cases a real history contains, and
both would go through the seed generator path with their own hidden
answers — not by editing the frozen fixtures.

## Spend (binding cap: $5.00 total pilot spend)

| item | USD |
|---|---|
| shape probe, memento-agent (1 turn) | 0.020895 |
| discarded contaminated turns (2) | 0.023259 + 0.023664 |
| neutral-instrument probe (1 turn) | 0.001182 |
| 10 graded runs (5 x arm 1 + 5 x arm 2) | 0.016383 |
| **pilot-attributable total** | **0.085383 (~$0.09)** |
| account billing delta over the pilot ($1.31 -> $1.43) | 0.12 (includes concurrent unrelated spend; per-trace costs above are authoritative) |

Per-run caps from the harness stayed armed (all runs far under 4000
words / $0.05). Billing was re-checked before and after every run batch;
cumulative pilot spend never approached the cap.

Live BPE token counts are not exposed by the trace API, so tokens are
harness word counts on the recorded basis and spend is platform cost; the
two are never mixed. The T5.1 handoff's ~5x live-vs-words factor applies
to both arms equally.

## What surprised

- The reference agent is unusable as the arm driver: its shopper default
  contaminates comparison turns silently (the replies read plausibly and
  even recommend the hidden right size for the wrong reasons). Any T5.5
  driver through it must verify the recall target per turn, not assume it.
- The neutral instrument is ~15x cheaper per turn ($0.001-0.002 vs
  $0.02-0.03): no tool round trips. Arm-2-length prompts roughly double
  the cost of arm-1 prompts.
- With nothing, Sonnet declines (4/5) or guesses a neutral default (1/5)
  rather than hallucinating a history. With raw history, it does the
  evidence algebra correctly on all five shoppers.
