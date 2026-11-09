#!/usr/bin/env bash
# T2.2b live run: classify the eleven labelled shopper answers through
# the real classifier service via the frozen client and compare
# category and confidence against the recorded T0.7 table.
#
# The key never enters this file: OPENROUTER_API_KEY comes from
# `source tools/env.sh`. Raw replies land in out/ (git-ignored); the
# transcript is the driver's stdout, kept in the handoff log.
#
# Usage, from the repository root:
#   source tools/env.sh
#   bash tests/live/reasons/run.sh
#
# Nothing here runs in CI and nothing here gates a build: there are no
# *_test.go files under tests/live/reasons/, so the gate builds the
# driver but never executes it.
set -euo pipefail
cd "$(dirname "$0")/../../.."

: "${OPENROUTER_API_KEY:?source tools/env.sh first}"
mkdir -p tests/live/reasons/out
go run ./tests/live/reasons
