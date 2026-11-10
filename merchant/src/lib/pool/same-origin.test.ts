// The gate for the same-origin read: the page posts product_fit to its
// own origin's proxy with no credential, and it cannot reach any other
// tool or smuggle a shopper field through it. A refusing proxy fails
// loudly so the page can read its bundled snapshot instead.

import { describe, expect, it, vi } from 'vitest';
import example from '../../../../testdata/api/product_fit.response.example.json';
import { createSameOriginTransport } from '$lib/pool/http';
import { MerchantClient } from '$lib/pool/client';
import { MerchantApiError } from '$lib/pool/errors';

const line = { brand_id: 'brand-x-001', category: 'shirts', fit_handle: 'regular', size: 'M' };

function okFetch(seen: { url: unknown; init: unknown }[]) {
	return vi.fn(async (url: unknown, init: unknown) => {
		seen.push({ url, init });
		return new Response(JSON.stringify(example), { status: 200 });
	});
}

describe('same-origin proxy transport', () => {
	it('posts product_fit to its own origin with no credential', async () => {
		const seen: { url: unknown; init: unknown }[] = [];
		const transport = createSameOriginTransport({
			fetchFn: okFetch(seen) as unknown as typeof fetch
		});
		const response = await new MerchantClient(transport).productFit(line);
		expect(response.pool.status).toBe('Earned');
		expect(seen).toHaveLength(1);
		expect(seen[0].url).toBe('/tools/product_fit');
		const init = seen[0].init as RequestInit;
		expect(init.method).toBe('POST');
		expect(init.headers).toEqual({ 'content-type': 'application/json' });
		expect(JSON.parse(init.body as string)).toEqual(line);
	});

	it('reaches no other tool through the proxy', async () => {
		const seen: { url: unknown; init: unknown }[] = [];
		const transport = createSameOriginTransport({
			fetchFn: okFetch(seen) as unknown as typeof fetch
		});
		await expect(transport.call('ledger_beliefs' as never, {})).rejects.toBeInstanceOf(
			MerchantApiError
		);
		expect(seen).toHaveLength(0);
	});

	it('rejects a shopper field before reaching the network', async () => {
		const seen: { url: unknown; init: unknown }[] = [];
		const transport = createSameOriginTransport({
			fetchFn: okFetch(seen) as unknown as typeof fetch
		});
		await expect(
			new MerchantClient(transport).productFit({ ...line, shopper_id: 'shopper_01' } as never)
		).rejects.toMatchObject({ code: 'unknown-request' });
		expect(seen).toHaveLength(0);
	});

	it('fails loudly when the proxy is not answering yet', async () => {
		const transport = createSameOriginTransport({
			fetchFn: (async () =>
				new Response('{"error":{"code":"key_not_configured"}}', { status: 503 })) as unknown as typeof fetch
		});
		await expect(new MerchantClient(transport).productFit(line)).rejects.toMatchObject({
			code: 'unknown-request'
		});
	});
});
