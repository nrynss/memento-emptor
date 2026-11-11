// The gate for the why view model: one belief shows its counted evidence,
// the correction beside it, the deliberately-not-counted rows with their
// reasons, and searchable text — at every replay step, offline.

import { describe, expect, it } from 'vitest';
import {
	createFixtureTransport,
	fixtureShopperId,
	LedgerClient,
	type LedgerFixtureStep,
	type ReasonCategory
} from '$lib/api';
import { tracesForBelief } from './traces';
import {
	buildWhyView,
	filterWhyView,
	matchesEvidenceQuery,
	notCountedReason,
	PENDING_WHY_NOT,
	reasonReading,
	reasonWord,
	type WhyViewData
} from './why';

function clientAt(step: LedgerFixtureStep): LedgerClient {
	return new LedgerClient(createFixtureTransport(step));
}

async function whyAt(step: LedgerFixtureStep, beliefId: string): Promise<WhyViewData> {
	const client = clientAt(step);
	const [one, evidence, pending] = await Promise.all([
		client.getBelief({ shopper_id: fixtureShopperId, belief_id: beliefId }),
		client.listEvidence({ shopper_id: fixtureShopperId }),
		client.listPendingQuestions({ shopper_id: fixtureShopperId })
	]);
	return buildWhyView(
		one.belief,
		one.corrections,
		one.seeded_notice,
		evidence.evidence,
		pending.questions,
		tracesForBelief(beliefId)
	);
}

const sizingId = 'blf_8f2c41';
const sellerId = 'blf_1a5e09';
const statementId = 'blf_77c310';

describe('why view for the demo L belief', () => {
	it('shows complete evidence with the frozen correction beside it', async () => {
		const view = await whyAt(9, sizingId);
		expect(view.belief.status).toBe('Out of true');
		expect(view.counted.map((row) => row.evidence_id)).toEqual([
			'ev_6d1a09',
			'ev_6d1a10',
			'ev_6d1a11'
		]);
		for (const row of view.counted) {
			expect(row.outcome.kind).toBe('refund');
			expect(row.item.size).toBe('M');
			expect(row.reason?.category).toBe('fit_too_small');
		}
		expect(view.belief.contradicted_by?.text).toBe("I've lost weight, use M");
		expect(view.corrections.map((row) => row.text)).toEqual(["I've lost weight, use M"]);
		expect(view.seeded_notice).toContain('Seeded history');
	});

	it('shows the dispute as not counted, against the seller', async () => {
		const view = await whyAt(9, sizingId);
		const dispute = view.not_counted.find(
			(row) => row.evidence.evidence_id === 'ev_9d4c22'
		);
		expect(dispute).toBeDefined();
		expect(dispute?.evidence.reason?.category).toBe('not_arrived');
		expect(dispute?.why_not).toMatch(/seller/i);
		expect(dispute?.why_not).toMatch(/never toward sizing/);
	});

	it('shows the M feedback as counted elsewhere, not here', async () => {
		const view = await whyAt(9, sizingId);
		const feedback = view.not_counted.find(
			(row) => row.evidence.evidence_id === 'ev_51c2d8'
		);
		expect(feedback?.why_not).toMatch(/another belief/);
	});

	it('shows the pending refund as counting toward nothing', async () => {
		const view = await whyAt(9, sizingId);
		expect(view.pending.map((row) => row.question.question_id)).toEqual(['q_9f04ab']);
		expect(view.pending[0].why_not).toBe(PENDING_WHY_NOT);
		expect(PENDING_WHY_NOT).toMatch(/nothing/);
	});
});

describe('why view for the seller belief', () => {
	it('shows its one dispute with the non-arrival reason', async () => {
		const view = await whyAt(9, sellerId);
		expect(view.counted.map((row) => row.evidence_id)).toEqual(['ev_9d4c22']);
		expect(view.counted[0].outcome.kind).toBe('dispute');
		expect(view.counted[0].reason?.category).toBe('not_arrived');
		expect(view.belief.contradicted).toBe(false);
	});

	it('shows the fit returns as not counted, toward sizing', async () => {
		const view = await whyAt(9, sellerId);
		const returns = view.not_counted.filter((row) =>
			row.evidence.evidence_id.startsWith('ev_6d1a')
		);
		expect(returns).toHaveLength(3);
		for (const row of returns) {
			expect(row.why_not).toMatch(/sizing/);
			expect(row.why_not).toMatch(/never against the seller/);
		}
	});

	it('names the kind the fits-well feedback counts toward, not nothing', async () => {
		const view = await whyAt(9, sellerId);
		const feedback = view.not_counted.find(
			(row) => row.evidence.evidence_id === 'ev_51c2d8'
		);
		expect(feedback?.evidence.counts_toward).toBe('fit');
		expect(feedback?.why_not).toMatch(/another belief/);
		expect(feedback?.why_not).not.toMatch(/nothing/);
	});
});

describe('why view across replay steps', () => {
	it('grows the counted evidence from one return to three', async () => {
		expect((await whyAt(2, sizingId)).counted).toHaveLength(1);
		expect((await whyAt(3, sizingId)).counted).toHaveLength(2);
		expect((await whyAt(4, sizingId)).counted).toHaveLength(3);
	});

	it('walks the status from Building to Earned to Out of true', async () => {
		expect((await whyAt(2, sizingId)).belief.status).toBe('Building');
		expect((await whyAt(4, sizingId)).belief.status).toBe('Earned');
		const corrected = await whyAt(5, sizingId);
		expect(corrected.belief.status).toBe('Out of true');
		expect(corrected.belief.contradicted_by?.text).toBe("I've lost weight, use M");
	});

	it('has no correction, dispute or pending before their steps', async () => {
		const early = await whyAt(4, sizingId);
		expect(early.belief.contradicted).toBe(false);
		expect(early.not_counted).toEqual([]);
		expect(early.pending).toEqual([]);
	});

	it('reports a missing belief at an early step instead of inventing one', async () => {
		await expect(whyAt(1, sizingId)).rejects.toThrowError(/no such belief/);
	});

	it('shows the general statement with nothing counted', async () => {
		const view = await whyAt(1, statementId);
		expect(view.counted).toEqual([]);
		expect(view.belief.contradicted).toBe(false);
	});
});

