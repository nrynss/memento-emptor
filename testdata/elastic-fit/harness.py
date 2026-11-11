#!/usr/bin/env python3
"""Live harness for T0.12, the Elastic fit index.

Runs against the deployment named by ELASTIC_URL with the key in
LAMBO_ES_API_KEY (source tools/env.sh first). Every index it creates is named
with the t012- prefix and is deleted before it exits. Recordings land beside
this file with the deployment URL and cluster identifiers replaced by
placeholders.

    python3 harness.py                 # every check, end to end
    python3 harness.py --cold-one      # one fresh-connection timing sample

The population is generated here, not copied from another project. It is
deterministic: the truth manifest for the line-and-size probe is written next
to the rows it describes.
"""

from __future__ import annotations

import copy
import hashlib
import json
import fnmatch
import os
import shutil
import statistics
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path

import requests

HERE = Path(__file__).resolve().parent
BASE = os.environ["ELASTIC_URL"].rstrip("/")
KEY = os.environ["LAMBO_ES_API_KEY"]

FIT_INDEX = "t012-fit-rows"
POP_INDEX = "t012-fit-pop"
MODE_PROBE = "t012-fit-mode"
EXCLUDE_PROBE = "t012-fit-exclude"
LAMBO_PREFIX = "t012-lambo"
LAMBO_BIN = os.environ.get("LAMBO_BIN", "lambo")

SESSION = requests.Session()
HEADERS = {"Authorization": "ApiKey " + KEY}

LINE = {"brand": "sunspel", "category": "shirt", "fit_handle": "regular"}
SIZE = "M"

# ---------------------------------------------------------------- redaction


class Redactor:
    """Replace the deployment URL and cluster identifiers in recordings."""

    def __init__(self) -> None:
        self.values: list[str] = [BASE]
        root = self.get("/")
        if root.status_code == 200:
            body = root.json()
            for key in ("cluster_name", "cluster_uuid"):
                if body.get(key):
                    self.values.append(body[key])
        lic = self.get("/_license")
        if lic.status_code == 200:
            uid = lic.json().get("license", {}).get("uid")
            if uid:
                self.values.append(uid)
        # Longest first so a value that contains another is replaced whole.
        self.values.sort(key=len, reverse=True)

    def get(self, path: str, **kw):
        return SESSION.get(BASE + path, headers=HEADERS, timeout=60, **kw)

    def text(self, value):
        if isinstance(value, str):
            for secret in self.values:
                value = value.replace(secret, "<deployment>")
            return value
        if isinstance(value, list):
            return [self.text(v) for v in value]
        if isinstance(value, dict):
            return {k: self.text(v) for k, v in value.items()}
        return value


REDACT = None  # set in main once the cluster identifiers are known


# ---------------------------------------------------------------- requests


def call(method: str, path: str, body=None, params=None, raw=False, timeout=120):
    """One request, returning a recording of it and the live response."""
    url = BASE + path
    kwargs = {"headers": HEADERS, "timeout": timeout}
    if params:
        kwargs["params"] = params
    if body is not None:
        kwargs["json"] = body
    response = SESSION.request(method, url, **kwargs)
    recording = {
        "method": method,
        "path": path,
        "params": params or {},
        # Deep-copied: check 2 mutates one row dict through its states, and a
        # recorded body that shares it would show the purchase's final row at
        # every step.
        "request": copy.deepcopy(body),
        "status": response.status_code,
    }
    if raw:
        recording["response"] = response.text
    else:
        try:
            recording["response"] = response.json()
        except ValueError:
            recording["response"] = response.text
    return recording, response


def bulk(rows, chunk=200):
    """Index rows one document per purchase, each at external version 1."""
    body_parts = []
    for row in rows:
        body_parts.append(json.dumps({"index": {"_id": row["txn_hash"], "version": 1,
                                                "version_type": "external_gte"}}))
        body_parts.append(json.dumps(row, separators=(",", ":")))
    payload = "\n".join(body_parts) + "\n"
    response = SESSION.post(BASE + "/" + POP_INDEX + "/_bulk?refresh=true",
                            headers={**HEADERS, "Content-Type": "application/x-ndjson"},
                            data=payload, timeout=180)
    body = response.json()
    statuses = {}
    for item in body.get("items", []):
        status = next(iter(item.values()))["status"]
        statuses[status] = statuses.get(status, 0) + 1
    return {
        "method": "POST",
        "path": "/" + POP_INDEX + "/_bulk?refresh=true",
        "request": {
            "actions": len(rows),
            "version_type": "external_gte",
            "version": 1,
            "first_action": json.loads(body_parts[0]),
            "first_document": json.loads(body_parts[1]),
            "body_omitted": "one NDJSON action + full row per purchase",
        },
        "status": response.status_code,
        "response": {
            "took": body.get("took"),
            "errors": body.get("errors"),
            "items": len(body.get("items", [])),
            "status_counts": statuses,
        },
    }


def count(index: str, query=None):
    path = "/" + index + "/_count"
    if query is None:
        rec, resp = call("GET", path)
    else:
        rec, resp = call("POST", path, body={"query": query})
    return rec, resp


def search(index: str, body):
    rec, resp = call("POST", "/" + index + "/_search", body=body)
    return rec, resp


def refresh(index: str):
    return call("POST", "/" + index + "/_refresh")


# ---------------------------------------------------------------- mapping


