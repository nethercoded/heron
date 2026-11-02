<script lang="ts">
	import { untrack } from 'svelte';
	import { ApiError, message } from '#lib/api.ts';
	import {
		formatHeaders,
		intervals,
		kindGroups,
		parseCodes,
		parseHeaders,
		slugify,
		targetCopy
	} from '#lib/monitor-form.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Channel, MonitorInput } from '#lib/types.ts';
	import Field from './Field.svelte';
	import Toggle from './Toggle.svelte';

	let {
		initial,
		channels,
		submitLabel,
		editing = false,
		onsubmit
	}: {
		initial: MonitorInput;
		channels: Channel[];
		submitLabel: string;
		editing?: boolean;
		onsubmit: (input: MonitorInput) => Promise<void>;
	} = $props();

	let form = $state<MonitorInput>(untrack(() => initial));
	let slugTouched = $state(untrack(() => editing));
	let headers = $state(untrack(() => formatHeaders(initial.settings.headers)));
	let codes = $state(untrack(() => (initial.settings.accepted_status ?? []).join(', ')));
	let fields = $state<Record<string, string>>({});
	let busy = $state(false);

	const target = $derived(targetCopy[form.kind]);
	const intervalOptions = $derived(
		intervals.some((i) => i.value === form.interval_s)
			? intervals
			: [...intervals, { value: form.interval_s, label: `${form.interval_s} seconds` }]
	);

	$effect(() => {
		if (!slugTouched) form.slug = slugify(form.name);
	});

	$effect(() => {
		if (form.timeout_s > form.interval_s) form.timeout_s = Math.min(form.interval_s, 10);
	});

	function toggleChannel(id: number, on: boolean) {
		form.channel_ids = on ? [...form.channel_ids, id] : form.channel_ids.filter((c) => c !== id);
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		fields = {};

		const settings = { ...form.settings };
		if (form.kind === 'http') {
			settings.headers = parseHeaders(headers);
			settings.accepted_status = parseCodes(codes);
		}

		try {
			await onsubmit({ ...$state.snapshot(form), settings });
		} catch (err) {
			if (err instanceof ApiError) fields = err.fields;
			toast.error(message(err));
		} finally {
			busy = false;
		}
	}

	const num = (v: string) => (v === '' ? undefined : Number(v));
</script>

