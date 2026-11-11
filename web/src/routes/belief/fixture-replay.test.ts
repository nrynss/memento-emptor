// The fixture replay is unchanged by the live line: a page with no shopper
// link still asks its fixture for the demo shopper alone and renders the
// frozen answer. The line the live page names belongs to a linked read; a
// replay that carried it would send a line its fixture never answers, so
// this pins the request body and the answer on screen.

import '../../lib/ledger/test-setup';

import { afterEach, describe, expect, it, vi } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../../node_modules/svelte/src/index-client.js';
import { fixtureShopperId } from '$lib/api';
import type { RecallCase, RecallToolName } from '$lib/ledger';
import BeliefPage from './+page.svelte';

const request = vi.hoisted(() => ({ url: new URL('http://localhost/belief') }));
const replayed = vi.hoisted(() => [] as Record<string, unknown>[]);
const fetched = vi.hoisted(() => [] as string[]);

vi.mock('$app/environment', () => ({ browser: true }));
vi.mock('$app/state', () => ({
	page: {
		get url() {
			return request.url;
		}
	}
}));

vi.mock('$lib/ledger', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/ledger')>();
	const real = actual.createFixtureRecallTransport;
	return {
		...actual,
		createFixtureRecallTransport: (recallCase: RecallCase) => {
			const inner = real(recallCase);
			return {
				call: (tool: RecallToolName, body: Record<string, unknown>) => {
					replayed.push({ ...body });
					return inner.call(tool, body);
				}
			};
		}
	};
});

const mounted: { comp: object; host: HTMLElement }[] = [];

afterEach(() => {
	for (const { comp, host } of mounted.splice(0)) {
		unmount(comp as Parameters<typeof unmount>[0]);
		host.remove();
	}
	replayed.length = 0;
	fetched.length = 0;
	vi.unstubAllGlobals();
});

function render(): HTMLElement {
	request.url = new URL('http://localhost/belief');
	vi.stubGlobal('fetch', (input: RequestInfo | URL) => {
		fetched.push(String(input));
		return Promise.reject(new Error('fixture mode must not read the edge'));
	});
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

describe('the fixture replay asks its fixture for the shopper alone', () => {
	it('sends no line and renders the frozen answer', async () => {
		const host = render();
		await waitFor(() => host.querySelector('.ledger-pool') !== null, 'the replayed pool');

		expect(replayed).toEqual([{ shopper_id: fixtureShopperId }]);
		expect(fetched).toEqual([]);
		expect(host.querySelector('[data-why-source="replay"]')).not.toBeNull();
		const decision = host.querySelector('.ledger-decision-text')?.textContent ?? '';
		expect(decision).toContain('Take size M for Brand X');
		const pool = host.querySelector('.ledger-pool')?.textContent ?? '';
		expect(pool).toContain('Earned');
		expect(pool).toContain('runs small');
		expect(host.querySelector('[data-pool-note]')).toBeNull();
	});
});