def mapping() -> dict:
    """The exact mapping later tasks create: structured fields, no prose."""
    return {
        "dynamic": "strict",
        "properties": {
            "txn_hash": {"type": "keyword"},
            "shopper_hash": {"type": "keyword"},
            "brand": {"type": "keyword"},
            "category": {"type": "keyword"},
            "fit_handle": {"type": "keyword"},
            "size": {"type": "keyword"},
            "purchase_state": {"type": "keyword"},
            "settled": {"type": "boolean"},
            "settled_kind": {"type": "keyword"},
            "reason_category": {"type": "keyword"},
            "fit_direction": {"type": "keyword"},
            "feedback_verdict": {"type": "keyword"},
            "reason_contested": {"type": "boolean"},
            "seeded": {"type": "boolean"},
            "captured_at": {"type": "date"},
            "refunded_at": {"type": "date"},
            "settled_at": {"type": "date"},
            "event_seq": {"type": "integer"},
        },
    }


def hash_txn(txn_id: str) -> str:
    return hashlib.sha256(("txn|" + txn_id).encode()).hexdigest()


def hash_shopper(line: dict, shopper_id: str) -> str:
    # Salted per product line: the line identity is the salt prefix here; the
    # deployment adds a secret. The same shopper in two lines must not link.
    salt = "|".join([line["brand"], line["category"], line["fit_handle"]])
    return hashlib.sha256((salt + "#" + shopper_id).encode()).hexdigest()


BASE_TIME = datetime(2026, 8, 1, tzinfo=timezone.utc)


def ts(hours: int, days: int = 0) -> str:
    return (BASE_TIME + timedelta(hours=hours, days=days)).strftime("%Y-%m-%dT%H:%M:%SZ")


# ---------------------------------------------------------------- population


def row(line, size, *, txn_id, shopper_id, seeded, state, seq=1, verdict=None,
        reason=None, direction=None, contested=False):
    captured = 0
    refunded = None
    settled_at = None
    kind = None
    settled = False
    if state == "captured":
        pass
    elif state == "refunded":
        refunded = ts(72)
    elif state == "reason_classified":
        refunded = ts(72)
        settled = True
        kind = "return_reason"
        settled_at = ts(120)
    elif state == "feedback":
        settled = True
        kind = "feedback"
        settled_at = ts(168)
    elif state == "kept":
        settled = True
        kind = "kept"
        settled_at = ts(0, 30)
    else:
        raise ValueError(state)
    doc = {
        "txn_hash": hash_txn(txn_id),
        "shopper_hash": hash_shopper(line, shopper_id),
        "brand": line["brand"],
        "category": line["category"],
        "fit_handle": line["fit_handle"],
        "size": size,
        "purchase_state": state,
        "settled": settled,
        "seeded": seeded,
        "captured_at": ts(captured),
        "event_seq": seq,
    }
    if refunded is not None:
        doc["refunded_at"] = refunded
    if settled_at is not None:
        doc["settled_at"] = settled_at
    if kind is not None:
        doc["settled_kind"] = kind
    if reason is not None:
        doc["reason_category"] = reason
    if direction is not None:
        doc["fit_direction"] = direction
    if verdict is not None:
        doc["feedback_verdict"] = verdict
    if contested:
        doc["reason_contested"] = True
    return doc


# The truth for the probe line and size, kept by hand so the generator has to
# match it rather than the other way round.
TRUTH = {
    "line": {**LINE, "size": SIZE},
    "total_docs": 120,
    "settled": 100,
    "unsettled": {"captured": 12, "refunded": 8},
    "settled_kind": {"return_reason": 70, "feedback": 20, "kept": 10},
    "fit_returns_by_direction": {"too_small": 40, "too_large": 15},
    "feedback_by_verdict": {"fits_well": 12, "too_small": 5, "too_large": 3},
    "distinct_shoppers_by_direction": {"too_small": 31, "too_large": 12},
    "reason_category": {
        "fit_too_small": 40,
        "fit_too_large": 15,
        "changed_mind": 8,
        "damaged": 5,
        "not_as_described": 2,
    },
    "seeded": {
        "docs": 100,
        "settled": 85,
        "unsettled": {"captured": 10, "refunded": 5},
        "fit_returns_by_direction": {"too_small": 35, "too_large": 13},
        "feedback_by_verdict": {"fits_well": 10, "too_small": 4, "too_large": 3},
        "distinct_shoppers_by_direction": {"too_small": 27, "too_large": 11},
    },
    "live": {
        "docs": 20,
        "settled": 15,
        "unsettled": {"captured": 2, "refunded": 3},
        "fit_returns_by_direction": {"too_small": 5, "too_large": 2},
        "feedback_by_verdict": {"fits_well": 2, "too_small": 1, "too_large": 0},
        "distinct_shoppers_by_direction": {"too_small": 4, "too_large": 1},
    },
}


