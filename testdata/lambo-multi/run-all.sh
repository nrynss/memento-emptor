# Replay the T0.11 checks on the Lambo v0.10.1 release binary.
#
#   LAMBO_BIN=/path/to/lambo ./run-all.sh
#
# The binary must be v0.10.1 built with the release feature set (`ship`):
#
#   git archive v0.10.1 | tar -x -C /tmp/lambo-v0.10.1
#   cd /tmp/lambo-v0.10.1
#   LAMBO_GIT_SHA=$(git rev-parse --short=7 v0.10.1) \
#     cargo build --locked --release --features ship
#
# Check 4 (the live Elastic recall tier) runs only when ELASTIC_URL and
# LAMBO_ES_API_KEY are set; source tools/env.sh from the repository root. It
# uses indices under `t011-multi-` and deletes every one it creates.
#
# The bearer token is generated here when absent and only ever exported: it
# never appears in a transcript or a file.
set -euo pipefail

: "${LAMBO_BIN:?set LAMBO_BIN to a Lambo v0.10.1 binary built with --features ship}"
export LAMBO_BIN
export LAMBO_MULTI_TOKEN="${LAMBO_MULTI_TOKEN:-$(python3 -c 'import secrets; print(secrets.token_urlsafe(32))')}"
export T011_RUN="${T011_RUN:-/tmp/lambo-multi-run}"

here="$(cd "$(dirname "$0")" && pwd)"

for check in check1.py check1b_eviction.py check2.py check3.py \
	check5.py check6.py check7.py; do
	echo "== $check"
	python3 "$here/$check"
done

if [[ -n "${ELASTIC_URL:-}" && -n "${LAMBO_ES_API_KEY:-}" ]]; then
	echo "== check4.py (live Elastic)"
	python3 "$here/check4.py"
else
	echo "== check4.py skipped: ELASTIC_URL / LAMBO_ES_API_KEY are not set"
fi
