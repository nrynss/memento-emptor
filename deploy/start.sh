#!/usr/bin/env bash
# Boot supervisor for the single Render web service: the ledger edge on
# $PORT, the memory layer replaying the frozen demo story, the Lambo hub
# that holds one session per shopper, and one reader for the demo
# shopper's live graph. Lambo's config is written here at boot from the
# environment (the deployment URL lives on the disk, never in the repo)
# and carries no `[recall]` tier: the hub is store-only by decision, see
# deploy/README.md. The seed runs once per contract, proved by a marker
# file on the same disk, so a restart reseeds nothing and loses nothing.
#
# Order matters. The seeder's short-lived stdio writer owns its session
# alone, so it runs before the hub starts; the hub then attaches the
# seeded session as its one pinned session and every other shopper's
# session on demand, at /mcp/s/<session>. The pivoted layer is the one
# exception to the order: it replays its seed into the hub, so the hub
# is up before it boots, and the old graph arc retires with the replay —
# only the fit population still seeds, through the writer.
#
# Lambo is best-effort at boot: when neither embedder answers, the
# replay service still serves the demo and the supervisor says so
# loudly. A later deploy retries the seed (no marker was written).
# LAMBO_SEED=0 skips the frozen demo arc, for a local hub loop.
set -euo pipefail

APP="${APP_DIR:-/app}"
DATA="${LAMBO_DATA_DIR:-/var/lambo-data}"
AGENT="${LAMBO_AGENT:-memento-seed}"
STORE="$DATA/lambo.db"
CONFIG="$DATA/lambo.toml"
WEB_PORT="${LAMBO_WEB_PORT:-7723}"
MCP_PORT="${LAMBO_MCP_PORT:-7724}"
MEMENTO_ADDR="${MEMENTO_ADDR:-127.0.0.1:18080}"

# The hub's session prefix, from internal/identity.SessionNamePrefix. Every
# shopper session is named with it, so a request at /mcp/s/<session> may
# create the session on demand.
SESSION_PREFIX="shopper-"

# The seeded demo shopper's session: the prefix plus the 64-hex HMAC-SHA256
# of payer id `shopper_01` under the published seed salt (internal/identity's
# SeedSaltHex), which is the session-name vector its test pins. The seeder and
# the hub's one pinned session both use it, so the demo story is reachable at
# /mcp/s/<session> like any shopper's.
DEMO_SESSION="${LAMBO_DEMO_SESSION:-shopper-00895719a8a9d1ba88c99984e50dd200a83b42cfab629a4919c635033e2e5caf}"

# The ledger reader's allowlist: the demo shoppers, space separated. The
# reader's list is fixed at start (Lambo issue 241), so a shopper created
# after boot is not readable here until a restart lists it.
WEB_SESSIONS="${LAMBO_WEB_SESSIONS:-$DEMO_SESSION}"

# The pivoted layer serves the pool, the fit writer and the hub-backed
# graph behind one service per shopper. Its hub sessions are named under
# the live salt, not the seed salt above, so the pinned session and the
# reader's allowlist move to the live demo name: the layer's own
# subcommand derives it, and the name is a keyed hash, never a secret.
# The old graph arc retires with the replay: nothing plants the pre-pivot
# session again, and the reader stops listing it.
PIVOTED=0
STANDING_DIR=""
if [ -n "${MEMENTO_IDENTITY_SALT:-}" ]; then
	PIVOTED=1
	DEMO_SESSION="$("$APP/bin/memento" session --shopper shopper_01)"
	WEB_SESSIONS="${LAMBO_WEB_SESSIONS:-$DEMO_SESSION}"
	STANDING_DIR="${MEMENTO_STANDING_DIR:-$DATA/memento-standing}"
	mkdir -p "$STANDING_DIR"
	echo "start: pivoted layer on; demo hub session $DEMO_SESSION"
fi

# The fit index the pool reader and the fit writer share. Production is
# fit-rows; a proof run points the layer and the seeder at a scratch
# prefix through MEMENTO_FIT_INDEX and deletes it afterwards.
FIT_INDEX="${MEMENTO_FIT_INDEX:-fit-rows}"

# The one credential the hub accepts. The deploy generates a token per boot
# when the environment holds none, so the hub needs no stored secret and the
# layer's graph client (T8.5) can read the same value; a value from the
# environment is left alone. It is exported, never written to a file or
# printed.
if [ -z "${LAMBO_MCP_TOKEN:-}" ]; then
	LAMBO_MCP_TOKEN="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
fi
export LAMBO_MCP_TOKEN
export LAMBO_MCP_URL="http://127.0.0.1:$MCP_PORT"

