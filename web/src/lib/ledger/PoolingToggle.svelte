<script lang="ts">
	// The shopper's pooling choice, through set_pooling and nothing else.
	// Opting out stops new fit rows and removes the ones already written, and
	// the layer says how many rows went; opting back in removes nothing. The
	// answered choice lands in local state, so the toggle shows the result
	// from the layer's own answer even where an outside notification cannot
	// reach the template. Svelte 5 runes only: props in, local $state out.

	import { choiceLineFor, resultLineFor, type PoolingState } from './pooling';

	interface Props {
		pooling: PoolingState;
	}

	let { pooling }: Props = $props();

	// The choice the toggle opened with. Every change after that lands here
	// from the layer's own answer, so the toggle never waits on an outside
	// notification to show what the shopper chose.
	function openedWith(): boolean {
		return pooling.pooling;
	}

	let busy = $state(false);
	let choice = $state(openedWith());
	let removed = $state<number | null>(null);
	let settled = $state(false);
	let formError = $state<string | null>(null);

	async function choose(next: boolean): Promise<void> {
		if (busy) return;
		busy = true;
		formError = null;
		try {
			const response = await pooling.setPooling(next);
			choice = response.pooling;
			removed = response.rows_removed;
			settled = true;
		} catch (error) {
			formError = error instanceof Error ? error.message : 'The pooling choice failed.';
		} finally {
			busy = false;
		}
	}
</script>

<section class="ledger-pooling" aria-label="Pooling choice">
	<p class="ledger-pooling-choice">{choiceLineFor(choice)}</p>
	<div class="ledger-pooling-buttons">
		<button
			type="button"
			class="ledger-pooling-button"
			disabled={busy || !choice}
			onclick={() => void choose(false)}
		>
			{busy && !choice ? 'Opting out…' : 'Opt out of pooling'}
		</button>
		<button
			type="button"
			class="ledger-pooling-button"
			disabled={busy || choice}
			onclick={() => void choose(true)}
		>
			{busy && choice ? 'Opting back in…' : 'Opt back in'}
		</button>
	</div>
	{#if resultLineFor(choice, removed, settled) !== null}
		<p role="status" class="ledger-pooling-result">{resultLineFor(choice, removed, settled)}</p>
	{/if}
	{#if formError !== null}
		<p role="alert" class="ledger-pooling-error">{formError}</p>
	{/if}
</section>

<style>
	.ledger-pooling {
		margin: 0 0 1.5rem;
		padding: 0.75rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		background: var(--surface);
		width: 100%;
		max-width: 100%;
		box-sizing: border-box;
	}

	.ledger-pooling-choice {
		margin: 0 0 0.5rem;
	}

	.ledger-pooling-buttons {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}

	.ledger-pooling-button {
		font: inherit;
		color: var(--text);
		background: var(--raised);
		border: 1px solid var(--line);
		border-radius: var(--radius-control);
		padding: 0.35rem 0.9rem;
		cursor: pointer;
		max-width: 100%;
	}

	.ledger-pooling-button:disabled {
		cursor: wait;
		opacity: 0.7;
	}

	.ledger-pooling-result {
		margin: 0.5rem 0 0;
		color: var(--ok);
	}

	.ledger-pooling-error {
		margin: 0.5rem 0 0;
		color: var(--stop);
	}
</style>
