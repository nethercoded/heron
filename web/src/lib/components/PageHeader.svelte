<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	let {
		title,
		description,
		back,
		actions
	}: { title: string; description?: string; back?: { href: string; label: string }; actions?: Snippet } = $props();
</script>

<svelte:head>
	<title>{title} · heron</title>
</svelte:head>

{#if back}
	<a href={back.href} class="mb-4 inline-flex items-center gap-1 text-sm text-muted hover:text-ink">
		<Icon name="back" size={14} />
		{back.label}
	</a>
{/if}

<div class="mb-8 flex flex-wrap items-end justify-between gap-4">
	<div class="min-w-0">
		<h1 class="text-2xl font-semibold tracking-tight">{title}</h1>
		{#if description}
			<p class="mt-1 text-sm text-muted">{description}</p>
		{/if}
	</div>
	{#if actions}
		<div class="flex flex-wrap items-center gap-2">{@render actions()}</div>
	{/if}
</div>
