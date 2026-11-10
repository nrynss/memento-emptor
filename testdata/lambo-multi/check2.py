"""Check 2: sessions on demand, and which names a request may address.

A session is created by the first request that names it under the serving
credential's prefix, in the running process. The addressed-name rule is
1..=128 bytes of [A-Za-z0-9._:-], not starting with '.', and a 64-character
hex payer hash fits it.

Run:  LAMBO_BIN=... LAMBO_MULTI_TOKEN=... python3 check2.py
"""

from __future__ import annotations

import hashlib
import json
import os
import subprocess
import sys

import harness
import mcplib

PREFIX = "shopper-"


def initialize_status(port: int, session: str) -> tuple[int, str]:
    client = harness.client(session, port=port)
    try:
        reply = client.initialize()
        return 200, reply["result"].get("instructions", "")[:120]
    except RuntimeError as exc:
        text = str(exc)
        # The client raises on a non-200; keep the status it saw.
        try:
            status = int(text.split("HTTP ")[1].split(":")[0])
        except (IndexError, ValueError):
            status = -1
        return status, text[:200]


def main() -> int:
    harness.check(bool(harness.BINARY), "LAMBO_BIN is not set")
    harness.check(bool(harness.TOKEN), "LAMBO_MULTI_TOKEN is not set")
    run = harness.run_dir("check2")
    store = os.path.join(run, "lambo.db")
    cfg = harness.base_config(os.path.join(run, "lambo.toml"), store)
    with open(cfg, "a") as fh:
        fh.write(harness.serve_section("shopper-pinned", max_attached=32))
    tr = harness.Transcript(os.path.join(run, "transcript.jsonl"))
    report: dict = {"check": 2, "names": [], "create_without_restart": None,
                    "create_capability": None}

    hub = harness.start_hub(cfg, "shopper-pinned", run)
    tr.add(event="hub_started", pid=hub.proc.pid, port=hub.port)

    payer_hash = hashlib.sha256(b"payer-id-fixture").hexdigest()
    harness.check(len(payer_hash) == 64, "the hash is not 64 hex characters")
    prefixed = PREFIX + payer_hash
    report["hash"] = {"payer_hash": payer_hash, "len": len(payer_hash),
                      "prefixed": prefixed, "prefixed_len": len(prefixed)}

    # A session that has never been written to: the first request creates it.
    before = hub.log_text()
    harness.check(prefixed not in before, "the session existed before the first request")
    client = harness.client(prefixed, port=hub.port)
    reply = client.initialize()
    harness.check(prefixed in reply["result"]["instructions"],
                  "initialize did not reach the new session")
    session_tools = mcplib.ToolSession(client, "memento-t011", tr)
    first_stats = session_tools.stats()
    harness.check(f"session '{prefixed}'" in first_stats,
                  f"stats named another session: {first_stats.splitlines()[0]}")
    harness.check("concepts=0" in first_stats,
                  f"a new session was not empty: {first_stats.splitlines()[1]}")
    session_tools.write(session_tools.derive(
        f"Payer hash {payer_hash[:8]} prefers Brand R in size M", "logic"))
    recall = session_tools.recall("Brand R size")
    harness.check(payer_hash[:8] in recall, "the new session's write did not read back")
    after = hub.log_text()
    report["create_without_restart"] = {
        "session": prefixed,
        "request": f"initialize at /mcp/s/{prefixed}",
        "first_stats_line": first_stats.splitlines()[0],
        "same_pid": hub.proc.poll() is None,
        "attach_logged": any("attached" in line and prefixed in line
                             for line in after.splitlines()),
        "wrote_and_read_back": payer_hash[:8] in recall,
    }
    tr.add(event="created_on_demand", **report["create_without_restart"])

    # The addressed-name matrix. Same running process, no restart.
    cases = [
        (prefixed, 200, "64-character hex hash behind the prefix (72 bytes)"),
        (payer_hash, 404, "bare hash: outside the credential's session_prefix scope"),
        ("shopper-" + "a" * 120, 200, "128 bytes, the documented maximum"),
        ("shopper-" + "a" * 121, 404, "129 bytes, one past the maximum"),
        ("shopper-Shopper:01_A.B-2", 200, "every allowed punctuation character"),
        ("shopper-a b", 404, "space"),
        ("shopper-a+b", 404, "plus"),
        ("shopper-%2Dencoded", 404, "percent sign (never decoded)"),
        ("shopper-\u00e4", 404, "non-ASCII"),
        ("other-a", 404, "well formed, outside the credential's prefix"),
        (".shopper-leading-dot", 404, "leading dot (fails the shape, like a prefix miss)"),
        ("shopper/slash", 404, "slash (unrouted path)"),
    ]
    for name, expected, why in cases:
        status, detail = initialize_status(hub.port, name)
        report["names"].append({"name": name, "len": len(name.encode()),
                                "expected": expected, "status": status, "why": why})
        tr.add(event="addressed_name", name=name, len=len(name.encode()),
               expected=expected, status=status, why=why, detail=detail)
        harness.check(status == expected, f"{name!r}: expected {expected}, got {status}")
    hub.stop()
    tr.add(event="hub_stopped", exit=hub.proc.returncode, log=hub.log_text()[-3000:])

    # `create` is a capability: without it an absent session is the uniform
    # 404, and an existing one still attaches on demand.
    run2 = harness.run_dir("check2-nocreate")
    store2 = os.path.join(run2, "lambo.db")
    cfg2 = harness.base_config(os.path.join(run2, "lambo.toml"), store2)
    with open(cfg2, "a") as fh:
        fh.write(harness.serve_section("shopper-pinned", max_attached=8))
        fh.write("\n")  # keep the section readable
    # Rewrite the credential without `create`.
    text = open(cfg2).read().replace("create = true", "create = false")
    with open(cfg2, "w") as fh:
        fh.write(text)
    existing = "shopper-existing-" + os.urandom(3).hex()
    harness.provision(cfg2)
    cli = subprocess.run(
        [harness.BINARY, "--config", cfg2, "derive", "--session", existing,
         "--agent", "memento-cli", "--content", "Brand S fits this shopper",
         "--kind", "logic"],
        capture_output=True, text=True,
    )
    harness.check(cli.returncode == 0, f"CLI derive failed: {cli.stderr[:500]}")
    tr.add(event="cli_created", session=existing, exit=cli.returncode)

    hub2 = harness.start_hub(cfg2, "shopper-pinned", run2, name="hub-nocreate")
    present, _ = initialize_status(hub2.port, existing)
    absent, detail = initialize_status(hub2.port, "shopper-absent-" + os.urandom(3).hex())
    report["create_capability"] = {
        "existing_session_status": present,
        "absent_session_status": absent,
        "absent_detail": detail,
        "note": "the uniform 404 never says whether the session exists",
    }
    tr.add(event="create_capability", **report["create_capability"])
    harness.check(present == 200, "an existing session did not attach without create")
    harness.check(absent == 404, "an absent session attached without create")
    hub2.stop()

    tr.close()
    with open(os.path.join(run, "report.json"), "w") as fh:
        json.dump(report, fh, indent=2)
    print(json.dumps(report, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
