<script lang="ts">
	import { formatDay, percent } from '#lib/format.ts';
	import type { Day } from '#lib/types.ts';

	let { days, since }: { days: Day[]; since?: string } = $props();

	const levels = [
		{ min: 1, tone: 'bg-up', label: '100%' },
		{ min: 0.999, tone: 'bg-up/60', label: '≥ 99.9%' },
		{ min: 0.99, tone: 'bg-degraded', label: '≥ 99%' },
		{ min: 0.95, tone: 'bg-down/55', label: '≥ 95%' },
		{ min: 0, tone: 'bg-down', label: '< 95%' }
	];

	function tone(day: Day) {
		if (day.uptime == null) return 'bg-sunken ring-1 ring-inset ring-line';
		return (levels.find((l) => (day.uptime ?? 0) >= l.min) ?? levels[levels.length - 1]!).tone;
	}

	function parse(date: string) {
		const [y, m, d] = date.split('-').map(Number);
		return new Date(y ?? 1970, (m ?? 1) - 1, d ?? 1);
	}

	const weeks = $derived.by(() => {
		const first = days[0];
		if (!first) return [] as (Day | null)[][];
		const cells: (Day | null)[] = [...Array(parse(first.date).getDay()).fill(null), ...days];
		const out: (Day | null)[][] = [];
		for (let i = 0; i < cells.length; i += 7) out.push(cells.slice(i, i + 7));
		return out;
	});

	const months = $derived.by(() => {
		const labels: { column: number; label: string }[] = [];
		let last = -1;
		weeks.forEach((week, column) => {
			const day = week.find((d) => d !== null);
			if (!day) return;
			const month = parse(day.date).getMonth();
			if (month !== last) {
				if (column > 0 || parse(day.date).getDate() <= 7) {
					labels.push({ column, label: parse(day.date).toLocaleDateString('en', { month: 'short' }) });
				}
				last = month;
			}
		});
		return labels.filter((l, i) => i === labels.length - 1 || labels[i + 1]!.column - l.column >= 3);
	});

	const summary = $derived.by(() => {
		let checks = 0;
		let down = 0;
		let clean = 0;
		let tracked = 0;
		for (const d of days) {
			if (d.uptime == null) continue;
			tracked++;
			checks += d.checks;
			down += d.down;
			if (d.down === 0) clean++;
		}
		return { tracked, clean, uptime: checks ? (checks - down) / checks : null };
	});

	let hovered = $state<Day | null>(null);
	let scroller = $state<HTMLDivElement>();

	$effect(() => {
		if (scroller && weeks.length) scroller.scrollLeft = scroller.scrollWidth;
	});
</script>

<div>
	<div class="mb-3 flex flex-wrap items-baseline justify-between gap-2">
		<p class="text-[13px] text-muted">
			<span class="font-semibold text-ink tabular-nums">{percent(summary.uptime)}</span> uptime over the last year ·
			<span class="font-semibold text-ink tabular-nums">{summary.clean}</span> of {summary.tracked} tracked days without failures
		</p>
		{#if since}
			<p class="text-xs text-faint">Monitoring since {formatDay(since)}</p>
		{/if}
	</div>

	<div class="rounded-md border border-line p-3">
		<div class="overflow-x-auto" bind:this={scroller}>
			<div class="inline-grid grid-cols-[auto_1fr] gap-x-2">
				<div></div>
				<div class="relative h-4 text-[10px] text-muted">
					{#each months as m (m.column)}
						<span class="absolute" style="left: {m.column * 14}px">{m.label}</span>
					{/each}
				</div>

				<div class="grid grid-rows-7 gap-[3px] text-[10px] leading-[11px] text-muted">
					<span></span><span>Mon</span><span></span><span>Wed</span><span></span><span>Fri</span><span></span>
				</div>

				<div class="flex gap-[3px]" role="presentation" onmouseleave={() => (hovered = null)}>
					{#each weeks as week, w (w)}
						<div class="grid grid-rows-7 gap-[3px]">
							{#each week as day, d (d)}
								{#if day}
									<button
										type="button"
										class="size-[11px] rounded-[2px] {tone(day)} {hovered === day ? 'ring-2 ring-ink/60' : ''}"
										aria-label="{formatDay(day.date)}: {percent(day.uptime)}"
										onmouseenter={() => (hovered = day)}
										onfocus={() => (hovered = day)}
									></button>
								{:else}
									<span class="size-[11px]"></span>
								{/if}
							{/each}
						</div>
					{/each}
				</div>
			</div>
		</div>

		<div class="mt-3 flex flex-wrap items-center justify-between gap-3 text-[11px] text-muted">
			<span class="min-h-4">
				{#if hovered}
					<span class="font-medium text-ink">{formatDay(hovered.date)}</span>
					{#if hovered.uptime == null}
						· no data
					{:else}
						· {percent(hovered.uptime, 3)} · {hovered.down} failed of {hovered.checks} checks
					{/if}
				{:else}
					Hover a day for details
				{/if}
			</span>
			<span class="flex items-center gap-1.5">
				Worse
				{#each [...levels].reverse() as level (level.label)}
					<span class="size-[11px] rounded-[2px] {level.tone}" title={level.label}></span>
				{/each}
				Better
			</span>
		</div>
	</div>
</div>
