<script lang="ts">
	import { onMount } from 'svelte';
	import { theme as themeStore, themeToggleLabel } from '@nrynss/chaaya/theme';
	import { reactive } from '@nrynss/chaaya/svelte';
	import BeliefGrid from './BeliefGrid.svelte';
	import type { BeliefEdit } from './grid';

	const theme = reactive(themeStore);
	let hydrated = $state(false);
	let lastEdit = $state<BeliefEdit | null>(null);

	onMount(() => {
		hydrated = true;
	});

	const label = $derived(hydrated ? themeToggleLabel(theme.resolved) : 'Switch theme');

	function toggle(): void {
		theme.set(theme.resolved === 'dark' ? 'light' : 'dark');
	}

	function handleEdit(edit: BeliefEdit): void {
		lastEdit = edit;
	}
</script>

<svelte:head>
	<title>AG Grid spike: belief ledger</title>
</svelte:head>

<header class="demo-header">
	<h1>Belief ledger spike</h1>
	<button type="button" onclick={toggle}>{label}</button>
</header>

<main class="demo-main">
	<p class="lede">
		Four belief statuses, rendered by AG Grid through its plain JavaScript API. The Status
		and Shopper note columns edit in place; the last committed edit shows below the grid.
	</p>

	<BeliefGrid onEdit={handleEdit} />

	<p class="last-edit" aria-live="polite">
		{#if lastEdit === null}
			No edits yet. Double-click a Status or Shopper note cell to edit it.
		{:else}
			Edited {lastEdit.id}: {lastEdit.field} changed from “{lastEdit.oldValue}” to
			“{lastEdit.newValue}”.
		{/if}
	</p>
</main>
