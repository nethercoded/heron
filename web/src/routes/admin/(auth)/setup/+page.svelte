<script lang="ts">
	import { ApiError, api, message } from '#lib/api.ts';
	import Field from '#lib/components/Field.svelte';
	import { goto, invalidate } from '$app/navigation';

	let username = $state('admin');
	let password = $state('');
	let confirm = $state('');
	let fields = $state<Record<string, string>>({});
	let error = $state('');
	let busy = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		fields = {};
		error = '';
		if (password !== confirm) {
			fields = { confirm: 'Passwords do not match' };
			return;
		}
		busy = true;
		try {
			await api.post('/auth/setup', { username, password });
			await invalidate('heron:auth');
			await goto('/admin', { replaceState: true });
		} catch (err) {
			if (err instanceof ApiError) fields = err.fields;
			error = message(err);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Welcome · heron</title>
</svelte:head>

<form class="card space-y-4 p-6" onsubmit={submit}>
	<div>
		<h1 class="text-lg font-semibold">Welcome to heron</h1>
		<p class="text-sm text-muted">Create the administrator account to get started.</p>
	</div>

	<Field label="Username" id="username" error={fields.username}>
		<input id="username" class="input" autocomplete="username" required bind:value={username} aria-invalid={!!fields.username} />
	</Field>
	<Field label="Password" id="password" hint="At least 10 characters" error={fields.password}>
		<input id="password" type="password" class="input" autocomplete="new-password" required bind:value={password} aria-invalid={!!fields.password} />
	</Field>
	<Field label="Repeat password" id="confirm" error={fields.confirm}>
		<input id="confirm" type="password" class="input" autocomplete="new-password" required bind:value={confirm} aria-invalid={!!fields.confirm} />
	</Field>

	{#if error && Object.keys(fields).length === 0}
		<p class="rounded-lg bg-down/10 px-3 py-2 text-sm text-down">{error}</p>
	{/if}

	<button class="btn-primary w-full" disabled={busy}>{busy ? 'Creating…' : 'Create account'}</button>
</form>
