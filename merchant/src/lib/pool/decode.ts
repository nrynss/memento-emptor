// Runtime decoders for the product_fit read. A response that misses its
// frozen shape fails with MerchantApiError instead of reaching the grid as
// the wrong type. The gate feeds the frozen example through these, so
// testdata/api and the client interfaces cannot drift apart.

import { MerchantApiError } from './errors';
import type {
	DirectionCountsView,
	FitDirection,
	PoolCountsView,
	PoolPrecedenceView,
	PoolStatus,
	PoolView,
	ProductFitResponse,
	ProductLineView,
	ReasonCountsView,
	ShopperCountsView,
	VerdictCountsView
} from './types';

const poolStatuses: readonly PoolStatus[] = ['Claimed', 'Building', 'Earned', 'Out of true'];
const fitDirections: readonly FitDirection[] = ['small', 'large'];

/** Whether a wire value is one of the two pool directions. */
function isFitDirection(value: string): value is FitDirection {
	return (fitDirections as readonly string[]).includes(value);
}

/** The only fields a product_fit request may carry. Anything else fails. */
export const productFitRequestFields = ['brand_id', 'category', 'fit_handle', 'size'] as const;

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function fail(what: string): never {
	throw new MerchantApiError('bad-response', `product_fit response has a bad ${what}`);
}

function stringField(record: Record<string, unknown>, name: string, what: string): string {
	const value = record[name];
	if (typeof value !== 'string') fail(`${what} ${name}`);
	return value as string;
}

function countField(record: Record<string, unknown>, name: string, what: string): number {
	const value = record[name];
	if (typeof value !== 'number' || !Number.isInteger(value) || value < 0) fail(`${what} ${name}`);
	return value as number;
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

function decodeLine(value: unknown): ProductLineView {
	if (!isRecord(value)) fail('line');
	return {
		brand_id: stringField(value, 'brand_id', 'line'),
		category: stringField(value, 'category', 'line'),
		fit_handle: stringField(value, 'fit_handle', 'line')
	};
}

function decodeDirectionCounts(value: unknown): DirectionCountsView {
	if (!isRecord(value)) fail('returns');
	return {
		too_small: countField(value, 'too_small', 'returns'),
		too_large: countField(value, 'too_large', 'returns')
	};
}

function decodeVerdictCounts(value: unknown): VerdictCountsView {
	if (!isRecord(value)) fail('feedback');
	return {
		fits_well: countField(value, 'fits_well', 'feedback'),
		too_small: countField(value, 'too_small', 'feedback'),
		too_large: countField(value, 'too_large', 'feedback')
	};
}

function decodeReasonCounts(value: unknown): ReasonCountsView {
	if (!isRecord(value)) fail('reasons');
	return {
		fit_too_small: countField(value, 'fit_too_small', 'reasons'),
		fit_too_large: countField(value, 'fit_too_large', 'reasons'),
		damaged_or_defective: countField(value, 'damaged_or_defective', 'reasons'),
		not_arrived: countField(value, 'not_arrived', 'reasons'),
		changed_mind: countField(value, 'changed_mind', 'reasons')
	};
}

function decodeShopperCounts(value: unknown): ShopperCountsView {
	if (!isRecord(value)) fail('shoppers');
	return {
		fit: countField(value, 'fit', 'shoppers'),
		too_small: countField(value, 'too_small', 'shoppers'),
		too_large: countField(value, 'too_large', 'shoppers')
	};
}

function decodeCounts(value: unknown, what: string): PoolCountsView {
	if (!isRecord(value)) fail(what);
	return {
		settled: countField(value, 'settled', what),
		kept: countField(value, 'kept', what),
		returns: decodeDirectionCounts(value['returns']),
		feedback: decodeVerdictCounts(value['feedback']),
		reasons: decodeReasonCounts(value['reasons']),
		shoppers: decodeShopperCounts(value['shoppers'])
	};
}

/** Decodes the pool precedence: required on the wire, null on product_fit. */
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

/** Decodes one pooled product-fit belief: counts, never identities. */
export function decodePool(value: unknown): PoolView {
	if (!isRecord(value)) fail('pool');
	if (!('precedence' in value)) fail('pool precedence');
	const direction = value['direction'];
	if (direction !== null && (typeof direction !== 'string' || !isFitDirection(direction))) {
		fail('pool direction');
	}
	return {
		line: decodeLine(value['line']),
		size: stringField(value, 'size', 'pool'),
		status: enumField(value, 'status', poolStatuses, 'pool'),
		direction: direction as FitDirection | null,
		counts: decodeCounts(value['counts'], 'counts'),
		seeded: decodeCounts(value['seeded'], 'seeded'),
		live: decodeCounts(value['live'], 'live'),
		recent: decodeCounts(value['recent'], 'recent'),
		precedence: decodePoolPrecedence(value['precedence'])
	};
}

/** Decodes a product_fit response: the pooled belief and its counts. */
export function decodeProductFitResponse(value: unknown): ProductFitResponse {
	if (!isRecord(value)) fail('response');
	return { pool: decodePool(value['pool']) };
}
