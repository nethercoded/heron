<script lang="ts">
	import { fly } from 'svelte/transition';
	import { toast } from '#lib/toast.svelte.ts';
	import Icon from './Icon.svelte';

	const icons = { success: 'check', error: 'alert', info: 'info' } as const;
	const tones = { success: 'text-up', error: 'text-down', info: 'text-accent' };
</script>

<div class="pointer-events-none fixed inset-x-0 bottom-0 z-50 flex flex-col items-center gap-2 p-4 sm:items-end" aria-live="polite">
	{#each toast.items as item (item.id)}
		<div
			class="card pointer-events-auto flex w-full max-w-sm items-start gap-3 p-3.5 text-sm shadow-lg"
			transition:fly={{ y: 12, duration: 180 }}
		>
			<Icon name={icons[item.tone]} class="mt-0.5 {tones[item.tone]}" />
			<p class="flex-1 text-ink">{item.text}</p>
			<button type="button" class="text-faint hover:text-ink" onclick={() => toast.dismiss(item.id)} aria-label="Dismiss">
				<Icon name="x" size={14} />
			</button>
		</div>
	{/each}
</div>
