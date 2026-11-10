# T5.4 Freeze: the comparison inputs are locked

Nothing about the shoppers, catalog, prompts, or grader changes after this
commit. T5.5 runs all three arms against exactly the files hashed below;
any mismatch fails the run before grading.

- Base commit (main at branch time): `9e10b6c`
- Frozen tree: the landing commit of branch `task/t54-freeze` carrying this
  file. File contents are pinned by the hashes below, not by the commit
  hash: re-hashing the tree must reproduce every row of section 1.
- Offline: this file was written and verified with no keys, no network, and
  no spend.

## 1. Frozen hashes (sha256)

Every path is relative to the repository root. The hidden answers file is
pinned by hash only: a hash reveals nothing about the content, but any later
change to the key is detectable.

```manifest
f0fb60fa45af71e60e1ac03b019737b08c8f4e0d91ad2542f5f02f67546fae8e  testdata/seed/histories/cmp-broken.json
af2052991ebda1db85fe1e6838cd240daef1b272cc6582d65323382cc9700b6f  testdata/seed/histories/cmp-changed-mind.json
db9853b5841a6109da424f92fb4bdad3d7462b6187b81ca8b32cd507264e147d  testdata/seed/histories/cmp-never-arrived.json
e0c3ecf47e0bb0020195cfb3370649ee78a18d5c9bcbdcbfa41d17108457b8db  testdata/seed/histories/cmp-size-changed.json
e5c0a426fadf63e2d5927b2aa33c1d514b72912790eca3d73f6f7fb9791ec7c2  testdata/seed/histories/cmp-unclear.json
5b19b45c2eb9136ce294fc0da9c03d92f17485cae6f89d5008be1bbfd05b7063  testdata/seed/histories/demo-shopper.json
b8a80cf744ec15897b06ee248de00fbcea428a8d284d0e2f6ce5fb19f1dee467  testdata/seed/hidden/answers.json
552f6aa67d0570b2f85882aa96dc88206f10f772a0de87637d2e10e447b6e9cc  seed/seed.go
264900cfb53002d836109e9714c32c0910154e42daaec1307ae626f8ff06a926  bench/harness/harness.go
b1fb55d443bd3245e1ed73311e8372add7a2a251d814a4b8dee477d700db0df1  bench/harness/doc.go
2772f032e0b753981aa0f6696022ef1ef67b52af60d7a479e4cea3139d1b7c3d  bench/grader/grader.go
0f3f46c175371f6c89ac6c4b52faf6f37a5a0308e45851afd3a9497a13e0b061  bench/grader/doc.go
fbbf17d5bf0e8f23f2076e961de5744d350f55deecc104b0afda0ee5fa4d43ac  bench/pilot/pilot.go
d7889d8a88391616dcbccaba1f531daca507d550709137018d7ece1458463db6  bench/pilot/record.go
82950928ffca95863de4630d0b9c6c9058de996604fd36faa600a622239233a5  bench/pilot/doc.go
13005fdc65516b0e4b72868063cfd7b3ca7849cbe0b6d3bf47f0d2d6e793c5d5  bench/pilot/cmd/drive/main.go
015559c23f89495d33641fb87458bbb7f64d704bb13b929a124b75d949187e74  bench/pilot/cmd/grade/main.go
dda21ff00332792ec7fc3817d7d41e3a7910d56a602ce3802d8c20cd115cee6f  bench/pilot/agent/agent/index.ts
337fb6804582fd44a7a90c5594b0c8e089fd3fb44c375548c7f71b86ad67eb10  bench/pilot/agent/astropods.yml
2c29bbbf7fa3566bf8d12aad18d458db7c769134b195734a4c41d9a996fc9aeb  bench/pilot/agent/Dockerfile
ec5268dbbd5e23663505a8f0ffe45bef5034090c994dd8628a91a1bd09a39e9e  bench/pilot/agent/package.json
```

What each row is:

| Rows | Role in the comparison |
|---|---|
| `testdata/seed/histories/cmp-*.json` (5) | The arm-facing comparison shoppers. The only histories any arm may read. |
| `demo-shopper.json` | Pinned so the contamination check's demo tokens cannot drift; never an arm input for the comparison. |
| `hidden/answers.json` | The key. No arm reads it; only `cmd/grade` loads it. Hash only, never content. |
| `seed/seed.go` (GoldenSeed 7) | Regenerating with the golden seed reproduces the histories byte for byte. Provenance, not an arm input. |
| `bench/harness/harness.go`, `doc.go` | The arm prompt template: `BaseSection` + `MemorySection` (`PromptFor`), `RawHistoryContext` (arm 2), `LayerContext` (arm-3 dry-run stand-in, documented in code as not the wire shape). |
| `bench/grader/grader.go`, `doc.go` | The grader: five measures as counts, `TokenBasis` harness-words-v1, feedback-counts-by-verdict rule. |
| `bench/pilot/pilot.go`, `record.go`, `doc.go` | The frozen extraction rule (`ExtractSize`, `ExtractIDRefs`, human-override procedure, contamination check) and the committed run-file schema. |
| `bench/pilot/cmd/drive/main.go`, `cmd/grade/main.go` | The live driver and the scorer T5.5 reuses. |
| `bench/pilot/agent/*` | The frozen neutral instrument (see section 4): same gateway model, no tools, no memory beyond the user message. |