<form class="space-y-6" onsubmit={submit} novalidate>
	<section class="card p-5 sm:p-6">
		<h2 class="mb-1 font-semibold">What should heron check?</h2>
		<p class="mb-5 text-sm text-muted">Pick a check type. You can change it later.</p>

		<div class="space-y-4" role="radiogroup" aria-label="Check type">
			{#each kindGroups as group (group.label)}
				<div>
					<p class="mb-2 text-[11px] font-semibold tracking-wider text-muted uppercase">{group.label}</p>
					<div class="grid grid-cols-2 gap-2 sm:grid-cols-5">
						{#each group.kinds as k (k.value)}
							<label
								class="cursor-pointer rounded-lg border px-3 py-2.5 transition {form.kind === k.value
									? 'border-ink bg-surface'
									: 'border-line hover:border-line-strong'}"
							>
								<input type="radio" name="kind" value={k.value} bind:group={form.kind} class="sr-only" />
								<span class="block text-sm font-medium">{k.label}</span>
								<span class="block text-xs text-muted">{k.hint}</span>
							</label>
						{/each}
					</div>
				</div>
			{/each}
		</div>

		<div class="mt-6 grid gap-4 sm:grid-cols-2">
			<Field label="Name" id="name" error={fields.name}>
				<input id="name" class="input" placeholder="Website" bind:value={form.name} aria-invalid={!!fields.name} />
			</Field>
			<Field label="Slug" id="slug" hint="Used in the public URL" error={fields.slug}>
				<input id="slug" class="input font-mono" bind:value={form.slug} oninput={() => (slugTouched = true)} aria-invalid={!!fields.slug} />
			</Field>

			{#if form.kind !== 'push'}
				<Field label={target.label} id="target" hint={target.hint} error={fields.target} class="sm:col-span-2">
					<input id="target" class="input font-mono" placeholder={target.placeholder} bind:value={form.target} aria-invalid={!!fields.target} />
				</Field>
			{/if}

			<Field label="Group" id="group" hint="Monitors with the same group are shown together">
				<input id="group" class="input" placeholder="API" bind:value={form.group} />
			</Field>
			<Field label="Description" id="description" hint="Shown on the public page">
				<input id="description" class="input" bind:value={form.description} />
			</Field>
		</div>
	</section>

	<section class="card p-5 sm:p-6">
		<h2 class="mb-5 font-semibold">Schedule</h2>
		<div class="grid gap-4 sm:grid-cols-3">
			<Field
				label={form.kind === 'push' ? 'Expected every' : 'Check every'}
				id="interval"
				error={fields.interval}
			>
				<select id="interval" class="input-select" bind:value={form.interval_s}>
					{#each intervalOptions as option (option.value)}
						<option value={option.value}>{option.label}</option>
					{/each}
				</select>
			</Field>
			{#if form.kind === 'push'}
				<Field label="Grace period" id="grace" hint="Seconds to wait before alerting" error={fields.grace_seconds}>
					<input id="grace" type="number" min="0" class="input" bind:value={form.settings.grace_seconds} />
				</Field>
			{:else}
				<Field label="Timeout" id="timeout" hint="Seconds" error={fields.timeout}>
					<input id="timeout" type="number" min="1" max={form.interval_s} class="input" bind:value={form.timeout_s} />
				</Field>
			{/if}
			<Field label="Retries" id="retries" hint="Failures before alerting" error={fields.retries}>
				<input id="retries" type="number" min="0" max="10" class="input" bind:value={form.retries} />
			</Field>
		</div>
	</section>

	{#if form.kind === 'http'}
		<section class="card p-5 sm:p-6">
			<h2 class="mb-5 font-semibold">Request</h2>
			<div class="grid gap-4 sm:grid-cols-3">
				<Field label="Method" id="method" error={fields.method}>
					<select id="method" class="input-select" bind:value={form.settings.method}>
						{#each ['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'] as method (method)}
							<option>{method}</option>
						{/each}
					</select>
				</Field>
				<Field label="Accepted status codes" id="codes" hint="Empty means 200–399" error={fields.accepted_status}>
					<input id="codes" class="input font-mono" placeholder="200, 204" bind:value={codes} />
				</Field>
				<Field label="Follow redirects" id="redirects" hint="0 disables" error={fields.max_redirects}>
					<input
						id="redirects"
						type="number"
						min="0"
						max="20"
						class="input"
						value={form.settings.max_redirects ?? 0}
						oninput={(e) => (form.settings.max_redirects = num(e.currentTarget.value))}
					/>
				</Field>
				<Field label="Headers" id="headers" hint="One per line, Name: value" class="sm:col-span-3">
					<textarea id="headers" class="input-area" rows="3" placeholder="Authorization: Bearer …" bind:value={headers}></textarea>
				</Field>
				{#if form.settings.method !== 'GET' && form.settings.method !== 'HEAD'}
					<Field label="Body" id="body" class="sm:col-span-3">
						<textarea id="body" class="input-area" rows="3" bind:value={form.settings.body}></textarea>
					</Field>
				{/if}
				<Field label="Keyword" id="keyword" hint="Response must contain this text" class="sm:col-span-2">
					<input id="keyword" class="input" bind:value={form.settings.keyword} />
				</Field>
				<div class="flex items-end pb-2">
					<Toggle label="Must be absent" bind:checked={form.settings.keyword_absent} />
				</div>
			</div>
		</section>
	{/if}

	{#if ['redis', 'smtp', 'imap', 'grpc', 'docker', 'minecraft', 'steam'].includes(form.kind)}
		<section class="card p-5 sm:p-6">
			<h2 class="mb-5 font-semibold">Connection</h2>
			<div class="grid gap-4 sm:grid-cols-3">
				{#if form.kind === 'redis'}
					<Field label="Password" id="password" hint="Sent with AUTH, leave empty if none">
						<input id="password" type="password" autocomplete="off" class="input" bind:value={form.settings.password} />
					</Field>
				{/if}
				{#if form.kind === 'grpc'}
					<Field label="Service" id="service" hint="Empty checks the whole server">
						<input id="service" class="input font-mono" placeholder="payments.v1.Payments" bind:value={form.settings.service} />
					</Field>
				{/if}
				{#if form.kind === 'docker'}
					<Field label="Docker host" id="docker-host" hint="Socket or tcp:// endpoint" error={fields.docker_host} class="sm:col-span-2">
						<input id="docker-host" class="input font-mono" placeholder="unix:///var/run/docker.sock" bind:value={form.settings.docker_host} />
					</Field>
				{/if}
				{#if form.kind === 'minecraft' || form.kind === 'steam'}
					<Field label="Minimum players" id="min-players" hint="Degraded below this, 0 to disable" error={fields.min_players}>
						<input id="min-players" type="number" min="0" class="input" bind:value={form.settings.min_players} />
					</Field>
				{/if}
				{#if ['redis', 'smtp', 'imap', 'grpc'].includes(form.kind)}
					<div class="flex items-end pb-2">
						<Toggle label="Use TLS" description="Connect over TLS from the start" bind:checked={form.settings.tls} />
					</div>
				{/if}
				{#if form.kind === 'smtp'}
					<div class="flex items-end pb-2">
						<Toggle label="Require STARTTLS" description="Fail when the server cannot upgrade" bind:checked={form.settings.starttls} />
					</div>
				{/if}
			</div>
			{#if form.kind === 'docker'}
				<p class="mt-4 rounded-md bg-sunken px-3 py-2 text-xs text-muted">
					heron needs read access to the Docker API. On a systemd install add the heron user to the docker group, or expose a read-only
					socket proxy and point the host at it.
				</p>
			{/if}
		</section>
	{/if}

	{#if form.kind === 'dns'}
		<section class="card p-5 sm:p-6">
			<h2 class="mb-5 font-semibold">Lookup</h2>
			<div class="grid gap-4 sm:grid-cols-3">
				<Field label="Record type" id="record" error={fields.record_type}>
					<select id="record" class="input-select" bind:value={form.settings.record_type}>
						{#each ['A', 'AAAA', 'CNAME', 'MX', 'NS', 'TXT'] as type (type)}
							<option>{type}</option>
						{/each}
					</select>
				</Field>
				<Field label="Resolver" id="resolver" hint="System resolver when empty" error={fields.resolver}>
					<input id="resolver" class="input font-mono" placeholder="1.1.1.1" bind:value={form.settings.resolver} />
				</Field>
				<Field label="Expected value" id="expect" hint="Any record must match">
					<input id="expect" class="input font-mono" placeholder="203.0.113.10" bind:value={form.settings.expect} />
				</Field>
			</div>
		</section>
	{/if}

	{#if form.kind !== 'push'}
		<section class="card p-5 sm:p-6">
			<h2 class="mb-5 font-semibold">Thresholds</h2>
			<div class="grid gap-4 sm:grid-cols-3">
				<Field label="Degraded above" id="degraded" hint="Milliseconds, empty to disable" error={fields.degraded_after_ms}>
					<input
						id="degraded"
						type="number"
						min="0"
						class="input"
						value={form.settings.degraded_after_ms ?? ''}
						oninput={(e) => (form.settings.degraded_after_ms = num(e.currentTarget.value))}
					/>
				</Field>
				{#if form.kind === 'http'}
					<Field label="Certificate warning" id="cert" hint="Days before expiry">
						<input id="cert" type="number" min="0" class="input" bind:value={form.settings.cert_expiry_days} />
					</Field>
					<div class="flex items-end pb-2">
						<Toggle label="Ignore TLS errors" bind:checked={form.settings.ignore_tls} />
					</div>
				{/if}
				{#if form.kind === 'icmp'}
					<Field label="Packets" id="packets" error={fields.packets}>
						<input id="packets" type="number" min="1" max="10" class="input" bind:value={form.settings.packets} />
					</Field>
				{/if}
			</div>
		</section>
	{/if}

	<section class="card divide-y divide-line">
		<div class="p-5 sm:p-6">
			<Toggle label="Show on the public status page" description="Private monitors are only visible here" bind:checked={form.public} />
		</div>
		{#if !editing}
			<div class="p-5 sm:p-6">
				<Toggle label="Start paused" description="Create the monitor without running checks" bind:checked={form.paused} />
			</div>
		{/if}
		<div class="p-5 sm:p-6">
			<p class="text-[13px] font-medium">Notify</p>
			{#if channels.length === 0}
				<p class="mt-1 text-xs text-muted">
					No notification channels yet. <a href="/admin/channels" class="text-accent hover:underline">Add one</a> to get alerts.
				</p>
			{:else}
				<div class="mt-3 flex flex-wrap gap-2">
					{#each channels as channel (channel.id)}
						{@const on = form.channel_ids.includes(channel.id)}
						<label class="chip cursor-pointer py-1 {on ? 'bg-ink text-canvas ring-ink' : 'text-muted ring-line hover:text-ink'}">
							<input type="checkbox" class="sr-only" checked={on} onchange={(e) => toggleChannel(channel.id, e.currentTarget.checked)} />
							{channel.name}
							<span class="opacity-60">{channel.kind}</span>
						</label>
					{/each}
				</div>
			{/if}
		</div>
	</section>

	<div class="flex justify-end gap-2">
		<a href="/admin" class="btn-secondary">Cancel</a>
		<button class="btn-primary" disabled={busy}>{busy ? 'Saving…' : submitLabel}</button>
	</div>
</form>
