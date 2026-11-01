export type Tone = 'success' | 'error' | 'info';

export interface Toast {
	id: number;
	tone: Tone;
	text: string;
}

class Toasts {
	items = $state<Toast[]>([]);
	#next = 0;

	push(tone: Tone, text: string, ttl = 4000) {
		const id = ++this.#next;
		this.items.push({ id, tone, text });
		setTimeout(() => this.dismiss(id), ttl);
	}

	success = (text: string) => this.push('success', text);
	error = (text: string) => this.push('error', text, 6000);
	info = (text: string) => this.push('info', text);

	dismiss(id: number) {
		this.items = this.items.filter((t) => t.id !== id);
	}
}

export const toast = new Toasts();
