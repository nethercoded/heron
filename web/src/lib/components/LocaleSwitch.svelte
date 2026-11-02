<script lang="ts">
	import { locales, useI18n } from '#lib/i18n.svelte.ts';

	const i18n = useI18n();

	let buttons = $state<HTMLButtonElement[]>([]);
	let pill = $state({ left: 0, width: 0, animate: false });
	let dir = '';

	const current = $derived(locales.findIndex((item) => item.code === i18n.locale));

	function place() {
		const el = buttons[current];
		if (!el) return;
		pill = { left: el.offsetLeft, width: el.offsetWidth, animate: dir === i18n.dir };
		dir = i18n.dir;
	}

	$effect(() => {
		void current;
		place();
		const ro = new ResizeObserver(place);
		for (const el of buttons) ro.observe(el);
		document.fonts?.ready.then(place);
		return () => ro.disconnect();
	});
</script>

<div class="relative flex gap-0.5 rounded-full bg-surface p-1" role="radiogroup" aria-label={i18n.t('nav.language')}>
	<span
		class="pointer-events-none absolute top-1 bottom-1 rounded-full bg-ink {pill.animate ? 'transition-[left,width] duration-500 ease-[cubic-bezier(0.34,1.32,0.5,1)]' : ''}"
		style="left: {pill.left}px; width: {pill.width}px; opacity: {pill.width ? 1 : 0}"
		aria-hidden="true"
	></span>
	{#each locales as item, i (item.code)}
		<button
			bind:this={buttons[i]}
			type="button"
			role="radio"
			aria-checked={i === current}
			lang={item.code}
			title={item.name}
			class="relative flex items-center rounded-full px-2.5 font-mono text-[11px] font-medium tracking-wider uppercase transition-colors duration-300 sm:px-3 {i === current
				? 'text-canvas'
				: 'text-muted hover:text-ink'}"
			onclick={() => i18n.set(item.code)}
		>
			{item.code}
		</button>
	{/each}
</div>
