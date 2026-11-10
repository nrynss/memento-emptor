// The merchant grid through AG Grid's plain JavaScript API. No framework
// wrapper is used, so there is no framework-adapter version to track. The
// grid is read-only: the merchant app explains the pool, and only the
// layer writes it.

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
	escapeMerchantHtml,
	SEEDED_MARK_LABEL,
	SEEDED_MARK_TOOLTIP,
	type MerchantGridRow
} from './rows';
import { merchantGridTheme, type MerchantGridMode } from './theme';

ModuleRegistry.registerModules([AllCommunityModule]);

/** CSS class for the status pill, one treatment per pooled status. */
export function statusPillClass(status: MerchantGridRow['status']): string {
	switch (status) {
		case 'Earned':
			return 'merchant-pill merchant-earned';
		case 'Building':
			return 'merchant-pill merchant-building';
		case 'Claimed':
			return 'merchant-pill merchant-claimed';
		case 'Out of true':
			return 'merchant-pill merchant-out-of-true';
	}
}

/** The status column: a pill per pooled status. */
export function statusCellRenderer(params: ICellRendererParams<MerchantGridRow>): string {
	const status = params.value as MerchantGridRow['status'];
	return `<span class="${statusPillClass(status)}">${escapeMerchantHtml(status)}</span>`;
}

/** The seeded column: the seeded part of the settled count, always shown. */
export function seededCellRenderer(params: ICellRendererParams<MerchantGridRow>): string {
	const count = params.value as number;
	return `<span class="merchant-seeded" title="${escapeMerchantHtml(SEEDED_MARK_TOOLTIP)}">${escapeMerchantHtml(String(count))} ${SEEDED_MARK_LABEL}</span>`;
}

/**
 * The merchant columns: line and size, settled purchases, returns by
 * direction, reasons by category, fits-well feedback, pool status and
 * direction, and the seeded count. No column names a shopper: the pool
 * carries counts, never identities.
 */
export function merchantColumnDefs(): ColDef<MerchantGridRow>[] {
	return [
		{ field: 'lineLabel', headerName: 'Line', flex: 2, minWidth: 180 },
		{ field: 'size', headerName: 'Size', width: 80 },
		{ field: 'settled', headerName: 'Settled', width: 90 },
		{ field: 'returnsSmall', headerName: 'Too small', width: 100 },
		{ field: 'returnsLarge', headerName: 'Too large', width: 100 },
		{ field: 'fitsWell', headerName: 'Fits well', width: 90 },
		{ field: 'reasonsFitSmall', headerName: 'Fit: small', width: 100 },
		{ field: 'reasonsFitLarge', headerName: 'Fit: large', width: 100 },
		{ field: 'reasonsDamaged', headerName: 'Damaged', width: 100 },
		{ field: 'reasonsNotArrived', headerName: 'Not arrived', width: 110 },
		{ field: 'reasonsChangedMind', headerName: 'Changed mind', width: 120 },
		{
			field: 'status',
			headerName: 'Status',
			flex: 1,
			minWidth: 110,
			cellRenderer: statusCellRenderer
		},
		{ field: 'direction', headerName: 'Direction', width: 110 },
		{
			field: 'seededSettled',
			headerName: 'Seeded',
			width: 120,
			cellRenderer: seededCellRenderer
		}
	];
}

export interface MerchantGridOptions {
	rows?: MerchantGridRow[];
	mode?: MerchantGridMode;
	onSelect?: (rowId: string | null) => void;
}

/**
 * Mounts the merchant grid on a host element. The caller owns the host's
 * lifetime and must call `destroy()` on the returned API. Module
 * registration runs once at import. A row opens its why when clicked.
 * Columns never virtualize away: narrow columns would drop out of a
 * headless DOM that reports no width, which would fail the gate loudly.
 */
export function createMerchantGrid(
	host: HTMLElement,
	options: MerchantGridOptions = {}
): GridApi<MerchantGridRow> {
	const gridOptions: GridOptions<MerchantGridRow> = {
		theme: merchantGridTheme(options.mode ?? 'light'),
		columnDefs: merchantColumnDefs(),
		rowData: options.rows ?? [],
		getRowId: (params) => params.data.id,
		defaultColDef: {
			resizable: true,
			sortable: true
		},
		rowSelection: 'single',
		suppressColumnVirtualisation: true,
		domLayout: 'autoHeight',
		onRowClicked: (event) => {
			options.onSelect?.(event.data?.id ?? null);
		}
	};
	return createGrid(host, gridOptions);
}
