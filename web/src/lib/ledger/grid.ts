// The ledger grid through AG Grid's plain JavaScript API. No framework
// wrapper is used, so there is no framework-adapter version to track. The
// grid is read-only: contesting a belief belongs to the contest task, so
// no column edits and no edit callback exist here.

import {
	AllCommunityModule,
	createGrid,
	ModuleRegistry,
	type ColDef,
	type GridApi,
	type GridOptions,
	type ICellRendererParams
} from 'ag-grid-community';
import {
	escapeLedgerHtml,
	OUT_OF_TRUE_TOOLTIP,
	SEEDED_MARK_LABEL,
	SEEDED_MARK_TOOLTIP,
	WEAK_SIGNAL_LABEL,
	WEAK_SIGNAL_TOOLTIP,
	type LedgerGridRow
} from './rows';
import { ledgerGridTheme, type LedgerGridMode } from './theme';

ModuleRegistry.registerModules([AllCommunityModule]);

/** CSS class for the status pill, one treatment per ledger status. */
export function statusPillClass(status: LedgerGridRow['status']): string {
	switch (status) {
		case 'Earned':
			return 'ledger-pill ledger-earned';
		case 'Building':
			return 'ledger-pill ledger-building';
		case 'Claimed':
			return 'ledger-pill ledger-claimed';
		case 'Out of true':
			return 'ledger-pill ledger-out-of-true';
	}
}

/**
 * The status column: a pill per status, with the one-line Out-of-true
 * tooltip on the contradicted pill, and the shopper's correction below it.
 */
export function statusCellRenderer(params: ICellRendererParams<LedgerGridRow>): string {
	const status = params.value as LedgerGridRow['status'];
	const tooltip =
		status === 'Out of true' ? ` title="${escapeLedgerHtml(OUT_OF_TRUE_TOOLTIP)}"` : '';
	const pill = `<span class="${statusPillClass(status)}"${tooltip}>${escapeLedgerHtml(status)}</span>`;
	const correction = params.data?.correction;
	if (status === 'Out of true' && typeof correction === 'string' && correction !== '') {
		return `${pill}<span class="ledger-correction">${escapeLedgerHtml(correction)}</span>`;
	}
	return pill;
}

/**
 * The evidence-count column: the count, plus the weak-signal mark when the
 * belief rests on kept-without-feedback observations.
 */
export function evidenceCellRenderer(params: ICellRendererParams<LedgerGridRow>): string {
	const count = `<span class="ledger-count">${params.value as number}</span>`;
	if (params.data?.weakSignal !== true) {
		return count;
	}
	return `${count} <span class="ledger-weak" title="${escapeLedgerHtml(WEAK_SIGNAL_TOOLTIP)}">${WEAK_SIGNAL_LABEL}</span>`;
}

/** The seeded column: a visible mark on seeded rows, nothing elsewhere. */
export function seededCellRenderer(params: ICellRendererParams<LedgerGridRow>): string {
	if (params.value !== true) {
		return '';
	}
	return `<span class="ledger-seeded" title="${escapeLedgerHtml(SEEDED_MARK_TOOLTIP)}">${SEEDED_MARK_LABEL}</span>`;
}

/** The last-change column: the calendar date, with the full stamp on hover. */
export function lastChangeCellRenderer(params: ICellRendererParams<LedgerGridRow>): string {
	const full = params.data?.lastChangeFull ?? '';
	return `<span title="${escapeLedgerHtml(full)}">${escapeLedgerHtml(params.value as string)}</span>`;
}

/** The six ledger columns plus the seeded mark. Status and seeded carry the renderers. */
export function ledgerColumnDefs(): ColDef<LedgerGridRow>[] {
	return [
		{ field: 'kindLabel', headerName: 'Kind', flex: 1, minWidth: 90 },
		{ field: 'subject', headerName: 'Subject', flex: 1, minWidth: 90 },
		{
			field: 'value',
			headerName: 'Belief',
			flex: 2,
			minWidth: 140,
			wrapText: true,
			autoHeight: true
		},
		{ field: 'status', headerName: 'Status', flex: 1, minWidth: 110, cellRenderer: statusCellRenderer },
		{
			field: 'evidenceCount',
			headerName: 'Evidence',
			flex: 1,
			minWidth: 80,
			cellRenderer: evidenceCellRenderer
		},
		{
			field: 'lastChange',
			headerName: 'Last change',
			flex: 1,
			minWidth: 100,
			cellRenderer: lastChangeCellRenderer
		},
		{
			field: 'seeded',
			headerName: 'Seeded',
			width: 84,
			cellRenderer: seededCellRenderer
		}
	];
}

export interface LedgerGridOptions {
	rows?: LedgerGridRow[];
	mode?: LedgerGridMode;
}

/**
 * Mounts the ledger grid on a host element. The caller owns the host's
 * lifetime and must call `destroy()` on the returned API. Module
 * registration runs once at import. Columns never virtualize away: the
 * seven narrow columns always fit side by side, which keeps rendering
 * complete in headless DOMs that report no width.
 */
export function createLedgerGrid(
	host: HTMLElement,
	options: LedgerGridOptions = {}
): GridApi<LedgerGridRow> {
	const gridOptions: GridOptions<LedgerGridRow> = {
		theme: ledgerGridTheme(options.mode ?? 'light'),
		columnDefs: ledgerColumnDefs(),
		rowData: options.rows ?? [],
		getRowId: (params) => params.data.id,
		defaultColDef: {
			resizable: true,
			sortable: true
		},
		suppressColumnVirtualisation: true,
		domLayout: 'autoHeight'
	};
	return createGrid(host, gridOptions);
}
