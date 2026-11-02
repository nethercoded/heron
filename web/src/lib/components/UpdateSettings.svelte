<script lang="ts">
	import { untrack } from 'svelte';
	import { api, message } from '#lib/api.ts';
	import { ago, formatDate } from '#lib/format.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Updates } from '#lib/types.ts';
	import { invalidate } from '$app/navigation';
	import Toggle from './Toggle.svelte';

	let { updates: initial }: { updates: Updates } = $props();

	let updates = $state(untrack(() => initial));
	let checking = $state(false);
	let installing = $state(false);

	const latest = $derived(updates.latest);
	const changes = $derived(
		(latest?.notes ?? '')
			.split('\n')
			.map((line) => line.trim())
			.filter((line) => line && !line.startsWith('#'))
			.map((line) =>
				line
					.replace(/^[*-]\s+/, '')
					.replace(/^[0-9a-f]{7,40}:\s+/, '')
					.replace(/\s+\(@[\w-]+\)$/, '')
			)
	);

	async function check() {
		checking = true;
		try {
			updates = await api.post<Updates>('/admin/updates/check');
			if (updates.error) toast.error('Could not reach GitHub, try again later');
			else if (!updates.available) toast.success(`heron ${updates.current} is the latest version`);
			await invalidate('heron:updates');
		} catch (err) {
			toast.error(message(err));
		} finally {
			checking = false;
		}
	}

	async function setAuto(auto: boolean) {
		try {
			updates = await api.put<Updates>('/admin/updates', { auto });
			toast.success(auto ? 'heron will update itself' : 'Automatic updates are off');
		} catch (err) {
			updates = { ...updates, auto: !auto };
			toast.error(message(err));
		}
	}

	async function install() {
		if (!latest) return;
		if (
			!confirm(
				`Install heron ${latest.version}? The panel and the status page go offline for a few seconds while heron restarts.`
			)
		)
			return;
		const target = latest.version;
		installing = true;
		try {
			await api.post('/admin/updates/install');
		} catch (err) {
			installing = false;
			toast.error(message(err));
			return;
		}

		const deadline = Date.now() + 5 * 60_000;
		while (Date.now() < deadline) {
			await new Promise((resolve) => setTimeout(resolve, 2000));
			const health = await fetch('/healthz')
				.then((res) => (res.ok ? res.json() : null))
				.catch(() => null);
			if (health?.version === target) {
				toast.success(`heron ${target} is running`);
				location.reload();
				return;
			}
			if (!health) continue;
			const state = await api.get<Updates>('/admin/updates').catch(() => null);
			if (state && !state.busy && state.failed) {
				updates = state;
				installing = false;
				toast.error('The update failed, heron keeps running the current version');
				return;
			}
		}
		installing = false;
		toast.error('heron did not come back on the new version, check the server logs');
	}
</script>

<section id="updates" class="card space-y-5 p-5 sm:p-6">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div class="min-w-0">
			<h2 class="font-semibold">Updates</h2>
			<p class="text-sm text-muted">
				{#if updates.available && latest}
					heron {latest.version} is out, this server runs {updates.current}.
				{:else if latest}
					You run the latest version, {updates.current}.
				{:else}
					This server runs heron {updates.current}.
				{/if}
				<span class="text-faint">Checked {ago(updates.checked_at)}.</span>
			</p>
		</div>
		<div class="flex gap-2">
			<button class="btn-ghost btn-sm" disabled={checking || installing} onclick={check}>{checking ? 'Checking…' : 'Check now'}</button>
			{#if updates.available && !updates.blocked}
				<button class="btn-primary btn-sm" disabled={installing || updates.busy} onclick={install}>
					{installing || updates.busy ? 'Updating…' : `Update to ${latest?.version}`}
				</button>
			{/if}
		</div>
	</div>

	{#if updates.available && latest}
		<div class="rounded-ctl border border-line p-4">
			<div class="flex flex-wrap items-baseline justify-between gap-2 text-sm">
				<span class="font-semibold">heron {latest.version}</span>
				<span class="text-faint">
					{formatDate(latest.published_at)} · <a class="text-ink underline" href={latest.url} target="_blank" rel="noreferrer">Release on GitHub</a>
				</span>
			</div>
			{#if changes.length > 0}
				<ul class="mt-3 max-h-56 list-disc space-y-1 overflow-y-auto pl-5 text-[13px] text-muted">
					{#each changes as change, i (i)}<li>{change}</li>{/each}
				</ul>
			{/if}
		</div>
	{/if}

	{#if updates.blocked}
		<p class="text-sm text-muted">heron cannot install updates here: {updates.blocked}.</p>
	{/if}
	{#if updates.failed}
		<p class="text-sm text-down">The last update failed: {updates.failed}</p>
	{/if}
	{#if updates.error}
		<p class="text-sm text-down">Could not reach GitHub on the last check.</p>
	{/if}

	<div class="border-t border-line pt-5">
		<Toggle
			bind:checked={updates.auto}
			label="Install updates automatically"
			description="heron looks for a new release on GitHub every six hours, verifies the checksum and restarts itself on it. Expect a few seconds of downtime."
			onchange={setAuto}
		/>
	</div>
</section>
