#!/usr/bin/env bash
# T8.3b live run: the pool reader against the real Elastic deployment.
#
# The driver creates the test-prefixed index t83b-fit-rows, writes eight
# seeded rows through the fit writer, reads the pool back through the pool
# reader's product_fit path with warm and cold latency, shows an empty size
# as a valid empty pool, then deletes the index. This script adds the
# cleanup check: the prefix listing must be empty afterwards, and no key or
# deployment location may have entered a file.
#
# Usage, from the repository root:
#   source tools/env.sh
#   bash tests/live/pool/run.sh
#
# Nothing here runs in CI and nothing here gates a build: there are no
# *_test.go files under tests/live/pool/, so the gate builds the driver
# but never executes it. Elastic spend: none beyond the existing project;
# the index is kilobyte-scale and deleted.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE/../../.."

: "${ELASTIC_URL:?source tools/env.sh first}"
: "${LAMBO_ES_API_KEY:?source tools/env.sh first}"
: "${MEMENTO_IDENTITY_SALT:?source tools/env.sh first}"

OUT="${OUT:-$HERE/out}"
mkdir -p "$OUT"

go run ./tests/live/pool | tee "$OUT/transcript.txt"

echo "--- cleanup check: the t83b- prefix is empty ---"
curl -s -m 60 -H "Authorization: ApiKey $LAMBO_ES_API_KEY" \
	"$ELASTIC_URL/_cat/indices/t83b-*?format=json&h=index" | tee "$OUT/indices-after.json"
echo
python3 - "$OUT/indices-after.json" <<'PY'
import json, sys

with open(sys.argv[1]) as handle:
    names = [row["index"] for row in json.load(handle)]
if names:
    print(f"FAIL: indices remain under t83b-: {names}", file=sys.stderr)
    sys.exit(1)
print("prefix t83b- is empty")
PY

echo "--- leak check: no key or deployment location entered tests/live/pool ---"
leaks=""
for value in "$ELASTIC_URL" "$LAMBO_ES_API_KEY" "$MEMENTO_IDENTITY_SALT"; do
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

echo "done: T8.3b live checks pass; index deleted; transcript in $OUT (git-ignored)"
