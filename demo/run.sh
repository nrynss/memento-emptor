#!/usr/bin/env bash
# T9.7 run: product.md's pivot demo, steps 0 to 7, against the hosted services.
#
# Start from a reset:  bash demo/reset.sh <run-number>
# Then:                bash demo/run.sh
#
# Every leg reads or writes a live service, except the deployed ledger page,
# which still replays the frozen story from fixtures (recorded as such). The
# driver walks the steps, writes raw answers and full trace ids to demo/out/
# (git-ignored), and writes the redacted evidence the run record cites to
# demo/evidence/run-<n>/.
#
# Agent turns are the only spend: the driver checks the spend baseline the
# reset wrote and records the delta. A layer read costs nothing.
#
# MEMENTO_SECOND_DEPLOYMENT must be set to the second agent's deployment id;
# the id never lives in this repository.
set -euo pipefail

cd "$(dirname "$0")/.."

OUT="${OUT:-demo/out}"
export OUT
if [ ! -f "$OUT/RUN" ]; then
	echo "run demo/reset.sh <run-number> first" >&2
	exit 2
fi

if [ -z "${MEMENTO_SECOND_DEPLOYMENT:-}" ]; then
	echo "set MEMENTO_SECOND_DEPLOYMENT to the second agent's deployment id" >&2
	exit 2
fi

if ! command -v secret-tool >/dev/null 2>&1; then
	echo "secret-tool is not on PATH; the keys come from the system keyring" >&2
	exit 2
fi

source tools/env.sh >/dev/null 2>&1 || true
export PATH="$HOME/.ast/bin:$PATH"

python3 demo/run.py

echo "done: run $(cat "$OUT/RUN"); evidence in demo/evidence/run-$(cat "$OUT/RUN")/"
