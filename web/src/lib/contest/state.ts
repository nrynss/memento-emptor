// Editable ledger rows in Svelte 5 views. The state holds the rows the
// ledger page already loaded and sends every edit through the correction
// client: the shopper's words go to record_correction, and the rows update
// from the layer's decoded answer. Nothing here rewrites a belief on its
// own — every field applied below comes from the correction response, so
// the evidence stays untouched and the contradicted row stays visible. A
// correction with no belief yet stands on its own beside the rows.
import { Notifier } from '@nrynss/chaaya/core';
import { LedgerApiError, type LedgerBeliefRow } from '$lib/api';
import { CorrectionClient } from './client';
import { createFixtureCorrectionTransport } from './fixture';
import type { ContestCorrectionView, RecordCorrectionResponse } from './types';

/** Where a correction stands. It starts idle and never returns there. */
export type ContestStatus = 'idle' | 'sending' | 'ready' | 'error';

/** What the form promises before anything is typed, in plain words. */
export const correctionPreviewLine =
	'Your correction now decides — the old belief stays below for the record.';

/** The plain-words result line for a recorded correction. */
export function confirmationLineFor(correctionText: string): string {
	return `Your correction now decides: “${correctionText}”. The old belief stays below for the record.`;
}

/**
 * Applies the layer's correction answer to its row. The status, standing
 * and warning come from the response; the evidence and the row itself are
 * carried over untouched, so the belief stays on record beside the edit.
 */function applyCorrectionResponse(
	row: LedgerBeliefRow,
	response: RecordCorrectionResponse
): LedgerBeliefRow {
	return {
		belief: {
			...row.belief,
			status: response.status,
			standing: response.standing,
			contradicted: response.contradicted,
			contradicted_by:
				response.correction.contradicts_belief_id === undefined
					? row.belief.contradicted_by
					: {
							correction_id: response.correction.correction_id,
							text: response.correction.text,
							contradicts_belief_id: response.correction.contradicts_belief_id,
							recorded_at: response.correction.recorded_at,
							takes_precedence: true
						},
			updated_at: response.correction.recorded_at
		},
		evidence_count: row.evidence_count,
		last_change_at: response.correction.recorded_at
	};
}

/** One shopper's editable rows, sent through record_correction only. */
export class ContestState extends Notifier {
	private readonly client: CorrectionClient;
	private readonly shopperId: string;
	private rows: LedgerBeliefRow[];
	private standing: ContestCorrectionView[] = [];
	private opStatus: ContestStatus = 'idle';
	private opError: string | null = null;
	private lastResponse: RecordCorrectionResponse | null = null;

	constructor(client: CorrectionClient, shopperId: string, rows: LedgerBeliefRow[]) {
		super();
		this.client = client;
		this.shopperId = shopperId;
		this.rows = [...rows];
	}

	/** Where the latest correction stands. */
	get status(): ContestStatus {
		return this.opStatus;
	}

	/** The rows, with any contested row kept in place. */
	get beliefs(): LedgerBeliefRow[] {
		return this.rows;
	}

	/** Corrections that stand on their own, with no belief yet, newest first. */
	get standingCorrections(): ContestCorrectionView[] {
		return this.standing;
	}

	/** Why the latest correction failed, or null. */
	get error(): string | null {
		return this.opError;
	}

	/** The layer's answer to the latest correction, or null. */
	get latest(): RecordCorrectionResponse | null {
		return this.lastResponse;
	}

	/**
	 * Sends the shopper's edit through record_correction and applies the
	 * layer's answer to its row. A blank edit and an unknown belief fail
	 * before anything is sent.
	 */
	async submitCorrection(beliefId: string, text: string): Promise<RecordCorrectionResponse> {
		const words = text.trim();
		if (words === '') {
			throw new LedgerApiError('unknown-request', 'a correction needs the shopper in words');
		}
		if (!this.rows.some((row) => row.belief.belief_id === beliefId)) {
			throw new LedgerApiError('unknown-request', 'no such belief in these rows');
		}
		this.opStatus = 'sending';
		this.opError = null;
		this.notify();
		try {
			const response = await this.client.recordCorrection({
				shopper_id: this.shopperId,
				text: words,
				contradicts_belief_id: beliefId
			});
			const answered = response.belief_id;
			if (answered !== undefined) {
				this.rows = this.rows.map((row) =>
					row.belief.belief_id === answered ? applyCorrectionResponse(row, response) : row
				);
			}
			this.lastResponse = response;
			this.opStatus = 'ready';
			this.notify();
			return response;
		} catch (error) {
			this.opStatus = 'error';
			this.opError = error instanceof Error ? error.message : 'The correction failed.';
			this.notify();
			throw error;
		}
	}

	/**
	 * Sends the shopper's correction with no belief yet. The layer keeps
	 * it as a standing record that decides at once; the rows stay
	 * untouched, and the correction lists beside them.
	 */
	async submitStandingCorrection(text: string): Promise<RecordCorrectionResponse> {
		const words = text.trim();
		if (words === '') {
			throw new LedgerApiError('unknown-request', 'a correction needs the shopper in words');
		}
		this.opStatus = 'sending';
		this.opError = null;
		this.notify();
		try {
			const response = await this.client.recordCorrection({
				shopper_id: this.shopperId,
				text: words
			});
			this.standing = [response.correction, ...this.standing];
			this.lastResponse = response;
			this.opStatus = 'ready';
			this.notify();
			return response;
		} catch (error) {
			this.opStatus = 'error';
			this.opError = error instanceof Error ? error.message : 'The correction failed.';
			this.notify();
			throw error;
		}
	}

	/**
	 * The current decision for a belief: the shopper's correction wins at
	 * once with precedence, and the belief value decides only while no
	 * correction contradicts it.
	 */
	decisionFor(beliefId: string): string | null {
		const row = this.rows.find((candidate) => candidate.belief.belief_id === beliefId);
		if (!row) return null;
		return row.belief.contradicted_by?.text ?? row.belief.value;
	}

	/**
	 * The plain-words result of the latest correction, or null before one
	 * lands. The ledger page and the editor read the same sentence, so the
	 * result looks the same wherever the judge sees it.
	 */
	confirmationLine(): string | null {
		if (this.opStatus !== 'ready' || this.lastResponse === null) return null;
		return confirmationLineFor(this.lastResponse.correction.text);
	}
}

/** Editable rows over the fixture correction path, for the ledger page. */
export function createFixtureContestState(
	shopperId: string,
	rows: LedgerBeliefRow[]
): ContestState {
	return new ContestState(new CorrectionClient(createFixtureCorrectionTransport()), shopperId, rows);
}
