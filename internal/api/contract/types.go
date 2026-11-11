package contract

import "github.com/nrynss/memento-emptor/internal/domain"

// FeedbackVerdict is the layer's reading of explicit fit feedback tied to
// a specific purchase. It is interface-local: the domain's reason
// categories describe why an outcome happened, while a verdict describes
// how the purchased size worked out, including the fits-well case the
// categories have no word for.
type FeedbackVerdict string

// The feedback verdicts.
const (
	// FeedbackFitsWell means the purchased size fits. It counts toward
	// fit as supporting evidence for the worn size.
	FeedbackFitsWell FeedbackVerdict = "fits_well"
	// FeedbackTooSmall means the purchased size was too small or too
	// tight. It counts toward fit.
	FeedbackTooSmall FeedbackVerdict = "too_small"
	// FeedbackTooLarge means the purchased size was too big or too
	// loose. It counts toward fit.
	FeedbackTooLarge FeedbackVerdict = "too_large"
	// FeedbackDamagedOrDefective means the item arrived broken or did not
	// work. It counts toward seller reliability, never toward sizing.
	FeedbackDamagedOrDefective FeedbackVerdict = "damaged_or_defective"
	// FeedbackNotArrived means the item never arrived. It counts toward
	// seller reliability, never toward sizing.
	FeedbackNotArrived FeedbackVerdict = "not_arrived"
	// FeedbackChangedMind means the shopper changed their mind with no
	// product fault. It counts toward nothing.
	FeedbackChangedMind FeedbackVerdict = "changed_mind"
	// FeedbackUnclear means no usable verdict. It counts toward nothing
	// and the agent asks again.
	FeedbackUnclear FeedbackVerdict = "unclear"
)

// FeedbackVerdicts lists every feedback verdict.
var FeedbackVerdicts = []FeedbackVerdict{
	FeedbackFitsWell,
	FeedbackTooSmall,
	FeedbackTooLarge,
	FeedbackDamagedOrDefective,
	FeedbackNotArrived,
	FeedbackChangedMind,
	FeedbackUnclear,
}

// TimelineKind names one row of a belief's history.
type TimelineKind string

// The timeline row kinds.
const (
	// TimelineEvidence is a structural write: a classified return,
	// dispute or fit feedback linked to its transaction.
	TimelineEvidence TimelineKind = "evidence"
	// TimelineCorrection is the shopper's explicit correction, declared
	// against the belief it contradicts.
	TimelineCorrection TimelineKind = "correction"
	// TimelineFeedback is fit feedback tied to a specific purchase.
	TimelineFeedback TimelineKind = "feedback"
	// TimelineStatus is a status change on the belief itself.
	TimelineStatus TimelineKind = "status"
)

// GuidanceBasis names the precedence step that decided what to recommend.
// The steps are ordered, strongest first: a correction wins over the
// shopper's own evidence, which wins over the pooled product fit, which
// wins over a general statement; with none of them, nothing applies.
type GuidanceBasis string

// The precedence steps, strongest first.
const (
	// GuidanceCorrection means the shopper's correction decides. The agent
	// recommends from it, immediately.
	GuidanceCorrection GuidanceBasis = "correction"
	// GuidanceOwnEvidence means the shopper's own fit evidence decides, at
	// any status: fit is personal, so it beats the pool.
	GuidanceOwnEvidence GuidanceBasis = "own_evidence"
	// GuidanceBelief is the name the served layer was written against; it
	// is the own-evidence step.
	GuidanceBelief = GuidanceOwnEvidence
	// GuidancePool means the pooled product fit decides, adjusted from the
	// shopper's usual size.
	GuidancePool GuidanceBasis = "pool"
	// GuidanceStatement means the shopper's general statement decides. It
	// is Claimed and never promotes.
	GuidanceStatement GuidanceBasis = "statement"
	// GuidanceNone means nothing applies yet, and the layer says so.
	GuidanceNone GuidanceBasis = "none"
)

// GuidanceBases lists the precedence steps, strongest first.
var GuidanceBases = []GuidanceBasis{
	GuidanceCorrection,
	GuidanceOwnEvidence,
	GuidancePool,
	GuidanceStatement,
	GuidanceNone,
}

