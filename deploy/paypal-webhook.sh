#!/usr/bin/env bash
# Registers the public PayPal webhook subscription for the demo: the
# three event types the T0.4 leg proved (capture completed, capture
# refunded, dispute created) delivered to the public URL pattern from
# the hosting brief. Creating and listing webhooks is free of charge;
# run this once the service answers its public URL (its first green
# deploy), never against a dark endpoint.
#
# Usage: PUBLIC_URL=https://<service>.onrender.com bash deploy/paypal-webhook.sh
# Needs PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET in the environment
# (tools/env.sh). Sandbox only, always.
set -euo pipefail

: "${PUBLIC_URL:?set PUBLIC_URL to the service's public URL}"
: "${PAYPAL_CLIENT_ID:?load tools/env.sh first}"
: "${PAYPAL_CLIENT_SECRET:?load tools/env.sh first}"

HOOK_URL="$PUBLIC_URL/webhooks/paypal"
TOKEN_URL="https://api.sandbox.paypal.com/v1/oauth2/token"
WEBHOOKS_URL="https://api.sandbox.paypal.com/v1/notifications/webhooks"

token="$(curl -fsSL -u "$PAYPAL_CLIENT_ID:$PAYPAL_CLIENT_SECRET" \
	-d grant_type=client_credentials "$TOKEN_URL" |
	python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])')"

list="$(curl -fsSL -H "Authorization: Bearer $token" "$WEBHOOKS_URL")"
existing="$(printf '%s' "$list" | python3 -c '
import json, sys
for hook in json.load(sys.stdin).get("webhooks", []):
    print(hook.get("id", ""), hook.get("url", ""))
')"

if printf '%s\n' "$existing" | grep -qF "$HOOK_URL"; then
	echo "webhook: $HOOK_URL already subscribed:"
	printf '%s\n' "$existing" | grep -F "$HOOK_URL"
	exit 0
fi

curl -fsSL -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
	-d "$(python3 -c 'import json; print(json.dumps({
		"url": "'"$HOOK_URL"'",
		"event_types": [{"name": n} for n in [
			"PAYMENT.CAPTURE.COMPLETED",
			"PAYMENT.CAPTURE.REFUNDED",
			"CUSTOMER.DISPUTE.CREATED"]]}))')" \
	"$WEBHOOKS_URL" | python3 -m json.tool
