// The why page's source: a shopper link reads the belief, its evidence, its
// pending questions and the recommendation from the live layer through the
// edge, and says so; a failed live read is labelled as a failure with no
// answer on screen, never filled in from the fixture story.

import '../../lib/ledger/test-setup';

import { afterEach, describe, expect, it, vi } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../../node_modules/svelte/src/index-client.js';
import beliefExample from '../../../../testdata/api/ledger_belief.response.example.json';
import beliefsExample from '../../../../testdata/api/ledger_beliefs.response.example.json';
import evidenceExample from '../../../../testdata/api/ledger_evidence.response.example.json';
import pendingExample from '../../../../testdata/api/pending_questions.response.example.json';
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

/** The frozen answers, served by tool path; a named tool can refuse. */
function liveEdge(refuse: string | null = null): { calls: Call[]; fetch: typeof fetch } {
	const calls: Call[] = [];
	const bodies: Record<string, unknown> = {
		ledger_beliefs: beliefsExample,
		ledger_belief: beliefExample,
		ledger_evidence: evidenceExample,
		pending_questions: pendingExample,
		recall_for_purchase: recallExample
	};
	const send = ((input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input);
		calls.push({ url, init });
		const tool = url.split('/').pop() ?? '';
		if (tool === refuse) {
			return Promise.resolve({
				ok: false,
				status: 401,
				text: () =>
					Promise.resolve('{"error":{"code":"unauthenticated","message":"unauthorized"}}')
			} as unknown as Response);
		}
		return Promise.resolve({
			ok: true,
			status: 200,
			text: () => Promise.resolve(JSON.stringify(bodies[tool] ?? {}))
		} as unknown as Response);
	}) as typeof globalThis.fetch;
	return { calls, fetch: send };
}

const mounted: { comp: object; host: HTMLElement }[] = [];

afterEach(() => {
	for (const { comp, host } of mounted.splice(0)) {
		unmount(comp as Parameters<typeof unmount>[0]);
		host.remove();
	}
	vi.unstubAllGlobals();
});

function render(url: string): HTMLElement {
	request.url = new URL(url);
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

describe('the why page reads live from a shopper link', () => {
	it('renders the belief, its evidence and the recommendation for the link', async () => {
		const edge = liveEdge();
		vi.stubGlobal('fetch', edge.fetch);

		const host = render(`http://localhost/belief?t=${token}&s=shopper_01`);
		await waitFor(() => host.querySelector('.why-value') !== null, 'the live belief');
		await waitFor(
			() => (host.textContent ?? '').includes('Take size M for Brand X'),
			'the live recommendation'
		);

		expect(host.querySelector('.why-value')?.textContent).toBe('runs small, take size L');
		expect(host.querySelector('[data-why-source="live"]')?.textContent).toContain('shopper_01');
		expect(host.querySelector('[data-why-source="replay"]')).toBeNull();
		// The fixture pickers have no meaning against the live layer.
		expect(host.querySelector('#why-step')).toBeNull();
		expect(host.querySelector('#why-case')).toBeNull();
		// The evidence section carries the live rows.
		expect(host.querySelector('.why-counted')?.textContent).toContain('3');

		const tools = edge.calls.map((call) => call.url.split('/').pop());
		expect(tools).toContain('ledger_beliefs');
		expect(tools).toContain('ledger_belief');
		expect(tools).toContain('ledger_evidence');
		expect(tools).toContain('pending_questions');
		expect(tools).toContain('recall_for_purchase');
		for (const call of edge.calls) {
			const headers = call.init?.headers as Record<string, string>;
			expect(headers['Authorization']).toBe(`Bearer ${token}`);
			expect(JSON.parse(String(call.init?.body))['shopper_id']).toBe('shopper_01');
		}
		expect(host.querySelector('a[href^="/ledger?t="]')?.getAttribute('href')).toBe(
			`/ledger?t=${token}&s=shopper_01`
		);
		expect(host.textContent ?? '').not.toContain(token);
	});

	it('labels a failed recommendation read and fills it in from nothing', async () => {
		const edge = liveEdge('recall_for_purchase');
		vi.stubGlobal('fetch', edge.fetch);

		const host = render(`http://localhost/belief?t=${token}&s=shopper_01`);
		await waitFor(() => host.querySelector('.why-value') !== null, 'the live belief');
		await waitFor(
			() => (host.textContent ?? '').includes('live recommendation read failed'),
			'the recommendation failure label'
		);

		expect(host.textContent ?? '').not.toContain('Take size M for Brand X');
		expect(host.textContent ?? '').toContain('No recommendation is shown');
	});

	it('labels a failed belief read and shows no belief', async () => {
		const edge = liveEdge('ledger_beliefs');
		vi.stubGlobal('fetch', edge.fetch);

		const host = render(`http://localhost/belief?t=${token}&s=shopper_09`);
		await waitFor(
			() => (host.textContent ?? '').includes('live why view read failed'),
			'the belief failure label'
		);

		// The recommendation read is independent and did answer here, so the
		// alert names what failed rather than claiming the page is all fixture.
		expect(host.textContent ?? '').toContain('No belief card is shown');
		expect(host.querySelector('.why-value')).toBeNull();
		expect(host.textContent ?? '').not.toContain(token);
	});

	it('claims no live source when nothing live answered', async () => {
		const send = (() =>
			Promise.resolve({
				ok: false,
				status: 401,
				text: () => Promise.resolve('{"error":{"code":"unauthenticated","message":"unauthorized"}}')
			} as unknown as Response)) as typeof globalThis.fetch;
		vi.stubGlobal('fetch', send);

		const host = render(`http://localhost/belief?t=${token}&s=shopper_09`);
		await waitFor(
			() => (host.textContent ?? '').includes('live recommendation read failed'),
			'the recommendation failure label'
		);
		await waitFor(
			() => (host.textContent ?? '').includes('live why view read failed'),
			'the belief failure label'
		);

		expect(host.querySelector('[data-why-source="live"]')).toBeNull();
		expect(host.querySelector('[data-why-source="replay"]')).toBeNull();
		expect(host.querySelector('.why-value')).toBeNull();
		expect(host.textContent ?? '').toContain('No recommendation is shown');
		expect(host.textContent ?? '').not.toContain(token);
	});
});
