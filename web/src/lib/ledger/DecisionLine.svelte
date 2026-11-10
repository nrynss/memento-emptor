<script lang="ts">
	// Which precedence step decided the recommendation, and which overridden
	// step it beat. The winner names its step; every overridden step lists
	// beside it, never hidden. Svelte 5 runes only: props in, nothing else.
	import {
		decisionLineFor,
		stepLabelFor,
		type GuidanceBasis,
		type OverriddenStep
	} from './precedence';

	interface Props {
		text: string;
		basis: GuidanceBasis;
		overridden: OverriddenStep[];
	}

	let { text, basis, overridden }: Props = $props();
</script>

<section class="ledger-decision" aria-label="What decided this recommendation">
	<p class="ledger-decision-text">“{text}”</p>
	<p class="ledger-decision-basis">{decisionLineFor({ text, basis, correction_id: null, belief_id: null })}</p>
	{#if overridden.length > 0}
		<ul class="ledger-decision-overridden">
			{#each overridden as item (item.step)}
				<li>
					<span class="ledger-decision-loser">{stepLabelFor(item.step)}</span>
					was overridden by {stepLabelFor(item.winner).toLowerCase()}.
				</li>
			{/each}
		</ul>
	{/if}
</section>

<style>
	.ledger-decision {
		margin: 0 0 1.5rem;
		padding: 0.75rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		background: var(--surface);
		width: 100%;
		max-width: 100%;
		box-sizing: border-box;
	}

	.ledger-decision-text {
		margin: 0 0 0.35rem;
		font-weight: 600;
	}

	.ledger-decision-basis {
		margin: 0;
		color: var(--dim);
	}

	.ledger-decision-overridden {
		margin: 0.5rem 0 0;
		padding-left: 1.25rem;
		color: var(--dim);
	}

	.ledger-decision-loser {
		text-decoration: line-through;
	}
</style>
