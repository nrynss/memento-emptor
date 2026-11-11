// The pooled product fit in TypeScript, mirroring the pivoted product_fit
// shapes in testdata/api and the Pool def in common.schema.json. Field
// names stay snake_case so the frozen example assigns without mapping. A
// pool carries counts only: no shopper identity, no row, no free text.
import { LedgerApiError } from '$lib/api';

/** The product line the pool is about: brand, category and cut. */
export interface PoolLineView {
	brand_id: string;
	category: string;
	fit_handle: string;
}

/** Settled returns by the direction they point. */
export interface PoolDirectionCounts {
	too_small: number;
	too_large: number;
}

/** Settled fit feedback by verdict. */
export interface PoolVerdictCounts {
	fits_well: number;
	too_small: number;
	too_large: number;
}

/** Settled returns by reason category: the merchant's "why it comes back". */
export interface PoolReasonCounts {
	fit_too_small: number;
	fit_too_large: number;
	damaged_or_defective: number;
	not_arrived: number;
	changed_mind: number;
}

/** Distinct shoppers, never identities. */
export interface PoolShopperCounts {
	fit: number;
	too_small: number;
	too_large: number;
}

/** The settled counts for one product line and size, seeded and live. */
export interface PoolCountsView {
	settled: number;
	kept: number;
	returns: PoolDirectionCounts;
	feedback: PoolVerdictCounts;
	reasons: PoolReasonCounts;
	shoppers: PoolShopperCounts;
}

/** The direction a pooled label rates, or null when none leads. */
export type PoolDirection = 'small' | 'large';

/** The pooled product-fit belief for one product line and one size. */
export interface PoolView {
	line: PoolLineView;
	size: string;
	status: 'Claimed' | 'Building' | 'Earned' | 'Out of true';
	direction: PoolDirection | null;
	counts: PoolCountsView;
	seeded: PoolCountsView;
	live: PoolCountsView;
	recent: PoolCountsView;
	precedence: PoolPrecedenceView | null;
}

