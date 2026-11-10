#!/usr/bin/env bash
# Spend guard for the merchant service, following deploy/guard.sh: Render
# has no native spend cap, and billing is fixed-rate (plan + disk, prorated
# to the second), so the guard audits the billable inventory through the
# API and stops what exceeds the ceiling:
#
#   guard.sh --check            live audit; exit 0 within ceiling, 1 on alert
#   guard.sh --check --stop     live audit; suspends drifted services, exit 2 when it stopped one
#   guard.sh --snapshot FILE [--stop]
#                               same audit against a recorded services/disks
#                               listing; with --stop it only prints the
#                               suspend it would issue (dry-run trip)
#
# The settled shape is both demo services: memento (starter + the 1 GB
# Lambo disk, $7.25/mo) and memento-merchant (starter, no disk, $7.00/mo),
# for a combined run-rate of $14.25/mo against a $15.00/mo ceiling. Rates
# pinned from the pricing page read in research/hosting.md; an unknown
# plan or disk size alerts (exit 1) and never auto-stops, so an API change
# cannot kill the demo by itself. Needs RENDER_API_KEY in the environment
# for live checks; snapshots need no key.
#
# Note: deploy/guard.sh (T9.6) still ceilings the single-service shape;
# it trips (exit 2, nothing suspended) once this service exists, until its
# ceiling is raised to the combined shape. This guard is the green one for
# the pair.
set -euo pipefail

API=https://api.render.com/v1
MAX_MONTHLY="${MAX_MONTHLY:-15.00}"
PLAN_STARTER=7.00
DISK_PER_GB=0.25

mode=check
stop=0
snapshot=""

while [ $# -gt 0 ]; do
	case "$1" in
	--check) mode=check; shift ;;
	--stop) stop=1; shift ;;
	--snapshot)
		mode=snapshot
		snapshot="${2:?--snapshot needs a file}"
		shift 2
		;;
	*) echo "guard: unknown flag $1" >&2 && exit 1 ;;
	esac
done

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

if [ "$mode" = snapshot ]; then
	cp "$snapshot" "$work/state.json"
else
	: "${RENDER_API_KEY:?live --check needs RENDER_API_KEY in the environment}"
	owner="$(curl -fsSL -H "Authorization: Bearer $RENDER_API_KEY" "$API/owners" | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["owner"]["id"])')"
	curl -fsSL -H "Authorization: Bearer $RENDER_API_KEY" "$API/services?limit=100" -o "$work/services.json"
	if curl -fsSL -H "Authorization: Bearer $RENDER_API_KEY" "$API/disks?ownerId=$owner" -o "$work/disks.json" 2>/dev/null; then
		:
	else
		echo '{"disks": "unlisted"}' >"$work/disks.json"
	fi
	python3 - "$work/services.json" "$work/disks.json" "$work/state.json" <<'EOF'
import json, sys
services = json.load(open(sys.argv[1]))
disks = json.load(open(sys.argv[2]))
json.dump({"services": services, "disks": disks}, open(sys.argv[3], "w"))
EOF
fi

python3 - "$work/state.json" "$MAX_MONTHLY" "$PLAN_STARTER" "$DISK_PER_GB" <<'EOF'
import json, sys

_, state_path, max_monthly, starter, disk_gb = sys.argv
max_monthly, starter, disk_gb = float(max_monthly), float(starter), float(disk_gb)
state = json.load(open(state_path))
services = state["services"] if isinstance(state.get("services"), list) else state.get("services", {}).get("items", state.get("services", []))
disks_node = state.get("disks", [])
disks = disks_node if isinstance(disks_node, list) else disks_node.get("items", [])

# The settled pair: the main demo service carries the 1 GB Lambo disk,
# the merchant service is stateless.
expected = {"memento": 1, "memento-merchant": 0}

rates = {"free": 0.0, "starter": starter, "standard": 25.0}
run_rate, unknown, drifted, notes = 0.0, [], [], []
for entry in services:
    svc = entry.get("service", entry) if isinstance(entry, dict) else entry
    sid = svc.get("id", "?")
    name = svc.get("name", "?")
    details = svc.get("serviceDetails", {})
    plan = details.get("plan", "?")
    if plan in rates:
        run_rate += rates[plan]
    else:
        unknown.append(f"service {sid} plan {plan!r}")
    if svc.get("type") != "web_service" or plan != "starter":
        drifted.append(sid)
    elif name not in expected:
        notes.append(f"service {sid} has an unexpected name {name!r} (still settled-rate)")
    elif name == "memento-merchant" and details.get("disk"):
        notes.append("service memento-merchant carries a disk (the merchant service is stateless; detach it)")
    elif name == "memento":
        disk = details.get("disk") or {}
        if disk.get("sizeGB") != 1 or disk.get("mountPath") != "/var/lambo-data":
            notes.append(f"service memento disk drifted: {json.dumps(disk)}")

disk_gb_total = 0
for entry in disks:
    disk = entry.get("disk", entry) if isinstance(entry, dict) else entry
    if not isinstance(disk, dict) or "sizeGB" not in disk:
        continue
    try:
        size = int(disk["sizeGB"])
    except (TypeError, ValueError):
        unknown.append(f"disk {disk.get('id', '?')} size {disk.get('sizeGB')!r}")
        continue
    disk_gb_total += size
    run_rate += size * disk_gb
    if size > 1:
        notes.append(f"disk {disk.get('id', '?')} is {size} GB (over the 1 GB shape; shrink is an operator step)")

print(f"guard: {len(services)} service(s), {disk_gb_total} GB disk, run-rate ${run_rate:.2f}/mo against ${max_monthly:.2f}/mo ceiling")
for line in unknown:
    print(f"guard: UNKNOWN (alert, no stop): {line}")
for sid in drifted:
    print(f"guard: DRIFT: {sid}")
for line in notes:
    print(f"guard: NOTE: {line}")

missing = [name for name in expected if not any(
    (e.get("service", e) if isinstance(e, dict) else e).get("name") == name for e in services)]
for name in missing:
    print(f"guard: NOTE: expected service {name!r} is absent (not yet provisioned, or removed)")

stop = run_rate > max_monthly or bool(drifted)
if unknown:
    print("guard: ALERT: unpriced inventory needs an operator decision; nothing stopped")
    sys.exit(1)
if stop:
    print("guard: STOP: over the ceiling")
    for sid in drifted:
        print(f"guard: action: POST /v1/services/{sid}/suspend")
    sys.exit(2)
print("guard: OK: within the settled shape")
EOF
audit=$?

if [ $audit -eq 2 ] && [ $stop -eq 1 ] && [ "$mode" = check ]; then
	: "${RENDER_API_KEY:?--stop needs RENDER_API_KEY in the environment}"
	python3 - "$work/state.json" "$API" <<'EOF'
import json, os, sys, urllib.request
state = json.load(open(sys.argv[1]))
api, key = sys.argv[2], os.environ["RENDER_API_KEY"]
services = state["services"] if isinstance(state.get("services"), list) else []
for entry in services:
    svc = entry.get("service", entry) if isinstance(entry, dict) else entry
    details = svc.get("serviceDetails", {})
    if svc.get("type") != "web_service" or details.get("plan") != "starter":
        req = urllib.request.Request(f"{api}/services/{svc['id']}/suspend", method="POST",
                                      headers={"Authorization": f"Bearer {key}"})
        urllib.request.urlopen(req)
        print(f"guard: suspended {svc['id']}")
EOF
fi
exit "$audit"
