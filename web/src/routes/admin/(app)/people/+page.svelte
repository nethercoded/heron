<script lang="ts">
	import { api, message } from '#lib/api.ts';
	import Icon from '#lib/components/Icon.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import Snippet from '#lib/components/Snippet.svelte';
	import { ago, formatDateTime } from '#lib/format.ts';
	import { roleHint, roleLabel } from '#lib/team.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Invite, Member, Role } from '#lib/types.ts';
	import { invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const owner = $derived(data.user.role === 'owner');
	const reload = () => invalidate('heron:people');

	let inviteRole = $state<Role>('admin');
	let fresh = $state<{ link: string; role: Role } | null>(null);
	let busy = $state(false);

	async function createInvite() {
		busy = true;
		try {
			const inv = await api.post<Invite>('/admin/invites', { role: inviteRole });
			fresh = { link: `${page.url.origin}/admin/join#${inv.token}`, role: inv.role };
			await reload();
		} catch (err) {
			toast.error(message(err));
		} finally {
			busy = false;
		}
	}

	async function act(run: () => Promise<unknown>, done: string) {
		try {
			await run();
			toast.success(done);
			await reload();
		} catch (err) {
			toast.error(message(err));
		}
	}

	const setRole = (m: Member, role: Role) =>
		act(() => api.put(`/admin/users/${m.id}`, { role }), `${m.username} is now ${roleLabel[role].toLowerCase()}`);

	function remove(m: Member) {
		if (!confirm(`Remove ${m.username}? They are signed out right away.`)) return;
		act(() => api.delete(`/admin/users/${m.id}`), `${m.username} removed`);
	}

	function signOut(m: Member) {
		if (!confirm(`Sign ${m.username} out on every device?`)) return;
		act(() => api.delete(`/admin/users/${m.id}/sessions`), `${m.username} signed out everywhere`);
	}

	function resetTwoFactor(m: Member) {
		if (!confirm(`Turn off two-factor sign in for ${m.username}? Use this when they lost their phone.`)) return;
		act(() => api.delete(`/admin/users/${m.id}/two-factor`), `Two-factor reset for ${m.username}`);
	}

	const revoke = (inv: Invite) => act(() => api.delete(`/admin/invites/${inv.id}`), 'Invite revoked');
</script>

<PageHeader title="People" description="Everyone who can sign in to this admin, and what they are allowed to do." />

{#if owner}
	<section class="card mb-6 p-5 sm:p-6">
		<div class="flex flex-wrap items-end justify-between gap-4">
			<div class="min-w-0">
				<h2 class="font-semibold">Invite someone</h2>
				<p class="mt-1 text-sm text-muted">heron makes a one-time link that works for 72 hours. Send it any way you like.</p>
			</div>
			<div class="flex w-full items-center gap-2 sm:w-auto">
				<select class="input-select h-10 sm:w-36" bind:value={inviteRole} aria-label="Role">
					<option value="admin">Admin</option>
					<option value="viewer">Viewer</option>
				</select>
				<button class="btn-primary shrink-0" disabled={busy} onclick={createInvite}><Icon name="plus" /> Create link</button>
			</div>
		</div>
		<p class="mt-3 text-[13px] text-faint">{roleHint[inviteRole]}.</p>

		{#if fresh}
			<div class="mt-5 border-t border-line pt-5">
				<p class="mb-2 text-sm text-ink-2">
					Link for a new <span class="font-semibold text-ink">{roleLabel[fresh.role].toLowerCase()}</span>. It is shown only once.
				</p>
				<Snippet label="Invite link" code={fresh.link} />
			</div>
		{/if}
	</section>
{/if}

<section class="card overflow-hidden">
	<header class="flex items-center gap-3 px-[22px] pt-[18px] pb-4">
		<h2 class="text-[22px] leading-tight font-semibold tracking-[-0.015em]">Members</h2>
		<span class="text-[15px] font-medium text-faint">{data.team.users.length}</span>
	</header>
	<ul class="divide-y divide-line border-t border-line">
		{#each data.team.users as m (m.id)}
			{@const self = m.id === data.user.id}
			<li class="flex flex-wrap items-center gap-x-4 gap-y-3 px-[22px] py-4">
				<span class="grid size-10 shrink-0 place-items-center rounded-ctl bg-raised text-[13px] font-bold uppercase">{m.username.slice(0, 2)}</span>
				<div class="min-w-0 flex-1 basis-48">
					<p class="flex flex-wrap items-center gap-2 text-[15px] font-semibold">
						<span class="truncate">{m.username}</span>
						{#if self}<span class="chip text-faint ring-line-strong">you</span>{/if}
						{#if m.two_factor || m.passkeys > 0}
							<span class="chip text-up ring-up/30" title={[m.two_factor && '2FA app', m.passkeys > 0 && `${m.passkeys} passkey${m.passkeys > 1 ? 's' : ''}`].filter(Boolean).join(', ')}>
								<Icon name="shield" size={11} /> secured
							</span>
						{/if}
					</p>
					<p class="mt-0.5 text-[13px] text-faint">
						{m.last_seen_at ? `signed in ${ago(m.last_seen_at)}` : 'never signed in'}
						· {m.sessions} active session{m.sessions === 1 ? '' : 's'}
						· joined <span title={formatDateTime(m.created_at)}>{ago(m.created_at)}</span>
					</p>
				</div>

				{#if owner && m.role !== 'owner'}
					<div class="flex w-full items-center gap-2 sm:w-auto">
						<select
							class="input-select h-9 text-[13px] sm:w-32"
							value={m.role}
							onchange={(e) => setRole(m, e.currentTarget.value as Role)}
							aria-label="Role of {m.username}"
						>
							<option value="admin">Admin</option>
							<option value="viewer">Viewer</option>
						</select>
						<button class="btn-ghost btn-sm" title="Sign out everywhere" aria-label="Sign out everywhere" onclick={() => signOut(m)} disabled={m.sessions === 0}>
							<Icon name="logout" size={14} />
						</button>
						{#if m.two_factor}
							<button class="btn-ghost btn-sm" title="Reset two-factor" aria-label="Reset two-factor" onclick={() => resetTwoFactor(m)}>
								<Icon name="shield" size={14} />
							</button>
						{/if}
						<button class="btn-ghost btn-sm text-down" title="Remove" aria-label="Remove {m.username}" onclick={() => remove(m)}>
							<Icon name="trash" size={14} />
						</button>
					</div>
				{:else}
					<span class="rounded-tag px-2 py-1 font-mono text-[11px] font-semibold tracking-wider uppercase {m.role === 'owner' ? 'bg-ink text-canvas' : 'bg-sunken text-ink-2'}">
						{roleLabel[m.role]}
					</span>
				{/if}
			</li>
		{/each}
	</ul>
</section>

{#if owner && data.team.invites.length > 0}
	<section class="card mt-6 overflow-hidden">
		<header class="flex items-center gap-3 px-[22px] pt-[18px] pb-4">
			<h2 class="text-[22px] leading-tight font-semibold tracking-[-0.015em]">Pending invites</h2>
			<span class="text-[15px] font-medium text-faint">{data.team.invites.length}</span>
		</header>
		<ul class="divide-y divide-line border-t border-line">
			{#each data.team.invites as inv (inv.id)}
				<li class="flex items-center gap-4 px-[22px] py-3.5">
					<span class="grid size-9 shrink-0 place-items-center rounded-ctl bg-sunken text-ink-2"><Icon name="link" size={15} /></span>
					<div class="min-w-0 flex-1">
						<p class="text-[14px] font-semibold">{roleLabel[inv.role]}</p>
						<p class="text-[13px] text-faint">created {ago(inv.created_at)} by {inv.created_by || 'someone'} · expires {formatDateTime(inv.expires_at)}</p>
					</div>
					<button class="btn-ghost btn-sm" onclick={() => revoke(inv)}>Revoke</button>
				</li>
			{/each}
		</ul>
	</section>
{/if}
