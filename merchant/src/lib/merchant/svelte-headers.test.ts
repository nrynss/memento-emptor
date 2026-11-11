// Svelte compiles markup-level text, so a file-header `//` comment renders in
// the page instead of staying a comment. Every component and route opens with
// markup, and its source note lives inside <script>. This is the sweep guard
// for the merchant grid and page headers.

import { describe, expect, it } from 'vitest';

const components = import.meta.glob('../../**/*.svelte', {
	query: '?raw',
	import: 'default',
	eager: true
}) as Record<string, string>;

describe('svelte sources open with markup', () => {
	it('finds the components and routes it guards', () => {
		const paths = Object.keys(components);
		expect(paths.some((path) => path.endsWith('/routes/+page.svelte'))).toBe(true);
		expect(paths.some((path) => path.endsWith('MerchantGrid.svelte'))).toBe(true);
	});

	it('never begins a file with a // comment that would render as page text', () => {
		for (const [path, source] of Object.entries(components)) {
			expect({ path, opensWithAComment: source.trimStart().startsWith('//') }).toEqual({
				path,
				opensWithAComment: false
			});
		}
	});
});
