// The line the why page may name, and its own words for a pool read that
// served nothing. A belief names a brand and, when its evidence agrees, one
// category; it never names a fit handle. Every shortfall is said out loud:
// a pool the layer was not asked about must never read as a pool that had
// nothing to say.

import { describe, expect, it } from 'vitest';
import beliefExample from '../../../../testdata/api/ledger_belief.response.example.json';
import recallExample from '../../../../testdata/api/recall_for_purchase.response.example.json';
import productFitExample from '../../../../testdata/api/product_fit.response.example.json';
import type { BeliefView } from '$lib/api';
import { decodeRecallResponse } from '$lib/ledger';
import { poolNoteFor, recallLineFor } from './pool-line';

const demoBelief = beliefExample.belief as unknown as BeliefView;

function belief(changes: Partial<BeliefView>): BeliefView {
	return { ...structuredClone(demoBelief), ...changes };
}

function recallWithoutPool(lookup: 'consulted' | 'unavailable') {
	return decodeRecallResponse({ ...recallExample, pool: null, pool_lookup: lookup });
}

function recallWithPool() {
	return decodeRecallResponse({
		...recallExample,
		pool: productFitExample.pool,
		pool_lookup: 'consulted'
	});
}

describe('the line a belief lets the why page name', () => {
	it('names the brand and the one category its evidence rows agree on', () => {
		expect(recallLineFor(demoBelief)).toEqual({
			brand: 'brand-x-001',
			category: 'shirts',
			shortfall: null
		});
	});

	it('omits the category when the evidence rows disagree', () => {
		const rows = structuredClone(demoBelief.evidence);
		rows[2].item.category = 'jeans';
		expect(recallLineFor(belief({ evidence: rows }))).toEqual({
			brand: 'brand-x-001',
			category: null,
			shortfall: 'many-categories'
		});
	});

	it('omits the category when no evidence row names one', () => {
		expect(recallLineFor(belief({ evidence: [] }))).toEqual({
			brand: 'brand-x-001',
			category: null,
			shortfall: 'no-category'
		});
	});

	it('names no brand for a belief about a seller', () => {
		expect(
			recallLineFor(
				belief({ subject: { kind: 'seller', id: 'seller-acme-01', name: 'Acme' } })
			)
		).toEqual({ brand: '', category: null, shortfall: 'no-brand' });
	});
});

describe('what the page says when the recall carries no pool', () => {
	it('says nothing over a pool that is shown', () => {
		expect(poolNoteFor(recallWithPool(), recallLineFor(demoBelief))).toBeNull();
	});

	it('says the pool was consulted and had nothing to say', () => {
		const note = poolNoteFor(recallWithoutPool('consulted'), recallLineFor(demoBelief));
		expect(note?.kind).toBe('consulted');
		expect(note?.text).toContain('consulted the pool');
		expect(note?.text).toContain('nothing to say yet');
	});

	it('says the pool was not consulted when no belief names a line', () => {
		const note = poolNoteFor(recallWithoutPool('unavailable'), null);
		expect(note?.kind).toBe('no-line');
		expect(note?.text).toContain('not consulted');
		expect(note?.text).toContain('no belief is on screen');
	});

	it('says a seller belief names no product line', () => {
		const seller = belief({ subject: { kind: 'seller', id: 'seller-acme-01', name: 'Acme' } });
		const note = poolNoteFor(recallWithoutPool('unavailable'), recallLineFor(seller));
		expect(note?.kind).toBe('no-line');
		expect(note?.text).toContain('about a seller');
	});

	it('says no product line can be named when the evidence names no category', () => {
		const note = poolNoteFor(
			recallWithoutPool('unavailable'),
			recallLineFor(belief({ evidence: [] }))
		);
		expect(note?.kind).toBe('no-line');
		expect(note?.text).toContain('name no category');
	});

	it('says the evidence named more than one category', () => {
		const rows = structuredClone(demoBelief.evidence);
		rows[2].item.category = 'jeans';
		const note = poolNoteFor(recallWithoutPool('unavailable'), recallLineFor(belief({ evidence: rows })));
		expect(note?.kind).toBe('ambiguous-line');
		expect(note?.text).toContain('more than one category');
	});

	it('names every cause it cannot tell apart when it sent a whole line', () => {
		const note = poolNoteFor(recallWithoutPool('unavailable'), recallLineFor(demoBelief));
		expect(note?.kind).toBe('unavailable');
		expect(note?.text).toContain('not consulted');
		expect(note?.text).toContain('no pool index for brand-x-001 · shirts');
		expect(note?.text).toContain('no single product line');
		expect(note?.text).toContain('could not reach');
	});
});
