// The contest route end to end: a shopper opens the editor from the
// ledger's Contest button, submits a correction, and sees the layer's
// answer with the contradicted belief and its evidence still on screen.
// The same test pins the ledger's why link and the why page's way back,
// and the deep-link helpers that make the link land on the right belief.

import './test-setup';

import { afterEach, describe, expect, it } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../../node_modules/svelte/src/index-client.js';
import LedgerPage from '../../routes/ledger/+page.svelte';
import BeliefPage from '../../routes/belief/+page.svelte';
import {
	fixtureContradictedBeliefId,
	fixtureCorrectionText
} from '$lib/contest';
import { linkedBeliefId, pickBeliefId } from './deeplink';

const mounted: { comp: object; host: HTMLElement }[] = [];

afterEach(() => {
	for (const { comp, host } of mounted.splice(0)) {
		unmount(comp as Parameters<typeof unmount>[0]);
		host.remove();
	}
});

function render(component: Parameters<typeof mount>[0], host: HTMLElement): void {
	document.body.appendChild(host);
	mounted.push({ comp: mount(component, { target: host }), host });
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

describe('the contest route', () => {
	it('opens the editor from a ledger row, applies the answer, and keeps the belief', async () => {
		const host = document.createElement('div');
		render(LedgerPage, host);

		await waitFor(() => host.querySelector('#ledger-step') !== null, 'the step picker');
		const picker = host.querySelector('#ledger-step') as HTMLSelectElement;
		picker.value = '4';
		picker.dispatchEvent(new Event('change', { bubbles: true }));
		await waitFor(
			() => rowFor(host, fixtureContradictedBeliefId)?.textContent?.includes('Earned') === true,
			'the Earned sizing row'
		);

		const before = rowFor(host, fixtureContradictedBeliefId)!;
		expect(before.querySelector('[col-id="value"]')?.textContent).toContain(
			'runs small, take size L'
		);
		expect(before.querySelector('[col-id="evidenceCount"]')?.textContent).toContain('3');
		expect(before.querySelector('[col-id="seeded"]')?.textContent).toContain('Seeded');
		expect(host.querySelectorAll('.ag-row[row-id]').length).toBe(2);

		const button = before.querySelector('[data-contest-belief]') as HTMLButtonElement | null;
		expect(button?.textContent).toBe('Contest');
		button?.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
		await waitFor(() => host.querySelector('form.contest-editor') !== null, 'the contest editor');

		const form = host.querySelector('form.contest-editor') as HTMLFormElement;
		expect(form.id).toBe(`contest-${fixtureContradictedBeliefId}`);
		expect(host.querySelector('label.contest-label')?.textContent).toBe('Contest this belief');
		expect(host.querySelector('.contest-current')?.textContent).toContain('runs small, take size L');
		const textarea = host.querySelector('textarea') as HTMLTextAreaElement;
		expect(textarea.id).toBe(`contest-${fixtureContradictedBeliefId}-text`);

		textarea.value = fixtureCorrectionText;
		textarea.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();
		form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
		await waitFor(() => host.querySelector('[role="status"]') !== null, 'the confirmation');
		expect(host.querySelector('[role="status"]')?.textContent).toBe(
			`Your correction now decides: “${fixtureCorrectionText}”. The old belief stays below for the record.`
		);

		await waitFor(
			() => rowFor(host, fixtureContradictedBeliefId)?.textContent?.includes('Out of true') === true,
			'the contradicted row'
		);
		const after = rowFor(host, fixtureContradictedBeliefId)!;
		expect(after.querySelector('[col-id="status"] .ledger-out-of-true')?.textContent).toBe(
			'Out of true'
		);
		expect(after.querySelector('.ledger-correction')?.textContent).toContain(fixtureCorrectionText);
		expect(after.querySelector('[col-id="value"]')?.textContent).toContain(
			'runs small, take size L'
		);
		expect(after.querySelector('[col-id="evidenceCount"]')?.textContent).toContain('3');
		expect(after.querySelector('[col-id="seeded"]')?.textContent).toContain('Seeded');
		expect(host.querySelectorAll('.ag-row[row-id]').length).toBe(2);
	});

	it('links each ledger row to its why view and the why page back to the ledger', async () => {
		const ledgerHost = document.createElement('div');
		render(LedgerPage, ledgerHost);
		await waitFor(
			() =>
				ledgerHost.querySelector(
					`.ag-row[row-id="${fixtureContradictedBeliefId}"] a.ledger-explain`
				) !== null,
			'the ledger rows'
		);
		const explain = ledgerHost.querySelector(
			`.ag-row[row-id="${fixtureContradictedBeliefId}"] a.ledger-explain`
		) as HTMLAnchorElement | null;
		expect(explain?.getAttribute('href')).toBe(`/belief?belief=${fixtureContradictedBeliefId}`);

		const beliefHost = document.createElement('div');
		render(BeliefPage, beliefHost);
		await waitFor(
			() => beliefHost.querySelector('a[href="/ledger"]') !== null,
			'the way back to the ledger'
		);
	});
});

describe('why-view deep links', () => {
	it('reads the belief from a link and ignores an empty or absent one', () => {
		expect(linkedBeliefId('/belief?belief=blf_8f2c41')).toBe('blf_8f2c41');
		expect(linkedBeliefId(new URL('http://localhost/belief?belief=blf_2b9d60'))).toBe(
			'blf_2b9d60'
		);
		expect(linkedBeliefId('/belief?belief=')).toBeNull();
		expect(linkedBeliefId('/belief')).toBeNull();
		expect(linkedBeliefId(null)).toBeNull();
	});

	it('keeps the linked belief when the step has it, else falls back', () => {
		const available = ['blf_8f2c41', 'blf_2b9d60'];
		expect(pickBeliefId('blf_2b9d60', available, 'blf_8f2c41')).toBe('blf_2b9d60');
		expect(pickBeliefId('blf_missing', available, 'blf_8f2c41')).toBe('blf_8f2c41');
		expect(pickBeliefId(null, ['blf_2b9d60'], 'blf_8f2c41')).toBe('blf_2b9d60');
		expect(pickBeliefId(null, [], 'blf_8f2c41')).toBeNull();
	});
});
