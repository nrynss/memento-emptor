// The gate for contesting a belief: a shopper's edit goes through
// record_correction and nothing else, the edit wins the decision at once
// without rewriting the belief, and the contradicted row stays visible
// with its warning beside it.
import { describe, expect, it } from 'vitest';
import requestExample from '../../../../testdata/api/record_correction.request.example.json';
import responseExample from '../../../../testdata/api/record_correction.response.example.json';
import {
	createFixtureTransport,
	fixtureShopperId,
	LedgerApiError,
	LedgerClient,
	type LedgerFixtureStep
} from '$lib/api';
import { ledgerColumnDefs } from '$lib/ledger';
import { toLedgerGridRow } from '$lib/ledger/rows';
import { CorrectionClient } from './client';
import {
	contestCellRenderer,
	contestEditColumnDefs,
	CONTEST_ACTION_LABEL,
	CONTEST_BUTTON_ATTRIBUTE
} from './columns';
import { decodeRecordCorrectionResponse } from './decode';
import {
	createFixtureCorrectionTransport,
	fixtureContradictedBeliefId,
	fixtureCorrectionText
} from './fixture';
import { createFixtureContestState, correctionPreviewLine } from './state';
import type { CorrectionTransport, RecordCorrectionResponse } from './types';

function clientAt(step: LedgerFixtureStep): LedgerClient {
	return new LedgerClient(createFixtureTransport(step));
}

function recordingFixtureTransport(calls: { tool: string; request: unknown }[]): CorrectionTransport {
	const inner = createFixtureCorrectionTransport();
	return {
		call: (tool: 'record_correction', request: Record<string, unknown>) => {
			calls.push({ tool, request });
			return inner.call(tool, request);
		}
	};
}

describe('correction path shape', () => {
	it('decodes the frozen response example as a typed correction', () => {
		const decoded = decodeRecordCorrectionResponse(responseExample);
		expect(decoded.belief_id).toBe(fixtureContradictedBeliefId);
		expect(decoded.status).toBe('Out of true');
		expect(decoded.standing).toBe('Venerable');
		expect(decoded.contradicted).toBe(true);
		expect(decoded.correction.takes_precedence).toBe(true);
	});

	it('replays the canonical correction exactly as frozen', async () => {
		const client = new CorrectionClient(createFixtureCorrectionTransport());
		const response = await client.recordCorrection({
			...(requestExample as {
				shopper_id: string;
				text: string;
				contradicts_belief_id: string;
				trace_id: string;
			})
		});
		expect(response).toEqual(responseExample as unknown as RecordCorrectionResponse);
	});

	it('fails loudly on unknown shoppers, beliefs and blank edits', async () => {
		const client = new CorrectionClient(createFixtureCorrectionTransport());
		await expect(
			client.recordCorrection({
				shopper_id: 'shopper_99',
				text: fixtureCorrectionText,
				contradicts_belief_id: fixtureContradictedBeliefId
			})
		).rejects.toBeInstanceOf(LedgerApiError);
		await expect(
			client.recordCorrection({
				shopper_id: fixtureShopperId,
				text: fixtureCorrectionText,
				contradicts_belief_id: 'blf_missing'
			})
		).rejects.toMatchObject({ code: 'unknown-request' });
		await expect(
			client.recordCorrection({
				shopper_id: fixtureShopperId,
				text: '   ',
				contradicts_belief_id: fixtureContradictedBeliefId
			})
		).rejects.toMatchObject({ code: 'unknown-request' });
	});
});

