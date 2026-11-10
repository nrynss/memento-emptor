#!/usr/bin/env bash
# T8.1b live run: the Lambo hub against the live Elastic recall tier.
#
# The driver starts one hub in the T8.1 shape (one pinned demo session,
# max_attached=16, the shopper- on-demand credential, --max-sessions 128
# --rate-limit-rps 200), creates three shoppers on demand, shows each
# recalls only its own concepts, serves them through the ledger reader's
# allowlist, records the 503 + Retry-After refusal with every place full,
# and measures the two flags. It deletes every test index afterwards.
# This script adds the cleanup check: the prefix listing must be empty
# afterwards, and no key or deployment location may have entered a file.
#
# Usage, from the repository root:
#   source tools/env.sh
#   LAMBO_BIN=/path/to/lambo-0.10.1 bash tests/live/hub/run.sh
#
# LAMBO_BIN must be the 0.10.1 release binary (the deploy pins its sha).
# Nothing here runs in CI and nothing here gates a build: there are no
# *_test.go files under tests/live/hub/, so the gate builds the driver
# but never executes it. Elastic spend is test-prefixed index churn only;
# embedding spend is $0.00 (the fixture embedder computes vectors in
# process) against the $0.20 budget.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE/../../.."

: "${ELASTIC_URL:?source tools/env.sh first}"
: "${LAMBO_ES_API_KEY:?source tools/env.sh first}"
: "${LAMBO_BIN:?set LAMBO_BIN to the 0.10.1 release binary first}"

"$LAMBO_BIN" --version

OUT="${OUT:-$HERE/out}"
mkdir -p "$OUT"

go run ./tests/live/hub | tee "$OUT/transcript.txt"

echo "--- cleanup check: the t81b- prefix is empty ---"
curl -s -m 60 -H "Authorization: ApiKey $LAMBO_ES_API_KEY" \
	"$ELASTIC_URL/_cat/indices/t81b-*?format=json&h=index" | tee "$OUT/indices-after.json"
echo
python3 - "$OUT/indices-after.json" <<'PY'
import json, sys

with open(sys.argv[1]) as handle:
    names = [row["index"] for row in json.load(handle)]
if names:
    print(f"FAIL: indices remain under t81b-: {names}", file=sys.stderr)
    sys.exit(1)
print("prefix t81b- is empty")
PY

echo "--- leak check: no key or deployment location entered tests/live/hub ---"
leaks=""
for value in "$ELASTIC_URL" "$LAMBO_ES_API_KEY"; do
	[ -n "$value" ] || continue
	found=$(grep -rlF --exclude-dir=out -- "$value" "$HERE" || true)
	[ -z "$found" ] || leaks="${leaks}${found}"$'\n'
done
if [ -n "$leaks" ]; then
	echo "secret value entered a file:" >&2
	printf '%s' "$leaks" >&2
	exit 1
fi

# The full transcript stays out of git.
git check-ignore -q "$HERE/out"

echo "done: T8.1b live checks pass; indices deleted; transcript in $OUT (git-ignored)"
