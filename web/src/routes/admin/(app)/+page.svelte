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

	const kpis: { status: Status; label: string; color: string }[] = [
		{ status: 'up', label: 'Operational', color: 'var(--color-up)' },
		{ status: 'degraded', label: 'Degraded', color: 'var(--color-degraded)' },
		{ status: 'down', label: 'Down', color: 'var(--color-down)' },
		{ status: 'paused', label: 'Paused', color: 'var(--color-paused)' }
	];

	const uptimeTone = (u: number | undefined) =>
		u == null ? 'bg-pending' : u >= 0.999 ? 'bg-up' : u >= 0.98 ? 'bg-degraded' : 'bg-down';
</script>

<PageHeader title="Monitors">
	{#snippet meta()}
		<span class="flex items-center gap-2"><span class="sq" style="--c: var(--color-up); --s: 8px"></span>{counts.up} operational</span>
		{#if counts.down > 0}
			<span class="flex items-center gap-2 text-down"><span class="sq blink" style="--c: var(--color-down); --s: 8px"></span>{counts.down} down</span>
		{/if}
		<span class="text-faint">{percent(fleetUptime)} uptime in 24h</span>
	{/snippet}
	{#snippet actions()}
		<a href="/admin/monitors/new" class="btn-primary"><Icon name="plus" /> New monitor</a>
	{/snippet}
</PageHeader>

<section class="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-5">
	<div class="rounded-card bg-surface px-5 py-4">
		<p class="text-[13px] font-medium text-ink-2">Total</p>
		<p class="mt-3 text-[34px] leading-none font-semibold tracking-tight">{monitors.length}</p>
	</div>
	{#each kpis as k (k.status)}
		<button
			type="button"
			class="rounded-card px-5 py-4 text-left transition-colors {filter === k.status ? 'bg-ink text-canvas' : 'bg-surface hover:bg-sunken'}"
			onclick={() => (filter = filter === k.status ? 'all' : k.status)}
			aria-pressed={filter === k.status}
		>
			<p class="flex items-center gap-2 text-[13px] font-medium {filter === k.status ? 'text-canvas/70' : 'text-ink-2'}">
				<span class="sq" style="--c: {k.color}; --s: 8px"></span>
				{k.label}
			</p>
			<p class="mt-3 text-[34px] leading-none font-semibold tracking-tight {counts[k.status] > 0 || filter === k.status ? '' : 'text-faint'}">
				{counts[k.status]}
			</p>
		</button>
	{/each}
</section>

{#if monitors.length === 0}
	<div class="card flex flex-col items-center px-6 py-16 text-center">
		<span class="grid size-10 place-items-center rounded-ctl bg-sunken text-ink-2"><Icon name="activity" size={18} /></span>
		<p class="mt-4 text-[17px] font-semibold">No monitors yet</p>
		<p class="mt-1 max-w-sm text-ink-2">Add a website, API, database, container or game server and heron starts checking it right away.</p>
		<a href="/admin/monitors/new" class="btn-primary mt-6"><Icon name="plus" /> Create monitor</a>
	</div>
{:else}
	<section class="card overflow-hidden">
		<header class="flex flex-wrap items-center gap-3 px-[22px] pt-[18px] pb-4">
			<h2 class="text-[22px] leading-tight font-semibold tracking-[-0.015em]">All monitors</h2>
			<span class="text-[15px] font-medium text-faint">{visible.length}</span>
			<div class="ml-auto flex w-full items-center gap-2 sm:w-auto">
				{#if filter !== 'all'}
					<button class="btn-secondary btn-sm" onclick={() => (filter = 'all')}>
						{statusLabel[filter]} <Icon name="x" size={11} />
					</button>
				{/if}
				<div class="relative w-full sm:w-72">
					<input class="input h-10 pl-10" type="search" placeholder="Search" bind:value={query} />
					<span class="pointer-events-none absolute top-1/2 left-3.5 -translate-y-1/2 text-faint"><Icon name="search" size={15} /></span>
				</div>
			</div>
		</header>

		<div class="overflow-x-auto">
			<table class="w-full">
				<thead class="border-y border-line">
					<tr>
						<th class="th pl-[22px]">Monitor</th>
						<th class="th w-36">Status</th>
						<th class="th hidden md:table-cell">Type</th>
						<th class="th hidden xl:table-cell">Group</th>
						<th class="th w-44">Uptime 24h</th>
						<th class="th hidden text-right sm:table-cell">Response</th>
						<th class="th hidden pr-[22px] text-right lg:table-cell">Checked</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each visible as m (m.id)}
						<tr class="relative transition-colors hover:bg-surface">
							<td class="min-w-64 py-3.5 pr-4 pl-[22px]">
								<a href="/admin/monitors/{m.id}" class="block after:absolute after:inset-0">
									<span class="flex items-center gap-2 text-[15px] font-semibold">
										<span class="truncate">{m.name}</span>
										{#if !m.public}
											<span class="chip text-faint ring-line-strong">private</span>
										{/if}
									</span>
									<span class="mt-0.5 block max-w-[24rem] truncate font-mono text-xs text-faint">
										{m.kind === 'push' ? `heartbeat every ${interval(m.interval_s)}` : displayTarget(m.target)}
									</span>
								</a>
							</td>
							<td class="px-4 py-3.5"><StatusBadge status={m.status} /></td>
							<td class="hidden px-4 py-3.5 whitespace-nowrap md:table-cell">
								<span class="text-[14px] text-ink-2">{kindLabel[m.kind]}</span>
								<span class="ml-1.5 font-mono text-xs text-faint">{interval(m.interval_s)}</span>
							</td>
							<td class="hidden px-4 py-3.5 text-ink-2 xl:table-cell">{m.group || '—'}</td>
							<td class="px-4 py-3.5">
								<div class="flex items-center gap-3">
									<span class="flex h-2.5 w-20 gap-[2px]">
										{#each Array(10) as _, i (i)}
											<span class="flex-1 rounded-[1px] {i < Math.round((m.uptime_24h ?? 0) * 10) ? uptimeTone(m.uptime_24h) : 'bg-pending'}"></span>
										{/each}
									</span>
									<span class="font-mono text-xs">{percent(m.uptime_24h)}</span>
								</div>
							</td>
							<td class="hidden px-4 py-3.5 text-right font-mono text-xs text-ink-2 sm:table-cell">
								{m.status === 'down' || m.status === 'paused' ? '—' : latency(m.latency_ms)}
							</td>
							<td class="hidden py-3.5 pr-[22px] pl-4 text-right text-[13px] whitespace-nowrap text-faint lg:table-cell">{ago(m.checked_at, clock.now)}</td>
						</tr>
					{:else}
						<tr><td colspan="7" class="px-4 py-12 text-center text-ink-2">Nothing matches the filter.</td></tr>
					{/each}
				</tbody>
			</table>
		</div>
	</section>
{/if}