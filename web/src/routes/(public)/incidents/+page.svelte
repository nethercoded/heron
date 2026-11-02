<script lang="ts">
	import IncidentCard from '#lib/components/IncidentCard.svelte';
	import { useI18n } from '#lib/i18n.svelte.ts';
	import type { Incident } from '#lib/types.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const i18n = useI18n();

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
	<title>{i18n.t('history.title')}</title>
</svelte:head>

<h1 class="text-2xl font-semibold tracking-tight">{i18n.t('history.title')}</h1>
<p class="mt-1 text-muted">{i18n.t('history.lead')}</p>

<div class="mt-10 space-y-10">
	{#each months as [month, incidents] (month)}
		<section>
			<h2 class="mb-4 text-sm font-semibold tracking-wide text-muted uppercase">{i18n.month(`${month}-01T00:00:00Z`)}</h2>
			<div class="space-y-4">
				{#each incidents as incident (incident.id)}
					<IncidentCard {incident} href="/incidents/{incident.id}" />
				{/each}
			</div>
		</section>
	{:else}
		<div class="card p-10 text-center">
			<p class="font-medium">{i18n.t('history.empty')}</p>
			<p class="mt-1 text-sm text-muted">{i18n.t('history.empty.lead')}</p>
		</div>
	{/each}
</div>
