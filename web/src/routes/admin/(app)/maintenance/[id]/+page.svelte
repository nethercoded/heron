<script lang="ts">
	import { api } from '#lib/api.ts';
	import MaintenanceForm from '#lib/components/MaintenanceForm.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import { toast } from '#lib/toast.svelte.ts';
	import type { MaintenanceInput } from '#lib/types.ts';
	import { goto } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	async function save(input: MaintenanceInput) {
		await api.put(`/admin/maintenance/${data.window.id}`, input);
		toast.success('Maintenance window updated');
		await goto('/admin/maintenance');
	}
</script>

<PageHeader title={data.window.title} back={{ href: '/admin/maintenance', label: 'Maintenance' }} />

{#key data.window.id}
	<MaintenanceForm initial={data.window} monitors={data.monitors} submitLabel="Save changes" onsubmit={save} />
{/key}