Recompute every hash (offline, from the repo root):

```sh
git rev-parse HEAD
sha256sum -c <(sed -n '/^```manifest$/,/^```$/p' bench/FROZEN.md | grep '  ')
```

Or file by file: `sha256sum <path>` must equal the row above.

## 2. Catalog: no Channel3 entry is an arm input

Zero Channel3 products feed any arm prompt. The comparison candidate is
synthetic: `candidateLine` takes the brand and line of the history's first
item row and recommends into "a new SKU on that line with the size
unchosen". The comparison brands (BRC-BRG) are fictional and match nothing
in `testdata/channel3/`. T5.5 keeps this: no catalog fetch belongs in any
arm, and no arm prompt carries a catalog product. The Channel3 fixtures stay
what they are (the reference agent's shelf); they are outside the frozen
comparison set, so they carry no row above.

## 3. Frozen rules T5.5 runs under

- Model: gateway Sonnet `claude-sonnet-4-6`, the T3.1 pick, in all three
  arms. Same string, same gateway, no substitution.
- Per-run caps (harness `DefaultCaps`, enforced in code before any model
  call): 4000 tokens, $0.05 at $5/1M blended. Token cap binds first.
- Cumulative cap: $5.00 total pilot+comparison live spend. $0.085 spent in
  the pilot; T5.5's budget is the remainder. See section 5 for enforcement.
- Token basis: `harness-words-v1` (whitespace words) recorded on every
  score. Platform trace cost is reported beside it, never mixed into it.
- Repeat count: 3 live turns per shopper per arm (5 shoppers x 3 arms x 3 =
  45 turns). The pilot's n=1 is a thin signal; 3 gives stability without
  overfitting the set or the budget (about $0.05-0.11 at pilot per-turn
  rates of $0.001-0.0024).
- Publish everything: every T5.5 turn is committed under `bench/runs/` in
  the `RunFile` schema (prompt verbatim, memory fetch, tool spans, reply,
  words, trace cost, extraction, score), scored by `cmd/grade` against the
  frozen key. Discarded turns (contamination, cap stops) are committed as
  marked run files, never deleted. No result is reported without its run.

## 4. Follow-up verdicts

Each item T5.1-T5.3 handed to the freeze is ADOPTED (frozen rule stated),
DEFERRED (rationale + owner), or OUT-OF-SCOPE (reason).

- T5.1 F1 (arm-3 tally counts fits-well as counts-toward-empty): ADOPTED.
  `LayerContext` stays exactly as hashed: a dry-run stand-in that says so
  in its own doc comment, never layer output. The frozen evidence rule is
  the grader's: classified fit reasons count toward fit through the domain
  rule, and explicit fit feedback tied to a purchase counts toward fit by
  verdict (the T1.2 handoff), via `supportKind`. No later stage may quote a
  `LayerContext` tally line as a layer verdict.
- T5.1 F2 (classified changed-mind tallied as "pending"): ADOPTED.
  In `LayerContext`, "pending" means counts-toward-none outcome rows; it
  sweeps in classified no-fault rows the real `pending_questions` would
  never list. That is frozen and documented behavior of the stand-in, not a
  claim about the layer. True pending semantics (only below-floor unclear
  stays open) come from the real `pending_questions` call in T5.5's arm-3
  memory fetch, and the per-row notes carry the underlying facts either way.
- T5.1 F3 (arm-2 input contract: amounts and all transaction IDs omitted):
  ADOPTED. Arm 2's input is `RawHistoryContext` exactly as hashed: outcome
  kind + status, signal, source, brand/line/SKU/size/category, reason
  category + text + confidence, write op, seed mark, note. No `AmountMinor`,
  no transaction IDs. Rationale: the pilot measured this exact contract
  (5/5 right-size), so changing it post-pilot would unmoor the baseline.
  Consequence, frozen with it: live arm-2 replies can cite only
  note-substring refs through the human mapping step; a verbatim
  transaction ID in an arm-2 reply matches no arm input and is graded as an
  unresolved cite (a hallucination signal), never repaired by hand.
- T5.1 F4 (arm-3 stub is not the `recall_for_purchase` wire shape):
  ADOPTED. The stub carries no belief ids, values, guidance, or pending
  count, and T5.5 must not grade from it. Every arm-3 turn fetches memory
  through the real `recall_for_purchase` (plus `pending_questions` where the
  recall representation carries it); the tool request (shopper + brand) and
  the full response are recorded in the run file, and the response text is
  what the prompt's memory block carries.
- T5.2 extraction rule (no pinned reply-to-Size/Cites rule): ADOPTED.
  `bench/pilot/pilot.go` as hashed is the rule: deterministic size
  (explicit `size X`, then standalone vocabulary token, then empty), one
  documented human override only when the deterministic pick is judged not
  to be the reply's recommendation (reason recorded in the run file);
  deterministic cites are verbatim history transaction IDs only (SKU/size
  mentions are never cites); human cites map prose to a note-substring ref
  resolving to exactly one row (quote + reason recorded; unmappable prose
  is not a cite); extraction never consults the hidden answers; the grade
  command refuses prompts carrying hidden rationale strings.
- Live-tokenizer re-measurement note (words run ~5x under live BPE):
  ADOPTED with the limitation stated. The trace API exposes no live BPE
  counts, so re-measurement is impossible from the pilot path (pilot
  README). Frozen rule: every score carries `TokenBasis harness-words-v1`
  and platform cost side by side, never mixed; the ~5x factor applies to
  all arms equally, so cross-arm token comparisons hold on the frozen
  basis.
- $5-cap code enforcement (declared, not enforced cumulatively): DEFERRED
  to T5.5 (owner: the T5.5 worker), rationale: this task owns only this
  file and cannot add enforcement code. Frozen until then: per-run caps
  enforced in code (`DefaultCaps`), plus the pilot's manual procedure every
  run batch (re-check billing before and after, keep the running total in
  the batch record, stop and report if the $5.00 cumulative cap is near).
  T5.5 must carry cumulative spend in code or in its batch log, not in
  memory.
- Probe-turn archiving (probe spend committed as marked run files):
  ADOPTED. The pilot's two contaminated probe turns are committed
  (`pilot-a1-broken-discarded-1/2.json`, never graded). T5.5 commits every
  probe, discarded, and capped turn the same way: marked run files with the
  reason recorded, never silently dropped.
- T5.3 neutral-vs-reference instrument question: ADOPTED — the neutral
  instrument (`bench/pilot/agent/`, deployment `pilot-baseline`) drives all
  three arms. Rationale: the reference agent's `shopper_01` default
  contaminated 2/2 pilot turns through it with plausible, right-sized, but
  wrong-memory replies; silent wrong-memory recall is the one failure the
  comparison cannot afford. The neutral instrument cannot recall at all, so
  contamination is impossible by construction and checked per reply anyway
  (`CheckContamination`: any demo-shopper token discards the run). Parity
  holds where it matters: same model string, same gateway, prompts verbatim
  from the harness differing only in the memory block. Arm-3 memory is
  fetched by the driver through the real layer calls and carried in the
  prompt's memory block, with the request target and full response in the
  run file (section 3 of F4). The reference agent as driver is rejected:
  per-turn recall-target verification is weaker than an instrument that
  cannot mis-recall, and T5.5 may not relitigate this without a new freeze.
- No-hardening decision (arm 2 scored 5/5 on n=1): ADOPTED. No shopper is
  added or changed: rebuilding the set on one run per cell would overfit it
  to defeat the baseline, the baseline earned its score by correct causal
  reasoning (damage is not fit, never-arrived is not fit, changed-mind and
  vague count toward nothing), and the lead is recorded elsewhere (tokens:
  arm-2 prompts run ~2.5x arm-1 words; cites: 7 of 9 arm-2 cites need human
  prose-mapping and resolve to non-fit rows, while arm-3 cites resolve
  mechanically to belief and evidence ids). Hardening candidates stay
  recorded, not built: histories long enough that signal drowns in noise,
  and multi-brand histories where one brand's evidence tempts another
  brand's recommendation. Both would go through the seed generator path with
  their own hidden answers, never by editing the frozen fixtures.

## 5. No arm-3 run exists before this commit

Checked over `bench/` at this commit:

- `bench/runs/` (the T5.5 run directory) does not exist.
- No JSON file under `bench/` names arm 3: the twelve pilot run files name
  only arms 1 and 2 (ten graded, two discarded contaminated arm-1 turns).
  The two discarded files mention `recall_for_purchase` only as the
  wrong-shopper tool span that disqualified them.
- The string `ArmMemento` appears only in harness code and its offline
  tests. Those tests execute all three arms in-memory with scripted replies
  (no model, no key, no network) and persist nothing: no run file, no
  score, no transcript. That is a prompt-construction assertion, not a
  measurement run, and it grades nothing.
- The pilot measured arms 1 and 2 only, one live turn per shopper per arm.

Reproduce: `ls bench/runs` (must fail) and
`grep -rln '"arm": 3' bench/ --include='*.json'` (must print nothing).

## 6. Deviation: the verifier is a procedure, not a gate-run script

Done-when asks for a script that verifies the hashes with the gate running
it. That path does not exist inside this task's owns (`bench/FROZEN.md`
only): a runnable script would be a new file under `bench/`, outside owns,
and `tools/check.sh` runs gofmt/vet/tests/svelte but never executes
markdown. Creating the file anyway would break rule 2 of the protocol, so
the closest compliant alternative is implemented here: section 1 is a
`sha256sum`-format manifest with exact recomputation commands any human or
CI step can run offline. The gap for the orchestrator: mint a follow-up
task owning `bench/freeze_check.sh` (generated from this manifest, run from
`tools/check.sh`) or extend the gate to `sha256sum -c` this manifest
directly. Until then, T5.5's first step is to run the section-1 command and
paste its output in the run record.
