"""Probe: can the sandbox produce a not-as-described dispute through the
customer-disputes API?

T1.1b mapped MERCHANDISE_OR_SERVICE_NOT_AS_DESCRIBED to a reason
category, but no recorded sandbox dispute carries the code, so the
mapping's spelling is documented, not observed. This probe asks the API
for such a case on an existing captured, unrefunded, undisputed
transaction and records the refusal. It creates nothing: the create call
is refused, and every other call is a read.

It prints no credential and no id: transaction ids appear as their last
six characters, and the response's debug id is dropped. The recording in
testdata/paypal/ keeps that key with a placeholder in its place.
Read the token from the environment (source tools/env.sh).

    source tools/env.sh
    python3 tests/live/ingest/probe_not_as_described.py
"""
import base64
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timedelta, timezone

BASE = "https://api-m.sandbox.paypal.com"
# Reporting event codes: a capture, a refund, and a dispute's hold.
CAPTURE, REFUND, DISPUTE_HOLD = "T0006", "T1107", "T1110"


def token():
    """Mint a sandbox access token from the environment."""
    cid, secret = os.environ["PAYPAL_CLIENT_ID"], os.environ["PAYPAL_CLIENT_SECRET"]
    basic = base64.b64encode(f"{cid}:{secret}".encode()).decode()
    req = urllib.request.Request(
        BASE + "/v1/oauth2/token",
        data=b"grant_type=client_credentials",
        headers={
            "Authorization": "Basic " + basic,
            "Content-Type": "application/x-www-form-urlencoded",
        },
    )
    with urllib.request.urlopen(req) as resp:
        return json.load(resp)["access_token"]


def call(tok, method, path, body=None):
    """Perform one authenticated sandbox call and return status and JSON."""
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        BASE + path,
        data=data,
        method=method,
        headers={"Authorization": "Bearer " + tok, "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req) as resp:
            return resp.status, json.load(resp)
    except urllib.error.HTTPError as err:
        return err.code, json.loads(err.read() or b"{}")


def free_captures(tok):
    """List captured transactions that no refund or dispute has claimed."""
    end = datetime.now(timezone.utc)
    start = end - timedelta(days=20)
    query = urllib.parse.urlencode(
        {
            "start_date": start.strftime("%Y-%m-%dT%H:%M:%SZ"),
            "end_date": end.strftime("%Y-%m-%dT%H:%M:%SZ"),
            "fields": "transaction_info",
            "page_size": "100",
        }
    )
    status, got = call(tok, "GET", "/v1/reporting/transactions?" + query)
    if status != 200:
        raise SystemExit(f"transactions list: status {status}")
    captures, claimed = [], set()
    for detail in got.get("transaction_details", []):
        info = detail.get("transaction_info", {})
        code = info.get("transaction_event_code")
        if code in (REFUND, DISPUTE_HOLD):
            claimed.add(info.get("paypal_reference_id"))
        if code == CAPTURE:
            captures.append(
                (
                    info.get("transaction_initiation_date"),
                    info.get("transaction_id"),
                    info.get("transaction_subject"),
                )
            )
    return sorted(c for c in captures if c[1] not in claimed)


def main():
    tok = token()
    free = free_captures(tok)
    print(f"captures open to a dispute: {len(free)}")
    for index, (when, tid, subject) in enumerate(free):
        print(f"  {index}  {when}  ...{tid[-6:]}  {(subject or '')[:40]}")
    if not free:
        raise SystemExit("no captured, unrefunded, undisputed transaction to probe")
    # The demo shopper's kept purchases are in this list too; a case on
    # one of those would put a dispute on demo history if the API ever
    # stopped refusing. Default to the newest free capture and let
    # PROBE_INDEX choose another from the list above.
    index = int(os.environ.get("PROBE_INDEX", str(len(free) - 1)))
    when, capture, subject = free[index]
    print(f"probing ...{capture[-6:]} captured {when}")
    # The item shape is what the API asks for: without it the request is
    # refused on schema before eligibility is ever looked at.
    body = {
        "reason": "MERCHANDISE_OR_SERVICE_NOT_AS_DESCRIBED",
        "disputed_transactions": [
            {
                "buyer_transaction_id": capture,
                "items": [
                    {
                        "item_id": os.environ.get("PROBE_SKU", "BRX-OXF-M"),
                        "item_name": subject or "item",
                        "item_quantity": "1",
                        "item_type": "PRODUCT",
                        "reason": "MERCHANDISE_OR_SERVICE_NOT_AS_DESCRIBED",
                    }
                ],
            }
        ],
    }
    status, payload = call(tok, "POST", "/v1/customer/disputes", body)
    payload.pop("debug_id", None)
    print(f"POST /v1/customer/disputes -> {status}")
    print(json.dumps(payload, indent=2))
    return 0 if status == 201 else 1


if __name__ == "__main__":
    sys.exit(main())
