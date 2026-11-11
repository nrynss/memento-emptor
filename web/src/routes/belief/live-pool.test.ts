// The why page's live recommendation names the product line it can honestly
// know: the belief's brand and the one category its evidence rows agree on,
// never a fit handle. The pool the layer serves then renders, and every way a
// pool read falls short is labelled in the page's own words: consulted with
// nothing to say, or not consulted at all and why.

import '../../lib/ledger/test-setup';

import { afterEach, describe, expect, it, vi } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../../node_modules/svelte/src/index-client.js';
import beliefExample from '../../../../testdata/api/ledger_belief.response.example.json';
import beliefsExample from '../../../../testdata/api/ledger_beliefs.response.example.json';
import evidenceExample from '../../../../testdata/api/ledger_evidence.response.example.json';
import pendingExample from '../../../../testdata/api/pending_questions.response.example.json';
import productFitExample from '../../../../testdata/api/product_fit.response.example.json';
import recallExample from '../../../../testdata/api/recall_for_purchase.response.example.json';
import BeliefPage from './+page.svelte';

const request = vi.hoisted(() => ({ url: new URL('http://localhost/belief') }));

vi.mock('$app/environment', () => ({ browser: true }));
vi.mock('$app/state', () => ({
	page: {
		get url() {
			return request.url;
		}
	}
}));

const token = 'memento_shop_belief_token';

interface Call {
	url: string;
	init: RequestInit | undefined;
}

/** The frozen answers, with any tool's body replaced by a case's own answer. */
function liveEdge(overrides: Record<string, unknown> = {}): {
	calls: Call[];
	fetch: typeof fetch;
} {
	const calls: Call[] = [];
	const bodies: Record<string, unknown> = {
		ledger_beliefs: beliefsExample,
		ledger_belief: beliefExample,
		ledger_evidence: evidenceExample,
		pending_questions: pendingExample,
		recall_for_purchase: recallExample,
		...overrides
	};
	const send = ((input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input);
		calls.push({ url, init });
		const tool = url.split('/').pop() ?? '';
		return Promise.resolve({
			ok: true,
			status: 200,
			text: () => Promise.resolve(JSON.stringify(bodies[tool] ?? {}))
		} as unknown as Response);
	}) as typeof globalThis.fetch;
	return { calls, fetch: send };
}

/** The belief response with one belief's fields replaced. */
function beliefBody(changes: Record<string, unknown>): unknown {
	const base = structuredClone(beliefExample) as {
		belief: Record<string, unknown>;
	};
	return { ...base, belief: { ...base.belief, ...changes } };
}

function recallBodyWithPool(): unknown {
	return {
		...structuredClone(recallExample),
		pool: structuredClone(productFitExample.pool),
		pool_lookup: 'consulted'
	};
}

function recallBodyWithoutPool(lookup: 'consulted' | 'unavailable'): unknown {
	return { ...structuredClone(recallExample), pool: null, pool_lookup: lookup };
}

/** The recall request body the page sent, or a failure saying none was sent. */
function recallRequest(calls: Call[]): Record<string, unknown> {
	const call = calls.find((one) => one.url.endsWith('/recall_for_purchase'));
	if (call === undefined) {
		throw new Error('the page sent no recall_for_purchase call');
	}
	return JSON.parse(String(call.init?.body)) as Record<string, unknown>;
}

const mounted: { comp: object; host: HTMLElement }[] = [];

afterEach(() => {
	for (const { comp, host } of mounted.splice(0)) {
		unmount(comp as Parameters<typeof unmount>[0]);
		host.remove();
	}
	vi.unstubAllGlobals();
});

function render(): HTMLElement {
	request.url = new URL(`http://localhost/belief?t=${token}&s=shopper_01`);
	const host = document.createElement('div');
	document.body.appendChild(host);
	mounted.push({ comp: mount(BeliefPage, { target: host }), host });
	return host;
}

