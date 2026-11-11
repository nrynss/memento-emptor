#!/usr/bin/env python3
"""T9.7 pivot demo: product.md's steps 0-7 against the hosted services.

The demo reads the served layer's edge, drives the two deployed agents over
the messaging proxy, and captures the two deployed read-only apps. Nothing
here is a stand-in: a step that cannot run live is recorded as a failure
with the request and the redacted answer, never filled in from a fixture.

Keys come from the system keyring, held in this process only, and never
printed or written. Raw answers and full trace ids stay in demo/out/
(git-ignored). The redacted evidence this script writes into
demo/evidence/run-<n>/ is what the run record cites.

Environment:
  MEMENTO_SECOND_DEPLOYMENT   the second agent's deployment id (required;
                              it never lives in the repository)
  MEMENTO_LAYER_URL           default https://memento-emptor.onrender.com
  MEMENTO_MERCHANT_URL        default https://memento-merchant.onrender.com
  OUT                         default demo/out

Usage:
  bash demo/reset.sh <run-number>
  bash demo/run.sh
"""

import calendar
import json
import os
import re
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.request

LAYER = os.environ.get("MEMENTO_LAYER_URL", "https://memento-emptor.onrender.com").rstrip("/")
MERCHANT = os.environ.get("MEMENTO_MERCHANT_URL", "https://memento-merchant.onrender.com").rstrip("/")
REF_DEPLOYMENT = os.environ.get("MEMENTO_REF_DEPLOYMENT", "9d3-n2x-s0a")
SECOND_DEPLOYMENT = os.environ.get("MEMENTO_SECOND_DEPLOYMENT", "")
OUT = os.environ.get("OUT", "demo/out")
UA = ("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/126.0 Safari/537.36")

SHOPPER = "shopper_01"
DEMO_LINE = {"brand_id": "brand-x-001", "category": "shirts", "fit_handle": "regular", "size": "M"}
TOOL_NAMES = (
    "recall_for_purchase", "record_correction", "pending_questions", "record_reason",
    "record_feedback", "explain_belief", "product_fit", "set_pooling",
    "ledger_beliefs", "ledger_belief", "ledger_evidence",
)

RUN = None
RAWDIR = None
EVIDENCE = None
GAPS = []
FEEDBACK_WRITTEN = False


def say(*parts):
    print("[t97]", *parts, flush=True)


def gap(step, text):
    entry = {"step": step, "text": text}
    if entry in GAPS:
        return
    GAPS.append(entry)
    if RAWDIR:
        try:
            os.makedirs(RAWDIR, exist_ok=True)
            with open(os.path.join(RAWDIR, "gaps.json"), "w") as f:
                json.dump(GAPS, f, indent=2)
        except OSError:
            pass
    say("GAP", step, "-", text)


# The owner each gap in the summary names. The ids are the tasks the T9.7
# review assigned; a gap without one is a defect in this list.
GAP_OWNERS = (
    ("0", "a new shopper cannot be recalled", "T9.15"),
    ("3", "reused the id", "T8.5m"),
    ("3", "does not quote", "T9.9"),
    ("4", "without recalling", "T9.9"),
    ("4", "did not call record_feedback", "T9.9"),
    ("4", "did not change the live settled", "T8.5l"),
    ("5", "seller belief never reaches recall", "T9.9"),
    ("6", "counts the same purchase", "T8.5l"),
    ("6", "repeated evidence id", "T8.5l"),
    ("6", "deployed ledger page", "T4.8"),
    ("7", "does not quote", "T9.9"),
)


def normalise_gaps():
    """Give every gap its owner, drop the step-7 label, and add step 2's missing tool."""
    out = []
    for entry in GAPS:
        if "merchant page reads live pool counts" in entry["text"]:
            continue
        text = entry["text"]
        if "Owner:" not in text:
            for step, needle, owner in GAP_OWNERS:
                if entry["step"] == step and needle in text:
                    text = text.rstrip() + " Owner: " + owner + "."
                    break
        out.append({"step": entry["step"], "text": text})
    if not any("product-search tool" in entry["text"] for entry in out):
        out.append({
            "step": "2",
            "text": ("the reference agent exposes no product-search tool, so no live turn names "
                     "the Channel3 candidate; the SKU exists only in step 4's captured purchase. "
                     "Owner: T9.17."),
        })
    GAPS[:] = out


def secret(service, kind=None):
    args = ["secret-tool", "lookup", "service", service, "account", "memento-emptor"]
    if kind:
        args += ["kind", kind]
    out = subprocess.run(args, capture_output=True, check=True)
    return out.stdout.decode().strip()


def token(kind):
    """One caller token from the keyring. Held in memory; never logged."""
    return secret("memento-tokens", kind)


