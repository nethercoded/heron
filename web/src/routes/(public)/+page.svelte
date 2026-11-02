<script lang="ts">
	import type { Snippet } from 'svelte';
	import { api } from '#lib/api.ts';
	import { defaultBlocks, historySpan, kindOf } from '#lib/blocks.ts';
	import Icon from '#lib/components/Icon.svelte';
	import StatusBlocks from '#lib/components/StatusBlocks.svelte';
	import { useI18n } from '#lib/i18n.svelte.ts';
	import { type BlockAction, getPreview, hasPreview } from '#lib/preview.svelte.ts';
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

	let drop = $state<{ id: string; after: boolean } | null>(null);

	function over(event: DragEvent, block: Block) {
		if (!preview?.dragging) return;
		event.preventDefault();
		const box = (event.currentTarget as HTMLElement).getBoundingClientRect();
		drop = { id: block.id, after: event.clientY > box.top + box.height / 2 };
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

	const act = (block: Block, action: BlockAction) => preview?.send({ type: 'heron:action', id: block.id, action });

	const tools: { action: BlockAction; icon: 'up' | 'down' | 'copy' | 'eye' | 'eye-off' | 'trash'; label: string }[] = [
		{ action: 'up', icon: 'up', label: 'Move up' },
		{ action: 'down', icon: 'down', label: 'Move down' },
		{ action: 'duplicate', icon: 'copy', label: 'Duplicate' },
		{ action: 'hide', icon: 'eye-off', label: 'Hide' },
		{ action: 'delete', icon: 'trash', label: 'Delete' }
	];
</script>

{#snippet insert(at: number)}
	{#if preview && !preview.readonly}
		<div class="group/insert relative flex h-6 items-center justify-center" data-tool>
			<span class="absolute inset-x-0 top-1/2 h-px bg-accent opacity-0 transition-opacity group-hover/insert:opacity-60"></span>
			<button
				type="button"
				class="relative grid size-6 place-items-center rounded-full bg-accent text-accent-ink opacity-0 shadow-pop transition-[opacity,scale] group-hover/insert:opacity-100 hover:scale-110"
				onclick={() => preview.send({ type: 'heron:insert', at })}
				aria-label="Add a block here"
			>
				<Icon name="plus" size={14} />
			</button>
		</div>
	{/if}
{/snippet}

{#snippet frame(block: Block, index: number, inner: Snippet)}
	{@const selected = preview?.selected === block.id}
	{@const kind = kindOf(block.type)}
	<div
		data-edit={block.id}
		data-selected={selected || undefined}
		data-invalid={preview?.errors.includes(block.id) || undefined}
		class="group/frame {preview?.dragging === block.id ? 'opacity-40' : ''}"
		role="listitem"
		ondragover={(e) => over(e, block)}
		ondragleave={() => drop?.id === block.id && (drop = null)}
		ondrop={(e) => place(e, index)}
	>
		{#if drop?.id === block.id}
			<span class="pointer-events-none absolute inset-x-0 z-20 h-[3px] rounded-full bg-accent {drop.after ? '-bottom-[9px]' : '-top-[9px]'}"></span>
		{/if}
		<div class="pointer-events-none absolute -top-3 start-4 z-10 items-center gap-1.5 rounded-md bg-accent py-0.5 ps-1 pe-2 text-[11px] font-semibold text-accent-ink {selected ? 'flex' : 'hidden group-hover/frame:flex'}" dir="ltr">
			{#if !preview?.readonly}
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
			{kind.label}{block.hidden ? ' · hidden' : ''}
		</div>
		{#if selected && !preview?.readonly}
			<div class="absolute -top-4 end-3 z-10 flex items-center gap-0.5 rounded-lg border border-line-strong bg-surface p-0.5 shadow-pop" dir="ltr" data-tool>
				{#each tools as tool (tool.action)}
					<button
						type="button"
						class="grid size-7 place-items-center rounded-md text-ink-2 hover:bg-sunken hover:text-ink disabled:opacity-30 {tool.action === 'delete' ? 'hover:text-down' : ''}"
						title={tool.action === 'hide' && block.hidden ? 'Show' : tool.label}
						aria-label={tool.label}
						disabled={(tool.action === 'up' && index === 0) || (tool.action === 'down' && index === blocks.length - 1) || (tool.action === 'duplicate' && block.type === 'hero')}
						onclick={() => act(block, tool.action)}
					>
						<Icon name={tool.action === 'hide' && block.hidden ? 'eye' : tool.icon} size={14} />
					</button>
				{/each}
			</div>
		{/if}
		<div class="transition-opacity {block.hidden ? 'opacity-35' : ''}">{@render inner()}</div>
	</div>
	{@render insert(index + 1)}
{/snippet}

{#if !data}
	<div class="card p-10 text-center">
		<p class="font-semibold">{i18n.t('page.unavailable')}</p>
		<p class="mt-1 text-muted">{store.error ?? i18n.t('page.later')}</p>
		<button class="btn-secondary mt-5" onclick={() => store.load()}><Icon name="refresh" /> {i18n.t('page.retry')}</button>
	</div>
{:else if preview}
	<div role="list">
		{@render insert(0)}
		<StatusBlocks {data} {blocks} {recent} editing frame={frame} />
	</div>
{:else}
	<StatusBlocks {data} {blocks} {recent} />
{/if}
