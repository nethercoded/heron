<script lang="ts">
	import IncidentCard from '#lib/components/IncidentCard.svelte';
	import { formatMonth } from '#lib/format.ts';
	import type { Incident } from '#lib/types.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const months = $derived.by(() => {
		const groups = new Map<string, Incident[]>();
		for (const incident of data.incidents) {
			const key = incident.started_at.slice(0, 7);
			groups.set(key, [...(groups.get(key) ?? []), incident]);
		}
		return [...groups.entries()];
	});
</script>

<svelte:head>
	<title>Incident history</title>
</svelte:head>

<h1 class="text-2xl font-semibold tracking-tight">Incident history</h1>
<p class="mt-1 text-muted">Everything that went wrong over the past year, and what we did about it.</p>

<div class="mt-10 space-y-10">
	{#each months as [month, incidents] (month)}
		<section>
			<h2 class="mb-4 text-sm font-semibold tracking-wide text-muted uppercase">{formatMonth(`${month}-01T00:00:00Z`)}</h2>
			<div class="space-y-4">
				{#each incidents as incident (incident.id)}
					<IncidentCard {incident} href="/incidents/{incident.id}" />
				{/each}
			</div>
		</section>
	{:else}
		<div class="card p-10 text-center">
			<p class="font-medium">No incidents reported</p>
			<p class="mt-1 text-sm text-muted">Nothing has gone wrong in the past year.</p>
		</div>
	{/each}
</div>
