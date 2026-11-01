<script lang="ts">
	import { api } from '#lib/api.ts';
	import MaintenanceForm from '#lib/components/MaintenanceForm.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import { toast } from '#lib/toast.svelte.ts';
	import type { MaintenanceInput } from '#lib/types.ts';
	import { goto } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	async function create(input: MaintenanceInput) {
		await api.post('/admin/maintenance', input);
		toast.success('Maintenance scheduled');
		await goto('/admin/maintenance');
	}
</script>

<PageHeader title="Schedule maintenance" back={{ href: '/admin/maintenance', label: 'Maintenance' }} />

<MaintenanceForm monitors={data.monitors} submitLabel="Schedule" onsubmit={create} />
