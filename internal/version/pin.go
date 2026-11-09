//go:build tools

// Package version reports what build this binary is.
package version

// The service builds on Keel. This blank import keeps the module requirement
// in go.mod until the first package imports Keel for real. The tools build tag
// keeps it out of the binary and out of the default gate.
import _ "github.com/nrynss/keel/wire"
