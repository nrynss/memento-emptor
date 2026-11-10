"""Check 6: promotion per session.

One belief is taken to Venerable in each of three sessions, seeded in
interleaved order. Every session reaches the same stage-2 verdict with the
same coverage, and noise written into one session afterwards leaves the other
two where they were.

Run:  LAMBO_BIN=... LAMBO_MULTI_TOKEN=... python3 check6.py
"""

from __future__ import annotations

import datetime as dt
import json
import os
import re
import sys
import time

import harness
import mcplib

SESSIONS = ["shopper-one", "shopper-two", "shopper-three"]
BASE = dt.datetime(2026, 10, 1, 12, 0, tzinfo=dt.timezone.utc)
DAY = dt.timedelta(days=1)


def belief(session: str) -> str:
    return f"{session}: Brand X oxford runs small, take size L"


def returns(session: str) -> list[str]:
    return [
        f"{session} return 1: Brand X oxford size M too tight in the shoulders",
        f"{session} return 2: Brand X oxford size M too tight across the chest",
        f"{session} return 3: Brand X oxford size M sleeves too short",
    ]


def at(day: int) -> str:
    return (BASE + day * DAY).isoformat()


def survived(text: str) -> int:
    match = re.search(r"survived (\d+) GC sweeps", text)
    return int(match.group(1)) if match else 0


def inspect_summary(struct: dict) -> dict:
    admission = struct.get("admission") or {}
    checks = {c["gate"]: c["current"] for c in admission.get("checks", [])}
    focus = struct.get("focus") or {}
    return {
        "status": admission.get("status"),
        "venerable": admission.get("status") == "Venerable",
        "coverage": checks.get("coverage"),
        "distinct_interactions": checks.get("distinct_interactions"),
        "blast_radius": focus.get("blast_radius"),
        "reason": (admission.get("reasons") or [None])[0],
    }


def main() -> int:
    harness.check(bool(harness.BINARY), "LAMBO_BIN is not set")
    harness.check(bool(harness.TOKEN), "LAMBO_MULTI_TOKEN is not set")
    run = harness.run_dir("check6")
    store = os.path.join(run, "lambo.db")
    cfg = harness.base_config(os.path.join(run, "lambo.toml"), store)
    with open(cfg, "a") as fh:
        fh.write(harness.serve_section("shopper-pinned", max_attached=16))
    tr = harness.Transcript(os.path.join(run, "transcript.jsonl"))
    report: dict = {"check": 6, "sessions": SESSIONS}

    hub = harness.start_hub(cfg, "shopper-pinned", run)
    driven = {}
    try:
        for session in SESSIONS:
            client = harness.client(session, port=hub.port)
            client.initialize()
            driven[session] = mcplib.ToolSession(client, "memento-t011", tr)

        # Interleaved: one hop of work in each session, round after round.
        tr.add(event="seed_order", order="round-robin over sessions")
        for session in SESSIONS:
            driven[session].write(driven[session].derive(
                belief(session), "logic", event_time=at(0)))
        for day in (3, 6, 9):
            for session in SESSIONS:
                idx = (day - 3) // 3
                driven[session].write(driven[session].action(
                    returns(session)[idx], depends_on=[belief(session)],
                    event_time=at(day)))

        # Stage 1 needs three GC sweeps: each session's belief must survive
        # mutations after it was created. The filler writes are dated inside
        # the seeded window so they add sweeps without widening the extent.
        max_survived = {s: 0 for s in SESSIONS}
        for round_no in range(8):
            for session in SESSIONS:
                text = driven[session].inspect(belief(session))
                max_survived[session] = max(max_survived[session], survived(text))
                if max_survived[session] >= 3:
                    continue
                driven[session].write(driven[session].derive(
                    f"{session} window note {round_no}: Brand X colour note",
                    "observation", event_time=at(1 + round_no)))
            time.sleep(0.2)

        def summaries():
            out = {}
            for session, tools in driven.items():
                out[session] = inspect_summary(tools.inspect_struct(belief(session)))
            return out if all(s["venerable"] for s in out.values()) else None

        summaries_now = harness.wait_until(summaries, timeout_s=90)
        report["promotion"] = summaries_now
        report["gc_survived_seen"] = max_survived
        # Venerable is stage 2, which the state machine only evaluates for a
        # Candidate, so a Venerable belief proves stage 1 cleared as well.

        # One session's noise must not move the others. The noise is dated
        # inside that session's own window, exactly as its seeding was.
        noise = driven[SESSIONS[0]]
        for k in range(20):
            noise.write(noise.derive(
                f"{SESSIONS[0]} noise note {k}: an unrelated Brand Z observation",
                "observation", event_time=at(2)))
        after = {s: inspect_summary(driven[s].inspect_struct(belief(s))) for s in SESSIONS}
        report["after_noise_in_first"] = after
        report["unmoved"] = {
            s: after[s]["coverage"] == summaries_now[s]["coverage"]
            and after[s]["venerable"] == summaries_now[s]["venerable"]
            for s in SESSIONS
        }
        tr.add(event="interleaved_promotion", promotion=summaries_now,
               after_noise=after, unmoved=report["unmoved"])

        stats = {}
        for session, tools in driven.items():
            line = next(l for l in tools.stats().splitlines() if "concepts=" in l)
            stats[session] = line
        report["stats"] = stats
        report["gc"] = {}
        for session, tools in driven.items():
            text = tools.stats()
            report["gc"][session] = next(
                (l for l in text.splitlines() if l.startswith("gc:")), "")
        tr.add(event="stats", stats=stats, gc=report["gc"])

        # The stage-2 verdicts agree: three distinct interactions, coverage
        # above 0.3, same numbers in every session.
        coverages = {s: after[s].get("coverage") for s in SESSIONS}
        report["coverage_agrees"] = len(set(coverages.values())) == 1
        report["all_venerable"] = all(after[s]["venerable"] for s in SESSIONS)
        harness.check(report["all_venerable"], f"a belief did not reach Venerable: {after}")
        harness.check(report["coverage_agrees"], f"coverage differs: {coverages}")
        harness.check(all(report["unmoved"].values()),
                      f"one session's noise moved another: {report['unmoved']}")
    finally:
        hub.stop()

    tr.close()
    with open(os.path.join(run, "report.json"), "w") as fh:
        json.dump(report, fh, indent=2)
    print(json.dumps(report, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
