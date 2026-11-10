#!/usr/bin/env bash
# T3.6 run: product.md's six demo steps, end to end, live on the real stack.
#
# Start from reset: bash demo/reset.sh <run-number>
# Then: bash demo/run.sh
#
# Legs (stops on the first failure; every spend leg has a guard):
#   1. Live PayPal read-back of the three T1.3b fit cycles (free).
#   2. Layer story on live Lambo + live Jev (4 classifications, ~$0.0001,
#      10x-per-call guard, pinned build) with recording payments whose
#      shapes T2.1b proved identical to the sandbox reads.
#   3. Channel3 live detail + frozen match (free; detail calls cost no
#      credits, no searches are used).
#   4. Two deployed-agent turns via the messaging proxy (~$0.03-0.04
#      each, $0.10 per-turn guard, billing re-checked around each turn).
#   5. One new sandbox purchase through agent/orders (sandbox only, free)
#      with a real explicit-yes plus the scripted shopper approval, then
#      the verified capture joins the layer and fits-well feedback ties
#      to it.
#   6. Live dispute read-back (free) + seller-only proof + the ledger and
#      trace span checks.
#
# Real ids stay in demo/out/ (git-ignored). Committed records use the
# placeholders from the T0.4 table plus RUN-scoped CUSTOM/ORD/CAP/EVT and
# CONV/TRACE ids.
set -euo pipefail

cd "$(dirname "$0")/.."

OUT="${OUT:-demo/out}"
export OUT
if [ ! -f "$OUT/SESSION" ]; then
	echo "run demo/reset.sh <run-number> first" >&2
	exit 2
fi

: "${PAYPAL_CLIENT_ID:?source tools/env.sh first}"
: "${PAYPAL_CLIENT_SECRET:?source tools/env.sh first}"
: "${OPENROUTER_API_KEY:?source tools/env.sh first}"
: "${ELASTIC_URL:?source tools/env.sh first}"
: "${LAMBO_ES_API_KEY:?source tools/env.sh first}"
: "${CHANNEL3_API_KEY:?source tools/env.sh first}"

export SHOPPER_EMAIL="$(secret-tool lookup service paypal-sandbox account memento-emptor kind shopper-email)"
export SHOPPER_PASSWORD="$(secret-tool lookup service paypal-sandbox account memento-emptor kind shopper-password)"

LAMBO_BIN="${LAMBO_BIN:-$HOME/work/lambo/target/release/lambo}"
export LAMBO_BIN
export PATH="$HOME/.ast/bin:$PATH"

go run ./demo/driver

echo "done: T3.6 run complete; transcripts under $OUT (git-ignored)"
