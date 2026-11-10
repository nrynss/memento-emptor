"""MCP clients for the Lambo multi-session checks.

Standard library only. Two transports:

* ``HttpMCP`` speaks streamable HTTP to ``lambo serve --transport http``,
  one client per addressed session (``/mcp/s/<session>``). Each client mints
  its own MCP session id from ``initialize``; nothing is shared between two
  clients but the server process.
* ``StdioMCP`` speaks newline-delimited JSON-RPC to ``lambo serve --session
  <one>``.

Every request is recorded by the caller, never here. No HTTP header is
written to a transcript: a bearer token would land in it.
"""

from __future__ import annotations

import http.client
import json
import os
import re
import signal
import socket
import subprocess
import time
import urllib.parse


def _decode_body(content_type: str, body: bytes) -> dict:
    """A JSON-RPC reply from a JSON or an SSE body."""
    text = body.decode("utf-8", "replace")
    if "text/event-stream" in content_type:
        frames = [
            line[len("data:"):].strip()
            for line in text.splitlines()
            if line.startswith("data:")
        ]
        for frame in reversed(frames):
            if frame and frame != "[DONE]":
                return json.loads(frame)
        raise RuntimeError(f"no data frame in SSE body: {text[:500]}")
    return json.loads(text)


class HttpMCP:
    """One MCP session addressed at ``/mcp/s/<session>``."""

    def __init__(self, host: str, port: int, session: str, token: str, agent: str):
        self.host = host
        self.port = port
        self.path = "/mcp/s/" + urllib.parse.quote(session, safe=":._-")
        self.token = token
        self.agent = agent
        self.session = session
        self.mcp_session_id: str | None = None
        self._next_id = 1

    def _post(self, body: dict) -> tuple[int, dict[str, str], bytes]:
        conn = http.client.HTTPConnection(self.host, self.port, timeout=120)
        payload = json.dumps(body).encode()
        headers = {
            "Content-Type": "application/json",
            "Accept": "application/json, text/event-stream",
            "Authorization": "Bearer " + self.token,
            "Content-Length": str(len(payload)),
        }
        if self.mcp_session_id:
            headers["Mcp-Session-Id"] = self.mcp_session_id
        conn.request("POST", self.path, body=payload, headers=headers)
        resp = conn.getresponse()
        raw = resp.read()
        got = {k.lower(): v for k, v in resp.getheaders()}
        conn.close()
        return resp.status, got, raw

    def initialize(self) -> dict:
        status, headers, raw = self._post(
            {
                "jsonrpc": "2.0",
                "id": self._next_id,
                "method": "initialize",
                "params": {
                    "protocolVersion": "2025-06-18",
                    "capabilities": {},
                    "clientInfo": {"name": "t011-harness", "version": "1"},
                },
            }
        )
        self._next_id += 1
        if status != 200:
            raise RuntimeError(f"initialize {self.path}: HTTP {status}: {raw[:400]!r}")
        self.mcp_session_id = headers.get("mcp-session-id")
        if not self.mcp_session_id:
            raise RuntimeError(f"initialize {self.path}: no Mcp-Session-Id header")
        reply = _decode_body(headers.get("content-type", ""), raw)
        status, _, raw = self._post({"jsonrpc": "2.0", "method": "notifications/initialized"})
        if status not in (200, 202, 204):
            raise RuntimeError(f"initialized notification {self.path}: HTTP {status}")
        return reply

    def list_tools(self) -> list[str]:
        reply = self.rpc("tools/list", {})
        return [t["name"] for t in reply["result"]["tools"]]

    def rpc(self, method: str, params: dict) -> dict:
        status, headers, raw = self._post(
            {"jsonrpc": "2.0", "id": self._next_id, "method": method, "params": params}
        )
        self._next_id += 1
        if status != 200:
            raise RuntimeError(f"{method} {self.path}: HTTP {status}: {raw[:400]!r}")
        return _decode_body(headers.get("content-type", ""), raw)

    def call(self, name: str, arguments: dict) -> dict:
        reply = self.rpc("tools/call", {"name": name, "arguments": arguments})
        if "error" in reply:
            raise RuntimeError(f"{name}: {reply['error']}")
        return reply["result"]

    def text(self, name: str, arguments: dict) -> str:
        result = self.call(name, arguments)
        return "\n".join(c.get("text", "") for c in result.get("content", []))


RECEIPT_RE = re.compile(r"receipt (lwr[0-9a-z.]+)")


