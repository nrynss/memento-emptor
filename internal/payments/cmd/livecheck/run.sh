#!/usr/bin/env bash
# T8.7 live check: four webhook deliveries against the sandbox verify API
# through the ingest gate. Reads one stored delivery; creates nothing.
# Credentials come from `source tools/env.sh`, never from this file.
set -euo pipefail

: "${PAYPAL_CLIENT_ID:?source tools/env.sh first}"
: "${PAYPAL_CLIENT_SECRET:?source tools/env.sh first}"

here="internal/payments/cmd/livecheck"
OUT="${OUT:-$here/out}"
mkdir -p "$OUT"
chmod 700 "$OUT"

go run "./$here" | tee "$OUT/transcript-redacted.txt"

# Leak check: neither credential value may appear in any file this run
# wrote. Only file names are printed, never a value.
leaks=""
for value in "$PAYPAL_CLIENT_ID" "$PAYPAL_CLIENT_SECRET"; do
	found=$(grep -rlF -- "$value" "$here" || true)
	[ -z "$found" ] || leaks="${leaks}${found}"$'\n'
done
if [ -n "$leaks" ]; then
	echo "secret value entered a file:" >&2
	printf '%s' "$leaks" >&2
	exit 1
fi

git check-ignore -q "$here/out/transcript.json"

echo "done: redacted transcript above; full transcript in $OUT/transcript.json"
