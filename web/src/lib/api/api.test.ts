// The gate for T4.1: the client's types are checked against
// testdata/api. Every frozen example is decoded through the client's own
// decoders, and step 9's embedded fixture is pinned equal to the frozen
// files, so a field that moves in testdata/api fails this suite until the
// types, decoders and fixture move together. The step tests replay the
// demo story at every step.

import { describe, expect, it, vi } from 'vitest';
import beliefExample from '../../../../testdata/api/ledger_belief.response.example.json';
import beliefsExample from '../../../../testdata/api/ledger_beliefs.response.example.json';
import evidenceExample from '../../../../testdata/api/ledger_evidence.response.example.json';
import pendingExample from '../../../../testdata/api/pending_questions.response.example.json';
import { LedgerClient } from './client';
import {
	decodeLedgerBeliefResponse,
	decodeLedgerBeliefsResponse,
	decodeLedgerEvidenceResponse,
	decodePendingQuestionsResponse
} from './decode';
import { LedgerApiError } from './errors';
import {
	createFixtureTransport,
	fixtureFinalBeliefs,
	fixtureFinalEvidence,
	fixtureFinalPending,
	fixtureFinalSizingRow,
	fixtureShopperId,
	type LedgerFixtureStep
} from './fixture';
import { createFixtureState } from './state';

function clientAt(step: LedgerFixtureStep): LedgerClient {
	return new LedgerClient(createFixtureTransport(step));
}

describe('frozen ledger snapshots satisfy the client types', () => {
	it('reads the final belief rows as a typed response', () => {
		const beliefs = decodeLedgerBeliefsResponse(beliefsExample);
		expect(beliefs.shopper_id).toBe(fixtureShopperId);
		expect(beliefs.beliefs).toHaveLength(4);
		const [sizing] = beliefs.beliefs;
		expect(sizing.belief.status).toBe('Out of true');
		expect(sizing.belief.standing).toBe('Venerable');
		expect(sizing.belief.contradicted).toBe(true);
		expect(sizing.belief.contradicted_by?.correction_id).toBe('corr_3b7e52');
		expect(sizing.belief.evidence).toHaveLength(3);
	});

	it('reads the final evidence rows as a typed response', () => {
		const evidence = decodeLedgerEvidenceResponse(evidenceExample);
		expect(evidence.evidence).toHaveLength(5);
		expect(evidence.evidence[0].evidence.reason?.category).toBe('none');
		expect(evidence.evidence[0].evidence.counts_toward).toBe('fit');
		const dispute = evidence.evidence.find((row) => row.evidence.outcome.kind === 'dispute');
		expect(dispute?.evidence.counts_toward).toBe('seller_reliability');
	});

	it('reads the pending refund as a typed response', () => {
		const pending = decodePendingQuestionsResponse(pendingExample);
		expect(pending.questions).toHaveLength(1);
		expect(pending.questions[0].question_id).toBe('q_9f04ab');
	});

	it('reads the full sizing row as a typed response', () => {
		const row = decodeLedgerBeliefResponse(beliefExample);
		expect(row.corrections).toHaveLength(1);
		expect(row.seeded_notice).toContain('Seeded history');
	});
});

describe('fixture snapshots equal the frozen files', () => {
	it('serves the final belief rows equal to the frozen snapshot', () => {
		expect(fixtureFinalBeliefs).toEqual(beliefsExample);
	});

	it('serves the final evidence rows equal to the frozen snapshot', () => {
		expect(fixtureFinalEvidence).toEqual(evidenceExample);
	});

	it('serves the final pending list equal to the frozen snapshot', () => {
		expect(fixtureFinalPending).toEqual(pendingExample);
	});

	it('serves the sizing row equal to the frozen snapshot', () => {
		expect(fixtureFinalSizingRow).toEqual(beliefExample);
	});

	it('serves step 9 through the client exactly as frozen', async () => {
		const client = clientAt(9);
		const [beliefs, evidence, pending, sizing] = await Promise.all([
			client.listBeliefs({ shopper_id: fixtureShopperId, include_seeded: true }),
			client.listEvidence({ shopper_id: fixtureShopperId }),
			client.listPendingQuestions({ shopper_id: fixtureShopperId }),
			client.getBelief({ shopper_id: fixtureShopperId, belief_id: 'blf_8f2c41' })
		]);
		expect(beliefs).toEqual(beliefsExample);
		expect(evidence).toEqual(evidenceExample);
		expect(pending).toEqual(pendingExample);
		expect(sizing).toEqual(beliefExample);
	});
});

