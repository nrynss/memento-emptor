"""Check 7: the ledger reads a session chosen per request, from a fixed list.

`lambo serve-web` resolves the session from the request path (`/s/<session>/`),
so one reader process serves several shoppers, but its allowlist is fixed at
start: a session created after it started is the uniform 404 until a restart
names it.

Run:  LAMBO_BIN=... LAMBO_MULTI_TOKEN=... python3 check7.py
"""

from __future__ import annotations

import http.client
import json
import os
import sys
import urllib.parse

import harness
import mcplib

SESSIONS = ["shopper-alpha", "shopper-bravo", "shopper-late"]


def get(port: int, path: str) -> tuple[int, bytes, dict]:
    conn = http.client.HTTPConnection("127.0.0.1", port, timeout=30)
    conn.request("GET", path, headers={"Host": f"127.0.0.1:{port}"})
    resp = conn.getresponse()
    body = resp.read()
    headers = {k.lower(): v for k, v in resp.getheaders()}
    conn.close()
    return resp.status, body, headers


def main() -> int:
    harness.check(bool(harness.BINARY), "LAMBO_BIN is not set")
    harness.check(bool(harness.TOKEN), "LAMBO_MULTI_TOKEN is not set")
    run = harness.run_dir("check7")
    store = os.path.join(run, "lambo.db")
    cfg = harness.base_config(os.path.join(run, "lambo.toml"), store)
    with open(cfg, "a") as fh:
        fh.write(harness.serve_section("shopper-pinned", max_attached=8))
        fh.write("\n[web]\nlist_sessions = true\n")
    tr = harness.Transcript(os.path.join(run, "transcript.jsonl"))
    report: dict = {"check": 7}

    hub = harness.start_hub(cfg, "shopper-pinned", run)
    beliefs = {}
    driven = {}
    for session in SESSIONS:
        client = harness.client(session, port=hub.port)
        client.initialize()
        tools = mcplib.ToolSession(client, "memento-t011", tr)
        driven[session] = tools
        beliefs[session] = f"{session}: Brand X oxford fits true to size"
        tools.write(tools.derive(beliefs[session], "logic"))

    # The reader loads the durable store: wait until every session's concept
    # is committed to the database, so this check measures the web allowlist,
    # not flush lag.
    import sqlite3

    def durable() -> bool:
        conn = sqlite3.connect(f"file:{store}?mode=ro", uri=True)
        try:
            rows = dict(conn.execute(
                "SELECT session_id, COUNT(*) FROM concepts GROUP BY session_id").fetchall())
        finally:
            conn.close()
        return all(rows.get(s, 0) == 1 for s in SESSIONS)

    harness.wait_until(durable, timeout_s=30, interval_s=0.1)

    # Two of the three served; the third exists in the store.
    web = harness.start_web(cfg, SESSIONS[:2], run)
    tr.add(event="web_started", port=web.port, served=SESSIONS[:2])

    results = {}

    def record(name: str, status: int, body: bytes, extra: dict | None = None):
        payload = body[:1500].decode("utf-8", "replace")
        results[name] = {"status": status, "body": payload}
        if extra:
            results[name].update(extra)
        tr.add(event="http", route=name, status=status, body=payload)

    try:
        status, body, _ = get(web.port, "/healthz")
        record("healthz", status, body)

        status, body, _ = get(web.port, "/api/sessions")
        listing = json.loads(body) if status == 200 else {}
        record("api_sessions", status, body, {"sessions": listing.get("sessions")})

        for session in SESSIONS:
            focus = beliefs[session]
            status, body, _ = get(
                web.port, f"/s/{session}/api/inspect?focus={urllib.parse.quote(focus)}")
            if status == 200:
                parsed = json.loads(body)
                record(f"inspect_{session}", status, body,
                       {"found": parsed.get("found"), "session": session})
            else:
                record(f"inspect_{session}", status, body, {"session": session})

        # The same focus read from another served session is a clean miss.
        status, body, _ = get(
            web.port,
            f"/s/shopper-bravo/api/inspect?focus={urllib.parse.quote(beliefs[SESSIONS[0]])}")
        record("cross_inspect", status, body,
               {"found": json.loads(body).get("found") if status == 200 else None})

        # The unscoped alias reads the default session (the first --session).
        status, body, _ = get(
            web.port, f"/api/inspect?focus={urllib.parse.quote(beliefs[SESSIONS[0]])}")
        record("alias_inspect", status, body,
               {"found": json.loads(body).get("found") if status == 200 else None})

        # A session that exists but is not on the start-time list stays a 404.
        status, body, _ = get(
            web.port,
            f"/s/shopper-late/api/inspect?focus={urllib.parse.quote(beliefs[SESSIONS[2]])}")
        record("unserved_session", status, body)

        web.stop()
        # Restart is the only way to add it: the allowlist is start-time state.
        web2 = harness.start_web(cfg, [SESSIONS[2]], run, name="web-restarted")
        try:
            status, body, _ = get(
                web2.port,
                f"/s/shopper-late/api/inspect?focus={urllib.parse.quote(beliefs[SESSIONS[2]])}")
            record("after_restart", status, body,
                   {"found": json.loads(body).get("found") if status == 200 else None})
        finally:
            web2.stop()
    finally:
        if web.proc.poll() is None:
            web.stop()
        hub.stop()

    report["results"] = {k: {kk: vv for kk, vv in v.items() if kk != "body"}
                         for k, v in results.items()}
    report["served"] = SESSIONS[:2]
    report["unserved"] = SESSIONS[2]
    harness.check(results["healthz"]["status"] == 200, "the reader is not healthy")
    harness.check(results["api_sessions"]["sessions"] == SESSIONS[:2],
                  f"the listing is not the served pair: {results['api_sessions']}")
    harness.check(results["inspect_shopper-alpha"]["status"] == 200
                  and results["inspect_shopper-alpha"]["found"] is True,
                  "the served session did not read")
    harness.check(results["inspect_shopper-bravo"]["status"] == 200
                  and results["inspect_shopper-bravo"]["found"] is True,
                  "the second served session did not read")
    harness.check(results["cross_inspect"]["found"] is False,
                  "a cross-session read found the other session's belief")
    harness.check(results["alias_inspect"]["found"] is True,
                  "the unscoped alias did not read the default session")
    harness.check(results["unserved_session"]["status"] == 404,
                  f"a session outside the allowlist was readable: "
                  f"{results['unserved_session']['status']}")
    harness.check(results["after_restart"]["found"] is True,
                  "a restart with the session named did not serve it")

    tr.close()
    with open(os.path.join(run, "report.json"), "w") as fh:
        json.dump(report, fh, indent=2)
    print(json.dumps(report, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
