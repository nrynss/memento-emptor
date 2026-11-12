<script lang="ts">
	// The front page: static copy, never a live read. It leads with the
	// headline the project chose, states the one count behind it with the
	// limits beside them, says what the layer is, and links every surface.

	import { resolve } from '$app/paths';
	import { productName } from '$lib/product';
	import { SEEDED_MARK_LABEL, SEEDED_MARK_TOOLTIP } from '$lib/ledger/rows';

	const merchantApp = 'https://memento-merchant.onrender.com';
	const repository = 'https://github.com/nrynss/memento-emptor';
	const round2Readme = `${repository}/blob/main/bench/round2/README.md`;
</script>

<svelte:head>
	<title>{productName}</title>
	<meta
		name="description"
		content="A memory layer that tells a shopping agent why a purchase came back, and what every other shopper's returns already taught."
	/>
</svelte:head>

<h1 data-headline>Right on day one for a new shopper</h1>

<p class="lede" data-claim data-count>
	On the new-to-brand case in round 2, arm 4 (Memento full) is 3/4 by the committed scores and
	4/4 by hand. Every other arm is 0/4.
	<a href={round2Readme} rel="external">The round-2 record</a>
	carries both readings and the run files behind them.
</p>

<p class="lede" data-claim data-limits>
	The limits sit beside the count. The case is constructed so the pool is the only input holding
	the answer. Two shoppers share one product line. The pool is the seeded population, not live
	traffic. It shows the mechanism, not real-world accuracy.
	<a href={resolve('/judges')}>The judges' page</a> states the same limits with their evidence.
</p>

<p class="lede" data-what>
	{productName} is a memory layer that tells a shopping agent why a purchase came back, and what
	every other shopper's returns already taught.
</p>

<section class="way-in" aria-labelledby="way-in-heading">
	<h2 id="way-in-heading">The way in</h2>
	<ul>
		<li class="way-in-lead">
			<a href={resolve('/judges')} data-link-judges>For judges: start here</a>
			<span>The shortest path through the demo, the evidence and the counts.</span>
		</li>
		<li>
			<a href={resolve('/ledger')} data-link-ledger>The evidence ledger</a>
			<span class="way-in-seeded" data-seeded-mark title={SEEDED_MARK_TOOLTIP}
				>{SEEDED_MARK_LABEL}</span
			>
			<span>Seeded rows are generated, not lived, and every one carries this mark.</span>
		</li>
		<li>
			<a href={resolve('/belief')} data-link-belief>The belief view</a>
			<span>One sizing belief with the returns behind it and the correction beside it.</span>
		</li>
		<li>
			<a href={merchantApp} rel="external" data-link-merchant>The merchant app</a>
			<span>The seller's read-only view of why a product line comes back.</span>
		</li>
		<li>
			<a href={repository} rel="external" data-link-repository>The repository</a>
			<span>The source, the demo records and the dev diary.</span>
		</li>
		<li>
			<a href={round2Readme} rel="external" data-link-comparison>The comparison</a>
			<span>Round 2's committed counts and the hand reading beside them.</span>
		</li>
	</ul>
</section>

<style>
	.way-in {
		margin: 2rem 0 0;
	}

	.way-in h2 {
		margin: 0 0 1rem;
		font-size: 1.35rem;
	}

	.way-in ul {
		display: grid;
		gap: 0.75rem;
		max-width: 46rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.way-in li {
		display: flex;
		align-items: baseline;
		flex-wrap: wrap;
		gap: 0.5rem;
		padding: 0.75rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		background: var(--surface);
	}

	.way-in-lead {
		border: 2px solid var(--accent);
	}

	.way-in a {
		color: var(--accent);
		font-weight: 600;
		text-decoration: underline;
		text-underline-offset: 2px;
	}

	.way-in li > span {
		color: var(--dim);
		font-size: 0.9375rem;
	}

	.way-in-seeded {
		display: inline-block;
		padding: 0.05rem 0.5rem;
		border: 1px dashed var(--dim);
		border-radius: 999px;
		color: var(--dim);
		font-size: 0.75rem;
		font-weight: 600;
		white-space: nowrap;
	}
</style>
