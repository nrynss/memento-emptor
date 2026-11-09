// The gate for the ledger table: the fixture story renders correctly at
// every step in both themes, and a keyboard user can reach every row.

import './test-setup';

import { afterEach, describe, expect, it } from 'vitest';
import type { GridApi } from 'ag-grid-community';
import {
	createFixtureTransport,
	fixtureShopperId,
	LedgerClient,
	type LedgerFixtureStep
} from '$lib/api';
import {
	createLedgerGrid,
	evidenceCellRenderer,
	lastChangeCellRenderer,
	seededCellRenderer,
	statusCellRenderer,
	statusPillClass
} from './grid';
import { ledgerGridTheme } from './theme';
import {
	OUT_OF_TRUE_TOOLTIP,
	toLedgerGridRows,
	WEAK_SIGNAL_LABEL,
	WEAK_SIGNAL_TOOLTIP,
	type LedgerGridRow
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
function afterFirstRender(api: GridApi<LedgerGridRow>): Promise<void> {
	return new Promise((resolve) => {
		const timer = setTimeout(resolve, 2000);
		api.addEventListener('firstDataRendered', () => {
			clearTimeout(timer);
			resolve();
		});
	});
}

async function mountAt(
	step: LedgerFixtureStep,
	options: { width?: string; mode?: 'light' | 'dark' } = {}
): Promise<{ api: GridApi<LedgerGridRow>; host: HTMLElement; rows: LedgerGridRow[] }> {
	const client = new LedgerClient(createFixtureTransport(step));
	const beliefs = await client.listBeliefs({ shopper_id: fixtureShopperId, include_seeded: true });
	const rows = toLedgerGridRows(beliefs.beliefs);
	const host = makeHost(options.width);
	const api = createLedgerGrid(host, { rows, mode: options.mode ?? 'light' });
	await afterFirstRender(api);
	return { api, host, rows };
}

function focusedRowId(): string | null {
	const active = document.activeElement;
	if (!(active instanceof HTMLElement)) return null;
	return active.closest('.ag-row')?.getAttribute('row-id') ?? null;
}

function pressArrowDown(): void {
	const active = document.activeElement;
	if (!(active instanceof HTMLElement)) throw new Error('nothing focused to move from');
	active.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
}

function pressArrowUp(): void {
	const active = document.activeElement;
	if (!(active instanceof HTMLElement)) throw new Error('nothing focused to move from');
	active.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true }));
}

afterEach(() => {
	for (const host of hosts) {
		host.remove();
	}
	hosts = [];
});

const expectedCounts = [1, 2, 2, 2, 2, 3, 3, 3, 4] as const;

describe('ledger grid story', () => {
	const steps = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;
	for (const [index, step] of steps.entries()) {
		it(`renders step ${String(step)} with ${String(expectedCounts[index])} rows`, async () => {
			const { api, host } = await mountAt(step, {
				mode: step % 2 === 0 ? 'dark' : 'light'
			});
			try {
				expect(api.getDisplayedRowCount()).toBe(expectedCounts[index]);
				const texts = cellTexts(host);
				expect(texts.join(' ')).toContain('Brand X');
				expect(host.querySelectorAll('.ag-row[row-id]').length).toBe(expectedCounts[index]);
			} finally {
				api.destroy();
			}
		});
	}

	it('walks the statuses from Building to Earned to Out of true', async () => {
		for (const [step, status] of [
			[2, 'Building'],
			[4, 'Earned'],
			[9, 'Out of true']
		] as const) {
			const { api, host } = await mountAt(step);
			try {
				const sizing = host.querySelector('.ag-row[row-id="blf_8f2c41"]');
				expect(sizing?.textContent).toContain(status);
			} finally {
				api.destroy();
			}
		}
	});

	it('shows seeded marks on seeded rows only', async () => {
		const { api, host } = await mountAt(9);
		try {
			const marks = [...host.querySelectorAll('.ledger-seeded')].map((mark) =>
				mark.textContent?.trim()
			);
			expect(marks).toEqual(['Seeded', 'Seeded']);
			expect(host.querySelector('.ag-row[row-id="blf_8f2c41"]')?.textContent).toContain('Seeded');
			expect(host.querySelector('.ag-row[row-id="blf_2b9d60"]')?.textContent).not.toContain(
				'Seeded'
			);
		} finally {
			api.destroy();
		}
	});

	it('explains Out of true where it first appears', async () => {
		const { api, host } = await mountAt(5);
		try {
			const pill = host.querySelector('.ag-row[row-id="blf_8f2c41"] .ledger-out-of-true');
			expect(pill?.getAttribute('title')).toBe(OUT_OF_TRUE_TOOLTIP);
			expect(host.querySelector('.ledger-correction')?.textContent).toContain(
				"I've lost weight, use M"
			);
		} finally {
			api.destroy();
		}
	});

	it('keeps every row across a theme switch in both directions', async () => {
		const { api } = await mountAt(9, { mode: 'dark' });
		try {
			expect(api.getDisplayedRowCount()).toBe(4);
			api.setGridOption('theme', ledgerGridTheme('light'));
			expect(api.getDisplayedRowCount()).toBe(4);
			api.setGridOption('theme', ledgerGridTheme('dark'));
			expect(api.getDisplayedRowCount()).toBe(4);
			const pills = [...(hosts[0] as HTMLElement).querySelectorAll('.ledger-pill')].map((pill) =>
				pill.textContent?.trim()
			);
			expect(pills.filter((pill) => pill === 'Claimed')).toHaveLength(3);
		} finally {
			api.destroy();
		}
	});

	it('stays usable at phone width', async () => {
		const { api, host } = await mountAt(9, { width: '360px' });
		try {
			expect(api.getDisplayedRowCount()).toBe(4);
			expect(host.querySelectorAll('.ag-row[row-id]').length).toBe(4);
			expect(host.querySelector('.ag-row[row-id="blf_8f2c41"]')?.textContent).toContain(
				'Out of true'
			);
		} finally {
			api.destroy();
		}
	});
});