// Rank is the step's place in the precedence order: a lower rank is
// stronger. An unknown step ranks zero and never wins.
func (b GuidanceBasis) Rank() int {
	switch b {
	case GuidanceCorrection:
		return 1
	case GuidanceOwnEvidence:
		return 2
	case GuidancePool:
		return 3
	case GuidanceStatement:
		return 4
	case GuidanceNone:
		return 5
	default:
		return 0
	}
}

// PrecedenceView says which step an item came from and whether a stronger
// step decided the purchase instead. An overridden item is still returned,
// so the agent and the ledger show both sides rather than hide one.
type PrecedenceView struct {
	// Step is the precedence step the item came from.
	Step GuidanceBasis `json:"step"`
	// Overridden is true when a stronger step decided. The item is still
	// returned and never sets the recommendation.
	Overridden bool `json:"overridden"`
	// OverriddenBy names the stronger step, or null when this item decided.
	OverriddenBy *GuidanceBasis `json:"overridden_by"`
}

// OutcomeView is the wire form of domain.Outcome: what happened to the
// money, kept separate from the item and the reason.
type OutcomeView struct {
	// Kind is what happened: refund, dispute, capture or kept.
	Kind domain.OutcomeKind `json:"kind"`
	// Status is the source's status for the outcome, such as COMPLETED.
	Status string `json:"status"`
	// AmountMinor is the amount in minor units, such as cents.
	AmountMinor int64 `json:"amount_minor"`
	// Currency is the ISO currency code, such as USD.
	Currency string `json:"currency"`
}

// ItemView is the wire form of domain.Item: the purchased thing the
// evidence is about.
type ItemView struct {
	// BrandID identifies the brand. Fit beliefs are keyed per brand.
	BrandID string `json:"brand_id"`
	// BrandName is the brand's display name.
	BrandName string `json:"brand_name"`
	// LineName is the product line, such as the oxford shirt line.
	LineName string `json:"line_name"`
	// SKU is the retailer's stock-keeping unit for the worn variant.
	SKU string `json:"sku"`
	// Size is the purchased size, such as M.
	Size string `json:"size"`
	// Category is the product category slug, such as shirts.
	Category string `json:"category"`
}

// ReasonView is the wire form of domain.Reason: why the outcome happened.
// A nil reason on an EvidenceView means the outcome has no known cause
// yet and counts toward nothing.
type ReasonView struct {
	// Category is the classified reason.
	Category domain.Category `json:"category"`
	// Text is the shopper's words, or the dispute code for a dispute.
	Text string `json:"text"`
	// Confidence is the classifier's confidence, from 0 to 1.
	Confidence float64 `json:"confidence"`
}

// TransactionLinkView is the wire form of domain.TransactionLink. The
// custom ID propagates across order, capture, refund and dispute records
// and is the join key; the event ID keys webhook idempotency.
type TransactionLinkView struct {
	// OrderID is the checkout order id.
	OrderID string `json:"order_id,omitempty"`
	// CaptureID is the capture id the refund or dispute refers to.
	CaptureID string `json:"capture_id,omitempty"`
	// RefundID is the refund id, when the outcome is a refund.
	RefundID string `json:"refund_id,omitempty"`
	// DisputeID is the dispute id, when the outcome is a dispute.
	DisputeID string `json:"dispute_id,omitempty"`
	// CustomID is the shopper-scoped reference across the records.
	CustomID string `json:"custom_id"`
	// EventID is the webhook event id, for idempotent ingest.
	EventID string `json:"event_id"`
}

