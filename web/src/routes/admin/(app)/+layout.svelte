<script lang="ts">
	import { api, message } from '#lib/api.ts';
	import Icon from '#lib/components/Icon.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import NavList, { type NavGroup } from '#lib/components/NavList.svelte';
	import { debounce, Live } from '#lib/live.svelte.ts';
	import { roleLabel } from '#lib/team.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import { goto, invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	let menuOpen = $state(false);

	const groups: NavGroup[] = $derived([
		{
			title: '',
			items: [
				{
					href: '/admin',
					label: 'Monitors',
					icon: 'activity',
					count: data.counts.monitors,
					alert: data.counts.down > 0
				},
				{
					href: '/admin/incidents',
					label: 'Incidents',
					icon: 'incident',
					count: data.counts.incidents,
					alert: data.counts.incidents > 0
				},
				{ href: '/admin/maintenance', label: 'Maintenance', icon: 'wrench' }
			]
		},
		{
			title: 'Configuration',
			items: [
				...(data.user.role === 'viewer'
					? []
					: [{ href: '/admin/channels', label: 'Notifications', icon: 'bell' } as const]),
				{ href: '/admin/people', label: 'People', icon: 'users' },
				{ href: '/admin/audit', label: 'Audit log', icon: 'history' },
				{ href: '/admin/settings', label: 'Settings', icon: 'settings' }
			]
		}
	]);

	const active = $derived.by(() => {
		const path = page.url.pathname;
		if (path === '/admin' || path.startsWith('/admin/monitors')) return '/admin';
		return groups.flatMap((g) => g.items).find((i) => i.href !== '/admin' && path.startsWith(i.href))?.href ?? '';
	});

	const tabs = $derived(groups[0]?.items.slice(0, 2) ?? []);

	const refresh = debounce(() => invalidate('heron:counts'), 1500);
	const live = new Live({ monitor: refresh, incident: refresh, layout: refresh }, true);

	async function logout() {
		try {
			await api.post('/auth/logout');
			await invalidate('heron:auth');
			await goto('/admin/login');
		} catch (err) {
			toast.error(message(err));
		}
	}

	$effect(() => {
		page.url.pathname;
		menuOpen = false;
	});
</script>

