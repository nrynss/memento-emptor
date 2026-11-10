// Fixture mode: the sandbox merchant's lines, replayed from the frozen
// contract snapshots. The Earned demo oxford is byte-equal in meaning to
// testdata/api; the other five lines are built so the pool rule in rule.ts
// decides the status each one serves, which the gate checks. Unknown tools,
// lines and requests fail instead of inventing an answer. No fixture row
// names a shopper: the pool carries counts, never identities.

import { checkProductFitRequest, type MerchantTransport } from './client';
import { decodeProductFitResponse } from './decode';
import { MerchantApiError } from './errors';
import earnedExample from '../../../../testdata/api/product_fit.response.example.json';
import { lineKey, merchantLines } from './lines';
import type { MerchantToolName, PoolView, ProductFitRequest, ProductFitResponse } from './types';

/** The Earned demo oxford, exactly as the frozen snapshot serves it. */
export const fixtureEarnedOxford: ProductFitResponse = decodeProductFitResponse(earnedExample);

function zeroCounts(): ProductFitResponse['pool']['counts'] {
	return {
		settled: 0,
		kept: 0,
		returns: { too_small: 0, too_large: 0 },
		feedback: { fits_well: 0, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 0, fit_too_large: 0, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 0, too_small: 0, too_large: 0 }
	};
}

/** One hand-built fixture row: the served status matches the rule's decide. */
function makeRow(pool: PoolView): ProductFitResponse {
	return decodeProductFitResponse({ pool });
}

/** The L size of the demo oxford: Building, rating large. */
const fixtureOxfordLarge: ProductFitResponse = makeRow({
	line: { brand_id: 'brand-x-001', category: 'shirts', fit_handle: 'regular' },
	size: 'L',
	status: 'Building',
	direction: 'large',
	counts: {
		settled: 5,
		kept: 1,
		returns: { too_small: 0, too_large: 4 },
		feedback: { fits_well: 0, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 0, fit_too_large: 4, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 4, too_small: 0, too_large: 4 }
	},
	seeded: {
		settled: 5,
		kept: 1,
		returns: { too_small: 0, too_large: 4 },
		feedback: { fits_well: 0, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 0, fit_too_large: 4, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 4, too_small: 0, too_large: 4 }
	},
	live: zeroCounts(),
	recent: {
		settled: 5,
		kept: 1,
		returns: { too_small: 0, too_large: 4 },
		feedback: { fits_well: 0, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 0, fit_too_large: 4, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 4, too_small: 0, too_large: 4 }
	}
});

/** The slim line: Building, rating small. */
const fixtureSlim: ProductFitResponse = makeRow({
	line: { brand_id: 'brand-y-002', category: 'shirts', fit_handle: 'slim' },
	size: 'M',
	status: 'Building',
	direction: 'small',
	counts: {
		settled: 8,
		kept: 0,
		returns: { too_small: 6, too_large: 1 },
		feedback: { fits_well: 1, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 6, fit_too_large: 1, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 7, too_small: 6, too_large: 1 }
	},
	seeded: {
		settled: 6,
		kept: 0,
		returns: { too_small: 5, too_large: 1 },
		feedback: { fits_well: 0, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 5, fit_too_large: 1, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 6, too_small: 5, too_large: 1 }
	},
	live: {
		settled: 2,
		kept: 0,
		returns: { too_small: 1, too_large: 0 },
		feedback: { fits_well: 1, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 1, fit_too_large: 0, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 1, too_small: 1, too_large: 0 }
	},
	recent: {
		settled: 8,
		kept: 0,
		returns: { too_small: 6, too_large: 1 },
		feedback: { fits_well: 1, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 6, fit_too_large: 1, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 7, too_small: 6, too_large: 1 }
	}
});

/** The second oxford line: Building, diluted by fits-well feedback. */
const fixtureSecondOxford: ProductFitResponse = makeRow({
	line: { brand_id: 'brand-z-003', category: 'shirts', fit_handle: 'regular' },
	size: 'M',
	status: 'Building',
	direction: 'small',
	counts: {
		settled: 17,
		kept: 1,
		returns: { too_small: 8, too_large: 0 },
		feedback: { fits_well: 8, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 8, fit_too_large: 0, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 12, too_small: 8, too_large: 0 }
	},
	seeded: {
		settled: 15,
		kept: 1,
		returns: { too_small: 7, too_large: 0 },
		feedback: { fits_well: 7, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 7, fit_too_large: 0, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 11, too_small: 7, too_large: 0 }
	},
	live: {
		settled: 2,
		kept: 0,
		returns: { too_small: 1, too_large: 0 },
		feedback: { fits_well: 1, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 1, fit_too_large: 0, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 1, too_small: 1, too_large: 0 }
	},
	recent: {
		settled: 17,
		kept: 1,
		returns: { too_small: 8, too_large: 0 },
		feedback: { fits_well: 8, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 8, fit_too_large: 0, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 12, too_small: 8, too_large: 0 }
	}
});