// EvidenceView is one unit of memory input with its write operation and
// its seeded mark, so the ledger can show what changed and why.
type EvidenceView struct {
	// EvidenceID identifies the evidence row.
	EvidenceID string `json:"evidence_id"`
	// Outcome is what happened to the money.
	Outcome OutcomeView `json:"outcome"`
	// Item is the purchased thing the evidence is about.
	Item ItemView `json:"item"`
	// Reason is why it happened, or null when no reason is known yet.
	Reason *ReasonView `json:"reason"`
	// CountsToward names which belief kind this evidence counts toward,
	// or null when it counts toward nothing.
	CountsToward *domain.BeliefKind `json:"counts_toward"`
	// WriteOp is how the signal was written: only record_action counts
	// as evidence toward promotion.
	WriteOp domain.WriteOp `json:"write_op"`
	// Link ties the outcome to its transaction.
	Link TransactionLinkView `json:"link"`
	// Seeded marks generated history. Every seeded row carries the mark.
	Seeded bool `json:"seeded"`
	// OccurredAt is when the evidence happened, RFC 3339, in event time
	// so seeded history counts as spread out.
	OccurredAt string `json:"occurred_at"`
}

// CorrectionView is the shopper's explicit correction. It applies at once
// with precedence over the belief it contradicts, and the belief itself
// is never deleted. A correction with no belief yet stands on its own: it
// carries no against declaration until a matching belief appears.
type CorrectionView struct {
	// CorrectionID identifies the correction.
	CorrectionID string `json:"correction_id"`
	// Text is the shopper's verbatim first-person words.
	Text string `json:"text"`
	// ContradictsBeliefID is the against declaration: the belief this
	// correction contradicts. It is absent while the correction stands
	// on its own.
	ContradictsBeliefID string `json:"contradicts_belief_id,omitempty"`
	// RecordedAt is when the correction was recorded, RFC 3339.
	RecordedAt string `json:"recorded_at"`
	// TakesPrecedence is always true. It marks the override so an agent
	// cannot read past it.
	TakesPrecedence bool `json:"takes_precedence"`
}

// BeliefView is what recall returns for one belief: the ledger status for
// display, the underlying graph standing without the contradiction
// overlay, and the warning inline. A contradicted belief reports status
// Out of true while standing stays Venerable: both travel together.
type BeliefView struct {
	// BeliefID identifies the belief.
	BeliefID string `json:"belief_id"`
	// Kind is whether the belief is about fit or seller reliability.
	Kind domain.BeliefKind `json:"kind"`
	// Subject is the brand or seller the belief is about.
	Subject domain.Subject `json:"subject"`
	// Value states the belief in words, such as runs small, take size L.
	Value string `json:"value"`
	// Status is the ledger status. It reads Out of true while a
	// correction applies.
	Status domain.BeliefStatus `json:"status"`
	// Standing is the underlying graph state with the overlay removed:
	// None, Candidate or Venerable. It never reads contradicted; that
	// overlay is the Contradicted flag beside it.
	Standing domain.GraphState `json:"standing"`
	// Contradicted is true while a correction applies to this belief.
	Contradicted bool `json:"contradicted"`
	// ContradictedBy carries the correction that applies to this belief,
	// or null when the belief is not contradicted. Two beliefs for one
	// brand may each carry a different correction here, so a second
	// correction is never hidden behind the first.
	ContradictedBy *CorrectionView `json:"contradicted_by"`
	// Evidence is every piece of evidence that counts toward this belief.
	Evidence []EvidenceView `json:"evidence"`
	// Precedence names the precedence step that produced the belief and
	// whether a stronger step decided instead. It is always present:
	// recall sets it, and the ledger reads carry null, because they show
	// the belief rather than the decision that served it.
	Precedence *PrecedenceView `json:"precedence"`
	// Seeded marks generated history. Every seeded row carries the mark.
	Seeded bool `json:"seeded"`
	// UpdatedAt is the last change to the belief, RFC 3339.
	UpdatedAt string `json:"updated_at"`
}

// GuidanceView tells the agent what to recommend from right now. The
// decision follows the shopper immediately: a correction wins over any
// belief without waiting for new evidence, and Basis says which precedence
// step decided.
type GuidanceView struct {
	// Text states what to recommend now, in words.
	Text string `json:"text"`
	// Basis names the precedence step the text comes from: correction,
	// own_evidence, pool, statement, or none.
	Basis GuidanceBasis `json:"basis"`
	// CorrectionID identifies the winning correction, or null.
	CorrectionID *string `json:"correction_id"`
	// BeliefID identifies the belief that decided, or null. It is null for
	// a pool decision, which names its line and size on the pool itself.
	BeliefID *string `json:"belief_id"`
}

