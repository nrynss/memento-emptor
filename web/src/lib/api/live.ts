// Where a ledger page reads from. A link carrying the shopper's token
// (?t=<token>) reads the live layer through the edge; a page with no token
// replays the demo story from fixtures and says so on screen, so a replay is
// never mistaken for live data. The layer bounds every scoped read to the
// shopper the token binds, and each request has to name that shopper, so the
// link names it too (?s=<shopper>); an absent name reads for the demo
// shopper the published demo token binds.

import { LedgerClient } from './client';
import { fixtureShopperId } from './fixture';
import { createHttpTransport, type HttpTransport, type HttpTransportOptions } from './http';
import { LedgerState } from './state';

/** What a shopper link carries: the bearer credential and the shopper it acts for. */
export interface LiveLink {
	/** The shopper's link token. It is never printed, logged or stored. */
	token: string;
	/** The shopper the token binds, named in every request body. */
	shopperId: string;
}

/** The shopper a link without ?s= reads for: the one the demo token binds. */
export const defaultLiveShopper = fixtureShopperId;

/**
 * The shopper link a page URL carries, or null when it carries no token.
 * The token is the `t` parameter and rides in the Authorization header; the
 * shopper is the `s` parameter, defaulting to the demo shopper.
 */
export function liveLinkFrom(url: URL | string | null | undefined): LiveLink | null {
	if (url === null || url === undefined) return null;
	const params = (typeof url === 'string' ? new URL(url, 'http://localhost') : url).searchParams;
	const token = (params.get('t') ?? '').trim();
	if (token === '') return null;
	const shopperId = (params.get('s') ?? '').trim();
	return { token, shopperId: shopperId === '' ? defaultLiveShopper : shopperId };
}

/** The transport a live link reads through: the edge, as its shopper. */
export function createLiveTransport(
	link: LiveLink,
	options: Omit<HttpTransportOptions, 'token'> = {}
): HttpTransport {
	return createHttpTransport({ ...options, token: link.token });
}

/** The observable ledger state a live link loads. */
export function createLiveState(
	link: LiveLink,
	options: Omit<HttpTransportOptions, 'token'> = {}
): LedgerState {
	return new LedgerState(new LedgerClient(createLiveTransport(link, options)), link.shopperId);
}
