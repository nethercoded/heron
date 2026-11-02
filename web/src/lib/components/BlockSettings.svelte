<script lang="ts">
	import { blockKinds, toneColor } from '#lib/blocks.ts';
	import type { Editor } from '#lib/editor.svelte.ts';
	import type { Block, BlockType, Tone } from '#lib/types.ts';
	import Field from './Field.svelte';
	import Icon from './Icon.svelte';
	import Toggle from './Toggle.svelte';

	let {
		editor,
		block,
		groups,
		readonly = false
	}: { editor: Editor; block: Block; groups: { name: string; count: number }[]; readonly?: boolean } = $props();

	const errors = $derived(editor.errorsFor(block));
	const otherHero = $derived(editor.draft.blocks.some((b) => b.type === 'hero' && b.id !== block.id));
	const claimed = $derived(
		new Set(
			editor.draft.blocks.filter((b) => b.type === 'monitors' && b.group && b.id !== block.id).map((b) => b.group)
		)
	);
	const missingGroup = $derived(!!block.group && !groups.some((g) => g.name === block.group));

	const tones: { value: Tone; label: string }[] = [
		{ value: 'info', label: 'Info' },
		{ value: 'success', label: 'Good news' },
		{ value: 'warning', label: 'Warning' },
		{ value: 'danger', label: 'Problem' }
	];

	const defaults: Partial<Record<BlockType, string>> = {
		incidents: 'Happening now',
		maintenance: 'Planned work',
		history: 'Last 7 days'
	};

	const text = (field: 'title' | 'body') => (event: Event) =>
		editor.set(block, field, (event.currentTarget as HTMLInputElement).value);
</script>

