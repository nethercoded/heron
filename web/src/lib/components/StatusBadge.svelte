<script lang="ts">
	import { useI18n } from '#lib/i18n.svelte.ts';
	import type { Status } from '#lib/types.ts';

	let { status }: { status: Status } = $props();

	const i18n = useI18n();
	const color: Record<Status, string> = {
		up: 'var(--color-up)',
		degraded: 'var(--color-degraded)',
		down: 'var(--color-down)',
		maintenance: 'var(--color-accent)',
		pending: 'var(--color-faint)',
		paused: 'var(--color-faint)'
	};
</script>

<span class="inline-flex items-center gap-2 font-mono text-[11px] font-medium tracking-wider whitespace-nowrap uppercase" style="color: {color[status]}">
	<span class="sq {status === 'down' ? 'blink' : ''} {status === 'paused' || status === 'pending' ? 'bg-transparent!' : ''}" style="--c: {color[status]}; --s: 9px"></span>
	{i18n.status(status)}
</span>
