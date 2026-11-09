// Package contract freezes the memory layer's public surface.
//
// Every agent built on the payment toolkit calls these tools over MCP:
// recall_for_purchase before it recommends, record_correction,
// pending_questions, record_reason and record_feedback as the shopper
// talks, explain_belief when challenged, and the ledger_* reads for the
// evidence ledger. Tool names, wire shapes and the JSON schemas in
// testdata/api are final; the document for builders is docs/interface.md.
//
// The representation carries both sides of a contradiction. A contradicted
// belief keeps its underlying graph standing (None, Candidate or
// Venerable) in BeliefView.Standing and reports the ledger status
// (Out of true) in BeliefView.Status, with the warning inline in
// BeliefView.ContradictedBy and the winning correction duplicated at the
// top of RecallResponse. An agent cannot read the belief without seeing
// the override.
package contract
