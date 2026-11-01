<script lang="ts">
	import { api, message } from '#lib/api.ts';
	import Icon, { type IconName } from '#lib/components/Icon.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import { toast } from '#lib/toast.svelte.ts';
	import { goto, invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	let menuOpen = $state(false);

	type Item = { href: string; label: string; icon: IconName };
	const sections: { title: string; items: Item[] }[] = [
		{
			title: 'Monitoring',
			items: [
				{ href: '/admin', label: 'Monitors', icon: 'activity' },
				{ href: '/admin/incidents', label: 'Incidents', icon: 'incident' },
				{ href: '/admin/maintenance', label: 'Maintenance', icon: 'wrench' }
			]
		},
		{
			title: 'Configuration',
			items: [
				{ href: '/admin/channels', label: 'Notifications', icon: 'bell' },
				{ href: '/admin/settings', label: 'Settings', icon: 'settings' }
			]
		}
	];

	const current = (href: string) =>
		href === '/admin'
			? page.url.pathname === '/admin' || page.url.pathname.startsWith('/admin/monitors')
			: page.url.pathname.startsWith(href);

	const section = $derived(sections.flatMap((s) => s.items).find((i) => current(i.href))?.label ?? 'Admin');
	const initials = $derived(data.user.username.slice(0, 2).toUpperCase());

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
		class="fixed inset-y-0 left-0 z-40 flex w-60 flex-col bg-chrome text-chrome-ink transition-transform lg:sticky lg:top-0 lg:h-dvh lg:translate-x-0 {menuOpen ? 'translate-x-0' : '-translate-x-full'}"
	>
		<a href="/admin" class="flex h-14 items-center gap-2.5 border-b border-chrome-line px-5">
			<Logo size={22} class="text-chrome-ink" />
			<span class="font-semibold tracking-tight">heron</span>
			<span class="ml-auto rounded border border-chrome-line px-1.5 py-0.5 font-mono text-[10px] text-chrome-muted">{data.version}</span>
		</a>

		<nav class="flex-1 overflow-y-auto px-3 py-4">
			{#each sections as group (group.title)}
				<p class="px-2.5 pb-1.5 text-[10px] font-semibold tracking-[0.1em] text-chrome-muted/80 uppercase">{group.title}</p>
				<div class="mb-5 space-y-0.5">
					{#each group.items as item (item.href)}
						<a
							href={item.href}
							class="relative flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[13px] font-medium transition {current(item.href)
								? 'bg-white/8 text-chrome-ink'
								: 'text-chrome-muted hover:bg-white/5 hover:text-chrome-ink'}"
							aria-current={current(item.href) ? 'page' : undefined}
						>
							{#if current(item.href)}
								<span class="absolute top-1.5 bottom-1.5 -left-3 w-0.5 rounded-r bg-accent"></span>
							{/if}
							<Icon name={item.icon} size={15} />
							{item.label}
						</a>
					{/each}
				</div>
			{/each}
		</nav>

		<div class="border-t border-chrome-line p-3">
			<a
				href="/"
				target="_blank"
				class="flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[13px] text-chrome-muted hover:bg-white/5 hover:text-chrome-ink"
			>
				<Icon name="external" size={15} /> View status page
			</a>
		</div>
	</aside>

	{#if menuOpen}
		<button class="fixed inset-0 z-30 bg-black/40 lg:hidden" aria-label="Close menu" onclick={() => (menuOpen = false)}></button>
	{/if}

	<div class="min-w-0">
		<header class="sticky top-0 z-20 flex h-14 items-center gap-3 border-b border-line bg-surface/90 px-4 backdrop-blur sm:px-8">
			<button class="btn-ghost size-8 px-0 lg:hidden" onclick={() => (menuOpen = true)} aria-label="Open menu">
				<Icon name="menu" />
			</button>
			<nav class="flex items-center gap-2 text-[13px]" aria-label="Breadcrumb">
				<span class="text-muted">Admin</span>
				<Icon name="chevron" size={12} class="text-faint" />
				<span class="font-medium">{section}</span>
			</nav>
			<div class="ml-auto flex items-center gap-1">
				<ThemeToggle />
				<span class="mx-2 h-5 w-px bg-line"></span>
				<span class="grid size-7 place-items-center rounded-full bg-accent/15 text-[11px] font-semibold text-accent">{initials}</span>
				<span class="hidden px-2 text-[13px] font-medium sm:block">{data.user.username}</span>
				<button class="btn-ghost size-8 px-0" onclick={logout} aria-label="Sign out" title="Sign out">
					<Icon name="logout" size={15} />
				</button>
			</div>
		</header>
		<main class="mx-auto max-w-6xl px-4 py-8 sm:px-8">
			{@render children()}
		</main>
	</div>
</div>
