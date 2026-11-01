<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import IncidentCard from '#lib/components/IncidentCard.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head>
	<title>{data.incident.title}</title>
</svelte:head>

<a href="/incidents" class="mb-6 inline-flex items-center gap-1 text-sm text-muted hover:text-ink">
	<Icon name="back" size={14} /> Incident history
</a>

<IncidentCard incident={data.incident} expanded />
