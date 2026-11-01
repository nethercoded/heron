<script lang="ts">
	import type { Range } from '#lib/types.ts';

	let { value = $bindable() }: { value: Range } = $props();

	const ranges: Range[] = ['24h', '7d', '30d', '90d'];
</script>

<div class="inline-flex rounded-lg bg-sunken p-0.5" role="tablist">
	{#each ranges as range (range)}
		<button
			type="button"
			role="tab"
			aria-selected={value === range}
			class="rounded-md px-2.5 py-1 text-xs font-medium transition {value === range ? 'bg-surface text-ink shadow-card' : 'text-muted hover:text-ink'}"
			onclick={() => (value = range)}
		>
			{range}
		</button>
	{/each}
</div>
