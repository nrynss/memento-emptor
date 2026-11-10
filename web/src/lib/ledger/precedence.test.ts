// The gate for the precedence reads: every step decodes from its frozen
// recall example, overridden steps list beside the one that won, the
// statement step decides on its own when the pool cannot be reached, and
// one case never shows another shopper's rows.
import { describe, expect, it } from 'vitest';
import correctionExample from '../../../../testdata/api/recall_for_purchase.response.example-correction.json';
import baseExample from '../../../../testdata/api/recall_for_purchase.response.example.json';
import noneExample from '../../../../testdata/api/recall_for_purchase.response.example-none.json';
import ownExample from '../../../../testdata/api/recall_for_purchase.response.example-own.json';
import poolExample from '../../../../testdata/api/recall_for_purchase.response.example-pool.json';
import { LedgerApiError } from '$lib/api';
import {
	createFixtureRecallTransport,
	fixtureRecallFor,
	recallCaseShopper,
	recallCases
} from './recall';
import {
	decisionLineFor,
	decodePrecedence,
	decodeRecallResponse,
	guidanceBases,
	overriddenLineFor,
	stepLabelFor,
	type GuidanceBasis
} from './precedence';

describe('precedence steps from frozen recall', () => {
	it('names every step strongest first', () => {
		expect(guidanceBases).toEqual(['correction', 'own_evidence', 'pool', 'statement', 'none']);
	});

	it('decodes the correction case with two overridden steps beside it', () => {
		const recall = decodeRecallResponse(correctionExample);
		expect(recall.guidance.basis).toBe('correction');
		const belief = recall.beliefs[0];
		expect(belief.precedence).toEqual({
			step: 'own_evidence',
			overridden: true,
			overridden_by: 'correction'
		});
		expect(recall.pool?.precedence).toEqual({
			step: 'pool',
			overridden: true,
			overridden_by: 'correction'
		});
		expect(recall.overriding_correction?.text).toBe("I've lost weight, use M");
	});

	it('decodes the own-evidence case with the pool overridden beside it', () => {
		const recall = decodeRecallResponse(ownExample);
		expect(recall.guidance.basis).toBe('own_evidence');
		expect(recall.beliefs[0].precedence).toEqual({
			step: 'own_evidence',
			overridden: false,
			overridden_by: null
		});
		expect(recall.pool?.precedence).toEqual({
			step: 'pool',
			overridden: true,
			overridden_by: 'own_evidence'
		});
		expect(recall.overriding_correction).toBeNull();
	});

	it('decodes the pool case with the statement overridden beside it', () => {
		const recall = decodeRecallResponse(poolExample);
		expect(recall.guidance.basis).toBe('pool');
		expect(recall.beliefs[0].precedence).toEqual({
			step: 'statement',
			overridden: true,
			overridden_by: 'pool'
		});
		expect(recall.pool?.precedence).toEqual({
			step: 'pool',
			overridden: false,
			overridden_by: null
		});
	});

	it('decodes the empty case as nothing, said plainly', () => {
		const recall = decodeRecallResponse(noneExample);
		expect(recall.guidance.basis).toBe('none');
		expect(recall.beliefs).toEqual([]);
		expect(recall.pool ?? null).toBeNull();
		expect(recall.overriding_correction).toBeNull();
	});

	it('lets the statement decide on its own when the pool cannot be reached', () => {
		const recall = fixtureRecallFor('statement');
		expect(recall.guidance.basis).toBe('statement');
		expect(recall.beliefs).toHaveLength(1);
		expect(recall.beliefs[0].precedence).toEqual({
			step: 'statement',
			overridden: false,
			overridden_by: null
		});
		expect(recall.pool).toBeNull();
		expect(recall.pool_lookup).toBe('unavailable');
	});

	it('covers every case the picker offers', () => {
		expect(recallCases).toEqual(['correction', 'own', 'pool', 'statement', 'none']);
		for (const recallCase of recallCases) {
			expect(decodeRecallResponse(fixtureRecallFor(recallCase)).guidance.basis).toBe(
				recallCase === 'own' ? 'own_evidence' : recallCase
			);
		}
	});
});

describe('precedence lines for the ledger', () => {
	it('names each step in plain words', () => {
		expect(stepLabelFor('correction')).toBe('Your correction');
		expect(stepLabelFor('own_evidence')).toBe('Your own fit evidence');
		expect(stepLabelFor('pool')).toBe('Pooled product fit');
		expect(stepLabelFor('statement')).toBe('Your general statement');
		expect(stepLabelFor('none')).toBe('Nothing yet');
	});

	it('says which step decided, and nothing dressed up', () => {
		for (const basis of guidanceBases) {
			const line = decisionLineFor({
				text: 'Take size M',
				basis,
				correction_id: null,
				belief_id: null
			});
			if (basis === 'none') {
				expect(line).toContain('No memory applies yet');
			} else {
				expect(line).toContain('Decided by');
			}
		}
	});

	it('says the overridden step beside the winner', () => {
		expect(overriddenLineFor('own_evidence', 'correction')).toBe(
			'Your own fit evidence was overridden by your correction.'
		);
		expect(overriddenLineFor('pool', 'own_evidence')).toBe(
			'Pooled product fit was overridden by your own fit evidence.'
		);
		expect(overriddenLineFor('statement', 'pool')).toBe(
			'Your general statement was overridden by pooled product fit.'
		);
	});

	it('accepts the pre-pool belief without a precedence view', () => {
		const recall = decodeRecallResponse(baseExample);
		expect(recall.guidance.basis).toBe('correction');
		expect(recall.beliefs).toHaveLength(1);
		expect(recall.beliefs[0].precedence).toBeNull();
		expect(recall.pool ?? null).toBeNull();
		expect(decodePrecedence(null)).toBeNull();
		expect(decodePrecedence(undefined)).toBeNull();
	});
});

describe('one shopper per recall case', () => {
	it('replays only the case shopper', () => {
		expect(recallCaseShopper['correction']).toBe('shopper_01');
		expect(recallCaseShopper['pool']).toBe('shopper_02');
	});

	it('never shows another shopper rows', async () => {
		const correction = createFixtureRecallTransport('correction');
		await expect(
			correction.call('recall_for_purchase', { shopper_id: 'shopper_02' })
		).rejects.toMatchObject({ code: 'unknown-request' });
		const pool = createFixtureRecallTransport('pool');
		await expect(
			pool.call('recall_for_purchase', { shopper_id: 'shopper_01' })
		).rejects.toMatchObject({ code: 'unknown-request' });
		const none = createFixtureRecallTransport('none');
		await expect(
			none.call('recall_for_purchase', { shopper_id: 'shopper_01' })
		).rejects.toMatchObject({ code: 'unknown-request' });
	});

	it('fails loudly on unknown tools', async () => {
		const transport = createFixtureRecallTransport('none');
		await expect(
			transport.call('ledger_beliefs' as never, { shopper_id: 'shopper_05' })
		).rejects.toBeInstanceOf(LedgerApiError);
	});

	it('rejects a misshapen recall instead of passing it on', () => {
		expect(() =>
			decodeRecallResponse({ shopper_id: 'shopper_01', beliefs: 'nope' })
		).toThrow(LedgerApiError);
		expect(() =>
			decodePrecedence({ step: 'pool', overridden: 'yes', overridden_by: null })
		).toThrow(LedgerApiError);
		const bad: GuidanceBasis = 'belief' as unknown as GuidanceBasis;
		expect(() => stepLabelFor(bad)).toThrow();
	});
});