def ast(*args, timeout=120):
    out = subprocess.run(["ast", *args], capture_output=True, text=True, timeout=timeout)
    if out.returncode != 0:
        raise RuntimeError(f"ast {' '.join(args)} failed: {out.stderr.strip()[:400]}")
    return out.stdout


def http(url, body=None, bearer=None, method="POST", timeout=120):
    """One JSON request. Returns (status, parsed-or-text). Never raises on 4xx/5xx."""
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Accept", "application/json")
    req.add_header("User-Agent", UA)
    if body is not None:
        req.add_header("Content-Type", "application/json")
    if bearer:
        req.add_header("Authorization", "Bearer " + bearer)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            raw = r.read().decode()
            status = r.status
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        status = e.code
    try:
        return status, json.loads(raw)
    except json.JSONDecodeError:
        return status, raw


def save(name, value):
    os.makedirs(RAWDIR, exist_ok=True)
    with open(os.path.join(RAWDIR, name), "w") as f:
        json.dump(value, f, indent=2, sort_keys=True)


def redact(value):
    """Drop full host trace ids from committed evidence (short ids stay)."""
    if isinstance(value, dict):
        return {k: redact(v) for k, v in value.items() if k != "trace_id"}
    if isinstance(value, list):
        return [redact(v) for v in value]
    return value


def evidence(name, value):
    os.makedirs(EVIDENCE, exist_ok=True)
    with open(os.path.join(EVIDENCE, name), "w") as f:
        json.dump(redact(value), f, indent=2, sort_keys=True)


def layer(tool, body, kind="agent", timeout=120):
    """One layer call through the public edge. Refusals are returned, not raised."""
    status, answer = http(f"{LAYER}/tools/{tool}", body, bearer=token(kind), timeout=timeout)
    return status, answer


def must_layer(step, tool, body, kind="agent"):
    status, answer = layer(tool, body, kind)
    if status != 200:
        raise RuntimeError(f"{step}: {tool} answered {status}: {json.dumps(answer)[:500]}")
    return answer


def merchant_fit():
    status, answer = http(f"{MERCHANT}/tools/product_fit", DEMO_LINE, timeout=120)
    return status, answer


def billing():
    text = ast("billing", "get")
    out = {}
    for line in text.splitlines():
        if "Usage spend:" in line:
            out["usage_spend"] = float(re.search(r"\$([0-9.]+)", line).group(1))
        elif "Credit left:" in line:
            out["credit_left"] = float(re.search(r"\$([0-9.]+)", line).group(1))
    return out


# --- messaging proxy -------------------------------------------------------

def messaging_call(method, path, body=None):
    for attempt in (1, 2):
        req = urllib.request.Request(
            f"https://astropods.com/api/v1/deployments/{path[0]}/messaging{path[1]}",
            method=method,
            headers={"Authorization": "Bearer " + astro_token(),
                     "Content-Type": "application/json",
                     "Accept": "application/json",
                     "User-Agent": UA})
        if body is not None:
            req.data = json.dumps(body).encode()
        try:
            with urllib.request.urlopen(req, timeout=180) as r:
                return r.status, r.read().decode()
        except urllib.error.HTTPError as e:
            raw = e.read().decode()[:2000]
            # A token rotated under a long run: re-read it once and retry.
            if e.code == 401 and attempt == 1:
                global _ASTRO_TOKEN
                _ASTRO_TOKEN = None
                continue
            return e.code, raw


_ASTRO_TOKEN = None


def astro_token():
    global _ASTRO_TOKEN
    if _ASTRO_TOKEN is None:
        out = subprocess.run(
            ["secret-tool", "lookup", "service", "astro-cli",
             "username", "default_org_01M46R4F87ZN49Y0SAKHQE8HP8_access_token"],
            capture_output=True, check=True)
        _ASTRO_TOKEN = out.stdout.decode().strip()
    return _ASTRO_TOKEN


def conversation(deployment):
    status, body = messaging_call("POST", (deployment, "/conversations"), {})
    if status // 100 != 2:
        raise RuntimeError(f"messaging create answered {status}: {body[:400]}")
    return json.loads(body)["conversation_id"]


def send(deployment, conv, text):
    status, body = messaging_call("POST", (deployment, f"/conversations/{conv}/messages"),
                                  {"content": text})
    if status // 100 != 2:
        raise RuntimeError(f"messaging send answered {status}: {body[:400]}")


def trace_list(deployment, limit=10):
    return json.loads(ast("agent", "trace", "--id", deployment, "--limit", str(limit), "--json"))["traces"]


def trace_detail(deployment, trace_id):
    return json.loads(ast("agent", "trace", "--id", deployment, "-t", trace_id, "--json"))


def tool_spans(detail):
    spans = []
    for obs in detail.get("observations", []):
        if obs.get("name") in TOOL_NAMES:
            spans.append({"name": obs["name"], "start": obs.get("start_time"), "end": obs.get("end_time")})
    return sorted(spans, key=lambda s: s["start"] or "")


