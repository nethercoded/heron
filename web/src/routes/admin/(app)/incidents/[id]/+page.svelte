<script lang="ts">
	import { untrack } from 'svelte';
	import { ApiError, api, message } from '#lib/api.ts';
	import Field from '#lib/components/Field.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import IncidentCard from '#lib/components/IncidentCard.svelte';
	import MonitorPicker from '#lib/components/MonitorPicker.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import { impactLabel, incidentStatusLabel } from '#lib/status.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Impact, IncidentStatus } from '#lib/types.ts';
	import { goto, invalidate } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const incident = $derived(data.incident);
	const impacts = Object.keys(impactLabel) as Impact[];
	const statuses = Object.keys(incidentStatusLabel) as IncidentStatus[];

	let status = $state<IncidentStatus>(untrack(() => (data.incident.status === 'resolved' ? 'resolved' : 'monitoring')));
	let body = $state('');
	let fields = $state<Record<string, string>>({});
	let busy = $state(false);

	let editing = $state(false);
	let title = $state('');
	let impact = $state<Impact>('minor');
	let monitorIds = $state<number[]>([]);

	function startEdit() {
		title = incident.title;
		impact = incident.impact;
		monitorIds = [...(incident.monitor_ids ?? [])];
		editing = true;
	}

	async function post(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		fields = {};
		try {
			await api.post(`/admin/incidents/${incident.id}/updates`, { status, body });
			body = '';
			toast.success(status === 'resolved' ? 'Incident resolved' : 'Update posted');
			await invalidate('heron:incident');
		} catch (err) {
			if (err instanceof ApiError) fields = err.fields;
			toast.error(message(err));
		} finally {
			busy = false;
		}
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		try {
			await api.put(`/admin/incidents/${incident.id}`, { title, impact, monitor_ids: monitorIds });
			editing = false;
			toast.success('Incident updated');
			await invalidate('heron:incident');
		} catch (err) {
			toast.error(message(err));
		}
	}

	async function remove() {
		if (!confirm('Delete this incident? It will disappear from the public history.')) return;
		try {
			await api.delete(`/admin/incidents/${incident.id}`);
			toast.success('Incident deleted');
			await goto('/admin/incidents');
		} catch (err) {
			toast.error(message(err));
		}
	}
</script>

<PageHeader title={incident.title} back={{ href: '/admin/incidents', label: 'Incidents' }}>
	{#snippet actions()}
		<button class="btn-secondary" onclick={startEdit}><Icon name="edit" /> Edit</button>
		<button class="btn-danger" onclick={remove} aria-label="Delete incident"><Icon name="trash" /></button>
	{/snippet}
</PageHeader>

<div class="grid gap-6 xl:grid-cols-[1fr_20rem]">
	<div class="min-w-0">
		<IncidentCard {incident} expanded />
	</div>

	<aside class="space-y-6">
		{#if editing}
			<form class="card space-y-4 p-5" onsubmit={save}>
				<h2 class="font-semibold">Edit incident</h2>
				<Field label="Title" id="edit-title">
					<input id="edit-title" class="input" bind:value={title} />
				</Field>
				<Field label="Impact" id="edit-impact">
					<select id="edit-impact" class="input-select" bind:value={impact}>
						{#each impacts as value (value)}<option {value}>{impactLabel[value]}</option>{/each}
					</select>
				</Field>
				<div>
					<p class="label">Affected monitors</p>
					<MonitorPicker monitors={data.monitors} bind:selected={monitorIds} />
				</div>
				<div class="flex justify-end gap-2">
					<button type="button" class="btn-ghost" onclick={() => (editing = false)}>Cancel</button>
					<button class="btn-primary">Save</button>
				</div>
			</form>
		{/if}

		<form class="card space-y-4 p-5" onsubmit={post} novalidate>
			<h2 class="font-semibold">Post an update</h2>
			<Field label="Status" id="status" error={fields.status}>
				<select id="status" class="input-select" bind:value={status}>
					{#each statuses as value (value)}<option {value}>{incidentStatusLabel[value]}</option>{/each}
				</select>
			</Field>
			<Field label="Message" id="body" error={fields.body}>
				<textarea
					id="body"
					class="input-area font-sans text-sm"
					rows="4"
					placeholder={status === 'resolved' ? 'The issue has been fixed.' : 'We are still looking into it.'}
					bind:value={body}
					aria-invalid={!!fields.body}
				></textarea>
			</Field>
			<button class="btn-primary w-full" disabled={busy}>
				{status === 'resolved' ? 'Resolve incident' : 'Post update'}
			</button>
		</form>
	</aside>
</div>
