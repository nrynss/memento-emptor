#!/usr/bin/env bash
# T2.3b live entry point: direct-Elastic legs, then the Lambo leg.
# Usage: source tools/env.sh && bash tests/live/evidence/run.sh
set -euo pipefail
cd "$(dirname "$0")/../../.."

INDEX="t23b-live-$(python3 -c 'import secrets; print(secrets.token_hex(3))')"
mkdir -p tests/live/evidence/out
python3 tests/live/evidence/live.py "$INDEX" tests/live/evidence/out
bash tests/live/evidence/lambo.sh
