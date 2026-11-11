// The gate for the pooled row: the frozen product_fit example decodes to
// counts with its seeded split, the ledger lines show counts only, and no
// shopper identity can reach the row because the shape has nowhere to hold
// one.
import { describe, expect, it } from 'vitest';
import productFitExample from '../../../../testdata/api/product_fit.response.example.json';
import { LedgerApiError } from '$lib/api';
import { createFixtureProductFitTransport, fixtureDemoPool } from './recall';
import {
	decodePoolView,
	decodeProductFitResponse,
	directionWordFor,
	poolSeededLine,
	poolSummaryLine
} from './pool';

describe('pooled belief from the frozen example', () => {
	it('decodes the demo oxford pool with its counts', () => {
		const { pool } = decodeProductFitResponse(productFitExample);
		expect(pool.status).toBe('Earned');
		expect(pool.direction).toBe('small');
		expect(directionWordFor(pool)).toBe('small');
		expect(pool.line).toEqual({ brand_id: 'brand-x-001', category: 'shirts', fit_handle: 'regular' });
		expect(pool.size).toBe('M');
		expect(pool.counts.settled).toBe(60);
		expect(pool.counts.returns).toEqual({ too_small: 24, too_large: 4 });
		expect(pool.counts.feedback.fits_well).toBe(12);
		expect(pool.counts.shoppers).toEqual({ fit: 39, too_small: 25, too_large: 6 });
	});

	it('splits the seeded share from the lived share', () => {
		const { pool } = decodeProductFitResponse(productFitExample);
		expect(pool.seeded.settled).toBe(48);
		expect(pool.live.settled).toBe(12);
		expect(pool.seeded.settled + pool.live.settled).toBe(pool.counts.settled);
	});

	it('decodes the pool precedence beside the winner', () => {
		const pool = decodePoolView({
			...(productFitExample.pool as Record<string, unknown>),
			precedence: { step: 'pool', overridden: true, overridden_by: 'correction' }
		});
		expect(pool.precedence).toEqual({ step: 'pool', overridden: true, overridden_by: 'correction' });
		const plain = decodePoolView(productFitExample.pool);
		expect(plain.precedence).toBeNull();
	});

	it('serves the demo pool through the fixture transport', () => {
		expect(fixtureDemoPool()).toEqual(productFitExample.pool);
	});
});

describe('pooled row shows counts only', () => {
	it('summarises the pool in counts with distinct shoppers', () => {
		const { pool } = decodeProductFitResponse(productFitExample);
		const line = poolSummaryLine(pool);
		expect(line).toContain('Earned');
		expect(line).toContain('60 settled purchases');
		expect(line).toContain('24 too-small returns');
		expect(line).toContain('4 too-large returns');
		expect(line).toContain('12 fits-well');
		expect(line).toContain('39 distinct shoppers');
		expect(directionWordFor(pool)).not.toBeNull();
	});

	it('names the seeded share in counts', () => {
		const { pool } = decodeProductFitResponse(productFitExample);
		expect(poolSeededLine(pool)).toBe('48 of the 60 settled purchases are seeded history.');
	});

	it('carries no shopper identity, row or free text anywhere', () => {
		const { pool } = decodeProductFitResponse(productFitExample);
		const serialised = JSON.stringify(pool).toLowerCase();
		for (const token of ['shopper_id', 'payer', 'identity', 'custom_id', 'event_id']) {
			expect(serialised).not.toContain(token);
		}
		expect(serialised).not.toContain('too tight in the shoulders');
		expect(Object.keys(pool).sort()).toEqual(
			['counts', 'direction', 'line', 'live', 'precedence', 'recent', 'seeded', 'size', 'status'].sort()
		);
	});
});

describe('pool fixture transport', () => {
	it('answers the demo line and size, and nothing else', async () => {
		const transport = createFixtureProductFitTransport();
		const answer = (await transport.call('product_fit', {
			brand_id: 'brand-x-001',
			category: 'shirts',
			fit_handle: 'regular',
			size: 'M'
		})) as { pool: unknown };
		expect(decodePoolView(answer.pool).status).toBe('Earned');
		await expect(
			transport.call('product_fit', {
				brand_id: 'brand-x-001',
				category: 'shirts',
				fit_handle: 'slim',
				size: 'M'
			})
		).rejects.toMatchObject({ code: 'unknown-request' });
		await expect(
			transport.call('ledger_beliefs' as never, {})
		).rejects.toBeInstanceOf(LedgerApiError);
	});

	it('rejects a misshapen pool instead of rendering it', () => {
		expect(() => decodePoolView({ line: 'nope' })).toThrow(LedgerApiError);
		expect(() => decodeProductFitResponse({ pool: null })).toThrow(LedgerApiError);
	});
});
