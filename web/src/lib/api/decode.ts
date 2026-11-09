// Runtime decoders for the ledger reads. A response that misses its
// frozen shape fails with LedgerApiError instead of reaching the ledger
// page as the wrong type. The gate feeds every frozen example through
// these, so testdata/api and the client interfaces cannot drift apart.

import { LedgerApiError } from './errors';
import type {
	BeliefKind,
	BeliefKindOrNull,
	BeliefStatus,
	BeliefView,
	CorrectionView,
	EvidenceView,
	GraphStanding,
	ItemView,
	LedgerBeliefResponse,
	LedgerBeliefsResponse,
	LedgerEvidenceResponse,
	LedgerEvidenceRow,
	OutcomeKind,
	OutcomeView,
	PendingQuestionsResponse,
	QuestionView,
	ReasonCategory,
	ReasonView,
	SubjectKind,
	TransactionLinkView,
	WriteOp
} from './types';

const beliefKinds: readonly BeliefKind[] = ['fit', 'seller_reliability'];
const beliefStatuses: readonly BeliefStatus[] = ['Claimed', 'Building', 'Earned', 'Out of true'];
const standings: readonly GraphStanding[] = ['None', 'Candidate', 'Venerable'];
const subjectKinds: readonly SubjectKind[] = ['brand', 'seller'];
const outcomeKinds: readonly OutcomeKind[] = ['refund', 'dispute', 'capture', 'kept'];
const reasonCategories: readonly ReasonCategory[] = [
	'fit_too_small',
	'fit_too_large',
	'damaged_or_defective',
	'not_arrived',
	'changed_mind',
	'unclear',
	'none'
];
const writeOps: readonly WriteOp[] = ['record_action', 'derive', 'observe'];

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function fail(what: string): never {
	throw new LedgerApiError('bad-response', `ledger response has a bad ${what}`);
}

function stringField(record: Record<string, unknown>, name: string, what: string): string {
	const value = record[name];
	if (typeof value !== 'string') fail(`${what} ${name}`);
	return value as string;
}

function optionalString(record: Record<string, unknown>, name: string, what: string): string | undefined {
	const value = record[name];
	if (value === undefined) return undefined;
	if (typeof value !== 'string') fail(`${what} ${name}`);
	return value;
}

function enumField<T extends string>(
	record: Record<string, unknown>,
	name: string,
	allowed: readonly T[],
	what: string
): T {
	const value = record[name];
	if (typeof value !== 'string' || !(allowed as readonly string[]).includes(value)) {
		fail(`${what} ${name}`);
	}
	return value as T;
}

function decodeOutcome(value: unknown): OutcomeView {
	if (!isRecord(value)) fail('outcome');
	return {
		kind: enumField(value, 'kind', outcomeKinds, 'outcome'),
		status: stringField(value, 'status', 'outcome'),
		amount_minor: typeof value['amount_minor'] === 'number' ? value['amount_minor'] : fail('outcome amount'),
		currency: stringField(value, 'currency', 'outcome')
	};
}

function decodeItem(value: unknown): ItemView {
	if (!isRecord(value)) fail('item');
	return {
		brand_id: stringField(value, 'brand_id', 'item'),
		brand_name: stringField(value, 'brand_name', 'item'),
		line_name: stringField(value, 'line_name', 'item'),
		sku: stringField(value, 'sku', 'item'),
		size: stringField(value, 'size', 'item'),
		category: stringField(value, 'category', 'item')
	};
}

function decodeReason(value: unknown): ReasonView | null {
	if (value === null) return null;
	if (!isRecord(value)) fail('reason');
	return {
		category: enumField(value, 'category', reasonCategories, 'reason'),
		text: stringField(value, 'text', 'reason'),
		confidence: typeof value['confidence'] === 'number' ? value['confidence'] : fail('reason confidence')
	};
}

function decodeCountsToward(value: unknown): BeliefKindOrNull {
	if (value === null) return null;
	if (typeof value !== 'string' || !(beliefKinds as readonly string[]).includes(value)) {
		fail('counts_toward');
	}
	return value as BeliefKind;
}

function decodeLink(value: unknown): TransactionLinkView {
	if (!isRecord(value)) fail('link');
	const link: TransactionLinkView = {
		custom_id: stringField(value, 'custom_id', 'link'),
		event_id: stringField(value, 'event_id', 'link')
	};
	for (const name of ['order_id', 'capture_id', 'refund_id', 'dispute_id'] as const) {
		const optional = optionalString(value, name, 'link');
		if (optional !== undefined) link[name] = optional;
	}
	return link;
}

function decodeEvidence(value: unknown): EvidenceView {
	if (!isRecord(value)) fail('evidence');
	const countsToward = value['counts_toward'];
	const writeOp = value['write_op'];
	if (typeof writeOp !== 'string' || !(writeOps as readonly string[]).includes(writeOp)) {
		fail('evidence write_op');
	}
	return {
		evidence_id: stringField(value, 'evidence_id', 'evidence'),
		outcome: decodeOutcome(value['outcome']),
		item: decodeItem(value['item']),
		reason: decodeReason(value['reason']),
		counts_toward: decodeCountsToward(countsToward),
		write_op: writeOp as WriteOp,
		link: decodeLink(value['link']),
		seeded: typeof value['seeded'] === 'boolean' ? value['seeded'] : fail('evidence seeded'),
		occurred_at: stringField(value, 'occurred_at', 'evidence')
	};
}