def build_population():
    rows = []
    counter = 0

    def add(line, size, seeded, state, **kw):
        nonlocal counter
        counter += 1
        rows.append(row(line, size, txn_id=f"txn-{counter:04d}", seeded=seeded,
                        state=state, **kw))

    # The probe line and size, counted exactly.
    for j in range(10):
        add(LINE, SIZE, True, "captured", shopper_id=f"l1m-un-s{j}")
    for j in range(5):
        add(LINE, SIZE, True, "refunded", shopper_id=f"l1m-un-r{j}")
    for j in range(35):
        add(LINE, SIZE, True, "reason_classified", shopper_id=f"l1m-small-seed-{j % 27}",
            reason="fit_too_small", direction="too_small")
    for j in range(13):
        add(LINE, SIZE, True, "reason_classified", shopper_id=f"l1m-large-seed-{j % 11}",
            reason="fit_too_large", direction="too_large")
    nonfit_seed = [("changed_mind", 6), ("damaged", 4), ("not_as_described", 2)]
    for reason, n in nonfit_seed:
        for j in range(n):
            add(LINE, SIZE, True, "reason_classified",
                shopper_id=f"l1m-nonfit-seed-{reason}-{j}", reason=reason)
    for verdict, n in [("fits_well", 10), ("too_small", 4), ("too_large", 3)]:
        for j in range(n):
            add(LINE, SIZE, True, "feedback", shopper_id=f"l1m-fb-seed-{verdict}-{j}",
                verdict=verdict, direction=None if verdict == "fits_well" else verdict)
    for j in range(8):
        add(LINE, SIZE, True, "kept", shopper_id=f"l1m-kept-seed-{j}")

    for j in range(2):
        add(LINE, SIZE, False, "captured", shopper_id=f"l1m-un-live-c{j}")
    for j in range(3):
        add(LINE, SIZE, False, "refunded", shopper_id=f"l1m-un-live-r{j}")
    for j in range(5):
        add(LINE, SIZE, False, "reason_classified", shopper_id=f"l1m-small-live-{j % 4}",
            reason="fit_too_small", direction="too_small")
    for j in range(2):
        add(LINE, SIZE, False, "reason_classified", shopper_id="l1m-large-live-0",
            reason="fit_too_large", direction="too_large")
    for reason, n in [("changed_mind", 2), ("damaged", 1)]:
        for j in range(n):
            add(LINE, SIZE, False, "reason_classified",
                shopper_id=f"l1m-nonfit-live-{reason}-{j}", reason=reason)
    for verdict, n in [("fits_well", 2), ("too_small", 1)]:
        for j in range(n):
            add(LINE, SIZE, False, "feedback", shopper_id=f"l1m-fb-live-{verdict}-{j}",
                verdict=verdict, direction=None if verdict == "fits_well" else verdict)
    for j in range(2):
        add(LINE, SIZE, False, "kept", shopper_id=f"l1m-kept-live-{j}")

    # Fill the rest of the population over more lines and sizes. The exact
    # truth above does not cover these; they exist to make the index realistic
    # and to give the cardinality check a real population.
    others = [
        ({**LINE}, "L", 60),
        ({"brand": "bonobos", "category": "shirt", "fit_handle": "regular"}, "M", 100),
        ({"brand": "bonobos", "category": "shirt", "fit_handle": "regular"}, "L", 50),
        ({"brand": "bonobos", "category": "shirt", "fit_handle": "regular"}, "S", 40),
        ({"brand": "asket", "category": "tee", "fit_handle": "slim"}, "M", 90),
        ({"brand": "asket", "category": "tee", "fit_handle": "slim"}, "L", 70),
        ({"brand": "asket", "category": "tee", "fit_handle": "slim"}, "S", 70),
    ]
    pattern = [
        ("captured", None, None, None),
        ("captured", None, None, None),
        ("refunded", None, None, None),
        ("return_small", "fit_too_small", "too_small", None),
        ("return_small", "fit_too_small", "too_small", None),
        ("return_small", "fit_too_small", "too_small", None),
        ("return_large", "fit_too_large", "too_large", None),
        ("return_large", "fit_too_large", "too_large", None),
        ("nonfit", "changed_mind", None, None),
        ("nonfit", "damaged", None, None),
        ("nonfit", "not_as_described", None, None),
        ("feedback", None, None, "fits_well"),
        ("feedback", None, None, "bogus"),  # replaced below by a real verdict
        ("feedback", None, None, "too_small"),
        ("feedback", None, None, "too_large"),
        ("kept", None, None, None),
        ("return_small", "fit_too_small", "too_small", None),
        ("return_large", "fit_too_large", "too_large", None),
        ("refunded", None, None, None),
        ("kept", None, None, None),
    ]
    other_counter = 0
    for line, size, n in others:
        for j in range(n):
            other_counter += 1
            state, reason, direction, verdict = pattern[j % len(pattern)]
            seeded = (j % 10) != 0
            if state == "return_small":
                state, reason, direction = "reason_classified", "fit_too_small", "too_small"
            elif state == "return_large":
                state, reason, direction = "reason_classified", "fit_too_large", "too_large"
            elif state == "nonfit":
                state = "reason_classified"
            elif state == "feedback":
                state = "feedback"
                if verdict == "bogus":
                    verdict = "fits_well"
                direction = None if verdict == "fits_well" else verdict
            add(line, size, seeded, state, shopper_id=f"o{other_counter}",
                reason=reason, direction=direction, verdict=verdict)
    return rows


# ---------------------------------------------------------------- checks


def create_index(name, settings=None):
    body = {"mappings": mapping()}
    if settings:
        body["settings"] = settings
    return call("PUT", "/" + name, body=body)


def check_mapping(result):
    rec, resp = create_index(FIT_INDEX)
    assert resp.status_code == 200, resp.text
    get_rec, get_resp = call("GET", "/" + FIT_INDEX + "/_mapping")
    settings_rec, settings_resp = call("GET", "/" + FIT_INDEX + "/_settings")
    result["create"] = rec
    result["get_mapping"] = get_rec
    result["settings"] = settings_rec
    props = get_resp.json()[FIT_INDEX]["mappings"]["properties"]
    result["fields"] = sorted(props)
    result["no_text_fields"] = all(v["type"] != "text" for v in props.values())
    assert result["no_text_fields"]
    prose, prose_resp = call("PUT", f"/{FIT_INDEX}/_doc/prose-probe",
                             body={"note": "a prose field the row does not allow"})
    result["prose_refused"] = {"status": prose_resp.status_code,
                               "error": prose_resp.json().get("error", {}).get("type")}
    assert prose_resp.status_code == 400
    return result