/** The relaxed line: Claimed, below the three-shopper floor. */
const fixtureRelaxed: ProductFitResponse = makeRow({
	line: { brand_id: 'brand-v-004', category: 'shirts', fit_handle: 'relaxed' },
	size: 'M',
	status: 'Claimed',
	direction: null,
	counts: {
		settled: 2,
		kept: 0,
		returns: { too_small: 1, too_large: 1 },
		feedback: { fits_well: 0, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 1, fit_too_large: 1, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 2, too_small: 1, too_large: 1 }
	},
	seeded: {
		settled: 2,
		kept: 0,
		returns: { too_small: 1, too_large: 1 },
		feedback: { fits_well: 0, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 1, fit_too_large: 1, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 2, too_small: 1, too_large: 1 }
	},
	live: zeroCounts(),
	recent: {
		settled: 2,
		kept: 0,
		returns: { too_small: 1, too_large: 1 },
		feedback: { fits_well: 0, too_small: 0, too_large: 0 },
		reasons: { fit_too_small: 1, fit_too_large: 1, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 2, too_small: 1, too_large: 1 }
	}
});

/** The reversed line: Out of true, Earned overall but quiet lately. */
const fixtureReversed: ProductFitResponse = makeRow({
	line: { brand_id: 'brand-w-005', category: 'shirts', fit_handle: 'regular' },
	size: 'M',
	status: 'Out of true',
	direction: 'small',
	counts: {
		settled: 30,
		kept: 2,
		returns: { too_small: 20, too_large: 4 },
		feedback: { fits_well: 4, too_small: 2, too_large: 0 },
		reasons: { fit_too_small: 20, fit_too_large: 4, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 14, too_small: 10, too_large: 3 }
	},
	seeded: {
		settled: 24,
		kept: 2,
		returns: { too_small: 17, too_large: 3 },
		feedback: { fits_well: 3, too_small: 1, too_large: 0 },
		reasons: { fit_too_small: 17, fit_too_large: 3, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 12, too_small: 9, too_large: 2 }
	},
	live: {
		settled: 6,
		kept: 0,
		returns: { too_small: 3, too_large: 1 },
		feedback: { fits_well: 1, too_small: 1, too_large: 0 },
		reasons: { fit_too_small: 3, fit_too_large: 1, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 2, too_small: 1, too_large: 1 }
	},
	recent: {
		settled: 8,
		kept: 1,
		returns: { too_small: 4, too_large: 1 },
		feedback: { fits_well: 1, too_small: 1, too_large: 0 },
		reasons: { fit_too_small: 4, fit_too_large: 1, damaged_or_defective: 0, not_arrived: 0, changed_mind: 0 },
		shoppers: { fit: 6, too_small: 5, too_large: 1 }
	}
});

const rowsByKey = new Map<string, ProductFitResponse>(
	[
		fixtureEarnedOxford,
		fixtureOxfordLarge,
		fixtureSlim,
		fixtureSecondOxford,
		fixtureRelaxed,
		fixtureReversed
	].map((row) => [
		lineKey({ brand_id: row.pool.line.brand_id, category: row.pool.line.category, fit_handle: row.pool.line.fit_handle, size: row.pool.size }),
		row
	])
);

/** Every fixture pool in the seller's line order. */
export function fixturePools(): ProductFitResponse[] {
	return merchantLines.map((line) => {
		const row = rowsByKey.get(lineKey(line));
		if (!row) {
			throw new MerchantApiError('unknown-request', 'fixture has no pool for this line');
		}
		return row;
	});
}

/** A transport that replays the seller's lines offline. */
export function createFixtureTransport(): MerchantTransport {
	return {
		async call(tool: MerchantToolName, request: Record<string, unknown>): Promise<unknown> {
			if (tool !== 'product_fit') {
				throw new MerchantApiError('unknown-tool', 'the merchant app reads product_fit only');
			}
			const read = checkProductFitRequest(request);
			const row = rowsByKey.get(lineKey(read as ProductFitRequest));
			if (!row) {
				throw new MerchantApiError('unknown-request', 'fixture has no pool for this line');
			}
			return row;
		}
	};
}
