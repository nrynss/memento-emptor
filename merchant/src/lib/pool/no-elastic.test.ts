// The gate for the read-only promise: no shipped source file configures a
// search tier, the package depends on none, and the only environment names
// the app reads are the layer URL and the merchant key. Test files are not
// scanned: a test may name the forbidden thing while proving it absent.

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { layerUrlEnvVar, merchantEnvVars, merchantKeyEnvVar } from './config';

const merchantRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

function sources(dir: string): string[] {
	const found: string[] = [];
	for (const entry of readdirSync(dir)) {
		const child = join(dir, entry);
		const info = statSync(child);
		if (info.isDirectory()) {
			if (entry === 'node_modules' || entry === 'build' || entry === '.svelte-kit') continue;
			found.push(...sources(child));
		} else if (/\.(ts|svelte|js|mjs)$/.test(entry) && !entry.endsWith('.test.ts')) {
			found.push(child);
		}
	}
	return found;
}

describe('no search-tier configuration', () => {
	it('ships no source file naming that tier', () => {
		const files = sources(join(merchantRoot, 'src'));
		expect(files.length).toBeGreaterThan(0);
		const offenders = files.filter((file) => /elastic/i.test(readFileSync(file, 'utf8')));
		expect(offenders).toEqual([]);
	});

	it('depends on no such package', () => {
		const pkg = JSON.parse(readFileSync(join(merchantRoot, 'package.json'), 'utf8')) as {
			dependencies: Record<string, string>;
			devDependencies: Record<string, string>;
		};
		const names = [...Object.keys(pkg.dependencies), ...Object.keys(pkg.devDependencies)];
		expect(names.filter((name) => /elastic/i.test(name))).toEqual([]);
	});

	it('reads only the layer URL and the merchant key from the environment', () => {
		expect(merchantKeyEnvVar).toBe('MEMENTO_MERCHANT_KEY');
		expect(layerUrlEnvVar).toBe('MEMENTO_LAYER_URL');
		expect([...merchantEnvVars]).toEqual(['MEMENTO_MERCHANT_KEY', 'MEMENTO_LAYER_URL']);
		for (const name of merchantEnvVars) {
			expect(name).not.toMatch(/elastic/i);
		}
	});
});