def check_states(result):
    index = FIT_INDEX
    rows = {}
    shared = {"brand": LINE["brand"], "category": LINE["category"],
              "fit_handle": LINE["fit_handle"], "size": SIZE, "seeded": False}
    a = "a" * 64
    b = "b" * 64
    c = "c" * 64

    def put(step, doc, version, version_type="external_gte"):
        rec, resp = call("PUT", f"/{index}/_doc/{doc['txn_hash']}?refresh=true",
                         params={"version": version, "version_type": version_type},
                         body=doc)
        rec["step"] = step
        result["steps"].append(rec)
        if resp.status_code < 300:
            got, _ = call("GET", f"/{index}/_doc/{doc['txn_hash']}")
            result["reads"].append({"step": step, "get": got})
        docs, dresp = count(index)
        result["counts"].append({"step": step, "index_count": dresp.json()["count"]})
        one, oresp = count(index, {"term": {"txn_hash": doc["txn_hash"]}})
        result["counts"][-1]["purchase_count"] = oresp.json()["count"]
        return rec, resp

    def walk_doc(txn_hash, state, seq, **kw):
        doc = {**shared, "txn_hash": txn_hash, "shopper_hash": hash_shopper(LINE, txn_hash),
               "purchase_state": state, "settled": kw.pop("settled", False),
               "captured_at": ts(0), "event_seq": seq}
        doc.update(kw)
        return doc

    # A: captured -> refunded, no reason -> reason classified -> contested.
    doc = walk_doc(a, "captured", 1)
    put("A captured", doc, 1)
    doc.pop("settled_kind", None)
    doc.update({"purchase_state": "refunded", "refunded_at": ts(72)})
    put("A refunded, no reason", doc, 2)
    doc.update({"purchase_state": "reason_classified", "settled": True,
                "settled_kind": "return_reason", "reason_category": "fit_too_small",
                "fit_direction": "too_small", "settled_at": ts(120)})
    put("A reason classified", doc, 3)

    # B: captured -> fit feedback.
    docb = walk_doc(b, "captured", 1)
    put("B captured", docb, 1)
    docb.update({"purchase_state": "feedback", "settled": True,
                 "settled_kind": "feedback", "feedback_verdict": "fits_well",
                 "settled_at": ts(168)})
    put("B fit feedback", docb, 2)

    # C: captured -> kept past the return window.
    docc = walk_doc(c, "captured", 1)
    put("C captured", docc, 1)
    docc.update({"purchase_state": "kept", "settled": True,
                 "settled_kind": "kept", "settled_at": ts(0, 30)})
    put("C kept past window", docc, 2)

    # A: contested reason corrects the row; it stays settled, stays one doc.
    doc.update({"purchase_state": "contested", "reason_contested": True,
                "reason_category": "fit_too_large", "fit_direction": "too_large",
                "event_seq": 4})
    put("A reason contested", doc, 4)

    # Replay the current write: accepted, one document, same content.
    replay, replay_resp = put("replay current (external_gte, v4)", doc, 4)
    replay["replay"] = True
    result["replay_current"] = {"status": replay_resp.status_code,
                                "result": replay_resp.json().get("result")}

    # Replay an older webhook: refused, the newer row stands.
    old = walk_doc(a, "reason_classified", 3, settled=True,
                   settled_kind="return_reason", reason_category="fit_too_small",
                   fit_direction="too_small", settled_at=ts(120))
    old["refunded_at"] = ts(72)
    stale, stale_resp = put("replay older (external_gte, v3)", old, 3)
    stale["replay"] = True
    result["replay_older"] = {"status": stale_resp.status_code,
                              "error": stale_resp.json().get("error", {}).get("type")}

    # Strict external versioning refuses an equal version; external_gte is
    # the choice for webhook retries for that reason.
    equal, equal_resp = call("PUT", f"/{index}/_doc/{c}",
                             params={"version": 2, "version_type": "external"},
                             body=docc)
    equal["step"] = "strict external, equal version"
    result["steps"].append(equal)
    result["replay_strict_external"] = {
        "status": equal_resp.status_code,
        "error": equal_resp.json().get("error", {}).get("type"),
    }

    docs, final = count(index)
    result["final_count"] = final.json()["count"]
    result["versioning"] = {
        "choice": "external_gte",
        "why": (
            "the event sequence is the external version; an equal-version replay is "
            "accepted (200) and replaces the same row, an older replay is refused (409) "
            "so a late webhook cannot undo a newer state, and no extra idempotency "
            "state is stored. strict `external` refuses the equal replay (409), which "
            "would make every retry an error the writer must classify."
        ),
    }
    assert result["final_count"] == 3
    for item in result["counts"]:
        assert item["purchase_count"] == 1
    return result


def agg_query():
    terms = [
        {"term": {"brand": LINE["brand"]}},
        {"term": {"category": LINE["category"]}},
        {"term": {"fit_handle": LINE["fit_handle"]}},
        {"term": {"size": SIZE}},
    ]
    return {
        "size": 0,
        "track_total_hits": True,
        "query": {"bool": {"filter": terms}},
        "aggs": {
            "split": {
                "filters": {"filters": {
                    "all": {"match_all": {}},
                    "seeded": {"term": {"seeded": True}},
                    "live": {"term": {"seeded": False}},
                }},
                "aggs": {
                    "settled": {
                        "filter": {"term": {"settled": True}},
                        "aggs": {
                            "shoppers": {"cardinality": {"field": "shopper_hash"}},
                            "settled_kind": {"terms": {"field": "settled_kind"}},
                            "reason_category": {"terms": {"field": "reason_category"}},
                            "fit_returns": {
                                "filter": {"term": {"settled_kind": "return_reason"}},
                                "aggs": {"direction": {
                                    "terms": {"field": "fit_direction"},
                                    "aggs": {"shoppers": {
                                        "cardinality": {"field": "shopper_hash"}}},
                                }},
                            },
                            "feedback": {
                                "filter": {"term": {"settled_kind": "feedback"}},
                                "aggs": {"verdict": {
                                    "terms": {"field": "feedback_verdict"}}},
                            },
                        },
                    },
                },
            },
        },
    }


