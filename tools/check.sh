#!/usr/bin/env bash
# The gate. Every check runs, in order, and the first failure names itself and
# stops the run. Nothing here is skipped silently.
#
# The same script runs on the workstation and in CI, so a local green run means
# a CI green run, check for check.

set -euo pipefail
cd "$(dirname "$0")/.."

fail() {
	echo "FAIL [$1] $2" >&2
	exit 1
}

for tool in go node npm; do
	command -v "$tool" >/dev/null 2>&1 || fail "tools" "$tool is not on PATH"
done

bench/freeze_check.sh || fail "freeze" "frozen comparison inputs mismatch (bench/FROZEN.md section 1)"

# Round 2: a runs directory is allowed only once T10.4 has opened it with the
# marker bench/round2/runs/FREEZE.json (bench/round2/FROZEN.md section 8).
# The marker's binding to this freeze is enforced by the Go test the gate
# runs below; here the point is that the directory itself is not a freeze
# break once it has been opened deliberately.
round2_freeze_flag=""
if [ -f bench/round2/runs/FREEZE.json ]; then
	round2_freeze_flag="--allow-runs"
fi
bench/round2/freeze_check.sh $round2_freeze_flag || fail "freeze round 2" "frozen round 2 inputs mismatch (bench/round2/FROZEN.md section 1)"

# The toolchain pins are compared across every file that states one, first in
# the gate. Go's are both deploy images' FROM lines, CI's setup-go, go.mod and
# the Go row of deploy/README.md's table; Node's are both deploy images'
# web-build stages, CI's setup-node and the table's Node row; Bun's are the
# `oven/bun:` stage of every Dockerfile in the tree, CI's setup-bun and the
# table's Bun row. That is 18 declarations in all. An image built on a
# toolchain CI does not use builds green in both places, so the pins are
# compared here. tools/pincheck's own table test proves the disagreements it
# catches; `go test` below runs it.
echo "-- toolchain pins"
go run ./tools/pincheck || fail "pins" "toolchain pins disagree (see above)"

echo "-- gofmt"
go_files=$(git ls-files '*.go')
if [ -n "$go_files" ]; then
	# Go file paths never carry whitespace in this repository.
	unformatted=$(gofmt -l $go_files)
	[ -z "$unformatted" ] || fail "gofmt" "not gofmt-clean: $unformatted"
fi

# Go tooling must not walk web/node_modules, which carries third-party Go
# snippets that are not this project's code.
all_pkgs=$(go list ./...)
go_pkgs=$(printf '%s\n' "$all_pkgs" | grep -v '^github.com/nrynss/memento-emptor/web/node_modules/' | grep -v '^github.com/nrynss/memento-emptor/merchant/node_modules/' || true)
[ -n "$go_pkgs" ] || fail "go list" "no Go packages found"

echo "-- go vet"
go vet $go_pkgs || fail "go vet" "go vet reported problems"

echo "-- go test -race"
go test -race $go_pkgs || fail "go test -race" "tests failed"

run_frontend() {
	local name=$1
	shift
	local log
	log=$(mktemp)
	if ! "$@" >"$log" 2>&1; then
		cat "$log"
		rm -f "$log"
		fail "$name" "$name reported problems"
	fi
	rm -f "$log"
}

check_frontend() {
	local dir=$1
	cd "$dir"

	[ -d node_modules ] || fail "$dir deps" "$dir/node_modules is missing; run npm ci in $dir"

	echo "-- $dir lockfile"
	run_frontend "$dir lockfile" node scripts/check-lock.mjs
	run_frontend "$dir lockfile" npm ci --dry-run

	echo "-- $dir svelte-check"
	run_frontend "$dir svelte-check" npm run check

	echo "-- $dir eslint"
	run_frontend "$dir eslint" npm run lint

	echo "-- $dir vitest"
	run_frontend "$dir vitest" npm run test

	echo "-- $dir build"
	run_frontend "$dir build" npm run build

	cd ..
}

check_frontend web
check_frontend merchant

