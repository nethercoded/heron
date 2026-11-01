<script lang="ts">
	import HeronField from '#lib/components/HeronField.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import IncidentCard from '#lib/components/IncidentCard.svelte';
	import MaintenanceCard from '#lib/components/MaintenanceCard.svelte';
	import UptimeBar from '#lib/components/UptimeBar.svelte';
	import { formatTime, latency, percent } from '#lib/format.ts';
	import { Clock } from '#lib/live.svelte.ts';
	import { impactLabel, overallCopy, statusLabel } from '#lib/status.ts';
	import { getStatus } from '#lib/status-page.svelte.ts';
	import type { Group, Incident, Status } from '#lib/types.ts';
	import type { PageProps } from './$types';

	let { data: props }: PageProps = $props();

	const store = getStatus();
	const clock = new Clock(30_000);

	const data = $derived(store.data);
	const overall = $derived(data ? overallCopy[data.status] : null);

	const color: Record<Status, string> = {
		up: 'var(--color-up)',
		degraded: 'var(--color-degraded)',
		down: 'var(--color-down)',
		maintenance: 'var(--color-accent)',
		pending: 'var(--color-faint)',
		paused: 'var(--color-faint)'
	};
	const water: Record<Status, string> = {
		up: '#4cc3f5',
		degraded: '#f5c53b',
		down: '#ff4d4f',
		maintenance: '#4cc3f5',
		pending: '#4cc3f5',
		paused: '#4cc3f5'
	};
	const word: Record<Status, string> = {
		up: 'Operational',
		degraded: 'Degraded',
		down: 'Outage',
		maintenance: 'Maintenance',
		pending: 'Waiting',
		paused: 'Paused'
	};
	const severity: Status[] = ['down', 'degraded', 'maintenance', 'pending', 'paused', 'up'];

	const monitors = $derived(data?.groups.flatMap((g) => g.monitors) ?? []);
	const average = $derived(
		monitors.length ? monitors.reduce((sum, m) => sum + m.uptime.history, 0) / monitors.length : null
	);
	const healthy = $derived(monitors.filter((m) => m.status === 'up' || m.status === 'degraded').length);
	const historyDays = $derived(monitors[0]?.days.length ?? data?.page.history_days ?? 90);
	const uptimeDigits = $derived(
		average == null ? '--.--' : (Math.floor(Math.min(average, 0.9999) * 10000) / 100).toFixed(2)
	);

	function worst(group: Group): Status {
		const statuses = group.monitors.map((m) => m.status);
		return severity.find((s) => statuses.includes(s)) ?? 'up';
	}

	const pastDays = $derived.by(() => {
		void clock.now;
		const today = new Date();
		return Array.from({ length: 7 }, (_, i) => {
			const d = new Date(today.getFullYear(), today.getMonth(), today.getDate() - i);
			return {
				key: d.toDateString(),
				label: d.toLocaleDateString('en', { month: 'short', day: 'numeric' }),
				weekday: i === 0 ? 'Today' : d.toLocaleDateString('en', { weekday: 'short' }),
				incidents: props.recent.filter((inc: Incident) => new Date(inc.started_at).toDateString() === d.toDateString())
			};
		});
	});
</script>

