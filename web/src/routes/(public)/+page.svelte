<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import IncidentCard from '#lib/components/IncidentCard.svelte';
	import MaintenanceCard from '#lib/components/MaintenanceCard.svelte';
	import StatusBadge from '#lib/components/StatusBadge.svelte';
	import UptimeBar from '#lib/components/UptimeBar.svelte';
	import { formatTime, latency, percent } from '#lib/format.ts';
	import { Clock } from '#lib/live.svelte.ts';
	import { impactLabel, overallCopy, statusLabel, statusText } from '#lib/status.ts';
	import { getStatus } from '#lib/status-page.svelte.ts';
	import type { Group, Incident, Status } from '#lib/types.ts';
	import type { PageProps } from './$types';

	let { data: props }: PageProps = $props();

	const store = getStatus();
	const clock = new Clock(30_000);

	const data = $derived(store.data);
	const overall = $derived(data ? overallCopy[data.status] : null);
	const accent: Record<Status, string> = {
		up: 'bg-up',
		degraded: 'bg-degraded',
		down: 'bg-down',
		paused: 'bg-paused',
		pending: 'bg-pending',
		maintenance: 'bg-accent'
	};
	const tint: Record<Status, string> = {
		up: 'bg-up/12 text-up',
		degraded: 'bg-degraded/15 text-degraded',
		down: 'bg-down/12 text-down',
		paused: 'bg-sunken text-muted',
		pending: 'bg-sunken text-muted',
		maintenance: 'bg-accent/12 text-accent'
	};
	const icons = {
		up: 'check',
		degraded: 'alert',
		down: 'x',
		paused: 'pause',
		pending: 'info',
		maintenance: 'wrench'
	} as const;
	const severity: Status[] = ['down', 'degraded', 'maintenance', 'pending', 'paused', 'up'];

	const monitors = $derived(data?.groups.flatMap((g) => g.monitors) ?? []);
	const average = $derived(
		monitors.length ? monitors.reduce((sum, m) => sum + m.uptime.history, 0) / monitors.length : null
	);
	const historyDays = $derived(monitors[0]?.days.length ?? data?.page.history_days ?? 90);

	function worst(group: Group): Status {
		const statuses = group.monitors.map((m) => m.status);
		return severity.find((s) => statuses.includes(s)) ?? 'up';
	}

	const pastDays = $derived.by(() => {
		void clock.now;
		const days: { key: string; label: string; incidents: Incident[] }[] = [];
		const today = new Date();
		for (let i = 0; i < 7; i++) {
			const d = new Date(today.getFullYear(), today.getMonth(), today.getDate() - i);
			const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
			days.push({
				key,
				label: d.toLocaleDateString('en', { month: 'short', day: 'numeric', year: 'numeric' }),
				incidents: props.recent.filter((inc) => {
					const s = new Date(inc.started_at);
					return s.getFullYear() === d.getFullYear() && s.getMonth() === d.getMonth() && s.getDate() === d.getDate();
				})
			});
		}
		return days;
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
		<section class="card relative overflow-hidden">
			<span class="absolute inset-y-0 left-0 w-1 {accent[overall.tone]}"></span>
			<div class="flex flex-wrap items-center justify-between gap-x-8 gap-y-4 py-5 pr-6 pl-7">
				<div class="flex items-center gap-4">
					<span class="grid size-11 shrink-0 place-items-center rounded-full {tint[overall.tone]}">
						<Icon name={icons[overall.tone]} size={20} />
					</span>
					<div>
						<h1 class="text-lg font-semibold tracking-tight">{overall.title}</h1>
						<p class="text-muted">{data.page.description || overall.detail}</p>
					</div>
				</div>
				<dl class="flex gap-8 text-right">
					<div>
						<dt class="eyebrow">Uptime · {historyDays}d</dt>
						<dd class="mt-1 text-lg font-semibold tabular-nums">{percent(average)}</dd>
					</div>
					<div>
						<dt class="eyebrow">Last updated</dt>
						<dd class="mt-1 text-lg font-semibold tabular-nums">{formatTime(data.updated_at)}</dd>
					</div>
				</dl>
			</div>
		</section>
	{/if}

	{#if data.incidents.length > 0}
		<section class="mt-10">
			<h2 class="eyebrow mb-3">Active incidents</h2>
			<div class="space-y-4">
				{#each data.incidents as incident (incident.id)}
					<IncidentCard {incident} href="/incidents/{incident.id}" />
				{/each}
			</div>
		</section>
	{/if}

	{#if data.maintenance.length > 0}
		<section class="mt-10">
			<h2 class="eyebrow mb-3">Scheduled maintenance</h2>
			<div class="space-y-4">
				{#each data.maintenance as window (window.id)}
					<MaintenanceCard {window} now={clock.now} />
				{/each}
			</div>
		</section>
	{/if}

	<section class="mt-10">
		<div class="mb-3 flex flex-wrap items-end justify-between gap-3">
			<h2 class="eyebrow">Services</h2>
			<div class="flex flex-wrap items-center gap-4 text-[11px] text-muted">
				<span class="flex items-center gap-1.5"><span class="size-2 rounded-[2px] bg-up"></span>Operational</span>
				<span class="flex items-center gap-1.5"><span class="size-2 rounded-[2px] bg-degraded"></span>Degraded</span>
				<span class="flex items-center gap-1.5"><span class="size-2 rounded-[2px] bg-down"></span>Outage</span>
				<span class="flex items-center gap-1.5"><span class="size-2 rounded-[2px] bg-pending"></span>No data</span>
			</div>
		</div>

		<div class="space-y-4">
			{#each data.groups as group (group.name)}
				<div class="card overflow-hidden">
					{#if group.name}
						<div class="flex items-center justify-between border-b border-line bg-sunken/70 px-5 py-2.5">
							<h3 class="text-[13px] font-semibold">{group.name}</h3>
							<span class="text-xs font-medium {statusText[worst(group)]}">
								{group.monitors.length}
								{group.monitors.length === 1 ? 'service' : 'services'} · {statusLabel[worst(group)]}
							</span>
						</div>
					{/if}
					<ul class="divide-y divide-line">
						{#each group.monitors as m (m.slug)}
							<li class="px-5 py-4">
								<div class="mb-3 flex items-center justify-between gap-4">
									<div class="flex min-w-0 items-center gap-2">
										<a href="/monitor/{m.slug}" class="truncate font-medium hover:text-accent">{m.name}</a>
										{#if m.description}
											<span class="text-faint" title={m.description}><Icon name="info" size={13} /></span>
										{/if}
									</div>
									<div class="flex shrink-0 items-center gap-3">
										{#if m.status !== 'down' && m.latency_ms > 0}
											<span class="hidden font-mono text-xs text-faint sm:inline">{latency(m.latency_ms)}</span>
										{/if}
										<StatusBadge status={m.status} />
									</div>
								</div>
								<div class="hidden sm:block"><UptimeBar days={m.days} uptime={m.uptime.history} /></div>
								<div class="sm:hidden"><UptimeBar days={m.days} uptime={m.uptime.history} compact /></div>
							</li>
						{/each}
					</ul>
				</div>
			{:else}
				<div class="card p-10 text-center text-muted">No services are being monitored yet.</div>
			{/each}
		</div>
	</section>

	<section class="mt-12">
		<div class="mb-3 flex items-end justify-between">
			<h2 class="eyebrow">Past incidents</h2>
			<a href="/incidents" class="text-xs font-medium text-accent hover:underline">Full history</a>
		</div>
		<div class="card divide-y divide-line">
			{#each pastDays as day (day.key)}
				<div class="px-5 py-4">
					<h3 class="text-[13px] font-semibold">{day.label}</h3>
					{#each day.incidents as incident (incident.id)}
						<a href="/incidents/{incident.id}" class="group mt-2 block">
							<span class="font-medium group-hover:text-accent {incident.impact === 'maintenance' ? 'text-accent' : incident.impact === 'minor' ? 'text-degraded' : 'text-down'}">
								{incident.title}
							</span>
							<span class="ml-1.5 text-xs text-faint">{impactLabel[incident.impact]}</span>
							{#if incident.updates[0]}
								<p class="mt-0.5 line-clamp-2 text-muted">{incident.updates[0].body}</p>
							{/if}
						</a>
					{:else}
						<p class="mt-1 text-muted">No incidents reported.</p>
					{/each}
				</div>
			{/each}
		</div>
	</section>
{/if}
