// The gate for the merchant table: every seller line renders with its
// status pill, a keyboard user can reach every row, no rendered row can
// show a personal row, and the column list names no shopper field.

import './test-setup';

import { afterEach, describe, expect, it } from 'vitest';
import type { GridApi } from 'ag-grid-community';
import { fixturePools } from '$lib/pool';
import {
	createMerchantGrid,
	merchantColumnDefs,
	seededCellRenderer,
	statusCellRenderer,
	statusPillClass
} from './grid';
import { merchantGridTheme } from './theme';
import {
	SEEDED_MARK_LABEL,
	toMerchantGridRows,
	type MerchantGridRow
} from './rows';

let hosts: HTMLElement[] = [];

function makeHost(width = '800px'): HTMLElement {
	const host = document.createElement('div');
	host.style.width = width;
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
function afterFirstRender(api: GridApi<MerchantGridRow>): Promise<void> {
	return new Promise((resolve) => {
		const timer = setTimeout(resolve, 2000);
		api.addEventListener('firstDataRendered', () => {
			clearTimeout(timer);
			resolve();
		});
	});
}

async function mountAll(
	options: { width?: string; mode?: 'light' | 'dark' } = {}
): Promise<{ api: GridApi<MerchantGridRow>; host: HTMLElement; rows: MerchantGridRow[] }> {
	const rows = toMerchantGridRows(fixturePools().map((row) => row.pool));
	const host = makeHost(options.width);
	const api = createMerchantGrid(host, { rows, mode: options.mode ?? 'light' });
	await afterFirstRender(api);
	return { api, host, rows };
}

function focusedRowId(): string | null {
	const active = document.activeElement;
	if (!(active instanceof HTMLElement)) return null;
	return active.closest('.ag-row')?.getAttribute('row-id') ?? null;
}

afterEach(() => {
	for (const host of hosts) {
		host.remove();
	}
	hosts = [];
});

describe('merchant grid story', () => {
	it('renders every seller line with its status', async () => {
		const { api, host } = await mountAll({ mode: 'light' });
		try {
			expect(api.getDisplayedRowCount()).toBe(6);
			expect(host.querySelectorAll('.ag-row[row-id]').length).toBe(6);
			const texts = cellTexts(host).join(' ');
			expect(texts).toContain('brand-x-001');
			expect(texts).toContain('brand-w-005');
			for (const status of ['Earned', 'Building', 'Claimed', 'Out of true'] as const) {
				expect(texts).toContain(status);
			}
		} finally {
			api.destroy();
		}
	});

	it('marks the seeded count on every row', async () => {
		const { api, host } = await mountAll();
		try {
			const marks = [...host.querySelectorAll('.merchant-seeded')].map((mark) =>
				mark.textContent?.trim()
			);
			expect(marks).toHaveLength(6);
			expect(marks[0]).toContain(`48 ${SEEDED_MARK_LABEL}`);
		} finally {
			api.destroy();
		}
	});

	it('keeps every row across a theme switch in both directions', async () => {
		const { api } = await mountAll({ mode: 'dark' });
		try {
			expect(api.getDisplayedRowCount()).toBe(6);
			api.setGridOption('theme', merchantGridTheme('light'));
			expect(api.getDisplayedRowCount()).toBe(6);
			api.setGridOption('theme', merchantGridTheme('dark'));
			expect(api.getDisplayedRowCount()).toBe(6);
			const pills = [...(hosts[0] as HTMLElement).querySelectorAll('.merchant-pill')].map((pill) =>
				pill.textContent?.trim()
			);
			expect(pills.filter((pill) => pill === 'Building')).toHaveLength(3);
		} finally {
			api.destroy();
		}
	});

	it('stays usable at phone width', async () => {
		const { api, host } = await mountAll({ width: '360px' });
		try {
			expect(api.getDisplayedRowCount()).toBe(6);
			expect(host.querySelectorAll('.ag-row[row-id]').length).toBe(6);
			expect(host.querySelector('.ag-row[row-id="brand-w-005/shirts/regular/M"]')?.textContent).toContain(
				'Out of true'
			);
		} finally {
			api.destroy();
		}
	});

	it('renders no personal row on any grid row', async () => {
		const { api, host } = await mountAll();
		try {
			expect(host.textContent ?? '').not.toContain('shopper_');
			expect(host.textContent ?? '').not.toContain('shopper-');
			for (const cell of host.querySelectorAll('.ag-cell')) {
				expect(cell.textContent ?? '').not.toMatch(/shopper/i);
			}
		} finally {
			api.destroy();
		}
	});

	it('names no shopper field in any column', () => {
		const fields = merchantColumnDefs().map((column) => String(column.field));
		expect(fields.join(' ')).not.toMatch(/shopper/i);
		expect(fields).toContain('settled');
		expect(fields).toContain('status');
		expect(fields).toContain('seededSettled');
	});
});

describe('merchant grid keyboard', () => {
	it('reaches every row with arrow keys', async () => {
		const { api, host } = await mountAll();
		try {
			const first = host.querySelector('.ag-row[row-index="0"] .ag-cell') as HTMLElement | null;
			expect(first).not.toBeNull();
			first?.focus();
			expect(focusedRowId()).toBe('brand-x-001/shirts/regular/M');
			const visited = [focusedRowId()];
			for (let move = 0; move < 5; move += 1) {
				const active = document.activeElement;
				if (!(active instanceof HTMLElement)) break;
				active.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
				await new Promise((resolve) => setTimeout(resolve, 50));
				visited.push(focusedRowId());
			}
			expect(new Set(visited).size).toBeGreaterThan(1);
		} finally {
			api.destroy();
		}
	});

	it('gives every row a focusable cell', async () => {
		const { api, host } = await mountAll();
		try {
			for (const row of host.querySelectorAll('.ag-row[row-id]')) {
				const focusable = [...row.querySelectorAll('.ag-cell')].some((cell) =>
					cell.hasAttribute('tabindex')
				);
				expect(focusable).toBe(true);
			}
		} finally {
			api.destroy();
		}
	});
});

describe('merchant cell renderers', () => {
	it('treats each status distinctly', () => {
		expect(statusPillClass('Earned')).toContain('merchant-earned');
		expect(statusPillClass('Building')).toContain('merchant-building');
		expect(statusPillClass('Claimed')).toContain('merchant-claimed');
		expect(statusPillClass('Out of true')).toContain('merchant-out-of-true');
	});

	it('renders the status text in the pill', () => {
		const rendered = statusCellRenderer({ value: 'Earned' } as Parameters<
			typeof statusCellRenderer
		>[0]);
		expect(rendered).toContain('merchant-earned');
		expect(rendered).toContain('Earned');
	});

	it('marks the seeded count with its tooltip', () => {
		const rendered = seededCellRenderer({ value: 48 } as Parameters<
			typeof seededCellRenderer
		>[0]);
		expect(rendered).toContain('48');
		expect(rendered).toContain(SEEDED_MARK_LABEL);
		expect(rendered).toContain('title=');
	});

	it('escapes status text in the pill renderer', () => {
		const rendered = statusCellRenderer({ value: 'Earned<script>' } as unknown as Parameters<
			typeof statusCellRenderer
		>[0]);
		expect(rendered).toContain('&lt;script&gt;');
		expect(rendered).not.toContain('<script>');
	});
});
