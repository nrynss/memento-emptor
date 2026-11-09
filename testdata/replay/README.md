# The offline replay

The demo shopper's whole story, from the first refund webhook to the final
ledger state, as fixtures P2, P3 and P4 can each use alone. No key, no
network, no spend: the gate proves it (`TestReplayOffline` runs the
story with the key variables unset or empty and the proxies broken, and
`TestNoKeyMaterial` fails the build if any replayed fixture contains
key-shaped material).

## Files

| File | Who uses it alone | What it holds |
|---|---|---|
| `manifest.json` | everyone | The P0 recordings each fake replays, the key variables the replay asserts absent, and the two decide-and-record items below |
| `story.json` | everyone | The committed log: nine steps in order, fixed event times |
| `p2-inputs.json` | T2.1-T2.4 | The payment leg: six webhook deliveries, the order reads behind them (line items live here only), the refund reads (which never carry a reason), and the dispute read with its reason code |
| `p3-turns.json` | T3.1-T3.5 | The agent-visible turns: recall, correction, pending questions, the answered-but-still-pending reason, feedback, and the belief explanation, each inlining its frozen contract example |
| `p4-ledger.json` | T4.1-T4.4 | The final ledger: four belief rows and five evidence rows copied verbatim from the frozen contract snapshots, plus the one pending question left open |

## Where the fake/real seam lives

`internal/fakes/` owns the four interfaces — `PaymentSource`,
`ReasonClassifier`, `EvidenceStore`, `EvidenceGraph` — and the replay
that drives them. P2's real clients implement these interfaces; callers
code to the interfaces, so the swap from fake to real touches no call
site. P2 adopts the interfaces as-is or records the difference in its
handoff.

## What a fake does with an uncovered request

It fails with `ErrNoRecording`, loudly. A fake never invents an answer:
an unknown webhook id, order, refund, dispute, answer text, search query,
graph content or recall query is an error matched with `errors.Is`,
never a guess. The unknown-request rule is tested per fake.

## The pending refund: answered, and left pending

`record_reason`'s accepted answer would add a fourth fit row the
contract snapshots predate. Contracts freeze at the end of P1, so the
replay must not move the frozen state. The replay therefore answers the
pending refund `q_9f04ab` with the recorded vague phrase "not great":
`unclear` at 0.94, under the 0.98 floor, `ask_again` true. The question
stays open, `pending_count` stays 1, and the sizing belief keeps exactly
its three documented returns. This also exercises the ask-again path
live, against a genuine recording, instead of only in the classifier's
table test.
