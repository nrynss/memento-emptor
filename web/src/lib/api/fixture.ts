// Fixture mode: the demo story at any of its nine steps, replayed from
// the frozen contract snapshots. Step 9 is the final ledger, byte-equal
// to testdata/api; earlier steps reveal the story in order: the general
// statement, three accumulating fit returns, the correction, the seller
// dispute, the pending refund, and the fits-well feedback. Unknown tools,
// shoppers and beliefs fail instead of inventing an answer.

import { type LedgerTransport } from './client';
import { LedgerApiError } from './errors';
import type {
	BeliefView,
	CorrectionView,
	LedgerBeliefResponse,
	LedgerBeliefRow,
	LedgerBeliefsResponse,
	LedgerEvidenceResponse,
	LedgerEvidenceRow,
	LedgerToolName,
	PendingQuestionsResponse,
	QuestionView
} from './types';

/** The demo shopper every fixture row belongs to. */
export const fixtureShopperId = 'shopper_01';

/** First and last steps of the replayed story. */
export const firstFixtureStep = 1;
export const finalFixtureStep = 9;

/** One fixture step, from the general statement to the final ledger. */
export type LedgerFixtureStep = 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9;

/** What each step adds, for a step picker in the ledger page. */
export const fixtureStepLabels: Record<LedgerFixtureStep, string> = {
	1: 'Statement: usually size M',
	2: 'First fit return',
	3: 'Second fit return',
	4: 'Third fit return: Earned',
	5: 'Correction: use M',
	6: 'Dispute counts toward the seller',
	7: 'Refund with no reason: pending',
	8: 'Vague answer: still pending',
	9: 'Fits-well feedback on the M'
};

/** The shopper's correction, verbatim from the frozen snapshots. */
const fixtureCorrection: CorrectionView = {
	correction_id: 'corr_3b7e52',
	text: "I've lost weight, use M",
	contradicts_belief_id: 'blf_8f2c41',
	recorded_at: '2026-11-05T14:02:11Z',
	takes_precedence: true
};

/** The three seeded fit returns behind the L belief, oldest first. */
const fitReturns: BeliefView['evidence'] = [
	{
		evidence_id: 'ev_6d1a09',
		outcome: { kind: 'refund', status: 'COMPLETED', amount_minor: 19500, currency: 'USD' },
		item: {
			brand_id: 'brand-x-001',
			brand_name: 'Brand X',
			line_name: 'oxford shirt',
			sku: 'tbnrl46',
			size: 'M',
			category: 'shirts'
		},
		reason: { category: 'fit_too_small', text: 'too tight in the shoulders', confidence: 1.0 },
		counts_toward: 'fit',
		write_op: 'record_action',
		link: {
			order_id: 'ORD-7XK-01',
			capture_id: 'CAP-7XK-01',
			refund_id: 'RFD-7XK-01',
			custom_id: 'memento-shopper01-01',
			event_id: 'WH-2EV-01'
		},
		seeded: true,
		occurred_at: '2026-10-14T10:00:00Z'
	},
	{
		evidence_id: 'ev_6d1a10',
		outcome: { kind: 'refund', status: 'COMPLETED', amount_minor: 19500, currency: 'USD' },
		item: {
			brand_id: 'brand-x-001',
			brand_name: 'Brand X',
			line_name: 'oxford shirt',
			sku: 'tbnrl46',
			size: 'M',
			category: 'shirts'
		},
		reason: { category: 'fit_too_small', text: 'too tight across the chest', confidence: 1.0 },
		counts_toward: 'fit',
		write_op: 'record_action',
		link: {
			order_id: 'ORD-7XK-02',
			capture_id: 'CAP-7XK-02',
			refund_id: 'RFD-7XK-02',
			custom_id: 'memento-shopper01-02',
			event_id: 'WH-2EV-02'
		},
		seeded: true,
		occurred_at: '2026-10-20T10:00:00Z'
	},
	{
		evidence_id: 'ev_6d1a11',
		outcome: { kind: 'refund', status: 'COMPLETED', amount_minor: 19500, currency: 'USD' },
		item: {
			brand_id: 'brand-x-001',
			brand_name: 'Brand X',
			line_name: 'oxford shirt',
			sku: 'tbnrl46',
			size: 'M',
			category: 'shirts'
		},
		reason: { category: 'fit_too_small', text: 'sleeves too short in M', confidence: 1.0 },
		counts_toward: 'fit',
		write_op: 'record_action',
		link: {
			order_id: 'ORD-7XK-03',
			capture_id: 'CAP-7XK-03',
			refund_id: 'RFD-7XK-03',
			custom_id: 'memento-shopper01-03',
			event_id: 'WH-2EV-03'
		},
		seeded: true,
		occurred_at: '2026-10-26T10:00:00Z'
	}
];

