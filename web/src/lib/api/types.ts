// The ledger's read surface in TypeScript, mirroring the frozen Go wire
// types in internal/api/contract and the schemas in testdata/api. Field
// names stay snake_case so a frozen example assigns without mapping.

/** What a belief is about: sizing for a brand, or a seller's reliability. */
export type BeliefKind = 'fit' | 'seller_reliability';

/** Which kind a row counts toward, or null when it counts toward nothing. */
export type BeliefKindOrNull = BeliefKind | null;

/** Whether a belief's subject is a brand or a seller. */
export type SubjectKind = 'brand' | 'seller';

/** What the ledger shows for a belief. */
export type BeliefStatus = 'Claimed' | 'Building' | 'Earned' | 'Out of true';

/** The underlying graph state with the contradiction overlay removed. */
export type GraphStanding = 'None' | 'Candidate' | 'Venerable';

/** What happened to the money. */
export type OutcomeKind = 'refund' | 'dispute' | 'capture' | 'kept';

/** Why an outcome happened, or none for a fits-well verdict. */
export type ReasonCategory =
	| 'fit_too_small'
	| 'fit_too_large'
	| 'damaged_or_defective'
	| 'not_arrived'
	| 'changed_mind'
	| 'unclear'
	| 'none';

/** How a signal was written. Only record_action counts toward promotion. */
export type WriteOp = 'record_action' | 'derive' | 'observe';

/** What a belief is about: a brand or a seller. */
export interface SubjectView {
	kind: SubjectKind;
	id: string;
	name: string;
}

/** What happened to the money, kept separate from the item and the reason. */
export interface OutcomeView {
	kind: OutcomeKind;
	status: string;
	amount_minor: number;
	currency: string;
}

/** The purchased thing the evidence is about. */
export interface ItemView {
	brand_id: string;
	brand_name: string;
	line_name: string;
	sku: string;
	size: string;
	category: string;
}

/** Why the outcome happened. Null on an evidence row with no known cause. */
export interface ReasonView {
	category: ReasonCategory;
	text: string;
	confidence: number;
}

/** The join key across order, capture, refund and dispute records. */
export interface TransactionLinkView {
	order_id?: string;
	capture_id?: string;
	refund_id?: string;
	dispute_id?: string;
	custom_id: string;
	event_id: string;
}

/** One unit of memory input with its write operation and its seeded mark. */
export interface EvidenceView {
	evidence_id: string;
	outcome: OutcomeView;
	item: ItemView;
	reason: ReasonView | null;
	counts_toward: BeliefKindOrNull;
	write_op: WriteOp;
	link: TransactionLinkView;
	seeded: boolean;
	occurred_at: string;
}

/** The shopper's explicit correction, declared against one belief. */
export interface CorrectionView {
	correction_id: string;
	text: string;
	contradicts_belief_id: string;
	recorded_at: string;
	takes_precedence: true;
}

/** Which precedence step a belief came from, and whether a stronger step decided. */
export interface BeliefPrecedenceView {
	step: 'correction' | 'own_evidence' | 'pool' | 'statement' | 'none';
	overridden: boolean;
	overridden_by: 'correction' | 'own_evidence' | 'pool' | 'statement' | 'none' | null;
}

/** One belief with its ledger status, underlying standing and warning. */
export interface BeliefView {
	belief_id: string;
	kind: BeliefKind;
	subject: SubjectView;
	value: string;
	status: BeliefStatus;
	standing: GraphStanding;
	contradicted: boolean;
	contradicted_by: CorrectionView | null;
	evidence: EvidenceView[];
	precedence: BeliefPrecedenceView | null;
	seeded: boolean;
	updated_at: string;
}

/** One refund still missing its reason. It counts toward nothing. */
export interface QuestionView {
	question_id: string;
	outcome: OutcomeView;
	item: ItemView;
	link: TransactionLinkView;
	seeded: boolean;
	asked_count: number;
	last_asked_at: string | null;
	awaiting_since: string;
}

/** One ledger row: the full belief plus row metadata. */
export interface LedgerBeliefRow {
	belief: BeliefView;
	evidence_count: number;
	last_change_at: string;
}

/** One ledger evidence row with the beliefs it counts toward. */
export interface LedgerEvidenceRow {
	evidence: EvidenceView;
	belief_ids: string[];
}

/** Lists the ledger's belief rows. Omit status_filter for all rows. */
export interface LedgerBeliefsRequest {
	shopper_id: string;
	status_filter?: BeliefStatus;
	include_seeded: boolean;
}

/** The rows, strongest first. */
export interface LedgerBeliefsResponse {
	shopper_id: string;
	beliefs: LedgerBeliefRow[];
}

/** Reads one ledger row in full. */
export interface LedgerBeliefRequest {
	shopper_id: string;
	belief_id: string;
}

/** The row with its corrections and its seeded notice in words. */
export interface LedgerBeliefResponse {
	belief: BeliefView;
	corrections: CorrectionView[];
	seeded_notice: string | null;
}

/** Lists the ledger's evidence rows, optionally kept to one belief. */
export interface LedgerEvidenceRequest {
	shopper_id: string;
	belief_id?: string;
}

/** The rows, newest first. */
export interface LedgerEvidenceResponse {
	shopper_id: string;
	evidence: LedgerEvidenceRow[];
}

/** Lists the refunds still missing a reason. */
export interface PendingQuestionsRequest {
	shopper_id: string;
	limit?: number;
}

/** The open questions, oldest first. */
export interface PendingQuestionsResponse {
	shopper_id: string;
	questions: QuestionView[];
}

/** The MCP tool names the ledger reads call. */
export const ledgerToolNames = [
	'ledger_beliefs',
	'ledger_belief',
	'ledger_evidence',
	'pending_questions'
] as const;

/** One of the ledger read tool names. */
export type LedgerToolName = (typeof ledgerToolNames)[number];
