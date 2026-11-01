<script lang="ts">
	import { untrack } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import StatusDot from '#lib/components/StatusDot.svelte';
	import { ago, interval, latency, percent } from '#lib/format.ts';
	import { Clock, debounce, Live } from '#lib/live.svelte.ts';
	import { kindLabel, statusLabel, statusText } from '#lib/status.ts';
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
			{ up: 0, degraded: 0, down: 0, pending: 0, paused: 0 }
		)
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

	const summary: { status: Status; label: string }[] = [
		{ status: 'up', label: 'Up' },
		{ status: 'degraded', label: 'Degraded' },
		{ status: 'down', label: 'Down' },
		{ status: 'paused', label: 'Paused' }
	];
</script>

<PageHeader title="Monitors" description="Everything heron is watching right now.">
	{#snippet actions()}
		<a href="/admin/monitors/new" class="btn-primary"><Icon name="plus" /> New monitor</a>
	{/snippet}
</PageHeader>

<div class="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
	{#each summary as s (s.status)}
		<button
			type="button"
			class="card px-4 py-3 text-left transition hover:border-faint {filter === s.status ? 'ring-2 ring-accent/40' : ''}"
			onclick={() => (filter = filter === s.status ? 'all' : s.status)}
		>
			<span class="flex items-center gap-2 text-xs text-muted"><StatusDot status={s.status} size="sm" /> {s.label}</span>
			<span class="mt-1 block text-2xl font-semibold tabular-nums">{counts[s.status]}</span>
		</button>
	{/each}
</div>

{#if monitors.length === 0}
	<div class="card flex flex-col items-center px-6 py-16 text-center">
		<span class="grid size-12 place-items-center rounded-full bg-sunken text-muted"><Icon name="activity" size={22} /></span>
		<h2 class="mt-4 font-semibold">No monitors yet</h2>
		<p class="mt-1 max-w-sm text-sm text-muted">Add a website, API, server or cron job and heron will start checking it right away.</p>
		<a href="/admin/monitors/new" class="btn-primary mt-6"><Icon name="plus" /> Create your first monitor</a>
	</div>
{:else}
	<div class="card overflow-hidden">
		<div class="flex flex-wrap items-center gap-3 border-b border-line p-3">
			<input class="input max-w-xs" type="search" placeholder="Search monitors" bind:value={query} />
			{#if filter !== 'all'}
				<button class="btn-ghost btn-sm" onclick={() => (filter = 'all')}>
					{statusLabel[filter]} <Icon name="x" size={12} />
				</button>
			{/if}
		</div>

		<div class="overflow-x-auto">
			<table class="w-full text-sm">
				<thead class="text-left text-xs text-muted">
					<tr class="border-b border-line">
						<th class="px-4 py-2.5 font-medium">Monitor</th>
						<th class="hidden px-4 py-2.5 font-medium md:table-cell">Type</th>
						<th class="px-4 py-2.5 text-right font-medium">Uptime 24h</th>
						<th class="hidden px-4 py-2.5 text-right font-medium sm:table-cell">Latency</th>
						<th class="hidden px-4 py-2.5 text-right font-medium lg:table-cell">Checked</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each visible as m (m.id)}
						<tr class="group relative transition hover:bg-sunken/60">
							<td class="px-4 py-3">
								<a href="/admin/monitors/{m.id}" class="flex items-center gap-3 after:absolute after:inset-0">
									<StatusDot status={m.status} />
									<span class="min-w-0">
										<span class="flex items-center gap-2 font-medium text-ink">
											<span class="truncate">{m.name}</span>
											{#if !m.public}
												<span class="rounded bg-sunken px-1.5 text-[10px] font-medium text-muted uppercase">private</span>
											{/if}
										</span>
										<span class="block max-w-[22rem] truncate text-xs text-muted">
											{m.kind === 'push' ? `heartbeat every ${interval(m.interval_s)}` : m.target}
										</span>
									</span>
								</a>
							</td>
							<td class="hidden px-4 py-3 text-muted md:table-cell">
								{kindLabel[m.kind]} · {interval(m.interval_s)}
							</td>
							<td class="px-4 py-3 text-right tabular-nums {m.uptime_24h != null && m.uptime_24h < 0.99 ? 'text-down' : ''}">
								{percent(m.uptime_24h)}
							</td>
							<td class="hidden px-4 py-3 text-right text-muted tabular-nums sm:table-cell">
								{m.status === 'down' ? '' : latency(m.latency_ms)}
								{#if m.status === 'down' || m.status === 'paused' || m.status === 'pending'}
									<span class={statusText[m.status]}>{statusLabel[m.status]}</span>
								{/if}
							</td>
							<td class="hidden px-4 py-3 text-right text-muted lg:table-cell">{ago(m.checked_at, clock.now)}</td>
						</tr>
					{:else}
						<tr><td colspan="5" class="px-4 py-10 text-center text-muted">Nothing matches your filter.</td></tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{/if}
