import type { ColDef, ICellRendererParams } from 'ag-grid-community';

/**
 * The four ledger statuses from product.md, in display order. Each maps a
 * Lambo state to the word the evidence ledger shows the shopper.
 */
export const BELIEF_STATUSES = ['Claimed', 'Building', 'Earned', 'Out of true'] as const;

export type BeliefStatus = (typeof BELIEF_STATUSES)[number];

/** One row of the spike ledger: a belief, its evidence, and its status. */
export interface BeliefRow {
	id: string;
	belief: string;
	evidence: string;
	status: BeliefStatus;
	note: string;
	seeded: boolean;
}

/** The spike's four rows, one per ledger status. */
export const BELIEF_ROWS: BeliefRow[] = [
	{
		id: 'belief-brandx-fit',
		belief: 'Brand X runs small, size up to L',
		evidence: '3 fit returns with reason fit: too small',
		status: 'Earned',
		note: '',
		seeded: true
	},
	{
		id: 'belief-brandy-fit',
		belief: 'Brand Y is true to size',
		evidence: '1 fit return with reason fit: too small',
		status: 'Building',
		note: '',
		seeded: true
	},
	{
		id: 'belief-usually-m',
		belief: 'Shopper usually wears M',
		evidence: 'General statement, no transaction',
		status: 'Claimed',
		note: '',
		seeded: false
	},
	{
		id: 'belief-brandx-l',
		belief: 'Size L for Brand X',
		evidence: 'Contradicted by correction "use M"',
		status: 'Out of true',
		note: '',
		seeded: true
	}
];

/** CSS class for the status pill, driven by Chaaya state tokens. */
export function statusPillClass(status: BeliefStatus): string {
	switch (status) {
		case 'Earned':
			return 'status-pill status-earned';
		case 'Building':
			return 'status-pill status-building';
		case 'Claimed':
			return 'status-pill status-claimed';
		case 'Out of true':
			return 'status-pill status-out-of-true';
	}
}

/**
 * Custom cell renderer for the status column. A plain function returning an
 * HTML string, the shape AG Grid's plain JavaScript API expects. The value
 * comes from the fixed status union, so there is nothing to escape.
 */
export function statusCellRenderer(params: ICellRendererParams<BeliefRow>): string {
	const status = params.value as BeliefStatus;
	return `<span class="${statusPillClass(status)}">${status}</span>`;
}

/** Renderer for the seeded-mark column. */
export function seededCellRenderer(params: ICellRendererParams<BeliefRow>): string {
	return params.value === true ? '<span class="seeded-mark" title="Seeded row">●</span>' : '';
}

/** Column definitions for the spike grid. Status and note edit in place. */
export function beliefColumnDefs(): ColDef<BeliefRow>[] {
	return [
		{ field: 'belief', headerName: 'Belief', flex: 2, minWidth: 220 },
		{ field: 'evidence', headerName: 'Evidence', flex: 2, minWidth: 220 },
		{
			field: 'status',
			headerName: 'Status',
			width: 140,
			cellRenderer: statusCellRenderer,
			editable: true,
			cellEditor: 'agSelectCellEditor',
			cellEditorParams: { values: [...BELIEF_STATUSES] }
		},
		{ field: 'note', headerName: 'Shopper note', flex: 1, minWidth: 160, editable: true },
		{
			field: 'seeded',
			headerName: 'Seeded',
			width: 90,
			cellRenderer: seededCellRenderer
		}
	];
}
