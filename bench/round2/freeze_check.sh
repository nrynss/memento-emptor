#!/usr/bin/env bash
# bench/round2/freeze_check.sh: verify the T10.3 frozen round-2 inputs.
#
# Extracts the sha256 manifest from bench/round2/FROZEN.md section 1 (the
# single ```manifest fenced block) and checks every listed file byte for byte
# with sha256sum -c. Any tampered, added (extra manifest row), or removed
# (missing file or dropped row) frozen file fails the run nonzero.
#
# Nothing is skipped silently: an unhashable path (missing file, unreadable
# file, empty manifest) fails loudly instead of being filtered out. The
# expected row count pins the manifest itself, so edits to FROZEN.md that add
# or drop rows fail here rather than passing a weaker check.
#
# The run directory is checked in the same place. Before T10.4 opens it, a
# bench/round2/runs directory that exists is a freeze break: the freeze lands
# first, and the runs come after. Once T10.4 has written the marker
# bench/round2/runs/FREEZE.json (round2.BeginRuns, FROZEN.md section 8), the
# gate passes --allow-runs and the directory is expected; the marker's
# binding to this freeze is then enforced by TestRunsBeginOnlyAfterTheFreeze.
#
# Usage: bench/round2/freeze_check.sh [--allow-runs]
#
# Offline: no keys, no network, no spend. Only POSIX shell plus coreutils
# (sed, grep, sha256sum) already present wherever tools/check.sh runs.

set -euo pipefail
cd "$(dirname "$0")/../.."

allow_runs=0
case "${1:-}" in
"") ;;
--allow-runs) allow_runs=1 ;;
*)
	echo "usage: bench/round2/freeze_check.sh [--allow-runs]" >&2
	exit 2
	;;
esac
[ "$#" -le 1 ] || {
	echo "usage: bench/round2/freeze_check.sh [--allow-runs]" >&2
	exit 2
}

# The one testable place for manifest extraction: print the sha256sum-format
# rows of the ```manifest block in bench/round2/FROZEN.md section 1. The sed
# range selects lines between the opening ```manifest fence and the next
# closing ``` fence; the grep keeps only lines carrying the two-space
# sha256sum separator, dropping the fences and any prose. Output feeds
# sha256sum -c unchanged, so this function and the check below cannot drift
# apart.
extract_manifest() {
	sed -n '/^```manifest$/,/^```$/p' bench/round2/FROZEN.md | grep '  '
}

# FROZEN.md section 1 pins exactly this many rows. A count mismatch means the
# manifest itself was edited (rows added or dropped), which is itself a freeze
# break: fail before hashing anything.
expected_rows=116
manifest=$(extract_manifest)
actual_rows=$(printf '%s\n' "$manifest" | grep -c '  ')
[ "$actual_rows" -eq "$expected_rows" ] || {
	echo "FAIL [freeze round 2] manifest row count $actual_rows, want $expected_rows: bench/round2/FROZEN.md section 1 was edited" >&2
	exit 1
}

# No arm 3 or arm 4 run may exist before the freeze commit. T10.4 owns
# bench/round2/runs/ and opens it with a marker; until that marker exists,
# the gate calls this script without the flag and a run directory fails here.
if [ "$allow_runs" -eq 0 ] && [ -e bench/round2/runs ]; then
	echo "FAIL [freeze round 2] bench/round2/runs exists: an arm 3 or arm 4 run predates the freeze" >&2
	exit 1
fi

# Strict check: sha256sum -c fails nonzero on a tampered hash, a missing
# file, or an unreadable file, and set -euo pipefail propagates it. No
# filtering, no allowlist, no fallback: anything unhashable fails the gate.
# --quiet keeps the 116 OK lines out of the gate's transcript; every failure
# still prints.
printf '%s\n' "$manifest" | sha256sum -c --quiet - || {
	echo "FAIL [freeze round 2] frozen comparison inputs mismatch: a file listed in bench/round2/FROZEN.md section 1 is tampered, added, or removed" >&2
	exit 1
}

if [ "$allow_runs" -eq 1 ]; then
	echo "OK round 2 freeze hashes match ($expected_rows rows, runs open with a marker)"
else
	echo "OK round 2 freeze hashes match ($expected_rows rows, no run yet)"
fi
