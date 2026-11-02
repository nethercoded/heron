<script lang="ts">
	import { untrack } from 'svelte';
	import { fly } from 'svelte/transition';
	import { ApiError, api, message } from '#lib/api.ts';
	import Field from '#lib/components/Field.svelte';
	import HeronField from '#lib/components/HeronField.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import { blank, fromMonitor } from '#lib/monitor-form.ts';
	import type { AdminMonitor, Channel, Page } from '#lib/types.ts';
	import { goto, invalidate } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const steps = [
		{ title: 'Setup code', hint: 'Prove you own the server' },
		{ title: 'Domain', hint: 'Where heron will live' },
		{ title: 'Administrator', hint: 'Your sign-in for this panel' },
		{ title: 'Status page', hint: 'Name and look' },
		{ title: 'First monitor', hint: 'Something to watch' },
		{ title: 'Alerts', hint: 'Telegram notifications' },
		{ title: 'Done', hint: 'Ready to go' }
	];

	let step = $state(untrack(() => data.step));
	let busy = $state(false);
	let fields = $state<Record<string, string>>({});
	let error = $state('');

	let code = $state('');
	let domain = $state('');
	let mode = $state<'direct' | 'proxy'>('direct');
	let email = $state('');
	let pending = $state<{ url: string; reachable: boolean } | null>(null);
	let username = $state('admin');
	let password = $state('');
	let confirm = $state('');

	let page = $state<Page>({ title: '', description: '', logo_url: '', history_days: 90, blocks: [] });
	let monitorName = $state('');
	let monitorURL = $state('');
	let monitorId = $state<number | null>(null);
	let botToken = $state('');
	let chatId = $state('');
	let channelId = $state<number | null>(null);
	let tested = $state(false);
	let siteURL = $state('');

	$effect(() => {
		if (step !== 6) return;
		api
			.get<{ url: string }>('/admin/site')
			.then((s) => (siteURL = s.url))
			.catch(() => {});
	});

	function go(next: number) {
		fields = {};
		error = '';
		step = next;
		history.replaceState(history.state, '', `?step=${next}`);
	}

	async function attempt(fn: () => Promise<void>) {
		busy = true;
		fields = {};
		error = '';
		try {
			await fn();
		} catch (err) {
			if (err instanceof ApiError) fields = err.fields;
			error = message(err);
		} finally {
			busy = false;
		}
	}

	$effect(() => {
		const carried = new URLSearchParams(location.hash.slice(1)).get('code');
		if (carried) {
			code = carried;
			history.replaceState(history.state, '', location.pathname + location.search);
		}
	});

	function checkCode(event: SubmitEvent) {
		event.preventDefault();
		if (code.trim().length < 6) {
			fields = { code: 'Paste the code from the server log' };
			return;
		}
		go(1);
	}

	async function saveDomain(event: SubmitEvent) {
		event.preventDefault();
		await attempt(async () => {
			let res: { url: string; reachable: boolean };
			try {
				res = await api.post('/install/domain', { code, domain, mode, email });
			} catch (err) {
				if (err instanceof ApiError && err.fields.code) {
					go(0);
					fields = { code: err.fields.code };
					return;
				}
				throw err;
			}
			const target = new URL(res.url);
			if (res.reachable && target.host !== location.host) {
				location.href = `${res.url}/admin/install?step=2#code=${encodeURIComponent(code)}`;
				return;
			}
			if (!res.reachable) {
				pending = res;
				return;
			}
			go(2);
		});
	}

	async function createAccount(event: SubmitEvent) {
		event.preventDefault();
		if (password !== confirm) {
			fields = { confirm: 'Passwords do not match' };
			return;
		}
		await attempt(async () => {
			try {
				await api.post('/auth/setup', { code, username, password });
			} catch (err) {
				if (err instanceof ApiError && err.fields.code) {
					go(0);
					fields = { code: err.fields.code };
					return;
				}
				throw err;
			}
			const current = await api.get<Page>('/admin/page');
			page = { ...current, title: current.title === 'Status' ? '' : current.title };
			go(3);
		});
	}

	async function savePage(event: SubmitEvent) {
		event.preventDefault();
		await attempt(async () => {
			await api.put('/admin/page', {
				...page,
				title: page.title.trim() || 'Status',
				history_days: page.history_days ?? 90
			});
			go(4);
		});
	}

	async function createMonitor(event: SubmitEvent) {
		event.preventDefault();
		await attempt(async () => {
			const input = { ...blank(), name: monitorName, target: monitorURL };
			const m = await api.post<AdminMonitor>('/admin/monitors', input);
			monitorId = m.id;
			go(5);
		});
	}

	async function createChannel(event: SubmitEvent) {
		event.preventDefault();
		await attempt(async () => {
			const input = {
				name: 'Telegram',
				kind: 'telegram',
				enabled: true,
				config: { bot_token: botToken.trim(), chat_id: chatId.trim() }
			};
			const channel = channelId
				? await api.put<Channel>(`/admin/channels/${channelId}`, input)
				: await api.post<Channel>('/admin/channels', input);
			channelId = channel.id;
			await api.post(`/admin/channels/${channel.id}/test`);
			tested = true;
			if (monitorId) {
				const { monitor } = await api.get<{ monitor: AdminMonitor }>(`/admin/monitors/${monitorId}`);
				const input = fromMonitor(monitor);
				await api.put(`/admin/monitors/${monitorId}`, { ...input, channel_ids: [...input.channel_ids, channel.id] });
			}
			go(6);
		});
	}

	async function finish() {
		await invalidate('heron:auth');
		await goto('/admin', { replaceState: true });
	}
