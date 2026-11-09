<script lang="ts">
	import { theme as themeStore } from '@nrynss/chaaya/theme';
	import { reactive } from '@nrynss/chaaya/svelte';
	import type { GridApi } from 'ag-grid-community';
	import { BELIEF_ROWS, type BeliefRow } from './beliefs';
	import { createBeliefGrid, type BeliefEdit } from './grid';
	import { chaayaGridTheme } from './gridTheme';

	interface Props {
		rows?: BeliefRow[];
		onEdit?: (edit: BeliefEdit) => void;
	}

	let { rows = BELIEF_ROWS, onEdit }: Props = $props();

	const theme = reactive(themeStore);

	let host: HTMLDivElement | undefined = $state(undefined);
	let api: GridApi<BeliefRow> | undefined = $state(undefined);

	$effect(() => {
		if (host === undefined) {
			return;
		}
		const gridApi = createBeliefGrid(host, { initialRows: rows, onEdit });
		api = gridApi;
		return () => {
			gridApi.destroy();
			api = undefined;
		};
	});

	$effect(() => {
		api?.setGridOption('theme', chaayaGridTheme(theme.resolved));
	});
</script>

<div class="belief-grid-host" bind:this={host}></div>
