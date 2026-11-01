<script lang="ts">
	import { ApiError, api, message } from '#lib/api.ts';
	import Field from '#lib/components/Field.svelte';
	import MonitorPicker from '#lib/components/MonitorPicker.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import { impactLabel, incidentStatusLabel } from '#lib/status.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Impact, Incident, IncidentStatus } from '#lib/types.ts';
	import { goto } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let title = $state('');
	let impact = $state<Impact>('minor');
	let status = $state<IncidentStatus>('investigating');
	let body = $state('');
	let monitorIds = $state<number[]>([]);
	let fields = $state<Record<string, string>>({});
	let busy = $state(false);

	const impacts = Object.keys(impactLabel) as Impact[];
	const statuses = Object.keys(incidentStatusLabel) as IncidentStatus[];

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		fields = {};
		try {
			const incident = await api.post<Incident>('/admin/incidents', {
				title,
				impact,
				status,
				body,
				monitor_ids: monitorIds
			});
			toast.success('Incident published');
			await goto(`/admin/incidents/${incident.id}`);
		} catch (err) {
			if (err instanceof ApiError) fields = err.fields;
			toast.error(message(err));
		} finally {
			busy = false;
		}
	}
</script>

<PageHeader title="Report incident" description="It will appear on the public page right away." back={{ href: '/admin/incidents', label: 'Incidents' }} />

<form class="card space-y-5 p-5 sm:p-6" onsubmit={submit} novalidate>
	<Field label="Title" id="title" error={fields.title}>
		<input id="title" class="input" placeholder="Elevated error rates on the API" bind:value={title} aria-invalid={!!fields.title} />
	</Field>

	<div class="grid gap-4 sm:grid-cols-2">
		<Field label="Impact" id="impact" error={fields.impact}>
			<select id="impact" class="input-select" bind:value={impact}>
				{#each impacts as value (value)}<option {value}>{impactLabel[value]}</option>{/each}
			</select>
		</Field>
		<Field label="Status" id="status" error={fields.status}>
			<select id="status" class="input-select" bind:value={status}>
				{#each statuses as value (value)}<option {value}>{incidentStatusLabel[value]}</option>{/each}
			</select>
		</Field>
	</div>

	<Field label="Message" id="body" hint="What is happening and what are you doing about it?" error={fields.body}>
		<textarea id="body" class="input-area font-sans text-sm" rows="4" bind:value={body} aria-invalid={!!fields.body}></textarea>
	</Field>

	<div>
		<p class="label">Affected monitors</p>
		<MonitorPicker monitors={data.monitors} bind:selected={monitorIds} />
	</div>

	<div class="flex justify-end gap-2 border-t border-line pt-5">
		<a href="/admin/incidents" class="btn-secondary">Cancel</a>
		<button class="btn-primary" disabled={busy}>{busy ? 'Publishing…' : 'Publish incident'}</button>
	</div>
</form>
