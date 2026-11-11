// The judges' page under a real mount. The page is a static guide: the table
// renders every place to start, every link is an anchor with an href, every
// claim carries a link to its evidence, the seeded links carry the ledger's
// own mark, and the round-2 placeholder reports no result. Nothing here may
// make a live read, so a fetch spy stays untouched.

import { afterEach, describe, expect, it, vi } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../../node_modules/svelte/src/index-client.js';
import { SEEDED_MARK_LABEL, SEEDED_MARK_TOOLTIP } from '$lib/ledger/rows';
import JudgesPage from './+page.svelte';
import pageSource from './+page.svelte?raw';

const fetchSpy = vi.fn(() => Promise.resolve(new Response('{}')));

const mounted: { comp: object; host: HTMLElement }[] = [];

afterEach(() => {
	for (const { comp, host } of mounted.splice(0)) {
		unmount(comp as Parameters<typeof unmount>[0]);
		host.remove();
	}
	vi.unstubAllGlobals();
	fetchSpy.mockClear();
});

async function render(): Promise<HTMLElement> {
	vi.stubGlobal('fetch', fetchSpy);
	const host = document.createElement('div');
	document.body.appendChild(host);
	mounted.push({ comp: mount(JudgesPage, { target: host }), host });
	await tick();
	return host;
}

describe('the judges page', () => {
	it('renders the start-here table with every required place to start', async () => {
		const host = await render();
		const table = host.querySelector('table[data-judges-table]');
		expect(table).not.toBeNull();

		const rows = [...table!.querySelectorAll('tbody tr')];
		expect(rows).toHaveLength(8);

		const labels = rows.map((row) => row.querySelector('th')?.textContent?.trim() ?? '');
		for (const required of [
			'Hosted demo',
			'Evidence ledger',
			'Why view',
			'Merchant app',
			'Traces',
			'Comparison, round 1',
			'Comparison, round 2',
			'Repository'
		]) {
			expect(labels).toContain(required);
		}
	});

	it('makes every link an anchor with an href and words to read', async () => {
		const host = await render();
		const links = [...host.querySelectorAll('a')];
		expect(links.length).toBeGreaterThan(10);
		for (const link of links) {
			expect(link.getAttribute('href')).toBeTruthy();
			expect(link.textContent?.trim()).not.toBe('');
		}

		const rows = [...host.querySelectorAll('table[data-judges-table] tbody tr')];
		for (const row of rows) {
			expect(row.querySelectorAll('a[href]').length).toBeGreaterThanOrEqual(2);
			const evidence = row.querySelector('td:last-child a[href]');
			expect(evidence).not.toBeNull();
		}
	});

	it('links every claim to its evidence', async () => {
		const host = await render();
		const claims = [...host.querySelectorAll('[data-claim]')];
		expect(claims.length).toBeGreaterThanOrEqual(10);
		for (const claim of claims) {
			expect(claim.querySelector('a[href]')).not.toBeNull();
		}
	});

	it('marks every seeded link with the ledger mark and states the notice', async () => {
		const host = await render();
		const seededLinks = [...host.querySelectorAll('a[data-seeded-link]')];
		expect(seededLinks.length).toBeGreaterThanOrEqual(2);
		for (const link of seededLinks) {
			const mark = link.parentElement?.querySelector('[data-seeded-mark]');
			expect(mark?.textContent?.trim()).toBe(SEEDED_MARK_LABEL);
			expect(mark?.getAttribute('title')).toBe(SEEDED_MARK_TOOLTIP);
		}

		const notice = host.querySelector('[data-seeded-notice]');
		expect(notice?.textContent).toContain('generated, not lived');
		expect(notice?.textContent).toContain(SEEDED_MARK_LABEL);
		expect(host.querySelectorAll('[data-simulated-mark]').length).toBeGreaterThanOrEqual(1);
	});

	it('reports the round-2 counts without choosing a headline', async () => {
		const host = await render();
		const round2 = host.querySelector('[data-round2]');
		expect(round2).not.toBeNull();

		const text = round2!.textContent ?? '';
		expect(text).toContain('T10.5');
		expect(text).toContain('10/16');
		expect(text).toContain('12/16');
		expect(text).toContain('16/16');
		expect(text).toContain('$0.789309');
		expect(text).toContain('Headline candidates');
		expect(text).toContain('the user picks');
		expect(text).toContain('none is chosen here');

		// Counts, never percentages; and the page states no verdict and picks
		// no headline for the user.
		expect(text).not.toMatch(/%/);
		expect(text).not.toMatch(
			/\b(?:better|worse|wins?|winning|ahead|leads?|proves?|matches?|percent|rate|best|winner|selected)\b/i
		);
		expect(text).not.toMatch(/\b(?:will|soon|coming|expected|future)\b/i);

		const candidates = [...round2!.querySelectorAll('[data-round2-candidate]')];
		expect(candidates).toHaveLength(4);
		for (const candidate of candidates) {
			expect(candidate.querySelector('a[href]')).not.toBeNull();
		}

		const first = round2!.querySelector('[data-round2-first]');
		expect(first).not.toBeNull();
		expect(first!.textContent).toContain('0/16');
		expect(first!.textContent).toContain('10/16');
		expect(first!.textContent).toContain('12/16');

		const hand = round2!.querySelector('[data-round2-hand]');
		expect(hand).not.toBeNull();
		expect(hand!.textContent).toContain('12/16');
		expect(hand!.textContent).toContain('16/16');

		const corrections = round2!.querySelector('[data-round2-corrections]');
		expect(corrections).not.toBeNull();
		expect(corrections!.textContent).toContain('4/12');
		expect(corrections!.textContent).toContain('12/12');

		const row = host.querySelector('[data-round2-row]');
		expect(row).not.toBeNull();
		const rowText = row!.textContent ?? '';
		expect(rowText).toContain('10/16');
		expect(rowText).toContain('12/16');
		expect(rowText).toContain('$0.789309');
		expect(rowText).not.toMatch(/%/);
		expect(rowText).not.toMatch(
			/\b(?:better|worse|wins?|winning|ahead|leads?|proves?|matches?|percent|rate|best|winner|selected)\b/i
		);
	});

	it('makes no live read when it mounts', async () => {
		await render();
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(pageSource).not.toMatch(/createLiveTransport|LedgerClient|createFixtureTransport/);
		expect(pageSource).not.toMatch(/\bfetch\(/);
	});

	it('carries no credential value, key value or private path in its source', async () => {
		expect(pageSource).not.toMatch(/secret-tool|client-secret|client-id|password|api[_-]?key/i);
		expect(pageSource).not.toMatch(/Bearer\s/);
		expect(pageSource).not.toMatch(/\S+@\S+/);
		expect(pageSource).not.toMatch(/\b\d{1,3}(?:\.\d{1,3}){3}\b/);
		expect(pageSource).not.toMatch(/\/home\//);
	});
});
