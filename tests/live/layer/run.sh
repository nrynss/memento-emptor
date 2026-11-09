#!/usr/bin/env bash
# T2.6 live run: the layer end to end with no agent, against the real
# pieces. Reads only where records exist; creates only trivial calls.
# Credentials come from `source tools/env.sh`, never from this file.
set -euo pipefail

: "${PAYPAL_CLIENT_ID:?source tools/env.sh first}"
: "${PAYPAL_CLIENT_SECRET:?source tools/env.sh first}"
: "${OPENROUTER_API_KEY:?source tools/env.sh first}"
: "${ELASTIC_URL:?source tools/env.sh first}"
: "${LAMBO_ES_API_KEY:?source tools/env.sh first}"

LAMBO_BIN="${LAMBO_BIN:-$HOME/work/lambo/target/release/lambo}"
export LAMBO_BIN
OUT="${OUT:-tests/live/layer/out}"
export OUT
mkdir -p "$OUT"

echo "=== leg 1: live PayPal read-back (nothing created) ==="
bash tests/live/ingest/run.sh

echo "=== leg 2: live Elastic hybrid legs (throwaway indices) ==="
bash tests/live/evidence/run.sh

echo "=== leg 3: interface story on live Lambo + live Jev ==="
go run ./tests/live/layer

echo "=== leak check: no secret value entered tests/live/layer ==="
leaks=""
for value in "$PAYPAL_CLIENT_ID" "$PAYPAL_CLIENT_SECRET" "$OPENROUTER_API_KEY" "$LAMBO_ES_API_KEY"; do
	found=$(grep -rlF -- "$value" tests/live/layer || true)
	[ -z "$found" ] || leaks="${leaks}${found}"$'\n'
done
if [ -n "$leaks" ]; then
	echo "secret value entered a file:" >&2
	printf '%s' "$leaks" >&2
	exit 1
fi
# The deployment URL is a locator, not a secret, but it still stays
# out of the repo: the generated config lives under out/ only.
if [ -n "${ELASTIC_URL:-}" ]; then
	found=$(grep -rlF --exclude-dir=out -- "$ELASTIC_URL" tests/live/layer || true)
	if [ -n "$found" ]; then
		echo "deployment URL entered a committed file:" >&2
		printf '%s' "$found" >&2
		exit 1
	fi
fi

# The full transcript must stay out of git.
git check-ignore -q tests/live/layer/out

echo "done: T2.6 live legs pass; transcripts under tests/live/*/out (git-ignored)"
