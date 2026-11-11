<script lang="ts">
	// The evidence ledger page. Beliefs render in AG Grid; the grid component
	// loads lazily so the landing page never pays for the grid bundle. The step
	// picker replays the demo story from fixtures, one step at a time. Every row
	// links to its why view and opens the contest editor, and a recorded
	// correction updates the rows from the layer's own answer, so the
	// contradicted belief stays on screen beside the shopper's words.

	import { type Component } from 'svelte';
	import {
		createFixtureState,
		finalFixtureStep,
		fixtureShopperId,
		fixtureStepLabels,
		type LedgerBeliefRow,
		type LedgerFixtureStep,
		type LedgerLoadStatus,
		type QuestionView
	} from '$lib/api';
	import {
		ContestEditor,
		contestEditColumnDefs,
		createFixtureContestState,
		type ContestState
	} from '$lib/contest';
	import { explainColumnDefs } from '$lib/ledger';
	import { toLedgerGridRows, type LedgerGridRow } from '$lib/ledger/rows';

	const stepOptions: { step: LedgerFixtureStep; label: string }[] = (
		Object.entries(fixtureStepLabels) as [string, string][]
	).map(([raw, label]) => ({ step: Number(raw) as LedgerFixtureStep, label }));

	const extraColumns = [...explainColumnDefs(), ...contestEditColumnDefs()];

	let step = $state<LedgerFixtureStep>(finalFixtureStep);
	let ledgerState = $derived(createFixtureState(fixtureShopperId, step));

	let status = $state<LedgerLoadStatus>('idle');
	let loadError = $state<string | null>(null);
	let pending = $state<QuestionView[]>([]);
	let rows = $state<LedgerGridRow[]>([]);
	let contest = $state<ContestState | null>(null);
	let contendingId = $state<string | null>(null);
	let contendingValue = $state('');

	// Load the chosen step. The rows land in local state, so the grid and the
	// editor read the same answer the layer gave without waiting on an outside
	// notification.
	$effect(() => {
		const current = ledgerState;
		let live = true;
		status = 'loading';
		loadError = null;
		void (async () => {
			await current.refresh();
			if (!live) return;
			status = current.status;
			loadError = current.error;
			pending = current.pending;
			if (current.status === 'ready') {
				loadRows(current.beliefs);
			}
		})();
		return () => {
			live = false;
		};
	});

	function loadRows(beliefs: LedgerBeliefRow[]): void {
		contest = createFixtureContestState(fixtureShopperId, beliefs);
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

<div class="ledger-controls">
	<label for="ledger-step">Story step</label>
	<select id="ledger-step" value={step} onchange={pickStep}>
		{#each stepOptions as option (option.step)}
			<option value={option.step}>{option.step}: {option.label}</option>
		{/each}
	</select>
</div>

{#if status === 'error'}
	<p role="alert" class="ledger-error">
		The ledger failed to load: {loadError ?? 'unknown error'}
	</p>
{:else if status !== 'ready' || LedgerGrid === undefined}
	<p class="ledger-loading">Loading the ledger…</p>
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

	.ledger-error {
		color: var(--stop);
	}
</style>
