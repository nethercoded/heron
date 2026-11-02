<script lang="ts">
	import { untrack } from 'svelte';
	import { slide } from 'svelte/transition';
	import { ApiError, api, message } from '#lib/api.ts';
	import {
		commonKinds,
		detect,
		formatHeaders,
		intervals,
		kindLabel,
		nameFrom,
		normalizeTarget,
		otherKinds,
		parseCodes,
		parseHeaders,
		slugify,
		targetCopy
	} from '#lib/monitor-form.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Channel, Kind, MonitorInput, Status } from '#lib/types.ts';
	import Field from './Field.svelte';
	import Icon from './Icon.svelte';
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

	let form = $state<MonitorInput>(
		untrack(() => ({
			...initial,
			channel_ids:
				editing || initial.channel_ids.length ? initial.channel_ids : channels.filter((c) => c.enabled).map((c) => c.id)
		}))
	);
	let picked = $state(untrack(() => editing));
	let choosing = $state(false);
	let moreKinds = $state(untrack(() => otherKinds.some((k) => k.value === initial.kind)));
	let advanced = $state(false);
	let slugTouched = $state(untrack(() => editing));
	let headers = $state(untrack(() => formatHeaders(initial.settings.headers)));
	let codes = $state(untrack(() => (initial.settings.accepted_status ?? []).join(', ')));
	let fields = $state<Record<string, string>>({});
	let busy = $state(false);
	let checking = $state(false);
	let result = $state<{ status: Status; latency_ms: number; message: string } | null>(null);

	const guessed = $derived(picked ? form.kind : (detect(form.target) ?? form.kind));
	const target = $derived(targetCopy[guessed]);
	const suggestedName = $derived(nameFrom(guessed, form.target));
	const address = $derived(normalizeTarget(guessed, form.target));
	const intervalOptions = $derived(
		intervals.some((i) => i.value === form.interval_s)
			? intervals
			: [...intervals, { value: form.interval_s, label: `${form.interval_s} seconds` }]
	);
	const main = ['name', 'target', 'kind'];

	$effect(() => {
		if (!picked) form.kind = guessed;
	});

	$effect(() => {
		if (!slugTouched) form.slug = slugify(form.name || suggestedName);
	});

	$effect(() => {
		if (form.timeout_s > form.interval_s) form.timeout_s = Math.min(form.interval_s, 10);
	});

	$effect(() => {
		void form.target;
		void form.kind;
		result = null;
	});

	function pick(kind: Kind) {
		form.kind = kind;
		picked = true;
		choosing = false;
	}

	function toggleChannel(id: number, on: boolean) {
		form.channel_ids = on ? [...form.channel_ids, id] : form.channel_ids.filter((c) => c !== id);
	}

	function payload(): MonitorInput {
		const settings = { ...$state.snapshot(form.settings) };
		if (form.kind === 'http') {
			settings.headers = parseHeaders(headers);
			settings.accepted_status = parseCodes(codes);
		}
		if (form.kind === 'redis' && /^rediss?:\/\//i.test(form.target)) {
			try {
				const url = new URL(form.target);
				if (url.password && !settings.password) settings.password = decodeURIComponent(url.password);
				if (url.protocol === 'rediss:') settings.tls = true;
			} catch {}
		}
		const name = form.name.trim() || suggestedName;
		return {
			...$state.snapshot(form),
			name,
			slug: form.slug || slugify(name),
			target: form.kind === 'push' ? '' : address,
			settings
		};
	}

	async function test() {
		checking = true;
		result = null;
		try {
			result = await api.post('/admin/monitors/test', payload());
		} catch (err) {
			if (err instanceof ApiError && err.fields.target) fields = { ...fields, target: err.fields.target };
			toast.error(message(err));
		} finally {
			checking = false;
		}
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		fields = {};
		try {
			await onsubmit(payload());
		} catch (err) {
			if (err instanceof ApiError) {
				fields = err.fields;
				if (Object.keys(err.fields).some((f) => !main.includes(f))) advanced = true;
			}
			toast.error(message(err));
		} finally {
			busy = false;
		}
	}

	const num = (v: string) => (v === '' ? undefined : Number(v));
	const tone: Record<Status, string> = {
		up: 'var(--color-up)',
		degraded: 'var(--color-degraded)',
		down: 'var(--color-down)',
		maintenance: 'var(--color-accent)',
		pending: 'var(--color-faint)',
		paused: 'var(--color-faint)'
	};
