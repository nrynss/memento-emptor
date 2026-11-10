"""Check 3: many writers, one SQLite store.

Case A: eight stdio writers, one session each, one store file, writing at the
same time. Case B: eight sessions writing at once through one HTTP hub. Both
wait every receipt, then the committed database is counted directly. Case C:
a second writer on one held session proxies into the holder (one writer per
session, enforced by the holder's lease).

Run:  LAMBO_BIN=... LAMBO_MULTI_TOKEN=... python3 check3.py
"""

from __future__ import annotations

import json
import os
import sqlite3
import threading
import time

import harness
import mcplib

WRITERS = 8
WRITES_EACH = 20


def count_db(store: str) -> dict[str, int]:
    conn = sqlite3.connect(f"file:{store}?mode=ro", uri=True)
    try:
        rows = conn.execute(
            "SELECT session_id, COUNT(*) FROM concepts GROUP BY session_id"
        ).fetchall()
    finally:
        conn.close()
    return {session: count for session, count in rows}


def scan_log(path: str) -> dict:
    with open(path, errors="replace") as fh:
        text = fh.read()
    return {
        "sqlite_busy": text.count("SQLITE_BUSY"),
        "database_locked": text.lower().count("database is locked"),
        "write_failed": text.count("write failed"),
        "timeouts": text.lower().count("timed out"),
        "errors": sum(
            1 for line in text.splitlines()
            if " ERROR " in line or line.startswith("ERROR")
        ),
    }


