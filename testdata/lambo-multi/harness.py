"""Shared rig for the Lambo multi-shopper checks.

Builds the configs, starts and stops the processes, writes transcripts and
talks to Elasticsearch. Transcripts are scrubbed here, once: the bearer token
becomes ``<token>``, the deployment URL ``<deployment>``, the run directory
``$RUN`` and the home directory ``~``. The run directory is under /tmp and the
SQLite file is never committed.
"""

from __future__ import annotations

import http.client
import json
import os
import re
import shutil
import signal
import socket
import subprocess
import time
import urllib.parse

import mcplib

BINARY = os.environ.get("LAMBO_BIN", "")
RUN_ROOT = os.environ.get("T011_RUN", "/tmp/opencode/t011/run")
TOKEN_ENV = "LAMBO_MULTI_TOKEN"
TOKEN = os.environ.get(TOKEN_ENV, "")


def run_dir(name: str, fresh: bool = True) -> str:
    path = os.path.join(RUN_ROOT, name)
    if fresh and os.path.isdir(path):
        shutil.rmtree(path)
    os.makedirs(path, exist_ok=True)
    return path


def base_config(path: str, store_path: str, extra_recall: str = "") -> str:
    """The daemon cadence from the T0.3 toml, SQLite, the fixture embedder."""
    text = f"""[store]
kind = "sqlite"
path = "{store_path}"

[embedder]
kind = "fixture"
dim = 1024
calibration = "off"

[daemon]
gc_interval = 1
gc_idle_floor = 1
gc_max_interval_secs = 3600
canonization_eval_interval_secs = 5
"""
    if extra_recall:
        text += "\n" + extra_recall
    with open(path, "w") as fh:
        fh.write(text)
    return path


def serve_section(pinned: str, max_attached: int = 32, idle_s: int = 900,
                  prefix: str = "shopper-") -> str:
    """One credential whose prefix reaches every shopper session on demand."""
    return f"""
[serve]
sessions = ["{pinned}"]
default_session = "{pinned}"
max_attached = {max_attached}
attach_concurrency = 1
idle_detach_secs = {idle_s}

[[serve.credential]]
name = "memento"
token_env = "{TOKEN_ENV}"
session_prefix = "{prefix}"
create = true
"""


def recall_section(url: str, prefix: str) -> str:
    return f"""
[recall]
kind = "elastic"
url = "{url}"
api_key = {{ env = "LAMBO_ES_API_KEY" }}
index_prefix = "{prefix}"
refresh = "true"
timeout_ms = 10000
"""


class Proc:
    """A supervised Lambo process whose stderr is scanned for its port."""

    def __init__(self, argv: list[str], log_path: str, port_re: str | None):
        self.log_path = log_path
        self.log = open(log_path, "wb")
        self.proc = subprocess.Popen(
            argv, stdin=subprocess.DEVNULL, stdout=self.log, stderr=self.log,
            start_new_session=True,
        )
        self.started = time.monotonic()
        self.port = None
        if port_re:
            self.port = self._wait_for_port(port_re)

    def _wait_for_port(self, pattern: str, timeout_s: float = 90.0) -> int:
        regex = re.compile(pattern)
        deadline = time.monotonic() + timeout_s
        while time.monotonic() < deadline:
            if self.proc.poll() is not None:
                raise RuntimeError(f"process exited: {self.log_text()[-2000:]}")
            text = self.log_text()
            match = regex.search(text)
            if match:
                return int(match.group(1))
            time.sleep(0.02)
        raise TimeoutError(f"no {pattern!r} in {self.log_text()[-2000:]}")

    def log_text(self) -> str:
        self.log.flush()
        with open(self.log_path, "r", errors="replace") as fh:
            return fh.read()

    def stop(self) -> int:
        if self.proc.poll() is None:
            self.proc.send_signal(signal.SIGTERM)
            try:
                self.proc.wait(timeout=45)
            except subprocess.TimeoutExpired:
                self.proc.kill()
                self.proc.wait(timeout=15)
        return self.proc.returncode

    def wait(self, needle: str, timeout_s: float = 90.0) -> None:
        deadline = time.monotonic() + timeout_s
        while time.monotonic() < deadline:
            if needle in self.log_text():
                return
            time.sleep(0.02)
        raise TimeoutError(f"no {needle!r} in log: {self.log_text()[-2000:]}")


def provision(config: str) -> None:
    """`lambo provision` is idempotent and writes the SQLite schema."""
    result = subprocess.run([BINARY, "--config", config, "provision"],
                            capture_output=True, text=True)
    if result.returncode != 0:
        raise RuntimeError(f"provision failed: {result.stderr[-2000:]}")


