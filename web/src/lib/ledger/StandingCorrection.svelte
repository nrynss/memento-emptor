<script lang="ts">
	// The shopper's standing correction: explicit, first-person, with no
	// belief yet. It decides at once with precedence and never promotes; when
	// a matching belief appears the layer declares it against that belief.
	// Svelte 5 runes only: props in, nothing else.

	interface Props {
		text: string;
		recordedAt: string;
	}

	let { text, recordedAt }: Props = $props();
</script>

<section class="ledger-standing" aria-label="Standing correction">
	<p class="ledger-standing-text">“{text}”</p>
	<p class="ledger-standing-note">
		Standing correction: no belief yet, so nothing is contradicted. It decides at once.
	</p>
	<p class="ledger-standing-when">Recorded {recordedAt.slice(0, 10)}.</p>
</section>

<style>
	.ledger-standing {
		margin: 0 0 1.5rem;
		padding: 0.75rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		background: var(--surface);
		width: 100%;
		max-width: 100%;
		box-sizing: border-box;
	}

	.ledger-standing-text {
		margin: 0 0 0.35rem;
		font-weight: 600;
	}

	.ledger-standing-note {
		margin: 0 0 0.25rem;
		color: var(--dim);
	}

	.ledger-standing-when {
		margin: 0;
		color: var(--dim);
		font-size: 0.8125rem;
	}
</style>
