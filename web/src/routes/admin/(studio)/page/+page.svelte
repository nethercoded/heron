<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { ApiError, api, message } from '#lib/api.ts';
	import { blockKinds, kindOf } from '#lib/blocks.ts';
	import BlockSettings from '#lib/components/BlockSettings.svelte';
	import Field from '#lib/components/Field.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import { Editor } from '#lib/editor.svelte.ts';
	import { type Locale, locales } from '#lib/i18n.svelte.ts';
	import type { FromPreview, ToPreview } from '#lib/preview.svelte.ts';
	import { canEdit } from '#lib/team.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Block, Page } from '#lib/types.ts';
	import { beforeNavigate } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const editor = new Editor(untrack(() => data.page));
	const readonly = $derived(!canEdit(data.user.role));

	let device = $state<'desktop' | 'phone'>('desktop');
	let locale = $state<Locale>('en');
	let saving = $state(false);

	let frame = $state<HTMLIFrameElement>();
	let ready = $state(false);
	let cw = $state(0);
	let ch = $state(0);

	const devices = [
		['desktop', 'monitor', 'Desktop'],
		['phone', 'phone', 'Phone']
	] as const;
	const phone = { width: 390, height: 844 };
	const desktop = 1280;
	const scale = $derived(
		device === 'phone'
			? Math.min(1, (ch - 48) / phone.height, (cw - 32) / phone.width)
			: Math.min(1, (cw - 32) / desktop)
	);
	const box = $derived(
		device === 'phone'
			? {
					width: phone.width,
					height: phone.height,
					left: (cw - phone.width * scale) / 2,
					top: Math.max(16, (ch - phone.height * scale) / 2)
				}
			: {
					width: desktop,
					height: Math.max(0, (ch - 16) / scale),
					left: Math.max(16, (cw - desktop * scale) / 2),
					top: 16
				}
	);

	const hero = $derived(editor.draft.blocks.some((b) => b.type === 'hero'));
	const pageField = $derived(
		editor.selected === '@brand' ? 'page-title' : editor.selected === '@footer' ? 'page-footer' : ''
	);

	$effect(() => {
		const message: ToPreview = {
			type: 'heron:draft',
			page: $state.snapshot(editor.draft),
			selected: editor.selected,
			errors: editor.invalid,
			locale,
			readonly
		};
		if (ready) frame?.contentWindow?.postMessage(message, location.origin);
	});

	$effect(() => {
		if (!pageField) return;
		tick().then(() => document.getElementById(pageField)?.focus());
	});

	$effect(() => {
		const receive = (event: MessageEvent<FromPreview>) => {
			if (event.origin !== location.origin || !frame || event.source !== frame.contentWindow) return;
			const msg = event.data;
			switch (msg?.type) {
				case 'heron:ready':
					ready = false;
					tick().then(() => (ready = true));
					break;
				case 'heron:select':
					editor.inserting = null;
					editor.selected = msg.id;
					break;
				case 'heron:move':
					if (!readonly) editor.move(msg.id, msg.to);
					break;
				case 'heron:action':
					if (!readonly) editor.act(msg.id, msg.action);
					break;
				case 'heron:insert':
					if (readonly) break;
					editor.selected = null;
					editor.inserting = msg.at;
					break;
				case 'heron:key':
					shortcut(msg.key, msg.shift);
					break;
			}
		};
		const keys = (event: KeyboardEvent) => {
			const mod = event.ctrlKey || event.metaKey;
			const typing = (event.target as HTMLElement | null)?.closest('input, textarea, select');
			const key = (mod ? 'mod+' : event.altKey ? 'alt+' : '') + event.key.toLowerCase();
			if (typing && key !== 'mod+s' && key !== 'escape') return;
			if (shortcut(key, event.shiftKey)) event.preventDefault();
		};
		const leave = (event: BeforeUnloadEvent) => {
			if (editor.dirty) event.preventDefault();
		};
		window.addEventListener('message', receive);
		window.addEventListener('keydown', keys);
		window.addEventListener('beforeunload', leave);
		return () => {
			window.removeEventListener('message', receive);
			window.removeEventListener('keydown', keys);
			window.removeEventListener('beforeunload', leave);
		};
	});

	beforeNavigate(({ cancel, type }) => {
		if (type === 'leave' || !editor.dirty) return;
		if (!confirm('Leave the editor? Changes you have not published will be lost.')) cancel();
	});

	function shortcut(key: string, shift: boolean) {
		if (key === 'mod+s') {
			publish();
			return true;
		}
		if (key === 'escape') {
			editor.selected = null;
			editor.inserting = null;
			(document.activeElement as HTMLElement | null)?.blur();
			return true;
		}
		if (readonly) return false;
		const block = editor.block;
		switch (key) {
			case 'mod+z':
				shift ? editor.redo() : editor.undo();
				return true;
			case 'mod+y':
				editor.redo();
				return true;
			case 'delete':
			case 'backspace':
				if (block) editor.act(block.id, 'delete');
				return !!block;
			case 'alt+arrowup':
				if (block) editor.act(block.id, 'up');
				return !!block;
			case 'alt+arrowdown':
				if (block) editor.act(block.id, 'down');
				return !!block;
		}
		return false;
	}

	async function publish() {
		if (readonly || saving || !editor.dirty) return;
		saving = true;
		try {
			const page = await api.put<Page>('/admin/page', {
				...editor.draft,
				history_days: Number(editor.draft.history_days) || 90
			});
			editor.reset(page);
			toast.success('Published, visitors see the new page');
		} catch (err) {
			if (err instanceof ApiError) {
				editor.fields = err.fields;
				const [first] = editor.invalid;
				editor.inserting = null;
				editor.selected =
					first ?? (Object.keys(err.fields).some((k) => !k.startsWith('blocks.')) ? '@page' : editor.selected);
			}
			toast.error(message(err));
		} finally {
			saving = false;
		}
	}

	function discard() {
		if (confirm('Throw away the changes you have not published?')) editor.discard();
	}

	function summary(block: Block) {
		switch (block.type) {
			case 'hero':
				return block.style === 'compact' ? 'Compact' : 'With the heron';
			case 'notice':
				return block.title || block.body || 'Empty';
			case 'monitors':
				return block.group || 'All other groups';
			case 'history':
				return `${block.days || 7} days`;
			default:
				return 'Only while there is something to show';
		}
	}

	let dragging = $state<string | null>(null);
	let target = $state<{ index: number; after: boolean } | null>(null);

	function dragOver(event: DragEvent, index: number) {
		if (!dragging) return;
		event.preventDefault();
		const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
		target = { index, after: event.clientY > rect.top + rect.height / 2 };
	}

	function dropOn(event: DragEvent) {
		event.preventDefault();
		if (!dragging || !target) return;
		const from = editor.draft.blocks.findIndex((b) => b.id === dragging);
		const at = target.after ? target.index + 1 : target.index;
		const to = at > from ? at - 1 : at;
		if (to !== from) editor.move(dragging, to);
		dragging = null;
		target = null;
	}
