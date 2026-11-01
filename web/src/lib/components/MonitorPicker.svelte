<script lang="ts">
	import type { AdminMonitor } from '#lib/types.ts';
	import StatusDot from './StatusDot.svelte';

	let { monitors, selected = $bindable([]) }: { monitors: AdminMonitor[]; selected: number[] } = $props();

	function toggle(id: number, on: boolean) {
		selected = on ? [...selected, id] : selected.filter((s) => s !== id);
	}
</script>

{#if monitors.length === 0}
	<p class="text-sm text-muted">No monitors yet.</p>
{:else}
	<div class="flex flex-wrap gap-2">
		{#each monitors as m (m.id)}
			{@const on = selected.includes(m.id)}
			<label class="chip cursor-pointer py-1 {on ? 'bg-ink text-canvas ring-ink' : 'text-muted ring-line hover:text-ink'}">
				<input type="checkbox" class="sr-only" checked={on} onchange={(e) => toggle(m.id, e.currentTarget.checked)} />
				<StatusDot status={m.status} size="sm" />
				{m.name}
			</label>
		{/each}
	</div>
{/if}
