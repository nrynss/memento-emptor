#!/usr/bin/env bash
# T9.7 reset: put the demo shopper back at the starting state of the pivot run.
#
# It never deletes the seeded pool or the seller's other lines. What it does:
#
#   1. records the spend baseline and opens a fresh run directory;
#   2. reads the demo line's pool and, when an earlier run left a live settled
#      row, opts the demo shopper out of pooling (which removes their own live
#      fit rows) and back in;
#   3. asserts the starting evidence: the pool is Earned small with the seeded
#      counts, the shopper's sizing belief carries the three seeded returns,
#      and the latest sandbox capture and dispute are in their memory.
#
# What it cannot do: delete a correction. Corrections are append-only and live
# in the layer's standing journal, so a run records a new correction and the
# newest one wins. The reset records which correction is in force at the start
# so the run's own correction can be told apart from it.
#
# Keys come from the system keyring, are read in this process, and are never
# printed or written. Live reads are free; the agent turns are the run's spend.
#
# Usage: bash demo/reset.sh <run-number>
set -euo pipefail

RUN="${1:?usage: bash demo/reset.sh <run-number>}"
cd "$(dirname "$0")/.."

OUT="${OUT:-demo/out}"
mkdir -p "$OUT"

source tools/env.sh >/dev/null 2>&1 || true

LAYER="${MEMENTO_LAYER_URL:-https://memento-emptor.onrender.com}"
LAYER="${LAYER%/}"
AGENT_TOKEN="$(secret-tool lookup service memento-tokens account memento-emptor kind agent)"
SHOPPER_TOKEN="$(secret-tool lookup service memento-tokens account memento-emptor kind shopper)"
export LAYER AGENT_TOKEN SHOPPER_TOKEN

finish() {
	unset AGENT_TOKEN SHOPPER_TOKEN
}
trap finish EXIT

call() {
	local tool="$1" token="$2" body="$3"
	curl -sS -m 120 -X POST "$LAYER/tools/$tool" \
		-H "Authorization: Bearer $token" \
		-H 'Content-Type: application/json' \
		-d "$body"
}

say() { echo "[reset] $*"; }

say "layer: $LAYER"
health="$(curl -sS -m 90 "$LAYER/healthz" || true)"
case "$health" in
*'"ok":true'*) say "health: ok" ;;
*)
	echo "reset: the layer is not healthy: $health" >&2
	exit 2
	;;
esac

LINE='{"brand_id":"brand-x-001","category":"shirts","fit_handle":"regular","size":"M"}'
rm -f "$OUT/pool-before.json" "$OUT/pool-after.json"
fit="$(call product_fit "$AGENT_TOKEN" "$LINE")"
printf '%s' "$fit" >"$OUT/pool-before.json"

live_settled="$(printf '%s' "$fit" | python3 -c 'import json,sys; d=json.load(sys.stdin); d=d.get("pool",d); print(((d.get("live") or {}).get("settled") or 0))')"
if [ "$live_settled" -gt 0 ]; then
	say "an earlier run left $live_settled live settled row(s); opting the demo shopper out and back in"
	off="$(call set_pooling "$AGENT_TOKEN" '{"shopper_id":"shopper_01","pooling":false}')"
	printf '%s' "$off" >"$OUT/pooling-off.json"
	on="$(call set_pooling "$AGENT_TOKEN" '{"shopper_id":"shopper_01","pooling":true}')"
	printf '%s' "$on" >"$OUT/pooling-on.json"
	say "rows removed: $(printf '%s' "$off" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("rows_removed"))')"
	fit="$(call product_fit "$AGENT_TOKEN" "$LINE")"
	printf '%s' "$fit" >"$OUT/pool-after.json"
fi

python3 - "$OUT" <<'PY'
import json, sys
out = sys.argv[1]
fit = json.load(open(out + "/pool-after.json")) if __import__("os").path.exists(out + "/pool-after.json") else json.load(open(out + "/pool-before.json"))
if "counts" not in fit and isinstance(fit.get("pool"), dict):
    fit = fit["pool"]
counts = fit.get("counts") or {}
seeded = fit.get("seeded") or {}
live = fit.get("live") or {}
ok = (fit.get("status") == "Earned" and fit.get("direction") == "small"
      and counts.get("settled") == 22 and seeded.get("settled") == 22
      and (live.get("settled") or 0) == 0)
print(f"[reset] demo line: {fit.get('status')} {fit.get('direction')} "
      f"settled={counts.get('settled')} seeded={seeded.get('settled')} live={live.get('settled')}")
if not ok:
    print("[reset] the demo line is not at its seeded starting counts; refusing to start", file=sys.stderr)
    sys.exit(3)
PY

RECALL='{"shopper_id":"shopper_01","brand_id":"brand-x-001","category":"shirts","fit_handle":"regular","size_considered":"M"}'
recall="$(call recall_for_purchase "$AGENT_TOKEN" "$RECALL")"
printf '%s' "$recall" >"$OUT/recall-baseline.json"
python3 - "$OUT" <<'PY'
import json, sys
out = sys.argv[1]
recall = json.load(open(out + "/recall-baseline.json"))
belief = next((b for b in recall.get("beliefs", []) if b["belief_id"] == "blf_8f2c41"), None)
if belief is None or len(belief.get("evidence", [])) != 3:
    print("[reset] the sizing belief does not carry its three seeded returns; refusing to start", file=sys.stderr)
    sys.exit(4)
correction = recall.get("overriding_correction") or {}
print(f"[reset] three seeded returns present; belief {belief['belief_id']} "
      f"{belief.get('status')}; correction in force: {correction.get('correction_id') or 'none'}")
PY

export PATH="$HOME/.ast/bin:$PATH"
python3 - "$OUT" <<'PY'
import json, re, subprocess, sys
out = sys.argv[1]
text = subprocess.run(["ast", "billing", "get"], capture_output=True, text=True, check=True).stdout
base = {}
for line in text.splitlines():
    if "Usage spend:" in line:
        base["usage_spend"] = float(re.search(r"\$([0-9.]+)", line).group(1))
    elif "Credit left:" in line:
        base["credit_left"] = float(re.search(r"\$([0-9.]+)", line).group(1))
json.dump(base, open(out + "/baseline.json", "w"), indent=2, sort_keys=True)
print(f"[reset] spend baseline: usage ${base.get('usage_spend')}, credit left ${base.get('credit_left')}")
PY

printf '%s' "$RUN" >"$OUT/RUN"
rm -rf "$OUT/run-$RUN"
mkdir -p "$OUT/run-$RUN"
cp "$OUT/recall-baseline.json" "$OUT/run-$RUN/recall-baseline.json"
[ -f "$OUT/pool-after.json" ] && cp "$OUT/pool-after.json" "$OUT/run-$RUN/pool-baseline.json"
[ -f "$OUT/pool-before.json" ] && [ ! -f "$OUT/pool-after.json" ] && cp "$OUT/pool-before.json" "$OUT/run-$RUN/pool-baseline.json"
say "run $RUN ready; seeded pool and the seller's other lines untouched"
