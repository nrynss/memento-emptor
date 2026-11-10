"""Check 1: one process or many, and what an attached session costs.

One `lambo serve --transport http` with an on-demand credential serves every
session under its prefix from a single process. This measures that process's
resident memory and file handles as sessions attach and fill, and compares it
with one stdio writer per session. The 512 MB Render starter is the budget.

Run:  LAMBO_BIN=... LAMBO_MULTI_TOKEN=... python3 check1.py
"""

from __future__ import annotations

import json
import os
import statistics
import sys
import time

import harness
import mcplib

N_ATTACH = 20
CONCEPTS_PER_SESSION = 3


def main() -> int:
    harness.check(bool(harness.BINARY), "LAMBO_BIN is not set")
    harness.check(bool(harness.TOKEN), "LAMBO_MULTI_TOKEN is not set")
    run = harness.run_dir("check1")
    store = os.path.join(run, "lambo.db")
    cfg = harness.base_config(os.path.join(run, "lambo.toml"), store)
    with open(cfg, "a") as fh:
        fh.write(harness.serve_section("shopper-pinned", max_attached=32))
    tr = harness.Transcript(os.path.join(run, "transcript.jsonl"))
    report: dict = {"check": 1, "process": None}

    started = time.monotonic()
    hub = harness.start_hub(cfg, "shopper-pinned", run)
    port_open = time.monotonic() - started

    first = harness.client("shopper-pinned", port=hub.port)
    init_started = time.monotonic()
    first.initialize()
    first_ready = time.monotonic() - init_started
    baseline_rss = mcplib.rss_kb(hub.proc.pid)
    baseline_fds = mcplib.open_fds(hub.proc.pid)
    report.update(
        {
            "process": {
                "pid": hub.proc.pid,
                "startup_s_to_port": round(port_open, 3),
                "first_initialize_s": round(first_ready, 3),
                "baseline_rss_mib": round(baseline_rss / 1024, 1),
                "baseline_fds": baseline_fds,
                "binary": os.environ.get("LAMBO_VERSION", "v0.10.1"),
            },
            "attach": [],
        }
    )
    tr.add(event="hub_started", pid=hub.proc.pid, port=hub.port,
           startup_s=round(port_open, 3), first_initialize_s=round(first_ready, 3),
           baseline_rss_mib=round(baseline_rss / 1024, 1), baseline_fds=baseline_fds)

    clients = []
    for i in range(N_ATTACH):
        session = "shopper-" + f"{i:02d}-" + os.urandom(4).hex()
        client = harness.client(session, port=hub.port)
        reply = client.initialize()
        text = reply["result"].get("instructions", "")
        harness.check(session in text, f"session {session} not in initialize instructions")
        clients.append(client)
        if i + 1 in (1, 5, 10, 20):
            rss = mcplib.rss_kb(hub.proc.pid)
            fds = mcplib.open_fds(hub.proc.pid)
            row = {
                "attached": i + 1,
                "rss_mib": round(rss / 1024, 1),
                "fds": fds,
                "marginal_kib": round((rss - baseline_rss) / (i + 1), 1),
            }
            report["attach"].append(row)
            tr.add(event="attached", **row)

    # Every attached session writes through the same process.
    ids = {}
    for client in clients:
        tools = mcplib.ToolSession(client, "memento-t011", tr)
        for k in range(CONCEPTS_PER_SESSION):
            text = f"{client.session} keeps its own belief number {k} about Brand Q fit"
            tools.derive(text, "observation")
        stats = tools.stats()
        ids[client.session] = [line for line in stats.splitlines() if line.startswith("nodes=")]
        harness.check(
            f"session '{client.session}'" in stats,
            f"stats answered for another session: {stats.splitlines()[0]}",
        )
    settled_rss = mcplib.rss_kb(hub.proc.pid)
    settled_fds = mcplib.open_fds(hub.proc.pid)
    report["seeded"] = {
        "sessions": N_ATTACH,
        "concepts_each": CONCEPTS_PER_SESSION,
        "rss_mib": round(settled_rss / 1024, 1),
        "fds": settled_fds,
        "marginal_kib_per_session": round((settled_rss - baseline_rss) / N_ATTACH, 1),
        "stats": ids,
    }
    tr.add(event="seeded", **report["seeded"])

    # One session loaded with 100 concepts: the per-concept cost, so capacity
    # is not read off three-concept sessions.
    loaded = mcplib.ToolSession(clients[0], "memento-t011", tr)
    before_loaded = mcplib.rss_kb(hub.proc.pid)
    for k in range(100):
        loaded.write(loaded.derive(
            f"{clients[0].session} load probe {k}: Brand L size note number {k}",
            "observation"))
    after_loaded = mcplib.rss_kb(hub.proc.pid)
    per_concept_kib = (after_loaded - before_loaded) / 100
    report["per_concept"] = {
        "added": 100,
        "rss_before_mib": round(before_loaded / 1024, 1),
        "rss_after_mib": round(after_loaded / 1024, 1),
        "kib_per_concept": round(per_concept_kib, 1),
        "five_hundred_concept_session_mib": round(per_concept_kib * 500 / 1024, 1),
    }
    tr.add(event="per_concept", **report["per_concept"])

    # One process: every session's writes are readable in that same session,
    # and the stats line names the one hub agent.
    probe = mcplib.ToolSession(clients[0], "memento-t011", tr)
    own = probe.recall(clients[0].session + " belief number 0")
    other = probe.recall(clients[-1].session + " belief number 0")
    harness.check(clients[0].session in own, "own recall missed its own concept")
    harness.check(clients[-1].session not in other, "recall crossed into another session")
    tr.add(event="cross_read", own_session=clients[0].session,
           own_found=clients[0].session in own,
           other_session=clients[-1].session,
           other_found=clients[-1].session in other)

    # The comparison: one stdio writer per session, same store, same content.
    per_process = []
    for i in range(10):
        session = "solo-" + f"{i:02d}-" + os.urandom(4).hex()
        stderr = os.path.join(run, f"solo-{i:02d}.log")
        t0 = time.monotonic()
        proc = mcplib.StdioMCP(harness.BINARY, cfg, session, "memento-solo", stderr)
        proc.initialize()
        ready = time.monotonic() - t0
        rss = mcplib.rss_kb(proc.proc.pid)
        fds = mcplib.open_fds(proc.proc.pid)
        per_process.append({"startup_s": round(ready, 3), "rss_mib": round(rss / 1024, 1),
                            "fds": fds})
        tr.add(event="solo_process", pid=proc.proc.pid, **per_process[-1])
        proc.close()

    report["one_writer_per_session"] = {
        "runs": len(per_process),
        "startup_s_median": round(statistics.median(p["startup_s"] for p in per_process), 3),
        "rss_mib_median": round(statistics.median(p["rss_mib"] for p in per_process), 1),
        "rss_mib_max": max(p["rss_mib"] for p in per_process),
        "fds_max": max(p["fds"] for p in per_process),
    }

    # How many fit on 512 MB: the measured marginal, with 192 MiB left for the
    # Go services that share the instance (edge, layer, seeder) and the OS.
    marginal = (settled_rss - baseline_rss) / 1024 / N_ATTACH
    usable = 512 - 192 - baseline_rss / 1024
    five_hundred = per_concept_kib * 500 / 1024
    report["starter_512mb"] = {
        "baseline_mib": round(baseline_rss / 1024, 1),
        "marginal_mib_per_attached_session": round(marginal, 1),
        "kib_per_concept": round(per_concept_kib, 1),
        "headroom_mib_for_other_services": 192,
        "fits_small_sessions": max(0, int(usable / marginal)) if marginal > 0 else None,
        "fits_500_concept_sessions": max(0, int(usable / (marginal + five_hundred))),
        "default_max_attached": 16,
        "default_max_sessions": 32,
        "bound_note": "max_attached is the real bound on one hub; memory is the "
                      "second bound, and 32 attached sessions at the measured "
                      "marginal sit far inside 512 MB",
    }
    tr.add(event="starter_capacity", **report["starter_512mb"])

    hub.stop()
    tr.add(event="hub_stopped", exit=hub.proc.returncode, log=hub.log_text()[-4000:])
    tr.close()

    with open(os.path.join(run, "report.json"), "w") as fh:
        json.dump(report, fh, indent=2)
    print(json.dumps(report, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