/** Which precedence step a pool came from, and whether a stronger step won. */
export interface PoolPrecedenceView {
	step: 'correction' | 'own_evidence' | 'pool' | 'statement' | 'none';
	overridden: boolean;
	overridden_by: 'correction' | 'own_evidence' | 'pool' | 'statement' | 'none' | null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function fail(what: string): never {
	throw new LedgerApiError('bad-response', `pool response has a bad ${what}`);
}

function countField(record: Record<string, unknown>, name: string, what: string): number {
	const value = record[name];
	if (typeof value !== 'number' || !Number.isInteger(value) || value < 0) fail(`${what} ${name}`);
	return value;
}

function stringField(record: Record<string, unknown>, name: string, what: string): string {
	const value = record[name];
	if (typeof value !== 'string') fail(`${what} ${name}`);
	return value as string;
}

function decodeDirectionCounts(value: unknown): PoolDirectionCounts {
	if (!isRecord(value)) fail('pool returns');
	return {
		too_small: countField(value, 'too_small', 'pool returns'),
		too_large: countField(value, 'too_large', 'pool returns')
	};
}

function decodeVerdictCounts(value: unknown): PoolVerdictCounts {
	if (!isRecord(value)) fail('pool feedback');
	return {
		fits_well: countField(value, 'fits_well', 'pool feedback'),
		too_small: countField(value, 'too_small', 'pool feedback'),
		too_large: countField(value, 'too_large', 'pool feedback')
	};
}

function decodeReasonCounts(value: unknown): PoolReasonCounts {
	if (!isRecord(value)) fail('pool reasons');
	return {
		fit_too_small: countField(value, 'fit_too_small', 'pool reasons'),
		fit_too_large: countField(value, 'fit_too_large', 'pool reasons'),
		damaged_or_defective: countField(value, 'damaged_or_defective', 'pool reasons'),
		not_arrived: countField(value, 'not_arrived', 'pool reasons'),
		changed_mind: countField(value, 'changed_mind', 'pool reasons')
	};
}

function decodeShopperCounts(value: unknown): PoolShopperCounts {
	if (!isRecord(value)) fail('pool shoppers');
	return {
		fit: countField(value, 'fit', 'pool shoppers'),
		too_small: countField(value, 'too_small', 'pool shoppers'),
		too_large: countField(value, 'too_large', 'pool shoppers')
	};
}

/** Decodes one settled-counts block. Counts only; no identity to decode. */
export function decodePoolCounts(value: unknown): PoolCountsView {
	if (!isRecord(value)) fail('pool counts');
	return {
		settled: countField(value, 'settled', 'pool counts'),
		kept: countField(value, 'kept', 'pool counts'),
		returns: decodeDirectionCounts(value['returns']),
		feedback: decodeVerdictCounts(value['feedback']),
		reasons: decodeReasonCounts(value['reasons']),
		shoppers: decodeShopperCounts(value['shoppers'])
	};
}

const poolStatuses = ['Claimed', 'Building', 'Earned', 'Out of true'] as const;
const poolDirections = ['small', 'large'] as const;

function decodePoolPrecedence(value: unknown): PoolPrecedenceView | null {
	if (value === null || value === undefined) return null;
	if (!isRecord(value)) fail('pool precedence');
	const step = value['step'];
	const overriddenBy = value['overridden_by'];
	const steps: readonly string[] = ['correction', 'own_evidence', 'pool', 'statement', 'none'];
	if (typeof step !== 'string' || !steps.includes(step)) fail('pool precedence step');
	if (typeof value['overridden'] !== 'boolean') fail('pool precedence flag');
	if (overriddenBy !== null && (typeof overriddenBy !== 'string' || !steps.includes(overriddenBy))) {
		fail('pool precedence winner');
	}
	return {
		step: step as PoolPrecedenceView['step'],
		overridden: value['overridden'] as boolean,
		overridden_by: overriddenBy as PoolPrecedenceView['overridden_by']
	};
}

/** Decodes one pooled product-fit belief: counts, never shopper rows. */
export function decodePoolView(value: unknown): PoolView {
	if (!isRecord(value)) fail('pool');
	if (!('precedence' in value)) fail('pool precedence');
	const line = value['line'];
	if (!isRecord(line)) fail('pool line');
	const direction = value['direction'];
	if (
		direction !== null &&
		(typeof direction !== 'string' || !(poolDirections as readonly string[]).includes(direction))
	) {
		fail('pool direction');
	}
	const status = value['status'];
	if (typeof status !== 'string' || !(poolStatuses as readonly string[]).includes(status)) {
		fail('pool status');
	}
	return {
		line: {
			brand_id: stringField(line, 'brand_id', 'pool line'),
			category: stringField(line, 'category', 'pool line'),
			fit_handle: stringField(line, 'fit_handle', 'pool line')
		},
		size: stringField(value, 'size', 'pool'),
		status: status as PoolView['status'],
		direction: direction as PoolDirection | null,
		counts: decodePoolCounts(value['counts']),
		seeded: decodePoolCounts(value['seeded']),
		live: decodePoolCounts(value['live']),
		recent: decodePoolCounts(value['recent']),
		precedence: decodePoolPrecedence(value['precedence'])
	};
}

/** Decodes a product_fit response: the pool with its counts behind it. */
export function decodeProductFitResponse(value: unknown): { pool: PoolView } {
	if (!isRecord(value)) fail('product_fit response');
	return { pool: decodePoolView(value['pool']) };
}

/** The direction word the ledger shows: "small" or "large", or null. */
export function directionWordFor(pool: PoolView): string | null {
	return pool.direction;
}

/**
 * One counts-only line for the pooled row: settled denominator, returns
 * and feedback by direction, distinct shoppers. No reason text, no
 * shopper, no row.
 */
export function poolSummaryLine(pool: PoolView): string {
	const counts = pool.counts;
	const direction = pool.direction === null ? 'no direction' : `runs ${pool.direction}`;
	return (
		`${pool.status}: ${direction} at size ${pool.size} — ` +
		`${String(counts.settled)} settled purchases, ` +
		`${String(counts.returns.too_small)} too-small returns, ` +
		`${String(counts.returns.too_large)} too-large returns, ` +
		`${String(counts.feedback.fits_well)} fits-well, ` +
		`from ${String(counts.shoppers.fit)} distinct shoppers.`
	);
}

/** The seeded split in words, so the ledger shows the generated share. */
export function poolSeededLine(pool: PoolView): string {
	return (
		`${String(pool.seeded.settled)} of the ${String(pool.counts.settled)} ` +
		`settled purchases are seeded history.`
	);
}
