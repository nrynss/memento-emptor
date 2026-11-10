#!/usr/bin/env bash
# Boot supervisor for the single Render web service: the ledger edge on
# $PORT, the memory layer replaying the frozen demo story, and Lambo
# beside them with its store on the persistent disk. Lambo's config is
# written here at boot from the environment (the deployment URL lives
# on the disk, never in the repo); the seed runs once per contract,
# proved by a marker file on the same disk, so a restart reseeds
# nothing and loses nothing.
#
# Lambo is best-effort at boot: when neither embedder answers, the
# replay service still serves the demo and the supervisor says so
# loudly. A later deploy retries the seed (no marker was written).
set -euo pipefail

APP="${APP_DIR:-/app}"
DATA="${LAMBO_DATA_DIR:-/var/lambo-data}"
SESSION="${LAMBO_SESSION:-demo-shopper}"
AGENT="${LAMBO_AGENT:-memento-seed}"
STORE="$DATA/lambo.db"
CONFIG="$DATA/lambo.toml"
WEB_PORT="${LAMBO_WEB_PORT:-7723}"
MEMENTO_ADDR="${MEMENTO_ADDR:-127.0.0.1:18080}"

mkdir -p "$DATA"

pids=()
stop_all() {
	for pid in "${pids[@]}"; do
		kill "$pid" 2>/dev/null || true
	done
}
trap stop_all TERM INT

write_config() {
	local kind="$1"
	{
		echo "# Written at boot by deploy/start.sh from the environment."
		echo "# The deployment URL lives on this disk, never in the repo."
		echo "[store]"
		echo "kind = \"sqlite\""
		echo "path = \"$STORE\""
		echo ""
		echo "[embedder]"
		case "$kind" in
		native)
			echo "kind = \"elastic\""
			echo "url = \"$ELASTIC_URL\""
			echo "model = \".jina-embeddings-v3\""
			echo "dim = 1024"
			echo "api_key_env = \"LAMBO_ES_API_KEY\""
			echo "connect_timeout_ms = 30000"
			echo "timeout_ms = 60000"
			echo "canary = \"verify\""
			;;
		bge)
			echo "kind = \"bge_m3\""
			echo "url = \"https://openrouter.ai/api\""
			echo "model = \"baai/bge-m3\""
			echo "dim = 1024"
			echo "api_key_env = \"OPENROUTER_API_KEY\""
			;;
		esac
		echo ""
		echo "# Cadence only, from the proven T0.3 toml; bars live in code."
		echo "[daemon]"
		echo "gc_interval = 1"
		echo "gc_idle_floor = 1"
		echo "gc_max_interval_secs = 3600"
		echo "canonization_eval_interval_secs = 5"
	} >"$CONFIG"
}

seed_kind() {
	local kind="$1"
	"$APP/bin/seed" \
		-lambo "$APP/bin/lambo" \
		-config "$CONFIG" \
		-session "$SESSION" \
		-agent "$AGENT" \
		-marker "$DATA/.seeded-$SESSION-$kind-v1"
}

echo "start: edge and memory layer first (health answers while Lambo seeds)"
MEMENTO_BACKEND="http://$MEMENTO_ADDR" EDGE_STATIC="$APP/web" "$APP/bin/edge" &
pids+=($!)
"$APP/bin/memento" -addr "$MEMENTO_ADDR" -seed -shopper shopper_01 \
	-schema-dir "$APP/testdata/api" -jev-dir "$APP/testdata/jev" &
pids+=($!)

if [ -n "${ELASTIC_URL:-}" ] && [ -n "${LAMBO_ES_API_KEY:-}" ]; then
	EMBEDDER=native
elif [ -n "${OPENROUTER_API_KEY:-}" ]; then
	EMBEDDER=bge
else
	EMBEDDER=none
fi

case "$EMBEDDER" in
native | bge)
	if [ "$EMBEDDER" = native ]; then
		write_config native
	else
		write_config bge
	fi
	"$APP/bin/lambo" --config "$CONFIG" provision
	set +e
	seed_kind "$EMBEDDER"
	seed_status=$?
	set -e
	if [ $seed_status -eq 3 ] && [ "$EMBEDDER" = native ] && [ -n "${OPENROUTER_API_KEY:-}" ]; then
		echo "start: native embedder unreachable, falling back to the spare contract once"
		write_config bge
		"$APP/bin/lambo" --config "$CONFIG" provision
		set +e
		seed_kind bge
		seed_status=$?
		set -e
	fi
	if [ $seed_status -ne 0 ]; then
		echo "start: WARNING lambo seed failed (status $seed_status); replay serves, graph retries next deploy"
	else
		echo "start: lambo seed holds; serving the live graph read-only"
		"$APP/bin/lambo" --config "$CONFIG" serve-web \
			--session "$SESSION" --port "$WEB_PORT" --bind 127.0.0.1 &
		pids+=($!)
	fi
	;;
none)
	echo "start: WARNING no embedder key in the environment; replay serves without the live graph"
	;;
esac

echo "start: all launched; a child exit stops the service so Render restarts it"
set +e
wait -n
status=$?
set -e
echo "start: a child exited ($status); stopping the rest"
stop_all
exit "$status"
