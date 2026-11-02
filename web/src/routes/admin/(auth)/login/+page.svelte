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

<form class="on-sheet space-y-5" onsubmit={submit}>
	<div>
		<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">Sign in</h1>
		<p class="mt-3 text-ink-2">Manage monitors, incidents and alerts.</p>
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
		<p class="flex items-center gap-2 text-[13px] font-semibold text-down"><i class="size-1.5 rounded-[1px] bg-down"></i>{error}</p>
	{/if}

	<button class="btn-primary h-[46px] w-full text-[15px]" disabled={busy}>{busy ? 'Signing in…' : 'Sign in'}</button>
</form>
