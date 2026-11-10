// The seller's product lines the merchant app reads. The Go helpers that
// name them (DemoOxfordLine, PopulationLines) are not importable from
// Svelte, so the list lives here as data, in the brand, category,
// fit-handle and size words the product_fit contract queries.

import type { ProductFitRequest } from './types';

/** One seller line the merchant app reads through product_fit. */
export interface MerchantLine extends ProductFitRequest {
	label: string;
}

/** The sandbox merchant's lines, one row per line and size in the grid. */
export const merchantLines: MerchantLine[] = [
	{ label: 'Oxford shirt · regular · M', brand_id: 'brand-x-001', category: 'shirts', fit_handle: 'regular', size: 'M' },
	{ label: 'Oxford shirt · regular · L', brand_id: 'brand-x-001', category: 'shirts', fit_handle: 'regular', size: 'L' },
	{ label: 'Slim shirt · slim · M', brand_id: 'brand-y-002', category: 'shirts', fit_handle: 'slim', size: 'M' },
	{ label: 'Oxford shirt · regular · M', brand_id: 'brand-z-003', category: 'shirts', fit_handle: 'regular', size: 'M' },
	{ label: 'Relaxed shirt · relaxed · M', brand_id: 'brand-v-004', category: 'shirts', fit_handle: 'relaxed', size: 'M' },
	{ label: 'Oxford shirt · regular · M', brand_id: 'brand-w-005', category: 'shirts', fit_handle: 'regular', size: 'M' }
];

/** The fixture key for one line and size. */
export function lineKey(line: ProductFitRequest): string {
	return `${line.brand_id}/${line.category}/${line.fit_handle}/${line.size}`;
}

/** The product_fit request for one seller line: the four fields, no label. */
export function toProductFitRequest(line: MerchantLine): ProductFitRequest {
	return {
		brand_id: line.brand_id,
		category: line.category,
		fit_handle: line.fit_handle,
		size: line.size
	};
}
