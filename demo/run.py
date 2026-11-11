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
  ASTRO_ORG                   the org key the messaging token belongs to;
                              default: the ast CLI profile's own org, read
                              from ~/.ast/credentials.json. The id is
                              account detail, so it is never in this file.
  OUT                         default demo/out

Usage:
  bash demo/reset.sh <run-number>
  bash demo/run.sh
  python3 demo/run.py --step2-trace <trace-id> [<evidence-file>]

--step2-trace records step 2 from a turn sent through demo/drive.py, reading
the platform's trace: it makes no live call of its own and writes the same
evidence a full run's step 2 writes. The optional evidence-file names the file
it writes (default 02-catalog-candidate.json), so a later turn can be recorded
beside an earlier one without overwriting it.
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
    "record_feedback", "explain_belief", "product_fit", "set_pooling", "search_catalog",
    "ledger_beliefs", "ledger_belief", "ledger_evidence",
)

# Step 2 asks for a product, not a size, so the turn goes to the catalog
# without a correction in the prompt.
STEP2_ASK = ("I need a Brand X oxford shirt in the regular fit. "
             "Which one would you recommend?")
# The catalog recording the agent's tool replays. The demo cut is the oxford
# line, so this is the answer a step-2 search works from.
CATALOG_RECORDING = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                                 "..", "testdata", "channel3", "search-oxford-shirts.json")
# The cut the demo asks for, in Channel3's own vocabulary. The layer's line
# identity and Channel3's differ (brand-x-001 / shirts / regular is Channel3's
# SMwk / dress-shirts / top-fit Regular), and that mapping is demo identity,
# carried here rather than derived from the layer.
CATALOG_CUT = {"brand": "SMwk", "category": "dress-shirts",
               "fit": "top-fit", "fit_value": "Regular"}
# The SKUs the shopper's memory already carries for the demo line: the three
# seeded returns (tbnrl46) and the captured purchase (PSW8GeK). A candidate
# is a different SKU; a turn that names one of these named the returned item.
RETURNED_SKUS = frozenset({"tbnrl46", "PSW8GeK"})

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