describe('ledger grid keyboard', () => {
	it('reaches every row with arrow keys', async () => {
		const { api, host } = await mountAt(9);
		try {
			const first = host.querySelector('.ag-row[row-index="0"] .ag-cell') as HTMLElement | null;
			expect(first).not.toBeNull();
			first?.focus();
			expect(focusedRowId()).toBe('blf_8f2c41');
			const visited = [focusedRowId()];
			for (let move = 0; move < 3; move += 1) {
				pressArrowDown();
				await new Promise((resolve) => setTimeout(resolve, 50));
				visited.push(focusedRowId());
			}
			expect(visited).toEqual(['blf_8f2c41', 'blf_2b9d60', 'blf_1a5e09', 'blf_77c310']);
			pressArrowUp();
			await new Promise((resolve) => setTimeout(resolve, 50));
			expect(focusedRowId()).toBe('blf_1a5e09');
		} finally {
			api.destroy();
		}
	});

	it('gives every row a focusable cell', async () => {
		const { api, host } = await mountAt(4);
		try {
			for (const row of host.querySelectorAll('.ag-row[row-id]')) {
				const focusable = [...row.querySelectorAll('.ag-cell')].some(
					(cell) => cell.hasAttribute('tabindex')
				);
				expect(focusable).toBe(true);
			}
		} finally {
			api.destroy();
		}
	});
});

describe('ledger cell renderers', () => {
	it('treats each status distinctly', () => {
		expect(statusPillClass('Earned')).toContain('ledger-earned');
		expect(statusPillClass('Building')).toContain('ledger-building');
		expect(statusPillClass('Claimed')).toContain('ledger-claimed');
		expect(statusPillClass('Out of true')).toContain('ledger-out-of-true');
	});

	it('tooltips only the Out-of-true pill', () => {
		const outOfTrue = statusCellRenderer({
			value: 'Out of true',
			data: {
				correction: "I've lost weight, use M"
			} as LedgerGridRow
		} as Parameters<typeof statusCellRenderer>[0]);
		expect(outOfTrue).toContain(`title="${OUT_OF_TRUE_TOOLTIP}"`);
		expect(outOfTrue).toContain('ledger-correction');
		const earned = statusCellRenderer({ value: 'Earned' } as Parameters<
			typeof statusCellRenderer
		>[0]);
		expect(earned).not.toContain('title=');
	});

	it('marks weak-signal evidence and nothing else', () => {
		const weak = evidenceCellRenderer({
			value: 1,
			data: { weakSignal: true } as LedgerGridRow
		} as Parameters<typeof evidenceCellRenderer>[0]);
		expect(weak).toContain(WEAK_SIGNAL_LABEL);
		expect(weak).toContain(`title="${WEAK_SIGNAL_TOOLTIP}"`);
		const strong = evidenceCellRenderer({
			value: 3,
			data: { weakSignal: false } as LedgerGridRow
		} as Parameters<typeof evidenceCellRenderer>[0]);
		expect(strong).not.toContain(WEAK_SIGNAL_LABEL);
	});

	it('marks seeded rows and leaves live rows blank', () => {
		expect(
			seededCellRenderer({ value: true } as Parameters<typeof seededCellRenderer>[0])
		).toContain('Seeded');
		expect(seededCellRenderer({ value: false } as Parameters<typeof seededCellRenderer>[0])).toBe(
			''
		);
	});

	it('dates the last change with the full stamp on hover', () => {
		const rendered = lastChangeCellRenderer({
			value: '2026-11-05',
			data: { lastChangeFull: '2026-11-05T14:02:11Z' } as LedgerGridRow
		} as Parameters<typeof lastChangeCellRenderer>[0]);
		expect(rendered).toContain('2026-11-05');
		expect(rendered).toContain('title="2026-11-05T14:02:11Z"');
	});

	it('escapes belief text in the status renderer', () => {
		const rendered = statusCellRenderer({
			value: 'Out of true',
			data: { correction: '<use M & "now">' } as LedgerGridRow
		} as Parameters<typeof statusCellRenderer>[0]);
		expect(rendered).toContain('&lt;use M &amp; &quot;now&quot;&gt;');
	});
});
