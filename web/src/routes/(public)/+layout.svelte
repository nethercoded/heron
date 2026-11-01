<script lang="ts">
	import { untrack } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import { StatusStore, setStatus } from '#lib/status-page.svelte.ts';
	import { page } from '$app/state';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	const store = new StatusStore(untrack(() => data.status));
	setStatus(store);

	const info = $derived(store.page);
	const nav = [
		{ href: '/', label: 'Overview' },
		{ href: '/incidents', label: 'Incident history' }
	];
	const active = (href: string) =>
		href === '/'
			? page.url.pathname === '/' || page.url.pathname.startsWith('/monitor')
			: page.url.pathname.startsWith(href);
</script>

<svelte:head>
	<title>{info.title}</title>
	{#if info.description}
		<meta name="description" content={info.description} />
	{/if}
</svelte:head>

<div class="flex min-h-dvh flex-col">
	<header class="border-b border-line bg-surface">
		<div class="mx-auto max-w-5xl px-4 sm:px-6">
			<div class="flex h-16 items-center justify-between gap-4">
				<a href={info.home_url || '/'} class="flex min-w-0 items-center gap-3">
					{#if info.logo_url}
						<img src={info.logo_url} alt="" class="h-7 w-auto max-w-36 object-contain" />
					{:else}
						<Logo size={26} class="text-ink" />
					{/if}
					<span class="truncate text-[15px] font-semibold tracking-tight">{info.title}</span>
					{#if !/status/i.test(info.title)}
						<span class="hidden h-4 w-px bg-line-strong sm:block"></span>
						<span class="hidden text-[13px] text-muted sm:block">Status</span>
					{/if}
				</a>

				<div class="flex items-center gap-2">
					<span
						class="hidden items-center gap-1.5 rounded-full border border-line px-2.5 py-1 text-[11px] font-medium text-muted sm:flex"
						title={store.live.connected ? 'Receiving live updates' : 'Reconnecting'}
					>
						<span class="relative flex size-1.5">
							{#if store.live.connected}
								<span class="absolute inset-0 animate-pulse-ring rounded-full bg-up"></span>
							{/if}
							<span class="relative size-1.5 rounded-full {store.live.connected ? 'bg-up' : 'bg-faint'}"></span>
						</span>
						{store.live.connected ? 'Live' : 'Offline'}
					</span>
					<ThemeToggle />
				</div>
			</div>

			<nav class="-mb-px flex gap-6">
				{#each nav as item (item.href)}
					<a
						href={item.href}
						class="border-b-2 pb-2.5 text-[13px] font-medium transition {active(item.href)
							? 'border-accent text-ink'
							: 'border-transparent text-muted hover:border-line-strong hover:text-ink'}"
						aria-current={active(item.href) ? 'page' : undefined}
					>
						{item.label}
					</a>
				{/each}
			</nav>
		</div>
	</header>

	<main class="mx-auto w-full max-w-5xl flex-1 px-4 py-8 sm:px-6 sm:py-10">
		{@render children()}
	</main>

	<footer class="border-t border-line bg-surface">
		<div class="mx-auto flex max-w-5xl flex-col items-center justify-between gap-3 px-4 py-5 text-xs text-muted sm:flex-row sm:px-6">
			<span>{info.footer || `© ${new Date().getFullYear()} ${info.title}. All rights reserved.`}</span>
			<a href="https://github.com/nethercode/heron" class="flex items-center gap-1.5 hover:text-ink" rel="noopener" target="_blank">
				<Logo size={13} /> Powered by heron <Icon name="external" size={11} />
			</a>
		</div>
	</footer>
</div>
