// Fixture mode for the correction path: the demo shopper's correction
// against the L belief, replayed from the frozen contract, plus the
// standing correction with no belief from its own frozen example. The
// canonical correction text returns the frozen response example verbatim;
// any other wording returns the same shape with the shopper's own words
// echoed, so an edit with fresh text still moves through the same path.
// Unknown shoppers and beliefs fail instead of inventing an answer.
import { fixtureShopperId, LedgerApiError } from '$lib/api';
import canonicalResponse from '../../../../testdata/api/record_correction.response.example.json';
import standingResponse from '../../../../testdata/api/record_correction.response.example-standing.json';
import type { CorrectionTransport, RecordCorrectionResponse } from './types';

/** The demo correction, verbatim from the frozen request examples. */
export const fixtureCorrectionText = "I've lost weight, use M";

/** The belief the demo correction contradicts. */
export const fixtureContradictedBeliefId = 'blf_8f2c41';

/** The standing correction's id, verbatim from the frozen standing example. */
export const fixtureStandingCorrectionId = 'corr_5d1a09';

function checkShopper(request: Record<string, unknown>): void {
	if (request['shopper_id'] !== fixtureShopperId) {
		throw new LedgerApiError('unknown-request', 'fixture knows only the demo shopper');
	}
}

function checkText(request: Record<string, unknown>): string {
	const text = request['text'];
	if (typeof text !== 'string' || text.trim() === '') {
		throw new LedgerApiError('unknown-request', 'a correction needs the shopper in words');
	}
	return text;
}

/** A transport that replays the demo correction at any story step. */
export function createFixtureCorrectionTransport(): CorrectionTransport {
	return {
		async call(tool, request: Record<string, unknown>): Promise<unknown> {
			if (tool !== 'record_correction') {
				throw new LedgerApiError('unknown-tool', `fixture correction knows no ${tool}`);
			}
			checkShopper(request);
			const text = checkText(request);
			const against = request['contradicts_belief_id'];
			if (against === undefined) {
				if (text === fixtureCorrectionText) {
					return { ...(standingResponse as unknown as RecordCorrectionResponse) };
				}
				return {
					correction: {
						correction_id: fixtureStandingCorrectionId,
						text,
						recorded_at: '2026-11-09T10:12:00Z',
						takes_precedence: true
					},
					status: 'Claimed',
					standing: 'None',
					contradicted: false,
					guidance: {
						text,
						basis: 'correction',
						correction_id: fixtureStandingCorrectionId,
						belief_id: null
					},
					trace_id: null
				} satisfies RecordCorrectionResponse;
			}
			if (against !== fixtureContradictedBeliefId) {
				throw new LedgerApiError('unknown-request', 'fixture has no such belief to contradict');
			}
			if (text === fixtureCorrectionText) {
				const canonical = canonicalResponse as unknown as RecordCorrectionResponse;
				return { ...canonical };
			}
			const response: RecordCorrectionResponse = {
				correction: {
					correction_id: 'corr_3b7e52',
					text,
					contradicts_belief_id: fixtureContradictedBeliefId,
					recorded_at: '2026-11-05T14:02:11Z',
					takes_precedence: true
				},
				belief_id: fixtureContradictedBeliefId,
				status: 'Out of true',
				standing: 'Venerable',
				contradicted: true,
				guidance: {
					text,
					basis: 'correction',
					correction_id: 'corr_3b7e52',
					belief_id: fixtureContradictedBeliefId
				},
				trace_id: null
			};
			return response;
		}
	};
}
