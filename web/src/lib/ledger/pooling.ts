// The shopper's pooling choice in TypeScript, mirroring the pivoted
// set_pooling schemas in testdata/api. Pooling is opt-out: the shopper's
// classified outcomes count toward the pool unless they say no. Opting out
// stops new fit rows and removes the ones already written; the response
// says how many rows went. Field names stay snake_case.
import { Notifier } from '@nrynss/chaaya/core';
import { LedgerApiError } from '$lib/api';
import optInExample from '../../../../testdata/api/set_pooling.response.example-optin.json';
import optOutExample from '../../../../testdata/api/set_pooling.response.example.json';

/** The shopper's pooling choice. True keeps pooling; false opts out. */
export interface SetPoolingRequest {
	shopper_id: string;
	pooling: boolean;
	trace_id?: string;
}

/** The resulting choice, and how many fit rows opting out removed. */
export interface SetPoolingResponse {
	shopper_id: string;
	pooling: boolean;
	rows_removed: number;
	trace_id: string | null;
}

/** The only tool the pooling path may call. */
export type PoolingToolName = 'set_pooling';

/** Sends the shopper's pooling choice to the layer. One tool, nothing else. */
export interface PoolingTransport {
	call(tool: PoolingToolName, request: Record<string, unknown>): Promise<unknown>;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function fail(what: string): never {
	throw new LedgerApiError('bad-response', `pooling response has a bad ${what}`);
}

/** Decodes a set_pooling response. Opting back in removes nothing. */
export function decodeSetPoolingResponse(value: unknown): SetPoolingResponse {
	if (!isRecord(value)) fail('response');
	if (typeof value['shopper_id'] !== 'string') fail('response shopper');
	if (typeof value['pooling'] !== 'boolean') fail('response choice');
	const removed = value['rows_removed'];
	if (typeof removed !== 'number' || !Number.isInteger(removed) || removed < 0) {
		fail('response rows removed');
	}
	const traceId = value['trace_id'];
	if (traceId !== null && typeof traceId !== 'string') fail('response trace id');
	return {
		shopper_id: value['shopper_id'] as string,
		pooling: value['pooling'] as boolean,
		rows_removed: removed,
		trace_id: traceId as string | null
	};
}

/** A typed client for the shopper's pooling choice. */
export class PoolingClient {
	private readonly transport: PoolingTransport;

	constructor(transport: PoolingTransport) {
		this.transport = transport;
	}

	/** Records the shopper's pooling choice through set_pooling only. */
	async setPooling(request: SetPoolingRequest): Promise<SetPoolingResponse> {
		const raw = await this.transport.call('set_pooling', { ...request });
		return decodeSetPoolingResponse(raw);
	}
}

/** The frozen answer for one pooling choice, decoded. */
function fixtureAnswer(pooling: boolean): SetPoolingResponse {
	return decodeSetPoolingResponse(pooling ? optInExample : optOutExample);
}

/**
 * A transport that replays the frozen pooling answers for the shopper they
 * belong to: opting out removes three fit rows, opting back in removes
 * nothing. The examples are the demo shopper's, so a transport built for
 * any other shopper has no answer and fails instead of handing back
 * another shopper's choice.
 */
export function createFixturePoolingTransport(shopperId: string): PoolingTransport {
	return {
		async call(tool, request: Record<string, unknown>): Promise<unknown> {
			if (tool !== 'set_pooling') {
				throw new LedgerApiError('unknown-tool', `fixture pooling knows no ${tool}`);
			}
			if (request['shopper_id'] !== shopperId) {
				throw new LedgerApiError('unknown-request', 'fixture pooling knows only one shopper');
			}
			const pooling = request['pooling'];
			if (pooling !== true && pooling !== false) {
				throw new LedgerApiError('unknown-request', 'pooling needs the shopper in true or false');
			}
			const answer = fixtureAnswer(pooling);
			if (answer.shopper_id !== shopperId) {
				throw new LedgerApiError(
					'unknown-request',
					'the pooling fixture has no answer for this shopper'
				);
			}
			return { ...answer } satisfies SetPoolingResponse;
		}
	};
}

/** Where the pooling choice stands. It starts idle and never returns there. */
export type PoolingStatus = 'idle' | 'sending' | 'ready' | 'error';

/** The plain-words line for a pooling choice, for the toggle. */
export function choiceLineFor(choice: boolean): string {
	return choice
		? 'Pooling is on: your classified outcomes count toward the pool.'
		: 'Pooling is off: nothing new is written, and your rows are gone from the pool.';
}

/** The plain-words result of a pooling change, or null before one lands. */
export function resultLineFor(
	choice: boolean,
	rowsRemoved: number | null,
	ready: boolean
): string | null {
	if (!ready || rowsRemoved === null) return null;
	if (choice) {
		return 'Pooling is back on: your classified outcomes count toward the pool again.';
	}
	return `Pooling is off: ${String(rowsRemoved)} existing fit rows removed.`;
}

/** One shopper's pooling choice, sent through set_pooling only. */
export class PoolingState extends Notifier {
	private readonly client: PoolingClient;
	private readonly shopperId: string;
	private choice: boolean;
	private removed: number | null = null;
	private opStatus: PoolingStatus = 'idle';
	private opError: string | null = null;

	constructor(client: PoolingClient, shopperId: string, initialPooling = true) {
		super();
		this.client = client;
		this.shopperId = shopperId;
		this.choice = initialPooling;
	}

	/** The current pooling choice. True pools; false opted out. */
	get pooling(): boolean {
		return this.choice;
	}

	/** How many fit rows the latest change removed, or null before one lands. */
	get rowsRemoved(): number | null {
		return this.removed;
	}

	/** Where the latest choice stands. */
	get status(): PoolingStatus {
		return this.opStatus;
	}

	/** Why the latest choice failed, or null. */
	get error(): string | null {
		return this.opError;
	}

	/** The plain-words line for the current choice, for the toggle. */
	get choiceLine(): string {
		return choiceLineFor(this.choice);
	}

	/** The plain-words result of the latest change, or null before one lands. */
	get resultLine(): string | null {
		return resultLineFor(this.choice, this.removed, this.opStatus === 'ready');
	}

	/** Records the shopper's choice and reports how many rows it removed. */
	async setPooling(pooling: boolean): Promise<SetPoolingResponse> {
		this.opStatus = 'sending';
		this.opError = null;
		this.notify();
		try {
			const response = await this.client.setPooling({
				shopper_id: this.shopperId,
				pooling
			});
			this.choice = response.pooling;
			this.removed = response.rows_removed;
			this.opStatus = 'ready';
			this.notify();
			return response;
		} catch (error) {
			this.opStatus = 'error';
			this.opError = error instanceof Error ? error.message : 'The pooling choice failed.';
			this.notify();
			throw error;
		}
	}
}

/** Pooling choice over the fixture path, for the ledger page. */
export function createFixturePoolingState(shopperId: string, initialPooling = true): PoolingState {
	return new PoolingState(
		new PoolingClient(createFixturePoolingTransport(shopperId)),
		shopperId,
		initialPooling
	);
}
