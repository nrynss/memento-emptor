#!/usr/bin/env bash
# T2.1b live driver: one sandbox refund and one sandbox dispute through
# the running ingest service. Reads only; creates nothing.
# Credentials come from `source tools/env.sh`, never from this file.
set -euo pipefail

: "${PAYPAL_CLIENT_ID:?source tools/env.sh first}"
: "${PAYPAL_CLIENT_SECRET:?source tools/env.sh first}"

OUT="${OUT:-tests/live/ingest/out}"
mkdir -p "$OUT"

go run ./tests/live/ingest | tee "$OUT/transcript-redacted.txt"

# Leak check: neither secret value may appear in any file (run.sh
# prints file names only; the values are never echoed).
leaks=""
for value in "$PAYPAL_CLIENT_ID" "$PAYPAL_CLIENT_SECRET"; do
	found=$(grep -rlF -- "$value" tests/live/ingest || true)
	[ -z "$found" ] || leaks="${leaks}${found}"$'\n'
done
if [ -n "$leaks" ]; then
	echo "secret value entered a file:" >&2
	printf '%s' "$leaks" >&2
	exit 1
fi

# The full transcript must stay out of git.
git check-ignore -q tests/live/ingest/out/transcript.json

echo "done: redacted transcript above; full transcript in $OUT/transcript.json"