# A demo session the credential cannot address would silently lose the demo
# story: the hub only attaches names under its prefix, and the addressed-name
# rule caps a name at 128 bytes of [A-Za-z0-9._:-].
case "$DEMO_SESSION" in
"$SESSION_PREFIX"*) ;;
*)
	echo "start: LAMBO_DEMO_SESSION must begin with $SESSION_PREFIX (the hub credential's prefix): $DEMO_SESSION" >&2
	exit 2
	;;
esac
if [[ ! "$DEMO_SESSION" =~ ^shopper-[A-Za-z0-9._:-]+$ ]] || [ "${#DEMO_SESSION}" -gt 128 ]; then
	echo "start: LAMBO_DEMO_SESSION is not an addressable session name (1-128 bytes of [A-Za-z0-9._:-]): $DEMO_SESSION" >&2
	exit 2
fi

mkdir -p "$DATA"

pids=()
stopping=0
stop_all() {
	if [ "$stopping" -eq 1 ]; then
		return
	fi
	stopping=1
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
		fixture)
			# The key-free local contract: vectors are computed in process.
			echo "kind = \"fixture\""
			echo "dim = 1024"
			echo "calibration = \"off\""
			;;
		esac
		echo ""
		# No [recall] section: the hub is store-only by decision. The tier
		# mirrors every session's vectors into Elastic on each flush and
		# moves recall's vector leg to a lagging kNN index, which answers
		# nothing the store's exact scan does not. See deploy/README.md.
		echo "# Cadence only, from the proven T0.3 toml; bars live in code."
		echo "[daemon]"
		echo "gc_interval = 1"
		echo "gc_idle_floor = 1"
		echo "gc_max_interval_secs = 3600"
		echo "canonization_eval_interval_secs = 5"
		echo ""
		echo "# One hub, one writer per shopper session. The demo shopper is the"
		echo "# one pinned session: /mcp serves it and it never idle-detaches;"
		echo "# every other session attaches on its first request under the"
		echo "# credential's prefix. Pinned sessions count against max_attached,"
		echo "# so 15 shoppers are attached at once beside it (16 measured safe)."
		echo "[serve]"
		echo "sessions = [\"$DEMO_SESSION\"]"
		echo "default_session = \"$DEMO_SESSION\""
		echo "max_attached = 16"
		echo "attach_concurrency = 1"
		echo "idle_detach_secs = 900"
		echo ""
		echo "[[serve.credential]]"
		echo "name = \"memento\""
		echo "token_env = \"LAMBO_MCP_TOKEN\""
		echo "session_prefix = \"$SESSION_PREFIX\""
		echo "create = true"
	} >"$CONFIG"
}

seed_kind() {
	local kind="$1"
	"$APP/bin/seed" \
		-lambo "$APP/bin/lambo" \
		-config "$CONFIG" \
		-session "$DEMO_SESSION" \
		-agent "$AGENT" \
		-marker "$DATA/.seeded-$DEMO_SESSION-$kind-v1"
}

# start_hub runs the one Lambo writer. It attaches the pinned demo session at
# startup and creates every other shopper's session on demand; the session
# ceiling and rate ceiling are raised from the defaults so an ingest burst is
# not throttled.
start_hub() {
	echo "start: hub on 127.0.0.1:$MCP_PORT; shopper sessions attach at /mcp/s/<session>"
	"$APP/bin/lambo" --config "$CONFIG" serve \
		--transport http --bind 127.0.0.1 --port "$MCP_PORT" \
		--agent memento-layer --max-sessions 128 --rate-limit-rps 200 &
	pids+=($!)
}

# start_reader runs the read-only ledger portal over the allowlist in
# WEB_SESSIONS, one window per demo shopper.
start_reader() {
	local args=(--config "$CONFIG" serve-web)
	local session
	for session in $WEB_SESSIONS; do
		args+=(--session "$session")
	done
	echo "start: ledger reader for [$WEB_SESSIONS] on 127.0.0.1:$WEB_PORT"
	"$APP/bin/lambo" "${args[@]}" --port "$WEB_PORT" --bind 127.0.0.1 &
	pids+=($!)
}

# start_memento runs the memory layer with the frozen demo story loaded.
# The pivoted layer replays its seed into the hub, so it boots only after
# the hub answers; the pre-pivot shape keeps the old order and boots
# first. The standing and fit flags are empty outside the pivot and the
# layer ignores them there.
start_memento() {
	echo "start: memory layer on $MEMENTO_ADDR (seed replays the demo story)"
	"$APP/bin/memento" -addr "$MEMENTO_ADDR" -seed -shopper shopper_01 \
		-schema-dir "$APP/testdata/api" -jev-dir "$APP/testdata/jev" \
		-standing-dir "$STANDING_DIR" -fit-index "$FIT_INDEX" &
	pids+=($!)
}

