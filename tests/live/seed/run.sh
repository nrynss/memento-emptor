#!/usr/bin/env bash
# One T1.3b cycle: create a wallet order, wait for the scripted shopper
# approval, capture it, optionally refund it, and read everything back.
# Credentials never enter this file: PAYPAL_CLIENT_ID/SECRET come from
# `source tools/env.sh`, and the approval itself runs in approve.py.
#
# Usage:
#   CUSTOM_ID=t13b-demo-003 SKU=BRX-OXF-WHT NAME="Brand X Button Down Oxford Shirt size M" \
#     AMOUNT=195.00 REFUND_NOTE="Fit return 1: too tight in the shoulders" \
#     bash tests/live/seed/run.sh
# Omit REFUND_NOTE for a kept/dispute cycle (capture only).
set -euo pipefail

: "${CUSTOM_ID:?set CUSTOM_ID, e.g. t13b-demo-003}"
: "${SKU:?set SKU}"
: "${NAME:?set NAME}"
: "${AMOUNT:?set AMOUNT, e.g. 195.00}"

OUT="${OUT:-tests/live/seed/out}"
mkdir -p "$OUT"
BASE=https://api-m.sandbox.paypal.com

token() {
	curl -s -u "$PAYPAL_CLIENT_ID:$PAYPAL_CLIENT_SECRET" \
		-d grant_type=client_credentials "$BASE/v1/oauth2/token" |
		python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])"
}

TKN="$(token)"
REQ_ID="$(python3 -c 'import uuid; print(uuid.uuid4())')"

# 1. Create the order (idempotent on PayPal-Request-Id).
python3 - "$OUT/$CUSTOM_ID-create.json" <<EOF
import json, subprocess
body = {
    "intent": "CAPTURE",
    "purchase_units": [{
        "reference_id": "$CUSTOM_ID",
        "custom_id": "$CUSTOM_ID",
        "description": "$NAME",
        "amount": {"currency_code": "USD", "value": "$AMOUNT",
                   "breakdown": {"item_total": {"currency_code": "USD", "value": "$AMOUNT"}}},
        "items": [{
            "name": "$NAME",
            "sku": "$SKU",
            "quantity": "1",
            "unit_amount": {"currency_code": "USD", "value": "$AMOUNT"},
        }],
    }],
    "payment_source": {"paypal": {
        "return_url": "https://example.com/return",
        "cancel_url": "https://example.com/cancel",
    }},
}
out = "$OUT/$CUSTOM_ID-create.json"
_ = out
req = subprocess.run(
    ["curl", "-s", "-X", "POST", "$BASE/v2/checkout/orders",
     "-H", "Content-Type: application/json",
     "-H", "Authorization: Bearer $TKN",
     "-H", "PayPal-Request-Id: $REQ_ID",
     "-d", json.dumps(body)],
    capture_output=True, text=True, check=True)
open(out, "w").write(req.stdout + "\n")
d = json.loads(req.stdout)
links = {l.get("rel"): l.get("href") for l in d.get("links", [])}
print("order:", d.get("id"), "status:", d.get("status"))
print("approve:", links.get("payer-action", "MISSING"))
EOF

# 2. Wait for the scripted approval (run approve.py with APPROVAL_URL
# from the line above, then this loop sees the status flip).
ORDER_ID="$(python3 -c "import json; print(json.load(open('$OUT/$CUSTOM_ID-create.json'))['id'])")"
echo "order $ORDER_ID waiting for approval..."
for _ in $(seq 1 40); do
	STATUS="$(curl -s -H "Authorization: Bearer $TKN" "$BASE/v2/checkout/orders/$ORDER_ID" |
		python3 -c "import sys,json; print(json.load(sys.stdin).get('status','UNKNOWN'))")"
	echo "  status: $STATUS"
	if [ "$STATUS" = "APPROVED" ]; then break; fi
	if [ "$STATUS" != "PAYER_ACTION_REQUIRED" ]; then
		echo "unexpected status $STATUS" >&2; exit 1
	fi
	sleep 15
done

# 3. Authoritative order read-back (line items live here).
curl -s -H "Authorization: Bearer $TKN" "$BASE/v2/checkout/orders/$ORDER_ID" |
	python3 -m json.tool | tee "$OUT/$CUSTOM_ID-order-get.json" >/dev/null
python3 -c "
import json
d = json.load(open('$OUT/$CUSTOM_ID-order-get.json'))
for pu in d.get('purchase_units', []):
    print('custom_id:', pu.get('custom_id'))
    for it in pu.get('items', []):
        print('item:', it.get('sku'), '|', it.get('name'), '| qty', it.get('quantity'))
"

# 4. Capture.
CAP_REQ_ID="$(python3 -c 'import uuid; print(uuid.uuid4())')"
curl -s -X POST -H "Authorization: Bearer $TKN" \
	-H "Content-Type: application/json" \
	-H "PayPal-Request-Id: $CAP_REQ_ID" \
	-d '{}' "$BASE/v2/checkout/orders/$ORDER_ID/capture" |
	python3 -m json.tool | tee "$OUT/$CUSTOM_ID-capture.json"
CAPTURE_ID="$(python3 -c "
import json
d = json.load(open('$OUT/$CUSTOM_ID-capture.json'))
print(d['purchase_units'][0]['payments']['captures'][0]['id'])
")"
echo "capture $CAPTURE_ID"

# 5. Optional full refund with the reason named in words.
if [ -n "${REFUND_NOTE:-}" ]; then
	REF_REQ_ID="$(python3 -c 'import uuid; print(uuid.uuid4())')"
	python3 -c "import json; print(json.dumps({'amount': {'currency_code': 'USD', 'value': '$AMOUNT'}, 'note_to_payer': '$REFUND_NOTE'}))" |
		curl -s -X POST -H "Authorization: Bearer $TKN" \
			-H "Content-Type: application/json" \
			-H "PayPal-Request-Id: $REF_REQ_ID" \
			-d @- "$BASE/v2/payments/captures/$CAPTURE_ID/refund" |
		python3 -m json.tool | tee "$OUT/$CUSTOM_ID-refund.json"
fi

echo "done: $CUSTOM_ID order=$ORDER_ID capture=$CAPTURE_ID"
