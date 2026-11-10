// Fixture mode for recall and the pooled belief: the pivoted recall
// examples replayed verbatim, plus one fixture-mode synthesis. Four cases
// come straight from testdata/api, one shopper each; the fifth covers the
// remaining precedence step — the general statement deciding while the
// pool index cannot be reached — built from the pool case's overridden
// statement belief, with no pool attached. Unknown shoppers and unknown
// cases fail instead of inventing an answer, so the ledger can never show
// another shopper's rows.
import { fixtureShopperId, LedgerApiError } from '$lib/api';
import correctionExample from '../../../../testdata/api/recall_for_purchase.response.example-correction.json';
import noneExample from '../../../../testdata/api/recall_for_purchase.response.example-none.json';
import ownExample from '../../../../testdata/api/recall_for_purchase.response.example-own.json';
import poolExample from '../../../../testdata/api/recall_for_purchase.response.example-pool.json';
import productFitExample from '../../../../testdata/api/product_fit.response.example.json';
import {
	decodeProductFitResponse,
	decodePoolView,
	type PoolView
} from './pool';
import { decodeRecallResponse, type RecallResponse } from './precedence';

/** One recall case, named for the precedence step that decides it. */
export type RecallCase = 'correction' | 'own' | 'pool' | 'statement' | 'none';

/** Every recall case, strongest step first, then nothing. */
export const recallCases: readonly RecallCase[] = ['correction', 'own', 'pool', 'statement', 'none'];

/** The plain-words name of a recall case for the case picker. */
export const recallCaseLabels: Record<RecallCase, string> = {
	correction: 'Correction decides',
	own: 'Your own evidence decides, ahead of the pool',
	pool: 'The pool decides from day one',
	statement: 'Your statement decides, pool unreachable',
	none: 'Nothing applies yet'
};

/** The shopper whose rows a case replays. One shopper at a time, never mixed. */
export const recallCaseShopper: Record<RecallCase, string> = {
	correction: fixtureShopperId,
	own: 'shopper_04',
	pool: 'shopper_02',
	statement: 'shopper_02',
	none: 'shopper_05'
};

/**
 * The statement-decides case: the pool case's overridden statement belief,
 * deciding on its own while the pool index cannot be reached. Recall
 * carries on without a pool and never pretends an empty one.
 */
function statementCase(): RecallResponse {
	const poolCase = decodeRecallResponse(poolExample);
	const statement = poolCase.beliefs.find((belief) => belief.precedence?.step === 'statement');
	if (!statement) {
		throw new LedgerApiError('unknown-request', 'fixture has no statement belief to decide');
	}
	return {
		shopper_id: poolCase.shopper_id,
		beliefs: [{ ...statement, precedence: { step: 'statement', overridden: false, overridden_by: null } }],
		pool: null,
		pool_lookup: 'unavailable',
		overriding_correction: null,
		guidance: {
			text: 'Your usual size M, from what you said: the pool could not be reached, so nothing pooled applies.',
			basis: 'statement',
			correction_id: null,
			belief_id: statement.belief_id
		},
		pending_count: 0,
		trace_id: null
	};
}

/** The frozen recall answer for a case, decoded through the recall shape. */
export function fixtureRecallFor(recallCase: RecallCase): RecallResponse {
	switch (recallCase) {
		case 'correction':
			return decodeRecallResponse(correctionExample);
		case 'own':
			return decodeRecallResponse(ownExample);
		case 'pool':
			return decodeRecallResponse(poolExample);
		case 'statement':
			return statementCase();
		case 'none':
			return decodeRecallResponse(noneExample);
	}
}

/** The tool name the recall path calls. One tool, nothing else. */
export type RecallToolName = 'recall_for_purchase';

/** Sends one recall for the shopper before the layer recommends. */
export interface RecallTransport {
	call(tool: RecallToolName, request: Record<string, unknown>): Promise<unknown>;
}

/** A transport that replays one recall case for its shopper only. */
export function createFixtureRecallTransport(recallCase: RecallCase): RecallTransport {
	const answer = fixtureRecallFor(recallCase);
	return {
		async call(tool, request: Record<string, unknown>): Promise<unknown> {
			if (tool !== 'recall_for_purchase') {
				throw new LedgerApiError('unknown-tool', `fixture recall knows no ${tool}`);
			}
			if (request['shopper_id'] !== answer.shopper_id) {
				throw new LedgerApiError('unknown-request', 'fixture recall knows only one shopper per case');
			}
			return answer;
		}
	};
}

/** The tool name the pooled-belief path calls. */
export type ProductFitToolName = 'product_fit';

/** Reads the pooled belief for one product line and size, and nothing else. */
export interface ProductFitTransport {
	call(tool: ProductFitToolName, request: Record<string, unknown>): Promise<unknown>;
}

/**
 * A transport that replays the frozen demo-oxford pool for its line and
 * size only. Any other line fails instead of inventing counts.
 */
export function createFixtureProductFitTransport(): ProductFitTransport {
	const pool = decodeProductFitResponse(productFitExample).pool;
	return {
		async call(tool, request: Record<string, unknown>): Promise<unknown> {
			if (tool !== 'product_fit') {
				throw new LedgerApiError('unknown-tool', `fixture pool knows no ${tool}`);
			}
			if (
				request['brand_id'] !== pool.line.brand_id ||
				request['category'] !== pool.line.category ||
				request['fit_handle'] !== pool.line.fit_handle ||
				request['size'] !== pool.size
			) {
				throw new LedgerApiError('unknown-request', 'fixture pool knows only the demo line and size');
			}
			return { pool };
		}
	};
}

/** The demo oxford's pooled belief, decoded from the frozen example. */
export function fixtureDemoPool(): PoolView {
	return decodePoolView(productFitExample.pool);
}