async function waitFor(condition: () => boolean, label: string): Promise<void> {
	const started = Date.now();
	while (!condition()) {
		if (Date.now() - started > 4000) {
			throw new Error(`timed out waiting for ${label}`);
		}
		await new Promise((resolve) => setTimeout(resolve, 10));
		await tick();
	}
}

describe('the live why page names the line its belief lets it name', () => {
	it('sends the brand and the one category, no fit handle, and renders the pool', async () => {
		const edge = liveEdge({ recall_for_purchase: recallBodyWithPool() });
		vi.stubGlobal('fetch', edge.fetch);

		const host = render();
		await waitFor(() => host.querySelector('.ledger-pool') !== null, 'the pooled belief');

		expect(recallRequest(edge.calls)).toEqual({
			shopper_id: 'shopper_01',
			brand_id: 'brand-x-001',
			category: 'shirts'
		});
		expect(Object.hasOwn(recallRequest(edge.calls), 'fit_handle')).toBe(false);
		const pool = host.querySelector('.ledger-pool');
		expect(pool?.textContent).toContain('Earned');
		expect(pool?.textContent).toContain('runs small');
		expect(host.querySelector('[data-pool-note]')).toBeNull();
	});

	it('omits the category when the evidence rows disagree and says the line is ambiguous', async () => {
		const rows = structuredClone(beliefExample.belief.evidence);
		rows[2].item.category = 'jeans';
		const edge = liveEdge({
			ledger_belief: beliefBody({ evidence: rows }),
			recall_for_purchase: recallBodyWithoutPool('unavailable')
		});
		vi.stubGlobal('fetch', edge.fetch);

		const host = render();
		await waitFor(
			() => host.querySelector('[data-pool-note="ambiguous-line"]') !== null,
			'the ambiguous-line note'
		);

		expect(recallRequest(edge.calls)).toEqual({
			shopper_id: 'shopper_01',
			brand_id: 'brand-x-001'
		});
		expect(host.querySelector('[data-pool-note]')?.textContent).toContain('more than one category');
		expect(host.textContent ?? '').not.toContain('No pooled belief applies here');
	});

	it('says the pool was consulted and had nothing to say', async () => {
		const edge = liveEdge({ recall_for_purchase: recallBodyWithoutPool('consulted') });
		vi.stubGlobal('fetch', edge.fetch);

		const host = render();
		await waitFor(
			() => host.querySelector('[data-pool-note="consulted"]') !== null,
			'the consulted note'
		);
		expect(host.querySelector('[data-pool-note]')?.textContent).toContain('nothing to say yet');
	});

	it('says the pool was not consulted and why, never silently empty', async () => {
		const edge = liveEdge({ recall_for_purchase: recallBodyWithoutPool('unavailable') });
		vi.stubGlobal('fetch', edge.fetch);

		const host = render();
		await waitFor(
			() => host.querySelector('[data-pool-note="unavailable"]') !== null,
			'the unavailable note'
		);

		const note = host.querySelector('[data-pool-note]')?.textContent ?? '';
		expect(note).toContain('not consulted');
		expect(note).toContain('no pool index for brand-x-001 · shirts');
		expect(note).toContain('no single product line');
		expect(host.textContent ?? '').not.toContain('No pooled belief applies here');
	});

	it('names no brand for a seller belief and says the pool was not consulted', async () => {
		const edge = liveEdge({
			ledger_belief: beliefBody({
				subject: { kind: 'seller', id: 'seller-acme-01', name: 'Acme' }
			}),
			recall_for_purchase: recallBodyWithoutPool('unavailable')
		});
		vi.stubGlobal('fetch', edge.fetch);

		const host = render();
		await waitFor(
			() => host.querySelector('[data-pool-note="no-line"]') !== null,
			'the no-line note'
		);

		expect(recallRequest(edge.calls)).toEqual({ shopper_id: 'shopper_01' });
		expect(host.querySelector('[data-pool-note]')?.textContent).toContain('about a seller');
	});
});
