// Observable ledger state for Svelte 5 views. The class is
// framework-neutral: a view wraps it with Chaaya's reactive() and reads
// the getters in a template or $derived, and notify() re-runs them. One
// state instance serves one step; a step picker builds a new one.
//
// The state holds the rows the ledger table renders: the beliefs, and the
// refunds still missing a reason. The evidence behind a belief is not one of
// them: the why view reads it for the one belief it explains, and the table's
// per-row count comes from the belief row. A failed evidence read therefore
// never labels the table failed over data it does not show.

import { Notifier } from '@nrynss/chaaya/core';
import { LedgerClient } from './client';
import { createFixtureTransport, type LedgerFixtureStep } from './fixture';
import type { LedgerBeliefRow, QuestionView } from './types';

/** Where a load stands. It starts idle and never returns there. */
export type LedgerLoadStatus = 'idle' | 'loading' | 'ready' | 'error';

/** One shopper's ledger rows, loaded through a typed client. */
export class LedgerState extends Notifier {
	private readonly client: LedgerClient;
	private readonly shopperId: string;
	private loadStatus: LedgerLoadStatus = 'idle';
	private beliefRows: LedgerBeliefRow[] = [];
	private pendingRows: QuestionView[] = [];
	private loadError: string | null = null;

	constructor(client: LedgerClient, shopperId: string) {
		super();
		this.client = client;
		this.shopperId = shopperId;
	}

	/** Where the current load stands. */
	get status(): LedgerLoadStatus {
		return this.loadStatus;
	}

	/** The belief rows, strongest first. Empty until ready. */
	get beliefs(): LedgerBeliefRow[] {
		return this.beliefRows;
	}

	/** The refunds still missing a reason. Empty until ready. */
	get pending(): QuestionView[] {
		return this.pendingRows;
	}

	/** Why the load failed, or null. */
	get error(): string | null {
		return this.loadError;
	}

	/**
	 * Loads the rows the ledger table shows. A failed read reports an error
	 * and keeps the old rows. Evidence is not read here: no ledger row
	 * renders it, so it is not part of what this load promises.
	 */
	async refresh(): Promise<void> {
		this.loadStatus = 'loading';
		this.loadError = null;
		this.notify();
		try {
			const [beliefs, pending] = await Promise.all([
				this.client.listBeliefs({ shopper_id: this.shopperId, include_seeded: true }),
				this.client.listPendingQuestions({ shopper_id: this.shopperId })
			]);
			this.beliefRows = beliefs.beliefs;
			this.pendingRows = pending.questions;
			this.loadStatus = 'ready';
		} catch (error) {
			this.loadStatus = 'error';
			this.loadError = error instanceof Error ? error.message : 'Ledger load failed';
		}
		this.notify();
	}
}

/** State for the fixture story at one step, for the ledger page. */
export function createFixtureState(shopperId: string, step: LedgerFixtureStep): LedgerState {
	return new LedgerState(new LedgerClient(createFixtureTransport(step)), shopperId);
}