// QuestionView is one refund still missing its reason. The agent asks the
// shopper; the answer goes to record_reason. No code path turns an
// unanswered question into a belief.
type QuestionView struct {
	// QuestionID identifies the pending question.
	QuestionID string `json:"question_id"`
	// Outcome is the refund awaiting its reason.
	Outcome OutcomeView `json:"outcome"`
	// Item is the purchased thing the refund concerns.
	Item ItemView `json:"item"`
	// Link ties the refund to its transaction.
	Link TransactionLinkView `json:"link"`
	// Seeded marks generated history. Every seeded row carries the mark.
	Seeded bool `json:"seeded"`
	// AskedCount is how many times the agent has asked already.
	AskedCount int `json:"asked_count"`
	// LastAskedAt is the last time the agent asked, RFC 3339, or null
	// when the shopper has never been asked.
	LastAskedAt *string `json:"last_asked_at"`
	// AwaitingSince is when the refund arrived without a reason, RFC 3339.
	AwaitingSince string `json:"awaiting_since"`
}

// TimelineEvent is one row of a belief's history for explain_belief.
type TimelineEvent struct {
	// At is when the row happened, RFC 3339.
	At string `json:"at"`
	// Kind names the row: evidence, correction, feedback or status.
	Kind TimelineKind `json:"kind"`
	// Summary states the row in one line.
	Summary string `json:"summary"`
	// RefID identifies the evidence, correction or feedback behind the row.
	RefID string `json:"ref_id"`
}

// LedgerBeliefRow is one ledger row: the full belief plus row metadata.
type LedgerBeliefRow struct {
	// Belief is the belief with its status, standing and warning.
	Belief BeliefView `json:"belief"`
	// EvidenceCount is how many evidence rows count toward the belief.
	EvidenceCount int `json:"evidence_count"`
	// LastChangeAt is the last change to the row, RFC 3339.
	LastChangeAt string `json:"last_change_at"`
}

// LedgerEvidenceRow is one ledger evidence row with its belief links.
type LedgerEvidenceRow struct {
	// Evidence is the evidence with its reason and seeded mark.
	Evidence EvidenceView `json:"evidence"`
	// BeliefIDs identifies the beliefs this row counts toward. It is
	// empty when the row counts toward nothing.
	BeliefIDs []string `json:"belief_ids"`
}

// RecallRequest asks what applies before the agent recommends. The agent
// calls it first on every turn that may end in a recommendation, so the
// trace shows recall before the recommendation.
type RecallRequest struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// BrandID identifies the brand under consideration.
	BrandID string `json:"brand_id"`
	// LineName is the product line under consideration, when known.
	LineName string `json:"line_name,omitempty"`
	// Category is the product category slug under consideration, when
	// known. With the brand and the fit handle it names the product line
	// the pool is keyed by.
	Category string `json:"category,omitempty"`
	// FitHandle is the candidate line's cut, when known.
	FitHandle string `json:"fit_handle,omitempty"`
	// SKU is the candidate variant under consideration, when known.
	SKU string `json:"sku,omitempty"`
	// SizeConsidered is the size the agent is leaning toward, when known.
	SizeConsidered string `json:"size_considered,omitempty"`
	// TraceID is the host trace for this turn, so per-trace review can
	// show recall ran first. It is omitted when the host has no trace.
	TraceID string `json:"trace_id,omitempty"`
}

