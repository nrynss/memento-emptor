<script lang="ts">
	// The evidence ledger page. Beliefs render in AG Grid; the grid component
	// loads lazily so the landing page never pays for the grid bundle. A link
	// carrying the shopper's token (?t=<token>) reads the live layer through
	// the edge and says so; a page without one replays the demo story from
	// fixtures and says that instead, so a replay is never shown as live data.
	// Every row links to its why view and opens the contest editor, and a
	// recorded correction updates the rows from the layer's own answer, so the
	// contradicted belief stays on screen beside the shopper's words.

	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import { type Component } from 'svelte';
	import {
		createFixtureState,
		createLiveState,
		createLiveTransport,
		finalFixtureStep,
		fixtureShopperId,
		fixtureStepLabels,
		liveLinkFrom,
		type LedgerBeliefRow,
		type LedgerFixtureStep,
		type LedgerLoadStatus,
		type LedgerState,
		type LiveLink,
		type QuestionView
	} from '$lib/api';
	import {
		ContestEditor,
		contestEditColumnDefs,
		ContestState,
		CorrectionClient,
		createFixtureContestState
	} from '$lib/contest';
	import { escapeLedgerHtml, explainColumnDefs } from '$lib/ledger';
	import { toLedgerGridRows, type LedgerGridRow } from '$lib/ledger/rows';
	import type { ColDef, ICellRendererParams } from 'ag-grid-community';

	const stepOptions: { step: LedgerFixtureStep; label: string }[] = (
		Object.entries(fixtureStepLabels) as [string, string][]
	).map(([raw, label]) => ({ step: Number(raw) as LedgerFixtureStep, label }));

	/**
	 * The why link for one belief. A live page carries the shopper link with
	 * it, so the why view it opens reads the same shopper's live layer instead
	 * of replaying the fixture story.
	 */
	function explainHref(link: LiveLink | null, beliefId: string): string {
		if (link === null) {
			return `/belief?belief=${encodeURIComponent(beliefId)}`;
		}
		const token = encodeURIComponent(link.token);
		const shopper = encodeURIComponent(link.shopperId);
		return `/belief?belief=${encodeURIComponent(beliefId)}&t=${token}&s=${shopper}`;
	}

	/** The why column, carrying the live link when the page has one. */
	function explainColumnsFor(link: LiveLink | null): ColDef<LedgerGridRow>[] {
		if (link === null) {
			return explainColumnDefs();
		}
		return explainColumnDefs().map((def) => ({
			...def,
			cellRenderer: (params: ICellRendererParams<LedgerGridRow>) =>
				`<a class="ledger-explain" href="${escapeLedgerHtml(
					explainHref(link, params.data?.id ?? '')
				)}">Why</a>`
		}));
	}

	// The link decides the source. A token means the live layer through the
	// edge; no token means the labelled fixture replay. The browser guard
	// keeps the request-bound URL off the server, where the page is
	// prerendered without a query.
	const liveLink = $derived(browser ? liveLinkFrom(page.url) : null);
	const extraColumns = $derived([...explainColumnsFor(liveLink), ...contestEditColumnDefs()]);

	let step = $state<LedgerFixtureStep>(finalFixtureStep);
	const ledgerState = $derived(
		liveLink === null ? createFixtureState(fixtureShopperId, step) : createLiveState(liveLink)
	);

	let status = $state<LedgerLoadStatus>('idle');
	let loadError = $state<string | null>(null);
	let pending = $state<QuestionView[]>([]);
	let rows = $state<LedgerGridRow[]>([]);
	let contest = $state<ContestState | null>(null);
	let contendingId = $state<string | null>(null);
	let contendingValue = $state('');

	// One load at a time: a later load (a step change, or the reader asking
	// again) makes an earlier answer stale, so it is dropped instead of
	// landing on the newer view.
	let generation = 0;

	async function load(current: LedgerState): Promise<void> {
		const mine = ++generation;
		status = 'loading';
		loadError = null;
		await current.refresh();
		if (mine !== generation) return;
		status = current.status;
		loadError = current.error;
		pending = current.pending;
		if (current.status === 'ready') {
			loadRows(current.beliefs);
		}
	}

	// Load the chosen source. The rows land in local state, so the grid and
	// the editor read the same answer the layer gave without waiting on an
	// outside notification.
	$effect(() => {
		void load(ledgerState);
		return () => {
			generation += 1;
		};
	});

	function loadRows(beliefs: LedgerBeliefRow[]): void {
		const link = liveLink;
		const transport = link === null ? null : createLiveTransport(link);
		contest =
			link !== null && transport !== null
				? new ContestState(new CorrectionClient(transport), link.shopperId, beliefs)
				: createFixtureContestState(fixtureShopperId, beliefs);
		rows = toLedgerGridRows(beliefs);
		contendingId = null;
		contendingValue = '';
	}

	let LedgerGrid = $state<Component<{
		rows: LedgerGridRow[];
		extraColumns: typeof extraColumns;
		onContest: (beliefId: string) => void;
	}> | undefined>(undefined);

	$effect(() => {
		let live = true;
		void import('$lib/ledger/LedgerGrid.svelte').then((module) => {
			if (live) {
				LedgerGrid = module.default;
			}
		});
		return () => {
			live = false;
		};
	});

	function pickStep(event: Event): void {
		step = Number((event.currentTarget as HTMLSelectElement).value) as LedgerFixtureStep;
	}

	function readAgain(): void {
		void load(ledgerState);
	}

	function openContest(beliefId: string): void {
		const row = contest?.beliefs.find((candidate) => candidate.belief.belief_id === beliefId);
		if (row === undefined) return;
		contendingId = beliefId;
		contendingValue = row.belief.value;
	}

	// The layer's answer landed: read the state's rows back so the grid shows
	// the contradicted belief with its correction beside it.
	function refreshRows(): void {
		if (contest !== null) {
			rows = toLedgerGridRows(contest.beliefs);
		}
	}

	const pendingLine = $derived(
		pending.length === 1
			? '1 refund still missing its reason. It counts toward nothing until the shopper answers.'
			: pending.length === 0
				? null
				: `${String(pending.length)} refunds still missing their reasons. They count toward nothing until the shopper answers.`
	);
