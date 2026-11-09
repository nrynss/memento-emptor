// The contest affordances for the ledger grid, owned by this task. The
// read-only grid in $lib/ledger stays untouched: these defs add one
// Contest column beside its columns, and the renderer escapes every id it
// embeds. A host page concatenates these defs after ledgerColumnDefs() and
// listens for clicks on the Contest button by belief id.
import type { ColDef, ICellRendererParams } from 'ag-grid-community';
import { escapeLedgerHtml, type LedgerGridRow } from '$lib/ledger/rows';

/** The label on the per-row contest button. */
export const CONTEST_ACTION_LABEL = 'Contest';

/** The attribute carrying the contested belief id on the Contest button. */
export const CONTEST_BUTTON_ATTRIBUTE = 'data-contest-belief';

/** One Contest button per row, carrying its belief id for the host page. */
export function contestCellRenderer(params: ICellRendererParams<LedgerGridRow>): string {
	const id = params.data?.id ?? '';
	const style =
		'font:inherit;color:var(--accent);background:transparent;' +
		'border:1px solid var(--accent);border-radius:var(--radius-control);' +
		'padding:0.2rem 0.8rem;cursor:pointer;white-space:nowrap';
	return `<button type="button" class="ledger-contest" style="${style}" ${CONTEST_BUTTON_ATTRIBUTE}="${escapeLedgerHtml(id)}">${CONTEST_ACTION_LABEL}</button>`;
}

/** The contest column defs: one labelled button column, never sortable. */
export function contestEditColumnDefs(): ColDef<LedgerGridRow>[] {
	return [
		{
			colId: 'contest',
			headerName: 'Contest',
			field: 'id',
			width: 110,
			sortable: false,
			resizable: false,
			cellRenderer: contestCellRenderer
		}
	];
}
