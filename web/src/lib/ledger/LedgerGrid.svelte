// The ledger grid as a Svelte 5 component. The grid is created once in an
// effect and destroyed in its teardown; row updates and theme switches
// arrive through grid options, so in-flight state survives a theme toggle.

<script lang="ts">
	import { theme as themeStore } from '@nrynss/chaaya/theme';
	import { reactive } from '@nrynss/chaaya/svelte';
	import type { GridApi } from 'ag-grid-community';
	import { createLedgerGrid } from './grid';
	import { ledgerGridTheme } from './theme';
	import type { LedgerGridRow } from './rows';

	interface Props {
		rows: LedgerGridRow[];
	}

	let { rows }: Props = $props();

	const theme = reactive(themeStore);

	let host: HTMLDivElement | undefined = $state(undefined);
	let api: GridApi<LedgerGridRow> | undefined = $state(undefined);

	$effect(() => {
		if (host === undefined) {
			return;
		}
		const gridApi = createLedgerGrid(host, { rows });
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
		api?.setGridOption('theme', ledgerGridTheme(theme.resolved));
	});
</script>

<div class="ledger-grid-host" bind:this={host} role="region" aria-label="Evidence ledger"></div>

<style>
	.ledger-grid-host {
		width: 100%;
		min-height: 8rem;
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		overflow: hidden;
		background: var(--surface);
	}

	.ledger-grid-host :global(.ledger-pill) {
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

	.ledger-grid-host :global(.ledger-earned) {
		color: var(--ok);
		border-color: var(--ok);
		background: var(--ok-soft);
	}

	.ledger-grid-host :global(.ledger-building) {
		color: var(--warn);
		border-color: var(--warn);
		background: var(--warn-soft);
	}

	.ledger-grid-host :global(.ledger-out-of-true) {
		color: var(--stop);
		border-color: var(--stop);
		background: var(--stop-soft);
	}

	.ledger-grid-host :global(.ledger-claimed) {
		color: var(--accent);
		border-color: var(--accent);
		background: var(--accent-soft);
	}

	.ledger-grid-host :global(.ledger-correction) {
		display: block;
		margin-top: 0.15rem;
		font-size: 0.8125rem;
		color: var(--dim);
		white-space: normal;
	}

	.ledger-grid-host :global(.ledger-weak) {
		font-size: 0.8125rem;
		color: var(--warn);
		white-space: nowrap;
	}

	.ledger-grid-host :global(.ledger-seeded) {
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
