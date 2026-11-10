import { readFileSync } from 'node:fs';

const pkg = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8'));
const lock = JSON.parse(readFileSync(new URL('../package-lock.json', import.meta.url), 'utf8'));
const root = lock.packages?.[''] ?? {};

function sorted(record) {
	return Object.entries(record ?? {}).sort(([a], [b]) => a.localeCompare(b));
}

function same(a, b) {
	return JSON.stringify(sorted(a)) === JSON.stringify(sorted(b));
}

const problems = [];
if (!same(pkg.dependencies, root.dependencies)) problems.push('dependencies');
if (!same(pkg.devDependencies, root.devDependencies)) problems.push('devDependencies');

if (problems.length > 0) {
	console.error(`package-lock.json is out of sync with package.json: ${problems.join(', ')}`);
	process.exit(1);
}
