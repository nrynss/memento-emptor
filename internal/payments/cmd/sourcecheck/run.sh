#!/usr/bin/env bash
# Reads the sandbox's real capture, refund and dispute through the production
# payment source and shows each mapped into the layer's event shapes. Every
# call is a GET; nothing is created, no webhook is sent, no model is called.
#
# The redacted transcript goes to stdout. Real ids stay in the git-ignored
# out/ directory; the driver refuses to finish if one survives redaction.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
source tools/env.sh
go run ./internal/payments/cmd/sourcecheck
