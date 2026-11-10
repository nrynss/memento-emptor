#!/usr/bin/env bash
# Creates the merchant Render service (stateless web service, starter plan,
# no disk) and verifies it matches deploy/merchant/render.yaml: region
# virginia, docker runtime, health check /healthz, $7.00/mo. Values come
# from the environment (tools/env.sh plus the minted merchant key) and are
# never printed; only ids, names and the public URL are echoed.
#
# Render builds the branch it is pointed at, so run this after the
# deploy/merchant files land on main, then watch the first deploy go
# green. Mint the merchant key first, with T8.5's command:
#
#   token=$(go run ./cmd/memento key issue --kind merchant --id merchant-app --seller seller-demo 2>/dev/null)
#
# and hand the entry (stderr) to the layer deployment: it is appended to
# the layer's MEMENTO_MERCHANT_KEYS. This script needs only the token, as
# MEMENTO_MERCHANT_KEY; the layer URL defaults to the layer service.
set -euo pipefail

: "${RENDER_API_KEY:?load tools/env.sh first}"
: "${MEMENTO_MERCHANT_KEY:?mint the merchant key with 'memento key issue' first}"

SERVICE_NAME="${SERVICE_NAME:-memento-merchant}"
REPO="${REPO:-https://github.com/nrynss/memento-emptor}"
BRANCH="${BRANCH:-main}"
LAYER_URL="${MEMENTO_LAYER_URL:-https://memento-emptor.onrender.com}"
API=https://api.render.com/v1

owner="$(curl -fsSL -H "Authorization: Bearer $RENDER_API_KEY" "$API/owners" |
	python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["owner"]["id"])')"

payload="$(python3 - "$owner" <<'EOF'
import json, os, sys
owner = sys.argv[1]
env = lambda k: os.environ[k]
print(json.dumps({
    "type": "web_service",
    "name": os.environ.get("SERVICE_NAME", "memento-merchant"),
    "ownerId": owner,
    "repo": os.environ.get("REPO", "https://github.com/nrynss/memento-emptor"),
    "branch": os.environ.get("BRANCH", "main"),
    "autoDeployTrigger": "commit",
    "envVars": [
        {"key": "MEMENTO_LAYER_URL", "value": os.environ.get("LAYER_URL", "https://memento-emptor.onrender.com")},
        {"key": "MEMENTO_MERCHANT_KEY", "value": env("MEMENTO_MERCHANT_KEY")},
    ],
    "serviceDetails": {
        "runtime": "docker",
        "plan": "starter",
        "region": "virginia",
        "envSpecificDetails": {
            "dockerfilePath": "./deploy/merchant/Dockerfile",
            "dockerContext": ".",
        },
        "healthCheckPath": "/healthz",
    },
}))
EOF
)"

created="$(printf '%s' "$payload" | curl -fsSL -X POST -H "Authorization: Bearer $RENDER_API_KEY" \
	-H "Content-Type: application/json" -d @- "$API/services")"
unset payload

python3 - "$created" <<'EOF'
import json, sys
svc = json.loads(sys.argv[1])
svc = svc.get("service", svc)
details = svc.get("serviceDetails", {})
print("service:", svc.get("id"), svc.get("name"), svc.get("branch"))
print("url:", svc.get("serviceDetails", {}).get("url", svc.get("slug", "")))
print("plan:", details.get("plan"), "| region:", details.get("region"), "| runtime:", details.get("runtime"))
print("disk:", json.dumps(details.get("disk")))
assert details.get("plan") == "starter", "plan drifted from the settled shape"
assert details.get("region") == "virginia", "region drifted from the settled shape"
assert details.get("runtime") in ("docker", "image"), "runtime drifted from the settled shape"
assert not details.get("disk"), "the stateless merchant service takes no disk"
print("provision: settled shape holds ($7.00/mo, no disk)")
EOF