def reply_text(detail):
    out = detail.get("trace", {}).get("output")
    if isinstance(out, dict):
        return out.get("text") or out.get("content") or json.dumps(out)
    return out if isinstance(out, str) else json.dumps(out)


USED_TRACES = set()


def turn(step, label, deployment, conv, text, timeout=240):
    """Send one message, wait for its trace, and return a redacted turn record."""
    before = time.time() - 2
    send(deployment, conv, text)
    deadline = time.time() + timeout
    found = None
    while time.time() < deadline:
        time.sleep(6)
        for t in trace_list(deployment):
            if t.get("trace_id") in USED_TRACES:
                continue
            stamp = t.get("timestamp", "")
            try:
                at = calendar.timegm(time.strptime(stamp[:19], "%Y-%m-%dT%H:%M:%S"))
            except ValueError:
                continue
            if at >= before:
                found = t
                break
        if found:
            break
    if not found:
        raise RuntimeError(f"{step}: no trace appeared for turn {label} within {timeout}s")
    USED_TRACES.add(found["trace_id"])
    detail = trace_detail(deployment, found["trace_id"])
    # The platform exports a turn's reply a moment after the trace appears.
    for _ in range(10):
        text_out = reply_text(detail)
        if text_out and text_out not in ("null", "None", ""):
            break
        time.sleep(6)
        detail = trace_detail(deployment, found["trace_id"])
    spans = tool_spans(detail)
    record = {
        "step": step,
        "label": label,
        "deployment": "MEMENTO-REF" if deployment == REF_DEPLOYMENT else "MEMENTO-SECOND",
        "trace_id": found["trace_id"],
        "trace_id_short": found["trace_id"][:8],
        "cost": detail.get("trace", {}).get("total_cost", found.get("total_cost")),
        "latency_ms": found.get("latency_ms"),
        "timestamp": found.get("timestamp"),
        "input": text,
        "tools": [{"name": s["name"], "start": s["start"], "end": s["end"]} for s in spans],
        "reply": reply_text(detail),
    }
    save(f"{step}-turn-{label}.json", record)
    say(f"{step}: turn {label} trace {record['trace_id_short']} cost ${record['cost']:.4f} "
        f"tools {[s['name'] for s in spans]}")
    return record


# --- redaction helpers -----------------------------------------------------

def correction_of(recall):
    c = recall.get("overriding_correction")
    return c or {}


def pool_summary(fit):
    if isinstance(fit, dict) and "counts" not in fit and isinstance(fit.get("pool"), dict):
        fit = fit["pool"]
    c = fit.get("counts", {})
    return {
        "line": fit.get("line"),
        "size": fit.get("size"),
        "status": fit.get("status"),
        "direction": fit.get("direction"),
        "settled": c.get("settled"),
        "returns": c.get("returns"),
        "feedback": c.get("feedback"),
        "kept": c.get("kept"),
        "shoppers": c.get("shoppers"),
        "seeded": (fit.get("seeded") or {}).get("settled"),
        "live": (fit.get("live") or {}).get("settled"),
        "recent": (fit.get("recent") or {}).get("settled"),
    }


def screenshot(name, url):
    if shutil.which("playwright") is None:
        gap("screens", "playwright is not on PATH; the screen was not captured")
        return None
    path = os.path.join(EVIDENCE, name)
    out = subprocess.run(
        ["playwright", "screenshot", "--full-page", "--wait-for-timeout", "6000", url, path],
        capture_output=True, text=True, timeout=180)
    if out.returncode != 0:
        gap("screens", f"{url} did not render for a screenshot: {out.stderr.strip()[:200]}")
        return None
    say("screen", name)
    return name


# --- the steps -------------------------------------------------------------

def step0():
    status, fit = layer("product_fit", DEMO_LINE)
    if status != 200:
        raise RuntimeError(f"step 0: product_fit answered {status}")
    state = {"product_fit": pool_summary(fit)}

    # The pool is the day-one sizing source. The empty-shopper recall is the
    # half that needs a shopper the agent key may act for.
    status, answer = layer("recall_for_purchase", {
        "shopper_id": "shopper_02", "brand_id": "brand-x-001",
        "category": "shirts", "fit_handle": "regular", "size_considered": "M"})
    state["new_shopper_recall"] = {
        "shopper_id": "shopper_02",
        "request": {"brand_id": "brand-x-001", "category": "shirts",
                    "fit_handle": "regular", "size_considered": "M"},
        "status": status,
        "answer": answer,
    }
    if status != 200:
        gap("0", "a new shopper cannot be recalled live: the agent key is bound to "
                 f"shopper_01 only, so shopper_02 answers {status} {json.dumps(answer)}. "
                 "Sizing a day-one shopper needs an operator to add the shopper to the "
                 "agent key's binding (a Render secret change).")
    evidence("00-pool-and-new-shopper.json", state)
    if state["product_fit"]["status"] != "Earned" or state["product_fit"]["direction"] != "small":
        raise RuntimeError(f"step 0: demo line is not Earned small: {state['product_fit']}")
    return state


