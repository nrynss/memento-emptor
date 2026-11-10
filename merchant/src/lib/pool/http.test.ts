// The gate for the HTTP transport: it posts product_fit to the layer's
// per-tool route with the merchant key, and it cannot reach any other
// tool through the same key.

import { describe, expect, it, vi } from 'vitest';
import example from '../../../../testdata/api/product_fit.response.example.json';
import { createLayerHttpTransport } from '$lib/pool/http';
import { MerchantClient } from '$lib/pool/client';
import { MerchantApiError } from '$lib/pool/errors';

const line = { brand_id: 'brand-x-001', category: 'shirts', fit_handle: 'regular', size: 'M' };

function okFetch(seen: { url: unknown; init: unknown }[]) {
	return vi.fn(async (url: unknown, init: unknown) => {
		seen.push({ url, init });
		return new Response(JSON.stringify(example), { status: 200 });
	});
}

describe('layer HTTP transport', () => {
	it('posts product_fit with the merchant key', async () => {
		const seen: { url: unknown; init: unknown }[] = [];
		const transport = createLayerHttpTransport({
			baseUrl: 'https://layer.example',
			merchantKey: 'merchant-key-for-test',
			fetchFn: okFetch(seen) as unknown as typeof fetch
		});
		const response = await new MerchantClient(transport).productFit(line);
		expect(response.pool.status).toBe('Earned');
		expect(seen).toHaveLength(1);
		expect(seen[0].url).toBe('https://layer.example/tools/product_fit');
		const init = seen[0].init as RequestInit;
		expect(init.method).toBe('POST');
		expect((init.headers as Record<string, string>)['authorization']).toBe(
			'Bearer merchant-key-for-test'
		);
		expect(JSON.parse(init.body as string)).toEqual(line);
	});

	it('reaches no other tool with the key', async () => {
		const seen: { url: unknown; init: unknown }[] = [];
		const transport = createLayerHttpTransport({
			baseUrl: 'https://layer.example',
			merchantKey: 'merchant-key-for-test',
			fetchFn: okFetch(seen) as unknown as typeof fetch
		});
		await expect(transport.call('ledger_beliefs' as never, {})).rejects.toBeInstanceOf(
			MerchantApiError
		);
		expect(seen).toHaveLength(0);
	});

	it('fails loudly when the layer refuses', async () => {
		const transport = createLayerHttpTransport({
			baseUrl: 'https://layer.example',
			merchantKey: 'merchant-key-for-test',
			fetchFn: (async () => new Response('forbidden', { status: 403 })) as unknown as typeof fetch
		});
		await expect(new MerchantClient(transport).productFit(line)).rejects.toMatchObject({
			code: 'unknown-request'
		});
	});
});
