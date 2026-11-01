<script lang="ts">
	import { formatDateTime, latency } from '#lib/format.ts';
	import { statusLabel, statusTone } from '#lib/status.ts';
	import type { Beat } from '#lib/types.ts';

	let { beats, slots = 60 }: { beats: Beat[]; slots?: number } = $props();

	const padded = $derived([
		...Array(Math.max(0, slots - beats.length)).fill(null),
		...beats.slice(-slots)
	] as (Beat | null)[]);
</script>

<div class="flex h-6 items-stretch gap-[2px]">
	{#each padded as beat, i (i)}
		{#if beat}
			<span
				class="flex-1 rounded-sm {statusTone[beat.status]}"
				title="{formatDateTime(beat.at)} · {statusLabel[beat.status]} · {latency(beat.latency_ms)}{beat.message ? ` · ${beat.message}` : ''}"
			></span>
		{:else}
			<span class="flex-1 rounded-sm bg-sunken"></span>
		{/if}
	{/each}
</div>
