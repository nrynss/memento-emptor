<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { theme as themeStore, themeToggleLabel } from '@nrynss/chaaya/theme';
	import { reactive } from '@nrynss/chaaya/svelte';
	import { merchantName, tagline } from '$lib/product';

	let { children } = $props();

	const theme = reactive(themeStore);
	let hydrated = $state(false);

	onMount(() => {
		hydrated = true;
	});

	const label = $derived(hydrated ? themeToggleLabel(theme.resolved) : 'Switch theme');

	function toggle(): void {
		theme.set(theme.resolved === 'dark' ? 'light' : 'dark');
	}
</script>

<div class="shell">
	<header class="shell-header">
		<a class="shell-brand" href={resolve('/')}>{merchantName}</a>
		<button type="button" onclick={toggle}>{label}</button>
	</header>

	<main class="shell-main">
		{@render children()}
	</main>

	<footer class="shell-footer">
		{tagline}
	</footer>
</div>