// RecallResponse returns what applies to this purchase. It never refuses:
// thin evidence comes back labelled thin. Beliefs are the shopper's own,
// each carrying the precedence step that produced it; Pool is the pooled
// product fit when the pool has something to say; Guidance names the step
// that decided. Every item carries its status label, and when steps
// disagree both are returned with the lower marked overridden. When
// OverridingCorrection is present the agent recommends from it first: it
// ships both at the top level and inside the belief it contradicts, so it
// cannot be read past. One brand can hold two beliefs, one each way, and
// the shopper can correct both: the top level then carries the newer
// correction and each belief carries its own inline, so every
// contradiction stays overridable and neither belief is dropped.
type RecallResponse struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// Beliefs are the shopper's own beliefs that apply, each with its
	// status, evidence and precedence step.
	Beliefs []BeliefView `json:"beliefs"`
	// Pool is the pooled product-fit belief for the product, or null
	// when the pool was not consulted or has nothing to say. It is
	// always present: an absent pool is null, never a missing key.
	Pool *PoolView `json:"pool"`
	// PoolLookup says whether recall consulted the pool: consulted when the
	// index answered, with Pool null when it had nothing to say;
	// unavailable when nothing could be consulted — no index is wired, the
	// request named no full line and size, or the index could not be
	// reached. It is always present. Recall carries on without a pool and
	// never pretends an empty one.
	PoolLookup PoolLookup `json:"pool_lookup"`
	// OverridingCorrection is the correction the agent must honour
	// first, or null when none applies. When more than one applies it
	// is the newer of them, and each contradicted belief carries its
	// own inline beside it.
	OverridingCorrection *CorrectionView `json:"overriding_correction"`
	// Guidance states what to recommend now, by precedence step.
	Guidance GuidanceView `json:"guidance"`
	// PendingCount is how many refunds still miss a reason. Above zero
	// means the agent should ask, via pending_questions.
	PendingCount int `json:"pending_count"`
	// TraceID echoes the request's trace id, or null.
	TraceID *string `json:"trace_id"`
}

// RecordCorrectionRequest records the shopper's explicit, first-person
// correction. ContradictsBeliefID is optional: with a belief the
// correction is declared against it at once, and with none it is kept as a
// standing record that never promotes. When a matching belief appears
// later, the layer declares the standing correction against it then.
type RecordCorrectionRequest struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// Text is the shopper's verbatim first-person words.
	Text string `json:"text"`
	// ContradictsBeliefID is the against declaration: the belief this
	// correction contradicts. It is omitted for a standing correction.
	ContradictsBeliefID string `json:"contradicts_belief_id,omitempty"`
	// TraceID is the host trace for this turn, when the host has one.
	TraceID string `json:"trace_id,omitempty"`
}

// RecordCorrectionResponse confirms the correction applies at once. With a
// belief, the contradicted belief keeps its status and standing and is
// shown beside the correction, never deleted. With no belief, the
// correction stands alone: there is no belief to report.
type RecordCorrectionResponse struct {
	// Correction is the recorded correction.
	Correction CorrectionView `json:"correction"`
	// BeliefID identifies the contradicted belief. It is absent while the
	// correction stands on its own.
	BeliefID string `json:"belief_id,omitempty"`
	// Status is the belief's ledger status, unchanged. A standing
	// correction reports its own label, Claimed.
	Status domain.BeliefStatus `json:"status"`
	// Standing is the belief's underlying graph state, unchanged. A
	// standing correction reports None.
	Standing domain.GraphState `json:"standing"`
	// Contradicted is true when a belief carries the warning now. It is
	// false for a standing correction, which contradicts no belief yet.
	Contradicted bool `json:"contradicted"`
	// Guidance states what to recommend now, from the correction.
	Guidance GuidanceView `json:"guidance"`
	// TraceID echoes the request's trace id, or null.
	TraceID *string `json:"trace_id"`
}

// PendingQuestionsRequest lists the refunds still missing a reason.
type PendingQuestionsRequest struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// Limit caps the questions returned. It is omitted for no cap.
	Limit int `json:"limit,omitempty"`
}

// PendingQuestionsResponse returns the open questions, oldest first.
type PendingQuestionsResponse struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// Questions are the refunds awaiting their reason.
	Questions []QuestionView `json:"questions"`
}

// RecordReasonRequest records the shopper's answer to one pending
// question. Classification happens on the layer's side; a vague answer
// or a low-confidence classification stays pending and the agent asks
// again.
type RecordReasonRequest struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// QuestionID identifies the pending question being answered.
	QuestionID string `json:"question_id"`
	// AnswerText is the shopper's verbatim answer.
	AnswerText string `json:"answer_text"`
	// TraceID is the host trace for this turn, when the host has one.
	TraceID string `json:"trace_id,omitempty"`
}

