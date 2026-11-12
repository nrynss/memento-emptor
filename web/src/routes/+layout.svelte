<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { theme as themeStore, themeToggleLabel } from '@nrynss/chaaya/theme';
	import { reactive } from '@nrynss/chaaya/svelte';
	import { productName, tagline } from '$lib/product';

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

	const merchantApp = 'https://memento-merchant.onrender.com';
</script>

<div class="shell">
	<header class="shell-header">
		<a class="shell-brand" href={resolve('/')}>{productName}</a>
		<nav class="shell-nav" aria-label="Main">
			<a href={resolve('/judges')}>Judges</a>
			<a href={resolve('/ledger')}>Ledger</a>
			<a href={resolve('/belief')}>Belief</a>
			<a href={merchantApp} rel="external">Merchant</a>
		</nav>
		<button type="button" onclick={toggle}>{label}</button>
	</header>

	<main class="shell-main">
		{@render children()}
	</main>

	<footer class="shell-footer">
		{tagline}
	</footer>
</div>

<style>
	.shell-nav {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.35rem 1rem;
		margin-left: auto;
	}

	.shell-nav a {
		color: var(--dim);
		font-size: 0.9375rem;
		text-decoration: none;
	}

	.shell-nav a:hover {
		color: var(--text);
		text-decoration: underline;
		text-underline-offset: 2px;
	}

	@media (max-width: 40rem) {
		.shell-header {
			flex-wrap: wrap;
		}

		.shell-nav {
			order: 3;
			flex-basis: 100%;
			margin-left: 0;
		}
	}
</style>