def step12():
    status, recall = layer("recall_for_purchase", {
        "shopper_id": SHOPPER, "brand_id": "brand-x-001",
        "category": "shirts", "fit_handle": "regular", "size_considered": "M"})
    if status != 200:
        raise RuntimeError(f"step 1: recall answered {status}")
    save("01-recall.json", recall)

    status, ledger = layer("ledger_beliefs", {"shopper_id": SHOPPER, "include_seeded": True}, "shopper")
    if status != 200:
        raise RuntimeError(f"step 1: ledger_beliefs answered {status}")
    save("01-ledger-beliefs.json", ledger)

    status, explain = layer("explain_belief", {"shopper_id": SHOPPER, "belief_id": "blf_8f2c41"}, "shopper")
    if status != 200:
        raise RuntimeError(f"step 1: explain_belief answered {status}")
    save("01-explain.json", explain)

    belief = next((b for b in recall.get("beliefs", []) if b["belief_id"] == "blf_8f2c41"), None)
    if belief is None or len(belief.get("evidence", [])) != 3:
        raise RuntimeError("step 1: the sizing belief does not carry three returns")
    returns = [{
        "evidence_id": ev["evidence_id"],
        "size": ev["item"]["size"],
        "reason": ev["reason"],
        "occurred_at": ev["occurred_at"],
        "seeded": ev["seeded"],
        "outcome": ev["outcome"]["kind"],
    } for ev in belief["evidence"]]
    reasons = sorted({r["reason"]["category"] for r in returns})
    if reasons != ["fit_too_small"]:
        raise RuntimeError(f"step 1: the three returns are not all fit_too_small: {reasons}")

    state = {
        "belief": {
            "belief_id": belief["belief_id"],
            "status": belief.get("status"),
            "standing": belief.get("standing"),
            "value": belief.get("value"),
            "seeded": belief.get("seeded"),
            "contradicted": belief.get("contradicted"),
        },
        "returns": returns,
        "pool": pool_summary(recall.get("pool") or {}),
        "guidance": recall.get("guidance"),
        "pool_lookup": recall.get("pool_lookup"),
    }
    evidence("01-three-returns-and-belief.json", state)
    evidence("02-belief-explained.json", explain)
    return state, recall


def pre_belief_view(pre):
    """The sizing belief's state in the recall taken before the run's correction turn."""
    belief = next((b for b in pre.get("beliefs", []) if b["belief_id"] == "blf_8f2c41"), None)
    if belief is None:
        return None
    return {
        "belief_id": belief["belief_id"],
        "status": belief.get("status"),
        "standing": belief.get("standing"),
        "value": belief.get("value"),
        "evidence_count": len(belief.get("evidence", [])),
        "guidance": pre.get("guidance"),
        "correction_in_force": (pre.get("overriding_correction") or {}).get("correction_id"),
    }