def normalise_gaps(catalog):
    """Give every gap its owner, and record step 2's from the run's own evidence."""
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
    # A run recorded before the agent had a catalog search has no step-2
    # catalog evidence; its summary keeps the gap that run carried. A run
    # whose turn searched but named a SKU the shopper's memory already
    # carries searched the catalog without offering a new candidate, so its
    # summary carries that limit rather than reading as a clean step 2.
    def recorded(needle):
        return any(needle in entry["text"] for entry in out)

    if catalog is None:
        if not recorded("product-search tool"):
            out.append({
                "step": "2",
                "text": ("the reference agent exposes no product-search tool, so no live turn "
                         "names the Channel3 candidate; the SKU exists only in step 4's captured "
                         "purchase. Owner: T9.17."),
            })
    else:
        candidate = catalog.get("candidate") or {}
        recall = catalog.get("recall") or {}
        if (candidate.get("returned_sku")
                and not recorded("returned item rather than a new one")):
            out.append({
                "step": "2",
                "text": ("the step-2 turn searched the catalog but named "
                         f"{candidate.get('sku')}, a SKU the shopper's memory already carries, "
                         "so the candidate is a returned item rather than a new one. "
                         "Owner: T9.22."),
            })
        if (recall.get("called") and not recall.get("answered_before_search")
                and not recorded("before recall answered")):
            out.append({
                "step": "2",
                "text": ("the step-2 turn issued search_catalog before recall answered, so the "
                         "search was never given the returned SKUs to exclude. "
                         "Owner: T9.22."),
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
    """Drop full host trace ids from committed evidence (short ids stay).

    Query strings are stripped from URLs as well: a catalog offer link can
    carry an affiliate id, which is account detail and never enters the
    committed evidence.
    """
    if isinstance(value, dict):
        return {k: redact(v) for k, v in value.items() if k != "trace_id"}
    if isinstance(value, list):
        return [redact(v) for v in value]
    if isinstance(value, str):
        return re.sub(r"https?://[^\s)\]\"']+",
                      lambda m: m.group(0).split("?", 1)[0], value)
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

# The ast CLI's own login file, and the org it is logged in to: account
# detail, read here and never written in this file.
ASTRO_CONFIG = "~/.ast/credentials.json"


def astro_org():
    """The Astropods org the CLI is logged in to, from ASTRO_ORG or its config.

    ASTRO_ORG names the org when more than one is logged in; otherwise the
    current profile's single org is the answer. There is no default: an org
    that cannot be resolved stops the run before it makes a messaging call.
    """
    named = os.environ.get("ASTRO_ORG", "").strip()
    if named:
        return named
    try:
        with open(os.path.expanduser(ASTRO_CONFIG)) as f:
            config = json.load(f)
    except OSError as e:
        raise SystemExit(f"[t97] no org: set ASTRO_ORG, or log in with the ast CLI "
                         f"({ASTRO_CONFIG}: {e.strerror})")
    except ValueError:
        raise SystemExit(f"[t97] no org: {ASTRO_CONFIG} is not JSON; set ASTRO_ORG")
    profiles = config.get("profiles") if isinstance(config, dict) else None
    current = config.get("current_profile") if isinstance(config, dict) else None
    profile = profiles.get(current or "") if isinstance(profiles, dict) else None
    tokens = profile.get("org_tokens") if isinstance(profile, dict) else None
    orgs = sorted(tokens) if isinstance(tokens, dict) else []
    if not orgs:
        raise SystemExit(f"[t97] no org: the ast CLI profile in {ASTRO_CONFIG} holds none; "
                         "set ASTRO_ORG")
    if len(orgs) > 1:
        raise SystemExit(f"[t97] no org: the ast CLI profile in {ASTRO_CONFIG} holds "
                         f"{len(orgs)}; set ASTRO_ORG to name one")
    return orgs[0]


def astro_token():
    global _ASTRO_TOKEN
    if _ASTRO_TOKEN is None:
        out = subprocess.run(
            ["secret-tool", "lookup", "service", "astro-cli",
             "username", f"default_{astro_org()}_access_token"],
            capture_output=True)
        if out.returncode != 0 or not out.stdout.strip():
            raise SystemExit("[t97] no token in the keyring for the resolved org "
                             f"(ASTRO_ORG or {ASTRO_CONFIG}); log in with the ast CLI")
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


def catalog_products():
    """The demo cut's recording, every product it holds, by id."""
    with open(CATALOG_RECORDING) as f:
        recorded = json.load(f)
    return {product["id"]: product for product in recorded.get("products", [])
            if isinstance(product, dict) and isinstance(product.get("id"), str)}


def cut_products():
    """The recording's products that match the cut the step-2 ask names.

    The trace carries tool span names and times, not the tool's answer, so
    this is the demo's reading of the recording rather than a copy of what
    the search returned. It keeps the check from accepting a product the
    cut excludes.
    """
    out = {}
    for sku, product in catalog_products().items():
        brands = {b.get("id") for b in product.get("brands") or [] if isinstance(b, dict)}
        category = (product.get("category") or {}).get("slug")
        attrs = product.get("structured_attributes")
        if isinstance(attrs, list):
            attrs = {a.get("name"): a.get("value") for a in attrs if isinstance(a, dict)}
        fit = (attrs or {}).get(CATALOG_CUT["fit"]) or []
        if isinstance(fit, str):
            fit = [fit]
        if (CATALOG_CUT["brand"] in brands
                and category == CATALOG_CUT["category"]
                and CATALOG_CUT["fit_value"] in fit):
            out[sku] = product
    return out


def answer_generation(detail):
    """When the answer's model generation starts: the last chat generation."""
    starts = [obs.get("start_time") for obs in detail.get("observations", [])
              if obs.get("type") == "generation"
              and (obs.get("name") or "").startswith("chat ")
              and obs.get("start_time")]
    return max(starts) if starts else None


SIZE_WORDS = "XXS|XS|S|M|L|XL|XXL"


def recommended_size(reply):
    """The size the reply recommends and where it names it, from the reply alone."""
    for pattern in (rf"\bsize:\s*\**({SIZE_WORDS})\b",
                    rf"\bsize\s+\**({SIZE_WORDS})\b",
                    rf"\b(?:recommend|recommended|take|go with)\b[^.\n]{{0,40}}\b({SIZE_WORDS})\b"):
        found = re.search(pattern, reply, re.IGNORECASE)
        if found:
            return found.group(1).upper(), found.start(1)
    return None, None


def catalog_use(detail, record):
    """Step 2's catalog evidence: recall's answer, the search, and the candidate.

    The trace carries tool span names and times, not their arguments or
    answers, so the named line and SKU are read from the reply and checked
    against the recording the tool replays. The size comes from recall: the
    catalog carries no size for the oxford line (variants is null). A search
    that starts before recall answers is recorded as it came rather than
    dropped, because that ordering is what step 2 is asked to prove.
    """
    recall = next((s for s in record["tools"] if s["name"] == "recall_for_purchase"), None)
    searches = sorted((s for s in record["tools"] if s["name"] == "search_catalog"),
                      key=lambda s: s["start"] or "")
    if not searches:
        raise RuntimeError("step 2: the turn did not search the catalog")
    catalog = searches[-1]
    answer_at = answer_generation(detail)
    if not catalog.get("end") or not answer_at or catalog["end"] > answer_at:
        raise RuntimeError("step 2: the catalog search did not finish before the answer")
    named = []
    cut = cut_products()
    for sku, product in catalog_products().items():
        at = re.search(r"\b" + re.escape(sku) + r"\b", record["reply"])
        if not at:
            continue
        title = product.get("title") or ""
        line = re.search(re.escape(title), record["reply"], re.IGNORECASE) if title else None
        named.append({
            "sku": sku,
            "line": title,
            "sku_at": at.start(),
            "line_at": line.start() if line else None,
            "line_named": line is not None,
            "in_cut": sku in cut,
            "returned_sku": sku in RETURNED_SKUS,
        })
    if not named:
        raise RuntimeError("step 2: the reply names no catalog SKU")
    in_cut = [n for n in named if n["in_cut"]]
    if not in_cut:
        raise RuntimeError("step 2: the reply names no SKU the demo cut matches")
    named.sort(key=lambda n: n["sku_at"])
    candidate = min(in_cut, key=lambda n: min([p for p in (n["line_at"], n["sku_at"])
                                               if p is not None]))
    if not candidate["line_named"]:
        raise RuntimeError(f"step 2: the reply names SKU {candidate['sku']} but not its line")
    size, size_at = recommended_size(record["reply"])
    if size is None:
        raise RuntimeError("step 2: the reply names no recommended size")
    candidate_at = min(p for p in (candidate["line_at"], candidate["sku_at"]) if p is not None)
    # Whether recall answered before the search was issued is the sequencing
    # T9.22's instruction asks for: the search must start after recall ends.
    recall_block = {
        "called": recall is not None,
        "start": recall.get("start") if recall else None,
        "end": recall.get("end") if recall else None,
        "started_before_search": bool(recall and recall.get("start") and catalog.get("start")
                                      and recall["start"] <= catalog["start"]),
        "answered_before_search": bool(recall and recall.get("end") and catalog.get("start")
                                       and recall["end"] <= catalog["start"]),
    }
    return {
        "recall": recall_block,
        "catalog": {
            "searched": True,
            "tool": "search_catalog",
            "calls": [{"start": s["start"], "end": s["end"]} for s in searches],
            "search_start": catalog.get("start"),
            "search_end": catalog.get("end"),
            "answer_generation_start": answer_at,
            "searched_before_answer": True,
            "search_after_recall": recall_block["answered_before_search"],
            "named_skus": [n["sku"] for n in named],
            "off_cut_named": [n["sku"] for n in named if not n["in_cut"]],
            "match_basis": ("the demo's reading of the recording: brand "
                            f"{CATALOG_CUT['brand']}, category {CATALOG_CUT['category']}, "
                            f"{CATALOG_CUT['fit']} {CATALOG_CUT['fit_value']}"),
        },
        "candidate": {
            "line": candidate["line"],
            "sku": candidate["sku"],
            "size": size,
            "size_source": ("recall: the catalog carries no size for this line "
                            "(variants is null)"),
            "line_named": candidate["line_named"],
            "named_before_size": candidate_at < size_at,
            "in_cut": candidate["in_cut"],
            "returned_sku": candidate["returned_sku"],
            "note": ("the named SKU is one the shopper's memory already carries; it matches "
                     "the cut, but it is not a new candidate") if candidate["returned_sku"]
                    else ("the named SKU is in the demo cut and none of the shopper's returned "
                          "rows carries it"),
        },
        "channel3": {"searches": len(searches), "credits": len(searches),
                     "note": ("one search per tool call costs one credit; product detail calls "
                              "are free")},
    }


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


def step2(ref_conv):
    """Ask for a product so the turn recalls, then searches the catalog, before it recommends."""
    record = turn("02", "catalog", REF_DEPLOYMENT, ref_conv, STEP2_ASK)
    state = {"turn": record}
    state.update(catalog_use(trace_detail(REF_DEPLOYMENT, record["trace_id"]), record))
    evidence("02-catalog-candidate.json", state)
    return state


def step2_trace(trace_id):
    """Record a step-2 turn sent through demo/drive.py, from the turn's trace."""
    detail = trace_detail(REF_DEPLOYMENT, trace_id)
    found = detail.get("trace", {})
    record = {
        "step": "02",
        "label": "catalog",
        "deployment": "MEMENTO-REF",
        "trace_id": trace_id,
        "trace_id_short": trace_id[:8],
        "cost": found.get("total_cost"),
        "latency_ms": found.get("latency_ms"),
        "timestamp": found.get("timestamp"),
        "input": found.get("input"),
        "tools": [{"name": s["name"], "start": s["start"], "end": s["end"]}
                  for s in tool_spans(detail)],
        "reply": reply_text(detail),
    }
    state = {"turn": record}
    state.update(catalog_use(detail, record))
    return state


def step2_main(trace_id, name="02-catalog-candidate.json"):
    """Write step 2's evidence for a trace, with no live call of its own."""
    global RAWDIR, EVIDENCE
    RAWDIR = os.path.join(OUT, "step2-catalog")
    EVIDENCE = os.path.join("demo", "evidence", "step2-catalog")
    os.makedirs(RAWDIR, exist_ok=True)
    os.makedirs(EVIDENCE, exist_ok=True)
    state = step2_trace(trace_id)
    save(name, state)
    evidence(name, state)
    candidate = state["candidate"]
    say("step 2: catalog evidence for trace", trace_id[:8], "-",
        candidate["line"], candidate["sku"], "size", candidate["size"],
        "| recall first:", state["recall"]["answered_before_search"],
        "| returned SKU:", candidate["returned_sku"])


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

    def load_catalog():
        """Step 2's evidence when this run has it; a pre-catalog run does not."""
        path = os.path.join(EVIDENCE, "02-catalog-candidate.json")
        return load_evidence("02-catalog-candidate.json") if os.path.isfile(path) else None

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
        catalog = load_catalog()
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
        catalog = load_catalog()
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
        catalog = step2(ref_conv)
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
    paid = ([catalog["turn"]] if catalog else []) + corrected["turns"] + [
        feedback["turn"], second["turn"]]
    for record in paid:
        trace_costs[record["trace_id_short"]] = record["cost"]
    if spend is None:
        spend = {
            "billing_before": baseline,
            "billing_after": after,
            "usage_spend_delta": round((after.get("usage_spend", 0) - baseline.get("usage_spend", 0)), 4)
            if baseline and after else None,
            "trace_costs": trace_costs,
            "trace_total": round(sum(c for c in trace_costs.values() if c), 4),
            "channel3_credits": (catalog or {}).get("channel3", {}).get("credits"),
            "note": ("trace costs are the agent turns; the billing delta also carries platform "
                     "compute. Channel3 searches are one credit each and are not in the billing "
                     "figure"),
        }

    normalise_gaps(catalog)
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
        "step2_catalog": catalog,
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
    if "--step2-trace" in sys.argv:
        at = sys.argv.index("--step2-trace")
        if at + 1 >= len(sys.argv):
            raise SystemExit("usage: demo/run.py --step2-trace <trace-id> [<evidence-file>]")
        name = sys.argv[at + 2] if at + 2 < len(sys.argv) else "02-catalog-candidate.json"
        step2_main(sys.argv[at + 1], name)
    else:
        main()