def concurrent_writes(drive, sessions, label, tr):
    """Run WRITES_EACH writes per session at once; return per-session timings."""
    results: dict[str, dict] = {s: {"ok": 0, "failed": 0, "latencies": [],
                                    "error": None} for s in sessions}
    lock = threading.Lock()

    def worker(session, tools):
        try:
            for k in range(WRITES_EACH):
                t0 = time.monotonic()
                text = tools.derive(
                    f"{session} evidence {k}: Brand K oxford returned, size too small",
                    "observation",
                )
                tools.write(text)
                elapsed = time.monotonic() - t0
                with lock:
                    results[session]["ok"] += 1
                    results[session]["latencies"].append(round(elapsed, 3))
        except Exception as exc:  # noqa: BLE001 - the failure is the measurement
            with lock:
                results[session]["failed"] += 1
                results[session]["error"] = f"{type(exc).__name__}: {exc}"[:400]

    threads = [
        threading.Thread(target=worker, args=(session, drive[session]))
        for session in sessions
    ]
    t0 = time.monotonic()
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()
    wall = time.monotonic() - t0

    out = {"case": label, "writers": len(sessions), "writes_each": WRITES_EACH,
           "wall_s": round(wall, 2),
           "writes_total": sum(r["ok"] for r in results.values()),
           "failures": sum(r["failed"] for r in results.values()),
           "latency_s": {}}
    for session, row in results.items():
        lat = sorted(row["latencies"])
        if lat:
            out["latency_s"][session] = {
                "median": lat[len(lat) // 2],
                "max": lat[-1],
                "error": row["error"],
            }
        tr.add(event="writer_done", case=label, session=session,
               ok=row["ok"], failed=row["failed"], error=row["error"],
               median_s=lat[len(lat) // 2] if lat else None,
               max_s=lat[-1] if lat else None)
    return out


def main() -> int:
    harness.check(bool(harness.BINARY), "LAMBO_BIN is not set")
    harness.check(bool(harness.TOKEN), "LAMBO_MULTI_TOKEN is not set")
    run = harness.run_dir("check3")
    store = os.path.join(run, "lambo.db")
    cfg = harness.base_config(os.path.join(run, "lambo.toml"), store)
    harness.provision(cfg)
    tr = harness.Transcript(os.path.join(run, "transcript.jsonl"))
    report: dict = {"check": 3}

    # --- Case A: one process per session ---
    sessions_a = [f"w-{i:02d}" for i in range(WRITERS)]
    drivers_a = {}
    procs = []
    for session in sessions_a:
        stderr = os.path.join(run, f"stdio-{session}.log")
        proc = mcplib.StdioMCP(harness.BINARY, cfg, session, "memento-a", stderr)
        proc.initialize()
        procs.append((session, proc))
        drivers_a[session] = mcplib.ToolSession(proc, "memento-a", tr)
    report["case_a"] = concurrent_writes(drivers_a, sessions_a, "stdio-one-per-session", tr)
    per_session = {}
    for session, proc in procs:
        text = drivers_a[session].stats()
        per_session[session] = next(
            line for line in text.splitlines() if "concepts=" in line
        )
    report["case_a"]["stats"] = per_session
    report["case_a"]["logs"] = {}
    for session, proc in procs:
        proc.close()
        report["case_a"]["logs"][session] = scan_log(proc.stderr_path)
    report["case_a"]["db_counts"] = count_db(store)
    tr.add(event="case_a_result", stats=per_session,
           logs=report["case_a"]["logs"], db_counts=report["case_a"]["db_counts"])

    # --- Case B: one hub, eight sessions at once ---
    cfg_b = harness.base_config(os.path.join(run, "lambo-b.toml"),
                                os.path.join(run, "lambo-b.db"))
    with open(cfg_b, "a") as fh:
        fh.write(harness.serve_section("shopper-pinned", max_attached=32))
    hub = harness.start_hub(cfg_b, "shopper-pinned", run, name="hub")
    sessions_b = [f"shopper-{i:02d}" for i in range(WRITERS)]
    drivers_b = {}
    for session in sessions_b:
        client = harness.client(session, port=hub.port)
        client.initialize()
        drivers_b[session] = mcplib.ToolSession(client, "memento-b", tr)
    report["case_b"] = concurrent_writes(drivers_b, sessions_b, "one-hub-eight-sessions", tr)
    per_session_b = {}
    for session in sessions_b:
        text = drivers_b[session].stats()
        per_session_b[session] = next(
            line for line in text.splitlines() if "concepts=" in line
        )
    report["case_b"]["stats"] = per_session_b
    hub.stop()
    report["case_b"]["logs"] = scan_log(os.path.join(run, "hub.log"))
    report["case_b"]["db_counts"] = count_db(os.path.join(run, "lambo-b.db"))
    tr.add(event="case_b_result", stats=per_session_b,
           logs=report["case_b"]["logs"], db_counts=report["case_b"]["db_counts"])

    # --- Case C: one writer per session; a second process proxies to the holder ---
    cfg_c = harness.base_config(os.path.join(run, "lambo-c.toml"),
                                os.path.join(run, "lambo-c.db"))
    with open(cfg_c, "a") as fh:
        fh.write(harness.serve_section("shopper-pinned", max_attached=8))
    hub_c = harness.start_hub(cfg_c, "shopper-pinned", run, name="hub-c")
    held = "shopper-held"
    holder_client = harness.client(held, port=hub_c.port)
    holder_client.initialize()
    holder_tools = mcplib.ToolSession(holder_client, "memento-holder", tr)
    holder_tools.write(holder_tools.derive("Brand H fits the held session", "logic"))

    second_log = os.path.join(run, "second-writer.log")
    second = mcplib.StdioMCP(harness.BINARY, cfg_c, held, "memento-second", second_log)
    second.initialize()
    second_tools = mcplib.ToolSession(second, "memento-second", tr)
    second_tools.write(second_tools.derive(
        "Brand H second writer note: proxied into the holder", "observation"))
    via_proxy = second_tools.stats()
    exit_code = second.close()

    holder_stats = holder_tools.stats()
    concept_count = next(
        line for line in holder_stats.splitlines() if "concepts=" in line
    )
    with open(second_log, errors="replace") as fh:
        proxy_log = fh.read()
    report["case_c"] = {
        "second_writer_exit": exit_code,
        "second_process_is_proxy": "proxying to the session holder" in proxy_log,
        "second_process_took_no_lease": "no lease was ever taken" in proxy_log,
        "write_landed_in_holder": "proxied into the holder" in holder_tools.recall("Brand H"),
        "holder_stats": concept_count,
        "proxied_stats_owner_line": next(
            (line for line in via_proxy.splitlines() if line.startswith("session ")), ""),
        "stderr": proxy_log.strip().splitlines()[-3:],
    }
    tr.add(event="second_writer", **report["case_c"])
    hub_c.stop()

    tr.close()
    with open(os.path.join(run, "report.json"), "w") as fh:
        json.dump(report, fh, indent=2)
    print(json.dumps(report, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
