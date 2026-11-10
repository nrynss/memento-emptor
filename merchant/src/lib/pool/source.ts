// The page's read order: the live proxy first, the bundled snapshot when
// the proxy is not answering yet. The snapshot is the fixture transport,
// which stays for tests and as the offline read; the whole grid comes
// from one vintage, never a mix of live and snapshot rows.

import { MerchantClient } from './client';
import { merchantLines, toProductFitRequest, type MerchantLine } from './lines';
import type { ProductFitResponse } from './types';

/** Where one grid load came from: the live proxy or the bundled snapshot. */
export type PoolSource = 'live' | 'snapshot';

/** One grid load: every seller row from one vintage, and which vintage. */
export interface SellerPools {
	pools: ProductFitResponse[];
	source: PoolSource;
}

/** Reads every seller line through one client, in the seller line order. */
async function readAll(client: MerchantClient, lines: MerchantLine[]): Promise<ProductFitResponse[]> {
	return Promise.all(lines.map((line) => client.productFit(toProductFitRequest(line))));
}

/** Loads the seller grid live first, and from the bundled snapshot when
 * the live read fails. A live failure reads nothing half-live: the whole
 * grid falls back together. A snapshot failure fails loudly instead of
 * rendering an empty grid. */
export async function loadSellerPools(
	live: MerchantClient,
	snapshot: MerchantClient,
	lines: MerchantLine[] = merchantLines
): Promise<SellerPools> {
	try {
		return { pools: await readAll(live, lines), source: 'live' };
	} catch {
		return { pools: await readAll(snapshot, lines), source: 'snapshot' };
	}
}
