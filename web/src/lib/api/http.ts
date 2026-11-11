// The live transport: one tool call over the edge's plain HTTP surface.
// The SPA is static, so a read goes same-origin to /tools/<tool> with the
// shopper's link token as the bearer credential. The edge authenticates the
// token, bounds the call to the shopper it names, and proxies the layer's
// own answer back. A refusal, an unreachable edge or an unreadable body
// fails as a LedgerApiError, so no view can render a failed live read as
// data.

import { LedgerApiError } from './errors';

/** How the live transport reaches the edge and who it acts for. */
export interface HttpTransportOptions {
	/** The shopper's link token, sent as the bearer credential. Never shown. */
	token: string;
	/** The origin to call. Empty means the page's own origin. */
	baseUrl?: string;
	/** The fetch to send with. Defaults to the page's global fetch. */
	fetch?: typeof fetch;
}

/** A transport that posts one tool request to the edge. */
export interface HttpTransport {
	call(tool: string, request: Record<string, unknown>): Promise<unknown>;
}

/** The message from a failed call, without assuming the failure is an Error. */
function failureText(error: unknown): string {
	return error instanceof Error && error.message !== '' ? error.message : 'no answer';
}

/**
 * The layer's own code and message from an error body, as one line, or null
 * when the body is not the shared error envelope. The envelope is
 * `{"error":{"code":"…","message":"…"}}`.
 */
function envelopeMessage(body: string): string | null {
	let parsed: unknown;
	try {
		parsed = JSON.parse(body) as unknown;
	} catch {
		return null;
	}
	if (typeof parsed !== 'object' || parsed === null || !('error' in parsed)) return null;
	const error = (parsed as { error: unknown }).error;
	if (typeof error !== 'object' || error === null) return null;
	const code = (error as { code?: unknown }).code;
	const message = (error as { message?: unknown }).message;
	if (typeof message !== 'string' || message === '') return null;
	return typeof code === 'string' && code !== '' ? `${code}: ${message}` : message;
}

/**
 * Builds the transport that calls the live layer through the edge. Every
 * call is one POST to /tools/<tool> with the token as its bearer credential;
 * a non-2xx answer fails with the layer's own code and message, and a
 * refused or unreachable edge never returns data.
 */
export function createHttpTransport(options: HttpTransportOptions): HttpTransport {
	const token = options.token.trim();
	if (token === '') {
		throw new LedgerApiError('unreachable', 'the link carries no shopper token');
	}
	const baseUrl = options.baseUrl ?? '';
	const send: typeof fetch =
		options.fetch ?? ((input, init) => globalThis.fetch(input, init));
	return {
		async call(tool: string, request: Record<string, unknown>): Promise<unknown> {
			let response: Response;
			try {
				response = await send(`${baseUrl}/tools/${tool}`, {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						Authorization: `Bearer ${token}`
					},
					body: JSON.stringify(request)
				});
			} catch (error) {
				throw new LedgerApiError(
					'unreachable',
					`the edge could not be reached: ${failureText(error)}`
				);
			}
			let body: string;
			try {
				body = await response.text();
			} catch (error) {
				throw new LedgerApiError(
					'unreachable',
					`the edge's answer could not be read: ${failureText(error)}`,
					response.status
				);
			}
			if (!response.ok) {
				throw new LedgerApiError(
					'refused',
					envelopeMessage(body) ?? `the edge answered ${String(response.status)}`,
					response.status
				);
			}
			if (body.trim() === '') {
				throw new LedgerApiError('bad-response', 'the layer answered with no body', response.status);
			}
			try {
				return JSON.parse(body) as unknown;
			} catch {
				throw new LedgerApiError(
					'bad-response',
					'the layer answered with a body that is not JSON',
					response.status
				);
			}
		}
	};
}
