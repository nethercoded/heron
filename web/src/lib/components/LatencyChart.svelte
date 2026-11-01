<script lang="ts">
	import { formatDateTime, latency } from '#lib/format.ts';
	import type { Point } from '#lib/types.ts';

	let { points, height = 180 }: { points: Point[]; height?: number } = $props();

	let width = $state(600);
	let hovered = $state<number | null>(null);

	const pad = { top: 12, right: 8, bottom: 22, left: 44 };

	const measured = $derived(points.filter((p) => p.avg_ms > 0));
	const peak = $derived(Math.max(...measured.map((p) => p.avg_ms), 1));
	const ceiling = $derived(niceCeil(peak * 1.15));
	const first = $derived(points[0]);
	const last = $derived(points.at(-1));
	const start = $derived(first ? new Date(first.at).getTime() : 0);
	const end = $derived(last ? new Date(last.at).getTime() : 1);

	const x = (t: string) =>
		pad.left + ((new Date(t).getTime() - start) / Math.max(end - start, 1)) * (width - pad.left - pad.right);
	const y = (ms: number) => pad.top + (1 - ms / ceiling) * (height - pad.top - pad.bottom);

	const line = $derived(
		measured.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(p.at).toFixed(1)},${y(p.avg_ms).toFixed(1)}`).join('')
	);
	const area = $derived.by(() => {
		const head = measured[0];
		const tail = measured.at(-1);
		if (!head || !tail) return '';
		return `${line}L${x(tail.at).toFixed(1)},${y(0)}L${x(head.at).toFixed(1)},${y(0)}Z`;
	});
	const ticks = $derived([0, 0.5, 1].map((f) => Math.round(ceiling * f)));
	const outages = $derived(points.filter((p) => p.down > 0));
	const active = $derived(hovered === null ? null : points[hovered]);

	function niceCeil(v: number) {
		const magnitude = 10 ** Math.floor(Math.log10(v));
		const step = [1, 2, 2.5, 5, 10].find((s) => s * magnitude >= v) ?? 10;
		return step * magnitude;
	}

	function track(event: PointerEvent) {
		const rect = (event.currentTarget as SVGElement).getBoundingClientRect();
		const px = event.clientX - rect.left;
		let best = 0;
		let distance = Infinity;
		points.forEach((p, i) => {
			const d = Math.abs(x(p.at) - px);
			if (d < distance) {
				distance = d;
				best = i;
			}
		});
		hovered = points.length ? best : null;
	}
</script>

<div class="relative" bind:clientWidth={width}>
	{#if points.length < 2}
		<div class="grid place-items-center text-sm text-muted" style="height: {height}px">Not enough data yet</div>
	{:else}
		<svg
			{width}
			{height}
			class="block touch-none select-none"
			role="img"
			aria-label="Response time chart"
			onpointermove={track}
			onpointerleave={() => (hovered = null)}
		>
			<defs>
				<linearGradient id="latency-fill" x1="0" x2="0" y1="0" y2="1">
					<stop offset="0%" stop-color="var(--color-accent)" stop-opacity="0.28" />
					<stop offset="100%" stop-color="var(--color-accent)" stop-opacity="0" />
				</linearGradient>
			</defs>

			{#each ticks as tick (tick)}
				<line x1={pad.left} x2={width - pad.right} y1={y(tick)} y2={y(tick)} stroke="var(--color-line)" stroke-dasharray={tick ? '3 4' : ''} />
				<text x={pad.left - 8} y={y(tick)} dy="0.32em" text-anchor="end" class="fill-faint text-[10px] tabular-nums">
					{tick >= 1000 ? `${tick / 1000}s` : `${tick}ms`}
				</text>
			{/each}

			{#each outages as p (p.at)}
				<rect x={x(p.at) - 1.5} y={pad.top} width="3" height={height - pad.top - pad.bottom} class="fill-down/25" />
			{/each}

			<path d={area} fill="url(#latency-fill)" />
			<path d={line} fill="none" stroke="var(--color-accent)" stroke-width="1.75" stroke-linejoin="round" />

			{#if active}
				<line x1={x(active.at)} x2={x(active.at)} y1={pad.top} y2={height - pad.bottom} stroke="var(--color-faint)" stroke-dasharray="2 3" />
				{#if active.avg_ms > 0}
					<circle cx={x(active.at)} cy={y(active.avg_ms)} r="4" fill="var(--color-surface)" stroke="var(--color-accent)" stroke-width="2" />
				{/if}
			{/if}

			{#if first && last}
				<text x={pad.left} y={height - 4} class="fill-faint text-[10px]">{formatDateTime(first.at)}</text>
				<text x={width - pad.right} y={height - 4} text-anchor="end" class="fill-faint text-[10px]">{formatDateTime(last.at)}</text>
			{/if}
		</svg>

		{#if active}
			<div
				class="pointer-events-none absolute top-0 z-10 w-44 -translate-x-1/2 rounded-lg border border-line bg-surface p-2.5 text-xs shadow-lg"
				style="left: clamp(5.5rem, {x(active.at)}px, calc(100% - 5.5rem))"
			>
				<div class="font-medium text-ink">{formatDateTime(active.at)}</div>
				<div class="mt-1 flex justify-between text-muted"><span>Average</span><span class="text-ink tabular-nums">{latency(active.avg_ms)}</span></div>
				<div class="flex justify-between text-muted"><span>Peak</span><span class="tabular-nums">{latency(active.max_ms)}</span></div>
				{#if active.down > 0}
					<div class="flex justify-between text-down"><span>Failed</span><span class="tabular-nums">{active.down} of {active.checks}</span></div>
				{/if}
			</div>
		{/if}
	{/if}
</div>
