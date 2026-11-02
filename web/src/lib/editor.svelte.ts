import { blank, defaultBlocks } from './blocks.ts';
import type { BlockAction } from './preview.svelte.ts';
import type { Block, BlockType, Page } from './types.ts';

const clone = (page: Page): Page => JSON.parse(JSON.stringify(page));

export class Editor {
	draft = $state<Page>({ title: '', blocks: [] });
	saved = $state('');
	past = $state<string[]>([]);
	future = $state<string[]>([]);
	selected = $state<string | null>(null);
	inserting = $state<number | null>(null);
	fields = $state<Record<string, string>>({});

	#key = '';
	#at = 0;

	constructor(page: Page) {
		this.reset(page);
	}

	get dirty() {
		return JSON.stringify(this.draft) !== this.saved;
	}

	get block() {
		return this.draft.blocks.find((b) => b.id === this.selected) ?? null;
	}

	get invalid() {
		const ids = new Set<string>();
		for (const key of Object.keys(this.fields)) {
			const [, index] = key.match(/^blocks\.(\d+)\./) ?? [];
			const block = index === undefined ? undefined : this.draft.blocks[Number(index)];
			if (block) ids.add(block.id);
		}
		return [...ids];
	}

	errorsFor(block: Block) {
		const index = this.draft.blocks.indexOf(block);
		const prefix = `blocks.${index}.`;
		return Object.fromEntries(
			Object.entries(this.fields)
				.filter(([key]) => key.startsWith(prefix))
				.map(([key, value]) => [key.slice(prefix.length), value])
		);
	}

	reset(page: Page) {
		const fresh = clone(page);
		if (!fresh.blocks?.length) fresh.blocks = defaultBlocks();
		this.draft = fresh;
		this.saved = JSON.stringify(fresh);
		this.fields = {};
		if (this.selected && !fresh.blocks.some((b) => b.id === this.selected)) this.selected = null;
	}

	change(mutate: (page: Page) => void, key = '') {
		const now = Date.now();
		if (!key || key !== this.#key || now - this.#at > 900) {
			this.past = [...this.past.slice(-99), JSON.stringify(this.draft)];
			this.future = [];
		}
		this.#key = key;
		this.#at = now;
		mutate(this.draft);
	}

	set<K extends keyof Block>(block: Block, field: K, value: Block[K]) {
		const id = block.id;
		this.change(
			(page) => {
				const target = page.blocks.find((b) => b.id === id);
				if (!target) return;
				if (value === '' || value === undefined || value === false) delete target[field];
				else target[field] = value;
			},
			`${id}:${String(field)}`
		);
		delete this.fields[`blocks.${this.draft.blocks.findIndex((b) => b.id === id)}.${String(field)}`];
	}

	setPage<K extends Exclude<keyof Page, 'blocks'>>(field: K, value: Page[K]) {
		this.change((page) => {
			page[field] = value;
		}, `page:${field}`);
		delete this.fields[field];
	}

	undo() {
		const prev = this.past.at(-1);
		if (prev === undefined) return;
		this.future = [...this.future, JSON.stringify(this.draft)];
		this.past = this.past.slice(0, -1);
		this.draft = JSON.parse(prev);
		this.fields = {};
		this.#key = '';
		this.keepSelection();
	}

	redo() {
		const next = this.future.at(-1);
		if (next === undefined) return;
		this.past = [...this.past, JSON.stringify(this.draft)];
		this.future = this.future.slice(0, -1);
		this.draft = JSON.parse(next);
		this.fields = {};
		this.#key = '';
		this.keepSelection();
	}

	keepSelection() {
		if (this.selected && !this.selected.startsWith('@') && !this.draft.blocks.some((b) => b.id === this.selected)) {
			this.selected = null;
		}
	}

	move(id: string, to: number) {
		this.change((page) => {
			const from = page.blocks.findIndex((b) => b.id === id);
			if (from < 0) return;
			const [block] = page.blocks.splice(from, 1);
			if (block) page.blocks.splice(Math.max(0, Math.min(to, page.blocks.length)), 0, block);
		});
		this.fields = {};
	}

	act(id: string, action: BlockAction) {
		const index = this.draft.blocks.findIndex((b) => b.id === id);
		const block = this.draft.blocks[index];
		if (!block) return;
		switch (action) {
			case 'up':
				if (index > 0) this.move(id, index - 1);
				break;
			case 'down':
				if (index < this.draft.blocks.length - 1) this.move(id, index + 1);
				break;
			case 'hide':
				this.set(block, 'hidden', !block.hidden);
				break;
			case 'duplicate': {
				if (block.type === 'hero') return;
				const copy = { ...$state.snapshot(block), id: blank(block.type, this.draft.blocks).id };
				this.change((page) => page.blocks.splice(index + 1, 0, copy));
				this.selected = copy.id;
				break;
			}
			case 'delete':
				this.change((page) => page.blocks.splice(index, 1));
				this.selected = null;
				this.fields = {};
				break;
		}
	}

	insert(type: BlockType, at = this.inserting ?? this.draft.blocks.length) {
		const block = blank(type, this.draft.blocks);
		this.change((page) => page.blocks.splice(at, 0, block));
		this.inserting = null;
		this.selected = block.id;
		this.fields = {};
	}

	retype(block: Block, type: BlockType) {
		const id = block.id;
		const fresh = blank(type, this.draft.blocks);
		this.change((page) => {
			const index = page.blocks.findIndex((b) => b.id === id);
			if (index < 0) return;
			page.blocks[index] = { ...fresh, id, hidden: page.blocks[index]?.hidden };
		});
		this.fields = {};
	}

	discard() {
		this.past = [...this.past, JSON.stringify(this.draft)];
		this.future = [];
		this.draft = JSON.parse(this.saved);
		this.fields = {};
		this.inserting = null;
		this.keepSelection();
	}

	restoreDefaults() {
		this.change((page) => {
			page.blocks = defaultBlocks();
		});
		this.selected = null;
		this.fields = {};
	}
}
