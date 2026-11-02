<script lang="ts">
	import { ApiError, api, message } from '#lib/api.ts';
	import Icon from '#lib/components/Icon.svelte';
	import { passkeyCancelled, passkeysSupported, signInWithPasskey } from '#lib/passkey.ts';
	import type { LoginResult } from '#lib/types.ts';
	import { goto, invalidate } from '$app/navigation';

	let username = $state('');
	let password = $state('');
	let code = $state('');
	let ticket = $state('');
	let error = $state('');
	let busy = $state(false);
	let codeInput = $state<HTMLInputElement>();

	const canPasskey = passkeysSupported();

	async function enter() {
		await invalidate('heron:auth');
		await goto('/admin', { replaceState: true });
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		try {
			const res = await api.post<LoginResult>('/auth/login', { username, password });
			if (res.ticket) {
				ticket = res.ticket;
				queueMicrotask(() => codeInput?.focus());
				return;
			}
			await enter();
		} catch (err) {
			error = err instanceof ApiError && err.status === 401 ? 'Wrong username or password' : message(err);
			password = '';
		} finally {
			busy = false;
		}
	}

	async function verify(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		try {
			await api.post<LoginResult>('/auth/login/code', { ticket, code });
			await enter();
		} catch (err) {
			if (err instanceof ApiError && err.code === 'ticket_expired') {
				ticket = '';
				password = '';
			}
			error =
				err instanceof ApiError && err.fields.code ? 'That code did not work, wait for the next one' : message(err);
			code = '';
		} finally {
			busy = false;
		}
	}

	async function passkey() {
		busy = true;
		error = '';
		try {
			await signInWithPasskey();
			await enter();
		} catch (err) {
			if (!passkeyCancelled(err)) error = message(err);
		} finally {
			busy = false;
		}
	}

	function back() {
		ticket = '';
		code = '';
		password = '';
		error = '';
	}
</script>

<svelte:head>
	<title>Sign in · heron</title>
</svelte:head>

{#snippet problem()}
	{#if error}
		<p class="flex items-center gap-2 text-[13px] font-semibold text-down"><i class="size-1.5 rounded-[1px] bg-down"></i>{error}</p>
	{/if}
{/snippet}

{#if ticket}
	<form class="on-sheet space-y-5" onsubmit={verify}>
		<div>
			<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">One more step</h1>
			<p class="mt-3 text-ink-2">Enter the 6-digit code from your authenticator app.</p>
		</div>

		<div>
			<label for="code" class="label">Code</label>
			<input
				id="code"
				class="input text-center font-mono text-[22px] tracking-[0.4em]"
				inputmode="numeric"
				autocomplete="one-time-code"
				maxlength="6"
				required
				bind:this={codeInput}
				bind:value={code}
				oninput={(e) => e.currentTarget.value.length === 6 && e.currentTarget.form?.requestSubmit()}
			/>
		</div>

		{@render problem()}

		<button class="btn-primary h-[46px] w-full text-[15px]" disabled={busy || code.length !== 6}>{busy ? 'Checking…' : 'Verify'}</button>
		<button type="button" class="w-full text-center text-[13px] font-medium text-muted hover:text-ink" onclick={back}>Use another account</button>
	</form>
{:else}
	<form class="on-sheet space-y-5" onsubmit={submit}>
		<div>
			<h1 class="text-[40px] leading-none font-semibold tracking-[-0.035em]">Sign in</h1>
			<p class="mt-3 text-ink-2">Manage monitors, incidents and alerts.</p>
		</div>

		<div>
			<label for="username" class="label">Username</label>
			<input id="username" class="input" autocomplete="username webauthn" required bind:value={username} />
		</div>
		<div>
			<label for="password" class="label">Password</label>
			<input id="password" type="password" class="input" autocomplete="current-password" required bind:value={password} />
		</div>

		{@render problem()}

		<button class="btn-primary h-[46px] w-full text-[15px]" disabled={busy}>{busy ? 'Signing in…' : 'Sign in'}</button>

		{#if canPasskey}
			<div class="flex items-center gap-3 text-[12px] text-faint">
				<span class="h-px flex-1 bg-line"></span>or<span class="h-px flex-1 bg-line"></span>
			</div>
			<button type="button" class="btn-secondary h-[46px] w-full text-[15px]" disabled={busy} onclick={passkey}>
				<Icon name="key" size={16} /> Sign in with a passkey
			</button>
		{/if}
	</form>
{/if}
