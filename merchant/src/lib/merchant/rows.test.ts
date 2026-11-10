// The gate for the merchant rows: the fixture pools map to the grid shape
// in line order, counts and marks render from the mapped rows, and no row
// carries anything a shopper could be read from.

import { describe, expect, it } from 'vitest';
import { fixturePools } from '$lib/pool';
import {
	escapeMerchantHtml,
	lineLabelFor,
	rowIdFor,
	toMerchantGridRows
} from './rows';

describe('merchant rows', () => {
	it('maps every fixture pool in line order', () => {
		const rows = toMerchantGridRows(fixturePools().map((row) => row.pool));
		expect(rows.map((row) => row.id)).toEqual([
			'brand-x-001/shirts/regular/M',
			'brand-x-001/shirts/regular/L',
			'brand-y-002/shirts/slim/M',
			'brand-z-003/shirts/regular/M',
			'brand-v-004/shirts/relaxed/M',
			'brand-w-005/shirts/regular/M'
		]);
		expect(rows.map((row) => row.status)).toEqual([
			'Earned',
			'Building',
			'Building',
			'Building',
			'Claimed',
			'Out of true'
		]);
	});

	it('keeps the Earned row counts and the seeded split', () => {
		const [earned] = toMerchantGridRows([fixturePools()[0].pool]);
		expect(earned.settled).toBe(60);
		expect(earned.returnsSmall).toBe(24);
		expect(earned.returnsLarge).toBe(4);
		expect(earned.fitsWell).toBe(12);
		expect(earned.reasonsFitSmall).toBe(24);
		expect(earned.reasonsChangedMind).toBe(4);
		expect(earned.direction).toBe('runs small');
		expect(earned.seededSettled).toBe(48);
	});

	it('marks a pool with no direction with an em dash', () => {
		const claimed = toMerchantGridRows([fixturePools()[4].pool])[0];
		expect(claimed.status).toBe('Claimed');
		expect(claimed.direction).toBe('—');
	});

	it('labels lines by brand, category and cut', () => {
		const [first] = fixturePools();
		expect(lineLabelFor(first.pool)).toBe('brand-x-001 · shirts · regular');
		expect(rowIdFor(first.pool)).toBe('brand-x-001/shirts/regular/M');
	});

	it('carries no shopper field on any row', () => {
		const rows = toMerchantGridRows(fixturePools().map((row) => row.pool));
		for (const row of rows) {
			expect(Object.keys(row).join(' ')).not.toMatch(/shopper/i);
			expect(JSON.stringify(row)).not.toContain('shopper_');
		}
	});

	it('escapes line text in the HTML-string renderers', () => {
		expect(escapeMerchantHtml('<oxford> & "regular"')).toBe(
			'&lt;oxford&gt; &amp; &quot;regular&quot;'
		);
	});
});
