<script lang="ts">
	import { untrack } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import { StatusStore, setStatus } from '#lib/status-page.svelte.ts';
	import { page } from '$app/state';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	const store = new StatusStore(untrack(() => data.status));
	setStatus(store);

	const info = $derived(store.page);
	const nav = [
		{ href: '/', label: 'Status' },
		{ href: '/incidents', label: 'History' }
	];
</script>

<svelte:head>
	<title>{info.title}</title>
	{#if info.description}
		<meta name="description" content={info.description} />
	{/if}
</svelte:head>

<div class="flex min-h-dvh flex-col">
	<header class="border-b border-line bg-surface/70 backdrop-blur supports-backdrop-filter:bg-surface/60">
		<div class="mx-auto flex h-16 max-w-4xl items-center justify-between gap-4 px-4 sm:px-6">
			<a href={info.home_url || '/'} class="flex min-w-0 items-center gap-3">
				{#if info.logo_url}
					<img src={info.logo_url} alt="" class="h-8 w-auto max-w-32 object-contain" />
				{:else}
					<img src="/favicon.svg" alt="" class="size-8" />
				{/if}
				<span class="truncate text-base font-semibold tracking-tight">{info.title}</span>
			</a>

			<nav class="flex items-center gap-1">
				{#each nav as item (item.href)}
					<a
						href={item.href}
						class="btn-ghost btn-sm {page.url.pathname === item.href ? 'bg-sunken text-ink' : ''}"
						aria-current={page.url.pathname === item.href ? 'page' : undefined}
					>
						{item.label}
					</a>
				{/each}
				<ThemeToggle />
			</nav>
		</div>
	</header>

	<main class="mx-auto w-full max-w-4xl flex-1 px-4 py-8 sm:px-6 sm:py-12">
		{@render children()}
	</main>

	<footer class="border-t border-line">
		<div class="mx-auto flex max-w-4xl flex-col items-center justify-between gap-2 px-4 py-6 text-xs text-faint sm:flex-row sm:px-6">
			<span>{info.footer || `© ${new Date().getFullYear()} ${info.title}`}</span>
			<span class="flex items-center gap-3">
				<span class="flex items-center gap-1.5" title={store.live.connected ? 'Live updates connected' : 'Reconnecting…'}>
					<span class="size-1.5 rounded-full {store.live.connected ? 'bg-up' : 'bg-faint'}"></span>
					{store.live.connected ? 'Live' : 'Offline'}
				</span>
				<a href="https://github.com/nethercode/heron" class="flex items-center gap-1 hover:text-muted" rel="noopener" target="_blank">
					Powered by heron <Icon name="external" size={11} />
				</a>
			</span>
		</div>
	</footer>
</div>
