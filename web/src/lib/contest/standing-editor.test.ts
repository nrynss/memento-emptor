// The contest editor with no belief: the standing mode records a
// correction that stands on its own, confirms it in plain words, and
// hands the layer's answer to the host so the ledger can list it.
import { afterEach, describe, expect, it, vi } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../../node_modules/svelte/src/index-client.js';
import { fixtureShopperId } from '$lib/api';
import ContestEditor from './ContestEditor.svelte';
import { CorrectionClient } from './client';
import { createFixtureCorrectionTransport } from './fixture';
import { ContestState } from './state';
import type { RecordCorrectionResponse } from './types';

function freshState(): ContestState {
	return new ContestState(
		new CorrectionClient(createFixtureCorrectionTransport()),
		fixtureShopperId,
		[]
	);
}

const mounted: { comp: object; host: HTMLElement }[] = [];

afterEach(() => {
	for (const { comp, host } of mounted.splice(0)) {
		unmount(comp as Parameters<typeof unmount>[0]);
		host.remove();
	}
});

function renderStanding(
	contest: ContestState,
	onRecorded: (response: RecordCorrectionResponse) => void
): HTMLElement {
	const host = document.createElement('div');
	document.body.appendChild(host);
	const comp = mount(ContestEditor, {
		target: host,
		props: { contest, onRecorded }
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

describe('contest editor standing mode', () => {
	it('offers a standing correction with no belief on the form', async () => {
		const host = renderStanding(freshState(), () => {});
		await tick();
		expect(host.querySelector('label')?.textContent).toBe('Record a standing correction');
		expect(host.querySelector('.contest-preview')?.textContent).toContain('stands on its own');
		expect(host.querySelector('.contest-current')).toBeNull();
		expect(host.querySelector('button[type="submit"]')?.textContent).toBe('Send correction');
	});

	it('refuses a blank standing correction and sends nothing', async () => {
		const contest = freshState();
		const host = renderStanding(contest, () => {});
		await tick();
		await submitForm(host);
		expect(host.querySelector('[role="alert"]')?.textContent).toBe(
			'Write the correction in your own words.'
		);
		expect(contest.standingCorrections).toEqual([]);
	});

	it('records the standing correction and hands it to the host', async () => {
		const contest = freshState();
		const recorded: RecordCorrectionResponse[] = [];
		const host = renderStanding(contest, (response) => {
			recorded.push(response);
		});
		await tick();
		typeInto(host, 'Use M from now on');
		await tick();
		await submitForm(host);
		expect(host.querySelector('[role="status"]')?.textContent).toContain(
			'Your correction now decides'
		);
		expect(host.querySelector('textarea')!.value).toBe('');
		expect(recorded).toHaveLength(1);
		expect(recorded[0].belief_id).toBeUndefined();
		expect(recorded[0].correction.text).toBe('Use M from now on');
		expect(contest.standingCorrections.map((correction) => correction.text)).toEqual([
			'Use M from now on'
		]);
		expect(contest.beliefs).toEqual([]);
	});

	it('leaves the declared path alone when a belief is given', async () => {
		const onRecorded = vi.fn();
		const host = document.createElement('div');
		document.body.appendChild(host);
		const comp = mount(ContestEditor, {
			target: host,
			props: {
				contest: freshState(),
				beliefId: 'blf_8f2c41',
				currentValue: 'runs small, take size L',
				onRecorded
			}
		});
		mounted.push({ comp, host });
		await tick();
		expect(host.querySelector('label')?.textContent).toBe('Contest this belief');
		expect(host.querySelector('.contest-current')?.textContent).toContain('runs small');
	});
});
