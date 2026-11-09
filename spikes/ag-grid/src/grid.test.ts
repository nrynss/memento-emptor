import { afterEach, describe, expect, it, vi } from 'vitest';
import type { GridApi } from 'ag-grid-community';
import { BELIEF_ROWS, type BeliefRow } from './beliefs';
import { createBeliefGrid, type BeliefEdit } from './grid';

let hosts: HTMLElement[] = [];

function makeHost(): HTMLElement {
	const host = document.createElement('div');
	host.style.width = '800px';
	host.style.height = '400px';
	document.body.appendChild(host);
	hosts.push(host);
	return host;
}

function cellTexts(host: HTMLElement): string[] {
	return [...host.querySelectorAll('.ag-cell')].map((cell) => cell.textContent?.trim() ?? '');
}

/**
 * The grid paints rows on the next animation frame, so every assertion on
 * rendered output waits for first render first. The timeout fallback keeps
 * a silent grid a failure instead of a hang.
 */
function afterFirstRender(api: GridApi<BeliefRow>): Promise<void> {
	return new Promise((resolve) => {
		const timer = setTimeout(resolve, 2000);
		api.addEventListener('firstDataRendered', () => {
			clearTimeout(timer);
			resolve();
		});
	});
}

afterEach(() => {
	for (const host of hosts) {
		host.remove();
	}
	hosts = [];
	vi.restoreAllMocks();
});

describe('belief grid', () => {
	it('renders the four belief statuses', async () => {
		const api = createBeliefGrid(makeHost());
		try {
			await afterFirstRender(api);
			expect(api.getDisplayedRowCount()).toBe(4);
			const texts = cellTexts(hosts[0] as HTMLElement);
			for (const row of BELIEF_ROWS) {
				expect(texts).toContain(row.status);
			}
		} finally {
			api.destroy();
		}
	});

	it('renders the status pill renderer in the status column', async () => {
		const api = createBeliefGrid(makeHost());
		try {
			await afterFirstRender(api);
			const pills = [
				...(hosts[0] as HTMLElement).querySelectorAll('.status-pill')
			].map((pill) => pill.textContent?.trim());
			expect(pills.sort()).toEqual(['Building', 'Claimed', 'Earned', 'Out of true']);
		} finally {
			api.destroy();
		}
	});

	it('reports a committed note edit through onEdit', async () => {
		const edits: BeliefEdit[] = [];
		const api = createBeliefGrid(makeHost(), {
			onEdit: (edit) => {
				edits.push(edit);
			}
		});
		try {
			await afterFirstRender(api);
			const node = api.getRowNode('belief-brandx-fit');
			expect(node).toBeDefined();
			node?.setDataValue('note', 'Shopper confirmed L fits');
			await new Promise((resolve) => setTimeout(resolve, 200));
			expect(edits).toEqual([
				{
					id: 'belief-brandx-fit',
					field: 'note',
					oldValue: '',
					newValue: 'Shopper confirmed L fits'
				}
			]);
			expect(api.getRowNode('belief-brandx-fit')?.data?.note).toBe('Shopper confirmed L fits');
		} finally {
			api.destroy();
		}
	});

	it('accepts a status change to another ledger status', async () => {
		const api = createBeliefGrid(makeHost());
		try {
			await afterFirstRender(api);
			api.getRowNode('belief-brandy-fit')?.setDataValue('status', 'Earned');
			expect(api.getRowNode('belief-brandy-fit')?.data?.status).toBe('Earned');
			const pills = [...(hosts[0] as HTMLElement).querySelectorAll('.status-pill')].map(
				(pill) => pill.textContent?.trim()
			);
			expect(pills).toContain('Earned');
		} finally {
			api.destroy();
		}
	});

	it('applies the dark theme without losing rows', async () => {
		const api = createBeliefGrid(makeHost(), { mode: 'dark' });
		try {
			expect(api.getDisplayedRowCount()).toBe(4);
			const { chaayaGridTheme } = await import('./gridTheme');
			api.setGridOption('theme', chaayaGridTheme('light'));
			expect(api.getDisplayedRowCount()).toBe(4);
		} finally {
			api.destroy();
		}
	});

	it('rows keep their identity after an edit', async () => {
		const api = createBeliefGrid(makeHost());
		try {
			const before = api.getRowNode('belief-usually-m');
			before?.setDataValue('note', 'Says M, returns say L');
			expect(api.getRowNode('belief-usually-m')?.data?.belief).toBe(
				(BELIEF_ROWS.find((row: BeliefRow) => row.id === 'belief-usually-m') as BeliefRow).belief
			);
		} finally {
			api.destroy();
		}
	});
});
