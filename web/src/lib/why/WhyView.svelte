<script lang="ts">
	// One belief, explained: the correction beside the belief it contradicts,
	// each counted piece of evidence with its outcome, item and reason side by
	// side, the evidence deliberately not counted with why not, and the agent
	// turns behind each recommendation. Plain HTML over Chaaya tokens, so both
	// themes hold without a theme switch of its own; the evidence columns stack
	// below phone width.

	import { kindLabelFor, OUT_OF_TRUE_TOOLTIP, SEEDED_MARK_LABEL, SEEDED_MARK_TOOLTIP } from '$lib/ledger/rows';
	import type { EvidenceView } from '$lib/api';
	import { filterWhyView, reasonReading, type WhyViewData } from './why';

	interface Props {
		view: WhyViewData;
	}

	let { view }: Props = $props();

	let query = $state('');

	let filtered = $derived(filterWhyView(view, query));

	let totalCount = $derived(
		view.counted.length + view.not_counted.length + view.pending.length
	);
	let shownCount = $derived(
		filtered.counted.length + filtered.not_counted.length + filtered.pending.length
	);

	const searching = $derived(query.trim() !== '');

	function money(minor: number, currency: string): string {
		return `${(minor / 100).toFixed(2)} ${currency}`;
	}
</script>

