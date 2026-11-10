// The gate for the merchant reads: the frozen product_fit snapshot
// satisfies the client types, the fixture serves every seller line, each
// status renders with its rule, the client exposes only the product_fit
// read, and any request naming anything but a line and size fails.

import { describe, expect, it } from 'vitest';
import example from '../../../../testdata/api/product_fit.response.example.json';
import {
	checkProductFitRequest,
	createFixtureTransport,
	decidePool,
	decodeProductFitResponse,
	explainWhy,
	fixtureEarnedOxford,
	fixturePools,
	MerchantApiError,
	MerchantClient,
	merchantLines,
	ruleForStatus,
	toProductFitRequest
} from '$lib/pool';

describe('frozen product_fit snapshot satisfies the client types', () => {
	it('reads the pooled belief as a typed response', () => {
		const response = decodeProductFitResponse(example);
		expect(response.pool.line).toEqual({
			brand_id: 'brand-x-001',
			category: 'shirts',
			fit_handle: 'regular'
		});
		expect(response.pool.size).toBe('M');
		expect(response.pool.status).toBe('Earned');
		expect(response.pool.direction).toBe('small');
		expect(response.pool.counts.settled).toBe(60);
		expect(response.pool.seeded.settled).toBe(48);
		expect(response.pool.counts.shoppers.fit).toBe(39);
	});

	it('serves the Earned demo oxford equal to the frozen snapshot', () => {
		expect(fixtureEarnedOxford).toEqual(example);
	});
});

describe('fixture serves the seller lines', () => {
	it('covers every line in the seller line list', () => {
		const pools = fixturePools();
		expect(pools).toHaveLength(merchantLines.length);
		expect(pools.map((row) => row.pool.status)).toEqual([
			'Earned',
			'Building',
			'Building',
			'Building',
			'Claimed',
			'Out of true'
		]);
	});

	it('decides every served status from its own counts', () => {
		for (const row of fixturePools()) {
			const decided = decidePool(row.pool.counts, row.pool.recent);
			expect({ status: row.pool.status, direction: row.pool.direction }).toEqual(decided);
		}
	});

	it('renders each status with its rule', () => {
		const rules: Record<string, string> = {
			Claimed: 'says nothing yet',
			Building: 'not yet Earned',
			Earned: 'at least 8 distinct shoppers',
			'Out of true': 'last 90 days'
		};
		for (const row of fixturePools()) {
			const rule = ruleForStatus(row.pool.status);
			expect(rule).toContain(rules[row.pool.status]);
			const why = explainWhy(row.pool);
			expect(why.join('\n')).toContain(rule);
			expect(why.join('\n')).toContain('seeded');
		}
	});

	it('serves every line through the client', async () => {
		const client = new MerchantClient(createFixtureTransport());
		const pools = await Promise.all(
			merchantLines.map((line) => client.productFit(toProductFitRequest(line)))
		);
		expect(pools.map((row) => row.pool.status)).toEqual([
			'Earned',
			'Building',
			'Building',
			'Building',
			'Claimed',
			'Out of true'
		]);
	});
});

describe('merchant reads name a line and size, nothing else', () => {
	it('exposes only the product_fit read', () => {
		const methods = Object.getOwnPropertyNames(MerchantClient.prototype).filter(
			(name) => name !== 'constructor'
		);
		expect(methods).toEqual(['productFit']);
	});

	it('accepts exactly one line and size', () => {
		expect(
			checkProductFitRequest({
				brand_id: 'brand-x-001',
				category: 'shirts',
				fit_handle: 'regular',
				size: 'M'
			})
		).toEqual({ brand_id: 'brand-x-001', category: 'shirts', fit_handle: 'regular', size: 'M' });
	});

	it('refuses anything naming more than the line and size', () => {
		expect(() =>
			checkProductFitRequest({
				brand_id: 'brand-x-001',
				category: 'shirts',
				fit_handle: 'regular',
				size: 'M',
				shopper_id: 'shopper_01'
			})
		).toThrow(MerchantApiError);
		expect(() =>
			checkProductFitRequest({ brand_id: 'brand-x-001', category: 'shirts', size: 'M' })
		).toThrow(MerchantApiError);
		expect(() =>
			checkProductFitRequest({
				brand_id: '',
				category: 'shirts',
				fit_handle: 'regular',
				size: 'M'
			})
		).toThrow(MerchantApiError);
	});

	it('fails loudly on unknown tools and lines', async () => {
		const transport = createFixtureTransport();
		await expect(
			transport.call('ledger_beliefs' as never, {
				brand_id: 'brand-x-001',
				category: 'shirts',
				fit_handle: 'regular',
				size: 'M'
			})
		).rejects.toBeInstanceOf(MerchantApiError);
		const client = new MerchantClient(transport);
		await expect(
			client.productFit({ brand_id: 'brand-x-001', category: 'shirts', fit_handle: 'regular', size: 'XL' })
		).rejects.toMatchObject({ code: 'unknown-request' });
		await expect(
			client.productFit({
				brand_id: 'brand-x-001',
				category: 'shirts',
				fit_handle: 'regular',
				size: 'M',
				shopper_id: 'shopper_01'
			} as never)
		).rejects.toMatchObject({ code: 'unknown-request' });
	});

	it('rejects a misshapen response instead of passing it on', () => {
		const broken = new MerchantClient({
			call: () => Promise.resolve({ pool: { status: 'Earned' } })
		});
		return expect(
			broken.productFit({ brand_id: 'brand-x-001', category: 'shirts', fit_handle: 'regular', size: 'M' })
		).rejects.toMatchObject({ code: 'bad-response' });
	});
});
