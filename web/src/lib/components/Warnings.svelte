<script lang="ts">
	import { ago } from '#lib/format.ts';
	import type { Warning } from '#lib/types.ts';

	let { warnings, linked = true }: { warnings: Warning[]; linked?: boolean } = $props();

	const label: Record<Warning['kind'], string> = {
		slow: 'Slower than usual',
		hidden_failures: 'Failing quietly',
		memory: 'Near its memory limit',
		disk: 'Disk filling up'
	};
</script>

{#if warnings.length > 0}
	<section class="card mb-6 overflow-hidden" style="border-color: color-mix(in oklab, var(--color-degraded) 45%, var(--color-line))">
		<div class="flex items-baseline justify-between gap-3 border-b border-line px-5 py-3.5">
			<h2 class="flex items-center gap-2.5 font-semibold">
				<span class="sq" style="--c: var(--color-degraded); --s: 8px"></span>
				Heads up
			</h2>
			<span class="text-[12.5px] text-faint">These have not failed, but they are heading that way.</span>
		</div>
		<ul class="divide-y divide-line">
			{#each warnings as w (w.monitor_id + w.kind)}
				<li class="flex flex-wrap items-baseline gap-x-3 gap-y-1 px-5 py-3">
					<span class="w-44 shrink-0 text-[13px] font-medium" style="color: var(--color-degraded)">{label[w.kind]}</span>
					<span class="min-w-0 flex-1 text-[13.5px] text-ink-2">
						{#if linked}<a href="/admin/monitors/{w.monitor_id}" class="font-medium text-ink hover:underline">{w.monitor_name}</a>.{/if}
						{w.message}
					</span>
					<span class="text-[12px] text-faint">since {ago(w.since)}</span>
				</li>
			{/each}
		</ul>
	</section>
{/if}
