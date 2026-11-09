module github.com/nrynss/memento-emptor

go 1.27.1

// The service builds on Keel. The first packages to import it arrive with the
// domain model, so the requirement is declared here rather than discovered
// later.
require github.com/nrynss/keel v0.9.0
