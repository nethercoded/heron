<script lang="ts">
	import { useI18n } from '#lib/i18n.svelte.ts';
	import type { Range } from '#lib/types.ts';

	let { value = $bindable() }: { value: Range } = $props();

	const i18n = useI18n();
	const ranges: Range[] = ['24h', '7d', '30d', '90d'];
</script>

<div class="inline-flex rounded-full bg-surface p-1" role="tablist">
	{#each ranges as range (range)}
		<button
			type="button"
			role="tab"
			aria-selected={value === range}
			class="rounded-full px-3 py-1 font-mono text-xs font-medium transition-colors {value === range ? 'bg-ink text-canvas' : 'text-muted hover:text-ink'}"
			onclick={() => (value = range)}
		>
			{i18n.range(range)}
		</button>
	{/each}
</div>
