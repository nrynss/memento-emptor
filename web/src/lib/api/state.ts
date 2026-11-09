// Observable ledger state for Svelte 5 views. The class is
// framework-neutral: a view wraps it with Chaaya's reactive() and reads
// the getters in a template or $derived, and notify() re-runs them. One
// state instance serves one step; a step picker builds a new one.

import { Notifier } from '@nrynss/chaaya/core';
import { LedgerClient } from './client';
import { createFixtureTransport, type LedgerFixtureStep } from './fixture';
import type { LedgerBeliefRow, LedgerEvidenceRow, QuestionView } from './types';

/** Where a load stands. It starts idle and never returns there. */
export type LedgerLoadStatus = 'idle' | 'loading' | 'ready' | 'error';

/** One shopper's ledger rows, loaded through a typed client. */
export class LedgerState extends Notifier {
	private readonly client: LedgerClient;
	private readonly shopperId: string;
	private loadStatus: LedgerLoadStatus = 'idle';
	private beliefRows: LedgerBeliefRow[] = [];
	private evidenceRows: LedgerEvidenceRow[] = [];
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

	/** The evidence rows, newest first. Empty until ready. */
	get evidence(): LedgerEvidenceRow[] {
		return this.evidenceRows;
	}

	/** The refunds still missing a reason. Empty until ready. */
	get pending(): QuestionView[] {
		return this.pendingRows;
	}

	/** Why the load failed, or null. */
	get error(): string | null {
		return this.loadError;
	}

	/** Loads every row for the shopper. A failure keeps the old rows. */
	async refresh(): Promise<void> {
		this.loadStatus = 'loading';
		this.loadError = null;
		this.notify();
		try {
			const [beliefs, evidence, pending] = await Promise.all([
				this.client.listBeliefs({ shopper_id: this.shopperId, include_seeded: true }),
				this.client.listEvidence({ shopper_id: this.shopperId }),
				this.client.listPendingQuestions({ shopper_id: this.shopperId })
			]);
			this.beliefRows = beliefs.beliefs;
			this.evidenceRows = evidence.evidence;
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
