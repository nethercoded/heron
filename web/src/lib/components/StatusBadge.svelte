<script lang="ts">
	import { statusLabel, statusText } from '#lib/status.ts';
	import type { Status } from '#lib/types.ts';
	import StatusDot from './StatusDot.svelte';

	let { status }: { status: Status } = $props();
</script>

<span class="inline-flex items-center gap-2 text-sm font-medium {statusText[status]}">
	<StatusDot {status} size="sm" />
	{statusLabel[status]}
</span>
