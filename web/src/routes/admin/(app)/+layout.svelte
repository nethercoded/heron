<script lang="ts">
	import { api, message } from '#lib/api.ts';
	import Icon, { type IconName } from '#lib/components/Icon.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import { toast } from '#lib/toast.svelte.ts';
	import { goto, invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	let menuOpen = $state(false);

	const nav: { href: string; label: string; icon: IconName }[] = [
		{ href: '/admin', label: 'Monitors', icon: 'activity' },
		{ href: '/admin/incidents', label: 'Incidents', icon: 'incident' },
		{ href: '/admin/channels', label: 'Notifications', icon: 'bell' },
		{ href: '/admin/settings', label: 'Settings', icon: 'settings' }
	];

	const current = (href: string) =>
		href === '/admin'
			? page.url.pathname === '/admin' || page.url.pathname.startsWith('/admin/monitors')
			: page.url.pathname.startsWith(href);

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

<div class="min-h-dvh lg:grid lg:grid-cols-[15rem_1fr]">
	<aside
		class="fixed inset-y-0 left-0 z-40 flex w-60 flex-col border-r border-line bg-surface transition-transform lg:sticky lg:top-0 lg:h-dvh lg:translate-x-0 {menuOpen ? 'translate-x-0' : '-translate-x-full'}"
	>
		<a href="/admin" class="flex h-16 items-center gap-2.5 px-5">
			<img src="/favicon.svg" alt="" class="size-7" />
			<span class="font-semibold tracking-tight">heron</span>
			<span class="rounded bg-sunken px-1.5 py-0.5 font-mono text-[10px] text-muted">{data.version}</span>
		</a>

		<nav class="flex-1 space-y-0.5 px-3 py-2">
			{#each nav as item (item.href)}
				<a
					href={item.href}
					class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition {current(item.href)
						? 'bg-sunken text-ink'
						: 'text-muted hover:bg-sunken/60 hover:text-ink'}"
					aria-current={current(item.href) ? 'page' : undefined}
				>
					<Icon name={item.icon} />
					{item.label}
				</a>
			{/each}
		</nav>

		<div class="border-t border-line p-3">
			<a href="/" target="_blank" class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-muted hover:bg-sunken/60 hover:text-ink">
				<Icon name="external" /> Public page
			</a>
			<div class="mt-1 flex items-center justify-between rounded-lg px-3 py-1.5">
				<span class="truncate text-sm text-muted">{data.user.username}</span>
				<span class="flex items-center">
					<ThemeToggle />
					<button class="btn-ghost size-9 px-0" onclick={logout} aria-label="Sign out" title="Sign out">
						<Icon name="logout" />
					</button>
				</span>
			</div>
		</div>
	</aside>

	{#if menuOpen}
		<button class="fixed inset-0 z-30 bg-black/30 lg:hidden" aria-label="Close menu" onclick={() => (menuOpen = false)}></button>
	{/if}

	<div class="min-w-0">
		<header class="sticky top-0 z-20 flex h-14 items-center gap-3 border-b border-line bg-surface/80 px-4 backdrop-blur lg:hidden">
			<button class="btn-ghost size-9 px-0" onclick={() => (menuOpen = true)} aria-label="Open menu">
				<Icon name="menu" />
			</button>
			<span class="font-semibold">heron</span>
		</header>
		<main class="mx-auto max-w-5xl px-4 py-8 sm:px-8">
			{@render children()}
		</main>
	</div>
</div>
