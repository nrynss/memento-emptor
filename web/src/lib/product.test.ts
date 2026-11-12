import { describe, expect, it } from 'vitest';
import { productName, tagline } from './product';

describe('product copy', () => {
	it('names the product', () => {
		expect(productName).toBe('Memento Emptor');
	});

	it('states the promise in one line', () => {
		// product.md's opening line, verbatim.
		expect(tagline).toBe(
			"PayPal tells your agent what happened to the money. Memento Emptor tells it why, and what every other shopper's returns already taught."
		);
		expect(tagline).not.toContain('\n');
	});
});
