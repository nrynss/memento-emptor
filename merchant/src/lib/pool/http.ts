// The HTTP transport: product_fit over plain HTTP with the merchant key.
// It posts to the layer's per-tool route and nothing else, so the key
// that is good only for product_fit can reach nothing else through it.

import { MerchantApiError } from './errors';
import type { MerchantToolName } from './types';

/** Where the layer serves product_fit, and the key that may read it. */
export interface LayerHttpOptions {
	baseUrl: string;
	merchantKey: string;
	fetchFn?: typeof fetch;
}

/** Posts product_fit reads to the layer with the merchant key. */
export function createLayerHttpTransport(options: LayerHttpOptions): {
	call: (tool: MerchantToolName, request: Record<string, unknown>) => Promise<unknown>;
} {
	const fetchFn = options.fetchFn ?? fetch;
	return {
		async call(tool: MerchantToolName, request: Record<string, unknown>): Promise<unknown> {
			if (tool !== 'product_fit') {
				throw new MerchantApiError('unknown-tool', 'the merchant app reads product_fit only');
			}
			const response = await fetchFn(`${options.baseUrl}/tools/product_fit`, {
				method: 'POST',
				headers: {
					'content-type': 'application/json',
					authorization: `Bearer ${options.merchantKey}`
				},
				body: JSON.stringify(request)
			});
			if (!response.ok) {
				throw new MerchantApiError('unknown-request', `the layer refused the read: ${response.status}`);
			}
			return (await response.json()) as unknown;
		}
	};
}
