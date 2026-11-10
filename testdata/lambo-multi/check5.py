"""Check 5: isolation at scale — 20 sessions with overlapping brands.

Every session holds five concepts about the same two brands. For every ordered
pair (380 of them) recall and inspect from one session must never reach the
other's concept. A positive control (own recall and own inspect) proves the
probe would have found a leak.

Run:  LAMBO_BIN=... LAMBO_MULTI_TOKEN=... python3 check5.py
"""

from __future__ import annotations

import json
import os
import sys

import harness
import mcplib

SESSIONS = [f"shopper-{i:02d}" for i in range(20)]
PER_SESSION = 5


def marker(session: str) -> str:
    """A word unique to one session, in a sentence that still shares brands."""
    return "Marker" + session.replace("-", "").upper()


def concepts(session: str) -> list[str]:
    tag = marker(session)
    return [
        f"{tag}: Brand X oxford runs small for this shopper, take size L",
        f"{tag}: Brand X chinos fit true to size for this shopper",
        f"{tag}: Brand Y shirt sleeves are too short for this shopper",
        f"{tag}: Brand Y jacket in size M fits across the shoulders",
        f"{tag}: shopper usually buys size M in every brand",
    ]


def main() -> int:
    harness.check(bool(harness.BINARY), "LAMBO_BIN is not set")
    harness.check(bool(harness.TOKEN), "LAMBO_MULTI_TOKEN is not set")
    run = harness.run_dir("check5")
    store = os.path.join(run, "lambo.db")
    cfg = harness.base_config(os.path.join(run, "lambo.toml"), store)
    with open(cfg, "a") as fh:
        fh.write(harness.serve_section("shopper-pinned", max_attached=32))
    tr = harness.Transcript(os.path.join(run, "transcript.jsonl"))
    report: dict = {"check": 5, "sessions": len(SESSIONS),
                    "concepts_each": PER_SESSION}

    hub = harness.start_hub(cfg, "shopper-pinned", run)
    driven = {}
    try:
        for session in SESSIONS:
            client = harness.client(session, port=hub.port)
            client.initialize()
            tools = mcplib.ToolSession(client, "memento-t011", None)
            for content in concepts(session):
                tools.write(tools.derive(content, "observation"))
            driven[session] = tools
        # One process, 20 sessions: the hub logs the attached count.
        report["hub_attached"] = [
            line for line in hub.log_text().splitlines()
            if "attached" in line
        ][-3:]
        harness.check(hub.proc.poll() is None, "the hub exited while seeding")

        stats = {}
        for session, tools in driven.items():
            text = tools.stats()
            line = next(l for l in text.splitlines() if "concepts=" in l)
            stats[session] = line
        report["stats"] = stats
        harness.check(all(f"concepts={PER_SESSION}" in l for l in stats.values()),
                      f"a session lost a concept: {stats}")

        # Positive control, then every ordered pair.
        controls = []
        leaks = []
        calls = 0
        for session, tools in driven.items():
            tag = marker(session)
            own = tools.recall(tag)
            calls += 1
            controls.append({"session": session, "own_recall": tag in own})
            harness.check(tag in own, f"{session}: own recall missed its marker")
            own_inspect = tools.inspect(concepts(session)[0])
            harness.check("no concept matching" not in own_inspect,
                          f"{session}: own inspect missed its belief")
            for other, other_tools in driven.items():
                if other == session:
                    continue
                other_tag = marker(other)
                stranger = tools.recall(f"{other_tag} Brand X oxford")
                calls += 1
                if other_tag in stranger:
                    leaks.append({"from": other, "into": session,
                                  "why": "recall"})
                miss = tools.inspect(concepts(other)[0])
                calls += 1
                if "no concept matching" not in miss:
                    leaks.append({"from": other, "into": session,
                                  "why": "inspect"})
        report["positive_control"] = all(c["own_recall"] for c in controls)
        report["pairs_checked"] = len(SESSIONS) * (len(SESSIONS) - 1)
        report["calls"] = calls
        report["leaks"] = leaks
        tr.add(event="isolation_matrix", sessions=len(SESSIONS),
               pairs_checked=report["pairs_checked"], calls=calls,
               positive_control=report["positive_control"], leaks=leaks)
        harness.check(not leaks, f"{len(leaks)} leaks: {leaks[:5]}")

        # Shared-brand recall returns only this session's rows.
        shared = []
        for session, tools in driven.items():
            text = tools.recall("Brand X oxford chinos sizing")
            foreign = [marker(o) for o in driven if o != session and marker(o) in text]
            shared.append({"session": session, "foreign_markers": foreign})
            if foreign:
                leaks.append({"session": session, "foreign": foreign})
        report["shared_brand_recall"] = shared
        tr.add(event="shared_brand_recall", results=shared)
        harness.check(not any(s["foreign_markers"] for s in shared),
                      "a shared-brand recall returned another session's rows")
    finally:
        hub.stop()
        tr.add(event="hub_stopped", exit=hub.proc.returncode,
               log=hub.log_text()[-2000:])

    # The private database agrees: 20 sessions, five concepts each.
    import sqlite3

    conn = sqlite3.connect(f"file:{store}?mode=ro", uri=True)
    try:
        counts = dict(conn.execute(
            "SELECT session_id, COUNT(*) FROM concepts GROUP BY session_id").fetchall())
    finally:
        conn.close()
    report["db_counts"] = counts
    harness.check(len(counts) == len(SESSIONS), "the store holds a different session set")
    harness.check(all(c == PER_SESSION for c in counts.values()), "db counts disagree")

    tr.close()
    with open(os.path.join(run, "report.json"), "w") as fh:
        json.dump(report, fh, indent=2)
    print(json.dumps(report, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
