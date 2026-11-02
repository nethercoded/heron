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
		<p class="mt-1.5 flex items-center gap-1.5 text-[12px] font-semibold text-down">
			<i class="size-1.5 shrink-0 rounded-[1px] bg-down"></i>
			{error}
		</p>
	{:else if hint}
		<p class="hint">{hint}</p>
	{/if}
</div>