</script>

{#snippet kindButton(kind: { value: Kind; label: string })}
	<button
		type="button"
		role="radio"
		aria-checked={form.kind === kind.value}
		class="rounded-ctl border px-3 py-2 text-start text-[13.5px] font-medium transition-colors {form.kind === kind.value
			? 'border-ink bg-ink text-canvas'
			: 'border-line text-ink-2 hover:border-line-strong hover:text-ink'}"
		onclick={() => pick(kind.value)}
	>
		{kind.label}
	</button>
{/snippet}

<form class="space-y-4" onsubmit={submit} novalidate>
	<section class="card space-y-5 p-5 sm:p-6">
		{#if form.kind === 'push'}
			<div>
				<h2 class="font-semibold">Heartbeat from a job</h2>
				<p class="mt-1 text-sm text-muted">
					After you create it, heron gives you a link. Call it at the end of your cron job or backup script, and heron alerts you when the
					calls stop coming.
				</p>
			</div>
			<Field label="Expect a call every" id="push-interval" error={fields.interval}>
				<select id="push-interval" class="input-select" bind:value={form.interval_s}>
					{#each intervalOptions as option (option.value)}
						<option value={option.value}>{option.label}</option>
					{/each}
				</select>
			</Field>
		{:else}
			<div>
				<label for="target" class="mb-2 block text-[15px] font-semibold">What should heron watch?</label>
				<div class="flex flex-col gap-2 sm:flex-row">
					<!-- svelte-ignore a11y_autofocus -->
					<input
						id="target"
						class="input h-12 flex-1 font-mono text-[15px]"
						placeholder={picked ? target.placeholder : 'example.com'}
						autocomplete="off"
						spellcheck="false"
						autofocus={!editing}
						bind:value={form.target}
						aria-invalid={!!fields.target}
					/>
					<button type="button" class="btn-secondary h-12" disabled={checking || !form.target.trim()} onclick={test}>
						{checking ? 'Checking…' : 'Check now'}
					</button>
				</div>
				{#if fields.target}
					<p class="mt-1.5 flex items-center gap-1.5 text-[12px] font-semibold text-down">
						<i class="size-1.5 shrink-0 rounded-[1px] bg-down"></i>
						{fields.target}
					</p>
				{:else if !form.target.trim()}
					<p class="hint">A website, an address with a port, an IP or a database URL. heron figures out the rest.</p>
				{/if}
			</div>
		{/if}

		{#if result}
			<div class="flex items-center gap-3 rounded-ctl bg-canvas px-4 py-3 text-sm" transition:slide={{ duration: 150 }}>
				<span class="sq" style="--c: {tone[result.status]}; --s: 9px"></span>
				<span class="font-semibold" style="color: {tone[result.status]}">{result.status === 'up' ? 'Works' : result.status === 'degraded' ? 'Slow' : 'Not reachable'}</span>
				{#if result.latency_ms > 0}<span class="font-mono text-faint">{result.latency_ms} ms</span>{/if}
				<span class="min-w-0 truncate text-muted">{result.message}</span>
			</div>
		{/if}

		<div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-[13px] text-muted">
			{#if form.kind !== 'push' && form.target.trim() && address !== form.target.trim()}
				<span>heron will check <span class="font-mono text-ink-2">{address}</span> ·</span>
			{/if}
			<span>Type: <span class="text-ink">{kindLabel(form.kind)}</span>{picked || !form.target.trim() ? '' : ', guessed from the address'}</span>
			<button type="button" class="text-ink-2 underline underline-offset-2 hover:text-ink" onclick={() => (choosing = !choosing)}>
				{choosing ? 'Done' : 'Change'}
			</button>
		</div>

		{#if choosing}
			<div class="space-y-2" role="radiogroup" aria-label="Check type" transition:slide={{ duration: 150 }}>
				<div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
					{#each commonKinds as kind (kind.value)}{@render kindButton(kind)}{/each}
				</div>
				{#if moreKinds}
					<div class="grid grid-cols-2 gap-2 sm:grid-cols-3" transition:slide={{ duration: 150 }}>
						{#each otherKinds as kind (kind.value)}{@render kindButton(kind)}{/each}
					</div>
				{:else}
					<button type="button" class="text-[13px] text-muted hover:text-ink" onclick={() => (moreKinds = true)}>More types: DNS, databases, mail, Docker, game servers…</button>
				{/if}
			</div>
		{/if}

		<Field label="Name" id="name" hint="How it appears on the status page" error={fields.name}>
			<input id="name" class="input" placeholder={suggestedName || 'My website'} bind:value={form.name} aria-invalid={!!fields.name} />
		</Field>
	</section>

	<section class="card divide-y divide-line">
		<div class="p-5 sm:p-6">
			<Toggle label="Show on the status page" description="Turn off to watch it privately" bind:checked={form.public} />
		</div>
		<div class="p-5 sm:p-6">
			<p class="text-[14px] font-medium">Send alerts to</p>
			{#if channels.length === 0}
				<p class="mt-1 text-[12.5px] text-faint">
					Nowhere yet. <a href="/admin/channels" class="text-ink-2 underline underline-offset-2 hover:text-ink">Connect Telegram or Discord</a> to hear when it goes down.
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

	<section class="card">
		<button
			type="button"
			class="flex w-full items-center justify-between gap-4 p-5 text-start sm:px-6"
			aria-expanded={advanced}
			onclick={() => (advanced = !advanced)}
		>
			<span>
				<span class="block text-[14px] font-medium">More settings</span>
				<span class="block text-[12.5px] text-faint">Group, how often to check, timeouts, retries and details for {kindLabel(form.kind)}</span>
			</span>
			<Icon name="chevron" size={16} class="shrink-0 text-muted transition-transform {advanced ? 'rotate-90' : ''}" />
		</button>

		{#if advanced}
			<div class="space-y-8 border-t border-line p-5 sm:p-6" transition:slide={{ duration: 200 }}>
				<div class="grid gap-4 sm:grid-cols-2">
					<Field label="Group" id="group" hint="Monitors in one group sit together on the page">
						<input id="group" class="input" placeholder="API" bind:value={form.group} />
					</Field>
					<Field label="Description" id="description" hint="A short line under the name">
						<input id="description" class="input" bind:value={form.description} />
					</Field>
					<Field label="Link name" id="slug" hint="The end of the monitor's public address" error={fields.slug}>
						<input id="slug" class="input font-mono" bind:value={form.slug} oninput={() => (slugTouched = true)} aria-invalid={!!fields.slug} />
					</Field>
				</div>

				<div class="grid gap-4 sm:grid-cols-3">
					{#if form.kind === 'push'}
						<Field label="Grace period" id="grace" hint="Extra seconds before alerting" error={fields.grace_seconds}>
							<input id="grace" type="number" min="0" class="input" bind:value={form.settings.grace_seconds} />
						</Field>
					{:else}
						<Field label="Check every" id="interval" error={fields.interval}>
							<select id="interval" class="input-select" bind:value={form.interval_s}>
								{#each intervalOptions as option (option.value)}
									<option value={option.value}>{option.label}</option>
								{/each}
							</select>
						</Field>
						<Field label="Give up after" id="timeout" hint="Seconds to wait for an answer" error={fields.timeout}>
							<input id="timeout" type="number" min="1" max={form.interval_s} class="input" bind:value={form.timeout_s} />
						</Field>
					{/if}
					<Field label="Alert after" id="retries" hint="Failed checks in a row" error={fields.retries}>
						<input id="retries" type="number" min="0" max="10" class="input" bind:value={form.retries} />
					</Field>
				</div>

				{#if form.kind === 'http'}
					<div class="grid gap-4 sm:grid-cols-3">
						<Field label="Method" id="method" error={fields.method}>
							<select id="method" class="input-select" bind:value={form.settings.method}>
								{#each ['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'] as method (method)}
									<option>{method}</option>
								{/each}
							</select>
						</Field>
						<Field label="Good status codes" id="codes" hint="Empty means 200 to 399" error={fields.accepted_status}>
							<input id="codes" class="input font-mono" placeholder="200, 204" bind:value={codes} />
						</Field>
						<Field label="Follow redirects" id="redirects" hint="0 to not follow" error={fields.max_redirects}>
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
						<Field label="Page must contain" id="keyword" hint="Text that has to be in the response" class="sm:col-span-2">
							<input id="keyword" class="input" bind:value={form.settings.keyword} />
						</Field>
						<div class="flex items-end pb-2">
							<Toggle label="Must not contain it" bind:checked={form.settings.keyword_absent} />
						</div>
						<Field label="Headers" id="headers" hint="One per line, Name: value" class="sm:col-span-3">
							<textarea id="headers" class="input-area" rows="2" placeholder="Authorization: Bearer …" bind:value={headers}></textarea>
						</Field>
						{#if form.settings.method && form.settings.method !== 'GET' && form.settings.method !== 'HEAD'}
							<Field label="Body" id="body" class="sm:col-span-3">
								<textarea id="body" class="input-area" rows="3" bind:value={form.settings.body}></textarea>
							</Field>
						{/if}
						<Field label="Certificate warning" id="cert" hint="Days before it expires">
							<input id="cert" type="number" min="0" class="input" bind:value={form.settings.cert_expiry_days} />
						</Field>
						<div class="flex items-end pb-2 sm:col-span-2">
							<Toggle label="Ignore certificate errors" description="For self-signed certificates" bind:checked={form.settings.ignore_tls} />
						</div>
					</div>
				{/if}

				{#if ['redis', 'smtp', 'imap', 'grpc', 'docker', 'minecraft', 'steam'].includes(form.kind)}
					<div class="grid gap-4 sm:grid-cols-3">
						{#if form.kind === 'redis'}
							<Field label="Password" id="password" hint="Leave empty if none">
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
							<Field label="Minimum players" id="min-players" hint="Slow below this, 0 to ignore" error={fields.min_players}>
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
						<p class="rounded-md bg-sunken px-3 py-2 text-xs text-muted">
							heron needs read access to the Docker API. On a systemd install add the heron user to the docker group, or expose a read-only
							socket proxy and point the host at it.
						</p>
					{/if}
				{/if}

				{#if form.kind === 'dns'}
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
				{/if}

				<div class="grid gap-4 sm:grid-cols-3">
					{#if form.kind !== 'push'}
						<Field label="Slow above" id="degraded" hint="Milliseconds, empty to ignore" error={fields.degraded_after_ms}>
							<input
								id="degraded"
								type="number"
								min="0"
								class="input"
								value={form.settings.degraded_after_ms ?? ''}
								oninput={(e) => (form.settings.degraded_after_ms = num(e.currentTarget.value))}
							/>
						</Field>
					{/if}
					{#if form.kind === 'icmp'}
						<Field label="Packets" id="packets" error={fields.packets}>
							<input id="packets" type="number" min="1" max="10" class="input" bind:value={form.settings.packets} />
						</Field>
					{/if}
					{#if !editing}
						<div class="flex items-end pb-2 sm:col-span-2">
							<Toggle label="Start paused" description="Create it without running checks yet" bind:checked={form.paused} />
						</div>
					{/if}
				</div>
			</div>
		{/if}
	</section>

	<div class="flex justify-end gap-2">
		<a href="/admin" class="btn-secondary">Cancel</a>
		<button class="btn-primary" disabled={busy || (form.kind !== 'push' && !form.target.trim())}>{busy ? 'Saving…' : submitLabel}</button>
	</div>
</form>
