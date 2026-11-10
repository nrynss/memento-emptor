#!/usr/bin/env bash
# Creates the settled Render shell (service + disk) from the API and
# verifies it matches research/hosting.md: plan starter, 1 GB disk at
# /var/lambo-data, region virginia, docker runtime. Values come from
# the environment (tools/env.sh) and are never printed; only ids,
# names and the public URL are echoed.
#
# Render builds the branch it is pointed at, so run this after the
# deploy files land on main, then watch the first deploy go green.
set -euo pipefail

: "${RENDER_API_KEY:?load tools/env.sh first}"
: "${PAYPAL_CLIENT_ID:?load tools/env.sh first}"
: "${PAYPAL_CLIENT_SECRET:?load tools/env.sh first}"
: "${LAMBO_ES_API_KEY:?load tools/env.sh first}"
: "${ELASTIC_URL:?load tools/env.sh first}"
: "${OPENROUTER_API_KEY:?load tools/env.sh first}"

SERVICE_NAME="${SERVICE_NAME:-memento-emptor}"
REPO="${REPO:-https://github.com/nrynss/memento-emptor}"
BRANCH="${BRANCH:-main}"
API=https://api.render.com/v1

owner="$(curl -fsSL -H "Authorization: Bearer $RENDER_API_KEY" "$API/owners" |
	python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["owner"]["id"])')"

payload="$(python3 - "$owner" <<'EOF'
import json, os, sys
owner = sys.argv[1]
env = lambda k: os.environ[k]
print(json.dumps({
    "type": "web_service",
    "name": os.environ.get("SERVICE_NAME", "memento-emptor"),
    "ownerId": owner,
    "repo": os.environ.get("REPO", "https://github.com/nrynss/memento-emptor"),
    "branch": os.environ.get("BRANCH", "main"),
    "autoDeploy": True,
    "serviceDetails": {
        "runtime": "docker",
        "plan": "starter",
        "region": "virginia",
        "dockerfilePath": "./deploy/Dockerfile",
        "dockerContext": ".",
        "healthCheckPath": "/healthz",
        "disk": {"name": "lambo-data", "mountPath": "/var/lambo-data", "sizeGB": 1},
        "envVars": [
            {"key": "PAYPAL_CLIENT_ID", "value": env("PAYPAL_CLIENT_ID")},
            {"key": "PAYPAL_CLIENT_SECRET", "value": env("PAYPAL_CLIENT_SECRET")},
            {"key": "LAMBO_ES_API_KEY", "value": env("LAMBO_ES_API_KEY")},
            {"key": "ELASTIC_URL", "value": env("ELASTIC_URL")},
            {"key": "OPENROUTER_API_KEY", "value": env("OPENROUTER_API_KEY")},
        ],
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
details = svc.get("serviceDetails", {})
print("service:", svc.get("id"), svc.get("name"), svc.get("branch"))
print("url:", svc.get("serviceDetails", {}).get("url", svc.get("slug", "")))
print("plan:", details.get("plan"), "| region:", details.get("region"), "| runtime:", details.get("runtime"))
print("disk:", json.dumps(details.get("disk")))
assert details.get("plan") == "starter", "plan drifted from the settled shape"
assert (details.get("disk") or {}).get("sizeGB") == 1, "disk drifted from the settled shape"
assert (details.get("disk") or {}).get("mountPath") == "/var/lambo-data", "mount drifted"
print("provision: settled shape holds")
EOF
