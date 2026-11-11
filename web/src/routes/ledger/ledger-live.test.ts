// The ledger page's source: a shopper link reads the live layer through the
// edge and says so, a failed live read is labelled as a failure with no data
// on screen, and a page with no token replays the demo story and says that
// instead. The token never reaches the page.

import '../../lib/ledger/test-setup';

import { afterEach, describe, expect, it, vi } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../../node_modules/svelte/src/index-client.js';
import beliefsExample from '../../../../testdata/api/ledger_beliefs.response.example.json';
import pendingExample from '../../../../testdata/api/pending_questions.response.example.json';
import LedgerPage from './+page.svelte';

const request = vi.hoisted(() => ({ url: new URL('http://localhost/ledger') }));

vi.mock('$app/environment', () => ({ browser: true }));
vi.mock('$app/state', () => ({
	page: {
		get url() {
			return request.url;
		}
	}
}));

const token = 'memento_shop_page_token';

interface Call {
	url: string;
	init: RequestInit | undefined;
}

/** The frozen read answers, served by tool path. A named tool can refuse. */
function liveEdge(refuse: string | null = null): { calls: Call[]; fetch: typeof fetch } {
	const calls: Call[] = [];
	const bodies: Record<string, unknown> = {
		ledger_beliefs: beliefsExample,
		pending_questions: pendingExample
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
	mounted.push({ comp: mount(LedgerPage, { target: host }), host });
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

function rowFor(host: HTMLElement, beliefId: string): HTMLElement | null {
	return host.querySelector(`.ag-row[row-id="${beliefId}"]`);
}

describe('the ledger page reads live from a shopper link', () => {
	it('renders the layer rows for the link and says they are live', async () => {
		const edge = liveEdge();
		vi.stubGlobal('fetch', edge.fetch);

		const host = render(`http://localhost/ledger?t=${token}&s=shopper_01`);
		await waitFor(
			() => rowFor(host, 'blf_8f2c41')?.textContent?.includes('runs small, take size L') === true,
			'the live sizing row'
		);

		await waitFor(
			() =>
				rowFor(host, 'blf_8f2c41')?.querySelector('.ledger-out-of-true') !== null,
			'the live status pill'
		);

		const source = host.querySelector('[data-ledger-source="live"]');
		expect(source?.textContent).toContain('Live');
		expect(source?.textContent).toContain('shopper_01');
		expect(host.querySelector('[data-ledger-source="replay"]')).toBeNull();
		expect(
			rowFor(host, 'blf_8f2c41')?.querySelector('[col-id="status"] .ledger-out-of-true')
				?.textContent
		).toBe('Out of true');
		// The two reads the table renders went to the edge with the link's
		// token and shopper. Evidence is not one of them: no ledger row
		// renders it, so the table never loads it.
		expect(edge.calls.map((call) => call.url.split('/').pop()).sort()).toEqual([
			'ledger_beliefs',
			'pending_questions'
		]);
		for (const call of edge.calls) {
			const headers = call.init?.headers as Record<string, string>;
			expect(headers['Authorization']).toBe(`Bearer ${token}`);
			expect(JSON.parse(String(call.init?.body))['shopper_id']).toBe('shopper_01');
		}
		// The reason link carries the shopper link, so the why view it opens
		// reads live too. The page itself prints no token.
		const why = rowFor(host, 'blf_8f2c41')?.querySelector('a.ledger-explain') as
			| HTMLAnchorElement
			| null
			| undefined;
		expect(why?.getAttribute('href')).toBe(
			`/belief?belief=blf_8f2c41&t=${token}&s=shopper_01`
		);
		expect(host.textContent ?? '').not.toContain(token);
	});

	it('shows the table when an evidence read would refuse, because it never asks', async () => {
		// The why view reads a belief's evidence for the one belief it
		// explains. The table renders none of it, so a refused evidence read
		// cannot label the page failed over data no row shows.
		const edge = liveEdge('ledger_evidence');
		vi.stubGlobal('fetch', edge.fetch);

		const host = render(`http://localhost/ledger?t=${token}&s=shopper_01`);
		await waitFor(
			() => rowFor(host, 'blf_8f2c41')?.textContent?.includes('runs small, take size L') === true,
			'the live sizing row'
		);
		await waitFor(
			() => host.querySelector('[data-ledger-source="live"]')?.textContent?.includes('Live') === true,
			'the live source label'
		);

		expect(host.querySelector('[role="alert"]')).toBeNull();
		expect(edge.calls.map((call) => call.url.split('/').pop())).not.toContain('ledger_evidence');
	});

	it('labels a failed live read as a failure and shows no data', async () => {
		const send = (() =>
			Promise.resolve({
				ok: false,
				status: 401,
				text: () => Promise.resolve('{"error":{"code":"unauthenticated","message":"unauthorized"}}')
			} as unknown as Response)) as typeof globalThis.fetch;
		vi.stubGlobal('fetch', send);

		const host = render(`http://localhost/ledger?t=${token}&s=shopper_09`);
		await waitFor(() => host.querySelector('[role="alert"]') !== null, 'the failure label');

		const alert = (host.querySelector('[role="alert"]')?.textContent ?? '').replace(/\s+/g, ' ');
		expect(alert).toContain('live ledger read failed');
		expect(alert).toContain('unauthorized');
		expect(alert).toContain('Nothing on this page is live data');
		expect(host.querySelector('[data-ledger-source="live"]')).toBeNull();
		expect(host.querySelector('[data-ledger-source="replay"]')).toBeNull();
		expect(host.querySelectorAll('.ag-row[row-id]')).toHaveLength(0);
		expect(host.textContent ?? '').not.toContain('runs small, take size L');
		expect(host.textContent ?? '').not.toContain(token);
	});
	it('reads again after a failure when the shopper asks', async () => {
		const edge = liveEdge();
		// A load is two reads, so the first two answers are refused and every
		// answer after the reader asks again is served.
		let asked = 0;
		const flaky = ((input: RequestInfo | URL, init?: RequestInit) => {
			asked += 1;
			if (asked <= 2) {
				return Promise.resolve({
					ok: false,
					status: 503,
					text: () => Promise.resolve('{"error":{"code":"internal","message":"try later"}}')
				} as unknown as Response);
			}
			return edge.fetch(input, init);
		}) as typeof globalThis.fetch;
		vi.stubGlobal('fetch', flaky);

		const host = render(`http://localhost/ledger?t=${token}&s=shopper_01`);
		await waitFor(() => host.querySelector('[role="alert"]') !== null, 'the failure label');
		const retry = [...host.querySelectorAll('[role="alert"] button')].find(
			(candidate) => candidate.textContent === 'Read again'
		);
		expect(retry).toBeDefined();
		retry?.dispatchEvent(new MouseEvent('click', { bubbles: true }));

		await waitFor(
			() => rowFor(host, 'blf_8f2c41')?.textContent?.includes('runs small, take size L') === true,
			'the live rows after a retry'
		);
		expect(host.querySelector('[role="alert"]')).toBeNull();
		expect(host.querySelector('[data-ledger-source="live"]')?.textContent).toContain('Live');
	});
});

describe('the ledger page without a link', () => {
	it('replays the demo story, labelled as a replay, with no live call', async () => {
		const fetch = vi.fn(() => Promise.reject(new Error('a replay must not call the edge')));
		vi.stubGlobal('fetch', fetch);

		const host = render('http://localhost/ledger');
		await waitFor(
			() => rowFor(host, 'blf_8f2c41')?.textContent?.includes('runs small, take size L') === true,
			'the fixture sizing row'
		);

		expect(host.querySelector('[data-ledger-source="replay"]')?.textContent).toContain(
			'Fixture replay'
		);
		expect(host.querySelector('[data-ledger-source="live"]')).toBeNull();
		expect(host.querySelector('#ledger-step')).not.toBeNull();
		expect(fetch).not.toHaveBeenCalled();
	});
});
