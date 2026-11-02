<script lang="ts">
	import type { Snippet } from 'svelte';
	import { api } from '#lib/api.ts';
	import { defaultBlocks, historySpan, kindOf, spanClass, widthLabel, widths } from '#lib/blocks.ts';
	import Icon from '#lib/components/Icon.svelte';
	import StatusBlocks from '#lib/components/StatusBlocks.svelte';
	import { useI18n } from '#lib/i18n.svelte.ts';
	import { type BlockAction, getPreview, hasPreview, type InlineEdit } from '#lib/preview.svelte.ts';
	import { getStatus } from '#lib/status-page.svelte.ts';
	import type { Block, Incident } from '#lib/types.ts';
	import type { PageProps } from './$types';

	let { data: props }: PageProps = $props();

	const store = getStatus();
	const i18n = useI18n();
	const preview = hasPreview() ? getPreview() : null;

	const data = $derived(store.data && { ...store.data, page: store.page });
	const blocks = $derived(data?.page.blocks?.length ? data.page.blocks : defaultBlocks());

	let extra = $state<{ days: number; list: Incident[] } | null>(null);
	const recent = $derived(extra?.list ?? props.recent);

	$effect(() => {
		const span = historySpan(blocks);
		if (!preview || span <= (extra?.days ?? 7)) return;
		api.get<Incident[]>(`/incidents?days=${span}`).then(
			(list) => (extra = { days: span, list }),
			() => {}
		);
	});

	let wide = $state(true);
	$effect(() => {
		if (!preview) return;
		const query = matchMedia('(min-width: 768px)');
		const sync = () => (wide = query.matches);
		sync();
		query.addEventListener('change', sync);
		return () => query.removeEventListener('change', sync);
	});

	const edit: InlineEdit = {
		get selected() {
			return preview && !preview.readonly ? preview.selected : null;
		},
		text: (id, field, value) => preview?.send({ type: 'heron:text', id, field, value })
	};

	let drop = $state<{ id: string; after: boolean; side: boolean } | null>(null);

	function over(event: DragEvent, block: Block) {
		if (!preview?.dragging || preview.dragging === block.id) return;
		event.preventDefault();
		const el = event.currentTarget as HTMLElement;
		const box = el.getBoundingClientRect();
		const grid = el.parentElement?.getBoundingClientRect();
		const side = wide && !!grid && box.width < grid.width * 0.75;
		const forward = side ? event.clientX > box.left + box.width / 2 : event.clientY > box.top + box.height / 2;
		drop = { id: block.id, after: side && i18n.dir === 'rtl' ? !forward : forward, side };
	}

	function place(event: DragEvent, index: number) {
		event.preventDefault();
		const id = preview?.dragging;
		if (!preview || !id || !drop) return;
		const from = blocks.findIndex((b) => b.id === id);
		const at = drop.after ? index + 1 : index;
		const to = at > from ? at - 1 : at;
		if (to !== from) preview.send({ type: 'heron:move', id, to });
		preview.dragging = null;
		drop = null;
	}

	let sizing = $state<{ id: string; width: number } | null>(null);

	function resize(event: PointerEvent, block: Block) {
		if (!preview) return;
		const handle = event.currentTarget as HTMLElement;
		const frame = handle.closest<HTMLElement>('[data-edit]');
		const grid = frame?.parentElement;
		if (!frame || !grid) return;
		event.preventDefault();
		handle.setPointerCapture(event.pointerId);
		const rtl = i18n.dir === 'rtl';
		const start = frame.getBoundingClientRect();
		const total = grid.getBoundingClientRect().width;
		sizing = { id: block.id, width: block.width ?? 0 };

		const move = (e: PointerEvent) => {
			const span = rtl ? start.right - e.clientX : e.clientX - start.left;
			const ratio = span / total;
			const best = widths.reduce((a, b) =>
				Math.abs((b.value || 12) / 12 - ratio) < Math.abs((a.value || 12) / 12 - ratio) ? b : a
			);
			if (sizing && best.value !== sizing.width) {
				sizing = { id: block.id, width: best.value };
				preview.send({ type: 'heron:resize', id: block.id, width: best.value });
			}
		};
		const up = () => {
			sizing = null;
			handle.removeEventListener('pointermove', move);
			handle.removeEventListener('pointerup', up);
			handle.removeEventListener('pointercancel', up);
		};
		handle.addEventListener('pointermove', move);
		handle.addEventListener('pointerup', up);
		handle.addEventListener('pointercancel', up);
	}

	const act = (block: Block, action: BlockAction) => preview?.send({ type: 'heron:action', id: block.id, action });

	const tools: { action: BlockAction; icon: 'up' | 'down' | 'copy' | 'eye' | 'eye-off' | 'trash'; label: string }[] = [
		{ action: 'up', icon: 'up', label: 'Move up' },
		{ action: 'down', icon: 'down', label: 'Move down' },
		{ action: 'duplicate', icon: 'copy', label: 'Duplicate' },
		{ action: 'hide', icon: 'eye-off', label: 'Hide' },
		{ action: 'delete', icon: 'trash', label: 'Delete' }
	];
