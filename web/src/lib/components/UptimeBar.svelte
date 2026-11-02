<script lang="ts">
	import { useI18n } from '#lib/i18n.svelte.ts';
	import { dayTone } from '#lib/status.ts';
	import type { Day } from '#lib/types.ts';

	let { days, uptime, compact = false }: { days: Day[]; uptime?: number; compact?: boolean } = $props();

	const i18n = useI18n();
	let hovered = $state<number | null>(null);
	const visible = $derived(compact ? days.slice(-45) : days);
	const active = $derived(hovered === null ? null : visible[hovered]);
</script>

<div class="relative">
	<div class="flex h-7 items-stretch gap-[2px]" dir="ltr" role="presentation" onmouseleave={() => (hovered = null)}>
		{#each visible as day, i (day.date)}
			<button
				type="button"
				aria-label="{i18n.day(day.date)}: {i18n.percent(day.uptime)}"
				class="min-w-0 flex-1 rounded-[1px] transition-opacity {dayTone(day.uptime, day.degraded)} {hovered !== null && hovered !== i ? 'opacity-50' : ''}"
				onmouseenter={() => (hovered = i)}
				onfocus={() => (hovered = i)}
				onblur={() => (hovered = null)}
			></button>
		{/each}
	</div>

	{#if active && hovered !== null}
		<div
			class="pointer-events-none absolute bottom-full z-20 mb-2 w-56 -translate-x-1/2 rounded-md border border-line bg-surface text-xs shadow-pop"
			style="left: clamp(7rem, {((hovered + 0.5) / visible.length) * 100}%, calc(100% - 7rem))"
		>
			<div class="border-b border-line px-3 py-2 font-semibold text-ink">{i18n.day(active.date)}</div>
			<div class="space-y-1 px-3 py-2">
				{#if active.uptime == null}
					<div class="text-muted">{i18n.t('bar.empty')}</div>
				{:else}
					<div class="flex justify-between text-muted">
						<span>{i18n.t('bar.uptime')}</span>
						<span class="font-semibold text-ink tabular-nums">{i18n.percent(active.uptime, 3)}</span>
					</div>
					<div class="flex justify-between text-muted">
						<span>{i18n.t('bar.failed')}</span>
						<span class="tabular-nums">{active.down} / {active.checks}</span>
					</div>
					{#if active.degraded > 0}
						<div class="flex justify-between text-muted">
							<span>{i18n.t('bar.degraded')}</span>
							<span class="tabular-nums">{active.degraded}</span>
						</div>
					{/if}
				{/if}
			</div>
		</div>
	{/if}

	<div class="mt-2 flex items-center gap-3 text-[11px] text-faint" dir="ltr">
		<span>{i18n.daysAgo(visible.length)}</span>
		<span class="h-px flex-1 bg-line"></span>
		{#if uptime !== undefined}
			<span class="font-medium text-muted tabular-nums">{i18n.t('bar.total', { percent: i18n.percent(uptime, 2) })}</span>
			<span class="h-px flex-1 bg-line"></span>
		{/if}
		<span>{i18n.t('week.today')}</span>
	</div>
</div>
