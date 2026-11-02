<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title,
		description,
		back,
		actions,
		meta
	}: {
		title: string;
		description?: string;
		back?: { href: string; label: string };
		actions?: Snippet;
		meta?: Snippet;
	} = $props();
</script>

<svelte:head>
	<title>{title} · heron</title>
</svelte:head>

<div class="mb-7">
	{#if back}
		<div class="mb-3 flex flex-wrap items-center gap-2 text-[14px] text-ink-2">
			<a href="/admin" class="underline-offset-[3px] hover:text-ink hover:underline">Admin</a>
			<span class="text-faint">/</span>
			<a href={back.href} class="underline-offset-[3px] hover:text-ink hover:underline">{back.label}</a>
		</div>
	{/if}
	<div class="flex flex-wrap items-end justify-between gap-x-6 gap-y-4">
		<div class="min-w-0 grow basis-80">
			<h1 class="text-[40px] leading-[0.98] font-semibold tracking-[-0.035em] break-words md:text-[52px] md:tracking-[-0.04em]">
				{title}
			</h1>
			{#if description || meta}
				<div class="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-[15px] text-ink-2">
					{#if description}<span class="min-w-0 break-all">{description}</span>{/if}
					{@render meta?.()}
				</div>
			{/if}
		</div>
		{#if actions}
			<div class="flex shrink-0 flex-wrap items-center gap-2 max-md:w-full">{@render actions()}</div>
		{/if}
	</div>
</div>
