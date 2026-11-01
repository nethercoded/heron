<script lang="ts">
	import { api, message } from '#lib/api.ts';
	import BeatStrip from '#lib/components/BeatStrip.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import LatencyChart from '#lib/components/LatencyChart.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import RangeTabs from '#lib/components/RangeTabs.svelte';
	import Snippet from '#lib/components/Snippet.svelte';
	import StatusBadge from '#lib/components/StatusBadge.svelte';
	import UptimeCalendar from '#lib/components/UptimeCalendar.svelte';
	import { badgeMarkdown } from '#lib/embed.ts';
	import { ago, between, formatDateTime, interval, latency } from '#lib/format.ts';
	import { Clock, Live } from '#lib/live.svelte.ts';
	import { kindLabel, statusLabel, statusText } from '#lib/status.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { AdminMonitor, Range, Series } from '#lib/types.ts';
	import { goto, invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const m = $derived(data.monitor);
	let range = $state<Range>('24h');
	let series = $state<Series | null>(null);
	let busy = $state(false);
	const clock = new Clock(10_000);

	const badges = $derived([
		{ label: 'Status', path: `/badge/${m.slug}` },
		{ label: 'Uptime 30d', path: `/badge/${m.slug}/uptime?period=30d` },
		{ label: 'Response time', path: `/badge/${m.slug}/response` }
	]);
	const pushURL = $derived(m.push_token ? `${page.url.origin}/api/push/${m.push_token}` : '');

	$effect(() => {
		const id = m.id;
		const selected = range;
		let cancelled = false;
		api.get<Series>(`/admin/monitors/${id}/series?range=${selected}`).then((s) => {
			if (!cancelled) series = s;
		});
		return () => {
			cancelled = true;
		};
	});

	new Live(
		{
			monitor: (event) => {
				if (event.id === m.id) invalidate('heron:monitor');
			}
		},
		true
	);

	async function action(name: string, path: string, method: 'post' | 'delete' = 'post') {
		busy = true;
		try {
			if (method === 'delete') await api.delete(path);
			else await api.post(path);
			toast.success(name);
			await invalidate('heron:monitor');
		} catch (err) {
			toast.error(message(err));
		} finally {
			busy = false;
		}
	}

	async function remove() {
		if (!confirm(`Delete ${m.name}? All of its history will be lost.`)) return;
		try {
			await api.delete(`/admin/monitors/${m.id}`);
			toast.success(`${m.name} deleted`);
			await goto('/admin');
		} catch (err) {
			toast.error(message(err));
		}
	}

	async function rotate() {
		if (!confirm('Generate a new push URL? The old one will stop working immediately.')) return;
		await action('Push URL regenerated', `/admin/monitors/${m.id}/token`);
	}

	async function copy(text: string) {
		await navigator.clipboard.writeText(text);
		toast.info('Copied to clipboard');
	}

	const details = $derived<[string, string][]>([
		['Type', kindLabel[m.kind]],
		[m.kind === 'push' ? 'Expected every' : 'Interval', interval(m.interval_s)],
		...(m.kind === 'push' ? [] : ([['Timeout', `${m.timeout_s}s`]] as [string, string][])),
		['Retries', String(m.retries)],
		['Visibility', m.public ? 'Public' : 'Private'],
		['Created', formatDateTime(m.created_at)]
	]);
</script>

<PageHeader title={m.name} description={m.kind === 'push' ? 'Push monitor' : m.target} back={{ href: '/admin', label: 'Monitors' }}>
	{#snippet actions()}
		{#if m.kind !== 'push' && !m.paused}
			<button class="btn-secondary" disabled={busy} onclick={() => action('Check scheduled', `/admin/monitors/${m.id}/check`)}>
				<Icon name="refresh" /> Check now
			</button>
		{/if}
		{#if m.paused}
			<button class="btn-secondary" disabled={busy} onclick={() => action('Monitor resumed', `/admin/monitors/${m.id}/resume`)}>
				<Icon name="play" /> Resume
			</button>
		{:else}
			<button class="btn-secondary" disabled={busy} onclick={() => action('Monitor paused', `/admin/monitors/${m.id}/pause`)}>
				<Icon name="pause" /> Pause
			</button>
		{/if}
		<a href="/admin/monitors/{m.id}/edit" class="btn-secondary"><Icon name="edit" /> Edit</a>
		<button class="btn-danger" onclick={remove} aria-label="Delete monitor"><Icon name="trash" /></button>
	{/snippet}
</PageHeader>

<div class="grid gap-6 xl:grid-cols-[1fr_18rem]">
	<div class="min-w-0 space-y-6">
		<section class="card p-5">
			<div class="flex flex-wrap items-center justify-between gap-4">
				<div>
					<StatusBadge status={m.status} />
					<p class="mt-1 text-xs text-muted">
						{#if m.since}for {between(m.since, new Date(clock.now).toISOString())} ·{/if}
						last check {ago(m.checked_at, clock.now)}
					</p>
				</div>
				{#if m.message}
					<p class="max-w-md truncate rounded-lg bg-sunken px-3 py-1.5 font-mono text-xs {statusText[m.status]}" title={m.message}>
						{m.message}
					</p>
				{/if}
			</div>
			<div class="mt-5"><BeatStrip beats={[...data.beats].reverse()} /></div>
		</section>

		{#if m.kind === 'push'}
			<section class="card p-5">
				<h2 class="font-semibold">Push URL</h2>
				<p class="mt-1 text-sm text-muted">
					Call this URL at least every {interval(m.interval_s)}. Add <code class="kbd whitespace-nowrap">?status=down</code> to report a failure.
				</p>
				<div class="mt-4 flex gap-2">
					<input class="input font-mono text-xs" readonly value={pushURL} onfocus={(e) => e.currentTarget.select()} />
					<button class="btn-secondary" onclick={() => copy(pushURL)} aria-label="Copy URL"><Icon name="copy" /></button>
					<button class="btn-secondary" onclick={rotate} aria-label="Regenerate URL" title="Regenerate"><Icon name="key" /></button>
				</div>
				<pre class="mt-4 rounded-lg bg-sunken p-3 font-mono text-xs break-all whitespace-pre-wrap text-muted">curl -fsS -m 10 --retry 3 "{pushURL}"</pre>
			</section>
		{/if}

		<section class="card p-5">
			<div class="mb-4 flex items-center justify-between gap-3">
				<h2 class="font-semibold">Response time</h2>
				<RangeTabs bind:value={range} />
			</div>
			<LatencyChart points={series?.points ?? []} />
		</section>

		<section class="card p-5">
			<h2 class="mb-4 font-semibold">Uptime calendar</h2>
			<UptimeCalendar days={data.calendar.days} since={data.calendar.since} />
		</section>

		<section class="card overflow-hidden">
			<h2 class="border-b border-line px-5 py-3.5 font-semibold">Recent checks</h2>
			<div class="max-h-[28rem] overflow-y-auto">
				<table class="w-full text-sm">
					<tbody class="divide-y divide-line">
						{#each data.beats as beat (beat.at + beat.status)}
							<tr>
								<td class="px-5 py-2 whitespace-nowrap text-muted tabular-nums">{formatDateTime(beat.at)}</td>
								<td class="px-3 py-2 font-medium whitespace-nowrap {statusText[beat.status]}">{statusLabel[beat.status]}</td>
								<td class="px-3 py-2 text-right whitespace-nowrap text-muted tabular-nums">{latency(beat.latency_ms)}</td>
								<td class="max-w-0 truncate px-5 py-2 font-mono text-xs text-muted" title={beat.message}>{beat.message}</td>
							</tr>
						{:else}
							<tr><td class="px-5 py-8 text-center text-muted">No checks yet.</td></tr>
						{/each}
					</tbody>
				</table>
			</div>
		</section>
	</div>

	<aside class="space-y-6">
		<section class="card p-5">
			<h2 class="mb-3 font-semibold">Details</h2>
			<dl class="space-y-2 text-sm">
				{#each details as [label, value] (label)}
					<div class="flex justify-between gap-3">
						<dt class="text-muted">{label}</dt>
						<dd class="text-right">{value}</dd>
					</div>
				{/each}
			</dl>
			{#if m.public}
				<a href="/monitor/{m.slug}" target="_blank" class="btn-ghost btn-sm mt-4 -ml-2.5">
					Public page <Icon name="external" size={12} />
				</a>
			{/if}
		</section>

		{#if m.public}
			<section class="card space-y-4 p-5">
				<div>
					<h2 class="font-semibold">Badges</h2>
					<p class="mt-0.5 text-xs text-muted">Markdown for a README or docs page.</p>
				</div>
				{#each badges as badge (badge.path)}
					<Snippet label={badge.label} preview={badge.path} code={badgeMarkdown(page.url.origin, badge.label, badge.path, `/monitor/${m.slug}`)} />
				{/each}
			</section>
		{/if}
	</aside>
</div>
