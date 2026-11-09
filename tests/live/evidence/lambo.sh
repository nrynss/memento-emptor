#!/usr/bin/env bash
# T2.3b Lambo leg: 0.10.1 binary with embed-elastic + recall-elastic against
# the real deployment (build only, /tmp/lambo-0.10.1; nothing committed).
#
# Proves: (a) the hardcoded-5s failure from T0.6b reproduces WITHOUT
# connect_timeout_ms and succeeds WITH it (#236 fix); (b) a new SKU-shaped
# recall finds the derived returned items through native embeddings;
# (c) the shipped jina-v3 calibration row behaves sanely on our pair.
# Cleanup: both sessions erased, every t23b-live* index deleted.
#
# Keys never enter files: the toml carries only api_key_env / {env=} refs.
# The deployment URL is written into /tmp scratch config only (never the repo).
# Usage: source tools/env.sh && bash tests/live/evidence/lambo.sh
set -uo pipefail

: "${ELASTIC_URL:?source tools/env.sh first}"
: "${LAMBO_ES_API_KEY:?source tools/env.sh first}"

WORK=/tmp/opencode/t23b-lambo
OUT=tests/live/evidence/out
BIN=/tmp/lambo-0.10.1
mkdir -p "$WORK" "$OUT"

mk_toml() { # $1 = file, $2 = extra embedder lines
cat > "$1" <<EOF
[store]
kind = "sqlite"
path = "$WORK/lambo.db"

[embedder]
kind = "elastic"
url = "$ELASTIC_URL"
model = ".jina-embeddings-v3"
dim = 1024
api_key_env = "LAMBO_ES_API_KEY"
canary = "verify"
$2

[recall]
kind = "elastic"
url = "$ELASTIC_URL"
api_key = { env = "LAMBO_ES_API_KEY" }
index_prefix = "t23b-live"
timeout_ms = 20000

[daemon]
gc_interval = 1
gc_idle_floor = 1
gc_max_interval_secs = 3600
canonization_eval_interval_secs = 5
EOF
}

echo "--- 0.10.1 binary"
"$BIN" --version | tee "$OUT/lambo-version.txt"

echo "--- negative: default 5s connect timeout (no connect_timeout_ms)"
mk_toml "$WORK/neg.toml" ""
"$BIN" --config "$WORK/neg.toml" provision 2>&1 | tail -1
if "$BIN" --config "$WORK/neg.toml" derive --session t23b-neg --agent t23b-live \
    --content "Brand X classic oxford shirt size M returned: too tight in the shoulders" \
    --kind observation > "$OUT/neg-derive.txt" 2>&1; then
  echo "UNEXPECTED: derive succeeded without connect_timeout_ms"
else
  echo "derive refused as in T0.6b (see out/neg-derive.txt):"
  head -c 600 "$OUT/neg-derive.txt"; echo
fi

echo "--- knob check: connect_timeout_ms = 0 is refused (knob is read, not ignored)"
mk_toml "$WORK/zero.toml" "connect_timeout_ms = 0"
"$BIN" --config "$WORK/zero.toml" derive --session t23b-zero --agent t23b-live \
  --content "knob probe" --kind observation > "$OUT/zero-derive.txt" 2>&1 \
  && echo "UNEXPECTED: zero timeout accepted" \
  || { echo "refused:"; head -c 300 "$OUT/zero-derive.txt"; echo; }
echo "--- positive: connect_timeout_ms = 20000"
mk_toml "$WORK/lambo.toml" "connect_timeout_ms = 20000
timeout_ms = 60000"
"$BIN" --config "$WORK/lambo.toml" provision 2>&1 | tail -1
"$BIN" --config "$WORK/lambo.toml" derive --session t23b-live --agent t23b-live \
  --content "Brand X classic oxford shirt size M returned: too tight in the shoulders" \
  --kind observation 2>&1 | tee "$OUT/derive-1.txt" | head -c 1500; echo
"$BIN" --config "$WORK/lambo.toml" derive --session t23b-live --agent t23b-live \
  --content "Brand X slim oxford shirt size M returned: too tight across the chest" \
  --kind observation 2>&1 | tee "$OUT/derive-2.txt" | head -c 1500; echo

echo "--- backfill + recall: new SKU-shaped query"
"$BIN" --config "$WORK/lambo.toml" recall-index backfill --session t23b-live 2>&1 | tee "$OUT/backfill.txt" | head -c 800; echo
"$BIN" --config "$WORK/lambo.toml" recall --session t23b-live \
  --query "new Brand X oxford shirt, snug across the chest" 2>&1 | tee "$OUT/recall.txt" | head -c 3000; echo

echo "--- calibration + indices before cleanup"
"$BIN" --config "$WORK/lambo.toml" stats --session t23b-live 2>&1 | head -c 800; echo
curl -s -m 30 -H "Authorization: ApiKey $LAMBO_ES_API_KEY" \
  "$ELASTIC_URL/_cat/indices/t23b-live*?format=json&h=index" | tee "$OUT/indices-before.txt"; echo

echo "--- cleanup: erase sessions, delete indices, verify empty"
"$BIN" --config "$WORK/lambo.toml" erase-session --session t23b-live --confirm t23b-live 2>&1 | head -c 500; echo
"$BIN" --config "$WORK/lambo.toml" erase-session --session t23b-neg --confirm t23b-neg 2>&1 | head -c 300; echo
for idx in $(curl -s -m 30 -H "Authorization: ApiKey $LAMBO_ES_API_KEY" \
    "$ELASTIC_URL/_cat/indices/t23b-live*?format=json&h=index" | python3 -c "import sys,json; print('\n'.join(r['index'] for r in json.load(sys.stdin)))"); do
  [ -n "$idx" ] || continue
  curl -s -m 30 -X DELETE -H "Authorization: ApiKey $LAMBO_ES_API_KEY" \
    "$ELASTIC_URL/$idx" -o /dev/null -w "deleted <index> %{http_code}\n"
done
echo "remaining t23b*:"
curl -s -m 30 -H "Authorization: ApiKey $LAMBO_ES_API_KEY" \
  "$ELASTIC_URL/_cat/indices/t23b*?format=json"
echo "all indices:"
curl -s -m 30 -H "Authorization: ApiKey $LAMBO_ES_API_KEY" \
  "$ELASTIC_URL/_cat/indices?format=json&h=index"
