<script lang="ts">
	import { untrack } from 'svelte';
	import { ApiError, message } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { AdminMonitor, MaintenanceInput, MaintenanceWindow, Repeat } from '#lib/types.ts';
	import Field from './Field.svelte';
	import MonitorPicker from './MonitorPicker.svelte';
	import Toggle from './Toggle.svelte';

	let {
		initial,
		monitors,
		submitLabel,
		onsubmit
	}: {
		initial?: MaintenanceWindow;
		monitors: AdminMonitor[];
		submitLabel: string;
		onsubmit: (input: MaintenanceInput) => Promise<void>;
	} = $props();

	function localInput(date: Date) {
		const pad = (n: number) => String(n).padStart(2, '0');
		return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
	}

	function nextSunday() {
		const d = new Date();
		d.setDate(d.getDate() + ((7 - d.getDay()) % 7 || 7));
		d.setHours(3, 0, 0, 0);
		return d;
	}

	const seed = untrack(() => initial);
	const seconds = seed?.duration_s ?? 3600;

	let title = $state(seed?.title ?? '');
	let body = $state(seed?.body ?? '');
	let monitorIds = $state<number[]>(seed?.monitor_ids ?? []);
	let start = $state(localInput(seed ? new Date(seed.starts_at) : nextSunday()));
	let unit = $state<'m' | 'h'>(seconds % 3600 === 0 ? 'h' : 'm');
	let amount = $state(seconds % 3600 === 0 ? seconds / 3600 : Math.round(seconds / 60));
	let repeat = $state<Repeat>(seed?.repeat ?? 'none');
	let enabled = $state(seed?.enabled ?? true);
	let fields = $state<Record<string, string>>({});
	let busy = $state(false);

	const end = $derived.by(() => {
		const s = new Date(start);
		if (Number.isNaN(s.getTime())) return null;
		return new Date(s.getTime() + amount * (unit === 'h' ? 3600 : 60) * 1000);
	});
	const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		fields = {};
		try {
			await onsubmit({
				title,
				body,
				monitor_ids: monitorIds,
				starts_at: new Date(start).toISOString(),
				duration_s: amount * (unit === 'h' ? 3600 : 60),
				repeat,
				enabled
			});
		} catch (err) {
			if (err instanceof ApiError) fields = err.fields;
			toast.error(message(err));
		} finally {
			busy = false;
		}
	}
</script>

<form class="space-y-6" onsubmit={submit} novalidate>
	<section class="card space-y-4 p-5 sm:p-6">
		<Field label="Title" id="title" error={fields.title}>
			<input id="title" class="input" placeholder="Database upgrade" bind:value={title} aria-invalid={!!fields.title} />
		</Field>
		<Field label="Details" id="body" hint="Shown on the public status page">
			<textarea id="body" class="input-area font-sans text-[13px]" rows="3" placeholder="Expect short periods of read-only access." bind:value={body}></textarea>
		</Field>
	</section>

	<section class="card p-5 sm:p-6">
		<h2 class="mb-1 font-semibold">Schedule</h2>
		<p class="mb-5 text-muted">Times are in your timezone, {zone}.</p>
		<div class="grid gap-4 sm:grid-cols-3">
			<Field label="Starts" id="start" error={fields.starts_at}>
				<input id="start" type="datetime-local" class="input" bind:value={start} aria-invalid={!!fields.starts_at} />
			</Field>
			<Field label="Duration" id="amount" error={fields.duration}>
				<div class="flex gap-2">
					<input id="amount" type="number" min="1" class="input" bind:value={amount} aria-invalid={!!fields.duration} />
					<select class="input-select w-28" bind:value={unit} aria-label="Duration unit">
						<option value="m">minutes</option>
						<option value="h">hours</option>
					</select>
				</div>
			</Field>
			<Field label="Repeat" id="repeat" error={fields.repeat}>
				<select id="repeat" class="input-select" bind:value={repeat}>
					<option value="none">Does not repeat</option>
					<option value="daily">Every day</option>
					<option value="weekly">Every week on {new Date(start).toLocaleDateString('en', { weekday: 'long' })}</option>
				</select>
			</Field>
		</div>
		{#if end}
			<p class="mt-4 rounded-md bg-sunken px-3 py-2 text-xs text-muted">
				Ends {end.toLocaleString('en', { dateStyle: 'medium', timeStyle: 'short' })}. Alerts and automatic incidents are muted for the
				selected monitors while the window is open, and the window appears on the status page a week in advance.
			</p>
		{/if}
	</section>

	<section class="card divide-y divide-line">
		<div class="p-5 sm:p-6">
			<p class="label">Affected monitors</p>
			<MonitorPicker {monitors} bind:selected={monitorIds} />
			{#if fields.monitor_ids}<p class="mt-1.5 text-xs text-down">{fields.monitor_ids}</p>{/if}
		</div>
		<div class="p-5 sm:p-6">
			<Toggle label="Enabled" description="Disabled windows are kept but never applied" bind:checked={enabled} />
		</div>
	</section>

	<div class="flex justify-end gap-2">
		<a href="/admin/maintenance" class="btn-secondary">Cancel</a>
		<button class="btn-primary" disabled={busy}>{busy ? 'Saving…' : submitLabel}</button>
	</div>
</form>
