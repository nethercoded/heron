<script lang="ts">
	import { onMount } from 'svelte';
	import { ApiError, api, message } from '#lib/api.ts';
	import { ago } from '#lib/format.ts';
	import { deviceName, passkeyCancelled, passkeysSupported, registerPasskey } from '#lib/passkey.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Passkey } from '#lib/types.ts';
	import { invalidate } from '$app/navigation';
	import Field from './Field.svelte';
	import Icon from './Icon.svelte';

	let { twoFactor }: { twoFactor: boolean } = $props();

	let enrolling = $state<{ secret: string; uri: string; qr: string } | null>(null);
	let code = $state('');
	let codeError = $state('');
	let disabling = $state(false);
	let busy = $state(false);

	let passkeys = $state<Passkey[]>([]);
	const canPasskey = passkeysSupported();

	onMount(loadPasskeys);

	async function loadPasskeys() {
		try {
			passkeys = await api.get<Passkey[]>('/admin/account/passkeys');
		} catch (err) {
			toast.error(message(err));
		}
	}

	async function startTOTP() {
		try {
			enrolling = await api.post('/admin/account/totp');
			code = '';
			codeError = '';
		} catch (err) {
			toast.error(message(err));
		}
	}

	async function confirmTOTP(event: SubmitEvent) {
		event.preventDefault();
		if (!enrolling) return;
		busy = true;
		codeError = '';
		try {
			await api.put('/admin/account/totp', { secret: enrolling.secret, code });
			enrolling = null;
			toast.success('Two-factor sign in is on');
			await invalidate('heron:auth');
		} catch (err) {
			codeError = err instanceof ApiError ? (err.fields.code ?? message(err)) : message(err);
		} finally {
			busy = false;
		}
	}

	async function turnOff(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		codeError = '';
		try {
			await api.post('/admin/account/totp/disable', { code });
			disabling = false;
			code = '';
			toast.success('Two-factor sign in is off');
			await invalidate('heron:auth');
		} catch (err) {
			codeError = err instanceof ApiError ? (err.fields.code ?? message(err)) : message(err);
		} finally {
			busy = false;
		}
	}

	async function addPasskey() {
		busy = true;
		try {
			const key = await registerPasskey(deviceName());
			passkeys = [...passkeys, key];
			toast.success(`Passkey “${key.name}” added`);
		} catch (err) {
			if (!passkeyCancelled(err)) toast.error(message(err));
		} finally {
			busy = false;
		}
	}

	async function removePasskey(key: Passkey) {
		if (!confirm(`Remove the passkey “${key.name}”?`)) return;
		try {
			await api.delete(`/admin/account/passkeys/${key.id}`);
			passkeys = passkeys.filter((p) => p.id !== key.id);
			toast.success('Passkey removed');
		} catch (err) {
			toast.error(message(err));
		}
	}
</script>

<section class="card p-5 sm:p-6">
	<h2 class="font-semibold">Sign-in security</h2>
	<p class="mb-5 text-sm text-muted">The admin is on the internet, so a password alone is thin protection.</p>

	<div class="space-y-4">
		<div class="rounded-ctl border border-line p-4">
			<div class="flex flex-wrap items-center gap-3">
				<span class="grid size-10 shrink-0 place-items-center rounded-lg bg-sunken {twoFactor ? 'text-up' : 'text-muted'}"><Icon name="shield" /></span>
				<div class="min-w-0 flex-1">
					<p class="font-medium">Authenticator app</p>
					<p class="text-[13px] text-muted">
						{twoFactor ? 'On. heron asks for a 6-digit code after your password.' : 'Google Authenticator, 1Password, Aegis or any TOTP app.'}
					</p>
				</div>
				{#if twoFactor && !disabling}
					<button class="btn-ghost btn-sm" onclick={() => ((disabling = true), (code = ''), (codeError = ''))}>Turn off</button>
				{:else if !twoFactor && !enrolling}
					<button class="btn-secondary btn-sm" onclick={startTOTP}>Set up</button>
				{/if}
			</div>

			{#if enrolling}
				<form class="mt-5 grid gap-5 border-t border-line pt-5 sm:grid-cols-[176px_1fr]" onsubmit={confirmTOTP}>
					<div class="size-44 overflow-hidden rounded-lg bg-white p-1.5">{@html enrolling.qr}</div>
					<div class="min-w-0 space-y-4">
						<p class="text-sm text-ink-2">Scan the code with your app, or enter the key by hand, then type the code it shows.</p>
						<p class="font-mono text-[13px] break-all text-ink select-all">{enrolling.secret.match(/.{1,4}/g)?.join(' ')}</p>
						<Field label="Code from the app" id="totp-code" error={codeError}>
							<input id="totp-code" class="input font-mono tracking-[0.3em] sm:w-44" inputmode="numeric" autocomplete="one-time-code" maxlength="6" bind:value={code} aria-invalid={!!codeError} />
						</Field>
						<div class="flex gap-2">
							<button class="btn-primary" disabled={busy || code.length !== 6}>Turn on</button>
							<button type="button" class="btn-ghost" onclick={() => (enrolling = null)}>Cancel</button>
						</div>
					</div>
				</form>
			{/if}

			{#if disabling}
				<form class="mt-5 flex flex-wrap items-end gap-3 border-t border-line pt-5" onsubmit={turnOff}>
					<Field label="Current code" id="totp-off" error={codeError}>
						<input id="totp-off" class="input font-mono tracking-[0.3em] sm:w-44" inputmode="numeric" autocomplete="one-time-code" maxlength="6" bind:value={code} aria-invalid={!!codeError} />
					</Field>
					<button class="btn-danger" disabled={busy || code.length !== 6}>Turn off</button>
					<button type="button" class="btn-ghost" onclick={() => (disabling = false)}>Cancel</button>
				</form>
			{/if}
		</div>

		<div class="rounded-ctl border border-line p-4">
			<div class="flex flex-wrap items-center gap-3">
				<span class="grid size-10 shrink-0 place-items-center rounded-lg bg-sunken {passkeys.length ? 'text-up' : 'text-muted'}"><Icon name="key" /></span>
				<div class="min-w-0 flex-1">
					<p class="font-medium">Passkeys</p>
					<p class="text-[13px] text-muted">Sign in with Face ID, Touch ID, Windows Hello or a security key. No password, nothing to phish.</p>
				</div>
				{#if canPasskey}
					<button class="btn-secondary btn-sm" disabled={busy} onclick={addPasskey}><Icon name="plus" size={14} /> Add passkey</button>
				{/if}
			</div>
			{#if !canPasskey}
				<p class="mt-3 text-[13px] text-faint">This browser cannot create passkeys.</p>
			{/if}
			{#if passkeys.length}
				<ul class="mt-4 divide-y divide-line border-t border-line">
					{#each passkeys as key (key.id)}
						<li class="flex items-center gap-3 py-3">
							<div class="min-w-0 flex-1">
								<p class="truncate text-[14px] font-medium">{key.name}</p>
								<p class="text-[12.5px] text-faint">added {ago(key.created_at)} · {key.last_used_at ? `used ${ago(key.last_used_at)}` : 'not used yet'}</p>
							</div>
							<button class="btn-ghost btn-sm text-down" aria-label="Remove {key.name}" onclick={() => removePasskey(key)}><Icon name="trash" size={14} /></button>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	</div>
</section>
