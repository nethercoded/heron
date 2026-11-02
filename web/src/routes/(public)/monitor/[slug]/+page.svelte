<script lang="ts">
	import { untrack } from 'svelte';
	import { api } from '#lib/api.ts';
	import BeatStrip from '#lib/components/BeatStrip.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import IncidentCard from '#lib/components/IncidentCard.svelte';
	import LatencyChart from '#lib/components/LatencyChart.svelte';
	import RangeTabs from '#lib/components/RangeTabs.svelte';
	import StatusBadge from '#lib/components/StatusBadge.svelte';
	import UptimeBar from '#lib/components/UptimeBar.svelte';
	import UptimeCalendar from '#lib/components/UptimeCalendar.svelte';
	import { useI18n } from '#lib/i18n.svelte.ts';
	import { Clock, Live } from '#lib/live.svelte.ts';
	import { kindLabel } from '#lib/status.ts';
	import type { Beat, Range, Series } from '#lib/types.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const i18n = useI18n();

	let detail = $state(untrack(() => data.detail));
	$effect.pre(() => {
		detail = data.detail;
	});

	let range = $state<Range>('24h');
	let series = $state<Series | null>(null);
	const clock = new Clock();

	const m = $derived(detail.monitor);
	const stats = $derived([
		{ key: 'day', label: i18n.t('last.hours', { n: 24 }), value: m.uptime.day },
		{ key: 'week', label: i18n.t('last.days', { n: 7 }), value: m.uptime.week },
		{ key: 'month', label: i18n.t('last.days', { n: 30 }), value: m.uptime.month },
		{ key: 'history', label: i18n.t('last.days', { n: m.days.length }), value: m.uptime.history }
	]);
	const average = $derived.by(() => {
		const measured = series?.points.filter((p) => p.avg_ms > 0) ?? [];
		if (!measured.length) return 0;
		return measured.reduce((sum, p) => sum + p.avg_ms * p.checks, 0) / measured.reduce((sum, p) => sum + p.checks, 0);
	});

	$effect(() => {
		const slug = m.slug;
		const selected = range;
		let cancelled = false;
		api.get<Series>(`/monitors/${slug}/series?range=${selected}`).then((s) => {
			if (!cancelled) series = s;
		});
		return () => {
			cancelled = true;
		};
	});

	new Live({
		monitor: (event) => {
			if (event.slug !== m.slug) return;
			const beat: Beat = {
				at: event.at ?? new Date().toISOString(),
				status: event.status,
				latency_ms: event.latency_ms ?? 0
			};
			if (m.status !== event.status) m.since = beat.at;
			m.status = event.status;
			m.latency_ms = beat.latency_ms;
			m.checked_at = beat.at;
			detail.beats = [...detail.beats.slice(-59), beat];
		}
	});
</script>

<svelte:head>
	<title>{m.name} · {i18n.t('nav.status')}</title>
</svelte:head>

<a href="/" class="mb-6 inline-flex items-center gap-1 text-sm text-muted hover:text-ink">
	<Icon name="back" size={14} class="rtl:-scale-x-100" /> {i18n.t('monitor.back')}
</a>

<header class="flex flex-wrap items-end justify-between gap-4">
	<div class="min-w-0">
		<p class="text-xs font-medium tracking-wide text-faint uppercase">{m.group || kindLabel[m.kind]}</p>
		<h1 class="mt-1 text-2xl font-semibold tracking-tight">{m.name}</h1>
		{#if m.description}
			<p class="mt-1 text-muted" dir="auto">{m.description}</p>
		{/if}
	</div>
	<div class="text-end">
		<StatusBadge status={m.status} />
		{#if m.since}
			<p class="mt-1 text-xs text-faint">{i18n.t('monitor.for', { time: i18n.between(m.since, new Date(clock.now).toISOString()) })}</p>
		{/if}
	</div>
</header>

<section class="mt-8 grid grid-cols-2 gap-3 sm:grid-cols-4">
	{#each stats as stat (stat.key)}
		<div class="card px-4 py-3">
			<p class="text-xs text-muted">{stat.label}</p>
			<p class="mt-1 text-xl font-semibold tabular-nums">{i18n.percent(stat.value)}</p>
		</div>
	{/each}
</section>

<section class="card mt-6 p-5">
	<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
		<div>
			<h2 class="font-semibold">{i18n.t('monitor.response')}</h2>
			<p class="text-xs text-muted">
				{i18n.t('monitor.average', { latency: i18n.latency(average), ago: i18n.ago(m.checked_at, clock.now) })}
			</p>
		</div>
		<RangeTabs bind:value={range} />
	</div>
	<LatencyChart points={series?.points ?? []} />
</section>

<section class="card mt-6 p-5">
	<div class="mb-3 flex items-center justify-between">
		<h2 class="font-semibold">{i18n.t('monitor.recent')}</h2>
		<span class="text-xs text-muted">{i18n.t('monitor.latest', { latency: i18n.latency(m.latency_ms) })}</span>
	</div>
	<BeatStrip beats={detail.beats} />
</section>

<section class="card mt-6 p-5">
	<h2 class="mb-4 font-semibold">{i18n.t('monitor.history')}</h2>
	<UptimeBar days={m.days} uptime={m.uptime.history} />
</section>

<section class="card mt-6 p-5">
	<h2 class="mb-4 font-semibold">{i18n.t('monitor.calendar')}</h2>
	<UptimeCalendar days={data.calendar.days} since={data.calendar.since} />
</section>

<section class="mt-10">
	<h2 class="mb-4 text-sm font-semibold tracking-wide text-muted uppercase">{i18n.t('monitor.incidents')}</h2>
	<div class="space-y-4">
		{#each detail.incidents as incident (incident.id)}
			<IncidentCard {incident} href="/incidents/{incident.id}" />
		{:else}
			<p class="card p-6 text-center text-sm text-muted">{i18n.t('monitor.quiet', { n: m.days.length })}</p>
		{/each}
	</div>
</section>
