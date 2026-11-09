// The flat rows the ledger grid renders. This module imports only types
// from the ledger client, so the ledger page can map rows without loading
// AG Grid: the grid bundle stays behind the lazy-loaded grid component.

import type { BeliefStatus, BeliefView, LedgerBeliefRow } from '$lib/api';

/** One-line tooltip explaining Out of true, shown where it first appears. */
export const OUT_OF_TRUE_TOOLTIP =
	'Out of true: the shopper corrected this belief, which stays on record but no longer decides.';

/** The mark for a kept-without-feedback row: inferred, never confirmed. */
export const WEAK_SIGNAL_LABEL = 'weak signal';

/** Tooltip for the weak-signal mark. */
export const WEAK_SIGNAL_TOOLTIP =
	'Kept without feedback: a weak inferred signal, not confirmed satisfaction.';

/** The visible mark on a seeded row. */
export const SEEDED_MARK_LABEL = 'Seeded';

/** Tooltip for the seeded mark. */
export const SEEDED_MARK_TOOLTIP = 'Seeded history: generated, not lived.';

/** One ledger belief flattened for a grid row. */
export interface LedgerGridRow {
	id: string;
	kindLabel: string;
	subject: string;
	value: string;
	status: BeliefStatus;
	evidenceCount: number;
	weakSignal: boolean;
	lastChange: string;
	lastChangeFull: string;
	seeded: boolean;
	correction: string | null;
}

/** The kind word the grid shows: Fit, or Seller reliability. */
export function kindLabelFor(kind: BeliefView['kind']): string {
	return kind === 'fit' ? 'Fit' : 'Seller reliability';
}

/** The calendar date of an ISO timestamp, without timezone drift. */
export function formatLedgerDate(iso: string): string {
	return iso.slice(0, 10);
}

/**
 * Whether any of the belief's evidence is a kept item with no feedback.
 * Kept-past-the-window rows arrive as observations, never as structural
 * writes, so the grid labels them weak instead of counting them as support.
 */
export function hasWeakSignal(belief: BeliefView): boolean {
	return belief.evidence.some(
		(row) => row.write_op === 'observe' || row.outcome.kind === 'kept'
	);
}

/** Flattens one ledger row for the grid, keeping every display field. */
export function toLedgerGridRow(row: LedgerBeliefRow): LedgerGridRow {
	const belief = row.belief;
	return {
		id: belief.belief_id,
		kindLabel: kindLabelFor(belief.kind),
		subject: belief.subject.name,
		value: belief.value,
		status: belief.status,
		evidenceCount: row.evidence_count,
		weakSignal: hasWeakSignal(belief),
		lastChange: formatLedgerDate(row.last_change_at),
		lastChangeFull: row.last_change_at,
		seeded: belief.seeded,
		correction: belief.contradicted_by?.text ?? null
	};
}

/** Flattens every ledger row for the grid, keeping the strongest-first order. */
export function toLedgerGridRows(rows: LedgerBeliefRow[]): LedgerGridRow[] {
	return rows.map(toLedgerGridRow);
}

/** Escapes text for the grid's HTML-string cell renderers. */
export function escapeLedgerHtml(text: string): string {
	return text
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#39;');
}
