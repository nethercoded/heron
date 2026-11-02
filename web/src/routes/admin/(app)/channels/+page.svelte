<script lang="ts">
	import { api, message } from '#lib/api.ts';
	import ChannelForm from '#lib/components/ChannelForm.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import Toggle from '#lib/components/Toggle.svelte';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Channel, ChannelKind } from '#lib/types.ts';
	import { invalidate } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let adding = $state(false);
	let editing = $state<number | null>(null);
	let testing = $state<number | null>(null);

	const reload = () => invalidate('heron:channels');

	const kinds: Record<ChannelKind, { label: string; icon: 'send' | 'message' | 'link' }> = {
		telegram: { label: 'Telegram', icon: 'send' },
		discord: { label: 'Discord', icon: 'message' },
		webhook: { label: 'Webhook', icon: 'link' }
	};

	async function create(input: Omit<Channel, 'id' | 'created_at' | 'updated_at'>) {
		await api.post('/admin/channels', input);
		adding = false;
		toast.success('Channel added');
		await reload();
	}

	async function update(channel: Channel, input: Omit<Channel, 'id' | 'created_at' | 'updated_at'>) {
		await api.put(`/admin/channels/${channel.id}`, input);
		editing = null;
		toast.success('Channel saved');
		await reload();
	}

	async function setEnabled(channel: Channel, enabled: boolean) {
		try {
			await api.put(`/admin/channels/${channel.id}`, {
				name: channel.name,
				kind: channel.kind,
				config: channel.config,
				enabled
			});
			await reload();
		} catch (err) {
			toast.error(message(err));
		}
	}

	async function test(channel: Channel) {
		testing = channel.id;
		try {
			await api.post(`/admin/channels/${channel.id}/test`);
			toast.success(`Test message sent to ${channel.name}`);
		} catch (err) {
			toast.error(message(err));
		} finally {
			testing = null;
		}
	}

	async function remove(channel: Channel) {
		if (!confirm(`Delete ${channel.name}? Monitors will stop sending alerts to it.`)) return;
		try {
			await api.delete(`/admin/channels/${channel.id}`);
			toast.success('Channel deleted');
			await reload();
		} catch (err) {
			toast.error(message(err));
		}
	}

	function summary(channel: Channel) {
		if (channel.kind === 'telegram') return `Chat ${channel.config.chat_id}`;
		if (channel.kind === 'discord') return channel.config.mention ? `Pings ${channel.config.mention}` : 'Webhook';
		try {
			return new URL(channel.config.url ?? '').host;
		} catch {
			return channel.config.url ?? '';
		}
	}
</script>

<PageHeader title="Notifications" description="Where heron sends alerts when a monitor goes down or recovers.">
	{#snippet actions()}
		{#if !adding}
			<button class="btn-primary" onclick={() => (adding = true)}><Icon name="plus" /> Add channel</button>
		{/if}
	{/snippet}
</PageHeader>

{#if adding}
	<section class="card mb-6 p-5 sm:p-6">
		<h2 class="mb-5 font-semibold">New channel</h2>
		<ChannelForm onsubmit={create} oncancel={() => (adding = false)} />
	</section>
{/if}

<div class="space-y-3">
	{#each data.channels as channel (channel.id)}
		<section class="card p-5">
			{#if editing === channel.id}
				<ChannelForm {channel} onsubmit={(input) => update(channel, input)} oncancel={() => (editing = null)} />
			{:else}
				<div class="flex flex-wrap items-center justify-between gap-4">
					<div class="flex min-w-0 items-center gap-3">
						<span class="grid size-10 place-items-center rounded-lg bg-sunken text-muted">
							<Icon name={kinds[channel.kind].icon} />
						</span>
						<div class="min-w-0">
							<p class="font-medium">{channel.name}</p>
							<p class="truncate text-xs text-muted">
								{kinds[channel.kind].label} · {summary(channel)}
							</p>
						</div>
					</div>
					<div class="flex items-center gap-2">
						<div class="mr-2"><Toggle label="" checked={channel.enabled} onchange={(on) => setEnabled(channel, on)} /></div>
						<button class="btn-secondary btn-sm" disabled={testing === channel.id} onclick={() => test(channel)}>
							<Icon name="send" size={14} />
							{testing === channel.id ? 'Sending…' : 'Test'}
						</button>
						<button class="btn-ghost btn-sm" onclick={() => (editing = channel.id)} aria-label="Edit"><Icon name="edit" size={14} /></button>
						<button class="btn-ghost btn-sm hover:text-down" onclick={() => remove(channel)} aria-label="Delete"><Icon name="trash" size={14} /></button>
					</div>
				</div>
			{/if}
		</section>
	{:else}
		{#if !adding}
			<div class="card flex flex-col items-center px-6 py-14 text-center">
				<span class="grid size-12 place-items-center rounded-full bg-sunken text-muted"><Icon name="bell" size={22} /></span>
				<h2 class="mt-4 font-semibold">No channels yet</h2>
				<p class="mt-1 max-w-sm text-sm text-muted">Connect a Telegram chat, a Discord channel or any webhook, then pick it on the monitors you care about.</p>
				<button class="btn-primary mt-6" onclick={() => (adding = true)}><Icon name="plus" /> Add channel</button>
			</div>
		{/if}
	{/each}
</div>
