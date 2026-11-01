<script lang="ts">
	import { toast } from '#lib/toast.svelte.ts';
	import Icon from './Icon.svelte';

	let { label, code, preview }: { label: string; code: string; preview?: string } = $props();

	async function copy() {
		await navigator.clipboard.writeText(code);
		toast.info('Copied to clipboard');
	}
</script>

<div>
	<div class="mb-1.5 flex items-center justify-between gap-3">
		<span class="text-xs font-medium">{label}</span>
		{#if preview}
			<img src={preview} alt={label} class="h-5" />
		{/if}
	</div>
	<div class="flex items-stretch gap-2">
		<code class="min-w-0 flex-1 truncate rounded-md border border-line bg-sunken px-2.5 py-1.5 font-mono text-[11px] text-muted" title={code}>{code}</code>
		<button type="button" class="btn-secondary btn-sm h-auto" onclick={copy} aria-label="Copy {label}"><Icon name="copy" size={13} /></button>
	</div>
</div>