def step3(ref_conv):
    pre = must_layer("step 3", "recall_for_purchase", {
        "shopper_id": SHOPPER, "brand_id": "brand-x-001",
        "category": "shirts", "fit_handle": "regular", "size_considered": "M"})
    pre_corr = correction_of(pre)

    ask = "I need a Brand X oxford shirt in the regular fit. What size should I take?"
    first = turn("03", "ask", REF_DEPLOYMENT, ref_conv, ask)
    if not first["tools"] or first["tools"][0]["name"] != "recall_for_purchase":
        raise RuntimeError("step 3: the first turn did not recall first")

    correction_text = ("Use M for me on Brand X oxfords from now on - I've gone down a size "
                       "since October. Record that correction.")
    second = turn("03", "correct", REF_DEPLOYMENT, ref_conv, correction_text)
    tool_order = [s["name"] for s in second["tools"]]
    if "record_correction" not in tool_order:
        gap("3", "the reference agent did not call record_correction; the demo recorded the "
                 "correction through the layer instead, so the write is live but not through "
                 "the agent")
        status, direct = layer("record_correction", {
            "shopper_id": SHOPPER, "text": correction_text,
            "contradicts_belief_id": "blf_8f2c41"})
        if status != 200:
            raise RuntimeError(f"step 3: direct record_correction answered {status}: {direct}")
    if tool_order.count("recall_for_purchase") < 1:
        gap("3", "the correction turn called record_correction without recalling first; the "
                 "agent relied on the earlier turn's recall. Recorded as it came; the next "
                 "recommendation turn recalls first")

    post = must_layer("step 3", "recall_for_purchase", {
        "shopper_id": SHOPPER, "brand_id": "brand-x-001",
        "category": "shirts", "fit_handle": "regular", "size_considered": "M"})
    post_corr = correction_of(post)
    if not post_corr:
        raise RuntimeError("step 3: recall serves no correction after the turn")
    if post_corr.get("text") == pre_corr.get("text"):
        raise RuntimeError("step 3: the correction in force did not change; the live correction "
                           f"before the turn was {pre_corr.get('correction_id')} "
                           f"{pre_corr.get('text')!r}, so the agent's write did not land or was "
                           "deduplicated")
    if post_corr.get("correction_id") == pre_corr.get("correction_id"):
        gap("3", "the new correction reused the id of the one it replaced: the served layer "
                 "restored the standing journal without advancing the id sequence, so the next "
                 "mint overwrote the restored correction. The new words are live and recalled; "
                 "the old correction is no longer in recall. A layer defect for the handoff, not "
                 "a demo workaround")

    # The next recommendation obeys the correction: ask once more in the same
    # conversation after the write, so the reply is the very next turn.
    third = turn("03", "next", REF_DEPLOYMENT, ref_conv,
                 "So what size do you recommend for the Brand X oxford now?")
    if not third["tools"] or third["tools"][0]["name"] != "recall_for_purchase":
        raise RuntimeError("step 3: the follow-up turn did not recall first")
    if post_corr.get("correction_id") not in third["reply"] and post_corr.get("text", "")[:20] not in third["reply"]:
        gap("3", "the follow-up reply does not quote the new correction id or its words; the "
                 "layer read is the proof of the correction, the reply is recorded as it came")

    state = {
        "belief_before_correction": pre_belief_view(pre),
        "correction_before": {"correction_id": pre_corr.get("correction_id"),
                              "text": pre_corr.get("text"),
                              "recorded_at": pre_corr.get("recorded_at")},
        "correction_after": {k: post_corr.get(k) for k in
                             ("correction_id", "text", "recorded_at", "contradicts_belief_id")},
        "guidance_after": post.get("guidance"),
        "belief_after": next(({"belief_id": b["belief_id"], "status": b.get("status"),
                               "contradicted": b.get("contradicted"),
                               "evidence_count": len(b.get("evidence", []))}
                              for b in post.get("beliefs", []) if b["belief_id"] == "blf_8f2c41"), None),
        "turns": [first, second, third],
    }
    evidence("03-correction-through-the-agent.json", state)
    return state, post


def step4():
    # The purchase is the live capture already in the shopper's memory (the
    # served layer cannot ingest a new one; see the README).
    status, ledger = layer("ledger_evidence", {"shopper_id": SHOPPER}, "shopper")
    if status != 200:
        raise RuntimeError(f"step 4: ledger_evidence answered {status}")
    capture = next((row for row in ledger.get("evidence", [])
                    if row["evidence"]["outcome"]["kind"] == "capture"), None)
    if capture is None:
        raise RuntimeError("step 4: no capture in the shopper's memory")
    custom = capture["evidence"]["link"]["custom_id"]

    before = merchant_fit()
    state = {
        "purchase": {
            "evidence_id": capture["evidence"]["evidence_id"],
            "outcome": capture["evidence"]["outcome"],
            "item": capture["evidence"]["item"],
            "custom_id": custom,
            "occurred_at": capture["evidence"]["occurred_at"],
            "seeded": capture["evidence"]["seeded"],
        },
        "product_fit_before": pool_summary(before[1]) if before[0] == 200 else before,
    }

    # Fit feedback through the agent, tied to that purchase's custom id.
    conv = conversation(REF_DEPLOYMENT)
    text = (f"The PSW8GeK oxford in M I ordered (order reference {custom}) arrived and "
            "fits well across the shoulders and in the sleeves. Record that for me.")
    record = turn("04", "feedback", REF_DEPLOYMENT, conv, text)
    state["turn"] = record
    wrote = any(s["name"] == "record_feedback" for s in record["tools"])
    if not any(s["name"] == "recall_for_purchase" for s in record["tools"]):
        gap("4", "the feedback turn called record_feedback without recalling first; recorded "
                 "as it came")
    if not wrote:
        gap("4", "the reference agent did not call record_feedback; the demo recorded the "
                 "feedback through the layer instead, so the fit row is written but not "
                 "through the agent")
        status, direct = layer("record_feedback", {
            "shopper_id": SHOPPER, "custom_id": custom,
            "sku": capture["evidence"]["item"]["sku"],
            "size": capture["evidence"]["item"]["size"],
            "feedback_text": "the M fits well across the shoulders and in the sleeves"})
        if status != 200:
            raise RuntimeError(f"step 4: direct record_feedback answered {status}: {direct}")

    status, after = merchant_fit()
    state["product_fit_after"] = pool_summary(after) if status == 200 else after
    if status != 200:
        raise RuntimeError(f"step 4: merchant product_fit answered {status}: {after}")
    live = state["product_fit_after"].get("live") or 0
    if live < 1:
        gap("4", "the fit row did not change the live settled count (0 live): the writer "
                 "answered but the pool reads no live settled row for the demo line")

    status, explain = layer("explain_belief", {"shopper_id": SHOPPER, "belief_id": "blf_2b9d60"}, "shopper")
    if status == 200:
        state["fit_belief"] = explain
    evidence("04-purchase-and-feedback.json", state)
    return state


