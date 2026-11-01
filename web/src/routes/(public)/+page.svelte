<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import IncidentCard from '#lib/components/IncidentCard.svelte';
	import StatusDot from '#lib/components/StatusDot.svelte';
	import UptimeBar from '#lib/components/UptimeBar.svelte';
	import { ago, latency, percent } from '#lib/format.ts';
	import { Clock } from '#lib/live.svelte.ts';
	import { overallCopy, statusLabel, statusText } from '#lib/status.ts';
	import { getStatus } from '#lib/status-page.svelte.ts';

	const store = getStatus();
	const clock = new Clock();

	const data = $derived(store.data);
	const overall = $derived(data ? overallCopy[data.status] : null);
	const tones = { up: 'bg-up', degraded: 'bg-degraded', down: 'bg-down', paused: 'bg-accent', pending: 'bg-pending' };
	const icons = { up: 'check', degraded: 'alert', down: 'x', paused: 'wrench', pending: 'info' } as const;
</script>

{#if !data}
	<div class="card p-10 text-center">
		<p class="font-medium">Status is unavailable right now</p>
		<p class="mt-1 text-sm text-muted">{store.error ?? 'Please try again in a moment.'}</p>
		<button class="btn-secondary mt-5" onclick={() => store.load()}><Icon name="refresh" /> Retry</button>
	</div>
{:else}
	{#if data.page.description}
		<p class="mb-6 max-w-2xl text-muted">{data.page.description}</p>
	{/if}

	{#if overall}
		<section class="flex items-center gap-4 rounded-card px-5 py-5 text-white shadow-card {tones[overall.tone]}">
			<span class="grid size-10 place-items-center rounded-full bg-white/20">
				<Icon name={icons[overall.tone]} size={20} />
			</span>
			<div>
				<h1 class="text-lg font-semibold tracking-tight">{overall.title}</h1>
				<p class="text-sm text-white/80">Updated {ago(data.updated_at, clock.now)}</p>
			</div>
		</section>
	{/if}

	{#if data.incidents.length > 0}
		<section class="mt-8 space-y-4">
			<h2 class="text-sm font-semibold tracking-wide text-muted uppercase">Active incidents</h2>
			{#each data.incidents as incident (incident.id)}
				<IncidentCard {incident} href="/incidents/{incident.id}" />
			{/each}
		</section>
	{/if}

	<section class="mt-10 space-y-8">
		{#each data.groups as group (group.name)}
			<div>
				{#if group.name}
					<h2 class="mb-3 text-sm font-semibold tracking-wide text-muted uppercase">{group.name}</h2>
				{/if}
				<ul class="card divide-y divide-line">
					{#each group.monitors as m (m.slug)}
						<li class="px-5 py-4">
							<div class="mb-3 flex items-center justify-between gap-4">
								<a href="/monitor/{m.slug}" class="group flex min-w-0 items-center gap-2.5">
									<StatusDot status={m.status} />
									<span class="truncate font-medium group-hover:underline">{m.name}</span>
									<span class="hidden text-sm text-faint tabular-nums sm:inline">{percent(m.uptime.history)}</span>
								</a>
								<span class="flex shrink-0 items-center gap-3 text-sm">
									{#if m.status !== 'down' && m.latency_ms > 0}
										<span class="hidden text-faint tabular-nums sm:inline">{latency(m.latency_ms)}</span>
									{/if}
									<span class="font-medium {statusText[m.status]}">{statusLabel[m.status]}</span>
								</span>
							</div>
							<div class="hidden sm:block"><UptimeBar days={m.days} /></div>
							<div class="sm:hidden"><UptimeBar days={m.days} compact /></div>
						</li>
					{/each}
				</ul>
			</div>
		{:else}
			<div class="card p-10 text-center text-muted">No services are being monitored yet.</div>
		{/each}
	</section>

	<div class="mt-10 text-center">
		<a href="/incidents" class="btn-ghost">View incident history <Icon name="chevron" size={14} /></a>
	</div>
{/if}