// RecordReasonResponse returns the classification. When Accepted is
// false the answer counted toward nothing: either AskAgain is true and
// the question stays in pending_questions (unclear or below-floor
// answer), or the classified reason matched no belief direction and
// the question closes.
type RecordReasonResponse struct {
	// ReasonID identifies the recorded reason.
	ReasonID string `json:"reason_id"`
	// QuestionID identifies the answered question.
	QuestionID string `json:"question_id"`
	// Category is the classified reason.
	Category domain.Category `json:"category"`
	// Confidence is the classifier's confidence, from 0 to 1.
	Confidence float64 `json:"confidence"`
	// CountsToward names which belief kind the answer counts toward, or
	// null when it counts toward nothing.
	CountsToward *domain.BeliefKind `json:"counts_toward"`
	// Accepted is true when the answer became evidence.
	Accepted bool `json:"accepted"`
	// AskAgain is true when the agent must ask the shopper again.
	AskAgain bool `json:"ask_again"`
	// BeliefID identifies the belief the answer now supports, or null.
	BeliefID *string `json:"belief_id"`
	// TraceID echoes the request's trace id, or null.
	TraceID *string `json:"trace_id"`
}

// RecordFeedbackRequest records explicit fit feedback tied to a specific
// purchase, joined on the custom ID that propagates across the order,
// capture, refund and dispute records.
type RecordFeedbackRequest struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// CustomID joins the feedback to its transaction.
	CustomID string `json:"custom_id"`
	// SKU is the purchased variant the feedback concerns.
	SKU string `json:"sku"`
	// Size is the purchased size the feedback concerns.
	Size string `json:"size"`
	// FeedbackText is the shopper's verbatim words.
	FeedbackText string `json:"feedback_text"`
	// TraceID is the host trace for this turn, when the host has one.
	TraceID string `json:"trace_id,omitempty"`
}

// RecordFeedbackResponse returns the verdict. A fits-well verdict counts
// toward fit as supporting evidence for the worn size; an unclear
// verdict counts toward nothing and the agent asks again.
type RecordFeedbackResponse struct {
	// FeedbackID identifies the recorded feedback.
	FeedbackID string `json:"feedback_id"`
	// Verdict is the layer's reading of the feedback.
	Verdict FeedbackVerdict `json:"verdict"`
	// CountsToward names which belief kind the feedback counts toward,
	// or null when it counts toward nothing.
	CountsToward *domain.BeliefKind `json:"counts_toward"`
	// BeliefID identifies the belief the feedback now supports, or null.
	BeliefID *string `json:"belief_id"`
	// Accepted is true when the feedback became evidence.
	Accepted bool `json:"accepted"`
	// TraceID echoes the request's trace id, or null.
	TraceID *string `json:"trace_id"`
}

// ExplainBeliefRequest asks why the layer believes something.
type ExplainBeliefRequest struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// BeliefID identifies the belief to explain.
	BeliefID string `json:"belief_id"`
}

// ExplainBeliefResponse answers with every piece of evidence and its
// reason, every correction, and the history in order.
type ExplainBeliefResponse struct {
	// Belief is the belief with its status, standing and warning.
	Belief BeliefView `json:"belief"`
	// Corrections are the corrections declared against the belief,
	// newest first. It is empty when none applies.
	Corrections []CorrectionView `json:"corrections"`
	// Timeline is the belief's history in order.
	Timeline []TimelineEvent `json:"timeline"`
}

// LedgerBeliefsRequest lists the ledger's belief rows.
type LedgerBeliefsRequest struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// StatusFilter keeps only rows at this ledger status. It is omitted
	// for all rows.
	StatusFilter domain.BeliefStatus `json:"status_filter,omitempty"`
	// IncludeSeeded sets whether seeded rows are included. The ledger
	// shows seeded rows with their mark; the flag exists for the
	// comparison harness, which must not read hidden answers through it.
	IncludeSeeded bool `json:"include_seeded"`
}

// LedgerBeliefsResponse returns the rows, strongest first.
type LedgerBeliefsResponse struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// Beliefs are the ledger rows.
	Beliefs []LedgerBeliefRow `json:"beliefs"`
}

