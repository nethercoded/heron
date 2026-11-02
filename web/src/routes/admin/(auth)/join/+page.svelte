<script lang="ts">
	import { onMount } from 'svelte';
	import { ApiError, api, message } from '#lib/api.ts';
	import Field from '#lib/components/Field.svelte';
	import { roleLabel } from '#lib/team.ts';
	import type { Invite } from '#lib/types.ts';
	import { goto, invalidate } from '$app/navigation';

	let token = $state('');
	let invite = $state<Invite | null>(null);
	let invalid = $state('');
	let username = $state('');
	let password = $state('');
	let repeat = $state('');
	let fields = $state<Record<string, string>>({});
	let error = $state('');
	let busy = $state(false);

	onMount(async () => {
		token = decodeURIComponent(location.hash.slice(1));
		history.replaceState(history.state, '', location.pathname);
		if (!token) {
			invalid = 'This page needs the invite link you were sent.';
			return;
		}
		try {
			invite = await api.post<Invite>('/auth/invite', { token });
		} catch (err) {
			invalid = message(err);
		}
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		fields = {};
		error = '';
		if (password !== repeat) {
			fields = { repeat: 'Passwords do not match' };
			return;
		}
		busy = true;
		try {
			await api.post('/auth/join', { token, username, password });
			await invalidate('heron:auth');
			await goto('/admin', { replaceState: true });
		} catch (err) {
			if (err instanceof ApiError) fields = err.fields;
			if (err instanceof ApiError && err.code === 'invite_invalid') invalid = err.message;
			else error = message(err);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Join · heron</title>
</svelte:head>

{#if invalid}
	<div class="on-sheet space-y-5">
		<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">Invite unavailable</h1>
		<p class="text-ink-2">{invalid} Ask whoever invited you for a fresh link.</p>
		<a href="/admin/login" class="btn-secondary h-[46px] w-full text-[15px]">Go to sign in</a>
	</div>
{:else if invite}
	<form class="on-sheet space-y-5" onsubmit={submit} novalidate>
		<div>
			<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">Join heron</h1>
			<p class="mt-3 text-ink-2">
				{invite.created_by || 'The owner'} invited you as <span class="font-semibold text-ink">{roleLabel[invite.role].toLowerCase()}</span>.
				Pick a username and password.
			</p>
		</div>

		<Field label="Username" id="username" error={fields.username}>
			<input id="username" class="input" autocomplete="username" required bind:value={username} aria-invalid={!!fields.username} />
		</Field>
		<Field label="Password" id="password" hint="At least 10 characters" error={fields.password}>
			<input id="password" type="password" class="input" autocomplete="new-password" required bind:value={password} aria-invalid={!!fields.password} />
		</Field>
		<Field label="Repeat password" id="repeat" error={fields.repeat}>
			<input id="repeat" type="password" class="input" autocomplete="new-password" required bind:value={repeat} aria-invalid={!!fields.repeat} />
		</Field>

		{#if error}
			<p class="flex items-center gap-2 text-[13px] font-semibold text-down"><i class="size-1.5 rounded-[1px] bg-down"></i>{error}</p>
		{/if}

		<button class="btn-primary h-[46px] w-full text-[15px]" disabled={busy}>{busy ? 'Creating account…' : 'Create account'}</button>
	</form>
{:else}
	<p class="on-sheet text-ink-2">Checking the invite…</p>
{/if}
