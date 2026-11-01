<script lang="ts">
	import { ago, between, formatDateTime } from '#lib/format.ts';
	import { impactLabel, impactTone, incidentStatusLabel } from '#lib/status.ts';
	import type { Incident } from '#lib/types.ts';
	import Icon from './Icon.svelte';

	let { incident, href, expanded = false }: { incident: Incident; href?: string; expanded?: boolean } = $props();

	const open = $derived(!incident.resolved_at);
	const updates = $derived(expanded ? incident.updates : incident.updates.slice(0, 1));
</script>

<article class="card overflow-hidden">
	<header class="flex flex-wrap items-start justify-between gap-3 border-b border-line px-5 py-4 {open ? 'bg-sunken/60' : ''}">
		<div class="min-w-0">
			<h3 class="font-semibold text-ink">
				{#if href}
					<a {href} class="hover:underline">{incident.title}</a>
				{:else}
					{incident.title}
				{/if}
			</h3>
			<p class="mt-1 text-xs text-muted">
				{formatDateTime(incident.started_at)}
				{#if incident.resolved_at}
					· resolved after {between(incident.started_at, incident.resolved_at)}
				{:else}
					· ongoing for {between(incident.started_at)}
				{/if}
			</p>
		</div>
		<div class="flex items-center gap-2">
			<span class="chip {impactTone[incident.impact]}">{impactLabel[incident.impact]}</span>
			<span class="chip {open ? 'text-ink ring-line' : 'text-up ring-up/30 bg-up/10'}">
				{#if !open}<Icon name="check" size={12} />{/if}
				{incidentStatusLabel[incident.status]}
			</span>
		</div>
	</header>

	<ol class="space-y-4 px-5 py-4">
		{#each updates as update (update.id)}
			<li class="relative pl-5">
				<span class="absolute top-1.5 left-0 size-2 rounded-full {update.status === 'resolved' ? 'bg-up' : 'bg-faint'}"></span>
				<p class="text-sm">
					<span class="font-medium text-ink">{incidentStatusLabel[update.status]}</span>
					<span class="text-muted">— {update.body}</span>
				</p>
				<time class="mt-0.5 block text-xs text-faint" datetime={update.created_at} title={formatDateTime(update.created_at)}>
					{ago(update.created_at)}
				</time>
			</li>
		{/each}
	</ol>

	{#if incident.monitors.length > 0}
		<footer class="flex flex-wrap items-center gap-1.5 border-t border-line px-5 py-3 text-xs text-muted">
			Affected:
			{#each incident.monitors as m (m.slug)}
				<a href="/monitor/{m.slug}" class="rounded-md bg-sunken px-1.5 py-0.5 text-ink hover:bg-line">{m.name}</a>
			{/each}
		</footer>
	{/if}
</article>