</script>

<svelte:head>
	<title>Status page editor · heron</title>
</svelte:head>

{#snippet toolButton(icon: 'undo' | 'redo', label: string, disabled: boolean, run: () => void)}
	<button type="button" class="grid size-9 place-items-center rounded-lg text-ink-2 transition-colors hover:bg-sunken hover:text-ink disabled:text-faint disabled:hover:bg-transparent" title={label} aria-label={label} {disabled} onclick={run}>
		<Icon name={icon} size={17} />
	</button>
{/snippet}

<div class="flex h-dvh flex-col bg-canvas">
	<header class="flex h-[58px] shrink-0 items-center gap-2 border-b border-line px-3">
		<a href="/admin/settings" class="flex items-center gap-2 rounded-lg py-1.5 ps-1.5 pe-2.5 text-ink-2 transition-colors hover:bg-sunken hover:text-ink">
			<Icon name="back" size={17} />
			<Logo size={20} class="text-ink" />
		</a>
		<div class="min-w-0 border-s border-line ps-3">
			<p class="truncate text-[14px] leading-tight font-semibold">Status page</p>
			<p class="flex items-center gap-1.5 text-[12px] text-faint">
				<span class="sq" style="--c: var(--color-{readonly ? 'faint' : editor.dirty ? 'degraded' : 'up'}); --s: 6px"></span>
				{readonly ? 'View only' : editor.dirty ? 'Unpublished changes' : 'Published'}
			</p>
		</div>

		<div class="mx-auto hidden items-center gap-2 md:flex">
			<div class="flex gap-0.5 rounded-ctl bg-surface p-1" role="radiogroup" aria-label="Preview size">
				{#each devices as [value, icon, label] (value)}
					<button
						type="button"
						role="radio"
						aria-checked={device === value}
						title={label}
						class="flex h-8 items-center gap-1.5 rounded-lg px-2.5 text-[13px] font-medium transition-colors {device === value ? 'bg-ink text-canvas' : 'text-muted hover:text-ink'}"
						onclick={() => (device = value)}
					>
						<Icon name={icon} size={15} />
						<span class="hidden xl:inline">{label}</span>
					</button>
				{/each}
			</div>
			<select class="input-select h-10 w-auto py-0 text-[13px]" bind:value={locale} aria-label="Preview language">
				{#each locales as l (l.code)}<option value={l.code}>{l.name}</option>{/each}
			</select>
		</div>

		<div class="ms-auto flex items-center gap-1 md:ms-0">
			{#if !readonly}
				{@render toolButton('undo', 'Undo (Ctrl+Z)', editor.past.length === 0, () => editor.undo())}
				{@render toolButton('redo', 'Redo (Ctrl+Shift+Z)', editor.future.length === 0, () => editor.redo())}
			{/if}
			<a href="/" target="_blank" class="grid size-9 place-items-center rounded-lg text-ink-2 transition-colors hover:bg-sunken hover:text-ink" title="Open the live page" aria-label="Open the live page">
				<Icon name="external" size={16} />
			</a>
			{#if !readonly}
				{#if editor.dirty}
					<button type="button" class="btn-ghost btn-sm hidden sm:inline-flex" onclick={discard}>Discard</button>
				{/if}
				<button type="button" class="btn-primary btn-sm ms-1" disabled={!editor.dirty || saving} onclick={publish}>
					{saving ? 'Publishing…' : 'Publish'}
				</button>
			{/if}
		</div>
	</header>

	<div class="grid min-h-0 flex-1 grid-rows-[minmax(380px,1fr)_auto] lg:grid-cols-[minmax(0,1fr)_360px] lg:grid-rows-1">
		<div class="relative min-h-0 overflow-hidden" bind:clientWidth={cw} bind:clientHeight={ch} style="background: radial-gradient(circle at 1px 1px, var(--color-line) 1px, transparent 0) 0 0 / 22px 22px">
			{#if cw > 0}
				<iframe
					bind:this={frame}
					src="/?edit=1"
					title="Status page preview"
					class="absolute origin-top-left border border-line-strong bg-canvas transition-[width,height,left,top] duration-300 {device === 'phone' ? 'rounded-[34px] border-[6px] border-raised' : 'rounded-t-card'}"
					style="width: {box.width}px; height: {box.height}px; left: {box.left}px; top: {box.top}px; transform: scale({scale})"
				></iframe>
			{/if}
		</div>

		<aside class="on-sheet min-h-0 overflow-y-auto border-t border-line bg-surface lg:border-t-0 lg:border-s">
			{#if editor.inserting !== null}
				<div class="p-5">
					<div class="mb-4 flex items-center justify-between">
						<h2 class="font-semibold">Add a block</h2>
						<button type="button" class="btn-ghost btn-sm" onclick={() => (editor.inserting = null)}>Cancel</button>
					</div>
					<div class="space-y-1.5">
						{#each blockKinds as kind (kind.type)}
							{@const taken = kind.type === 'hero' && hero}
							<button
								type="button"
								disabled={taken}
								class="flex w-full items-start gap-3 rounded-ctl border border-line p-3 text-start transition-colors hover:border-ink disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:border-line"
								onclick={() => editor.insert(kind.type)}
							>
								<span class="grid size-9 shrink-0 place-items-center rounded-lg bg-canvas text-ink-2"><Icon name={kind.icon as 'info'} size={17} /></span>
								<span class="min-w-0">
									<span class="block text-[14px] font-semibold">{kind.label}</span>
									<span class="block text-[12.5px] text-faint">{taken ? 'The page already has one' : kind.hint}</span>
								</span>
							</button>
						{/each}
					</div>
				</div>
			{:else if editor.block}
				{@const block = editor.block}
				{@const kind = kindOf(block.type)}
				<div class="sticky top-0 z-10 flex items-center gap-2 border-b border-line bg-surface px-3 py-2.5">
					<button type="button" class="grid size-8 place-items-center rounded-lg text-ink-2 hover:bg-sunken hover:text-ink" aria-label="All blocks" title="All blocks (Esc)" onclick={() => (editor.selected = null)}>
						<Icon name="back" size={16} />
					</button>
					<span class="grid size-8 place-items-center rounded-lg bg-canvas text-ink-2"><Icon name={kind.icon as 'info'} size={15} /></span>
					<h2 class="min-w-0 flex-1 truncate text-[14px] font-semibold">{kind.label}</h2>
					{#if !readonly}
						<button type="button" class="grid size-8 place-items-center rounded-lg text-ink-2 hover:bg-sunken hover:text-ink disabled:opacity-30" title="Duplicate" aria-label="Duplicate" disabled={block.type === 'hero'} onclick={() => editor.act(block.id, 'duplicate')}>
							<Icon name="copy" size={15} />
						</button>
						<button type="button" class="grid size-8 place-items-center rounded-lg text-ink-2 hover:bg-sunken hover:text-down" title="Delete (Del)" aria-label="Delete" onclick={() => editor.act(block.id, 'delete')}>
							<Icon name="trash" size={15} />
						</button>
					{/if}
				</div>
				<div class="p-5">
					{#key block.id}
						<BlockSettings {editor} {block} groups={data.groups} {readonly} />
					{/key}
				</div>
			{:else}
				<div class="space-y-7 p-5">
					<fieldset class="space-y-4" disabled={readonly}>
						<h2 class="font-semibold">Page</h2>
						<Field label="Title" id="page-title" hint="Next to the logo and in the browser tab" error={editor.fields.title}>
							<input id="page-title" class="input" maxlength="80" value={editor.draft.title} oninput={(e) => editor.setPage('title', e.currentTarget.value)} aria-invalid={!!editor.fields.title} />
						</Field>
						<Field label="Logo URL" id="page-logo" hint="Empty shows the heron" error={editor.fields.logo_url}>
							<input id="page-logo" class="input" placeholder="https://example.com/logo.svg" value={editor.draft.logo_url ?? ''} oninput={(e) => editor.setPage('logo_url', e.currentTarget.value)} aria-invalid={!!editor.fields.logo_url} />
						</Field>
						<Field label="Website" id="page-home" hint="Where the logo leads" error={editor.fields.home_url}>
							<input id="page-home" class="input" placeholder="https://example.com" value={editor.draft.home_url ?? ''} oninput={(e) => editor.setPage('home_url', e.currentTarget.value)} aria-invalid={!!editor.fields.home_url} />
						</Field>
						<Field label="Footer" id="page-footer" hint="Empty shows a copyright line">
							<input id="page-footer" class="input" value={editor.draft.footer ?? ''} oninput={(e) => editor.setPage('footer', e.currentTarget.value)} />
						</Field>
						<Field label="Uptime history" id="page-history" hint="Days in every uptime bar" error={editor.fields.history_days}>
							<select id="page-history" class="input-select" value={editor.draft.history_days ?? 90} onchange={(e) => editor.setPage('history_days', Number(e.currentTarget.value))}>
								{#each [30, 60, 90, 180, 365] as days (days)}<option value={days}>{days} days</option>{/each}
							</select>
						</Field>
					</fieldset>

					<section>
						<div class="mb-3 flex items-baseline justify-between">
							<h2 class="font-semibold">Blocks</h2>
							<span class="text-[12px] text-faint">Drag to reorder</span>
						</div>
						<ol class="space-y-1" ondragover={(e) => dragging && e.preventDefault()} ondrop={dropOn}>
							{#each editor.draft.blocks as block, index (block.id)}
								{@const kind = kindOf(block.type)}
								{@const invalid = editor.invalid.includes(block.id)}
								<li
									class="relative"
									draggable={!readonly}
									ondragstart={(e) => {
										dragging = block.id;
										e.dataTransfer?.setData('text/plain', block.id);
									}}
									ondragend={() => {
										dragging = null;
										target = null;
									}}
									ondragover={(e) => dragOver(e, index)}
								>
									{#if target?.index === index && dragging && dragging !== block.id}
										<span class="pointer-events-none absolute inset-x-1 z-10 h-0.5 rounded-full bg-accent {target.after ? '-bottom-[3px]' : '-top-[3px]'}"></span>
									{/if}
									<div class="group flex items-center gap-1 rounded-ctl border bg-canvas transition-colors {invalid ? 'border-down' : 'border-line hover:border-line-strong'} {dragging === block.id ? 'opacity-40' : ''}">
										<button type="button" class="flex min-w-0 flex-1 items-center gap-2.5 py-2 ps-2 text-start" onclick={() => (editor.selected = block.id)}>
											{#if !readonly}<Icon name="grip" size={14} class="cursor-grab text-faint" />{/if}
											<span class="grid size-7 shrink-0 place-items-center rounded-md bg-surface {block.hidden ? 'text-faint' : 'text-ink-2'}"><Icon name={kind.icon as 'info'} size={14} /></span>
											<span class="min-w-0 {block.hidden ? 'opacity-50' : ''}">
												<span class="block truncate text-[13.5px] font-medium">{kind.label}</span>
												<span class="block truncate text-[12px] text-faint">{summary(block)}</span>
											</span>
										</button>
										{#if !readonly}
											<button
												type="button"
												class="me-1 grid size-8 shrink-0 place-items-center rounded-lg transition-colors hover:bg-surface {block.hidden ? 'text-ink-2' : 'text-faint opacity-0 group-hover:opacity-100 focus-visible:opacity-100'}"
												title={block.hidden ? 'Show on the page' : 'Hide from the page'}
												aria-label={block.hidden ? 'Show on the page' : 'Hide from the page'}
												onclick={() => editor.act(block.id, 'hide')}
											>
												<Icon name={block.hidden ? 'eye-off' : 'eye'} size={15} />
											</button>
										{/if}
									</div>
								</li>
							{/each}
						</ol>
						{#if !readonly}
							<button type="button" class="btn-secondary mt-3 w-full" onclick={() => (editor.inserting = editor.draft.blocks.length)}>
								<Icon name="plus" /> Add a block
							</button>
							<button type="button" class="mt-4 w-full text-center text-[12.5px] text-faint hover:text-ink" onclick={() => confirm('Go back to the default layout? You can undo this.') && editor.restoreDefaults()}>
								Restore the default layout
							</button>
						{/if}
					</section>

					<p class="border-t border-line pt-5 text-[12px] leading-relaxed text-faint">
						Click anything on the page to edit it. Hover between blocks to add one.
						<span class="kbd">Alt ↑ ↓</span> moves the selected block, <span class="kbd">Del</span> removes it,
						<span class="kbd">Ctrl S</span> publishes.
					</p>
				</div>
			{/if}
		</aside>
	</div>
</div>
