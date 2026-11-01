<script lang="ts">
	import { ApiError, api, message } from '#lib/api.ts';
	import { goto, invalidate } from '$app/navigation';

	let username = $state('');
	let password = $state('');
	let error = $state('');
	let busy = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		try {
			await api.post('/auth/login', { username, password });
			await invalidate('heron:auth');
			await goto('/admin', { replaceState: true });
		} catch (err) {
			error = err instanceof ApiError && err.status === 401 ? 'Wrong username or password' : message(err);
			password = '';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Sign in · heron</title>
</svelte:head>

<form class="card space-y-4 p-6" onsubmit={submit}>
	<div>
		<h1 class="text-lg font-semibold">Sign in</h1>
		<p class="text-sm text-muted">Manage monitors, incidents and alerts.</p>
	</div>

	<div>
		<label for="username" class="label">Username</label>
		<input id="username" class="input" autocomplete="username" required bind:value={username} />
	</div>
	<div>
		<label for="password" class="label">Password</label>
		<input id="password" type="password" class="input" autocomplete="current-password" required bind:value={password} />
	</div>

	{#if error}
		<p class="rounded-lg bg-down/10 px-3 py-2 text-sm text-down">{error}</p>
	{/if}

	<button class="btn-primary w-full" disabled={busy}>{busy ? 'Signing in…' : 'Sign in'}</button>
</form>
