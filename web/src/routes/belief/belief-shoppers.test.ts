// One shopper at a time on the belief page. Each fixture answers only the
// shopper it was built for and rejects a request naming another, the
// standing fixture answers the shopper its frozen request names, the
// pooling toggle remounts when the recommendation case changes so no
// earlier shopper's result stays on screen, and the standing-correction
// empty state prints its sentence once. Vitest reports `$app/environment`
// as the server, so the browser guard and the request URL are supplied
// here; everything else below is the page's own wiring.

import '../../lib/ledger/test-setup';

import { afterEach, describe, expect, it, vi } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../../node_modules/svelte/src/index-client.js';
import standingRequest from '../../../../testdata/api/record_correction.request.example-standing.json';
import standingResponse from '../../../../testdata/api/record_correction.response.example-standing.json';
import { fixtureShopperId } from '$lib/api';
import {
	createFixtureCorrectionTransport,
	fixtureContradictedBeliefId,
	fixtureCorrectionText
} from '$lib/contest';
import { createFixturePoolingTransport } from '$lib/ledger';
import BeliefPage from './+page.svelte';

const otherShopper = standingRequest['shopper_id'];

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

async function openPage(): Promise<HTMLElement> {
	const host = document.createElement('div');
	document.body.appendChild(host);
	mounted.push({ comp: mount(BeliefPage, { target: host }), host });
	await waitFor(() => host.querySelector('#why-case') !== null, 'the case picker');
	return host;
}

function pickCase(host: HTMLElement, recallCase: string): void {
	const select = host.querySelector('#why-case') as HTMLSelectElement;
	select.value = recallCase;
	select.dispatchEvent(new Event('change', { bubbles: true }));
}

function poolingResult(host: HTMLElement): string | null {
	return host.querySelector('.ledger-pooling-result')?.textContent?.trim() ?? null;
}

function poolingChoice(host: HTMLElement): string {
	return host.querySelector('.ledger-pooling-choice')?.textContent?.trim() ?? '';
}

describe('the pooling fixture answers one shopper', () => {
	it('replays the frozen answers for the shopper they belong to', async () => {
		const transport = createFixturePoolingTransport(fixtureShopperId);
		const out = await transport.call('set_pooling', {
			shopper_id: fixtureShopperId,
			pooling: false
		});
		expect(out).toMatchObject({
			shopper_id: fixtureShopperId,
			pooling: false,
			rows_removed: 3
		});
	});

	it('rejects a case shopper whose answer the fixture does not carry', async () => {
		const transport = createFixturePoolingTransport(otherShopper);
		await expect(
			transport.call('set_pooling', { shopper_id: otherShopper, pooling: false })
		).rejects.toMatchObject({ code: 'unknown-request' });
	});

	it('never answers a request naming another shopper', async () => {
		const transport = createFixturePoolingTransport(fixtureShopperId);
		await expect(
			transport.call('set_pooling', { shopper_id: otherShopper, pooling: false })
		).rejects.toMatchObject({ code: 'unknown-request' });
	});
});

describe('the standing fixture answers the shopper its frozen request names', () => {
	it('replays the frozen standing answer for that shopper byte for byte', async () => {
		const transport = createFixtureCorrectionTransport(otherShopper);
		const answer = await transport.call('record_correction', {
			...standingRequest
		});
		expect(otherShopper).toBe('shopper_02');
		expect(answer).toEqual(standingResponse);
	});

	it('rejects a standing request naming another shopper', async () => {
		const transport = createFixtureCorrectionTransport(otherShopper);
		await expect(
			transport.call('record_correction', { shopper_id: fixtureShopperId, text: fixtureCorrectionText })
		).rejects.toMatchObject({ code: 'unknown-request' });
	});

	it('refuses to contradict the demo belief from another shopper', async () => {
		const transport = createFixtureCorrectionTransport(otherShopper);
		await expect(
			transport.call('record_correction', {
				shopper_id: otherShopper,
				text: fixtureCorrectionText,
				contradicts_belief_id: fixtureContradictedBeliefId
			})
		).rejects.toMatchObject({ code: 'unknown-request' });
	});
});

describe('the belief page keeps one shopper at a time', () => {
	it('prints the standing-correction empty state once', async () => {
		const host = await openPage();
		await waitFor(() => host.querySelector('.contest-preview') !== null, 'the standing form');
		const text = host.textContent ?? '';
		expect(text.match(/No belief yet:/g) ?? []).toHaveLength(1);
		expect(host.querySelector('.contest-preview')?.textContent).toContain(
			'No belief yet: your correction stands on its own and decides at once.'
		);
	});

	it('remounts the pooling toggle when the case changes', async () => {
		const host = await openPage();
		await waitFor(() => host.querySelector('.ledger-pooling') !== null, 'the pooling toggle');
		expect(poolingChoice(host)).toContain('Pooling is on');

		const optOut = host.querySelector('.ledger-pooling-button') as HTMLButtonElement;
		optOut.click();
		await waitFor(
			() => poolingResult(host) === 'Pooling is off: 3 existing fit rows removed.',
			"the demo shopper's answer"
		);

		pickCase(host, 'pool');
		await tick();
		expect((host.querySelector('#why-case') as HTMLSelectElement).value).toBe('pool');
		expect(poolingResult(host)).toBeNull();
		expect(poolingChoice(host)).toContain('Pooling is on');

		pickCase(host, 'correction');
		await tick();
		expect(poolingResult(host)).toBeNull();
		expect(poolingChoice(host)).toContain('Pooling is on');
	});

	it('remounts the standing editor when the case changes', async () => {
		const host = await openPage();
		await waitFor(() => host.querySelector('.contest-editor') !== null, 'the standing form');

		const input = () => host.querySelector('.contest-input') as HTMLTextAreaElement;
		input().value = 'Use M from now on';
		input().dispatchEvent(new Event('input', { bubbles: true }));
		host
			.querySelector('.contest-editor')!
			.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
		await waitFor(
			() => host.querySelector('.contest-done') !== null,
			"the demo shopper's standing confirmation"
		);

		pickCase(host, 'pool');
		await tick();
		expect(host.querySelector('.contest-done')).toBeNull();
		expect(input().value).toBe('');
	});

	it('shows no pooling answer at all for a shopper the fixture does not carry', async () => {
		const host = await openPage();
		pickCase(host, 'pool');
		await tick();
		(host.querySelector('.ledger-pooling-button') as HTMLButtonElement).click();
		await waitFor(
			() => host.querySelector('.ledger-pooling [role="alert"]') !== null,
			'the fail-closed answer'
		);
		expect(poolingResult(host)).toBeNull();
		expect(host.querySelector('.ledger-pooling')?.textContent).not.toContain('fit rows');
	});
});
