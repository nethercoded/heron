<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import { between, formatDateTime } from '#lib/format.ts';
	import { impactLabel, impactTone, incidentStatusLabel } from '#lib/status.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let tab = $state<'open' | 'resolved'>('open');
	const open = $derived(data.incidents.filter((i) => !i.resolved_at));
	const resolved = $derived(data.incidents.filter((i) => i.resolved_at));
	const list = $derived(tab === 'open' ? open : resolved);
</script>

<PageHeader title="Incidents" description="Opened automatically when a monitor goes down, or written by you.">
	{#snippet actions()}
		<a href="/admin/incidents/new" class="btn-primary"><Icon name="plus" /> Report incident</a>
	{/snippet}
</PageHeader>

<div class="mb-4 inline-flex rounded-lg bg-sunken p-0.5">
	{#each [['open', `Open · ${open.length}`], ['resolved', `Resolved · ${resolved.length}`]] as [value, label] (value)}
		<button
			class="rounded-md px-3 py-1.5 text-sm font-medium transition {tab === value ? 'bg-surface text-ink shadow-card' : 'text-muted hover:text-ink'}"
			onclick={() => (tab = value as typeof tab)}
		>
			{label}
		</button>
	{/each}
</div>

<div class="card divide-y divide-line">
	{#each list as incident (incident.id)}
		<a href="/admin/incidents/{incident.id}" class="flex flex-wrap items-center justify-between gap-3 px-5 py-4 transition hover:bg-sunken/60">
			<div class="min-w-0">
				<p class="flex items-center gap-2 font-medium">
					<span class="truncate">{incident.title}</span>
					{#if incident.automatic}
						<span class="rounded bg-sunken px-1.5 text-[10px] font-medium text-muted uppercase">auto</span>
					{/if}
				</p>
				<p class="mt-0.5 text-xs text-muted">
					{formatDateTime(incident.started_at)} · {incident.resolved_at ? `lasted ${between(incident.started_at, incident.resolved_at)}` : `ongoing for ${between(incident.started_at)}`}
					{#if incident.monitors.length}· {incident.monitors.map((m) => m.name).join(', ')}{/if}
				</p>
			</div>
			<div class="flex items-center gap-2">
				<span class="chip {impactTone[incident.impact]}">{impactLabel[incident.impact]}</span>
				<span class="chip text-muted ring-line">{incidentStatusLabel[incident.status]}</span>
			</div>
		</a>
	{:else}
		<p class="px-5 py-12 text-center text-sm text-muted">
			{tab === 'open' ? 'No open incidents. Everything is calm.' : 'No resolved incidents yet.'}
		</p>
	{/each}
</div>
