// The gate for the page's read order: the grid loads live when the
// proxy answers, and wholly from the bundled snapshot when the live read
// fails. A live failure never renders a half-live grid, and a snapshot
// failure fails loudly instead of rendering an empty one.

import { describe, expect, it } from 'vitest';
import {
	createFixtureTransport,
	createSameOriginTransport,
	decodeProductFitResponse,
	fixturePools,
	loadSellerPools,
	MerchantClient
} from '$lib/pool';
import example from '../../../../testdata/api/product_fit.response.example.json';

function liveClient(): MerchantClient {
	return new MerchantClient(
		createSameOriginTransport({
			fetchFn: (async () =>
				new Response(JSON.stringify(example), { status: 200 })) as unknown as typeof fetch
		})
	);
}

function refusingClient(): MerchantClient {
	return new MerchantClient(
		createSameOriginTransport({
			fetchFn: (async () => new Response('unwired', { status: 503 })) as unknown as typeof fetch
		})
	);
}

describe('live first, snapshot until then', () => {
	it('loads the grid live when the proxy answers', async () => {
		const loaded = await loadSellerPools(liveClient(), new MerchantClient(createFixtureTransport()));
		expect(loaded.source).toBe('live');
		expect(loaded.pools).toHaveLength(fixturePools().length);
		expect(loaded.pools).toEqual(loaded.pools.map(() => decodeProductFitResponse(example)));
	});

	it('loads the whole grid from the snapshot when the proxy refuses', async () => {
		const loaded = await loadSellerPools(
			refusingClient(),
			new MerchantClient(createFixtureTransport())
		);
		expect(loaded.source).toBe('snapshot');
		expect(loaded.pools).toEqual(fixturePools());
	});

	it('fails loudly when the snapshot itself cannot serve', async () => {
		const broken = new MerchantClient({
			call: () => Promise.reject(new Error('no snapshot'))
		});
		await expect(loadSellerPools(refusingClient(), broken)).rejects.toThrow('no snapshot');
	});
});
