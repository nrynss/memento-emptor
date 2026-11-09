package contract

// ToolName is the MCP tool name for one interface call. Names are frozen:
// agents on the payment toolkit and the ledger code against them.
type ToolName string

// The frozen tool names: six layer calls and three ledger reads.
const (
	// ToolRecallForPurchase recalls the beliefs that apply to a purchase,
	// each with its status and evidence, plus any overriding correction.
	ToolRecallForPurchase ToolName = "recall_for_purchase"
	// ToolRecordCorrection records the shopper's explicit correction at
	// once, declared against the belief it contradicts.
	ToolRecordCorrection ToolName = "record_correction"
	// ToolPendingQuestions lists the refunds still missing a reason.
	ToolPendingQuestions ToolName = "pending_questions"
	// ToolRecordReason records the shopper's answer to a pending
	// question, classified on the layer's side.
	ToolRecordReason ToolName = "record_reason"
	// ToolRecordFeedback records fit feedback tied to a purchase.
	ToolRecordFeedback ToolName = "record_feedback"
	// ToolExplainBelief explains a belief: every piece of evidence and
	// its reason, every correction, the history in order.
	ToolExplainBelief ToolName = "explain_belief"
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
			Description:    "Recall what applies before recommending: the beliefs for this shopper and product, each with status and evidence, plus any shopper correction that overrides them. Call first on every turn that may end in a recommendation; when a correction is present, recommend from it.",
			RequestSchema:  "recall_for_purchase.request.schema.json",
			ResponseSchema: "recall_for_purchase.response.schema.json",
		},
		{
			Name:           ToolRecordCorrection,
			Description:    "Record the shopper's explicit correction at once. The correction takes precedence over the contradicted belief immediately; the belief stays and is shown beside it. The contradicted belief id is required.",
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
			Description:    "Record explicit fit feedback tied to a specific purchase, joined on the transaction's custom id. Counts toward fit as evidence for the worn size.",
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
