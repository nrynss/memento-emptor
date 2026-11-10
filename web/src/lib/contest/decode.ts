// Runtime decoder for the record_correction response. A response that
// misses its frozen shape fails instead of reaching the ledger as the
// wrong type. The gate feeds both frozen examples through this — the
// declared correction and the standing one with no belief — so
// testdata/api and the contest types cannot drift apart.
import { LedgerApiError, type BeliefStatus, type GraphStanding } from '$lib/api';
import type {
	ContestCorrectionView,
	CorrectionGuidanceBasis,
	RecordCorrectionResponse
} from './types';

const beliefStatuses: readonly BeliefStatus[] = ['Claimed', 'Building', 'Earned', 'Out of true'];
const standings: readonly GraphStanding[] = ['None', 'Candidate', 'Venerable'];
const guidanceBases: readonly CorrectionGuidanceBasis[] = [
	'correction',
	'own_evidence',
	'pool',
	'statement',
	'none'
];

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function fail(what: string): never {
	throw new LedgerApiError('bad-response', `correction response has a bad ${what}`);
}

function stringField(record: Record<string, unknown>, name: string, what: string): string {
	const value = record[name];
	if (typeof value !== 'string') fail(`${what} ${name}`);
	return value as string;
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

function nullableString(record: Record<string, unknown>, name: string, what: string): string | null {
	const value = record[name];
	if (value === null) return null;
	if (typeof value !== 'string') fail(`${what} ${name}`);
	return value;
}

function decodeCorrection(value: unknown): ContestCorrectionView {
	if (!isRecord(value)) fail('correction');
	if (value['takes_precedence'] !== true) fail('correction precedence');
	const against = optionalStringField(value, 'contradicts_belief_id', 'correction');
	return {
		correction_id: stringField(value, 'correction_id', 'correction'),
		text: stringField(value, 'text', 'correction'),
		...(against === undefined ? {} : { contradicts_belief_id: against }),
		recorded_at: stringField(value, 'recorded_at', 'correction'),
		takes_precedence: true
	};
}

/**
 * Decodes a record_correction response. A standing correction carries no
 * belief_id and contradicts nothing yet; a declared one names its belief
 * and carries the warning.
 */
export function decodeRecordCorrectionResponse(value: unknown): RecordCorrectionResponse {
	if (!isRecord(value)) fail('response');
	const status = value['status'];
	if (typeof status !== 'string' || !(beliefStatuses as readonly string[]).includes(status)) {
		fail('status');
	}
	const standing = value['standing'];
	if (typeof standing !== 'string' || !(standings as readonly string[]).includes(standing)) {
		fail('standing');
	}
	if (typeof value['contradicted'] !== 'boolean') fail('warning');
	const guidance = value['guidance'];
	if (!isRecord(guidance)) fail('guidance');
	const basis = guidance['basis'];
	if (typeof basis !== 'string' || !(guidanceBases as readonly string[]).includes(basis)) {
		fail('guidance basis');
	}
	const traceId = value['trace_id'];
	if (traceId !== null && typeof traceId !== 'string') fail('trace id');
	const beliefId = optionalStringField(value, 'belief_id', 'response');
	return {
		correction: decodeCorrection(value['correction']),
		...(beliefId === undefined ? {} : { belief_id: beliefId }),
		status: status as BeliefStatus,
		standing: standing as GraphStanding,
		contradicted: value['contradicted'] as boolean,
		guidance: {
			text: stringField(guidance, 'text', 'guidance'),
			basis: basis as CorrectionGuidanceBasis,
			correction_id: nullableString(guidance, 'correction_id', 'guidance'),
			belief_id: nullableString(guidance, 'belief_id', 'guidance')
		},
		trace_id: traceId
	};
}
