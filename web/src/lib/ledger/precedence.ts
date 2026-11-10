// What the layer recommends, and which precedence step decided it. The
// five steps run strongest first: the shopper's correction, their own fit
// evidence, the pooled product fit, their general statement, and nothing.
// When steps disagree both are returned and the lower one is marked
// overridden, never hidden. Field names stay snake_case so the frozen
// recall examples assign without mapping.
import { LedgerApiError, type BeliefView } from '$lib/api';
import { decodePoolView, type PoolView } from './pool';

/** The precedence step that decided what to recommend, strongest first. */
export type GuidanceBasis = 'correction' | 'own_evidence' | 'pool' | 'statement' | 'none';

/** Which step an item came from, and whether a stronger step decided. */
export interface PrecedenceView {
	step: GuidanceBasis;
	overridden: boolean;
	overridden_by: GuidanceBasis | null;
}

/** The shopper's correction as recall returns it, declared or standing. */
export interface RecallCorrectionView {
	correction_id: string;
	text: string;
	contradicts_belief_id?: string;
	recorded_at: string;
	takes_precedence: true;
}

/** One of the shopper's own beliefs, with the step that produced it. */
export interface RecallBelief extends BeliefView {
	precedence?: PrecedenceView | null;
}

/** What the layer states to recommend now, by precedence step. */
export interface RecallGuidance {
	text: string;
	basis: GuidanceBasis;
	correction_id: string | null;
	belief_id: string | null;
}

/** Whether recall consulted the pool index. */
export type PoolLookup = 'consulted' | 'unavailable';

/** What applies to this purchase. It never refuses. */
export interface RecallResponse {
	shopper_id: string;
	beliefs: RecallBelief[];
	pool?: PoolView | null;
	pool_lookup?: PoolLookup;
	overriding_correction: RecallCorrectionView | null;
	guidance: RecallGuidance;
	pending_count: number;
	trace_id: string | null;
}

/** The five precedence steps, strongest first. */
export const guidanceBases: readonly GuidanceBasis[] = [
	'correction',
	'own_evidence',
	'pool',
	'statement',
	'none'
];

/** The plain-words name of a precedence step for the ledger. */
export function stepLabelFor(step: GuidanceBasis): string {
	switch (step) {
		case 'correction':
			return 'Your correction';
		case 'own_evidence':
			return 'Your own fit evidence';
		case 'pool':
			return 'Pooled product fit';
		case 'statement':
			return 'Your general statement';
		case 'none':
			return 'Nothing yet';
		default:
			throw new LedgerApiError('bad-response', 'recall response has a bad precedence step');
	}
}

/** One overridden step beside the winner. */
export interface OverriddenStep {
	step: GuidanceBasis;
	winner: GuidanceBasis;
}

/** One line saying which step decided, for the recommendation header. */
export function decisionLineFor(guidance: RecallGuidance): string {
	if (guidance.basis === 'none') {
		return 'No memory applies yet: the layer says so plainly.';
	}
	return `Decided by ${stepLabelFor(guidance.basis).toLowerCase()}.`;
}

