import { describe, expect, it } from 'vitest';
import {
	BELIEF_ROWS,
	BELIEF_STATUSES,
	beliefColumnDefs,
	seededCellRenderer,
	statusCellRenderer,
	statusPillClass,
	type BeliefRow
} from './beliefs';

function rendererValue(html: string): string {
	return html;
}

describe('ledger statuses', () => {
	it('covers the four statuses product.md names', () => {
		expect([...BELIEF_STATUSES]).toEqual(['Claimed', 'Building', 'Earned', 'Out of true']);
	});

	it('seeds one row per status', () => {
		const statuses = BELIEF_ROWS.map((row) => row.status).sort();
		expect(statuses).toEqual(['Building', 'Claimed', 'Earned', 'Out of true']);
	});

	it('gives every row a stable id for getRowId', () => {
		const ids = BELIEF_ROWS.map((row) => row.id);
		expect(new Set(ids).size).toBe(BELIEF_ROWS.length);
	});
});

describe('status renderer', () => {
	it('renders each status as a labelled pill', () => {
		for (const status of BELIEF_STATUSES) {
			const html = rendererValue(
				statusCellRenderer({ value: status } as unknown as Parameters<typeof statusCellRenderer>[0])
			);
			expect(html).toContain(status);
			expect(html).toContain(statusPillClass(status));
		}
	});

	it('paints Out of true as a stop-state pill, never as earned', () => {
		expect(statusPillClass('Out of true')).toContain('out-of-true');
		expect(statusPillClass('Out of true')).not.toContain('earned');
	});
});

describe('seeded renderer', () => {
	it('marks seeded rows and leaves the rest blank', () => {
		const marked = seededCellRenderer({ value: true } as unknown as Parameters<
			typeof seededCellRenderer
		>[0]);
		const unmarked = seededCellRenderer({ value: false } as unknown as Parameters<
			typeof seededCellRenderer
		>[0]);
		expect(marked).toContain('seeded-mark');
		expect(unmarked).toBe('');
	});
});

describe('column definitions', () => {
	it('makes status and note editable, nothing else', () => {
		const defs = beliefColumnDefs();
		const editable = defs.filter((def) => def.editable === true).map((def) => def.field);
		expect(editable.sort()).toEqual(['note', 'status']);
	});

	it('edits status through the four-status select editor', () => {
		const status = beliefColumnDefs().find((def) => def.field === 'status');
		expect(status?.cellEditor).toBe('agSelectCellEditor');
		expect(status?.cellEditorParams).toEqual({ values: [...BELIEF_STATUSES] });
	});

	it('keeps row identity stable for edits', () => {
		const row: BeliefRow = BELIEF_ROWS[0] as BeliefRow;
		expect(row.id).toBe('belief-brandx-fit');
	});
});