// LedgerBeliefRequest reads one ledger row in full.
type LedgerBeliefRequest struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// BeliefID identifies the belief.
	BeliefID string `json:"belief_id"`
}

// LedgerBeliefResponse returns the row with its corrections and a seeded
// notice. SeededNotice carries the seeded mark in words when the belief
// is seeded, and is null otherwise.
type LedgerBeliefResponse struct {
	// Belief is the belief with its status, standing and warning.
	Belief BeliefView `json:"belief"`
	// Corrections are the corrections declared against the belief,
	// newest first. It is empty when none applies.
	Corrections []CorrectionView `json:"corrections"`
	// SeededNotice marks generated history, or null for lived rows.
	SeededNotice *string `json:"seeded_notice"`
}

// LedgerEvidenceRequest lists the ledger's evidence rows, optionally kept
// to one belief.
type LedgerEvidenceRequest struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// BeliefID keeps only rows counting toward this belief. It is
	// omitted for all rows.
	BeliefID string `json:"belief_id,omitempty"`
}

// LedgerEvidenceResponse returns the rows, newest first.
type LedgerEvidenceResponse struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// Evidence are the evidence rows with their belief links.
	Evidence []LedgerEvidenceRow `json:"evidence"`
}

// PoolLookup names whether recall consulted the pool.
type PoolLookup string

// The pool lookup outcomes.
const (
	// PoolConsulted means the pool index answered. The Pool object is
	// present when the pool has something to say and absent when it does
	// not.
	PoolConsulted PoolLookup = "consulted"
	// PoolUnavailable means nothing could be consulted: no pool index is
	// wired, the request named no full line and size, or the index could
	// not be reached. Recall carries on without it and never pretends an
	// empty pool.
	PoolUnavailable PoolLookup = "unavailable"
)

// ProductLineView is the wire form of domain.ProductLine: the brand, the
// category and the fit handle that name one product line together.
type ProductLineView struct {
	// BrandID identifies the brand.
	BrandID string `json:"brand_id"`
	// Category is the product category slug, such as shirts.
	Category string `json:"category"`
	// FitHandle is the line's cut, such as regular or slim.
	FitHandle string `json:"fit_handle"`
}

// DirectionCountsView counts settled returns by the direction they point.
type DirectionCountsView struct {
	// TooSmall counts settled returns whose reason says the item ran small.
	TooSmall int `json:"too_small"`
	// TooLarge counts settled returns whose reason says it ran large.
	TooLarge int `json:"too_large"`
}

// VerdictCountsView counts settled fit feedback by verdict.
type VerdictCountsView struct {
	// FitsWell counts feedback saying the size fits.
	FitsWell int `json:"fits_well"`
	// TooSmall counts feedback saying the size was too small.
	TooSmall int `json:"too_small"`
	// TooLarge counts feedback saying the size was too large.
	TooLarge int `json:"too_large"`
}

// ReasonCountsView counts settled returns by reason category. It is the
// merchant's "why it comes back": a fit reason is a sizing cause, and the
// other categories are not.
type ReasonCountsView struct {
	// FitTooSmall counts returns whose reason is too small.
	FitTooSmall int `json:"fit_too_small"`
	// FitTooLarge counts returns whose reason is too large.
	FitTooLarge int `json:"fit_too_large"`
	// DamagedOrDefective counts returns of a broken or defective item.
	DamagedOrDefective int `json:"damaged_or_defective"`
	// NotArrived counts returns of an item that never arrived.
	NotArrived int `json:"not_arrived"`
	// ChangedMind counts returns with no product fault.
	ChangedMind int `json:"changed_mind"`
}

// ShopperCountsView counts distinct shoppers, never identities. Fit is the
// Building floor and is not the sum of the directions: a shopper who
// returned one size each way counts once.
type ShopperCountsView struct {
	// Fit counts distinct shoppers with any fit-classified outcome.
	Fit int `json:"fit"`
	// TooSmall counts distinct shoppers in the too-small direction.
	TooSmall int `json:"too_small"`
	// TooLarge counts distinct shoppers in the too-large direction.
	TooLarge int `json:"too_large"`
}

