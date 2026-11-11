<script lang="ts">
	// The why view page. One belief at a time: the view shows what counts
	// toward the belief, what was deliberately not counted and why, the
	// correction beside it, and the trace placeholders. Below it, the pivot's
	// reads for one shopper at a time: which precedence step decided the
	// recommendation with any overridden step beside the one that won, the
	// pooled belief as counts with its seeded split, a standing correction
	// with no belief, and the pooling opt-out with rows removed.
	//
	// A link carrying the shopper's token (?t=<token>) reads all of it from
	// the live layer through the edge; a page without one replays the demo
	// story from fixtures and says so. A failed live read is labelled as a
	// failure of that read and leaves no earlier answer on screen; the live
	// source label appears only once a live read has answered, so nothing
	// here can be mistaken for live data.
	//
	// The live recommendation reads the product line the page can honestly
	// name: the belief's subject's brand and the one category its evidence
	// rows agree on, never a fit handle. The layer resolves the handle from
	// its own population, and a pool it could not read is said out loud
	// beside the recommendation rather than reported as nothing applying.

	import { browser } from '$app/environment';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		createFixtureTransport,
		createLiveTransport,
		finalFixtureStep,
		fixtureShopperId,
		fixtureStepLabels,
		LedgerClient,
		liveLinkFrom,
		type LedgerBeliefRow,
		type LedgerFixtureStep
	} from '$lib/api';
	import {
		ContestState,
		CorrectionClient,
		createFixtureContestState
	} from '$lib/contest';
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
		PoolingClient,
		PoolingState,
		PoolingToggle,
		recallCaseLabels,
		recallCaseShopper,
		recallCases,
		StandingCorrection,
		type OverriddenStep,
		type RecallCase,
		type RecallResponse,
		type RecallTransport
	} from '$lib/ledger';
	import { toLedgerGridRow } from '$lib/ledger/rows';
	import { buildWhyView, tracesForBelief, WhyView, type WhyViewData } from '$lib/why';
	import { poolNoteFor, recallLineFor } from './pool-line';

	const stepOptions: { step: LedgerFixtureStep; label: string }[] = (
		Object.entries(fixtureStepLabels) as [string, string][]
	).map(([raw, label]) => ({ step: Number(raw) as LedgerFixtureStep, label }));

	const caseOptions: { recallCase: RecallCase; label: string }[] = recallCases.map(
		(recallCase) => ({ recallCase, label: recallCaseLabels[recallCase] })
	);

	const preferredBeliefId = 'blf_8f2c41';

	// A link with the shopper's token reads the live layer; no token replays
	// the demo story. The browser guard keeps the request-bound URL off the
	// server, where the page is prerendered without a query.
	const liveLink = $derived(browser ? liveLinkFrom(page.url) : null);

	let step = $state<LedgerFixtureStep>(finalFixtureStep);
	let beliefId = $state<string | null>(null);
	// A link from the ledger names the belief to explain. The query wins on
	// arrival and the picker works from there.
	const linkedId = $derived(browser ? linkedBeliefId(page.url) : null);

	// The way back to the ledger keeps the shopper link, so a live why view
	// returns to the live ledger instead of the fixture replay.
	const ledgerHref = $derived(
		liveLink === null
			? resolve('/ledger')
			: `${resolve('/ledger')}?t=${encodeURIComponent(liveLink.token)}&s=${encodeURIComponent(liveLink.shopperId)}`
	);

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

	// The belief read and the recommendation read are independent, so one can
	// fail while the other answers. The page claims a live source only once one
	// of them did: a page that answered nothing says so in its alerts instead
	// of wearing a "Live" badge over an empty screen.
	const liveAnswered = $derived(detail !== null || recall !== null);

	// The shopper the pivot's surfaces act for. A live link names one
	// shopper; a replay names the shopper its case replays.
	const caseShopper = $derived(
		liveLink !== null ? liveLink.shopperId : recallCaseShopper[recallCase]
	);

	// The product line the page can honestly name for the belief it explains:
	// its subject's brand and the one category its evidence rows agree on,
	// never a fit handle. The live recall sends exactly this and the layer
	// resolves the handle from its own population; a replay sends the shopper
	// alone, as its fixture answers only that.
	const recallLine = $derived(detail === null ? null : recallLineFor(detail.belief));

	// One correction transport per shopper: a live link's corrects for that
	// shopper through the edge, and a replay's is the fixture's for the
	// shopper its case names, so a standing correction can never be recorded
	// under another shopper's name. The editor is keyed by that shopper below,
	// so a change remounts it with no earlier words or answer on screen.
	const contestState = $derived(
		liveLink !== null
			? new ContestState(new CorrectionClient(createLiveTransport(liveLink)), caseShopper, [])
			: createFixtureContestState(caseShopper, [])
	);
	const poolingState = $derived(
		liveLink !== null
			? new PoolingState(new PoolingClient(createLiveTransport(liveLink)), caseShopper)
			: createFixturePoolingState(caseShopper)
	);
	let standingList = $state<ContestCorrectionView[]>([]);

	// What the page says when the recall carries no pool: consulted with
	// nothing to say, or not consulted at all and why.
	const poolNote = $derived(recall === null ? null : poolNoteFor(recall, recallLine));

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
		const link = liveLink;
		const currentStep = step;
		const wanted = beliefId;
		let live = true;
		loading = true;
		loadError = null;
		void (async () => {
			try {
				const client =
					link === null
						? new LedgerClient(createFixtureTransport(currentStep))
						: new LedgerClient(createLiveTransport(link));
				const shopper = link === null ? fixtureShopperId : link.shopperId;
				const listed = await client.listBeliefs({
					shopper_id: shopper,
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
					client.getBelief({ shopper_id: shopper, belief_id: chosen }),
					client.listEvidence({ shopper_id: shopper }),
					client.listPendingQuestions({ shopper_id: shopper })
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
		const link = liveLink;
		if (link === null) {
			// A replay answers its fixture's shopper alone, exactly as before.
			const current = recallCase;
			let live = true;
			recallLoading = true;
			recallError = null;
			void (async () => {
				try {
					const transport = createFixtureRecallTransport(current);
					const shopper = recallCaseShopper[current];
					const raw = await transport.call('recall_for_purchase', {
						shopper_id: shopper
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
		}
		// A live recall reads the line the belief lets the page name, so it
		// waits for the belief read to settle instead of asking for a pool the
		// layer has no way to key. A belief that failed to load names no line:
		// the recall still runs, and the page says why the pool was not
		// consulted.
		const line = recallLine;
		if (loading) return;
		let live = true;
		recallLoading = true;
		recallError = null;
		void (async () => {
			try {
				const transport: RecallTransport = createLiveTransport(link);
				const request: Record<string, unknown> = { shopper_id: link.shopperId };
				if (line !== null && line.brand !== '') {
					request['brand_id'] = line.brand;
					if (line.category !== null) {
						request['category'] = line.category;
					}
				}
				const raw = await transport.call('recall_for_purchase', request);
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
<p class="why-back">
	<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- the live query is dynamic; the path is resolve()d -->
	<a href={ledgerHref}>Back to the ledger table</a>
</p>

{#if liveLink === null}
	<p class="why-replay" data-why-source="replay">
		Fixture replay: the demo story, not a live ledger. Open this page with a shopper
		link to read the live layer.
	</p>
{:else if liveAnswered}
	<p class="why-live" data-why-source="live">
		Live: the layer's own answer for shopper {liveLink.shopperId}.
	</p>
{/if}

<div class="why-controls">
	{#if liveLink === null}
		<label for="why-step">Story step</label>
		<select id="why-step" value={step} onchange={pickStep}>
			{#each stepOptions as option (option.step)}
				<option value={option.step}>{option.step}: {option.label}</option>
			{/each}
		</select>
	{/if}
	<label for="why-belief">Belief</label>
	<select id="why-belief" value={beliefId ?? ''} onchange={pickBelief}>
		{#each pickerRows as row (row.belief.belief_id)}
			<option value={row.belief.belief_id}>{optionLabel(row)}</option>
		{/each}
	</select>
</div>

{#if loadError !== null}
	<p role="alert" class="why-error">
		{#if liveLink !== null}
			The live why view read failed: {loadError}. No belief card is shown.
		{:else}
			The why view failed to load: {loadError}
		{/if}
	</p>
{:else if loading}
	<p class="why-loading">Loading the why view…</p>
{:else if detail === null}
	<p class="why-empty">No belief to explain yet: the layer holds none for this shopper.</p>
{:else}
	<WhyView view={detail} />
{/if}

<h2>What the layer recommends</h2>
<p class="lede">
	Which precedence step decided, with any overridden step beside the one that won.
	{#if liveLink === null}
		One shopper at a time: each case replays a single shopper's rows, and any other
		shopper fails instead of mixing in.
	{:else}
		Live for shopper {liveLink.shopperId}: the layer answers for that shopper alone.
	{/if}
</p>

{#if liveLink === null}
	<div class="why-controls">
		<label for="why-case">Recommendation case</label>
		<select id="why-case" value={recallCase} onchange={pickCase}>
			{#each caseOptions as option (option.recallCase)}
				<option value={option.recallCase}
					>{option.label} · {recallCaseShopper[option.recallCase]}</option
				>
			{/each}
		</select>
	</div>
{/if}

{#if recallError !== null}
	<p role="alert" class="why-error">
		{#if liveLink !== null}
			The live recommendation read failed: {recallError}. No recommendation is shown.
		{:else}
			The recommendation failed to load: {recallError}
		{/if}
	</p>
{:else if recallLoading || recall === null}
	<p class="why-loading">Loading the recommendation…</p>
{:else}
	<DecisionLine text={recall.guidance.text} basis={recall.guidance.basis} overridden={overridden} />
	{#if recall.pool !== null && recall.pool !== undefined}
		<PoolCounts pool={recall.pool} />
	{:else if poolNote !== null}
		<p class="why-pool-note" data-pool-note={poolNote.kind}>{poolNote.text}</p>
	{/if}
{/if}

<h2>Your standing correction</h2>
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
{#key caseShopper}
	<PoolingToggle pooling={poolingState} />
{/key}

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

	.why-pool-note {
		color: var(--dim);
		margin: 0 0 1.5rem;
	}

	.why-empty {
		border: 1px dashed var(--dim);
		border-radius: var(--radius-panel);
		padding: 1rem 1.25rem;
		color: var(--dim);
	}

	.why-back {
		margin: 0 0 1.5rem;
	}

	.why-replay {
		border: 1px dashed var(--dim);
		border-radius: var(--radius-panel);
		padding: 0.6rem 0.9rem;
		color: var(--dim);
		margin: 0 0 1.5rem;
	}

	.why-live {
		font-weight: 600;
		margin: 0 0 1.5rem;
	}

	.why-error {
		color: var(--stop);
	}
</style>
