// The merchant app's read surface in TypeScript, mirroring the frozen Go
// wire types in internal/api/contract and the schemas in testdata/api.
// Field names stay snake_case so a frozen example assigns without mapping.
//
// The merchant app reads product_fit and nothing else: no request names a
// shopper, and no response carries one. That is the read-only guarantee,
// in the types first.

/** The pooled label, in the ledger's words. */
export type PoolStatus = 'Claimed' | 'Building' | 'Earned' | 'Out of true';

/** Which way the pool rates a line and size, or null when none leads. */
export type FitDirection = 'small' | 'large';

/** The brand, the category and the cut that name one product line. */
export interface ProductLineView {
	brand_id: string;
	category: string;
	fit_handle: string;
}

/** Settled returns by the direction they point. */
export interface DirectionCountsView {
	too_small: number;
	too_large: number;
}

/** Settled fit feedback by verdict. */
export interface VerdictCountsView {
	fits_well: number;
	too_small: number;
	too_large: number;
}

/** Settled returns by reason category: the merchant's why it comes back. */
export interface ReasonCountsView {
	fit_too_small: number;
	fit_too_large: number;
	damaged_or_defective: number;
	not_arrived: number;
	changed_mind: number;
}

/** Distinct shoppers, never identities. */
export interface ShopperCountsView {
	fit: number;
	too_small: number;
	too_large: number;
}

/**
 * The settled counts for one product line and size. Only settled purchases
 * count: a purchase still inside its window, and a refund with no reason
 * yet, count toward nothing.
 */
export interface PoolCountsView {
	settled: number;
	kept: number;
	returns: DirectionCountsView;
	feedback: VerdictCountsView;
	reasons: ReasonCountsView;
	shoppers: ShopperCountsView;
}

/**
 * The pooled product-fit belief for one product line and one size: its
 * label, the direction it rates, and the settled counts behind it. It
 * carries no shopper identity, no row and no free text.
 */
export interface PoolView {
	line: ProductLineView;
	size: string;
	status: PoolStatus;
	direction: FitDirection | null;
	counts: PoolCountsView;
	seeded: PoolCountsView;
	live: PoolCountsView;
	recent: PoolCountsView;
}

/** Asks for the pooled fit of one product line and size. */
export interface ProductFitRequest {
	brand_id: string;
	category: string;
	fit_handle: string;
	size: string;
}

/** The pooled belief with its status and the counts behind it. */
export interface ProductFitResponse {
	pool: PoolView;
}

/** The only MCP tool the merchant app may call. */
export const merchantToolNames = ['product_fit'] as const;

/** The only MCP tool the merchant app may call. */
export type MerchantToolName = (typeof merchantToolNames)[number];
