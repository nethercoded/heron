<script lang="ts">
	import { untrack } from 'svelte';
	import { ApiError, message } from '#lib/api.ts';
	import { formatHeaders, parseHeaders } from '#lib/monitor-form.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Channel, ChannelConfig, ChannelKind } from '#lib/types.ts';
	import Field from './Field.svelte';
	import Toggle from './Toggle.svelte';

	type Input = { name: string; kind: ChannelKind; enabled: boolean; config: ChannelConfig };

	let {
		channel,
		onsubmit,
		oncancel
	}: { channel?: Channel; onsubmit: (input: Input) => Promise<void>; oncancel: () => void } = $props();

	let form = $state<Input>(
		untrack(() =>
			channel
				? { name: channel.name, kind: channel.kind, enabled: channel.enabled, config: { ...channel.config } }
				: { name: '', kind: 'telegram', enabled: true, config: {} }
		)
	);
	let headers = $state(untrack(() => formatHeaders(channel?.config.headers)));
	let fields = $state<Record<string, string>>({});
	let busy = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		fields = {};
		const config: ChannelConfig =
			form.kind === 'telegram'
				? {
						bot_token: form.config.bot_token,
						chat_id: form.config.chat_id,
						thread_id: form.config.thread_id || undefined
					}
				: form.kind === 'discord'
					? { url: form.config.url, mention: form.config.mention || undefined }
					: { url: form.config.url, secret: form.config.secret, headers: parseHeaders(headers) };
		try {
			await onsubmit({ ...form, config });
		} catch (err) {
			if (err instanceof ApiError) fields = err.fields;
			toast.error(message(err));
		} finally {
			busy = false;
		}
	}
</script>

<form class="space-y-4" onsubmit={submit} novalidate>
	<div class="grid gap-4 sm:grid-cols-2">
		<Field label="Name" id="channel-name" error={fields.name}>
			<input id="channel-name" class="input" placeholder="On-call" bind:value={form.name} aria-invalid={!!fields.name} />
		</Field>
		<Field label="Type" id="channel-kind" error={fields.kind}>
			<select id="channel-kind" class="input-select" bind:value={form.kind}>
				<option value="telegram">Telegram</option>
				<option value="discord">Discord</option>
				<option value="webhook">Webhook</option>
			</select>
		</Field>

		{#if form.kind === 'telegram'}
			<Field label="Bot token" id="bot-token" hint="From @BotFather" error={fields.bot_token} class="sm:col-span-2">
				<input id="bot-token" class="input font-mono" autocomplete="off" placeholder="123456:ABC-DEF…" bind:value={form.config.bot_token} aria-invalid={!!fields.bot_token} />
			</Field>
			<Field label="Chat ID" id="chat-id" hint="User, group or @channel" error={fields.chat_id}>
				<input id="chat-id" class="input font-mono" placeholder="-1001234567890" bind:value={form.config.chat_id} aria-invalid={!!fields.chat_id} />
			</Field>
			<Field label="Topic ID" id="thread-id" hint="Optional, for forum groups">
				<input id="thread-id" type="number" class="input font-mono" bind:value={form.config.thread_id} />
			</Field>
		{:else if form.kind === 'discord'}
			<Field label="Webhook URL" id="discord-url" hint="Channel settings → Integrations → Webhooks → Copy Webhook URL" error={fields.url} class="sm:col-span-2">
				<input id="discord-url" class="input font-mono" autocomplete="off" placeholder="https://discord.com/api/webhooks/…" bind:value={form.config.url} aria-invalid={!!fields.url} />
			</Field>
			<Field label="Mention" id="discord-mention" hint="Optional. A role ID, @here or @everyone, pinged when a monitor goes down" class="sm:col-span-2">
				<input id="discord-mention" class="input font-mono" placeholder="@here" bind:value={form.config.mention} />
			</Field>
		{:else}
			<Field label="URL" id="hook-url" hint="Receives a JSON POST on every status change" error={fields.url} class="sm:col-span-2">
				<input id="hook-url" class="input font-mono" placeholder="https://example.com/hooks/heron" bind:value={form.config.url} aria-invalid={!!fields.url} />
			</Field>
			<Field label="Signing secret" id="hook-secret" hint="Adds an X-Heron-Signature HMAC header" class="sm:col-span-2">
				<input id="hook-secret" class="input font-mono" autocomplete="off" bind:value={form.config.secret} />
			</Field>
			<Field label="Extra headers" id="hook-headers" hint="One per line, Name: value" class="sm:col-span-2">
				<textarea id="hook-headers" class="input-area" rows="2" bind:value={headers}></textarea>
			</Field>
		{/if}
	</div>

	<Toggle label="Enabled" bind:checked={form.enabled} />

	<div class="flex justify-end gap-2 border-t border-line pt-4">
		<button type="button" class="btn-ghost" onclick={oncancel}>Cancel</button>
		<button class="btn-primary" disabled={busy}>{channel ? 'Save' : 'Add channel'}</button>
	</div>
</form>