/** The L belief before the correction: Building while returns accumulate. */
function earlySizingBelief(returnCount: 1 | 2 | 3): BeliefView {
	const evidence = fitReturns.slice(0, returnCount);
	const earned = returnCount === 3;
	return {
		belief_id: 'blf_8f2c41',
		kind: 'fit',
		subject: { kind: 'brand', id: 'brand-x-001', name: 'Brand X' },
		value: 'runs small, take size L',
		status: earned ? 'Earned' : 'Building',
		standing: earned ? 'Venerable' : 'Candidate',
		contradicted: false,
		contradicted_by: null,
		evidence,
		seeded: true,
		updated_at: evidence[evidence.length - 1].occurred_at
	};
}

/** The L belief from the correction on: Out of true, standing kept. */
const lateSizingBelief: BeliefView = {
	belief_id: 'blf_8f2c41',
	kind: 'fit',
	subject: { kind: 'brand', id: 'brand-x-001', name: 'Brand X' },
	value: 'runs small, take size L',
	status: 'Out of true',
	standing: 'Venerable',
	contradicted: true,
	contradicted_by: fixtureCorrection,
	evidence: fitReturns,
	seeded: true,
	updated_at: '2026-11-05T14:02:11Z'
};

/** The emerging M belief behind one fits-well feedback, verbatim. */
const emergingFitBelief: BeliefView = {
	belief_id: 'blf_2b9d60',
	kind: 'fit',
	subject: { kind: 'brand', id: 'brand-x-001', name: 'Brand X' },
	value: 'size M fits this shopper now',
	status: 'Claimed',
	standing: 'None',
	contradicted: false,
	contradicted_by: null,
	evidence: [
		{
			evidence_id: 'ev_51c2d8',
			outcome: { kind: 'capture', status: 'COMPLETED', amount_minor: 19500, currency: 'USD' },
			item: {
				brand_id: 'brand-x-001',
				brand_name: 'Brand X',
				line_name: 'oxford shirt',
				sku: 'PSW8GeK',
				size: 'M',
				category: 'shirts'
			},
			reason: { category: 'none', text: 'the M fits well across the shoulders', confidence: 1.0 },
			counts_toward: 'fit',
			write_op: 'record_action',
			link: {
				order_id: 'ORD-7XK-06',
				capture_id: 'CAP-7XK-06',
				custom_id: 'memento-shopper01-06',
				event_id: 'WH-2EV-06'
			},
			seeded: false,
			occurred_at: '2026-11-08T20:10:00Z'
		}
	],
	seeded: false,
	updated_at: '2026-11-08T20:10:00Z'
};

/** The seller belief behind one non-arrival dispute, verbatim. */
const sellerBelief: BeliefView = {
	belief_id: 'blf_1a5e09',
	kind: 'seller_reliability',
	subject: { kind: 'seller', id: 'seller-acme-01', name: 'Acme' },
	value: 'ships reliably for this shopper',
	status: 'Claimed',
	standing: 'None',
	contradicted: false,
	contradicted_by: null,
	evidence: [
		{
			evidence_id: 'ev_9d4c22',
			outcome: { kind: 'dispute', status: 'OPEN', amount_minor: 8900, currency: 'USD' },
			item: {
				brand_id: 'brand-x-001',
				brand_name: 'Brand X',
				line_name: 'chino',
				sku: 'chino-khaki-32',
				size: '32',
				category: 'pants'
			},
			reason: {
				category: 'not_arrived',
				text: 'MERCHANDISE_OR_SERVICE_NOT_RECEIVED',
				confidence: 1.0
			},
			counts_toward: 'seller_reliability',
			write_op: 'record_action',
			link: {
				order_id: 'ORD-7XK-04',
				capture_id: 'CAP-7XK-04',
				dispute_id: 'PP-D-51713',
				custom_id: 'memento-shopper01-04',
				event_id: 'WH-2EV-04'
			},
			seeded: false,
			occurred_at: '2026-11-06T11:30:00Z'
		}
	],
	seeded: false,
	updated_at: '2026-11-06T11:30:00Z'
};

