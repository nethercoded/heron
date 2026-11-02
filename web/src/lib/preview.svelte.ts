import { createContext } from 'svelte';
import type { Attachment } from 'svelte/attachments';
import type { Locale } from './i18n.svelte.ts';
import type { Page } from './types.ts';

export type BlockAction = 'up' | 'down' | 'duplicate' | 'hide' | 'delete';

export type ToPreview = {
	type: 'heron:draft';
	page: Page;
	selected: string | null;
	errors: string[];
	locale: Locale;
	readonly: boolean;
	inserting: number | null;
};

export type FromPreview =
	| { type: 'heron:ready' }
	| { type: 'heron:select'; id: string | null }
	| { type: 'heron:move'; id: string; to: number }
	| { type: 'heron:action'; id: string; action: BlockAction }
	| { type: 'heron:insert'; at: number }
	| { type: 'heron:key'; key: string; shift: boolean }
	| { type: 'heron:text'; id: string; field: TextField; value: string }
	| { type: 'heron:resize'; id: string; width: number };

export type TextField = 'title' | 'body' | 'description';

export type InlineEdit = {
	selected: string | null;
	text: (id: string, field: TextField, value: string) => void;
};

export const editable =
	(value: string, commit: (value: string) => void, multiline = false): Attachment<HTMLElement> =>
	(el) => {
		if (document.activeElement !== el && el.textContent !== value) el.textContent = value;
		const input = () => commit((el.textContent ?? '').replace(/\n$/, ''));
		const key = (event: KeyboardEvent) => {
			if (event.key === 'Enter' && !multiline) {
				event.preventDefault();
				el.blur();
			}
		};
		el.addEventListener('input', input);
		el.addEventListener('keydown', key);
		return () => {
			el.removeEventListener('input', input);
			el.removeEventListener('keydown', key);
		};
	};

export const inPreview = (url: { searchParams: { get(name: string): string | null } }) =>
	url.searchParams.get('edit') === '1' && window.self !== window.top;

export class Preview {
	page = $state<Page | null>(null);
	selected = $state<string | null>(null);
	errors = $state<string[]>([]);
	locale = $state<Locale | null>(null);
	readonly = $state(true);
	dragging = $state<string | null>(null);
	inserting = $state<number | null>(null);

	constructor() {
		$effect(() => {
			const receive = (event: MessageEvent<ToPreview>) => {
				if (event.origin !== location.origin || event.source !== window.parent) return;
				if (event.data?.type !== 'heron:draft') return;
				const { page, selected, errors, locale, readonly, inserting } = event.data;
				this.page = page;
				this.errors = errors;
				this.locale = locale;
				this.readonly = readonly;
				this.inserting = inserting;
				if (selected !== this.selected) {
					this.selected = selected;
					if (selected) requestAnimationFrame(() => this.reveal(selected));
				}
			};
			const keys = (event: KeyboardEvent) => {
				const typing = (event.target as HTMLElement | null)?.closest('input, textarea, select, [contenteditable]');
				const mod = event.ctrlKey || event.metaKey;
				if (typing) {
					if (event.key === 'Escape') (typing as HTMLElement).blur();
					if (!(mod && event.key.toLowerCase() === 's')) return;
				}
				if (
					(mod && ['z', 'y', 's'].includes(event.key.toLowerCase())) ||
					['Delete', 'Backspace', 'Escape'].includes(event.key) ||
					(event.altKey && event.key.startsWith('Arrow'))
				) {
					event.preventDefault();
					this.send({
						type: 'heron:key',
						key: (mod ? 'mod+' : event.altKey ? 'alt+' : '') + event.key.toLowerCase(),
						shift: event.shiftKey
					});
				}
			};
			window.addEventListener('message', receive);
			window.addEventListener('keydown', keys);
			this.send({ type: 'heron:ready' });
			return () => {
				window.removeEventListener('message', receive);
				window.removeEventListener('keydown', keys);
			};
		});
	}

	send(message: FromPreview) {
		window.parent.postMessage(message, location.origin);
	}

	reveal(id: string) {
		const el = document.querySelector<HTMLElement>(`[data-edit="${CSS.escape(id)}"]`);
		if (!el) return;
		const box = el.getBoundingClientRect();
		if (box.top < 0 || box.top > innerHeight - 80) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
	}

	click(event: MouseEvent) {
		const target = event.target as Element;
		if (target.closest('[data-tool]')) return;
		event.preventDefault();
		event.stopPropagation();
		const id = target.closest<HTMLElement>('[data-edit]')?.dataset.edit ?? null;
		this.send({ type: 'heron:select', id });
	}
}

export const [getPreview, setPreview, hasPreview] = createContext<Preview>();
