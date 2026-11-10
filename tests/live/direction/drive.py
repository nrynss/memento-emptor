#!/usr/bin/env python3
"""T2.7b live driver: re-prove the direction rule against a real Lambo.

Seeds the runs-small belief with three same-direction (too-small) returns,
each its own interaction citing the belief in depends_on (the frozen write
rule in internal/memory: a reason's direction must match the belief's
direction to count toward it). Then records ONE opposite-direction
(too-large) return as its own interaction with neither depends_on nor
against. This models the graph-level consequence of the refusal (opposing
content present but unlinked) to check the daemon does not auto-attach or
auto-contradict; it is not the service path itself, which this driver
bypasses with direct MCP calls. The refusal (writer touches nothing,
ErrDirectionMismatch) and the service routing (a match-less answer is
stored as an unattached ledger row with no graph write at all) are proved
offline by T2.7's writer and service tests.

Usage:
  drive.py --lambo BIN --config CFG --session S --agent A --out OUT [--probe]

--probe writes one concept and waits for its receipt, then exits 0 when the
embedder applied it or 2 when the embedder is unavailable (the DNS stall
from T0.6b).

The transcript records every MCP request/response pair, T0.3 style. It
never carries a key or URL: requests hold only demo-fiction contents, and
responses are checked for secret patterns before anything is committed.
"""

import argparse
import datetime
import json
import subprocess
import sys
import time

WAIT_MS = 34000

BELIEF = "Brand X oxford runs small for this shopper: take size L"
RET1 = "Fit return 1: Brand X oxford size M refunded; too tight in the shoulders"
RET2 = "Fit return 2: Brand X oxford size M refunded; too tight across the chest"
RET3 = "Fit return 3: Brand X oxford size M refunded; sleeves too short in M"
RET4 = "Fit return 4: Brand X oxford size M refunded; far too big, swims on me"
RECALL_Q = "Brand X oxford sizing for this shopper"
RECALL_OPPOSITE_Q = "Brand X oxford too big, swims on me"


def utcnow():
    return datetime.datetime.now(datetime.timezone.utc)


def rfc3339(moment):
    return moment.isoformat(timespec="microseconds")


class Driver:
    def __init__(self, proc, agent, transcript):
        self.proc = proc
        self.agent = agent
        self.transcript = transcript
        self.seq = 0

    def _send(self, message):
        line = json.dumps(message) + "\n"
        self.proc.stdin.write(line.encode())
        self.proc.stdin.flush()

    def _recv(self):
        line = self.proc.stdout.readline()
        if not line:
            raise RuntimeError("serve closed stdout mid-run")
        return json.loads(line.decode())

    def _log(self, entry):
        entry["t"] = rfc3339(utcnow())
        self.transcript.write(json.dumps(entry) + "\n")
        self.transcript.flush()

    def call(self, name, args):
        self.seq += 1
        request = {
            "jsonrpc": "2.0",
            "id": self.seq,
            "method": "tools/call",
            "params": {"name": name, "arguments": args},
        }
        self._send(request)
        response = self._recv()
        self._log({"method": "tools/call", "request": request, "response": response})
        result = response.get("result", {})
        texts = [b.get("text", "") for b in result.get("content", []) if b.get("type") == "text"]
        return ("\n".join(texts), result)

    def derive(self, content, concept_type, event_time):
        args = {
            "agent_id": self.agent,
            "concepts": [{"content": content, "concept_type": concept_type}],
        }
        if event_time is not None:
            args["event_time"] = event_time
        text, result = self.call("lambo_derive", args)
        receipt = (result.get("structuredContent") or {}).get("receipt", "")
        return receipt

    def record_action(self, action, depends_on=None, against=None, event_time=None):
        args = {"agent_id": self.agent, "action": action}
        if depends_on:
            args["depends_on"] = depends_on
        if against:
            args["against"] = against
        if event_time is not None:
            args["event_time"] = event_time
        text, result = self.call("lambo_record_action", args)
        receipt = (result.get("structuredContent") or {}).get("receipt", "")
        return receipt

    def wait(self, receipt):
        text, _ = self.call("lambo_stats", {"agent_id": self.agent, "receipt": receipt, "wait_ms": WAIT_MS})
        return text

    def wait_applied(self, receipt, what):
        text = self.wait(receipt)
        if "embedder unavailable" in text or "error sending request" in text:
            print(f"EMBEDDER_DOWN after {what}: {text.strip().splitlines()[-1]}")
            return None
        if "applied" not in text:
            raise RuntimeError(f"receipt for {what} never applied: {text[:500]}")
        return text

    def inspect(self, focus, depth=2):
        text, _ = self.call("lambo_inspect", {"agent_id": self.agent, "focus": focus, "depth": depth})
        return text

    def recall(self, query):
        text, _ = self.call("lambo_recall", {"agent_id": self.agent, "query": query})
        return text

    def stats(self):
        text, _ = self.call("lambo_stats", {"agent_id": self.agent})
        return text

    def wait_for(self, focus, want, timeout_secs=180, poll_secs=5):
        """Poll inspect until the focus text contains `want`.

        Promotion is eventual: the daemon evaluates on its own cadence, so
        the verdict right after the last receipt can predate the final
        flush. Returns the converged inspect text, or raises on timeout.
        """
        deadline = utcnow() + datetime.timedelta(seconds=timeout_secs)
        seen = ""
        while utcnow() < deadline:
            seen = self.inspect(focus)
            if want in seen:
                return seen
            time.sleep(poll_secs)
        raise RuntimeError(f"timed out waiting for {want!r} on {focus!r}: {seen[:500]}")


