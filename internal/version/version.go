// Package version reports what build this binary is.
package version

// Version is the build's version. A release build sets it with the linker:
//
//	go build -ldflags "-X github.com/nrynss/memento-emptor/internal/version.Version=v1.2.3"
//
// A build that sets nothing reports "dev".
var Version = "dev"
