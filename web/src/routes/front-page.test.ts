// The front page under a real mount. The page is static copy: it leads with
// the headline the project chose, states the one count behind it with the
// limits beside them, and links every surface. Nothing here may make a live
// read, so a fetch spy stays untouched, and the copy carries no percentage,
// no verdict, no "earned" and no promise about the future.

import { afterEach, describe, expect, it, vi } from 'vitest';
// @ts-expect-error: deep client entry ships no types; same signature as 'svelte'.
import { mount, tick, unmount } from '../../node_modules/svelte/src/index-client.js';
import { resolve } from '$app/paths';
import { SEEDED_MARK_LABEL, SEEDED_MARK_TOOLTIP } from '$lib/ledger/rows';
import FrontPage from './+page.svelte';
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
	mounted.push({ comp: mount(FrontPage, { target: host }), host });
	await tick();
	return host;
}

describe('the front page', () => {
	it('leads with the chosen headline', async () => {
		const host = await render();
		const heading = host.querySelector('h1[data-headline]');
		expect(heading?.textContent?.trim()).toBe('Right on day one for a new shopper');
	});

	it('states the count and the limits beside it', async () => {
		const host = await render();
		const count = host.querySelector('[data-count]');
		expect(count).not.toBeNull();
		const countText = count!.textContent ?? '';
		expect(countText).toContain('3/4');
		expect(countText).toContain('4/4');
		expect(countText).toContain('0/4');

		const limits = host.querySelector('[data-limits]');
		expect(limits).not.toBeNull();
		const limitsText = limits!.textContent ?? '';
		expect(limitsText).toContain('constructed');
		expect(limitsText).toContain('seeded');
		expect(limitsText).toMatch(/not live\s+traffic/);
		expect(limitsText).toMatch(/not real-world\s+accuracy/);
	});

	it('says what the layer is', async () => {
		const host = await render();
		const what = host.querySelector('[data-what]');
		expect(what).not.toBeNull();
		expect(what!.textContent).toContain('memory layer');
		expect(what!.textContent).toContain('why a purchase came back');
		expect(what!.textContent).toContain("every other shopper's returns already taught");
	});

	it('links every surface with a real href and words to read', async () => {
		const host = await render();

		const internal: [string, string][] = [
			['[data-link-judges]', resolve('/judges')],
			['[data-link-ledger]', resolve('/ledger')],
			['[data-link-belief]', resolve('/belief')]
		];
		for (const [selector, href] of internal) {
			const link = host.querySelector(selector);
			expect(link, selector).not.toBeNull();
			expect(link!.getAttribute('href')).toBe(href);
			expect(link!.textContent?.trim()).not.toBe('');
		}

		const external: [string, string][] = [
			['[data-link-merchant]', 'https://memento-merchant.onrender.com'],
			['[data-link-repository]', 'https://github.com/nrynss/memento-emptor'],
			[
				'[data-link-comparison]',
				'https://github.com/nrynss/memento-emptor/blob/main/bench/round2/README.md'
			]
		];
		for (const [selector, href] of external) {
			const link = host.querySelector(selector);
			expect(link, selector).not.toBeNull();
			expect(link!.getAttribute('href')).toBe(href);
			expect(link!.getAttribute('rel')).toBe('external');
			expect(link!.textContent?.trim()).not.toBe('');
		}
	});

	it('explains the seeded mark beside the ledger link', async () => {
		const host = await render();
		const ledger = host.querySelector('[data-link-ledger]');
		expect(ledger).not.toBeNull();
		const mark = ledger!.parentElement?.querySelector('[data-seeded-mark]');
		expect(mark?.textContent?.trim()).toBe(SEEDED_MARK_LABEL);
		expect(mark?.getAttribute('title')).toBe(SEEDED_MARK_TOOLTIP);
		expect(host.textContent).toContain('generated, not lived');
	});

	it('states no percentage, verdict, earned status or future promise', async () => {
		const host = await render();
		const text = host.textContent ?? '';
		expect(text).not.toMatch(/%/);
		expect(pageSource).not.toContain('percent');
		expect(text).not.toMatch(
			/\b(?:better|worse|wins?|winning|ahead|leads?|proves?|matches?|percent|rate|best|winner|selected)\b/i
		);
		expect(text).not.toMatch(/earned/i);
		expect(text).not.toMatch(/\b(?:will|soon|coming|expected|future)\b/i);
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