def buckets(agg):
    return {b["key"]: b["doc_count"] for b in agg["buckets"]}


def shape(bucket):
    return {
        "docs": bucket["doc_count"],
        "settled": bucket["settled"]["doc_count"],
        "settled_kind": buckets(bucket["settled"]["settled_kind"]),
        "reason_category": buckets(bucket["settled"]["reason_category"]),
        "fit_returns_by_direction": buckets(bucket["settled"]["fit_returns"]["direction"]),
        "feedback_by_verdict": buckets(bucket["settled"]["feedback"]["verdict"]),
        "distinct_shoppers_by_direction": {
            b["key"]: b["shoppers"]["value"]
            for b in bucket["settled"]["fit_returns"]["direction"]["buckets"]
        },
        "distinct_shoppers_settled": bucket["settled"]["shoppers"]["value"],
    }


def check_aggregations(result):
    rec, resp = search(POP_INDEX, agg_query())
    assert resp.status_code == 200, resp.text
    result["query"] = rec
    aggs = resp.json()["aggregations"]["split"]["buckets"]
    got = {name: shape(bucket) for name, bucket in aggs.items()}

    expected_all = {
        "docs": TRUTH["total_docs"],
        "settled": TRUTH["settled"],
        "settled_kind": TRUTH["settled_kind"],
        "reason_category": TRUTH["reason_category"],
        "fit_returns_by_direction": TRUTH["fit_returns_by_direction"],
        "feedback_by_verdict": TRUTH["feedback_by_verdict"],
        "distinct_shoppers_by_direction": TRUTH["distinct_shoppers_by_direction"],
    }
    result["expected_all"] = expected_all
    result["got_all"] = got["all"]
    for key, value in expected_all.items():
        assert got["all"][key] == value, (key, got["all"][key], value)

    for name in ("seeded", "live"):
        truth = TRUTH[name]
        expected = {
            "docs": truth["docs"],
            "settled": truth["settled"],
            "fit_returns_by_direction": truth["fit_returns_by_direction"],
            "feedback_by_verdict": truth["feedback_by_verdict"],
            "distinct_shoppers_by_direction": truth["distinct_shoppers_by_direction"],
        }
        result["expected_" + name] = expected
        result["got_" + name] = got[name]
        for key, value in expected.items():
            if isinstance(value, dict):
                value = {k: v for k, v in value.items() if v}
            assert got[name][key] == value, (name, key, got[name][key], value)

    # Unsettled purchases and a no-reason refund are in the index and count
    # toward nothing.
    unsettled = []
    for state in ("captured", "refunded"):
        q = {"bool": {"filter": [
            {"term": {"brand": LINE["brand"]}},
            {"term": {"category": LINE["category"]}},
            {"term": {"fit_handle": LINE["fit_handle"]}},
            {"term": {"size": SIZE}},
            {"term": {"seeded": False}},
            {"term": {"purchase_state": state}},
        ]}}
        rec2, resp2 = search(POP_INDEX, {
            "size": 5,
            "_source": ["txn_hash", "purchase_state", "settled", "seeded"],
            "query": q,
        })
        assert resp2.status_code == 200
        hit = resp2.json()["hits"]["hits"][0]
        unsettled.append({"state": state, "sample": hit,
                          "settled_filter": None})
        rec3, resp3 = search(POP_INDEX, {
            "size": 0,
            "query": {"bool": {"filter": [q, {"term": {"settled": True}}]}},
        })
        unsettled[-1]["settled_filter"] = resp3.json()["hits"]["total"]["value"]
        result.setdefault("unsettled_queries", []).append({"query": rec2, "settled_query": rec3})
    result["unsettled"] = unsettled
    for item in unsettled:
        assert item["sample"]["_source"]["settled"] is False
        assert item["settled_filter"] == 0

    docs, total = count(POP_INDEX)
    filt, settled = count(POP_INDEX, {"bool": {"filter": [
        {"term": {"brand": LINE["brand"]}},
        {"term": {"category": LINE["category"]}},
        {"term": {"fit_handle": LINE["fit_handle"]}},
        {"term": {"size": SIZE}},
    ]}})
    result["population_count"] = total.json()["count"]
    result["line_size_count"] = settled.json()["count"]
    assert result["population_count"] == 600
    assert result["line_size_count"] == TRUTH["total_docs"]
    return result


def check_cardinality(result):
    expected_whole = len({r["shopper_hash"] for r in POPULATION})
    probe_rows = [r for r in POPULATION if r["brand"] == LINE["brand"]
                  and r["category"] == LINE["category"]
                  and r["fit_handle"] == LINE["fit_handle"] and r["size"] == SIZE
                  and r["settled"]]
    expected_probe = len({r["shopper_hash"] for r in probe_rows})

    def card(field, query, threshold=None):
        body = {"size": 0, "query": query,
                "aggs": {"distinct": {"cardinality": {"field": field}}}}
        if threshold is not None:
            body["aggs"]["distinct"]["cardinality"]["precision_threshold"] = threshold
        rec, resp = search(POP_INDEX, body)
        return rec, resp

    probe_filter = {"bool": {"filter": [
        {"term": {"brand": LINE["brand"]}},
        {"term": {"category": LINE["category"]}},
        {"term": {"fit_handle": LINE["fit_handle"]}},
        {"term": {"size": SIZE}},
        {"term": {"settled": True}},
    ]}}
    match_all = {"match_all": {}}

    samples = {}
    for name, query, expected, threshold in [
        ("probe_default", probe_filter, expected_probe, None),
        ("probe_40000", probe_filter, expected_probe, 40000),
        ("whole_default", match_all, expected_whole, None),
        ("whole_40000", match_all, expected_whole, 40000),
        ("whole_100", match_all, expected_whole, 100),
        ("whole_100000", match_all, expected_whole, 100000),
    ]:
        rec, resp = card("shopper_hash", query, threshold)
        entry = {"request": rec, "expected": expected}
        if resp.status_code == 200:
            entry["value"] = resp.json()["aggregations"]["distinct"]["value"]
        else:
            entry["status"] = resp.status_code
        samples[name] = entry

    result["expected_probe_distinct"] = expected_probe
    result["expected_whole_distinct"] = expected_whole
    result["samples"] = samples
    result["precision_threshold"] = {
        "default": 3000,
        "max": 40000,
        "exact_below": (
            "counts are accurate up to the configured precision_threshold; the "
            "population's whole-index distinct count is below the 3000 default, so "
            "it is exact here. Above 3000 distinct values the default count is an "
            "HLL++ estimate; raising precision_threshold to its 40000 maximum moves "
            "the approximate boundary out to 40000."
        ),
    }
    assert samples["probe_default"]["value"] == expected_probe
    assert samples["probe_40000"]["value"] == expected_probe
    assert samples["whole_default"]["value"] == expected_whole
    assert samples["whole_40000"]["value"] == expected_whole
    return result


