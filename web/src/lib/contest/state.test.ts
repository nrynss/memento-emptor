// The gate for the fixture contest state's transport: the helper builds the
// transport for the shopper it is given, so a caller passing a shopper other
// than the demo one gets answers for that shopper instead of "fixture
// correction knows only one shopper". The demo path keeps its frozen answer.
import { describe, expect, it } from 'vitest';
import standingExample from '../../../../testdata/api/record_correction.response.example-standing.json';
import { createFixtureTransport, fixtureShopperId, LedgerClient } from '$lib/api';
import { CorrectionClient } from './client';
import {
	createFixtureCorrectionTransport,
	fixtureContradictedBeliefId,
	fixtureCorrectionText
} from './fixture';
import { createFixtureContestState } from './state';
import type { RecordCorrectionResponse } from './types';

describe('the fixture contest state builds its transport for its shopper', () => {
	it('answers a non-demo shopper whose correction it was built for', async () => {
		const contest = createFixtureContestState('shopper_02', []);
		const response = await contest.submitStandingCorrection(fixtureCorrectionText);
		expect(response).toEqual(standingExample as unknown as RecordCorrectionResponse);
		expect(contest.status).toBe('ready');
		expect(contest.standingCorrections.map((correction) => correction.text)).toEqual([
			fixtureCorrectionText
		]);
	});

	it('keeps a non-demo shopper alive through fresh wording too', async () => {
		const contest = createFixtureContestState('shopper_04', []);
		const response = await contest.submitStandingCorrection('Take size M');
		expect(response.correction.text).toBe('Take size M');
		expect(contest.error).toBeNull();
	});

	it('passes the shopper check on the belief path', async () => {
		// The demo belief belongs to the demo shopper, so a shopper_02 transport
		// refuses it on ownership. Reaching that reason at all proves the
		// transport was built for shopper_02: a demo-built transport fails
		// earlier, at "fixture correction knows only one shopper".
		const contest = createFixtureContestState('shopper_02', await demoRows());
		await expect(
			contest.submitCorrection(fixtureContradictedBeliefId, fixtureCorrectionText)
		).rejects.toThrow(/belongs to another shopper/);
		expect(contest.status).toBe('error');
	});

	it('leaves the demo shopper on the frozen belief answer', async () => {
		const contest = createFixtureContestState(fixtureShopperId, await demoRows());
		await contest.submitCorrection(fixtureContradictedBeliefId, fixtureCorrectionText);
		expect(contest.decisionFor(fixtureContradictedBeliefId)).toBe(fixtureCorrectionText);
		expect(contest.status).toBe('ready');
	});

	it('still refuses a request that names a shopper it was not built for', async () => {
		// The fixture guard is unchanged: a transport taken on its own answers
		// only the shopper it was built for.
		const demo = new CorrectionClient(createFixtureCorrectionTransport());
		await expect(
			demo.recordCorrection({ shopper_id: 'shopper_02', text: fixtureCorrectionText })
		).rejects.toThrow(/knows only one shopper/);
	});
});

/** The demo shopper's rows at the final step, holding the contradicted belief. */
async function demoRows() {
	const listed = await new LedgerClient(createFixtureTransport(4)).listBeliefs({
		shopper_id: fixtureShopperId,
		include_seeded: true
	});
	return listed.beliefs;
}
