<script lang="ts">
	// The pooled belief that applies to the shopper, as counts with its
	// seeded split. Counts only: the pool carries no shopper identity, no
	// row and no free text, and this view renders nothing else. Svelte 5
	// runes only: props in, nothing else.

	import { poolSeededLine, poolSummaryLine, type PoolView } from './pool';

	interface Props {
		pool: PoolView;
	}

	let { pool }: Props = $props();

	const counts = $derived(pool.counts);
</script>

<section class="ledger-pool" aria-label="Pooled product fit">
	<h2 class="ledger-pool-title">
		Pooled fit for {pool.line.brand_id} · {pool.line.category} · {pool.line.fit_handle}, size {pool.size}
	</h2>
	<p class="ledger-pool-summary">{poolSummaryLine(pool)}</p>
	<dl class="ledger-pool-counts">
		<div>
			<dt>Settled purchases</dt>
			<dd>{counts.settled}</dd>
		</div>
		<div>
			<dt>Too-small returns</dt>
			<dd>{counts.returns.too_small}</dd>
		</div>
		<div>
			<dt>Too-large returns</dt>
			<dd>{counts.returns.too_large}</dd>
		</div>
		<div>
			<dt>Fits-well feedback</dt>
			<dd>{counts.feedback.fits_well}</dd>
		</div>
		<div>
			<dt>Distinct shoppers</dt>
			<dd>{counts.shoppers.fit}</dd>
		</div>
	</dl>
	<p class="ledger-pool-seeded">{poolSeededLine(pool)}</p>
</section>

<style>
	.ledger-pool {
		margin: 0 0 1.5rem;
		padding: 0.75rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		background: var(--surface);
		width: 100%;
		max-width: 100%;
		box-sizing: border-box;
	}

	.ledger-pool-title {
		margin: 0 0 0.35rem;
		font-size: 1rem;
	}

	.ledger-pool-summary {
		margin: 0 0 0.5rem;
		font-weight: 600;
	}

	.ledger-pool-counts {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
		gap: 0.5rem;
		margin: 0 0 0.5rem;
	}

	.ledger-pool-counts div {
		margin: 0;
		padding: 0.35rem 0.6rem;
		border: 1px solid var(--line);
		border-radius: var(--radius-control);
		background: var(--raised);
	}

	.ledger-pool-counts dt {
		font-size: 0.8125rem;
		color: var(--dim);
	}

	.ledger-pool-counts dd {
		margin: 0;
		font-size: 1.125rem;
		font-weight: 600;
	}

	.ledger-pool-seeded {
		margin: 0;
		color: var(--dim);
	}
</style>
