<script lang="ts">
	import { statusLabel, statusPill, statusTone } from '#lib/status.ts';
	import type { Status } from '#lib/types.ts';

	let { status }: { status: Status } = $props();
</script>

<span class="chip {statusPill[status]}">
	<span class="size-1.5 rounded-full {statusTone[status]}"></span>
	{statusLabel[status]}
</span>
