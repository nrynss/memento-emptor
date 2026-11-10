// The gate for the pooling choice: both frozen set_pooling answers decode,
// opting out removes three fit rows and says so, opting back in removes
// nothing, and one shopper's choice never touches another shopper's pool.
import { describe, expect, it } from 'vitest';
import optInExample from '../../../../testdata/api/set_pooling.response.example-optin.json';
import optOutExample from '../../../../testdata/api/set_pooling.response.example.json';
import { LedgerApiError } from '$lib/api';
import {
	createFixturePoolingState,
	decodeSetPoolingResponse,
	choiceLineFor,
	resultLineFor,
	PoolingClient,
	type PoolingTransport
} from './pooling';

describe('pooling answers from the frozen examples', () => {
	it('decodes opting out with three rows removed', () => {
		const decoded = decodeSetPoolingResponse(optOutExample);
		expect(decoded.pooling).toBe(false);
		expect(decoded.rows_removed).toBe(3);
	});

	it('decodes opting back in with nothing removed', () => {
		const decoded = decodeSetPoolingResponse(optInExample);
		expect(decoded.pooling).toBe(true);
		expect(decoded.rows_removed).toBe(0);
	});

	it('rejects a misshapen answer instead of applying it', () => {
		expect(() => decodeSetPoolingResponse({ pooling: 'no' })).toThrow(LedgerApiError);
		expect(() => decodeSetPoolingResponse({ pooling: true, rows_removed: -1 })).toThrow(
			LedgerApiError
		);
	});
});

describe('pooling lines in plain words', () => {
	it('names the choice and the rows removed', () => {
		expect(choiceLineFor(true)).toContain('Pooling is on');
		expect(choiceLineFor(false)).toContain('Pooling is off');
		expect(resultLineFor(true, 0, true)).toContain('Pooling is back on');
		expect(resultLineFor(false, 3, true)).toBe('Pooling is off: 3 existing fit rows removed.');
		expect(resultLineFor(false, null, false)).toBeNull();
		expect(resultLineFor(false, 3, false)).toBeNull();
	});
});

describe('pooling choice over the fixture path', () => {
	it('opts out and shows the rows removed', async () => {
		const state = createFixturePoolingState('shopper_01');
		expect(state.pooling).toBe(true);
		expect(state.rowsRemoved).toBeNull();
		expect(state.choiceLine).toContain('Pooling is on');
		const response = await state.setPooling(false);
		expect(response.rows_removed).toBe(3);
		expect(state.pooling).toBe(false);
		expect(state.rowsRemoved).toBe(3);
		expect(state.resultLine).toBe('Pooling is off: 3 existing fit rows removed.');
		expect(state.choiceLine).toContain('Pooling is off');
	});

	it('opts back in and removes nothing', async () => {
		const state = createFixturePoolingState('shopper_01', false);
		const response = await state.setPooling(true);
		expect(response.rows_removed).toBe(0);
		expect(state.pooling).toBe(true);
		expect(state.resultLine).toBe(
			'Pooling is back on: your classified outcomes count toward the pool again.'
		);
	});

	it('sends through set_pooling and nothing else', async () => {
		const { createFixturePoolingTransport } = await import('./pooling');
		const calls: { tool: string; request: unknown }[] = [];
		const inner = createFixturePoolingTransport('shopper_01');
		const recording: PoolingTransport = {
			call: (tool, request) => {
				calls.push({ tool, request });
				return inner.call(tool, request);
			}
		};
		const client = new PoolingClient(recording);
		await client.setPooling({ shopper_id: 'shopper_01', pooling: false });
		expect(calls).toHaveLength(1);
		expect(calls[0].tool).toBe('set_pooling');
		expect(calls[0].request).toMatchObject({ shopper_id: 'shopper_01', pooling: false });
	});

	it('never touches another shopper pool', async () => {
		const { createFixturePoolingTransport } = await import('./pooling');
		const other = new PoolingClient(createFixturePoolingTransport('shopper_02'));
		await expect(
			other.setPooling({ shopper_id: 'shopper_01', pooling: false })
		).rejects.toMatchObject({ code: 'unknown-request' });
	});

	it('reports a failed choice and keeps the old one', async () => {
		const failing: PoolingTransport = {
			call: () => Promise.reject(new Error('layer is down'))
		};
		const { PoolingState } = await import('./pooling');
		const state = new PoolingState(new PoolingClient(failing), 'shopper_01', true);
		await expect(state.setPooling(false)).rejects.toThrow('layer is down');
		expect(state.status).toBe('error');
		expect(state.error).toBe('layer is down');
		expect(state.pooling).toBe(true);
		expect(state.resultLine).toBeNull();
	});
});
