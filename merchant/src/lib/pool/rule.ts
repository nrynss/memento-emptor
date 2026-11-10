// The pooled-status rule, in the app's own words. The layer decides: this
// module only explains. The thresholds mirror internal/domain/pool.go, the
// one definition the pool reader and the ledger read from; if they ever
// disagree the layer wins and this module's tests fail first.

import type { FitDirection, PoolCountsView, PoolStatus, PoolView } from './types';

/** Distinct shoppers with a fit-classified outcome before the pool speaks. */
export const POOL_BUILDING_SHOPPERS = 3;

/** Distinct shoppers one direction needs before it is Earned. */
export const POOL_EARNED_SHOPPERS = 8;

/** The share of fit-classified outcomes one direction needs. */
export const POOL_EARNED_SHARE_PERCENT = 60;

/** How many times the opposite direction a direction must reach. */
export const POOL_EARNED_RATIO = 2;

/** How far back the Out-of-true rule reads, in days of event time. */
export const POOL_RECENT_DAYS = 90;

/** The rule the grid's why panel shows for a Claimed pool. */
export const CLAIMED_RULE =
	'Claimed: fewer than 3 distinct shoppers with a fit-classified outcome, so the pool says nothing yet.';

/** The rule the grid's why panel shows for a Building pool. */
export const BUILDING_RULE =
	'Building: at least 3 distinct shoppers with a fit-classified outcome, and not yet Earned.';

/** The rule the grid's why panel shows for an Earned pool. */
export const EARNED_RULE =
	'Earned: at least 8 distinct shoppers in one direction, at least 60% of fit-classified outcomes, and at least twice the opposite direction.';

/** The rule the grid's why panel shows for an Out-of-true pool. */
export const OUT_OF_TRUE_RULE =
	'Out of true: Earned overall, but the last 90 days alone fail the Earned rule for that direction or meet it for the opposite one.';

/** The rule judged for a pool at one status. */
export function ruleForStatus(status: PoolStatus): string {
	switch (status) {
		case 'Claimed':
			return CLAIMED_RULE;
		case 'Building':
			return BUILDING_RULE;
		case 'Earned':
			return EARNED_RULE;
		case 'Out of true':
			return OUT_OF_TRUE_RULE;
	}
}

/** One direction's outcomes on settled counts. */
interface DirectionTally {
	outcomes: number;
	shoppers: number;
}

/** The settled outcomes pointing one way on a counts view. */
function tally(counts: PoolCountsView, direction: FitDirection): DirectionTally {
	if (direction === 'small') {
		return {
			outcomes: counts.returns.too_small + counts.feedback.too_small,
			shoppers: counts.shoppers.too_small
		};
	}
	return {
		outcomes: counts.returns.too_large + counts.feedback.too_large,
		shoppers: counts.shoppers.too_large
	};
}

/** Every settled outcome with a fit classification, whichever way it points. */
function fitOutcomes(counts: PoolCountsView): number {
	return (
		counts.returns.too_small +
		counts.returns.too_large +
		counts.feedback.too_small +
		counts.feedback.too_large +
		counts.feedback.fits_well
	);
}

/** Whether one direction meets the Earned rule on these counts. */
function earns(counts: PoolCountsView, direction: FitDirection): boolean {
	const one = tally(counts, direction);
	const other = tally(counts, direction === 'small' ? 'large' : 'small');
	const fit = fitOutcomes(counts);
	if (fit === 0) return false;
	return (
		one.shoppers >= POOL_EARNED_SHOPPERS &&
		one.outcomes * 100 >= fit * POOL_EARNED_SHARE_PERCENT &&
		one.outcomes >= POOL_EARNED_RATIO * other.outcomes
	);
}

/** Which direction leads on outcomes, or null when the two are level. */
function leading(counts: PoolCountsView): FitDirection | null {
	const small = tally(counts, 'small').outcomes;
	const large = tally(counts, 'large').outcomes;
	if (small > large) return 'small';
	if (large > small) return 'large';
	return null;
}

/**
 * Decides the pooled label from the all-time and recent counts, the same
 * rule the layer reads: below the floor the pool says nothing, at the
 * floor it is Building, an Earned direction holds unless the recent window
 * on its own fails it.
 */
export function decidePool(all: PoolCountsView, recent: PoolCountsView): {
	status: PoolStatus;
	direction: FitDirection | null;
} {
	if (all.shoppers.fit < POOL_BUILDING_SHOPPERS) {
		return { status: 'Claimed', direction: null };
	}
	let earned: FitDirection | null = null;
	if (earns(all, 'small')) earned = 'small';
	else if (earns(all, 'large')) earned = 'large';
	if (earned === null) {
		return { status: 'Building', direction: leading(all) };
	}
	if (recent.shoppers.fit >= POOL_BUILDING_SHOPPERS) {
		if (!earns(recent, earned)) {
			return { status: 'Out of true', direction: earned };
		}
	}
	return { status: 'Earned', direction: earned };
}

/** The word the why panel shows for a direction, or an em dash for none. */
export function directionLabel(direction: FitDirection | null): string {
	if (direction === 'small') return 'runs small';
	if (direction === 'large') return 'runs large';
	return '—';
}

/**
 * The why behind one pooled row: the counts behind its status and the rule
 * it was judged by. A row opens this in the panel beside the grid.
 */
export function explainWhy(pool: PoolView): string[] {
	const decided = decidePool(pool.counts, pool.recent);
	const agreement =
		decided.status === pool.status && decided.direction === pool.direction
			? 'The counts agree with the served status.'
			: 'The counts disagree with the served status: ask the layer, not the grid.';
	return [
		`${pool.line.brand_id} · ${pool.line.category} · ${pool.line.fit_handle}, size ${pool.size}: ${pool.status}${pool.direction === null ? '' : `, ${directionLabel(pool.direction)}`}.`,
		`Settled purchases: ${pool.counts.settled} (${pool.seeded.settled} seeded, ${pool.counts.settled - pool.seeded.settled} live).`,
		`Distinct shoppers with a fit-classified outcome: ${pool.counts.shoppers.fit} (${pool.counts.shoppers.too_small} small, ${pool.counts.shoppers.too_large} large).`,
		`Returns: ${pool.counts.returns.too_small} too small, ${pool.counts.returns.too_large} too large.`,
		`Reasons: fit too small ${pool.counts.reasons.fit_too_small}, fit too large ${pool.counts.reasons.fit_too_large}, damaged or defective ${pool.counts.reasons.damaged_or_defective}, not arrived ${pool.counts.reasons.not_arrived}, changed mind ${pool.counts.reasons.changed_mind}.`,
		`Feedback: fits well ${pool.counts.feedback.fits_well}, too small ${pool.counts.feedback.too_small}, too large ${pool.counts.feedback.too_large}.`,
		`Kept past the window with no feedback: ${pool.counts.kept}.`,
		ruleForStatus(pool.status),
		agreement
	];
}