COLD_SNIPPET = r"""
import json, os, time, requests
base = os.environ["ELASTIC_URL"].rstrip("/")
session = requests.Session()
headers = {"Authorization": "ApiKey " + os.environ["LAMBO_ES_API_KEY"]}
body = json.loads(os.environ["T012_COLD_QUERY"])
t0 = time.perf_counter()
r = session.post(base + "/" + os.environ["T012_COLD_INDEX"] + "/_search",
                 headers=headers, json=body, timeout=60)
dt = (time.perf_counter() - t0) * 1000
print(json.dumps({"ms": round(dt, 1), "status": r.status_code,
                  "took": r.json().get("took")}))
"""


def check_latency(result):
    query = agg_query()
    warm = []
    for _ in range(25):
        t0 = time.perf_counter()
        resp = SESSION.post(BASE + "/" + POP_INDEX + "/_search", headers=HEADERS,
                            json=query, timeout=60)
        elapsed = (time.perf_counter() - t0) * 1000
        assert resp.status_code == 200
        warm.append({"ms": round(elapsed, 1), "took": resp.json()["took"]})
    cold = []
    env = {**os.environ, "T012_COLD_QUERY": json.dumps(query),
           "T012_COLD_INDEX": POP_INDEX}
    for _ in range(8):
        proc = subprocess.run([sys.executable, "-c", COLD_SNIPPET], env=env,
                              capture_output=True, text=True, timeout=120)
        assert proc.returncode == 0, proc.stderr
        cold.append(json.loads(proc.stdout.strip().splitlines()[-1]))
    warm_ms = [s["ms"] for s in warm]
    cold_ms = [s["ms"] for s in cold]

    def summary(values):
        return {
            "samples": len(values),
            "median_ms": round(statistics.median(values), 1),
            "p95_ms": round(sorted(values)[max(0, int(len(values) * 0.95) - 1)], 1),
            "min_ms": round(min(values), 1),
            "max_ms": round(max(values), 1),
        }

    result["query"] = {"size": 0, "query": query["query"], "aggs": "the line-and-size aggregation"}
    result["warm"] = {"summary": summary(warm_ms), "samples": warm,
                      "es_took_ms": {"median": statistics.median([s["took"] for s in warm]),
                                     "max": max(s["took"] for s in warm)}}
    result["cold"] = {"definition": "a fresh Python process and a new TLS connection for each sample",
                      "summary": summary(cold_ms), "samples": cold,
                      "es_took_ms": {"median": statistics.median([s["took"] for s in cold]),
                                     "max": max(s["took"] for s in cold)}}
    result["over_5s"] = {"warm": sum(1 for v in warm_ms if v > 5000),
                         "cold": sum(1 for v in cold_ms if v > 5000)}
    return result


def list_indices(pattern="*"):
    rec, resp = call("GET", f"/_cat/indices/{pattern}?h=index,docs.count,store.size&s=index&format=json")
    return rec, resp


def list_aliases():
    return call("GET", "/_cat/aliases?format=json&h=alias,index")


def template_names(kind):
    rec, resp = call("GET", "/" + kind + "?format=json")
    if resp.status_code != 200:
        return rec, []
    body = resp.json()
    key = "index_templates" if kind == "_index_template" else "component_templates"
    return rec, sorted(item["name"] for item in body.get(key, []))


def templates_matching(names):
    _, resp = call("GET", "/_index_template?format=json")
    checked, matches = [], []
    for item in resp.json().get("index_templates", []):
        checked.append(item["name"])
        for pattern in item["index_template"].get("index_patterns", []):
            if any(fnmatch.fnmatchcase(name, pattern) for name in names):
                matches.append({"template": item["name"], "pattern": pattern})
    return {"templates_checked": len(checked), "matches": matches}


