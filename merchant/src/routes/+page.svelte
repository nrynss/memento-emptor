<script lang="ts">
	// The merchant page: one read-only table over the seller's lines, and the
	// why for the selected row. Every row comes from product_fit through the
	// same-origin proxy, with no caller credential: counts and a status, never
	// a personal row. Until the proxy answers, the grid reads the bundled
	// snapshot. Selecting a row opens the counts behind its status and the
	// rule it was judged by.

	import {
		createFixtureTransport,
		createSameOriginTransport,
		explainWhy,
		loadSellerPools,
		MerchantClient,
		merchantLines,
		type PoolSource,
		type PoolView
	} from '$lib/pool';
	import { toMerchantGridRows, type MerchantGridRow } from '$lib/merchant/rows';
	import MerchantGrid from '$lib/merchant/MerchantGrid.svelte';
	import { merchantName } from '$lib/product';

	const client = new MerchantClient(createSameOriginTransport());
	const snapshot = new MerchantClient(createFixtureTransport());

	let pools: PoolView[] = $state([]);
	let selectedId: string | null = $state(null);
	let source: PoolSource | null = $state(null);
	let error: string | null = $state(null);

	async function load(): Promise<void> {
		try {
			const loaded = await loadSellerPools(client, snapshot, merchantLines);
			pools = loaded.pools.map((row) => row.pool);
			source = loaded.source;
			const gridRows = toMerchantGridRows(pools);
			selectedId = gridRows[0]?.id ?? null;
		} catch (loadError) {
			error = loadError instanceof Error ? loadError.message : 'Pool load failed';
		}
	}

	$effect(() => {
		void load();
	});

	const rows: MerchantGridRow[] = $derived(toMerchantGridRows(pools));

	const selected: PoolView | null = $derived.by(() => {
		if (selectedId === null) return null;
		return (
			pools.find(
				(pool) =>
					`${pool.line.brand_id}/${pool.line.category}/${pool.line.fit_handle}/${pool.size}` ===
					selectedId
			) ?? null
		);
	});

	const why: string[] = $derived(selected === null ? [] : explainWhy(selected));

	function onSelect(rowId: string | null): void {
		selectedId = rowId;
	}
</script>

<svelte:head>
	<title>{merchantName}</title>
	<meta
		name="description"
		content="The seller's read-only view of pooled fit: why products come back, by reason and size."
	/>
</svelte:head>

<h1>Why it comes back.</h1>
<p class="lede">
	One row per product line and size: settled purchases, returns by reason category and
	direction, fits-well feedback, the pool status, and the seeded count. Select a row to see
	the counts behind its status and the rule it was judged by.
</p>

{#if error !== null}
	<p role="alert">{error}</p>
{:else}
	{#if source !== null}
		<p class="source-line">
			{#if source === 'live'}
				Live pool counts, read through the merchant proxy.
			{:else}
				Bundled snapshot: the live read is not answering yet.
			{/if}
		</p>
	{/if}
	<MerchantGrid {rows} {selectedId} {onSelect} />
	{#if selected !== null}
		<section class="why-panel" aria-label="Why this row">
			<h2>
				{selected.line.brand_id} · {selected.line.category} · {selected.line.fit_handle}, size {selected.size}
			</h2>
			<ul>
				{#each why as line (line)}
					<li>{line}</li>
				{/each}
			</ul>
		</section>
	{/if}
{/if}
