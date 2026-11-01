<script lang="ts">
	import { api } from '#lib/api.ts';
	import MonitorForm from '#lib/components/MonitorForm.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import { fromMonitor } from '#lib/monitor-form.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { MonitorInput } from '#lib/types.ts';
	import { goto } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	async function save(input: MonitorInput) {
		await api.put(`/admin/monitors/${data.monitor.id}`, input);
		toast.success('Changes saved');
		await goto(`/admin/monitors/${data.monitor.id}`);
	}
</script>

<PageHeader title="Edit {data.monitor.name}" back={{ href: `/admin/monitors/${data.monitor.id}`, label: data.monitor.name }} />

{#key data.monitor.id}
	<MonitorForm initial={fromMonitor(data.monitor)} channels={data.channels} submitLabel="Save changes" editing onsubmit={save} />
{/key}
