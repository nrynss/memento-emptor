// The why view page. One belief at a time, replayed from fixtures: a step
// picker walks the demo story, a belief picker walks the rows at that step,
// and the view shows what counts toward the belief, what was deliberately
// not counted and why, the correction beside it, and the trace placeholders.

<script lang="ts">
	import { resolve } from '$app/paths';
	import {
		createFixtureTransport,
		finalFixtureStep,
		fixtureShopperId,
		fixtureStepLabels,
		LedgerClient,
		type LedgerBeliefRow,
		type LedgerFixtureStep
	} from '$lib/api';
	import { toLedgerGridRow } from '$lib/ledger/rows';
	import { buildWhyView, tracesForBelief, WhyView, type WhyViewData } from '$lib/why';

	const stepOptions: { step: LedgerFixtureStep; label: string }[] = (
		Object.entries(fixtureStepLabels) as [string, string][]
	).map(([raw, label]) => ({ step: Number(raw) as LedgerFixtureStep, label }));

	const preferredBeliefId = 'blf_8f2c41';

	let step = $state<LedgerFixtureStep>(finalFixtureStep);
	let beliefId = $state<string | null>(null);
	let pickerRows = $state<LedgerBeliefRow[]>([]);
	let detail = $state<WhyViewData | null>(null);
	let loadError = $state<string | null>(null);
	let loading = $state(true);

	$effect(() => {
		const currentStep = step;
		const wanted = beliefId;
		let live = true;
		loading = true;
		loadError = null;
		void (async () => {
			try {
				const client = new LedgerClient(createFixtureTransport(currentStep));
				const listed = await client.listBeliefs({
					shopper_id: fixtureShopperId,
					include_seeded: true
				});
				if (!live) return;
				pickerRows = listed.beliefs;
				const available = listed.beliefs.map((row) => row.belief.belief_id);
				const chosen =
					wanted !== null && available.includes(wanted)
						? wanted
						: available.includes(preferredBeliefId)
							? preferredBeliefId
							: (available[0] ?? null);
				if (chosen === null) {
					detail = null;
					return;
				}
				if (chosen !== wanted) {
					beliefId = chosen;
				}
				const [one, evidence, pending] = await Promise.all([
					client.getBelief({ shopper_id: fixtureShopperId, belief_id: chosen }),
					client.listEvidence({ shopper_id: fixtureShopperId }),
					client.listPendingQuestions({ shopper_id: fixtureShopperId })
				]);
				if (!live) return;
				detail = buildWhyView(
					one.belief,
					one.corrections,
					one.seeded_notice,
					evidence.evidence,
					pending.questions,
					tracesForBelief(chosen)
				);
			} catch (error) {
				if (!live) return;
				detail = null;
				loadError = error instanceof Error ? error.message : 'The why view failed to load';
			} finally {
				if (live) loading = false;
			}
		})();
		return () => {
			live = false;
		};
	});

	function optionLabel(row: LedgerBeliefRow): string {
		const flat = toLedgerGridRow(row);
		return `${flat.kindLabel} · ${flat.subject} — ${flat.value}`;
	}

	function pickStep(event: Event): void {
		step = Number((event.currentTarget as HTMLSelectElement).value) as LedgerFixtureStep;
	}

	function pickBelief(event: Event): void {
		beliefId = (event.currentTarget as HTMLSelectElement).value;
	}
</script>

<svelte:head>
	<title>Why the layer believes it · Memento Emptor</title>
	<meta
		name="description"
		content="One belief with its evidence: what counts toward it, what was deliberately not counted and why, and the correction beside it."
	/>
</svelte:head>

<h1>Why the layer believes it</h1>
<p class="lede">
	One belief with its evidence: what counts toward it, what was deliberately not
	counted and why, and the correction beside it.
</p>
<p class="why-back"><a href={resolve('/ledger')}>Back to the ledger table</a></p>

<div class="why-controls">
	<label for="why-step">Story step</label>
	<select id="why-step" value={step} onchange={pickStep}>
		{#each stepOptions as option (option.step)}
			<option value={option.step}>{option.step}: {option.label}</option>
		{/each}
	</select>
	<label for="why-belief">Belief</label>
	<select id="why-belief" value={beliefId ?? ''} onchange={pickBelief}>
		{#each pickerRows as row (row.belief.belief_id)}
			<option value={row.belief.belief_id}>{optionLabel(row)}</option>
		{/each}
	</select>
</div>

{#if loadError !== null}
	<p role="alert" class="why-error">The why view failed to load: {loadError}</p>
{:else if loading || detail === null}
	<p class="why-loading">Loading the why view…</p>
{:else}
	<WhyView view={detail} />
{/if}

<style>
	.why-controls {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.75rem;
		margin: 0 0 2rem;
	}

	.why-controls label {
		font-weight: 600;
	}

	.why-controls select {
		font: inherit;
		color: inherit;
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: var(--radius-control);
		padding: 0.35rem 0.7rem;
		max-width: 100%;
	}

	.why-loading {
		color: var(--dim);
	}

	.why-back {
		margin: 0 0 1.5rem;
	}

	.why-error {
		color: var(--stop);
	}
</style>
