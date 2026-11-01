<script lang="ts">
	import { api } from '#lib/api.ts';
	import MonitorForm from '#lib/components/MonitorForm.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import { blank } from '#lib/monitor-form.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { AdminMonitor, MonitorInput } from '#lib/types.ts';
	import { goto } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	async function create(input: MonitorInput) {
		const m = await api.post<AdminMonitor>('/admin/monitors', input);
		toast.success(`${m.name} is being monitored`);
		await goto(`/admin/monitors/${m.id}`);
	}
</script>

<PageHeader title="New monitor" back={{ href: '/admin', label: 'Monitors' }} />

<MonitorForm initial={blank()} channels={data.channels} submitLabel="Create monitor" onsubmit={create} />
