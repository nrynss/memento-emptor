// Runtime decoder for the record_correction response. A response that
// misses its frozen shape fails instead of reaching the ledger as the
// wrong type. The gate feeds the frozen example through this, so
// testdata/api and the contest types cannot drift apart.
import { LedgerApiError, type BeliefStatus, type CorrectionView, type GraphStanding } from '$lib/api';
import type { RecordCorrectionResponse } from './types';

const beliefStatuses: readonly BeliefStatus[] = ['Claimed', 'Building', 'Earned', 'Out of true'];
const standings: readonly GraphStanding[] = ['None', 'Candidate', 'Venerable'];

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

function nullableString(record: Record<string, unknown>, name: string, what: string): string | null {
	const value = record[name];
	if (value === null) return null;
	if (typeof value !== 'string') fail(`${what} ${name}`);
	return value;
}

function decodeCorrection(value: unknown): CorrectionView {
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

/** Decodes a record_correction response. The belief stays contradicted. */
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
	if (value['contradicted'] !== true) fail('warning');
	const guidance = value['guidance'];
	if (!isRecord(guidance)) fail('guidance');
	const basis = guidance['basis'];
	if (basis !== 'correction' && basis !== 'belief' && basis !== 'none') fail('guidance basis');
	const traceId = value['trace_id'];
	if (traceId !== null && typeof traceId !== 'string') fail('trace id');
	return {
		correction: decodeCorrection(value['correction']),
		belief_id: stringField(value, 'belief_id', 'response'),
		status: status as BeliefStatus,
		standing: standing as GraphStanding,
		contradicted: true,
		guidance: {
			text: stringField(guidance, 'text', 'guidance'),
			basis,
			correction_id: nullableString(guidance, 'correction_id', 'guidance'),
			belief_id: nullableString(guidance, 'belief_id', 'guidance')
		},
		trace_id: traceId
	};
}