/** The shopper's general statement: Claimed, never promotes. */
const generalStatement: BeliefView = {
	belief_id: 'blf_77c310',
	kind: 'fit',
	subject: { kind: 'brand', id: 'brand-x-001', name: 'Brand X' },
	value: 'shopper says they are usually size M',
	status: 'Claimed',
	standing: 'None',
	contradicted: false,
	contradicted_by: null,
	evidence: [],
	seeded: true,
	updated_at: '2026-10-14T10:00:00Z'
};

/** The seeded notice for the L belief, verbatim from the frozen row. */
const sizingSeededNotice =
	'Seeded history: generated, not lived. The three returns are seeded; the correction is live.';

/** The refund still missing its reason, verbatim from the frozen snapshot. */
const pendingQuestion: QuestionView = {
	question_id: 'q_9f04ab',
	outcome: { kind: 'refund', status: 'COMPLETED', amount_minor: 19500, currency: 'USD' },
	item: {
		brand_id: 'brand-x-001',
		brand_name: 'Brand X',
		line_name: 'oxford shirt',
		sku: 'tbnrl46',
		size: 'M',
		category: 'shirts'
	},
	link: {
		order_id: 'ORD-7XK-05',
		capture_id: 'CAP-7XK-05',
		refund_id: 'RFD-7XK-05',
		custom_id: 'memento-shopper01-05',
		event_id: 'WH-2EV-05'
	},
	seeded: false,
	asked_count: 1,
	last_asked_at: '2026-11-08T18:40:00Z',
	awaiting_since: '2026-11-07T09:15:00Z'
};

/** The full final ledger: step 9, byte-equal in meaning to testdata/api. */
export const fixtureFinalBeliefs: LedgerBeliefsResponse = {
	shopper_id: fixtureShopperId,
	beliefs: [
		{ belief: lateSizingBelief, evidence_count: 3, last_change_at: '2026-11-05T14:02:11Z' },
		{ belief: emergingFitBelief, evidence_count: 1, last_change_at: '2026-11-08T20:10:00Z' },
		{ belief: sellerBelief, evidence_count: 1, last_change_at: '2026-11-06T11:30:00Z' },
		{ belief: generalStatement, evidence_count: 0, last_change_at: '2026-10-14T10:00:00Z' }
	]
};

/** The full final evidence list, newest first. */
export const fixtureFinalEvidence: LedgerEvidenceResponse = {
	shopper_id: fixtureShopperId,
	evidence: [
		{ evidence: emergingFitBelief.evidence[0], belief_ids: ['blf_2b9d60'] },
		{ evidence: sellerBelief.evidence[0], belief_ids: ['blf_1a5e09'] },
		{ evidence: fitReturns[2], belief_ids: ['blf_8f2c41'] },
		{ evidence: fitReturns[1], belief_ids: ['blf_8f2c41'] },
		{ evidence: fitReturns[0], belief_ids: ['blf_8f2c41'] }
	]
};

/** The full final pending list: one refund still missing its reason. */
export const fixtureFinalPending: PendingQuestionsResponse = {
	shopper_id: fixtureShopperId,
	questions: [pendingQuestion]
};

/** The full L row with its correction, verbatim from the frozen snapshot. */
export const fixtureFinalSizingRow: LedgerBeliefResponse = {
	belief: lateSizingBelief,
	corrections: [fixtureCorrection],
	seeded_notice: sizingSeededNotice
};

function checkStep(step: number): asserts step is LedgerFixtureStep {
	if (!Number.isInteger(step) || step < firstFixtureStep || step > finalFixtureStep) {
		throw new LedgerApiError('unknown-request', `fixture has no step ${String(step)}`);
	}
}

function checkShopper(shopperId: unknown): void {
	if (shopperId !== fixtureShopperId) {
		throw new LedgerApiError('unknown-request', 'fixture knows only the demo shopper');
	}
}

