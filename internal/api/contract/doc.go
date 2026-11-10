// Package contract freezes the memory layer's public surface.
//
// Every agent built on the payment toolkit calls these tools over MCP:
// recall_for_purchase before it recommends, record_correction,
// pending_questions, record_reason and record_feedback as the shopper
// talks, explain_belief when challenged, product_fit for the merchant
// view, set_pooling for the shopper's consent, and the ledger_* reads for
// the evidence ledger. Tool names, wire shapes and the JSON schemas in
// testdata/api are final; the document for builders is docs/interface.md.
//
// Recall never refuses. Every item it returns carries its status label
// (Claimed, Building, Earned, Out of true) and names the precedence step
// that produced it: correction, own evidence, pool, statement, or nothing.
// When steps disagree both are returned and the lower one is marked
// overridden. The representation still carries both sides of a
// contradiction: a contradicted belief keeps its underlying graph
// standing in BeliefView.Standing and reports the ledger status in
// BeliefView.Status, with the warning inline in BeliefView.ContradictedBy
// and the winning correction duplicated at the top of RecallResponse.
package contract
