<script lang="ts">
	// The merchant grid as a Svelte 5 component. The grid is created once in
	// an effect and destroyed in its teardown; row updates, theme switches and
	// the selected row arrive through grid options, so in-flight state survives
	// a theme toggle. A row click reports its id to the page, which opens the
	// why panel beside the grid.

	import { theme as themeStore } from '@nrynss/chaaya/theme';
	import { reactive } from '@nrynss/chaaya/svelte';
	import type { GridApi } from 'ag-grid-community';
	import { createMerchantGrid } from './grid';
	import { merchantGridTheme } from './theme';
	import type { MerchantGridRow } from './rows';

	interface Props {
		rows: MerchantGridRow[];
		selectedId?: string | null;
		onSelect?: (rowId: string | null) => void;
	}

	let { rows, selectedId = null, onSelect }: Props = $props();

	const theme = reactive(themeStore);

	let host: HTMLDivElement | undefined = $state(undefined);
	let api: GridApi<MerchantGridRow> | undefined = $state(undefined);

	$effect(() => {
		if (host === undefined) {
			return;
		}
		const gridApi = createMerchantGrid(host, { rows, onSelect });
		api = gridApi;
		return () => {
			gridApi.destroy();
			api = undefined;
		};
	});

	$effect(() => {
		api?.setGridOption('rowData', rows);
	});

	$effect(() => {
		api?.setGridOption('theme', merchantGridTheme(theme.resolved));
	});
</script>

<div
	class="merchant-grid-host"
	bind:this={host}
	role="region"
	aria-label="Merchant pool"
	data-selected={selectedId ?? ''}
></div>

<style>
	.merchant-grid-host {
		width: 100%;
		min-height: 8rem;
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		overflow: hidden;
		background: var(--surface);
	}

	.merchant-grid-host :global(.merchant-pill) {
		display: inline-block;
		padding: 0.1rem 0.6rem;
		border-radius: 999px;
		font-size: 0.8125rem;
		font-weight: 600;
		line-height: 1.6;
		border: 1px solid var(--line);
		color: var(--text);
		background: var(--raised);
		white-space: nowrap;
	}

	.merchant-grid-host :global(.merchant-earned) {
		color: var(--ok);
		border-color: var(--ok);
		background: var(--ok-soft);
	}

	.merchant-grid-host :global(.merchant-building) {
		color: var(--warn);
		border-color: var(--warn);
		background: var(--warn-soft);
	}

	.merchant-grid-host :global(.merchant-out-of-true) {
		color: var(--stop);
		border-color: var(--stop);
		background: var(--stop-soft);
	}

	.merchant-grid-host :global(.merchant-claimed) {
		color: var(--accent);
		border-color: var(--accent);
		background: var(--accent-soft);
	}

	.merchant-grid-host :global(.merchant-seeded) {
		display: inline-block;
		padding: 0.1rem 0.5rem;
		border-radius: 999px;
		font-size: 0.75rem;
		font-weight: 600;
		border: 1px dashed var(--dim);
		color: var(--dim);
		white-space: nowrap;
	}
</style>
