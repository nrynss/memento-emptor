#!/usr/bin/env bash
# T3.6 reset: return the demo to its starting state.
#
# A reset mints a FRESH Lambo session (an erased session id is tombstoned
# and can never serve again), empties the run scratch dir, and prints the
# spend baseline. PayPal sandbox records are immutable history: nothing
# deletes them, and the demo re-reads the T1.3b cycles every run.
#
# Usage: bash demo/reset.sh <run-number>   (run-number: 1, 2, ...)
set -euo pipefail

RUN="${1:?usage: bash demo/reset.sh <run-number>}"
cd "$(dirname "$0")/.."

OUT="demo/out"
mkdir -p "$OUT"

# Best-effort erase of the previous session, if one is recorded. An
# already-erased id reports already_absent; either way the old id is
# never reused.
if [ -f "$OUT/SESSION" ]; then
	OLD="$(cat "$OUT/SESSION")"
	LAMBO_BIN="${LAMBO_BIN:-$HOME/work/lambo/target/release/lambo}"
	if [ -x "$LAMBO_BIN" ] && [ -f "$OUT/lambo.toml" ]; then
		"$LAMBO_BIN" --config "$OUT/lambo.toml" erase-session \
			--session "$OLD" --confirm "$OLD" 2>&1 | tail -1 || true
	fi
fi

rm -rf "$OUT"
mkdir -p "$OUT"
SESSION="t36-run-${RUN}-$(date -u +%H%M%S)"
printf '%s' "$SESSION" > "$OUT/SESSION"
printf '%s' "$RUN" > "$OUT/RUN"
echo "session: $SESSION"

source tools/env.sh
export PATH="$HOME/.ast/bin:$PATH"
echo "--- spend baseline ---"
ast billing get 2>&1 | grep -E "Usage|Credit|Limit" || true
echo "reset done: demo/out holds SESSION only; sandbox history untouched."