</script>

<svelte:head>
	<title>Install · heron</title>
</svelte:head>

<div class="grid min-h-dvh gap-3 p-3 lg:grid-cols-[minmax(420px,560px)_1fr]">
	<section class="flex flex-col rounded-card bg-surface p-6 sm:p-10">
		<div class="flex items-end gap-[11px]">
			<Logo size={30} class="text-ink" />
			<span class="text-[23px] leading-5 font-bold tracking-[-0.04em]">heron</span>
			<span class="ml-auto font-mono text-[11px] tracking-wider text-faint uppercase">Install · {Math.min(step + 1, 7)}/7</span>
		</div>

		<ol class="mt-8 grid grid-cols-7 gap-1.5" aria-label="Progress">
			{#each steps as s, i (s.title)}
				<li class="h-1 rounded-full transition-colors duration-500 {i <= step ? 'bg-ink' : 'bg-raised'}" title={s.title}></li>
			{/each}
		</ol>

		<div class="flex flex-1 flex-col justify-center py-10">
			{#key step}
				<div class="on-sheet w-full max-w-md" in:fly={{ x: 24, duration: 320, delay: 80 }}>
					<p class="mb-3 flex items-center gap-2 font-mono text-[11px] tracking-wider text-muted uppercase">
						<span class="sq" style="--c: var(--color-ink); --s: 7px"></span>
						Step {step + 1} · {steps[step]?.hint}
					</p>

					{#if step === 0}
						<form class="space-y-5" onsubmit={checkCode}>
							<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">Welcome to heron</h1>
							<p class="text-ink-2">
								Let's get your status page running. First, paste the one-time setup code heron printed when it started, so only you can
								finish this install.
							</p>
							<div class="rounded-ctl bg-canvas px-4 py-3 font-mono text-[12.5px] leading-6 text-ink-2">
								<span class="text-faint">$</span> journalctl -u heron | grep setup_code<br />
								<span class="text-faint">$</span> docker logs heron | grep setup_code
							</div>
							<Field label="Setup code" id="code" error={fields.code}>
								<input
									id="code"
									class="input font-mono tracking-[0.2em] uppercase"
									autocomplete="one-time-code"
									spellcheck="false"
									placeholder="XXXXXXXXXXXX"
									bind:value={code}
									aria-invalid={!!fields.code}
								/>
							</Field>
							<button class="btn-primary h-[46px] w-full text-[15px]">Continue <Icon name="chevron" /></button>
						</form>
					{:else if step === 1}
						{@render domainStep()}
					{:else if step === 2}
						<form class="space-y-5" onsubmit={createAccount}>
							<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">Create your account</h1>
							<p class="text-ink-2">This is the administrator who manages monitors, incidents and alerts.</p>
							<Field label="Username" id="username" error={fields.username}>
								<input id="username" class="input" autocomplete="username" bind:value={username} aria-invalid={!!fields.username} />
							</Field>
							<Field label="Password" id="password" hint="At least 10 characters" error={fields.password}>
								<input id="password" type="password" class="input" autocomplete="new-password" bind:value={password} aria-invalid={!!fields.password} />
							</Field>
							<Field label="Repeat password" id="confirm" error={fields.confirm}>
								<input id="confirm" type="password" class="input" autocomplete="new-password" bind:value={confirm} aria-invalid={!!fields.confirm} />
							</Field>
							{@render failure()}
							<div class="flex gap-2">
								<button type="button" class="btn-secondary h-[46px]" onclick={() => go(1)}><Icon name="back" /></button>
								<button class="btn-primary h-[46px] flex-1 text-[15px]" disabled={busy}>{busy ? 'Creating…' : 'Create account'}</button>
							</div>
						</form>
					{:else if step === 3}
						<form class="space-y-5" onsubmit={savePage}>
							<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">Name your page</h1>
							<p class="text-ink-2">This is what visitors see at the top of the public status page. You can change it later in Settings.</p>
							<Field label="Title" id="title" error={fields.title}>
								<input id="title" class="input" placeholder="Acme Cloud" bind:value={page.title} aria-invalid={!!fields.title} />
							</Field>
							<Field label="Description" id="description">
								<input id="description" class="input" placeholder="Live status of our services" bind:value={page.description} />
							</Field>
							<Field label="Logo URL" id="logo" hint="Optional, the heron mark is used otherwise" error={fields.logo_url}>
								<input id="logo" class="input" placeholder="https://example.com/logo.svg" bind:value={page.logo_url} aria-invalid={!!fields.logo_url} />
							</Field>
							{@render failure()}
							<button class="btn-primary h-[46px] w-full text-[15px]" disabled={busy}>Save and continue <Icon name="chevron" /></button>
						</form>
					{:else if step === 4}
						<form class="space-y-5" onsubmit={createMonitor}>
							<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">Watch your first site</h1>
							<p class="text-ink-2">
								Start with a website or API. heron checks it every minute. Databases, containers, game servers and cron jobs can be added
								from the panel afterwards.
							</p>
							<Field label="Name" id="monitor-name" error={fields.name || fields.slug}>
								<input id="monitor-name" class="input" placeholder="Website" bind:value={monitorName} aria-invalid={!!fields.name} />
							</Field>
							<Field label="URL" id="monitor-url" error={fields.target}>
								<input id="monitor-url" class="input font-mono" placeholder="https://example.com" bind:value={monitorURL} aria-invalid={!!fields.target} />
							</Field>
							{@render failure()}
							<div class="flex gap-2">
								<button type="button" class="btn-ghost h-[46px]" onclick={() => go(5)}>Skip</button>
								<button class="btn-primary h-[46px] flex-1 text-[15px]" disabled={busy}>{busy ? 'Adding…' : 'Add monitor'}</button>
							</div>
						</form>
					{:else if step === 5}
						<form class="space-y-5" onsubmit={createChannel}>
							<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">Get alerted</h1>
							<p class="text-ink-2">
								Create a bot with <span class="font-mono text-ink">@BotFather</span>, add it to your chat and paste its token. heron sends a
								test message right away.
							</p>
							<Field label="Bot token" id="bot-token" error={fields.bot_token}>
								<input id="bot-token" class="input font-mono" autocomplete="off" placeholder="123456:ABC-DEF…" bind:value={botToken} aria-invalid={!!fields.bot_token} />
							</Field>
							<Field label="Chat ID" id="chat-id" hint="A user, group or @channel" error={fields.chat_id}>
								<input id="chat-id" class="input font-mono" placeholder="-1001234567890" bind:value={chatId} aria-invalid={!!fields.chat_id} />
							</Field>
							{@render failure()}
							<div class="flex gap-2">
								<button type="button" class="btn-ghost h-[46px]" onclick={() => go(6)}>Skip</button>
								<button class="btn-primary h-[46px] flex-1 text-[15px]" disabled={busy}>{busy ? 'Sending test…' : 'Connect Telegram'}</button>
							</div>
							{#if channelId && !tested}
								<p class="text-xs text-faint">The channel was saved. Check the token and chat, then try again or skip.</p>
							{/if}
						</form>
					{:else}
						<div class="space-y-5">
							<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">You're all set</h1>
							<p class="text-ink-2">heron is running. Here is what happened:</p>
							<ul class="space-y-2.5 text-[15px]">
								{#each [[siteURL ? `Served at ${siteURL}` : 'No domain yet, served on the server address', !!siteURL], ['Administrator account created', true], ['Status page named', true], [monitorId ? 'First monitor is being checked' : 'No monitors yet', !!monitorId], [tested ? 'Telegram alerts connected' : 'Alerts not connected', tested]] as [label, done] (label)}
									<li class="flex items-center gap-3">
										<span class="sq" style="--c: var(--color-{done ? 'up' : 'faint'}); --s: 9px"></span>
										<span class={done ? '' : 'text-faint'}>{label}</span>
									</li>
								{/each}
							</ul>
							<div class="flex flex-wrap gap-2 pt-2">
								<button class="btn-primary h-[46px] flex-1 text-[15px]" onclick={finish}>Open the panel</button>
								<a href="/" target="_blank" class="btn-secondary h-[46px]">Status page <Icon name="external" /></a>
							</div>
						</div>
					{/if}
				</div>
			{/key}
		</div>

		<ol class="hidden gap-x-5 gap-y-2 text-[13px] sm:flex sm:flex-wrap">
			{#each steps as s, i (s.title)}
				<li class="flex items-center gap-2 {i === step ? 'text-ink' : i < step ? 'text-ink-2' : 'text-faint'}">
					<span class="sq {i === step ? 'blink' : ''}" style="--c: var(--color-{i < step ? 'up' : i === step ? 'ink' : 'faint'}); --s: 7px"></span>
					{s.title}
				</li>
			{/each}
		</ol>
	</section>

	<div class="relative hidden overflow-hidden rounded-card bg-surface lg:block">
		<HeronField class="absolute inset-0 size-full" />
	</div>
</div>

{#snippet domainStep()}
	<form class="space-y-5" onsubmit={saveDomain}>
		<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">Pick a domain</h1>
		<p class="text-ink-2">
			Point a domain or subdomain at this server with an A record, then enter it here. heron checks the record, gets a free certificate
			from Let's Encrypt and switches to HTTPS on its own.
		</p>

		<Field label="Domain" id="domain" hint="For example status.example.com" error={fields.domain}>
			<input
				id="domain"
				class="input font-mono"
				spellcheck="false"
				autocapitalize="off"
				placeholder="status.example.com"
				bind:value={domain}
				aria-invalid={!!fields.domain}
			/>
		</Field>

		<div class="grid gap-2" role="radiogroup" aria-label="HTTPS">
			{#each [['direct', 'heron handles HTTPS', 'Uses ports 80 and 443 on this server. Nothing else to set up.'], ['proxy', 'I use nginx, Caddy or a tunnel', 'heron only remembers the address. Point your proxy at port 8080.']] as [value, title, text] (value)}
				<label class="cursor-pointer rounded-ctl border px-4 py-3 transition-colors {mode === value ? 'border-ink bg-canvas' : 'border-line-strong hover:border-faint'}">
					<input type="radio" name="mode" {value} bind:group={mode} class="sr-only" />
					<span class="flex items-center gap-2.5 text-[14px] font-semibold">
						<span class="sq {mode === value ? '' : 'bg-transparent!'}" style="--c: var(--color-{mode === value ? 'ink' : 'faint'}); --s: 9px"></span>
						{title}
					</span>
					<span class="mt-0.5 block pl-[19px] text-[12.5px] text-faint">{text}</span>
				</label>
			{/each}
		</div>

		{#if mode === 'direct'}
			<Field label="Email for Let's Encrypt" id="email" hint="Optional, used only for certificate expiry notices" error={fields.email}>
				<input id="email" type="email" class="input" placeholder="you@example.com" bind:value={email} aria-invalid={!!fields.email} />
			</Field>
		{/if}

		{@render failure()}

		{#if pending}
			<div class="rounded-ctl border border-line-strong px-4 py-3 text-[13px] text-ink-2">
				Saved. {pending.url} does not reach heron yet. Finish your proxy setup, then open
				<a class="font-semibold text-ink underline" href="{pending.url}/admin/install?step=2#code={encodeURIComponent(code)}">{pending.url}</a>
				or keep going here.
			</div>
		{/if}

		<div class="flex gap-2">
			<button type="button" class="btn-ghost h-[46px]" onclick={() => go(2)}>{pending ? 'Continue here' : 'Skip'}</button>
			<button class="btn-primary h-[46px] flex-1 text-[15px]" disabled={busy || !domain.trim()}>
				{busy ? (mode === 'direct' ? 'Checking DNS and getting a certificate…' : 'Saving…') : 'Set up domain'}
			</button>
		</div>
	</form>
{/snippet}

{#snippet failure()}
	{#if error && Object.keys(fields).length === 0}
		<p class="flex items-center gap-2 text-[13px] font-semibold text-down"><i class="size-1.5 rounded-[1px] bg-down"></i>{error}</p>
	{/if}
{/snippet}
