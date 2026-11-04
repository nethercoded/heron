<script lang="ts">
	import { untrack } from 'svelte';
	import { ApiError, api, message } from '#lib/api.ts';
	import { ago } from '#lib/format.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Channel, PelicanSettings } from '#lib/types.ts';
	import { invalidate } from '$app/navigation';
	import Field from './Field.svelte';
	import Toggle from './Toggle.svelte';

	let {
		settings: initial,
		channels,
		owner
	}: { settings: PelicanSettings; channels: Channel[]; owner: boolean } = $props();

	let settings = $state(untrack(() => initial));
	let form = $state(
		untrack(() => ({
			enabled: initial.enabled,
			url: initial.url,
			key: '',
			public: initial.enabled ? initial.public : true,
			channel_ids: initial.enabled ? [...initial.channel_ids] : channels.filter((c) => c.enabled).map((c) => c.id)
		}))
	);
	let fields = $state<Record<string, string>>({});
	let busy = $state(false);

	const status = $derived(settings.status);

	$effect(() => {
		if (!status.syncing) return;
		const id = setInterval(async () => {
			settings = await api.get<PelicanSettings>('/admin/pelican').catch(() => settings);
		}, 2000);
		return () => clearInterval(id);
	});

	function toggleChannel(id: number, on: boolean) {
		form.channel_ids = on ? [...form.channel_ids, id] : form.channel_ids.filter((c) => c !== id);
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		fields = {};
		try {
			settings = await api.put<PelicanSettings>('/admin/pelican', form);
			form.key = '';
			if (!settings.enabled) toast.success('Pelican is disconnected, its monitors stay until you delete them');
			else if (settings.status.error) toast.error(settings.status.error);
			else toast.success(`Synced ${settings.status.nodes} nodes and ${settings.status.servers} servers`);
			await invalidate('heron:counts');
		} catch (err) {
			if (err instanceof ApiError) fields = err.fields;
			toast.error(message(err));
		} finally {
			busy = false;
		}
	}

	async function sync() {
		try {
			await api.post('/admin/pelican/sync');
			settings = { ...settings, status: { ...settings.status, syncing: true } };
		} catch (err) {
			toast.error(message(err));
		}
	}
</script>

<form class="card space-y-5 p-5 sm:p-6" onsubmit={save} novalidate>
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div class="max-w-xl">
			<h2 class="font-semibold">Pelican</h2>
			<p class="text-sm text-muted">
				Connect your Pelican or Pterodactyl panel and heron watches every node and game server on it. New servers show up by themselves,
				and when a node goes down you get one alert for it instead of one for every server on it.
			</p>
		</div>
		{#if settings.enabled}
			<button type="button" class="btn-ghost btn-sm" disabled={status.syncing} onclick={sync}>{status.syncing ? 'Syncing…' : 'Sync now'}</button>
		{/if}
	</div>

	{#if settings.enabled}
		<p class="flex items-center gap-2 text-[13px] {status.error ? 'text-down' : 'text-muted'}">
			<span class="sq" style="--c: var(--color-{status.error ? 'down' : status.synced_at ? 'up' : 'faint'}); --s: 7px"></span>
			{#if status.error}
				{status.error}
			{:else if status.synced_at}
				{status.nodes} nodes, {status.servers} servers, synced {ago(status.synced_at)}
			{:else}
				Waiting for the first sync
			{/if}
		</p>
	{/if}

	<fieldset class="space-y-5" disabled={!owner}>
		<div class="grid gap-4 sm:grid-cols-2">
			<Field label="Panel address" id="pelican-url" error={fields.url}>
				<input id="pelican-url" class="input font-mono" placeholder="https://panel.example.com" bind:value={form.url} aria-invalid={!!fields.url} />
			</Field>
			<Field
				label="Application API key"
				id="pelican-key"
				hint={settings.has_key ? 'Saved. Leave empty to keep it' : 'Admin, API Keys. Read access to nodes and servers is enough'}
				error={fields.key}
			>
				<input
					id="pelican-key"
					type="password"
					class="input font-mono"
					autocomplete="off"
					placeholder={settings.has_key ? '••••••••••••' : 'papp_…'}
					bind:value={form.key}
					aria-invalid={!!fields.key}
				/>
			</Field>
		</div>

		<Toggle label="Show new servers on the status page" description="Applies to servers heron adds from now on" bind:checked={form.public} />

		{#if channels.length > 0}
			<div>
				<p class="text-[14px] font-medium">Send alerts to</p>
				<div class="mt-2.5 flex flex-wrap gap-2">
					{#each channels as channel (channel.id)}
						{@const on = form.channel_ids.includes(channel.id)}
						<label class="chip cursor-pointer py-1 {on ? 'bg-ink text-canvas ring-ink' : 'text-muted ring-line hover:text-ink'}">
							<input type="checkbox" class="sr-only" checked={on} onchange={(e) => toggleChannel(channel.id, e.currentTarget.checked)} />
							{channel.name}
						</label>
					{/each}
				</div>
			</div>
		{/if}

		<div class="flex items-center justify-between gap-4 border-t border-line pt-5">
			<Toggle label="Connected" bind:checked={form.enabled} />
			<button class="btn-primary" disabled={busy}>{busy ? 'Connecting…' : form.enabled ? 'Save and sync' : 'Save'}</button>
		</div>
	</fieldset>
</form>