# wait_hub blocks until the hub accepts TCP on its MCP port, so a layer
# that replays into it never races its startup. A hub that never answers
# fails the boot loudly instead of serving half a layer.
wait_hub() {
	echo "start: waiting for the hub on 127.0.0.1:$MCP_PORT"
	attempts=0
	while [ "$attempts" -lt 60 ]; do
		if timeout 1 bash -c "</dev/tcp/127.0.0.1/$MCP_PORT" 2>/dev/null; then
			echo "start: hub answers"
			return 0
		fi
		attempts=$((attempts + 1))
		sleep 1
	done
	echo "start: hub never answered on 127.0.0.1:$MCP_PORT" >&2
	return 1
}

# seed_fit_only plants the fit population through the writer with the
# seeding moment as its now, then reads every line back through the pool
# reader before the disk marker lands. It never plants the pre-pivot
# graph arc: that session retires with the replay, and the pivoted layer
# never reads it. LAMBO_SEED=0 skips the seed for a local hub loop; the
# binary itself skips when the embedder is fixture or MEMENTO_FIT_SEED=0,
# so the documented key-free loop never touches the live index. The marker
# shares the disk with the legacy one, so an index the staged seeder
# already planted is honoured, never rewritten.
seed_fit_only() {
	if [ "${LAMBO_SEED:-1}" = "0" ]; then
		echo "start: LAMBO_SEED=0; the fit population is not planted this boot"
		return 0
	fi
	"$APP/bin/seed" \
		-lambo "$APP/bin/lambo" \
		-config "$CONFIG" \
		-session "$DEMO_SESSION" \
		-agent "$AGENT" \
		-marker "$DATA/.seeded-pivoted-v1" \
		-fit-index "$FIT_INDEX" \
		-fit-only
}

echo "start: edge first (health answers while the layer and Lambo boot)"
MEMENTO_BACKEND="http://$MEMENTO_ADDR" EDGE_STATIC="$APP/web" "$APP/bin/edge" &
pids+=($!)

EMBEDDER="${LAMBO_EMBEDDER:-}"
if [ -z "$EMBEDDER" ]; then
	if [ -n "${ELASTIC_URL:-}" ] && [ -n "${LAMBO_ES_API_KEY:-}" ]; then
		EMBEDDER=native
	elif [ -n "${OPENROUTER_API_KEY:-}" ]; then
		EMBEDDER=bge
	else
		EMBEDDER=none
	fi
fi

# A forced embedder contract needs the variables its section names, so the
# config it writes never holds an empty URL or a key that is not there.
case "$EMBEDDER" in
native)
	if [ -z "${ELASTIC_URL:-}" ] || [ -z "${LAMBO_ES_API_KEY:-}" ]; then
		echo "start: LAMBO_EMBEDDER=native needs ELASTIC_URL and LAMBO_ES_API_KEY" >&2
		exit 2
	fi
	;;
bge)
	if [ -z "${OPENROUTER_API_KEY:-}" ]; then
		echo "start: LAMBO_EMBEDDER=bge needs OPENROUTER_API_KEY" >&2
		exit 2
	fi
	;;
esac

case "$EMBEDDER" in
native | bge | fixture)
	write_config "$EMBEDDER"
	"$APP/bin/lambo" --config "$CONFIG" provision
	if [ "$PIVOTED" = "1" ]; then
		start_hub
		if ! wait_hub; then
			exit 1
		fi
		start_reader
		start_memento
		set +e
		seed_fit_only
		seed_status=$?
		set -e
		if [ $seed_status -ne 0 ]; then
			echo "start: WARNING fit seed failed (status $seed_status); the pool reads what the index holds and the seed retries next deploy"
		else
			echo "start: fit seed holds"
		fi
	else
		start_memento
		seeded=ok
		if [ "${LAMBO_SEED:-1}" = "0" ]; then
			echo "start: LAMBO_SEED=0; the frozen demo arc is not planted this boot"
			seeded=skipped
			seed_status=0
		else
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
				seeded=failed
			fi
		fi
		case "$seeded" in
		ok)
			echo "start: lambo seed holds"
			;;
		failed)
			echo "start: WARNING lambo seed failed (status $seed_status); the graph serves what the store holds and the arc retries next deploy"
			;;
		esac
		start_hub
		start_reader
	fi
	;;
none)
	echo "start: WARNING no embedder key in the environment; replay serves without the live graph"
	;;
*)
	echo "start: LAMBO_EMBEDDER must be native, bge, fixture or none: $EMBEDDER" >&2
	exit 2
	;;
esac

echo "start: all launched; a child exit stops the service so Render restarts it"
set +e
wait -n
status=$?
set -e
echo "start: a child exited ($status); stopping the rest"
stop_all
# Let the children finish their own close before this process exits. Lambo's
# memory writes are a write-behind log with no on-disk WAL, so a second
# signal, or the container going away mid-close, loses the unflushed tail.
set +e
wait
set -e
exit "$status"