def step5():
    status, seller = layer("explain_belief", {"shopper_id": SHOPPER, "belief_id": "blf_1a5e09"}, "shopper")
    if status != 200:
        raise RuntimeError(f"step 5: seller explain_belief answered {status}")
    status, sizing = layer("explain_belief", {"shopper_id": SHOPPER, "belief_id": "blf_8f2c41"}, "shopper")
    if status != 200:
        raise RuntimeError(f"step 5: sizing explain_belief answered {status}")
    status, recall = layer("recall_for_purchase", {
        "shopper_id": SHOPPER, "brand_id": "brand-x-001",
        "category": "shirts", "fit_handle": "regular", "size_considered": "M"})
    if status != 200:
        raise RuntimeError(f"step 5: recall answered {status}")

    sizing_evidence = next((b for b in recall.get("beliefs", []) if b["belief_id"] == "blf_8f2c41"), {}).get("evidence", [])
    dispute_in_sizing = [ev["evidence_id"] for ev in sizing_evidence
                         if ev["outcome"]["kind"] == "dispute"]
    seller_evidence = seller.get("belief", {}).get("evidence", [])
    state = {
        "seller_belief": {
            "belief_id": "blf_1a5e09",
            "value": seller.get("belief", {}).get("value"),
            "status": seller.get("belief", {}).get("status"),
            "standing": seller.get("belief", {}).get("standing"),
            "evidence_count": len(seller_evidence),
            "rows": [{"reason": r.get("reason"),
                      "counts_toward": r.get("counts_toward"),
                      "outcome": r.get("outcome", {}).get("kind")}
                     for r in seller_evidence],
        },
        "sizing_belief_evidence": [ev["evidence_id"] for ev in sizing_evidence],
        "dispute_rows_in_sizing": dispute_in_sizing,
        "recall_guidance": recall.get("guidance"),
        "recall_pool_status": (recall.get("pool") or {}).get("status"),
    }
    if len(seller_evidence) != 1:
        raise RuntimeError("step 5: the seller belief does not carry exactly one dispute row")
    if dispute_in_sizing:
        raise RuntimeError(f"step 5: a dispute row reached the sizing belief: {dispute_in_sizing}")
    gap("5", "the dispute's seller belief never reaches recall (the contract serves seller "
             "beliefs through the ledger and explain reads only), and the agent key is refused "
             "on ledger reads, so a live agent turn cannot mention it; the demo shows the "
             "seller row and the untouched sizing belief by live read instead")
    evidence("05-dispute-leaves-sizing-alone.json", state)
    return state


