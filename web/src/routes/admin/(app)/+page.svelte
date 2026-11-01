<script lang="ts">
	import { untrack } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import StatusBadge from '#lib/components/StatusBadge.svelte';
	import { ago, displayTarget, interval, latency, percent } from '#lib/format.ts';
	import { Clock, debounce, Live } from '#lib/live.svelte.ts';
	import { kindLabel, statusLabel } from '#lib/status.ts';
	import type { AdminMonitor, Status } from '#lib/types.ts';
	import { invalidate } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let monitors = $state<AdminMonitor[]>(untrack(() => data.monitors));
	$effect.pre(() => {
		monitors = data.monitors;
	});

	let query = $state('');
	let filter = $state<Status | 'all'>('all');
	const clock = new Clock(10_000);

	const counts = $derived(
		monitors.reduce<Record<Status, number>>(
			(acc, m) => {
				acc[m.status]++;
				return acc;
			},
			{ up: 0, degraded: 0, down: 0, pending: 0, paused: 0, maintenance: 0 }
		)
	);

	const measured = $derived(monitors.filter((m) => m.uptime_24h != null));
	const fleetUptime = $derived(
		measured.length ? measured.reduce((s, m) => s + (m.uptime_24h ?? 0), 0) / measured.length : null
	);

	const visible = $derived(
		monitors.filter((m) => {
			if (filter !== 'all' && m.status !== filter) return false;
			const q = query.trim().toLowerCase();
			return (
				!q ||
				m.name.toLowerCase().includes(q) ||
				m.target.toLowerCase().includes(q) ||
				m.group.toLowerCase().includes(q)
			);
		})
	);

	const refresh = debounce(() => invalidate('heron:monitors'), 500);
	new Live(
		{
			monitor: (event) => {
				const m = monitors.find((m) => m.id === event.id);
				if (!m) return refresh();
				m.status = event.status;
				m.latency_ms = event.latency_ms ?? 0;
				m.checked_at = event.at;
				m.message = event.message ?? '';
			},
			layout: refresh
		},
		true
	);

	const kpis: { status: Status; label: string; tone: string }[] = [
		{ status: 'up', label: 'Operational', tone: 'text-up' },
		{ status: 'degraded', label: 'Degraded', tone: 'text-degraded' },
		{ status: 'down', label: 'Down', tone: 'text-down' },
		{ status: 'paused', label: 'Paused', tone: 'text-muted' }
	];

	const uptimeTone = (u: number | undefined) =>
		u == null ? 'bg-pending' : u >= 0.999 ? 'bg-up' : u >= 0.98 ? 'bg-degraded' : 'bg-down';
</script>

