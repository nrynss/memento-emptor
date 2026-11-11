// The why view arriving from a ledger link. The page reads the belief named
// in the query when the browser has the URL, explains that belief, and falls
// back to the page's preferred belief when the link names none the step
// carries. Vitest reports `$app/environment` as the server, so the browser
// guard and the request URL are supplied here; everything else below is the
// page's own wiring.

import './test-setup';

import { afterEach, describe, expect, it, vi } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../../node_modules/svelte/src/index-client.js';
import BeliefPage from '../../routes/belief/+page.svelte';

const request = vi.hoisted(() => ({ url: new URL('http://localhost/belief') }));

vi.mock('$app/environment', () => ({ browser: true }));
vi.mock('$app/state', () => ({
	page: {
		get url() {
			return request.url;
		}
	}
}));

const mounted: { comp: object; host: HTMLElement }[] = [];

afterEach(() => {
	for (const { comp, host } of mounted.splice(0)) {
		unmount(comp as Parameters<typeof unmount>[0]);
		host.remove();
	}
});

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

async function explain(url: string): Promise<HTMLElement> {
	request.url = new URL(url);
	const host = document.createElement('div');
	document.body.appendChild(host);
	mounted.push({ comp: mount(BeliefPage, { target: host }), host });
	await waitFor(() => host.querySelector('#why-belief') !== null, 'the belief picker');
	return host;
}

function pickedBelief(host: HTMLElement): string {
	return (host.querySelector('#why-belief') as HTMLSelectElement).value;
}

describe('the why view from a ledger link', () => {
	it('explains the belief the link names', async () => {
		const host = await explain('http://localhost/belief?belief=blf_2b9d60');
		await waitFor(() => pickedBelief(host) === 'blf_2b9d60', 'the linked belief');
		await waitFor(() => host.querySelector('.why-value') !== null, 'the explained belief');
		expect(host.querySelector('.why-value')?.textContent).toBe('size M fits this shopper now');
	});

	it('falls back to the preferred belief for an unknown belief id', async () => {
		const host = await explain('http://localhost/belief?belief=blf_missing');
		await waitFor(() => pickedBelief(host) !== '', 'a chosen belief');
		expect(pickedBelief(host)).toBe('blf_8f2c41');
		expect(host.querySelector('[role="alert"]')).toBeNull();
		await waitFor(() => host.querySelector('.why-value') !== null, 'the explained belief');
		expect(host.querySelector('.why-value')?.textContent).toBe('runs small, take size L');
	});

	it('explains the preferred belief when the link names none', async () => {
		const host = await explain('http://localhost/belief');
		await waitFor(() => pickedBelief(host) !== '', 'a chosen belief');
		expect(pickedBelief(host)).toBe('blf_8f2c41');
	});
});