describe('why view search', () => {
	it('matches reason words, items and outcomes case-insensitively', async () => {
		const view = await whyAt(9, sizingId);
		expect(filterWhyView(view, 'shoulders').counted.map((row) => row.evidence_id)).toEqual([
			'ev_6d1a09'
		]);
		expect(
			filterWhyView(view, 'CHINO').not_counted.map((row) => row.evidence.evidence_id)
		).toEqual(['ev_9d4c22']);
		expect(filterWhyView(view, 'refund').counted).toHaveLength(3);
	});

	it('leaves a blank query unfiltered and an impossible one empty', async () => {
		const view = await whyAt(9, sizingId);
		const blank = filterWhyView(view, '   ');
		expect(blank.counted).toHaveLength(3);
		expect(blank.not_counted.map((row) => row.evidence.evidence_id)).toContain('ev_9d4c22');
		const none = filterWhyView(view, 'no-such-words-anywhere');
		expect(none.counted).toEqual([]);
		expect(none.not_counted).toEqual([]);
		expect(none.pending).toEqual([]);
	});

	it('matches pending questions by item words', async () => {
		const view = await whyAt(9, sizingId);
		expect(filterWhyView(view, 'oxford').pending).toHaveLength(1);
		expect(matchesEvidenceQuery(view.counted[0], '')).toBe(true);
	});
});

describe('why view traces', () => {
	it('links the L belief to placeholder traces, never a guessed host', async () => {
		const view = await whyAt(9, sizingId);
		expect(view.traces.length).toBeGreaterThan(0);
		for (const trace of view.traces) {
			expect(trace.href).toBe(`#trace-${trace.trace_id}`);
			expect(trace.href).not.toMatch(/^https?:/);
		}
	});

	it('leaves beliefs with no recorded turn linkless', async () => {
		expect((await whyAt(9, sellerId)).traces).toEqual([]);
		expect(tracesForBelief('blf_unknown')).toEqual([]);
	});
});

describe('why view plain language', () => {
	it('never explains an exclusion with graph jargon', async () => {
		for (const step of [2, 4, 6, 9] as const) {
			const view = await whyAt(step, sizingId);
			for (const row of view.not_counted) {
				expect(notCountedReason(row.evidence, view.belief)).not.toMatch(
					/Venerable|Candidate|Canonical/
				);
			}
		}
	});

	it('names every classified reason category in plain words, never the raw code', () => {
		const classified: ReasonCategory[] = [
			'fit_too_small',
			'fit_too_large',
			'damaged_or_defective',
			'not_arrived',
			'changed_mind',
			'unclear'
		];
		for (const category of classified) {
			const word = reasonWord(category) ?? '';
			expect(word).not.toBe('');
			expect(word).not.toMatch(/_/);
			expect(word).toMatch(/^[a-z ]+$/);
		}
		expect(reasonWord('fit_too_small')).toBe('too small');
		expect(reasonWord('fit_too_large')).toBe('too large');
		expect(reasonWord('damaged_or_defective')).toBe('damaged');
		expect(reasonWord('not_arrived')).toBe('not arrived');
		expect(reasonWord('changed_mind')).toBe('changed mind');
	});

	it('names no category for the layer zero value, which is no classified reason', () => {
		// domain.CategoryNone is the zero value: no classified reason, not a
		// verdict. The unauthorised-transaction dispute is stored with it, so a
		// card calling it "fits well" would assert a verdict nobody gave.
		expect(reasonWord('none')).toBeNull();
	});

	it('quotes the shopper and names the category for a classified refund', async () => {
		const view = await whyAt(9, sizingId);
		const first = view.counted[0];
		expect(first.reason?.text).toBe('too tight in the shoulders');
		expect(reasonReading(first)).toEqual({
			text: 'too tight in the shoulders',
			quoted: true,
			word: 'too small'
		});
	});

	it('never quotes a dispute code as the shopper words', async () => {
		const view = await whyAt(9, sellerId);
		const dispute = view.counted[0];
		expect(dispute.outcome.kind).toBe('dispute');
		expect(reasonReading(dispute)).toEqual({
			text: 'MERCHANDISE_OR_SERVICE_NOT_RECEIVED',
			quoted: false,
			word: 'not arrived'
		});
	});

	it('reports an outcome with no known cause rather than reading one in', async () => {
		const view = await whyAt(9, sizingId);
		const pendingRow = view.counted[0];
		expect(reasonReading({ ...pendingRow, reason: null })).toBeNull();
	});

	it('reads the demo evidence back in plain words', async () => {
		const view = await whyAt(9, sizingId);
		expect(
			view.counted.map((row) => (row.reason === null ? null : reasonWord(row.reason.category)))
		).toEqual(['too small', 'too small', 'too small']);
		const dispute = view.not_counted.find((row) => row.evidence.evidence_id === 'ev_9d4c22');
		const disputeReason = dispute?.evidence.reason;
		expect(
			disputeReason === null || disputeReason === undefined
				? null
				: reasonWord(disputeReason.category)
		).toBe('not arrived');
	});
});
