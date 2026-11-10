// The components the ledger shows for the pivot: which precedence step
// decided, the overridden step beside the one that won, the standing
// correction with no belief, the pooled row in counts only, and the
// pooling toggle with the rows removed shown.
import { afterEach, describe, expect, it } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../../node_modules/svelte/src/index-client.js';
import DecisionLine from './DecisionLine.svelte';
import PoolCounts from './PoolCounts.svelte';
import PoolingToggle from './PoolingToggle.svelte';
import StandingCorrection from './StandingCorrection.svelte';
import type { GuidanceBasis, OverriddenStep } from './precedence';
import { createFixturePoolingState } from './pooling';
import { fixtureDemoPool } from './recall';

const mounted: { comp: object; host: HTMLElement }[] = [];

afterEach(() => {
	for (const { comp, host } of mounted.splice(0)) {
		unmount(comp as Parameters<typeof unmount>[0]);
		host.remove();
	}
});

function render(component: object, props: Record<string, unknown>): HTMLElement {
	const host = document.createElement('div');
	document.body.appendChild(host);
	const comp = mount(component as Parameters<typeof mount>[0], {
		target: host,
		props
	});
	mounted.push({ comp, host });
	return host;
}

const winnerText: Record<GuidanceBasis, string> = {
	correction: 'Take size M for Brand X',
	own_evidence: 'Take size S for Brand X',
	pool: 'Size up to L for Brand X',
	statement: 'Your usual size M',
	none: 'No memory applies yet'
};

describe('decision line per precedence step', () => {
	for (const basis of Object.keys(winnerText) as GuidanceBasis[]) {
		it(`names ${basis} as the deciding step`, async () => {
			const host = render(DecisionLine, { text: winnerText[basis], basis, overridden: [] });
			await tick();
			const section = host.querySelector('section');
			expect(section?.getAttribute('aria-label')).toBe('What decided this recommendation');
			expect(section?.textContent).toContain(winnerText[basis]);
			if (basis === 'none') {
				expect(section?.textContent).toContain('No memory applies yet');
			} else {
				expect(section?.textContent).toContain('Decided by');
			}
			expect(host.innerHTML).not.toContain('// Which precedence');
		});
	}

	it('lists an overridden step beside the one that won', async () => {
		const overridden: OverriddenStep[] = [
			{ step: 'own_evidence', winner: 'correction' },
			{ step: 'pool', winner: 'correction' }
		];
		const host = render(DecisionLine, {
			text: winnerText['correction'],
			basis: 'correction' as GuidanceBasis,
			overridden
		});
		await tick();
		const text = host.textContent ?? '';
		expect(text).toContain('Decided by your correction.');
		expect(text).toContain('Your own fit evidence');
		expect(text).toContain('was overridden by your correction.');
		expect(text).toContain('Pooled product fit');
		const struck = host.querySelector('.ledger-decision-loser');
		expect(struck?.textContent).toBe('Your own fit evidence');
	});

	it('shows the unlike-the-crowd pair: the pool loses to own evidence', async () => {
		const host = render(DecisionLine, {
			text: winnerText['own_evidence'],
			basis: 'own_evidence' as GuidanceBasis,
			overridden: [{ step: 'pool', winner: 'own_evidence' }]
		});
		await tick();
		expect(host.textContent).toContain('Decided by your own fit evidence.');
		expect(host.textContent).toContain('Pooled product fit was overridden by your own fit evidence.');
	});
});

describe('standing correction', () => {
	it('renders the correction with no belief and its date', async () => {
		const host = render(StandingCorrection, {
			text: "I've lost weight, use M",
			recordedAt: '2026-11-09T10:12:00Z'
		});
		await tick();
		const section = host.querySelector('section');
		expect(section?.getAttribute('aria-label')).toBe('Standing correction');
		expect(section?.textContent).toContain("I've lost weight, use M");
		expect(section?.textContent).toContain('no belief yet');
		expect(section?.textContent).toContain('2026-11-09');
	});
});

describe('pooled row in counts only', () => {
	it('renders the counts with the seeded split and no shopper rows', async () => {
		const host = render(PoolCounts, { pool: fixtureDemoPool() });
		await tick();
		const text = host.textContent ?? '';
		expect(text).toContain('60 settled purchases');
		expect(text).toContain('24');
		expect(text).toContain('39');
		expect(text).toContain('48 of the 60 settled purchases are seeded history.');
		const html = host.innerHTML;
		for (const token of ['shopper_01', 'shopper_02', 'custom_id', 'event_id']) {
			expect(html).not.toContain(token);
		}
		expect(host.querySelector('section')?.getAttribute('aria-label')).toBe('Pooled product fit');
	});
});

describe('pooling toggle', () => {
	function renderToggle(initial = true): HTMLElement {
		const state = createFixturePoolingState('shopper_01', initial);
		return render(PoolingToggle, { pooling: state });
	}

	function clickButton(host: HTMLElement, label: string): void {
		const button = [...host.querySelectorAll('button')].find(
			(candidate) => candidate.textContent === label
		);
		if (!button) throw new Error(`no button labelled ${label}`);
		button.dispatchEvent(new Event('click', { bubbles: true }));
	}

	it('opts out and shows the rows removed', async () => {
		const host = renderToggle(true);
		await tick();
		expect(host.textContent).toContain('Pooling is on');
		expect(host.querySelector('[role="status"]')).toBeNull();
		clickButton(host, 'Opt out of pooling');
		await tick();
		await tick();
		await tick();
		expect(host.querySelector('[role="status"]')?.textContent).toBe(
			'Pooling is off: 3 existing fit rows removed.'
		);
		expect(host.textContent).toContain('Pooling is off');
		expect(host.innerHTML).not.toContain('// The shopper');
	});

	it('opts back in and removes nothing', async () => {
		const host = renderToggle(false);
		await tick();
		clickButton(host, 'Opt back in');
		await tick();
		await tick();
		await tick();
		expect(host.querySelector('[role="status"]')?.textContent).toContain('Pooling is back on');
	});

	it('says a failed choice failed', async () => {
		const { PoolingClient, PoolingState } = await import('./pooling');
		const failing = new PoolingState(
			new PoolingClient({
				call: () => Promise.reject(new Error('layer is down'))
			}),
			'shopper_01',
			true
		);
		const host = render(PoolingToggle, { pooling: failing });
		await tick();
		clickButton(host, 'Opt out of pooling');
		await tick();
		await tick();
		await tick();
		expect(host.querySelector('[role="alert"]')?.textContent).toBe('layer is down');
		expect(host.querySelector('[role="status"]')).toBeNull();
	});
});
