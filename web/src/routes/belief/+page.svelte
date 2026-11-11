<script lang="ts">
	// The why view page. One belief at a time, replayed from fixtures: a step
	// picker walks the demo story, a belief picker walks the rows at that step,
	// and the view shows what counts toward the belief, what was deliberately
	// not counted and why, the correction beside it, and the trace placeholders.
	// Below it, the pivot's reads for one shopper at a time: which precedence
	// step decided the recommendation with any overridden step beside the one
	// that won, the pooled belief as counts with its seeded split, a standing
	// correction with no belief, and the pooling opt-out with rows removed.

	import { browser } from '$app/environment';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		createFixtureTransport,
		finalFixtureStep,
		fixtureShopperId,
		fixtureStepLabels,
		LedgerClient,
		type LedgerBeliefRow,
		type LedgerFixtureStep
	} from '$lib/api';
	import { createFixtureContestState } from '$lib/contest';
	import type { ContestCorrectionView } from '$lib/contest';
	import ContestEditor from '$lib/contest/ContestEditor.svelte';
	import {
		createFixturePoolingState,
		createFixtureRecallTransport,
		DecisionLine,
		decodeRecallResponse,
		linkedBeliefId,
		pickBeliefId,
		PoolCounts,
		PoolingToggle,
		recallCaseLabels,
		recallCaseShopper,
		recallCases,
		StandingCorrection,
		type OverriddenStep,
		type RecallCase,
		type RecallResponse
	} from '$lib/ledger';
	import { toLedgerGridRow } from '$lib/ledger/rows';
	import { buildWhyView, tracesForBelief, WhyView, type WhyViewData } from '$lib/why';

	const stepOptions: { step: LedgerFixtureStep; label: string }[] = (
		Object.entries(fixtureStepLabels) as [string, string][]
	).map(([raw, label]) => ({ step: Number(raw) as LedgerFixtureStep, label }));

	const caseOptions: { recallCase: RecallCase; label: string }[] = recallCases.map(
		(recallCase) => ({ recallCase, label: recallCaseLabels[recallCase] })
	);

	const preferredBeliefId = 'blf_8f2c41';

	let step = $state<LedgerFixtureStep>(finalFixtureStep);
	let beliefId = $state<string | null>(null);
	// A link from the ledger names the belief to explain. The query wins on
	// arrival and the picker works from there. The browser guard keeps the
	// request-bound URL off the server, where the page is prerendered without
	// a query.
	const linkedId = $derived(browser ? linkedBeliefId(page.url) : null);

	$effect(() => {
		if (linkedId !== null) {
			beliefId = linkedId;
		}
	});

	let pickerRows = $state<LedgerBeliefRow[]>([]);
	let detail = $state<WhyViewData | null>(null);
	let loadError = $state<string | null>(null);
	let loading = $state(true);

	let recallCase = $state<RecallCase>('correction');
	let recall = $state<RecallResponse | null>(null);
	let recallError = $state<string | null>(null);
	let recallLoading = $state(true);

	const caseShopper = $derived(recallCaseShopper[recallCase]);
	const contestState = $derived(createFixtureContestState(caseShopper, []));
	const poolingState = $derived(createFixturePoolingState(caseShopper));
	let standingList = $state<ContestCorrectionView[]>([]);

	const overridden = $derived.by((): OverriddenStep[] => {
		const current = recall;
		if (current === null) return [];
		const seen: string[] = [];
		const out: OverriddenStep[] = [];
		for (const belief of current.beliefs) {
			const precedence = belief.precedence;
			if (precedence?.overridden === true && precedence.overridden_by !== null) {
				const key = `${precedence.step}>${precedence.overridden_by}`;
				if (!seen.includes(key)) {
					seen.push(key);
					out.push({ step: precedence.step, winner: precedence.overridden_by });
				}
			}
		}
		const poolPrecedence = current.pool?.precedence;
		if (poolPrecedence?.overridden === true && poolPrecedence.overridden_by !== null) {
			const key = `${poolPrecedence.step}>${poolPrecedence.overridden_by}`;
			if (!seen.includes(key)) {
				seen.push(key);
				out.push({ step: poolPrecedence.step, winner: poolPrecedence.overridden_by });
			}
		}
		return out;
	});

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
				const chosen = pickBeliefId(wanted, available, preferredBeliefId);
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

	$effect(() => {
		const current = recallCase;
		let live = true;
		recallLoading = true;
		recallError = null;
		void (async () => {
			try {
				const transport = createFixtureRecallTransport(current);
				const raw = await transport.call('recall_for_purchase', {
					shopper_id: recallCaseShopper[current]
				});
				if (!live) return;
				recall = decodeRecallResponse(raw);
			} catch (error) {
				if (!live) return;
				recall = null;
				recallError = error instanceof Error ? error.message : 'The recommendation failed to load';
			} finally {
				if (live) recallLoading = false;
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

	function pickCase(event: Event): void {
		recallCase = (event.currentTarget as HTMLSelectElement).value as RecallCase;
		standingList = [];
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

<h2>What the layer recommends</h2>
<p class="lede">
	Which precedence step decided, with any overridden step beside the one that won.
	One shopper at a time: each case replays a single shopper's rows, and any other
	shopper fails instead of mixing in.
</p>

<div class="why-controls">
	<label for="why-case">Recommendation case</label>
	<select id="why-case" value={recallCase} onchange={pickCase}>
		{#each caseOptions as option (option.recallCase)}
			<option value={option.recallCase}>{option.label} · {recallCaseShopper[option.recallCase]}</option>
		{/each}
	</select>
</div>

{#if recallError !== null}
	<p role="alert" class="why-error">The recommendation failed to load: {recallError}</p>
{:else if recallLoading || recall === null}
	<p class="why-loading">Loading the recommendation…</p>
{:else}
	<DecisionLine text={recall.guidance.text} basis={recall.guidance.basis} overridden={overridden} />
	{#if recall.pool !== null && recall.pool !== undefined}
		<PoolCounts pool={recall.pool} />
	{:else}
		<p class="why-loading">No pooled belief applies here.</p>
	{/if}
{/if}

<h2>Your standing correction</h2>
<p class="lede">
	No belief yet: record a correction that stands on its own and decides at once.
</p>
{#key caseShopper}
	<ContestEditor
		contest={contestState}
		onRecorded={(response) => {
			if (response.belief_id === undefined) {
				standingList = [response.correction, ...standingList];
			}
		}}
	/>
{/key}
{#each standingList as correction (correction.correction_id)}
	<StandingCorrection text={correction.text} recordedAt={correction.recorded_at} />
{/each}

<h2>Pooling choice</h2>
<p class="lede">
	Pooling is opt-out. Opting out stops new fit rows and removes the ones already
	written; the layer says how many rows went.
</p>
<PoolingToggle pooling={poolingState} />

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
