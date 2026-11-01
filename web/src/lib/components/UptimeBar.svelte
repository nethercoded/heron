<script lang="ts">
	import { formatDay, percent } from '#lib/format.ts';
	import { dayTone } from '#lib/status.ts';
	import type { Day } from '#lib/types.ts';

	let { days, compact = false }: { days: Day[]; compact?: boolean } = $props();

	let hovered = $state<number | null>(null);
	const visible = $derived(compact ? days.slice(-45) : days);
	const active = $derived(hovered === null ? null : visible[hovered]);
</script>

<div class="relative">
	<div class="flex h-8 items-stretch gap-[2px]" role="presentation" onmouseleave={() => (hovered = null)}>
		{#each visible as day, i (day.date)}
			<button
				type="button"
				aria-label="{formatDay(day.date)}: {percent(day.uptime)}"
				class="min-w-0 flex-1 rounded-[3px] transition-opacity {dayTone(day.uptime, day.degraded)} {hovered !== null && hovered !== i ? 'opacity-60' : ''}"
				onmouseenter={() => (hovered = i)}
				onfocus={() => (hovered = i)}
				onblur={() => (hovered = null)}
			></button>
		{/each}
	</div>

	{#if active && hovered !== null}
		<div
			class="pointer-events-none absolute bottom-full z-20 mb-2 w-52 -translate-x-1/2 rounded-lg border border-line bg-surface p-3 text-xs shadow-lg"
			style="left: clamp(6.5rem, {((hovered + 0.5) / visible.length) * 100}%, calc(100% - 6.5rem))"
		>
			<div class="font-medium text-ink">{formatDay(active.date)}</div>
			{#if active.uptime == null}
				<div class="mt-1 text-muted">No data</div>
			{:else}
				<div class="mt-1 flex justify-between text-muted">
					<span>Uptime</span>
					<span class="font-medium text-ink tabular-nums">{percent(active.uptime, 3)}</span>
				</div>
				<div class="flex justify-between text-muted">
					<span>Failed checks</span>
					<span class="tabular-nums">{active.down} of {active.checks}</span>
				</div>
				{#if active.degraded > 0}
					<div class="flex justify-between text-muted">
						<span>Degraded</span>
						<span class="tabular-nums">{active.degraded}</span>
					</div>
				{/if}
			{/if}
		</div>
	{/if}

	<div class="mt-2 flex justify-between text-xs text-faint">
		<span>{visible.length} days ago</span>
		<span>Today</span>
	</div>
</div>