/** One line for an overridden item beside the one that won. */
export function overriddenLineFor(step: GuidanceBasis, winner: GuidanceBasis): string {
	return `${stepLabelFor(step)} was overridden by ${stepLabelFor(winner).toLowerCase()}.`;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function fail(what: string): never {
	throw new LedgerApiError('bad-response', `recall response has a bad ${what}`);
}

function stringField(record: Record<string, unknown>, name: string, what: string): string {
	const value = record[name];
	if (typeof value !== 'string') fail(`${what} ${name}`);
	return value as string;
}

function nullableString(record: Record<string, unknown>, name: string, what: string): string | null {
	const value = record[name];
	if (value === null) return null;
	if (typeof value !== 'string') fail(`${what} ${name}`);
	return value;
}

function optionalStringField(
	record: Record<string, unknown>,
	name: string,
	what: string
): string | undefined {
	const value = record[name];
	if (value === undefined) return undefined;
	if (typeof value !== 'string') fail(`${what} ${name}`);
	return value;
}

function basisField(record: Record<string, unknown>, name: string, what: string): GuidanceBasis {
	const value = record[name];
	if (typeof value !== 'string' || !(guidanceBases as readonly string[]).includes(value)) {
		fail(`${what} ${name}`);
	}
	return value as GuidanceBasis;
}

/** Decodes one precedence view: the step and the stronger winner, if any. */
export function decodePrecedence(value: unknown): PrecedenceView | null {
	if (value === null || value === undefined) return null;
	if (!isRecord(value)) fail('precedence');
	if (typeof value['overridden'] !== 'boolean') fail('precedence flag');
	const winner = value['overridden_by'];
	if (winner !== null && (typeof winner !== 'string' || !guidanceBases.includes(winner as GuidanceBasis))) {
		fail('precedence winner');
	}
	return {
		step: basisField(value, 'step', 'precedence'),
		overridden: value['overridden'] as boolean,
		overridden_by: winner as GuidanceBasis | null
	};
}

function decodeRecallCorrection(value: unknown): RecallCorrectionView | null {
	if (value === null) return null;
	if (!isRecord(value)) fail('recall correction');
	if (value['takes_precedence'] !== true) fail('recall correction precedence');
	const against = optionalStringField(value, 'contradicts_belief_id', 'recall correction');
	return {
		correction_id: stringField(value, 'correction_id', 'recall correction'),
		text: stringField(value, 'text', 'recall correction'),
		...(against === undefined ? {} : { contradicts_belief_id: against }),
		recorded_at: stringField(value, 'recorded_at', 'recall correction'),
		takes_precedence: true
	};
}

function decodeRecallGuidance(value: unknown): RecallGuidance {
	if (!isRecord(value)) fail('recall guidance');
	return {
		text: stringField(value, 'text', 'recall guidance'),
		basis: basisField(value, 'basis', 'recall guidance'),
		correction_id: nullableString(value, 'correction_id', 'recall guidance'),
		belief_id: nullableString(value, 'belief_id', 'recall guidance')
	};
}

/** Decodes one recall_for_purchase response with its precedence steps. */
export function decodeRecallResponse(value: unknown): RecallResponse {
	if (!isRecord(value)) fail('recall response');
	const beliefs = value['beliefs'];
	if (!Array.isArray(beliefs)) fail('recall beliefs');
	const pending = value['pending_count'];
	if (typeof pending !== 'number' || !Number.isInteger(pending) || pending < 0) {
		fail('recall pending count');
	}
	const lookup = value['pool_lookup'];
	if (
		lookup !== undefined &&
		(lookup === null || (typeof lookup !== 'string' && typeof lookup !== 'undefined'))
	) {
		fail('recall pool lookup');
	}
	if (lookup !== undefined && lookup !== 'consulted' && lookup !== 'unavailable') {
		fail('recall pool lookup');
	}
	const traceId = value['trace_id'];
	if (traceId !== null && traceId !== undefined && typeof traceId !== 'string') {
		fail('recall trace id');
	}
	return {
		shopper_id: stringField(value, 'shopper_id', 'recall response'),
		beliefs: beliefs.map((belief) => {
			if (!isRecord(belief)) fail('recall belief');
			const precedence = 'precedence' in belief ? decodePrecedence(belief['precedence']) : null;
			const rest: Record<string, unknown> = { ...belief };
			delete rest['precedence'];
			return { ...(rest as unknown as BeliefView), precedence };
		}),
		...('pool' in value
			? { pool: value['pool'] === null || value['pool'] === undefined ? null : decodePoolView(value['pool']) }
			: {}),
		...(lookup === undefined ? {} : { pool_lookup: lookup as PoolLookup }),
		overriding_correction: decodeRecallCorrection(value['overriding_correction']),
		guidance: decodeRecallGuidance(value['guidance']),
		pending_count: pending,
		trace_id: (traceId ?? null) as string | null
	};
}
