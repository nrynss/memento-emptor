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

	it('searches, links traces as placeholders, and says what the replay is', () => {
		expect(componentSource).toContain('id="why-search-box"');
		expect(componentSource).toContain('trace.href');
		expect(componentSource).toContain('This is a fixture replay');
		expect(componentSource).not.toMatch(/live mode/i);
	});

	it('leads each card with the shopper words, never a raw reason code', () => {
		expect(componentSource).toContain('row.reason.text');
		expect(componentSource).toContain('reasonWord(row.reason.category)');
		expect(componentSource).toContain('row.evidence.reason.text');
		expect(componentSource).toContain('reasonWord(row.evidence.reason.category)');
		expect(componentSource).not.toContain('{row.reason.category}');
		expect(componentSource).not.toContain('{row.evidence.reason.category}');
		expect(componentSource).not.toContain('fit_too_small');
		expect(componentSource.indexOf('<q>{row.reason.text}</q>')).toBeLessThan(
			componentSource.indexOf('{reasonWord(row.reason.category)}')
		);
		expect(componentSource.indexOf('<q>{row.evidence.reason.text}</q>')).toBeLessThan(
			componentSource.indexOf('{reasonWord(row.evidence.reason.category)}')
		);
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