# --- agent/frontend: the public shopper page (T9.12) ---
# The relay's request validation and its 30-turn anonymous spend cap live in
# agent/frontend/server/relay.ts, which imports nothing, so its tests run on
# the node this gate already requires: no bun, no package install, no
# gateway, no model call.
echo "-- agent/frontend relay tests"
run_frontend "agent/frontend relay tests" node --test agent/frontend/server/relay.test.ts

# The package's own check is `bun run typecheck` (tsc --noEmit over server/),
# and it needs bun plus agent/frontend/node_modules, which only `bun install`
# provides: the package tracks bun.lock, not package-lock.json, so `npm ci`
# cannot stand in for it. CI supplies both -- setup-bun at the version
# `tools/pincheck` compares against the images, then `bun install
# --frozen-lockfile` in agent/frontend/ -- so the whole package is checked on
# every push. A workstation with neither bun nor the modules checks the two
# import-free modules the tests above cover -- server/relay.ts and
# server/status.ts -- with the TypeScript it already installed for web/
# (that install is required by check_frontend above), and prints the limit
# instead of passing the package silently.
if command -v bun >/dev/null 2>&1 && [ -d agent/frontend/node_modules ]; then
	echo "-- agent/frontend typecheck (bun run typecheck)"
	(cd agent/frontend && bun run typecheck) || fail "agent/frontend typecheck" "tsc reported problems in agent/frontend/server"
else
	echo "-- agent/frontend typecheck (no bun here, or agent/frontend/node_modules absent: server/relay.ts and server/status.ts only)"
	(cd agent/frontend && ../../web/node_modules/.bin/tsc --noEmit --strict --target ES2022 --module ESNext --moduleResolution Bundler server/relay.ts server/status.ts) ||
		fail "agent/frontend typecheck" "tsc reported problems in agent/frontend/server/relay.ts or server/status.ts"
fi

# --- agents: the two deployments' offline checks and type-checks (T9.9, T9.19, T9.21) ---
# Each package's check runs its own tool bindings against the frozen contract
# examples: the ids the model is offered, their input schemas, the request each
# `execute` sends, the edge's access table, and the memory contract. No model,
# no keys, no external network. A changed tool id, a renamed or retyped field,
# or a tool the edge would refuse fails the gate here.
#
# Both checks need bun plus the package's modules, which only `bun install`
# provides: each package tracks bun.lock, not package-lock.json, so `npm ci`
# cannot stand in. The checks read the package's dependencies through the
# package's own resolution, and they use Bun's runtime (`Bun.file`,
# `import.meta.dir`), so node cannot run them in bun's place. CI installs both
# with `bun install --frozen-lockfile`; a workstation without bun, or without a
# package's modules, prints which check it skipped and why rather than passing
# silently.
#
# After each offline check, the package is type-checked with its own TypeScript
# (`tsc --noEmit -p tsconfig.json`, run from the package so it reads that
# tsconfig): bun transpiles without checking types, so a type error its
# runtime passes over -- an optional string passed where the tool schema
# requires one, say -- would otherwise reach the gate green. It sits inside
# the same guard as the check above, so it is skipped, and said so, only where
# bun or that package's modules are absent.
check_agent() {
	local dir=$1
	local script=$2
	local name=$3

	if ! command -v bun >/dev/null 2>&1; then
		echo "-- $name (skipped: bun is not on PATH; it cannot be run on node)"
		return
	fi
	if [ ! -d "$dir/node_modules" ]; then
		echo "-- $name (skipped: $dir/node_modules is absent; run bun install --frozen-lockfile in $dir)"
		return
	fi

	echo "-- $name (bun $script)"
	run_frontend "$name" bun "$script"

	echo "-- $dir typecheck (tsc --noEmit)"
	(
		cd "$dir"
		run_frontend "$dir typecheck" ./node_modules/.bin/tsc --noEmit -p tsconfig.json
	)
}

check_agent agent/runtime agent/runtime/agent/offline-check.ts "agent/runtime offline check"
check_agent agent/second agent/second/agent/check.ts "agent/second offline check"

echo "OK all checks passed"