def step6():
    status, beliefs = layer("ledger_beliefs", {"shopper_id": SHOPPER, "include_seeded": True}, "shopper")
    if status != 200:
        raise RuntimeError(f"step 6: ledger_beliefs answered {status}")
    status, ledger = layer("ledger_evidence", {"shopper_id": SHOPPER}, "shopper")
    if status != 200:
        raise RuntimeError(f"step 6: ledger_evidence answered {status}")
    status, recall = layer("recall_for_purchase", {
        "shopper_id": SHOPPER, "brand_id": "brand-x-001",
        "category": "shirts", "fit_handle": "regular", "size_considered": "M"})
    if status != 200:
        raise RuntimeError(f"step 6: recall answered {status}")

    rows = [{
        "belief_id": r["belief"]["belief_id"],
        "kind": r["belief"].get("kind"),
        "status": r["belief"].get("status"),
        "standing": r["belief"].get("standing"),
        "seeded": r["belief"].get("seeded"),
        "contradicted": r["belief"].get("contradicted"),
        "evidence_count": len(r["belief"].get("evidence", [])),
        "value": r["belief"].get("value"),
    } for r in beliefs.get("beliefs", [])]
    doubled = []
    for r in beliefs.get("beliefs", []):
        ids = [ev["evidence_id"] for ev in r["belief"].get("evidence", [])]
        if len(ids) != len(set(ids)):
            doubled.append({"belief_id": r["belief"]["belief_id"],
                            "evidence_id": sorted({i for i in ids if ids.count(i) > 1})})
    if doubled:
        gap("6", "a belief counts the same purchase more than once: "
                 + json.dumps(doubled)
                 + ". Each record_feedback for the same purchase appends that purchase's evidence "
                   "to the belief again, so one kept purchase's repeated feedback can lift the "
                   "belief's standing. The pool row stays one (the writer is idempotent there). "
                   "A layer defect for the handoff")
    raw_rows = ledger.get("evidence", [])
    seen = set()
    evidence_rows = []
    duplicates = []
    for row in raw_rows:
        eid = row["evidence"]["evidence_id"]
        if eid in seen:
            duplicates.append(eid)
            continue
        seen.add(eid)
        evidence_rows.append({
            "evidence_id": eid,
            "kind": row["evidence"]["outcome"]["kind"],
            "reason": row["evidence"].get("reason"),
            "seeded": row["evidence"].get("seeded"),
            "belief_ids": sorted(set(row.get("belief_ids") or [])),
        })
    state = {
        "ledger_rows": rows,
        "evidence_rows": evidence_rows,
        "raw_evidence_rows": len(raw_rows),
        "duplicate_evidence_ids": sorted(set(duplicates)),
        "recall_decision": {
            "basis": recall.get("guidance", {}).get("basis"),
            "guidance": recall.get("guidance", {}).get("text"),
            "correction": correction_of(recall).get("correction_id"),
            "pool_precedence": (recall.get("pool") or {}).get("precedence"),
            "belief_precedence": [
                {"belief_id": b["belief_id"], "precedence": b.get("precedence")}
                for b in recall.get("beliefs", [])],
        },
        "screens": [],
    }
    for name, url in (("06-ledger-belief.png", f"{LAYER}/belief"),
                      ("06-ledger-table.png", f"{LAYER}/ledger")):
        shot = screenshot(name, url)
        if shot:
            state["screens"].append(shot)
    if duplicates:
        gap("6", "the live ledger lists a repeated evidence id "
                 f"({', '.join(sorted(set(duplicates)))}): a second record_feedback for the same "
                 "purchase appends the same row to its belief again, so the ledger shows the "
                 f"purchase more than once (raw rows {len(raw_rows)}). The evidence below is "
                 "deduplicated by id. A layer defect for the handoff")
    gap("6", "the deployed ledger page replays the frozen story from fixtures (its counts read "
             "60 settled / 48 seeded, the fixture vintage); it has no live transport yet. The "
             "live ledger reads are the API answers recorded here; the screens are the page as "
             "deployed and are labelled as a fixture replay. Owner: T4.8.")
    evidence("06-ledger.json", state)
    return state


def step7(correction):
    if not SECOND_DEPLOYMENT:
        raise RuntimeError("step 7: MEMENTO_SECOND_DEPLOYMENT is not set")
    conv = conversation(SECOND_DEPLOYMENT)
    text = "I need a Brand X oxford shirt. What size should I take?"
    record = turn("07", "second-agent", SECOND_DEPLOYMENT, conv, text)
    if not record["tools"] or record["tools"][0]["name"] != "recall_for_purchase":
        raise RuntimeError("step 7: the second agent did not recall first")
    if correction.get("correction_id") and correction["correction_id"] not in record["reply"]:
        if correction.get("text", "")[:20] not in record["reply"]:
            gap("7", "the second agent's reply does not quote the correction id or its words; "
                     "the trace's tool order and the reply are recorded as they came")

    status, fit = merchant_fit()
    state = {
        "correction": correction,
        "turn": record,
        "merchant_product_fit": pool_summary(fit) if status == 200 else {"status": status, "answer": fit},
        "screens": [],
    }
    shot = screenshot("07-merchant.png", f"{MERCHANT}/")
    if shot:
        state["screens"].append(shot)
    evidence("07-second-agent-and-merchant.json", state)
    return state


