<script lang="ts">
	import { productName } from '$lib/product';
</script>

<svelte:head>
	<title>{productName}</title>
	<meta
		name="description"
		content="A memory layer for shopping agents built on PayPal, learned from what happened to the money."
	/>
</svelte:head>

<h1>Remember, buyer.</h1>
<p class="lede">
	A shopping agent starts every session with no memory. Memento Emptor gives it beliefs it earned
	from refunds, disputes and fit feedback, each tied to the evidence behind it.
</p>
<p class="lede">
	A payment outcome is not a cause. The layer keeps the outcome, the item and the reason apart, so
	a parcel that never arrived never changes a size recommendation.
</p>
