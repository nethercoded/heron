<script lang="ts">
	import { useI18n } from '#lib/i18n.svelte.ts';
	import type { MaintenanceWindow } from '#lib/types.ts';
	import Icon from './Icon.svelte';

	let { window: w, now = Date.now() }: { window: MaintenanceWindow; now?: number } = $props();

	const i18n = useI18n();
	const sameDay = $derived(new Date(w.starts_at).toDateString() === new Date(w.ends_at).toDateString());
	const repeatLabel = $derived({
		none: '',
		daily: i18n.t('maintenance.daily'),
		weekly: i18n.t('maintenance.weekly')
	});
</script>

<article class="card relative overflow-hidden">
	<span class="absolute inset-y-0 start-0 w-1 bg-accent"></span>
	<div class="flex flex-wrap items-start justify-between gap-3 py-4 ps-6 pe-5">
		<div class="flex min-w-0 items-start gap-3">
			<span class="mt-0.5 grid size-8 shrink-0 place-items-center rounded-full bg-accent/12 text-accent">
				<Icon name="wrench" size={15} />
			</span>
			<div class="min-w-0">
				<h3 class="text-[15px] font-semibold" dir="auto">{w.title}</h3>
				<p class="mt-0.5 text-xs text-muted">
					{i18n.dateTime(w.starts_at)} – {sameDay ? i18n.time(w.ends_at) : i18n.dateTime(w.ends_at)}
					· {i18n.duration(w.duration_s * 1000)}
					{#if repeatLabel[w.repeat]}· {repeatLabel[w.repeat]}{/if}
				</p>
				{#if w.body}
					<p class="mt-2 leading-relaxed text-muted" dir="auto">{w.body}</p>
				{/if}
			</div>
		</div>
		<span class="chip {w.active ? 'bg-accent/12 text-accent ring-accent/30' : 'text-muted ring-line-strong'}">
			{w.active ? i18n.t('maintenance.active') : i18n.t('maintenance.starts', { when: i18n.ago(w.starts_at, now) })}
		</span>
	</div>
	{#if w.monitors.length > 0}
		<footer class="flex flex-wrap items-center gap-1.5 border-t border-line bg-sunken/60 py-2.5 ps-6 pe-5 text-xs text-muted">
			<span class="me-1">{i18n.t('card.affected')}</span>
			{#each w.monitors as m (m.slug)}
				<a href="/monitor/{m.slug}" class="rounded border border-line bg-surface px-1.5 py-0.5 font-medium text-ink hover:border-line-strong">{m.name}</a>
			{/each}
		</footer>
	{/if}
</article>
