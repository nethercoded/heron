<script lang="ts">
	import { statusTone } from '#lib/status.ts';
	import type { Status } from '#lib/types.ts';

	let { status, size = 'md' }: { status: Status; size?: 'sm' | 'md' | 'lg' } = $props();

	const dimensions = { sm: 'size-2', md: 'size-2.5', lg: 'size-3.5' };
	const live = $derived(status === 'up' || status === 'down');
</script>

<span class="relative inline-flex {dimensions[size]}">
	{#if live}
		<span class="absolute inset-0 rounded-full {statusTone[status]} animate-pulse-ring"></span>
	{/if}
	<span class="relative inline-flex rounded-full {dimensions[size]} {statusTone[status]}"></span>
</span>