describe('fixture replays the demo story at every step', () => {
	it('starts with only the general statement', async () => {
		const client = clientAt(1);
		const beliefs = await client.listBeliefs({ shopper_id: fixtureShopperId, include_seeded: true });
		expect(beliefs.beliefs.map((row) => row.belief.belief_id)).toEqual(['blf_77c310']);
		const evidence = await client.listEvidence({ shopper_id: fixtureShopperId });
		expect(evidence.evidence).toEqual([]);
		const pending = await client.listPendingQuestions({ shopper_id: fixtureShopperId });
		expect(pending.questions).toEqual([]);
	});

	it('accumulates the fit returns as Building, then Earned', async () => {
		for (const [step, count] of [[2, 1], [3, 2]] as const) {
			const client = clientAt(step);
			const sizing = await client.getBelief({ shopper_id: fixtureShopperId, belief_id: 'blf_8f2c41' });
			expect(sizing.belief.status).toBe('Building');
			expect(sizing.belief.standing).toBe('Candidate');
			expect(sizing.belief.contradicted).toBe(false);
			expect(sizing.belief.evidence).toHaveLength(count);
		}
		const earned = await clientAt(4).getBelief({
			shopper_id: fixtureShopperId,
			belief_id: 'blf_8f2c41'
		});
		expect(earned.belief.status).toBe('Earned');
		expect(earned.belief.standing).toBe('Venerable');
		expect(earned.belief.contradicted).toBe(false);
		expect(earned.belief.contradicted_by).toBeNull();
	});

	it('shows the warning beside the kept belief from the correction on', async () => {
		const sizing = await clientAt(5).getBelief({
			shopper_id: fixtureShopperId,
			belief_id: 'blf_8f2c41'
		});
		expect(sizing.belief.status).toBe('Out of true');
		expect(sizing.belief.standing).toBe('Venerable');
		expect(sizing.belief.contradicted).toBe(true);
		expect(sizing.belief.contradicted_by?.text).toBe("I've lost weight, use M");
		expect(sizing.corrections).toHaveLength(1);
	});

	it('counts the dispute toward the seller from step 6', async () => {
		const before = await clientAt(5).listBeliefs({
			shopper_id: fixtureShopperId,
			include_seeded: true
		});
		expect(before.beliefs.some((row) => row.belief.belief_id === 'blf_1a5e09')).toBe(false);
		const client = clientAt(6);
		const seller = await client.getBelief({ shopper_id: fixtureShopperId, belief_id: 'blf_1a5e09' });
		expect(seller.belief.status).toBe('Claimed');
		expect(seller.belief.evidence[0].counts_toward).toBe('seller_reliability');
		expect(seller.belief.evidence[0].reason?.category).toBe('not_arrived');
	});

	it('holds the pending refund open from step 7', async () => {
		const before = await clientAt(6).listPendingQuestions({ shopper_id: fixtureShopperId });
		expect(before.questions).toEqual([]);
		for (const step of [7, 8, 9] as const) {
			const pending = await clientAt(step).listPendingQuestions({ shopper_id: fixtureShopperId });
			expect(pending.questions.map((question) => question.question_id)).toEqual(['q_9f04ab']);
		}
	});

	it('adds the fits-well feedback only at the final step', async () => {
		const before = await clientAt(8).listBeliefs({
			shopper_id: fixtureShopperId,
			include_seeded: true
		});
		expect(before.beliefs.some((row) => row.belief.belief_id === 'blf_2b9d60')).toBe(false);
		const after = await clientAt(9).getBelief({
			shopper_id: fixtureShopperId,
			belief_id: 'blf_2b9d60'
		});
		expect(after.belief.status).toBe('Claimed');
		expect(after.belief.evidence[0].reason?.category).toBe('none');
	});
});

describe('client honors the request shapes', () => {
	it('drops seeded rows when include_seeded is false', async () => {
		const beliefs = await clientAt(9).listBeliefs({
			shopper_id: fixtureShopperId,
			include_seeded: false
		});
		expect(beliefs.beliefs.map((row) => row.belief.belief_id).sort()).toEqual([
			'blf_1a5e09',
			'blf_2b9d60'
		]);
	});

	it('filters rows by status and evidence by belief', async () => {
		const client = clientAt(9);
		const claimed = await client.listBeliefs({
			shopper_id: fixtureShopperId,
			status_filter: 'Claimed',
			include_seeded: true
		});
		expect(claimed.beliefs).toHaveLength(3);
		const sizing = await client.listEvidence({
			shopper_id: fixtureShopperId,
			belief_id: 'blf_8f2c41'
		});
		expect(sizing.evidence).toHaveLength(3);
		const pending = await client.listPendingQuestions({ shopper_id: fixtureShopperId, limit: 0 });
		expect(pending.questions).toEqual([]);
	});

	it('fails loudly on unknown shoppers, beliefs and steps', async () => {
		const client = clientAt(9);
		await expect(
			client.listBeliefs({ shopper_id: 'shopper_99', include_seeded: true })
		).rejects.toBeInstanceOf(LedgerApiError);
		await expect(
			client.getBelief({ shopper_id: fixtureShopperId, belief_id: 'blf_missing' })
		).rejects.toMatchObject({ code: 'unknown-request' });
		await expect(
			client.listEvidence({ shopper_id: fixtureShopperId, belief_id: 'blf_missing' })
		).rejects.toBeInstanceOf(LedgerApiError);
		expect(() => createFixtureTransport(10 as unknown as LedgerFixtureStep)).toThrow(LedgerApiError);
	});

	it('rejects a misshapen response instead of passing it on', async () => {
		const broken = new LedgerClient({
			call: () => Promise.resolve({ shopper_id: fixtureShopperId })
		});
		await expect(
			broken.listBeliefs({ shopper_id: fixtureShopperId, include_seeded: true })
		).rejects.toMatchObject({ code: 'bad-response' });
	});
});

describe('ledger state', () => {
	it('loads every row for the shopper and notifies', async () => {
		const state = createFixtureState(fixtureShopperId, 9);
		expect(state.status).toBe('idle');
		const listener = vi.fn();
		state.subscribe(listener);
		await state.refresh();
		expect(state.status).toBe('ready');
		expect(state.beliefs).toHaveLength(4);
		expect(state.evidence).toHaveLength(5);
		expect(state.pending.map((question) => question.question_id)).toEqual(['q_9f04ab']);
		expect(state.error).toBeNull();
		expect(listener).toHaveBeenCalled();
	});

	it('keeps the old rows and reports the failure', async () => {
		const failing = new LedgerClient({
			call: () => Promise.reject(new Error('layer is down'))
		});
		const { LedgerState } = await import('./state');
		const state = new LedgerState(failing, fixtureShopperId);
		await state.refresh();
		expect(state.status).toBe('error');
		expect(state.error).toBe('layer is down');
		expect(state.beliefs).toEqual([]);
	});
});
