// Guards for the why view's UI bar: plain ledger language on screen, a
// one-line explanation where each term first appears, an onward path from
// every screen, and phone-width stacking. The content itself is pinned by
// why.test.ts; svelte-check and the build prove the template compiles.
// Runtime mounting is out of reach here (vitest resolves bare `svelte` to
// its server build while components compile for the client, and the vite
// config that would reunite them sits outside this task's paths), so these
// read the sources the gate already type-checks. The node functions below
// are declared locally: the web tsconfig carries no node types, and adding
// any sits outside this task's paths.

import { describe, expect, it } from 'vitest';

// Raw sources so the guards read the same files the gate type-checks.
import componentSource from './WhyView.svelte?raw';
import routeSource from '../../routes/belief/+page.svelte?raw';

describe('why view simplicity guards', () => {
	it('speaks ledger statuses, never graph standing', () => {
		expect(componentSource).not.toMatch(/Venerable|Candidate|Canonical/);
		expect(componentSource).not.toMatch(/standing:/);
		expect(componentSource).toContain('earned before the correction');
	});

	it('explains each marked term where it first appears', () => {
		expect(componentSource).toContain('OUT_OF_TRUE_TOOLTIP');
		expect(componentSource).toContain('SEEDED_MARK_TOOLTIP');
	});

	it('searches, links traces as placeholders, and says what the links are', () => {
		expect(componentSource).toContain('id="why-search-box"');
		expect(componentSource).toContain('trace.href');
		expect(componentSource).toContain('trace links are placeholders on this page');
		expect(componentSource).toContain('no per-belief trace list');
		expect(componentSource).not.toMatch(/live mode/i);
		// The note describes the links, not the data: a live read is not a
		// fixture replay, and a promise of a coming trace viewer would be
		// claiming a transport that does not exist.
		expect(componentSource).not.toMatch(/fixture replay/i);
		expect(componentSource).not.toMatch(/still to come/i);
	});

	it('leads each card with the shopper words, never a raw reason code', () => {
		expect(componentSource).toContain('reasonColumn(row)');
		expect(componentSource).toContain('reasonColumn(row.evidence)');
		expect(componentSource).toContain('reasonReading(evidence)');
		// The category word is chosen in the model, never printed raw here.
		expect(componentSource).not.toContain('reason.category');
		expect(componentSource).not.toContain('fit_too_small');
		// The shopper's words are quoted, and the plain word follows them.
		expect(componentSource.indexOf('<q>{reading.text}</q>')).toBeLessThan(
			componentSource.indexOf('<p class="why-reason-word">{reading.word}</p>')
		);
		// A dispute code is not the shopper's words: the plain word leads and
		// the recorded text follows it unquoted.
		expect(componentSource.indexOf('<p class="why-words">{reading.word}</p>')).toBeLessThan(
			componentSource.indexOf('<p class="why-reason-word">{reading.text}</p>')
		);
		expect(componentSource).not.toContain('fits well');
	});

	it('stacks the evidence columns at phone width', () => {
		expect(componentSource).toContain('@media (max-width: 560px)');
		expect(componentSource).toContain('why-columns');
	});

	it('gives every screen an onward path and every state a picker', () => {
		expect(routeSource).toContain("resolve('/ledger')");
		expect(routeSource).toContain('id="why-step"');
		expect(routeSource).toContain('id="why-belief"');
	});
});