def check(name, condition, detail=""):
    print(("PASS " if condition else "FAIL ") + name + (f" -- {detail}" if detail and not condition else ""))
    if not condition:
        check.failed = True


check.failed = False


def run_probe(driver):
    receipt = driver.derive("T2.7b embedder probe concept", "observation", None)
    if not receipt:
        print("FAIL probe: no receipt issued")
        return 2
    text = driver.wait(receipt)
    if "embedder unavailable" in text or "error sending request" in text:
        print("PROBE_RESULT=down")
        return 2
    if "applied" in text:
        print("PROBE_RESULT=up")
        return 0
    print("PROBE_RESULT=unknown")
    return 2


def run_full(driver, anchor):
    def at(day):
        return rfc3339(anchor + datetime.timedelta(days=day))

    # The runs-small belief, then three same-direction returns, each its
    # own interaction citing the belief. Same shape as T2.4b.
    r = driver.derive(BELIEF, "logic", at(0))
    driver.wait_applied(r, "runs-small belief")
    for i, (content, day) in enumerate([(RET1, 3), (RET2, 6), (RET3, 9)], 1):
        r = driver.record_action(content, depends_on=[BELIEF], event_time=at(day))
        text = driver.wait_applied(r, f"same-direction return {i}")
        if text is None:
            raise RuntimeError("embedder went down mid-seeding")

    # Check 1: same-direction returns earn. Promotion is eventual, so poll
    # to convergence before asserting.
    seen = driver.wait_for(BELIEF, "[Logic, venerable]")
    check("belief inspects [Logic, venerable]", "[Logic, venerable]" in seen)
    check("blast radius is 3", "blast radius: 3" in seen)
    for ret in (RET1, RET2, RET3):
        check(f"dependent present: {ret[:40]}", ret in seen)
    stats = driver.stats()
    check("Swarm is the policy", "promotion_policy=Swarm" in stats)
    check("GC sweeps run from the file cadence", "trigger=mutations" in stats)

    # Check 2: the opposite-direction return attaches nowhere. It is its
    # own interaction with neither depends_on (no support) nor against (no
    # contradiction). This checks graph-level non-interference: the belief's
    # neighbourhood must be unchanged and the daemon must not declare a
    # contradiction on directly opposing wording. (The writer refusal and
    # the service's ledger-only unattached row are T2.7's offline tests;
    # this live leg bypasses both with direct MCP calls.)
    r = driver.record_action(RET4, event_time=at(10))
    text = driver.wait_applied(r, "opposite-direction return")
    if text is None:
        raise RuntimeError("embedder went down on the opposite return")
    sizing_after = driver.inspect(BELIEF)
    check("belief stays Venerable after the opposite return", "[Logic, venerable]" in sizing_after)
    check("blast radius still 3 after the opposite return", "blast radius: 3" in sizing_after)
    check("opposite return is not a dependent", RET4 not in sizing_after)
    check("no contradiction edge declared", "Contradicts" not in sizing_after)
    check("no contradiction warning", "ontradict" not in sizing_after.lower())
    opposite_seen = driver.inspect(RET4, depth=1)
    check("opposite return stands alone", RET4 in opposite_seen)
    check("opposite return declares nothing against", "Contradicts" not in opposite_seen)
    check("opposite return cites no belief", BELIEF not in opposite_seen)

    # Check 3: recall reflects the corrected behavior.
    recalled = driver.recall(RECALL_Q)
    check("recall lists the sizing belief", BELIEF in recalled)
    check("recall carries no contradiction warning", "Contradicted" not in recalled)
    recalled_opposite = driver.recall(RECALL_OPPOSITE_Q)
    check("opposite return is visible on its own query", RET4 in recalled_opposite)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--lambo", required=True)
    parser.add_argument("--config", required=True)
    parser.add_argument("--session", required=True)
    parser.add_argument("--agent", default="memento-t27b")
    parser.add_argument("--out", required=True)
    parser.add_argument("--probe", action="store_true")
    args = parser.parse_args()

    with open(args.out + "/transcript.jsonl", "a") as transcript:
        proc = subprocess.Popen(
            [args.lambo, "--config", args.config, "serve", "--session", args.session, "--agent", args.agent],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        )
        try:
            driver = Driver(proc, args.agent, transcript)
            driver.seq = 0
            init = {
                "jsonrpc": "2.0", "id": 1, "method": "initialize",
                "params": {"protocolVersion": "2024-11-05", "capabilities": {},
                           "clientInfo": {"name": "t27b-driver", "version": "0.1.0"}},
            }
            driver._send(init)
            resp = driver._recv()
            driver._log({"method": "initialize", "request": init, "response": resp})
            driver.seq = 1
            notif = {"jsonrpc": "2.0", "method": "notifications/initialized"}
            driver._send(notif)
            driver._log({"notification": notif})
            driver.seq = 2
            lst_req = {"jsonrpc": "2.0", "id": 2, "method": "tools/list"}
            driver._send(lst_req)
            lst_resp = driver._recv()
            driver._log({"method": "tools/list", "request": lst_req, "response": lst_resp})

            if args.probe:
                return run_probe(driver)

            anchor = utcnow() - datetime.timedelta(days=11)
            # Pin the anchor to the log so the transcript's event times read back.
            print(f"ANCHOR={rfc3339(anchor)}")
            run_full(driver, anchor)
            return 0 if not check.failed else 1
        finally:
            try:
                proc.stdin.close()
            except BrokenPipeError:
                pass
            proc.terminate()
            proc.wait(timeout=30)


if __name__ == "__main__":
    sys.exit(main())
