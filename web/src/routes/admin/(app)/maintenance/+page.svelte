<script lang="ts">
	import { api, message } from '#lib/api.ts';
	import Icon from '#lib/components/Icon.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import { ago, duration, formatDateTime } from '#lib/format.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { MaintenanceWindow } from '#lib/types.ts';
	import { invalidate } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const repeatLabel = { none: 'Once', daily: 'Daily', weekly: 'Weekly' };

	function state(w: MaintenanceWindow) {
		if (!w.enabled) return { label: 'Disabled', tone: 'text-muted ring-line-strong' };
		if (w.active) return { label: 'In progress', tone: 'bg-accent/12 text-accent ring-accent/30' };
		if (w.finished) return { label: 'Completed', tone: 'text-faint ring-line' };
		return { label: 'Scheduled', tone: 'bg-up/10 text-up ring-up/25' };
	}

	async function remove(w: MaintenanceWindow) {
		if (!confirm(`Delete "${w.title}"?`)) return;
		try {
			await api.delete(`/admin/maintenance/${w.id}`);
			toast.success('Maintenance window deleted');
			await invalidate('heron:maintenance');
		} catch (err) {
			toast.error(message(err));
		}
	}
</script>

<PageHeader title="Maintenance" description="Planned work windows. Alerts are muted and the status page shows the work as scheduled.">
	{#snippet actions()}
		<a href="/admin/maintenance/new" class="btn-primary"><Icon name="plus" size={14} /> Schedule maintenance</a>
	{/snippet}
</PageHeader>

{#if data.windows.length === 0}
	<div class="card flex flex-col items-center px-6 py-16 text-center">
		<span class="grid size-12 place-items-center rounded-full bg-sunken text-muted"><Icon name="wrench" size={22} /></span>
		<h2 class="mt-4 text-[15px] font-semibold">No maintenance scheduled</h2>
		<p class="mt-1 max-w-sm text-muted">Plan upgrades or recurring backups so heron stays quiet and your users know what to expect.</p>
		<a href="/admin/maintenance/new" class="btn-primary mt-6"><Icon name="plus" size={14} /> Schedule maintenance</a>
	</div>
{:else}
	<div class="card overflow-x-auto">
		<table class="w-full">
			<thead class="border-b border-line bg-sunken/70">
				<tr>
					<th class="th">Window</th>
					<th class="th">Next occurrence</th>
					<th class="th hidden md:table-cell">Repeat</th>
					<th class="th hidden lg:table-cell">Monitors</th>
					<th class="th">State</th>
					<th class="th w-10"></th>
				</tr>
			</thead>
			<tbody class="divide-y divide-line">
				{#each data.windows as w (w.id)}
					{@const s = state(w)}
					<tr class="relative transition hover:bg-sunken/60">
						<td class="px-4 py-3">
							<a href="/admin/maintenance/{w.id}" class="font-medium after:absolute after:inset-0">{w.title}</a>
							<span class="block text-xs text-muted">{duration(w.duration_s * 1000)}</span>
						</td>
						<td class="px-4 py-3 text-xs whitespace-nowrap">
							{formatDateTime(w.starts_at)}
							{#if !w.finished && !w.active}<span class="block text-faint">{ago(w.starts_at)}</span>{/if}
						</td>
						<td class="hidden px-4 py-3 text-muted md:table-cell">{repeatLabel[w.repeat]}</td>
						<td class="hidden max-w-xs truncate px-4 py-3 text-muted lg:table-cell">{w.monitors.map((m) => m.name).join(', ')}</td>
						<td class="px-4 py-3"><span class="chip {s.tone}">{s.label}</span></td>
						<td class="relative z-10 px-2 py-3 text-right">
							<button class="btn-ghost btn-sm hover:text-down" onclick={() => remove(w)} aria-label="Delete"><Icon name="trash" size={14} /></button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
