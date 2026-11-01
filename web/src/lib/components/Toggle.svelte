<script lang="ts">
	let {
		checked = $bindable(),
		label,
		description,
		onchange
	}: { checked?: boolean; label: string; description?: string; onchange?: (value: boolean) => void } = $props();
</script>

<label class="flex cursor-pointer items-start justify-between gap-4">
	<span>
		<span class="block text-[13px] font-medium text-ink">{label}</span>
		{#if description}
			<span class="block text-xs text-muted">{description}</span>
		{/if}
	</span>
	<span class="relative mt-0.5 inline-flex shrink-0">
		<input type="checkbox" class="peer sr-only" bind:checked onchange={() => onchange?.(checked ?? false)} />
		<span class="h-5 w-9 rounded-full bg-line-strong transition peer-checked:bg-accent peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-accent"></span>
		<span class="absolute top-0.5 left-0.5 size-4 rounded-full bg-white shadow transition peer-checked:translate-x-4"></span>
	</span>
</label>
