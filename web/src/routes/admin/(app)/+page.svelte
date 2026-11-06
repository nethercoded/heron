<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { flip } from 'svelte/animate';
	import { api, message } from '#lib/api.ts';
	import Icon from '#lib/components/Icon.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import StatusBadge from '#lib/components/StatusBadge.svelte';
	import Warnings from '#lib/components/Warnings.svelte';
	import { ago, displayTarget, interval, latency, percent } from '#lib/format.ts';
	import { Clock, debounce, Live } from '#lib/live.svelte.ts';
	import { kindLabel, statusLabel } from '#lib/status.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { AdminMonitor, Kind, Status } from '#lib/types.ts';
	import { goto, invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let monitors = $state<AdminMonitor[]>(untrack(() => data.monitors));
	$effect.pre(() => {
		monitors = data.monitors;
	});

	const params = untrack(() => page.url.searchParams);
	const initialStatus = params.get('status') ?? '';

	let query = $state(params.get('q') ?? '');
	let filter = $state<Status | 'all'>(initialStatus in statusLabel ? (initialStatus as Status) : 'all');
	let kind = $state(params.get('kind') ?? '');
	let group = $state(params.get('group') ?? '');
	const clock = new Clock(10_000);

	const canEdit = $derived((page.data as { user?: { role?: string } }).user?.role !== 'viewer');

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

	const kinds = $derived(
		[...new Set(monitors.map((m) => m.kind))].sort((a, b) => kindLabel[a].localeCompare(kindLabel[b]))
	);
	const groups = $derived(
		[...new Set(monitors.map((m) => m.group).filter(Boolean))].sort((a, b) => a.localeCompare(b))
	);
	const ungrouped = $derived(monitors.some((m) => !m.group));

	const filtered = $derived(query.trim() !== '' || filter !== 'all' || kind !== '' || group !== '');

	const visible = $derived(
		monitors.filter((m) => {
			if (filter !== 'all' && m.status !== filter) return false;
			if (kind && m.kind !== kind) return false;
			if (group && m.group !== group) return false;
			const q = query.trim().toLowerCase();
			return (
				!q ||
				m.name.toLowerCase().includes(q) ||
				m.slug.toLowerCase().includes(q) ||
				m.target.toLowerCase().includes(q) ||
				m.group.toLowerCase().includes(q) ||
				kindLabel[m.kind].toLowerCase().includes(q)
			);
		})
	);

	function clear() {
		query = '';
		filter = 'all';
		kind = '';
		group = '';
	}

	$effect(() => {
		const next = new URLSearchParams();
		const q = query.trim();
		if (q) next.set('q', q);
		if (filter !== 'all') next.set('status', filter);
		if (kind) next.set('kind', kind);
		if (group) next.set('group', group);
		const search = next.size ? `?${next}` : '';
		const url = untrack(() => page.url);
		if (search === url.search) return;
		goto(`${url.pathname}${search}`, { replace: true, reset: false });
	});

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

	let table = $state<HTMLTableElement>();
	let dragging = $state<number | null>(null);
	let snapshot: AdminMonitor[] | null = null;
	let pending: AdminMonitor[] | null = null;
	let timer: ReturnType<typeof setTimeout> | undefined;

	const reorderable = $derived(canEdit && !filtered && monitors.length > 1);

	function move(from: number, to: number) {
		const next = monitors.slice();
		next.splice(to, 0, ...next.splice(from, 1));
		monitors = next;
	}

	function commit(before: AdminMonitor[], wait: number) {
		pending ??= before;
		clearTimeout(timer);
		timer = setTimeout(save, wait);
	}

	async function save() {
		const before = pending;
		pending = null;
		if (!before) return;
		const ids = monitors.map((m) => m.id);
		if (ids.every((id, i) => before[i]?.id === id)) return;
		try {
			await api.put('/admin/monitors/order', { ids });
			toast.success('Order saved');
		} catch (err) {
			monitors = before;
			toast.error(message(err));
		}
	}

	function grab(e: PointerEvent, id: number) {
		if (!reorderable || e.button !== 0) return;
		e.preventDefault();
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
		snapshot = monitors.slice();
		dragging = id;
	}

	function drag(e: PointerEvent) {
		if (dragging == null || !table) return;
		const rows = table.querySelectorAll<HTMLTableRowElement>('tr[data-id]');
		let top = rows[0]?.parentElement?.getBoundingClientRect().top ?? 0;
		for (const row of rows) {
			const bottom = top + row.offsetHeight;
			const inside = e.clientY >= top && e.clientY < bottom;
			top = bottom;
			if (!inside) continue;
			const from = monitors.findIndex((m) => m.id === dragging);
			const to = monitors.findIndex((m) => m.id === Number(row.dataset.id));
			if (from !== to && from >= 0 && to >= 0) move(from, to);
			break;
		}
	}

	function drop() {
		if (dragging == null) return;
		dragging = null;
		if (snapshot) commit(snapshot, 0);
		snapshot = null;
	}

	function cancel() {
		if (dragging == null) return;
		dragging = null;
		if (snapshot) monitors = snapshot;
		snapshot = null;
	}

	async function nudge(e: KeyboardEvent, id: number) {
		if (!e.altKey || (e.key !== 'ArrowUp' && e.key !== 'ArrowDown')) return;
		e.preventDefault();
		if (!reorderable) return;
		const from = monitors.findIndex((m) => m.id === id);
		const to = from + (e.key === 'ArrowUp' ? -1 : 1);
		if (from < 0 || to < 0 || to >= monitors.length) return;
		const before = monitors.slice();
		move(from, to);
		commit(before, 700);
		await tick();
		table?.querySelector<HTMLElement>(`[data-grip="${id}"]`)?.focus();
	}

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
		{#if canEdit}
			<a href="/admin/monitors/new" class="btn-primary"><Icon name="plus" /> New monitor</a>
		{/if}
	{/snippet}
</PageHeader>

<Warnings warnings={data.warnings} linked />

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
		{#if canEdit}
			<a href="/admin/monitors/new" class="btn-primary mt-6"><Icon name="plus" /> Create monitor</a>
		{/if}
	</div>
{:else}
	<section class="card overflow-hidden">
		<header class="flex flex-wrap items-center gap-3 px-[22px] pt-[18px] pb-4">
			<h2 class="text-[22px] leading-tight font-semibold tracking-[-0.015em]">All monitors</h2>
			<span class="text-[15px] font-medium text-faint">{visible.length}</span>
			<div class="ml-auto flex w-full flex-wrap items-center gap-2 sm:w-auto sm:flex-nowrap">
				{#if kinds.length > 1}
					<select class="input-select h-10 w-auto min-w-0 flex-1 text-[14px] sm:flex-none" bind:value={kind} aria-label="Type">
						<option value="">All types</option>
						{#each kinds as k (k)}
							<option value={k}>{kindLabel[k]}</option>
						{/each}
					</select>
				{/if}
				{#if groups.length > 1 || (groups.length === 1 && ungrouped)}
					<select class="input-select h-10 w-auto min-w-0 flex-1 text-[14px] sm:flex-none" bind:value={group} aria-label="Group">
						<option value="">All groups</option>
						{#each groups as g (g)}
							<option value={g}>{g}</option>
						{/each}
					</select>
				{/if}
				<div class="relative w-full sm:w-72">
					<input class="input h-10 pl-10" type="search" placeholder="Search" bind:value={query} />
					<span class="pointer-events-none absolute top-1/2 left-3.5 -translate-y-1/2 text-faint"><Icon name="search" size={15} /></span>
				</div>
			</div>
			{#if filtered}
				<div class="flex w-full flex-wrap items-center gap-2">
					{#if filter !== 'all'}
						<button class="btn-secondary btn-sm" onclick={() => (filter = 'all')}>
							{statusLabel[filter]} <Icon name="x" size={11} />
						</button>
					{/if}
					{#if kind}
						<button class="btn-secondary btn-sm" onclick={() => (kind = '')}>
							{kindLabel[kind as Kind] ?? kind} <Icon name="x" size={11} />
						</button>
					{/if}
					{#if group}
						<button class="btn-secondary btn-sm" onclick={() => (group = '')}>
							{group} <Icon name="x" size={11} />
						</button>
					{/if}
					{#if query.trim()}
						<button class="btn-secondary btn-sm max-w-60" onclick={() => (query = '')}>
							<span class="truncate">"{query.trim()}"</span> <Icon name="x" size={11} />
						</button>
					{/if}
					<button class="btn-ghost btn-sm" onclick={clear}>Clear</button>
				</div>
			{/if}
		</header>

		<div class="overflow-x-auto">
			<table class="w-full" bind:this={table}>
				<thead class="border-y border-line">
					<tr>
						{#if canEdit}
							<th class="th w-0 pr-0 pl-3"><span class="sr-only">Order</span></th>
						{/if}
						<th class="th {canEdit ? 'pl-2' : 'pl-[22px]'}">Monitor</th>
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
						<tr
							class="relative transition-colors {dragging === m.id ? 'bg-surface' : 'hover:bg-surface'}"
							data-id={m.id}
							animate:flip={{ duration: 180 }}
						>
							{#if canEdit}
								<td class="w-0 py-3.5 pr-0 pl-3">
									<button
										type="button"
										class="relative z-10 grid size-7 touch-none place-items-center rounded-lg text-faint transition-colors {reorderable
											? dragging === m.id
												? 'cursor-grabbing bg-sunken text-ink'
												: 'cursor-grab hover:bg-sunken hover:text-ink'
											: 'cursor-not-allowed opacity-40'}"
										data-grip={m.id}
										aria-disabled={!reorderable}
										aria-label="Reorder {m.name}"
										title={reorderable ? 'Drag to reorder, or Alt+Up / Alt+Down' : 'Clear search and filters to reorder'}
										onpointerdown={(e) => grab(e, m.id)}
										onpointermove={drag}
										onpointerup={drop}
										onpointercancel={cancel}
										onkeydown={(e) => nudge(e, m.id)}
									>
										<Icon name="grip" size={14} />
									</button>
								</td>
							{/if}
							<td class="min-w-64 py-3.5 pr-4 {canEdit ? 'pl-2' : 'pl-[22px]'}">
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
						<tr><td colspan={canEdit ? 8 : 7} class="px-4 py-12 text-center text-ink-2">Nothing matches the filter.</td></tr>
					{/each}
				</tbody>
			</table>
		</div>
	</section>
{/if}