def main():
    global RUN, RAWDIR, EVIDENCE
    if not os.path.isfile(os.path.join(OUT, "RUN")):
        raise SystemExit("run demo/reset.sh <run-number> first")
    RUN = open(os.path.join(OUT, "RUN")).read().strip()
    RAWDIR = os.path.join(OUT, f"run-{RUN}")
    EVIDENCE = os.path.join("demo", "evidence", f"run-{RUN}")
    os.makedirs(RAWDIR, exist_ok=True)
    os.makedirs(EVIDENCE, exist_ok=True)

    baseline = {}
    if os.path.isfile(os.path.join(OUT, "baseline.json")):
        baseline = json.load(open(os.path.join(OUT, "baseline.json")))

    say(f"run {RUN}: layer {LAYER}")

    def load_evidence(name):
        with open(os.path.join(EVIDENCE, name)) as f:
            return json.load(f)

    def recorded_gaps(pooled, feedback):
        """The step gaps that follow from the recorded evidence itself."""
        attempt = pooled.get("new_shopper_recall", {})
        if attempt.get("status") != 200:
            gap("0", "a new shopper cannot be recalled live: the agent key is bound to "
                     "shopper_01 only, so shopper_02 answers "
                     f"{attempt.get('status')} {json.dumps(attempt.get('answer'))}. Sizing a "
                     "day-one shopper needs an operator to add the shopper to the agent key's "
                     "binding (a Render secret change).")
        tools = [t["name"] for t in feedback.get("turn", {}).get("tools", [])]
        if "recall_for_purchase" not in tools:
            gap("4", "the feedback turn called record_feedback without recalling first; recorded "
                     "as it came")

    prior_spend = None
    if "--summary-only" in sys.argv:
        gaps_file = os.path.join(RAWDIR, "gaps.json")
        if os.path.isfile(gaps_file):
            GAPS[:] = [g for g in json.load(open(gaps_file))
                       if g["step"] not in ("0", "4")]
        prior_summary = os.path.join(EVIDENCE, "summary.json")
        prior_spend = json.load(open(prior_summary)).get("spend") if os.path.isfile(prior_summary) else None
        pooled = load_evidence("00-pool-and-new-shopper.json")
        seeded = load_evidence("01-three-returns-and-belief.json")
        corrected = load_evidence("03-correction-through-the-agent.json")
        feedback = load_evidence("04-purchase-and-feedback.json")
        dispute = load_evidence("05-dispute-leaves-sizing-alone.json")
        ledger = load_evidence("06-ledger.json")
        second = load_evidence("07-second-agent-and-merchant.json")
        recorded_gaps(pooled, feedback)
        say("summary rebuilt from the recorded evidence; no live call")
    elif "--resume" in sys.argv:
        # A later step failed on a transient answer from the agent platform:
        # keep the paid turns already recorded, and re-read the free layer
        # state plus drive the missing turn.
        gaps_file = os.path.join(RAWDIR, "gaps.json")
        if os.path.isfile(gaps_file):
            kept = [g for g in json.load(open(gaps_file)) if g["step"] in ("0", "4")]
            GAPS[:] = kept
        pooled = load_evidence("00-pool-and-new-shopper.json")
        corrected = load_evidence("03-correction-through-the-agent.json")
        feedback = load_evidence("04-purchase-and-feedback.json")
        say("resuming from the recorded steps; re-reading the free layer state and driving step 7")
        seeded, _ = step12()
        dispute = step5()
        ledger = step6()
        second = step7(corrected["correction_after"])
    else:
        ref_conv = conversation(REF_DEPLOYMENT)
        say("reference conversation opened")
        pooled = step0()
        seeded, _ = step12()
        corrected, _ = step3(ref_conv)
        feedback = step4()
        dispute = step5()
        ledger = step6()
        second = step7(corrected["correction_after"])

    after = billing()
    if "--summary-only" in sys.argv and prior_spend:
        spend = prior_spend
    else:
        spend = None
    trace_costs = {}
    for record in (corrected["turns"] + [feedback["turn"], second["turn"]]):
        trace_costs[record["trace_id_short"]] = record["cost"]
    if spend is None:
        spend = {
            "billing_before": baseline,
            "billing_after": after,
            "usage_spend_delta": round((after.get("usage_spend", 0) - baseline.get("usage_spend", 0)), 4)
            if baseline and after else None,
            "trace_costs": trace_costs,
            "trace_total": round(sum(c for c in trace_costs.values() if c), 4),
            "note": "trace costs are the agent turns; the billing delta also carries platform compute",
        }

    normalise_gaps()
    before_correction = corrected.get("belief_before_correction")
    summary = {
        "run": RUN,
        "layer": LAYER,
        "merchant": MERCHANT,
        "pool": pooled,
        "seeded_returns": seeded,
        "belief_before_correction": before_correction,
        "belief_before_correction_note": None if before_correction else (
            "Not recorded: this summary was rebuilt from a run whose run-time pre-turn recall "
            "was not kept. The full pass was resumed after step 3, and the resume re-read step 1 "
            "after the correction, replacing demo/out/run-6/01-recall.json with a "
            "post-correction read; step 3's evidence kept the correction views, not the belief "
            "state the turn started from. A full run records it (demo/out/recall-baseline.json "
            "holds the reset baseline, the same state, but it is the reset's read, not the "
            "run's)."
        ),
        "correction": corrected,
        "purchase_and_feedback": feedback,
        "dispute": dispute,
        "ledger": ledger,
        "second_agent": second,
        "spend": spend,
        "gaps": GAPS,
    }
    save("summary.json", summary)
    evidence("summary.json", summary)
    with open(os.path.join(RAWDIR, "summary.md"), "w") as f:
        f.write(f"# Run {RUN}\n\n")
        for g in GAPS:
            f.write(f"- step {g['step']}: {g['text']}\n")
    say("spend", json.dumps(spend))
    say("run complete; evidence in", EVIDENCE)


if __name__ == "__main__":
    main()