describe('fixture edit through the correction path', () => {
	it('moves the Earned belief to Out of true and leaves it visible', async () => {
		const beliefs = await clientAt(4).listBeliefs({
			shopper_id: fixtureShopperId,
			include_seeded: true
		});
		const before = beliefs.beliefs.find(
			(row) => row.belief.belief_id === fixtureContradictedBeliefId
		);
		expect(before?.belief.status).toBe('Earned');

		const calls: { tool: string; request: unknown }[] = [];
		const { ContestState } = await import('./state');
		const { CorrectionClient: SpyClient } = await import('./client');
		const contest = new ContestState(
			new SpyClient(recordingFixtureTransport(calls)),
			fixtureShopperId,
			beliefs.beliefs
		);
		const response = await contest.submitCorrection(
			fixtureContradictedBeliefId,
			fixtureCorrectionText
		);

		expect(calls).toHaveLength(1);
		expect(calls[0].tool).toBe('record_correction');
		expect(calls[0].request).toMatchObject({
			shopper_id: fixtureShopperId,
			text: fixtureCorrectionText,
			contradicts_belief_id: fixtureContradictedBeliefId
		});
		expect(response).toEqual(responseExample as unknown as RecordCorrectionResponse);

		const after = contest.beliefs.find(
			(row) => row.belief.belief_id === fixtureContradictedBeliefId
		);
		expect(after).toBeDefined();
		expect(after?.belief.status).toBe('Out of true');
		expect(after?.belief.standing).toBe('Venerable');
		expect(after?.belief.contradicted).toBe(true);
		expect(after?.belief.contradicted_by?.text).toBe(fixtureCorrectionText);
		expect(after?.belief.evidence).toHaveLength(before?.belief.evidence.length ?? 0);
		expect(after?.evidence_count).toBe(before?.evidence_count);
		expect(contest.beliefs).toHaveLength(beliefs.beliefs.length);
	});

	it('lets the edit win the decision at once', async () => {
		const beliefs = await clientAt(4).listBeliefs({
			shopper_id: fixtureShopperId,
			include_seeded: true
		});
		const contest = createFixtureContestState(fixtureShopperId, beliefs.beliefs);
		expect(contest.decisionFor(fixtureContradictedBeliefId)).toBe('runs small, take size L');
		await contest.submitCorrection(fixtureContradictedBeliefId, fixtureCorrectionText);
		expect(contest.decisionFor(fixtureContradictedBeliefId)).toBe(fixtureCorrectionText);
		expect(contest.status).toBe('ready');
	});

	it('accepts fresh wording through the same path', async () => {
		const beliefs = await clientAt(4).listBeliefs({
			shopper_id: fixtureShopperId,
			include_seeded: true
		});
		const calls: { tool: string; request: unknown }[] = [];
		const { ContestState } = await import('./state');
		const contest = new ContestState(
			new CorrectionClient(recordingFixtureTransport(calls)),
			fixtureShopperId,
			beliefs.beliefs
		);
		await contest.submitCorrection(fixtureContradictedBeliefId, 'Actually M fits me now');
		expect(calls).toHaveLength(1);
		expect(calls[0].tool).toBe('record_correction');
		expect(contest.decisionFor(fixtureContradictedBeliefId)).toBe('Actually M fits me now');
		const after = contest.beliefs.find(
			(row) => row.belief.belief_id === fixtureContradictedBeliefId
		);
		expect(after?.belief.status).toBe('Out of true');
		expect(after?.belief.standing).toBe('Venerable');
	});

	it('keeps the warning beside the kept belief in grid shape', async () => {
		const beliefs = await clientAt(4).listBeliefs({
			shopper_id: fixtureShopperId,
			include_seeded: true
		});
		const contest = createFixtureContestState(fixtureShopperId, beliefs.beliefs);
		await contest.submitCorrection(fixtureContradictedBeliefId, fixtureCorrectionText);
		const after = contest.beliefs.find(
			(row) => row.belief.belief_id === fixtureContradictedBeliefId
		);
		const gridRow = toLedgerGridRow(after!);
		expect(gridRow.status).toBe('Out of true');
		expect(gridRow.correction).toBe(fixtureCorrectionText);
		expect(gridRow.seeded).toBe(true);
	});
});