function decodeCorrection(value: unknown): CorrectionView | null {
	if (value === null) return null;
	if (!isRecord(value)) fail('correction');
	if (value['takes_precedence'] !== true) fail('correction precedence');
	return {
		correction_id: stringField(value, 'correction_id', 'correction'),
		text: stringField(value, 'text', 'correction'),
		contradicts_belief_id: stringField(value, 'contradicts_belief_id', 'correction'),
		recorded_at: stringField(value, 'recorded_at', 'correction'),
		takes_precedence: true
	};
}

function decodeBelief(value: unknown): BeliefView {
	if (!isRecord(value)) fail('belief');
	const evidence = value['evidence'];
	if (!Array.isArray(evidence)) fail('belief evidence');
	return {
		belief_id: stringField(value, 'belief_id', 'belief'),
		kind: enumField(value, 'kind', beliefKinds, 'belief'),
		subject: {
			kind: enumField(
				isRecord(value['subject']) ? value['subject'] : fail('belief subject'),
				'kind',
				subjectKinds,
				'subject'
			),
			id: stringField(
				isRecord(value['subject']) ? value['subject'] : fail('belief subject'),
				'id',
				'subject'
			),
			name: stringField(
				isRecord(value['subject']) ? value['subject'] : fail('belief subject'),
				'name',
				'subject'
			)
		},
		value: stringField(value, 'value', 'belief'),
		status: enumField(value, 'status', beliefStatuses, 'belief'),
		standing: enumField(value, 'standing', standings, 'belief'),
		contradicted:
			typeof value['contradicted'] === 'boolean' ? value['contradicted'] : fail('belief warning'),
		contradicted_by: decodeCorrection(value['contradicted_by']),
		evidence: evidence.map(decodeEvidence),
		seeded: typeof value['seeded'] === 'boolean' ? value['seeded'] : fail('belief seeded'),
		updated_at: stringField(value, 'updated_at', 'belief')
	};
}

function decodeShopperId(value: unknown): string {
	if (!isRecord(value) || typeof value['shopper_id'] !== 'string') fail('shopper');
	return value['shopper_id'];
}

/** Decodes a ledger_beliefs response, strongest row first. */
export function decodeLedgerBeliefsResponse(value: unknown): LedgerBeliefsResponse {
	if (!isRecord(value) || !Array.isArray(value['beliefs'])) fail('belief rows');
	return {
		shopper_id: decodeShopperId(value),
		beliefs: (value['beliefs'] as unknown[]).map((row) => {
			if (!isRecord(row)) fail('belief row');
			const evidenceCount = row['evidence_count'];
			if (typeof evidenceCount !== 'number') fail('belief row count');
			return {
				belief: decodeBelief(row['belief']),
				evidence_count: evidenceCount,
				last_change_at: stringField(row, 'last_change_at', 'belief row')
			};
		})
	};
}

/** Decodes a ledger_belief response with its corrections and seeded notice. */
export function decodeLedgerBeliefResponse(value: unknown): LedgerBeliefResponse {
	if (!isRecord(value) || !Array.isArray(value['corrections'])) fail('belief row');
	const notice = value['seeded_notice'];
	if (notice !== null && typeof notice !== 'string') fail('seeded notice');
	return {
		belief: decodeBelief(value['belief']),
		corrections: (value['corrections'] as unknown[]).map((correction) => {
			const decoded = decodeCorrection(correction);
			if (decoded === null) fail('correction row');
			return decoded;
		}),
		seeded_notice: notice
	};
}

/** Decodes a ledger_evidence response, newest row first. */
export function decodeLedgerEvidenceResponse(value: unknown): LedgerEvidenceResponse {
	if (!isRecord(value) || !Array.isArray(value['evidence'])) fail('evidence rows');
	return {
		shopper_id: decodeShopperId(value),
		evidence: (value['evidence'] as unknown[]).map((row): LedgerEvidenceRow => {
			if (!isRecord(row) || !Array.isArray(row['belief_ids'])) fail('evidence row');
			return {
				evidence: decodeEvidence(row['evidence']),
				belief_ids: (row['belief_ids'] as unknown[]).map((id) => {
					if (typeof id !== 'string') fail('evidence row link');
					return id;
				})
			};
		})
	};
}

/** Decodes a pending_questions response, oldest question first. */
export function decodePendingQuestionsResponse(value: unknown): PendingQuestionsResponse {
	if (!isRecord(value) || !Array.isArray(value['questions'])) fail('questions');
	return {
		shopper_id: decodeShopperId(value),
		questions: (value['questions'] as unknown[]).map((question): QuestionView => {
			if (!isRecord(question)) fail('question');
			const lastAsked = question['last_asked_at'];
			if (lastAsked !== null && typeof lastAsked !== 'string') fail('question last asked');
			return {
				question_id: stringField(question, 'question_id', 'question'),
				outcome: decodeOutcome(question['outcome']),
				item: decodeItem(question['item']),
				link: decodeLink(question['link']),
				seeded:
					typeof question['seeded'] === 'boolean' ? question['seeded'] : fail('question seeded'),
				asked_count:
					typeof question['asked_count'] === 'number'
						? question['asked_count']
						: fail('question asked count'),
				last_asked_at: lastAsked,
				awaiting_since: stringField(question, 'awaiting_since', 'question')
			};
		})
	};
}
