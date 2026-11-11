// The product line the why view can honestly name for one belief, and the
// page's own words for each way the pool read falls short. A belief carries
// its subject's brand and its evidence rows carry a category, never a fit
// handle: the page names what it knows and the layer resolves the handle from
// its own population. Nothing here guesses a handle, special-cases a brand or
// invents a line, and a line the page cannot name is said out loud rather
// than reported as a pool that had nothing to say.

import type { BeliefView } from '$lib/api';
import type { RecallResponse } from '$lib/ledger';

/** Why the page could not name one complete product line for a belief. */
export type RecallShortfall = 'no-brand' | 'no-category' | 'many-categories';

/** What the page can honestly say about one belief's product line. */
export interface RecallLine {
	/** The belief's brand, or '' when the belief is not about a brand. */
	brand: string;
	/** The one category the belief's evidence rows agree on, or null. */
	category: string | null;
	/** Why the line is incomplete, or null when it is whole. */
	shortfall: RecallShortfall | null;
}

/**
 * Reads the line one belief lets the page name: the subject's brand when the
 * belief is about a brand, and the single category its evidence rows agree
 * on. A seller subject names no brand, an evidence list with no category
 * names no category, and evidence rows that disagree name no single one: each
 * of those is a shortfall the page reports instead of filling in.
 */
export function recallLineFor(belief: BeliefView): RecallLine {
	if (belief.subject.kind !== 'brand') {
		return { brand: '', category: null, shortfall: 'no-brand' };
	}
	const categories = [
		...new Set(belief.evidence.map((row) => row.item.category).filter((value) => value !== ''))
	].sort();
	if (categories.length === 1) {
		return { brand: belief.subject.id, category: categories[0], shortfall: null };
	}
	return {
		brand: belief.subject.id,
		category: null,
		shortfall: categories.length === 0 ? 'no-category' : 'many-categories'
	};
}

/** How one recall's pool read fell short of a pool. */
export type PoolNoteKind = 'consulted' | 'no-line' | 'ambiguous-line' | 'unavailable';

/** One honest sentence about a recall that serves no pool. */
export interface PoolNote {
	kind: PoolNoteKind;
	text: string;
}

/**
 * Says what happened to the pool read when the response carries no pool, or
 * null when it does carry one. The frozen lookup is only consulted or
 * unavailable, so the page never pretends to know which of the unavailable
 * causes the layer hit when it sent a whole line: it names them all. When the
 * page itself could not name a line, it says which part was missing, because
 * that is something it knows.
 */
export function poolNoteFor(recall: RecallResponse, line: RecallLine | null): PoolNote | null {
	if (recall.pool !== null) {
		return null;
	}
	if (recall.pool_lookup === 'consulted') {
		return {
			kind: 'consulted',
			text: 'The layer consulted the pool for this line and size and it has nothing to say yet: no pooled belief applies.'
		};
	}
	if (line === null) {
		return {
			kind: 'no-line',
			text: 'The pool was not consulted: no belief is on screen, so the page could not name a product line.'
		};
	}
	switch (line.shortfall) {
		case 'no-brand':
			return {
				kind: 'no-line',
				text: 'The pool was not consulted: this belief is about a seller, not a brand, so it names no product line.'
			};
		case 'no-category':
			return {
				kind: 'no-line',
				text: `The pool was not consulted: this belief's evidence rows name no category, so no product line can be named for ${line.brand}.`
			};
		case 'many-categories':
			return {
				kind: 'ambiguous-line',
				text: `The pool was not consulted: this belief's evidence rows name more than one category, so no single product line can be named for ${line.brand}.`
			};
		case null:
			return {
				kind: 'unavailable',
				text: `The pool was not consulted: the layer has no pool index for ${line.brand} · ${line.category}, no single product line matching them (none matched, or more than one did), no size to read (no usual size learned and none under consideration), or an index it could not reach. Nothing pooled applies.`
			};
	}
}