def start_hub(config: str, pinned: str, run: str, name: str = "hub",
              max_sessions: int = 128, extra: list[str] | None = None) -> Proc:
    provision(config)
    argv = [
        BINARY, "--config", config, "serve", "--transport", "http",
        "--port", "0", "--bind", "127.0.0.1", "--session", pinned,
        "--agent", "memento-hub", "--max-sessions", str(max_sessions),
        "--rate-limit-rps", "1000",
    ] + (extra or [])
    # The token reaches the child through the environment only.
    log_path = os.path.join(run, name + ".log")
    old = os.environ.get(TOKEN_ENV)
    os.environ[TOKEN_ENV] = TOKEN
    try:
        return Proc(argv, log_path, r"addr=127\.0\.0\.1:(\d+)")
    finally:
        if old is None:
            os.environ.pop(TOKEN_ENV, None)
        else:
            os.environ[TOKEN_ENV] = old


def start_web(config: str, sessions: list[str], run: str, name: str = "web",
              list_sessions: bool = True) -> Proc:
    provision(config)
    argv = [BINARY, "--config", config, "serve-web", "--port", "0",
            "--bind", "127.0.0.1"]
    for s in sessions:
        argv += ["--session", s]
    log_path = os.path.join(run, name + ".log")
    return Proc(argv, log_path, r"http://127\.0\.0\.1:(\d+)")


def client(session: str, port: int, agent: str = "memento-t011") -> mcplib.HttpMCP:
    return mcplib.HttpMCP("127.0.0.1", port, session, TOKEN, agent)


class Transcript:
    """A JSONL file of what the checks asked and got, scrubbed on write."""

    def __init__(self, path: str):
        self.path = path
        self.fh = open(path, "w")

    def add(self, **fields) -> None:
        self.fh.write(json.dumps(_scrub(fields), ensure_ascii=False) + "\n")
        self.fh.flush()

    def append(self, entry: dict) -> None:
        """`ToolSession`'s record hook: one line per tool call."""
        self.add(event="tool_call", **entry)

    def close(self) -> None:
        self.fh.close()


def _scrub(value):
    if isinstance(value, str):
        out = value
        if TOKEN:
            out = out.replace(TOKEN, "<token>")
        url = os.environ.get("ELASTIC_URL", "")
        if url:
            out = out.replace(url, "<deployment>")
            host = urllib.parse.urlparse(url).hostname
            if host:
                out = out.replace(host, "<deployment-host>")
        out = out.replace(RUN_ROOT, "$RUN")
        home = os.path.expanduser("~")
        if home and home != "/":
            out = out.replace(home, "~")
        return out
    if isinstance(value, dict):
        return {k: _scrub(v) for k, v in value.items()}
    if isinstance(value, list):
        return [_scrub(v) for v in value]
    return value


# --- Elasticsearch (check 4 only) -------------------------------------------


def es(method: str, path: str, body: dict | None = None) -> tuple[int, dict]:
    url = urllib.parse.urlparse(os.environ["ELASTIC_URL"])
    conn = http.client.HTTPSConnection(url.hostname, url.port or 443, timeout=60)
    payload = json.dumps(body).encode() if body is not None else None
    headers = {
        "Authorization": "ApiKey " + os.environ["LAMBO_ES_API_KEY"],
        "Content-Type": "application/json",
    }
    conn.request(method, path, body=payload, headers=headers)
    resp = conn.getresponse()
    raw = resp.read()
    conn.close()
    text = raw.decode("utf-8", "replace")
    try:
        parsed = json.loads(text) if text else {}
    except json.JSONDecodeError:
        parsed = {"raw": text[:2000]}
    return resp.status, parsed


def es_indices(pattern: str) -> list[str]:
    status, body = es("GET", f"/_cat/indices/{pattern}?format=json&h=index&expand_wildcards=all")
    if status == 404:
        return []
    assert status == 200, (status, body)
    return sorted(row["index"] for row in body)


def es_delete(pattern: str) -> None:
    for index in es_indices(pattern):
        status, body = es("DELETE", "/" + index)
        if status not in (200, 404):
            raise RuntimeError(f"delete {index}: {status} {body}")


# --- assertion helpers -------------------------------------------------------


def check(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def wait_until(fn, timeout_s: float = 60.0, interval_s: float = 0.5):
    deadline = time.monotonic() + timeout_s
    last = None
    while time.monotonic() < deadline:
        last = fn()
        if last:
            return last
        time.sleep(interval_s)
    raise TimeoutError(f"condition not met in {timeout_s}s: {last!r}")


def free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]