{#snippet reasonColumn(evidence: EvidenceView)}
	{@const reading = reasonReading(evidence)}
	<div class="why-column">
		<h4>Reason</h4>
		{#if reading === null}
			<p>No reason recorded.</p>
		{:else if reading.quoted}
			<p class="why-words"><q>{reading.text}</q></p>
			{#if reading.word !== null}
				<p class="why-reason-word">{reading.word}</p>
			{/if}
		{:else if reading.word !== null}
			<p class="why-words">{reading.word}</p>
			<p class="why-reason-word">{reading.text}</p>
		{:else}
			<p class="why-words">{reading.text}</p>
		{/if}
	</div>
{/snippet}

<article class="why" aria-label={`Why the layer believes ${view.belief.value}`}>
	<header class="why-head">
		<p class="why-kind">
			{kindLabelFor(view.belief.kind)} · {view.belief.subject.name}
			{#if view.belief.seeded}
				<span class="why-seeded" title={SEEDED_MARK_TOOLTIP}>{SEEDED_MARK_LABEL}</span>
			{/if}
		</p>
		<h2 class="why-value">{view.belief.value}</h2>
		<p class="why-status">
			Status:
			<strong
				class="why-status-word"
				title={view.belief.status === 'Out of true' ? OUT_OF_TRUE_TOOLTIP : undefined}
			>
				{view.belief.status}
			</strong>
			{#if view.belief.contradicted}
				<span class="why-superseded">— earned before the correction; the correction decides now.</span>
			{/if}
		</p>
		{#if view.seeded_notice !== null}
			<p class="why-seeded-notice">{view.seeded_notice}</p>
		{/if}
	</header>

	{#if view.belief.contradicted && view.belief.contradicted_by !== null}
		{@const correction = view.belief.contradicted_by}
		<section class="why-correction" aria-label="The correction beside the belief">
			<h3>The correction beside the belief</h3>
			<p class="why-versus">
				You said <q>{correction.text}</q>; the evidence below said
				<q>{view.belief.value}</q>.
			</p>
			<p class="why-precedence">
				Recorded {correction.recorded_at.slice(0, 10)}. It wins the recommendation at
				once, and the belief stays on record but no longer decides.
			</p>
		</section>
	{/if}

	<section class="why-search" aria-label="Search across the evidence">
		<label for="why-search-box">Search across the evidence</label>
		<input
			id="why-search-box"
			type="search"
			placeholder="Shoulders, chino, dispute, refund…"
			bind:value={query}
		/>
		{#if searching}
			<p class="why-search-count">
				{shownCount} of {totalCount} rows match.
			</p>
		{/if}
	</section>

	<section class="why-counted" aria-label="Evidence counted toward this belief">
		<h3>What counts toward this belief ({filtered.counted.length})</h3>
		{#if filtered.counted.length === 0}
			<p class="why-empty">
				{#if searching}
					No counted evidence matches this search.
				{:else}
					Nothing counts toward this belief yet.
				{/if}
			</p>
		{:else}
			<ol class="why-list">
				{#each filtered.counted as row (row.evidence_id)}
					<li class="why-card">
						<div class="why-columns">
							{@render reasonColumn(row)}
							<div class="why-column">
								<h4>Outcome</h4>
								<p>{row.outcome.kind} · {row.outcome.status}</p>
								<p>{money(row.outcome.amount_minor, row.outcome.currency)}</p>
							</div>
							<div class="why-column">
								<h4>Item</h4>
								<p>{row.item.brand_name} {row.item.line_name}, size {row.item.size}</p>
								<p>SKU {row.item.sku} · {row.item.category}</p>
							</div>
						</div>
						<p class="why-meta">
							{row.occurred_at.slice(0, 10)} · {row.evidence_id}
							{#if row.seeded}
								· <span class="why-seeded" title={SEEDED_MARK_TOOLTIP}>{SEEDED_MARK_LABEL}</span>
							{/if}
						</p>
					</li>
				{/each}
			</ol>
		{/if}
	</section>

	<section class="why-not-counted" aria-label="Evidence deliberately not counted">
		<h3>Deliberately not counted ({filtered.not_counted.length})</h3>
		{#if filtered.not_counted.length === 0}
			<p class="why-empty">
				{#if searching}
					No not-counted evidence matches this search.
				{:else}
					Everything in the ledger counts toward this belief.
				{/if}
			</p>
		{:else}
			<ol class="why-list">
				{#each filtered.not_counted as row (row.evidence.evidence_id)}
					<li class="why-card">
						<p class="why-not">{row.why_not}</p>
						<div class="why-columns">
							{@render reasonColumn(row.evidence)}
							<div class="why-column">
								<h4>Outcome</h4>
								<p>{row.evidence.outcome.kind} · {row.evidence.outcome.status}</p>
								<p>
									{money(row.evidence.outcome.amount_minor, row.evidence.outcome.currency)}
								</p>
							</div>
							<div class="why-column">
								<h4>Item</h4>
								<p>
									{row.evidence.item.brand_name}
									{row.evidence.item.line_name}, size {row.evidence.item.size}
								</p>
								<p>SKU {row.evidence.item.sku} · {row.evidence.item.category}</p>
							</div>
						</div>
						<p class="why-meta">
							{row.evidence.occurred_at.slice(0, 10)} · {row.evidence.evidence_id}
							{#if row.evidence.seeded}
								· <span class="why-seeded" title={SEEDED_MARK_TOOLTIP}>{SEEDED_MARK_LABEL}</span>
							{/if}
						</p>
					</li>
				{/each}
			</ol>
		{/if}
	</section>

	{#if view.pending.length > 0}
		<section class="why-pending" aria-label="Refunds still waiting on the shopper">
			<h3>Still waiting on the shopper ({filtered.pending.length})</h3>
			{#if filtered.pending.length === 0}
				<p class="why-empty">None of the open questions match this search.</p>
			{:else}
				<ol class="why-list">
					{#each filtered.pending as row (row.question.question_id)}
						<li class="why-card">
							<p class="why-not">{row.why_not}</p>
							<p>
								{row.question.outcome.kind} ·
								{row.question.item.brand_name}
								{row.question.item.line_name}, size {row.question.item.size} ·
								{money(
									row.question.outcome.amount_minor,
									row.question.outcome.currency
								)}
							</p>
						</li>
					{/each}
				</ol>
			{/if}
		</section>
	{/if}

	<section class="why-traces" aria-label="Agent turns behind the recommendations">
		<h3>Agent turns behind the recommendations</h3>
		<p class="why-fixture-note">
			This is a fixture replay: each link is a placeholder on this page, not a trace viewer.
			A live transport for the trace links is still to come.
		</p>
		{#if view.traces.length === 0}
			<p class="why-empty">No recorded turn touches this belief in the fixture replay.</p>
		{:else}
			<ul class="why-trace-list">
				{#each view.traces as trace (trace.trace_id)}
					<li id={`trace-${trace.trace_id}`}>
						<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- same-page placeholder anchor, not navigation -->
						<a href={trace.href}>{trace.trace_id}</a> — {trace.label}
					</li>
				{/each}
			</ul>
		{/if}
	</section>
</article>

<style>
	.why {
		display: flex;
		flex-direction: column;
		gap: 2rem;
		max-width: 56rem;
	}

	.why-kind {
		margin: 0 0 0.25rem;
		color: var(--dim);
		font-size: 0.875rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.why-value {
		margin: 0 0 0.5rem;
		font-size: clamp(1.5rem, 4vw, 2.25rem);
		line-height: 1.1;
		letter-spacing: -0.02em;
	}

	.why-status {
		margin: 0;
	}

	.why-superseded {
		color: var(--dim);
	}

	.why-seeded {
		display: inline-block;
		padding: 0.05rem 0.5rem;
		border-radius: 999px;
		font-size: 0.75rem;
		font-weight: 600;
		border: 1px dashed var(--dim);
		color: var(--dim);
		white-space: nowrap;
	}

	.why-seeded-notice {
		color: var(--dim);
		font-size: 0.9375rem;
	}

	.why-correction {
		border: 1px solid var(--line);
		border-left: 4px solid var(--stop);
		border-radius: var(--radius-panel);
		background: var(--surface);
		padding: 1rem 1.25rem;
	}

	.why-correction h3,
	.why-counted h3,
	.why-not-counted h3,
	.why-pending h3,
	.why-traces h3 {
		margin: 0 0 0.75rem;
		font-size: 1.125rem;
	}

	.why-versus {
		font-size: 1.0625rem;
		margin: 0 0 0.5rem;
	}

	.why-precedence {
		margin: 0;
		color: var(--dim);
	}

	.why-search {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.why-search label {
		font-weight: 600;
	}

	.why-search input {
		font: inherit;
		color: inherit;
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: var(--radius-control);
		padding: 0.45rem 0.8rem;
		max-width: 28rem;
		width: 100%;
	}

	.why-search-count {
		margin: 0;
		color: var(--dim);
		font-size: 0.9375rem;
	}

	.why-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.why-card {
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		background: var(--surface);
		padding: 1rem 1.25rem;
	}

	.why-columns {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 1rem;
	}

	.why-column h4 {
		margin: 0 0 0.25rem;
		font-size: 0.8125rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--dim);
	}

	.why-column p {
		margin: 0 0 0.25rem;
	}

	.why-words {
		font-size: 1.0625rem;
	}

	.why-reason-word {
		color: var(--dim);
		font-size: 0.875rem;
	}

	.why-meta {
		margin: 0.75rem 0 0;
		color: var(--dim);
		font-size: 0.875rem;
	}

	.why-not {
		margin: 0 0 0.75rem;
		font-weight: 600;
	}

	.why-empty {
		color: var(--dim);
		margin: 0;
	}

	.why-fixture-note {
		color: var(--dim);
		font-size: 0.9375rem;
		margin: 0 0 0.5rem;
	}

	.why-trace-list {
		margin: 0;
		padding-left: 1.25rem;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	@media (max-width: 560px) {
		.why-columns {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