</script>

<svelte:head>
	<title>Evidence ledger · Memento Emptor</title>
	<meta
		name="description"
		content="The beliefs the memory layer holds for this shopper, the evidence behind each, and the seeded rows marked."
	/>
</svelte:head>

<h1>Evidence ledger</h1>
<p class="lede">
	What the layer believes for this shopper, and what earned it. Seeded rows were
	generated, not lived. A kept item with no feedback is a weak inferred signal,
	never confirmed satisfaction. Open a belief's why view for its evidence, or
	contest the belief in your own words.
</p>

{#if liveLink === null}
	<p class="ledger-replay" data-ledger-source="replay">
		Fixture replay: the demo story at a chosen step, not a live ledger. Open this page
		with a shopper link to read the live layer.
	</p>
	<div class="ledger-controls">
		<label for="ledger-step">Story step</label>
		<select id="ledger-step" value={step} onchange={pickStep}>
			{#each stepOptions as option (option.step)}
				<option value={option.step}>{option.step}: {option.label}</option>
			{/each}
		</select>
	</div>
{:else if status !== 'error'}
	<div class="ledger-controls" data-ledger-source="live">
		<p class="ledger-live">
			{#if status === 'ready'}
				Live: the layer's own rows for shopper {liveLink.shopperId}.
			{:else}
				Reading the live ledger for shopper {liveLink.shopperId}…
			{/if}
		</p>
		<button type="button" onclick={readAgain} disabled={status === 'loading'}>Read again</button>
	</div>
{/if}

{#if status === 'error'}
	<p role="alert" class="ledger-error">
		{#if liveLink !== null}
			The live ledger read failed: {loadError ?? 'the layer gave no reason'}. Nothing on
			this page is live data.
			<button type="button" onclick={readAgain}>Read again</button>
		{:else}
			The fixture replay failed to load: {loadError ?? 'unknown error'}
		{/if}
	</p>
{:else if status !== 'ready' || LedgerGrid === undefined}
	<p class="ledger-loading">Loading the ledger…</p>
{:else if rows.length === 0}
	<p class="ledger-empty">The layer holds no beliefs for this shopper yet.</p>
{:else}
	<LedgerGrid {rows} {extraColumns} onContest={openContest} />
{/if}

{#if status === 'ready' && contest !== null && contendingId !== null}
	<section class="ledger-contest" aria-label="Contest this belief">
		<h2>Contest this belief</h2>
		{#key contendingId}
			<ContestEditor
				contest={contest}
				beliefId={contendingId}
				currentValue={contendingValue}
				formId={`contest-${contendingId}`}
				onRecorded={refreshRows}
			/>
		{/key}
	</section>
{/if}

{#if status === 'ready' && pendingLine !== null}
	<p class="ledger-pending">{pendingLine}</p>
{/if}

<style>
	.ledger-controls {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.75rem;
		margin: 0 0 1.5rem;
	}

	.ledger-controls label {
		font-weight: 600;
	}

	.ledger-controls select {
		font: inherit;
		color: inherit;
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: var(--radius-control);
		padding: 0.35rem 0.7rem;
		max-width: 100%;
	}

	.ledger-controls button {
		font: inherit;
		color: inherit;
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: var(--radius-control);
		padding: 0.35rem 0.9rem;
	}

	.ledger-controls button:disabled {
		color: var(--dim);
	}

	.ledger-replay {
		border: 1px dashed var(--dim);
		border-radius: var(--radius-panel);
		padding: 0.6rem 0.9rem;
		color: var(--dim);
		margin: 0 0 1rem;
	}

	.ledger-live {
		margin: 0;
		font-weight: 600;
	}

	.ledger-contest {
		margin: 1.5rem 0 0;
		padding: 1rem;
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		background: var(--surface);
	}

	.ledger-contest h2 {
		margin: 0 0 0.75rem;
		font-size: 1.1rem;
	}

	.ledger-loading,
	.ledger-pending {
		color: var(--dim);
	}

	.ledger-empty {
		border: 1px dashed var(--dim);
		border-radius: var(--radius-panel);
		padding: 1rem 1.25rem;
		color: var(--dim);
	}

	.ledger-error {
		color: var(--stop);
	}

	.ledger-error button {
		font: inherit;
		color: inherit;
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: var(--radius-control);
		padding: 0.35rem 0.9rem;
		margin-left: 0.5rem;
	}
</style>
