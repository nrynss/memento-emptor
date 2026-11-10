// A typed client for the merchant app's only read. The client takes a
// transport so the page uses one call shape in both modes: the fixture
// transport replays the seller's lines offline, and the HTTP transport
// calls the layer's product_fit with the merchant key. There is no second
// method to add a transport for: product_fit is the whole surface.

import { decodeProductFitResponse, productFitRequestFields } from './decode';
import { MerchantApiError } from './errors';
import type { MerchantToolName, ProductFitRequest, ProductFitResponse } from './types';

/** Sends one pool read to the layer. The response shape is checked. */
export interface MerchantTransport {
	call(tool: MerchantToolName, request: Record<string, unknown>): Promise<unknown>;
}

/** Refuses any request that is not exactly one product_fit read. */
export function checkProductFitRequest(request: unknown): ProductFitRequest {
	if (typeof request !== 'object' || request === null || Array.isArray(request)) {
		throw new MerchantApiError('unknown-request', 'product_fit reads one line and size, nothing else');
	}
	const record = request as Record<string, unknown>;
	const fields = Object.keys(record).sort();
	const wanted = [...productFitRequestFields].sort();
	if (fields.length !== wanted.length || fields.some((field, index) => field !== wanted[index])) {
		throw new MerchantApiError('unknown-request', 'product_fit reads one line and size, nothing else');
	}
	for (const field of wanted) {
		if (typeof record[field] !== 'string' || (record[field] as string) === '') {
			throw new MerchantApiError('unknown-request', `product_fit needs ${field}`);
		}
	}
	return {
		brand_id: record['brand_id'] as string,
		category: record['category'] as string,
		fit_handle: record['fit_handle'] as string,
		size: record['size'] as string
	};
}

/** Typed reads over any transport. Responses that miss their shape fail. */
export class MerchantClient {
	private readonly transport: MerchantTransport;

	constructor(transport: MerchantTransport) {
		this.transport = transport;
	}

	/** Reads the pooled fit for one product line and size. Any request naming
	 * more than the line and size fails before reaching the transport, so no
	 * transport can carry a shopper field to the layer. */
	async productFit(request: ProductFitRequest): Promise<ProductFitResponse> {
		const checked = checkProductFitRequest(request);
		const raw = await this.transport.call('product_fit', { ...checked });
		return decodeProductFitResponse(raw);
	}
}
