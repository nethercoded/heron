<script lang="ts">
	import { ago, duration, formatDateTime, formatTime } from '#lib/format.ts';
	import type { MaintenanceWindow } from '#lib/types.ts';
	import Icon from './Icon.svelte';

	let { window: w, now = Date.now() }: { window: MaintenanceWindow; now?: number } = $props();

	const sameDay = $derived(new Date(w.starts_at).toDateString() === new Date(w.ends_at).toDateString());
	const repeatLabel = { none: '', daily: 'Repeats daily', weekly: 'Repeats weekly' };
</script>

<article class="card relative overflow-hidden">
	<span class="absolute inset-y-0 left-0 w-1 bg-accent"></span>
	<div class="flex flex-wrap items-start justify-between gap-3 py-4 pr-5 pl-6">
		<div class="flex min-w-0 items-start gap-3">
			<span class="mt-0.5 grid size-8 shrink-0 place-items-center rounded-full bg-accent/12 text-accent">
				<Icon name="wrench" size={15} />
			</span>
			<div class="min-w-0">
				<h3 class="text-[15px] font-semibold">{w.title}</h3>
				<p class="mt-0.5 text-xs text-muted">
					{formatDateTime(w.starts_at)} – {sameDay ? formatTime(w.ends_at) : formatDateTime(w.ends_at)}
					· {duration(w.duration_s * 1000)}
					{#if repeatLabel[w.repeat]}· {repeatLabel[w.repeat]}{/if}
				</p>
				{#if w.body}
					<p class="mt-2 leading-relaxed text-muted">{w.body}</p>
				{/if}
			</div>
		</div>
		<span class="chip {w.active ? 'bg-accent/12 text-accent ring-accent/30' : 'text-muted ring-line-strong'}">
			{w.active ? 'In progress' : `Starts ${ago(w.starts_at, now)}`}
		</span>
	</div>
	{#if w.monitors.length > 0}
		<footer class="flex flex-wrap items-center gap-1.5 border-t border-line bg-sunken/60 py-2.5 pr-5 pl-6 text-xs text-muted">
			<span class="mr-1">Affected services</span>
			{#each w.monitors as m (m.slug)}
				<a href="/monitor/{m.slug}" class="rounded border border-line bg-surface px-1.5 py-0.5 font-medium text-ink hover:border-line-strong">{m.name}</a>
			{/each}
		</footer>
	{/if}
</article>
