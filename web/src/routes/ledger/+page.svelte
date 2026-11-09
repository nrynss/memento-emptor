// The evidence ledger page. Beliefs render in AG Grid; the grid component
// loads lazily so the landing page never pays for the grid bundle. The step
// picker replays the demo story from fixtures, one step at a time.

<script lang="ts">
	import { untrack, type Component } from 'svelte';
	import { reactive } from '@nrynss/chaaya/svelte';
	import {
		createFixtureState,
		finalFixtureStep,
		fixtureShopperId,
		fixtureStepLabels,
		type LedgerFixtureStep
	} from '$lib/api';
	import { toLedgerGridRows, type LedgerGridRow } from '$lib/ledger/rows';

	const stepOptions: { step: LedgerFixtureStep; label: string }[] = (
		Object.entries(fixtureStepLabels) as [string, string][]
	).map(([raw, label]) => ({ step: Number(raw) as LedgerFixtureStep, label }));

	let step = $state<LedgerFixtureStep>(finalFixtureStep);
	let ledgerState = $derived(createFixtureState(fixtureShopperId, step));
	let view = $derived(reactive(ledgerState));

	// Refresh when the step changes. The refresh call runs untracked: every
	// property read on the reactive view subscribes, and subscribing this
	// effect to the state's own notifications would re-run the refresh on
	// every notify. The template below reads the view and re-renders it.
	$effect(() => {
		const current = ledgerState;
		untrack(() => {
			void reactive(current).refresh();
		});
	});

	let rows = $derived(view.status === 'ready' ? toLedgerGridRows(view.beliefs) : []);

	let LedgerGrid = $state<Component<{ rows: LedgerGridRow[] }> | undefined>(undefined);

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

	const pendingLine = $derived(
		view.pending.length === 1
			? '1 refund still missing its reason. It counts toward nothing until the shopper answers.'
			: view.pending.length === 0
				? null
				: `${String(view.pending.length)} refunds still missing their reasons. They count toward nothing until the shopper answers.`
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
	never confirmed satisfaction.
</p>

<div class="ledger-controls">
	<label for="ledger-step">Story step</label>
	<select id="ledger-step" value={step} onchange={pickStep}>
		{#each stepOptions as option (option.step)}
			<option value={option.step}>{option.step}: {option.label}</option>
		{/each}
	</select>
</div>

{#if view.status === 'error'}
	<p role="alert" class="ledger-error">
		The ledger failed to load: {view.error ?? 'unknown error'}
	</p>
{:else if view.status !== 'ready' || LedgerGrid === undefined}
	<p class="ledger-loading">Loading the ledger…</p>
{:else if LedgerGrid !== undefined}
	<LedgerGrid rows={rows} />
{/if}

{#if view.status === 'ready' && pendingLine !== null}
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

	.ledger-loading,
	.ledger-pending {
		color: var(--dim);
	}

	.ledger-error {
		color: var(--stop);
	}
</style>
