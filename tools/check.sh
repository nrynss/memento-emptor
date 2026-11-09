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
go_pkgs=$(printf '%s\n' "$all_pkgs" | grep -v '^github.com/nrynss/memento-emptor/web/' || true)
[ -n "$go_pkgs" ] || fail "go list" "no Go packages found"

echo "-- go vet"
go vet $go_pkgs || fail "go vet" "go vet reported problems"

echo "-- go test -race"
go test -race $go_pkgs || fail "go test -race" "tests failed"

cd web

[ -d node_modules ] || fail "web deps" "web/node_modules is missing; run npm ci in web"

run_web() {
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

echo "-- svelte-check"
run_web "svelte-check" npm run check

echo "-- eslint"
run_web "eslint" npm run lint

echo "-- vitest"
run_web "vitest" npm run test

echo "-- web build"
run_web "web build" npm run build

cd ..

echo "OK all checks passed"