class ToolSession:
    """The tool calls the checks use, over either transport."""

    def __init__(self, client, agent: str, transcript: list | None = None):
        self.client = client
        self.agent = agent
        self.transcript = transcript if transcript is not None else []

    def _record(self, name: str, arguments: dict, text: str) -> None:
        self.transcript.append(
            {"tool": name, "arguments": arguments, "result_text": text}
        )

    def call(self, name: str, arguments: dict) -> str:
        arguments = dict(arguments)
        arguments.setdefault("agent_id", self.agent)
        text = self.client.text(name, arguments)
        self._record(name, arguments, text)
        return text

    def derive(self, content: str, concept_type: str = "logic", event_time=None,
               depends_on=None) -> str:
        concept = {"content": content, "concept_type": concept_type}
        args = {"concepts": [concept]}
        if event_time:
            args["event_time"] = event_time
        if depends_on:
            args["depends_on"] = depends_on
        return self.call("lambo_derive", args)

    def action(self, action: str, depends_on=None, against=None, event_time=None) -> str:
        args = {"action": action}
        if depends_on:
            args["depends_on"] = depends_on
        if against:
            args["against"] = against
        if event_time:
            args["event_time"] = event_time
        return self.call("lambo_record_action", args)

    def recall(self, query: str) -> str:
        return self.call("lambo_recall", {"query": query})

    def inspect(self, focus: str, depth: int = 2) -> str:
        return self.call("lambo_inspect", {"focus": focus, "depth": depth})

    def inspect_struct(self, focus: str, depth: int = 2) -> dict:
        """The same call's structured content: admission, checks, blast radius."""
        arguments = {"focus": focus, "depth": depth, "agent_id": self.agent}
        result = self.client.call("lambo_inspect", arguments)
        self._record("lambo_inspect", arguments,
                     "\n".join(c.get("text", "") for c in result.get("content", [])))
        return result.get("structuredContent") or {}

    def stats(self, receipt: str | None = None, wait_ms: int | None = None) -> str:
        args = {}
        if receipt:
            args["receipt"] = receipt
        if wait_ms:
            args["wait_ms"] = wait_ms
        return self.call("lambo_stats", args)

    def wait(self, text: str, timeout_s: float = 60.0) -> str:
        """Wait for the receipt in a write's reply; return the outcome text."""
        match = RECEIPT_RE.search(text)
        if not match:
            return text
        receipt = match.group(1)
        deadline = time.monotonic() + timeout_s
        last = ""
        while time.monotonic() < deadline:
            last = self.stats(receipt=receipt, wait_ms=34000)
            if "applied" in last or "failed" in last or "refused" in last:
                return last
        raise TimeoutError(f"receipt {receipt} did not settle: {last[:500]}")

    def write(self, text: str, timeout_s: float = 60.0) -> str:
        return self.wait(text, timeout_s)


class StdioMCP:
    """One stdio serve: one session, one writer process."""

    def __init__(self, binary: str, config: str, session: str, agent: str,
                 stderr_path: str):
        self.proc = subprocess.Popen(
            [binary, "--config", config, "serve", "--session", session,
             "--agent", agent],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=open(stderr_path, "wb"),
            text=True,
            bufsize=1,
        )
        self.session = session
        self.agent = agent
        self._next_id = 1
        self.stderr_path = stderr_path

    def initialize(self) -> dict:
        reply = self.rpc(
            "initialize",
            {
                "protocolVersion": "2025-06-18",
                "capabilities": {},
                "clientInfo": {"name": "t011-harness", "version": "1"},
            },
        )
        self._write_frame({"jsonrpc": "2.0", "method": "notifications/initialized"})
        return reply

    def _write_frame(self, frame: dict) -> None:
        assert self.proc.stdin is not None
        self.proc.stdin.write(json.dumps(frame) + "\n")
        self.proc.stdin.flush()

    def rpc(self, method: str, params: dict) -> dict:
        msg_id = self._next_id
        self._next_id += 1
        self._write_frame({"jsonrpc": "2.0", "id": msg_id, "method": method, "params": params})
        assert self.proc.stdout is not None
        deadline = time.monotonic() + 120
        while time.monotonic() < deadline:
            line = self.proc.stdout.readline()
            if not line:
                raise RuntimeError(f"stdio serve closed stdout (stderr {self.stderr_path})")
            try:
                reply = json.loads(line)
            except json.JSONDecodeError:
                continue
            if reply.get("id") == msg_id:
                if "error" in reply:
                    raise RuntimeError(f"{method}: {reply['error']}")
                return reply
        raise TimeoutError(f"no reply to {method} on {self.session}")

    def call(self, name: str, arguments: dict) -> dict:
        return self.rpc("tools/call", {"name": name, "arguments": arguments})["result"]

    def text(self, name: str, arguments: dict) -> str:
        return "\n".join(
            c.get("text", "") for c in self.call(name, arguments).get("content", [])
        )

    def close(self) -> int:
        if self.proc.poll() is None:
            self.proc.stdin.close()
            try:
                return self.proc.wait(timeout=30)
            except subprocess.TimeoutExpired:
                self.proc.send_signal(signal.SIGTERM)
                return self.proc.wait(timeout=30)
        return self.proc.returncode


def wait_port(host: str, port: int, timeout_s: float = 60.0) -> None:
    deadline = time.monotonic() + timeout_s
    while time.monotonic() < deadline:
        try:
            with socket.create_connection((host, port), timeout=0.2):
                return
        except OSError:
            time.sleep(0.02)
    raise TimeoutError(f"{host}:{port} never listened")


def rss_kb(pid: int) -> int:
    with open(f"/proc/{pid}/status") as fh:
        for line in fh:
            if line.startswith("VmRSS:"):
                return int(line.split()[1])
    raise RuntimeError(f"no VmRSS for {pid}")


def open_fds(pid: int) -> int:
    return len(os.listdir(f"/proc/{pid}/fd"))
