package contract

// ToolName is the MCP tool name for one interface call. Names are frozen:
// agents on the payment toolkit and the ledger code against them.
type ToolName string

// The frozen tool names: eight layer calls and three ledger reads.
const (
	// ToolRecallForPurchase recalls what applies to a purchase: the
	// shopper's beliefs, the pooled fit, any correction, each with its
	// status and precedence step.
	ToolRecallForPurchase ToolName = "recall_for_purchase"
	// ToolRecordCorrection records the shopper's explicit correction at
	// once, against a belief or as a standing record.
	ToolRecordCorrection ToolName = "record_correction"
	// ToolPendingQuestions lists the refunds still missing a reason.
	ToolPendingQuestions ToolName = "pending_questions"
	// ToolRecordReason records the shopper's answer to a pending
	// question, classified on the layer's side.
	ToolRecordReason ToolName = "record_reason"
	// ToolRecordFeedback records explicit feedback tied to a purchase: a
	// fit verdict counts toward fit, a seller verdict toward seller
	// reliability, and unclear or changed-mind toward nothing. A repeat
	// answers the recorded verdict and id; a different verdict is refused.
	ToolRecordFeedback ToolName = "record_feedback"
	// ToolExplainBelief explains a belief: every piece of evidence and
	// its reason, every correction, the history in order.
	ToolExplainBelief ToolName = "explain_belief"
	// ToolProductFit reads the pooled product fit for one product line and
	// size: its status and the counts behind it, never a shopper row.
	ToolProductFit ToolName = "product_fit"
	// ToolSetPooling records the shopper's opt-out of pooling, or opts
	// them back in.
	ToolSetPooling ToolName = "set_pooling"
	// ToolLedgerBeliefs lists the ledger's belief rows.
	ToolLedgerBeliefs ToolName = "ledger_beliefs"
	// ToolLedgerBelief reads one ledger row in full.
	ToolLedgerBelief ToolName = "ledger_belief"
	// ToolLedgerEvidence lists the ledger's evidence rows.
	ToolLedgerEvidence ToolName = "ledger_evidence"
)

// ToolNames lists every frozen tool name.
var ToolNames = []ToolName{
	ToolRecallForPurchase,
	ToolRecordCorrection,
	ToolPendingQuestions,
	ToolRecordReason,
	ToolRecordFeedback,
	ToolExplainBelief,
	ToolProductFit,
	ToolSetPooling,
	ToolLedgerBeliefs,
	ToolLedgerBelief,
	ToolLedgerEvidence,
}

// ToolDescriptor describes one MCP tool: its frozen name, the short
// description an agent reads on tools/list, and the schema files its
// request and response validate against in testdata/api.
type ToolDescriptor struct {
	// Name is the frozen MCP tool name.
	Name ToolName
	// Description is the short text an agent reads before calling.
	Description string
	// RequestSchema is the request schema file in testdata/api.
	RequestSchema string
	// ResponseSchema is the response schema file in testdata/api.
	ResponseSchema string
}

// Tools returns the descriptor for every frozen tool, in call order.
func Tools() []ToolDescriptor {
	return []ToolDescriptor{
		{
			Name:           ToolRecallForPurchase,
			Description:    "Recall what applies before recommending. It never refuses: the shopper's beliefs and the pooled product fit come back each with a status label, thin evidence included. Guidance names the precedence step that decided (correction, own_evidence, pool, statement, none); when steps disagree both are returned with the lower marked overridden. When a correction is present, recommend from it.",
			RequestSchema:  "recall_for_purchase.request.schema.json",
			ResponseSchema: "recall_for_purchase.response.schema.json",
		},
		{
			Name:           ToolRecordCorrection,
			Description:    "Record the shopper's explicit correction at once. With a belief it takes precedence over it immediately and the belief stays beside the warning; with no belief it is kept as a standing record that contradicts nothing yet and is declared against a matching belief when one appears.",
			RequestSchema:  "record_correction.request.schema.json",
			ResponseSchema: "record_correction.response.schema.json",
		},
		{
			Name:           ToolPendingQuestions,
			Description:    "List the refunds still missing a reason, oldest first. Ask the shopper about the first one; send the answer to record_reason. An unanswered refund counts toward nothing.",
			RequestSchema:  "pending_questions.request.schema.json",
			ResponseSchema: "pending_questions.response.schema.json",
		},
		{
			Name:           ToolRecordReason,
			Description:    "Record the shopper's answer to a pending question. The layer classifies it; when the answer is vague or confidence is low the question stays open and the response asks again.",
			RequestSchema:  "record_reason.request.schema.json",
			ResponseSchema: "record_reason.response.schema.json",
		},
		{
			Name:           ToolRecordFeedback,
			Description:    "Record explicit feedback tied to a specific purchase, joined on the transaction's custom id. A fit verdict (fits_well, too_small, too_large) counts toward fit as evidence for the worn size; a seller verdict (damaged_or_defective, not_arrived) counts toward seller reliability, never toward fit; unclear and changed_mind count toward nothing. A repeat of the recorded verdict answers that verdict and feedback id and writes nothing; a different verdict for a purchase already holding one is refused.",
			RequestSchema:  "record_feedback.request.schema.json",
			ResponseSchema: "record_feedback.response.schema.json",
		},
		{
			Name:           ToolExplainBelief,
			Description:    "Explain why the layer believes something: every piece of evidence with its reason, every correction, and the history in order. Cite this, not the raw transaction history, when justifying a recommendation.",
			RequestSchema:  "explain_belief.request.schema.json",
			ResponseSchema: "explain_belief.response.schema.json",
		},
		{
			Name:           ToolProductFit,
			Description:    "Read the pooled product fit for one product line and size: its status, the direction it rates, settled counts by direction, feedback by verdict and reason by category, distinct shoppers by direction, and the seeded split. Carries no shopper identity, no row and no free text.",
			RequestSchema:  "product_fit.request.schema.json",
			ResponseSchema: "product_fit.response.schema.json",
		},
		{
			Name:           ToolSetPooling,
			Description:    "Record the shopper's pooling choice. Pooling is opt-out: their classified outcomes count toward the pool unless they say no. Opting out stops new fit rows and removes the rows already written, and the response says how many.",
			RequestSchema:  "set_pooling.request.schema.json",
			ResponseSchema: "set_pooling.response.schema.json",
		},
		{
			Name:           ToolLedgerBeliefs,
			Description:    "List the ledger's belief rows: every belief with its ledger status, underlying standing, contradiction warning, seeded mark and evidence count.",
			RequestSchema:  "ledger_beliefs.request.schema.json",
			ResponseSchema: "ledger_beliefs.response.schema.json",
		},
		{
			Name:           ToolLedgerBelief,
			Description:    "Read one ledger belief row in full, with its corrections and its seeded notice.",
			RequestSchema:  "ledger_belief.request.schema.json",
			ResponseSchema: "ledger_belief.response.schema.json",
		},
		{
			Name:           ToolLedgerEvidence,
			Description:    "List the ledger's evidence rows: outcomes with their reasons, what each counts toward, seeded marks and belief links.",
			RequestSchema:  "ledger_evidence.request.schema.json",
			ResponseSchema: "ledger_evidence.response.schema.json",
		},
	}
}
