// Fixture mode for the correction path: the demo shopper's correction
// against the L belief, replayed from the frozen contract. The canonical
// correction text returns the frozen response example verbatim; any other
// wording returns the same shape with the shopper's own words echoed, so
// an edit with fresh text still moves the belief through the same path.
// Unknown shoppers and beliefs fail instead of inventing an answer.
import { fixtureShopperId, LedgerApiError } from '$lib/api';
import canonicalResponse from '../../../../testdata/api/record_correction.response.example.json';
import type { CorrectionTransport, RecordCorrectionResponse } from './types';

/** The demo correction, verbatim from the frozen request example. */
export const fixtureCorrectionText = "I've lost weight, use M";

/** The belief the demo correction contradicts. */
export const fixtureContradictedBeliefId = 'blf_8f2c41';

/** A transport that replays the demo correction at any story step. */
export function createFixtureCorrectionTransport(): CorrectionTransport {
	return {
		async call(tool, request: Record<string, unknown>): Promise<unknown> {
			if (tool !== 'record_correction') {
				throw new LedgerApiError('unknown-tool', `fixture correction knows no ${tool}`);
			}
			if (request['shopper_id'] !== fixtureShopperId) {
				throw new LedgerApiError('unknown-request', 'fixture knows only the demo shopper');
			}
			if (request['contradicts_belief_id'] !== fixtureContradictedBeliefId) {
				throw new LedgerApiError('unknown-request', 'fixture has no such belief to contradict');
			}
			const text = request['text'];
			if (typeof text !== 'string' || text.trim() === '') {
				throw new LedgerApiError('unknown-request', 'a correction needs the shopper in words');
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