// PoolCountsView holds the settled counts for one product line and size.
// Only settled purchases count: a purchase still inside its window, and a
// refund with no reason yet, count toward nothing.
type PoolCountsView struct {
	// Settled is the denominator: every settled purchase in the line and
	// size, including returns with a non-fit reason and kept purchases.
	Settled int `json:"settled"`
	// Kept counts purchases kept past the return window with no feedback.
	Kept int `json:"kept"`
	// Returns counts settled returns by direction.
	Returns DirectionCountsView `json:"returns"`
	// Feedback counts settled feedback by verdict.
	Feedback VerdictCountsView `json:"feedback"`
	// Reasons counts settled returns by reason category.
	Reasons ReasonCountsView `json:"reasons"`
	// Shoppers counts distinct shoppers, never identities.
	Shoppers ShopperCountsView `json:"shoppers"`
}

// PoolView is the pooled product-fit belief for one product line and one
// size: its label, the direction it rates, and the settled counts behind
// it. It carries no shopper identity, no row and no free text.
type PoolView struct {
	// Line is the product line the pool is about.
	Line ProductLineView `json:"line"`
	// Size is the size the counts are for: the shopper's usual size when
	// one is known, otherwise the size under consideration.
	Size string `json:"size"`
	// Status is the pooled label, in the ledger's words.
	Status domain.BeliefStatus `json:"status"`
	// Direction is the direction the label rates, or null when no
	// direction leads.
	Direction *domain.FitDirection `json:"direction"`
	// Counts is every settled purchase in the line and size, seeded and
	// live.
	Counts PoolCountsView `json:"counts"`
	// Seeded is the seeded part of the counts. Every seeded row carries
	// the mark, and the ledger shows the split.
	Seeded PoolCountsView `json:"seeded"`
	// Live is the lived part of the counts.
	Live PoolCountsView `json:"live"`
	// Recent is the last 90 days of event time alone, the window the Out
	// of true rule reads.
	Recent PoolCountsView `json:"recent"`
	// Precedence names the precedence step when recall returns the pool,
	// and whether a stronger step decided instead. It is always present:
	// recall sets it, and product_fit carries null, because the merchant
	// read shows the pool rather than the decision that served it.
	Precedence *PrecedenceView `json:"precedence"`
}

// ProductFitRequest asks for the pooled product fit of one product line and
// size. The merchant app reads this and nothing else.
type ProductFitRequest struct {
	// BrandID identifies the brand.
	BrandID string `json:"brand_id"`
	// Category is the product category slug, such as shirts.
	Category string `json:"category"`
	// FitHandle is the line's cut, such as regular or slim.
	FitHandle string `json:"fit_handle"`
	// Size is the size the counts are for.
	Size string `json:"size"`
}

// ProductFitResponse is the pooled belief with its status and the counts
// behind it.
type ProductFitResponse struct {
	// Pool is the pooled product-fit belief for the line and size.
	Pool PoolView `json:"pool"`
}

// SetPoolingRequest records the shopper's pooling choice. Pooling is
// opt-out: a shopper's classified outcomes count toward the pool unless
// they say no. Pooling false opts out and removes the rows already
// written; Pooling true opts back in.
type SetPoolingRequest struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// Pooling is the shopper's choice: true keeps their outcomes in the
	// pool, false opts out.
	Pooling bool `json:"pooling"`
	// TraceID is the host trace for this turn, when the host has one.
	TraceID string `json:"trace_id,omitempty"`
}

// SetPoolingResponse confirms the choice and says how many existing fit
// rows opting out removed. Opting back in removes nothing.
type SetPoolingResponse struct {
	// ShopperID identifies the shopper.
	ShopperID string `json:"shopper_id"`
	// Pooling is the resulting choice.
	Pooling bool `json:"pooling"`
	// RowsRemoved is how many fit rows the change removed.
	RowsRemoved int `json:"rows_removed"`
	// TraceID echoes the request's trace id, or null.
	TraceID *string `json:"trace_id"`
}
