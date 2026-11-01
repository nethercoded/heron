<script lang="ts">
	import Logo from '#lib/components/Logo.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<div class="relative grid min-h-dvh place-items-center px-4 py-12">
	<div class="absolute top-4 right-4"><ThemeToggle /></div>
	<div class="w-full max-w-sm">
		<a href="/" class="mb-8 flex items-center justify-center gap-2.5">
			<Logo size={32} water class="text-ink" />
			<span class="text-lg font-semibold tracking-tight">heron</span>
		</a>
		{@render children()}
	</div>
</div>
