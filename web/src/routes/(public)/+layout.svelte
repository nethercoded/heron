<script lang="ts">
	import { untrack } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Logo from '#lib/components/Logo.svelte';
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
	const active = (href: string) =>
		href === '/'
			? page.url.pathname === '/' || page.url.pathname.startsWith('/monitor')
			: page.url.pathname.startsWith(href);

	$effect(() => {
		const root = document.documentElement;
		root.classList.add('public');
		return () => root.classList.remove('public');
	});
</script>

<svelte:head>
	<title>{info.title}</title>
	<meta name="theme-color" content="#000000" />
	{#if info.description}
		<meta name="description" content={info.description} />
	{/if}
</svelte:head>

<div class="flex min-h-dvh flex-col bg-canvas">
	<header class="mx-auto flex w-full max-w-5xl items-center justify-between gap-4 px-4 pt-6 pb-2 sm:px-6">
		<a href={info.home_url || '/'} class="flex min-w-0 items-center gap-3">
			{#if info.logo_url}
				<img src={info.logo_url} alt="" class="h-8 w-auto max-w-40 object-contain" />
			{:else}
				<Logo size={30} class="text-ink" />
			{/if}
			<span class="truncate text-lg font-semibold tracking-tight">{info.title}</span>
		</a>

		<nav class="flex items-center gap-1 rounded-full bg-surface p-1">
			{#each nav as item (item.href)}
				<a
					href={item.href}
					class="rounded-full px-3.5 py-1.5 text-[13px] font-medium transition {active(item.href) ? 'bg-ink text-canvas' : 'text-muted hover:text-ink'}"
					aria-current={active(item.href) ? 'page' : undefined}
				>
					{item.label}
				</a>
			{/each}
			<span
				class="ml-1 hidden items-center gap-2 px-3 font-mono text-[11px] text-muted sm:flex"
				title={store.live.connected ? 'Receiving live updates' : 'Reconnecting'}
			>
				<span class="sq {store.live.connected ? 'blink' : ''}" style="--c: var(--color-{store.live.connected ? 'up' : 'faint'}); --s: 8px"></span>
				{store.live.connected ? 'LIVE' : 'OFFLINE'}
			</span>
		</nav>
	</header>

	<main class="mx-auto w-full max-w-5xl flex-1 px-4 py-6 sm:px-6">
		{@render children()}
	</main>

	<footer class="mx-auto flex w-full max-w-5xl flex-col items-center justify-between gap-3 px-4 py-8 text-xs text-faint sm:flex-row sm:px-6">
		<span>{info.footer || `© ${new Date().getFullYear()} ${info.title}`}</span>
		<a href="https://github.com/nethercode/heron" class="flex items-center gap-2 hover:text-muted" rel="noopener" target="_blank">
			<Logo size={14} /> heron <Icon name="external" size={11} />
		</a>
	</footer>
</div>
