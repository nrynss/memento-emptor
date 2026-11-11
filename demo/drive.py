#!/usr/bin/env python3
"""T3.6 messaging driver: one conversation with the deployed reference agent.

Route and auth follow the pattern T3.4 proved: the platform messaging proxy
at astropods.com, authenticated with the CLI's own login held in process
memory and never printed or stored. The token travels only in the
Authorization header; SHOPPER/merchant values never enter this file.

The org the token belongs to is account detail: the driver reads it from the
ast CLI's own config (`~/.ast/credentials.json`), or from ASTRO_ORG when more
than one org is logged in, and never writes it here. An org that cannot be
resolved stops the driver with a message before any call is made.

Usage:
  OUT=demo/out python3 demo/drive.py create
  OUT=demo/out python3 demo/drive.py send <conv-id> <text>
  OUT=demo/out python3 demo/drive.py history <conv-id>

Every call re-checks billing and appends a redacted line to OUT/turns.log.
Bodies are redacted to 2000 chars; ids stay in OUT (git-ignored) and out of
committed records, which use CONV-T36-R<n> placeholders.
"""
import json
import os
import subprocess
import sys
import urllib.request
import urllib.error

API = "https://astropods.com/api/v1"
DEPLOYMENT = "9d3-n2x-s0a"
UA = ("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/126.0 Safari/537.36")

OUT = os.environ.get("OUT", "demo/out")

# The ast CLI's own login file, and the org it is logged in to. The org id is
# account detail, so it is read from here or from ASTRO_ORG, never written in
# this file.
ASTRO_CONFIG = "~/.ast/credentials.json"


def log(*parts):
    print("[drive]", *parts, flush=True, file=sys.stderr)
    os.makedirs(OUT, exist_ok=True)
    with open(os.path.join(OUT, "turns.log"), "a") as f:
        f.write(" ".join(str(p) for p in parts) + "\n")


def astro_org():
    """The Astropods org the CLI is logged in to, from ASTRO_ORG or its config.

    ASTRO_ORG names the org when more than one is logged in; otherwise the
    current profile's single org is the answer. There is no default: an org
    that cannot be resolved stops the driver before it makes a call.
    """
    named = os.environ.get("ASTRO_ORG", "").strip()
    if named:
        return named
    try:
        with open(os.path.expanduser(ASTRO_CONFIG)) as f:
            config = json.load(f)
    except OSError as e:
        raise SystemExit(f"drive: no org: set ASTRO_ORG, or log in with the ast CLI "
                         f"({ASTRO_CONFIG}: {e.strerror})")
    except ValueError:
        raise SystemExit(f"drive: no org: {ASTRO_CONFIG} is not JSON; set ASTRO_ORG")
    profiles = config.get("profiles") if isinstance(config, dict) else None
    current = config.get("current_profile") if isinstance(config, dict) else None
    profile = profiles.get(current or "") if isinstance(profiles, dict) else None
    tokens = profile.get("org_tokens") if isinstance(profile, dict) else None
    orgs = sorted(tokens) if isinstance(tokens, dict) else []
    if not orgs:
        raise SystemExit(f"drive: no org: the ast CLI profile in {ASTRO_CONFIG} holds none; "
                         "set ASTRO_ORG")
    if len(orgs) > 1:
        raise SystemExit(f"drive: no org: the ast CLI profile in {ASTRO_CONFIG} holds "
                         f"{len(orgs)}; set ASTRO_ORG to name one")
    return orgs[0]


ORG = astro_org()


def token():
    out = subprocess.run(
        ["secret-tool", "lookup", "service", "astro-cli",
         "username", f"default_{ORG}_access_token"],
        capture_output=True)
    if out.returncode != 0 or not out.stdout.strip():
        raise SystemExit("drive: no token in the keyring for the resolved org "
                         f"(ASTRO_ORG or {ASTRO_CONFIG}); log in with the ast CLI")
    return out.stdout.decode().strip()


def call(method, path, body=None):
    req = urllib.request.Request(
        f"{API}/deployments/{DEPLOYMENT}/messaging{path}",
        method=method,
        headers={"Authorization": "Bearer " + token(),
                 "Content-Type": "application/json",
                 "Accept": "application/json",
                 "User-Agent": UA})
    if body is not None:
        req.data = json.dumps(body).encode()
    try:
        with urllib.request.urlopen(req, timeout=180) as r:
            return r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()[:2000]


def billing():
    out = subprocess.run(["ast", "billing", "get"],
                         capture_output=True, text=True, timeout=60)
    for line in out.stdout.splitlines():
        if any(k in line for k in ("Usage", "Credit", "Limit")):
            log("billing:", line.strip())


def main():
    cmd = sys.argv[1]
    billing()
    if cmd == "create":
        s, b = call("POST", "/conversations", {})
        log("create status:", s)
        log("create body:", b[:2000])
        print(b)
    elif cmd == "send":
        conv, text = sys.argv[2], sys.argv[3]
        s, b = call("POST", f"/conversations/{conv}/messages",
                    {"content": text})
        log("send status:", s)
        log("send body:", b[:2000])
        if s // 100 != 2:
            # A non-2xx (e.g. a retryable 502 from the edge) must fail
            # loudly so the caller can back off and retry; nothing is
            # printed to stdout, so no partial body enters the pipeline.
            print(b[:2000], file=sys.stderr)
            sys.exit(2)
        print(b)
    elif cmd == "history":
        conv = sys.argv[2]
        s, b = call("GET", f"/conversations/{conv}/history")
        log("history status:", s)
        with open(os.path.join(OUT, "history.json"), "w") as f:
            f.write(b)
        log("history saved, chars:", len(b))
        print(b[:2000])


if __name__ == "__main__":
    main()