def run_lambo(workdir):
    config = workdir / "lambo.toml"
    config.write_text(f"""[store]
kind = "sqlite"
path = "{workdir / 'lambo.db'}"

[embedder]
kind = "fixture"
dim = 64

[recall]
kind = "elastic"
url = "{BASE}"
api_key = {{ env = "LAMBO_ES_API_KEY" }}
index_prefix = "{LAMBO_PREFIX}"
timeout_ms = 20000
""")
    transcript = []
    for argv in (
        [LAMBO_BIN, "--version"],
        [LAMBO_BIN, "provision", "--config", str(config)],
        [LAMBO_BIN, "derive", "--session", "t012recall", "--agent", "t012",
         "--content", "Brand X oxford shirts run small through the shoulders",
         "--kind", "logic", "--config", str(config)],
        [LAMBO_BIN, "recall", "--session", "t012recall",
         "--query", "do these oxford shirts run small?", "--config", str(config)],
    ):
        proc = subprocess.run(argv, env={**os.environ, "LAMBO_ES_API_KEY": KEY},
                              capture_output=True, text=True, timeout=180)
        transcript.append({
            "command": " ".join(argv).replace(LAMBO_BIN, "<lambo>")
                .replace(str(Path.home()), "~").replace(str(workdir), "<tmp>").replace(BASE, "<deployment>"),
            "exit": proc.returncode,
            "stdout": proc.stdout.replace(str(Path.home()), "~")
                .replace(str(workdir), "<tmp>").replace(BASE, "<deployment>"),
            "stderr": proc.stderr.replace(str(Path.home()), "~")
                .replace(str(workdir), "<tmp>").replace(BASE, "<deployment>"),
        })
    time.sleep(2)  # Lambo searches the index; the refresh interval is 1s.
    return transcript, config


def check_coexistence(result):
    before_idx, _ = list_indices("*")
    before_alias, _ = list_aliases()
    _, templates_before = template_names("_index_template")
    _, components_before = template_names("_component_template")
    fit_before, _ = count(POP_INDEX)

    workdir = Path(tempfile.mkdtemp(prefix="t012-lambo-"))
    try:
        transcript, config = run_lambo(workdir)
    finally:
        pass
    result["lambo_transcript"] = transcript
    result["lambo_config"] = config.read_text().replace(BASE, "<deployment>").replace(str(workdir), "<tmp>")
    shutil.rmtree(workdir, ignore_errors=True)

    after_idx, after_idx_resp = list_indices("*")
    after_alias, _ = list_aliases()
    _, templates_after = template_names("_index_template")
    _, components_after = template_names("_component_template")
    coverage = templates_matching([n for n in
                                   [row["index"] for row in after_idx_resp.json()]
                                   if n.startswith("t012")])
    fit_after, _ = count(POP_INDEX)

    names = [row["index"] for row in after_idx_resp.json()]
    result["indices_before"] = before_idx["response"]
    result["indices_after"] = after_idx_resp.json()
    result["fit_index_names"] = sorted(n for n in names if n.startswith("t012-fit"))
    result["lambo_index_names"] = sorted(n for n in names if n.startswith(LAMBO_PREFIX))
    result["aliases_before"] = before_alias["response"]
    result["aliases_after"] = after_alias["response"]
    result["templates_before"] = templates_before
    result["templates_after"] = templates_after
    result["component_templates_before"] = components_before
    result["component_templates_after"] = components_after
    result["template_counts"] = {"index_before": len(templates_before),
                                 "index_after": len(templates_after),
                                 "component_before": len(components_before),
                                 "component_after": len(components_after)}
    result["templates_matching_t012"] = coverage
    result["fit_count_before"] = fit_before["response"]["count"]
    result["fit_count_after"] = fit_after["response"]["count"]
    result["recall_answered"] = any("run small through the shoulders" in t["stdout"]
                                    for t in transcript if "recall" in t["command"])
    result["assertions"] = {
        "different_names": not set(result["fit_index_names"]) & set(result["lambo_index_names"]),
        "no_alias": result["aliases_after"] == [],
        "no_new_template": templates_before == templates_after,
        "no_new_component_template": components_before == components_after,
        "no_template_matches_t012": coverage["matches"] == [],
        "fit_untouched": result["fit_count_before"] == result["fit_count_after"],
        "recall_answered": result["recall_answered"],
    }
    assert all(result["assertions"].values()), result["assertions"]
    return result


def check_limits(result):
    settings, _ = call("GET", "/" + POP_INDEX + "/_settings")
    result["settings"] = settings["response"]

    mode_rec, mode_resp = create_index(MODE_PROBE, {"index": {"mode": "standard"}})
    result["mode_override"] = {"status": mode_resp.status_code, "response": mode_resp.json()}

    exclude_rec, exclude_resp = create_index(
        EXCLUDE_PROBE, {"index": {"mapping": {"exclude_source_vectors": False}}})
    result["exclude_source_vectors_override"] = {
        "status": exclude_resp.status_code, "response": exclude_resp.json()}
    if exclude_resp.status_code < 300:
        call("DELETE", "/" + EXCLUDE_PROBE)

    # A probe document for update-by-id behaviour, deleted afterwards.
    probe_doc = row(LINE, SIZE, txn_id="limits-probe", shopper_id="limits",
                    seeded=False, state="captured")
    probe_id = probe_doc["txn_hash"]
    probe_doc["event_seq"] = 1
    doc_rec, doc_resp = call("PUT", f"/{POP_INDEX}/_doc/{probe_id}?refresh=true",
                             params={"version": 1, "version_type": "external_gte"},
                             body=probe_doc)
    result["probe_doc"] = {"status": doc_resp.status_code, "response": doc_resp.json()}

    upd_ext, upd_ext_resp = call(
        "POST", f"/{POP_INDEX}/_update/{probe_id}",
        params={"version": 2, "version_type": "external_gte"},
        body={"doc": {"event_seq": 2}})
    result["update_with_external_version"] = {
        "status": upd_ext_resp.status_code, "response": upd_ext_resp.json()}

    upd_plain, upd_plain_resp = call(
        "POST", f"/{POP_INDEX}/_update/{probe_id}",
        body={"doc": {"event_seq": 2}})
    result["update_by_id_plain"] = {
        "status": upd_plain_resp.status_code, "response": upd_plain_resp.json()}

    ubq, ubq_resp = call("POST", f"/{POP_INDEX}/_update_by_query?conflicts=proceed",
                         body={"script": {"source": "ctx._source.event_seq = 1"},
                               "query": {"term": {"txn_hash": probe_id}}})
    result["update_by_query"] = {"status": ubq_resp.status_code,
                                 "response": ubq_resp.json()}

    search_rec, search_resp = search(POP_INDEX, {"size": 0, "query": {"match_all": {}},
                                                 "aggs": {"n": {"value_count": {"field": "txn_hash"}}}})
    result["aggregation"] = {"status": search_resp.status_code,
                             "response": search_resp.json()}

    count_rec, count_resp = count(POP_INDEX)
    result["count_api"] = {"status": count_resp.status_code, "response": count_resp.json()}

    call("DELETE", f"/{POP_INDEX}/_doc/{probe_id}?refresh=true")
    result["docs_after_probe_delete"] = count(POP_INDEX)[1].json()["count"]

    result["notes"] = {
        "project_type": (
            "The deployment is an Elasticsearch Vector Database project, not a "
            "general Elasticsearch Serverless project: `index.mode: standard` is "
            "refused with `index mode [standard] is not allowed for vectordb "
            "projects; only [vectordb_document] is supported`, and every index is "
            "created in `vectordb_document` mode with "
            "`index.mapping.exclude_source_vectors: true`."
        ),
        "update_by_id": (
            "Whole-document PUTs by _id accept external versioning. The partial "
            "`_update` API refuses it (`internal versioning can not be used for "
            "optimistic concurrency control; use if_seq_no and if_primary_term`), "
            "so the writer rewrites the whole row at the event's external version."
        ),
        "aggregations": (
            "terms, filter, filters and cardinality aggregations all answer inside "
            "vectordb_document mode; structured keyword fields and `_source` are "
            "stored and returned as usual."
        ),
        "billing": (
            "Vector Database billing is storage, reserved search capacity (Search "
            "Power), ingest volume and an hourly infrastructure fee, not the "
            "VCU model of Elasticsearch Serverless projects. Query volume itself "
            "does not change the search charge. No console reading was provided; "
            "these calls created only kilobyte-scale test indices, all deleted."
        ),
    }
    return result


