// The contest editor under a real mount: the form a judge sees, the
// blank-submit guard, the sending state, and the plain-words result.
// The source-contract test in contest.test.ts proves the strings exist;
// these tests prove the form behaves: every state renders sanely at
// phone width in both themes (theme variables only, no fixed colours).
// mount comes from the Svelte client entry by relative path because
// vitest resolves bare 'svelte' to the server build; a config-level
// resolve.conditions change is out of this task's scope.
import { afterEach, describe, expect, it } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../../node_modules/svelte/src/index-client.js';
import responseExample from '../../../../testdata/api/record_correction.response.example.json';
import {
	createFixtureTransport,
	fixtureShopperId,
	LedgerClient,
	type LedgerFixtureStep
} from '$lib/api';
import ContestEditor from './ContestEditor.svelte';
import { CorrectionClient } from './client';
import { createFixtureCorrectionTransport, fixtureContradictedBeliefId } from './fixture';
import { ContestState, correctionPreviewLine } from './state';
import type { CorrectionTransport, RecordCorrectionResponse } from './types';

function clientAt(step: LedgerFixtureStep): LedgerClient {
	return new LedgerClient(createFixtureTransport(step));
}

async function freshRows(): Promise<ContestState> {
	const beliefs = await clientAt(4).listBeliefs({
		shopper_id: fixtureShopperId,
		include_seeded: true
	});
	return new ContestState(
		new CorrectionClient(createFixtureCorrectionTransport()),
		fixtureShopperId,
		beliefs.beliefs
	);
}

const mounted: { comp: object; host: HTMLElement }[] = [];

afterEach(() => {
	for (const { comp, host } of mounted.splice(0)) {
		unmount(comp as Parameters<typeof unmount>[0]);
		host.remove();
	}
});

function renderEditor(contest: ContestState): HTMLElement {
	const host = document.createElement('div');
	document.body.appendChild(host);
	const comp = mount(ContestEditor, {
		target: host,
		props: { contest, beliefId: fixtureContradictedBeliefId, currentValue: 'runs small, take size L' }
	});
	mounted.push({ comp, host });
	return host;
}

function typeInto(host: HTMLElement, value: string): void {
	const area = host.querySelector('textarea')!;
	area.value = value;
	area.dispatchEvent(new Event('input', { bubbles: true }));
}

async function submitForm(host: HTMLElement): Promise<void> {
	host
		.querySelector('form')!
		.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
	await tick();
	await tick();
	await tick();
}

describe('contest editor behaviour', () => {
	it('renders the whole form with nothing yet to report', async () => {
		const host = renderEditor(await freshRows());
		await tick();
		expect(host.querySelector('label')?.textContent).toBe('Contest this belief');
		expect(host.querySelector('.contest-current')?.textContent).toContain('runs small, take size L');
		expect(host.querySelector('.contest-preview')?.textContent).toBe(correctionPreviewLine);
		expect(host.querySelector('textarea')?.getAttribute('placeholder')).toBe(
			"I've lost weight, use M"
		);
		const button = host.querySelector('button[type="submit"]')!;
		expect(button.textContent).toBe('Send correction');
		expect(button.hasAttribute('disabled')).toBe(false);
		expect(host.querySelector('[role="alert"]')).toBeNull();
		expect(host.querySelector('[role="status"]')).toBeNull();
		expect(host.innerHTML).not.toContain('// The shopper');
	});

	it('refuses a blank edit in plain words and sends nothing', async () => {
		const contest = await freshRows();
		const host = renderEditor(contest);
		await tick();
		await submitForm(host);
		expect(host.querySelector('[role="alert"]')?.textContent).toBe(
			'Write the correction in your own words.'
		);
		expect(host.querySelector('[role="status"]')).toBeNull();
		expect(contest.status).toBe('idle');
	});

	it('confirms a sent correction in plain words and clears the box', async () => {
		const contest = await freshRows();
		const host = renderEditor(contest);
		await tick();
		typeInto(host, "I've lost weight, use M");
		await tick();
		await submitForm(host);
		const done = host.querySelector('[role="status"]');
		expect(done?.textContent).toBe(
			'Your correction now decides: “I\'ve lost weight, use M”. The old belief stays below for the record.'
		);
		expect(host.querySelector('textarea')!.value).toBe('');
		expect(host.querySelector('[role="alert"]')).toBeNull();
		expect(contest.decisionFor(fixtureContradictedBeliefId)).toBe("I've lost weight, use M");
	});

	it('holds the button while the correction is in flight', async () => {
		let release!: (value: unknown) => void;
		const gated: CorrectionTransport = {
			call: () => new Promise<unknown>((resolve) => (release = resolve))
		};
		const beliefs = await clientAt(4).listBeliefs({
			shopper_id: fixtureShopperId,
			include_seeded: true
		});
		const contest = new ContestState(
			new CorrectionClient(gated),
			fixtureShopperId,
			beliefs.beliefs
		);
		const host = renderEditor(contest);
		await tick();
		typeInto(host, "I've lost weight, use M");
		await tick();
		host
			.querySelector('form')!
			.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
		await tick();
		await tick();
		const button = host.querySelector('button[type="submit"]')!;
		expect(button.textContent).toBe('Sending…');
		expect(button.hasAttribute('disabled')).toBe(true);
		release(responseExample as unknown as RecordCorrectionResponse);
		await tick();
		await tick();
		await tick();
		expect(host.querySelector('[role="status"]')?.textContent).toContain(
			'Your correction now decides'
		);
		expect(host.querySelector('button[type="submit"]')!.hasAttribute('disabled')).toBe(false);
	});

	it('says a failed correction failed and lets the judge retry', async () => {
		const failing: CorrectionTransport = {
			call: async () => {
				throw new Error('The correction failed.');
			}
		};
		const beliefs = await clientAt(4).listBeliefs({
			shopper_id: fixtureShopperId,
			include_seeded: true
		});
		const contest = new ContestState(
			new CorrectionClient(failing),
			fixtureShopperId,
			beliefs.beliefs
		);
		const host = renderEditor(contest);
		await tick();
		typeInto(host, "I've lost weight, use M");
		await tick();
		await submitForm(host);
		expect(host.querySelector('[role="alert"]')?.textContent).toBe('The correction failed.');
		expect(host.querySelector('[role="status"]')).toBeNull();
		expect(host.querySelector('button[type="submit"]')!.hasAttribute('disabled')).toBe(false);
	});
});
