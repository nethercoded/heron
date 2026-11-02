<script lang="ts">
	import { useI18n } from '#lib/i18n.svelte.ts';
	import { statusTone } from '#lib/status.ts';
	import type { Beat } from '#lib/types.ts';

	let { beats, slots = 60 }: { beats: Beat[]; slots?: number } = $props();

	const i18n = useI18n();
	const padded = $derived([
		...Array(Math.max(0, slots - beats.length)).fill(null),
		...beats.slice(-slots)
	] as (Beat | null)[]);
</script>

<div class="flex h-6 items-stretch gap-[2px]" dir="ltr">
	{#each padded as beat, i (i)}
		{#if beat}
			<span
				class="flex-1 rounded-sm {statusTone[beat.status]}"
				title="{i18n.dateTime(beat.at)} · {i18n.status(beat.status)} · {i18n.latency(beat.latency_ms)}{beat.message ? ` · ${beat.message}` : ''}"
			></span>
		{:else}
			<span class="flex-1 rounded-sm bg-sunken"></span>
		{/if}
	{/each}
</div>
