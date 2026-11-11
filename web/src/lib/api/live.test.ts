// Where a page reads from: the link's token and shopper, and the live state
// they build. A link with a token reads the live layer through the edge; a
// URL without one is a replay, and the page says so instead of showing live.

import { afterEach, describe, expect, it, vi } from 'vitest';
import beliefsExample from '../../../../testdata/api/ledger_beliefs.response.example.json';
import evidenceExample from '../../../../testdata/api/ledger_evidence.response.example.json';
import pendingExample from '../../../../testdata/api/pending_questions.response.example.json';
import { defaultLiveShopper, createLiveState, liveLinkFrom } from './live';

interface Call {
	url: string;
	init: RequestInit | undefined;
}

/** Serves the frozen read examples by tool, recording what arrived. A named tool can refuse. */
function stubLiveEdge(refuse: string | null = null): Call[] {
	const calls: Call[] = [];
	const bodies: Record<string, unknown> = {
		ledger_beliefs: beliefsExample,
		ledger_evidence: evidenceExample,
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
		const body = JSON.stringify(bodies[tool] ?? {});
		return Promise.resolve({
			ok: true,
			status: 200,
			text: () => Promise.resolve(body)
		} as unknown as Response);
	}) as typeof globalThis.fetch;
	vi.stubGlobal('fetch', send);
	return calls;
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('the shopper link', () => {
	it('reads the token and the shopper it acts for', () => {
		expect(liveLinkFrom('http://localhost/ledger?t=memento_shop_abc')).toEqual({
			token: 'memento_shop_abc',
			shopperId: defaultLiveShopper
		});
		expect(liveLinkFrom('http://localhost/ledger?t=memento_shop_abc&s=shopper_09')).toEqual({
			token: 'memento_shop_abc',
			shopperId: 'shopper_09'
		});
	});

	it('accepts a URL object and a relative path', () => {
		expect(liveLinkFrom(new URL('http://localhost/ledger?t=tok'))?.token).toBe('tok');
		expect(liveLinkFrom('/ledger?t=tok&s=shopper_02')).toEqual({
			token: 'tok',
			shopperId: 'shopper_02'
		});
	});

	it('is null when the page carries no usable token', () => {
		expect(liveLinkFrom(null)).toBeNull();
		expect(liveLinkFrom(undefined)).toBeNull();
		expect(liveLinkFrom('/ledger')).toBeNull();
		expect(liveLinkFrom('/ledger?s=shopper_02')).toBeNull();
		expect(liveLinkFrom('/ledger?t=%20%20')).toBeNull();
	});
});

describe('the live ledger state', () => {
	it('reads the rows the table renders from the edge as its shopper', async () => {
		const calls = stubLiveEdge();
		const state = createLiveState({ token: 'memento_shop_abc', shopperId: 'shopper_09' });
		await state.refresh();

		expect(state.status).toBe('ready');
		expect(state.beliefs).toHaveLength(4);
		expect(state.pending).toHaveLength(1);
		expect(calls.map((call) => call.url).sort()).toEqual([
			'/tools/ledger_beliefs',
			'/tools/pending_questions'
		]);
		for (const call of calls) {
			const headers = call.init?.headers as Record<string, string>;
			expect(headers['Authorization']).toBe('Bearer memento_shop_abc');
			expect(JSON.parse(String(call.init?.body))['shopper_id']).toBe('shopper_09');
		}
	});

	it('loads when an evidence read would refuse, because it never asks for one', async () => {
		const calls = stubLiveEdge('ledger_evidence');
		const state = createLiveState({ token: 'memento_shop_abc', shopperId: 'shopper_09' });
		await state.refresh();

		expect(state.status).toBe('ready');
		expect(state.error).toBeNull();
		expect(state.beliefs).toHaveLength(4);
		expect(calls.map((call) => call.url)).not.toContain('/tools/ledger_evidence');
	});

	it('reports a refused read as an error and keeps no rows', async () => {
		const send = (() =>
			Promise.resolve({
				ok: false,
				status: 401,
				text: () => Promise.resolve('{"error":{"code":"unauthenticated","message":"unauthorized"}}')
			} as unknown as Response)) as typeof globalThis.fetch;
		vi.stubGlobal('fetch', send);
		const state = createLiveState({ token: 'memento_shop_abc', shopperId: 'shopper_09' });
		await state.refresh();

		expect(state.status).toBe('error');
		expect(state.error).toBe('unauthenticated: unauthorized');
		expect(state.beliefs).toEqual([]);
		expect(state.pending).toEqual([]);
	});
});
