<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		label,
		id,
		hint,
		error,
		class: className = '',
		children
	}: { label: string; id: string; hint?: string; error?: string; class?: string; children: Snippet } = $props();
</script>

<div class={className}>
	<label for={id} class="label">{label}</label>
	{@render children()}
	{#if error}
		<p class="mt-1.5 text-xs text-down">{error}</p>
	{:else if hint}
		<p class="hint">{hint}</p>
	{/if}
</div>
