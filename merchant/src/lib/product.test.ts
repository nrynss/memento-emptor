import { describe, expect, it } from 'vitest';
import { merchantName, tagline } from './product';

describe('merchant copy', () => {
	it('names the merchant app', () => {
		expect(merchantName).toBe('Memento Merchant');
	});

	it('states the read-only promise in one line', () => {
		expect(tagline).toContain('Read-only');
		expect(tagline).not.toContain('\n');
	});
});