<PageHeader title="Monitors" description="Health checks running against your infrastructure.">
	{#snippet actions()}
		<a href="/admin/monitors/new" class="btn-primary"><Icon name="plus" size={14} /> New monitor</a>
	{/snippet}
</PageHeader>

<section class="card mb-6 grid grid-cols-2 divide-line sm:grid-cols-3 lg:grid-cols-6 lg:divide-x">
	<div class="px-5 py-4">
		<p class="eyebrow">Total</p>
		<p class="mt-1.5 text-2xl font-semibold tabular-nums">{monitors.length}</p>
	</div>
	{#each kpis as k (k.status)}
		<button
			type="button"
			class="px-5 py-4 text-left transition hover:bg-sunken/70 {filter === k.status ? 'bg-sunken' : ''}"
			onclick={() => (filter = filter === k.status ? 'all' : k.status)}
			aria-pressed={filter === k.status}
		>
			<p class="eyebrow">{k.label}</p>
			<p class="mt-1.5 text-2xl font-semibold tabular-nums {counts[k.status] > 0 ? k.tone : 'text-faint'}">{counts[k.status]}</p>
		</button>
	{/each}
	<div class="px-5 py-4">
		<p class="eyebrow">Uptime · 24h</p>
		<p class="mt-1.5 text-2xl font-semibold tabular-nums">{percent(fleetUptime)}</p>
	</div>
</section>

{#if monitors.length === 0}
	<div class="card flex flex-col items-center px-6 py-16 text-center">
		<span class="grid size-12 place-items-center rounded-full bg-sunken text-muted"><Icon name="activity" size={22} /></span>
		<h2 class="mt-4 text-[15px] font-semibold">No monitors configured</h2>
		<p class="mt-1 max-w-sm text-muted">Add a website, API, server or scheduled job and heron will start checking it immediately.</p>
		<a href="/admin/monitors/new" class="btn-primary mt-6"><Icon name="plus" size={14} /> Create monitor</a>
	</div>
{:else}
	<div class="card overflow-hidden">
		<div class="flex flex-wrap items-center gap-3 border-b border-line px-4 py-3">
			<div class="relative w-full max-w-xs">
				<input class="input pl-8" type="search" placeholder="Filter by name, target or group" bind:value={query} />
				<span class="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-faint"><Icon name="search" size={13} /></span>
			</div>
			{#if filter !== 'all'}
				<button class="btn-secondary btn-sm" onclick={() => (filter = 'all')}>
					Status: {statusLabel[filter]} <Icon name="x" size={11} />
				</button>
			{/if}
			<span class="ml-auto text-xs text-muted">{visible.length} of {monitors.length}</span>
		</div>

		<div class="overflow-x-auto">
			<table class="w-full">
				<thead class="border-b border-line bg-sunken/70">
					<tr>
						<th class="th w-32">Status</th>
						<th class="th">Monitor</th>
						<th class="th hidden md:table-cell">Type</th>
						<th class="th hidden xl:table-cell">Group</th>
						<th class="th w-40">Uptime 24h</th>
						<th class="th hidden text-right sm:table-cell">Response</th>
						<th class="th hidden text-right lg:table-cell">Last check</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each visible as m (m.id)}
						<tr class="relative transition hover:bg-sunken/60">
							<td class="px-4 py-3"><StatusBadge status={m.status} /></td>
							<td class="min-w-64 px-4 py-3">
								<a href="/admin/monitors/{m.id}" class="block after:absolute after:inset-0">
									<span class="flex items-center gap-2 font-medium">
										<span class="truncate">{m.name}</span>
										{#if !m.public}
											<span class="chip text-faint ring-line-strong">Private</span>
										{/if}
									</span>
									<span class="block max-w-[22rem] truncate font-mono text-xs text-muted">
										{m.kind === 'push' ? `heartbeat · every ${interval(m.interval_s)}` : displayTarget(m.target)}
									</span>
								</a>
							</td>
							<td class="hidden px-4 py-3 whitespace-nowrap md:table-cell">
								<span class="rounded border border-line px-1.5 py-0.5 font-mono text-[11px] text-muted">{kindLabel[m.kind]}</span>
								<span class="ml-1.5 text-xs text-faint">{interval(m.interval_s)}</span>
							</td>
							<td class="hidden px-4 py-3 text-muted xl:table-cell">{m.group || '—'}</td>
							<td class="px-4 py-3">
								<div class="flex items-center gap-2.5">
									<span class="h-1.5 w-16 overflow-hidden rounded-full bg-sunken">
										<span class="block h-full {uptimeTone(m.uptime_24h)}" style="width: {(m.uptime_24h ?? 0) * 100}%"></span>
									</span>
									<span class="text-xs font-medium tabular-nums">{percent(m.uptime_24h)}</span>
								</div>
							</td>
							<td class="hidden px-4 py-3 text-right font-mono text-xs text-muted sm:table-cell">
								{m.status === 'down' || m.status === 'paused' ? '—' : latency(m.latency_ms)}
							</td>
							<td class="hidden px-4 py-3 text-right text-xs whitespace-nowrap text-muted lg:table-cell">{ago(m.checked_at, clock.now)}</td>
						</tr>
					{:else}
						<tr><td colspan="7" class="px-4 py-10 text-center text-muted">No monitors match the current filter.</td></tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{/if}