</script>

{#snippet plus(at: number, edge: 'top' | 'bottom')}
	<button
		type="button"
		data-tool
		class="absolute start-1/2 z-20 grid size-7 -translate-x-1/2 place-items-center rounded-full bg-accent text-accent-ink opacity-0 shadow-pop transition-[opacity,scale] group-hover/frame:opacity-100 hover:scale-110 focus-visible:opacity-100 rtl:translate-x-1/2 {edge === 'top' ? '-top-[38px]' : '-bottom-[38px]'}"
		onclick={() => preview?.send({ type: 'heron:insert', at })}
		aria-label="Add a block here"
		title="Add a block here"
	>
		<Icon name="plus" size={15} />
	</button>
{/snippet}

{#snippet marker(edge: 'top' | 'bottom')}
	<div class="pointer-events-none absolute inset-x-0 z-20 flex items-center gap-2 {edge === 'top' ? '-top-[30px]' : '-bottom-[30px]'}" dir="ltr">
		<span class="h-[3px] flex-1 animate-pulse rounded-full bg-accent"></span>
		<span class="rounded-md bg-accent px-2 py-0.5 text-[11px] font-semibold text-accent-ink">New block goes here</span>
		<span class="h-[3px] flex-1 animate-pulse rounded-full bg-accent"></span>
	</div>
{/snippet}

{#snippet frame(block: Block, index: number, inner: Snippet)}
	{@const selected = preview?.selected === block.id}
	{@const kind = kindOf(block.type)}
	{@const editable = !!preview && !preview.readonly}
	{@const width = sizing?.id === block.id ? sizing.width : (block.width ?? 0)}
	<div
		data-edit={block.id}
		data-selected={selected || undefined}
		data-invalid={preview?.errors.includes(block.id) || undefined}
		class="group/frame {spanClass(block)} {preview?.dragging === block.id ? 'opacity-40' : ''}"
		role="listitem"
		ondragover={(e) => over(e, block)}
		ondragleave={() => drop?.id === block.id && (drop = null)}
		ondrop={(e) => place(e, index)}
	>
		{#if drop?.id === block.id}
			<span
				class="pointer-events-none absolute z-20 rounded-full bg-accent {drop.side
					? `inset-y-0 w-[3px] ${drop.after ? '-end-[14px]' : '-start-[14px]'}`
					: `inset-x-0 h-[3px] ${drop.after ? '-bottom-[26px]' : '-top-[26px]'}`}"
			></span>
		{/if}
		{#if preview?.inserting === index}
			{@render marker('top')}
		{:else if preview?.inserting === index + 1 && index === blocks.length - 1}
			{@render marker('bottom')}
		{/if}

		<div
			class="pointer-events-none absolute -top-3 start-4 z-10 items-center gap-1.5 rounded-md bg-accent py-0.5 ps-1 pe-2 text-[11px] font-semibold text-accent-ink {selected ? 'flex' : 'hidden group-hover/frame:flex'}"
			dir="ltr"
		>
			{#if editable}
				<span
					class="pointer-events-auto grid cursor-grab place-items-center rounded px-0.5 active:cursor-grabbing"
					draggable="true"
					data-tool
					role="button"
					tabindex="-1"
					aria-label="Drag to move"
					ondragstart={(e) => {
						if (!preview) return;
						preview.dragging = block.id;
						e.dataTransfer?.setData('text/plain', block.id);
						e.dataTransfer?.setDragImage((e.currentTarget as HTMLElement).closest('[data-edit]') as Element, 24, 12);
					}}
					ondragend={() => {
						if (preview) preview.dragging = null;
						drop = null;
					}}
				>
					<Icon name="grip" size={13} />
				</span>
			{/if}
			{kind.label}{width ? ` · ${widthLabel(width)}` : ''}{block.hidden ? ' · hidden' : ''}
		</div>

		{#if selected && editable}
			<div class="absolute -top-4 end-3 z-10 flex items-center gap-0.5 rounded-lg border border-line-strong bg-surface p-0.5 shadow-pop" dir="ltr" data-tool>
				{#each tools as tool (tool.action)}
					<button
						type="button"
						class="grid size-7 place-items-center rounded-md text-ink-2 hover:bg-sunken hover:text-ink disabled:opacity-30 {tool.action === 'delete' ? 'hover:text-down' : ''}"
						title={tool.action === 'hide' && block.hidden ? 'Show' : tool.label}
						aria-label={tool.label}
						disabled={(tool.action === 'up' && index === 0) ||
							(tool.action === 'down' && index === blocks.length - 1) ||
							(tool.action === 'duplicate' && block.type === 'hero')}
						onclick={() => act(block, tool.action)}
					>
						<Icon name={tool.action === 'hide' && block.hidden ? 'eye' : tool.icon} size={14} />
					</button>
				{/each}
			</div>

			{#if block.type !== 'hero' && wide}
				<span
					class="absolute top-1/2 -end-[11px] z-20 flex h-12 w-[14px] -translate-y-1/2 cursor-ew-resize touch-none items-center justify-center"
					data-tool
					role="slider"
					tabindex="-1"
					aria-label="Drag to change the width"
					aria-valuenow={width || 12}
					aria-valuemin={4}
					aria-valuemax={12}
					title="Drag to change the width"
					onpointerdown={(e) => resize(e, block)}
				>
					<span class="h-full w-[6px] rounded-full border-2 border-canvas bg-accent transition-[height] {sizing?.id === block.id ? 'h-16' : ''}"></span>
				</span>
			{/if}
		{/if}

		{#if sizing?.id === block.id}
			<div class="pointer-events-none absolute inset-0 z-30 grid place-items-center rounded-card bg-accent/10" dir="ltr">
				<span class="rounded-lg bg-accent px-3 py-1 font-mono text-sm font-semibold text-accent-ink">{widthLabel(sizing.width)}</span>
			</div>
		{/if}

		<div class="h-full transition-opacity {block.hidden ? 'opacity-35' : ''}">{@render inner()}</div>

		{#if editable}
			{#if index === 0}{@render plus(0, 'top')}{/if}
			{@render plus(index + 1, 'bottom')}
		{/if}
	</div>
{/snippet}

{#if !data}
	<div class="card p-10 text-center">
		<p class="font-semibold">{i18n.t('page.unavailable')}</p>
		<p class="mt-1 text-muted">{store.error ?? i18n.t('page.later')}</p>
		<button class="btn-secondary mt-5" onclick={() => store.load()}><Icon name="refresh" /> {i18n.t('page.retry')}</button>
	</div>
{:else if preview}
	<div role="list" class="pt-6 pb-10">
		<StatusBlocks {data} {blocks} {recent} editing {frame} {edit} />
	</div>
{:else}
	<StatusBlocks {data} {blocks} {recent} />
{/if}