{#snippet segmented(options: { value: string | number; label: string }[], current: string | number, pick: (value: string | number) => void, label: string)}
	<div class="grid auto-cols-fr grid-flow-col gap-1 rounded-ctl bg-canvas p-1" role="radiogroup" aria-label={label}>
		{#each options as option (option.value)}
			<button
				type="button"
				role="radio"
				aria-checked={current === option.value}
				disabled={readonly}
				class="h-8 rounded-lg text-[13px] font-medium transition-colors {current === option.value ? 'bg-ink text-canvas' : 'text-muted hover:text-ink'}"
				onclick={() => pick(option.value)}
			>
				{option.label}
			</button>
		{/each}
	</div>
{/snippet}

<fieldset class="space-y-5" disabled={readonly}>
	<Field label="This block shows" id="block-type">
		<select
			id="block-type"
			class="input-select"
			value={block.type}
			onchange={(e) => editor.retype(block, e.currentTarget.value as BlockType)}
		>
			{#each blockKinds as kind (kind.type)}
				<option value={kind.type} disabled={kind.type === 'hero' && otherHero}>{kind.label}</option>
			{/each}
		</select>
	</Field>

	{#if block.type === 'hero'}
		<div>
			<span class="label">Look</span>
			{@render segmented(
				[
					{ value: 'full', label: 'With the heron' },
					{ value: 'compact', label: 'Compact' }
				],
				block.style === 'compact' ? 'compact' : 'full',
				(v) => editor.set(block, 'style', v === 'full' ? '' : String(v)),
				'Look'
			)}
		</div>
		<Field label="Subtitle" id="hero-description" hint="Under the headline. Empty shows a line that matches the current status.">
			<input
				id="hero-description"
				class="input"
				placeholder="Every service is responding normally."
				value={editor.draft.description ?? ''}
				oninput={(e) => editor.setPage('description', e.currentTarget.value)}
			/>
		</Field>
		<Toggle
			label="Show the numbers"
			description="Uptime, services online and the time of the last check"
			checked={block.stats !== 'hide'}
			onchange={(on) => editor.set(block, 'stats', on ? '' : 'hide')}
		/>
	{:else if block.type === 'notice'}
		<div>
			<span class="label">Tone</span>
			<div class="grid grid-cols-2 gap-1.5">
				{#each tones as tone (tone.value)}
					{@const on = (block.tone ?? 'info') === tone.value}
					<button
						type="button"
						class="flex h-9 items-center gap-2 rounded-ctl border px-3 text-[13px] font-medium transition-colors {on ? 'border-ink text-ink' : 'border-line text-muted hover:border-line-strong hover:text-ink'}"
						aria-pressed={on}
						onclick={() => editor.set(block, 'tone', tone.value === 'info' ? undefined : tone.value)}
					>
						<span class="sq" style="--c: {toneColor[tone.value]}; --s: 8px"></span>
						{tone.label}
					</button>
				{/each}
			</div>
		</div>
		<Field label="Heading" id="notice-title" error={errors.title}>
			<input id="notice-title" class="input" maxlength="80" value={block.title ?? ''} oninput={text('title')} aria-invalid={!!errors.title} />
		</Field>
		<Field label="Text" id="notice-body" hint="Line breaks are kept" error={errors.body}>
			<textarea id="notice-body" class="input-area font-sans text-[14px]" rows="5" maxlength="2000" value={block.body ?? ''} oninput={text('body')} aria-invalid={!!errors.body}></textarea>
		</Field>
	{:else if block.type === 'monitors'}
		<Field
			label="Group"
			id="monitors-group"
			hint={block.group ? 'Only this group, in the order from the monitor list' : 'Every group that has no block of its own'}
			error={errors.group}
		>
			<select id="monitors-group" class="input-select" value={block.group ?? ''} onchange={(e) => editor.set(block, 'group', e.currentTarget.value)}>
				<option value="">All other groups</option>
				{#each groups as group (group.name)}
					<option value={group.name} disabled={claimed.has(group.name)}>
						{group.name} · {group.count}{claimed.has(group.name) ? ' · has a block' : ''}
					</option>
				{/each}
				{#if missingGroup}
					<option value={block.group}>{block.group} · no public monitors</option>
				{/if}
			</select>
		</Field>
		{#if block.group}
			<Field label="Heading" id="monitors-title" hint="Empty uses the group name" error={errors.title}>
				<input id="monitors-title" class="input" maxlength="80" placeholder={block.group} value={block.title ?? ''} oninput={text('title')} />
			</Field>
		{/if}
		<div>
			<span class="label">Rows</span>
			{@render segmented(
				[
					{ value: 'bars', label: 'With uptime bars' },
					{ value: 'list', label: 'Compact list' }
				],
				block.style === 'list' ? 'list' : 'bars',
				(v) => editor.set(block, 'style', v === 'bars' ? '' : String(v)),
				'Rows'
			)}
		</div>
	{:else if block.type === 'history'}
		<div>
			<span class="label">Days</span>
			{@render segmented(
				[
					{ value: 7, label: '7' },
					{ value: 14, label: '14' },
					{ value: 30, label: '30' }
				],
				block.days || 7,
				(v) => editor.set(block, 'days', Number(v)),
				'Days'
			)}
		</div>
		<Field label="Heading" id="history-title" hint="Empty is translated for every visitor" error={errors.title}>
			<input id="history-title" class="input" maxlength="80" placeholder="Last {block.days || 7} days" value={block.title ?? ''} oninput={text('title')} />
		</Field>
	{:else}
		<p class="rounded-ctl bg-canvas px-3.5 py-3 text-[13px] text-muted">
			<Icon name="info" size={14} class="me-1 inline -translate-y-px" />
			{block.type === 'incidents'
				? 'Visitors only see this block while an incident is open. Until then it takes no space.'
				: 'Visitors only see this block while maintenance is planned or running.'}
		</p>
		<Field label="Heading" id="section-title" hint="Empty is translated for every visitor" error={errors.title}>
			<input id="section-title" class="input" maxlength="80" placeholder={defaults[block.type]} value={block.title ?? ''} oninput={text('title')} />
		</Field>
	{/if}

	<div class="border-t border-line pt-5">
		<Toggle
			label="Show on the page"
			description={block.hidden ? 'Hidden from visitors, kept here so you can bring it back' : 'Visitors see this block'}
			checked={!block.hidden}
			onchange={(on) => editor.set(block, 'hidden', !on)}
		/>
	</div>
</fieldset>