/** The sizing belief as visible at a step, or null before the first return. */
function sizingAt(step: LedgerFixtureStep): BeliefView | null {
	if (step < 2) return null;
	if (step < 5) return earlySizingBelief((step - 1) as 1 | 2 | 3);
	return lateSizingBelief;
}

/** Every belief row visible at a step, strongest first. */
function rowsAt(step: LedgerFixtureStep): LedgerBeliefRow[] {
	const rows: LedgerBeliefRow[] = [];
	const sizing = sizingAt(step);
	if (sizing) {
		rows.push({
			belief: sizing,
			evidence_count: sizing.evidence.length,
			last_change_at: sizing.updated_at
		});
	}
	if (step >= 9) {
		rows.push({
			belief: emergingFitBelief,
			evidence_count: 1,
			last_change_at: emergingFitBelief.updated_at
		});
	}
	if (step >= 6) {
		rows.push({
			belief: sellerBelief,
			evidence_count: 1,
			last_change_at: sellerBelief.updated_at
		});
	}
	rows.push({
		belief: generalStatement,
		evidence_count: 0,
		last_change_at: generalStatement.updated_at
	});
	return rows;
}

/** Every evidence row visible at a step, newest first. */
function evidenceAt(step: LedgerFixtureStep): LedgerEvidenceRow[] {
	return fixtureFinalEvidence.evidence.filter((row) => {
		switch (row.evidence.evidence_id) {
			case 'ev_51c2d8':
				return step >= 9;
			case 'ev_9d4c22':
				return step >= 6;
			case 'ev_6d1a11':
				return step >= 4;
			case 'ev_6d1a10':
				return step >= 3;
			case 'ev_6d1a09':
				return step >= 2;
			default:
				return false;
		}
	});
}

/** A transport that replays the demo story at one fixed step. */
export function createFixtureTransport(step: LedgerFixtureStep): LedgerTransport {
	checkStep(step);
	const rows = rowsAt(step);
	const evidenceRows = evidenceAt(step);
	const questions = step >= 7 ? [pendingQuestion] : [];

	return {
		async call(tool: LedgerToolName, request: Record<string, unknown>): Promise<unknown> {
			checkShopper(request['shopper_id']);
			switch (tool) {
				case 'ledger_beliefs': {
					const filtered = rows.filter((row) => {
						if (request['include_seeded'] === false && row.belief.seeded) return false;
						const statusFilter = request['status_filter'];
						if (typeof statusFilter === 'string' && row.belief.status !== statusFilter) return false;
						return true;
					});
					const response: LedgerBeliefsResponse = {
						shopper_id: fixtureShopperId,
						beliefs: filtered
					};
					return response;
				}
				case 'ledger_belief': {
					const row = rows.find((candidate) => candidate.belief.belief_id === request['belief_id']);
					if (!row) {
						throw new LedgerApiError('unknown-request', 'fixture has no such belief at this step');
					}
					const contradicted = row.belief.belief_id === fixtureCorrection.contradicts_belief_id;
					const response: LedgerBeliefResponse = {
						belief: row.belief,
						corrections: contradicted && step >= 5 ? [fixtureCorrection] : [],
						seeded_notice: row.belief.seeded
							? row.belief.belief_id === 'blf_8f2c41' && step >= 5
								? sizingSeededNotice
								: 'Seeded history: generated, not lived.'
							: null
					};
					return response;
				}
				case 'ledger_evidence': {
					const beliefId = request['belief_id'];
					if (typeof beliefId === 'string') {
						const known = rows.some((row) => row.belief.belief_id === beliefId);
						if (!known) {
							throw new LedgerApiError('unknown-request', 'fixture has no such belief at this step');
						}
					}
					const response: LedgerEvidenceResponse = {
						shopper_id: fixtureShopperId,
						evidence:
							typeof beliefId === 'string'
								? evidenceRows.filter((row) => row.belief_ids.includes(beliefId))
								: evidenceRows
					};
					return response;
				}
				case 'pending_questions': {
					const limit = request['limit'];
					const response: PendingQuestionsResponse = {
						shopper_id: fixtureShopperId,
						questions:
							typeof limit === 'number' && Number.isInteger(limit) && limit >= 0
								? questions.slice(0, limit)
								: questions
					};
					return response;
				}
			}
		}
	};
}
