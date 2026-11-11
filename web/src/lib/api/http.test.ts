// The live transport's own gate: one POST per tool to the edge's plain HTTP
// surface, the shopper token as the bearer credential, and every failure a
// typed LedgerApiError instead of data. A refusal carries the layer's code
// and message; an unreachable edge and an unreadable body carry their own
// codes, so a view can label a failed live read as a failure.

import { describe, expect, it } from 'vitest';
import { LedgerApiError } from './errors';
import { createHttpTransport } from './http';

/** A response-shaped answer, so the tests need no fetch implementation. */
interface FakeResponse {
	ok: boolean;
	status: number;
	text(): Promise<string>;
}

function answer(status: number, body: string): FakeResponse {
	return {
		ok: status >= 200 && status < 300,
		status,
		text: () => Promise.resolve(body)
	};
}

interface Call {
	url: string;
	init: RequestInit | undefined;
}

function transportAnswering(
	reply: (url: string) => FakeResponse | Promise<FakeResponse>,
	token = 'memento_shop_test'
): { transport: ReturnType<typeof createHttpTransport>; calls: Call[] } {
	const calls: Call[] = [];
	const send = ((input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input);
		calls.push({ url, init });
		return Promise.resolve(reply(url)).then((value) => value as unknown as Response);
	}) as typeof globalThis.fetch;
	return { transport: createHttpTransport({ token, fetch: send }), calls };
}

describe('the live transport calls the edge', () => {
	it('posts one tool to /tools/<tool> with the token as a bearer credential', async () => {
		const { transport, calls } = transportAnswering(() => answer(200, '{"shopper_id":"shopper_01"}'));
		const body = await transport.call('ledger_beliefs', {
			shopper_id: 'shopper_01',
			include_seeded: true
		});

		expect(body).toEqual({ shopper_id: 'shopper_01' });
		expect(calls).toHaveLength(1);
		const [call] = calls;
		expect(call.url).toBe('/tools/ledger_beliefs');
		expect(call.init?.method).toBe('POST');
		const headers = call.init?.headers as Record<string, string>;
		expect(headers['Authorization']).toBe('Bearer memento_shop_test');
		expect(headers['Content-Type']).toBe('application/json');
		expect(JSON.parse(String(call.init?.body))).toEqual({
			shopper_id: 'shopper_01',
			include_seeded: true
		});
	});

	it('reads the same-origin edge unless a base URL is given', async () => {
		const calls: Call[] = [];
		const send = ((input: RequestInfo | URL, init?: RequestInit) => {
			calls.push({ url: String(input), init });
			return Promise.resolve(answer(200, '{}') as unknown as Response);
		}) as typeof globalThis.fetch;
		const transport = createHttpTransport({
			token: 'token',
			baseUrl: 'https://layer.test',
			fetch: send
		});
		await transport.call('ledger_belief', { shopper_id: 'shopper_01', belief_id: 'blf_1' });
		expect(calls[0].url).toBe('https://layer.test/tools/ledger_belief');
	});

	it('carries the layer code and message from a refusal', async () => {
		const { transport } = transportAnswering(() =>
			answer(401, '{"error":{"code":"unauthenticated","message":"unauthorized"}}')
		);
		const failure = await transport
			.call('ledger_beliefs', { shopper_id: 'shopper_09', include_seeded: true })
			.catch((error: unknown) => error);
		expect(failure).toBeInstanceOf(LedgerApiError);
		expect(failure).toMatchObject({
			code: 'refused',
			status: 401,
			message: 'unauthenticated: unauthorized'
		});
	});

	it('names the status when a refusal carries no error envelope', async () => {
		const { transport } = transportAnswering(() => answer(502, 'bad gateway'));
		const failure = await transport
			.call('ledger_evidence', { shopper_id: 'shopper_01' })
			.catch((error: unknown) => error);
		expect(failure).toMatchObject({ code: 'refused', status: 502, message: 'the edge answered 502' });
	});

	it('fails an unreachable edge without inventing an answer', async () => {
		const send = (() => Promise.reject(new TypeError('fetch failed'))) as typeof globalThis.fetch;
		const transport = createHttpTransport({ token: 'token', fetch: send });
		const failure = await transport
			.call('ledger_beliefs', { shopper_id: 'shopper_01', include_seeded: true })
			.catch((error: unknown) => error);
		expect(failure).toBeInstanceOf(LedgerApiError);
		expect(failure).toMatchObject({ code: 'unreachable' });
		expect((failure as LedgerApiError).status).toBeNull();
	});

	it('fails a bodiless or unreadable answer as a bad response', async () => {
		const empty = transportAnswering(() => answer(200, ''));
		await expect(
			empty.transport.call('ledger_beliefs', { shopper_id: 'shopper_01', include_seeded: true })
		).rejects.toMatchObject({ code: 'bad-response' });
		const notJson = transportAnswering(() => answer(200, 'not json'));
		await expect(
			notJson.transport.call('ledger_beliefs', { shopper_id: 'shopper_01', include_seeded: true })
		).rejects.toMatchObject({ code: 'bad-response' });
	});

	it('refuses to build without a token, and never prints one in a failure', async () => {
		expect(() => createHttpTransport({ token: '   ' })).toThrow(LedgerApiError);
		const token = 'memento_shop_secret_value';
		const { transport } = transportAnswering(() => answer(401, ''), token);
		const failure = await transport
			.call('ledger_beliefs', { shopper_id: 'shopper_01', include_seeded: true })
			.catch((error: unknown) => error);
		expect((failure as Error).message).not.toContain(token);
	});
});
