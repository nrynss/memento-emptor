// The flat rows the merchant grid renders. This module imports only pool
// types, so the page can map rows without loading AG Grid: the grid bundle
// stays behind the lazily created grid component.

import { directionLabel } from '$lib/pool';
import type { PoolStatus, PoolView } from '$lib/pool';

/** The visible mark on a seeded count. */
export const SEEDED_MARK_LABEL = 'Seeded';

/** Tooltip for the seeded mark. */
export const SEEDED_MARK_TOOLTIP = 'Seeded history: generated, not lived.';

/** One pooled line and size flattened for a grid row. */
export interface MerchantGridRow {
	id: string;
	lineLabel: string;
	size: string;
	settled: number;
	returnsSmall: number;
	returnsLarge: number;
	fitsWell: number;
	reasonsFitSmall: number;
	reasonsFitLarge: number;
	reasonsDamaged: number;
	reasonsNotArrived: number;
	reasonsChangedMind: number;
	status: PoolStatus;
	direction: string;
	seededSettled: number;
}

/** The line words the grid shows: brand, category, cut and size. */
export function lineLabelFor(pool: PoolView): string {
	return `${pool.line.brand_id} · ${pool.line.category} · ${pool.line.fit_handle}`;
}

/** The stable row id for one line and size. */
export function rowIdFor(pool: PoolView): string {
	return `${pool.line.brand_id}/${pool.line.category}/${pool.line.fit_handle}/${pool.size}`;
}

/** Flattens one pooled belief for the grid, keeping every display field. */
export function toMerchantGridRow(pool: PoolView): MerchantGridRow {
	return {
		id: rowIdFor(pool),
		lineLabel: lineLabelFor(pool),
		size: pool.size,
		settled: pool.counts.settled,
		returnsSmall: pool.counts.returns.too_small,
		returnsLarge: pool.counts.returns.too_large,
		fitsWell: pool.counts.feedback.fits_well,
		reasonsFitSmall: pool.counts.reasons.fit_too_small,
		reasonsFitLarge: pool.counts.reasons.fit_too_large,
		reasonsDamaged: pool.counts.reasons.damaged_or_defective,
		reasonsNotArrived: pool.counts.reasons.not_arrived,
		reasonsChangedMind: pool.counts.reasons.changed_mind,
		status: pool.status,
		direction: directionLabel(pool.direction),
		seededSettled: pool.seeded.settled
	};
}

/** Flattens every pooled row for the grid, keeping the line order. */
export function toMerchantGridRows(pools: PoolView[]): MerchantGridRow[] {
	return pools.map(toMerchantGridRow);
}

/** Escapes text for the grid's HTML-string cell renderers. */
export function escapeMerchantHtml(text: string): string {
	return text
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#39;');
}
