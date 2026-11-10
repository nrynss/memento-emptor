// The merchant app's pool reads: types, decoders, client, transports,
// the seller's line list, the status rule, and the offline fixture.
export { checkProductFitRequest, MerchantClient, type MerchantTransport } from './client';
export { layerUrlEnvVar, merchantEnvVars, merchantKeyEnvVar } from './config';
export { decodePool, decodeProductFitResponse, productFitRequestFields } from './decode';
export { MerchantApiError } from './errors';
export { createFixtureTransport, fixtureEarnedOxford, fixturePools } from './fixture';
export { createLayerHttpTransport, createSameOriginTransport, type LayerHttpOptions, type SameOriginOptions } from './http';
export { lineKey, merchantLines, toProductFitRequest, type MerchantLine } from './lines';
export { loadSellerPools, type PoolSource, type SellerPools } from './source';
export {
	BUILDING_RULE,
	CLAIMED_RULE,
	decidePool,
	directionLabel,
	EARNED_RULE,
	explainWhy,
	OUT_OF_TRUE_RULE,
	POOL_BUILDING_SHOPPERS,
	POOL_EARNED_RATIO,
	POOL_EARNED_SHARE_PERCENT,
	POOL_EARNED_SHOPPERS,
	POOL_RECENT_DAYS,
	ruleForStatus
} from './rule';
export { merchantToolNames, type MerchantToolName } from './types';
export type {
	DirectionCountsView,
	FitDirection,
	PoolCountsView,
	PoolStatus,
	PoolView,
	ProductFitRequest,
	ProductFitResponse,
	ProductLineView,
	ReasonCountsView,
	ShopperCountsView,
	VerdictCountsView
} from './types';
