<script lang="ts">
	import type { Status } from '#lib/types.ts';

	let { status }: { status: Status } = $props();

	const word: Record<Status, string> = {
		up: 'Operational',
		degraded: 'Degraded',
		down: 'Down',
		maintenance: 'Maintenance',
		pending: 'Pending',
		paused: 'Paused'
	};
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
	{word[status]}
</span>
