#!/usr/bin/env bash
# T8.5b live run: the served layer end to end, through the hub and a
# test-prefixed fit index.
#
# The driver boots one hub in the T8.1 shape (fixture embedder, live
# recall tier, test-prefixed indices t85b-*), wires one served layer per
# shopper over its own hub session plus the live pool reader and fit
# writer on t85b-fit-rows, decides one purchase per precedence step
# (correction, own evidence, pool, statement, nothing), shows the
# cross-session correction holding over a fresh hub MCP session and a
# layer restart, and pins the declared-view re-link T8.5d landed (after a
# restart and after a divergent rebuild) plus the consulted-but-empty
# pool serving. It deletes every test index afterwards. This script adds
# the cleanup check: the prefix listing must be empty afterwards, and no
# key or deployment location may have entered a file.
#
# Usage, from the repository root:
#   source tools/env.sh
#   LAMBO_BIN=/path/to/lambo-0.10.1 bash tests/live/precedence/run.sh
#
# LAMBO_BIN must be the 0.10.1 release binary (the deploy pins its sha).
# Nothing here runs in CI and nothing here gates a build: there are no
# *_test.go files under tests/live/precedence/, so the gate builds the
# driver but never executes it. Spend: $0.00 of embedding spend (the
# fixture embedder computes vectors in process; every answer replays a
# labelled recording), against the $0.20 budget. Elastic usage is
# test-prefixed index churn only.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE/../../.."

: "${ELASTIC_URL:?source tools/env.sh first}"
: "${LAMBO_ES_API_KEY:?source tools/env.sh first}"
: "${MEMENTO_IDENTITY_SALT:?source tools/env.sh first}"
: "${LAMBO_BIN:?set LAMBO_BIN to the 0.10.1 release binary first}"

"$LAMBO_BIN" --version

OUT="${OUT:-$HERE/out}"
mkdir -p "$OUT"

go run ./tests/live/precedence | tee "$OUT/transcript.txt"

echo "--- cleanup check: the t85b- prefix is empty ---"
curl -s -m 60 -H "Authorization: ApiKey $LAMBO_ES_API_KEY" \
	"$ELASTIC_URL/_cat/indices/t85b-*?format=json&h=index" | tee "$OUT/indices-after.json"
echo
python3 - "$OUT/indices-after.json" <<'PY'
import json, sys

with open(sys.argv[1]) as handle:
    names = [row["index"] for row in json.load(handle)]
if names:
    print(f"FAIL: indices remain under t85b-: {names}", file=sys.stderr)
    sys.exit(1)
print("prefix t85b- is empty")
PY

echo "--- leak check: no key or deployment location entered tests/live/precedence ---"
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

echo "done: T8.5b live checks pass; indices deleted; transcript in $OUT (git-ignored)"
