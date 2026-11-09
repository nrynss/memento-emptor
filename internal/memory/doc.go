// Package memory earns beliefs in the evidence graph.
//
// It applies the domain's write rule through the Graph seam: each reasoned
// outcome is its own structural write citing its belief in depends_on,
// corrections are declared with against while the belief stays standing,
// general statements and kept-past-window observations go through derive
// as notes that never promote, and evidence whose reason matches no belief
// kind writes nothing to any belief. ReadBelief maps a belief's graph
// state to the ledger's four statuses through the frozen round trip, with
// the contradiction overlay deciding Out of true while the underlying
// standing is kept beside it.
package memory
