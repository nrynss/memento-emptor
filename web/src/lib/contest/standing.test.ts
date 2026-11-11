// The gate for the standing correction: a correction with no belief is
// schema-valid now, the decoder accepts it, the fixture replays it, and
// the state keeps it beside the rows without touching them.
import { describe, expect, it } from 'vitest';
import standingExample from '../../../../testdata/api/record_correction.response.example-standing.json';
import declaredExample from '../../../../testdata/api/record_correction.response.example.json';
import standingRequestExample from '../../../../testdata/api/record_correction.request.example-standing.json';
import { fixtureShopperId, LedgerApiError } from '$lib/api';
import { CorrectionClient } from './client';
import { decodeRecordCorrectionResponse } from './decode';
import { createFixtureCorrectionTransport } from './fixture';
import { createFixtureContestState } from './state';
import type { CorrectionTransport, RecordCorrectionResponse } from './types';

describe('standing correction shape', () => {
	it('decodes the frozen standing example with no belief', () => {
		const decoded = decodeRecordCorrectionResponse(standingExample);
		expect(decoded.correction.contradicts_belief_id).toBeUndefined();
		expect(decoded.belief_id).toBeUndefined();
		expect(decoded.status).toBe('Claimed');
		expect(decoded.standing).toBe('None');
		expect(decoded.contradicted).toBe(false);
		expect(decoded.guidance.basis).toBe('correction');
		expect(decoded.guidance.belief_id).toBeNull();
		expect(decoded.correction.takes_precedence).toBe(true);
	});

	it('still decodes the declared correction with its belief', () => {
		const decoded = decodeRecordCorrectionResponse(declaredExample);
		expect(decoded.belief_id).toBe('blf_8f2c41');
		expect(decoded.correction.contradicts_belief_id).toBe('blf_8f2c41');
		expect(decoded.contradicted).toBe(true);
	});

	it('matches the frozen standing request: no against declaration', () => {
		expect(standingRequestExample).not.toHaveProperty('contradicts_belief_id');
		expect(standingRequestExample['shopper_id']).toBe('shopper_02');
	});

	it('accepts every pivoted guidance basis', () => {
		for (const basis of ['correction', 'own_evidence', 'pool', 'statement', 'none'] as const) {
			const decoded = decodeRecordCorrectionResponse({
				...(standingExample as Record<string, unknown>),
				guidance: {
					...((standingExample as Record<string, unknown>)['guidance'] as Record<string, unknown>),
					basis
				}
			});
			expect(decoded.guidance.basis).toBe(basis);
		}
	});

	it('rejects a misshapen standing answer instead of showing it', () => {
		expect(() =>
			decodeRecordCorrectionResponse({ ...(standingExample as object), contradicted: 'yes' })
		).toThrow(LedgerApiError);
		expect(() =>
			decodeRecordCorrectionResponse({ ...(standingExample as object), belief_id: 42 })
		).toThrow(LedgerApiError);
		expect(() => decodeRecordCorrectionResponse({})).toThrow(LedgerApiError);
	});
});

describe('standing correction through the fixture path', () => {
	it('replays the canonical standing correction exactly as frozen', async () => {
		const client = new CorrectionClient(
			createFixtureCorrectionTransport(standingRequestExample['shopper_id'])
		);
		const response = await client.recordCorrection({ ...standingRequestExample });
		expect(response).toEqual(standingExample as unknown as RecordCorrectionResponse);
	});

	it('echoes fresh wording with no belief attached', async () => {
		const client = new CorrectionClient(createFixtureCorrectionTransport());
		const response = await client.recordCorrection({
			shopper_id: fixtureShopperId,
			text: 'Size up for me from now on'
		});
		expect(response.belief_id).toBeUndefined();
		expect(response.correction.contradicts_belief_id).toBeUndefined();
		expect(response.correction.text).toBe('Size up for me from now on');
		expect(response.contradicted).toBe(false);
		expect(response.guidance.belief_id).toBeNull();
	});

	it('sends the standing request without an against declaration', async () => {
		const calls: { tool: string; request: unknown }[] = [];
		const inner = createFixtureCorrectionTransport();
		const recording: CorrectionTransport = {
			call: (tool, request) => {
				calls.push({ tool, request });
				return inner.call(tool, request);
			}
		};
		const client = new CorrectionClient(recording);
		await client.recordCorrection({ shopper_id: fixtureShopperId, text: 'Use M' });
		expect(calls).toHaveLength(1);
		expect(calls[0].request).not.toHaveProperty('contradicts_belief_id');
	});

	it('fails loudly on blank edits and unknown shoppers', async () => {
		const client = new CorrectionClient(createFixtureCorrectionTransport());
		await expect(
			client.recordCorrection({ shopper_id: fixtureShopperId, text: '   ' })
		).rejects.toMatchObject({ code: 'unknown-request' });
		await expect(
			client.recordCorrection({ shopper_id: 'shopper_99', text: 'Use M' })
		).rejects.toBeInstanceOf(LedgerApiError);
	});
});

describe('standing correction in contest state', () => {
	it('keeps the standing correction beside untouched rows', async () => {
		const contest = createFixtureContestState(fixtureShopperId, []);
		expect(contest.standingCorrections).toEqual([]);
		const response = await contest.submitStandingCorrection('Use M from now on');
		expect(response.belief_id).toBeUndefined();
		expect(contest.standingCorrections.map((correction) => correction.text)).toEqual([
			'Use M from now on'
		]);
		expect(contest.beliefs).toEqual([]);
		expect(contest.status).toBe('ready');
		expect(contest.confirmationLine()).toContain('Your correction now decides');
	});

	it('lists standing corrections newest first', async () => {
		const contest = createFixtureContestState(fixtureShopperId, []);
		await contest.submitStandingCorrection('First words');
		await contest.submitStandingCorrection('Second words');
		expect(contest.standingCorrections.map((correction) => correction.text)).toEqual([
			'Second words',
			'First words'
		]);
	});

	it('refuses a blank standing correction before sending', async () => {
		const contest = createFixtureContestState(fixtureShopperId, []);
		await expect(contest.submitStandingCorrection('   ')).rejects.toMatchObject({
			code: 'unknown-request'
		});
		expect(contest.status).toBe('idle');
		expect(contest.standingCorrections).toEqual([]);
	});
});