describe('the edit calls the correction path and nothing else', () => {
	const ownModules = import.meta.glob('./*.ts', {
		query: '?raw',
		import: 'default',
		eager: true
	}) as Record<string, string>;

	function ownSources(): { name: string; text: string }[] {
		return Object.entries(ownModules)
			.filter(([path]) => !path.endsWith('.test.ts'))
			.map(([path, text]) => ({ name: path.split('/').pop() ?? path, text }));
	}

	it('sends through one transport call to the correction tool', () => {
		const sources = ownSources();
		const calls = sources.flatMap(({ name, text }) =>
			[...text.matchAll(/\.call\(/g)].map(() => name)
		);
		expect(calls).toEqual(['client.ts']);
		const client = sources.find((source) => source.name === 'client.ts')!.text;
		expect(client).toContain("'record_correction'");
	});

	it('never reads the ledger or rewrites a belief directly', () => {
		for (const { name, text } of ownSources()) {
			expect(
				`${name}: ${text.includes('ledger_beliefs') || text.includes('ledger_evidence') || text.includes('pending_questions')}`
			).toBe(`${name}: false`);
			expect(`${name}: ${/\.status\s*=/.test(text)}`).toBe(`${name}: false`);
			expect(`${name}: ${/\.contradicted\s*=/.test(text)}`).toBe(`${name}: false`);
			if (name !== 'fixture.ts' && name !== 'decode.ts') {
				expect(`${name}: ${text.includes("'Out of true'")}`).toBe(`${name}: false`);
			}
		}
	});
});

describe('contest editor simplicity', () => {
	const ownSvelte = import.meta.glob('./*.svelte', {
		query: '?raw',
		import: 'default',
		eager: true
	}) as Record<string, string>;

	function editorSource(): string {
		const entry = Object.entries(ownSvelte).find(([path]) => path.endsWith('ContestEditor.svelte'));
		expect(entry).toBeDefined();
		return entry![1];
	}

	it('carries everything a judge needs on the form itself', () => {
		expect(correctionPreviewLine).toBe(
			'Your correction now decides — the old belief stays below for the record.'
		);
		const editor = editorSource();
		expect(editor).toContain('Contest this belief');
		expect(editor).toContain('correctionPreviewLine');
		expect(editor).toContain('aria-describedby="contest-preview"');
		expect(editor).toContain('role="alert"');
		expect(editor).toContain('role="status"');
		expect(editor).toContain('confirmationLineFor(confirmed)');
		expect(editor).toContain('Send correction');
	});

	it('holds phone width on theme variables only', () => {
		const editor = editorSource();
		expect(editor).toContain('max-width: 100%');
		expect(editor).toContain('box-sizing: border-box');
		expect(editor).toContain('var(--surface)');
		expect(editor).not.toMatch(/#[0-9a-fA-F]{3,8}/);
	});

	it('confirms the result in the same plain words', async () => {
		const beliefs = await clientAt(4).listBeliefs({
			shopper_id: fixtureShopperId,
			include_seeded: true
		});
		const contest = createFixtureContestState(fixtureShopperId, beliefs.beliefs);
		expect(contest.confirmationLine()).toBeNull();
		await contest.submitCorrection(fixtureContradictedBeliefId, fixtureCorrectionText);
		expect(contest.confirmationLine()).toBe(
			`Your correction now decides: “${fixtureCorrectionText}”. The old belief stays below for the record.`
		);
	});
});

describe('contest column defs beside the frozen grid', () => {
	it('leaves the read-only grid columns alone', () => {
		expect(ledgerColumnDefs()).toHaveLength(7);
		const contest = contestEditColumnDefs();
		expect(contest).toHaveLength(1);
		expect(contest[0].colId).toBe('contest');
		expect(contest[0].sortable).toBe(false);
	});

	it('renders one Contest button carrying its belief id, escaped', () => {
		const rendered = contestCellRenderer({
			data: { id: 'blf_8f2c41' }
		} as Parameters<typeof contestCellRenderer>[0]);
		expect(rendered).toContain(CONTEST_ACTION_LABEL);
		expect(rendered).toContain(`${CONTEST_BUTTON_ATTRIBUTE}="blf_8f2c41"`);
		const hostile = contestCellRenderer({
			data: { id: '"><script>alert(1)</script>' }
		} as Parameters<typeof contestCellRenderer>[0]);
		expect(hostile).not.toContain('"><script>');
		expect(hostile).toContain('&quot;&gt;&lt;script&gt;');
	});
});
