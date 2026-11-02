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
		{#if label}<span class="block text-[14px] font-medium text-ink">{label}</span>{/if}
		{#if description}
			<span class="mt-0.5 block text-[12.5px] text-faint">{description}</span>
		{/if}
	</span>
	<span class="relative mt-0.5 inline-flex shrink-0">
		<input type="checkbox" class="peer sr-only" bind:checked onchange={() => onchange?.(checked ?? false)} />
		<span class="h-6 w-11 rounded-full border border-line-strong bg-canvas transition-colors peer-checked:border-ink peer-checked:bg-ink peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-ink"></span>
		<span class="absolute top-[5px] left-[5px] size-3.5 rounded-[3px] bg-faint transition-[translate,background-color] duration-200 peer-checked:translate-x-5 peer-checked:bg-canvas"></span>
	</span>
</label>
