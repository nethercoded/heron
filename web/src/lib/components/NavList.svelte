<script lang="ts" module>
	import type { IconName } from './Icon.svelte';

	export type NavItem = { href: string; label: string; icon: IconName; count?: number; alert?: boolean };
	export type NavGroup = { title: string; items: NavItem[] };
</script>

<script lang="ts">
	import Icon from './Icon.svelte';

	let { groups, active }: { groups: NavGroup[]; active: string } = $props();

	let links = $state<Record<string, HTMLAnchorElement>>({});
	let pill = $state({ top: 0, height: 0, animate: false });
	let settled = false;

	function place() {
		const el = links[active];
		if (!el) {
			pill = { top: 0, height: 0, animate: false };
			return;
		}
		pill = { top: el.offsetTop, height: el.offsetHeight, animate: settled };
		settled = true;
	}

	$effect(() => {
		void active;
		place();
		const ro = new ResizeObserver(place);
		for (const el of Object.values(links)) ro.observe(el);
		return () => ro.disconnect();
	});
</script>

<div class="relative">
	<span
		class="pointer-events-none absolute inset-x-0 rounded-ctl bg-ink {pill.animate ? 'transition-[top,height] duration-500 ease-[cubic-bezier(0.34,1.32,0.5,1)]' : ''}"
		style="top: {pill.top}px; height: {pill.height}px; opacity: {pill.height ? 1 : 0}"
		aria-hidden="true"
	></span>

	{#each groups as group (group.title)}
		<nav class="relative flex flex-col gap-1.5 {group.title ? 'mt-5' : ''}" aria-label={group.title || 'Navigation'}>
			{#if group.title}
				<p class="px-3 pb-0.5 text-[11px] font-semibold tracking-[0.08em] text-faint uppercase">{group.title}</p>
			{/if}
			{#each group.items as item (item.href)}
				{@const on = active === item.href}
				<a
					bind:this={links[item.href]}
					href={item.href}
					aria-current={on ? 'page' : undefined}
					class="relative flex h-[42px] shrink-0 items-center gap-3 rounded-ctl px-3 text-[15px] transition-colors duration-300 {on
						? 'font-semibold text-canvas'
						: 'font-medium text-ink hover:bg-sunken'}"
				>
					<Icon name={item.icon} size={18} class={on ? 'text-canvas' : 'text-ink-2'} />
					{item.label}
					{#if item.count !== undefined}
						<span class="ml-auto flex items-center gap-2 text-[13px] font-medium {on ? 'text-[#4d4d4d]' : 'text-faint'}">
							{#if item.alert}<span class="sq blink" style="--c: var(--color-down); --s: 7px"></span>{/if}
							{item.count}
						</span>
					{/if}
				</a>
			{/each}
		</nav>
	{/each}
</div>