{#if !data}
	<div class="card p-10 text-center">
		<p class="font-semibold">Status is unavailable right now</p>
		<p class="mt-1 text-muted">{store.error ?? 'Please try again in a moment.'}</p>
		<button class="btn-secondary mt-5" onclick={() => store.load()}><Icon name="refresh" /> Retry</button>
	</div>
{:else}
	{#if overall}
		<section class="relative isolate overflow-hidden rounded-card bg-surface">
			<HeronField water={water[overall.tone]} class="absolute inset-0 -z-10 size-full" />
			<div class="flex min-h-[340px] flex-col justify-between gap-10 p-6 sm:min-h-[380px] sm:p-9">
				<div class="max-w-md">
					<p class="flex items-center gap-2.5 font-mono text-xs tracking-wider uppercase" style="color: {color[overall.tone]}">
						<span class="sq blink" style="--c: {color[overall.tone]}; --s: 9px"></span>
						{word[overall.tone]}
					</p>
					<h1 class="mt-4 text-4xl leading-[1.05] font-semibold tracking-tight text-balance sm:text-5xl">{overall.title}</h1>
					<p class="mt-3 text-ink/60">{data.page.description || overall.detail}</p>
				</div>

				<dl class="flex flex-wrap items-end gap-x-10 gap-y-5">
					<div>
						<dt class="mb-2 text-xs text-muted">Uptime, {historyDays} days</dt>
						<dd class="flex items-baseline gap-1 text-3xl font-semibold">
							<span class="flap">{#each uptimeDigits.split('') as ch, i (i)}<i>{ch}</i>{/each}</span>
							<span class="text-xl text-muted">%</span>
						</dd>
					</div>
					<div>
						<dt class="mb-2 text-xs text-muted">Services online</dt>
						<dd class="font-mono text-3xl font-semibold">{healthy}<span class="text-muted">/{monitors.length}</span></dd>
					</div>
					<div>
						<dt class="mb-2 text-xs text-muted">Checked</dt>
						<dd class="font-mono text-3xl font-semibold">{formatTime(data.updated_at)}</dd>
					</div>
				</dl>
			</div>
		</section>
	{/if}

	{#if data.incidents.length > 0}
		<section class="mt-12">
			<h2 class="mb-4 flex items-center gap-2.5 text-sm font-medium text-muted">
				<span class="sq blink" style="--c: var(--color-down); --s: 8px"></span> Happening now
			</h2>
			<div class="space-y-3">
				{#each data.incidents as incident (incident.id)}
					<IncidentCard {incident} href="/incidents/{incident.id}" />
				{/each}
			</div>
		</section>
	{/if}

	{#if data.maintenance.length > 0}
		<section class="mt-12">
			<h2 class="mb-4 flex items-center gap-2.5 text-sm font-medium text-muted">
				<span class="sq" style="--c: var(--color-accent); --s: 8px"></span> Planned work
			</h2>
			<div class="space-y-3">
				{#each data.maintenance as window (window.id)}
					<MaintenanceCard {window} now={clock.now} />
				{/each}
			</div>
		</section>
	{/if}

	<section class="mt-12 space-y-8">
		{#each data.groups as group (group.name)}
			{@const state = worst(group)}
			<div>
				<div class="mb-3 flex items-baseline justify-between gap-4 px-1">
					<h2 class="text-lg font-semibold tracking-tight">{group.name || 'Services'}</h2>
					<span class="flex items-center gap-2 font-mono text-[11px] tracking-wider uppercase" style="color: {color[state]}">
						<span class="sq" style="--c: {color[state]}; --s: 7px"></span>
						{state === 'up' ? 'All operational' : statusLabel[state]}
					</span>
				</div>
				<ul class="space-y-1 rounded-card bg-surface p-2">
					{#each group.monitors as m (m.slug)}
						<li>
							<a href="/monitor/{m.slug}" class="group block rounded-xl px-4 py-3.5 transition hover:bg-sunken">
								<div class="mb-3 flex items-center justify-between gap-4">
									<div class="flex min-w-0 items-center gap-3">
										<span class="sq" style="--c: {color[m.status]}; --s: 10px"></span>
										<span class="truncate font-medium">{m.name}</span>
										{#if m.description}
											<span class="hidden truncate text-sm text-faint md:inline">{m.description}</span>
										{/if}
									</div>
									<div class="flex shrink-0 items-center gap-4 font-mono text-xs">
										{#if m.status !== 'down' && m.latency_ms > 0}
											<span class="hidden text-faint sm:inline">{latency(m.latency_ms)}</span>
										{/if}
										<span class="text-muted">{percent(m.uptime.history)}</span>
										<span class="w-24 text-right tracking-wider uppercase" style="color: {color[m.status]}">{word[m.status]}</span>
									</div>
								</div>
								<div class="hidden sm:block"><UptimeBar days={m.days} /></div>
								<div class="sm:hidden"><UptimeBar days={m.days} compact /></div>
							</a>
						</li>
					{/each}
				</ul>
			</div>
		{:else}
			<div class="rounded-card bg-surface p-10 text-center text-muted">No services are being monitored yet.</div>
		{/each}
	</section>

	<section class="mt-14">
		<div class="mb-4 flex items-baseline justify-between px-1">
			<h2 class="text-lg font-semibold tracking-tight">Last 7 days</h2>
			<a href="/incidents" class="flex items-center gap-1 text-sm text-muted hover:text-ink">All history <Icon name="chevron" size={13} /></a>
		</div>
		<ol class="rounded-card bg-surface p-2">
			{#each pastDays as day (day.key)}
				<li class="grid grid-cols-[5.5rem_1fr] gap-4 rounded-xl px-4 py-3">
					<div class="font-mono text-xs leading-5">
						<span class="block text-ink">{day.label}</span>
						<span class="text-faint">{day.weekday}</span>
					</div>
					<div class="min-w-0">
						{#each day.incidents as incident (incident.id)}
							<a href="/incidents/{incident.id}" class="group mb-2 block last:mb-0">
								<span class="flex items-center gap-2 font-medium group-hover:underline">
									<span
										class="sq"
										style="--c: var(--color-{incident.impact === 'maintenance' ? 'accent' : incident.impact === 'minor' ? 'degraded' : 'down'}); --s: 8px"
									></span>
									{incident.title}
									<span class="font-mono text-[11px] tracking-wider text-faint uppercase">{impactLabel[incident.impact]}</span>
								</span>
								{#if incident.updates[0]}
									<p class="mt-0.5 line-clamp-1 pl-4 text-sm text-muted">{incident.updates[0].body}</p>
								{/if}
							</a>
						{:else}
							<p class="text-sm leading-10 text-faint">No incidents</p>
						{/each}
					</div>
				</li>
			{/each}
		</ol>
	</section>
{/if}
