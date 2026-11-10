#!/usr/bin/env python3
"""T9.5 messaging driver: turns with the second agent deployment.

Same route T3.4 proved: the platform messaging proxy at astropods.com,
authenticated with the CLI's own login held in process memory and never
printed or stored. The deployment id arrives by environment, never baked
in; ids stay in OUT (git-ignored) and out of committed records.

Usage:
  DEPLOYMENT=<id> OUT=<dir> python3 drive.py create
  DEPLOYMENT=<id> OUT=<dir> python3 drive.py send <conv-id> <text>
  DEPLOYMENT=<id> OUT=<dir> python3 drive.py history <conv-id>
"""
import json
import os
import subprocess
import sys
import urllib.request
import urllib.error

API = "https://astropods.com/api/v1"
DEPLOYMENT = os.environ["DEPLOYMENT"]
UA = ("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/126.0 Safari/537.36")

OUT = os.environ.get("OUT", "/tmp/opencode/memento-t95/out")


def log(*parts):
    print("[drive]", *parts, flush=True, file=sys.stderr)
    os.makedirs(OUT, exist_ok=True)
    with open(os.path.join(OUT, "turns.log"), "a") as f:
        f.write(" ".join(str(p) for p in parts) + "\n")


def token():
    out = subprocess.run(
        ["secret-tool", "lookup", "service", "astro-cli",
         "username", "default_org_01M46R4F87ZN49Y0SAKHQE8HP8_access_token"],
        capture_output=True, check=True)
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
    out = subprocess.run([os.path.expanduser("~/.ast/bin/ast"), "billing", "get"],
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


main()
