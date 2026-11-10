"""Check 4: the Elastic recall tier with many sessions.

One `[recall]` table, one index per embedding contract, every session's
documents beside each other, and every query filtered by `session_id`. Live
Elastic, test indices only, prefix ``t011-multi-*``; deleted at the end.

The fixture embedder computes vectors in-process, so no embedding API is
called and the spend is $0. The tier mirrors those vectors to Elastic.

Run:  LAMBO_BIN=... LAMBO_MULTI_TOKEN=... ELASTIC_URL=... LAMBO_ES_API_KEY=... \
      python3 check4.py
"""

from __future__ import annotations

import json
import os
import sys
import time

import harness
import mcplib

SESSIONS = ["shopper-" + name for name in ("alpha", "bravo", "charlie")]
TOKENS = {
    SESSIONS[0]: "Zephyrline",
    SESSIONS[1]: "Quarrystone",
    SESSIONS[2]: "Marlinspike",
}


def main() -> int:
    harness.check(bool(harness.BINARY), "LAMBO_BIN is not set")
    harness.check(bool(harness.TOKEN), "LAMBO_MULTI_TOKEN is not set")
    harness.check("ELASTIC_URL" in os.environ, "ELASTIC_URL is not set")
    harness.check("LAMBO_ES_API_KEY" in os.environ, "LAMBO_ES_API_KEY is not set")

    prefix = "t011-multi-" + os.urandom(3).hex()
    run = harness.run_dir("check4")
    store = os.path.join(run, "lambo.db")
    cfg = harness.base_config(
        os.path.join(run, "lambo.toml"), store,
        harness.recall_section(os.environ["ELASTIC_URL"], prefix),
    )
    with open(cfg, "a") as fh:
        fh.write(harness.serve_section("shopper-pinned", max_attached=16))
    tr = harness.Transcript(os.path.join(run, "transcript.jsonl"))
    report: dict = {"check": 4, "index_prefix": prefix, "sessions": SESSIONS}

    tr.add(event="start", index_prefix=prefix,
           note="fixture embedder, so no embedding API call and no spend")
    existing = harness.es_indices(f"{prefix}*")
    harness.check(not existing, f"the test prefix already has indices: {existing}")

    hub = harness.start_hub(cfg, "shopper-pinned", run)
    driven = {}
    try:
        for session in SESSIONS:
            client = harness.client(session, port=hub.port)
            client.initialize()
            tools = mcplib.ToolSession(client, "memento-t011", tr)
            driven[session] = tools
            token = TOKENS[session]
            tools.write(tools.derive(
                f"{token} oxford in Brand V runs one size small for this shopper", "logic"))
            tools.write(tools.derive(
                f"{session} keeps {token} as its own brand record", "observation"))
            tools.write(tools.derive(
                f"{token} returns came back too tight in the shoulders", "observation"))

        # The data index and the marker index appear under the test prefix.
        def indices():
            found = harness.es_indices(f"{prefix}*")
            return found if len(found) >= 2 else None

        found = harness.wait_until(indices, timeout_s=90)
        report["indices"] = found
        data_indices = [i for i in found if "-v-" in i]
        harness.check(len(data_indices) == 1,
                      f"expected one data index per contract, got {data_indices}")
        data_index = data_indices[0]
        meta_index = next(i for i in found if i.endswith("-meta"))
        tr.add(event="indices", indices=found, data=data_index, meta=meta_index)

        # All three sessions in the one data index, each document filtered by
        # session_id; and one marker document per session.
        def docs():
            status, body = harness.es(
                "GET", f"/{data_index}/_search?size=100&_source_excludes=embedding")
            harness.check(status == 200, f"search {data_index}: {status} {body}")
            return body

        def all_docs_there():
            body = docs()
            per = {}
            for hit in body["hits"]["hits"]:
                per.setdefault(hit["_source"]["session_id"], 0)
                per[hit["_source"]["session_id"]] += 1
            return per if all(per.get(s, 0) == 3 for s in SESSIONS) else None

        per_session = harness.wait_until(all_docs_there, timeout_s=90)
        report["data_index"] = {
            "name": data_index,
            "documents_per_session": per_session,
            "total": sum(per_session.values()),
            "one_index_for_every_session": len(per_session) == len(SESSIONS),
        }
        tr.add(event="data_index", **report["data_index"])

        status, meta = harness.es(
            "GET", f"/{meta_index}/_search?size=100&_source_excludes=embedding")
        harness.check(status == 200, f"marker search: {meta}")
        markers = [h["_source"] for h in meta["hits"]["hits"]]
        report["markers"] = {
            "count": len(markers),
            "sessions": sorted(m["session_id"] for m in markers),
            "ids_are_hashes": all(len(h["_id"]) == 64 for h in meta["hits"]["hits"]),
        }
        tr.add(event="markers", **report["markers"])
        harness.check(set(SESSIONS) <= set(report["markers"]["sessions"]),
                      "a session has no marker document")

        # Recall stays inside its session on the tier: no pair leaks.
        pairs = []
        for session in SESSIONS:
            own = driven[session].recall(TOKENS[session])
            leaks = []
            for other in SESSIONS:
                if other == session:
                    continue
                stranger = driven[session].recall(
                    f"{TOKENS[other]} oxford Brand V")
                if TOKENS[other] in stranger:
                    leaks.append(other)
            pairs.append({"session": session,
                          "own_token_found": TOKENS[session] in own,
                          "leaked_from": leaks})
            tr.add(event="recall_pair", session=session,
                   own_token_found=TOKENS[session] in own, leaked_from=leaks)
        report["recall_isolation"] = pairs
        harness.check(all(p["own_token_found"] for p in pairs),
                      "a session's own recall missed its own concept")
        harness.check(not any(p["leaked_from"] for p in pairs),
                      f"recall crossed sessions: {pairs}")

        # Inspect stays inside its session on the tier too.
        inspections = []
        for session in SESSIONS:
            other = next(s for s in SESSIONS if s != session)
            text = driven[session].inspect(
                f"{TOKENS[other]} oxford in Brand V runs one size small for this shopper")
            miss = "no concept matching" in text
            inspections.append({"session": session, "other": other, "miss": miss})
            tr.add(event="inspect_pair", **inspections[-1])
        report["inspect_isolation"] = inspections
        harness.check(all(i["miss"] for i in inspections),
                      f"inspect crossed sessions: {inspections}")

        # The document shape, minus the vector, for the record.
        status, sample = harness.es(
            "GET", f"/{data_index}/_search?size=1&_source_excludes=embedding")
        report["sample_document"] = sample["hits"]["hits"][0]["_source"]
        tr.add(event="sample_document", **report["sample_document"])
    finally:
        hub.stop()

    # Delete every index this run created; show the empty listing.
    before = harness.es_indices(f"{prefix}*")
    report["deleted"] = before
    harness.es_delete(f"{prefix}*")
    after = harness.es_indices(f"{prefix}*")
    report["indices_after_delete"] = after
    tr.add(event="deleted", before=before, after=after)
    harness.check(after == [], f"test indices remain: {after}")

    # Nothing else under t011- is ours; list it for the record without touching it.
    report["other_t011_indices"] = [
        i for i in harness.es_indices("t011-*") if not i.startswith(prefix)
    ]
    report["spend_usd"] = 0.0
    report["spend_note"] = (
        "fixture embedder computes vectors in-process; the tier mirrors them, "
        "so no embedding endpoint was called"
    )
    tr.add(event="spend", usd=0.0, note=report["spend_note"])
    tr.close()

    with open(os.path.join(run, "report.json"), "w") as fh:
        json.dump(report, fh, indent=2)
    print(json.dumps(report, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