# ---------------------------------------------------------------- cleanup


def cleanup():
    rec, resp = list_indices("t012-*")
    removed = []
    for row in resp.json():
        name = row["index"]
        drec, dresp = call("DELETE", "/" + name)
        removed.append({"index": name, "status": dresp.status_code})
    time.sleep(1)
    left, left_resp = list_indices("t012-*")
    return {"removed": removed, "remaining": left_resp.json(), "listing": left}


# ---------------------------------------------------------------- main


POPULATION = []


def main():
    global REDACT, POPULATION
    REDACT = Redactor()
    started = cleanup()  # clear anything a cancelled run left behind
    outputs = {}
    errors = {}
    try:
        POPULATION = build_population()
        assert len(POPULATION) == 600, len(POPULATION)
        truth_manifest = {
            "note": "hand-written truth for the probe line and size; the generator builds rows to it",
            "truth": TRUTH,
            "population_rows": len(POPULATION),
            "lines": sorted({(r["brand"], r["category"], r["fit_handle"]) for r in POPULATION}),
            "sizes": sorted({r["size"] for r in POPULATION}),
            "distinct_shoppers_whole_index": len({r["shopper_hash"] for r in POPULATION}),
            "shopper_salt": "line identity plus a synthetic salt in this harness; production adds a secret, so one shopper maps to unrelated hashes across lines",
        }
        assert len({hash_shopper(LINE, "same-person")}) == 1
        assert hash_shopper(LINE, "same-person") != hash_shopper(
            {"brand": "bonobos", "category": "shirt", "fit_handle": "regular"}, "same-person")

        outputs["mapping"] = check_mapping({})
        outputs["states"] = check_states({"steps": [], "reads": [], "counts": []})
        pop_create, pop_create_resp = create_index(POP_INDEX)
        assert pop_create_resp.status_code == 200, pop_create_resp.text
        load = bulk(POPULATION)
        assert load["response"]["errors"] is False, load["response"]
        assert load["response"]["items"] == len(POPULATION)
        outputs["population"] = {"manifest": truth_manifest, "create": pop_create,
                                 "load": load}
        refresh(POP_INDEX)
        outputs["aggregations"] = check_aggregations({})
        outputs["cardinality"] = check_cardinality({})
        outputs["latency"] = check_latency({})
        outputs["coexistence"] = check_coexistence({})
        outputs["limits"] = check_limits({})
    except Exception as exc:  # keep the evidence gathered so far
        errors["main"] = f"{type(exc).__name__}: {exc}"
        raise
    finally:
        outputs["cleanup"] = cleanup()
        outputs["run"] = {
            "started_cleanup": started,
            "errors": errors,
            "engine": "9.7.0",
            "python": sys.version.split()[0],
            "requests": requests.__version__,
        }
        for name, value in outputs.items():
            path = HERE / f"{name}.json"
            path.write_text(json.dumps(REDACT.text(value), indent=2) + "\n")
            print(f"wrote {path.name}", file=sys.stderr)


def cold_one():
    body = json.loads(os.environ["T012_COLD_QUERY"])
    session = requests.Session()
    headers = {"Authorization": "ApiKey " + os.environ["LAMBO_ES_API_KEY"]}
    base = os.environ["ELASTIC_URL"].rstrip("/")
    t0 = time.perf_counter()
    resp = session.post(base + "/" + os.environ["T012_COLD_INDEX"] + "/_search",
                        headers=headers, json=body, timeout=60)
    elapsed = (time.perf_counter() - t0) * 1000
    print(json.dumps({"ms": round(elapsed, 1), "status": resp.status_code,
                      "took": resp.json().get("took")}))


if __name__ == "__main__":
    if "--cold-one" in sys.argv:
        cold_one()
    else:
        main()