{#snippet liveState()}
	<span class="flex items-center gap-2 font-mono text-[11px] tracking-wider text-muted uppercase">
		<span class="sq {live.connected ? 'blink' : ''}" style="--c: var(--color-{live.connected ? 'up' : 'faint'}); --s: 8px"></span>
		{live.connected ? 'Live' : 'Offline'}
	</span>
{/snippet}

{#snippet sidebar(drawer: boolean)}
	<div class="flex h-full min-h-0 flex-col bg-surface px-3 pt-[18px] pb-3 {drawer ? 'rounded-r-card' : 'rounded-card'}">
		<div class="flex items-center px-2.5 pb-5">
			<a href="/admin" class="flex min-w-0 items-end gap-[11px]">
				<Logo size={30} class="text-ink" />
				<span class="truncate text-[23px] leading-5 font-bold tracking-[-0.04em]">heron</span>
			</a>
			<span class="ml-auto rounded-tag bg-ink px-1.5 py-0.5 font-mono text-[10px] font-semibold tracking-wider text-canvas uppercase">admin</span>
		</div>

		<div class="-mx-1 min-h-0 flex-1 overflow-y-auto px-1">
			<NavList {groups} {active} />
		</div>

		<div class="mt-3 flex flex-col gap-2.5">
			<div class="flex items-center justify-between px-3">
				{@render liveState()}
				{#if data.updates?.available && data.updates.latest}
					<a
						href="/admin/settings#updates"
						title="heron {data.updates.latest.version} is available"
						class="flex items-center gap-1.5 font-mono text-[11px] text-ink hover:underline"
					>
						<span class="sq" style="--c: var(--color-up); --s: 6px"></span>
						{data.updates.latest.version}
					</a>
				{:else}
					<span class="font-mono text-[11px] text-faint">{data.version}</span>
				{/if}
			</div>
			<a
				href="/"
				target="_blank"
				class="flex h-11 items-center justify-between rounded-ctl border border-line-strong pr-3 pl-3.5 text-[14px] font-semibold transition-colors hover:border-ink"
			>
				Status page <Icon name="external" size={15} />
			</a>
			<div class="grid grid-cols-[36px_minmax(0,1fr)_34px] items-center gap-2.5 border-t border-line pt-3 pr-1 pb-0.5 pl-1.5">
				<span class="grid size-9 place-items-center rounded-ctl bg-raised text-[13px] font-bold uppercase">{data.user.username.slice(0, 2)}</span>
				<div class="min-w-0">
					<div class="truncate text-[14px] leading-tight font-semibold">{data.user.username}</div>
					<div class="truncate text-[12.5px] text-faint">{roleLabel[data.user.role].toLowerCase()}</div>
				</div>
				<button
					onclick={logout}
					title="Sign out"
					aria-label="Sign out"
					class="grid size-[34px] place-items-center rounded-lg text-ink-2 transition-colors hover:bg-sunken hover:text-ink"
				>
					<Icon name="logout" size={17} />
				</button>
			</div>
		</div>
	</div>
{/snippet}

<div class="min-h-dvh lg:grid lg:grid-cols-[216px_minmax(0,1fr)] lg:gap-3 lg:p-3 xl:grid-cols-[248px_minmax(0,1fr)]">
	<aside class="sticky top-3 hidden h-[calc(100dvh-24px)] self-start lg:block">{@render sidebar(false)}</aside>

	<header class="flex h-[60px] items-center gap-2.5 px-3 lg:hidden">
		<a href="/admin" class="flex min-w-0 items-end gap-2.5">
			<Logo size={26} class="text-ink" />
			<span class="truncate text-[21px] leading-[18px] font-bold tracking-[-0.04em]">heron</span>
		</a>
		<span class="ml-auto">{@render liveState()}</span>
	</header>

	<main class="flex min-w-0 flex-col px-3 pt-2 pb-[calc(92px+env(safe-area-inset-bottom))] lg:pt-[26px] lg:pr-7 lg:pb-10 lg:pl-6">
		<div class="mx-auto w-full max-w-[1180px]">
			{@render children()}
		</div>
	</main>

	<nav class="fixed inset-x-0 bottom-0 z-30 grid grid-cols-3 gap-1.5 border-t border-line bg-surface px-3 pt-2 pb-[calc(8px+env(safe-area-inset-bottom))] lg:hidden">
		{#each tabs as tab (tab.href)}
			{@const on = active === tab.href && !menuOpen}
			<a
				href={tab.href}
				class="flex h-[54px] flex-col items-center justify-center gap-[3px] rounded-ctl text-[12px] font-semibold transition-colors {on
					? 'bg-ink text-canvas'
					: 'text-ink-2 hover:bg-sunken hover:text-ink'}"
			>
				<Icon name={tab.icon} size={20} />
				{tab.label}
			</a>
		{/each}
		<button
			type="button"
			onclick={() => (menuOpen = true)}
			class="flex h-[54px] flex-col items-center justify-center gap-[3px] rounded-ctl text-[12px] font-semibold transition-colors {menuOpen
				? 'bg-ink text-canvas'
				: 'text-ink-2 hover:bg-sunken hover:text-ink'}"
		>
			<Icon name="menu" size={20} />
			Menu
		</button>
	</nav>

	{#if menuOpen}
		<div class="fixed inset-0 z-40 bg-black/70 lg:hidden" role="presentation" onclick={() => (menuOpen = false)}>
			<aside class="h-full w-full max-w-[360px]" role="presentation" onclick={(e) => e.stopPropagation()}>
				{@render sidebar(true)}
			</aside>
		</div>
	{/if}
</div>
