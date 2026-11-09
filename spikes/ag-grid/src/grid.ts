import {
	AllCommunityModule,
	createGrid,
	ModuleRegistry,
	type CellValueChangedEvent,
	type GridApi,
	type GridOptions
} from 'ag-grid-community';
import { BELIEF_ROWS, beliefColumnDefs, type BeliefRow } from './beliefs';
import { chaayaGridTheme, type GridMode } from './gridTheme';

ModuleRegistry.registerModules([AllCommunityModule]);

/** One committed cell edit: which row and field changed, and from what. */
export interface BeliefEdit {
	id: string;
	field: string;
	oldValue: string;
	newValue: string;
}

export interface BeliefGridOptions {
	initialRows?: BeliefRow[];
	mode?: GridMode;
	onEdit?: (edit: BeliefEdit) => void;
}

/**
 * Mount the belief ledger grid on a host element through AG Grid's plain
 * JavaScript API. The caller owns the host's lifetime and must call
 * `destroy()` on the returned API. Module registration runs once at import.
 */
export function createBeliefGrid(host: HTMLElement, options: BeliefGridOptions = {}): GridApi<BeliefRow> {
	const gridOptions: GridOptions<BeliefRow> = {
		theme: chaayaGridTheme(options.mode ?? 'light'),
		columnDefs: beliefColumnDefs(),
		rowData: options.initialRows ?? BELIEF_ROWS,
		getRowId: (params) => params.data.id,
		defaultColDef: {
			resizable: true,
			sortable: true
		},
		// The ledger grid is five narrow columns that always fit side by side,
		// so column virtualization buys nothing and hiding it keeps rendering
		// complete in headless DOMs that report no width.
		suppressColumnVirtualisation: true,
		onCellValueChanged: (event: CellValueChangedEvent<BeliefRow>) => {
			if (event.data === undefined || event.colDef.field === undefined) {
				return;
			}
			options.onEdit?.({
				id: event.data.id,
				field: event.colDef.field,
				oldValue: String(event.oldValue ?? ''),
				newValue: String(event.newValue ?? '')
			});
		}
	};
	return createGrid(host, gridOptions);
}
