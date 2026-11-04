<script lang="ts">
	import { untrack } from 'svelte';
	import { ApiError, api, message } from '#lib/api.ts';
	import Field from '#lib/components/Field.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import KumaImport from '#lib/components/KumaImport.svelte';
	import MetricsSettings from '#lib/components/MetricsSettings.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import PelicanSettings from '#lib/components/PelicanSettings.svelte';
	import SecuritySettings from '#lib/components/SecuritySettings.svelte';
	import Snippet from '#lib/components/Snippet.svelte';
	import UpdateSettings from '#lib/components/UpdateSettings.svelte';
	import { badgeMarkdown, widgetSnippet } from '#lib/embed.ts';
	import { canEdit } from '#lib/team.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { SiteSettings } from '#lib/types.ts';
	import { goto } from '$app/navigation';
	import { page as route } from '$app/state';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const owner = $derived(data.user.role === 'owner');
	const editor = $derived(canEdit(data.user.role));

	const origin = $derived(route.url.origin);
	const widget = $derived(widgetSnippet(origin));

	let site = $state(untrack(() => ({ ...data.site, mode: data.site.mode || 'direct' })));
	let siteFields = $state<Record<string, string>>({});
	let siteBusy = $state(false);

	async function saveSite(event: SubmitEvent) {
		event.preventDefault();
		siteBusy = true;
		siteFields = {};
		try {
			const res = await api.put<SiteSettings>('/admin/site', {
				domain: site.domain,
				mode: site.domain ? site.mode : '',
				email: site.email
			});
			site = { ...res, mode: res.mode || 'direct' };
			if (res.url && !res.reachable) toast.info(`Saved. ${res.url} does not reach heron yet, check your proxy.`);
			else toast.success(res.url ? `heron now answers at ${res.url}` : 'Domain removed');
		} catch (err) {
			if (err instanceof ApiError) siteFields = err.fields;
			toast.error(message(err));
		} finally {
			siteBusy = false;
		}
	}

	let current = $state('');
	let next = $state('');
	let repeat = $state('');
	let passwordFields = $state<Record<string, string>>({});

	async function changePassword(event: SubmitEvent) {
		event.preventDefault();
		passwordFields = {};
		if (next !== repeat) {
			passwordFields = { repeat: 'Passwords do not match' };
			return;
		}
		try {
			await api.put('/admin/account/password', { current, next });
			current = next = repeat = '';
			toast.success('Password changed, other sessions were signed out');
		} catch (err) {
			if (err instanceof ApiError) passwordFields = err.fields;
			if (err instanceof ApiError && err.status === 401) await goto('/admin/login');
			else toast.error(message(err));
		}
	}
</script>

<PageHeader title="Settings" />

<div class="space-y-6">
	{#if owner}
	<form class="card p-5 sm:p-6" onsubmit={saveSite} novalidate>
		<h2 class="font-semibold">Domain</h2>
		<p class="mb-5 text-sm text-muted">
			{#if data.site.url}
				heron answers at <a class="text-ink underline" href={data.site.url}>{data.site.url}</a>
				{data.site.mode === 'direct' ? 'with its own Let’s Encrypt certificate.' : 'behind your reverse proxy.'}
			{:else}
				No domain yet. heron is reachable on the server address only.
			{/if}
		</p>
		<div class="grid gap-4 sm:grid-cols-[1fr_auto]">
			<Field label="Domain" id="site-domain" hint="Leave empty to stop serving a domain" error={siteFields.domain}>
				<input id="site-domain" class="input font-mono" placeholder="status.example.com" bind:value={site.domain} aria-invalid={!!siteFields.domain} />
			</Field>
			<Field label="HTTPS" id="site-mode">
				<select id="site-mode" class="input-select" bind:value={site.mode}>
					<option value="direct">heron handles it</option>
					<option value="proxy">Reverse proxy</option>
				</select>
			</Field>
		</div>
		<div class="mt-6 flex justify-end">
			<button class="btn-primary" disabled={siteBusy}>{siteBusy ? 'Checking…' : 'Save domain'}</button>
		</div>
	</form>

	{#if data.updates}
		<UpdateSettings updates={data.updates} />
	{/if}

	<section id="backups" class="card flex flex-wrap items-center justify-between gap-4 p-5 sm:p-6">
		<div class="max-w-xl min-w-0">
			<h2 class="font-semibold">Backups</h2>
			<p class="text-sm text-muted">
				One file with every monitor, incident, person and setting. heron also keeps the last five copies from before each update in
				<code class="kbd">backups</code> next to the database. To bring one back, stop heron and run
				<code class="kbd">heron restore file.db</code>.
			</p>
		</div>
		<a href="/api/admin/backup" download class="btn-secondary"><Icon name="download" /> Download a backup</a>
	</section>
	{/if}

	{#if editor}

	<section class="card flex flex-wrap items-center justify-between gap-4 p-5 sm:p-6">
		<div class="min-w-0">
			<h2 class="font-semibold">Status page</h2>
			<p class="text-sm text-muted">
				{data.page.title} · {data.page.blocks.filter((b) => !b.hidden).length} blocks. Arrange them, write notices and pick what each one shows
				right on the page.
			</p>
		</div>
		<a href="/admin/page" class="btn-primary"><Icon name="layout" /> Open the editor</a>
	</section>

	{#if data.pelican}
		<PelicanSettings settings={data.pelican} channels={data.channels} {owner} />
	{/if}

	<KumaImport />
	<section class="card space-y-4 p-5 sm:p-6">
		<div>
			<h2 class="font-semibold">Embed</h2>
			<p class="text-muted">
				Drop the widget on your own site and visitors see a small notice whenever there is an active incident or maintenance. It stays
				invisible while everything is fine.
			</p>
		</div>
		<Snippet label="Status widget" code={widget} />
		<Snippet label="Overall status badge" preview="/badge" code={badgeMarkdown(origin, 'Status', '/badge', '')} />
		<p class="text-xs text-muted">
			Widget options: <code class="kbd">data-position</code> (bottom-right, bottom-left, top-right, top-left),
			<code class="kbd">data-upcoming="true"</code> to announce maintenance in advance, <code class="kbd">data-refresh</code> in seconds.
		</p>
	</section>

		<MetricsSettings {origin} />
	{/if}

	<SecuritySettings twoFactor={data.user.two_factor} />

	<form class="card p-5 sm:p-6" onsubmit={changePassword} novalidate>
		<h2 class="font-semibold">Password</h2>
		<p class="mb-5 text-sm text-muted">Changing it signs out every other session.</p>

		<div class="grid gap-4 sm:grid-cols-3">
			<Field label="Current password" id="current" error={passwordFields.current}>
				<input id="current" type="password" class="input" autocomplete="current-password" bind:value={current} aria-invalid={!!passwordFields.current} />
			</Field>
			<Field label="New password" id="next" error={passwordFields.next}>
				<input id="next" type="password" class="input" autocomplete="new-password" bind:value={next} aria-invalid={!!passwordFields.next} />
			</Field>
			<Field label="Repeat" id="repeat" error={passwordFields.repeat}>
				<input id="repeat" type="password" class="input" autocomplete="new-password" bind:value={repeat} aria-invalid={!!passwordFields.repeat} />
			</Field>
		</div>

		<div class="mt-6 flex justify-end">
			<button class="btn-secondary">Change password</button>
		</div>
	</form>
</div>
