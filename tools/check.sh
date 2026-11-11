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

# The Go and Node pins live in six places: both images' FROM lines, CI's
# setup-go and setup-node, go.mod, and deploy/README.md's table. An image
# built on a toolchain CI does not use builds green in both places, so the
# pins are compared here, first in the gate. tools/pincheck's own table
# test proves the disagreement it catches; `go test` below runs it.
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
# cannot stand in for it, and CI installs neither. Where both are present the
# whole package is checked. Where they are not, the gate type-checks the
# module the tests above cover with the TypeScript it already installed for
# web/ (that install is required by check_frontend above), and prints the
# limit instead of passing the package silently.
if command -v bun >/dev/null 2>&1 && [ -d agent/frontend/node_modules ]; then
	echo "-- agent/frontend typecheck (bun run typecheck)"
	(cd agent/frontend && bun run typecheck) || fail "agent/frontend typecheck" "tsc reported problems in agent/frontend/server"
else
	echo "-- agent/frontend typecheck (bun or agent/frontend/node_modules absent: server/relay.ts only)"
	(cd agent/frontend && ../../web/node_modules/.bin/tsc --noEmit --strict --target ES2022 --module ESNext --moduleResolution Bundler server/relay.ts) ||
		fail "agent/frontend typecheck" "tsc reported problems in agent/frontend/server/relay.ts"
fi

echo "OK all checks passed"
