// Package standing persists one shopper's layer-owned state across
// restarts: the pooling choice, the correction views recall serves, and
// the standing-corrections journal that declares a standing correction
// against a matching belief when the belief is written later.
//
// The graph cannot rebuild the journal: a correction's subject, line and
// size live only in the journal, never in the graph, and the seam offers
// no enumeration of a session's concepts. So the layer keeps its own
// store, one file per shopper, written after every mutation and read when
// the shopper's partition is created. Only undeclared standings travel in
// the journal: a declared correction is history, and re-declaring it
// after a restart would write a duplicate against edge for a belief the
// restart already forgot.
package standing
