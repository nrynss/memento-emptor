"""Check 1b: the bounds on a hub — eviction at max_attached and idle detach.

An attach that finds `max_attached` sessions attached evicts the least
recently used idle one; an on-demand session unused for `idle_detach_secs` is
detached by the sweeper. Both leave the session's data in the store: the next
request re-attaches it.

Run:  LAMBO_BIN=... LAMBO_MULTI_TOKEN=... python3 check1b_eviction.py
"""

from __future__ import annotations

import json
import os
import sys
import time

import harness
import mcplib

MAX_ATTACHED = 4
IDLE_SECS = 2


def attach(port: int, session: str, agent: str = "memento-t011"):
    client = harness.client(session, port=port)
    client.initialize()
    return client


def detaches_with(log: str, reason: str) -> list[str]:
    """Detach log lines for one reason (the value is rendered quoted)."""
    return [
        line for line in log.splitlines()
        if "detaching a session" in line
        and (f'reason="{reason}"' in line or f"reason={reason}" in line)
    ]


def main() -> int:
    harness.check(bool(harness.BINARY), "LAMBO_BIN is not set")
    harness.check(bool(harness.TOKEN), "LAMBO_MULTI_TOKEN is not set")
    run = harness.run_dir("check1b")
    store = os.path.join(run, "lambo.db")
    cfg = harness.base_config(os.path.join(run, "lambo.toml"), store)
    with open(cfg, "a") as fh:
        fh.write(harness.serve_section(
            "shopper-pinned", max_attached=MAX_ATTACHED, idle_s=IDLE_SECS))
    tr = harness.Transcript(os.path.join(run, "transcript.jsonl"))
    report: dict = {"check": "1b", "max_attached": MAX_ATTACHED,
                    "idle_detach_secs": IDLE_SECS}

    hub = harness.start_hub(cfg, "shopper-pinned", run)
    try:
        sessions = [f"shopper-{name}" for name in "abc"]
        for session in sessions:
            client = attach(hub.port, session)
            tools = mcplib.ToolSession(client, "memento-t011", tr)
            tools.write(tools.derive(f"{session} keeps one concept about Brand E", "logic"))
        tr.add(event="attached_at_limit", sessions=sessions)

        # One place is taken by the pinned session, so the three above fill
        # max_attached = 4. A fourth needs an eviction, and one is only
        # possible once a victim has been idle for 2 s.
        time.sleep(2.5)
        evictor = None
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            try:
                evictor = attach(hub.port, "shopper-d")
                break
            except RuntimeError as exc:
                if "503" not in str(exc):
                    raise
                time.sleep(0.5)
        harness.check(evictor is not None, "a fourth on-demand session never attached")
        time.sleep(0.5)
        log = hub.log_text()
        evicted = detaches_with(log, "evicted")
        report["eviction"] = {
            "attach_after_limit": "shopper-d",
            "detach_lines": evicted,
        }
        harness.check(bool(evicted),
                      "an attach at max_attached did not evict an idle session")

        # The evicted session's concept is still in the store: a new request
        # re-attaches it and reads its own concept back.
        again = None
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            try:
                again = attach(hub.port, sessions[0])
                break
            except RuntimeError as exc:
                if "503" not in str(exc):
                    raise
                time.sleep(0.5)
        harness.check(again is not None, "the evicted session never re-attached")
        tools = mcplib.ToolSession(again, "memento-t011", tr)
        recall = tools.recall("Brand E concept")
        report["reattach_after_eviction"] = {
                "session": sessions[0],
                "concept_read_back": sessions[0] in recall,
                "content": recall[:200],
        }
        harness.check(sessions[0] in recall,
                      "a re-attached session lost its concept")

        # Idle detach: stop touching the sessions and let the sweeper run.
        time.sleep(IDLE_SECS + 3)
        log = hub.log_text()
        idle = detaches_with(log, "idle")
        report["idle_detach"] = {"detach_lines": idle}
        harness.check(bool(idle), "the idle sweeper detached nothing")
    finally:
        hub.stop()
        tr.add(event="hub_stopped", exit=hub.proc.returncode,
               log=hub.log_text()[-2500:])

    tr.close()
    with open(os.path.join(run, "report.json"), "w") as fh:
        json.dump(report, fh, indent=2)
    print(json.dumps(report, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
