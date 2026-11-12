import { describe, expect, it } from 'vitest';
import { productName, tagline } from './product';

describe('product copy', () => {
	it('names the product', () => {
		expect(productName).toBe('Memento Emptor');
	});

	it('states the promise in one line', () => {
		expect(tagline).toContain('what happened to the money');
		expect(tagline).toContain('every other shopper');
		expect(tagline).not.toContain('\n');
	});
});
