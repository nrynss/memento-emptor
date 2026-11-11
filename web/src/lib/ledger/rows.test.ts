// The gate for the ledger rows: the fixture story maps to the grid shape
// at every step, statuses and marks render from the mapped rows, and a
// kept-without-feedback observation maps to the weak-signal label.

import { describe, expect, it } from 'vitest';
import {
	createFixtureTransport,
	fixtureShopperId,
	LedgerClient,
	type BeliefView,
	type LedgerBeliefRow,
	type LedgerFixtureStep
} from '$lib/api';
import {
	escapeLedgerHtml,
	formatLedgerDate,
	hasWeakSignal,
	kindLabelFor,
	OUT_OF_TRUE_TOOLTIP,
	toLedgerGridRows,
	WEAK_SIGNAL_LABEL
} from './rows';

function clientAt(step: LedgerFixtureStep): LedgerClient {
	return new LedgerClient(createFixtureTransport(step));
}

async function gridRowsAt(step: LedgerFixtureStep) {
	const client = clientAt(step);
	const beliefs = await client.listBeliefs({ shopper_id: fixtureShopperId, include_seeded: true });
	return toLedgerGridRows(beliefs.beliefs);
}

function keptBeliefRow(): LedgerBeliefRow {
	const belief: BeliefView = {
		belief_id: 'blf_kept',
		kind: 'fit',
		subject: { kind: 'brand', id: 'brand-x-001', name: 'Brand X' },
		value: 'true to size for this shopper',
		status: 'Claimed',
		standing: 'None',
		contradicted: false,
		contradicted_by: null,
		evidence: [
			{
				evidence_id: 'ev_kept',
				outcome: { kind: 'kept', status: 'COMPLETED', amount_minor: 19500, currency: 'USD' },
				item: {
					brand_id: 'brand-x-001',
					brand_name: 'Brand X',
					line_name: 'oxford shirt',
					sku: 'PSW8GeK',
					size: 'M',
					category: 'shirts'
				},
				reason: null,
				counts_toward: null,
				write_op: 'observe',
				link: {
					order_id: 'ORD-7XK-07',
					capture_id: 'CAP-7XK-07',
					custom_id: 'memento-shopper01-07',
					event_id: 'WH-2EV-07'
				},
				seeded: false,
				occurred_at: '2026-11-09T10:00:00Z'
			}
		],
		precedence: null,
		seeded: false,
		updated_at: '2026-11-09T10:00:00Z'
	};
	return { belief, evidence_count: 1, last_change_at: '2026-11-09T10:00:00Z' };
}

describe('ledger grid rows', () => {
	it('maps the final step with kinds, subjects, values and counts', async () => {
		const rows = await gridRowsAt(9);
		expect(rows.map((row) => row.id)).toEqual(['blf_8f2c41', 'blf_2b9d60', 'blf_1a5e09', 'blf_77c310']);
		expect(rows.map((row) => row.kindLabel)).toEqual([
			'Fit',
			'Fit',
			'Seller reliability',
			'Fit'
		]);
		expect(rows.map((row) => row.subject)).toEqual(['Brand X', 'Brand X', 'Acme', 'Brand X']);
		expect(rows[0]?.value).toBe('runs small, take size L');
		expect(rows.map((row) => row.evidenceCount)).toEqual([3, 1, 1, 0]);
		expect(rows.map((row) => row.lastChange)).toEqual([
			'2026-11-05',
			'2026-11-08',
			'2026-11-06',
			'2026-10-14'
		]);
	});

	it('carries every status with its standing at each step', async () => {
		expect((await gridRowsAt(2)).find((row) => row.id === 'blf_8f2c41')?.status).toBe('Building');
		expect((await gridRowsAt(3)).find((row) => row.id === 'blf_8f2c41')?.status).toBe('Building');
		expect((await gridRowsAt(4)).find((row) => row.id === 'blf_8f2c41')?.status).toBe('Earned');
		for (const step of [5, 6, 7, 8, 9] as const) {
			const sizing = (await gridRowsAt(step)).find((row) => row.id === 'blf_8f2c41');
			expect(sizing?.status).toBe('Out of true');
			expect(sizing?.correction).toBe("I've lost weight, use M");
		}
		const early = await gridRowsAt(4);
		expect(early.find((row) => row.id === 'blf_8f2c41')?.correction).toBeNull();
	});

	it('grows the row list through the story', async () => {
		const counts: number[] = [];
		for (const step of [1, 2, 3, 4, 5, 6, 7, 8, 9] as const) {
			counts.push((await gridRowsAt(step)).length);
		}
		expect(counts).toEqual([1, 2, 2, 2, 2, 3, 3, 3, 4]);
	});

	it('marks the seeded rows and only those', async () => {
		const rows = await gridRowsAt(9);
		expect(rows.filter((row) => row.seeded).map((row) => row.id).sort()).toEqual([
			'blf_77c310',
			'blf_8f2c41'
		]);
	});

	it('carries no shopper identity or transaction link', async () => {
		const rows = await gridRowsAt(9);
		const serialised = JSON.stringify(rows).toLowerCase();
		for (const token of [
			'shopper_',
			'payer',
			'custom_id',
			'event_id',
			'order_id',
			'capture_id',
			'refund_id',
			'dispute_id'
		]) {
			expect(serialised).not.toContain(token);
		}
		expect(Object.keys(rows[0] ?? {}).sort()).toEqual(
			[
				'correction',
				'evidenceCount',
				'id',
				'kindLabel',
				'lastChange',
				'lastChangeFull',
				'seeded',
				'status',
				'subject',
				'value',
				'weakSignal'
			].sort()
		);
	});

	it('labels kept-without-feedback as a weak inferred signal', () => {
		expect(hasWeakSignal(keptBeliefRow().belief)).toBe(true);
		const [row] = toLedgerGridRows([keptBeliefRow()]);
		expect(row?.weakSignal).toBe(true);
		expect(WEAK_SIGNAL_LABEL).toBe('weak signal');
	});

	it('marks no fixture row weak: the story has no kept evidence', async () => {
		for (const step of [1, 2, 3, 4, 5, 6, 7, 8, 9] as const) {
			expect((await gridRowsAt(step)).every((row) => !row.weakSignal)).toBe(true);
		}
	});

	it('keeps the Out-of-true tooltip to one line on the correction', () => {
		expect(OUT_OF_TRUE_TOOLTIP).not.toContain('\n');
		expect(OUT_OF_TRUE_TOOLTIP.startsWith('Out of true:')).toBe(true);
	});

	it('formats dates without timezone drift', () => {
		expect(formatLedgerDate('2026-11-05T14:02:11Z')).toBe('2026-11-05');
	});

	it('labels kinds for the grid', () => {
		expect(kindLabelFor('fit')).toBe('Fit');
		expect(kindLabelFor('seller_reliability')).toBe('Seller reliability');
	});

	it('escapes markup for the HTML-string renderers', () => {
		expect(escapeLedgerHtml('<script>"&\'</script>')).toBe(
			'&lt;script&gt;&quot;&amp;&#39;&lt;/script&gt;'
		);
	});
});
