#!/usr/bin/env bash
# Registers the public PayPal webhook subscription for the demo: the three
# event types the layer stores (capture completed, capture refunded, dispute
# created) delivered to the public URL pattern from the hosting brief.
#
# Idempotent on both the URL and the event list: a subscription already at
# the URL gets any missing event types added, and one that already carries
# all three is left alone. Creating, listing and patching webhooks is free of
# charge; run this once the service answers its public URL (its first green
# deploy), never against a dark endpoint.
#
# Usage: PUBLIC_URL=https://<service>.onrender.com bash deploy/paypal-webhook.sh
# Needs PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET in the environment
# (tools/env.sh). Sandbox only, always.
set -euo pipefail

: "${PUBLIC_URL:?set PUBLIC_URL to the service public URL}"
: "${PAYPAL_CLIENT_ID:?load tools/env.sh first}"
: "${PAYPAL_CLIENT_SECRET:?load tools/env.sh first}"

HOOK_URL="${PUBLIC_URL%/}/webhooks/paypal"
API="https://api-m.sandbox.paypal.com"
TOKEN_URL="$API/v1/oauth2/token"
WEBHOOKS_URL="$API/v1/notifications/webhooks"
WANTED="CUSTOMER.DISPUTE.CREATED PAYMENT.CAPTURE.COMPLETED PAYMENT.CAPTURE.REFUNDED"

token="$(curl -fsSL -u "$PAYPAL_CLIENT_ID:$PAYPAL_CLIENT_SECRET" \
	-d grant_type=client_credentials "$TOKEN_URL" |
	python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])')"

# match is one line for the subscription at HOOK_URL: its id, a tab, and the
# wanted event types it is missing, comma-separated. Empty output means no
# subscription serves HOOK_URL yet.
match="$(curl -fsSL -H "Authorization: Bearer $token" "$WEBHOOKS_URL" |
	python3 -c '
import json, sys
url, wanted = sys.argv[1], sys.argv[2].split()
for hook in json.load(sys.stdin).get("webhooks", []):
    if hook.get("url") != url:
        continue
    have = [e.get("name") for e in hook.get("event_types", []) if e.get("name")]
    missing = [name for name in wanted if name not in have]
    print(hook.get("id", ""), ",".join(missing), sep="\t")
    break
' "$HOOK_URL" "$WANTED")"

if [ -n "$match" ]; then
	subscription="${match%%$'\t'*}"
	missing="${match#*$'\t'}"
	if [ -z "$missing" ]; then
		echo "webhook: $HOOK_URL already subscribed with the three event types"
		echo "subscription: $subscription"
		exit 0
	fi
	echo "webhook: $HOOK_URL already subscribed; adding event types: $missing"
	patch="$(curl -fsSL -H "Authorization: Bearer $token" "$WEBHOOKS_URL/$subscription" |
		python3 -c '
import json, sys
missing = sys.argv[1].split(",")
hook = json.load(sys.stdin)
events = [ e.get("name") for e in hook.get("event_types", []) if e.get("name") ]
for name in missing:
    if name and name not in events:
        events.append(name)
print(json.dumps([{"op": "replace", "path": "/event_types", "value": [{"name": name} for name in events]}]))
' "$missing")"
	curl -fsSL -X PATCH -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
		-d "$patch" "$WEBHOOKS_URL/$subscription" |
		python3 -c 'import json,sys; d=json.load(sys.stdin); print("webhook:", d.get("url")); print("subscription:", d.get("id")); print("events:", ", ".join(e.get("name","") for e in d.get("event_types",[])))'
	exit 0
fi

echo "webhook: creating $HOOK_URL"
curl -fsSL -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
	-d "$(python3 -c '
import json, sys
print(json.dumps({"url": sys.argv[1], "event_types": [{"name": name} for name in sys.argv[2].split()]}))
' "$HOOK_URL" "$WANTED")" \
	"$WEBHOOKS_URL" |
	python3 -c 'import json,sys; d=json.load(sys.stdin); print("webhook:", d.get("url")); print("subscription:", d.get("id")); print("events:", ", ".join(e.get("name","") for e in d.get("event_types",[])))'
