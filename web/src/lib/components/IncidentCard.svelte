<script lang="ts">
	import { between, formatDateTime } from '#lib/format.ts';
	import { impactLabel, impactTone, incidentStatusLabel } from '#lib/status.ts';
	import type { Impact, Incident } from '#lib/types.ts';
	import Icon from './Icon.svelte';

	let { incident, href, expanded = false }: { incident: Incident; href?: string; expanded?: boolean } = $props();

	const open = $derived(!incident.resolved_at);
	const updates = $derived(expanded ? incident.updates : incident.updates.slice(0, 2));
	const stripe: Record<Impact, string> = {
		minor: 'bg-degraded',
		major: 'bg-down',
		critical: 'bg-down',
		maintenance: 'bg-accent'
	};
</script>

<article class="card relative overflow-hidden">
	<span class="absolute inset-y-0 left-0 w-1 {open ? stripe[incident.impact] : 'bg-up'}"></span>

	<header class="flex flex-wrap items-start justify-between gap-3 border-b border-line py-4 pr-5 pl-6">
		<div class="min-w-0">
			<h3 class="text-[15px] font-semibold text-ink">
				{#if href}
					<a {href} class="hover:text-accent">{incident.title}</a>
				{:else}
					{incident.title}
				{/if}
			</h3>
			<p class="mt-1 text-xs text-muted">
				Started {formatDateTime(incident.started_at)}
				{#if incident.resolved_at}
					· Resolved in {between(incident.started_at, incident.resolved_at)}
				{:else}
					· Ongoing for {between(incident.started_at)}
				{/if}
			</p>
		</div>
		<div class="flex items-center gap-2">
			<span class="chip {impactTone[incident.impact]}">{impactLabel[incident.impact]}</span>
			<span class="chip {open ? 'text-ink ring-line-strong' : 'bg-up/10 text-up ring-up/25'}">
				{#if !open}<Icon name="check" size={11} />{/if}
				{incidentStatusLabel[incident.status]}
			</span>
		</div>
	</header>

	<ol class="py-1 pr-5 pl-6">
		{#each updates as update, i (update.id)}
			<li class="grid gap-x-6 py-3 sm:grid-cols-[8.5rem_1fr] {i > 0 ? 'border-t border-line' : ''}">
				<div>
					<p class="text-[13px] font-semibold {update.status === 'resolved' ? 'text-up' : 'text-ink'}">{incidentStatusLabel[update.status]}</p>
					<time class="text-xs text-faint" datetime={update.created_at}>{formatDateTime(update.created_at)}</time>
				</div>
				<p class="mt-1 leading-relaxed text-muted sm:mt-0">{update.body}</p>
			</li>
		{/each}
		{#if !expanded && incident.updates.length > updates.length && href}
			<li class="border-t border-line py-2.5">
				<a {href} class="text-xs font-medium text-accent hover:underline">
					View {incident.updates.length - updates.length} earlier {incident.updates.length - updates.length === 1 ? 'update' : 'updates'}
				</a>
			</li>
		{/if}
	</ol>

	{#if incident.monitors.length > 0}
		<footer class="flex flex-wrap items-center gap-1.5 border-t border-line bg-sunken/60 py-2.5 pr-5 pl-6 text-xs text-muted">
			<span class="mr-1">Affected services</span>
			{#each incident.monitors as m (m.slug)}
				<a href="/monitor/{m.slug}" class="rounded border border-line bg-surface px-1.5 py-0.5 font-medium text-ink hover:border-line-strong">{m.name}</a>
			{/each}
		</footer>
	{/if}
</article>
