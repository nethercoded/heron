<script lang="ts">
	import HeronField from '#lib/components/HeronField.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<div class="grid min-h-dvh gap-3 p-3 lg:grid-cols-[minmax(380px,520px)_1fr]">
	<section class="flex flex-col rounded-card bg-surface p-6 sm:p-10">
		<a href="/" class="flex items-end gap-[11px]">
			<Logo size={30} class="text-ink" />
			<span class="text-[23px] leading-5 font-bold tracking-[-0.04em]">heron</span>
		</a>
		<div class="flex flex-1 flex-col justify-center py-12">
			<div class="w-full max-w-sm">
				{@render children()}
			</div>
		</div>
		<p class="text-xs text-faint">Self-hosted status page and uptime monitor.</p>
	</section>

	<div class="relative hidden overflow-hidden rounded-card bg-surface lg:block">
		<HeronField class="absolute inset-0 size-full" />
	</div>
</div>
